package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"seoblog/apps/backend/internal/security"
)

const timeFormat = "2006-01-02 15:04:05"

var (
	ErrEmailAlreadyExists      = errors.New("email already exists")
	ErrBootstrapAlreadyCreated = errors.New("a user already exists")
	ErrForbidden               = errors.New("forbidden")
	ErrProjectHasContent       = errors.New("project has retained content")
)

type AdminUser struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
	LastSeenAt string `json:"lastSeenAt,omitempty"`
}

type UserCredential struct {
	User         AdminUser
	PasswordHash string
}

type Session struct {
	TokenHash         string
	CSRFTokenHash     string
	UserID            string
	ReauthenticatedAt string
	IdleExpiresAt     string
	AbsoluteExpiresAt string
}

type AdminProject struct {
	ID                  string   `json:"id"`
	WorkspaceID         string   `json:"workspaceId"`
	WorkspaceSlug       string   `json:"workspaceSlug"`
	WorkspaceName       string   `json:"workspaceName"`
	Slug                string   `json:"slug"`
	Name                string   `json:"name"`
	Status              string   `json:"status"`
	PublicProjectKey    string   `json:"publicProjectKey"`
	PrimaryDomain       string   `json:"primaryDomain,omitempty"`
	VerifiedDomains     []string `json:"verifiedDomains"`
	BlogBasePath        string   `json:"blogBasePath"`
	DefaultLocale       string   `json:"defaultLocale"`
	SupportedLocales    []string `json:"supportedLocales"`
	Timezone            string   `json:"timezone"`
	PublisherName       string   `json:"publisherName,omitempty"`
	PublisherURL        string   `json:"publisherUrl,omitempty"`
	DefaultRobotsPolicy string   `json:"defaultRobotsPolicy"`
	Role                string   `json:"role,omitempty"`
	CreatedAt           string   `json:"createdAt"`
	UpdatedAt           string   `json:"updatedAt"`
}

type ProjectInput struct {
	WorkspaceID         string
	WorkspaceSlug       string
	WorkspaceName       string
	Slug                string
	Name                string
	PrimaryDomain       string
	VerifiedDomains     []string
	BlogBasePath        string
	DefaultLocale       string
	SupportedLocales    []string
	Timezone            string
	PublisherName       string
	PublisherURL        string
	DefaultRobotsPolicy string
}

type ProjectPatch struct {
	Name                *string
	PrimaryDomain       *string
	VerifiedDomains     *[]string
	BlogBasePath        *string
	DefaultLocale       *string
	SupportedLocales    *[]string
	Timezone            *string
	PublisherName       *string
	PublisherURL        *string
	DefaultRobotsPolicy *string
}

type ProjectDeletionImpact struct {
	ProjectID             string `json:"projectId"`
	CanDelete             bool   `json:"canDelete"`
	ActiveAPIKeys         int64  `json:"activeApiKeys"`
	ActiveMembers         int64  `json:"activeMembers"`
	ContentItems          int64  `json:"contentItems"`
	PublishedPublications int64  `json:"publishedPublications"`
	ScheduledPublications int64  `json:"scheduledPublications"`
	Redirects             int64  `json:"redirects"`
	Assets                int64  `json:"assets"`
	Webhooks              int64  `json:"webhooks"`
	PendingJobs           int64  `json:"pendingJobs"`
}

