package httpapi

import (
	"bytes"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"seoblog/apps/backend/internal/security"
	"seoblog/apps/backend/internal/store"
)

const (
	sessionCookieName       = "seoblog_session"
	csrfCookieName          = "seoblog_csrf"
	adminUserContextKey     = "adminUser"
	adminSessionContextKey  = "adminSession"
	sessionHashContextKey   = "adminSessionHash"
	sessionCookieMaxAge     = 30 * 24 * time.Hour
	reauthenticationWindow  = 5 * time.Minute
	defaultProjectListLimit = 50
	maxProjectListLimit     = 100
)

func (s *Server) registerAdminRoutes() {
	api := s.app.Group("/api/v1")

	api.Post("/auth/login", s.login)
	api.Post("/auth/forgot-password", func(c *fiber.Ctx) error { return notImplemented(c, "password reset request") })
	api.Post("/auth/reset-password", func(c *fiber.Ctx) error { return notImplemented(c, "password reset completion") })
	api.Post("/invitations/:token/accept", invitationAcceptanceSourceRateLimiter(), invitationTokenRateLimiter(), s.acceptInvitation)

	api.Get("/auth/me", s.requireAdminSession, s.currentUser)
	api.Get("/auth/csrf", s.requireAdminSession, s.csrfToken)
	api.Post("/auth/reauthenticate", s.requireAdminSession, s.requireAdminCSRF, s.reauthenticate)
	api.Post("/auth/logout", s.requireAdminSession, s.requireAdminCSRF, s.logout)

	api.Get("/projects", s.requireAdminSession, s.listProjects)
	api.Post("/projects", s.requireAdminSession, s.requireAdminCSRF, s.createProject)
	api.Get("/projects/:projectID", s.requireAdminSession, s.getProject)
	api.Patch("/projects/:projectID", s.requireAdminSession, s.requireAdminCSRF, s.updateProject)
	api.Post("/projects/:projectID/suspend", s.requireAdminSession, s.requireAdminCSRF, s.suspendProject)
	api.Post("/projects/:projectID/archive", s.requireAdminSession, s.requireAdminCSRF, s.archiveProject)
	api.Get("/projects/:projectID/deletion-impact", s.requireAdminSession, s.deletionImpact)
	api.Delete("/projects/:projectID", s.requireAdminSession, s.requireAdminCSRF, s.deleteProject)

	api.Get("/projects/:projectID/members", s.requireAdminSession, s.listProjectMembers)
	api.Post("/projects/:projectID/invitations", invitationCreationSourceRateLimiter(), s.requireAdminSession, s.requireAdminCSRF, invitationRecipientRateLimiter(), s.inviteProjectMember)
	api.Patch("/projects/:projectID/members/:userID", s.requireAdminSession, s.requireAdminCSRF, s.updateProjectMemberRole)
	api.Delete("/projects/:projectID/members/:userID", s.requireAdminSession, s.requireAdminCSRF, s.removeProjectMember)

	api.Get("/projects/:projectID/api-keys", s.requireAdminSession, s.listProjectAPIKeys)
	api.Post("/projects/:projectID/api-keys", s.requireAdminSession, s.requireAdminCSRF, s.requireRecentReauthentication, s.createProjectAPIKey)
	api.Post("/projects/:projectID/api-keys/:keyID/rotate", s.requireAdminSession, s.requireAdminCSRF, s.requireRecentReauthentication, s.rotateProjectAPIKey)
	api.Post("/projects/:projectID/api-keys/:keyID/revoke", s.requireAdminSession, s.requireAdminCSRF, s.requireRecentReauthentication, s.revokeProjectAPIKey)

	api.Get("/projects/:projectID/articles", s.requireAdminSession, s.listArticles)
	api.Post("/projects/:projectID/articles", s.requireAdminSession, s.requireAdminCSRF, s.createArticle)
	api.Get("/projects/:projectID/articles/:articleID", s.requireAdminSession, s.getArticle)
	api.Post("/projects/:projectID/articles/:articleID/revisions", s.requireAdminSession, s.requireAdminCSRF, s.createRevision)
	api.Post("/projects/:projectID/revisions/:revisionID/submit", s.requireAdminSession, s.requireAdminCSRF, s.submitRevision)
	api.Post("/projects/:projectID/revisions/:revisionID/request-changes", s.requireAdminSession, s.requireAdminCSRF, s.requestRevisionChanges)
	api.Post("/projects/:projectID/revisions/:revisionID/approve", s.requireAdminSession, s.requireAdminCSRF, s.approveRevision)
	api.Post("/projects/:projectID/articles/:articleID/publish", s.requireAdminSession, s.requireAdminCSRF, s.publishArticle)
	api.Post("/projects/:projectID/articles/:articleID/schedule", s.requireAdminSession, s.requireAdminCSRF, s.scheduleArticle)
	api.Post("/projects/:projectID/articles/:articleID/unpublish", s.requireAdminSession, s.requireAdminCSRF, s.unpublishArticle)
	api.Post("/projects/:projectID/articles/:articleID/rollback", s.requireAdminSession, s.requireAdminCSRF, s.rollbackArticle)
	api.Post("/projects/:projectID/articles/:articleID/copy-to-project", func(c *fiber.Ctx) error { return notImplemented(c, "article copy to project") })

	api.Get("/projects/:projectID/categories", s.requireAdminSession, s.listAdminCategories)
	api.Post("/projects/:projectID/categories", s.requireAdminSession, s.requireAdminCSRF, s.createCategory)
	api.Patch("/projects/:projectID/categories/:termID", s.requireAdminSession, s.requireAdminCSRF, s.updateCategory)
	api.Get("/projects/:projectID/tags", s.requireAdminSession, s.listAdminTags)
	api.Post("/projects/:projectID/tags", s.requireAdminSession, s.requireAdminCSRF, s.createTag)
	api.Get("/projects/:projectID/authors", s.requireAdminSession, s.listAdminAuthors)
	api.Post("/projects/:projectID/authors", s.requireAdminSession, s.requireAdminCSRF, s.createAuthor)
	api.Patch("/projects/:projectID/authors/:authorID", s.requireAdminSession, s.requireAdminCSRF, s.updateAuthor)
	api.Get("/projects/:projectID/series", s.requireAdminSession, s.listAdminSeries)
	api.Post("/projects/:projectID/series", s.requireAdminSession, s.requireAdminCSRF, s.createSeries)

	api.Get("/projects/:projectID/media", func(c *fiber.Ctx) error { return notImplemented(c, "media library") })
	api.Post("/projects/:projectID/media/uploads", func(c *fiber.Ctx) error { return notImplemented(c, "media upload initiation") })
	api.Get("/projects/:projectID/media/:assetID", func(c *fiber.Ctx) error { return notImplemented(c, "media detail") })
	api.Patch("/projects/:projectID/media/:assetID", func(c *fiber.Ctx) error { return notImplemented(c, "media metadata update") })
	api.Delete("/projects/:projectID/media/:assetID", func(c *fiber.Ctx) error { return notImplemented(c, "media deletion") })

	api.Get("/projects/:projectID/sources", func(c *fiber.Ctx) error { return notImplemented(c, "source library") })
	api.Post("/projects/:projectID/sources", func(c *fiber.Ctx) error { return notImplemented(c, "source creation") })
	api.Patch("/projects/:projectID/sources/:sourceID", func(c *fiber.Ctx) error { return notImplemented(c, "source update") })
	api.Get("/projects/:projectID/revisions/:revisionID/claims", func(c *fiber.Ctx) error { return notImplemented(c, "revision claims") })
	api.Post("/projects/:projectID/revisions/:revisionID/claims", func(c *fiber.Ctx) error { return notImplemented(c, "claim creation") })
	api.Post("/projects/:projectID/claims/:claimID/verify", func(c *fiber.Ctx) error { return notImplemented(c, "claim verification") })

	api.Get("/projects/:projectID/articles/:articleID/comments", s.requireAdminSession, s.listReviewComments)
	api.Post("/projects/:projectID/articles/:articleID/comments", s.requireAdminSession, s.requireAdminCSRF, s.createReviewComment)
	api.Post("/projects/:projectID/comments/:commentID/resolve", s.requireAdminSession, s.requireAdminCSRF, s.resolveReviewComment)
	api.Post("/projects/:projectID/comments/:commentID/reopen", s.requireAdminSession, s.requireAdminCSRF, s.reopenReviewComment)
	api.Get("/projects/:projectID/articles/:articleID/assignments", func(c *fiber.Ctx) error { return notImplemented(c, "review assignments") })
	api.Post("/projects/:projectID/articles/:articleID/assignments", func(c *fiber.Ctx) error { return notImplemented(c, "review assignment creation") })

	api.Get("/projects/:projectID/articles/:articleID/disclosures", func(c *fiber.Ctx) error { return notImplemented(c, "public disclosures") })
	api.Post("/projects/:projectID/articles/:articleID/disclosures", func(c *fiber.Ctx) error { return notImplemented(c, "public disclosure creation") })
	api.Get("/projects/:projectID/articles/:articleID/corrections", func(c *fiber.Ctx) error { return notImplemented(c, "correction notices") })
	api.Post("/projects/:projectID/articles/:articleID/corrections", func(c *fiber.Ctx) error { return notImplemented(c, "correction notice creation") })

	api.Get("/projects/:projectID/voice-profile", func(c *fiber.Ctx) error { return notImplemented(c, "voice profile") })
	api.Post("/projects/:projectID/voice-profile", func(c *fiber.Ctx) error { return notImplemented(c, "voice profile version creation") })
	api.Get("/projects/:projectID/evidence-packets", func(c *fiber.Ctx) error { return notImplemented(c, "evidence packet listing") })
	api.Post("/projects/:projectID/evidence-packets", func(c *fiber.Ctx) error { return notImplemented(c, "evidence packet creation") })
	api.Post("/projects/:projectID/evidence-packets/:packetID/approve", func(c *fiber.Ctx) error { return notImplemented(c, "evidence packet approval") })

	api.Post("/projects/:projectID/ai/jobs", func(c *fiber.Ctx) error { return notImplemented(c, "AI job creation") })
	api.Get("/projects/:projectID/ai/jobs/:jobID", func(c *fiber.Ctx) error { return notImplemented(c, "AI job detail") })
	api.Post("/projects/:projectID/ai/jobs/:jobID/cancel", func(c *fiber.Ctx) error { return notImplemented(c, "AI job cancellation") })
	api.Get("/projects/:projectID/ai/jobs/:jobID/events", func(c *fiber.Ctx) error { return notImplemented(c, "AI job event stream") })
	api.Get("/projects/:projectID/ai/runs", func(c *fiber.Ctx) error { return notImplemented(c, "AI run history") })
	api.Get("/projects/:projectID/quality-checks", func(c *fiber.Ctx) error { return notImplemented(c, "quality check results") })

	api.Get("/projects/:projectID/webhooks", func(c *fiber.Ctx) error { return notImplemented(c, "webhook endpoints") })
	api.Post("/projects/:projectID/webhooks", func(c *fiber.Ctx) error { return notImplemented(c, "webhook endpoint creation") })
	api.Post("/projects/:projectID/webhooks/:endpointID/revoke", func(c *fiber.Ctx) error { return notImplemented(c, "webhook endpoint revocation") })
	api.Get("/projects/:projectID/webhook-attempts", func(c *fiber.Ctx) error { return notImplemented(c, "webhook attempts") })
	api.Post("/projects/:projectID/webhook-attempts/:attemptID/replay", func(c *fiber.Ctx) error { return notImplemented(c, "webhook replay") })

	api.Get("/projects/:projectID/audit-events", s.requireAdminSession, s.listAuditEvents)
	api.Get("/projects/:projectID/delivery/status", func(c *fiber.Ctx) error { return notImplemented(c, "landing delivery status") })
	api.Post("/projects/:projectID/preview-tokens", func(c *fiber.Ctx) error { return notImplemented(c, "preview token creation") })
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type reauthenticationRequest struct {
	Password string `json:"password"`
}

type invitationAcceptanceRequest struct {
	Password string `json:"password"`
}

type reauthenticationResponse struct {
	ValidUntil string `json:"validUntil"`
}

type authResponse struct {
	User      store.AdminUser `json:"user"`
	CSRFToken string          `json:"csrfToken"`
}

type csrfResponse struct {
	CSRFToken string `json:"csrfToken"`
}

type projectRequest struct {
	WorkspaceID         string   `json:"workspaceId"`
	WorkspaceSlug       string   `json:"workspaceSlug"`
	WorkspaceName       string   `json:"workspaceName"`
	Slug                string   `json:"slug"`
	Name                string   `json:"name"`
	PrimaryDomain       string   `json:"primaryDomain"`
	VerifiedDomains     []string `json:"verifiedDomains"`
	BlogBasePath        string   `json:"blogBasePath"`
	DefaultLocale       string   `json:"defaultLocale"`
	SupportedLocales    []string `json:"supportedLocales"`
	Timezone            string   `json:"timezone"`
	PublisherName       string   `json:"publisherName"`
	PublisherURL        string   `json:"publisherUrl"`
	DefaultRobotsPolicy string   `json:"defaultRobotsPolicy"`
}

type projectPatchRequest struct {
	Name                *string   `json:"name"`
	PrimaryDomain       *string   `json:"primaryDomain"`
	VerifiedDomains     *[]string `json:"verifiedDomains"`
	BlogBasePath        *string   `json:"blogBasePath"`
	DefaultLocale       *string   `json:"defaultLocale"`
	SupportedLocales    *[]string `json:"supportedLocales"`
	Timezone            *string   `json:"timezone"`
	PublisherName       *string   `json:"publisherName"`
	PublisherURL        *string   `json:"publisherUrl"`
	DefaultRobotsPolicy *string   `json:"defaultRobotsPolicy"`
}

type memberInvitationRequest struct {
	Email     string `json:"email"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expiresAt"`
}

type memberPatchRequest struct {
	Role string `json:"role"`
}

type articleRequest struct {
	ArticleType       string `json:"articleType"`
	Title             string `json:"title"`
	Slug              string `json:"slug"`
	Locale            string `json:"locale"`
	PrimaryCategoryID string `json:"primaryCategoryId"`
	Deck              string `json:"deck"`
	Excerpt           string `json:"excerpt"`
	ShortAnswer       string `json:"shortAnswer"`
	BodyDocument      any    `json:"bodyDocument"`
	HTML              string `json:"html"`
}

type revisionRequest struct {
	Title             string `json:"title"`
	PrimaryCategoryID string `json:"primaryCategoryId"`
	Deck              string `json:"deck"`
	Excerpt           string `json:"excerpt"`
	ShortAnswer       string `json:"shortAnswer"`
	BodyDocument      any    `json:"bodyDocument"`
	HTML              string `json:"html"`
}

type revisionDecisionRequest struct {
	Note string `json:"note"`
}

type publicationRequest struct {
	RevisionID      string `json:"revisionId"`
	Slug            string `json:"slug"`
	Locale          string `json:"locale"`
	CanonicalURL    string `json:"canonicalUrl"`
	ScheduledFor    string `json:"scheduledFor"`
	ScheduledForUTC string `json:"scheduledForUtc"`
}

type rollbackRequest struct {
	RevisionID string `json:"revisionId"`
	Locale     string `json:"locale,omitempty"`
}

type apiKeyRequest struct {
	Environment string   `json:"environment"`
	Name        string   `json:"name"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   string   `json:"expiresAt"`
}

type termRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentID    string `json:"parentId"`
	Indexable   *bool  `json:"indexable"`
}

type termPatchRequest struct {
	Slug        *string `json:"slug"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	ParentID    *string `json:"parentId"`
	Indexable   *bool   `json:"indexable"`
}

type seriesRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Indexable   *bool  `json:"indexable"`
}

