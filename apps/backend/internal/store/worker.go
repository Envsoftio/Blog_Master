package store

import (
	"context"
)

type duePublication struct {
	ID             string
	ProjectID      string
	ContentID      string
	RevisionID     string
	CanonicalURL   string
	CurrentVersion int64
}

// PublishDueSchedules atomically publishes valid due schedules and appends
// durable outbox events. Invalid schedules remain visible as scheduled instead
// of being partially published.
func (s *Store) PublishDueSchedules(ctx context.Context, batchSize int) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT publication.id, publication.project_id, publication.content_id,
		       publication.published_revision_id, publication.canonical_url,
		       publication.publication_version
		FROM project_publications publication
		JOIN content_revisions revision
		  ON revision.project_id = publication.project_id
		 AND revision.content_id = publication.content_id
		 AND revision.id = publication.published_revision_id
		 AND revision.editorial_state = 'approved'
		WHERE publication.publication_state = 'scheduled'
		  AND publication.scheduled_for_utc <= CURRENT_TIMESTAMP
		  AND 1 = (
		    SELECT COUNT(*)
		    FROM article_taxonomy assignment
		    JOIN taxonomy_terms term
		      ON term.project_id = assignment.project_id
		     AND term.id = assignment.taxonomy_term_id
		    WHERE assignment.project_id = publication.project_id
		      AND assignment.content_id = publication.content_id
		      AND assignment.is_primary = 1
		      AND term.type = 'category'
		  )
		ORDER BY publication.scheduled_for_utc, publication.id
		LIMIT ?
	`, batchSize)
	if err != nil {
		return 0, err
	}

	var due []duePublication
	for rows.Next() {
		var item duePublication
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.ContentID, &item.RevisionID, &item.CanonicalURL, &item.CurrentVersion); err != nil {
			rows.Close()
			return 0, err
		}
		due = append(due, item)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	published := 0
	for _, item := range due {
		version := item.CurrentVersion + 1
		result, err := tx.ExecContext(ctx, `
			UPDATE project_publications
			SET publication_state = 'published',
			    first_published_at = COALESCE(first_published_at, CURRENT_TIMESTAMP),
			    materially_modified_at = CURRENT_TIMESTAMP,
			    scheduled_for_utc = NULL,
			    publication_version = ?,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND project_id = ? AND publication_state = 'scheduled'
		`, version, item.ID, item.ProjectID)
		if err != nil {
			return 0, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if affected == 0 {
			continue
		}
		if err := incrementProjectGeneration(ctx, tx, item.ProjectID); err != nil {
			return 0, err
		}
		if err := insertPublicationOutbox(ctx, tx, item.ProjectID, item.ContentID, item.RevisionID, "content.published", item.CanonicalURL, version); err != nil {
			return 0, err
		}
		published++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return published, nil
}