type AdminAPIKey struct {
	ID          string   `json:"id"`
	ProjectID   string   `json:"projectId"`
	Environment string   `json:"environment"`
	Name        string   `json:"name"`
	TokenPrefix string   `json:"tokenPrefix"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   string   `json:"expiresAt,omitempty"`
	LastUsedAt  string   `json:"lastUsedAt,omitempty"`
	CreatedBy   string   `json:"createdBy"`
	CreatedAt   string   `json:"createdAt"`
	RevokedAt   string   `json:"revokedAt,omitempty"`
}

type APIKeyWithSecret struct {
	Key    AdminAPIKey `json:"key"`
	Secret string      `json:"secret"`
}

type APIKeyInput struct {
	Environment string
	Name        string
	Scopes      []string
	ExpiresAt   string
}

var defaultPublishedReadScopes = []string{
	"content:published:read",
	"taxonomy:published:read",
	"authors:published:read",
	"discovery:read",
	"redirects:read",
}

func (s *Store) BootstrapOwner(ctx context.Context, email, passwordHash string) (AdminUser, error) {
	email = normalizeEmail(email)
	if email == "" {
		return AdminUser{}, errors.New("email is required")
	}
	userID, err := security.RandomID("usr")
	if err != nil {
		return AdminUser{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminUser{}, err
	}
	defer tx.Rollback()

	var userCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM users`).Scan(&userCount); err != nil {
		return AdminUser{}, err
	}
	if userCount > 0 {
		return AdminUser{}, ErrBootstrapAlreadyCreated
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users(id, email_normalized, password_hash, status, email_verified_at, password_changed_at)
		VALUES (?, ?, ?, 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID, email, passwordHash); err != nil {
		return AdminUser{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminUser{}, err
	}
	return s.GetUser(ctx, userID)
}

func (s *Store) FindUserCredentialByEmail(ctx context.Context, email string) (UserCredential, error) {
	var credential UserCredential
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email_normalized, status, COALESCE(password_hash, ''), created_at
		FROM users
		WHERE email_normalized = ?
	`, normalizeEmail(email)).Scan(
		&credential.User.ID,
		&credential.User.Email,
		&credential.User.Status,
		&credential.PasswordHash,
		&credential.User.CreatedAt,
	)
	return credential, err
}

func (s *Store) FindUserCredentialByID(ctx context.Context, userID string) (UserCredential, error) {
	var credential UserCredential
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email_normalized, status, COALESCE(password_hash, ''), created_at
		FROM users
		WHERE id = ?
	`, userID).Scan(
		&credential.User.ID,
		&credential.User.Email,
		&credential.User.Status,
		&credential.PasswordHash,
		&credential.User.CreatedAt,
	)
	return credential, err
}

func (s *Store) GetUser(ctx context.Context, userID string) (AdminUser, error) {
	var user AdminUser
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email_normalized, status, created_at
		FROM users
		WHERE id = ?
	`, userID).Scan(&user.ID, &user.Email, &user.Status, &user.CreatedAt)
	return user, err
}

func (s *Store) CreateSession(ctx context.Context, userID, tokenHash, csrfTokenHash string, now time.Time) error {
	idleExpiresAt := now.Add(8 * time.Hour).UTC().Format(timeFormat)
	absoluteExpiresAt := now.Add(30 * 24 * time.Hour).UTC().Format(timeFormat)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions(
		  token_hash, csrf_token_hash, user_id, reauthenticated_at,
		  idle_expires_at, absolute_expires_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`, tokenHash, csrfTokenHash, userID, now.UTC().Format(timeFormat), idleExpiresAt, absoluteExpiresAt)
	return err
}

func (s *Store) GetSessionUser(ctx context.Context, tokenHash string) (AdminUser, Session, error) {
	var user AdminUser
	var session Session
	err := s.db.QueryRowContext(ctx, `
		SELECT session.token_hash, session.csrf_token_hash, session.user_id,
		       COALESCE(session.reauthenticated_at, session.created_at),
		       session.idle_expires_at, session.absolute_expires_at,
		       user.id, user.email_normalized, user.status, user.created_at, session.last_seen_at
		FROM sessions session
		JOIN users user ON user.id = session.user_id
		WHERE session.token_hash = ?
		  AND session.revoked_at IS NULL
		  AND session.idle_expires_at > CURRENT_TIMESTAMP
		  AND session.absolute_expires_at > CURRENT_TIMESTAMP
		  AND user.status = 'active'
	`, tokenHash).Scan(
		&session.TokenHash,
		&session.CSRFTokenHash,
		&session.UserID,
		&session.ReauthenticatedAt,
		&session.IdleExpiresAt,
		&session.AbsoluteExpiresAt,
		&user.ID,
		&user.Email,
		&user.Status,
		&user.CreatedAt,
		&user.LastSeenAt,
	)
	if err != nil {
		return AdminUser{}, Session{}, err
	}
	_, _ = s.db.ExecContext(ctx, `
		UPDATE sessions
		SET last_seen_at = CURRENT_TIMESTAMP,
		    idle_expires_at = datetime(CURRENT_TIMESTAMP, '+8 hours')
		WHERE token_hash = ?
	`, tokenHash)
	return user, session, nil
}

