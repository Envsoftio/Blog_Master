package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"seoblog/apps/backend/internal/security"
)

var (
	ErrValidation      = errors.New("validation failed")
	ErrInvalidWorkflow = errors.New("invalid workflow transition")
)

var projectScopedBodyReferenceKeys = map[string]struct{}{
	"articleid":         {},
	"articleids":        {},
	"assetid":           {},
	"assetids":          {},
	"authorid":          {},
	"authorids":         {},
	"categoryid":        {},
	"categoryids":       {},
	"claimid":           {},
	"claimids":          {},
	"contentid":         {},
	"contentids":        {},
	"ctaid":             {},
	"ctaids":            {},
	"mediaid":           {},
	"mediaids":          {},
	"projectid":         {},
	"relatedarticleid":  {},
	"relatedarticleids": {},
	"revisionid":        {},
	"revisionids":       {},
	"seriesid":          {},
	"seriesids":         {},
	"sourceid":          {},
	"sourceids":         {},
	"tagid":             {},
	"tagids":            {},
	"taxonomytermid":    {},
	"taxonomytermids":   {},
	"targetcontentid":   {},
	"targetcontentids":  {},
}

var projectScopedHTMLReferencePattern = regexp.MustCompile(`(?i)\bdata-(?:article|asset|author|category|claim|content|cta|media|project|related-article|revision|series|source|tag|taxonomy-term|target-content)-ids?\s*=`)

type AdminRevision struct {
	ID             string `json:"id"`
	ProjectID      string `json:"projectId"`
	ArticleID      string `json:"articleId"`
	RevisionNumber int64  `json:"revisionNumber"`
	Title          string `json:"title"`
	Deck           string `json:"deck,omitempty"`
	Excerpt        string `json:"excerpt,omitempty"`
	ShortAnswer    string `json:"shortAnswer,omitempty"`
	EditorialState string `json:"editorialState"`
	ContentHash    string `json:"contentHash"`
	CreatedAt      string `json:"createdAt"`
}

type AdminRevisionSummary struct {
	AdminRevision
	BaseRevisionID string `json:"baseRevisionId,omitempty"`
	Published      bool   `json:"published"`
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
	OriginProjectID  string         `json:"originProjectId,omitempty"`
	OriginArticleID  string         `json:"originArticleId,omitempty"`
	ArticleType      string         `json:"articleType"`
	Slug             string         `json:"slug"`
	Title            string         `json:"title"`
	EditorialState   string         `json:"editorialState"`
	PublicationState string         `json:"publicationState"`
	CanonicalPolicy  string         `json:"canonicalPolicy"`
	ScheduledForUTC  string         `json:"scheduledForUtc,omitempty"`
	PublishedAt      string         `json:"publishedAt,omitempty"`
	CanonicalURL     string         `json:"canonicalUrl,omitempty"`
	ArchivedAt       string         `json:"archivedAt,omitempty"`
	LatestRevision   *AdminRevision `json:"latestRevision,omitempty"`
	CreatedAt        string         `json:"createdAt"`
}

type ArticleListFilter struct {
	Search           string
	EditorialState   string
	PublicationState string
	IncludeArchived  bool
}

type ArticleInput struct {
	ArticleType       string
	Title             string
	Slug              string
	PrimaryCategoryID string
	Contributors      []RevisionContributorInput
	Deck              string
	Excerpt           string
	ShortAnswer       string
	BodyDocument      any
	HTML              string
	SEO               SEOInput
}

type RevisionInput struct {
	BaseRevisionID    string
	Title             string
	PrimaryCategoryID string
	Contributors      []RevisionContributorInput
	Deck              string
	Excerpt           string
	ShortAnswer       string
	BodyDocument      any
	HTML              string
	SEO               SEOInput
}

type RevisionContributorInput struct {
	AuthorID string `json:"authorId"`
	Role     string `json:"role"`
	Position int    `json:"position"`
}

type SEOInput struct {
	Title            string
	Description      string
	Robots           string
	OpenGraphTitle   string
	OpenGraphSummary string
	OpenGraphImage   string
}

type PublicationInput struct {
	RevisionID      string
	Slug            string
	CanonicalURL    string
	ScheduledForUTC string
}

type RollbackInput struct {
	RevisionID string
}

type CopyArticleInput struct {
	DestinationProjectID string
	SourceRevisionID     string
	PrimaryCategoryID    string
	Slug                 string
	CanonicalDecision    string
	CanonicalOriginalURL string
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
}