type authorRequest struct {
	Slug             string   `json:"slug"`
	DisplayName      string   `json:"displayName"`
	ShortBio         string   `json:"shortBio"`
	FullBio          string   `json:"fullBio"`
	PhotoAssetID     string   `json:"photoAssetId"`
	JobTitle         string   `json:"jobTitle"`
	Organization     string   `json:"organization"`
	Credentials      []string `json:"credentials"`
	Expertise        []string `json:"expertise"`
	ProfileURL       string   `json:"profileUrl"`
	ExternalProfiles []string `json:"externalProfiles"`
	SameAs           []string `json:"sameAs"`
	Status           string   `json:"status"`
}

type authorPatchRequest struct {
	Slug             *string   `json:"slug"`
	DisplayName      *string   `json:"displayName"`
	ShortBio         *string   `json:"shortBio"`
	FullBio          *string   `json:"fullBio"`
	PhotoAssetID     *string   `json:"photoAssetId"`
	JobTitle         *string   `json:"jobTitle"`
	Organization     *string   `json:"organization"`
	Credentials      *[]string `json:"credentials"`
	Expertise        *[]string `json:"expertise"`
	ProfileURL       *string   `json:"profileUrl"`
	ExternalProfiles *[]string `json:"externalProfiles"`
	SameAs           *[]string `json:"sameAs"`
	Status           *string   `json:"status"`
}

