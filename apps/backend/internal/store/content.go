package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

type PublishedCursor struct {
	SortAt string `json:"sortAt"`
	ID     string `json:"id"`
}

type ChangeCursor struct {
	CreatedAt string `json:"createdAt"`
	ID        string `json:"id"`
}

const publishedDisclosureJSON = `
	COALESCE((
		SELECT json_group_array(json_object(
			'id', disclosure.id,
			'projectId', disclosure.project_id,
			'articleId', disclosure.content_id,
			'revisionId', COALESCE(disclosure.revision_id, ''),
			'disclosureType', disclosure.disclosure_type,
			'publicText', disclosure.public_text,
			'createdBy', disclosure.created_by,
			'createdAt', disclosure.created_at
		))
		FROM (
			SELECT *
			FROM disclosures
			WHERE project_id = pp.project_id
			  AND content_id = pp.content_id
			ORDER BY created_at, id
		) disclosure
	), cr.disclosure_snapshot_json)
`

const publishedCorrectionsJSON = `
	COALESCE((
		SELECT json_group_array(json_object(
			'id', correction.id,
			'projectId', correction.project_id,
			'articleId', correction.content_id,
			'affectedRevisionId', COALESCE(correction.affected_revision_id, ''),
			'publicNote', correction.public_note,
			'correctedBy', correction.corrected_by,
			'correctedAt', correction.corrected_at,
			'supersedesNoticeId', COALESCE(correction.supersedes_notice_id, '')
		))
		FROM (
			SELECT *
			FROM correction_notices
			WHERE project_id = pp.project_id
			  AND content_id = pp.content_id
			ORDER BY corrected_at, id
		) correction
	), cr.correction_summary_json)
`

const publishedPostColumns = `
	ci.id, ci.article_type, pp.slug, pp.locale, cr.revision_number, cr.title,
	COALESCE(cr.deck, ''), COALESCE(cr.excerpt, ''), COALESCE(cr.short_answer, ''),
	cr.body_document_json, cr.sanitized_html, cr.table_of_contents_json,
	cr.seo_snapshot_json, cr.taxonomy_snapshot_json, cr.author_snapshot_json,
	cr.contributor_snapshot_json, cr.source_snapshot_json, cr.claim_snapshot_json,
	cr.media_snapshot_json, ` + publishedDisclosureJSON + `, ` + publishedCorrectionsJSON + `,
	pp.canonical_url, pp.robots_directive, cr.content_hash,
	COALESCE(pp.first_published_at, ''), COALESCE(pp.materially_modified_at, ''),
	COALESCE(pp.first_published_at, pp.created_at)
`

