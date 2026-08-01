package httpapi

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"

	"seoblog/apps/backend/internal/mailer"
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
	passwordMinLength       = 15
	passwordMaxLength       = 128
)

func (s *Server) registerAdminRoutes() {
	api := s.app.Group("/api/v1")

	api.Post("/auth/login", s.login)
	api.Post("/auth/forgot-password", passwordResetRequestSourceRateLimiter(), passwordResetEmailRateLimiter(), s.forgotPassword)
	api.Post("/auth/reset-password", passwordResetCompletionSourceRateLimiter(), passwordResetTokenRateLimiter(), s.resetPassword)
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
	api.Post("/projects/:projectID/members/:userID/disable-login", s.requireAdminSession, s.requireAdminCSRF, s.requireRecentReauthentication, s.disableProjectMemberLogin)
	api.Post("/projects/:projectID/members/:userID/enable-login", s.requireAdminSession, s.requireAdminCSRF, s.requireRecentReauthentication, s.enableProjectMemberLogin)

	api.Get("/projects/:projectID/api-keys", s.requireAdminSession, s.listProjectAPIKeys)
	api.Post("/projects/:projectID/api-keys", s.requireAdminSession, s.requireAdminCSRF, s.requireRecentReauthentication, s.createProjectAPIKey)
	api.Post("/projects/:projectID/api-keys/:keyID/rotate", s.requireAdminSession, s.requireAdminCSRF, s.requireRecentReauthentication, s.rotateProjectAPIKey)
	api.Post("/projects/:projectID/api-keys/:keyID/revoke", s.requireAdminSession, s.requireAdminCSRF, s.requireRecentReauthentication, s.revokeProjectAPIKey)

	api.Get("/projects/:projectID/articles", s.requireAdminSession, s.listArticles)
	api.Post("/projects/:projectID/articles", s.requireAdminSession, s.requireAdminCSRF, s.createArticle)
	api.Get("/projects/:projectID/articles/:articleID", s.requireAdminSession, s.getArticle)
	api.Put("/projects/:projectID/articles/:articleID", s.requireAdminSession, s.requireAdminCSRF, s.updateArticle)
	api.Delete("/projects/:projectID/articles/:articleID", s.requireAdminSession, s.requireAdminCSRF, s.archiveArticle)
	api.Post("/projects/:projectID/articles/:articleID/restore", s.requireAdminSession, s.requireAdminCSRF, s.restoreArticle)
	api.Get("/projects/:projectID/articles/:articleID/autosave", s.requireAdminSession, s.getArticleAutosave)
	api.Put("/projects/:projectID/articles/:articleID/autosave", s.requireAdminSession, s.requireAdminCSRF, s.saveArticleAutosave)
	api.Delete("/projects/:projectID/articles/:articleID/autosave", s.requireAdminSession, s.requireAdminCSRF, s.deleteArticleAutosave)
	api.Post("/projects/:projectID/articles/:articleID/publish", s.requireAdminSession, s.requireAdminCSRF, s.publishArticle)
	api.Post("/projects/:projectID/articles/:articleID/schedule", s.requireAdminSession, s.requireAdminCSRF, s.scheduleArticle)
	api.Post("/projects/:projectID/articles/:articleID/unpublish", s.requireAdminSession, s.requireAdminCSRF, s.unpublishArticle)
	api.Post("/projects/:projectID/articles/:articleID/copy-to-project", s.requireAdminSession, s.requireAdminCSRF, s.copyArticleToProject)

	api.Get("/projects/:projectID/categories", s.requireAdminSession, s.listAdminCategories)
	api.Post("/projects/:projectID/categories", s.requireAdminSession, s.requireAdminCSRF, s.createCategory)
	api.Patch("/projects/:projectID/categories/:termID", s.requireAdminSession, s.requireAdminCSRF, s.updateCategory)
	api.Get("/projects/:projectID/tags", s.requireAdminSession, s.listAdminTags)
	api.Post("/projects/:projectID/tags", s.requireAdminSession, s.requireAdminCSRF, s.createTag)
	api.Get("/projects/:projectID/authors", s.requireAdminSession, s.listAdminAuthors)
	api.Post("/projects/:projectID/authors", s.requireAdminSession, s.requireAdminCSRF, s.createAuthor)
	api.Get("/projects/:projectID/authors/:authorID", s.requireAdminSession, s.getAdminAuthor)
	api.Patch("/projects/:projectID/authors/:authorID", s.requireAdminSession, s.requireAdminCSRF, s.updateAuthor)
	api.Delete("/projects/:projectID/authors/:authorID", s.requireAdminSession, s.requireAdminCSRF, s.deleteAuthor)
	api.Get("/projects/:projectID/series", s.requireAdminSession, s.listAdminSeries)
	api.Post("/projects/:projectID/series", s.requireAdminSession, s.requireAdminCSRF, s.createSeries)

	api.Get("/projects/:projectID/media", s.requireAdminSession, s.listMediaAssets)
	api.Post("/projects/:projectID/media/uploads", s.requireAdminSession, s.requireAdminCSRF, s.createMediaAsset)
	api.Get("/projects/:projectID/media/:assetID/file", s.requireAdminSession, s.serveMediaAssetFile)
	api.Get("/projects/:projectID/media/:assetID", s.requireAdminSession, s.getMediaAsset)
	api.Post("/projects/:projectID/media/:assetID/complete", s.requireAdminSession, s.requireAdminCSRF, s.completeMediaUpload)
	api.Patch("/projects/:projectID/media/:assetID", s.requireAdminSession, s.requireAdminCSRF, s.updateMediaAsset)
	api.Delete("/projects/:projectID/media/:assetID", s.requireAdminSession, s.requireAdminCSRF, s.deleteMediaAsset)

	api.Get("/projects/:projectID/sources", s.requireAdminSession, s.listSources)
	api.Post("/projects/:projectID/sources", s.requireAdminSession, s.requireAdminCSRF, s.createSource)
	api.Patch("/projects/:projectID/sources/:sourceID", s.requireAdminSession, s.requireAdminCSRF, s.updateSource)

	api.Get("/projects/:projectID/articles/:articleID/disclosures", s.requireAdminSession, s.listDisclosures)
	api.Post("/projects/:projectID/articles/:articleID/disclosures", s.requireAdminSession, s.requireAdminCSRF, s.createDisclosure)
	api.Get("/projects/:projectID/articles/:articleID/corrections", s.requireAdminSession, s.listCorrections)
	api.Post("/projects/:projectID/articles/:articleID/corrections", s.requireAdminSession, s.requireAdminCSRF, s.createCorrection)

	api.Get("/projects/:projectID/voice-profile", s.requireAdminSession, s.getVoiceProfile)
	api.Post("/projects/:projectID/voice-profile", s.requireAdminSession, s.requireAdminCSRF, s.createVoiceProfile)
	api.Get("/projects/:projectID/evidence-packets", s.requireAdminSession, s.listEvidencePackets)
	api.Post("/projects/:projectID/evidence-packets", s.requireAdminSession, s.requireAdminCSRF, s.createEvidencePacket)
	api.Post("/projects/:projectID/evidence-packets/:packetID/approve", s.requireAdminSession, s.requireAdminCSRF, s.approveEvidencePacket)

	api.Get("/projects/:projectID/ai/jobs", s.requireAdminSession, s.listAIJobs)
	api.Post("/projects/:projectID/ai/jobs", s.requireAdminSession, s.requireAdminCSRF, s.createAIJob)
	api.Get("/projects/:projectID/ai/jobs/:jobID", s.requireAdminSession, s.getAIJob)
	api.Post("/projects/:projectID/ai/jobs/:jobID/cancel", s.requireAdminSession, s.requireAdminCSRF, s.cancelAIJob)
	api.Get("/projects/:projectID/ai/jobs/:jobID/events", s.requireAdminSession, s.listAIJobEvents)
	api.Get("/projects/:projectID/ai/runs", s.requireAdminSession, s.listAIRuns)
	api.Get("/projects/:projectID/quality-checks", s.requireAdminSession, s.listQualityCheckResults)

	api.Get("/projects/:projectID/webhooks", s.requireAdminSession, s.listWebhooks)
	api.Post("/projects/:projectID/webhooks", s.requireAdminSession, s.requireAdminCSRF, s.createWebhook)
	api.Post("/projects/:projectID/webhooks/:endpointID/revoke", s.requireAdminSession, s.requireAdminCSRF, s.revokeWebhook)
	api.Get("/projects/:projectID/webhook-attempts", s.requireAdminSession, s.listWebhookAttempts)
	api.Post("/projects/:projectID/webhook-attempts/:attemptID/replay", s.requireAdminSession, s.requireAdminCSRF, s.replayWebhookAttempt)

	api.Get("/projects/:projectID/audit-events", s.requireAdminSession, s.listAuditEvents)
	api.Get("/projects/:projectID/delivery/status", s.requireAdminSession, s.deliveryStatus)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type forgotPasswordRequest struct {
	Email string `json:"email" format:"email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token" minLength:"1"`
	Password string `json:"password" minLength:"15" maxLength:"128"`
}

type passwordResetResponse struct {
	Data map[string]any `json:"data"`
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
	Slug                string   `json:"slug"`
	Name                string   `json:"name"`
	PrimaryDomain       string   `json:"primaryDomain"`
	VerifiedDomains     []string `json:"verifiedDomains"`
	BlogBasePath        string   `json:"blogBasePath"`
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
	ArticleType       string               `json:"articleType"`
	Title             string               `json:"title"`
	Slug              string               `json:"slug"`
	PrimaryCategoryID string               `json:"primaryCategoryId"`
	TagIDs            []string             `json:"tagIds"`
	Contributors      []contributorRequest `json:"contributors"`
	Deck              string               `json:"deck"`
	Excerpt           string               `json:"excerpt"`
	ShortAnswer       string               `json:"shortAnswer"`
	BodyDocument      any                  `json:"bodyDocument"`
	HTML              string               `json:"html"`
	SEO               seoRequest           `json:"seo"`
}

type articleSaveRequest struct {
	BaseRevisionID    string               `json:"baseRevisionId"`
	Title             string               `json:"title"`
	PrimaryCategoryID string               `json:"primaryCategoryId"`
	TagIDs            []string             `json:"tagIds"`
	Contributors      []contributorRequest `json:"contributors"`
	Deck              string               `json:"deck"`
	Excerpt           string               `json:"excerpt"`
	ShortAnswer       string               `json:"shortAnswer"`
	BodyDocument      any                  `json:"bodyDocument"`
	HTML              string               `json:"html"`
	SEO               seoRequest           `json:"seo"`
}

type articleAutosaveRequest struct {
	BaseRevisionID  string                      `json:"baseRevisionId"`
	ExpectedVersion *int64                      `json:"expectedVersion"`
	Draft           *store.ArticleAutosaveDraft `json:"draft"`
}

type contributorRequest struct {
	AuthorID string `json:"authorId"`
	Role     string `json:"role"`
	Position int    `json:"position"`
}

type seoRequest struct {
	Title            string `json:"title"`
	Description      string `json:"description"`
	Robots           string `json:"robots"`
	OpenGraphTitle   string `json:"openGraphTitle"`
	OpenGraphSummary string `json:"openGraphDescription"`
	OpenGraphImage   string `json:"openGraphImage"`
}

type publicationRequest struct {
	Slug            string  `json:"slug"`
	CanonicalURL    *string `json:"canonicalUrl"`
	ScheduledFor    string  `json:"scheduledFor"`
	ScheduledForUTC string  `json:"scheduledForUtc"`
}

type copyArticleRequest struct {
	DestinationProjectID string                          `json:"destinationProjectId"`
	PrimaryCategoryID    string                          `json:"primaryCategoryId"`
	Slug                 string                          `json:"slug"`
	CanonicalDecision    string                          `json:"canonicalDecision"`
	CanonicalOriginalURL string                          `json:"canonicalOriginalUrl,omitempty"`
	ContributorMappings  []copyContributorMappingRequest `json:"contributorMappings"`
}

type copyContributorMappingRequest struct {
	SourceAuthorID      string `json:"sourceAuthorId"`
	DestinationAuthorID string `json:"destinationAuthorId"`
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
	LoginUserID      string   `json:"loginUserId"`
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
	LoginUserID      *string   `json:"loginUserId"`
	Status           *string   `json:"status"`
}

type sourceRequest struct {
	Title                 string `json:"title"`
	Publisher             string `json:"publisher"`
	Author                string `json:"author"`
	URL                   string `json:"url"`
	PublicationDate       string `json:"publicationDate"`
	AccessedAt            string `json:"accessedAt"`
	SourceType            string `json:"sourceType"`
	IsPrimary             bool   `json:"isPrimary"`
	ArchivedCopyReference string `json:"archivedCopyReference"`
	Notes                 string `json:"notes"`
}

type sourcePatchRequest struct {
	Title                 *string `json:"title"`
	Publisher             *string `json:"publisher"`
	Author                *string `json:"author"`
	URL                   *string `json:"url"`
	PublicationDate       *string `json:"publicationDate"`
	AccessedAt            *string `json:"accessedAt"`
	SourceType            *string `json:"sourceType"`
	IsPrimary             *bool   `json:"isPrimary"`
	ArchivedCopyReference *string `json:"archivedCopyReference"`
	Notes                 *string `json:"notes"`
}

type disclosureRequest struct {
	RevisionID     string `json:"revisionId"`
	DisclosureType string `json:"disclosureType"`
	PublicText     string `json:"publicText"`
}

type correctionRequest struct {
	AffectedRevisionID string `json:"affectedRevisionId"`
	PublicNote         string `json:"publicNote"`
	SupersedesNoticeID string `json:"supersedesNoticeId"`
}

func (s *Server) login(c fiber.Ctx) error {
	var input loginRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}

	credential, err := s.store.FindUserCredentialByEmail(c.Context(), input.Email)
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
	if err := s.store.CreateSession(c.Context(), credential.User.ID, security.TokenHash(sessionToken), security.TokenHash(csrfToken), time.Now().UTC()); err != nil {
		s.logger.Error("create session", "user_id", credential.User.ID, "error", err)
		return problem(c, fiber.StatusInternalServerError, "Could not create session", "")
	}

	s.setSessionCookie(c, sessionToken)
	s.setCSRFCookie(c, csrfToken)
	return writeJSON(c, fiber.StatusOK, Envelope[authResponse]{
		Data: authResponse{User: credential.User, CSRFToken: csrfToken},
	})
}

