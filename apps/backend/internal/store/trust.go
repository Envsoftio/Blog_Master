package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"seoblog/apps/backend/internal/security"
)

type Source struct {
	ID                    string `json:"id"`
	ProjectID             string `json:"projectId"`
	Title                 string `json:"title"`
	Publisher             string `json:"publisher,omitempty"`
	Author                string `json:"author,omitempty"`
	URL                   string `json:"url,omitempty"`
	PublicationDate       string `json:"publicationDate,omitempty"`
	AccessedAt            string `json:"accessedAt,omitempty"`
	SourceType            string `json:"sourceType"`
	IsPrimary             bool   `json:"isPrimary"`
	ArchivedCopyReference string `json:"archivedCopyReference,omitempty"`
	Notes                 string `json:"notes,omitempty"`
	CreatedAt             string `json:"createdAt"`
}

type SourceInput struct {
	Title                 string
	Publisher             string
	Author                string
	URL                   string
	PublicationDate       string
	AccessedAt            string
	SourceType            string
	IsPrimary             bool
	ArchivedCopyReference string
	Notes                 string
}

type SourcePatch struct {
	Title                 *string
	Publisher             *string
	Author                *string
	URL                   *string
	PublicationDate       *string
	AccessedAt            *string
	SourceType            *string
	IsPrimary             *bool
	ArchivedCopyReference *string
	Notes                 *string
}

type Claim struct {
	ID                string   `json:"id"`
	ProjectID         string   `json:"projectId"`
	ArticleID         string   `json:"articleId"`
	RevisionID        string   `json:"revisionId"`
	ClaimText         string   `json:"claimText"`
	BlockID           string   `json:"blockId,omitempty"`
	Importance        string   `json:"importance"`
	VerificationState string   `json:"verificationState"`
	VerifiedBy        string   `json:"verifiedBy,omitempty"`
	VerifiedAt        string   `json:"verifiedAt,omitempty"`
	SourceIDs         []string `json:"sourceIds"`
}

type ClaimInput struct {
	ClaimText  string
	BlockID    string
	Importance string
	SourceIDs  []string
}

type ClaimVerificationInput struct {
	VerificationState string
	SourceIDs         *[]string
}

type Disclosure struct {
	ID             string `json:"id"`
	ProjectID      string `json:"projectId"`
	ArticleID      string `json:"articleId"`
	RevisionID     string `json:"revisionId,omitempty"`
	DisclosureType string `json:"disclosureType"`
	PublicText     string `json:"publicText"`
	CreatedBy      string `json:"createdBy"`
	CreatedAt      string `json:"createdAt"`
}

type DisclosureInput struct {
	RevisionID     string
	DisclosureType string
	PublicText     string
}

type CorrectionNotice struct {
	ID                 string `json:"id"`
	ProjectID          string `json:"projectId"`
	ArticleID          string `json:"articleId"`
	AffectedRevisionID string `json:"affectedRevisionId,omitempty"`
	PublicNote         string `json:"publicNote"`
	CorrectedBy        string `json:"correctedBy"`
	CorrectedAt        string `json:"correctedAt"`
	SupersedesNoticeID string `json:"supersedesNoticeId,omitempty"`
}

type CorrectionInput struct {
	AffectedRevisionID string
	PublicNote         string
	SupersedesNoticeID string
}