type reviewCommentRequest struct {
	RevisionID string `json:"revisionId"`
	BlockID    string `json:"blockId"`
	Body       string `json:"body"`
}

func (s *Server) login(c *fiber.Ctx) error {
	var input loginRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}

	credential, err := s.store.FindUserCredentialByEmail(c.UserContext(), input.Email)
	if err != nil {
		s.logger.Warn("login failed", "email", strings.ToLower(strings.TrimSpace(input.Email)), "error", err)
		return problem(c, fiber.StatusUnauthorized, "Invalid email or password", "")
	}
	if credential.User.Status != "active" || credential.PasswordHash == "" {
		return problem(c, fiber.StatusUnauthorized, "Invalid email or password", "")
	}
	valid, err := security.VerifyPassword(credential.PasswordHash, input.Password)
	if err != nil || !valid {
		if err != nil {
			s.logger.Warn("password verification failed", "user_id", credential.User.ID, "error", err)
		}
		return problem(c, fiber.StatusUnauthorized, "Invalid email or password", "")
	}

	sessionToken, err := security.RandomToken(32)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not create session", "")
	}
	csrfToken, err := security.RandomToken(32)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not create session", "")
	}
	if err := s.store.CreateSession(c.UserContext(), credential.User.ID, security.TokenHash(sessionToken), security.TokenHash(csrfToken), time.Now().UTC()); err != nil {
		s.logger.Error("create session", "user_id", credential.User.ID, "error", err)
		return problem(c, fiber.StatusInternalServerError, "Could not create session", "")
	}

	s.setSessionCookie(c, sessionToken)
	s.setCSRFCookie(c, csrfToken)
	return writeJSON(c, fiber.StatusOK, Envelope[authResponse]{
		Data: authResponse{User: credential.User, CSRFToken: csrfToken},
	})
}