func (s *Store) ListArticlesForUser(ctx context.Context, userID, projectID, cursor string, limit int, filter ArticleListFilter) ([]AdminArticle, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	filter, err := normalizeArticleListFilter(filter)
	if err != nil {
		return nil, err
	}
	searchPattern := "%" + strings.ToLower(filter.Search) + "%"
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
		WHERE item.project_id = ?
		  AND (? = 1 OR item.archived_at IS NULL)
		  AND (? = '' OR item.id > ?)
		  AND (? = '' OR revision.editorial_state = ?)
		  AND (? = '' OR COALESCE(publication.publication_state, 'unpublished') = ?)
		  AND (
		    ? = ''
		    OR LOWER(revision.title) LIKE ? ESCAPE '\'
		    OR LOWER(COALESCE(publication.slug, '')) LIKE ? ESCAPE '\'
		    OR LOWER(item.article_type) LIKE ? ESCAPE '\'
		  )
		ORDER BY item.id
		LIMIT ?
	`, projectID, boolToInt(filter.IncludeArchived), cursor, cursor,
		filter.EditorialState, filter.EditorialState,
		filter.PublicationState, filter.PublicationState,
		filter.Search, searchPattern, searchPattern, searchPattern,
		limit)
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

func normalizeArticleListFilter(filter ArticleListFilter) (ArticleListFilter, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	filter.EditorialState = strings.TrimSpace(filter.EditorialState)
	filter.PublicationState = strings.TrimSpace(filter.PublicationState)
	if len([]rune(filter.Search)) > 100 {
		return ArticleListFilter{}, fmt.Errorf("%w: article search cannot exceed 100 characters", ErrValidation)
	}
	if filter.Search != "" {
		filter.Search = strings.NewReplacer(
			`\`, `\\`,
			`%`, `\%`,
			`_`, `\_`,
		).Replace(strings.ToLower(filter.Search))
	}
	switch filter.EditorialState {
	case "", "draft", "in_review", "changes_requested", "approved":
	default:
		return ArticleListFilter{}, fmt.Errorf("%w: unsupported editorialState", ErrValidation)
	}
	switch filter.PublicationState {
	case "", "unpublished", "scheduled", "published", "archived":
	default:
		return ArticleListFilter{}, fmt.Errorf("%w: unsupported publicationState", ErrValidation)
	}
	if filter.PublicationState == "archived" {
		filter.IncludeArchived = true
	}
	return filter, nil
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
	rendered, err := renderRevisionBody(input.BodyDocument, input.HTML, input.Title)
	if err != nil {
		return AdminArticle{}, err
	}
	taxonomyJSON, err := taxonomySnapshotJSON(category)
	if err != nil {
		return AdminArticle{}, err
	}
	input.SEO = normalizeSEOInput(input.SEO, input.Title, input.Excerpt)
	if err := validateSEOInput(input.SEO); err != nil {
		return AdminArticle{}, err
	}
	seoJSON, err := seoSnapshotJSON(input.SEO, canonicalURL(project, input.Slug))
	if err != nil {
		return AdminArticle{}, err
	}
	attribution, err := buildRevisionAttribution(ctx, tx, projectID, "", input.Contributors)
	if err != nil {
		return AdminArticle{}, err
	}
	contentHash, err := revisionContentHash(
		input.Title, rendered.HTML, rendered.DocumentJSON, taxonomyJSON, seoJSON,
		attribution.AuthorSnapshotJSON, attribution.ContributorSnapshotJSON,
	)
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
		  markdown_export, table_of_contents_json, word_count, reading_time_seconds,
		  author_snapshot_json, contributor_snapshot_json, taxonomy_snapshot_json,
		  seo_snapshot_json, content_hash, editorial_state
		) VALUES (?, ?, ?, 1, 'human', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft')
	`, revisionID, projectID, articleID, actorUserID, input.Title, nullIfEmpty(input.Deck),
		nullIfEmpty(input.Excerpt), nullIfEmpty(input.ShortAnswer), rendered.DocumentJSON, rendered.HTML, rendered.PlainText,
		rendered.Markdown, rendered.TableOfContents, wordCount(rendered.PlainText), readingTimeSeconds(rendered.PlainText),
		attribution.AuthorSnapshotJSON, attribution.ContributorSnapshotJSON,
		taxonomyJSON, seoJSON, contentHash); err != nil {
		return AdminArticle{}, err
	}
	if err := insertRevisionContributors(ctx, tx, projectID, revisionID, attribution.Records); err != nil {
		return AdminArticle{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_publications(
		  id, project_id, content_id, slug, canonical_url, publication_state
		) VALUES (?, ?, ?, ?, ?, 'unpublished')
	`, publicationID, projectID, articleID, input.Slug, canonicalURL(project, input.Slug)); err != nil {
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
	seoProvided := hasSEOInput(input.SEO)
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
	if !seoProvided {
		var baseSEOJSON string
		if err := tx.QueryRowContext(ctx, `
			SELECT seo_snapshot_json
			FROM content_revisions
			WHERE project_id = ? AND content_id = ? AND id = ?
		`, projectID, articleID, baseRevisionID).Scan(&baseSEOJSON); err != nil {
			return AdminRevision{}, err
		}
		input.SEO = seoInputFromSnapshot(baseSEOJSON, input.Title, input.Excerpt)
	}
	input.SEO = normalizeSEOInput(input.SEO, input.Title, input.Excerpt)
	if err := validateSEOInput(input.SEO); err != nil {
		return AdminRevision{}, err
	}
	revisionID, err := securityRandomID("rev")
	if err != nil {
		return AdminRevision{}, err
	}
	rendered, err := renderRevisionBody(input.BodyDocument, input.HTML, input.Title)
	if err != nil {
		return AdminRevision{}, err
	}
	taxonomyJSON, err := taxonomySnapshotJSON(category)
	if err != nil {
		return AdminRevision{}, err
	}
	seoJSON, err := seoSnapshotJSON(input.SEO, "")
	if err != nil {
		return AdminRevision{}, err
	}
	attribution, err := buildRevisionAttribution(ctx, tx, projectID, baseRevisionID, input.Contributors)
	if err != nil {
		return AdminRevision{}, err
	}
	contentHash, err := revisionContentHash(
		input.Title, rendered.HTML, rendered.DocumentJSON, taxonomyJSON, seoJSON,
		attribution.AuthorSnapshotJSON, attribution.ContributorSnapshotJSON,
	)
	if err != nil {
		return AdminRevision{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, base_revision_id, created_by_type, created_by_user_id,
		  title, deck, excerpt, short_answer, body_document_json, sanitized_html, plain_text,
		  markdown_export, table_of_contents_json, word_count, reading_time_seconds,
		  author_snapshot_json, contributor_snapshot_json, taxonomy_snapshot_json,
		  seo_snapshot_json, content_hash, editorial_state
		) VALUES (?, ?, ?, ?, ?, 'human', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft')
	`, revisionID, projectID, articleID, nextNumber, input.BaseRevisionID, actorUserID, input.Title, nullIfEmpty(input.Deck),
		nullIfEmpty(input.Excerpt), nullIfEmpty(input.ShortAnswer), rendered.DocumentJSON, rendered.HTML, rendered.PlainText,
		rendered.Markdown, rendered.TableOfContents, wordCount(rendered.PlainText), readingTimeSeconds(rendered.PlainText),
		attribution.AuthorSnapshotJSON, attribution.ContributorSnapshotJSON,
		taxonomyJSON, seoJSON, contentHash); err != nil {
		return AdminRevision{}, err
	}
	if err := insertRevisionContributors(ctx, tx, projectID, revisionID, attribution.Records); err != nil {
		return AdminRevision{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM article_autosaves
		WHERE project_id = ? AND content_id = ? AND user_id = ?
	`, projectID, articleID, actorUserID); err != nil {
		return AdminRevision{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminRevision{}, err
	}
	return s.GetRevisionForUser(ctx, actorUserID, projectID, revisionID)
}

func (s *Store) CopyArticleToProject(ctx context.Context, actorUserID, sourceProjectID, sourceArticleID string, input CopyArticleInput) (AdminArticle, error) {
	input = applyCopyArticleDefaults(input)
	if err := validateCopyArticleInput(sourceProjectID, input); err != nil {
		return AdminArticle{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminArticle{}, err
	}
	defer tx.Rollback()

	if _, err := projectRoleTx(ctx, tx, actorUserID, sourceProjectID); err != nil {
		return AdminArticle{}, err
	}
	destinationRole, err := projectRoleTx(ctx, tx, actorUserID, input.DestinationProjectID)
	if err != nil {
		return AdminArticle{}, err
	}
	if !canWriteContent(destinationRole) {
		return AdminArticle{}, ErrForbidden
	}

	sourceProject, err := loadWorkflowProject(ctx, tx, sourceProjectID)
	if err != nil {
		return AdminArticle{}, err
	}
	if sourceProject.Status != "active" {
		return AdminArticle{}, fmt.Errorf("%w: source project must be active", ErrInvalidWorkflow)
	}
	destinationProject, err := loadWorkflowProject(ctx, tx, input.DestinationProjectID)
	if err != nil {
		return AdminArticle{}, err
	}
	if destinationProject.Status != "active" {
		return AdminArticle{}, fmt.Errorf("%w: destination project must be active", ErrInvalidWorkflow)
	}
	destinationCategory, err := loadCategory(ctx, tx, input.DestinationProjectID, input.PrimaryCategoryID)
	if err != nil {
		return AdminArticle{}, err
	}
	source, err := loadCopySourceRevision(ctx, tx, sourceProjectID, sourceArticleID, input.SourceRevisionID)
	if err != nil {
		return AdminArticle{}, err
	}
	if err := validateCopyBodyReferences(source.BodyDocumentJSON, source.SanitizedHTML, source.MarkdownExport); err != nil {
		return AdminArticle{}, err
	}
	if input.CanonicalDecision == "canonical_original" {
		if strings.TrimSpace(source.CanonicalURL) == "" {
			return AdminArticle{}, fmt.Errorf("%w: selected source revision has no canonical URL", ErrInvalidWorkflow)
		}
		if input.CanonicalOriginalURL != "" && !canonicalURLsEqual(input.CanonicalOriginalURL, source.CanonicalURL) {
			return AdminArticle{}, fmt.Errorf("%w: canonicalOriginalUrl must match the selected source revision's canonical URL", ErrValidation)
		}
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
	taxonomyJSON, err := taxonomySnapshotJSON(destinationCategory)
	if err != nil {
		return AdminArticle{}, err
	}
	destinationCanonicalURL := canonicalURL(destinationProject, input.Slug)
	if input.CanonicalDecision == "canonical_original" {
		destinationCanonicalURL = source.CanonicalURL
	}
	seoJSON, err := copySEOSnapshotJSON(source.SEOSnapshotJSON, source.Title, source.Excerpt, destinationCanonicalURL)
	if err != nil {
		return AdminArticle{}, err
	}
	contentHash, err := revisionContentHash(
		source.Title, source.SanitizedHTML, source.BodyDocumentJSON, taxonomyJSON, seoJSON, "[]", "[]",
	)
	if err != nil {
		return AdminArticle{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content_items(
		  id, project_id, article_type, origin_project_id, origin_content_id, created_by
		) VALUES (?, ?, ?, ?, ?, ?)
	`, articleID, input.DestinationProjectID, source.ArticleType, sourceProjectID, sourceArticleID, actorUserID); err != nil {
		return AdminArticle{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO article_taxonomy(project_id, content_id, taxonomy_term_id, is_primary)
		VALUES (?, ?, ?, 1)
	`, input.DestinationProjectID, articleID, destinationCategory.ID); err != nil {
		return AdminArticle{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, created_by_type, created_by_user_id,
		  title, alternate_title, deck, excerpt, short_answer, body_document_json,
		  sanitized_html, plain_text, markdown_export, table_of_contents_json,
		  word_count, reading_time_seconds, taxonomy_snapshot_json, seo_snapshot_json,
		  change_summary, content_hash, ai_assistance_level, ai_provenance_summary_json,
		  editorial_state
		) VALUES (
		  ?, ?, ?, 1, 'human', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft'
		)
	`, revisionID, input.DestinationProjectID, articleID, actorUserID,
		source.Title, nullIfEmpty(source.AlternateTitle), nullIfEmpty(source.Deck),
		nullIfEmpty(source.Excerpt), nullIfEmpty(source.ShortAnswer), source.BodyDocumentJSON,
		source.SanitizedHTML, source.PlainText, source.MarkdownExport, source.TableOfContentsJSON,
		source.WordCount, source.ReadingTimeSeconds, taxonomyJSON, seoJSON,
		fmt.Sprintf("Copied from %s/%s revision %s", sourceProjectID, sourceArticleID, input.SourceRevisionID),
		contentHash, source.AIAssistanceLevel, source.AIProvenanceSummaryJSON,
	); err != nil {
		return AdminArticle{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_publications(
		  id, project_id, content_id, slug, canonical_url, canonical_policy, publication_state
		) VALUES (?, ?, ?, ?, ?, ?, 'unpublished')
	`, publicationID, input.DestinationProjectID, articleID, input.Slug,
		destinationCanonicalURL, input.CanonicalDecision); err != nil {
		return AdminArticle{}, err
	}

	auditMetadata := map[string]string{
		"sourceProjectId":      sourceProjectID,
		"sourceArticleId":      sourceArticleID,
		"sourceRevisionId":     input.SourceRevisionID,
		"destinationProjectId": input.DestinationProjectID,
		"destinationArticleId": articleID,
		"canonicalDecision":    input.CanonicalDecision,
		"canonicalUrl":         destinationCanonicalURL,
	}
	if err := insertAuditEventTx(
		ctx, tx, sourceProjectID, "user", actorUserID, "content.copy_from",
		"content", sourceArticleID, "success", auditMetadata,
	); err != nil {
		return AdminArticle{}, err
	}
	if err := insertAuditEventTx(
		ctx, tx, input.DestinationProjectID, "user", actorUserID, "content.copy_to",
		"content", articleID, "success", auditMetadata,
	); err != nil {
		return AdminArticle{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminArticle{}, err
	}
	return s.GetArticleForUser(ctx, actorUserID, input.DestinationProjectID, articleID)
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
	if revision.EditorialState == "approved" {
		return AdminRevision{}, fmt.Errorf("%w: revision is already approved", ErrInvalidWorkflow)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminRevision{}, err
	}
	defer tx.Rollback()
	selfApproval, err := enforceRevisionApprovalPolicy(ctx, tx, actorUserID, projectID, revisionID)
	if err != nil {
		return AdminRevision{}, err
	}
	if err := ensureRevisionPrimaryAuthor(ctx, tx, projectID, revisionID); err != nil {
		return AdminRevision{}, err
	}
	if err := ensureRevisionClaimsApproved(ctx, tx, projectID, revisionID); err != nil {
		return AdminRevision{}, err
	}
	if err := ensureRevisionQualityApproved(ctx, tx, projectID, revisionID); err != nil {
		return AdminRevision{}, err
	}
	sourceSnapshotJSON, claimSnapshotJSON, err := buildRevisionTrustSnapshots(ctx, tx, projectID, revisionID)
	if err != nil {
		return AdminRevision{}, err
	}
	approvedContentHash, err := approvalContentHash(revision.ContentHash, sourceSnapshotJSON, claimSnapshotJSON)
	if err != nil {
		return AdminRevision{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE content_revisions
		SET editorial_state = 'approved',
		    source_snapshot_json = ?,
		    claim_snapshot_json = ?,
		    content_hash = ?
		WHERE project_id = ? AND id = ?
	`, sourceSnapshotJSON, claimSnapshotJSON, approvedContentHash, projectID, revisionID); err != nil {
		return AdminRevision{}, err
	}
	decisionID, err := securityRandomID("appr")
	if err != nil {
		return AdminRevision{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO approval_decisions(
		  id, project_id, content_id, revision_id, decision, content_hash,
		  decided_by, note, self_approval
		) VALUES (?, ?, ?, ?, 'approved', ?, ?, ?, ?)
	`, decisionID, projectID, revision.ArticleID, revisionID, approvedContentHash,
		actorUserID, nullIfEmpty(note), selfApproval); err != nil {
		return AdminRevision{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "revision.approve", "revision", revisionID, "success", map[string]any{
		"self_approval": selfApproval,
		"content_hash":  approvedContentHash,
	}); err != nil {
		return AdminRevision{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminRevision{}, err
	}
	return s.GetRevisionForUser(ctx, actorUserID, projectID, revisionID)
}

func ensureRevisionPrimaryAuthor(ctx context.Context, tx *sql.Tx, projectID, revisionID string) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT contributor.author_id, contributor.public_snapshot_json
		FROM revision_contributors contributor
		WHERE contributor.project_id = ?
		  AND contributor.revision_id = ?
		  AND contributor.role = 'primary_author'
	`, projectID, revisionID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type primaryAuthorRecord struct {
		AuthorID     string
		SnapshotJSON string
	}
	primaryAuthors := []primaryAuthorRecord{}
	for rows.Next() {
		var record primaryAuthorRecord
		if err := rows.Scan(&record.AuthorID, &record.SnapshotJSON); err != nil {
			return err
		}
		primaryAuthors = append(primaryAuthors, record)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(primaryAuthors) != 1 {
		return fmt.Errorf("%w: approval requires exactly one accountable primary author", ErrInvalidWorkflow)
	}

	var snapshot Author
	if err := json.Unmarshal([]byte(primaryAuthors[0].SnapshotJSON), &snapshot); err != nil {
		return fmt.Errorf("%w: primary author snapshot is invalid", ErrInvalidWorkflow)
	}
	if snapshot.ID != primaryAuthors[0].AuthorID || strings.TrimSpace(snapshot.DisplayName) == "" {
		return fmt.Errorf("%w: primary author snapshot is incomplete", ErrInvalidWorkflow)
	}
	return nil
}

func enforceRevisionApprovalPolicy(
	ctx context.Context,
	tx *sql.Tx,
	actorUserID, projectID, revisionID string,
) (bool, error) {
	var role, creatorType, creatorUserID string
	var soloOwnerApprovalEnabled bool
	err := tx.QueryRowContext(ctx, `
		SELECT membership.role, revision.created_by_type,
		       COALESCE(revision.created_by_user_id, ''),
		       project.solo_owner_approval_enabled
		FROM content_revisions revision
		JOIN projects project
		  ON project.id = revision.project_id
		JOIN project_memberships membership
		  ON membership.project_id = revision.project_id
		 AND membership.user_id = ?
		 AND membership.status = 'active'
		WHERE revision.project_id = ? AND revision.id = ?
	`, actorUserID, projectID, revisionID).Scan(
		&role,
		&creatorType,
		&creatorUserID,
		&soloOwnerApprovalEnabled,
	)
	if err != nil {
		return false, err
	}
	selfApproval := creatorType == "human" && creatorUserID != "" && creatorUserID == actorUserID
	if !selfApproval {
		return false, nil
	}
	if role != "project_owner" {
		return false, fmt.Errorf("%w: reviewers cannot approve a revision they created", ErrInvalidWorkflow)
	}
	if !soloOwnerApprovalEnabled {
		return false, fmt.Errorf("%w: project owner self-approval requires explicit solo-owner mode", ErrInvalidWorkflow)
	}
	return true, nil
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

	publication, err := loadPublication(ctx, tx, projectID, articleID)
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
		publication.Slug,
		publication.CanonicalURL,
		"",
		"published",
	)
	if err != nil {
		return AdminArticle{}, err
	}
	version, err := loadPublicationVersion(ctx, tx, projectID, articleID)
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

func (s *Store) ArchiveArticle(ctx context.Context, actorUserID, projectID, articleID string) error {
	if err := s.requireContentPublish(ctx, actorUserID, projectID); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	project, err := loadWorkflowProject(ctx, tx, projectID)
	if err != nil {
		return err
	}
	if project.Status != "active" {
		return fmt.Errorf("%w: project must be active", ErrInvalidWorkflow)
	}
	if _, err := loadArticleType(ctx, tx, projectID, articleID); err != nil {
		return err
	}
	publication, publicationErr := loadPublication(ctx, tx, projectID, articleID)
	if publicationErr != nil && !errors.Is(publicationErr, sql.ErrNoRows) {
		return publicationErr
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE content_items
		SET archived_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND id = ? AND archived_at IS NULL
	`, projectID, articleID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE project_publications
		SET publication_state = 'archived',
		    scheduled_for_utc = NULL,
		    unpublished_at = CASE
		      WHEN publication_state = 'published' THEN COALESCE(unpublished_at, CURRENT_TIMESTAMP)
		      ELSE unpublished_at
		    END,
		    retired_at = COALESCE(retired_at, CURRENT_TIMESTAMP),
		    publication_version = publication_version + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND content_id = ? AND publication_state <> 'archived'
	`, projectID, articleID); err != nil {
		return err
	}
	if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
		return err
	}
	if publicationErr == nil && publication.PublicationState != "archived" {
		if err := insertPublicationOutbox(
			ctx,
			tx,
			projectID,
			articleID,
			publication.PublishedRevisionID,
			"content.archived",
			publication.CanonicalURL,
			publication.PublicationVersion+1,
		); err != nil {
			return err
		}
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "content.archive", "content", articleID, "success", nil); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RestoreArticle(ctx context.Context, actorUserID, projectID, articleID string) (AdminArticle, error) {
	if err := s.requireContentPublish(ctx, actorUserID, projectID); err != nil {
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
	var publicationID, revisionID, canonicalURL string
	var publicationVersion int64
	if err := tx.QueryRowContext(ctx, `
		SELECT publication.id, COALESCE(publication.published_revision_id, ''),
		       publication.canonical_url, publication.publication_version
		FROM content_items item
		JOIN project_publications publication
		  ON publication.project_id = item.project_id
		 AND publication.content_id = item.id
		WHERE item.project_id = ? AND item.id = ? AND item.archived_at IS NOT NULL
		  AND publication.publication_state = 'archived'
	`, projectID, articleID).Scan(&publicationID, &revisionID, &canonicalURL, &publicationVersion); err != nil {
		return AdminArticle{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE content_items
		SET archived_at = NULL
		WHERE project_id = ? AND id = ? AND archived_at IS NOT NULL
	`, projectID, articleID)
	if err != nil {
		return AdminArticle{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return AdminArticle{}, err
	}
	if affected != 1 {
		return AdminArticle{}, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE project_publications
		SET publication_state = 'unpublished', scheduled_for_utc = NULL,
		    retired_at = NULL, publication_version = publication_version + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE project_id = ? AND content_id = ? AND publication_state = 'archived'
	`, projectID, articleID); err != nil {
		return AdminArticle{}, err
	}
	if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
		return AdminArticle{}, err
	}
	if err := insertPublicationOutbox(ctx, tx, projectID, articleID, revisionID, "content.restored", canonicalURL, publicationVersion+1); err != nil {
		return AdminArticle{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "content.restore", "content", articleID, "success", map[string]string{
		"publication_id": publicationID,
	}); err != nil {
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
		       editorial_state, content_hash, created_at
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
		       COALESCE(revision.short_answer, ''), revision.editorial_state,
		       revision.content_hash, revision.created_at, COALESCE(revision.base_revision_id, ''),
		       EXISTS(
		         SELECT 1 FROM project_publications publication
		         WHERE publication.project_id = revision.project_id
		           AND publication.content_id = revision.content_id
		           AND publication.published_revision_id = revision.id
		           AND publication.publication_state = 'published'
		       )
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
		       COALESCE(revision.short_answer, ''), revision.editorial_state,
		       revision.content_hash, revision.created_at, COALESCE(revision.base_revision_id, ''),
		       EXISTS(
		         SELECT 1 FROM project_publications publication
		         WHERE publication.project_id = revision.project_id
		           AND publication.content_id = revision.content_id
		           AND publication.published_revision_id = revision.id
		           AND publication.publication_state = 'published'
		       ),
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
	previousPublication, previousPublicationErr := loadPublication(ctx, tx, projectID, articleID)
	if previousPublicationErr != nil && !errors.Is(previousPublicationErr, sql.ErrNoRows) {
		return AdminArticle{}, previousPublicationErr
	}
	publicationID, err := upsertPublication(ctx, tx, projectID, articleID, revision.ID, input.Slug, canonical, input.ScheduledForUTC, state)
	if err != nil {
		return AdminArticle{}, err
	}
	if state == "published" {
		version, err := loadPublicationVersion(ctx, tx, projectID, articleID)
		if err != nil {
			return AdminArticle{}, err
		}
		if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
			return AdminArticle{}, err
		}
		eventType := "content.published"
		var slugChange []string
		if previousPublicationErr == nil && previousPublication.FirstPublishedAt != "" {
			switch {
			case previousPublication.Slug != input.Slug:
				eventType = "content.slug_changed"
				slugChange = []string{previousPublication.Slug, input.Slug}
				if err := insertArticleSlugRedirect(ctx, tx, project, previousPublication.Slug, input.Slug); err != nil {
					return AdminArticle{}, err
				}
			case previousPublication.PublicationState == "unpublished":
				eventType = "content.restored"
			default:
				eventType = "content.updated"
			}
		}
		if err := insertPublicationOutbox(ctx, tx, projectID, articleID, revision.ID, eventType, canonical, version, slugChange...); err != nil {
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
	if canWriteContent(role) {
		return nil
	}
	return ErrForbidden
}

func canWriteContent(role string) bool {
	return role == "project_owner" || role == "project_admin" || role == "editor" || role == "writer"
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
	item.id, item.project_id, COALESCE(item.origin_project_id, ''),
	COALESCE(item.origin_content_id, ''), item.article_type,
	COALESCE(publication.slug, ''),
	revision.title, revision.editorial_state,
	COALESCE(publication.publication_state, 'unpublished'),
	COALESCE(publication.canonical_policy, 'self'),
	COALESCE(publication.scheduled_for_utc, ''),
	COALESCE(publication.first_published_at, ''),
	COALESCE(publication.canonical_url, ''),
	COALESCE(item.archived_at, ''),
	revision.id, revision.revision_number, revision.title,
	COALESCE(revision.deck, ''), COALESCE(revision.excerpt, ''),
	COALESCE(revision.short_answer, ''),
	revision.editorial_state, revision.content_hash, revision.created_at,
	item.created_at
`

func scanAdminArticle(row rowScanner) (AdminArticle, error) {
	var article AdminArticle
	revision := AdminRevision{}
	err := row.Scan(
		&article.ID,
		&article.ProjectID,
		&article.OriginProjectID,
		&article.OriginArticleID,
		&article.ArticleType,
		&article.Slug,
		&article.Title,
		&article.EditorialState,
		&article.PublicationState,
		&article.CanonicalPolicy,
		&article.ScheduledForUTC,
		&article.PublishedAt,
		&article.CanonicalURL,
		&article.ArchivedAt,
		&revision.ID,
		&revision.RevisionNumber,
		&revision.Title,
		&revision.Deck,
		&revision.Excerpt,
		&revision.ShortAnswer,
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
		&revision.EditorialState,
		&revision.ContentHash,
		&revision.CreatedAt,
	)
	return revision, err
}

func scanAdminRevisionSummary(row rowScanner) (AdminRevisionSummary, error) {
	var revision AdminRevisionSummary
	err := row.Scan(
		&revision.ID,
		&revision.ProjectID,
		&revision.ArticleID,
		&revision.RevisionNumber,
		&revision.Title,
		&revision.Deck,
		&revision.Excerpt,
		&revision.ShortAnswer,
		&revision.EditorialState,
		&revision.ContentHash,
		&revision.CreatedAt,
		&revision.BaseRevisionID,
		&revision.Published,
	)
	if err != nil {
		return AdminRevisionSummary{}, err
	}
	return revision, nil
}

func scanAdminRevisionDetail(row rowScanner) (AdminRevisionDetail, error) {
	var revision AdminRevisionDetail
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
		&revision.EditorialState,
		&revision.ContentHash,
		&revision.CreatedAt,
		&revision.BaseRevisionID,
		&revision.Published,
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
	Slug                string
	CanonicalURL        string
	PublicationState    string
	PublicationVersion  int64
	FirstPublishedAt    string
}

type copySourceRevision struct {
	ArticleType             string
	Title                   string
	AlternateTitle          string
	Deck                    string
	Excerpt                 string
	ShortAnswer             string
	BodyDocumentJSON        string
	SanitizedHTML           string
	PlainText               string
	MarkdownExport          string
	TableOfContentsJSON     string
	WordCount               int64
	ReadingTimeSeconds      int64
	CanonicalURL            string
	AIAssistanceLevel       string
	AIProvenanceSummaryJSON string
	SEOSnapshotJSON         string
}

func loadCopySourceRevision(
	ctx context.Context,
	tx *sql.Tx,
	projectID,
	articleID,
	revisionID string,
) (copySourceRevision, error) {
	var revision copySourceRevision
	err := tx.QueryRowContext(ctx, `
		SELECT item.article_type, revision.title, COALESCE(revision.alternate_title, ''),
		       COALESCE(revision.deck, ''), COALESCE(revision.excerpt, ''),
		       COALESCE(revision.short_answer, ''), revision.body_document_json,
		       revision.sanitized_html, revision.plain_text, revision.markdown_export,
		       revision.table_of_contents_json, revision.word_count,
		       revision.reading_time_seconds, COALESCE((
		         SELECT publication.canonical_url
		         FROM project_publications publication
		         WHERE publication.project_id = revision.project_id
		           AND publication.content_id = revision.content_id
		         LIMIT 1
		       ), ''), revision.ai_assistance_level,
		       revision.ai_provenance_summary_json, revision.seo_snapshot_json
		FROM content_items item
		JOIN content_revisions revision
		  ON revision.project_id = item.project_id
		 AND revision.content_id = item.id
		WHERE item.project_id = ?
		  AND item.id = ?
		  AND item.archived_at IS NULL
		  AND revision.id = ?
	`, projectID, articleID, revisionID).Scan(
		&revision.ArticleType,
		&revision.Title,
		&revision.AlternateTitle,
		&revision.Deck,
		&revision.Excerpt,
		&revision.ShortAnswer,
		&revision.BodyDocumentJSON,
		&revision.SanitizedHTML,
		&revision.PlainText,
		&revision.MarkdownExport,
		&revision.TableOfContentsJSON,
		&revision.WordCount,
		&revision.ReadingTimeSeconds,
		&revision.CanonicalURL,
		&revision.AIAssistanceLevel,
		&revision.AIProvenanceSummaryJSON,
		&revision.SEOSnapshotJSON,
	)
	return revision, err
}

func loadPublication(ctx context.Context, tx *sql.Tx, projectID, articleID string) (publicationRecord, error) {
	var publication publicationRecord
	err := tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(published_revision_id, ''), slug, canonical_url,
		       publication_state, publication_version, COALESCE(first_published_at, '')
		FROM project_publications
		WHERE project_id = ?
		  AND content_id = ?
	`, projectID, articleID).Scan(
		&publication.ID,
		&publication.PublishedRevisionID,
		&publication.Slug,
		&publication.CanonicalURL,
		&publication.PublicationState,
		&publication.PublicationVersion,
		&publication.FirstPublishedAt,
	)
	return publication, err
}

func loadWorkflowProject(ctx context.Context, tx *sql.Tx, projectID string) (workflowProject, error) {
	var project workflowProject
	err := tx.QueryRowContext(ctx, `
		SELECT id, status, COALESCE(primary_domain, ''), blog_base_path
		FROM projects
		WHERE id = ?
	`, projectID).Scan(&project.ID, &project.Status, &project.PrimaryDomain, &project.BlogBasePath)
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
		       editorial_state, content_hash, created_at
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

func upsertPublication(ctx context.Context, tx *sql.Tx, projectID, articleID, revisionID, slug, canonicalURL, scheduledForUTC, state string) (string, error) {
	publicationID, err := securityRandomID("pubn")
	if err != nil {
		return "", err
	}
	robotsDirective := "index,follow"
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(NULLIF(json_extract(seo_snapshot_json, '$.robots'), ''), 'index,follow')
		FROM content_revisions
		WHERE project_id = ? AND content_id = ? AND id = ?
	`, projectID, articleID, revisionID).Scan(&robotsDirective); err != nil {
		return "", err
	}
	if state == "scheduled" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO project_publications(
			  id, project_id, content_id, slug, canonical_url,
			  robots_directive, published_revision_id, publication_state, scheduled_for_utc
			) VALUES (?, ?, ?, ?, ?, ?, ?, 'scheduled', ?)
			ON CONFLICT(project_id, content_id) DO UPDATE SET
			  slug = excluded.slug,
			  canonical_url = excluded.canonical_url,
			  robots_directive = excluded.robots_directive,
			  published_revision_id = excluded.published_revision_id,
			  publication_state = 'scheduled',
			  scheduled_for_utc = excluded.scheduled_for_utc,
			  updated_at = CURRENT_TIMESTAMP
		`, publicationID, projectID, articleID, slug, canonicalURL, robotsDirective, revisionID, scheduledForUTC)
	} else {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO project_publications(
			  id, project_id, content_id, slug, canonical_url,
			  robots_directive, published_revision_id, publication_state, first_published_at,
			  materially_modified_at, publication_version
			) VALUES (?, ?, ?, ?, ?, ?, ?, 'published', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)
			ON CONFLICT(project_id, content_id) DO UPDATE SET
			  slug = excluded.slug,
			  canonical_url = excluded.canonical_url,
			  robots_directive = excluded.robots_directive,
			  published_revision_id = excluded.published_revision_id,
			  publication_state = 'published',
			  scheduled_for_utc = NULL,
			  first_published_at = COALESCE(project_publications.first_published_at, CURRENT_TIMESTAMP),
			  materially_modified_at = CURRENT_TIMESTAMP,
			  publication_version = project_publications.publication_version + 1,
			  updated_at = CURRENT_TIMESTAMP
		`, publicationID, projectID, articleID, slug, canonicalURL, robotsDirective, revisionID)
	}
	if err != nil {
		return "", err
	}
	var storedID string
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM project_publications
		WHERE project_id = ? AND content_id = ?
	`, projectID, articleID).Scan(&storedID)
	return storedID, err
}

func loadPublicationVersion(ctx context.Context, tx *sql.Tx, projectID, articleID string) (int64, error) {
	var version int64
	err := tx.QueryRowContext(ctx, `
		SELECT publication_version
		FROM project_publications
		WHERE project_id = ? AND content_id = ?
	`, projectID, articleID).Scan(&version)
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

func insertPublicationOutbox(
	ctx context.Context,
	tx *sql.Tx,
	projectID, articleID, revisionID, eventType, canonicalURL string,
	version int64,
	slugChange ...string,
) error {
	payloadValue := map[string]any{
		"project_id":          projectID,
		"content_id":          articleID,
		"revision_id":         revisionID,
		"publication_version": version,
		"canonical_url":       canonicalURL,
	}
	if len(slugChange) == 2 {
		payloadValue["old_slug"] = slugChange[0]
		payloadValue["new_slug"] = slugChange[1]
	}
	payload, err := json.Marshal(payloadValue)
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
		) VALUES (?, ?, ?, 'content', ?, ?, ?)
	`, eventID, projectID, eventType, articleID, string(payload), fmt.Sprintf("%s:%s:%s:%d", eventType, articleID, revisionID, version))
	return err
}

func insertArticleSlugRedirect(
	ctx context.Context,
	tx *sql.Tx,
	project workflowProject,
	oldSlug, newSlug string,
) error {
	sourcePath := articlePublicPath(project, oldSlug)
	targetPath := articlePublicPath(project, newSlug)
	if sourcePath == targetPath {
		return nil
	}
	var reservedTarget string
	err := tx.QueryRowContext(ctx, `
		SELECT target_path
		FROM slug_redirects
		WHERE project_id = ? AND source_path = ?
	`, project.ID, targetPath).Scan(&reservedTarget)
	switch {
	case err == nil && reservedTarget != sourcePath:
		return fmt.Errorf("%w: article slug is reserved by redirect history", ErrValidation)
	case err == nil:
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM slug_redirects
			WHERE project_id = ? AND source_path = ?
		`, project.ID, targetPath); err != nil {
			return err
		}
	case errors.Is(err, sql.ErrNoRows):
	default:
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE slug_redirects
		SET target_path = ?
		WHERE project_id = ? AND target_path = ?
	`, targetPath, project.ID, sourcePath); err != nil {
		return err
	}
	redirectID, err := securityRandomID("redirect")
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO slug_redirects(id, project_id, source_path, target_path, status_code)
		VALUES (?, ?, ?, ?, 301)
		ON CONFLICT(project_id, source_path) DO UPDATE SET
		  target_path = excluded.target_path,
		  status_code = 301
	`, redirectID, project.ID, sourcePath, targetPath); err != nil {
		return err
	}
	return nil
}

func articlePublicPath(project workflowProject, slug string) string {
	basePath := strings.TrimRight(project.BlogBasePath, "/")
	if basePath == "" {
		basePath = "/blog"
	}
	return basePath + "/" + slug
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
	input.PrimaryCategoryID = strings.TrimSpace(input.PrimaryCategoryID)
	input.Deck = strings.TrimSpace(input.Deck)
	input.Excerpt = strings.TrimSpace(input.Excerpt)
	input.ShortAnswer = strings.TrimSpace(input.ShortAnswer)
	input.HTML = strings.TrimSpace(input.HTML)
	return input
}

func applyCopyArticleDefaults(input CopyArticleInput) CopyArticleInput {
	input.DestinationProjectID = strings.TrimSpace(input.DestinationProjectID)
	input.SourceRevisionID = strings.TrimSpace(input.SourceRevisionID)
	input.PrimaryCategoryID = strings.TrimSpace(input.PrimaryCategoryID)
	input.Slug = slugify(input.Slug)
	input.CanonicalDecision = strings.ToLower(strings.TrimSpace(input.CanonicalDecision))
	input.CanonicalOriginalURL = strings.TrimSpace(input.CanonicalOriginalURL)
	return input
}

func validateCopyArticleInput(sourceProjectID string, input CopyArticleInput) error {
	if input.DestinationProjectID == "" {
		return fmt.Errorf("%w: destinationProjectId is required", ErrValidation)
	}
	if input.DestinationProjectID == sourceProjectID {
		return fmt.Errorf("%w: destinationProjectId must identify another project", ErrValidation)
	}
	if input.SourceRevisionID == "" {
		return fmt.Errorf("%w: sourceRevisionId is required", ErrValidation)
	}
	if input.PrimaryCategoryID == "" {
		return fmt.Errorf("%w: primaryCategoryId is required", ErrValidation)
	}
	if input.Slug == "" {
		return fmt.Errorf("%w: slug is required", ErrValidation)
	}
	switch input.CanonicalDecision {
	case "canonical_original":
		if input.CanonicalOriginalURL != "" {
			if _, err := normalizeCanonicalURL(input.CanonicalOriginalURL); err != nil {
				return fmt.Errorf("%w: canonicalOriginalUrl must be an absolute HTTP or HTTPS URL for canonical_original", ErrValidation)
			}
		}
	case "material_adaptation":
		if input.CanonicalOriginalURL != "" {
			return fmt.Errorf("%w: canonicalOriginalUrl is only allowed for canonical_original", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: canonicalDecision must be canonical_original or material_adaptation", ErrValidation)
	}
	return nil
}

func validateCopyBodyReferences(bodyDocumentJSON, sanitizedHTML, markdownExport string) error {
	var document any
	if err := json.Unmarshal([]byte(bodyDocumentJSON), &document); err != nil {
		return fmt.Errorf("%w: selected source revision has an invalid structured body", ErrInvalidWorkflow)
	}
	if reference := findProjectScopedBodyReference(document); reference != "" {
		return fmt.Errorf(
			"%w: source revision body contains project-scoped reference %q; remove or remap it before copying",
			ErrValidation,
			reference,
		)
	}
	for _, rendered := range []string{sanitizedHTML, markdownExport} {
		if match := projectScopedHTMLReferencePattern.FindString(rendered); match != "" {
			return fmt.Errorf(
				"%w: source revision body contains project-scoped reference %q; remove or remap it before copying",
				ErrValidation,
				strings.TrimSpace(match),
			)
		}
	}
	return nil
}

func findProjectScopedBodyReference(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalizedKey := strings.NewReplacer("-", "", "_", "").Replace(strings.ToLower(key))
			if _, scoped := projectScopedBodyReferenceKeys[normalizedKey]; scoped && hasReferenceValue(child) {
				return key
			}
			if reference := findProjectScopedBodyReference(child); reference != "" {
				return reference
			}
		}
	case []any:
		for _, child := range typed {
			if reference := findProjectScopedBodyReference(child); reference != "" {
				return reference
			}
		}
	}
	return ""
}

func hasReferenceValue(value any) bool {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	case nil:
		return false
	default:
		return true
	}
}

func canonicalURLsEqual(left, right string) bool {
	normalizedLeft, leftErr := normalizeCanonicalURL(left)
	normalizedRight, rightErr := normalizeCanonicalURL(right)
	return leftErr == nil && rightErr == nil && normalizedLeft == normalizedRight
}

func normalizeCanonicalURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", ErrValidation
	}
	scheme := strings.ToLower(parsed.Scheme)
	if parsed.Host == "" ||
		(scheme != "http" && scheme != "https") ||
		parsed.User != nil ||
		parsed.Fragment != "" {
		return "", ErrValidation
	}
	parsed.Scheme = scheme
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), nil
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

func taxonomySnapshotJSON(primary TaxonomyTerm) (string, error) {
	raw, err := json.Marshal(PublishedTaxonomy{
		PrimaryCategory: &primary,
		Categories:      []TaxonomyTerm{primary},
		Tags:            []TaxonomyTerm{},
		Topics:          []TaxonomyTerm{},
	})
	return string(raw), err
}

func normalizeSEOInput(input SEOInput, fallbackTitle, fallbackDescription string) SEOInput {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		input.Title = strings.TrimSpace(fallbackTitle)
	}
	input.Description = strings.TrimSpace(input.Description)
	if input.Description == "" {
		input.Description = strings.TrimSpace(fallbackDescription)
	}
	input.Robots = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(input.Robots), " ", ""))
	if input.Robots == "" {
		input.Robots = "index,follow"
	}
	input.OpenGraphTitle = strings.TrimSpace(input.OpenGraphTitle)
	if input.OpenGraphTitle == "" {
		input.OpenGraphTitle = input.Title
	}
	input.OpenGraphSummary = strings.TrimSpace(input.OpenGraphSummary)
	if input.OpenGraphSummary == "" {
		input.OpenGraphSummary = input.Description
	}
	input.OpenGraphImage = strings.TrimSpace(input.OpenGraphImage)
	return input
}