func (s *Store) ListPublishedPosts(
	ctx context.Context,
	projectID, locale, category, tag, author, articleType, seriesSlug string,
	exactCategory bool,
	publishedFrom, publishedTo string,
	cursor PublishedCursor,
	limit int,
) ([]PublishedPost, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH RECURSIVE category_tree(id) AS (
			SELECT id
			FROM taxonomy_terms
			WHERE project_id = ? AND type = 'category' AND slug = ?
			UNION
			SELECT child.id
			FROM taxonomy_terms child
			JOIN category_tree parent ON parent.id = child.parent_id
			WHERE child.project_id = ? AND child.type = 'category'
		)
		SELECT `+publishedPostColumns+`
		FROM project_publications pp
		JOIN content_items ci
		  ON ci.project_id = pp.project_id AND ci.id = pp.content_id
		JOIN content_revisions cr
		  ON cr.project_id = pp.project_id AND cr.content_id = pp.content_id AND cr.id = pp.published_revision_id
		WHERE pp.project_id = ?
		  AND pp.publication_state = 'published'
		  AND (? = '' OR pp.locale = ?)
		  AND (? = '' OR ci.article_type = ?)
		  AND (? = '' OR EXISTS (
		      SELECT 1
		      FROM article_taxonomy at
		      JOIN taxonomy_terms assigned
		        ON assigned.project_id = at.project_id
		       AND assigned.id = at.taxonomy_term_id
		      WHERE at.project_id = ci.project_id
		        AND at.content_id = ci.id
		        AND (
		          (? = 1 AND assigned.type = 'category' AND assigned.slug = ?)
		          OR
		          (? = 0 AND at.taxonomy_term_id IN (SELECT id FROM category_tree))
		        )
		  ))
		  AND (? = '' OR EXISTS (
		      SELECT 1
		      FROM article_taxonomy at
		      JOIN taxonomy_terms tt
		        ON tt.project_id = at.project_id AND tt.id = at.taxonomy_term_id
		      WHERE at.project_id = ci.project_id
		        AND at.content_id = ci.id
		        AND tt.type = 'tag'
		        AND tt.slug = ?
		  ))
		  AND (? = '' OR EXISTS (
		      SELECT 1
		      FROM revision_contributors rc
		      WHERE rc.project_id = ci.project_id
		        AND rc.revision_id = cr.id
		        AND json_extract(rc.public_snapshot_json, '$.slug') = ?
		  ))
		  AND (? = '' OR EXISTS (
		      SELECT 1
		      FROM series_articles sa
		      JOIN series series_filter
		        ON series_filter.project_id = sa.project_id AND series_filter.id = sa.series_id
		      WHERE sa.project_id = ci.project_id
		        AND sa.content_id = ci.id
		        AND series_filter.slug = ?
		  ))
		  AND (? = '' OR pp.first_published_at >= ?)
		  AND (? = '' OR pp.first_published_at <= ?)
		  AND (
		    ? = '' OR
		    COALESCE(pp.first_published_at, pp.created_at) < ? OR
		    (COALESCE(pp.first_published_at, pp.created_at) = ? AND ci.id < ?)
		  )
		ORDER BY COALESCE(pp.first_published_at, pp.created_at) DESC, ci.id DESC
		LIMIT ?
	`,
		projectID, category, projectID,
		projectID,
		locale, locale,
		articleType, articleType,
		category, exactCategory, category, exactCategory,
		tag, tag,
		author, author,
		seriesSlug, seriesSlug,
		publishedFrom, publishedFrom,
		publishedTo, publishedTo,
		cursor.SortAt, cursor.SortAt, cursor.SortAt, cursor.ID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []PublishedPost
	for rows.Next() {
		post, err := scanPost(rows, nil)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

func (s *Store) GetPublishedPostBySlug(ctx context.Context, projectID, slug, locale string) (PublishedPost, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+publishedPostColumns+`
		FROM project_publications pp
		JOIN content_items ci
		  ON ci.project_id = pp.project_id AND ci.id = pp.content_id
		JOIN content_revisions cr
		  ON cr.project_id = pp.project_id AND cr.content_id = pp.content_id AND cr.id = pp.published_revision_id
		WHERE pp.project_id = ?
		  AND pp.slug = ?
		  AND pp.locale = ?
		  AND pp.publication_state = 'published'
	`, projectID, slug, locale)
	return scanPost(row, nil)
}

func (s *Store) GetPublishedPostByID(ctx context.Context, projectID, contentID, locale string) (PublishedPost, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+publishedPostColumns+`
		FROM project_publications pp
		JOIN content_items ci
		  ON ci.project_id = pp.project_id AND ci.id = pp.content_id
		JOIN content_revisions cr
		  ON cr.project_id = pp.project_id AND cr.content_id = pp.content_id AND cr.id = pp.published_revision_id
		WHERE pp.project_id = ?
		  AND pp.content_id = ?
		  AND pp.locale = ?
		  AND pp.publication_state = 'published'
	`, projectID, contentID, locale)
	return scanPost(row, nil)
}

func (s *Store) ListRelatedPosts(ctx context.Context, projectID, slug, locale string, limit int) ([]RelatedPost, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH source AS (
			SELECT content_id, locale
			FROM project_publications
			WHERE project_id = ? AND slug = ? AND locale = ? AND publication_state = 'published'
		)
		SELECT `+publishedPostColumns+`, rel.origin
		FROM content_relationships rel
		JOIN source ON source.content_id = rel.source_content_id
		JOIN project_publications pp
		  ON pp.project_id = rel.project_id
		 AND pp.content_id = rel.target_content_id
		 AND pp.locale = source.locale
		JOIN content_items ci
		  ON ci.project_id = pp.project_id AND ci.id = pp.content_id
		JOIN content_revisions cr
		  ON cr.project_id = pp.project_id AND cr.content_id = pp.content_id AND cr.id = pp.published_revision_id
		WHERE rel.project_id = ?
		  AND rel.relationship_type = 'related'
		  AND pp.publication_state = 'published'
		ORDER BY rel.position ASC
		LIMIT ?
	`, projectID, slug, locale, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var related []RelatedPost
	for rows.Next() {
		var origin string
		post, err := scanPost(rows, &origin)
		if err != nil {
			return nil, err
		}
		related = append(related, RelatedPost{Post: post, Origin: origin})
	}
	return related, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPost(row rowScanner, relationshipOrigin *string) (PublishedPost, error) {
	var post PublishedPost
	var bodyJSON, tocJSON, seoJSON, taxonomyJSON, authorsJSON, contributorsJSON string
	var sourcesJSON, claimsJSON, mediaJSON, disclosuresJSON, correctionsJSON string
	dest := []any{
		&post.ID, &post.ArticleType, &post.Slug, &post.Locale, &post.Revision,
		&post.Title, &post.Deck, &post.Excerpt, &post.ShortAnswer,
		&bodyJSON, &post.Content.HTML, &tocJSON,
		&seoJSON, &taxonomyJSON, &authorsJSON, &contributorsJSON,
		&sourcesJSON, &claimsJSON, &mediaJSON, &disclosuresJSON, &correctionsJSON,
		&post.SEO.CanonicalURL, &post.SEO.Robots, &post.ContentHash,
		&post.PublishedAt, &post.ModifiedAt, &post.PaginationKey,
	}
	if relationshipOrigin != nil {
		dest = append(dest, relationshipOrigin)
	}
	if err := row.Scan(dest...); err != nil {
		return post, err
	}

	post.Content.Format = "tiptap-json"
	post.Content.Document = decodeJSON(bodyJSON, map[string]any{})
	post.Content.TableOfContents = decodeJSON(tocJSON, []any{})

	seo := decodeJSONObject(seoJSON)
	post.SEO.Title = stringFromMap(seo, "title", post.Title)
	post.SEO.Description = stringFromMap(seo, "description", post.Excerpt)
	post.SEO.Index = !strings.Contains(strings.ToLower(post.SEO.Robots), "noindex")
	post.SEO.OpenGraph = mapValue(seo, "openGraph", map[string]any{})
	post.SEO.StructuredData = mapValue(seo, "structuredData", []any{})
	post.SEO.Hreflang = mapValue(seo, "hreflang", []any{})

	post.Taxonomy = PublishedTaxonomy{
		Categories: []TaxonomyTerm{},
		Tags:       []TaxonomyTerm{},
		Topics:     []TaxonomyTerm{},
	}
	decodeInto(taxonomyJSON, &post.Taxonomy)
	post.Authors = []Author{}
	decodeInto(authorsJSON, &post.Authors)
	post.Contributors = []Contributor{}
	decodeInto(contributorsJSON, &post.Contributors)
	post.Sources = decodeJSON(sourcesJSON, []any{})
	post.Claims = decodeJSON(claimsJSON, []any{})
	post.Media = decodeJSON(mediaJSON, map[string]any{})
	post.Disclosures = decodeJSON(disclosuresJSON, []any{})
	post.Corrections = decodeJSON(correctionsJSON, []any{})
	return post, nil
}

func (s *Store) ListTerms(ctx context.Context, projectID, termType string) ([]TaxonomyTerm, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, slug, name, COALESCE(description, ''), COALESCE(parent_id, ''), indexability
		FROM taxonomy_terms
		WHERE project_id = ? AND type = ? AND status = 'active'
		ORDER BY name, id
	`, projectID, termType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var terms []TaxonomyTerm
	for rows.Next() {
		term, err := scanTerm(rows)
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hydrateTermHierarchy(terms), nil
}

func (s *Store) GetTerm(ctx context.Context, projectID, termType, slug string) (TaxonomyTerm, error) {
	terms, err := s.ListTerms(ctx, projectID, termType)
	if err != nil {
		return TaxonomyTerm{}, err
	}
	for _, term := range terms {
		if term.Slug == slug {
			return term, nil
		}
	}
	return TaxonomyTerm{}, sql.ErrNoRows
}

func hydrateTermHierarchy(terms []TaxonomyTerm) []TaxonomyTerm {
	byID := make(map[string]TaxonomyTerm, len(terms))
	for _, term := range terms {
		byID[term.ID] = term
	}
	for index := range terms {
		if terms[index].ParentID != "" {
			if parent, ok := byID[terms[index].ParentID]; ok {
				parent.Children = append(parent.Children, terms[index])
				byID[parent.ID] = parent
			}
			current := terms[index].ParentID
			seen := map[string]bool{}
			for current != "" && !seen[current] {
				seen[current] = true
				parent, ok := byID[current]
				if !ok {
					break
				}
				terms[index].Ancestors = append([]TaxonomyTerm{withoutRelations(parent)}, terms[index].Ancestors...)
				current = parent.ParentID
			}
		}
	}
	for index := range terms {
		if current, ok := byID[terms[index].ID]; ok {
			terms[index].Children = current.Children
		}
	}
	return terms
}

func withoutRelations(term TaxonomyTerm) TaxonomyTerm {
	term.Ancestors = nil
	term.Children = nil
	return term
}

func scanTerm(row rowScanner) (TaxonomyTerm, error) {
	var term TaxonomyTerm
	var indexability string
	if err := row.Scan(&term.ID, &term.Type, &term.Slug, &term.Name, &term.Description, &term.ParentID, &indexability); err != nil {
		return term, err
	}
	term.Indexable = indexability == "index"
	return term, nil
}

func (s *Store) ListAuthors(ctx context.Context, projectID string) ([]Author, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+authorColumns+`
		FROM authors
		WHERE project_id = ? AND status = 'active'
		ORDER BY display_name, id
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authors []Author
	for rows.Next() {
		author, err := scanAuthor(rows)
		if err != nil {
			return nil, err
		}
		authors = append(authors, author)
	}
	return authors, rows.Err()
}

func (s *Store) GetAuthor(ctx context.Context, projectID, slug string) (Author, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+authorColumns+`
		FROM authors
		WHERE project_id = ? AND slug = ? AND status = 'active'
	`, projectID, slug)
	return scanAuthor(row)
}

