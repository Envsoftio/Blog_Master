package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const reviewNotificationLockDuration = 30 * time.Second

type ReviewAssignmentNotificationDelivery struct {
	ID              string
	ProjectID       string
	ProjectName     string
	AssignmentID    string
	ArticleID       string
	ArticleTitle    string
	RevisionID      string
	RecipientUserID string
	RecipientEmail  string
	AssignmentType  string
	DueAt           string
	AttemptCount    int
	MaxAttempts     int
	CreatedAt       string
}

func (s *Store) ClaimReviewAssignmentNotifications(ctx context.Context, workerID string, now time.Time, limit int) ([]ReviewAssignmentNotificationDelivery, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return nil, fmt.Errorf("%w: notification worker ID is required", ErrValidation)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("%w: notification batch size must be positive", ErrValidation)
	}
	now = now.UTC()
	nowValue := now.Format(timeFormat)
	lockUntil := now.Add(reviewNotificationLockDuration).Format(timeFormat)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE review_assignment_notifications
		SET status = 'suppressed',
		    locked_by = NULL,
		    locked_until = NULL
			WHERE (
			    status IN ('queued','retry')
			    OR (status = 'processing' AND locked_until <= ?)
			  )
			  AND (
		    NOT EXISTS (
		      SELECT 1
		      FROM review_assignments assignment
		      WHERE assignment.id = review_assignment_notifications.assignment_id
		        AND assignment.status = 'open'
		    )
		    OR NOT EXISTS (
		      SELECT 1
		      FROM project_memberships membership
		      JOIN users user ON user.id = membership.user_id
		      WHERE membership.project_id = review_assignment_notifications.project_id
		        AND membership.user_id = review_assignment_notifications.recipient_user_id
		        AND membership.status = 'active'
		        AND user.status = 'active'
		    )
		  )
	`, nowValue); err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, `
		UPDATE review_assignment_notifications
		SET status = 'processing',
		    attempt_count = attempt_count + 1,
		    locked_by = ?,
		    locked_until = ?
		WHERE id IN (
		  SELECT id
			  FROM review_assignment_notifications
			  WHERE attempt_count < max_attempts
			    AND (
			      (status IN ('queued','retry') AND next_attempt_at <= ?)
			      OR (status = 'processing' AND locked_until <= ?)
			    )
			    AND EXISTS (
			      SELECT 1
			      FROM review_assignments assignment
			      WHERE assignment.id = review_assignment_notifications.assignment_id
			        AND assignment.status = 'open'
			    )
			    AND EXISTS (
			      SELECT 1
			      FROM project_memberships membership
			      JOIN users user ON user.id = membership.user_id
			      WHERE membership.project_id = review_assignment_notifications.project_id
			        AND membership.user_id = review_assignment_notifications.recipient_user_id
			        AND membership.status = 'active'
			        AND user.status = 'active'
			    )
		  ORDER BY next_attempt_at, id
		  LIMIT ?
		)
		RETURNING id
	`, workerID, lockUntil, nowValue, nowValue, limit)
	if err != nil {
		return nil, err
	}
	var notificationIDs []string
	for rows.Next() {
		var notificationID string
		if err := rows.Scan(&notificationID); err != nil {
			rows.Close()
			return nil, err
		}
		notificationIDs = append(notificationIDs, notificationID)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	deliveries := make([]ReviewAssignmentNotificationDelivery, 0, len(notificationIDs))
	for _, notificationID := range notificationIDs {
		delivery, err := getReviewAssignmentNotificationTx(ctx, tx, notificationID, workerID)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return deliveries, nil
}

func (s *Store) MarkReviewAssignmentNotificationDelivered(ctx context.Context, notificationID, workerID string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE review_assignment_notifications
		SET status = 'delivered',
		    delivered_at = ?,
		    locked_by = NULL,
		    locked_until = NULL,
		    last_error_safe_message = NULL
		WHERE id = ? AND status = 'processing' AND locked_by = ?
	`, now.UTC().Format(timeFormat), notificationID, workerID)
	if err != nil {
		return err
	}
	return requireOneRowAffected(result)
}

func (s *Store) MarkReviewAssignmentNotificationFailed(ctx context.Context, delivery ReviewAssignmentNotificationDelivery, workerID string, now time.Time, deliveryErr error) error {
	status := "retry"
	nextAttemptAt := now.UTC().Add(reviewNotificationRetryDelay(delivery.AttemptCount))
	if delivery.AttemptCount >= delivery.MaxAttempts {
		status = "dead_letter"
		nextAttemptAt = now.UTC()
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE review_assignment_notifications
		SET status = ?,
		    next_attempt_at = ?,
		    locked_by = NULL,
		    locked_until = NULL,
		    last_error_safe_message = ?
		WHERE id = ? AND status = 'processing' AND locked_by = ?
	`, status, nextAttemptAt.Format(timeFormat), safeNotificationError(deliveryErr), delivery.ID, workerID)
	if err != nil {
		return err
	}
	return requireOneRowAffected(result)
}

func getReviewAssignmentNotificationTx(ctx context.Context, tx *sql.Tx, notificationID, workerID string) (ReviewAssignmentNotificationDelivery, error) {
	var delivery ReviewAssignmentNotificationDelivery
	err := tx.QueryRowContext(ctx, `
		SELECT notification.id, notification.project_id, project.name,
		       notification.assignment_id, assignment.content_id,
		       COALESCE(
		         revision.title,
		         (
		           SELECT latest.title
		           FROM content_revisions latest
		           WHERE latest.project_id = assignment.project_id
		             AND latest.content_id = assignment.content_id
		           ORDER BY latest.revision_number DESC
		           LIMIT 1
		         ),
		         'Untitled article'
		       ),
		       COALESCE(assignment.revision_id, ''),
		       notification.recipient_user_id, user.email_normalized,
		       assignment.assignment_type, COALESCE(assignment.due_at, ''),
		       notification.attempt_count, notification.max_attempts,
		       notification.created_at
		FROM review_assignment_notifications notification
		JOIN review_assignments assignment ON assignment.id = notification.assignment_id
		JOIN projects project ON project.id = notification.project_id
		JOIN users user ON user.id = notification.recipient_user_id
		LEFT JOIN content_revisions revision
		  ON revision.project_id = assignment.project_id
		 AND revision.content_id = assignment.content_id
		 AND revision.id = assignment.revision_id
		WHERE notification.id = ?
		  AND notification.status = 'processing'
		  AND notification.locked_by = ?
	`, notificationID, workerID).Scan(
		&delivery.ID,
		&delivery.ProjectID,
		&delivery.ProjectName,
		&delivery.AssignmentID,
		&delivery.ArticleID,
		&delivery.ArticleTitle,
		&delivery.RevisionID,
		&delivery.RecipientUserID,
		&delivery.RecipientEmail,
		&delivery.AssignmentType,
		&delivery.DueAt,
		&delivery.AttemptCount,
		&delivery.MaxAttempts,
		&delivery.CreatedAt,
	)
	return delivery, err
}

func reviewNotificationRetryDelay(attempt int) time.Duration {
	switch {
	case attempt <= 1:
		return time.Minute
	case attempt == 2:
		return 5 * time.Minute
	case attempt == 3:
		return 15 * time.Minute
	default:
		return time.Hour
	}
}

func safeNotificationError(err error) string {
	if err == nil {
		return "notification delivery failed"
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func requireOneRowAffected(result sql.Result) error {
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return sql.ErrNoRows
	}
	return nil
}