func (s *Store) MarkSessionReauthenticated(ctx context.Context, tokenHash string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET reauthenticated_at = ?
		WHERE token_hash = ?
		  AND revoked_at IS NULL
		  AND idle_expires_at > CURRENT_TIMESTAMP
		  AND absolute_expires_at > CURRENT_TIMESTAMP
	`, now.UTC().Format(timeFormat), tokenHash)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) RotateSessionCSRF(ctx context.Context, tokenHash, csrfTokenHash string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET csrf_token_hash = ?
		WHERE token_hash = ?
		  AND revoked_at IS NULL
		  AND idle_expires_at > CURRENT_TIMESTAMP
		  AND absolute_expires_at > CURRENT_TIMESTAMP
	`, csrfTokenHash, tokenHash)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE token_hash = ?
	`, tokenHash)
	return err
}

func (s *Store) ListProjectsForUser(ctx context.Context, userID string, cursor string, limit int) ([]AdminProject, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+adminProjectColumns+`
		FROM projects project
		JOIN workspaces workspace ON workspace.id = project.workspace_id
		JOIN project_memberships membership
		  ON membership.project_id = project.id
		 AND membership.user_id = ?
		 AND membership.status = 'active'
		WHERE (? = '' OR project.id > ?)
		ORDER BY project.id
		LIMIT ?
	`, userID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []AdminProject
	for rows.Next() {
		project, err := scanAdminProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s *Store) GetProjectForUser(ctx context.Context, userID, projectID string) (AdminProject, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+adminProjectColumns+`
		FROM projects project
		JOIN workspaces workspace ON workspace.id = project.workspace_id
		JOIN project_memberships membership
		  ON membership.project_id = project.id
		 AND membership.user_id = ?
		 AND membership.status = 'active'
		WHERE project.id = ?
	`, userID, projectID)
	return scanAdminProject(row)
}

func (s *Store) CreateProject(ctx context.Context, actorUserID string, input ProjectInput) (AdminProject, error) {
	input = applyProjectDefaults(input)
	if err := validateProjectInput(input); err != nil {
		return AdminProject{}, err
	}

	projectID, err := security.RandomID("prj")
	if err != nil {
		return AdminProject{}, err
	}
	publicProjectKey, err := security.RandomID("pub")
	if err != nil {
		return AdminProject{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminProject{}, err
	}
	defer tx.Rollback()

	workspaceID := input.WorkspaceID
	if workspaceID == "" {
		workspaceID, err = ensureWorkspace(ctx, tx, input.WorkspaceSlug, input.WorkspaceName)
		if err != nil {
			return AdminProject{}, err
		}
	}

	verifiedDomainsJSON, err := jsonString(input.VerifiedDomains)
	if err != nil {
		return AdminProject{}, err
	}
	supportedLocalesJSON, err := jsonString(input.SupportedLocales)
	if err != nil {
		return AdminProject{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO projects(
		  id, workspace_id, slug, name, public_project_key, primary_domain,
		  verified_domains_json, blog_base_path, default_locale, supported_locales,
		  timezone, publisher_name, publisher_url, default_robots_policy, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		projectID, workspaceID, input.Slug, input.Name, publicProjectKey, nullIfEmpty(input.PrimaryDomain),
		verifiedDomainsJSON, input.BlogBasePath, input.DefaultLocale, supportedLocalesJSON,
		input.Timezone, nullIfEmpty(input.PublisherName), nullIfEmpty(input.PublisherURL),
		input.DefaultRobotsPolicy, actorUserID,
	); err != nil {
		return AdminProject{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'project_owner', 'active', CURRENT_TIMESTAMP)
	`, projectID, actorUserID); err != nil {
		return AdminProject{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_events(project_id, actor_type, actor_id, action, target_type, target_id, outcome, metadata_json)
		VALUES (?, 'user', ?, 'project.create', 'project', ?, 'success', '{}')
	`, projectID, actorUserID, projectID); err != nil {
		return AdminProject{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminProject{}, err
	}
	return s.GetProjectForUser(ctx, actorUserID, projectID)
}

func (s *Store) UpdateProject(ctx context.Context, actorUserID, projectID string, patch ProjectPatch) (AdminProject, error) {
	if err := s.requireProjectManagement(ctx, actorUserID, projectID); err != nil {
		return AdminProject{}, err
	}

	current, err := s.GetProjectForUser(ctx, actorUserID, projectID)
	if err != nil {
		return AdminProject{}, err
	}
	next := ProjectInput{
		WorkspaceID:         current.WorkspaceID,
		WorkspaceSlug:       current.WorkspaceSlug,
		WorkspaceName:       current.WorkspaceName,
		Slug:                current.Slug,
		Name:                current.Name,
		PrimaryDomain:       current.PrimaryDomain,
		VerifiedDomains:     current.VerifiedDomains,
		BlogBasePath:        current.BlogBasePath,
		DefaultLocale:       current.DefaultLocale,
		SupportedLocales:    current.SupportedLocales,
		Timezone:            current.Timezone,
		PublisherName:       current.PublisherName,
		PublisherURL:        current.PublisherURL,
		DefaultRobotsPolicy: current.DefaultRobotsPolicy,
	}
	if patch.Name != nil {
		next.Name = strings.TrimSpace(*patch.Name)
	}
	if patch.PrimaryDomain != nil {
		next.PrimaryDomain = strings.TrimSpace(*patch.PrimaryDomain)
	}
	if patch.VerifiedDomains != nil {
		next.VerifiedDomains = cleanStringSlice(*patch.VerifiedDomains)
	}
	if patch.BlogBasePath != nil {
		next.BlogBasePath = strings.TrimSpace(*patch.BlogBasePath)
	}
	if patch.DefaultLocale != nil {
		next.DefaultLocale = strings.TrimSpace(*patch.DefaultLocale)
	}
	if patch.SupportedLocales != nil {
		next.SupportedLocales = cleanStringSlice(*patch.SupportedLocales)
	}
	if patch.Timezone != nil {
		next.Timezone = strings.TrimSpace(*patch.Timezone)
	}
	if patch.PublisherName != nil {
		next.PublisherName = strings.TrimSpace(*patch.PublisherName)
	}
	if patch.PublisherURL != nil {
		next.PublisherURL = strings.TrimSpace(*patch.PublisherURL)
	}
	if patch.DefaultRobotsPolicy != nil {
		next.DefaultRobotsPolicy = strings.TrimSpace(*patch.DefaultRobotsPolicy)
	}
	next = applyProjectDefaults(next)
	if err := validateProjectInput(next); err != nil {
		return AdminProject{}, err
	}

	verifiedDomainsJSON, err := jsonString(next.VerifiedDomains)
	if err != nil {
		return AdminProject{}, err
	}
	supportedLocalesJSON, err := jsonString(next.SupportedLocales)
	if err != nil {
		return AdminProject{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE projects
		SET name = ?,
		    primary_domain = ?,
		    verified_domains_json = ?,
		    blog_base_path = ?,
		    default_locale = ?,
		    supported_locales = ?,
		    timezone = ?,
		    publisher_name = ?,
		    publisher_url = ?,
		    default_robots_policy = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, next.Name, nullIfEmpty(next.PrimaryDomain), verifiedDomainsJSON, next.BlogBasePath,
		next.DefaultLocale, supportedLocalesJSON, next.Timezone, nullIfEmpty(next.PublisherName),
		nullIfEmpty(next.PublisherURL), next.DefaultRobotsPolicy, projectID)
	if err != nil {
		return AdminProject{}, err
	}
	_, _ = s.db.ExecContext(ctx, `
		INSERT INTO audit_events(project_id, actor_type, actor_id, action, target_type, target_id, outcome, metadata_json)
		VALUES (?, 'user', ?, 'project.update', 'project', ?, 'success', '{}')
	`, projectID, actorUserID, projectID)
	return s.GetProjectForUser(ctx, actorUserID, projectID)
}

func (s *Store) SetProjectStatus(ctx context.Context, actorUserID, projectID, status string) (AdminProject, error) {
	if status != "suspended" && status != "archived" {
		return AdminProject{}, errors.New("unsupported project status")
	}
	if err := s.requireProjectManagement(ctx, actorUserID, projectID); err != nil {
		return AdminProject{}, err
	}
	archivedAtSQL := "archived_at"
	if status == "archived" {
		archivedAtSQL = "CURRENT_TIMESTAMP"
	}
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`
		UPDATE projects
		SET status = ?, archived_at = %s, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, archivedAtSQL), status, projectID); err != nil {
		return AdminProject{}, err
	}
	_, _ = s.db.ExecContext(ctx, `
		INSERT INTO audit_events(project_id, actor_type, actor_id, action, target_type, target_id, outcome, metadata_json)
		VALUES (?, 'user', ?, ?, 'project', ?, 'success', '{}')
	`, projectID, actorUserID, "project."+status, projectID)
	return s.GetProjectForUser(ctx, actorUserID, projectID)
}