const authorColumns = `
	id, slug, display_name, COALESCE(short_bio, ''), COALESCE(full_bio, ''),
	COALESCE(photo_asset_id, ''), COALESCE(job_title, ''), COALESCE(organization, ''),
	credentials_json, expertise_json, COALESCE(profile_url, ''),
	external_profiles_json, same_as_json, status, created_at, updated_at
`

func scanAuthor(row rowScanner) (Author, error) {
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

func (s *Store) ListSeries(ctx context.Context, projectID string) ([]Series, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, slug, name, COALESCE(description, ''), indexability
		FROM series
		WHERE project_id = ?
		ORDER BY name, id
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Series
	for rows.Next() {
		item, err := scanSeries(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetSeries(ctx context.Context, projectID, slug string) (Series, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, COALESCE(description, ''), indexability
		FROM series
		WHERE project_id = ? AND slug = ?
	`, projectID, slug)
	return scanSeries(row)
}

func scanSeries(row rowScanner) (Series, error) {
	var item Series
	var indexability string
	if err := row.Scan(&item.ID, &item.Slug, &item.Name, &item.Description, &indexability); err != nil {
		return item, err
	}
	item.Indexable = indexability == "index"
	return item, nil
}

func (s *Store) ListRedirects(ctx context.Context, projectID string) ([]RedirectRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_path, target_path, status_code
		FROM slug_redirects
		WHERE project_id = ?
		ORDER BY source_path
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var redirects []RedirectRecord
	for rows.Next() {
		var redirect RedirectRecord
		if err := rows.Scan(&redirect.SourcePath, &redirect.TargetPath, &redirect.StatusCode); err != nil {
			return nil, err
		}
		redirects = append(redirects, redirect)
	}
	return redirects, rows.Err()
}

func (s *Store) GetRedirect(ctx context.Context, projectID, sourcePath string) (RedirectRecord, error) {
	var redirect RedirectRecord
	err := s.db.QueryRowContext(ctx, `
		SELECT source_path, target_path, status_code
		FROM slug_redirects
		WHERE project_id = ? AND source_path = ?
	`, projectID, sourcePath).Scan(&redirect.SourcePath, &redirect.TargetPath, &redirect.StatusCode)
	return redirect, err
}

func (s *Store) ListChanges(ctx context.Context, projectID string, cursor ChangeCursor, limit int) ([]ChangeRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, event_type, aggregate_id, created_at
		FROM outbox_events
		WHERE project_id = ?
		  AND (
		    ? = '' OR
		    created_at > ? OR
		    (created_at = ? AND id > ?)
		  )
		ORDER BY created_at ASC, id ASC
		LIMIT ?
	`, projectID, cursor.CreatedAt, cursor.CreatedAt, cursor.CreatedAt, cursor.ID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var changes []ChangeRecord
	for rows.Next() {
		var change ChangeRecord
		if err := rows.Scan(&change.ID, &change.Type, &change.AggregateID, &change.CreatedAt); err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, rows.Err()
}

func (s *Store) ListDiscovery(ctx context.Context, projectID, locale string) ([]DiscoveryEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT content_id, locale, canonical_url,
		       COALESCE(materially_modified_at, first_published_at, updated_at)
		FROM project_publications
		WHERE project_id = ?
		  AND publication_state = 'published'
		  AND robots_directive NOT LIKE '%noindex%'
		  AND (? = '' OR locale = ?)
		ORDER BY canonical_url
	`, projectID, locale, locale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DiscoveryEntry
	for rows.Next() {
		var entry DiscoveryEntry
		if err := rows.Scan(&entry.ID, &entry.Locale, &entry.CanonicalURL, &entry.LastModified); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func decodeJSON(raw string, fallback any) any {
	var value any
	if raw == "" {
		return fallback
	}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return fallback
	}
	return value
}

func decodeJSONObject(raw string) map[string]any {
	var value map[string]any
	if raw == "" || json.Unmarshal([]byte(raw), &value) != nil || value == nil {
		return map[string]any{}
	}
	return value
}

func decodeInto(raw string, destination any) {
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), destination)
	}
}

func mapValue(values map[string]any, key string, fallback any) any {
	value, ok := values[key]
	if !ok || value == nil {
		return fallback
	}
	return value
}

func stringFromMap(values map[string]any, key, fallback string) string {
	value, ok := values[key].(string)
	if !ok || value == "" {
		return fallback
	}
	return value
}
