package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/0001_initial.sql
var initialMigration string

//go:embed migrations/0002_session_reauthentication.sql
var sessionReauthenticationMigration string

//go:embed migrations/0003_invitation_revocation.sql
var invitationRevocationMigration string

//go:embed migrations/0004_audit_event_ids.sql
var auditEventIDsMigration string

//go:embed migrations/0005_author_photo_asset_guard.sql
var authorPhotoAssetGuardMigration string

//go:embed migrations/0006_review_comments_index.sql
var reviewCommentsIndexMigration string

//go:embed migrations/0007_revision_base_guard.sql
var revisionBaseGuardMigration string

//go:embed migrations/0008_admin_frontend_services.sql
var adminFrontendServicesMigration string

//go:embed migrations/0009_preview_tokens.sql
var previewTokensMigration string

//go:embed migrations/0010_ai_observability.sql
var aiObservabilityMigration string

//go:embed migrations/0011_ai_context.sql
var aiContextMigration string

//go:embed migrations/0012_ai_job_context.sql
var aiJobContextMigration string

//go:embed migrations/0013_password_reset_index.sql
var passwordResetIndexMigration string

//go:embed migrations/0014_review_assignments_integrity.sql
var reviewAssignmentsIntegrityMigration string

//go:embed migrations/0015_review_assignment_workflow.sql
var reviewAssignmentWorkflowMigration string

//go:embed migrations/0016_webhook_delivery.sql
var webhookDeliveryMigration string

type migration struct {
	version    string
	statements string
}

func OpenSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}

	return db, nil
}

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return err
	}

	migrations := []migration{
		{version: "0001_initial", statements: initialMigration},
		{version: "0002_session_reauthentication", statements: sessionReauthenticationMigration},
		{version: "0003_invitation_revocation", statements: invitationRevocationMigration},
		{version: "0004_audit_event_ids", statements: auditEventIDsMigration},
		{version: "0005_author_photo_asset_guard", statements: authorPhotoAssetGuardMigration},
		{version: "0006_review_comments_index", statements: reviewCommentsIndexMigration},
		{version: "0007_revision_base_guard", statements: revisionBaseGuardMigration},
		{version: "0008_admin_frontend_services", statements: adminFrontendServicesMigration},
		{version: "0009_preview_tokens", statements: previewTokensMigration},
		{version: "0010_ai_observability", statements: aiObservabilityMigration},
		{version: "0011_ai_context", statements: aiContextMigration},
		{version: "0012_ai_job_context", statements: aiJobContextMigration},
		{version: "0013_password_reset_index", statements: passwordResetIndexMigration},
		{version: "0014_review_assignments_integrity", statements: reviewAssignmentsIntegrityMigration},
		{version: "0015_review_assignment_workflow", statements: reviewAssignmentWorkflowMigration},
		{version: "0016_webhook_delivery", statements: webhookDeliveryMigration},
	}
	for _, item := range migrations {
		if err := applyMigration(db, item); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(db *sql.DB, item migration) error {
	var exists int
	if err := db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, item.version).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, stmt := range strings.Split(item.statements, "-- statement") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migration %s: %w\n%s", item.version, err, stmt)
		}
	}

	if _, err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, item.version); err != nil {
		return err
	}
	return tx.Commit()
}