func (s *Server) currentUser(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminUser]{Data: user})
}

func (s *Server) acceptInvitation(c *fiber.Ctx) error {
	var input invitationAcceptanceRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid invitation", "The invitation is invalid, expired, or already used")
	}
	acceptance, err := s.store.AcceptProjectInvitation(c.UserContext(), c.Params("token"), input.Password)
	if err != nil {
		if errors.Is(err, store.ErrValidation) {
			return problem(c, fiber.StatusBadRequest, "Invalid password", err.Error())
		}
		if errors.Is(err, store.ErrInvalidInvitation) {
			return problem(c, fiber.StatusBadRequest, "Invalid invitation", "The invitation is invalid, expired, or already used")
		}
		s.logger.Error("accept invitation", "error", err)
		return problem(c, fiber.StatusInternalServerError, "Could not accept invitation", "")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.ProjectInvitationAcceptance]{Data: acceptance})
}

func (s *Server) reauthenticate(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	sessionHash, ok := c.Locals(sessionHashContextKey).(string)
	if !ok || sessionHash == "" {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input reauthenticationRequest
	if err := decodeRequestBody(c, &input); err != nil || strings.TrimSpace(input.Password) == "" {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "Current password is required")
	}
	credential, err := s.store.FindUserCredentialByID(c.UserContext(), user.ID)
	if err != nil || credential.User.Status != "active" || credential.PasswordHash == "" {
		return problem(c, fiber.StatusUnauthorized, "Reauthentication failed", "The current password is incorrect")
	}
	valid, err := security.VerifyPassword(credential.PasswordHash, input.Password)
	if err != nil || !valid {
		return problem(c, fiber.StatusUnauthorized, "Reauthentication failed", "The current password is incorrect")
	}
	now := time.Now().UTC()
	if err := s.store.MarkSessionReauthenticated(c.UserContext(), sessionHash, now); err != nil {
		return problem(c, fiber.StatusUnauthorized, "Session expired", "Sign in again to continue")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[reauthenticationResponse]{
		Data: reauthenticationResponse{ValidUntil: now.Add(reauthenticationWindow).Format(time.RFC3339)},
	})
}

func (s *Server) csrfToken(c *fiber.Ctx) error {
	sessionHash, ok := c.Locals(sessionHashContextKey).(string)
	if !ok || sessionHash == "" {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	csrfToken, err := security.RandomToken(32)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not create CSRF token", "")
	}
	if err := s.store.RotateSessionCSRF(c.UserContext(), sessionHash, security.TokenHash(csrfToken)); err != nil {
		return problem(c, fiber.StatusUnauthorized, "Session expired", "")
	}
	s.setCSRFCookie(c, csrfToken)
	return writeJSON(c, fiber.StatusOK, Envelope[csrfResponse]{Data: csrfResponse{CSRFToken: csrfToken}})
}

func (s *Server) logout(c *fiber.Ctx) error {
	if sessionHash, ok := c.Locals(sessionHashContextKey).(string); ok && sessionHash != "" {
		if err := s.store.RevokeSession(c.UserContext(), sessionHash); err != nil {
			s.logger.Warn("revoke session", "error", err)
		}
	}
	s.clearAuthCookies(c)
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) listProjects(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), maxProjectListLimit)
	if limit == 0 {
		limit = defaultProjectListLimit
	}
	projects, err := s.store.ListProjectsForUser(c.UserContext(), user.ID, c.Query("cursor"), limit+1)
	if err != nil {
		s.logger.Error("list admin projects", "user_id", user.ID, "error", err)
		return problem(c, fiber.StatusInternalServerError, "Could not list projects", "")
	}
	nextCursor := ""
	if len(projects) > limit {
		projects = projects[:limit]
		nextCursor = projects[len(projects)-1].ID
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.AdminProject]{
		Data: projects,
		Meta: PageMeta{Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) createProject(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input projectRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	project, err := s.store.CreateProject(c.UserContext(), user.ID, input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create project")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.AdminProject]{Data: project})
}

func (s *Server) getProject(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	project, err := s.store.GetProjectForUser(c.UserContext(), user.ID, c.Params("projectID"))
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not load project")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminProject]{Data: project})
}

func (s *Server) updateProject(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input projectPatchRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	project, err := s.store.UpdateProject(c.UserContext(), user.ID, c.Params("projectID"), input.toStorePatch())
	if err != nil {
		return s.adminMutationError(c, err, "Could not update project")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminProject]{Data: project})
}

func (s *Server) suspendProject(c *fiber.Ctx) error {
	return s.setProjectStatus(c, "suspended")
}

func (s *Server) archiveProject(c *fiber.Ctx) error {
	return s.setProjectStatus(c, "archived")
}

func (s *Server) setProjectStatus(c *fiber.Ctx, status string) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	project, err := s.store.SetProjectStatus(c.UserContext(), user.ID, c.Params("projectID"), status)
	if err != nil {
		return s.adminMutationError(c, err, "Could not update project status")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminProject]{Data: project})
}

func (s *Server) deletionImpact(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	impact, err := s.store.ProjectDeletionImpact(c.UserContext(), user.ID, c.Params("projectID"))
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not load project deletion impact")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.ProjectDeletionImpact]{Data: impact})
}