func (s *Server) forgotPassword(c fiber.Ctx) error {
	var input forgotPasswordRequest
	if err := decodeStrictRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}

	token, err := security.RandomToken(32)
	if err != nil {
		s.logger.Error("generate password reset token", "error", err)
		return problem(c, fiber.StatusInternalServerError, "Could not request password reset", "")
	}
	target, created, err := s.store.CreatePasswordReset(
		c.Context(),
		input.Email,
		security.TokenHash(token),
		time.Now().UTC(),
	)
	if err != nil {
		s.logger.Error("create password reset", "error", err)
		return problem(c, fiber.StatusInternalServerError, "Could not request password reset", "")
	}
	if created {
		resetURL, urlErr := s.passwordResetURL(token)
		if urlErr != nil {
			s.logger.Error("build password reset URL", "user_id", target.UserID, "error", urlErr)
		} else {
			s.sendPasswordResetEmail(target, resetURL)
		}
	}
	return writeJSON(c, fiber.StatusAccepted, passwordResetResponse{Data: map[string]any{}})
}

func (s *Server) sendPasswordResetEmail(target store.PasswordResetTarget, resetURL string) {
	select {
	case s.mailSlots <- struct{}{}:
	default:
		s.logger.Error("password reset email queue is full", "user_id", target.UserID)
		return
	}
	go func() {
		defer func() { <-s.mailSlots }()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		safeResetURL := html.EscapeString(resetURL)
		if err := s.mailer.Send(ctx, mailer.Message{
			To:      target.Email,
			Subject: "Reset your SEO Blog password",
			Text: "A password reset was requested for your SEO Blog account.\n\n" +
				"Reset your password using this link:\n" + resetURL + "\n\n" +
				"This link expires in one hour and can be used only once. If you did not request it, you can ignore this email.",
			HTML: "<p>A password reset was requested for your SEO Blog account.</p>" +
				`<p><a href="` + safeResetURL + `">Reset your password</a></p>` +
				"<p>This link expires in one hour and can be used only once. If you did not request it, you can ignore this email.</p>",
		}); err != nil {
			s.logger.Error("send password reset email", "user_id", target.UserID, "error", err)
		}
	}()
}

