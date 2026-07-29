package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	htmlpkg "html"
	"regexp"
	"strings"

	"seoblog/apps/backend/internal/security"
)

var (
	ErrValidation      = errors.New("validation failed")
	ErrInvalidWorkflow = errors.New("invalid workflow transition")
)

type AdminRevision struct {
	ID             string `json:"id"`
	ProjectID      string `json:"projectId"`
	ArticleID      string `json:"articleId"`
	RevisionNumber int64  `json:"revisionNumber"`
	Title          string `json:"title"`
	Deck           string `json:"deck,omitempty"`
	Excerpt        string `json:"excerpt,omitempty"`
	ShortAnswer    string `json:"shortAnswer,omitempty"`
	Locale         string `json:"locale"`
	EditorialState string `json:"editorialState"`
	ContentHash    string `json:"contentHash"`
	CreatedAt      string `json:"createdAt"`
}

type AdminRevisionSummary struct {
	AdminRevision
	BaseRevisionID   string   `json:"baseRevisionId,omitempty"`
	PublishedLocales []string `json:"publishedLocales" nullable:"false"`
}

type AdminRevisionDetail struct {
	AdminRevisionSummary
	AlternateTitle      string `json:"alternateTitle,omitempty"`
	BodyDocument        any    `json:"bodyDocument"`
	TableOfContents     any    `json:"tableOfContents"`
	AuthorSnapshot      any    `json:"authorSnapshot"`
	ContributorSnapshot any    `json:"contributorSnapshot"`
	TaxonomySnapshot    any    `json:"taxonomySnapshot"`
	SourceSnapshot      any    `json:"sourceSnapshot"`
	ClaimSnapshot       any    `json:"claimSnapshot"`
	SEOSnapshot         any    `json:"seoSnapshot"`
	SocialSnapshot      any    `json:"socialSnapshot"`
	MediaSnapshot       any    `json:"mediaSnapshot"`
	DisclosureSnapshot  any    `json:"disclosureSnapshot"`
	CorrectionSummary   any    `json:"correctionSummary"`
	SanitizedHTML       string `json:"sanitizedHtml"`
	PlainText           string `json:"plainText"`
	MarkdownExport      string `json:"markdownExport"`
	WordCount           int64  `json:"wordCount"`
	ReadingTimeSeconds  int64  `json:"readingTimeSeconds"`
	ChangeSummary       string `json:"changeSummary,omitempty"`
}

type AdminArticle struct {
	ID               string         `json:"id"`
	ProjectID        string         `json:"projectId"`
	ArticleType      string         `json:"articleType"`
	Slug             string         `json:"slug"`
	Locale           string         `json:"locale"`
	Title            string         `json:"title"`
	EditorialState   string         `json:"editorialState"`
	PublicationState string         `json:"publicationState"`
	ScheduledForUTC  string         `json:"scheduledForUtc,omitempty"`
	PublishedAt      string         `json:"publishedAt,omitempty"`
	CanonicalURL     string         `json:"canonicalUrl,omitempty"`
	LatestRevision   *AdminRevision `json:"latestRevision,omitempty"`
	CreatedAt        string         `json:"createdAt"`
}

type ArticleInput struct {
	ArticleType       string
	Title             string
	Slug              string
	Locale            string
	PrimaryCategoryID string
	Deck              string
	Excerpt           string
	ShortAnswer       string
	BodyDocument      any
	HTML              string
}

type RevisionInput struct {
	BaseRevisionID    string
	Title             string
	PrimaryCategoryID string
	Deck              string
	Excerpt           string
	ShortAnswer       string
	BodyDocument      any
	HTML              string
}

type PublicationInput struct {
	RevisionID      string
	Slug            string
	Locale          string
	CanonicalURL    string
	ScheduledForUTC string
}

type RollbackInput struct {
	RevisionID string
	Locale     string
}

type TermInput struct {
	Slug        string
	Name        string
	Description string
	ParentID    string
	Indexable   bool
}

type TermPatch struct {
	Slug        *string
	Name        *string
	Description *string
	ParentID    *string
	Indexable   *bool
}

type SeriesInput struct {
	Slug        string
	Name        string
	Description string
	Indexable   bool
}

type workflowProject struct {
	ID            string
	Status        string
	PrimaryDomain string
	BlogBasePath  string
	DefaultLocale string
}

func (s *Store) ListArticlesForUser(ctx context.Context, userID, projectID, cursor string, limit int) ([]AdminArticle, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+adminArticleColumns+`
		FROM content_items item
		JOIN content_revisions revision
		  ON revision.project_id = item.project_id
		 AND revision.content_id = item.id
		 AND revision.revision_number = (
		    SELECT MAX(inner_revision.revision_number)
		    FROM content_revisions inner_revision
		    WHERE inner_revision.project_id = item.project_id
		      AND inner_revision.content_id = item.id
		 )
		LEFT JOIN project_publications publication
		  ON publication.project_id = item.project_id
		 AND publication.content_id = item.id
		 AND publication.locale = revision.locale
		WHERE item.project_id = ?
		  AND item.archived_at IS NULL
		  AND (? = '' OR item.id > ?)
		ORDER BY item.id
		LIMIT ?
	`, projectID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []AdminArticle
	for rows.Next() {
		article, err := scanAdminArticle(rows)
		if err != nil {
			return nil, err
		}
		articles = append(articles, article)
	}
	return articles, rows.Err()
}