func (s *Server) deleteProject(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	if err := s.store.DeleteProject(c.UserContext(), user.ID, c.Params("projectID")); err != nil {
		return s.adminMutationError(c, err, "Could not delete project")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) listProjectMembers(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	members, err := s.store.ListProjectMembers(c.UserContext(), user.ID, c.Params("projectID"), c.Query("cursor"), limit+1)
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list project members")
	}
	nextCursor := ""
	if len(members) > limit {
		members = members[:limit]
		nextCursor = members[len(members)-1].UserID
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.AdminProjectMember]{
		Data: members,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) inviteProjectMember(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input memberInvitationRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	invitation, err := s.store.InviteProjectMember(
		c.UserContext(),
		user.ID,
		c.Params("projectID"),
		input.toStoreInput(),
		recentlyReauthenticated(c),
	)
	if err != nil {
		return s.adminMutationError(c, err, "Could not invite project member")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.ProjectMemberInvitation]{Data: invitation})
}

func (s *Server) updateProjectMemberRole(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input memberPatchRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	member, err := s.store.UpdateProjectMemberRole(
		c.UserContext(),
		user.ID,
		c.Params("projectID"),
		c.Params("userID"),
		input.toStorePatch(),
		recentlyReauthenticated(c),
	)
	if err != nil {
		return s.adminMutationError(c, err, "Could not update project member")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminProjectMember]{Data: member})
}

func (s *Server) removeProjectMember(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	if err := s.store.RemoveProjectMember(
		c.UserContext(),
		user.ID,
		c.Params("projectID"),
		c.Params("userID"),
		recentlyReauthenticated(c),
	); err != nil {
		return s.adminMutationError(c, err, "Could not remove project member")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) listAuditEvents(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	cursor, err := decodeCursor[store.AuditCursor](c.Query("cursor"))
	if err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
	}
	events, err := s.store.ListAuditEventsForUser(c.UserContext(), user.ID, c.Params("projectID"), cursor, limit+1)
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list audit events")
	}
	nextCursor := ""
	if len(events) > limit {
		events = events[:limit]
		last := events[len(events)-1]
		nextCursor = encodeCursor(store.AuditCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.AuditEvent]{
		Data: events,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) listProjectAPIKeys(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	keys, err := s.store.ListProjectAPIKeys(c.UserContext(), user.ID, c.Params("projectID"), c.Query("cursor"), limit+1)
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list project API keys")
	}
	nextCursor := ""
	if len(keys) > limit {
		keys = keys[:limit]
		nextCursor = keys[len(keys)-1].ID
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.AdminAPIKey]{
		Data: keys,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) createProjectAPIKey(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input apiKeyRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	key, err := s.store.CreateProjectAPIKey(c.UserContext(), user.ID, c.Params("projectID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create project API key")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.APIKeyWithSecret]{Data: key})
}

func (s *Server) rotateProjectAPIKey(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	key, err := s.store.RotateProjectAPIKey(c.UserContext(), user.ID, c.Params("projectID"), c.Params("keyID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not rotate project API key")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.APIKeyWithSecret]{Data: key})
}

func (s *Server) revokeProjectAPIKey(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	key, err := s.store.RevokeProjectAPIKey(c.UserContext(), user.ID, c.Params("projectID"), c.Params("keyID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not revoke project API key")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminAPIKey]{Data: key})
}

func (s *Server) listArticles(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	articles, err := s.store.ListArticlesForUser(c.UserContext(), user.ID, c.Params("projectID"), c.Query("cursor"), limit+1)
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list articles")
	}
	nextCursor := ""
	if len(articles) > limit {
		articles = articles[:limit]
		nextCursor = articles[len(articles)-1].ID
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.AdminArticle]{
		Data: articles,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) createArticle(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input articleRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	article, err := s.store.CreateArticle(c.UserContext(), user.ID, c.Params("projectID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create article")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.AdminArticle]{Data: article})
}

func (s *Server) getArticle(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	article, err := s.store.GetArticleForUser(c.UserContext(), user.ID, c.Params("projectID"), c.Params("articleID"))
	if err != nil {
		return s.adminReadError(c, err, "Article not found", "Could not load article")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminArticle]{Data: article})
}

func (s *Server) createRevision(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input revisionRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	revision, err := s.store.CreateRevision(c.UserContext(), user.ID, c.Params("projectID"), c.Params("articleID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create revision")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.AdminRevision]{Data: revision})
}

func (s *Server) submitRevision(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	revision, err := s.store.SubmitRevision(c.UserContext(), user.ID, c.Params("projectID"), c.Params("revisionID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not submit revision")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminRevision]{Data: revision})
}

func (s *Server) requestRevisionChanges(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	revision, err := s.store.RequestRevisionChanges(c.UserContext(), user.ID, c.Params("projectID"), c.Params("revisionID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not request revision changes")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminRevision]{Data: revision})
}

func (s *Server) approveRevision(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input revisionDecisionRequest
	if len(c.Body()) > 0 {
		if err := decodeRequestBody(c, &input); err != nil {
			return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
		}
	}
	revision, err := s.store.ApproveRevision(c.UserContext(), user.ID, c.Params("projectID"), c.Params("revisionID"), input.Note)
	if err != nil {
		return s.adminMutationError(c, err, "Could not approve revision")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminRevision]{Data: revision})
}

func (s *Server) publishArticle(c *fiber.Ctx) error {
	return s.publicationAction(c, false)
}

func (s *Server) scheduleArticle(c *fiber.Ctx) error {
	return s.publicationAction(c, true)
}

func (s *Server) publicationAction(c *fiber.Ctx, scheduled bool) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input publicationRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	storeInput, err := input.toStoreInput(scheduled)
	if err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request", err.Error())
	}
	var article store.AdminArticle
	if scheduled {
		article, err = s.store.ScheduleArticle(c.UserContext(), user.ID, c.Params("projectID"), c.Params("articleID"), storeInput)
	} else {
		article, err = s.store.PublishArticle(c.UserContext(), user.ID, c.Params("projectID"), c.Params("articleID"), storeInput)
	}
	if err != nil {
		return s.adminMutationError(c, err, "Could not update publication")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminArticle]{Data: article})
}