func validateSEOInput(input SEOInput) error {
	if len([]rune(input.Title)) > 300 || len([]rune(input.Description)) > 500 ||
		len([]rune(input.OpenGraphTitle)) > 300 || len([]rune(input.OpenGraphSummary)) > 500 {
		return fmt.Errorf("%w: SEO title or description exceeds its size limit", ErrValidation)
	}
	switch input.Robots {
	case "index,follow", "index,nofollow", "noindex,follow", "noindex,nofollow":
	default:
		return fmt.Errorf("%w: unsupported robots directive", ErrValidation)
	}
	if input.OpenGraphImage != "" && !safeRevisionURL(input.OpenGraphImage, false) {
		return fmt.Errorf("%w: Open Graph image must use HTTPS or a root-relative URL", ErrValidation)
	}
	return nil
}

func hasSEOInput(input SEOInput) bool {
	return strings.TrimSpace(input.Title) != "" ||
		strings.TrimSpace(input.Description) != "" ||
		strings.TrimSpace(input.Robots) != "" ||
		strings.TrimSpace(input.OpenGraphTitle) != "" ||
		strings.TrimSpace(input.OpenGraphSummary) != "" ||
		strings.TrimSpace(input.OpenGraphImage) != ""
}

func seoSnapshotJSON(input SEOInput, canonicalURL string) (string, error) {
	raw, err := json.Marshal(map[string]any{
		"title":        input.Title,
		"description":  input.Description,
		"canonicalUrl": canonicalURL,
		"robots":       input.Robots,
		"openGraph": map[string]any{
			"title":       input.OpenGraphTitle,
			"description": input.OpenGraphSummary,
			"image":       input.OpenGraphImage,
		},
		"structuredData": []any{},
	})
	return string(raw), err
}