func (s *Store) ListSources(ctx context.Context, userID, projectID, cursor string, limit int) ([]Source, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, title, COALESCE(publisher, ''), COALESCE(author, ''),
		       COALESCE(url, ''), COALESCE(publication_date, ''), COALESCE(accessed_at, ''),
		       source_type, is_primary, COALESCE(archived_copy_reference, ''),
		       COALESCE(notes, ''), created_at
		FROM sources
		WHERE project_id = ?
		  AND (? = '' OR id > ?)
		ORDER BY id
		LIMIT ?
	`, projectID, cursor, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := []Source{}
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *Store) CreateSource(ctx context.Context, actorUserID, projectID string, input SourceInput) (Source, error) {
	if err := s.requireContentWrite(ctx, actorUserID, projectID); err != nil {
		return Source{}, err
	}
	input = applySourceDefaults(input)
	if err := validateSourceInput(input); err != nil {
		return Source{}, err
	}
	sourceID, err := security.RandomID("src")
	if err != nil {
		return Source{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Source{}, err
	}
	defer tx.Rollback()

	if err := requireActiveProjectTx(ctx, tx, projectID); err != nil {
		return Source{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sources(
		  id, project_id, title, publisher, author, url, publication_date,
		  accessed_at, source_type, is_primary, archived_copy_reference, notes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, sourceID, projectID, input.Title, nullIfEmpty(input.Publisher), nullIfEmpty(input.Author),
		nullIfEmpty(input.URL), nullIfEmpty(input.PublicationDate), nullIfEmpty(input.AccessedAt),
		input.SourceType, boolToInt(input.IsPrimary), nullIfEmpty(input.ArchivedCopyReference),
		nullIfEmpty(input.Notes)); err != nil {
		return Source{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "source.create", "source", sourceID, "success", map[string]string{
		"sourceType": input.SourceType,
	}); err != nil {
		return Source{}, err
	}
	if err := tx.Commit(); err != nil {
		return Source{}, err
	}
	return s.GetSource(ctx, actorUserID, projectID, sourceID)
}

func (s *Store) UpdateSource(ctx context.Context, actorUserID, projectID, sourceID string, patch SourcePatch) (Source, error) {
	if err := s.requireContentWrite(ctx, actorUserID, projectID); err != nil {
		return Source{}, err
	}
	current, err := s.GetSource(ctx, actorUserID, projectID, sourceID)
	if err != nil {
		return Source{}, err
	}
	next := SourceInput{
		Title:                 current.Title,
		Publisher:             current.Publisher,
		Author:                current.Author,
		URL:                   current.URL,
		PublicationDate:       current.PublicationDate,
		AccessedAt:            current.AccessedAt,
		SourceType:            current.SourceType,
		IsPrimary:             current.IsPrimary,
		ArchivedCopyReference: current.ArchivedCopyReference,
		Notes:                 current.Notes,
	}
	if patch.Title != nil {
		next.Title = *patch.Title
	}
	if patch.Publisher != nil {
		next.Publisher = *patch.Publisher
	}
	if patch.Author != nil {
		next.Author = *patch.Author
	}
	if patch.URL != nil {
		next.URL = *patch.URL
	}
	if patch.PublicationDate != nil {
		next.PublicationDate = *patch.PublicationDate
	}
	if patch.AccessedAt != nil {
		next.AccessedAt = *patch.AccessedAt
	}
	if patch.SourceType != nil {
		next.SourceType = *patch.SourceType
	}
	if patch.IsPrimary != nil {
		next.IsPrimary = *patch.IsPrimary
	}
	if patch.ArchivedCopyReference != nil {
		next.ArchivedCopyReference = *patch.ArchivedCopyReference
	}
	if patch.Notes != nil {
		next.Notes = *patch.Notes
	}
	next = applySourceDefaults(next)
	if err := validateSourceInput(next); err != nil {
		return Source{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Source{}, err
	}
	defer tx.Rollback()

	if err := requireActiveProjectTx(ctx, tx, projectID); err != nil {
		return Source{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE sources
		SET title = ?,
		    publisher = ?,
		    author = ?,
		    url = ?,
		    publication_date = ?,
		    accessed_at = ?,
		    source_type = ?,
		    is_primary = ?,
		    archived_copy_reference = ?,
		    notes = ?
		WHERE project_id = ? AND id = ?
	`, next.Title, nullIfEmpty(next.Publisher), nullIfEmpty(next.Author),
		nullIfEmpty(next.URL), nullIfEmpty(next.PublicationDate), nullIfEmpty(next.AccessedAt),
		next.SourceType, boolToInt(next.IsPrimary), nullIfEmpty(next.ArchivedCopyReference),
		nullIfEmpty(next.Notes), projectID, sourceID)
	if err != nil {
		return Source{}, err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return Source{}, err
	} else if changed != 1 {
		return Source{}, sql.ErrNoRows
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "source.update", "source", sourceID, "success", map[string]string{
		"sourceType": next.SourceType,
	}); err != nil {
		return Source{}, err
	}
	if err := tx.Commit(); err != nil {
		return Source{}, err
	}
	return s.GetSource(ctx, actorUserID, projectID, sourceID)
}

func (s *Store) GetSource(ctx context.Context, userID, projectID, sourceID string) (Source, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return Source{}, err
	}
	return getSourceTx(ctx, s.db, projectID, sourceID)
}

