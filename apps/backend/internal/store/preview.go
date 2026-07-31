package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"seoblog/apps/backend/internal/security"
)

type PreviewToken struct {
	ID         string `json:"id"`
	ProjectID  string `json:"projectId"`
	ArticleID  string `json:"articleId"`
	RevisionID string `json:"revisionId"`
	ExpiresAt  string `json:"expiresAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
	CreatedBy  string `json:"createdBy"`
	CreatedAt  string `json:"createdAt"`
	RevokedAt  string `json:"revokedAt,omitempty"`
}

type PreviewTokenWithSecret struct {
	Token  PreviewToken `json:"token"`
	Secret string       `json:"secret"`
}

type PreviewTokenInput struct {
	ArticleID  string
	RevisionID string
	TTLMinutes int
}

type PreviewTokenContext struct {
	ID         string
	ProjectID  string
	ArticleID  string
	RevisionID string
}

func (s *Store) CreatePreviewToken(ctx context.Context, actorUserID, projectID string, input PreviewTokenInput) (PreviewTokenWithSecret, error) {
	if err := s.requireContentWrite(ctx, actorUserID, projectID); err != nil {
		return PreviewTokenWithSecret{}, err
	}
	input = applyPreviewTokenDefaults(input)
	if err := validatePreviewTokenInput(input); err != nil {
		return PreviewTokenWithSecret{}, err
	}
	tokenID, err := security.RandomID("prev")
	if err != nil {
		return PreviewTokenWithSecret{}, err
	}
	secret, tokenHash, err := newPreviewTokenSecret()
	if err != nil {
		return PreviewTokenWithSecret{}, err
	}
	expiresAt := time.Now().UTC().Add(time.Duration(input.TTLMinutes) * time.Minute).Format(timeFormat)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PreviewTokenWithSecret{}, err
	}
	defer tx.Rollback()

	if err := requireActiveProjectTx(ctx, tx, projectID); err != nil {
		return PreviewTokenWithSecret{}, err
	}
	if err := articleExistsTx(ctx, tx, projectID, input.ArticleID); err != nil {
		return PreviewTokenWithSecret{}, err
	}
	if err := revisionBelongsToArticleTx(ctx, tx, projectID, input.ArticleID, input.RevisionID); err != nil {
		return PreviewTokenWithSecret{}, err
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO preview_tokens(
		  id, token_hash, project_id, content_id, revision_id, created_by, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id, project_id, content_id, revision_id, expires_at,
		          COALESCE(last_used_at, ''), created_by, created_at, COALESCE(revoked_at, '')
	`, tokenID, tokenHash, projectID, input.ArticleID, input.RevisionID, actorUserID, expiresAt)
	token, err := scanPreviewToken(row)
	if err != nil {
		return PreviewTokenWithSecret{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "preview_token.create", "preview_token", tokenID, "success", map[string]string{
		"article_id":  input.ArticleID,
		"revision_id": input.RevisionID,
		"expires_at":  expiresAt,
	}); err != nil {
		return PreviewTokenWithSecret{}, err
	}
	if err := tx.Commit(); err != nil {
		return PreviewTokenWithSecret{}, err
	}
	return PreviewTokenWithSecret{Token: token, Secret: secret}, nil
}