func (s *Server) resetPassword(c fiber.Ctx) error {
	var input resetPasswordRequest
	if err := decodeStrictRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	input.Token = strings.TrimSpace(input.Token)
	if input.Token == "" {
		return problem(c, fiber.StatusBadRequest, "Invalid or expired reset link", "")
	}
	passwordLength := utf8.RuneCountInString(input.Password)
	if passwordLength < passwordMinLength || passwordLength > passwordMaxLength {
		return problem(c, fiber.StatusBadRequest, "Invalid password", "Password must be between 15 and 128 characters")
	}
	passwordHash, err := security.HashPassword(input.Password)
	if err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid password", "")
	}
	err = s.store.CompletePasswordReset(
		c.Context(),
		security.TokenHash(input.Token),
		passwordHash,
		time.Now().UTC(),
	)
	if errors.Is(err, store.ErrInvalidPasswordReset) {
		return problem(c, fiber.StatusBadRequest, "Invalid or expired reset link", "")
	}
	if err != nil {
		s.logger.Error("complete password reset", "error", err)
		return problem(c, fiber.StatusInternalServerError, "Could not reset password", "")
	}
	s.clearAuthCookies(c)
	return writeJSON(c, fiber.StatusOK, passwordResetResponse{Data: map[string]any{}})
}

