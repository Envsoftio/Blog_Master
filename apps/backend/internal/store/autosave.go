package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const maxArticleAutosaveBytes = 2 << 20

type ArticleAutosaveDraft struct {
	Title                string                     `json:"title"`
	PrimaryCategoryID    string                     `json:"primaryCategoryId"`
	Contributors         []RevisionContributorInput `json:"contributors"`
	AttributionEdited    bool                       `json:"attributionEdited"`
	Deck                 string                     `json:"deck"`
	Excerpt              string                     `json:"excerpt"`
	ShortAnswer          string                     `json:"shortAnswer"`
	SEOTitle             string                     `json:"seoTitle"`
	SEODescription       string                     `json:"seoDescription"`
	Robots               string                     `json:"robots"`
	OpenGraphTitle       string                     `json:"openGraphTitle"`
	OpenGraphDescription string                     `json:"openGraphDescription"`
	OpenGraphImage       string                     `json:"openGraphImage"`
	HTML                 string                     `json:"html"`
	BodyDocument         any                        `json:"bodyDocument,omitempty"`
}

type ArticleAutosaveInput struct {
	BaseRevisionID  string               `json:"baseRevisionId"`
	ExpectedVersion int64                `json:"expectedVersion"`
	Draft           ArticleAutosaveDraft `json:"draft"`
}

type ArticleAutosave struct {
	ProjectID      string               `json:"projectId"`
	ArticleID      string               `json:"articleId"`
	UserID         string               `json:"userId"`
	BaseRevisionID string               `json:"baseRevisionId"`
	Version        int64                `json:"version"`
	Draft          ArticleAutosaveDraft `json:"draft"`
	Stale          bool                 `json:"stale"`
	CreatedAt      string               `json:"createdAt"`
	UpdatedAt      string               `json:"updatedAt"`
}

