package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) ContentGeneration(ctx context.Context, projectID string) (int64, error) {
	var generation int64
	err := s.db.QueryRowContext(ctx, `
		SELECT content_generation
		FROM projects
		WHERE id = ?
	`, projectID).Scan(&generation)
	return generation, err
}

type APIKey struct {
	ID        string
	ProjectID string
	Scopes    []string
}

func (s *Store) FindAPIKeyByHash(ctx context.Context, tokenHash string) (APIKey, error) {
	var key APIKey
	var scopesJSON string
	err := s.db.QueryRowContext(ctx, `
		SELECT key.id, key.project_id, key.scopes
		FROM project_api_keys key
		JOIN projects project ON project.id = key.project_id
		WHERE key.token_hash = ?
		  AND key.revoked_at IS NULL
		  AND (key.expires_at IS NULL OR key.expires_at > CURRENT_TIMESTAMP)
		  AND project.status = 'active'
	`, tokenHash).Scan(&key.ID, &key.ProjectID, &scopesJSON)
	if err != nil {
		return APIKey{}, err
	}
	if err := json.Unmarshal([]byte(scopesJSON), &key.Scopes); err != nil {
		return APIKey{}, fmt.Errorf("decode API key scopes: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE project_api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, key.ID)
	return key, nil
}
