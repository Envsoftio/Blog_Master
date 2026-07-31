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
	ErrInvalidInvitation       = errors.New("invalid or expired invitation")
	ErrProjectHasContent       = errors.New("project has retained content")
	ErrRecentReauthentication  = errors.New("recent reauthentication required")
)

type requestIDContextKey struct{}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

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
	ID                       string   `json:"id"`
	WorkspaceID              string   `json:"workspaceId"`
	WorkspaceSlug            string   `json:"workspaceSlug"`
	WorkspaceName            string   `json:"workspaceName"`
	Slug                     string   `json:"slug"`
	Name                     string   `json:"name"`
	Status                   string   `json:"status"`
	PublicProjectKey         string   `json:"publicProjectKey"`
	PrimaryDomain            string   `json:"primaryDomain,omitempty"`
	VerifiedDomains          []string `json:"verifiedDomains"`
	BlogBasePath             string   `json:"blogBasePath"`
	DefaultLocale            string   `json:"defaultLocale"`
	SupportedLocales         []string `json:"supportedLocales"`
	Timezone                 string   `json:"timezone"`
	PublisherName            string   `json:"publisherName,omitempty"`
	PublisherURL             string   `json:"publisherUrl,omitempty"`
	DefaultRobotsPolicy      string   `json:"defaultRobotsPolicy"`
	SoloOwnerApprovalEnabled bool     `json:"soloOwnerApprovalEnabled"`
	Role                     string   `json:"role,omitempty"`
	CreatedAt                string   `json:"createdAt"`
	UpdatedAt                string   `json:"updatedAt"`
}