func (s *Store) RevokePreviewToken(ctx context.Context, actorUserID, projectID, tokenID string) (PreviewToken, error) {
	if err := s.requireContentWrite(ctx, actorUserID, projectID); err != nil {
		return PreviewToken{}, err
	}
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return PreviewToken{}, fmt.Errorf("%w: preview token id is required", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PreviewToken{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		UPDATE preview_tokens
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ? AND revoked_at IS NULL
		RETURNING id, project_id, content_id, revision_id, expires_at,
		          COALESCE(last_used_at, ''), created_by, created_at, COALESCE(revoked_at, '')
	`, projectID, tokenID)
	token, err := scanPreviewToken(row)
	if err != nil {
		return PreviewToken{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "preview_token.revoke", "preview_token", tokenID, "success", map[string]string{
		"article_id":  token.ArticleID,
		"revision_id": token.RevisionID,
	}); err != nil {
		return PreviewToken{}, err
	}
	if err := tx.Commit(); err != nil {
		return PreviewToken{}, err
	}
	return token, nil
}

func (s *Store) FindPreviewToken(ctx context.Context, secret, revisionID string) (PreviewTokenContext, error) {
	secret = strings.TrimSpace(secret)
	revisionID = strings.TrimSpace(revisionID)
	if secret == "" || revisionID == "" {
		return PreviewTokenContext{}, sql.ErrNoRows
	}
	tokenHash := security.TokenHash(secret)
	var token PreviewTokenContext
	err := s.db.QueryRowContext(ctx, `
		SELECT token.id, token.project_id, token.content_id, token.revision_id
		FROM preview_tokens token
		JOIN projects project ON project.id = token.project_id
		WHERE token.token_hash = ?
		  AND token.revision_id = ?
		  AND token.revoked_at IS NULL
		  AND token.expires_at > CURRENT_TIMESTAMP
		  AND project.status = 'active'
	`, tokenHash, revisionID).Scan(&token.ID, &token.ProjectID, &token.ArticleID, &token.RevisionID)
	if err != nil {
		return PreviewTokenContext{}, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE preview_tokens SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, token.ID)
	return token, nil
}

func (s *Store) GetPreviewPost(ctx context.Context, projectID, articleID, revisionID string) (PublishedPost, error) {
	post, err := scanPost(s.db.QueryRowContext(ctx, `
		SELECT ci.id, ci.article_type, COALESCE(pp.slug, ci.id), cr.locale,
		       cr.revision_number, cr.title, COALESCE(cr.deck, ''),
		       COALESCE(cr.excerpt, ''), COALESCE(cr.short_answer, ''),
		       cr.body_document_json, cr.sanitized_html, cr.table_of_contents_json,
		       cr.seo_snapshot_json, cr.taxonomy_snapshot_json, cr.author_snapshot_json,
		       cr.contributor_snapshot_json, cr.source_snapshot_json, cr.claim_snapshot_json,
		       cr.media_snapshot_json, `+publishedDisclosureJSON+`, `+publishedCorrectionsJSON+`,
		       COALESCE(pp.canonical_url, ''), 'noindex,nofollow', cr.content_hash,
		       '', cr.created_at, cr.created_at,
		       COALESCE(p.publisher_name, p.name), COALESCE(p.publisher_url, '')
		FROM content_revisions cr
		JOIN content_items ci
		  ON ci.project_id = cr.project_id AND ci.id = cr.content_id
		JOIN projects p ON p.id = cr.project_id
		LEFT JOIN project_publications pp
		  ON pp.project_id = cr.project_id
		 AND pp.content_id = cr.content_id
		 AND pp.locale = cr.locale
		WHERE cr.project_id = ? AND cr.content_id = ? AND cr.id = ?
		  AND ci.archived_at IS NULL
	`, projectID, articleID, revisionID), nil)
	if err != nil {
		return PublishedPost{}, err
	}
	sourceSnapshot, claimSnapshot, err := buildRevisionTrustSnapshots(ctx, s.db, projectID, revisionID)
	if err != nil {
		return PublishedPost{}, err
	}
	post.Sources = decodeJSON(sourceSnapshot, []any{})
	post.Claims = decodeJSON(claimSnapshot, []any{})
	post.SEO.Robots = "noindex,nofollow"
	post.SEO.Index = false
	return post, nil
}

func applyPreviewTokenDefaults(input PreviewTokenInput) PreviewTokenInput {
	input.ArticleID = strings.TrimSpace(input.ArticleID)
	input.RevisionID = strings.TrimSpace(input.RevisionID)
	if input.TTLMinutes == 0 {
		input.TTLMinutes = 30
	}
	return input
}

func validatePreviewTokenInput(input PreviewTokenInput) error {
	if input.ArticleID == "" {
		return fmt.Errorf("%w: articleId is required", ErrValidation)
	}
	if input.RevisionID == "" {
		return fmt.Errorf("%w: revisionId is required", ErrValidation)
	}
	if input.TTLMinutes < 15 || input.TTLMinutes > 60 {
		return fmt.Errorf("%w: ttlMinutes must be between 15 and 60", ErrValidation)
	}
	return nil
}

func scanPreviewToken(row rowScanner) (PreviewToken, error) {
	var token PreviewToken
	err := row.Scan(
		&token.ID,
		&token.ProjectID,
		&token.ArticleID,
		&token.RevisionID,
		&token.ExpiresAt,
		&token.LastUsedAt,
		&token.CreatedBy,
		&token.CreatedAt,
		&token.RevokedAt,
	)
	return token, err
}

func newPreviewTokenSecret() (secret string, tokenHash string, err error) {
	random, err := security.RandomToken(32)
	if err != nil {
		return "", "", err
	}
	secret = "sbprev_" + random
	return secret, security.TokenHash(secret), nil
}
