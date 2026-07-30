package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const preciseSQLiteTimeFormat = "2006-01-02 15:04:05.000000000"

type ReviewComment struct {
	ID         string `json:"id"`
	ProjectID  string `json:"projectId"`
	ArticleID  string `json:"articleId"`
	RevisionID string `json:"revisionId,omitempty"`
	BlockID    string `json:"blockId,omitempty"`
	Body       string `json:"body"`
	Status     string `json:"status"`
	CreatedBy  string `json:"createdBy"`
	CreatedAt  string `json:"createdAt"`
	ResolvedBy string `json:"resolvedBy,omitempty"`
	ResolvedAt string `json:"resolvedAt,omitempty"`
}

type ReviewCommentInput struct {
	RevisionID string
	BlockID    string
	Body       string
}

type ReviewAssignment struct {
	ID             string `json:"id"`
	ProjectID      string `json:"projectId"`
	ArticleID      string `json:"articleId"`
	RevisionID     string `json:"revisionId,omitempty"`
	AssignedTo     string `json:"assignedTo"`
	AssigneeEmail  string `json:"assigneeEmail,omitempty"`
	AssigneeRole   string `json:"assigneeRole,omitempty"`
	AssignmentType string `json:"assignmentType"`
	DueAt          string `json:"dueAt,omitempty"`
	Status         string `json:"status"`
	CreatedBy      string `json:"createdBy"`
	CreatedAt      string `json:"createdAt"`
	ClosedBy       string `json:"closedBy,omitempty"`
	ClosedAt       string `json:"closedAt,omitempty"`
}

type ReviewAssignmentInput struct {
	RevisionID     string
	AssignedTo     string
	AssignmentType string
	DueAt          string
}

type ReviewAssignmentCursor struct {
	CreatedAt string `json:"createdAt"`
	ID        string `json:"id"`
}