func copySEOSnapshotJSON(raw, fallbackTitle, fallbackDescription, canonicalURL string) (string, error) {
	input := seoInputFromSnapshot(raw, fallbackTitle, fallbackDescription)
	if err := validateSEOInput(input); err != nil {
		return "", err
	}
	return seoSnapshotJSON(input, canonicalURL)
}

func seoInputFromSnapshot(raw, fallbackTitle, fallbackDescription string) SEOInput {
	seo := decodeJSONObject(raw)
	return normalizeSEOInput(SEOInput{
		Title:            stringFromMap(seo, "title", fallbackTitle),
		Description:      stringFromMap(seo, "description", fallbackDescription),
		Robots:           stringFromMap(seo, "robots", "index,follow"),
		OpenGraphTitle:   stringFromMap(decodeMapValue(seo, "openGraph"), "title", ""),
		OpenGraphSummary: stringFromMap(decodeMapValue(seo, "openGraph"), "description", ""),
		OpenGraphImage:   stringFromMap(decodeMapValue(seo, "openGraph"), "image", ""),
	}, fallbackTitle, fallbackDescription)
}

func decodeMapValue(parent map[string]any, key string) map[string]any {
	value, _ := parent[key].(map[string]any)
	return value
}

type revisionContributorRecord struct {
	AuthorID           string
	Role               string
	Position           int
	Author             Author
	PublicSnapshotJSON string
}