func (s *Store) ProjectDeletionImpact(ctx context.Context, actorUserID, projectID string) (ProjectDeletionImpact, error) {
	if err := s.requireProjectManagement(ctx, actorUserID, projectID); err != nil {
		return ProjectDeletionImpact{}, err
	}
	impact := ProjectDeletionImpact{ProjectID: projectID}
	counts := []struct {
		destination *int64
		query       string
	}{
		{&impact.ActiveAPIKeys, `SELECT COUNT(1) FROM project_api_keys WHERE project_id = ? AND revoked_at IS NULL`},
		{&impact.ActiveMembers, `SELECT COUNT(1) FROM project_memberships WHERE project_id = ? AND status = 'active'`},
		{&impact.ContentItems, `SELECT COUNT(1) FROM content_items WHERE project_id = ?`},
		{&impact.PublishedPublications, `SELECT COUNT(1) FROM project_publications WHERE project_id = ? AND publication_state = 'published'`},
		{&impact.ScheduledPublications, `SELECT COUNT(1) FROM project_publications WHERE project_id = ? AND publication_state = 'scheduled'`},
		{&impact.Redirects, `SELECT COUNT(1) FROM slug_redirects WHERE project_id = ?`},
		{&impact.Assets, `SELECT COUNT(1) FROM assets WHERE project_id = ?`},
		{&impact.Webhooks, `SELECT COUNT(1) FROM webhook_endpoints WHERE project_id = ? AND revoked_at IS NULL`},
		{&impact.PendingJobs, `SELECT COUNT(1) FROM jobs WHERE project_id = ? AND status IN ('queued', 'running')`},
	}
	for _, count := range counts {
		if err := s.db.QueryRowContext(ctx, count.query, projectID).Scan(count.destination); err != nil {
			return ProjectDeletionImpact{}, err
		}
	}
	impact.CanDelete = impact.ContentItems == 0 &&
		impact.PublishedPublications == 0 &&
		impact.ScheduledPublications == 0 &&
		impact.PendingJobs == 0
	return impact, nil
}

