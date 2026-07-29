package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"
)

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