type revisionAttribution struct {
	Records                 []revisionContributorRecord
	AuthorSnapshotJSON      string
	ContributorSnapshotJSON string
}

var contributorRoleOrder = map[string]int{
	"primary_author":  0,
	"co_author":       1,
	"editor":          2,
	"expert_reviewer": 3,
	"photographer":    4,
	"other":           5,
}

func buildRevisionAttribution(
	ctx context.Context,
	tx *sql.Tx,
	projectID, baseRevisionID string,
	input []RevisionContributorInput,
) (revisionAttribution, error) {
	if input == nil && baseRevisionID != "" {
		return inheritRevisionAttribution(ctx, tx, projectID, baseRevisionID)
	}
	if len(input) == 0 {
		return revisionAttribution{
			Records:                 []revisionContributorRecord{},
			AuthorSnapshotJSON:      "[]",
			ContributorSnapshotJSON: "[]",
		}, nil
	}

	normalized, err := normalizeRevisionContributorInputs(input)
	if err != nil {
		return revisionAttribution{}, err
	}
	records := make([]revisionContributorRecord, 0, len(normalized))
	for _, contributor := range normalized {
		row := tx.QueryRowContext(ctx, `
			SELECT `+authorColumns+`
			FROM authors
			WHERE project_id = ? AND id = ? AND status = 'active'
		`, projectID, contributor.AuthorID)
		author, err := scanAuthor(row)
		if errors.Is(err, sql.ErrNoRows) {
			return revisionAttribution{}, fmt.Errorf("%w: contributor author %q must be active in the selected project", ErrValidation, contributor.AuthorID)
		}
		if err != nil {
			return revisionAttribution{}, err
		}
		publicSnapshot, err := json.Marshal(author)
		if err != nil {
			return revisionAttribution{}, err
		}
		records = append(records, revisionContributorRecord{
			AuthorID:           contributor.AuthorID,
			Role:               contributor.Role,
			Position:           contributor.Position,
			Author:             author,
			PublicSnapshotJSON: string(publicSnapshot),
		})
	}
	return attributionFromRecords(records)
}