type AdminProjectMember struct {
	ProjectID  string `json:"projectId"`
	UserID     string `json:"userId"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	UserStatus string `json:"userStatus"`
	InvitedBy  string `json:"invitedBy,omitempty"`
	InvitedAt  string `json:"invitedAt,omitempty"`
	JoinedAt   string `json:"joinedAt,omitempty"`
	UpdatedAt  string `json:"updatedAt"`
	RemovedAt  string `json:"removedAt,omitempty"`
}

type ProjectMemberInvitation struct {
	Member    AdminProjectMember `json:"member"`
	Token     string             `json:"token"`
	ExpiresAt string             `json:"expiresAt"`
}

type ProjectInvitationAcceptance struct {
	ProjectID string `json:"projectId"`
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Role      string `json:"role"`
}

type invitationAcceptanceCandidate struct {
	ProjectID    string
	UserID       string
	Email        string
	Role         string
	UserStatus   string
	PasswordHash string
}

type ProjectInput struct {
	WorkspaceID              string
	WorkspaceSlug            string
	WorkspaceName            string
	Slug                     string
	Name                     string
	PrimaryDomain            string
	VerifiedDomains          []string
	BlogBasePath             string
	DefaultLocale            string
	SupportedLocales         []string
	Timezone                 string
	PublisherName            string
	PublisherURL             string
	DefaultRobotsPolicy      string
	SoloOwnerApprovalEnabled bool
}

type ProjectPatch struct {
	Name                     *string
	PrimaryDomain            *string
	VerifiedDomains          *[]string
	BlogBasePath             *string
	DefaultLocale            *string
	SupportedLocales         *[]string
	Timezone                 *string
	PublisherName            *string
	PublisherURL             *string
	DefaultRobotsPolicy      *string
	SoloOwnerApprovalEnabled *bool
}

type ProjectMemberInviteInput struct {
	Email     string
	Role      string
	ExpiresAt string
}

type ProjectMemberPatch struct {
	Role string
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

type AuditCursor struct {
	CreatedAt string `json:"createdAt"`
	ID        string `json:"id"`
}

type AuditEvent struct {
	ID         string         `json:"id"`
	ProjectID  string         `json:"projectId,omitempty"`
	ActorType  string         `json:"actorType"`
	ActorID    string         `json:"actorId,omitempty"`
	Action     string         `json:"action"`
	TargetType string         `json:"targetType,omitempty"`
	TargetID   string         `json:"targetId,omitempty"`
	Outcome    string         `json:"outcome"`
	RequestID  string         `json:"requestId,omitempty"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  string         `json:"createdAt"`
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

type AuthorInput struct {
	Slug             string
	DisplayName      string
	ShortBio         string
	FullBio          string
	PhotoAssetID     string
	JobTitle         string
	Organization     string
	Credentials      []string
	Expertise        []string
	ProfileURL       string
	ExternalProfiles []string
	SameAs           []string
	LoginUserID      string
	Status           string
}

type AuthorPatch struct {
	Slug             *string
	DisplayName      *string
	ShortBio         *string
	FullBio          *string
	PhotoAssetID     *string
	JobTitle         *string
	Organization     *string
	Credentials      *[]string
	Expertise        *[]string
	ProfileURL       *string
	ExternalProfiles *[]string
	SameAs           *[]string
	LoginUserID      *string
	Status           *string
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
		  timezone, publisher_name, publisher_url, default_robots_policy,
		  solo_owner_approval_enabled, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		projectID, workspaceID, input.Slug, input.Name, publicProjectKey, nullIfEmpty(input.PrimaryDomain),
		verifiedDomainsJSON, input.BlogBasePath, input.DefaultLocale, supportedLocalesJSON,
		input.Timezone, nullIfEmpty(input.PublisherName), nullIfEmpty(input.PublisherURL),
		input.DefaultRobotsPolicy, input.SoloOwnerApprovalEnabled, actorUserID,
	); err != nil {
		return AdminProject{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'project_owner', 'active', CURRENT_TIMESTAMP)
	`, projectID, actorUserID); err != nil {
		return AdminProject{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "project.create", "project", projectID, "success", nil); err != nil {
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
	if patch.SoloOwnerApprovalEnabled != nil {
		if err := s.requireProjectOwner(ctx, actorUserID, projectID); err != nil {
			return AdminProject{}, err
		}
	}

	current, err := s.GetProjectForUser(ctx, actorUserID, projectID)
	if err != nil {
		return AdminProject{}, err
	}
	next := ProjectInput{
		WorkspaceID:              current.WorkspaceID,
		WorkspaceSlug:            current.WorkspaceSlug,
		WorkspaceName:            current.WorkspaceName,
		Slug:                     current.Slug,
		Name:                     current.Name,
		PrimaryDomain:            current.PrimaryDomain,
		VerifiedDomains:          current.VerifiedDomains,
		BlogBasePath:             current.BlogBasePath,
		DefaultLocale:            current.DefaultLocale,
		SupportedLocales:         current.SupportedLocales,
		Timezone:                 current.Timezone,
		PublisherName:            current.PublisherName,
		PublisherURL:             current.PublisherURL,
		DefaultRobotsPolicy:      current.DefaultRobotsPolicy,
		SoloOwnerApprovalEnabled: current.SoloOwnerApprovalEnabled,
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
	if patch.SoloOwnerApprovalEnabled != nil {
		next.SoloOwnerApprovalEnabled = *patch.SoloOwnerApprovalEnabled
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminProject{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
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
		    solo_owner_approval_enabled = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, next.Name, nullIfEmpty(next.PrimaryDomain), verifiedDomainsJSON, next.BlogBasePath,
		next.DefaultLocale, supportedLocalesJSON, next.Timezone, nullIfEmpty(next.PublisherName),
		nullIfEmpty(next.PublisherURL), next.DefaultRobotsPolicy, next.SoloOwnerApprovalEnabled, projectID)
	if err != nil {
		return AdminProject{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return AdminProject{}, err
	} else if changed != 1 {
		return AdminProject{}, sql.ErrNoRows
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "project.update", "project", projectID, "success", map[string]any{
		"solo_owner_approval_enabled": next.SoloOwnerApprovalEnabled,
	}); err != nil {
		return AdminProject{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminProject{}, err
	}
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminProject{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE projects
		SET status = ?, archived_at = %s, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, archivedAtSQL), status, projectID)
	if err != nil {
		return AdminProject{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return AdminProject{}, err
	} else if changed != 1 {
		return AdminProject{}, sql.ErrNoRows
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "project."+status, "project", projectID, "success", nil); err != nil {
		return AdminProject{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminProject{}, err
	}
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "project.delete", "project", projectID, "success", nil); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, projectID)
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
	return tx.Commit()
}

func (s *Store) ListAuditEventsForUser(ctx context.Context, actorUserID, projectID string, cursor AuditCursor, limit int) ([]AuditEvent, error) {
	if err := s.requireProjectManagement(ctx, actorUserID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(project_id, ''), actor_type, COALESCE(actor_id, ''),
		       action, COALESCE(target_type, ''), COALESCE(target_id, ''),
		       outcome, COALESCE(request_id, ''), metadata_json, created_at
		FROM audit_events
		WHERE project_id = ?
		  AND (
		    ? = '' OR
		    created_at < ? OR
		    (created_at = ? AND id < ?)
		  )
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, projectID, cursor.CreatedAt, cursor.CreatedAt, cursor.CreatedAt, cursor.ID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		event, err := scanAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) ListAuthorsForUser(ctx context.Context, actorUserID, projectID string) ([]Author, error) {
	role, err := s.projectRole(ctx, actorUserID, projectID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+adminAuthorColumns+`
		FROM authors author
		LEFT JOIN project_memberships membership
		  ON membership.project_id = author.project_id
		 AND membership.user_id = author.login_user_id
		LEFT JOIN users user
		  ON user.id = author.login_user_id
		WHERE author.project_id = ?
		ORDER BY author.display_name, author.id
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authors []Author
	for rows.Next() {
		author, err := scanAdminAuthor(rows)
		if err != nil {
			return nil, err
		}
		if !canManageProjectRole(role) {
			clearAuthorLoginMetadata(&author)
		}
		authors = append(authors, author)
	}
	return authors, rows.Err()
}

func (s *Store) GetAuthorForUser(ctx context.Context, actorUserID, projectID, authorID string) (Author, error) {
	role, err := s.projectRole(ctx, actorUserID, projectID)
	if err != nil {
		return Author{}, err
	}
	author, err := s.getAdminAuthorByID(ctx, projectID, authorID)
	if err != nil {
		return Author{}, err
	}
	if !canManageProjectRole(role) {
		clearAuthorLoginMetadata(&author)
	}
	return author, nil
}

func (s *Store) CreateAuthor(ctx context.Context, actorUserID, projectID string, input AuthorInput) (Author, error) {
	actorRole, err := s.projectRole(ctx, actorUserID, projectID)
	if err != nil {
		return Author{}, err
	}
	if !canManageAuthorsRole(actorRole) {
		return Author{}, ErrForbidden
	}
	input = applyAuthorDefaults(input)
	if err := validateAuthorInput(input); err != nil {
		return Author{}, err
	}
	if input.LoginUserID != "" && !canManageProjectRole(actorRole) {
		return Author{}, ErrForbidden
	}
	authorID, err := security.RandomID("auth")
	if err != nil {
		return Author{}, err
	}
	credentialsJSON, expertiseJSON, externalProfilesJSON, sameAsJSON, err := authorJSONFields(input)
	if err != nil {
		return Author{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Author{}, err
	}
	defer tx.Rollback()

	status, err := projectStatus(ctx, tx, projectID)
	if err != nil {
		return Author{}, err
	}
	if status != "active" {
		return Author{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	if err := validateAuthorPhotoAsset(ctx, tx, projectID, input.PhotoAssetID); err != nil {
		return Author{}, err
	}
	if err := validateAuthorLoginUser(ctx, tx, projectID, input.LoginUserID); err != nil {
		return Author{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO authors(
		  id, project_id, slug, display_name, short_bio, full_bio,
		  photo_asset_id, job_title, organization, credentials_json,
		  expertise_json, profile_url, external_profiles_json, same_as_json, login_user_id, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, authorID, projectID, input.Slug, input.DisplayName, nullIfEmpty(input.ShortBio),
		nullIfEmpty(input.FullBio), nullIfEmpty(input.PhotoAssetID), nullIfEmpty(input.JobTitle),
		nullIfEmpty(input.Organization), credentialsJSON, expertiseJSON, nullIfEmpty(input.ProfileURL),
		externalProfilesJSON, sameAsJSON, nullIfEmpty(input.LoginUserID), input.Status); err != nil {
		return Author{}, err
	}
	if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
		return Author{}, err
	}
	if err := insertAuthorOutbox(ctx, tx, projectID, authorID, "author.created", input); err != nil {
		return Author{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "author.create", "author", authorID, "success", nil); err != nil {
		return Author{}, err
	}
	if err := tx.Commit(); err != nil {
		return Author{}, err
	}
	author, err := s.getAdminAuthorByID(ctx, projectID, authorID)
	if err != nil {
		return Author{}, err
	}
	if !canManageProjectRole(actorRole) {
		clearAuthorLoginMetadata(&author)
	}
	return author, nil
}

func (s *Store) UpdateAuthor(ctx context.Context, actorUserID, projectID, authorID string, patch AuthorPatch) (Author, error) {
	actorRole, err := s.projectRole(ctx, actorUserID, projectID)
	if err != nil {
		return Author{}, err
	}
	if !canManageAuthorsRole(actorRole) {
		return Author{}, ErrForbidden
	}
	current, err := s.getAdminAuthorByID(ctx, projectID, authorID)
	if err != nil {
		return Author{}, err
	}
	next := AuthorInput{
		Slug:             current.Slug,
		DisplayName:      current.DisplayName,
		ShortBio:         current.ShortBio,
		FullBio:          current.FullBio,
		PhotoAssetID:     current.PhotoAssetID,
		JobTitle:         current.JobTitle,
		Organization:     current.Organization,
		Credentials:      current.Credentials,
		Expertise:        current.Expertise,
		ProfileURL:       current.ProfileURL,
		ExternalProfiles: current.ExternalProfiles,
		SameAs:           current.SameAs,
		LoginUserID:      current.LoginUserID,
		Status:           current.Status,
	}
	if patch.Slug != nil {
		next.Slug = *patch.Slug
	}
	if patch.DisplayName != nil {
		next.DisplayName = *patch.DisplayName
	}
	if patch.ShortBio != nil {
		next.ShortBio = *patch.ShortBio
	}
	if patch.FullBio != nil {
		next.FullBio = *patch.FullBio
	}
	if patch.PhotoAssetID != nil {
		next.PhotoAssetID = *patch.PhotoAssetID
	}
	if patch.JobTitle != nil {
		next.JobTitle = *patch.JobTitle
	}
	if patch.Organization != nil {
		next.Organization = *patch.Organization
	}
	if patch.Credentials != nil {
		next.Credentials = *patch.Credentials
	}
	if patch.Expertise != nil {
		next.Expertise = *patch.Expertise
	}
	if patch.ProfileURL != nil {
		next.ProfileURL = *patch.ProfileURL
	}
	if patch.ExternalProfiles != nil {
		next.ExternalProfiles = *patch.ExternalProfiles
	}
	if patch.SameAs != nil {
		next.SameAs = *patch.SameAs
	}
	loginPatched := patch.LoginUserID != nil
	if patch.LoginUserID != nil {
		if !canManageProjectRole(actorRole) {
			return Author{}, ErrForbidden
		}
		next.LoginUserID = *patch.LoginUserID
	}
	if patch.Status != nil {
		next.Status = strings.ToLower(strings.TrimSpace(*patch.Status))
		if next.Status == "" {
			return Author{}, fmt.Errorf("%w: author status is required", ErrValidation)
		}
	}
	next = applyAuthorDefaults(next)
	if err := validateAuthorInput(next); err != nil {
		return Author{}, err
	}
	credentialsJSON, expertiseJSON, externalProfilesJSON, sameAsJSON, err := authorJSONFields(next)
	if err != nil {
		return Author{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Author{}, err
	}
	defer tx.Rollback()

	status, err := projectStatus(ctx, tx, projectID)
	if err != nil {
		return Author{}, err
	}
	if status != "active" {
		return Author{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	if err := validateAuthorPhotoAsset(ctx, tx, projectID, next.PhotoAssetID); err != nil {
		return Author{}, err
	}
	if loginPatched {
		if err := validateAuthorLoginUser(ctx, tx, projectID, next.LoginUserID); err != nil {
			return Author{}, err
		}
	}
	var result sql.Result
	if loginPatched {
		result, err = tx.ExecContext(ctx, `
			UPDATE authors
			SET slug = ?,
			    display_name = ?,
			    short_bio = ?,
			    full_bio = ?,
			    photo_asset_id = ?,
			    job_title = ?,
			    organization = ?,
			    credentials_json = ?,
			    expertise_json = ?,
			    profile_url = ?,
			    external_profiles_json = ?,
			    same_as_json = ?,
			    login_user_id = ?,
			    status = ?,
			    updated_at = CURRENT_TIMESTAMP
			WHERE project_id = ? AND id = ?
		`, next.Slug, next.DisplayName, nullIfEmpty(next.ShortBio), nullIfEmpty(next.FullBio),
			nullIfEmpty(next.PhotoAssetID), nullIfEmpty(next.JobTitle), nullIfEmpty(next.Organization),
			credentialsJSON, expertiseJSON, nullIfEmpty(next.ProfileURL), externalProfilesJSON,
			sameAsJSON, nullIfEmpty(next.LoginUserID), next.Status, projectID, authorID)
	} else {
		result, err = tx.ExecContext(ctx, `
			UPDATE authors
			SET slug = ?,
			    display_name = ?,
			    short_bio = ?,
			    full_bio = ?,
			    photo_asset_id = ?,
			    job_title = ?,
			    organization = ?,
			    credentials_json = ?,
			    expertise_json = ?,
			    profile_url = ?,
			    external_profiles_json = ?,
			    same_as_json = ?,
			    status = ?,
			    updated_at = CURRENT_TIMESTAMP
			WHERE project_id = ? AND id = ?
		`, next.Slug, next.DisplayName, nullIfEmpty(next.ShortBio), nullIfEmpty(next.FullBio),
			nullIfEmpty(next.PhotoAssetID), nullIfEmpty(next.JobTitle), nullIfEmpty(next.Organization),
			credentialsJSON, expertiseJSON, nullIfEmpty(next.ProfileURL), externalProfilesJSON,
			sameAsJSON, next.Status, projectID, authorID)
	}
	if err != nil {
		return Author{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return Author{}, err
	} else if changed != 1 {
		return Author{}, sql.ErrNoRows
	}
	if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
		return Author{}, err
	}
	if err := insertAuthorOutbox(ctx, tx, projectID, authorID, "author.updated", next); err != nil {
		return Author{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "author.update", "author", authorID, "success", nil); err != nil {
		return Author{}, err
	}
	if err := tx.Commit(); err != nil {
		return Author{}, err
	}
	author, err := s.getAdminAuthorByID(ctx, projectID, authorID)
	if err != nil {
		return Author{}, err
	}
	if !canManageProjectRole(actorRole) {
		clearAuthorLoginMetadata(&author)
	}
	return author, nil
}

func (s *Store) DeleteAuthor(ctx context.Context, actorUserID, projectID, authorID string) (Author, error) {
	actorRole, err := s.projectRole(ctx, actorUserID, projectID)
	if err != nil {
		return Author{}, err
	}
	if !canManageAuthorsRole(actorRole) {
		return Author{}, ErrForbidden
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Author{}, err
	}
	defer tx.Rollback()

	status, err := projectStatus(ctx, tx, projectID)
	if err != nil {
		return Author{}, err
	}
	if status != "active" {
		return Author{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	var slug, authorStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT slug, status
		FROM authors
		WHERE project_id = ? AND id = ?
	`, projectID, authorID).Scan(&slug, &authorStatus); err != nil {
		return Author{}, err
	}
	if authorStatus == "inactive" {
		if err := tx.Commit(); err != nil {
			return Author{}, err
		}
		author, err := s.getAdminAuthorByID(ctx, projectID, authorID)
		if err != nil {
			return Author{}, err
		}
		if !canManageProjectRole(actorRole) {
			clearAuthorLoginMetadata(&author)
		}
		return author, nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE authors
		SET status = 'inactive',
		    updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ?
	`, projectID, authorID)
	if err != nil {
		return Author{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return Author{}, err
	} else if changed != 1 {
		return Author{}, sql.ErrNoRows
	}
	if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
		return Author{}, err
	}
	if err := insertAuthorOutbox(ctx, tx, projectID, authorID, "author.deleted", AuthorInput{
		Slug:   slug,
		Status: "inactive",
	}); err != nil {
		return Author{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "author.delete", "author", authorID, "success", nil); err != nil {
		return Author{}, err
	}
	if err := tx.Commit(); err != nil {
		return Author{}, err
	}
	author, err := s.getAdminAuthorByID(ctx, projectID, authorID)
	if err != nil {
		return Author{}, err
	}
	if !canManageProjectRole(actorRole) {
		clearAuthorLoginMetadata(&author)
	}
	return author, nil
}

func (s *Store) ListProjectMembers(ctx context.Context, actorUserID, projectID, cursor string, limit int) ([]AdminProjectMember, error) {
	if err := s.requireProjectManagement(ctx, actorUserID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+adminProjectMemberColumns+`
		FROM project_memberships membership
		JOIN users user ON user.id = membership.user_id
		WHERE membership.project_id = ?
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

func (s *Store) InviteProjectMember(ctx context.Context, actorUserID, projectID string, input ProjectMemberInviteInput, allowOwnershipChange bool) (ProjectMemberInvitation, error) {
	input = applyProjectMemberInviteDefaults(input)
	if err := validateProjectMemberInviteInput(input); err != nil {
		return ProjectMemberInvitation{}, err
	}

	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)
	if input.ExpiresAt != "" {
		parsed, err := parseSQLiteTime(input.ExpiresAt)
		if err != nil {
			return ProjectMemberInvitation{}, fmt.Errorf("%w: expiresAt must be RFC3339 or YYYY-MM-DD HH:MM:SS", ErrValidation)
		}
		if !parsed.After(time.Now().UTC()) {
			return ProjectMemberInvitation{}, fmt.Errorf("%w: expiresAt must be in the future", ErrValidation)
		}
		expiresAt = parsed
	}
	token, err := newInvitationToken()
	if err != nil {
		return ProjectMemberInvitation{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProjectMemberInvitation{}, err
	}
	defer tx.Rollback()

	actorRole, err := projectRoleTx(ctx, tx, actorUserID, projectID)
	if err != nil {
		return ProjectMemberInvitation{}, err
	}
	if actorRole != "project_owner" && (actorRole != "project_admin" || input.Role == "project_owner") {
		return ProjectMemberInvitation{}, ErrForbidden
	}
	if input.Role == "project_owner" && !allowOwnershipChange {
		return ProjectMemberInvitation{}, ErrRecentReauthentication
	}

	status, err := projectStatus(ctx, tx, projectID)
	if err != nil {
		return ProjectMemberInvitation{}, err
	}
	if status != "active" {
		return ProjectMemberInvitation{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}

	userID, userStatus, err := lookupUserForInvitation(ctx, tx, input.Email)
	if err != nil {
		return ProjectMemberInvitation{}, err
	}
	if userID == "" {
		userID, err = security.RandomID("usr")
		if err != nil {
			return ProjectMemberInvitation{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO users(id, email_normalized, status)
			VALUES (?, ?, 'invited')
		`, userID, input.Email); err != nil {
			return ProjectMemberInvitation{}, err
		}
	} else if userStatus == "disabled" {
		return ProjectMemberInvitation{}, fmt.Errorf("%w: disabled users cannot be invited", ErrInvalidWorkflow)
	}

	var currentStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT status
		FROM project_memberships
		WHERE project_id = ? AND user_id = ?
	`, projectID, userID).Scan(&currentStatus)
	switch {
	case err == sql.ErrNoRows:
		_, err = tx.ExecContext(ctx, `
			INSERT INTO project_memberships(project_id, user_id, role, status, invited_by, invited_at)
			VALUES (?, ?, ?, 'invited', ?, CURRENT_TIMESTAMP)
		`, projectID, userID, input.Role, actorUserID)
	case err != nil:
		return ProjectMemberInvitation{}, err
	case currentStatus == "active":
		return ProjectMemberInvitation{}, fmt.Errorf("%w: user is already an active project member", ErrInvalidWorkflow)
	default:
		_, err = tx.ExecContext(ctx, `
			UPDATE project_memberships
			SET role = ?,
			    status = 'invited',
			    invited_by = ?,
			    invited_at = CURRENT_TIMESTAMP,
			    joined_at = NULL,
			    updated_at = CURRENT_TIMESTAMP,
			    removed_at = NULL
			WHERE project_id = ? AND user_id = ?
		`, input.Role, actorUserID, projectID, userID)
	}
	if err != nil {
		return ProjectMemberInvitation{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE invitations
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE project_id = ?
		  AND email_normalized = ?
		  AND accepted_at IS NULL
		  AND revoked_at IS NULL
	`, projectID, input.Email); err != nil {
		return ProjectMemberInvitation{}, err
	}

	expiry := expiresAt.UTC().Format(timeFormat)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO invitations(token_hash, project_id, email_normalized, role, invited_by, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, security.TokenHash(token), projectID, input.Email, input.Role, actorUserID, expiry); err != nil {
		return ProjectMemberInvitation{}, err
	}
	if err := insertProjectMemberAudit(ctx, tx, projectID, actorUserID, "member.invite", userID, map[string]string{
		"email": input.Email,
		"role":  input.Role,
	}); err != nil {
		return ProjectMemberInvitation{}, err
	}

	member, err := getProjectMemberTx(ctx, tx, projectID, userID)
	if err != nil {
		return ProjectMemberInvitation{}, err
	}
	if err := tx.Commit(); err != nil {
		return ProjectMemberInvitation{}, err
	}
	return ProjectMemberInvitation{Member: member, Token: token, ExpiresAt: expiry}, nil
}

func (s *Store) AcceptProjectInvitation(ctx context.Context, token, password string) (ProjectInvitationAcceptance, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return ProjectInvitationAcceptance{}, ErrInvalidInvitation
	}
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		if errors.Is(err, security.ErrPasswordTooShort) {
			return ProjectInvitationAcceptance{}, fmt.Errorf("%w: %v", ErrValidation, err)
		}
		return ProjectInvitationAcceptance{}, err
	}

	tokenHash := security.TokenHash(token)
	candidate, err := s.invitationAcceptanceCandidate(ctx, tokenHash)
	if err != nil {
		return ProjectInvitationAcceptance{}, normalizeInvitationError(err)
	}
	if candidate.UserStatus == "active" {
		if candidate.PasswordHash == "" {
			return ProjectInvitationAcceptance{}, ErrInvalidInvitation
		}
		valid, err := security.VerifyPassword(candidate.PasswordHash, password)
		if err != nil || !valid {
			return ProjectInvitationAcceptance{}, ErrInvalidInvitation
		}
	} else if candidate.UserStatus != "invited" {
		return ProjectInvitationAcceptance{}, ErrInvalidInvitation
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProjectInvitationAcceptance{}, err
	}
	defer tx.Rollback()

	current, err := invitationAcceptanceCandidateTx(ctx, tx, tokenHash)
	if err != nil {
		return ProjectInvitationAcceptance{}, normalizeInvitationError(err)
	}
	if current != candidate {
		return ProjectInvitationAcceptance{}, ErrInvalidInvitation
	}

	if current.UserStatus == "invited" {
		result, err := tx.ExecContext(ctx, `
			UPDATE users
			SET password_hash = ?,
			    status = 'active',
			    email_verified_at = COALESCE(email_verified_at, CURRENT_TIMESTAMP),
			    password_changed_at = CURRENT_TIMESTAMP,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND status = 'invited'
		`, passwordHash, current.UserID)
		if err != nil {
			return ProjectInvitationAcceptance{}, err
		}
		if changed, err := result.RowsAffected(); err != nil || changed != 1 {
			if err != nil {
				return ProjectInvitationAcceptance{}, err
			}
			return ProjectInvitationAcceptance{}, ErrInvalidInvitation
		}
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE project_memberships
		SET status = 'active',
		    joined_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP,
		    removed_at = NULL
		WHERE project_id = ?
		  AND user_id = ?
		  AND role = ?
		  AND status = 'invited'
	`, current.ProjectID, current.UserID, current.Role)
	if err != nil {
		return ProjectInvitationAcceptance{}, err
	}
	if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return ProjectInvitationAcceptance{}, err
		}
		return ProjectInvitationAcceptance{}, ErrInvalidInvitation
	}

	result, err = tx.ExecContext(ctx, `
		UPDATE invitations
		SET accepted_at = CURRENT_TIMESTAMP
		WHERE token_hash = ?
		  AND accepted_at IS NULL
		  AND revoked_at IS NULL
		  AND expires_at > CURRENT_TIMESTAMP
	`, tokenHash)
	if err != nil {
		return ProjectInvitationAcceptance{}, err
	}
	if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return ProjectInvitationAcceptance{}, err
		}
		return ProjectInvitationAcceptance{}, ErrInvalidInvitation
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE invitations
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE project_id = ?
		  AND email_normalized = ?
		  AND token_hash <> ?
		  AND accepted_at IS NULL
		  AND revoked_at IS NULL
	`, current.ProjectID, current.Email, tokenHash); err != nil {
		return ProjectInvitationAcceptance{}, err
	}
	if err := insertProjectMemberAudit(ctx, tx, current.ProjectID, current.UserID, "member.accept", current.UserID, map[string]string{
		"email": current.Email,
		"role":  current.Role,
	}); err != nil {
		return ProjectInvitationAcceptance{}, err
	}
	if err := tx.Commit(); err != nil {
		return ProjectInvitationAcceptance{}, err
	}
	return ProjectInvitationAcceptance{
		ProjectID: current.ProjectID,
		UserID:    current.UserID,
		Email:     current.Email,
		Role:      current.Role,
	}, nil
}

func (s *Store) UpdateProjectMemberRole(ctx context.Context, actorUserID, projectID, targetUserID string, patch ProjectMemberPatch, allowOwnershipChange bool) (AdminProjectMember, error) {
	patch.Role = normalizeProjectRole(patch.Role)
	if !allowedProjectRole(patch.Role) {
		return AdminProjectMember{}, fmt.Errorf("%w: unsupported project role", ErrValidation)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminProjectMember{}, err
	}
	defer tx.Rollback()

	actorRole, err := projectRoleTx(ctx, tx, actorUserID, projectID)
	if err != nil {
		return AdminProjectMember{}, err
	}
	if actorRole != "project_owner" && actorRole != "project_admin" {
		return AdminProjectMember{}, ErrForbidden
	}

	var currentRole, currentStatus, targetEmail string
	err = tx.QueryRowContext(ctx, `
		SELECT membership.role, membership.status, user.email_normalized
		FROM project_memberships membership
		JOIN users user ON user.id = membership.user_id
		WHERE membership.project_id = ?
		  AND membership.user_id = ?
		  AND membership.status IN ('active', 'invited')
	`, projectID, targetUserID).Scan(&currentRole, &currentStatus, &targetEmail)
	if err != nil {
		return AdminProjectMember{}, err
	}
	if actorRole != "project_owner" && (currentRole == "project_owner" || patch.Role == "project_owner") {
		return AdminProjectMember{}, ErrForbidden
	}
	if currentRole != patch.Role && (currentRole == "project_owner" || patch.Role == "project_owner") && !allowOwnershipChange {
		return AdminProjectMember{}, ErrRecentReauthentication
	}
	if currentStatus == "active" && currentRole == "project_owner" && patch.Role != "project_owner" {
		if err := ensureAnotherActiveOwner(ctx, tx, projectID, targetUserID); err != nil {
			return AdminProjectMember{}, err
		}
	}
	if currentRole == patch.Role {
		return getProjectMemberTx(ctx, tx, projectID, targetUserID)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE project_memberships
		SET role = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND user_id = ? AND status IN ('active', 'invited')
	`, patch.Role, projectID, targetUserID); err != nil {
		return AdminProjectMember{}, err
	}
	if currentStatus == "invited" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE invitations
			SET role = ?
			WHERE project_id = ?
			  AND email_normalized = ?
			  AND accepted_at IS NULL
			  AND revoked_at IS NULL
			  AND expires_at > CURRENT_TIMESTAMP
		`, patch.Role, projectID, targetEmail); err != nil {
			return AdminProjectMember{}, err
		}
	} else if err := revokePendingInvitationsTx(ctx, tx, projectID, targetEmail); err != nil {
		return AdminProjectMember{}, err
	}
	if err := insertProjectMemberAudit(ctx, tx, projectID, actorUserID, "member.role_update", targetUserID, map[string]string{
		"fromRole": currentRole,
		"toRole":   patch.Role,
	}); err != nil {
		return AdminProjectMember{}, err
	}
	member, err := getProjectMemberTx(ctx, tx, projectID, targetUserID)
	if err != nil {
		return AdminProjectMember{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminProjectMember{}, err
	}
	return member, nil
}

func (s *Store) RemoveProjectMember(ctx context.Context, actorUserID, projectID, targetUserID string, allowOwnershipChange bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	actorRole, err := projectRoleTx(ctx, tx, actorUserID, projectID)
	if err != nil {
		return err
	}
	if actorRole != "project_owner" && actorRole != "project_admin" {
		return ErrForbidden
	}

	var currentRole, currentStatus, targetEmail string
	err = tx.QueryRowContext(ctx, `
		SELECT membership.role, membership.status, user.email_normalized
		FROM project_memberships membership
		JOIN users user ON user.id = membership.user_id
		WHERE membership.project_id = ?
		  AND membership.user_id = ?
		  AND membership.status IN ('active', 'invited')
	`, projectID, targetUserID).Scan(&currentRole, &currentStatus, &targetEmail)
	if err != nil {
		return err
	}
	if actorRole != "project_owner" && currentRole == "project_owner" {
		return ErrForbidden
	}
	if currentRole == "project_owner" && !allowOwnershipChange {
		return ErrRecentReauthentication
	}
	if currentStatus == "active" && currentRole == "project_owner" {
		if err := ensureAnotherActiveOwner(ctx, tx, projectID, targetUserID); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE project_memberships
		SET status = 'removed',
		    updated_at = CURRENT_TIMESTAMP,
		    removed_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND user_id = ? AND status IN ('active', 'invited')
	`, projectID, targetUserID)
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
	if err := revokePendingInvitationsTx(ctx, tx, projectID, targetEmail); err != nil {
		return err
	}
	if err := insertProjectMemberAudit(ctx, tx, projectID, actorUserID, "member.remove", targetUserID, map[string]string{
		"role":   currentRole,
		"status": currentStatus,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DisableProjectMemberLogin(ctx context.Context, actorUserID, projectID, targetUserID string) (AdminProjectMember, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminProjectMember{}, err
	}
	defer tx.Rollback()

	actorRole, err := projectRoleTx(ctx, tx, actorUserID, projectID)
	if err != nil {
		return AdminProjectMember{}, err
	}
	if actorRole != "project_owner" {
		return AdminProjectMember{}, ErrForbidden
	}
	if actorUserID == targetUserID {
		return AdminProjectMember{}, fmt.Errorf("%w: you cannot disable your own account", ErrInvalidWorkflow)
	}

	var currentRole, currentStatus, targetEmail, targetUserStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT membership.role, membership.status, user.email_normalized, user.status
		FROM project_memberships membership
		JOIN users user ON user.id = membership.user_id
		WHERE membership.project_id = ?
		  AND membership.user_id = ?
		  AND membership.status IN ('active', 'invited')
	`, projectID, targetUserID).Scan(&currentRole, &currentStatus, &targetEmail, &targetUserStatus)
	if err != nil {
		return AdminProjectMember{}, err
	}
	if currentStatus != "active" {
		return AdminProjectMember{}, fmt.Errorf("%w: only active project members can have login disabled", ErrInvalidWorkflow)
	}
	if targetUserStatus == "disabled" {
		return getProjectMemberTx(ctx, tx, projectID, targetUserID)
	}
	if targetUserStatus != "active" {
		return AdminProjectMember{}, fmt.Errorf("%w: only active users can have login disabled", ErrInvalidWorkflow)
	}
	if err := ensureUserCanBeDisabled(ctx, tx, targetUserID); err != nil {
		return AdminProjectMember{}, err
	}

	now := time.Now().UTC().Format(timeFormat)
	result, err := tx.ExecContext(ctx, `
		UPDATE users
		SET status = 'disabled',
		    updated_at = ?
		WHERE id = ?
		  AND status = 'active'
	`, now, targetUserID)
	if err != nil {
		return AdminProjectMember{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return AdminProjectMember{}, err
	}
	if changed != 1 {
		return AdminProjectMember{}, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sessions
		SET revoked_at = ?
		WHERE user_id = ?
		  AND revoked_at IS NULL
	`, now, targetUserID); err != nil {
		return AdminProjectMember{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE invitations
		SET revoked_at = ?
		WHERE email_normalized = ?
		  AND accepted_at IS NULL
		  AND revoked_at IS NULL
	`, now, targetEmail); err != nil {
		return AdminProjectMember{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "user.disable_login", "user", targetUserID, "success", map[string]string{
		"email":            targetEmail,
		"fromStatus":       targetUserStatus,
		"toStatus":         "disabled",
		"membershipRole":   currentRole,
		"membershipStatus": currentStatus,
	}); err != nil {
		return AdminProjectMember{}, err
	}
	member, err := getProjectMemberTx(ctx, tx, projectID, targetUserID)
	if err != nil {
		return AdminProjectMember{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminProjectMember{}, err
	}
	return member, nil
}

func (s *Store) EnableProjectMemberLogin(ctx context.Context, actorUserID, projectID, targetUserID string) (AdminProjectMember, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminProjectMember{}, err
	}
	defer tx.Rollback()

	actorRole, err := projectRoleTx(ctx, tx, actorUserID, projectID)
	if err != nil {
		return AdminProjectMember{}, err
	}
	if actorRole != "project_owner" {
		return AdminProjectMember{}, ErrForbidden
	}

	var currentRole, currentStatus, targetEmail, targetUserStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT membership.role, membership.status, user.email_normalized, user.status
		FROM project_memberships membership
		JOIN users user ON user.id = membership.user_id
		WHERE membership.project_id = ?
		  AND membership.user_id = ?
		  AND membership.status IN ('active', 'invited')
	`, projectID, targetUserID).Scan(&currentRole, &currentStatus, &targetEmail, &targetUserStatus)
	if err != nil {
		return AdminProjectMember{}, err
	}
	if currentStatus != "active" {
		return AdminProjectMember{}, fmt.Errorf("%w: only active project members can have login enabled", ErrInvalidWorkflow)
	}
	if targetUserStatus == "active" {
		return getProjectMemberTx(ctx, tx, projectID, targetUserID)
	}
	if targetUserStatus != "disabled" {
		return AdminProjectMember{}, fmt.Errorf("%w: only disabled users can have login enabled", ErrInvalidWorkflow)
	}

	now := time.Now().UTC().Format(timeFormat)
	result, err := tx.ExecContext(ctx, `
		UPDATE users
		SET status = 'active',
		    updated_at = ?
		WHERE id = ?
		  AND status = 'disabled'
	`, now, targetUserID)
	if err != nil {
		return AdminProjectMember{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return AdminProjectMember{}, err
	}
	if changed != 1 {
		return AdminProjectMember{}, sql.ErrNoRows
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "user.enable_login", "user", targetUserID, "success", map[string]string{
		"email":            targetEmail,
		"fromStatus":       targetUserStatus,
		"toStatus":         "active",
		"membershipRole":   currentRole,
		"membershipStatus": currentStatus,
	}); err != nil {
		return AdminProjectMember{}, err
	}
	member, err := getProjectMemberTx(ctx, tx, projectID, targetUserID)
	if err != nil {
		return AdminProjectMember{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminProjectMember{}, err
	}
	return member, nil
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
	if canManageProjectRole(role) {
		return nil
	}
	return ErrForbidden
}

func (s *Store) requireAuthorManage(ctx context.Context, userID, projectID string) error {
	role, err := s.projectRole(ctx, userID, projectID)
	if err != nil {
		return err
	}
	if canManageAuthorsRole(role) {
		return nil
	}
	return ErrForbidden
}

func canManageProjectRole(role string) bool {
	return role == "project_owner" || role == "project_admin"
}

func canManageAuthorsRole(role string) bool {
	return canManageProjectRole(role) || role == "editor"
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
	project.solo_owner_approval_enabled,
	membership.role, project.created_at, project.updated_at
`

const adminAuthorColumns = `
	author.id, author.slug, author.display_name, COALESCE(author.short_bio, ''), COALESCE(author.full_bio, ''),
	COALESCE(author.photo_asset_id, ''), COALESCE(author.job_title, ''), COALESCE(author.organization, ''),
	author.credentials_json, author.expertise_json, COALESCE(author.profile_url, ''),
	author.external_profiles_json, author.same_as_json, author.status, author.created_at, author.updated_at,
	COALESCE(author.login_user_id, ''), COALESCE(user.email_normalized, ''), COALESCE(membership.role, ''),
	COALESCE(membership.status, '')
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
		&project.SoloOwnerApprovalEnabled,
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

const adminProjectMemberColumns = `
	membership.project_id, user.id, user.email_normalized, membership.role, membership.status, user.status,
	COALESCE(membership.invited_by, ''), COALESCE(membership.invited_at, ''),
	COALESCE(membership.joined_at, ''), membership.updated_at, COALESCE(membership.removed_at, '')
`

func scanProjectMember(row rowScanner) (AdminProjectMember, error) {
	var member AdminProjectMember
	err := row.Scan(
		&member.ProjectID,
		&member.UserID,
		&member.Email,
		&member.Role,
		&member.Status,
		&member.UserStatus,
		&member.InvitedBy,
		&member.InvitedAt,
		&member.JoinedAt,
		&member.UpdatedAt,
		&member.RemovedAt,
	)
	return member, err
}

func scanAuditEvent(row rowScanner) (AuditEvent, error) {
	var event AuditEvent
	var metadataJSON string
	err := row.Scan(
		&event.ID,
		&event.ProjectID,
		&event.ActorType,
		&event.ActorID,
		&event.Action,
		&event.TargetType,
		&event.TargetID,
		&event.Outcome,
		&event.RequestID,
		&metadataJSON,
		&event.CreatedAt,
	)
	if err != nil {
		return AuditEvent{}, err
	}
	event.Metadata = decodeJSONObject(metadataJSON)
	return event, nil
}

func getProjectMemberTx(ctx context.Context, tx *sql.Tx, projectID, userID string) (AdminProjectMember, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT `+adminProjectMemberColumns+`
		FROM project_memberships membership
		JOIN users user ON user.id = membership.user_id
		WHERE membership.project_id = ? AND membership.user_id = ?
	`, projectID, userID)
	return scanProjectMember(row)
}

func (s *Store) getAuthorByID(ctx context.Context, projectID, authorID string) (Author, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+authorColumns+`
		FROM authors
		WHERE project_id = ? AND id = ?
	`, projectID, authorID)
	return scanAuthor(row)
}

func (s *Store) getAdminAuthorByID(ctx context.Context, projectID, authorID string) (Author, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+adminAuthorColumns+`
		FROM authors author
		LEFT JOIN project_memberships membership
		  ON membership.project_id = author.project_id
		 AND membership.user_id = author.login_user_id
		LEFT JOIN users user
		  ON user.id = author.login_user_id
		WHERE author.project_id = ? AND author.id = ?
	`, projectID, authorID)
	return scanAdminAuthor(row)
}

func scanAdminAuthor(row rowScanner) (Author, error) {
	var author Author
	var credentialsJSON, expertiseJSON, externalProfilesJSON, sameAsJSON string
	err := row.Scan(
		&author.ID,
		&author.Slug,
		&author.DisplayName,
		&author.ShortBio,
		&author.FullBio,
		&author.PhotoAssetID,
		&author.JobTitle,
		&author.Organization,
		&credentialsJSON,
		&expertiseJSON,
		&author.ProfileURL,
		&externalProfilesJSON,
		&sameAsJSON,
		&author.Status,
		&author.CreatedAt,
		&author.UpdatedAt,
		&author.LoginUserID,
		&author.LoginEmail,
		&author.LoginRole,
		&author.LoginStatus,
	)
	if err != nil {
		return Author{}, err
	}
	decodeInto(credentialsJSON, &author.Credentials)
	decodeInto(expertiseJSON, &author.Expertise)
	decodeInto(externalProfilesJSON, &author.ExternalProfiles)
	decodeInto(sameAsJSON, &author.SameAs)
	return author, nil
}

func clearAuthorLoginMetadata(author *Author) {
	author.LoginUserID = ""
	author.LoginEmail = ""
	author.LoginRole = ""
	author.LoginStatus = ""
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

func applyAuthorDefaults(input AuthorInput) AuthorInput {
	input.Slug = slugify(input.Slug)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.Slug == "" {
		input.Slug = slugify(input.DisplayName)
	}
	input.ShortBio = strings.TrimSpace(input.ShortBio)
	input.FullBio = strings.TrimSpace(input.FullBio)
	input.PhotoAssetID = strings.TrimSpace(input.PhotoAssetID)
	input.JobTitle = strings.TrimSpace(input.JobTitle)
	input.Organization = strings.TrimSpace(input.Organization)
	input.Credentials = cleanStringSlice(input.Credentials)
	input.Expertise = cleanStringSlice(input.Expertise)
	input.ProfileURL = strings.TrimSpace(input.ProfileURL)
	input.ExternalProfiles = cleanStringSlice(input.ExternalProfiles)
	input.SameAs = cleanStringSlice(input.SameAs)
	input.LoginUserID = strings.TrimSpace(input.LoginUserID)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = "active"
	}
	return input
}

func validateAuthorInput(input AuthorInput) error {
	if input.DisplayName == "" {
		return fmt.Errorf("%w: displayName is required", ErrValidation)
	}
	if input.Slug == "" {
		return fmt.Errorf("%w: author slug is required", ErrValidation)
	}
	if input.Status != "active" && input.Status != "inactive" {
		return fmt.Errorf("%w: unsupported author status", ErrValidation)
	}
	for _, value := range append(append([]string{}, input.ExternalProfiles...), input.SameAs...) {
		if !hasHTTPScheme(value) {
			return fmt.Errorf("%w: author profile links must use http or https", ErrValidation)
		}
	}
	if input.ProfileURL != "" && !hasHTTPScheme(input.ProfileURL) {
		return fmt.Errorf("%w: profileUrl must use http or https", ErrValidation)
	}
	return nil
}

func authorJSONFields(input AuthorInput) (credentialsJSON, expertiseJSON, externalProfilesJSON, sameAsJSON string, err error) {
	if credentialsJSON, err = jsonString(input.Credentials); err != nil {
		return "", "", "", "", err
	}
	if expertiseJSON, err = jsonString(input.Expertise); err != nil {
		return "", "", "", "", err
	}
	if externalProfilesJSON, err = jsonString(input.ExternalProfiles); err != nil {
		return "", "", "", "", err
	}
	if sameAsJSON, err = jsonString(input.SameAs); err != nil {
		return "", "", "", "", err
	}
	return credentialsJSON, expertiseJSON, externalProfilesJSON, sameAsJSON, nil
}

func validateAuthorPhotoAsset(ctx context.Context, tx *sql.Tx, projectID, photoAssetID string) error {
	if photoAssetID == "" {
		return nil
	}
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM assets
		WHERE project_id = ? AND id = ?
	`, projectID, photoAssetID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: photoAssetId must reference an asset in this project", ErrValidation)
	}
	return err
}

func validateAuthorLoginUser(ctx context.Context, tx *sql.Tx, projectID, loginUserID string) error {
	if loginUserID == "" {
		return nil
	}
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM project_memberships membership
		JOIN users user ON user.id = membership.user_id
		WHERE membership.project_id = ?
		  AND membership.user_id = ?
		  AND membership.status IN ('active','invited')
		  AND user.status IN ('active','invited')
	`, projectID, loginUserID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: loginUserId must reference an invited or active member of this project", ErrValidation)
	}
	return err
}

func insertAuthorOutbox(ctx context.Context, tx *sql.Tx, projectID, authorID, eventType string, author AuthorInput) error {
	payload, err := json.Marshal(map[string]string{
		"project_id": projectID,
		"author_id":  authorID,
		"slug":       author.Slug,
		"status":     author.Status,
	})
	if err != nil {
		return err
	}
	eventID, err := security.RandomID("event")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox_events(
		  id, project_id, event_type, aggregate_type, aggregate_id,
		  payload_json, idempotency_key
		) VALUES (?, ?, ?, 'author', ?, ?, ?)
	`, eventID, projectID, eventType, authorID, string(payload), fmt.Sprintf("%s:%s:%s", eventType, authorID, eventID))
	return err
}

func hasHTTPScheme(value string) bool {
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://")
}

func applyProjectMemberInviteDefaults(input ProjectMemberInviteInput) ProjectMemberInviteInput {
	input.Email = normalizeEmail(input.Email)
	input.Role = normalizeProjectRole(input.Role)
	if input.Role == "" {
		input.Role = "writer"
	}
	input.ExpiresAt = strings.TrimSpace(input.ExpiresAt)
	return input
}

func validateProjectMemberInviteInput(input ProjectMemberInviteInput) error {
	if input.Email == "" || !strings.Contains(input.Email, "@") {
		return fmt.Errorf("%w: a valid email is required", ErrValidation)
	}
	if !allowedProjectRole(input.Role) {
		return fmt.Errorf("%w: unsupported project role", ErrValidation)
	}
	return nil
}

func normalizeProjectRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

func allowedProjectRole(role string) bool {
	switch role {
	case "project_owner", "project_admin", "editor", "reviewer", "writer":
		return true
	default:
		return false
	}
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

func newInvitationToken() (string, error) {
	random, err := security.RandomToken(32)
	if err != nil {
		return "", err
	}
	return "sbinv_" + random, nil
}

func lookupUserForInvitation(ctx context.Context, tx *sql.Tx, email string) (userID string, status string, err error) {
	err = tx.QueryRowContext(ctx, `
		SELECT id, status
		FROM users
		WHERE email_normalized = ?
	`, email).Scan(&userID, &status)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return userID, status, err
}

func (s *Store) invitationAcceptanceCandidate(ctx context.Context, tokenHash string) (invitationAcceptanceCandidate, error) {
	return scanInvitationAcceptanceCandidate(s.db.QueryRowContext(ctx, invitationAcceptanceQuery, tokenHash))
}

func invitationAcceptanceCandidateTx(ctx context.Context, tx *sql.Tx, tokenHash string) (invitationAcceptanceCandidate, error) {
	return scanInvitationAcceptanceCandidate(tx.QueryRowContext(ctx, invitationAcceptanceQuery, tokenHash))
}

const invitationAcceptanceQuery = `
	SELECT invitation.project_id, membership.user_id, invitation.email_normalized,
	       membership.role, user.status, COALESCE(user.password_hash, '')
	FROM invitations invitation
	JOIN projects project
	  ON project.id = invitation.project_id
	 AND project.status = 'active'
	JOIN users user
	  ON user.email_normalized = invitation.email_normalized
	JOIN project_memberships membership
	  ON membership.project_id = invitation.project_id
	 AND membership.user_id = user.id
	 AND membership.status = 'invited'
	 AND membership.role = invitation.role
	WHERE invitation.token_hash = ?
	  AND invitation.accepted_at IS NULL
	  AND invitation.revoked_at IS NULL
	  AND invitation.expires_at > CURRENT_TIMESTAMP
`

func scanInvitationAcceptanceCandidate(row rowScanner) (invitationAcceptanceCandidate, error) {
	var candidate invitationAcceptanceCandidate
	err := row.Scan(
		&candidate.ProjectID,
		&candidate.UserID,
		&candidate.Email,
		&candidate.Role,
		&candidate.UserStatus,
		&candidate.PasswordHash,
	)
	return candidate, err
}

func normalizeInvitationError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidInvitation
	}
	return err
}

func projectRoleTx(ctx context.Context, tx *sql.Tx, userID, projectID string) (string, error) {
	var role string
	err := tx.QueryRowContext(ctx, `
		SELECT role
		FROM project_memberships
		WHERE user_id = ? AND project_id = ? AND status = 'active'
	`, userID, projectID).Scan(&role)
	return role, err
}

func revokePendingInvitationsTx(ctx context.Context, tx *sql.Tx, projectID, email string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE invitations
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE project_id = ?
		  AND email_normalized = ?
		  AND accepted_at IS NULL
		  AND revoked_at IS NULL
	`, projectID, email)
	return err
}

func ensureAnotherActiveOwner(ctx context.Context, tx *sql.Tx, projectID, excludedUserID string) error {
	var ownerCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM project_memberships membership
		JOIN users user ON user.id = membership.user_id
		WHERE membership.project_id = ?
		  AND membership.user_id <> ?
		  AND membership.role = 'project_owner'
		  AND membership.status = 'active'
		  AND user.status = 'active'
	`, projectID, excludedUserID).Scan(&ownerCount); err != nil {
		return err
	}
	if ownerCount == 0 {
		return fmt.Errorf("%w: every active project must retain at least one active owner", ErrInvalidWorkflow)
	}
	return nil
}

func ensureUserCanBeDisabled(ctx context.Context, tx *sql.Tx, targetUserID string) error {
	var projectID string
	err := tx.QueryRowContext(ctx, `
		SELECT membership.project_id
		FROM project_memberships membership
		WHERE membership.user_id = ?
		  AND membership.role = 'project_owner'
		  AND membership.status = 'active'
		  AND NOT EXISTS (
		    SELECT 1
		    FROM project_memberships other
		    JOIN users other_user ON other_user.id = other.user_id
		    WHERE other.project_id = membership.project_id
		      AND other.user_id <> membership.user_id
		      AND other.role = 'project_owner'
		      AND other.status = 'active'
		      AND other_user.status = 'active'
		  )
		LIMIT 1
	`, targetUserID).Scan(&projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: disabling this user would leave project %s without an active owner", ErrInvalidWorkflow, projectID)
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

func insertAuditEventTx(ctx context.Context, tx *sql.Tx, projectID, actorType, actorID, action, targetType, targetID, outcome string, auditMetadata any) error {
	eventID, err := security.RandomID("audit")
	if err != nil {
		return err
	}
	metadata, err := auditMetadataJSON(auditMetadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_events(id, project_id, actor_type, actor_id, action, target_type, target_id, outcome, request_id, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, eventID, projectID, actorType, nullIfEmpty(actorID), action, nullIfEmpty(targetType), nullIfEmpty(targetID), outcome, nullIfEmpty(requestIDFromContext(ctx)), metadata)
	return err
}

func insertProjectMemberAudit(ctx context.Context, tx *sql.Tx, projectID, actorUserID, action, targetUserID string, auditMetadata map[string]string) error {
	return insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, action, "project_member", targetUserID, "success", auditMetadata)
}

func insertAPIKeyAudit(ctx context.Context, tx *sql.Tx, projectID, actorUserID, action, keyID string, auditMetadata map[string]string) error {
	return insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, action, "api_key", keyID, "success", auditMetadata)
}

func auditMetadataJSON(auditMetadata any) (string, error) {
	if auditMetadata == nil {
		return "{}", nil
	}
	metadata, err := json.Marshal(auditMetadata)
	if err != nil {
		return "", err
	}
	if string(metadata) == "null" {
		return "{}", nil
	}
	return string(metadata), nil
}

func parseSQLiteTime(raw string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC(), nil
	}
	return time.ParseInLocation(timeFormat, raw, time.UTC)
}