func (s *Server) unpublishArticle(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	article, err := s.store.UnpublishArticle(c.UserContext(), user.ID, c.Params("projectID"), c.Params("articleID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not unpublish article")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminArticle]{Data: article})
}

func (s *Server) rollbackArticle(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input rollbackRequest
	if err := decodeStrictRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", err.Error())
	}
	article, err := s.store.RollbackArticle(
		c.UserContext(),
		user.ID,
		c.Params("projectID"),
		c.Params("articleID"),
		store.RollbackInput{RevisionID: input.RevisionID, Locale: input.Locale},
	)
	if err != nil {
		return s.adminMutationError(c, err, "Could not rollback article")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminArticle]{Data: article})
}

func (s *Server) listAdminCategories(c *fiber.Ctx) error {
	return s.listAdminTerms(c, "category")
}

func (s *Server) listAdminTags(c *fiber.Ctx) error {
	return s.listAdminTerms(c, "tag")
}

func (s *Server) listAdminTerms(c *fiber.Ctx, termType string) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	terms, err := s.store.ListAdminTerms(c.UserContext(), user.ID, c.Params("projectID"), termType)
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list taxonomy")
	}
	page, next, pageErr := paginateByID(terms, c.Query("cursor"), boundedLimit(c.Query("limit", "50"), 100), func(term store.TaxonomyTerm) string { return term.ID })
	if pageErr != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.TaxonomyTerm]{
		Data: page,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: len(page), NextCursor: next},
	})
}

func (s *Server) createCategory(c *fiber.Ctx) error {
	return s.createAdminTerm(c, "category")
}

func (s *Server) createTag(c *fiber.Ctx) error {
	return s.createAdminTerm(c, "tag")
}

func (s *Server) updateCategory(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input termPatchRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	term, err := s.store.UpdateTerm(c.UserContext(), user.ID, c.Params("projectID"), c.Params("termID"), "category", input.toStorePatch())
	if err != nil {
		return s.adminMutationError(c, err, "Could not update taxonomy term")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.TaxonomyTerm]{Data: term})
}

func (s *Server) createAdminTerm(c *fiber.Ctx, termType string) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input termRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	term, err := s.store.CreateTerm(c.UserContext(), user.ID, c.Params("projectID"), termType, input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create taxonomy term")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.TaxonomyTerm]{Data: term})
}

func (s *Server) listAdminSeries(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	items, err := s.store.ListAdminSeries(c.UserContext(), user.ID, c.Params("projectID"))
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list series")
	}
	page, next, pageErr := paginateByID(items, c.Query("cursor"), boundedLimit(c.Query("limit", "50"), 100), func(item store.Series) string { return item.ID })
	if pageErr != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.Series]{
		Data: page,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: len(page), NextCursor: next},
	})
}

func (s *Server) createSeries(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input seriesRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	series, err := s.store.CreateSeries(c.UserContext(), user.ID, c.Params("projectID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create series")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.Series]{Data: series})
}

func (s *Server) listAdminAuthors(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	authors, err := s.store.ListAuthorsForUser(c.UserContext(), user.ID, c.Params("projectID"))
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list authors")
	}
	page, next, pageErr := paginateByID(authors, c.Query("cursor"), boundedLimit(c.Query("limit", "50"), 100), func(author store.Author) string { return author.ID })
	if pageErr != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.Author]{
		Data: page,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: len(page), NextCursor: next},
	})
}

func (s *Server) createAuthor(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input authorRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	author, err := s.store.CreateAuthor(c.UserContext(), user.ID, c.Params("projectID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create author")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.Author]{Data: author})
}

func (s *Server) updateAuthor(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input authorPatchRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	author, err := s.store.UpdateAuthor(c.UserContext(), user.ID, c.Params("projectID"), c.Params("authorID"), input.toStorePatch())
	if err != nil {
		return s.adminMutationError(c, err, "Could not update author")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.Author]{Data: author})
}

func (s *Server) listReviewComments(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	comments, err := s.store.ListReviewComments(c.UserContext(), user.ID, c.Params("projectID"), c.Params("articleID"), c.Query("cursor"), limit+1)
	if err != nil {
		return s.adminReadError(c, err, "Article not found", "Could not list review comments")
	}
	nextCursor := ""
	if len(comments) > limit {
		comments = comments[:limit]
		nextCursor = comments[len(comments)-1].ID
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.ReviewComment]{
		Data: comments,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) createReviewComment(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input reviewCommentRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	comment, err := s.store.CreateReviewComment(c.UserContext(), user.ID, c.Params("projectID"), c.Params("articleID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create review comment")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.ReviewComment]{Data: comment})
}

func (s *Server) resolveReviewComment(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	comment, err := s.store.ResolveReviewComment(c.UserContext(), user.ID, c.Params("projectID"), c.Params("commentID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not resolve review comment")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.ReviewComment]{Data: comment})
}

func (s *Server) reopenReviewComment(c *fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	comment, err := s.store.ReopenReviewComment(c.UserContext(), user.ID, c.Params("projectID"), c.Params("commentID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not reopen review comment")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.ReviewComment]{Data: comment})
}

func (s *Server) requireAdminSession(c *fiber.Ctx) error {
	rawSession := c.Cookies(sessionCookieName)
	if rawSession == "" {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "Sign in to access the admin API")
	}
	sessionHash := security.TokenHash(rawSession)
	user, session, err := s.store.GetSessionUser(c.UserContext(), sessionHash)
	if err != nil {
		return problem(c, fiber.StatusUnauthorized, "Session expired", "Sign in again to continue")
	}
	c.Locals(adminUserContextKey, user)
	c.Locals(adminSessionContextKey, session)
	c.Locals(sessionHashContextKey, sessionHash)
	return c.Next()
}