func normalizeRevisionContributorInputs(input []RevisionContributorInput) ([]RevisionContributorInput, error) {
	normalized := make([]RevisionContributorInput, 0, len(input))
	seenAssignments := map[string]struct{}{}
	seenPositions := map[string]struct{}{}
	primaryAuthors := 0
	for _, contributor := range input {
		contributor.AuthorID = strings.TrimSpace(contributor.AuthorID)
		contributor.Role = strings.ToLower(strings.TrimSpace(contributor.Role))
		if contributor.AuthorID == "" {
			return nil, fmt.Errorf("%w: contributor authorId is required", ErrValidation)
		}
		if _, supported := contributorRoleOrder[contributor.Role]; !supported {
			return nil, fmt.Errorf("%w: unsupported contributor role %q", ErrValidation, contributor.Role)
		}
		if contributor.Position < 0 {
			return nil, fmt.Errorf("%w: contributor position cannot be negative", ErrValidation)
		}
		if contributor.Role == "primary_author" {
			primaryAuthors++
			if contributor.Position != 0 {
				return nil, fmt.Errorf("%w: primary author position must be zero", ErrValidation)
			}
		}
		assignmentKey := contributor.AuthorID + "\x00" + contributor.Role
		if _, duplicate := seenAssignments[assignmentKey]; duplicate {
			return nil, fmt.Errorf("%w: contributor role is duplicated for author %q", ErrValidation, contributor.AuthorID)
		}
		seenAssignments[assignmentKey] = struct{}{}
		positionKey := contributor.Role + "\x00" + fmt.Sprint(contributor.Position)
		if _, duplicate := seenPositions[positionKey]; duplicate {
			return nil, fmt.Errorf("%w: contributor position %d is duplicated for role %q", ErrValidation, contributor.Position, contributor.Role)
		}
		seenPositions[positionKey] = struct{}{}
		normalized = append(normalized, contributor)
	}
	if primaryAuthors != 1 {
		return nil, fmt.Errorf("%w: contributors must include exactly one primary author", ErrValidation)
	}
	sort.SliceStable(normalized, func(left, right int) bool {
		leftRank := contributorRoleOrder[normalized[left].Role]
		rightRank := contributorRoleOrder[normalized[right].Role]
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if normalized[left].Position != normalized[right].Position {
			return normalized[left].Position < normalized[right].Position
		}
		return normalized[left].AuthorID < normalized[right].AuthorID
	})
	return normalized, nil
}