func (s *Server) passwordResetURL(token string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(s.cfg.AdminPublicURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", errors.New("SEOBLOG_ADMIN_PUBLIC_URL must be an absolute URL")
	}
	if s.cfg.Env == "production" && base.Scheme != "https" {
		return "", errors.New("SEOBLOG_ADMIN_PUBLIC_URL must use HTTPS in production")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/reset-password"
	base.RawPath = ""
	base.Fragment = ""
	query := base.Query()
	query.Set("token", token)
	base.RawQuery = query.Encode()
	return base.String(), nil
}

func (s *Server) currentUser(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminUser]{Data: user})
}

func (s *Server) acceptInvitation(c fiber.Ctx) error {
	var input invitationAcceptanceRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid invitation", "The invitation is invalid, expired, or already used")
	}
	acceptance, err := s.store.AcceptProjectInvitation(c.Context(), c.Params("token"), input.Password)
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

func (s *Server) reauthenticate(c fiber.Ctx) error {
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
	credential, err := s.store.FindUserCredentialByID(c.Context(), user.ID)
	if err != nil || credential.User.Status != "active" || credential.PasswordHash == "" {
		return problem(c, fiber.StatusUnauthorized, "Reauthentication failed", "The current password is incorrect")
	}
	valid, err := security.VerifyPassword(credential.PasswordHash, input.Password)
	if err != nil || !valid {
		return problem(c, fiber.StatusUnauthorized, "Reauthentication failed", "The current password is incorrect")
	}
	now := time.Now().UTC()
	if err := s.store.MarkSessionReauthenticated(c.Context(), sessionHash, now); err != nil {
		return problem(c, fiber.StatusUnauthorized, "Session expired", "Sign in again to continue")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[reauthenticationResponse]{
		Data: reauthenticationResponse{ValidUntil: now.Add(reauthenticationWindow).Format(time.RFC3339)},
	})
}

func (s *Server) csrfToken(c fiber.Ctx) error {
	sessionHash, ok := c.Locals(sessionHashContextKey).(string)
	if !ok || sessionHash == "" {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	csrfToken, err := security.RandomToken(32)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "Could not create CSRF token", "")
	}
	if err := s.store.RotateSessionCSRF(c.Context(), sessionHash, security.TokenHash(csrfToken)); err != nil {
		return problem(c, fiber.StatusUnauthorized, "Session expired", "")
	}
	s.setCSRFCookie(c, csrfToken)
	return writeJSON(c, fiber.StatusOK, Envelope[csrfResponse]{Data: csrfResponse{CSRFToken: csrfToken}})
}

func (s *Server) logout(c fiber.Ctx) error {
	if sessionHash, ok := c.Locals(sessionHashContextKey).(string); ok && sessionHash != "" {
		if err := s.store.RevokeSession(c.Context(), sessionHash); err != nil {
			s.logger.Warn("revoke session", "error", err)
		}
	}
	s.clearAuthCookies(c)
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) listProjects(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), maxProjectListLimit)
	if limit == 0 {
		limit = defaultProjectListLimit
	}
	projects, err := s.store.ListProjectsForUser(c.Context(), user.ID, c.Query("cursor"), limit+1)
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

func (s *Server) createProject(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input projectRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	project, err := s.store.CreateProject(c.Context(), user.ID, input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create project")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.AdminProject]{Data: project})
}

func (s *Server) getProject(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	project, err := s.store.GetProjectForUser(c.Context(), user.ID, c.Params("projectID"))
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not load project")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminProject]{Data: project})
}

func (s *Server) updateProject(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input projectPatchRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	project, err := s.store.UpdateProject(c.Context(), user.ID, c.Params("projectID"), input.toStorePatch())
	if err != nil {
		return s.adminMutationError(c, err, "Could not update project")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminProject]{Data: project})
}

func (s *Server) suspendProject(c fiber.Ctx) error {
	return s.setProjectStatus(c, "suspended")
}

func (s *Server) archiveProject(c fiber.Ctx) error {
	return s.setProjectStatus(c, "archived")
}

func (s *Server) setProjectStatus(c fiber.Ctx, status string) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	project, err := s.store.SetProjectStatus(c.Context(), user.ID, c.Params("projectID"), status)
	if err != nil {
		return s.adminMutationError(c, err, "Could not update project status")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminProject]{Data: project})
}

