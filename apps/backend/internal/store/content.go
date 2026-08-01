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
	ci.id, ci.article_type, pp.slug, cr.revision_number, cr.title,
	COALESCE(cr.deck, ''), COALESCE(cr.excerpt, ''), COALESCE(cr.short_answer, ''),
	cr.body_document_json, cr.sanitized_html, cr.table_of_contents_json,
	cr.seo_snapshot_json, cr.taxonomy_snapshot_json, cr.author_snapshot_json,
	cr.contributor_snapshot_json, cr.source_snapshot_json, cr.claim_snapshot_json,
	cr.media_snapshot_json, ` + publishedDisclosureJSON + `, ` + publishedCorrectionsJSON + `,
	pp.canonical_url, pp.robots_directive, cr.content_hash,
	COALESCE(pp.first_published_at, ''), COALESCE(pp.materially_modified_at, ''),
	COALESCE(pp.first_published_at, pp.created_at),
	COALESCE(p.publisher_name, p.name), COALESCE(p.publisher_url, '')
`

func (s *Store) ListPublishedPosts(
	ctx context.Context,
	projectID, category, tag, author, articleType, seriesSlug string,
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
		JOIN projects p ON p.id = pp.project_id
		JOIN content_revisions cr
		  ON cr.project_id = pp.project_id AND cr.content_id = pp.content_id AND cr.id = pp.published_revision_id
		WHERE pp.project_id = ?
		  AND pp.publication_state = 'published'
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range posts {
		if err := s.hydratePublishedRelationships(ctx, projectID, &posts[index]); err != nil {
			return nil, err
		}
	}
	return posts, nil
}

func (s *Store) GetPublishedPostBySlug(ctx context.Context, projectID, slug string) (PublishedPost, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+publishedPostColumns+`
		FROM project_publications pp
		JOIN content_items ci
		  ON ci.project_id = pp.project_id AND ci.id = pp.content_id
		JOIN projects p ON p.id = pp.project_id
		JOIN content_revisions cr
		  ON cr.project_id = pp.project_id AND cr.content_id = pp.content_id AND cr.id = pp.published_revision_id
		WHERE pp.project_id = ?
		  AND pp.slug = ?
		  AND pp.publication_state = 'published'
	`, projectID, slug)
	post, err := scanPost(row, nil)
	if err != nil {
		return PublishedPost{}, err
	}
	err = s.hydratePublishedRelationships(ctx, projectID, &post)
	return post, err
}

func (s *Store) GetPublishedPostByID(ctx context.Context, projectID, contentID string) (PublishedPost, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+publishedPostColumns+`
		FROM project_publications pp
		JOIN content_items ci
		  ON ci.project_id = pp.project_id AND ci.id = pp.content_id
		JOIN projects p ON p.id = pp.project_id
		JOIN content_revisions cr
		  ON cr.project_id = pp.project_id AND cr.content_id = pp.content_id AND cr.id = pp.published_revision_id
		WHERE pp.project_id = ?
		  AND pp.content_id = ?
		  AND pp.publication_state = 'published'
	`, projectID, contentID)
	post, err := scanPost(row, nil)
	if err != nil {
		return PublishedPost{}, err
	}
	err = s.hydratePublishedRelationships(ctx, projectID, &post)
	return post, err
}

func (s *Store) ListRelatedPosts(ctx context.Context, projectID, slug string, limit int) ([]RelatedPost, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH source AS (
			SELECT content_id
			FROM project_publications
			WHERE project_id = ? AND slug = ? AND publication_state = 'published'
		)
		SELECT `+publishedPostColumns+`, rel.origin
		FROM content_relationships rel
		JOIN source ON source.content_id = rel.source_content_id
		JOIN project_publications pp
		  ON pp.project_id = rel.project_id
		 AND pp.content_id = rel.target_content_id
		JOIN content_items ci
		  ON ci.project_id = pp.project_id AND ci.id = pp.content_id
		JOIN projects p ON p.id = pp.project_id
		JOIN content_revisions cr
		  ON cr.project_id = pp.project_id AND cr.content_id = pp.content_id AND cr.id = pp.published_revision_id
		WHERE rel.project_id = ?
		  AND rel.relationship_type = 'related'
		  AND pp.publication_state = 'published'
		ORDER BY rel.position ASC
		LIMIT ?
	`, projectID, slug, projectID, limit)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range related {
		if err := s.hydratePublishedRelationships(ctx, projectID, &related[index].Post); err != nil {
			return nil, err
		}
	}
	return related, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPost(row rowScanner, relationshipOrigin *string) (PublishedPost, error) {
	var post PublishedPost
	var bodyJSON, tocJSON, seoJSON, taxonomyJSON, authorsJSON, contributorsJSON string
	var sourcesJSON, claimsJSON, mediaJSON, disclosuresJSON, correctionsJSON string
	dest := []any{
		&post.ID, &post.ArticleType, &post.Slug, &post.Revision,
		&post.Title, &post.Deck, &post.Excerpt, &post.ShortAnswer,
		&bodyJSON, &post.Content.HTML, &tocJSON,
		&seoJSON, &taxonomyJSON, &authorsJSON, &contributorsJSON,
		&sourcesJSON, &claimsJSON, &mediaJSON, &disclosuresJSON, &correctionsJSON,
		&post.SEO.CanonicalURL, &post.SEO.Robots, &post.ContentHash,
		&post.PublishedAt, &post.ModifiedAt, &post.PaginationKey,
		&post.PublisherName, &post.PublisherURL,
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
	post.Media = publishedMediaFromSnapshot(mediaJSON)
	post.Disclosures = decodeJSON(disclosuresJSON, []any{})
	post.Corrections = decodeJSON(correctionsJSON, []any{})
	post.RelatedArticles = []PublishedArticleLink{}
	post.TopicRelationships = []PublishedArticleLink{}
	return post, nil
}

func (s *Store) hydratePublishedRelationships(ctx context.Context, projectID string, post *PublishedPost) error {
	if post == nil {
		return nil
	}
	series, err := s.publishedSeriesMembership(ctx, projectID, post.ID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil {
		post.Taxonomy.Series = &series
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT target.id, target_publication.slug, target_revision.title,
		       COALESCE(target_revision.excerpt, ''), target_publication.canonical_url,
		       relationship.relationship_type, relationship.origin, relationship.position
		FROM content_relationships relationship
		JOIN project_publications target_publication
		  ON target_publication.project_id = relationship.project_id
		 AND target_publication.content_id = relationship.target_content_id
		 AND target_publication.publication_state = 'published'
		JOIN content_items target
		  ON target.project_id = target_publication.project_id
		 AND target.id = target_publication.content_id
		JOIN content_revisions target_revision
		  ON target_revision.project_id = target_publication.project_id
		 AND target_revision.content_id = target_publication.content_id
		 AND target_revision.id = target_publication.published_revision_id
		WHERE relationship.project_id = ? AND relationship.source_content_id = ?
		  AND relationship.relationship_type IN ('related', 'pillar', 'cluster')
		ORDER BY relationship.position, target.id
	`, projectID, post.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var link PublishedArticleLink
		if err := rows.Scan(
			&link.Article.ID, &link.Article.Slug, &link.Article.Title,
			&link.Article.Excerpt, &link.Article.CanonicalURL,
			&link.RelationshipType, &link.Origin, &link.Position,
		); err != nil {
			return err
		}
		if link.RelationshipType == "related" {
			post.RelatedArticles = append(post.RelatedArticles, link)
		} else {
			post.TopicRelationships = append(post.TopicRelationships, link)
		}
	}
	return rows.Err()
}

func (s *Store) publishedSeriesMembership(ctx context.Context, projectID, contentID string) (Series, error) {
	var series Series
	var indexability string
	var previousID, previousSlug, previousTitle, previousExcerpt, previousCanonical string
	var nextID, nextSlug, nextTitle, nextExcerpt, nextCanonical string
	err := s.db.QueryRowContext(ctx, `
		SELECT series.id, series.slug, series.name, COALESCE(series.description, ''), series.indexability,
		       membership.position,
		       COALESCE(previous_item.id, ''), COALESCE(previous_publication.slug, ''),
		       COALESCE(previous_revision.title, ''), COALESCE(previous_revision.excerpt, ''),
		       COALESCE(previous_publication.canonical_url, ''),
		       COALESCE(next_item.id, ''), COALESCE(next_publication.slug, ''),
		       COALESCE(next_revision.title, ''), COALESCE(next_revision.excerpt, ''),
		       COALESCE(next_publication.canonical_url, '')
		FROM series_articles membership
		JOIN series ON series.project_id = membership.project_id AND series.id = membership.series_id
		LEFT JOIN series_articles previous_membership
		  ON previous_membership.project_id = membership.project_id
		 AND previous_membership.series_id = membership.series_id
		 AND previous_membership.position = membership.position - 1
		LEFT JOIN project_publications previous_publication
		  ON previous_publication.project_id = previous_membership.project_id
		 AND previous_publication.content_id = previous_membership.content_id
		 AND previous_publication.publication_state = 'published'
		LEFT JOIN content_items previous_item
		  ON previous_item.project_id = previous_publication.project_id AND previous_item.id = previous_publication.content_id
		LEFT JOIN content_revisions previous_revision
		  ON previous_revision.project_id = previous_publication.project_id
		 AND previous_revision.content_id = previous_publication.content_id
		 AND previous_revision.id = previous_publication.published_revision_id
		LEFT JOIN series_articles next_membership
		  ON next_membership.project_id = membership.project_id
		 AND next_membership.series_id = membership.series_id
		 AND next_membership.position = membership.position + 1
		LEFT JOIN project_publications next_publication
		  ON next_publication.project_id = next_membership.project_id
		 AND next_publication.content_id = next_membership.content_id
		 AND next_publication.publication_state = 'published'
		LEFT JOIN content_items next_item
		  ON next_item.project_id = next_publication.project_id AND next_item.id = next_publication.content_id
		LEFT JOIN content_revisions next_revision
		  ON next_revision.project_id = next_publication.project_id
		 AND next_revision.content_id = next_publication.content_id
		 AND next_revision.id = next_publication.published_revision_id
		WHERE membership.project_id = ? AND membership.content_id = ?
		ORDER BY series.id
		LIMIT 1
	`, projectID, contentID).Scan(
		&series.ID, &series.Slug, &series.Name, &series.Description, &indexability, &series.Position,
		&previousID, &previousSlug, &previousTitle, &previousExcerpt, &previousCanonical,
		&nextID, &nextSlug, &nextTitle, &nextExcerpt, &nextCanonical,
	)
	if err != nil {
		return Series{}, err
	}
	series.Indexable = indexability == "index"
	if previousID != "" {
		series.Previous = &PublishedArticleSummary{ID: previousID, Slug: previousSlug, Title: previousTitle, Excerpt: previousExcerpt, CanonicalURL: previousCanonical}
	}
	if nextID != "" {
		series.Next = &PublishedArticleSummary{ID: nextID, Slug: nextSlug, Title: nextTitle, Excerpt: nextExcerpt, CanonicalURL: nextCanonical}
	}
	return series, nil
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

func (s *Store) ListDiscovery(ctx context.Context, projectID string) ([]DiscoveryEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT content_id, canonical_url,
		       COALESCE(materially_modified_at, first_published_at, updated_at)
		FROM project_publications
		WHERE project_id = ?
		  AND publication_state = 'published'
		  AND robots_directive NOT LIKE '%noindex%'
		ORDER BY canonical_url
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DiscoveryEntry
	for rows.Next() {
		var entry DiscoveryEntry
		if err := rows.Scan(&entry.ID, &entry.CanonicalURL, &entry.LastModified); err != nil {
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