func (s *Store) ListReviewAssignees(ctx context.Context, userID, projectID, cursor string, limit int) ([]AdminProjectMember, error) {
	if err := s.requireReviewAssignmentManage(ctx, userID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+adminProjectMemberColumns+`
		FROM project_memberships membership
		JOIN users user ON user.id = membership.user_id
		WHERE membership.project_id = ?
		  AND membership.status = 'active'
		  AND user.status = 'active'
		  AND membership.role IN ('project_owner','project_admin','editor','reviewer')
		  AND (? = '' OR membership.user_id > ?)
		ORDER BY membership.user_id
		LIMIT ?
	`, projectID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []AdminProjectMember
	for rows.Next() {
		member, err := scanProjectMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *Store) ListReviewAssignments(ctx context.Context, userID, projectID, articleID string, cursor ReviewAssignmentCursor, limit int) ([]ReviewAssignment, int, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, 0, err
	}
	if err := s.articleExists(ctx, projectID, articleID); err != nil {
		return nil, 0, err
	}
	if (cursor.CreatedAt == "") != (cursor.ID == "") {
		return nil, 0, fmt.Errorf("%w: review assignment cursor is incomplete", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	var openCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM review_assignments
		WHERE project_id = ? AND content_id = ? AND status = 'open'
	`, projectID, articleID).Scan(&openCount); err != nil {
		return nil, 0, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT assignment.id, assignment.project_id, assignment.content_id,
		       COALESCE(assignment.revision_id, ''), assignment.assigned_to,
		       user.email_normalized, membership.role, assignment.assignment_type,
		       COALESCE(assignment.due_at, ''), assignment.status,
		       assignment.created_by, assignment.created_at,
		       COALESCE(assignment.closed_by, ''), COALESCE(assignment.closed_at, '')
		FROM review_assignments assignment
		JOIN project_memberships membership
		  ON membership.project_id = assignment.project_id
		 AND membership.user_id = assignment.assigned_to
		JOIN users user ON user.id = assignment.assigned_to
		WHERE assignment.project_id = ?
		  AND assignment.content_id = ?
		  AND (
		    ? = ''
		    OR assignment.created_at < ?
		    OR (assignment.created_at = ? AND assignment.id < ?)
		  )
		ORDER BY assignment.created_at DESC, assignment.id DESC
		LIMIT ?
	`, projectID, articleID, cursor.CreatedAt, cursor.CreatedAt, cursor.CreatedAt, cursor.ID, limit)
	if err != nil {
		return nil, 0, err
	}
	var assignments []ReviewAssignment
	for rows.Next() {
		assignment, err := scanReviewAssignment(rows)
		if err != nil {
			rows.Close()
			return nil, 0, err
		}
		assignments = append(assignments, assignment)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}
	return assignments, openCount, nil
}

func (s *Store) CreateReviewAssignment(ctx context.Context, actorUserID, projectID, articleID string, input ReviewAssignmentInput) (ReviewAssignment, error) {
	if err := s.requireReviewAssignmentManage(ctx, actorUserID, projectID); err != nil {
		return ReviewAssignment{}, err
	}
	input.AssignedTo = strings.TrimSpace(input.AssignedTo)
	input.RevisionID = strings.TrimSpace(input.RevisionID)
	input.AssignmentType = strings.ToLower(strings.TrimSpace(input.AssignmentType))
	if input.AssignmentType == "" {
		input.AssignmentType = "reviewer"
	}
	if input.AssignedTo == "" {
		return ReviewAssignment{}, fmt.Errorf("%w: assignedTo is required", ErrValidation)
	}
	if input.AssignmentType != "editor" && input.AssignmentType != "reviewer" && input.AssignmentType != "sme" {
		return ReviewAssignment{}, fmt.Errorf("%w: assignmentType must be editor, reviewer or sme", ErrValidation)
	}
	dueAt := ""
	if strings.TrimSpace(input.DueAt) != "" {
		parsed, err := parseSQLiteTime(strings.TrimSpace(input.DueAt))
		if err != nil {
			return ReviewAssignment{}, fmt.Errorf("%w: dueAt must be RFC3339 or YYYY-MM-DD HH:MM:SS", ErrValidation)
		}
		dueAt = parsed.UTC().Format(timeFormat)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReviewAssignment{}, err
	}
	defer tx.Rollback()

	if err := requireActiveProjectTx(ctx, tx, projectID); err != nil {
		return ReviewAssignment{}, err
	}
	if err := articleExistsTx(ctx, tx, projectID, articleID); err != nil {
		return ReviewAssignment{}, err
	}
	if input.RevisionID != "" {
		if err := revisionBelongsToArticleTx(ctx, tx, projectID, articleID, input.RevisionID); err != nil {
			return ReviewAssignment{}, err
		}
	}
	assigneeEmail, assigneeRole, err := reviewAssigneeTx(ctx, tx, input.AssignedTo, projectID)
	if err != nil {
		return ReviewAssignment{}, err
	}
	if !assignmentTypeAllowedForRole(input.AssignmentType, assigneeRole) {
		return ReviewAssignment{}, ErrForbidden
	}
	var existingOpenAssignments int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM review_assignments
		WHERE project_id = ?
		  AND content_id = ?
		  AND COALESCE(revision_id, '') = ?
		  AND assigned_to = ?
		  AND assignment_type = ?
		  AND status = 'open'
	`, projectID, articleID, input.RevisionID, input.AssignedTo, input.AssignmentType).Scan(&existingOpenAssignments); err != nil {
		return ReviewAssignment{}, err
	}
	if existingOpenAssignments > 0 {
		return ReviewAssignment{}, fmt.Errorf("%w: an open assignment already exists for this assignee, revision and role", ErrValidation)
	}
	assignmentID, err := securityRandomID("assign")
	if err != nil {
		return ReviewAssignment{}, err
	}
	createdAt := time.Now().UTC().Format(preciseSQLiteTimeFormat)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO review_assignments(
		  id, project_id, content_id, revision_id, assigned_to,
		  assignment_type, due_at, created_by, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, assignmentID, projectID, articleID, nullIfEmpty(input.RevisionID), input.AssignedTo, input.AssignmentType, nullIfEmpty(dueAt), actorUserID, createdAt); err != nil {
		return ReviewAssignment{}, err
	}
	notificationID, err := securityRandomID("notification")
	if err != nil {
		return ReviewAssignment{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO review_assignment_notifications(
		  id, project_id, assignment_id, recipient_user_id, recipient_email
		) VALUES (?, ?, ?, ?, ?)
	`, notificationID, projectID, assignmentID, input.AssignedTo, assigneeEmail); err != nil {
		return ReviewAssignment{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "assignment.create", "review_assignment", assignmentID, "success", map[string]string{
		"article_id":       articleID,
		"assigned_to":      input.AssignedTo,
		"assignment_type":  input.AssignmentType,
		"assigned_to_role": assigneeRole,
		"notification_id":  notificationID,
	}); err != nil {
		return ReviewAssignment{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReviewAssignment{}, err
	}
	return s.GetReviewAssignment(ctx, actorUserID, projectID, assignmentID)
}

func (s *Store) CompleteReviewAssignment(ctx context.Context, actorUserID, projectID, assignmentID string) (ReviewAssignment, error) {
	return s.setReviewAssignmentStatus(ctx, actorUserID, projectID, assignmentID, "completed")
}

func (s *Store) CancelReviewAssignment(ctx context.Context, actorUserID, projectID, assignmentID string) (ReviewAssignment, error) {
	return s.setReviewAssignmentStatus(ctx, actorUserID, projectID, assignmentID, "cancelled")
}

func (s *Store) GetReviewAssignment(ctx context.Context, userID, projectID, assignmentID string) (ReviewAssignment, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return ReviewAssignment{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT assignment.id, assignment.project_id, assignment.content_id,
		       COALESCE(assignment.revision_id, ''), assignment.assigned_to,
		       user.email_normalized, membership.role, assignment.assignment_type,
		       COALESCE(assignment.due_at, ''), assignment.status,
		       assignment.created_by, assignment.created_at,
		       COALESCE(assignment.closed_by, ''), COALESCE(assignment.closed_at, '')
		FROM review_assignments assignment
		JOIN project_memberships membership
		  ON membership.project_id = assignment.project_id
		 AND membership.user_id = assignment.assigned_to
		JOIN users user ON user.id = assignment.assigned_to
		WHERE assignment.project_id = ? AND assignment.id = ?
	`, projectID, assignmentID)
	return scanReviewAssignment(row)
}

func (s *Store) setReviewAssignmentStatus(ctx context.Context, actorUserID, projectID, assignmentID, status string) (ReviewAssignment, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReviewAssignment{}, err
	}
	defer tx.Rollback()

	if err := requireActiveProjectTx(ctx, tx, projectID); err != nil {
		return ReviewAssignment{}, err
	}
	actorRole, err := projectRoleTx(ctx, tx, actorUserID, projectID)
	if err != nil {
		return ReviewAssignment{}, err
	}
	var articleID, assignedTo, currentStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT content_id, assigned_to, status
		FROM review_assignments
		WHERE project_id = ? AND id = ?
	`, projectID, assignmentID).Scan(&articleID, &assignedTo, &currentStatus); err != nil {
		return ReviewAssignment{}, err
	}
	switch status {
	case "completed":
		if actorUserID != assignedTo && !canManageReviewAssignments(actorRole) {
			return ReviewAssignment{}, ErrForbidden
		}
	case "cancelled":
		if !canManageReviewAssignments(actorRole) {
			return ReviewAssignment{}, ErrForbidden
		}
	default:
		return ReviewAssignment{}, fmt.Errorf("%w: unsupported assignment status", ErrValidation)
	}
	if currentStatus != "open" {
		return ReviewAssignment{}, fmt.Errorf("%w: only open assignments can be %s", ErrInvalidWorkflow, status)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE review_assignments
		SET status = ?, closed_by = ?, closed_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ? AND status = 'open'
	`, status, actorUserID, projectID, assignmentID)
	if err != nil {
		return ReviewAssignment{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return ReviewAssignment{}, err
	}
	if changed != 1 {
		return ReviewAssignment{}, fmt.Errorf("%w: only open assignments can be %s", ErrInvalidWorkflow, status)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE review_assignment_notifications
		SET status = 'suppressed',
		    locked_by = NULL,
		    locked_until = NULL
		WHERE assignment_id = ?
		  AND status IN ('queued','retry')
	`, assignmentID); err != nil {
		return ReviewAssignment{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "assignment."+status, "review_assignment", assignmentID, "success", map[string]string{
		"article_id":  articleID,
		"assigned_to": assignedTo,
	}); err != nil {
		return ReviewAssignment{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReviewAssignment{}, err
	}
	return s.GetReviewAssignment(ctx, actorUserID, projectID, assignmentID)
}

func (s *Store) ListReviewComments(ctx context.Context, userID, projectID, articleID, cursor string, limit int) ([]ReviewComment, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	if err := s.articleExists(ctx, projectID, articleID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, content_id, COALESCE(revision_id, ''), COALESCE(block_id, ''),
		       body, status, created_by, created_at, COALESCE(resolved_by, ''), COALESCE(resolved_at, '')
		FROM review_comments
		WHERE project_id = ?
		  AND content_id = ?
		  AND (? = '' OR id > ?)
		ORDER BY id
		LIMIT ?
	`, projectID, articleID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []ReviewComment
	for rows.Next() {
		comment, err := scanReviewComment(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func (s *Store) CreateReviewComment(ctx context.Context, actorUserID, projectID, articleID string, input ReviewCommentInput) (ReviewComment, error) {
	if _, err := s.projectRole(ctx, actorUserID, projectID); err != nil {
		return ReviewComment{}, err
	}
	input.Body = strings.TrimSpace(input.Body)
	input.RevisionID = strings.TrimSpace(input.RevisionID)
	input.BlockID = strings.TrimSpace(input.BlockID)
	if input.Body == "" {
		return ReviewComment{}, fmt.Errorf("%w: comment body is required", ErrValidation)
	}
	if utf8.RuneCountInString(input.Body) > 4000 {
		return ReviewComment{}, fmt.Errorf("%w: comment body cannot exceed 4000 characters", ErrValidation)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReviewComment{}, err
	}
	defer tx.Rollback()

	if err := requireActiveProjectTx(ctx, tx, projectID); err != nil {
		return ReviewComment{}, err
	}
	if err := articleExistsTx(ctx, tx, projectID, articleID); err != nil {
		return ReviewComment{}, err
	}
	if input.RevisionID != "" {
		if err := revisionBelongsToArticleTx(ctx, tx, projectID, articleID, input.RevisionID); err != nil {
			return ReviewComment{}, err
		}
	}
	commentID, err := securityRandomID("comment")
	if err != nil {
		return ReviewComment{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO review_comments(id, project_id, content_id, revision_id, block_id, body, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, commentID, projectID, articleID, nullIfEmpty(input.RevisionID), nullIfEmpty(input.BlockID), input.Body, actorUserID); err != nil {
		return ReviewComment{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "comment.create", "review_comment", commentID, "success", map[string]string{"article_id": articleID}); err != nil {
		return ReviewComment{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReviewComment{}, err
	}
	return s.GetReviewComment(ctx, actorUserID, projectID, commentID)
}

func (s *Store) ResolveReviewComment(ctx context.Context, actorUserID, projectID, commentID string) (ReviewComment, error) {
	return s.setReviewCommentStatus(ctx, actorUserID, projectID, commentID, "resolve", "comment.resolve")
}

func (s *Store) ReopenReviewComment(ctx context.Context, actorUserID, projectID, commentID string) (ReviewComment, error) {
	return s.setReviewCommentStatus(ctx, actorUserID, projectID, commentID, "reopen", "comment.reopen")
}

func (s *Store) GetReviewComment(ctx context.Context, userID, projectID, commentID string) (ReviewComment, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return ReviewComment{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, content_id, COALESCE(revision_id, ''), COALESCE(block_id, ''),
		       body, status, created_by, created_at, COALESCE(resolved_by, ''), COALESCE(resolved_at, '')
		FROM review_comments
		WHERE project_id = ? AND id = ?
	`, projectID, commentID)
	return scanReviewComment(row)
}

func (s *Store) setReviewCommentStatus(ctx context.Context, actorUserID, projectID, commentID, transition, action string) (ReviewComment, error) {
	if err := s.requireContentReview(ctx, actorUserID, projectID); err != nil {
		return ReviewComment{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReviewComment{}, err
	}
	defer tx.Rollback()

	if err := requireActiveProjectTx(ctx, tx, projectID); err != nil {
		return ReviewComment{}, err
	}
	var result sql.Result
	if transition == "resolve" {
		result, err = tx.ExecContext(ctx, `
			UPDATE review_comments
			SET status = 'resolved', resolved_by = ?, resolved_at = CURRENT_TIMESTAMP
			WHERE project_id = ? AND id = ? AND status <> 'resolved'
		`, actorUserID, projectID, commentID)
	} else {
		result, err = tx.ExecContext(ctx, `
			UPDATE review_comments
			SET status = 'reopened', resolved_by = NULL, resolved_at = NULL
			WHERE project_id = ? AND id = ? AND status = 'resolved'
		`, projectID, commentID)
	}
	if err != nil {
		return ReviewComment{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return ReviewComment{}, err
	}
	if changed != 1 {
		if err := commentExistsTx(ctx, tx, projectID, commentID); err != nil {
			return ReviewComment{}, err
		}
		return ReviewComment{}, fmt.Errorf("%w: comment cannot %s from its current state", ErrInvalidWorkflow, transition)
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, action, "review_comment", commentID, "success", nil); err != nil {
		return ReviewComment{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReviewComment{}, err
	}
	return s.GetReviewComment(ctx, actorUserID, projectID, commentID)
}

func scanReviewComment(row rowScanner) (ReviewComment, error) {
	var comment ReviewComment
	err := row.Scan(
		&comment.ID,
		&comment.ProjectID,
		&comment.ArticleID,
		&comment.RevisionID,
		&comment.BlockID,
		&comment.Body,
		&comment.Status,
		&comment.CreatedBy,
		&comment.CreatedAt,
		&comment.ResolvedBy,
		&comment.ResolvedAt,
	)
	return comment, err
}

func requireActiveProjectTx(ctx context.Context, tx *sql.Tx, projectID string) error {
	project, err := loadWorkflowProject(ctx, tx, projectID)
	if err != nil {
		return err
	}
	if project.Status != "active" {
		return fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	return nil
}

func (s *Store) articleExists(ctx context.Context, projectID, articleID string) error {
	var exists int
	err := s.db.QueryRowContext(ctx, `
		SELECT 1
		FROM content_items
		WHERE project_id = ? AND id = ? AND archived_at IS NULL
	`, projectID, articleID).Scan(&exists)
	return err
}

func articleExistsTx(ctx context.Context, tx *sql.Tx, projectID, articleID string) error {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM content_items
		WHERE project_id = ? AND id = ? AND archived_at IS NULL
	`, projectID, articleID).Scan(&exists)
	return err
}

func revisionBelongsToArticleTx(ctx context.Context, tx *sql.Tx, projectID, articleID, revisionID string) error {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM content_revisions
		WHERE project_id = ? AND content_id = ? AND id = ?
	`, projectID, articleID, revisionID).Scan(&exists)
	return err
}

func commentExistsTx(ctx context.Context, tx *sql.Tx, projectID, commentID string) error {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM review_comments
		WHERE project_id = ? AND id = ?
	`, projectID, commentID).Scan(&exists)
	return err
}

func assignmentTypeAllowedForRole(assignmentType, role string) bool {
	switch assignmentType {
	case "editor":
		return role == "project_owner" || role == "project_admin" || role == "editor"
	case "reviewer", "sme":
		return role == "project_owner" || role == "project_admin" || role == "editor" || role == "reviewer"
	default:
		return false
	}
}

func canManageReviewAssignments(role string) bool {
	return role == "project_owner" || role == "project_admin" || role == "editor"
}

func (s *Store) requireReviewAssignmentManage(ctx context.Context, userID, projectID string) error {
	role, err := s.projectRole(ctx, userID, projectID)
	if err != nil {
		return err
	}
	if canManageReviewAssignments(role) {
		return nil
	}
	return ErrForbidden
}

func reviewAssigneeTx(ctx context.Context, tx *sql.Tx, userID, projectID string) (string, string, error) {
	var email, role string
	err := tx.QueryRowContext(ctx, `
		SELECT user.email_normalized, membership.role
		FROM project_memberships membership
		JOIN users user ON user.id = membership.user_id
		WHERE membership.user_id = ?
		  AND membership.project_id = ?
		  AND membership.status = 'active'
		  AND user.status = 'active'
	`, userID, projectID).Scan(&email, &role)
	return email, role, err
}

func scanReviewAssignment(row rowScanner) (ReviewAssignment, error) {
	var assignment ReviewAssignment
	err := row.Scan(
		&assignment.ID,
		&assignment.ProjectID,
		&assignment.ArticleID,
		&assignment.RevisionID,
		&assignment.AssignedTo,
		&assignment.AssigneeEmail,
		&assignment.AssigneeRole,
		&assignment.AssignmentType,
		&assignment.DueAt,
		&assignment.Status,
		&assignment.CreatedBy,
		&assignment.CreatedAt,
		&assignment.ClosedBy,
		&assignment.ClosedAt,
	)
	return assignment, err
}