func (s *Store) GetArticleForUser(ctx context.Context, userID, projectID, articleID string) (AdminArticle, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return AdminArticle{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT `+adminArticleColumns+`
		FROM content_items item
		JOIN content_revisions revision
		  ON revision.project_id = item.project_id
		 AND revision.content_id = item.id
		 AND revision.revision_number = (
		    SELECT MAX(inner_revision.revision_number)
		    FROM content_revisions inner_revision
		    WHERE inner_revision.project_id = item.project_id
		      AND inner_revision.content_id = item.id
		 )
		LEFT JOIN project_publications publication
		  ON publication.project_id = item.project_id
		 AND publication.content_id = item.id
		 AND publication.locale = revision.locale
		WHERE item.project_id = ?
		  AND item.id = ?
		  AND item.archived_at IS NULL
	`, projectID, articleID)
	return scanAdminArticle(row)
}

func (s *Store) CreateArticle(ctx context.Context, actorUserID, projectID string, input ArticleInput) (AdminArticle, error) {
	if err := s.requireContentWrite(ctx, actorUserID, projectID); err != nil {
		return AdminArticle{}, err
	}
	input = applyArticleDefaults(input)
	if err := validateArticleInput(input); err != nil {
		return AdminArticle{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminArticle{}, err
	}
	defer tx.Rollback()

	project, err := loadWorkflowProject(ctx, tx, projectID)
	if err != nil {
		return AdminArticle{}, err
	}
	if project.Status != "active" {
		return AdminArticle{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	if input.Locale == "" {
		input.Locale = project.DefaultLocale
	}
	category, err := loadCategory(ctx, tx, projectID, input.PrimaryCategoryID)
	if err != nil {
		return AdminArticle{}, err
	}

	articleID, err := securityRandomID("art")
	if err != nil {
		return AdminArticle{}, err
	}
	revisionID, err := securityRandomID("rev")
	if err != nil {
		return AdminArticle{}, err
	}
	publicationID, err := securityRandomID("pubn")
	if err != nil {
		return AdminArticle{}, err
	}
	bodyJSON, html, plainText, err := renderRevisionBody(input.BodyDocument, input.HTML, input.Title)
	if err != nil {
		return AdminArticle{}, err
	}
	taxonomyJSON, err := taxonomySnapshotJSON(category)
	if err != nil {
		return AdminArticle{}, err
	}
	seoJSON, err := seoSnapshotJSON(input.Title, input.Excerpt, canonicalURL(project, input.Slug))
	if err != nil {
		return AdminArticle{}, err
	}
	contentHash, err := revisionContentHash(input.Title, html, bodyJSON, taxonomyJSON, seoJSON)
	if err != nil {
		return AdminArticle{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content_items(id, project_id, article_type, created_by)
		VALUES (?, ?, ?, ?)
	`, articleID, projectID, input.ArticleType, actorUserID); err != nil {
		return AdminArticle{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO article_taxonomy(project_id, content_id, taxonomy_term_id, is_primary)
		VALUES (?, ?, ?, 1)
	`, projectID, articleID, category.ID); err != nil {
		return AdminArticle{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, created_by_type, created_by_user_id,
		  title, deck, excerpt, short_answer, body_document_json, sanitized_html, plain_text,
		  table_of_contents_json, word_count, reading_time_seconds, locale, taxonomy_snapshot_json,
		  seo_snapshot_json, content_hash, editorial_state
		) VALUES (?, ?, ?, 1, 'human', ?, ?, ?, ?, ?, ?, ?, ?, '[]', ?, ?, ?, ?, ?, ?, 'draft')
	`, revisionID, projectID, articleID, actorUserID, input.Title, nullIfEmpty(input.Deck),
		nullIfEmpty(input.Excerpt), nullIfEmpty(input.ShortAnswer), bodyJSON, html, plainText,
		wordCount(plainText), readingTimeSeconds(plainText), input.Locale, taxonomyJSON, seoJSON, contentHash); err != nil {
		return AdminArticle{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_publications(
		  id, project_id, content_id, locale, slug, canonical_url, publication_state
		) VALUES (?, ?, ?, ?, ?, ?, 'unpublished')
	`, publicationID, projectID, articleID, input.Locale, input.Slug, canonicalURL(project, input.Slug)); err != nil {
		return AdminArticle{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "content.create", "content", articleID, "success", nil); err != nil {
		return AdminArticle{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminArticle{}, err
	}
	return s.GetArticleForUser(ctx, actorUserID, projectID, articleID)
}

func (s *Store) CreateRevision(ctx context.Context, actorUserID, projectID, articleID string, input RevisionInput) (AdminRevision, error) {
	if err := s.requireContentWrite(ctx, actorUserID, projectID); err != nil {
		return AdminRevision{}, err
	}
	input.BaseRevisionID = strings.TrimSpace(input.BaseRevisionID)
	input.Title = strings.TrimSpace(input.Title)
	if input.BaseRevisionID == "" {
		return AdminRevision{}, fmt.Errorf("%w: baseRevisionId is required", ErrValidation)
	}
	if input.Title == "" {
		return AdminRevision{}, fmt.Errorf("%w: title is required", ErrValidation)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminRevision{}, err
	}
	defer tx.Rollback()

	if _, err := loadArticleType(ctx, tx, projectID, articleID); err != nil {
		return AdminRevision{}, err
	}
	categoryID := strings.TrimSpace(input.PrimaryCategoryID)
	if categoryID == "" {
		categoryID, err = loadPrimaryCategoryID(ctx, tx, projectID, articleID)
		if err != nil {
			return AdminRevision{}, err
		}
	}
	category, err := loadCategory(ctx, tx, projectID, categoryID)
	if err != nil {
		return AdminRevision{}, err
	}
	if input.PrimaryCategoryID != "" {
		if err := replacePrimaryCategory(ctx, tx, projectID, articleID, category.ID); err != nil {
			return AdminRevision{}, err
		}
	}
	nextNumber, err := nextRevisionNumber(ctx, tx, projectID, articleID)
	if err != nil {
		return AdminRevision{}, err
	}
	baseRevisionID, err := latestRevisionID(ctx, tx, projectID, articleID)
	if err != nil {
		return AdminRevision{}, err
	}
	if input.BaseRevisionID != baseRevisionID {
		return AdminRevision{}, fmt.Errorf("%w: base revision is stale; refresh the article before saving", ErrInvalidWorkflow)
	}
	revisionID, err := securityRandomID("rev")
	if err != nil {
		return AdminRevision{}, err
	}
	bodyJSON, html, plainText, err := renderRevisionBody(input.BodyDocument, input.HTML, input.Title)
	if err != nil {
		return AdminRevision{}, err
	}
	taxonomyJSON, err := taxonomySnapshotJSON(category)
	if err != nil {
		return AdminRevision{}, err
	}
	seoJSON, err := seoSnapshotJSON(input.Title, input.Excerpt, "")
	if err != nil {
		return AdminRevision{}, err
	}
	contentHash, err := revisionContentHash(input.Title, html, bodyJSON, taxonomyJSON, seoJSON)
	if err != nil {
		return AdminRevision{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, base_revision_id, created_by_type, created_by_user_id,
		  title, deck, excerpt, short_answer, body_document_json, sanitized_html, plain_text,
		  table_of_contents_json, word_count, reading_time_seconds, locale, taxonomy_snapshot_json,
		  seo_snapshot_json, content_hash, editorial_state
		) VALUES (?, ?, ?, ?, ?, 'human', ?, ?, ?, ?, ?, ?, ?, ?, '[]', ?, ?, COALESCE((
		  SELECT locale FROM project_publications WHERE project_id = ? AND content_id = ? LIMIT 1
		), 'en'), ?, ?, ?, 'draft')
	`, revisionID, projectID, articleID, nextNumber, input.BaseRevisionID, actorUserID, input.Title, nullIfEmpty(input.Deck),
		nullIfEmpty(input.Excerpt), nullIfEmpty(input.ShortAnswer), bodyJSON, html, plainText,
		wordCount(plainText), readingTimeSeconds(plainText), projectID, articleID, taxonomyJSON, seoJSON, contentHash); err != nil {
		return AdminRevision{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminRevision{}, err
	}
	return s.GetRevisionForUser(ctx, actorUserID, projectID, revisionID)
}

func (s *Store) SubmitRevision(ctx context.Context, actorUserID, projectID, revisionID string) (AdminRevision, error) {
	if err := s.requireContentWrite(ctx, actorUserID, projectID); err != nil {
		return AdminRevision{}, err
	}
	return s.setRevisionState(ctx, actorUserID, projectID, revisionID, "in_review", "revision.submit")
}

func (s *Store) RequestRevisionChanges(ctx context.Context, actorUserID, projectID, revisionID string) (AdminRevision, error) {
	if err := s.requireContentReview(ctx, actorUserID, projectID); err != nil {
		return AdminRevision{}, err
	}
	return s.setRevisionState(ctx, actorUserID, projectID, revisionID, "changes_requested", "revision.request_changes")
}

func (s *Store) ApproveRevision(ctx context.Context, actorUserID, projectID, revisionID, note string) (AdminRevision, error) {
	if err := s.requireContentReview(ctx, actorUserID, projectID); err != nil {
		return AdminRevision{}, err
	}
	revision, err := s.GetRevisionForUser(ctx, actorUserID, projectID, revisionID)
	if err != nil {
		return AdminRevision{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminRevision{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE content_revisions
		SET editorial_state = 'approved'
		WHERE project_id = ? AND id = ?
	`, projectID, revisionID); err != nil {
		return AdminRevision{}, err
	}
	decisionID, err := securityRandomID("appr")
	if err != nil {
		return AdminRevision{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO approval_decisions(
		  id, project_id, content_id, revision_id, decision, content_hash, decided_by, note
		) VALUES (?, ?, ?, ?, 'approved', ?, ?, ?)
	`, decisionID, projectID, revision.ArticleID, revisionID, revision.ContentHash, actorUserID, nullIfEmpty(note)); err != nil {
		return AdminRevision{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "revision.approve", "revision", revisionID, "success", nil); err != nil {
		return AdminRevision{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminRevision{}, err
	}
	return s.GetRevisionForUser(ctx, actorUserID, projectID, revisionID)
}

func (s *Store) ScheduleArticle(ctx context.Context, actorUserID, projectID, articleID string, input PublicationInput) (AdminArticle, error) {
	if strings.TrimSpace(input.ScheduledForUTC) == "" {
		return AdminArticle{}, fmt.Errorf("%w: scheduledForUtc is required", ErrValidation)
	}
	return s.setArticlePublication(ctx, actorUserID, projectID, articleID, input, "scheduled")
}

func (s *Store) PublishArticle(ctx context.Context, actorUserID, projectID, articleID string, input PublicationInput) (AdminArticle, error) {
	return s.setArticlePublication(ctx, actorUserID, projectID, articleID, input, "published")
}

func (s *Store) RollbackArticle(ctx context.Context, actorUserID, projectID, articleID string, input RollbackInput) (AdminArticle, error) {
	if err := s.requireContentPublish(ctx, actorUserID, projectID); err != nil {
		return AdminArticle{}, err
	}
	input.RevisionID = strings.TrimSpace(input.RevisionID)
	input.Locale = strings.TrimSpace(input.Locale)
	if input.RevisionID == "" {
		return AdminArticle{}, fmt.Errorf("%w: revisionId is required", ErrValidation)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminArticle{}, err
	}
	defer tx.Rollback()

	project, err := loadWorkflowProject(ctx, tx, projectID)
	if err != nil {
		return AdminArticle{}, err
	}
	if project.Status != "active" {
		return AdminArticle{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	if _, err := loadArticleType(ctx, tx, projectID, articleID); err != nil {
		return AdminArticle{}, err
	}
	revision, err := loadRevision(ctx, tx, projectID, articleID, input.RevisionID)
	if err != nil {
		return AdminArticle{}, err
	}
	if revision.EditorialState != "approved" {
		return AdminArticle{}, fmt.Errorf("%w: rollback revision must be approved", ErrInvalidWorkflow)
	}
	if err := ensurePublishableTaxonomy(ctx, tx, projectID, articleID); err != nil {
		return AdminArticle{}, err
	}

	locale := input.Locale
	if locale == "" {
		locale = revision.Locale
	}
	if locale == "" {
		locale = project.DefaultLocale
	}
	publication, err := loadPublicationForLocale(ctx, tx, projectID, articleID, locale)
	if errors.Is(err, sql.ErrNoRows) {
		return AdminArticle{}, fmt.Errorf("%w: article must be published before rollback", ErrInvalidWorkflow)
	}
	if err != nil {
		return AdminArticle{}, err
	}
	if publication.PublicationState != "published" {
		return AdminArticle{}, fmt.Errorf("%w: article must be published before rollback", ErrInvalidWorkflow)
	}
	if publication.PublishedRevisionID == revision.ID {
		return AdminArticle{}, fmt.Errorf("%w: revision is already published", ErrInvalidWorkflow)
	}

	publicationID, err := upsertPublication(
		ctx,
		tx,
		projectID,
		articleID,
		revision.ID,
		locale,
		publication.Slug,
		publication.CanonicalURL,
		"",
		"published",
	)
	if err != nil {
		return AdminArticle{}, err
	}
	version, err := loadPublicationVersion(ctx, tx, projectID, articleID, locale)
	if err != nil {
		return AdminArticle{}, err
	}
	if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
		return AdminArticle{}, err
	}
	if err := insertPublicationOutbox(ctx, tx, projectID, articleID, revision.ID, "content.restored", publication.CanonicalURL, version); err != nil {
		return AdminArticle{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "article.rollback", "publication", publicationID, "success", map[string]string{
		"revision_id": revision.ID,
	}); err != nil {
		return AdminArticle{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminArticle{}, err
	}
	return s.GetArticleForUser(ctx, actorUserID, projectID, articleID)
}

func (s *Store) UnpublishArticle(ctx context.Context, actorUserID, projectID, articleID string) (AdminArticle, error) {
	if err := s.requireContentPublish(ctx, actorUserID, projectID); err != nil {
		return AdminArticle{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminArticle{}, err
	}
	defer tx.Rollback()
	publication, err := loadPublication(ctx, tx, projectID, articleID)
	if err != nil {
		return AdminArticle{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE project_publications
		SET publication_state = 'unpublished',
		    scheduled_for_utc = NULL,
		    unpublished_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND content_id = ?
	`, projectID, articleID)
	if err != nil {
		return AdminArticle{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return AdminArticle{}, err
	}
	if affected == 0 {
		return AdminArticle{}, sql.ErrNoRows
	}
	if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
		return AdminArticle{}, err
	}
	if err := insertPublicationOutbox(ctx, tx, projectID, articleID, publication.PublishedRevisionID, "content.unpublished", publication.CanonicalURL, publication.PublicationVersion+1); err != nil {
		return AdminArticle{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminArticle{}, err
	}
	return s.GetArticleForUser(ctx, actorUserID, projectID, articleID)
}

func (s *Store) GetRevisionForUser(ctx context.Context, userID, projectID, revisionID string) (AdminRevision, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return AdminRevision{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, content_id, revision_number, title,
		       COALESCE(deck, ''), COALESCE(excerpt, ''), COALESCE(short_answer, ''),
		       locale, editorial_state, content_hash, created_at
		FROM content_revisions
		WHERE project_id = ? AND id = ?
	`, projectID, revisionID)
	return scanAdminRevision(row)
}

func (s *Store) ListRevisionHistoryForUser(ctx context.Context, userID, projectID, articleID, cursorID string, limit int) ([]AdminRevisionSummary, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `
		SELECT 1
		FROM content_items
		WHERE project_id = ? AND id = ? AND archived_at IS NULL
	`, projectID, articleID).Scan(&exists); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, fmt.Errorf("%w: revision history limit must be positive", ErrValidation)
	}

	var cursorRevisionNumber int64
	if cursorID != "" {
		err := s.db.QueryRowContext(ctx, `
			SELECT revision_number
			FROM content_revisions
			WHERE project_id = ? AND content_id = ? AND id = ?
		`, projectID, articleID, cursorID).Scan(&cursorRevisionNumber)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: revision history cursor is not valid for this article", ErrValidation)
		}
		if err != nil {
			return nil, err
		}
	}

	query := `
		SELECT revision.id, revision.project_id, revision.content_id, revision.revision_number,
		       revision.title, COALESCE(revision.deck, ''), COALESCE(revision.excerpt, ''),
		       COALESCE(revision.short_answer, ''), revision.locale, revision.editorial_state,
		       revision.content_hash, revision.created_at, COALESCE(revision.base_revision_id, ''),
		       COALESCE((
		         SELECT json_group_array(current_publication.locale)
		         FROM (
		           SELECT publication.locale
		           FROM project_publications publication
		           WHERE publication.project_id = revision.project_id
		             AND publication.content_id = revision.content_id
		             AND publication.published_revision_id = revision.id
		             AND publication.publication_state = 'published'
		           ORDER BY publication.locale
		         ) current_publication
		       ), '[]')
		FROM content_revisions revision
		WHERE revision.project_id = ? AND revision.content_id = ?
	`
	queryArguments := []any{projectID, articleID}
	if cursorID != "" {
		query += ` AND revision.revision_number < ?`
		queryArguments = append(queryArguments, cursorRevisionNumber)
	}
	query += ` ORDER BY revision.revision_number DESC LIMIT ?`
	queryArguments = append(queryArguments, limit)

	rows, err := s.db.QueryContext(ctx, query, queryArguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	revisions := []AdminRevisionSummary{}
	for rows.Next() {
		revision, err := scanAdminRevisionSummary(rows)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}
	return revisions, rows.Err()
}

func (s *Store) GetRevisionDetailForUser(ctx context.Context, userID, projectID, articleID, revisionID string) (AdminRevisionDetail, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return AdminRevisionDetail{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT revision.id, revision.project_id, revision.content_id, revision.revision_number,
		       revision.title, COALESCE(revision.deck, ''), COALESCE(revision.excerpt, ''),
		       COALESCE(revision.short_answer, ''), revision.locale, revision.editorial_state,
		       revision.content_hash, revision.created_at, COALESCE(revision.base_revision_id, ''),
		       COALESCE((
		         SELECT json_group_array(current_publication.locale)
		         FROM (
		           SELECT publication.locale
		           FROM project_publications publication
		           WHERE publication.project_id = revision.project_id
		             AND publication.content_id = revision.content_id
		             AND publication.published_revision_id = revision.id
		             AND publication.publication_state = 'published'
		           ORDER BY publication.locale
		         ) current_publication
		       ), '[]'),
		       COALESCE(revision.alternate_title, ''), revision.body_document_json,
		       revision.sanitized_html, revision.plain_text, revision.markdown_export,
		       revision.table_of_contents_json, revision.word_count, revision.reading_time_seconds,
		       revision.author_snapshot_json, revision.contributor_snapshot_json,
		       revision.taxonomy_snapshot_json, revision.source_snapshot_json,
		       revision.claim_snapshot_json, revision.seo_snapshot_json,
		       revision.social_snapshot_json, revision.media_snapshot_json,
		       revision.disclosure_snapshot_json, revision.correction_summary_json,
		       COALESCE(revision.change_summary, '')
		FROM content_revisions revision
		JOIN content_items item
		  ON item.project_id = revision.project_id AND item.id = revision.content_id
		WHERE revision.project_id = ? AND revision.content_id = ? AND revision.id = ?
		  AND item.archived_at IS NULL
	`, projectID, articleID, revisionID)
	return scanAdminRevisionDetail(row)
}

func (s *Store) ListAdminTerms(ctx context.Context, userID, projectID, termType string) ([]TaxonomyTerm, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	return s.ListTerms(ctx, projectID, termType)
}

func (s *Store) CreateTerm(ctx context.Context, actorUserID, projectID, termType string, input TermInput) (TaxonomyTerm, error) {
	if err := s.requireTaxonomyManage(ctx, actorUserID, projectID); err != nil {
		return TaxonomyTerm{}, err
	}
	input = applyTermDefaults(input)
	if err := validateTermInput(termType, input); err != nil {
		return TaxonomyTerm{}, err
	}
	termID, err := securityRandomID("term")
	if err != nil {
		return TaxonomyTerm{}, err
	}
	indexability := "noindex"
	if input.Indexable {
		indexability = "index"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TaxonomyTerm{}, err
	}
	defer tx.Rollback()

	status, err := projectStatus(ctx, tx, projectID)
	if err != nil {
		return TaxonomyTerm{}, err
	}
	if status != "active" {
		return TaxonomyTerm{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	if err := ensureTaxonomySlugNotReserved(ctx, tx, projectID, termType, input.Slug); err != nil {
		return TaxonomyTerm{}, err
	}
	if err := validateTermParent(ctx, tx, projectID, "", termType, input.ParentID); err != nil {
		return TaxonomyTerm{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO taxonomy_terms(id, project_id, type, parent_id, slug, name, description, indexability)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, termID, projectID, termType, nullIfEmpty(input.ParentID), input.Slug, input.Name, nullIfEmpty(input.Description), indexability); err != nil {
		return TaxonomyTerm{}, taxonomyConstraintError(err)
	}
	if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
		return TaxonomyTerm{}, err
	}
	if err := insertTermOutbox(ctx, tx, projectID, termID, "taxonomy.created", termType, input); err != nil {
		return TaxonomyTerm{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "taxonomy.create", "taxonomy_term", termID, "success", map[string]string{
		"type": termType,
		"slug": input.Slug,
	}); err != nil {
		return TaxonomyTerm{}, err
	}
	if err := tx.Commit(); err != nil {
		return TaxonomyTerm{}, err
	}
	return s.GetTerm(ctx, projectID, termType, input.Slug)
}

func (s *Store) UpdateTerm(ctx context.Context, actorUserID, projectID, termID, termType string, patch TermPatch) (TaxonomyTerm, error) {
	if err := s.requireTaxonomyManage(ctx, actorUserID, projectID); err != nil {
		return TaxonomyTerm{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TaxonomyTerm{}, err
	}
	defer tx.Rollback()

	status, err := projectStatus(ctx, tx, projectID)
	if err != nil {
		return TaxonomyTerm{}, err
	}
	if status != "active" {
		return TaxonomyTerm{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	current, err := queryTermByID(ctx, tx, projectID, termID, termType)
	if err != nil {
		return TaxonomyTerm{}, err
	}
	next := termInputFromTerm(current)
	if patch.Slug != nil {
		next.Slug = *patch.Slug
	}
	if patch.Name != nil {
		next.Name = *patch.Name
	}
	if patch.Description != nil {
		next.Description = *patch.Description
	}
	if patch.ParentID != nil {
		next.ParentID = *patch.ParentID
	}
	if patch.Indexable != nil {
		next.Indexable = *patch.Indexable
	}
	next = applyTermDefaults(next)
	if err := validateTermInput(termType, next); err != nil {
		return TaxonomyTerm{}, err
	}
	if err := validateTermParent(ctx, tx, projectID, termID, termType, next.ParentID); err != nil {
		return TaxonomyTerm{}, err
	}

	assignments := make([]string, 0, 5)
	args := make([]any, 0, 8)
	if patch.ParentID != nil && next.ParentID != current.ParentID {
		assignments = append(assignments, "parent_id = ?")
		args = append(args, nullIfEmpty(next.ParentID))
	}
	if patch.Slug != nil && next.Slug != current.Slug {
		if err := ensureTaxonomySlugNotReserved(ctx, tx, projectID, termType, next.Slug); err != nil {
			return TaxonomyTerm{}, err
		}
		assignments = append(assignments, "slug = ?")
		args = append(args, next.Slug)
	}
	if patch.Name != nil && next.Name != current.Name {
		assignments = append(assignments, "name = ?")
		args = append(args, next.Name)
	}
	if patch.Description != nil && next.Description != current.Description {
		assignments = append(assignments, "description = ?")
		args = append(args, nullIfEmpty(next.Description))
	}
	if patch.Indexable != nil && next.Indexable != current.Indexable {
		indexability := "noindex"
		if next.Indexable {
			indexability = "index"
		}
		assignments = append(assignments, "indexability = ?")
		args = append(args, indexability)
	}
	if len(assignments) == 0 {
		if err := tx.Commit(); err != nil {
			return TaxonomyTerm{}, err
		}
		return s.getHydratedTermByID(ctx, projectID, termID, termType)
	}

	args = append(args, projectID, termID, termType)
	result, err := tx.ExecContext(ctx, `
		UPDATE taxonomy_terms
		SET `+strings.Join(assignments, ", ")+`,
		    updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ? AND type = ? AND status = 'active'
	`, args...)
	if err != nil {
		return TaxonomyTerm{}, taxonomyConstraintError(err)
	}
	if changed, err := result.RowsAffected(); err != nil {
		return TaxonomyTerm{}, err
	} else if changed != 1 {
		return TaxonomyTerm{}, sql.ErrNoRows
	}
	stored, err := queryTermByID(ctx, tx, projectID, termID, termType)
	if err != nil {
		return TaxonomyTerm{}, err
	}
	storedInput := termInputFromTerm(stored)
	if current.Slug != stored.Slug {
		if err := insertTaxonomySlugRedirect(ctx, tx, projectID, termType, current.Slug, stored.Slug); err != nil {
			return TaxonomyTerm{}, err
		}
	}
	if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
		return TaxonomyTerm{}, err
	}
	if err := insertTermOutbox(ctx, tx, projectID, termID, "taxonomy.updated", termType, storedInput); err != nil {
		return TaxonomyTerm{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "taxonomy.update", "taxonomy_term", termID, "success", map[string]string{
		"type": termType,
		"slug": stored.Slug,
	}); err != nil {
		return TaxonomyTerm{}, err
	}
	if err := tx.Commit(); err != nil {
		return TaxonomyTerm{}, err
	}
	return s.getHydratedTermByID(ctx, projectID, termID, termType)
}

func (s *Store) ListAdminSeries(ctx context.Context, userID, projectID string) ([]Series, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	return s.ListSeries(ctx, projectID)
}

func (s *Store) CreateSeries(ctx context.Context, actorUserID, projectID string, input SeriesInput) (Series, error) {
	if err := s.requireTaxonomyManage(ctx, actorUserID, projectID); err != nil {
		return Series{}, err
	}
	input = applySeriesDefaults(input)
	if err := validateSeriesInput(input); err != nil {
		return Series{}, err
	}
	seriesID, err := securityRandomID("ser")
	if err != nil {
		return Series{}, err
	}
	indexability := "noindex"
	if input.Indexable {
		indexability = "index"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Series{}, err
	}
	defer tx.Rollback()

	status, err := projectStatus(ctx, tx, projectID)
	if err != nil {
		return Series{}, err
	}
	if status != "active" {
		return Series{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO series(id, project_id, slug, name, description, indexability)
		VALUES (?, ?, ?, ?, ?, ?)
	`, seriesID, projectID, input.Slug, input.Name, nullIfEmpty(input.Description), indexability); err != nil {
		return Series{}, seriesConstraintError(err)
	}
	if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
		return Series{}, err
	}
	if err := insertSeriesOutbox(ctx, tx, projectID, seriesID, "series.created", input); err != nil {
		return Series{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "series.create", "series", seriesID, "success", map[string]string{
		"slug": input.Slug,
	}); err != nil {
		return Series{}, err
	}
	if err := tx.Commit(); err != nil {
		return Series{}, err
	}
	return s.getSeriesByID(ctx, projectID, seriesID)
}

func (s *Store) setArticlePublication(ctx context.Context, actorUserID, projectID, articleID string, input PublicationInput, state string) (AdminArticle, error) {
	if err := s.requireContentPublish(ctx, actorUserID, projectID); err != nil {
		return AdminArticle{}, err
	}
	input.Slug = slugify(input.Slug)
	if input.Slug == "" {
		return AdminArticle{}, fmt.Errorf("%w: slug is required", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminArticle{}, err
	}
	defer tx.Rollback()

	project, err := loadWorkflowProject(ctx, tx, projectID)
	if err != nil {
		return AdminArticle{}, err
	}
	if project.Status != "active" {
		return AdminArticle{}, fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	articleType, err := loadArticleType(ctx, tx, projectID, articleID)
	if err != nil {
		return AdminArticle{}, err
	}
	_ = articleType
	revision, err := loadRevision(ctx, tx, projectID, articleID, input.RevisionID)
	if err != nil {
		return AdminArticle{}, err
	}
	if revision.EditorialState != "approved" {
		return AdminArticle{}, fmt.Errorf("%w: revision must be approved before publication", ErrInvalidWorkflow)
	}
	if err := ensurePublishableTaxonomy(ctx, tx, projectID, articleID); err != nil {
		return AdminArticle{}, err
	}
	canonical := strings.TrimSpace(input.CanonicalURL)
	if canonical == "" {
		canonical = canonicalURL(project, input.Slug)
	}
	publicationID, err := upsertPublication(ctx, tx, projectID, articleID, revision.ID, input.Locale, input.Slug, canonical, input.ScheduledForUTC, state)
	if err != nil {
		return AdminArticle{}, err
	}
	if state == "published" {
		version, err := loadPublicationVersion(ctx, tx, projectID, articleID, input.Locale)
		if err != nil {
			return AdminArticle{}, err
		}
		if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
			return AdminArticle{}, err
		}
		if err := insertPublicationOutbox(ctx, tx, projectID, articleID, revision.ID, "content.published", canonical, version); err != nil {
			return AdminArticle{}, err
		}
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "article."+state, "publication", publicationID, "success", nil); err != nil {
		return AdminArticle{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminArticle{}, err
	}
	return s.GetArticleForUser(ctx, actorUserID, projectID, articleID)
}

func (s *Store) setRevisionState(ctx context.Context, actorUserID, projectID, revisionID, state, action string) (AdminRevision, error) {
	revision, err := s.GetRevisionForUser(ctx, actorUserID, projectID, revisionID)
	if err != nil {
		return AdminRevision{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminRevision{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE content_revisions
		SET editorial_state = ?
		WHERE project_id = ? AND id = ?
	`, state, projectID, revisionID)
	if err != nil {
		return AdminRevision{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return AdminRevision{}, err
	} else if changed != 1 {
		return AdminRevision{}, sql.ErrNoRows
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, action, "revision", revision.ID, "success", nil); err != nil {
		return AdminRevision{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminRevision{}, err
	}
	return s.GetRevisionForUser(ctx, actorUserID, projectID, revisionID)
}

func (s *Store) requireContentWrite(ctx context.Context, userID, projectID string) error {
	role, err := s.projectRole(ctx, userID, projectID)
	if err != nil {
		return err
	}
	if role == "project_owner" || role == "project_admin" || role == "editor" || role == "writer" {
		return nil
	}
	return ErrForbidden
}

func (s *Store) requireContentReview(ctx context.Context, userID, projectID string) error {
	role, err := s.projectRole(ctx, userID, projectID)
	if err != nil {
		return err
	}
	if role == "project_owner" || role == "project_admin" || role == "editor" || role == "reviewer" {
		return nil
	}
	return ErrForbidden
}

func (s *Store) requireContentPublish(ctx context.Context, userID, projectID string) error {
	role, err := s.projectRole(ctx, userID, projectID)
	if err != nil {
		return err
	}
	if role == "project_owner" || role == "project_admin" || role == "editor" {
		return nil
	}
	return ErrForbidden
}

func (s *Store) requireTaxonomyManage(ctx context.Context, userID, projectID string) error {
	role, err := s.projectRole(ctx, userID, projectID)
	if err != nil {
		return err
	}
	if role == "project_owner" || role == "project_admin" || role == "editor" {
		return nil
	}
	return ErrForbidden
}

func (s *Store) getTermByID(ctx context.Context, projectID, termID, termType string) (TaxonomyTerm, error) {
	return queryTermByID(ctx, s.db, projectID, termID, termType)
}

type termQueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func queryTermByID(ctx context.Context, queryer termQueryRower, projectID, termID, termType string) (TaxonomyTerm, error) {
	row := queryer.QueryRowContext(ctx, `
		SELECT id, type, slug, name, COALESCE(description, ''), COALESCE(parent_id, ''), indexability
		FROM taxonomy_terms
		WHERE project_id = ? AND id = ? AND type = ? AND status = 'active'
	`, projectID, termID, termType)
	return scanTerm(row)
}

func (s *Store) getHydratedTermByID(ctx context.Context, projectID, termID, termType string) (TaxonomyTerm, error) {
	terms, err := s.ListTerms(ctx, projectID, termType)
	if err != nil {
		return TaxonomyTerm{}, err
	}
	for _, term := range terms {
		if term.ID == termID {
			return term, nil
		}
	}
	return TaxonomyTerm{}, sql.ErrNoRows
}

func (s *Store) getSeriesByID(ctx context.Context, projectID, seriesID string) (Series, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, COALESCE(description, ''), indexability
		FROM series
		WHERE project_id = ? AND id = ?
	`, projectID, seriesID)
	return scanSeries(row)
}

const adminArticleColumns = `
	item.id, item.project_id, item.article_type,
	COALESCE(publication.slug, ''), COALESCE(publication.locale, revision.locale),
	revision.title, revision.editorial_state,
	COALESCE(publication.publication_state, 'unpublished'),
	COALESCE(publication.scheduled_for_utc, ''),
	COALESCE(publication.first_published_at, ''),
	COALESCE(publication.canonical_url, ''),
	revision.id, revision.revision_number, revision.title,
	COALESCE(revision.deck, ''), COALESCE(revision.excerpt, ''),
	COALESCE(revision.short_answer, ''), revision.locale,
	revision.editorial_state, revision.content_hash, revision.created_at,
	item.created_at
`

func scanAdminArticle(row rowScanner) (AdminArticle, error) {
	var article AdminArticle
	revision := AdminRevision{}
	err := row.Scan(
		&article.ID,
		&article.ProjectID,
		&article.ArticleType,
		&article.Slug,
		&article.Locale,
		&article.Title,
		&article.EditorialState,
		&article.PublicationState,
		&article.ScheduledForUTC,
		&article.PublishedAt,
		&article.CanonicalURL,
		&revision.ID,
		&revision.RevisionNumber,
		&revision.Title,
		&revision.Deck,
		&revision.Excerpt,
		&revision.ShortAnswer,
		&revision.Locale,
		&revision.EditorialState,
		&revision.ContentHash,
		&revision.CreatedAt,
		&article.CreatedAt,
	)
	if err != nil {
		return AdminArticle{}, err
	}
	revision.ProjectID = article.ProjectID
	revision.ArticleID = article.ID
	article.LatestRevision = &revision
	return article, nil
}

func scanAdminRevision(row rowScanner) (AdminRevision, error) {
	var revision AdminRevision
	err := row.Scan(
		&revision.ID,
		&revision.ProjectID,
		&revision.ArticleID,
		&revision.RevisionNumber,
		&revision.Title,
		&revision.Deck,
		&revision.Excerpt,
		&revision.ShortAnswer,
		&revision.Locale,
		&revision.EditorialState,
		&revision.ContentHash,
		&revision.CreatedAt,
	)
	return revision, err
}

func scanAdminRevisionSummary(row rowScanner) (AdminRevisionSummary, error) {
	var revision AdminRevisionSummary
	var publishedLocalesJSON string
	err := row.Scan(
		&revision.ID,
		&revision.ProjectID,
		&revision.ArticleID,
		&revision.RevisionNumber,
		&revision.Title,
		&revision.Deck,
		&revision.Excerpt,
		&revision.ShortAnswer,
		&revision.Locale,
		&revision.EditorialState,
		&revision.ContentHash,
		&revision.CreatedAt,
		&revision.BaseRevisionID,
		&publishedLocalesJSON,
	)
	if err != nil {
		return AdminRevisionSummary{}, err
	}
	revision.PublishedLocales = []string{}
	decodeInto(publishedLocalesJSON, &revision.PublishedLocales)
	return revision, nil
}

func scanAdminRevisionDetail(row rowScanner) (AdminRevisionDetail, error) {
	var revision AdminRevisionDetail
	var publishedLocalesJSON string
	var bodyDocumentJSON, tableOfContentsJSON string
	var authorJSON, contributorJSON, taxonomyJSON, sourceJSON, claimJSON string
	var seoJSON, socialJSON, mediaJSON, disclosureJSON, correctionJSON string
	err := row.Scan(
		&revision.ID,
		&revision.ProjectID,
		&revision.ArticleID,
		&revision.RevisionNumber,
		&revision.Title,
		&revision.Deck,
		&revision.Excerpt,
		&revision.ShortAnswer,
		&revision.Locale,
		&revision.EditorialState,
		&revision.ContentHash,
		&revision.CreatedAt,
		&revision.BaseRevisionID,
		&publishedLocalesJSON,
		&revision.AlternateTitle,
		&bodyDocumentJSON,
		&revision.SanitizedHTML,
		&revision.PlainText,
		&revision.MarkdownExport,
		&tableOfContentsJSON,
		&revision.WordCount,
		&revision.ReadingTimeSeconds,
		&authorJSON,
		&contributorJSON,
		&taxonomyJSON,
		&sourceJSON,
		&claimJSON,
		&seoJSON,
		&socialJSON,
		&mediaJSON,
		&disclosureJSON,
		&correctionJSON,
		&revision.ChangeSummary,
	)
	if err != nil {
		return AdminRevisionDetail{}, err
	}
	revision.PublishedLocales = []string{}
	decodeInto(publishedLocalesJSON, &revision.PublishedLocales)
	revision.BodyDocument = decodeJSON(bodyDocumentJSON, map[string]any{})
	revision.TableOfContents = decodeJSON(tableOfContentsJSON, []any{})
	revision.AuthorSnapshot = decodeJSON(authorJSON, []any{})
	revision.ContributorSnapshot = decodeJSON(contributorJSON, []any{})
	revision.TaxonomySnapshot = decodeJSON(taxonomyJSON, map[string]any{})
	revision.SourceSnapshot = decodeJSON(sourceJSON, []any{})
	revision.ClaimSnapshot = decodeJSON(claimJSON, []any{})
	revision.SEOSnapshot = decodeJSON(seoJSON, map[string]any{})
	revision.SocialSnapshot = decodeJSON(socialJSON, map[string]any{})
	revision.MediaSnapshot = decodeJSON(mediaJSON, map[string]any{})
	revision.DisclosureSnapshot = decodeJSON(disclosureJSON, []any{})
	revision.CorrectionSummary = decodeJSON(correctionJSON, []any{})
	return revision, nil
}

type publicationRecord struct {
	ID                  string
	PublishedRevisionID string
	Locale              string
	Slug                string
	CanonicalURL        string
	PublicationState    string
	PublicationVersion  int64
}

func loadPublication(ctx context.Context, tx *sql.Tx, projectID, articleID string) (publicationRecord, error) {
	return loadPublicationForLocale(ctx, tx, projectID, articleID, "")
}

func loadPublicationForLocale(ctx context.Context, tx *sql.Tx, projectID, articleID, locale string) (publicationRecord, error) {
	var publication publicationRecord
	err := tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(published_revision_id, ''), locale, slug, canonical_url,
		       publication_state, publication_version
		FROM project_publications
		WHERE project_id = ?
		  AND content_id = ?
		  AND (? = '' OR locale = ?)
		ORDER BY updated_at DESC, id DESC
		LIMIT 1
	`, projectID, articleID, locale, locale).Scan(
		&publication.ID,
		&publication.PublishedRevisionID,
		&publication.Locale,
		&publication.Slug,
		&publication.CanonicalURL,
		&publication.PublicationState,
		&publication.PublicationVersion,
	)
	return publication, err
}

func loadWorkflowProject(ctx context.Context, tx *sql.Tx, projectID string) (workflowProject, error) {
	var project workflowProject
	err := tx.QueryRowContext(ctx, `
		SELECT id, status, COALESCE(primary_domain, ''), blog_base_path, default_locale
		FROM projects
		WHERE id = ?
	`, projectID).Scan(&project.ID, &project.Status, &project.PrimaryDomain, &project.BlogBasePath, &project.DefaultLocale)
	return project, err
}

func loadArticleType(ctx context.Context, tx *sql.Tx, projectID, articleID string) (string, error) {
	var articleType string
	err := tx.QueryRowContext(ctx, `
		SELECT article_type
		FROM content_items
		WHERE project_id = ? AND id = ? AND archived_at IS NULL
	`, projectID, articleID).Scan(&articleType)
	return articleType, err
}

func loadRevision(ctx context.Context, tx *sql.Tx, projectID, articleID, revisionID string) (AdminRevision, error) {
	if strings.TrimSpace(revisionID) == "" {
		revisionID = latestApprovedRevisionID(ctx, tx, projectID, articleID)
	}
	row := tx.QueryRowContext(ctx, `
		SELECT id, project_id, content_id, revision_number, title,
		       COALESCE(deck, ''), COALESCE(excerpt, ''), COALESCE(short_answer, ''),
		       locale, editorial_state, content_hash, created_at
		FROM content_revisions
		WHERE project_id = ? AND content_id = ? AND id = ?
	`, projectID, articleID, revisionID)
	return scanAdminRevision(row)
}

func latestApprovedRevisionID(ctx context.Context, tx *sql.Tx, projectID, articleID string) string {
	var revisionID string
	_ = tx.QueryRowContext(ctx, `
		SELECT id
		FROM content_revisions
		WHERE project_id = ? AND content_id = ? AND editorial_state = 'approved'
		ORDER BY revision_number DESC
		LIMIT 1
	`, projectID, articleID).Scan(&revisionID)
	return revisionID
}

func loadCategory(ctx context.Context, tx *sql.Tx, projectID, categoryID string) (TaxonomyTerm, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, type, slug, name, COALESCE(description, ''), COALESCE(parent_id, ''), indexability
		FROM taxonomy_terms
		WHERE project_id = ? AND id = ? AND type = 'category' AND status = 'active'
	`, projectID, categoryID)
	return scanTerm(row)
}

func loadPrimaryCategoryID(ctx context.Context, tx *sql.Tx, projectID, articleID string) (string, error) {
	var categoryID string
	err := tx.QueryRowContext(ctx, `
		SELECT taxonomy_term_id
		FROM article_taxonomy
		WHERE project_id = ? AND content_id = ? AND is_primary = 1
	`, projectID, articleID).Scan(&categoryID)
	return categoryID, err
}

func replacePrimaryCategory(ctx context.Context, tx *sql.Tx, projectID, articleID, categoryID string) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM article_taxonomy
		WHERE project_id = ? AND content_id = ? AND is_primary = 1
	`, projectID, articleID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO article_taxonomy(project_id, content_id, taxonomy_term_id, is_primary)
		VALUES (?, ?, ?, 1)
	`, projectID, articleID, categoryID)
	return err
}

func nextRevisionNumber(ctx context.Context, tx *sql.Tx, projectID, articleID string) (int64, error) {
	var current int64
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(revision_number), 0)
		FROM content_revisions
		WHERE project_id = ? AND content_id = ?
	`, projectID, articleID).Scan(&current)
	return current + 1, err
}

func latestRevisionID(ctx context.Context, tx *sql.Tx, projectID, articleID string) (string, error) {
	var revisionID string
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM content_revisions
		WHERE project_id = ? AND content_id = ?
		ORDER BY revision_number DESC
		LIMIT 1
	`, projectID, articleID).Scan(&revisionID)
	return revisionID, err
}

func ensurePublishableTaxonomy(ctx context.Context, tx *sql.Tx, projectID, articleID string) error {
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM article_taxonomy assignment
		JOIN taxonomy_terms term
		  ON term.project_id = assignment.project_id
		 AND term.id = assignment.taxonomy_term_id
		WHERE assignment.project_id = ?
		  AND assignment.content_id = ?
		  AND assignment.is_primary = 1
		  AND term.type = 'category'
	`, projectID, articleID).Scan(&count)
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("%w: article requires exactly one primary category", ErrInvalidWorkflow)
	}
	return nil
}

func upsertPublication(ctx context.Context, tx *sql.Tx, projectID, articleID, revisionID, locale, slug, canonicalURL, scheduledForUTC, state string) (string, error) {
	if locale == "" {
		var defaultLocale string
		if err := tx.QueryRowContext(ctx, `SELECT default_locale FROM projects WHERE id = ?`, projectID).Scan(&defaultLocale); err != nil {
			return "", err
		}
		locale = defaultLocale
	}
	publicationID, err := securityRandomID("pubn")
	if err != nil {
		return "", err
	}
	if state == "scheduled" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO project_publications(
			  id, project_id, content_id, locale, slug, canonical_url,
			  published_revision_id, publication_state, scheduled_for_utc
			) VALUES (?, ?, ?, ?, ?, ?, ?, 'scheduled', ?)
			ON CONFLICT(project_id, content_id, locale) DO UPDATE SET
			  slug = excluded.slug,
			  canonical_url = excluded.canonical_url,
			  published_revision_id = excluded.published_revision_id,
			  publication_state = 'scheduled',
			  scheduled_for_utc = excluded.scheduled_for_utc,
			  updated_at = CURRENT_TIMESTAMP
		`, publicationID, projectID, articleID, locale, slug, canonicalURL, revisionID, scheduledForUTC)
	} else {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO project_publications(
			  id, project_id, content_id, locale, slug, canonical_url,
			  published_revision_id, publication_state, first_published_at,
			  materially_modified_at, publication_version
			) VALUES (?, ?, ?, ?, ?, ?, ?, 'published', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)
			ON CONFLICT(project_id, content_id, locale) DO UPDATE SET
			  slug = excluded.slug,
			  canonical_url = excluded.canonical_url,
			  published_revision_id = excluded.published_revision_id,
			  publication_state = 'published',
			  scheduled_for_utc = NULL,
			  first_published_at = COALESCE(project_publications.first_published_at, CURRENT_TIMESTAMP),
			  materially_modified_at = CURRENT_TIMESTAMP,
			  publication_version = project_publications.publication_version + 1,
			  updated_at = CURRENT_TIMESTAMP
		`, publicationID, projectID, articleID, locale, slug, canonicalURL, revisionID)
	}
	if err != nil {
		return "", err
	}
	var storedID string
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM project_publications
		WHERE project_id = ? AND content_id = ? AND locale = ?
	`, projectID, articleID, locale).Scan(&storedID)
	return storedID, err
}

func loadPublicationVersion(ctx context.Context, tx *sql.Tx, projectID, articleID, locale string) (int64, error) {
	var version int64
	err := tx.QueryRowContext(ctx, `
		SELECT publication_version
		FROM project_publications
		WHERE project_id = ? AND content_id = ? AND (? = '' OR locale = ?)
		ORDER BY updated_at DESC
		LIMIT 1
	`, projectID, articleID, locale, locale).Scan(&version)
	return version, err
}

func incrementProjectGeneration(ctx context.Context, tx *sql.Tx, projectID string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE projects
		SET content_generation = content_generation + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, projectID)
	return err
}

func insertPublicationOutbox(ctx context.Context, tx *sql.Tx, projectID, articleID, revisionID, eventType, canonicalURL string, version int64) error {
	payload, _ := json.Marshal(map[string]any{
		"project_id":          projectID,
		"content_id":          articleID,
		"revision_id":         revisionID,
		"publication_version": version,
		"canonical_url":       canonicalURL,
	})
	eventID, err := securityRandomID("event")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox_events(
		  id, project_id, event_type, aggregate_type, aggregate_id,
		  payload_json, idempotency_key
		) VALUES (?, ?, ?, 'content', ?, ?, ?)
	`, eventID, projectID, eventType, articleID, string(payload), fmt.Sprintf("%s:%s:%s:%d", eventType, articleID, revisionID, version))
	return err
}

func insertTermOutbox(ctx context.Context, tx *sql.Tx, projectID, termID, eventType, termType string, input TermInput) error {
	payload, err := json.Marshal(map[string]any{
		"project_id":  projectID,
		"term_id":     termID,
		"type":        termType,
		"slug":        input.Slug,
		"parent_id":   input.ParentID,
		"indexable":   input.Indexable,
		"description": input.Description,
	})
	if err != nil {
		return err
	}
	eventID, err := securityRandomID("event")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox_events(
		  id, project_id, event_type, aggregate_type, aggregate_id,
		  payload_json, idempotency_key
		) VALUES (?, ?, ?, 'taxonomy_term', ?, ?, ?)
	`, eventID, projectID, eventType, termID, string(payload), fmt.Sprintf("%s:%s:%s", eventType, termID, eventID))
	return err
}

func insertSeriesOutbox(ctx context.Context, tx *sql.Tx, projectID, seriesID, eventType string, input SeriesInput) error {
	payload, err := json.Marshal(map[string]any{
		"project_id":  projectID,
		"series_id":   seriesID,
		"slug":        input.Slug,
		"indexable":   input.Indexable,
		"description": input.Description,
	})
	if err != nil {
		return err
	}
	eventID, err := securityRandomID("event")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox_events(
		  id, project_id, event_type, aggregate_type, aggregate_id,
		  payload_json, idempotency_key
		) VALUES (?, ?, ?, 'series', ?, ?, ?)
	`, eventID, projectID, eventType, seriesID, string(payload), fmt.Sprintf("%s:%s:%s", eventType, seriesID, eventID))
	return err
}

func applyArticleDefaults(input ArticleInput) ArticleInput {
	input.ArticleType = strings.TrimSpace(input.ArticleType)
	if input.ArticleType == "" {
		input.ArticleType = "standard"
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Slug = slugify(input.Slug)
	if input.Slug == "" {
		input.Slug = slugify(input.Title)
	}
	input.Locale = strings.TrimSpace(input.Locale)
	input.PrimaryCategoryID = strings.TrimSpace(input.PrimaryCategoryID)
	input.Deck = strings.TrimSpace(input.Deck)
	input.Excerpt = strings.TrimSpace(input.Excerpt)
	input.ShortAnswer = strings.TrimSpace(input.ShortAnswer)
	input.HTML = strings.TrimSpace(input.HTML)
	return input
}

func validateArticleInput(input ArticleInput) error {
	if input.Title == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	if input.Slug == "" {
		return fmt.Errorf("%w: slug is required", ErrValidation)
	}
	if input.PrimaryCategoryID == "" {
		return fmt.Errorf("%w: primaryCategoryId is required", ErrValidation)
	}
	switch input.ArticleType {
	case "standard", "guide", "tutorial", "comparison", "case_study", "research", "listicle", "news_update", "opinion", "reference", "glossary", "release_note":
		return nil
	default:
		return fmt.Errorf("%w: unsupported article type", ErrValidation)
	}
}

func applyTermDefaults(input TermInput) TermInput {
	input.Slug = slugify(input.Slug)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.ParentID = strings.TrimSpace(input.ParentID)
	return input
}

func termInputFromTerm(term TaxonomyTerm) TermInput {
	return TermInput{
		Slug:        term.Slug,
		Name:        term.Name,
		Description: term.Description,
		ParentID:    term.ParentID,
		Indexable:   term.Indexable,
	}
}

func validateTermInput(termType string, input TermInput) error {
	if input.Slug == "" {
		return fmt.Errorf("%w: taxonomy slug is required", ErrValidation)
	}
	if input.Name == "" {
		return fmt.Errorf("%w: taxonomy name is required", ErrValidation)
	}
	if termType != "category" && termType != "tag" {
		return fmt.Errorf("%w: unsupported taxonomy type", ErrValidation)
	}
	if termType != "category" && input.ParentID != "" {
		return fmt.Errorf("%w: tags cannot have parents", ErrValidation)
	}
	return nil
}

func validateTermParent(ctx context.Context, tx *sql.Tx, projectID, termID, termType, parentID string) error {
	if parentID == "" {
		return nil
	}
	if termType != "category" {
		return fmt.Errorf("%w: tags cannot have parents", ErrValidation)
	}
	if termID != "" && parentID == termID {
		return fmt.Errorf("%w: category cannot parent itself", ErrValidation)
	}
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM taxonomy_terms
		WHERE project_id = ?
		  AND id = ?
		  AND type = 'category'
		  AND status = 'active'
	`, projectID, parentID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: parentId must reference an active category in this project", ErrValidation)
	}
	return err
}

func ensureTaxonomySlugNotReserved(ctx context.Context, tx *sql.Tx, projectID, termType, slug string) error {
	sourcePath, err := taxonomyArchivePath(termType, slug)
	if err != nil {
		return err
	}
	var exists int
	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM slug_redirects
		WHERE project_id = ? AND source_path = ?
	`, projectID, sourcePath).Scan(&exists)
	switch {
	case err == nil:
		return fmt.Errorf("%w: taxonomy slug is reserved by redirect history", ErrValidation)
	case errors.Is(err, sql.ErrNoRows):
		return nil
	default:
		return err
	}
}

func insertTaxonomySlugRedirect(ctx context.Context, tx *sql.Tx, projectID, termType, oldSlug, newSlug string) error {
	sourcePath, err := taxonomyArchivePath(termType, oldSlug)
	if err != nil {
		return err
	}
	targetPath, err := taxonomyArchivePath(termType, newSlug)
	if err != nil {
		return err
	}
	if sourcePath == targetPath {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE slug_redirects
		SET target_path = ?
		WHERE project_id = ? AND target_path = ?
	`, targetPath, projectID, sourcePath); err != nil {
		return err
	}
	redirectID, err := securityRandomID("redirect")
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO slug_redirects(id, project_id, source_path, target_path, status_code)
		VALUES (?, ?, ?, ?, 301)
	`, redirectID, projectID, sourcePath, targetPath); err != nil {
		return taxonomyConstraintError(err)
	}
	return nil
}

func taxonomyArchivePath(termType, slug string) (string, error) {
	switch termType {
	case "category":
		return "/categories/" + slug, nil
	case "tag":
		return "/tags/" + slug, nil
	default:
		return "", fmt.Errorf("%w: unsupported taxonomy type", ErrValidation)
	}
}

func taxonomyConstraintError(err error) error {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "taxonomy_terms.project_id, taxonomy_terms.slug") ||
		strings.Contains(message, "slug_redirects.project_id, slug_redirects.source_path") ||
		strings.Contains(message, "category cannot parent itself") ||
		strings.Contains(message, "only categories may have category parents") ||
		strings.Contains(message, "category hierarchy cannot contain a cycle") ||
		strings.Contains(message, "category hierarchy cannot exceed three levels") ||
		strings.Contains(message, "a taxonomy term with children must remain a category") {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return err
}

func applySeriesDefaults(input SeriesInput) SeriesInput {
	input.Slug = slugify(input.Slug)
	input.Name = strings.TrimSpace(input.Name)
	if input.Slug == "" {
		input.Slug = slugify(input.Name)
	}
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func validateSeriesInput(input SeriesInput) error {
	if input.Slug == "" {
		return fmt.Errorf("%w: series slug is required", ErrValidation)
	}
	if input.Name == "" {
		return fmt.Errorf("%w: series name is required", ErrValidation)
	}
	return nil
}

func seriesConstraintError(err error) error {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "series.project_id, series.slug") {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return err
}

func renderRevisionBody(document any, html, title string) (string, string, string, error) {
	if document == nil {
		document = map[string]any{
			"type":    "doc",
			"content": []any{},
		}
	}
	bodyJSONBytes, err := json.Marshal(document)
	if err != nil {
		return "", "", "", err
	}
	if strings.TrimSpace(html) == "" {
		html = "<p>" + htmlpkg.EscapeString(title) + "</p>"
	}
	plainText := strings.TrimSpace(stripTags(html))
	return string(bodyJSONBytes), html, plainText, nil
}

func taxonomySnapshotJSON(primary TaxonomyTerm) (string, error) {
	raw, err := json.Marshal(PublishedTaxonomy{
		PrimaryCategory: &primary,
		Categories:      []TaxonomyTerm{primary},
		Tags:            []TaxonomyTerm{},
		Topics:          []TaxonomyTerm{},
	})
	return string(raw), err
}

func seoSnapshotJSON(title, description, canonicalURL string) (string, error) {
	raw, err := json.Marshal(map[string]any{
		"title":          title,
		"description":    description,
		"canonicalUrl":   canonicalURL,
		"openGraph":      map[string]any{},
		"structuredData": []any{},
		"hreflang":       []any{},
	})
	return string(raw), err
}

func revisionContentHash(title, html, bodyJSON, taxonomyJSON, seoJSON string) (string, error) {
	raw, err := json.Marshal(map[string]string{
		"title":    title,
		"html":     html,
		"body":     bodyJSON,
		"taxonomy": taxonomyJSON,
		"seo":      seoJSON,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

var tagPattern = regexp.MustCompile(`<[^>]+>`)

func stripTags(value string) string {
	return tagPattern.ReplaceAllString(value, " ")
}

func wordCount(value string) int {
	return len(strings.Fields(value))
}

func readingTimeSeconds(value string) int {
	words := wordCount(value)
	if words == 0 {
		return 0
	}
	seconds := (words * 60) / 225
	if seconds < 1 {
		return 1
	}
	return seconds
}

func canonicalURL(project workflowProject, slug string) string {
	domain := strings.TrimSpace(project.PrimaryDomain)
	if domain == "" {
		domain = project.ID + ".invalid"
	}
	if !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
		domain = "https://" + domain
	}
	basePath := strings.TrimRight(project.BlogBasePath, "/")
	if basePath == "" {
		basePath = "/blog"
	}
	return strings.TrimRight(domain, "/") + basePath + "/" + slug
}

func securityRandomID(prefix string) (string, error) {
	return security.RandomID(prefix)
}