func (s *Server) requireAdminCSRF(c *fiber.Ctx) error {
	if !s.sameOrigin(c) {
		return problem(c, fiber.StatusForbidden, "Invalid request origin", "")
	}
	session, ok := c.Locals(adminSessionContextKey).(store.Session)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	token := strings.TrimSpace(c.Get("X-CSRF-Token"))
	if token == "" {
		return problem(c, fiber.StatusForbidden, "Missing CSRF token", "")
	}
	actualHash := security.TokenHash(token)
	if subtle.ConstantTimeCompare([]byte(actualHash), []byte(session.CSRFTokenHash)) != 1 {
		return problem(c, fiber.StatusForbidden, "Invalid CSRF token", "")
	}
	return c.Next()
}

func (s *Server) requireRecentReauthentication(c *fiber.Ctx) error {
	if _, ok := c.Locals(adminSessionContextKey).(store.Session); !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	if !recentlyReauthenticated(c) {
		return problem(
			c,
			fiber.StatusForbidden,
			"Recent reauthentication required",
			"Confirm your current password to continue",
		)
	}
	return c.Next()
}

func recentlyReauthenticated(c *fiber.Ctx) bool {
	session, ok := c.Locals(adminSessionContextKey).(store.Session)
	if !ok {
		return false
	}
	reauthenticatedAt := parseDatabaseTime(session.ReauthenticatedAt)
	if reauthenticatedAt.IsZero() {
		return false
	}
	age := time.Since(reauthenticatedAt)
	return age >= 0 && age <= reauthenticationWindow
}

func (s *Server) adminReadError(c *fiber.Ctx, err error, notFoundTitle, internalTitle string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return problem(c, fiber.StatusNotFound, notFoundTitle, "")
	}
	if errors.Is(err, store.ErrForbidden) {
		return problem(c, fiber.StatusForbidden, "Insufficient permission", "")
	}
	s.logger.Error("admin read failed", "error", err)
	return problem(c, fiber.StatusInternalServerError, internalTitle, "")
}

func (s *Server) adminMutationError(c *fiber.Ctx, err error, internalTitle string) error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return problem(c, fiber.StatusNotFound, "Resource not found", "")
	case errors.Is(err, store.ErrForbidden):
		return problem(c, fiber.StatusForbidden, "Insufficient permission", "")
	case errors.Is(err, store.ErrValidation):
		return problem(c, fiber.StatusBadRequest, "Invalid request", err.Error())
	case errors.Is(err, store.ErrInvalidWorkflow):
		return problem(c, fiber.StatusConflict, "Invalid workflow transition", err.Error())
	case errors.Is(err, store.ErrRecentReauthentication):
		return problem(c, fiber.StatusForbidden, "Recent reauthentication required", "Confirm your current password to continue")
	case errors.Is(err, store.ErrProjectHasContent):
		return problem(c, fiber.StatusConflict, "Project cannot be deleted", "Resolve retained content before deleting this project")
	default:
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "required") || strings.Contains(message, "cannot contain") || strings.Contains(message, "unique") {
			return problem(c, fiber.StatusBadRequest, "Invalid request", err.Error())
		}
		s.logger.Error("admin mutation failed", "error", err)
		return problem(c, fiber.StatusInternalServerError, internalTitle, "")
	}
}

func (s *Server) setSessionCookie(c *fiber.Ctx, value string) {
	c.Cookie(&fiber.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  time.Now().Add(sessionCookieMaxAge),
		HTTPOnly: true,
		Secure:   s.secureCookies(),
		SameSite: fiber.CookieSameSiteStrictMode,
	})
}

func (s *Server) setCSRFCookie(c *fiber.Ctx, value string) {
	c.Cookie(&fiber.Cookie{
		Name:     csrfCookieName,
		Value:    value,
		Path:     "/",
		Expires:  time.Now().Add(sessionCookieMaxAge),
		HTTPOnly: false,
		Secure:   s.secureCookies(),
		SameSite: fiber.CookieSameSiteStrictMode,
	})
}

func (s *Server) clearAuthCookies(c *fiber.Ctx) {
	expires := time.Unix(0, 0)
	for _, name := range []string{sessionCookieName, csrfCookieName} {
		c.Cookie(&fiber.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Expires:  expires,
			HTTPOnly: name == sessionCookieName,
			Secure:   s.secureCookies(),
			SameSite: fiber.CookieSameSiteStrictMode,
		})
	}
}

func (s *Server) secureCookies() bool {
	return s.cfg.Env != "development"
}

func (s *Server) sameOrigin(c *fiber.Ctx) bool {
	origin := c.Get("Origin")
	if origin == "" {
		referer := c.Get("Referer")
		if referer == "" {
			return true
		}
		origin = referer
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	if s.cfg.Env == "development" && isLocalNuxtDevHost(parsed.Host) {
		return true
	}
	host := c.Get("X-Forwarded-Host")
	if host == "" {
		host = c.Get("Host")
	}
	if host == "" {
		host = c.Hostname()
	}
	return strings.EqualFold(parsed.Host, host)
}

func isLocalNuxtDevHost(host string) bool {
	return strings.EqualFold(host, "localhost:3000") ||
		strings.EqualFold(host, "127.0.0.1:3000") ||
		strings.EqualFold(host, "[::1]:3000")
}

func adminUser(c *fiber.Ctx) (store.AdminUser, bool) {
	value := c.Locals(adminUserContextKey)
	if value == nil {
		return store.AdminUser{}, false
	}
	user, ok := value.(store.AdminUser)
	return user, ok
}

func decodeRequestBody(c *fiber.Ctx, destination any) error {
	return json.Unmarshal(c.Body(), destination)
}

func decodeStrictRequestBody(c *fiber.Ctx, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain one JSON object")
		}
		return err
	}
	return nil
}

const sqliteUTCFormat = "2006-01-02 15:04:05"

func (input projectRequest) toStoreInput() store.ProjectInput {
	return store.ProjectInput{
		WorkspaceID:         input.WorkspaceID,
		WorkspaceSlug:       input.WorkspaceSlug,
		WorkspaceName:       input.WorkspaceName,
		Slug:                input.Slug,
		Name:                input.Name,
		PrimaryDomain:       input.PrimaryDomain,
		VerifiedDomains:     input.VerifiedDomains,
		BlogBasePath:        input.BlogBasePath,
		DefaultLocale:       input.DefaultLocale,
		SupportedLocales:    input.SupportedLocales,
		Timezone:            input.Timezone,
		PublisherName:       input.PublisherName,
		PublisherURL:        input.PublisherURL,
		DefaultRobotsPolicy: input.DefaultRobotsPolicy,
	}
}