func (s *Server) deletionImpact(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	impact, err := s.store.ProjectDeletionImpact(c.Context(), user.ID, c.Params("projectID"))
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not load project deletion impact")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.ProjectDeletionImpact]{Data: impact})
}

func (s *Server) deleteProject(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	if err := s.store.DeleteProject(c.Context(), user.ID, c.Params("projectID")); err != nil {
		return s.adminMutationError(c, err, "Could not delete project")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) listProjectMembers(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	members, err := s.store.ListProjectMembers(c.Context(), user.ID, c.Params("projectID"), c.Query("cursor"), limit+1)
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

func (s *Server) inviteProjectMember(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input memberInvitationRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	invitation, err := s.store.InviteProjectMember(
		c.Context(),
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

func (s *Server) updateProjectMemberRole(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input memberPatchRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	member, err := s.store.UpdateProjectMemberRole(
		c.Context(),
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

func (s *Server) removeProjectMember(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	if err := s.store.RemoveProjectMember(
		c.Context(),
		user.ID,
		c.Params("projectID"),
		c.Params("userID"),
		recentlyReauthenticated(c),
	); err != nil {
		return s.adminMutationError(c, err, "Could not remove project member")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) disableProjectMemberLogin(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	member, err := s.store.DisableProjectMemberLogin(
		c.Context(),
		user.ID,
		c.Params("projectID"),
		c.Params("userID"),
	)
	if err != nil {
		return s.adminMutationError(c, err, "Could not disable member login")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminProjectMember]{Data: member})
}

func (s *Server) enableProjectMemberLogin(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	member, err := s.store.EnableProjectMemberLogin(
		c.Context(),
		user.ID,
		c.Params("projectID"),
		c.Params("userID"),
	)
	if err != nil {
		return s.adminMutationError(c, err, "Could not enable member login")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminProjectMember]{Data: member})
}

func (s *Server) listAuditEvents(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	cursor, err := decodeCursor[store.AuditCursor](c.Query("cursor"))
	if err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid cursor", "")
	}
	events, err := s.store.ListAuditEventsForUser(c.Context(), user.ID, c.Params("projectID"), cursor, limit+1)
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

func (s *Server) listProjectAPIKeys(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	keys, err := s.store.ListProjectAPIKeys(c.Context(), user.ID, c.Params("projectID"), c.Query("cursor"), limit+1)
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

func (s *Server) createProjectAPIKey(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input apiKeyRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	key, err := s.store.CreateProjectAPIKey(c.Context(), user.ID, c.Params("projectID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create project API key")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.APIKeyWithSecret]{Data: key})
}

func (s *Server) rotateProjectAPIKey(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	key, err := s.store.RotateProjectAPIKey(c.Context(), user.ID, c.Params("projectID"), c.Params("keyID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not rotate project API key")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.APIKeyWithSecret]{Data: key})
}

func (s *Server) revokeProjectAPIKey(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	key, err := s.store.RevokeProjectAPIKey(c.Context(), user.ID, c.Params("projectID"), c.Params("keyID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not revoke project API key")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminAPIKey]{Data: key})
}

func (s *Server) listArticles(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	includeArchived := false
	if raw := strings.TrimSpace(c.Query("includeArchived")); raw != "" {
		var err error
		includeArchived, err = strconv.ParseBool(raw)
		if err != nil {
			return problem(c, fiber.StatusBadRequest, "Invalid request", "includeArchived must be true or false")
		}
	}
	articles, err := s.store.ListArticlesForUser(
		c.Context(),
		user.ID,
		c.Params("projectID"),
		c.Query("cursor"),
		limit+1,
		store.ArticleListFilter{
			Search:           c.Query("q"),
			PublicationState: c.Query("publicationState"),
			IncludeArchived:  includeArchived,
		},
	)
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

func (s *Server) createArticle(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input articleRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	article, err := s.store.CreateArticle(c.Context(), user.ID, c.Params("projectID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create article")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.AdminArticle]{Data: article})
}

func (s *Server) getArticle(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	article, err := s.store.GetArticleForUser(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID"))
	if err != nil {
		return s.adminReadError(c, err, "Article not found", "Could not load article")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminArticle]{Data: article})
}

func (s *Server) updateArticle(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input articleSaveRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	var provided map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &provided); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	current, err := s.store.GetArticleForUser(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID"))
	if err != nil {
		return s.adminReadError(c, err, "Article not found", "Could not load article before saving")
	}
	if current.LatestRevision != nil && strings.TrimSpace(input.BaseRevisionID) == "" {
		input.BaseRevisionID = current.LatestRevision.ID
	}
	if _, ok := provided["tagIds"]; !ok {
		input.TagIDs = current.TagIDs
	}
	if _, ok := provided["deck"]; !ok {
		input.Deck = current.Deck
	}
	if _, ok := provided["excerpt"]; !ok {
		input.Excerpt = current.Excerpt
	}
	if _, ok := provided["shortAnswer"]; !ok {
		input.ShortAnswer = current.ShortAnswer
	}
	_, bodyDocumentProvided := provided["bodyDocument"]
	_, htmlProvided := provided["html"]
	if bodyDocumentProvided != htmlProvided {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "bodyDocument and html must be saved together")
	}
	if !bodyDocumentProvided {
		input.BodyDocument = current.BodyDocument
	}
	if !htmlProvided {
		input.HTML = current.HTML
	}
	if rawSEO, ok := provided["seo"]; ok {
		var seoFields map[string]json.RawMessage
		if err := json.Unmarshal(rawSEO, &seoFields); err != nil {
			return problem(c, fiber.StatusBadRequest, "Invalid request body", "seo must be an object")
		}
		if _, ok := seoFields["title"]; !ok {
			input.SEO.Title = current.SEO.Title
		}
		if _, ok := seoFields["description"]; !ok {
			input.SEO.Description = current.SEO.Description
		}
		if _, ok := seoFields["robots"]; !ok {
			input.SEO.Robots = current.SEO.Robots
		}
		if _, ok := seoFields["openGraphTitle"]; !ok {
			input.SEO.OpenGraphTitle = current.SEO.OpenGraphTitle
		}
		if _, ok := seoFields["openGraphDescription"]; !ok {
			input.SEO.OpenGraphSummary = current.SEO.OpenGraphSummary
		}
		if _, ok := seoFields["openGraphImage"]; !ok {
			input.SEO.OpenGraphImage = current.SEO.OpenGraphImage
		}
	}
	if _, err := s.store.CreateRevision(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID"), input.toStoreInput()); err != nil {
		return s.adminMutationError(c, err, "Could not save article")
	}
	article, err := s.store.GetArticleForUser(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID"))
	if err != nil {
		return s.adminReadError(c, err, "Article not found", "Could not load saved article")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminArticle]{Data: article})
}

func (s *Server) archiveArticle(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	if err := s.store.ArchiveArticle(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID")); err != nil {
		return s.adminMutationError(c, err, "Could not archive article")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) restoreArticle(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	article, err := s.store.RestoreArticle(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not restore article")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminArticle]{Data: article})
}

func (s *Server) getArticleAutosave(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	autosave, err := s.store.GetArticleAutosaveForUser(
		c.Context(),
		user.ID,
		c.Params("projectID"),
		c.Params("articleID"),
	)
	if err != nil {
		return s.adminReadError(c, err, "Autosave not found", "Could not load article autosave")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.ArticleAutosave]{Data: autosave})
}

func (s *Server) saveArticleAutosave(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input articleAutosaveRequest
	if err := decodeStrictRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", err.Error())
	}
	if input.ExpectedVersion == nil || input.Draft == nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "expectedVersion and draft are required")
	}
	autosave, err := s.store.SaveArticleAutosave(
		c.Context(),
		user.ID,
		c.Params("projectID"),
		c.Params("articleID"),
		store.ArticleAutosaveInput{
			BaseRevisionID:  input.BaseRevisionID,
			ExpectedVersion: *input.ExpectedVersion,
			Draft:           *input.Draft,
		},
	)
	if err != nil {
		return s.adminMutationError(c, err, "Could not save article autosave")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.ArticleAutosave]{Data: autosave})
}