func (s *Store) DeleteProject(ctx context.Context, actorUserID, projectID string) error {
	if err := s.requireProjectOwner(ctx, actorUserID, projectID); err != nil {
		return err
	}
	impact, err := s.ProjectDeletionImpact(ctx, actorUserID, projectID)
	if err != nil {
		return err
	}
	if !impact.CanDelete {
		return ErrProjectHasContent
	}
	_, _ = s.db.ExecContext(ctx, `
		INSERT INTO audit_events(project_id, actor_type, actor_id, action, target_type, target_id, outcome, metadata_json)
		VALUES (?, 'user', ?, 'project.delete', 'project', ?, 'success', '{}')
	`, projectID, actorUserID, projectID)
	result, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, projectID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ListProjectAPIKeys(ctx context.Context, actorUserID, projectID, cursor string, limit int) ([]AdminAPIKey, error) {
	if err := s.requireProjectManagement(ctx, actorUserID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, environment, name, token_prefix, scopes,
		       COALESCE(expires_at, ''), COALESCE(last_used_at, ''),
		       created_by, created_at, COALESCE(revoked_at, '')
		FROM project_api_keys
		WHERE project_id = ?
		  AND (? = '' OR id > ?)
		ORDER BY id
		LIMIT ?
	`, projectID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []AdminAPIKey
	for rows.Next() {
		key, err := scanAdminAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *Store) CreateProjectAPIKey(ctx context.Context, actorUserID, projectID string, input APIKeyInput) (APIKeyWithSecret, error) {
	if err := s.requireProjectManagement(ctx, actorUserID, projectID); err != nil {
		return APIKeyWithSecret{}, err
	}
	input = applyAPIKeyDefaults(input)
	if err := validateAPIKeyInput(input); err != nil {
		return APIKeyWithSecret{}, err
	}
	if input.ExpiresAt != "" {
		parsed, err := parseSQLiteTime(input.ExpiresAt)
		if err != nil {
			return APIKeyWithSecret{}, fmt.Errorf("%w: expiresAt must be RFC3339 or YYYY-MM-DD HH:MM:SS", ErrValidation)
		}
		if !parsed.After(time.Now().UTC()) {
			return APIKeyWithSecret{}, fmt.Errorf("%w: expiresAt must be in the future", ErrValidation)
		}
		input.ExpiresAt = parsed.UTC().Format(timeFormat)
	}
	keyID, err := security.RandomID("key")
	if err != nil {
		return APIKeyWithSecret{}, err
	}
	secret, tokenPrefix, tokenHash, err := newProjectAPIKeySecret(input.Environment)
	if err != nil {
		return APIKeyWithSecret{}, err
	}
	scopesJSON, err := jsonString(input.Scopes)
	if err != nil {
		return APIKeyWithSecret{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return APIKeyWithSecret{}, err
	}
	defer tx.Rollback()

	project, err := projectStatus(ctx, tx, projectID)
	if err != nil {
		return APIKeyWithSecret{}, err
	}
	if project != "active" {
		return APIKeyWithSecret{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO project_api_keys(
		  id, project_id, environment, name, token_prefix, token_hash,
		  scopes, expires_at, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, project_id, environment, name, token_prefix, scopes,
		          COALESCE(expires_at, ''), COALESCE(last_used_at, ''),
		          created_by, created_at, COALESCE(revoked_at, '')
	`, keyID, projectID, input.Environment, input.Name, tokenPrefix, tokenHash,
		scopesJSON, nullIfEmpty(input.ExpiresAt), actorUserID)
	key, err := scanAdminAPIKey(row)
	if err != nil {
		return APIKeyWithSecret{}, err
	}
	if err := insertAPIKeyAudit(ctx, tx, projectID, actorUserID, "api_key.create", keyID, map[string]string{
		"environment": input.Environment,
		"tokenPrefix": tokenPrefix,
	}); err != nil {
		return APIKeyWithSecret{}, err
	}
	if err := tx.Commit(); err != nil {
		return APIKeyWithSecret{}, err
	}
	return APIKeyWithSecret{Key: key, Secret: secret}, nil
}

func (s *Store) RotateProjectAPIKey(ctx context.Context, actorUserID, projectID, keyID string) (APIKeyWithSecret, error) {
	if err := s.requireProjectManagement(ctx, actorUserID, projectID); err != nil {
		return APIKeyWithSecret{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return APIKeyWithSecret{}, err
	}
	defer tx.Rollback()

	var environment, name, scopesJSON, expiresAt string
	err = tx.QueryRowContext(ctx, `
		SELECT environment, name, scopes, COALESCE(expires_at, '')
		FROM project_api_keys
		WHERE project_id = ? AND id = ? AND revoked_at IS NULL
	`, projectID, keyID).Scan(&environment, &name, &scopesJSON, &expiresAt)
	if err != nil {
		return APIKeyWithSecret{}, err
	}
	if expiresAt != "" {
		expires, err := parseSQLiteTime(expiresAt)
		if err != nil {
			return APIKeyWithSecret{}, err
		}
		if !expires.After(time.Now().UTC()) {
			return APIKeyWithSecret{}, fmt.Errorf("%w: expired API keys cannot be rotated", ErrInvalidWorkflow)
		}
	}
	project, err := projectStatus(ctx, tx, projectID)
	if err != nil {
		return APIKeyWithSecret{}, err
	}
	if project != "active" {
		return APIKeyWithSecret{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	replacementID, err := security.RandomID("key")
	if err != nil {
		return APIKeyWithSecret{}, err
	}
	secret, tokenPrefix, tokenHash, err := newProjectAPIKeySecret(environment)
	if err != nil {
		return APIKeyWithSecret{}, err
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO project_api_keys(
		  id, project_id, environment, name, token_prefix, token_hash,
		  scopes, expires_at, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, project_id, environment, name, token_prefix, scopes,
		          COALESCE(expires_at, ''), COALESCE(last_used_at, ''),
		          created_by, created_at, COALESCE(revoked_at, '')
	`, replacementID, projectID, environment, name, tokenPrefix, tokenHash,
		scopesJSON, nullIfEmpty(expiresAt), actorUserID)
	replacement, err := scanAdminAPIKey(row)
	if err != nil {
		return APIKeyWithSecret{}, err
	}
	if err := insertAPIKeyAudit(ctx, tx, projectID, actorUserID, "api_key.rotate", replacementID, map[string]string{
		"environment":   environment,
		"tokenPrefix":   tokenPrefix,
		"replacesKeyId": keyID,
	}); err != nil {
		return APIKeyWithSecret{}, err
	}
	if err := tx.Commit(); err != nil {
		return APIKeyWithSecret{}, err
	}
	return APIKeyWithSecret{Key: replacement, Secret: secret}, nil
}

func (s *Store) RevokeProjectAPIKey(ctx context.Context, actorUserID, projectID, keyID string) (AdminAPIKey, error) {
	if err := s.requireProjectManagement(ctx, actorUserID, projectID); err != nil {
		return AdminAPIKey{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminAPIKey{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		UPDATE project_api_keys
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ? AND revoked_at IS NULL
		RETURNING id, project_id, environment, name, token_prefix, scopes,
		          COALESCE(expires_at, ''), COALESCE(last_used_at, ''),
		          created_by, created_at, COALESCE(revoked_at, '')
	`, projectID, keyID)
	key, err := scanAdminAPIKey(row)
	if err != nil {
		return AdminAPIKey{}, err
	}
	if err := insertAPIKeyAudit(ctx, tx, projectID, actorUserID, "api_key.revoke", keyID, map[string]string{
		"environment": key.Environment,
		"tokenPrefix": key.TokenPrefix,
	}); err != nil {
		return AdminAPIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminAPIKey{}, err
	}
	return key, nil
}

func (s *Store) requireProjectManagement(ctx context.Context, userID, projectID string) error {
	role, err := s.projectRole(ctx, userID, projectID)
	if err != nil {
		return err
	}
	if role == "project_owner" || role == "project_admin" {
		return nil
	}
	return ErrForbidden
}

func (s *Store) requireProjectOwner(ctx context.Context, userID, projectID string) error {
	role, err := s.projectRole(ctx, userID, projectID)
	if err != nil {
		return err
	}
	if role == "project_owner" {
		return nil
	}
	return ErrForbidden
}

func (s *Store) projectRole(ctx context.Context, userID, projectID string) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx, `
		SELECT role
		FROM project_memberships
		WHERE user_id = ? AND project_id = ? AND status = 'active'
	`, userID, projectID).Scan(&role)
	return role, err
}

const adminProjectColumns = `
	project.id, project.workspace_id, workspace.slug, workspace.name,
	project.slug, project.name, project.status, project.public_project_key,
	COALESCE(project.primary_domain, ''), project.verified_domains_json,
	project.blog_base_path, project.default_locale, project.supported_locales,
	project.timezone, COALESCE(project.publisher_name, ''),
	COALESCE(project.publisher_url, ''), project.default_robots_policy,
	membership.role, project.created_at, project.updated_at
`

func scanAdminProject(row rowScanner) (AdminProject, error) {
	var project AdminProject
	var verifiedDomainsJSON, supportedLocalesJSON string
	err := row.Scan(
		&project.ID,
		&project.WorkspaceID,
		&project.WorkspaceSlug,
		&project.WorkspaceName,
		&project.Slug,
		&project.Name,
		&project.Status,
		&project.PublicProjectKey,
		&project.PrimaryDomain,
		&verifiedDomainsJSON,
		&project.BlogBasePath,
		&project.DefaultLocale,
		&supportedLocalesJSON,
		&project.Timezone,
		&project.PublisherName,
		&project.PublisherURL,
		&project.DefaultRobotsPolicy,
		&project.Role,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		return AdminProject{}, err
	}
	decodeInto(verifiedDomainsJSON, &project.VerifiedDomains)
	decodeInto(supportedLocalesJSON, &project.SupportedLocales)
	if project.VerifiedDomains == nil {
		project.VerifiedDomains = []string{}
	}
	if project.SupportedLocales == nil {
		project.SupportedLocales = []string{}
	}
	return project, nil
}

func scanAdminAPIKey(row rowScanner) (AdminAPIKey, error) {
	var key AdminAPIKey
	var scopesJSON string
	err := row.Scan(
		&key.ID,
		&key.ProjectID,
		&key.Environment,
		&key.Name,
		&key.TokenPrefix,
		&scopesJSON,
		&key.ExpiresAt,
		&key.LastUsedAt,
		&key.CreatedBy,
		&key.CreatedAt,
		&key.RevokedAt,
	)
	if err != nil {
		return AdminAPIKey{}, err
	}
	if err := json.Unmarshal([]byte(scopesJSON), &key.Scopes); err != nil {
		return AdminAPIKey{}, err
	}
	if key.Scopes == nil {
		key.Scopes = []string{}
	}
	return key, nil
}

func ensureWorkspace(ctx context.Context, tx *sql.Tx, slug, name string) (string, error) {
	if slug == "" {
		slug = "default"
	}
	if name == "" {
		name = "Default workspace"
	}
	var existingID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE slug = ?`, slug).Scan(&existingID)
	if err == nil {
		return existingID, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	workspaceID, err := security.RandomID("wrk")
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workspaces(id, slug, name)
		VALUES (?, ?, ?)
	`, workspaceID, slug, name); err != nil {
		return "", err
	}
	return workspaceID, nil
}

func applyProjectDefaults(input ProjectInput) ProjectInput {
	input.WorkspaceSlug = slugify(input.WorkspaceSlug)
	input.Slug = slugify(input.Slug)
	input.Name = strings.TrimSpace(input.Name)
	input.PrimaryDomain = strings.TrimSpace(input.PrimaryDomain)
	input.PublisherName = strings.TrimSpace(input.PublisherName)
	input.PublisherURL = strings.TrimSpace(input.PublisherURL)
	input.VerifiedDomains = cleanStringSlice(input.VerifiedDomains)
	input.SupportedLocales = cleanStringSlice(input.SupportedLocales)
	if input.BlogBasePath == "" {
		input.BlogBasePath = "/blog"
	}
	if !strings.HasPrefix(input.BlogBasePath, "/") {
		input.BlogBasePath = "/" + input.BlogBasePath
	}
	if input.DefaultLocale == "" {
		input.DefaultLocale = "en"
	}
	if len(input.SupportedLocales) == 0 {
		input.SupportedLocales = []string{input.DefaultLocale}
	}
	if input.Timezone == "" {
		input.Timezone = "UTC"
	}
	if input.DefaultRobotsPolicy == "" {
		input.DefaultRobotsPolicy = "index,follow"
	}
	if input.WorkspaceName == "" {
		input.WorkspaceName = "Default workspace"
	}
	return input
}

func validateProjectInput(input ProjectInput) error {
	if input.Slug == "" {
		return errors.New("project slug is required")
	}
	if input.Name == "" {
		return errors.New("project name is required")
	}
	if strings.Contains(input.BlogBasePath, " ") {
		return errors.New("blog base path cannot contain spaces")
	}
	if input.DefaultLocale == "" {
		return errors.New("default locale is required")
	}
	return nil
}

func jsonString(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func cleanStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func applyAPIKeyDefaults(input APIKeyInput) APIKeyInput {
	input.Environment = strings.ToLower(strings.TrimSpace(input.Environment))
	if input.Environment == "" {
		input.Environment = "production"
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Scopes = cleanStringSlice(input.Scopes)
	if len(input.Scopes) == 0 {
		input.Scopes = append([]string(nil), defaultPublishedReadScopes...)
	}
	input.ExpiresAt = strings.TrimSpace(input.ExpiresAt)
	return input
}

func validateAPIKeyInput(input APIKeyInput) error {
	if input.Name == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	switch input.Environment {
	case "production", "staging", "development", "preview":
	default:
		return fmt.Errorf("%w: unsupported API key environment", ErrValidation)
	}
	for _, scope := range input.Scopes {
		if !allowedAPIKeyScope(scope) {
			return fmt.Errorf("%w: unsupported API key scope", ErrValidation)
		}
	}
	if input.ExpiresAt != "" {
		if _, err := parseSQLiteTime(input.ExpiresAt); err != nil {
			return fmt.Errorf("%w: expiresAt must be RFC3339 or YYYY-MM-DD HH:MM:SS", ErrValidation)
		}
	}
	return nil
}

func allowedAPIKeyScope(scope string) bool {
	switch scope {
	case "content:published:read", "taxonomy:published:read", "authors:published:read", "discovery:read", "redirects:read":
		return true
	default:
		return false
	}
}

func newProjectAPIKeySecret(environment string) (secret string, tokenPrefix string, tokenHash string, err error) {
	random, err := security.RandomToken(32)
	if err != nil {
		return "", "", "", err
	}
	envPart := "prod"
	switch environment {
	case "staging":
		envPart = "stg"
	case "development":
		envPart = "dev"
	case "preview":
		envPart = "prev"
	}
	secret = "sbk_" + envPart + "_" + random
	if len(secret) < 18 {
		return "", "", "", errors.New("generated API key secret is unexpectedly short")
	}
	tokenPrefix = secret[:18]
	tokenHash = security.TokenHash(secret)
	return secret, tokenPrefix, tokenHash, nil
}

func projectStatus(ctx context.Context, tx *sql.Tx, projectID string) (string, error) {
	var status string
	err := tx.QueryRowContext(ctx, `
		SELECT status
		FROM projects
		WHERE id = ?
	`, projectID).Scan(&status)
	return status, err
}

func insertAPIKeyAudit(ctx context.Context, tx *sql.Tx, projectID, actorUserID, action, keyID string, auditMetadata map[string]string) error {
	metadata, err := jsonString(auditMetadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_events(project_id, actor_type, actor_id, action, target_type, target_id, outcome, metadata_json)
		VALUES (?, 'user', ?, ?, 'api_key', ?, 'success', ?)
	`, projectID, actorUserID, action, keyID, metadata)
	return err
}

func parseSQLiteTime(raw string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC(), nil
	}
	return time.ParseInLocation(timeFormat, raw, time.UTC)
}
