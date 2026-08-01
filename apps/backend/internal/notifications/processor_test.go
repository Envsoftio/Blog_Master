package notifications

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"seoblog/apps/backend/internal/mailer"
	"seoblog/apps/backend/internal/platform/database"
	"seoblog/apps/backend/internal/store"
)

func TestProcessorDeliversQueuedReviewAssignmentNotification(t *testing.T) {
	notificationStore, db := testNotificationStore(t)
	assignment, err := notificationStore.CreateReviewAssignment(context.Background(), "owner", "project", "article", store.ReviewAssignmentInput{
		RevisionID:     "revision",
		AssignedTo:     "reviewer",
		AssignmentType: "reviewer",
		DueAt:          "2026-08-01T10:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE users SET email_normalized = 'reviewer-current@example.test' WHERE id = 'reviewer'`); err != nil {
		t.Fatal(err)
	}
	sender := &recordingSender{}
	processor := Processor{
		Store:          notificationStore,
		Sender:         sender,
		WorkerID:       "notification-worker",
		AdminPublicURL: "https://admin.example.test/cms",
	}

	delivered, failed, err := processor.Process(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if delivered != 1 || failed != 0 {
		t.Fatalf("unexpected processor result delivered=%d failed=%d", delivered, failed)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("expected one email, got %d", len(sender.messages))
	}
	message := sender.messages[0]
	if message.To != "reviewer-current@example.test" ||
		!strings.Contains(message.Subject, "REVIEWER review assignment: Article title") ||
		!strings.Contains(message.Text, "https://admin.example.test/cms/projects/project/articles/article") ||
		!strings.Contains(message.Text, "2026-08-01 10:00:00 UTC") {
		t.Fatalf("unexpected notification message %#v", message)
	}
	var status string
	var attempts int
	if err := db.QueryRow(`
		SELECT status, attempt_count
		FROM review_assignment_notifications
		WHERE assignment_id = ?
	`, assignment.ID).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != "delivered" || attempts != 1 {
		t.Fatalf("unexpected notification state status=%q attempts=%d", status, attempts)
	}

	delivered, failed, err = processor.Process(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if delivered != 0 || failed != 0 || len(sender.messages) != 1 {
		t.Fatalf("delivered notification should not be claimed again")
	}
}

func TestProcessorSuppressesNotificationAfterMembershipRemoval(t *testing.T) {
	notificationStore, db := testNotificationStore(t)
	assignment, err := notificationStore.CreateReviewAssignment(context.Background(), "owner", "project", "article", store.ReviewAssignmentInput{
		RevisionID:     "revision",
		AssignedTo:     "reviewer",
		AssignmentType: "reviewer",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		UPDATE project_memberships
		SET status = 'removed', removed_at = CURRENT_TIMESTAMP
		WHERE project_id = 'project' AND user_id = 'reviewer'
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		UPDATE review_assignment_notifications
		SET status = 'processing',
		    locked_by = 'crashed-worker',
		    locked_until = '2000-01-01 00:00:00'
		WHERE assignment_id = ?
	`, assignment.ID); err != nil {
		t.Fatal(err)
	}
	sender := &recordingSender{}
	processor := Processor{
		Store:          notificationStore,
		Sender:         sender,
		WorkerID:       "notification-worker",
		AdminPublicURL: "https://admin.example.test",
	}

	delivered, failed, err := processor.Process(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if delivered != 0 || failed != 0 || len(sender.messages) != 0 {
		t.Fatalf("removed member must not receive assignment email")
	}
	var status string
	if err := db.QueryRow(`
		SELECT status
		FROM review_assignment_notifications
		WHERE assignment_id = ?
	`, assignment.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "suppressed" {
		t.Fatalf("expected removed-member notification to be suppressed, got %q", status)
	}
}

func TestProcessorRetriesFailedNotificationWhenItBecomesDue(t *testing.T) {
	notificationStore, db := testNotificationStore(t)
	assignment, err := notificationStore.CreateReviewAssignment(context.Background(), "owner", "project", "article", store.ReviewAssignmentInput{
		RevisionID:     "revision",
		AssignedTo:     "reviewer",
		AssignmentType: "reviewer",
	})
	if err != nil {
		t.Fatal(err)
	}
	sender := &recordingSender{err: errors.New("temporary SMTP outage")}
	processor := Processor{
		Store:          notificationStore,
		Sender:         sender,
		WorkerID:       "notification-worker",
		AdminPublicURL: "https://admin.example.test",
	}

	delivered, failed, err := processor.Process(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if delivered != 0 || failed != 1 {
		t.Fatalf("unexpected first attempt delivered=%d failed=%d", delivered, failed)
	}
	delivered, failed, err = processor.Process(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if delivered != 0 || failed != 0 {
		t.Fatalf("notification should wait until next_attempt_at")
	}
	if _, err := db.Exec(`
		UPDATE review_assignment_notifications
		SET next_attempt_at = '2000-01-01 00:00:00'
		WHERE assignment_id = ?
	`, assignment.ID); err != nil {
		t.Fatal(err)
	}
	sender.err = nil
	delivered, failed, err = processor.Process(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if delivered != 1 || failed != 0 {
		t.Fatalf("unexpected retry result delivered=%d failed=%d", delivered, failed)
	}
	var status string
	var attempts int
	if err := db.QueryRow(`
		SELECT status, attempt_count
		FROM review_assignment_notifications
		WHERE assignment_id = ?
	`, assignment.ID).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != "delivered" || attempts != 2 {
		t.Fatalf("unexpected retried notification state status=%q attempts=%d", status, attempts)
	}
}

func TestProcessorDeadLettersAfterFinalAttempt(t *testing.T) {
	notificationStore, db := testNotificationStore(t)
	assignment, err := notificationStore.CreateReviewAssignment(context.Background(), "owner", "project", "article", store.ReviewAssignmentInput{
		RevisionID:     "revision",
		AssignedTo:     "reviewer",
		AssignmentType: "sme",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		UPDATE review_assignment_notifications
		SET max_attempts = 1
		WHERE assignment_id = ?
	`, assignment.ID); err != nil {
		t.Fatal(err)
	}
	processor := Processor{
		Store:          notificationStore,
		Sender:         &recordingSender{err: errors.New("temporary SMTP outage")},
		WorkerID:       "notification-worker",
		AdminPublicURL: "https://admin.example.test",
	}

	delivered, failed, err := processor.Process(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if delivered != 0 || failed != 1 {
		t.Fatalf("unexpected processor result delivered=%d failed=%d", delivered, failed)
	}
	var status, lastError string
	var attempts int
	if err := db.QueryRow(`
		SELECT status, attempt_count, COALESCE(last_error_safe_message, '')
		FROM review_assignment_notifications
		WHERE assignment_id = ?
	`, assignment.ID).Scan(&status, &attempts, &lastError); err != nil {
		t.Fatal(err)
	}
	if status != "dead_letter" || attempts != 1 || !strings.Contains(lastError, "SMTP outage") {
		t.Fatalf("unexpected failed notification state status=%q attempts=%d error=%q", status, attempts, lastError)
	}
}

func testNotificationStore(t *testing.T) (*store.Store, *sql.DB) {
	t.Helper()
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "notifications.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO projects(id, slug, name, public_project_key)
		VALUES ('project', 'project', 'Project name', 'public-project');
		INSERT INTO users(id, email_normalized, status)
		VALUES
		  ('owner', 'owner@example.test', 'active'),
		  ('reviewer', 'reviewer@example.test', 'active');
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES
		  ('project', 'owner', 'project_owner', 'active', CURRENT_TIMESTAMP),
		  ('project', 'reviewer', 'reviewer', 'active', CURRENT_TIMESTAMP);
		INSERT INTO content_items(id, project_id, article_type, created_by)
		VALUES ('article', 'project', 'standard', 'owner');
		INSERT INTO content_revisions(
		  id, project_id, content_id, revision_number, created_by_type, title, content_hash
		) VALUES ('revision', 'project', 'article', 1, 'human', 'Article title', 'hash');
	`); err != nil {
		t.Fatal(err)
	}
	return store.New(db), db
}

type recordingSender struct {
	messages []mailer.Message
	err      error
}

func (s *recordingSender) Send(_ context.Context, message mailer.Message) error {
	s.messages = append(s.messages, message)
	return s.err
}