func (s *Server) deleteArticleAutosave(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	if err := s.store.DeleteArticleAutosave(
		c.Context(),
		user.ID,
		c.Params("projectID"),
		c.Params("articleID"),
	); err != nil {
		return s.adminMutationError(c, err, "Could not delete article autosave")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) publishArticle(c fiber.Ctx) error {
	return s.publicationAction(c, false)
}

func (s *Server) scheduleArticle(c fiber.Ctx) error {
	return s.publicationAction(c, true)
}

func (s *Server) publicationAction(c fiber.Ctx, scheduled bool) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input publicationRequest
	if len(c.Body()) > 0 {
		if err := decodeRequestBody(c, &input); err != nil {
			return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
		}
	}
	storeInput, err := input.toStoreInput(scheduled)
	if err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request", err.Error())
	}
	var article store.AdminArticle
	if scheduled {
		article, err = s.store.ScheduleArticle(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID"), storeInput)
	} else {
		article, err = s.store.PublishArticle(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID"), storeInput)
	}
	if err != nil {
		return s.adminMutationError(c, err, "Could not update publication")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminArticle]{Data: article})
}

func (s *Server) unpublishArticle(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	article, err := s.store.UnpublishArticle(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not unpublish article")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.AdminArticle]{Data: article})
}

func (s *Server) copyArticleToProject(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input copyArticleRequest
	if err := decodeStrictRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", err.Error())
	}
	article, err := s.store.CopyArticleToProject(
		c.Context(),
		user.ID,
		c.Params("projectID"),
		c.Params("articleID"),
		input.toStoreInput(),
	)
	if err != nil {
		return s.adminMutationError(c, err, "Could not copy article")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.AdminArticle]{Data: article})
}

func (s *Server) listAdminCategories(c fiber.Ctx) error {
	return s.listAdminTerms(c, "category")
}

func (s *Server) listAdminTags(c fiber.Ctx) error {
	return s.listAdminTerms(c, "tag")
}

func (s *Server) listAdminTerms(c fiber.Ctx, termType string) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	terms, err := s.store.ListAdminTerms(c.Context(), user.ID, c.Params("projectID"), termType)
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

func (s *Server) createCategory(c fiber.Ctx) error {
	return s.createAdminTerm(c, "category")
}

func (s *Server) createTag(c fiber.Ctx) error {
	return s.createAdminTerm(c, "tag")
}

func (s *Server) updateCategory(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input termPatchRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	term, err := s.store.UpdateTerm(c.Context(), user.ID, c.Params("projectID"), c.Params("termID"), "category", input.toStorePatch())
	if err != nil {
		return s.adminMutationError(c, err, "Could not update taxonomy term")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.TaxonomyTerm]{Data: term})
}

func (s *Server) createAdminTerm(c fiber.Ctx, termType string) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input termRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	term, err := s.store.CreateTerm(c.Context(), user.ID, c.Params("projectID"), termType, input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create taxonomy term")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.TaxonomyTerm]{Data: term})
}