func (s *Store) GetArticleAutosaveForUser(ctx context.Context, userID, projectID, articleID string) (ArticleAutosave, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return ArticleAutosave{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT autosave.project_id, autosave.content_id, autosave.user_id,
		       autosave.base_revision_id, autosave.version, autosave.draft_json,
		       autosave.created_at, autosave.updated_at,
		       autosave.base_revision_id <> (
		         SELECT latest.id
		         FROM content_revisions latest
		         WHERE latest.project_id = autosave.project_id
		           AND latest.content_id = autosave.content_id
		         ORDER BY latest.revision_number DESC
		         LIMIT 1
		       )
		FROM article_autosaves autosave
		JOIN content_items item
		  ON item.project_id = autosave.project_id
		 AND item.id = autosave.content_id
		WHERE autosave.project_id = ? AND autosave.content_id = ?
		  AND autosave.user_id = ? AND item.archived_at IS NULL
	`, projectID, articleID, userID)
	return scanArticleAutosave(row)
}

func (s *Store) SaveArticleAutosave(
	ctx context.Context,
	userID, projectID, articleID string,
	input ArticleAutosaveInput,
) (ArticleAutosave, error) {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return ArticleAutosave{}, err
	}
	input.BaseRevisionID = strings.TrimSpace(input.BaseRevisionID)
	if input.BaseRevisionID == "" {
		return ArticleAutosave{}, fmt.Errorf("%w: baseRevisionId is required", ErrValidation)
	}
	if input.ExpectedVersion < 0 {
		return ArticleAutosave{}, fmt.Errorf("%w: expectedVersion cannot be negative", ErrValidation)
	}
	draftJSON, err := json.Marshal(input.Draft)
	if err != nil {
		return ArticleAutosave{}, fmt.Errorf("%w: draft must be valid JSON", ErrValidation)
	}
	if len(draftJSON) > maxArticleAutosaveBytes {
		return ArticleAutosave{}, fmt.Errorf("%w: autosave draft cannot exceed %d bytes", ErrValidation, maxArticleAutosaveBytes)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ArticleAutosave{}, err
	}
	defer tx.Rollback()

	project, err := loadWorkflowProject(ctx, tx, projectID)
	if err != nil {
		return ArticleAutosave{}, err
	}
	if project.Status != "active" {
		return ArticleAutosave{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	var latestRevisionID string
	if err := tx.QueryRowContext(ctx, `
		SELECT revision.id
		FROM content_items item
		JOIN content_revisions revision
		  ON revision.project_id = item.project_id
		 AND revision.content_id = item.id
		WHERE item.project_id = ? AND item.id = ? AND item.archived_at IS NULL
		ORDER BY revision.revision_number DESC
		LIMIT 1
	`, projectID, articleID).Scan(&latestRevisionID); err != nil {
		return ArticleAutosave{}, err
	}
	if input.BaseRevisionID != latestRevisionID {
		return ArticleAutosave{}, fmt.Errorf("%w: autosave base revision is stale; reload before saving", ErrInvalidWorkflow)
	}
	if categoryID := strings.TrimSpace(input.Draft.PrimaryCategoryID); categoryID != "" {
		if _, err := loadCategory(ctx, tx, projectID, categoryID); err != nil {
			return ArticleAutosave{}, err
		}
	}
	for _, contributor := range input.Draft.Contributors {
		authorID := strings.TrimSpace(contributor.AuthorID)
		if authorID == "" {
			continue
		}
		var exists int
		if err := tx.QueryRowContext(ctx, `
			SELECT 1 FROM authors WHERE project_id = ? AND id = ?
		`, projectID, authorID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return ArticleAutosave{}, fmt.Errorf("%w: autosave contributor %q must belong to the selected project", ErrValidation, authorID)
		} else if err != nil {
			return ArticleAutosave{}, err
		}
	}

	var currentVersion int64
	versionErr := tx.QueryRowContext(ctx, `
		SELECT version
		FROM article_autosaves
		WHERE project_id = ? AND content_id = ? AND user_id = ?
	`, projectID, articleID, userID).Scan(&currentVersion)
	switch {
	case errors.Is(versionErr, sql.ErrNoRows) && input.ExpectedVersion != 0:
		return ArticleAutosave{}, fmt.Errorf("%w: autosave version is stale; reload before saving", ErrInvalidWorkflow)
	case versionErr != nil && !errors.Is(versionErr, sql.ErrNoRows):
		return ArticleAutosave{}, versionErr
	case versionErr == nil && input.ExpectedVersion != currentVersion:
		return ArticleAutosave{}, fmt.Errorf("%w: autosave version is stale; another tab saved newer work", ErrInvalidWorkflow)
	}

	if errors.Is(versionErr, sql.ErrNoRows) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO article_autosaves(
			  project_id, content_id, user_id, base_revision_id, version, draft_json
			) VALUES (?, ?, ?, ?, 1, ?)
		`, projectID, articleID, userID, input.BaseRevisionID, string(draftJSON)); err != nil {
			return ArticleAutosave{}, err
		}
		currentVersion = 1
	} else {
		result, err := tx.ExecContext(ctx, `
			UPDATE article_autosaves
			SET base_revision_id = ?, version = version + 1, draft_json = ?,
			    updated_at = CURRENT_TIMESTAMP
			WHERE project_id = ? AND content_id = ? AND user_id = ? AND version = ?
		`, input.BaseRevisionID, string(draftJSON), projectID, articleID, userID, input.ExpectedVersion)
		if err != nil {
			return ArticleAutosave{}, err
		}
		if changed, err := result.RowsAffected(); err != nil {
			return ArticleAutosave{}, err
		} else if changed != 1 {
			return ArticleAutosave{}, fmt.Errorf("%w: autosave version is stale; another tab saved newer work", ErrInvalidWorkflow)
		}
		currentVersion++
	}

	autosave, err := queryArticleAutosaveTx(ctx, tx, userID, projectID, articleID)
	if err != nil {
		return ArticleAutosave{}, err
	}
	if autosave.Version != currentVersion {
		return ArticleAutosave{}, fmt.Errorf("autosave version mismatch after save")
	}
	if err := tx.Commit(); err != nil {
		return ArticleAutosave{}, err
	}
	return autosave, nil
}

func (s *Store) DeleteArticleAutosave(ctx context.Context, userID, projectID, articleID string) error {
	if err := s.requireContentWrite(ctx, userID, projectID); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM article_autosaves
		WHERE project_id = ? AND content_id = ? AND user_id = ?
	`, projectID, articleID, userID)
	if err != nil {
		return err
	}
	if _, err := result.RowsAffected(); err != nil {
		return err
	}
	return nil
}

func queryArticleAutosaveTx(
	ctx context.Context,
	tx *sql.Tx,
	userID, projectID, articleID string,
) (ArticleAutosave, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT project_id, content_id, user_id, base_revision_id, version,
		       draft_json, created_at, updated_at, 0
		FROM article_autosaves
		WHERE project_id = ? AND content_id = ? AND user_id = ?
	`, projectID, articleID, userID)
	return scanArticleAutosave(row)
}

func scanArticleAutosave(row rowScanner) (ArticleAutosave, error) {
	var autosave ArticleAutosave
	var draftJSON string
	if err := row.Scan(
		&autosave.ProjectID,
		&autosave.ArticleID,
		&autosave.UserID,
		&autosave.BaseRevisionID,
		&autosave.Version,
		&draftJSON,
		&autosave.CreatedAt,
		&autosave.UpdatedAt,
		&autosave.Stale,
	); err != nil {
		return ArticleAutosave{}, err
	}
	if err := json.Unmarshal([]byte(draftJSON), &autosave.Draft); err != nil {
		return ArticleAutosave{}, fmt.Errorf("decode article autosave: %w", err)
	}
	if autosave.Draft.Contributors == nil {
		autosave.Draft.Contributors = []RevisionContributorInput{}
	}
	return autosave, nil
}