func (s *Store) ListRevisionClaims(ctx context.Context, userID, projectID, revisionID string) ([]Claim, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	if _, err := revisionArticleID(ctx, s.db, projectID, revisionID); err != nil {
		return nil, err
	}
	return listRevisionClaims(ctx, s.db, projectID, revisionID)
}

func (s *Store) CreateRevisionClaim(ctx context.Context, actorUserID, projectID, revisionID string, input ClaimInput) (Claim, error) {
	if _, err := s.projectRole(ctx, actorUserID, projectID); err != nil {
		return Claim{}, err
	}
	input = applyClaimDefaults(input)
	if err := validateClaimInput(input); err != nil {
		return Claim{}, err
	}
	claimID, err := security.RandomID("claim")
	if err != nil {
		return Claim{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, err
	}
	defer tx.Rollback()

	if err := requireActiveProjectTx(ctx, tx, projectID); err != nil {
		return Claim{}, err
	}
	articleID, err := editableRevisionArticleID(ctx, tx, projectID, revisionID)
	if err != nil {
		return Claim{}, err
	}
	if err := ensureSourceIDsBelongToProject(ctx, tx, projectID, input.SourceIDs); err != nil {
		return Claim{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO claims(id, project_id, revision_id, claim_text, block_id, importance)
		VALUES (?, ?, ?, ?, ?, ?)
	`, claimID, projectID, revisionID, input.ClaimText, nullIfEmpty(input.BlockID), input.Importance); err != nil {
		return Claim{}, err
	}
	if err := replaceClaimSources(ctx, tx, projectID, claimID, input.SourceIDs); err != nil {
		return Claim{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "claim.create", "claim", claimID, "success", map[string]string{
		"article_id":  articleID,
		"revision_id": revisionID,
		"importance":  input.Importance,
	}); err != nil {
		return Claim{}, err
	}
	if err := tx.Commit(); err != nil {
		return Claim{}, err
	}
	return s.GetClaim(ctx, actorUserID, projectID, claimID)
}

func (s *Store) VerifyClaim(ctx context.Context, actorUserID, projectID, claimID string, input ClaimVerificationInput) (Claim, error) {
	if err := s.requireContentReview(ctx, actorUserID, projectID); err != nil {
		return Claim{}, err
	}
	input.VerificationState = strings.ToLower(strings.TrimSpace(input.VerificationState))
	if !allowedClaimVerificationState(input.VerificationState) {
		return Claim{}, fmt.Errorf("%w: unsupported verificationState", ErrValidation)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, err
	}
	defer tx.Rollback()

	if err := requireActiveProjectTx(ctx, tx, projectID); err != nil {
		return Claim{}, err
	}
	claim, err := getClaimTx(ctx, tx, projectID, claimID)
	if err != nil {
		return Claim{}, err
	}
	if _, err := editableRevisionArticleID(ctx, tx, projectID, claim.RevisionID); err != nil {
		return Claim{}, err
	}
	sourceIDs := claim.SourceIDs
	if input.SourceIDs != nil {
		sourceIDs = cleanStringSlice(*input.SourceIDs)
		if err := ensureSourceIDsBelongToProject(ctx, tx, projectID, sourceIDs); err != nil {
			return Claim{}, err
		}
		if err := replaceClaimSources(ctx, tx, projectID, claimID, sourceIDs); err != nil {
			return Claim{}, err
		}
	}
	if input.VerificationState == "supported" && len(sourceIDs) == 0 {
		return Claim{}, fmt.Errorf("%w: supported claims require at least one source", ErrValidation)
	}
	verifiedBy := any(actorUserID)
	verified := 1
	if input.VerificationState == "unverified" {
		verifiedBy = nil
		verified = 0
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE claims
		SET verification_state = ?,
		    verified_by = ?,
		    verified_at = CASE WHEN ? = 0 THEN NULL ELSE CURRENT_TIMESTAMP END
		WHERE project_id = ? AND id = ?
	`, input.VerificationState, verifiedBy, verified, projectID, claimID); err != nil {
		return Claim{}, err
	}
	if err := insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, "claim.verify", "claim", claimID, "success", map[string]string{
		"state": input.VerificationState,
	}); err != nil {
		return Claim{}, err
	}
	if err := tx.Commit(); err != nil {
		return Claim{}, err
	}
	return s.GetClaim(ctx, actorUserID, projectID, claimID)
}

func (s *Store) GetClaim(ctx context.Context, userID, projectID, claimID string) (Claim, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return Claim{}, err
	}
	return getClaimTx(ctx, s.db, projectID, claimID)
}