func (s *Server) listAdminSeries(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	items, err := s.store.ListAdminSeries(c.Context(), user.ID, c.Params("projectID"))
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

func (s *Server) createSeries(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input seriesRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	series, err := s.store.CreateSeries(c.Context(), user.ID, c.Params("projectID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create series")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.Series]{Data: series})
}

func (s *Server) listAdminAuthors(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	authors, err := s.store.ListAuthorsForUser(c.Context(), user.ID, c.Params("projectID"))
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

func (s *Server) createAuthor(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input authorRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	author, err := s.store.CreateAuthor(c.Context(), user.ID, c.Params("projectID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create author")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.Author]{Data: author})
}

func (s *Server) getAdminAuthor(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	author, err := s.store.GetAuthorForUser(c.Context(), user.ID, c.Params("projectID"), c.Params("authorID"))
	if err != nil {
		return s.adminReadError(c, err, "Author not found", "Could not load author")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.Author]{Data: author})
}

func (s *Server) updateAuthor(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input authorPatchRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	author, err := s.store.UpdateAuthor(c.Context(), user.ID, c.Params("projectID"), c.Params("authorID"), input.toStorePatch())
	if err != nil {
		return s.adminMutationError(c, err, "Could not update author")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.Author]{Data: author})
}

func (s *Server) deleteAuthor(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	author, err := s.store.DeleteAuthor(c.Context(), user.ID, c.Params("projectID"), c.Params("authorID"))
	if err != nil {
		return s.adminMutationError(c, err, "Could not delete author")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.Author]{Data: author})
}

func (s *Server) listSources(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	limit := boundedLimit(c.Query("limit", "50"), 100)
	sources, err := s.store.ListSources(c.Context(), user.ID, c.Params("projectID"), c.Query("cursor"), limit+1)
	if err != nil {
		return s.adminReadError(c, err, "Project not found", "Could not list sources")
	}
	nextCursor := ""
	if len(sources) > limit {
		sources = sources[:limit]
		nextCursor = sources[len(sources)-1].ID
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.Source]{
		Data: sources,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: limit, NextCursor: nextCursor},
	})
}

func (s *Server) createSource(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input sourceRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	source, err := s.store.CreateSource(c.Context(), user.ID, c.Params("projectID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create source")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.Source]{Data: source})
}

func (s *Server) updateSource(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input sourcePatchRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	source, err := s.store.UpdateSource(c.Context(), user.ID, c.Params("projectID"), c.Params("sourceID"), input.toStorePatch())
	if err != nil {
		return s.adminMutationError(c, err, "Could not update source")
	}
	return writeJSON(c, fiber.StatusOK, Envelope[store.Source]{Data: source})
}

func (s *Server) listDisclosures(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	disclosures, err := s.store.ListDisclosures(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID"))
	if err != nil {
		return s.adminReadError(c, err, "Article not found", "Could not list disclosures")
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.Disclosure]{
		Data: disclosures,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: len(disclosures)},
	})
}