func (input projectPatchRequest) toStorePatch() store.ProjectPatch {
	return store.ProjectPatch{
		Name:                input.Name,
		PrimaryDomain:       input.PrimaryDomain,
		VerifiedDomains:     input.VerifiedDomains,
		BlogBasePath:        input.BlogBasePath,
		DefaultLocale:       input.DefaultLocale,
		SupportedLocales:    input.SupportedLocales,
		Timezone:            input.Timezone,
		PublisherName:       input.PublisherName,
		PublisherURL:        input.PublisherURL,
		DefaultRobotsPolicy: input.DefaultRobotsPolicy,
	}
}

func (input memberInvitationRequest) toStoreInput() store.ProjectMemberInviteInput {
	return store.ProjectMemberInviteInput{
		Email:     input.Email,
		Role:      input.Role,
		ExpiresAt: input.ExpiresAt,
	}
}

func (input memberPatchRequest) toStorePatch() store.ProjectMemberPatch {
	return store.ProjectMemberPatch{Role: input.Role}
}

func (input articleRequest) toStoreInput() store.ArticleInput {
	return store.ArticleInput{
		ArticleType:       input.ArticleType,
		Title:             input.Title,
		Slug:              input.Slug,
		Locale:            input.Locale,
		PrimaryCategoryID: input.PrimaryCategoryID,
		Deck:              input.Deck,
		Excerpt:           input.Excerpt,
		ShortAnswer:       input.ShortAnswer,
		BodyDocument:      input.BodyDocument,
		HTML:              input.HTML,
	}
}

func (input revisionRequest) toStoreInput() store.RevisionInput {
	return store.RevisionInput{
		Title:             input.Title,
		PrimaryCategoryID: input.PrimaryCategoryID,
		Deck:              input.Deck,
		Excerpt:           input.Excerpt,
		ShortAnswer:       input.ShortAnswer,
		BodyDocument:      input.BodyDocument,
		HTML:              input.HTML,
	}
}

func (input publicationRequest) toStoreInput(scheduled bool) (store.PublicationInput, error) {
	scheduledForUTC := strings.TrimSpace(input.ScheduledForUTC)
	if scheduledForUTC == "" {
		scheduledForUTC = strings.TrimSpace(input.ScheduledFor)
	}
	if scheduled {
		parsed, err := parseAdminTime(scheduledForUTC)
		if err != nil {
			return store.PublicationInput{}, err
		}
		scheduledForUTC = parsed
	}
	return store.PublicationInput{
		RevisionID:      input.RevisionID,
		Slug:            input.Slug,
		Locale:          input.Locale,
		CanonicalURL:    input.CanonicalURL,
		ScheduledForUTC: scheduledForUTC,
	}, nil
}

func (input apiKeyRequest) toStoreInput() store.APIKeyInput {
	return store.APIKeyInput{
		Environment: input.Environment,
		Name:        input.Name,
		Scopes:      input.Scopes,
		ExpiresAt:   input.ExpiresAt,
	}
}

func (input termRequest) toStoreInput() store.TermInput {
	indexable := true
	if input.Indexable != nil {
		indexable = *input.Indexable
	}
	return store.TermInput{
		Slug:        input.Slug,
		Name:        input.Name,
		Description: input.Description,
		ParentID:    input.ParentID,
		Indexable:   indexable,
	}
}

func (input termPatchRequest) toStorePatch() store.TermPatch {
	return store.TermPatch{
		Slug:        input.Slug,
		Name:        input.Name,
		Description: input.Description,
		ParentID:    input.ParentID,
		Indexable:   input.Indexable,
	}
}

func (input seriesRequest) toStoreInput() store.SeriesInput {
	indexable := true
	if input.Indexable != nil {
		indexable = *input.Indexable
	}
	return store.SeriesInput{
		Slug:        input.Slug,
		Name:        input.Name,
		Description: input.Description,
		Indexable:   indexable,
	}
}

func (input authorRequest) toStoreInput() store.AuthorInput {
	return store.AuthorInput{
		Slug:             input.Slug,
		DisplayName:      input.DisplayName,
		ShortBio:         input.ShortBio,
		FullBio:          input.FullBio,
		PhotoAssetID:     input.PhotoAssetID,
		JobTitle:         input.JobTitle,
		Organization:     input.Organization,
		Credentials:      input.Credentials,
		Expertise:        input.Expertise,
		ProfileURL:       input.ProfileURL,
		ExternalProfiles: input.ExternalProfiles,
		SameAs:           input.SameAs,
		Status:           input.Status,
	}
}

func (input authorPatchRequest) toStorePatch() store.AuthorPatch {
	return store.AuthorPatch{
		Slug:             input.Slug,
		DisplayName:      input.DisplayName,
		ShortBio:         input.ShortBio,
		FullBio:          input.FullBio,
		PhotoAssetID:     input.PhotoAssetID,
		JobTitle:         input.JobTitle,
		Organization:     input.Organization,
		Credentials:      input.Credentials,
		Expertise:        input.Expertise,
		ProfileURL:       input.ProfileURL,
		ExternalProfiles: input.ExternalProfiles,
		SameAs:           input.SameAs,
		Status:           input.Status,
	}
}

func (input reviewCommentRequest) toStoreInput() store.ReviewCommentInput {
	return store.ReviewCommentInput{
		RevisionID: input.RevisionID,
		BlockID:    input.BlockID,
		Body:       input.Body,
	}
}

func parseAdminTime(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("scheduledForUtc is required")
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC().Format(sqliteUTCFormat), nil
	}
	parsed, err := time.ParseInLocation(sqliteUTCFormat, raw, time.UTC)
	if err != nil {
		return "", errors.New("scheduledForUtc must be RFC3339 or YYYY-MM-DD HH:MM:SS")
	}
	return parsed.UTC().Format(sqliteUTCFormat), nil
}