func (s *Store) ListDisclosures(ctx context.Context, userID, projectID, articleID string) ([]Disclosure, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	if err := s.articleExists(ctx, projectID, articleID); err != nil {
		return nil, err
	}
	return listDisclosures(ctx, s.db, projectID, articleID)
}

func (s *Store) CreateDisclosure(ctx context.Context, actorUserID, projectID, articleID string, input DisclosureInput) (Disclosure, error) {
	if err := s.requireContentReview(ctx, actorUserID, projectID); err != nil {
		return Disclosure{}, err
	}
	input = applyDisclosureDefaults(input)
	if err := validateDisclosureInput(input); err != nil {
		return Disclosure{}, err
	}
	disclosureID, err := security.RandomID("disc")
	if err != nil {
		return Disclosure{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Disclosure{}, err
	}
	defer tx.Rollback()

	if err := requireActiveProjectTx(ctx, tx, projectID); err != nil {
		return Disclosure{}, err
	}
	if err := articleExistsTx(ctx, tx, projectID, articleID); err != nil {
		return Disclosure{}, err
	}
	if input.RevisionID != "" {
		if err := revisionBelongsToArticleTx(ctx, tx, projectID, articleID, input.RevisionID); err != nil {
			return Disclosure{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO disclosures(id, project_id, content_id, revision_id, disclosure_type, public_text, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, disclosureID, projectID, articleID, nullIfEmpty(input.RevisionID), input.DisclosureType, input.PublicText, actorUserID); err != nil {
		return Disclosure{}, err
	}
	if err := recordPublicTrustChange(ctx, tx, projectID, articleID, input.RevisionID, "disclosure.create", "disclosure", disclosureID, actorUserID); err != nil {
		return Disclosure{}, err
	}
	if err := tx.Commit(); err != nil {
		return Disclosure{}, err
	}
	return getDisclosure(ctx, s.db, projectID, disclosureID)
}

func (s *Store) ListCorrections(ctx context.Context, userID, projectID, articleID string) ([]CorrectionNotice, error) {
	if _, err := s.projectRole(ctx, userID, projectID); err != nil {
		return nil, err
	}
	if err := s.articleExists(ctx, projectID, articleID); err != nil {
		return nil, err
	}
	return listCorrections(ctx, s.db, projectID, articleID)
}

func (s *Store) CreateCorrection(ctx context.Context, actorUserID, projectID, articleID string, input CorrectionInput) (CorrectionNotice, error) {
	if err := s.requireContentReview(ctx, actorUserID, projectID); err != nil {
		return CorrectionNotice{}, err
	}
	input = applyCorrectionDefaults(input)
	if err := validateCorrectionInput(input); err != nil {
		return CorrectionNotice{}, err
	}
	correctionID, err := security.RandomID("corr")
	if err != nil {
		return CorrectionNotice{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CorrectionNotice{}, err
	}
	defer tx.Rollback()

	if err := requireActiveProjectTx(ctx, tx, projectID); err != nil {
		return CorrectionNotice{}, err
	}
	if err := articleExistsTx(ctx, tx, projectID, articleID); err != nil {
		return CorrectionNotice{}, err
	}
	if input.AffectedRevisionID != "" {
		if err := revisionBelongsToArticleTx(ctx, tx, projectID, articleID, input.AffectedRevisionID); err != nil {
			return CorrectionNotice{}, err
		}
	}
	if input.SupersedesNoticeID != "" {
		if err := correctionBelongsToArticleTx(ctx, tx, projectID, articleID, input.SupersedesNoticeID); err != nil {
			return CorrectionNotice{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO correction_notices(
		  id, project_id, content_id, affected_revision_id, public_note,
		  corrected_by, supersedes_notice_id
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, correctionID, projectID, articleID, nullIfEmpty(input.AffectedRevisionID), input.PublicNote,
		actorUserID, nullIfEmpty(input.SupersedesNoticeID)); err != nil {
		return CorrectionNotice{}, err
	}
	if err := recordPublicTrustChange(ctx, tx, projectID, articleID, input.AffectedRevisionID, "correction.create", "correction_notice", correctionID, actorUserID); err != nil {
		return CorrectionNotice{}, err
	}
	if err := tx.Commit(); err != nil {
		return CorrectionNotice{}, err
	}
	return getCorrection(ctx, s.db, projectID, correctionID)
}

func ensureRevisionClaimsApproved(ctx context.Context, tx *sql.Tx, projectID, revisionID string) error {
	var blockingCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM claims
		WHERE project_id = ?
		  AND revision_id = ?
		  AND importance IN ('material', 'critical')
		  AND verification_state NOT IN ('supported', 'not_applicable')
	`, projectID, revisionID).Scan(&blockingCount); err != nil {
		return err
	}
	if blockingCount > 0 {
		return fmt.Errorf("%w: material claims must be supported or marked not applicable before approval", ErrInvalidWorkflow)
	}
	return nil
}

func buildRevisionTrustSnapshots(ctx context.Context, tx *sql.Tx, projectID, revisionID string) (string, string, error) {
	claims, err := listRevisionClaims(ctx, tx, projectID, revisionID)
	if err != nil {
		return "", "", err
	}
	sourceIDs := map[string]struct{}{}
	for _, claim := range claims {
		for _, sourceID := range claim.SourceIDs {
			sourceIDs[sourceID] = struct{}{}
		}
	}
	orderedSourceIDs := make([]string, 0, len(sourceIDs))
	for sourceID := range sourceIDs {
		orderedSourceIDs = append(orderedSourceIDs, sourceID)
	}
	sort.Strings(orderedSourceIDs)
	sources := make([]Source, 0, len(orderedSourceIDs))
	for _, sourceID := range orderedSourceIDs {
		source, err := getSourceTx(ctx, tx, projectID, sourceID)
		if err != nil {
			return "", "", err
		}
		sources = append(sources, source)
	}
	sourceSnapshot, err := json.Marshal(sources)
	if err != nil {
		return "", "", err
	}
	claimSnapshot, err := json.Marshal(claims)
	if err != nil {
		return "", "", err
	}
	return string(sourceSnapshot), string(claimSnapshot), nil
}

func scanSource(row rowScanner) (Source, error) {
	var source Source
	var isPrimary int
	err := row.Scan(
		&source.ID,
		&source.ProjectID,
		&source.Title,
		&source.Publisher,
		&source.Author,
		&source.URL,
		&source.PublicationDate,
		&source.AccessedAt,
		&source.SourceType,
		&isPrimary,
		&source.ArchivedCopyReference,
		&source.Notes,
		&source.CreatedAt,
	)
	source.IsPrimary = isPrimary == 1
	return source, err
}

func scanClaim(row rowScanner) (Claim, error) {
	var claim Claim
	var sourceIDsJSON string
	err := row.Scan(
		&claim.ID,
		&claim.ProjectID,
		&claim.ArticleID,
		&claim.RevisionID,
		&claim.ClaimText,
		&claim.BlockID,
		&claim.Importance,
		&claim.VerificationState,
		&claim.VerifiedBy,
		&claim.VerifiedAt,
		&sourceIDsJSON,
	)
	if err != nil {
		return Claim{}, err
	}
	claim.SourceIDs = []string{}
	decodeInto(sourceIDsJSON, &claim.SourceIDs)
	return claim, nil
}

func scanDisclosure(row rowScanner) (Disclosure, error) {
	var disclosure Disclosure
	err := row.Scan(
		&disclosure.ID,
		&disclosure.ProjectID,
		&disclosure.ArticleID,
		&disclosure.RevisionID,
		&disclosure.DisclosureType,
		&disclosure.PublicText,
		&disclosure.CreatedBy,
		&disclosure.CreatedAt,
	)
	return disclosure, err
}

func scanCorrection(row rowScanner) (CorrectionNotice, error) {
	var correction CorrectionNotice
	err := row.Scan(
		&correction.ID,
		&correction.ProjectID,
		&correction.ArticleID,
		&correction.AffectedRevisionID,
		&correction.PublicNote,
		&correction.CorrectedBy,
		&correction.CorrectedAt,
		&correction.SupersedesNoticeID,
	)
	return correction, err
}

type trustQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type trustExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func getSourceTx(ctx context.Context, queryer trustQueryer, projectID, sourceID string) (Source, error) {
	return scanSource(queryer.QueryRowContext(ctx, `
		SELECT id, project_id, title, COALESCE(publisher, ''), COALESCE(author, ''),
		       COALESCE(url, ''), COALESCE(publication_date, ''), COALESCE(accessed_at, ''),
		       source_type, is_primary, COALESCE(archived_copy_reference, ''),
		       COALESCE(notes, ''), created_at
		FROM sources
		WHERE project_id = ? AND id = ?
	`, projectID, sourceID))
}

func listRevisionClaims(ctx context.Context, queryer trustQueryer, projectID, revisionID string) ([]Claim, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT claim.id, claim.project_id, revision.content_id, claim.revision_id,
		       claim.claim_text, COALESCE(claim.block_id, ''), claim.importance,
		       claim.verification_state, COALESCE(claim.verified_by, ''),
		       COALESCE(claim.verified_at, ''), COALESCE((
		         SELECT json_group_array(source_id)
		         FROM (
		           SELECT source_id
		           FROM claim_sources
		           WHERE project_id = claim.project_id AND claim_id = claim.id
		           ORDER BY source_id
		         )
		       ), '[]')
		FROM claims claim
		JOIN content_revisions revision
		  ON revision.project_id = claim.project_id
		 AND revision.id = claim.revision_id
		WHERE claim.project_id = ? AND claim.revision_id = ?
		ORDER BY claim.id
	`, projectID, revisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	claims := []Claim{}
	for rows.Next() {
		claim, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		claims = append(claims, claim)
	}
	return claims, rows.Err()
}

func getClaimTx(ctx context.Context, queryer trustQueryer, projectID, claimID string) (Claim, error) {
	return scanClaim(queryer.QueryRowContext(ctx, `
		SELECT claim.id, claim.project_id, revision.content_id, claim.revision_id,
		       claim.claim_text, COALESCE(claim.block_id, ''), claim.importance,
		       claim.verification_state, COALESCE(claim.verified_by, ''),
		       COALESCE(claim.verified_at, ''), COALESCE((
		         SELECT json_group_array(source_id)
		         FROM (
		           SELECT source_id
		           FROM claim_sources
		           WHERE project_id = claim.project_id AND claim_id = claim.id
		           ORDER BY source_id
		         )
		       ), '[]')
		FROM claims claim
		JOIN content_revisions revision
		  ON revision.project_id = claim.project_id
		 AND revision.id = claim.revision_id
		WHERE claim.project_id = ? AND claim.id = ?
	`, projectID, claimID))
}

func listDisclosures(ctx context.Context, queryer trustQueryer, projectID, articleID string) ([]Disclosure, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, project_id, content_id, COALESCE(revision_id, ''),
		       disclosure_type, public_text, created_by, created_at
		FROM disclosures
		WHERE project_id = ? AND content_id = ?
		ORDER BY created_at, id
	`, projectID, articleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	disclosures := []Disclosure{}
	for rows.Next() {
		disclosure, err := scanDisclosure(rows)
		if err != nil {
			return nil, err
		}
		disclosures = append(disclosures, disclosure)
	}
	return disclosures, rows.Err()
}

func getDisclosure(ctx context.Context, queryer trustQueryer, projectID, disclosureID string) (Disclosure, error) {
	return scanDisclosure(queryer.QueryRowContext(ctx, `
		SELECT id, project_id, content_id, COALESCE(revision_id, ''),
		       disclosure_type, public_text, created_by, created_at
		FROM disclosures
		WHERE project_id = ? AND id = ?
	`, projectID, disclosureID))
}

func listCorrections(ctx context.Context, queryer trustQueryer, projectID, articleID string) ([]CorrectionNotice, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, project_id, content_id, COALESCE(affected_revision_id, ''),
		       public_note, corrected_by, corrected_at, COALESCE(supersedes_notice_id, '')
		FROM correction_notices
		WHERE project_id = ? AND content_id = ?
		ORDER BY corrected_at, id
	`, projectID, articleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	corrections := []CorrectionNotice{}
	for rows.Next() {
		correction, err := scanCorrection(rows)
		if err != nil {
			return nil, err
		}
		corrections = append(corrections, correction)
	}
	return corrections, rows.Err()
}

func getCorrection(ctx context.Context, queryer trustQueryer, projectID, correctionID string) (CorrectionNotice, error) {
	return scanCorrection(queryer.QueryRowContext(ctx, `
		SELECT id, project_id, content_id, COALESCE(affected_revision_id, ''),
		       public_note, corrected_by, corrected_at, COALESCE(supersedes_notice_id, '')
		FROM correction_notices
		WHERE project_id = ? AND id = ?
	`, projectID, correctionID))
}

func revisionArticleID(ctx context.Context, queryer trustQueryer, projectID, revisionID string) (string, error) {
	var articleID string
	err := queryer.QueryRowContext(ctx, `
		SELECT content_id
		FROM content_revisions
		WHERE project_id = ? AND id = ?
	`, projectID, revisionID).Scan(&articleID)
	return articleID, err
}

func editableRevisionArticleID(ctx context.Context, queryer trustQueryer, projectID, revisionID string) (string, error) {
	var articleID string
	var state string
	err := queryer.QueryRowContext(ctx, `
		SELECT content_id, editorial_state
		FROM content_revisions
		WHERE project_id = ? AND id = ?
	`, projectID, revisionID).Scan(&articleID, &state)
	if err != nil {
		return "", err
	}
	if state == "approved" {
		return "", fmt.Errorf("%w: approved revisions cannot be changed", ErrInvalidWorkflow)
	}
	return articleID, nil
}

func ensureSourceIDsBelongToProject(ctx context.Context, queryer trustQueryer, projectID string, sourceIDs []string) error {
	for _, sourceID := range cleanStringSlice(sourceIDs) {
		var exists int
		err := queryer.QueryRowContext(ctx, `
			SELECT 1
			FROM sources
			WHERE project_id = ? AND id = ?
		`, projectID, sourceID).Scan(&exists)
		if errorsIsNoRows(err) {
			return fmt.Errorf("%w: sourceIds must reference sources in this project", ErrValidation)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func replaceClaimSources(ctx context.Context, execer interface {
	trustExecer
	trustQueryer
}, projectID, claimID string, sourceIDs []string) error {
	if _, err := execer.ExecContext(ctx, `
		DELETE FROM claim_sources
		WHERE project_id = ? AND claim_id = ?
	`, projectID, claimID); err != nil {
		return err
	}
	for _, sourceID := range cleanStringSlice(sourceIDs) {
		if _, err := execer.ExecContext(ctx, `
			INSERT INTO claim_sources(project_id, claim_id, source_id)
			VALUES (?, ?, ?)
		`, projectID, claimID, sourceID); err != nil {
			return err
		}
	}
	return nil
}

func correctionBelongsToArticleTx(ctx context.Context, tx *sql.Tx, projectID, articleID, correctionID string) error {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM correction_notices
		WHERE project_id = ? AND content_id = ? AND id = ?
	`, projectID, articleID, correctionID).Scan(&exists)
	return err
}

func recordPublicTrustChange(ctx context.Context, tx *sql.Tx, projectID, articleID, revisionID, action, targetType, targetID, actorUserID string) error {
	if err := incrementProjectGeneration(ctx, tx, projectID); err != nil {
		return err
	}
	if err := insertContentChangeOutbox(ctx, tx, projectID, articleID, revisionID, "content.updated", action); err != nil {
		return err
	}
	return insertAuditEventTx(ctx, tx, projectID, "user", actorUserID, action, targetType, targetID, "success", map[string]string{
		"article_id":  articleID,
		"revision_id": revisionID,
	})
}

func insertContentChangeOutbox(ctx context.Context, tx *sql.Tx, projectID, articleID, revisionID, eventType, reason string) error {
	payload, err := json.Marshal(map[string]any{
		"project_id":  projectID,
		"content_id":  articleID,
		"revision_id": revisionID,
		"reason":      reason,
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
		) VALUES (?, ?, ?, 'content', ?, ?, ?)
	`, eventID, projectID, eventType, articleID, string(payload), fmt.Sprintf("%s:%s:%s:%s", eventType, articleID, reason, eventID))
	return err
}

func applySourceDefaults(input SourceInput) SourceInput {
	input.Title = strings.TrimSpace(input.Title)
	input.Publisher = strings.TrimSpace(input.Publisher)
	input.Author = strings.TrimSpace(input.Author)
	input.URL = strings.TrimSpace(input.URL)
	input.PublicationDate = strings.TrimSpace(input.PublicationDate)
	input.AccessedAt = strings.TrimSpace(input.AccessedAt)
	input.SourceType = strings.ToLower(strings.TrimSpace(input.SourceType))
	if input.SourceType == "" {
		input.SourceType = "web"
	}
	input.ArchivedCopyReference = strings.TrimSpace(input.ArchivedCopyReference)
	input.Notes = strings.TrimSpace(input.Notes)
	return input
}

func validateSourceInput(input SourceInput) error {
	if input.Title == "" {
		return fmt.Errorf("%w: source title is required", ErrValidation)
	}
	if utf8.RuneCountInString(input.Title) > 240 {
		return fmt.Errorf("%w: source title cannot exceed 240 characters", ErrValidation)
	}
	if input.URL != "" && !hasHTTPScheme(input.URL) {
		return fmt.Errorf("%w: source URL must use http or https", ErrValidation)
	}
	if !allowedSourceType(input.SourceType) {
		return fmt.Errorf("%w: unsupported sourceType", ErrValidation)
	}
	return nil
}

func allowedSourceType(sourceType string) bool {
	switch sourceType {
	case "web", "book", "report", "dataset", "interview", "first_party", "primary", "other":
		return true
	default:
		return false
	}
}

func applyClaimDefaults(input ClaimInput) ClaimInput {
	input.ClaimText = strings.TrimSpace(input.ClaimText)
	input.BlockID = strings.TrimSpace(input.BlockID)
	input.Importance = strings.ToLower(strings.TrimSpace(input.Importance))
	if input.Importance == "" {
		input.Importance = "normal"
	}
	input.SourceIDs = cleanStringSlice(input.SourceIDs)
	return input
}

func validateClaimInput(input ClaimInput) error {
	if input.ClaimText == "" {
		return fmt.Errorf("%w: claimText is required", ErrValidation)
	}
	if utf8.RuneCountInString(input.ClaimText) > 1000 {
		return fmt.Errorf("%w: claimText cannot exceed 1000 characters", ErrValidation)
	}
	if !allowedClaimImportance(input.Importance) {
		return fmt.Errorf("%w: unsupported claim importance", ErrValidation)
	}
	return nil
}

func allowedClaimImportance(importance string) bool {
	switch importance {
	case "low", "normal", "material", "critical":
		return true
	default:
		return false
	}
}

func allowedClaimVerificationState(state string) bool {
	switch state {
	case "unverified", "supported", "partially_supported", "unsupported", "outdated", "subject_expert_required", "not_applicable":
		return true
	default:
		return false
	}
}

func applyDisclosureDefaults(input DisclosureInput) DisclosureInput {
	input.RevisionID = strings.TrimSpace(input.RevisionID)
	input.DisclosureType = strings.ToLower(strings.TrimSpace(input.DisclosureType))
	input.PublicText = strings.TrimSpace(input.PublicText)
	return input
}

func validateDisclosureInput(input DisclosureInput) error {
	if input.DisclosureType == "" {
		return fmt.Errorf("%w: disclosureType is required", ErrValidation)
	}
	if input.PublicText == "" {
		return fmt.Errorf("%w: publicText is required", ErrValidation)
	}
	if utf8.RuneCountInString(input.PublicText) > 2000 {
		return fmt.Errorf("%w: publicText cannot exceed 2000 characters", ErrValidation)
	}
	switch input.DisclosureType {
	case "sponsorship", "affiliate", "ai_assistance", "methodology", "limitations", "other":
		return nil
	default:
		return fmt.Errorf("%w: unsupported disclosureType", ErrValidation)
	}
}

func applyCorrectionDefaults(input CorrectionInput) CorrectionInput {
	input.AffectedRevisionID = strings.TrimSpace(input.AffectedRevisionID)
	input.PublicNote = strings.TrimSpace(input.PublicNote)
	input.SupersedesNoticeID = strings.TrimSpace(input.SupersedesNoticeID)
	return input
}

func validateCorrectionInput(input CorrectionInput) error {
	if input.PublicNote == "" {
		return fmt.Errorf("%w: publicNote is required", ErrValidation)
	}
	if utf8.RuneCountInString(input.PublicNote) > 2000 {
		return fmt.Errorf("%w: publicNote cannot exceed 2000 characters", ErrValidation)
	}
	return nil
}

func approvalContentHash(baseHash, sourceSnapshot, claimSnapshot string) (string, error) {
	raw, err := json.Marshal(map[string]string{
		"base":    baseHash,
		"sources": sourceSnapshot,
		"claims":  claimSnapshot,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func errorsIsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