func (s *Server) createDisclosure(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input disclosureRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	disclosure, err := s.store.CreateDisclosure(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create disclosure")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.Disclosure]{Data: disclosure})
}

func (s *Server) listCorrections(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	corrections, err := s.store.ListCorrections(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID"))
	if err != nil {
		return s.adminReadError(c, err, "Article not found", "Could not list corrections")
	}
	return writeJSON(c, fiber.StatusOK, ListEnvelope[store.CorrectionNotice]{
		Data: corrections,
		Meta: PageMeta{ProjectID: c.Params("projectID"), Limit: len(corrections)},
	})
}

func (s *Server) createCorrection(c fiber.Ctx) error {
	user, ok := adminUser(c)
	if !ok {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "")
	}
	var input correctionRequest
	if err := decodeRequestBody(c, &input); err != nil {
		return problem(c, fiber.StatusBadRequest, "Invalid request body", "")
	}
	correction, err := s.store.CreateCorrection(c.Context(), user.ID, c.Params("projectID"), c.Params("articleID"), input.toStoreInput())
	if err != nil {
		return s.adminMutationError(c, err, "Could not create correction")
	}
	return writeJSON(c, fiber.StatusCreated, Envelope[store.CorrectionNotice]{Data: correction})
}

func (s *Server) requireAdminSession(c fiber.Ctx) error {
	rawSession := c.Cookies(sessionCookieName)
	if rawSession == "" {
		return problem(c, fiber.StatusUnauthorized, "Missing session", "Sign in to access the admin API")
	}
	sessionHash := security.TokenHash(rawSession)
	user, session, err := s.store.GetSessionUser(c.Context(), sessionHash)
	if err != nil {
		return problem(c, fiber.StatusUnauthorized, "Session expired", "Sign in again to continue")
	}
	c.Locals(adminUserContextKey, user)
	c.Locals(adminSessionContextKey, session)
	c.Locals(sessionHashContextKey, sessionHash)
	return c.Next()
}

func (s *Server) requireAdminCSRF(c fiber.Ctx) error {
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

func (s *Server) requireRecentReauthentication(c fiber.Ctx) error {
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

func recentlyReauthenticated(c fiber.Ctx) bool {
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

func (s *Server) adminReadError(c fiber.Ctx, err error, notFoundTitle, internalTitle string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return problem(c, fiber.StatusNotFound, notFoundTitle, "")
	}
	if errors.Is(err, store.ErrForbidden) {
		return problem(c, fiber.StatusForbidden, "Insufficient permission", "")
	}
	if errors.Is(err, store.ErrValidation) {
		return problem(c, fiber.StatusBadRequest, "Invalid request", err.Error())
	}
	s.logger.Error("admin read failed", "error", err)
	return problem(c, fiber.StatusInternalServerError, internalTitle, "")
}

func (s *Server) adminMutationError(c fiber.Ctx, err error, internalTitle string) error {
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

func (s *Server) setSessionCookie(c fiber.Ctx, value string) {
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

func (s *Server) setCSRFCookie(c fiber.Ctx, value string) {
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

func (s *Server) clearAuthCookies(c fiber.Ctx) {
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

func (s *Server) sameOrigin(c fiber.Ctx) bool {
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

func adminUser(c fiber.Ctx) (store.AdminUser, bool) {
	value := c.Locals(adminUserContextKey)
	if value == nil {
		return store.AdminUser{}, false
	}
	user, ok := value.(store.AdminUser)
	return user, ok
}

func decodeRequestBody(c fiber.Ctx, destination any) error {
	return json.Unmarshal(c.Body(), destination)
}

func decodeStrictRequestBody(c fiber.Ctx, destination any) error {
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
		Slug:                input.Slug,
		Name:                input.Name,
		PrimaryDomain:       input.PrimaryDomain,
		VerifiedDomains:     input.VerifiedDomains,
		BlogBasePath:        input.BlogBasePath,
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
		PrimaryCategoryID: input.PrimaryCategoryID,
		TagIDs:            input.TagIDs,
		Contributors:      contributorInputs(input.Contributors),
		Deck:              input.Deck,
		Excerpt:           input.Excerpt,
		ShortAnswer:       input.ShortAnswer,
		BodyDocument:      input.BodyDocument,
		HTML:              input.HTML,
		SEO:               input.SEO.toStoreInput(),
	}
}

func (input articleSaveRequest) toStoreInput() store.RevisionInput {
	return store.RevisionInput{
		BaseRevisionID:    input.BaseRevisionID,
		Title:             input.Title,
		PrimaryCategoryID: input.PrimaryCategoryID,
		TagIDs:            input.TagIDs,
		Contributors:      contributorInputs(input.Contributors),
		Deck:              input.Deck,
		Excerpt:           input.Excerpt,
		ShortAnswer:       input.ShortAnswer,
		BodyDocument:      input.BodyDocument,
		HTML:              input.HTML,
		SEO:               input.SEO.toStoreInput(),
	}
}

func contributorInputs(input []contributorRequest) []store.RevisionContributorInput {
	if input == nil {
		return nil
	}
	out := make([]store.RevisionContributorInput, 0, len(input))
	for _, contributor := range input {
		out = append(out, store.RevisionContributorInput{
			AuthorID: contributor.AuthorID,
			Role:     contributor.Role,
			Position: contributor.Position,
		})
	}
	return out
}

func (input seoRequest) toStoreInput() store.SEOInput {
	return store.SEOInput{
		Title:            input.Title,
		Description:      input.Description,
		Robots:           input.Robots,
		OpenGraphTitle:   input.OpenGraphTitle,
		OpenGraphSummary: input.OpenGraphSummary,
		OpenGraphImage:   input.OpenGraphImage,
	}
}

func (input copyArticleRequest) toStoreInput() store.CopyArticleInput {
	mappings := make([]store.CopyContributorMapping, 0, len(input.ContributorMappings))
	for _, mapping := range input.ContributorMappings {
		mappings = append(mappings, store.CopyContributorMapping{
			SourceAuthorID: mapping.SourceAuthorID, DestinationAuthorID: mapping.DestinationAuthorID,
		})
	}
	return store.CopyArticleInput{
		DestinationProjectID: input.DestinationProjectID,
		PrimaryCategoryID:    input.PrimaryCategoryID,
		Slug:                 input.Slug,
		CanonicalDecision:    input.CanonicalDecision,
		CanonicalOriginalURL: input.CanonicalOriginalURL,
		ContributorMappings:  mappings,
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
		Slug:            input.Slug,
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
		LoginUserID:      input.LoginUserID,
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
		LoginUserID:      input.LoginUserID,
		Status:           input.Status,
	}
}

func (input sourceRequest) toStoreInput() store.SourceInput {
	return store.SourceInput{
		Title:                 input.Title,
		Publisher:             input.Publisher,
		Author:                input.Author,
		URL:                   input.URL,
		PublicationDate:       input.PublicationDate,
		AccessedAt:            input.AccessedAt,
		SourceType:            input.SourceType,
		IsPrimary:             input.IsPrimary,
		ArchivedCopyReference: input.ArchivedCopyReference,
		Notes:                 input.Notes,
	}
}

func (input sourcePatchRequest) toStorePatch() store.SourcePatch {
	return store.SourcePatch{
		Title:                 input.Title,
		Publisher:             input.Publisher,
		Author:                input.Author,
		URL:                   input.URL,
		PublicationDate:       input.PublicationDate,
		AccessedAt:            input.AccessedAt,
		SourceType:            input.SourceType,
		IsPrimary:             input.IsPrimary,
		ArchivedCopyReference: input.ArchivedCopyReference,
		Notes:                 input.Notes,
	}
}

func (input disclosureRequest) toStoreInput() store.DisclosureInput {
	return store.DisclosureInput{
		RevisionID:     input.RevisionID,
		DisclosureType: input.DisclosureType,
		PublicText:     input.PublicText,
	}
}

func (input correctionRequest) toStoreInput() store.CorrectionInput {
	return store.CorrectionInput{
		AffectedRevisionID: input.AffectedRevisionID,
		PublicNote:         input.PublicNote,
		SupersedesNoticeID: input.SupersedesNoticeID,
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