func inheritRevisionAttribution(
	ctx context.Context,
	tx *sql.Tx,
	projectID, baseRevisionID string,
) (revisionAttribution, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT author_id, role, position, public_snapshot_json
		FROM revision_contributors
		WHERE project_id = ? AND revision_id = ?
	`, projectID, baseRevisionID)
	if err != nil {
		return revisionAttribution{}, err
	}
	defer rows.Close()

	records := []revisionContributorRecord{}
	for rows.Next() {
		var record revisionContributorRecord
		if err := rows.Scan(&record.AuthorID, &record.Role, &record.Position, &record.PublicSnapshotJSON); err != nil {
			return revisionAttribution{}, err
		}
		if err := json.Unmarshal([]byte(record.PublicSnapshotJSON), &record.Author); err != nil {
			return revisionAttribution{}, fmt.Errorf("decode inherited contributor snapshot: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return revisionAttribution{}, err
	}
	if len(records) > 0 {
		return attributionFromRecords(records)
	}

	var authorSnapshotJSON, contributorSnapshotJSON string
	if err := tx.QueryRowContext(ctx, `
		SELECT author_snapshot_json, contributor_snapshot_json
		FROM content_revisions
		WHERE project_id = ? AND id = ?
	`, projectID, baseRevisionID).Scan(&authorSnapshotJSON, &contributorSnapshotJSON); err != nil {
		return revisionAttribution{}, err
	}
	return revisionAttribution{
		Records:                 records,
		AuthorSnapshotJSON:      authorSnapshotJSON,
		ContributorSnapshotJSON: contributorSnapshotJSON,
	}, nil
}

func attributionFromRecords(records []revisionContributorRecord) (revisionAttribution, error) {
	sort.SliceStable(records, func(left, right int) bool {
		leftRank := contributorRoleOrder[records[left].Role]
		rightRank := contributorRoleOrder[records[right].Role]
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if records[left].Position != records[right].Position {
			return records[left].Position < records[right].Position
		}
		return records[left].AuthorID < records[right].AuthorID
	})
	authors := []Author{}
	contributors := []Contributor{}
	for _, record := range records {
		switch record.Role {
		case "primary_author", "co_author":
			authors = append(authors, record.Author)
		default:
			contributors = append(contributors, Contributor{
				Author:   record.Author,
				Role:     record.Role,
				Position: record.Position,
			})
		}
	}
	authorSnapshotJSON, err := json.Marshal(authors)
	if err != nil {
		return revisionAttribution{}, err
	}
	contributorSnapshotJSON, err := json.Marshal(contributors)
	if err != nil {
		return revisionAttribution{}, err
	}
	return revisionAttribution{
		Records:                 records,
		AuthorSnapshotJSON:      string(authorSnapshotJSON),
		ContributorSnapshotJSON: string(contributorSnapshotJSON),
	}, nil
}

func insertRevisionContributors(
	ctx context.Context,
	tx *sql.Tx,
	projectID, revisionID string,
	records []revisionContributorRecord,
) error {
	for _, record := range records {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO revision_contributors(
			  project_id, revision_id, author_id, role, position, public_snapshot_json
			) VALUES (?, ?, ?, ?, ?, ?)
		`, projectID, revisionID, record.AuthorID, record.Role, record.Position, record.PublicSnapshotJSON); err != nil {
			return err
		}
	}
	return nil
}

func revisionContentHash(
	title, html, bodyJSON, taxonomyJSON, seoJSON, authorJSON, contributorJSON string,
) (string, error) {
	raw, err := json.Marshal(map[string]string{
		"title":        title,
		"html":         html,
		"body":         bodyJSON,
		"taxonomy":     taxonomyJSON,
		"seo":          seoJSON,
		"authors":      authorJSON,
		"contributors": contributorJSON,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
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
