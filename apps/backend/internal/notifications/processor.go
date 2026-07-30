package notifications

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"seoblog/apps/backend/internal/mailer"
	"seoblog/apps/backend/internal/store"
)

type Processor struct {
	Store          *store.Store
	Sender         mailer.Sender
	Logger         *slog.Logger
	WorkerID       string
	AdminPublicURL string
}

func (p *Processor) Process(ctx context.Context, limit int) (int, int, error) {
	now := time.Now().UTC()
	deliveries, err := p.Store.ClaimReviewAssignmentNotifications(ctx, p.WorkerID, now, limit)
	if err != nil {
		return 0, 0, err
	}
	delivered := 0
	failed := 0
	for _, delivery := range deliveries {
		message, messageErr := p.message(delivery)
		if messageErr == nil {
			messageErr = p.Sender.Send(ctx, message)
		}
		if messageErr != nil {
			failed++
			if err := p.Store.MarkReviewAssignmentNotificationFailed(ctx, delivery, p.WorkerID, time.Now().UTC(), messageErr); err != nil {
				return delivered, failed, err
			}
			if p.Logger != nil {
				p.Logger.Warn("review assignment notification failed",
					"notification_id", delivery.ID,
					"assignment_id", delivery.AssignmentID,
					"attempt", delivery.AttemptCount,
					"error", messageErr,
				)
			}
			continue
		}
		if err := p.Store.MarkReviewAssignmentNotificationDelivered(ctx, delivery.ID, p.WorkerID, time.Now().UTC()); err != nil {
			return delivered, failed, err
		}
		delivered++
	}
	return delivered, failed, nil
}

func (p *Processor) message(delivery store.ReviewAssignmentNotificationDelivery) (mailer.Message, error) {
	articleURL, err := assignmentURL(p.AdminPublicURL, delivery.ProjectID, delivery.ArticleID)
	if err != nil {
		return mailer.Message{}, err
	}
	assignmentLabel := strings.ToUpper(delivery.AssignmentType)
	dueText := "No due date was set."
	if delivery.DueAt != "" {
		dueText = "Due: " + delivery.DueAt + " UTC."
	}
	subject := fmt.Sprintf("%s review assignment: %s", assignmentLabel, delivery.ArticleTitle)
	text := fmt.Sprintf(
		"You were assigned as %s for %q in %s.\n\n%s\n\nOpen the article:\n%s",
		delivery.AssignmentType,
		delivery.ArticleTitle,
		delivery.ProjectName,
		dueText,
		articleURL,
	)
	htmlBody := "<p>You were assigned as <strong>" + html.EscapeString(delivery.AssignmentType) +
		"</strong> for <strong>" + html.EscapeString(delivery.ArticleTitle) +
		"</strong> in " + html.EscapeString(delivery.ProjectName) + ".</p>" +
		"<p>" + html.EscapeString(dueText) + "</p>" +
		`<p><a href="` + html.EscapeString(articleURL) + `">Open the article</a></p>`
	return mailer.Message{
		To:      delivery.RecipientEmail,
		Subject: subject,
		Text:    text,
		HTML:    htmlBody,
	}, nil
}

func assignmentURL(adminPublicURL, projectID, articleID string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(adminPublicURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("SEOBLOG_ADMIN_PUBLIC_URL must be an absolute URL")
	}
	base.Path = strings.TrimRight(base.Path, "/") +
		"/projects/" + url.PathEscape(projectID) +
		"/articles/" + url.PathEscape(articleID)
	base.RawPath = ""
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}
