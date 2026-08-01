package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"seoblog/apps/backend/internal/config"
	"seoblog/apps/backend/internal/mailer"
	"seoblog/apps/backend/internal/platform/database"
	"seoblog/apps/backend/internal/security"
	"seoblog/apps/backend/internal/store"
)

func TestPasswordResetIsEnumerationSafeSingleUseAndRevokesSessions(t *testing.T) {
	sender := &recordingMailer{}
	server, db := newAdminTestServerWithMailer(t, sender)
	oldPassword := "correct horse battery staple"
	newPassword := "new correct horse battery staple"
	seedOwner(t, db, "owner@example.test", oldPassword)
	login := adminLogin(t, server, "owner@example.test", oldPassword)

	firstBody := requestPasswordReset(t, server, "OWNER@example.test", http.StatusAccepted)
	firstMessage := sender.message(t, 0)
	firstToken := resetTokenFromMessage(t, firstMessage)
	if !strings.Contains(firstMessage.HTML, "Reset your password</a>") {
		t.Fatalf("expected an HTML password-reset message, got %q", firstMessage.HTML)
	}
	if strings.Contains(firstBody, firstToken) || strings.Contains(firstBody, "owner@example.test") {
		t.Fatalf("password-reset response leaked account or token data: %s", firstBody)
	}

	var storedHash string
	if err := db.QueryRow(`
		SELECT token_hash
		FROM password_resets
		ORDER BY created_at DESC, rowid DESC
		LIMIT 1
	`).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash != security.TokenHash(firstToken) {
		t.Fatalf("expected reset verifier %q, got %q", security.TokenHash(firstToken), storedHash)
	}
	if storedHash == firstToken {
		t.Fatal("raw reset token was stored")
	}

	unknownBody := requestPasswordReset(t, server, "missing@example.test", http.StatusAccepted)
	if unknownBody != firstBody {
		t.Fatalf("known and unknown reset responses differ:\nknown: %s\nunknown: %s", firstBody, unknownBody)
	}
	if sender.count() != 1 {
		t.Fatalf("expected no email for an unknown account, got %d messages", sender.count())
	}

	requestPasswordReset(t, server, "owner@example.test", http.StatusAccepted)
	secondToken := resetTokenFromMessage(t, sender.message(t, 1))
	completePasswordReset(t, server, firstToken, newPassword, http.StatusBadRequest)
	completePasswordReset(t, server, secondToken, "too short", http.StatusBadRequest)
	completePasswordReset(t, server, secondToken, newPassword, http.StatusOK)
	completePasswordReset(t, server, secondToken, newPassword, http.StatusBadRequest)

	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	addCookies(meRequest, login.cookies)
	meResponse := mustTest(t, server, meRequest)
	if meResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected the pre-reset session to be revoked, got %d", meResponse.StatusCode)
	}

	oldLoginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(
		`{"email":"owner@example.test","password":"`+oldPassword+`"}`,
	))
	oldLoginRequest.Header.Set("Content-Type", "application/json")
	oldLoginResponse := mustTest(t, server, oldLoginRequest)
	if oldLoginResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected old password login to fail, got %d", oldLoginResponse.StatusCode)
	}
	adminLogin(t, server, "owner@example.test", newPassword)

	var pendingResetCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM password_resets
		WHERE user_id = ? AND used_at IS NULL
	`, login.userID).Scan(&pendingResetCount); err != nil {
		t.Fatal(err)
	}
	if pendingResetCount != 0 {
		t.Fatalf("expected all reset links to be consumed, got %d pending", pendingResetCount)
	}

	expiredToken, err := security.RandomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO password_resets(token_hash, user_id, expires_at)
		VALUES (?, ?, datetime(CURRENT_TIMESTAMP, '-1 minute'))
	`, security.TokenHash(expiredToken), login.userID); err != nil {
		t.Fatal(err)
	}
	completePasswordReset(t, server, expiredToken, newPassword, http.StatusBadRequest)
}

func TestAdminLoginMeAndProjectCreate(t *testing.T) {
	server, db := newAdminTestServer(t)
	password := "correct horse battery staple"
	seedOwner(t, db, "owner@example.test", password)

	login := adminLogin(t, server, "OWNER@example.test", password)

	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	addCookies(meRequest, login.cookies)
	meResponse := mustTest(t, server, meRequest)
	if meResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected /auth/me 200, got %d", meResponse.StatusCode)
	}

	body := strings.NewReader(`{"slug":"Demo Project","name":"Demo Project","primaryDomain":"example.test"}`)
	createWithoutCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/projects", body)
	createWithoutCSRF.Header.Set("Content-Type", "application/json")
	addCookies(createWithoutCSRF, login.cookies)
	missingCSRFResponse := mustTest(t, server, createWithoutCSRF)
	if missingCSRFResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected create without CSRF to fail with 403, got %d", missingCSRFResponse.StatusCode)
	}

	body = strings.NewReader(`{"slug":"Demo Project","name":"Demo Project","primaryDomain":"example.test"}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects", body)
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(createRequest, login.cookies)
	createResponse := mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected create project 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}

	var created Envelope[store.AdminProject]
	decodeJSONResponse(t, createResponse, &created)
	if created.Data.Slug != "demo-project" {
		t.Fatalf("expected slug to be normalized, got %q", created.Data.Slug)
	}
	if created.Data.Role != "project_owner" {
		t.Fatalf("expected creator owner membership, got %q", created.Data.Role)
	}
	if created.Data.PublicProjectKey == "" {
		t.Fatal("expected public project key")
	}

	var membershipRole string
	if err := db.QueryRow(`
		SELECT role
		FROM project_memberships
		WHERE project_id = ? AND user_id = ?
	`, created.Data.ID, login.userID).Scan(&membershipRole); err != nil {
		t.Fatal(err)
	}
	if membershipRole != "project_owner" {
		t.Fatalf("expected project_owner membership, got %q", membershipRole)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	addCookies(listRequest, login.cookies)
	listResponse := mustTest(t, server, listRequest)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected project list 200, got %d", listResponse.StatusCode)
	}
	var list ListEnvelope[store.AdminProject]
	decodeJSONResponse(t, listResponse, &list)
	if len(list.Data) != 1 || list.Data[0].ID != created.Data.ID {
		t.Fatalf("expected project list to include created project, got %#v", list.Data)
	}
}

func TestAdminProjectAccessIsMembershipScoped(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	otherLogin := seedAndLogin(t, server, db, "other@example.test", "another correct horse battery staple")

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"slug":"private","name":"Private Project"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("X-CSRF-Token", ownerLogin.csrfToken)
	addCookies(createRequest, ownerLogin.cookies)
	createResponse := mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected create project 201, got %d", createResponse.StatusCode)
	}
	var created Envelope[store.AdminProject]
	decodeJSONResponse(t, createResponse, &created)

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+created.Data.ID, nil)
	addCookies(getRequest, otherLogin.cookies)
	getResponse := mustTest(t, server, getRequest)
	if getResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected non-member project lookup to return 404, got %d", getResponse.StatusCode)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	addCookies(listRequest, otherLogin.cookies)
	listResponse := mustTest(t, server, listRequest)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected list 200, got %d", listResponse.StatusCode)
	}
	var list ListEnvelope[store.AdminProject]
	decodeJSONResponse(t, listResponse, &list)
	if len(list.Data) != 0 {
		t.Fatalf("expected non-member project list to be empty, got %#v", list.Data)
	}
}

func TestProjectPublisherUpdateAdvancesContentGeneration(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"publisher-cache","name":"Publisher Cache"}`)

	var before int64
	if err := db.QueryRow(`SELECT content_generation FROM projects WHERE id = ?`, project.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	updateRequest := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+project.ID,
		`{"publisherName":"Example Publishing","publisherUrl":"https://publisher.example/about"}`,
		login,
	)
	updateResponse := mustTest(t, server, updateRequest)
	if updateResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected publisher update 200, got %d: %s", updateResponse.StatusCode, readBody(t, updateResponse))
	}

	var after int64
	if err := db.QueryRow(`SELECT content_generation FROM projects WHERE id = ?`, project.ID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before+1 {
		t.Fatalf("expected publisher settings to advance content generation from %d to %d, got %d", before, before+1, after)
	}
}

func TestProjectMembershipInvitationAndRoleLifecycle(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"members","name":"Members Project"}`)

	inviteRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/invitations", strings.NewReader(`{"email":" Writer@Example.Test ","role":"writer"}`))
	inviteRequest.Header.Set("Content-Type", "application/json")
	inviteRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(inviteRequest, login.cookies)
	inviteResponse := mustTest(t, server, inviteRequest)
	if inviteResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected invite 201, got %d: %s", inviteResponse.StatusCode, readBody(t, inviteResponse))
	}
	var invited Envelope[store.ProjectMemberInvitation]
	decodeJSONResponse(t, inviteResponse, &invited)
	if invited.Data.Token == "" {
		t.Fatal("expected one-time invitation token")
	}
	if invited.Data.Member.Email != "writer@example.test" {
		t.Fatalf("expected normalized invite email, got %q", invited.Data.Member.Email)
	}
	if invited.Data.Member.Role != "writer" || invited.Data.Member.Status != "invited" {
		t.Fatalf("expected invited writer membership, got %#v", invited.Data.Member)
	}

	var storedTokenHash string
	if err := db.QueryRow(`
		SELECT token_hash
		FROM invitations
		WHERE project_id = ? AND email_normalized = ?
	`, project.ID, "writer@example.test").Scan(&storedTokenHash); err != nil {
		t.Fatal(err)
	}
	if storedTokenHash == invited.Data.Token {
		t.Fatal("expected invitation token verifier to be stored instead of the raw token")
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/members", nil)
	addCookies(listRequest, login.cookies)
	listResponse := mustTest(t, server, listRequest)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected members list 200, got %d: %s", listResponse.StatusCode, readBody(t, listResponse))
	}
	var list ListEnvelope[store.AdminProjectMember]
	decodeJSONResponse(t, listResponse, &list)
	if findProjectMember(list.Data, invited.Data.Member.UserID).UserID == "" {
		t.Fatalf("expected list to include invited member, got %#v", list.Data)
	}

	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/"+project.ID+"/members/"+invited.Data.Member.UserID, strings.NewReader(`{"role":"reviewer"}`))
	patchRequest.Header.Set("Content-Type", "application/json")
	patchRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(patchRequest, login.cookies)
	patchResponse := mustTest(t, server, patchRequest)
	if patchResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected role update 200, got %d: %s", patchResponse.StatusCode, readBody(t, patchResponse))
	}
	var patched Envelope[store.AdminProjectMember]
	decodeJSONResponse(t, patchResponse, &patched)
	if patched.Data.Role != "reviewer" || patched.Data.Status != "invited" {
		t.Fatalf("expected invited reviewer after patch, got %#v", patched.Data)
	}

	removeRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+project.ID+"/members/"+invited.Data.Member.UserID, nil)
	removeRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(removeRequest, login.cookies)
	removeResponse := mustTest(t, server, removeRequest)
	if removeResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("expected remove member 204, got %d: %s", removeResponse.StatusCode, readBody(t, removeResponse))
	}

	listAfterRemove := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/members", nil)
	addCookies(listAfterRemove, login.cookies)
	listAfterRemoveResponse := mustTest(t, server, listAfterRemove)
	if listAfterRemoveResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected members list after remove 200, got %d", listAfterRemoveResponse.StatusCode)
	}
	var afterRemove ListEnvelope[store.AdminProjectMember]
	decodeJSONResponse(t, listAfterRemoveResponse, &afterRemove)
	removed := findProjectMember(afterRemove.Data, invited.Data.Member.UserID)
	if removed.Status != "removed" || removed.RemovedAt == "" {
		t.Fatalf("expected removed member to stay auditable, got %#v", removed)
	}
}

func TestProjectMembershipOwnerRetentionAndAdminLimits(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	adminLogin := seedAndLogin(t, server, db, "admin@example.test", "another correct horse battery staple")
	project := createTestProject(t, server, ownerLogin, `{"slug":"owner-retention","name":"Owner Retention"}`)

	_, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'project_admin', 'active', CURRENT_TIMESTAMP)
	`, project.ID, adminLogin.userID)
	if err != nil {
		t.Fatal(err)
	}

	adminInviteOwner := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/invitations", strings.NewReader(`{"email":"new-owner@example.test","role":"project_owner"}`))
	adminInviteOwner.Header.Set("Content-Type", "application/json")
	adminInviteOwner.Header.Set("X-CSRF-Token", adminLogin.csrfToken)
	addCookies(adminInviteOwner, adminLogin.cookies)
	adminInviteOwnerResponse := mustTest(t, server, adminInviteOwner)
	if adminInviteOwnerResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected project admin owner invite to fail with 403, got %d: %s", adminInviteOwnerResponse.StatusCode, readBody(t, adminInviteOwnerResponse))
	}

	demoteOnlyOwner := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/"+project.ID+"/members/"+ownerLogin.userID, strings.NewReader(`{"role":"project_admin"}`))
	demoteOnlyOwner.Header.Set("Content-Type", "application/json")
	demoteOnlyOwner.Header.Set("X-CSRF-Token", ownerLogin.csrfToken)
	addCookies(demoteOnlyOwner, ownerLogin.cookies)
	demoteOnlyOwnerResponse := mustTest(t, server, demoteOnlyOwner)
	if demoteOnlyOwnerResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected last owner demotion to fail with 409, got %d: %s", demoteOnlyOwnerResponse.StatusCode, readBody(t, demoteOnlyOwnerResponse))
	}

	removeOnlyOwner := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+project.ID+"/members/"+ownerLogin.userID, nil)
	removeOnlyOwner.Header.Set("X-CSRF-Token", ownerLogin.csrfToken)
	addCookies(removeOnlyOwner, ownerLogin.cookies)
	removeOnlyOwnerResponse := mustTest(t, server, removeOnlyOwner)
	if removeOnlyOwnerResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected last owner removal to fail with 409, got %d: %s", removeOnlyOwnerResponse.StatusCode, readBody(t, removeOnlyOwnerResponse))
	}

	adminDemoteOwner := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/"+project.ID+"/members/"+ownerLogin.userID, strings.NewReader(`{"role":"editor"}`))
	adminDemoteOwner.Header.Set("Content-Type", "application/json")
	adminDemoteOwner.Header.Set("X-CSRF-Token", adminLogin.csrfToken)
	addCookies(adminDemoteOwner, adminLogin.cookies)
	adminDemoteOwnerResponse := mustTest(t, server, adminDemoteOwner)
	if adminDemoteOwnerResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected project admin owner demotion to fail with 403, got %d: %s", adminDemoteOwnerResponse.StatusCode, readBody(t, adminDemoteOwnerResponse))
	}

	adminInviteWriter := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/invitations", strings.NewReader(`{"email":"writer@example.test","role":"writer"}`))
	adminInviteWriter.Header.Set("Content-Type", "application/json")
	adminInviteWriter.Header.Set("X-CSRF-Token", adminLogin.csrfToken)
	addCookies(adminInviteWriter, adminLogin.cookies)
	adminInviteWriterResponse := mustTest(t, server, adminInviteWriter)
	if adminInviteWriterResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected project admin writer invite 201, got %d: %s", adminInviteWriterResponse.StatusCode, readBody(t, adminInviteWriterResponse))
	}
}

func TestOwnershipChangesRequireRecentReauthentication(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	secondOwnerLogin := seedAndLogin(t, server, db, "second-owner@example.test", "second owner correct password")
	project := createTestProject(t, server, ownerLogin, `{"slug":"ownership-reauth","name":"Ownership Reauthentication"}`)

	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'project_owner', 'active', CURRENT_TIMESTAMP)
	`, project.ID, secondOwnerLogin.userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		UPDATE sessions
		SET reauthenticated_at = datetime(CURRENT_TIMESTAMP, '-10 minutes')
		WHERE user_id = ?
	`, ownerLogin.userID); err != nil {
		t.Fatal(err)
	}

	writerInvite := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/invitations",
		`{"email":"writer@example.test","role":"writer"}`,
		ownerLogin,
	)
	writerInviteResponse := mustTest(t, server, writerInvite)
	if writerInviteResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected a non-ownership invite to remain available, got %d: %s", writerInviteResponse.StatusCode, readBody(t, writerInviteResponse))
	}

	ownerInvite := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/invitations",
		`{"email":"new-owner@example.test","role":"project_owner"}`,
		ownerLogin,
	)
	ownerInviteResponse := mustTest(t, server, ownerInvite)
	assertRecentReauthenticationRequired(t, ownerInviteResponse)

	demoteOwner := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+project.ID+"/members/"+secondOwnerLogin.userID,
		`{"role":"editor"}`,
		ownerLogin,
	)
	demoteOwnerResponse := mustTest(t, server, demoteOwner)
	assertRecentReauthenticationRequired(t, demoteOwnerResponse)

	removeOwner := newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/members/"+secondOwnerLogin.userID,
		"",
		ownerLogin,
	)
	removeOwnerResponse := mustTest(t, server, removeOwner)
	assertRecentReauthenticationRequired(t, removeOwnerResponse)
}

func TestProjectInvitationAcceptanceUsesCurrentRoleAndIsSingleUse(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, ownerLogin, `{"slug":"acceptance","name":"Invitation Acceptance"}`)
	invitation := createTestInvitation(t, server, ownerLogin, project.ID, `{"email":"new-user@example.test","role":"writer"}`)

	patchRequest := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+project.ID+"/members/"+invitation.Member.UserID,
		`{"role":"reviewer"}`,
		ownerLogin,
	)
	patchResponse := mustTest(t, server, patchRequest)
	if patchResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected invited role update 200, got %d: %s", patchResponse.StatusCode, readBody(t, patchResponse))
	}

	var storedRole string
	if err := db.QueryRow(`
		SELECT role
		FROM invitations
		WHERE token_hash = ?
	`, security.TokenHash(invitation.Token)).Scan(&storedRole); err != nil {
		t.Fatal(err)
	}
	if storedRole != "reviewer" {
		t.Fatalf("expected pending invitation role reviewer, got %q", storedRole)
	}

	password := "new user correct password"
	acceptance := acceptTestInvitation(t, server, invitation.Token, password, http.StatusOK)
	if acceptance.Role != "reviewer" || acceptance.UserID != invitation.Member.UserID {
		t.Fatalf("unexpected invitation acceptance %#v", acceptance)
	}

	var userStatus, membershipStatus string
	if err := db.QueryRow(`SELECT status FROM users WHERE id = ?`, invitation.Member.UserID).Scan(&userStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`
		SELECT status
		FROM project_memberships
		WHERE project_id = ? AND user_id = ?
	`, project.ID, invitation.Member.UserID).Scan(&membershipStatus); err != nil {
		t.Fatal(err)
	}
	if userStatus != "active" || membershipStatus != "active" {
		t.Fatalf("expected active user and membership, got user=%q membership=%q", userStatus, membershipStatus)
	}

	_ = adminLogin(t, server, "new-user@example.test", password)
	_ = acceptTestInvitation(t, server, invitation.Token, password, http.StatusBadRequest)
}

func TestProjectInvitationReissueAndRemovalRevokePendingTokens(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, ownerLogin, `{"slug":"revocation","name":"Invitation Revocation"}`)

	first := createTestInvitation(t, server, ownerLogin, project.ID, `{"email":"pending@example.test","role":"editor"}`)
	second := createTestInvitation(t, server, ownerLogin, project.ID, `{"email":"pending@example.test","role":"writer"}`)
	if first.Token == second.Token {
		t.Fatal("expected reissued invitation to use a new token")
	}
	assertInvitationRevoked(t, db, first.Token)
	_ = acceptTestInvitation(t, server, first.Token, "pending account password", http.StatusBadRequest)

	removeRequest := newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/members/"+second.Member.UserID,
		"",
		ownerLogin,
	)
	removeResponse := mustTest(t, server, removeRequest)
	if removeResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("expected pending member removal 204, got %d: %s", removeResponse.StatusCode, readBody(t, removeResponse))
	}
	assertInvitationRevoked(t, db, second.Token)
	_ = acceptTestInvitation(t, server, second.Token, "pending account password", http.StatusBadRequest)
}

func TestMembershipRoleChangePreservesSessionsAndUsesLiveAuthorization(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	memberLogin := seedAndLogin(t, server, db, "member@example.test", "member correct horse password")
	project := createTestProject(t, server, ownerLogin, `{"slug":"sessions","name":"Session Revocation"}`)
	otherProject := createTestProject(t, server, memberLogin, `{"slug":"other-project","name":"Other Project"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, memberLogin.userID); err != nil {
		t.Fatal(err)
	}

	patchRequest := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+project.ID+"/members/"+memberLogin.userID,
		`{"role":"reviewer"}`,
		ownerLogin,
	)
	patchResponse := mustTest(t, server, patchRequest)
	if patchResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected role update 200, got %d: %s", patchResponse.StatusCode, readBody(t, patchResponse))
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	addCookies(meRequest, memberLogin.cookies)
	meResponse := mustTest(t, server, meRequest)
	if meResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected the project-scoped role change to preserve the session, got %d: %s", meResponse.StatusCode, readBody(t, meResponse))
	}

	projectRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID, nil)
	addCookies(projectRequest, memberLogin.cookies)
	projectResponse := mustTest(t, server, projectRequest)
	if projectResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected project access through the existing session, got %d: %s", projectResponse.StatusCode, readBody(t, projectResponse))
	}
	var changedProject Envelope[store.AdminProject]
	decodeJSONResponse(t, projectResponse, &changedProject)
	if changedProject.Data.Role != "reviewer" {
		t.Fatalf("expected the existing session to observe the new reviewer role, got %q", changedProject.Data.Role)
	}

	otherProjectRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+otherProject.ID, nil)
	addCookies(otherProjectRequest, memberLogin.cookies)
	otherProjectResponse := mustTest(t, server, otherProjectRequest)
	if otherProjectResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected the role change to preserve access to other projects, got %d: %s", otherProjectResponse.StatusCode, readBody(t, otherProjectResponse))
	}

	removeRequest := newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/members/"+memberLogin.userID,
		"",
		ownerLogin,
	)
	removeResponse := mustTest(t, server, removeRequest)
	if removeResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("expected project removal 204, got %d: %s", removeResponse.StatusCode, readBody(t, removeResponse))
	}
	meAfterRemovalRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	addCookies(meAfterRemovalRequest, memberLogin.cookies)
	meAfterRemovalResponse := mustTest(t, server, meAfterRemovalRequest)
	if meAfterRemovalResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected project removal to preserve the global session, got %d: %s", meAfterRemovalResponse.StatusCode, readBody(t, meAfterRemovalResponse))
	}
	removedProjectRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID, nil)
	addCookies(removedProjectRequest, memberLogin.cookies)
	removedProjectResponse := mustTest(t, server, removedProjectRequest)
	if removedProjectResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected removed project access to end immediately, got %d: %s", removedProjectResponse.StatusCode, readBody(t, removedProjectResponse))
	}
	otherProjectAfterRemovalRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+otherProject.ID, nil)
	addCookies(otherProjectAfterRemovalRequest, memberLogin.cookies)
	otherProjectAfterRemovalResponse := mustTest(t, server, otherProjectAfterRemovalRequest)
	if otherProjectAfterRemovalResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected shared-directory removal to end access to every project, got %d: %s", otherProjectAfterRemovalResponse.StatusCode, readBody(t, otherProjectAfterRemovalResponse))
	}
}

func TestProjectMemberLoginDisableRevokesSessionsAndCanBeReenabled(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	memberPassword := "member correct horse password"
	memberLogin := seedAndLogin(t, server, db, "member@example.test", memberPassword)
	project := createTestProject(t, server, ownerLogin, `{"slug":"disable-login","name":"Disable Login"}`)
	otherProject := createTestProject(t, server, memberLogin, `{"slug":"member-owned","name":"Member Owned"}`)
	if _, err := db.Exec(`
		INSERT OR REPLACE INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, memberLogin.userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT OR REPLACE INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'project_owner', 'active', CURRENT_TIMESTAMP)
	`, otherProject.ID, ownerLogin.userID); err != nil {
		t.Fatal(err)
	}

	disableRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/members/"+memberLogin.userID+"/disable-login",
		"",
		ownerLogin,
	)
	disableResponse := mustTest(t, server, disableRequest)
	if disableResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected disable login 200, got %d: %s", disableResponse.StatusCode, readBody(t, disableResponse))
	}
	var disabled Envelope[store.AdminProjectMember]
	decodeJSONResponse(t, disableResponse, &disabled)
	if disabled.Data.UserStatus != "disabled" || disabled.Data.Status != "active" {
		t.Fatalf("expected disabled account with retained active membership, got %#v", disabled.Data)
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	addCookies(meRequest, memberLogin.cookies)
	meResponse := mustTest(t, server, meRequest)
	if meResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected disabled user's existing session to be revoked, got %d: %s", meResponse.StatusCode, readBody(t, meResponse))
	}

	loginBody, err := json.Marshal(loginRequest{Email: "member@example.test", Password: memberPassword})
	if err != nil {
		t.Fatal(err)
	}
	disabledLoginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	disabledLoginRequest.Header.Set("Content-Type", "application/json")
	disabledLoginResponse := mustTest(t, server, disabledLoginRequest)
	if disabledLoginResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected disabled user login to fail, got %d: %s", disabledLoginResponse.StatusCode, readBody(t, disabledLoginResponse))
	}

	var selectedMembershipStatus, otherMembershipStatus string
	if err := db.QueryRow(`
		SELECT status
		FROM project_memberships
		WHERE project_id = ? AND user_id = ?
	`, project.ID, memberLogin.userID).Scan(&selectedMembershipStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`
		SELECT status
		FROM project_memberships
		WHERE project_id = ? AND user_id = ?
	`, otherProject.ID, memberLogin.userID).Scan(&otherMembershipStatus); err != nil {
		t.Fatal(err)
	}
	if selectedMembershipStatus != "active" || otherMembershipStatus != "active" {
		t.Fatalf("expected disabling login to preserve memberships, got selected=%q other=%q", selectedMembershipStatus, otherMembershipStatus)
	}

	enableRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/members/"+memberLogin.userID+"/enable-login",
		"",
		ownerLogin,
	)
	enableResponse := mustTest(t, server, enableRequest)
	if enableResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected enable login 200, got %d: %s", enableResponse.StatusCode, readBody(t, enableResponse))
	}
	var enabled Envelope[store.AdminProjectMember]
	decodeJSONResponse(t, enableResponse, &enabled)
	if enabled.Data.UserStatus != "active" {
		t.Fatalf("expected enabled account status, got %#v", enabled.Data)
	}

	reenabledLogin := adminLogin(t, server, "member@example.test", memberPassword)
	projectRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID, nil)
	addCookies(projectRequest, reenabledLogin.cookies)
	projectResponse := mustTest(t, server, projectRequest)
	if projectResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected re-enabled user to regain retained project membership, got %d: %s", projectResponse.StatusCode, readBody(t, projectResponse))
	}
}

func TestProjectMemberLoginDisableGuardrails(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	adminLogin := seedAndLogin(t, server, db, "admin@example.test", "admin correct horse password")
	memberLogin := seedAndLogin(t, server, db, "member@example.test", "member correct horse password")
	project := createTestProject(t, server, ownerLogin, `{"slug":"disable-guardrails","name":"Disable Guardrails"}`)
	memberOwnedProject := createTestProject(t, server, memberLogin, `{"slug":"solo-owned","name":"Solo Owned"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'project_admin', 'active', CURRENT_TIMESTAMP)
	`, project.ID, adminLogin.userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, memberLogin.userID); err != nil {
		t.Fatal(err)
	}

	adminDisable := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/members/"+memberLogin.userID+"/disable-login",
		"",
		adminLogin,
	)
	adminDisableResponse := mustTest(t, server, adminDisable)
	if adminDisableResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected project admin disable-login denial, got %d: %s", adminDisableResponse.StatusCode, readBody(t, adminDisableResponse))
	}

	selfDisable := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/members/"+ownerLogin.userID+"/disable-login",
		"",
		ownerLogin,
	)
	selfDisableResponse := mustTest(t, server, selfDisable)
	if selfDisableResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected self-disable conflict, got %d: %s", selfDisableResponse.StatusCode, readBody(t, selfDisableResponse))
	}

	soloOwnerDisable := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/members/"+memberLogin.userID+"/disable-login",
		"",
		ownerLogin,
	)
	soloOwnerDisableResponse := mustTest(t, server, soloOwnerDisable)
	if soloOwnerDisableResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected shared owners to prevent project %s from being left ownerless, got %d: %s", memberOwnedProject.ID, soloOwnerDisableResponse.StatusCode, readBody(t, soloOwnerDisableResponse))
	}
}

func TestMembershipAuthorizationAndSharedProjectDirectory(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerALogin := seedAndLogin(t, server, db, "owner-a@example.test", "owner a correct password")
	ownerBLogin := seedAndLogin(t, server, db, "owner-b@example.test", "owner b correct password")
	writerLogin := seedAndLogin(t, server, db, "writer@example.test", "writer correct password")
	projectA := createTestProject(t, server, ownerALogin, `{"slug":"project-a","name":"Project A"}`)
	projectB := createTestProject(t, server, ownerBLogin, `{"slug":"project-b","name":"Project B"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, projectA.ID, writerLogin.userID); err != nil {
		t.Fatal(err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectA.ID+"/members", nil)
	addCookies(listRequest, writerLogin.cookies)
	listResponse := mustTest(t, server, listRequest)
	if listResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer member list denial, got %d: %s", listResponse.StatusCode, readBody(t, listResponse))
	}

	inviteRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectA.ID+"/invitations",
		`{"email":"another@example.test","role":"writer"}`,
		writerLogin,
	)
	inviteResponse := mustTest(t, server, inviteRequest)
	if inviteResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer invitation denial, got %d: %s", inviteResponse.StatusCode, readBody(t, inviteResponse))
	}

	crossProjectRequest := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+projectB.ID+"/members/"+ownerBLogin.userID,
		`{"role":"editor"}`,
		ownerALogin,
	)
	crossProjectResponse := mustTest(t, server, crossProjectRequest)
	if crossProjectResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected an owner to manage the shared member directory from either project, got %d: %s", crossProjectResponse.StatusCode, readBody(t, crossProjectResponse))
	}
}

func TestProjectMembersCursorPagination(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, ownerLogin, `{"slug":"pagination","name":"Member Pagination"}`)
	for index := 0; index < 55; index++ {
		userID := fmt.Sprintf("usr_page_%03d", index)
		email := fmt.Sprintf("page-%03d@example.test", index)
		if _, err := db.Exec(`
			INSERT INTO users(id, email_normalized, status)
			VALUES (?, ?, 'invited')
		`, userID, email); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`
			INSERT INTO project_memberships(project_id, user_id, role, status, invited_by, invited_at)
			VALUES (?, ?, 'writer', 'invited', ?, CURRENT_TIMESTAMP)
		`, project.ID, userID, ownerLogin.userID); err != nil {
			t.Fatal(err)
		}
	}

	cursor := ""
	seen := map[string]bool{}
	for {
		path := "/api/v1/projects/" + project.ID + "/members?limit=20"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		request := httptest.NewRequest(http.MethodGet, path, nil)
		addCookies(request, ownerLogin.cookies)
		response := mustTest(t, server, request)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("expected paginated member list 200, got %d: %s", response.StatusCode, readBody(t, response))
		}
		var page ListEnvelope[store.AdminProjectMember]
		decodeJSONResponse(t, response, &page)
		for _, member := range page.Data {
			if seen[member.UserID] {
				t.Fatalf("member %q appeared in more than one page", member.UserID)
			}
			seen[member.UserID] = true
		}
		cursor = page.Meta.NextCursor
		if cursor == "" {
			break
		}
	}
	if len(seen) != 56 {
		t.Fatalf("expected owner plus 55 invited members, got %d", len(seen))
	}
}

func TestInvitationRateLimitSeparatesRecipients(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, ownerLogin, `{"slug":"rate-limit","name":"Invitation Rate Limit"}`)

	for attempt := 1; attempt <= 6; attempt++ {
		request := newMemberMutationRequest(
			http.MethodPost,
			"/api/v1/projects/"+project.ID+"/invitations",
			`{"email":"limited@example.test","role":"writer"}`,
			ownerLogin,
		)
		response := mustTest(t, server, request)
		expected := http.StatusCreated
		if attempt == 6 {
			expected = http.StatusTooManyRequests
		}
		if response.StatusCode != expected {
			t.Fatalf("attempt %d: expected %d, got %d: %s", attempt, expected, response.StatusCode, readBody(t, response))
		}
	}

	otherRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/invitations",
		`{"email":"other@example.test","role":"writer"}`,
		ownerLogin,
	)
	otherResponse := mustTest(t, server, otherRequest)
	if otherResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected a separate recipient identity to remain available, got %d: %s", otherResponse.StatusCode, readBody(t, otherResponse))
	}
}

func TestInvitationSourceRateLimitIgnoresUntrustedForwardedFor(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, ownerLogin, `{"slug":"source-rate-limit","name":"Source Rate Limit"}`)

	for attempt := 1; attempt <= 31; attempt++ {
		request := newMemberMutationRequest(
			http.MethodPost,
			"/api/v1/projects/"+project.ID+"/invitations",
			fmt.Sprintf(`{"email":"recipient-%02d@example.test","role":"writer"}`, attempt),
			ownerLogin,
		)
		request.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", attempt))
		response := mustTest(t, server, request)
		expected := http.StatusCreated
		if attempt == 31 {
			expected = http.StatusTooManyRequests
		}
		if response.StatusCode != expected {
			t.Fatalf("attempt %d: expected %d, got %d: %s", attempt, expected, response.StatusCode, readBody(t, response))
		}
	}
}

func TestConcurrentOwnerRemovalRetainsOneOwner(t *testing.T) {
	server, db := newAdminTestServer(t)
	firstLogin := seedAndLogin(t, server, db, "first@example.test", "first owner correct password")
	secondLogin := seedAndLogin(t, server, db, "second@example.test", "second owner correct password")
	project := createTestProject(t, server, firstLogin, `{"slug":"concurrent-owners","name":"Concurrent Owners"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'project_owner', 'active', CURRENT_TIMESTAMP)
	`, project.ID, secondLogin.userID); err != nil {
		t.Fatal(err)
	}

	requests := []*http.Request{
		newMemberMutationRequest(http.MethodDelete, "/api/v1/projects/"+project.ID+"/members/"+secondLogin.userID, "", firstLogin),
		newMemberMutationRequest(http.MethodDelete, "/api/v1/projects/"+project.ID+"/members/"+firstLogin.userID, "", secondLogin),
	}
	statuses := make(chan int, len(requests))
	var wait sync.WaitGroup
	for _, request := range requests {
		wait.Add(1)
		go func(request *http.Request) {
			defer wait.Done()
			response, err := server.app.Test(request, fiber.TestConfig{Timeout: 15 * time.Second})
			if err != nil {
				statuses <- 0
				return
			}
			defer response.Body.Close()
			statuses <- response.StatusCode
		}(request)
	}
	wait.Wait()
	close(statuses)

	successes := 0
	for status := range statuses {
		if status == http.StatusNoContent {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one concurrent owner removal to succeed, got %d", successes)
	}
	var ownerCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM project_memberships
		WHERE project_id = ? AND role = 'project_owner' AND status = 'active'
	`, project.ID).Scan(&ownerCount); err != nil {
		t.Fatal(err)
	}
	if ownerCount != 1 {
		t.Fatalf("expected one active owner after concurrent removals, got %d", ownerCount)
	}
}

func TestAdminLogoutRevokesSession(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(logoutRequest, login.cookies)
	logoutResponse := mustTest(t, server, logoutRequest)
	if logoutResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("expected logout 204, got %d", logoutResponse.StatusCode)
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	addCookies(meRequest, login.cookies)
	meResponse := mustTest(t, server, meRequest)
	if meResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked session to fail with 401, got %d", meResponse.StatusCode)
	}
}

func TestProjectAPIKeyLifecycleAndContentAuth(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"keys","name":"Keys Project"}`)
	_ = createTestCategory(t, server, login, project.ID, `{"slug":"docs","name":"Docs"}`)

	first := createTestAPIKey(t, server, login, project.ID, `{"environment":"production","name":"production build"}`)
	if first.Data.Secret == "" {
		t.Fatal("expected raw API key secret at creation")
	}
	if first.Data.Key.Status != "active" {
		t.Fatalf("expected a newly created key to be active, got %q", first.Data.Key.Status)
	}
	if !strings.HasPrefix(first.Data.Secret, first.Data.Key.TokenPrefix) {
		t.Fatalf("expected token prefix to match secret prefix")
	}

	var storedHash string
	if err := db.QueryRow(`SELECT token_hash FROM project_api_keys WHERE id = ?`, first.Data.Key.ID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash == "" || storedHash == first.Data.Secret {
		t.Fatalf("expected only a verifier to be stored, got %q", storedHash)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/api-keys", nil)
	addCookies(listRequest, login.cookies)
	listResponse := mustTest(t, server, listRequest)
	listBody := readBody(t, listResponse)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected API key list 200, got %d: %s", listResponse.StatusCode, listBody)
	}
	if strings.Contains(listBody, first.Data.Secret) || strings.Contains(listBody, `"secret"`) {
		t.Fatalf("expected list response to omit raw secrets, got %s", listBody)
	}

	assertContentCategoriesStatus(t, server, first.Data.Secret, http.StatusOK)

	rotatedRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/api-keys/"+first.Data.Key.ID+"/rotate", nil)
	rotatedRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(rotatedRequest, login.cookies)
	rotatedResponse := mustTest(t, server, rotatedRequest)
	if rotatedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected rotate key 200, got %d: %s", rotatedResponse.StatusCode, readBody(t, rotatedResponse))
	}
	var rotated Envelope[store.APIKeyWithSecret]
	decodeJSONResponse(t, rotatedResponse, &rotated)
	if rotated.Data.Secret == "" || rotated.Data.Secret == first.Data.Secret {
		t.Fatal("expected rotation to return a new one-time secret")
	}
	if rotated.Data.Key.ID == first.Data.Key.ID {
		t.Fatal("expected rotation to create a replacement key")
	}
	assertContentCategoriesStatus(t, server, first.Data.Secret, http.StatusOK)
	assertContentCategoriesStatus(t, server, rotated.Data.Secret, http.StatusOK)

	second := createTestAPIKey(t, server, login, project.ID, `{"environment":"production","name":"production runtime"}`)
	assertContentCategoriesStatus(t, server, second.Data.Secret, http.StatusOK)

	revokeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/api-keys/"+first.Data.Key.ID+"/revoke", nil)
	revokeRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(revokeRequest, login.cookies)
	revokeResponse := mustTest(t, server, revokeRequest)
	if revokeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected revoke key 200, got %d: %s", revokeResponse.StatusCode, readBody(t, revokeResponse))
	}
	var revoked Envelope[store.AdminAPIKey]
	decodeJSONResponse(t, revokeResponse, &revoked)
	if revoked.Data.RevokedAt == "" {
		t.Fatal("expected revoked key response to include revokedAt")
	}
	if revoked.Data.Status != "revoked" {
		t.Fatalf("expected revoked key status, got %q", revoked.Data.Status)
	}
	assertContentCategoriesStatus(t, server, first.Data.Secret, http.StatusUnauthorized)
	assertContentCategoriesStatus(t, server, rotated.Data.Secret, http.StatusOK)
	assertContentCategoriesStatus(t, server, second.Data.Secret, http.StatusOK)

	var rotationAuditCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND action = 'api_key.rotate'
		  AND target_id = ?
		  AND json_extract(metadata_json, '$.replacesKeyId') = ?
	`, project.ID, rotated.Data.Key.ID, first.Data.Key.ID).Scan(&rotationAuditCount); err != nil {
		t.Fatal(err)
	}
	if rotationAuditCount != 1 {
		t.Fatalf("expected one rotation audit event, got %d", rotationAuditCount)
	}
}

func TestProjectAuditEventsHaveIDsAndOmitSecrets(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	writerLogin := seedAndLogin(t, server, db, "writer@example.test", "writer correct password")
	project := createTestProject(t, server, login, `{"slug":"audit","name":"Audit Project"}`)
	const invitationRequestID = "request-audited-invitation"
	invitationRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/invitations",
		`{"email":"audited@example.test","role":"writer"}`,
		login,
	)
	invitationRequest.Header.Set("X-Request-ID", invitationRequestID)
	invitationResponse := mustTest(t, server, invitationRequest)
	if invitationResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected invitation creation 201, got %d: %s", invitationResponse.StatusCode, readBody(t, invitationResponse))
	}
	var invitationPayload Envelope[store.ProjectMemberInvitation]
	decodeJSONResponse(t, invitationResponse, &invitationPayload)
	invitation := invitationPayload.Data
	apiKey := createTestAPIKey(t, server, login, project.ID, `{"environment":"production","name":"audited key"}`)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/audit-events?limit=2", nil)
	addCookies(request, login.cookies)
	response := mustTest(t, server, request)
	body := readBody(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected audit list 200, got %d: %s", response.StatusCode, body)
	}
	if strings.Contains(body, invitation.Token) || strings.Contains(body, apiKey.Data.Secret) {
		t.Fatalf("audit response leaked a one-time secret: %s", body)
	}
	if strings.Contains(body, security.TokenHash(invitation.Token)) || strings.Contains(body, security.TokenHash(apiKey.Data.Secret)) {
		t.Fatalf("audit response leaked a token verifier: %s", body)
	}

	var firstPage ListEnvelope[store.AuditEvent]
	if err := json.Unmarshal([]byte(body), &firstPage); err != nil {
		t.Fatal(err)
	}
	if len(firstPage.Data) != 2 || firstPage.Meta.NextCursor == "" {
		t.Fatalf("expected first audit page to have two rows and a cursor, got %#v", firstPage)
	}
	for _, event := range firstPage.Data {
		if event.ID == "" {
			t.Fatalf("expected audit event IDs, got %#v", event)
		}
		if event.ProjectID != project.ID {
			t.Fatalf("expected project-scoped audit event, got %#v", event)
		}
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/audit-events?cursor="+firstPage.Meta.NextCursor, nil)
	addCookies(secondRequest, login.cookies)
	secondResponse := mustTest(t, server, secondRequest)
	if secondResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected second audit page 200, got %d: %s", secondResponse.StatusCode, readBody(t, secondResponse))
	}
	var secondPage ListEnvelope[store.AuditEvent]
	decodeJSONResponse(t, secondResponse, &secondPage)

	actions := map[string]bool{}
	requestIDs := map[string]bool{}
	for _, event := range append(firstPage.Data, secondPage.Data...) {
		actions[event.Action] = true
		requestIDs[event.RequestID] = true
	}
	for _, action := range []string{"project.create", "member.invite", "api_key.create"} {
		if !actions[action] {
			t.Fatalf("expected audit action %q in %#v", action, actions)
		}
	}
	if !requestIDs[invitationRequestID] {
		t.Fatalf("expected invitation audit request ID %q in %#v", invitationRequestID, requestIDs)
	}

	_, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, writerLogin.userID)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/audit-events", nil)
	addCookies(forbidden, writerLogin.cookies)
	forbiddenResponse := mustTest(t, server, forbidden)
	if forbiddenResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer audit read denial with 403, got %d: %s", forbiddenResponse.StatusCode, readBody(t, forbiddenResponse))
	}
}

func TestAuditedProjectMutationsRollbackWhenAuditInsertionFails(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"audit-rollback","name":"Original Name"}`)

	if _, err := db.Exec(`
		CREATE TRIGGER fail_project_update_audit
		BEFORE INSERT ON audit_events
		WHEN NEW.action = 'project.update'
		BEGIN
			SELECT RAISE(ABORT, 'forced audit failure');
		END
	`); err != nil {
		t.Fatal(err)
	}
	updateRequest := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+project.ID,
		`{"name":"Changed Name"}`,
		login,
	)
	updateResponse := mustTest(t, server, updateRequest)
	if updateResponse.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected failed audit insertion to fail project update, got %d: %s", updateResponse.StatusCode, readBody(t, updateResponse))
	}
	var storedName string
	if err := db.QueryRow(`SELECT name FROM projects WHERE id = ?`, project.ID).Scan(&storedName); err != nil {
		t.Fatal(err)
	}
	if storedName != "Original Name" {
		t.Fatalf("expected project update rollback, got name %q", storedName)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_project_update_audit`); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(`
		CREATE TRIGGER fail_project_delete_audit
		BEFORE INSERT ON audit_events
		WHEN NEW.action = 'project.delete'
		BEGIN
			SELECT RAISE(ABORT, 'forced audit failure');
		END
	`); err != nil {
		t.Fatal(err)
	}
	deleteRequest := newMemberMutationRequest(http.MethodDelete, "/api/v1/projects/"+project.ID, "", login)
	deleteResponse := mustTest(t, server, deleteRequest)
	if deleteResponse.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected failed audit insertion to fail project deletion, got %d: %s", deleteResponse.StatusCode, readBody(t, deleteResponse))
	}
	var projectCount int
	if err := db.QueryRow(`SELECT COUNT(1) FROM projects WHERE id = ?`, project.ID).Scan(&projectCount); err != nil {
		t.Fatal(err)
	}
	if projectCount != 1 {
		t.Fatal("expected project deletion to roll back when audit insertion fails")
	}
}

func TestWriterCannotManageProjectAPIKeys(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	writerLogin := seedAndLogin(t, server, db, "writer@example.test", "another correct horse battery staple")
	project := createTestProject(t, server, ownerLogin, `{"slug":"writer-denied","name":"Writer Denied"}`)

	_, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, writerLogin.userID)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/api-keys", strings.NewReader(`{"environment":"production","name":"should fail"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", writerLogin.csrfToken)
	addCookies(request, writerLogin.cookies)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer key creation to fail with 403, got %d: %s", response.StatusCode, readBody(t, response))
	}
}

func TestAdminCategoryUpdateLifecycleAndHierarchy(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerALogin := seedAndLogin(t, server, db, "owner-a@example.test", "owner a correct password")
	ownerBLogin := seedAndLogin(t, server, db, "owner-b@example.test", "owner b correct password")
	writerLogin := seedAndLogin(t, server, db, "writer@example.test", "writer correct password")
	projectA := createTestProject(t, server, ownerALogin, `{"slug":"taxonomy-a","name":"Taxonomy A"}`)
	projectB := createTestProject(t, server, ownerBLogin, `{"slug":"taxonomy-b","name":"Taxonomy B"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, projectA.ID, writerLogin.userID); err != nil {
		t.Fatal(err)
	}

	root := createTestCategory(t, server, ownerALogin, projectA.ID, `{"slug":"root","name":"Root"}`)
	child := createTestCategory(t, server, ownerALogin, projectA.ID, `{"slug":"child","name":"Child","parentId":"`+root.ID+`"}`)
	crossProjectParent := createTestCategory(t, server, ownerBLogin, projectB.ID, `{"slug":"elsewhere","name":"Elsewhere"}`)

	writerPatch := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+projectA.ID+"/categories/"+child.ID,
		`{"name":"Writer Edited"}`,
		writerLogin,
	)
	writerPatchResponse := mustTest(t, server, writerPatch)
	if writerPatchResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer category update denial, got %d: %s", writerPatchResponse.StatusCode, readBody(t, writerPatchResponse))
	}

	update := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+projectA.ID+"/categories/"+child.ID,
		`{"name":"Technical Guides","slug":"Technical Guides","description":"Evergreen technical work","indexable":false}`,
		ownerALogin,
	)
	updateResponse := mustTest(t, server, update)
	if updateResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected category update 200, got %d: %s", updateResponse.StatusCode, readBody(t, updateResponse))
	}
	var updated Envelope[store.TaxonomyTerm]
	decodeJSONResponse(t, updateResponse, &updated)
	if updated.Data.Slug != "technical-guides" || updated.Data.ParentID != root.ID || updated.Data.Indexable {
		t.Fatalf("unexpected updated category %#v", updated.Data)
	}
	if len(updated.Data.Ancestors) != 1 || updated.Data.Ancestors[0].ID != root.ID {
		t.Fatalf("expected category update response to include its ancestor path, got %#v", updated.Data.Ancestors)
	}

	oldSlugRequest := httptest.NewRequest(http.MethodGet, "/content/v1/categories/child", nil)
	oldSlugRequest.Header.Set("X-Dev-Project-ID", projectA.ID)
	oldSlugResponse := mustTest(t, server, oldSlugRequest)
	if oldSlugResponse.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("expected old category slug to redirect with 301, got %d: %s", oldSlugResponse.StatusCode, readBody(t, oldSlugResponse))
	}
	if location := oldSlugResponse.Header.Get("Location"); location != "/content/v1/categories/technical-guides" {
		t.Fatalf("expected old category slug redirect location, got %q", location)
	}

	crossProjectMove := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+projectA.ID+"/categories/"+child.ID,
		`{"parentId":"`+crossProjectParent.ID+`"}`,
		ownerALogin,
	)
	crossProjectMoveResponse := mustTest(t, server, crossProjectMove)
	if crossProjectMoveResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected cross-project parent to fail with 400, got %d: %s", crossProjectMoveResponse.StatusCode, readBody(t, crossProjectMoveResponse))
	}

	grandchild := createTestCategory(t, server, ownerALogin, projectA.ID, `{"slug":"grandchild","name":"Grandchild","parentId":"`+child.ID+`"}`)
	fourthLevel := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectA.ID+"/categories",
		`{"slug":"fourth","name":"Fourth","parentId":"`+grandchild.ID+`"}`,
		ownerALogin,
	)
	fourthLevelResponse := mustTest(t, server, fourthLevel)
	if fourthLevelResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected fourth hierarchy level to fail with 400, got %d: %s", fourthLevelResponse.StatusCode, readBody(t, fourthLevelResponse))
	}

	updateRoot := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+projectA.ID+"/categories/"+root.ID,
		`{"description":"Top-level category"}`,
		ownerALogin,
	)
	updateRootResponse := mustTest(t, server, updateRoot)
	if updateRootResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected root category update 200, got %d: %s", updateRootResponse.StatusCode, readBody(t, updateRootResponse))
	}
	var updatedRoot Envelope[store.TaxonomyTerm]
	decodeJSONResponse(t, updateRootResponse, &updatedRoot)
	if len(updatedRoot.Data.Children) != 1 || updatedRoot.Data.Children[0].ID != child.ID {
		t.Fatalf("expected category update response to include direct children, got %#v", updatedRoot.Data.Children)
	}

	cycleMove := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+projectA.ID+"/categories/"+root.ID,
		`{"parentId":"`+grandchild.ID+`"}`,
		ownerALogin,
	)
	cycleMoveResponse := mustTest(t, server, cycleMove)
	if cycleMoveResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected category cycle to fail with 400, got %d: %s", cycleMoveResponse.StatusCode, readBody(t, cycleMoveResponse))
	}

	duplicateSlug := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+projectA.ID+"/categories/"+root.ID,
		`{"slug":"technical-guides"}`,
		ownerALogin,
	)
	duplicateSlugResponse := mustTest(t, server, duplicateSlug)
	if duplicateSlugResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected duplicate category slug to fail with 400, got %d: %s", duplicateSlugResponse.StatusCode, readBody(t, duplicateSlugResponse))
	}

	reuseHistoricalSlug := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+projectA.ID+"/categories/"+root.ID,
		`{"slug":"child"}`,
		ownerALogin,
	)
	reuseHistoricalSlugResponse := mustTest(t, server, reuseHistoricalSlug)
	if reuseHistoricalSlugResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected historical category slug reuse to fail with 400, got %d: %s", reuseHistoricalSlugResponse.StatusCode, readBody(t, reuseHistoricalSlugResponse))
	}

	createWithHistoricalSlug := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectA.ID+"/categories",
		`{"slug":"child","name":"Reused Child"}`,
		ownerALogin,
	)
	createWithHistoricalSlugResponse := mustTest(t, server, createWithHistoricalSlug)
	if createWithHistoricalSlugResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected category creation with a historical slug to fail with 400, got %d: %s", createWithHistoricalSlugResponse.StatusCode, readBody(t, createWithHistoricalSlugResponse))
	}

	publishedCategories := httptest.NewRequest(http.MethodGet, "/content/v1/categories", nil)
	publishedCategories.Header.Set("X-Dev-Project-ID", projectA.ID)
	publishedCategoriesResponse := mustTest(t, server, publishedCategories)
	if publishedCategoriesResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected content category list 200, got %d: %s", publishedCategoriesResponse.StatusCode, readBody(t, publishedCategoriesResponse))
	}
	var categoryList ListEnvelope[store.TaxonomyTerm]
	decodeJSONResponse(t, publishedCategoriesResponse, &categoryList)
	listed := findTaxonomyTerm(categoryList.Data, child.ID)
	if listed.Slug != "technical-guides" || listed.ParentID != root.ID || listed.Indexable {
		t.Fatalf("expected updated category in content API, got %#v", listed)
	}

	redirectsRequest := httptest.NewRequest(http.MethodGet, "/content/v1/redirects", nil)
	redirectsRequest.Header.Set("X-Dev-Project-ID", projectA.ID)
	redirectsResponse := mustTest(t, server, redirectsRequest)
	if redirectsResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected redirect list 200, got %d: %s", redirectsResponse.StatusCode, readBody(t, redirectsResponse))
	}
	var redirects ListEnvelope[store.RedirectRecord]
	decodeJSONResponse(t, redirectsResponse, &redirects)
	foundCategoryRedirect := false
	for _, redirect := range redirects.Data {
		if redirect.SourcePath == "/categories/child" && redirect.TargetPath == "/categories/technical-guides" && redirect.StatusCode == http.StatusMovedPermanently {
			foundCategoryRedirect = true
			break
		}
	}
	if !foundCategoryRedirect {
		t.Fatalf("expected category slug redirect in redirect manifest, got %#v", redirects.Data)
	}

	changesRequest := httptest.NewRequest(http.MethodGet, "/content/v1/changes", nil)
	changesRequest.Header.Set("X-Dev-Project-ID", projectA.ID)
	changesResponse := mustTest(t, server, changesRequest)
	if changesResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected changes list 200, got %d: %s", changesResponse.StatusCode, readBody(t, changesResponse))
	}
	var changes ListEnvelope[store.ChangeRecord]
	decodeJSONResponse(t, changesResponse, &changes)
	changeTypes := map[string]bool{}
	for _, change := range changes.Data {
		if change.AggregateID == child.ID {
			changeTypes[change.Type] = true
		}
	}
	if !changeTypes["taxonomy.created"] || !changeTypes["taxonomy.updated"] {
		t.Fatalf("expected taxonomy create and update change events for child, got %#v", changes.Data)
	}

	var contentGeneration int64
	if err := db.QueryRow(`SELECT content_generation FROM projects WHERE id = ?`, projectA.ID).Scan(&contentGeneration); err != nil {
		t.Fatal(err)
	}
	if contentGeneration != 6 {
		t.Fatalf("expected successful taxonomy writes to advance content generation to 6, got %d", contentGeneration)
	}

	var auditCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND target_id = ?
		  AND action IN ('taxonomy.create', 'taxonomy.update')
	`, projectA.ID, child.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 2 {
		t.Fatalf("expected category create and update audit events, got %d", auditCount)
	}
}

func TestCategorySlugRedirectsRemainOneHop(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "owner correct password")
	project := createTestProject(t, server, ownerLogin, `{"slug":"redirects","name":"Redirects"}`)
	category := createTestCategory(t, server, ownerLogin, project.ID, `{"slug":"original","name":"Original"}`)

	for _, slug := range []string{"second", "final"} {
		request := newMemberMutationRequest(
			http.MethodPatch,
			"/api/v1/projects/"+project.ID+"/categories/"+category.ID,
			`{"slug":"`+slug+`"}`,
			ownerLogin,
		)
		response := mustTest(t, server, request)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("expected category rename to %q to return 200, got %d: %s", slug, response.StatusCode, readBody(t, response))
		}
	}

	rows, err := db.Query(`
		SELECT source_path, target_path
		FROM slug_redirects
		WHERE project_id = ?
		ORDER BY source_path
	`, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	redirects := map[string]string{}
	for rows.Next() {
		var sourcePath, targetPath string
		if err := rows.Scan(&sourcePath, &targetPath); err != nil {
			t.Fatal(err)
		}
		redirects[sourcePath] = targetPath
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{
		"/categories/original": "/categories/final",
		"/categories/second":   "/categories/final",
	}
	if !reflect.DeepEqual(redirects, expected) {
		t.Fatalf("expected one-hop category redirects %#v, got %#v", expected, redirects)
	}

	for _, slug := range []string{"original", "second"} {
		request := httptest.NewRequest(http.MethodGet, "/content/v1/categories/"+slug, nil)
		request.Header.Set("X-Dev-Project-ID", project.ID)
		response := mustTest(t, server, request)
		if response.StatusCode != http.StatusMovedPermanently {
			t.Fatalf("expected historical slug %q to redirect with 301, got %d: %s", slug, response.StatusCode, readBody(t, response))
		}
		if location := response.Header.Get("Location"); location != "/content/v1/categories/final" {
			t.Fatalf("expected historical slug %q to redirect directly to final slug, got %q", slug, location)
		}
	}
}

func TestAdminSeriesCreateAndPublishedVisibility(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "owner correct password")
	writerLogin := seedAndLogin(t, server, db, "writer@example.test", "writer correct password")
	project := createTestProject(t, server, ownerLogin, `{"slug":"series","name":"Series Project"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, writerLogin.userID); err != nil {
		t.Fatal(err)
	}

	writerList := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/series", nil)
	addCookies(writerList, writerLogin.cookies)
	writerListResponse := mustTest(t, server, writerList)
	if writerListResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected writer series list 200, got %d: %s", writerListResponse.StatusCode, readBody(t, writerListResponse))
	}

	writerCreate := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/series",
		`{"name":"Writer Series"}`,
		writerLogin,
	)
	writerCreateResponse := mustTest(t, server, writerCreate)
	if writerCreateResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer series creation denial, got %d: %s", writerCreateResponse.StatusCode, readBody(t, writerCreateResponse))
	}

	created := createTestSeries(t, server, ownerLogin, project.ID, `{"name":"Getting Started","slug":"Getting Started","description":"Core sequence","indexable":false}`)
	if created.Slug != "getting-started" || created.Description != "Core sequence" || created.Indexable {
		t.Fatalf("unexpected created series %#v", created)
	}

	duplicateCreate := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/series",
		`{"name":"Duplicate","slug":"getting-started"}`,
		ownerLogin,
	)
	duplicateCreateResponse := mustTest(t, server, duplicateCreate)
	if duplicateCreateResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected duplicate series slug to fail with 400, got %d: %s", duplicateCreateResponse.StatusCode, readBody(t, duplicateCreateResponse))
	}

	adminList := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/series", nil)
	addCookies(adminList, ownerLogin.cookies)
	adminListResponse := mustTest(t, server, adminList)
	if adminListResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected admin series list 200, got %d: %s", adminListResponse.StatusCode, readBody(t, adminListResponse))
	}
	var adminSeries ListEnvelope[store.Series]
	decodeJSONResponse(t, adminListResponse, &adminSeries)
	if findSeries(adminSeries.Data, created.ID).ID == "" {
		t.Fatalf("expected admin list to include created series, got %#v", adminSeries.Data)
	}

	publishedSeries := httptest.NewRequest(http.MethodGet, "/content/v1/series", nil)
	publishedSeries.Header.Set("X-Dev-Project-ID", project.ID)
	publishedSeriesResponse := mustTest(t, server, publishedSeries)
	if publishedSeriesResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected content series list 200, got %d: %s", publishedSeriesResponse.StatusCode, readBody(t, publishedSeriesResponse))
	}
	var contentSeries ListEnvelope[store.Series]
	decodeJSONResponse(t, publishedSeriesResponse, &contentSeries)
	listed := findSeries(contentSeries.Data, created.ID)
	if listed.Slug != "getting-started" || listed.Indexable {
		t.Fatalf("expected created series in content API, got %#v", listed)
	}

	publishedSeriesDetail := httptest.NewRequest(http.MethodGet, "/content/v1/series/getting-started", nil)
	publishedSeriesDetail.Header.Set("X-Dev-Project-ID", project.ID)
	publishedSeriesDetailResponse := mustTest(t, server, publishedSeriesDetail)
	if publishedSeriesDetailResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected content series detail 200, got %d: %s", publishedSeriesDetailResponse.StatusCode, readBody(t, publishedSeriesDetailResponse))
	}

	changesRequest := httptest.NewRequest(http.MethodGet, "/content/v1/changes", nil)
	changesRequest.Header.Set("X-Dev-Project-ID", project.ID)
	changesResponse := mustTest(t, server, changesRequest)
	if changesResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected changes list 200, got %d: %s", changesResponse.StatusCode, readBody(t, changesResponse))
	}
	var changes ListEnvelope[store.ChangeRecord]
	decodeJSONResponse(t, changesResponse, &changes)
	changeTypes := map[string]bool{}
	for _, change := range changes.Data {
		if change.AggregateID == created.ID {
			changeTypes[change.Type] = true
		}
	}
	if !changeTypes["series.created"] {
		t.Fatalf("expected series create change event, got %#v", changes.Data)
	}

	var contentGeneration int64
	if err := db.QueryRow(`SELECT content_generation FROM projects WHERE id = ?`, project.ID).Scan(&contentGeneration); err != nil {
		t.Fatal(err)
	}
	if contentGeneration != 2 {
		t.Fatalf("expected series creation to advance content generation to 2, got %d", contentGeneration)
	}

	var auditCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND target_id = ?
		  AND action = 'series.create'
	`, project.ID, created.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("expected one series create audit event, got %d", auditCount)
	}
}

func TestAdminAuthorLifecycleAndPublishedVisibility(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"authors","name":"Authors Project"}`)
	if _, err := db.Exec(`
		INSERT INTO assets(
		  id, project_id, object_key, filename, mime_type, byte_size, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "asset-priya", project.ID, "authors/priya.jpg", "priya.jpg", "image/jpeg", 1024, login.userID); err != nil {
		t.Fatal(err)
	}

	author := createTestAuthor(t, server, login, project.ID, `{
		"displayName":"Priya Shah",
		"slug":"Priya Shah",
		"loginUserId":"`+login.userID+`",
		"shortBio":"Search strategist",
		"fullBio":"Priya leads evidence-backed search programs.",
		"photoAssetId":"asset-priya",
		"jobTitle":"Principal SEO",
		"organization":"Example Co",
		"credentials":["MBA","MBA","Technical SEO"],
		"expertise":["Search","Content"],
		"profileUrl":"https://example.test/authors/priya",
		"externalProfiles":["https://www.linkedin.com/in/priya"],
		"sameAs":["https://example.test/team/priya"]
	}`)
	if author.Slug != "priya-shah" {
		t.Fatalf("expected normalized author slug, got %q", author.Slug)
	}
	if author.Status != "active" {
		t.Fatalf("expected active author, got %q", author.Status)
	}
	if len(author.Credentials) != 2 {
		t.Fatalf("expected credentials to be cleaned and de-duplicated, got %#v", author.Credentials)
	}
	if author.PhotoAssetID != "asset-priya" {
		t.Fatalf("expected author photo asset to round-trip, got %q", author.PhotoAssetID)
	}
	if author.LoginUserID != login.userID || author.LoginEmail != "owner@example.test" || author.LoginRole != "project_owner" || author.LoginStatus != "active" {
		t.Fatalf("expected author to link owner login metadata, got %#v", author)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/authors/"+author.ID, nil)
	addCookies(getRequest, login.cookies)
	getResponse := mustTest(t, server, getRequest)
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected author detail 200, got %d: %s", getResponse.StatusCode, readBody(t, getResponse))
	}
	var detail Envelope[store.Author]
	decodeJSONResponse(t, getResponse, &detail)
	if detail.Data.ID != author.ID || detail.Data.LoginUserID != login.userID {
		t.Fatalf("expected author detail with linked login, got %#v", detail.Data)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/authors", nil)
	addCookies(listRequest, login.cookies)
	listResponse := mustTest(t, server, listRequest)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected author list 200, got %d: %s", listResponse.StatusCode, readBody(t, listResponse))
	}
	var list ListEnvelope[store.Author]
	decodeJSONResponse(t, listResponse, &list)
	if len(list.Data) != 1 || list.Data[0].ID != author.ID || list.Data[0].LoginEmail != "owner@example.test" {
		t.Fatalf("expected author list to include created author, got %#v", list.Data)
	}

	publishedRequest := httptest.NewRequest(http.MethodGet, "/content/v1/authors", nil)
	publishedRequest.Header.Set("X-Dev-Project-ID", project.ID)
	publishedResponse := mustTest(t, server, publishedRequest)
	if publishedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected published author list 200, got %d: %s", publishedResponse.StatusCode, readBody(t, publishedResponse))
	}
	var published ListEnvelope[store.Author]
	decodeJSONResponse(t, publishedResponse, &published)
	if len(published.Data) != 1 ||
		published.Data[0].ProfileURL != "https://example.test/authors/priya" ||
		published.Data[0].PhotoAssetID != "asset-priya" ||
		published.Data[0].LoginUserID != "" ||
		published.Data[0].LoginEmail != "" {
		t.Fatalf("expected active author in content API, got %#v", published.Data)
	}

	patchRequest := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+project.ID+"/authors/"+author.ID,
		`{"displayName":"Priya S.","photoAssetId":"","sameAs":["https://example.test/people/priya"]}`,
		login,
	)
	patchResponse := mustTest(t, server, patchRequest)
	if patchResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected author update 200, got %d: %s", patchResponse.StatusCode, readBody(t, patchResponse))
	}
	var patched Envelope[store.Author]
	decodeJSONResponse(t, patchResponse, &patched)
	if patched.Data.DisplayName != "Priya S." || patched.Data.Status != "active" || patched.Data.PhotoAssetID != "" || patched.Data.LoginUserID != login.userID {
		t.Fatalf("expected patched active author, got %#v", patched.Data)
	}

	deleteRequest := newMemberMutationRequest(http.MethodDelete, "/api/v1/projects/"+project.ID+"/authors/"+author.ID, `{}`, login)
	deleteResponse := mustTest(t, server, deleteRequest)
	if deleteResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected author delete 200, got %d: %s", deleteResponse.StatusCode, readBody(t, deleteResponse))
	}
	var deleted Envelope[store.Author]
	decodeJSONResponse(t, deleteResponse, &deleted)
	if deleted.Data.ID != author.ID || deleted.Data.Status != "inactive" {
		t.Fatalf("expected delete to soft-inactivate author, got %#v", deleted.Data)
	}

	duplicateDeleteRequest := newMemberMutationRequest(http.MethodDelete, "/api/v1/projects/"+project.ID+"/authors/"+author.ID, `{}`, login)
	duplicateDeleteResponse := mustTest(t, server, duplicateDeleteRequest)
	if duplicateDeleteResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected duplicate author delete 200, got %d: %s", duplicateDeleteResponse.StatusCode, readBody(t, duplicateDeleteResponse))
	}
	var duplicateDeleted Envelope[store.Author]
	decodeJSONResponse(t, duplicateDeleteResponse, &duplicateDeleted)
	if duplicateDeleted.Data.ID != author.ID || duplicateDeleted.Data.Status != "inactive" {
		t.Fatalf("expected duplicate delete to return inactive author without another state change, got %#v", duplicateDeleted.Data)
	}

	inactivePublishedRequest := httptest.NewRequest(http.MethodGet, "/content/v1/authors", nil)
	inactivePublishedRequest.Header.Set("X-Dev-Project-ID", project.ID)
	inactivePublishedResponse := mustTest(t, server, inactivePublishedRequest)
	if inactivePublishedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected published author list after inactivation 200, got %d", inactivePublishedResponse.StatusCode)
	}
	var inactivePublished ListEnvelope[store.Author]
	decodeJSONResponse(t, inactivePublishedResponse, &inactivePublished)
	if len(inactivePublished.Data) != 0 {
		t.Fatalf("expected inactive authors to be hidden from content API, got %#v", inactivePublished.Data)
	}

	adminListAfterInactive := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/authors", nil)
	addCookies(adminListAfterInactive, login.cookies)
	adminListAfterInactiveResponse := mustTest(t, server, adminListAfterInactive)
	if adminListAfterInactiveResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected admin author list after inactivation 200, got %d", adminListAfterInactiveResponse.StatusCode)
	}
	var adminInactiveList ListEnvelope[store.Author]
	decodeJSONResponse(t, adminListAfterInactiveResponse, &adminInactiveList)
	if len(adminInactiveList.Data) != 1 || adminInactiveList.Data[0].Status != "inactive" {
		t.Fatalf("expected admin list to include inactive author, got %#v", adminInactiveList.Data)
	}

	changesRequest := httptest.NewRequest(http.MethodGet, "/content/v1/changes", nil)
	changesRequest.Header.Set("X-Dev-Project-ID", project.ID)
	changesResponse := mustTest(t, server, changesRequest)
	if changesResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected changes list 200, got %d: %s", changesResponse.StatusCode, readBody(t, changesResponse))
	}
	var changes ListEnvelope[store.ChangeRecord]
	decodeJSONResponse(t, changesResponse, &changes)
	changeTypes := map[string]bool{}
	for _, change := range changes.Data {
		if change.AggregateID == author.ID {
			changeTypes[change.Type] = true
		}
	}
	if !changeTypes["author.created"] || !changeTypes["author.updated"] || !changeTypes["author.deleted"] {
		t.Fatalf("expected author create, update, and delete change events, got %#v", changes.Data)
	}

	var contentGeneration int64
	if err := db.QueryRow(`
		SELECT content_generation
		FROM projects
		WHERE id = ?
	`, project.ID).Scan(&contentGeneration); err != nil {
		t.Fatal(err)
	}
	if contentGeneration != 4 {
		t.Fatalf("expected author writes to advance content generation to 4, got %d", contentGeneration)
	}

	var auditCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND target_id = ?
		  AND action IN ('author.create', 'author.update', 'author.delete')
	`, project.ID, author.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 3 {
		t.Fatalf("expected author create, update, and delete audit events, got %d", auditCount)
	}
}

func TestRevisionContributorsUseSharedAuthorsAndRemainOrderedAndImmutable(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "owner correct horse password")
	otherOwnerLogin := seedAndLogin(t, server, db, "other-owner@example.test", "other owner correct password")
	project := createTestProject(t, server, ownerLogin, `{"slug":"attribution","name":"Attribution"}`)
	otherProject := createTestProject(t, server, otherOwnerLogin, `{"slug":"other-attribution","name":"Other Attribution"}`)
	category := createTestCategory(t, server, ownerLogin, project.ID, `{"slug":"guides","name":"Guides"}`)
	primaryAuthor := createTestAuthor(t, server, ownerLogin, project.ID, `{"slug":"primary-author","displayName":"Primary Author","shortBio":"Primary bio"}`)
	coAuthor := createTestAuthor(t, server, ownerLogin, project.ID, `{"slug":"co-author","displayName":"Co Author"}`)
	editor := createTestAuthor(t, server, ownerLogin, project.ID, `{"slug":"editor-credit","displayName":"Editor Credit"}`)
	foreignAuthor := createTestAuthor(t, server, otherOwnerLogin, otherProject.ID, `{"slug":"foreign-author","displayName":"Foreign Author"}`)

	crossProjectRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles",
		`{"articleType":"guide","title":"Foreign Byline","slug":"foreign-byline","primaryCategoryId":"`+category.ID+`","html":"<p>Foreign.</p>","contributors":[{"authorId":"`+foreignAuthor.ID+`","role":"primary_author","position":0}]}`,
		ownerLogin,
	)
	crossProjectResponse := mustTest(t, server, crossProjectRequest)
	if crossProjectResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected a shared author to be assignable from another project, got %d: %s", crossProjectResponse.StatusCode, readBody(t, crossProjectResponse))
	}

	multiplePrimaryRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles",
		`{"articleType":"guide","title":"Multiple Primary","slug":"multiple-primary","primaryCategoryId":"`+category.ID+`","html":"<p>Invalid.</p>","contributors":[{"authorId":"`+primaryAuthor.ID+`","role":"primary_author","position":0},{"authorId":"`+coAuthor.ID+`","role":"primary_author","position":0}]}`,
		ownerLogin,
	)
	multiplePrimaryResponse := mustTest(t, server, multiplePrimaryRequest)
	if multiplePrimaryResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected multiple primary authors to fail with 400, got %d: %s", multiplePrimaryResponse.StatusCode, readBody(t, multiplePrimaryResponse))
	}

	article := createTestArticle(
		t,
		server,
		ownerLogin,
		project.ID,
		`{"articleType":"guide","title":"Attributed Guide","slug":"attributed-guide","primaryCategoryId":"`+category.ID+`","html":"<p>Attributed.</p>","contributors":[{"authorId":"`+editor.ID+`","role":"editor","position":0},{"authorId":"`+coAuthor.ID+`","role":"co_author","position":0},{"authorId":"`+primaryAuthor.ID+`","role":"primary_author","position":0}]}`,
	)
	firstRevisionID := article.LatestRevision.ID
	firstDetail := getTestRevisionDetail(t, db, project.ID, article.ID, firstRevisionID)
	firstAuthors := decodeAuthorSnapshot(t, firstDetail.AuthorSnapshot)
	firstContributors := decodeContributorSnapshot(t, firstDetail.ContributorSnapshot)
	if len(firstAuthors) != 2 || firstAuthors[0].ID != primaryAuthor.ID || firstAuthors[1].ID != coAuthor.ID {
		t.Fatalf("expected primary author followed by ordered co-authors, got %#v", firstAuthors)
	}
	if len(firstContributors) != 1 || firstContributors[0].Author.ID != editor.ID ||
		firstContributors[0].Role != "editor" || firstContributors[0].Position != 0 {
		t.Fatalf("expected an ordered editor credit, got %#v", firstContributors)
	}
	var contributorRows int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM revision_contributors
		WHERE project_id = ? AND revision_id = ?
	`, project.ID, firstRevisionID).Scan(&contributorRows); err != nil {
		t.Fatal(err)
	}
	if contributorRows != 3 {
		t.Fatalf("expected three revision contributor records, got %d", contributorRows)
	}

	renameRequest := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+project.ID+"/authors/"+primaryAuthor.ID,
		`{"displayName":"Renamed Author","slug":"renamed-author"}`,
		ownerLogin,
	)
	renameResponse := mustTest(t, server, renameRequest)
	if renameResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected author rename 200, got %d: %s", renameResponse.StatusCode, readBody(t, renameResponse))
	}
	firstDetailAfterRename := getTestRevisionDetail(t, db, project.ID, article.ID, firstRevisionID)
	if got := decodeAuthorSnapshot(t, firstDetailAfterRename.AuthorSnapshot)[0].DisplayName; got != "Primary Author" {
		t.Fatalf("expected historical revision byline to remain immutable, got %q", got)
	}

	inheritedRevision := createTestRevision(
		t,
		server,
		ownerLogin,
		project.ID,
		article.ID,
		`{"title":"Attributed Guide","html":"<p>Attributed.</p>"}`,
	)
	inheritedDetail := getTestRevisionDetail(t, db, project.ID, article.ID, inheritedRevision.ID)
	inheritedAuthors := decodeAuthorSnapshot(t, inheritedDetail.AuthorSnapshot)
	if len(inheritedAuthors) != 2 || inheritedAuthors[0].DisplayName != "Primary Author" {
		t.Fatalf("expected omitted contributor input to inherit the immutable byline, got %#v", inheritedAuthors)
	}
	var inheritedRows int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM revision_contributors
		WHERE project_id = ? AND revision_id = ?
	`, project.ID, inheritedRevision.ID).Scan(&inheritedRows); err != nil {
		t.Fatal(err)
	}
	if inheritedRows != 3 {
		t.Fatalf("expected inherited contributor records, got %d", inheritedRows)
	}

	refreshedRevision := createTestRevision(
		t,
		server,
		ownerLogin,
		project.ID,
		article.ID,
		`{"title":"Attributed Guide","html":"<p>Attributed.</p>","contributors":[{"authorId":"`+primaryAuthor.ID+`","role":"primary_author","position":0}]}`,
	)
	refreshedDetail := getTestRevisionDetail(t, db, project.ID, article.ID, refreshedRevision.ID)
	refreshedAuthors := decodeAuthorSnapshot(t, refreshedDetail.AuthorSnapshot)
	if len(refreshedAuthors) != 1 || refreshedAuthors[0].DisplayName != "Renamed Author" {
		t.Fatalf("expected explicit reassignment to snapshot the current public profile, got %#v", refreshedAuthors)
	}
	if refreshedRevision.ContentHash == inheritedRevision.ContentHash {
		t.Fatal("expected attribution changes to alter the approval-bound content hash")
	}

	publishTestArticle(t, server, ownerLogin, project.ID, article.ID, "attributed-guide")
	publishedRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/attributed-guide", nil)
	publishedRequest.Header.Set("X-Dev-Project-ID", project.ID)
	publishedResponse := mustTest(t, server, publishedRequest)
	if publishedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected attributed published article 200, got %d: %s", publishedResponse.StatusCode, readBody(t, publishedResponse))
	}
	var published Envelope[store.PublishedPost]
	decodeJSONResponse(t, publishedResponse, &published)
	if len(published.Data.Authors) != 1 || published.Data.Authors[0].DisplayName != "Renamed Author" || len(published.Data.Contributors) != 0 {
		t.Fatalf("expected the latest saved attribution in public JSON, got authors=%#v contributors=%#v", published.Data.Authors, published.Data.Contributors)
	}

	filteredRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts?author=renamed-author", nil)
	filteredRequest.Header.Set("X-Dev-Project-ID", project.ID)
	filteredResponse := mustTest(t, server, filteredRequest)
	if filteredResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected author-filtered list 200, got %d: %s", filteredResponse.StatusCode, readBody(t, filteredResponse))
	}
	var filtered ListEnvelope[store.PublishedPost]
	decodeJSONResponse(t, filteredResponse, &filtered)
	if len(filtered.Data) != 1 || filtered.Data[0].ID != article.ID {
		t.Fatalf("expected author-filtered published article, got %#v", filtered.Data)
	}
}

func TestPublicationRequiresAccountablePrimaryAuthor(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "owner correct horse password")
	project := createTestProject(t, server, ownerLogin, `{"slug":"publish-byline","name":"Publish Byline"}`)
	category := createTestCategory(t, server, ownerLogin, project.ID, `{"slug":"guides","name":"Guides"}`)
	primaryAuthor := createTestAuthor(t, server, ownerLogin, project.ID, `{"slug":"accountable-author","displayName":"Accountable Author"}`)

	createRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles",
		`{"articleType":"guide","title":"Legacy Byline","slug":"legacy-byline","primaryCategoryId":"`+category.ID+`","html":"<p>Missing attribution.</p>","contributors":[]}`,
		ownerLogin,
	)
	createResponse := mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected unattributed draft fixture 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var article Envelope[store.AdminArticle]
	decodeJSONResponse(t, createResponse, &article)
	firstRevisionID := article.Data.LatestRevision.ID

	publishRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.Data.ID+"/publish",
		"",
		ownerLogin,
	)
	publishResponse := mustTest(t, server, publishRequest)
	if publishResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected missing primary author to block publication with 409, got %d: %s", publishResponse.StatusCode, readBody(t, publishResponse))
	}
	if body := readBody(t, publishResponse); !strings.Contains(body, "accountable primary author") {
		t.Fatalf("expected primary-author publication error, got %s", body)
	}

	saveRequest := newMemberMutationRequest(
		http.MethodPut,
		"/api/v1/projects/"+project.ID+"/articles/"+article.Data.ID,
		`{"title":"Attribution restored","bodyDocument":{"type":"doc","content":[]},"html":"<p>Attribution restored.</p>","contributors":[{"authorId":"`+primaryAuthor.ID+`","role":"primary_author","position":0}]}`,
		ownerLogin,
	)
	saveResponse := mustTest(t, server, saveRequest)
	if saveResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected attributed save 200, got %d: %s", saveResponse.StatusCode, readBody(t, saveResponse))
	}
	var saved Envelope[store.AdminArticle]
	decodeJSONResponse(t, saveResponse, &saved)

	publishLatestRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.Data.ID+"/publish",
		`{"revisionId":"`+firstRevisionID+`"}`,
		ownerLogin,
	)
	publishLatestResponse := mustTest(t, server, publishLatestRequest)
	if publishLatestResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected attributed latest save to publish, got %d: %s", publishLatestResponse.StatusCode, readBody(t, publishLatestResponse))
	}
	var published Envelope[store.AdminArticle]
	decodeJSONResponse(t, publishLatestResponse, &published)
	if published.Data.LatestRevision == nil || published.Data.LatestRevision.ID != saved.Data.LatestRevision.ID || published.Data.Title != "Attribution restored" {
		t.Fatalf("expected publication to ignore obsolete revision selection and use the latest save, got %#v", published.Data)
	}
}

func TestSourcesClaimsAndDirectPublication(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "owner correct password")
	ownerBLogin := seedAndLogin(t, server, db, "owner-b@example.test", "owner b correct password")
	writerLogin := seedAndLogin(t, server, db, "writer@example.test", "writer correct password")
	project := createTestProject(t, server, ownerLogin, `{"slug":"trust","name":"Trust Project"}`)
	projectB := createTestProject(t, server, ownerBLogin, `{"slug":"trust-b","name":"Trust Project B"}`)
	category := createTestCategory(t, server, ownerLogin, project.ID, `{"slug":"guides","name":"Guides"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, writerLogin.userID); err != nil {
		t.Fatal(err)
	}

	sourceRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/sources",
		`{"title":"Primary benchmark","url":"https://example.test/benchmark","sourceType":"report","isPrimary":true}`,
		ownerLogin,
	)
	sourceResponse := mustTest(t, server, sourceRequest)
	if sourceResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected source create 201, got %d: %s", sourceResponse.StatusCode, readBody(t, sourceResponse))
	}
	var sourcePayload Envelope[store.Source]
	decodeJSONResponse(t, sourceResponse, &sourcePayload)
	source := sourcePayload.Data
	if source.ID == "" || !source.IsPrimary || source.SourceType != "report" {
		t.Fatalf("unexpected source payload %#v", source)
	}

	updateSourceRequest := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+project.ID+"/sources/"+source.ID,
		`{"title":"Primary benchmark update","notes":"Reviewed for Q3."}`,
		ownerLogin,
	)
	updateSourceResponse := mustTest(t, server, updateSourceRequest)
	if updateSourceResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected source update 200, got %d: %s", updateSourceResponse.StatusCode, readBody(t, updateSourceResponse))
	}
	var updatedSourcePayload Envelope[store.Source]
	decodeJSONResponse(t, updateSourceResponse, &updatedSourcePayload)
	if updatedSourcePayload.Data.Title != "Primary benchmark update" || updatedSourcePayload.Data.Notes != "Reviewed for Q3." {
		t.Fatalf("unexpected updated source %#v", updatedSourcePayload.Data)
	}

	listSourceRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/sources", nil)
	addCookies(listSourceRequest, ownerLogin.cookies)
	listSourceResponse := mustTest(t, server, listSourceRequest)
	if listSourceResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected source list 200, got %d: %s", listSourceResponse.StatusCode, readBody(t, listSourceResponse))
	}
	var listedSources ListEnvelope[store.Source]
	decodeJSONResponse(t, listSourceResponse, &listedSources)
	foundSource := false
	for _, listed := range listedSources.Data {
		if listed.ID == source.ID && listed.Title == "Primary benchmark update" {
			foundSource = true
		}
	}
	if !foundSource {
		t.Fatalf("expected updated source in list, got %#v", listedSources.Data)
	}

	projectBSourceRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectB.ID+"/sources",
		`{"title":"Project B source","url":"https://example.test/project-b","sourceType":"web"}`,
		ownerBLogin,
	)
	projectBSourceResponse := mustTest(t, server, projectBSourceRequest)
	if projectBSourceResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected project B source create 201, got %d: %s", projectBSourceResponse.StatusCode, readBody(t, projectBSourceResponse))
	}
	var projectBSourcePayload Envelope[store.Source]
	decodeJSONResponse(t, projectBSourceResponse, &projectBSourcePayload)

	article := createTestArticle(
		t,
		server,
		ownerLogin,
		project.ID,
		`{"articleType":"guide","title":"Claimed Guide","slug":"claimed-guide","primaryCategoryId":"`+category.ID+`","html":"<p>Benchmarked result.</p>"}`,
	)
	revisionID := article.LatestRevision.ID

	if _, err := server.store.CreateRevisionClaim(context.Background(), ownerLogin.userID, project.ID, revisionID, store.ClaimInput{
		ClaimText: "Cross-project source should fail.", Importance: "material", SourceIDs: []string{projectBSourcePayload.Data.ID},
	}); !errors.Is(err, store.ErrValidation) {
		t.Fatalf("expected cross-project source claim to fail validation, got %v", err)
	}

	claim, err := server.store.CreateRevisionClaim(context.Background(), ownerLogin.userID, project.ID, revisionID, store.ClaimInput{
		ClaimText: "The benchmark improved conversion by 12%.", Importance: "material", SourceIDs: []string{source.ID},
	})
	if err != nil {
		t.Fatalf("expected internal claim fixture: %v", err)
	}
	if claim.VerificationState != "unverified" || len(claim.SourceIDs) != 1 || claim.SourceIDs[0] != source.ID {
		t.Fatalf("unexpected claim payload %#v", claim)
	}

	listedClaims, err := server.store.ListRevisionClaims(context.Background(), ownerLogin.userID, project.ID, revisionID)
	if err != nil || len(listedClaims) != 1 || listedClaims[0].ID != claim.ID {
		t.Fatalf("expected created internal claim, got claims=%#v err=%v", listedClaims, err)
	}

	if _, err := server.store.VerifyClaim(context.Background(), writerLogin.userID, project.ID, claim.ID, store.ClaimVerificationInput{
		VerificationState: "supported",
	}); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("expected writer claim verification denial, got %v", err)
	}

	verified, err := server.store.VerifyClaim(context.Background(), ownerLogin.userID, project.ID, claim.ID, store.ClaimVerificationInput{
		VerificationState: "supported",
	})
	if err != nil || verified.VerificationState != "supported" || verified.VerifiedBy != ownerLogin.userID {
		t.Fatalf("unexpected verified claim %#v err=%v", verified, err)
	}

	published := publishTestArticle(t, server, ownerLogin, project.ID, article.ID, "claimed-guide")
	if published.PublicationState != "published" {
		t.Fatalf("expected direct publication to finalize the revision, got %#v", published)
	}
	detail := getTestRevisionDetail(t, db, project.ID, article.ID, revisionID)
	if !jsonContainsID(detail.SourceSnapshot, source.ID) {
		t.Fatalf("expected source snapshot to include %q, got %#v", source.ID, detail.SourceSnapshot)
	}
	if !jsonContainsID(detail.ClaimSnapshot, claim.ID) {
		t.Fatalf("expected claim snapshot to include %q, got %#v", claim.ID, detail.ClaimSnapshot)
	}

	if _, err := server.store.CreateRevisionClaim(context.Background(), ownerLogin.userID, project.ID, revisionID, store.ClaimInput{
		ClaimText: "Late change", Importance: "normal",
	}); !errors.Is(err, store.ErrInvalidWorkflow) {
		t.Fatalf("expected finalized revision claim mutation to fail, got %v", err)
	}

	var claimAuditCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND target_id = ?
		  AND action IN ('claim.create', 'claim.verify')
	`, project.ID, claim.ID).Scan(&claimAuditCount); err != nil {
		t.Fatal(err)
	}
	if claimAuditCount != 2 {
		t.Fatalf("expected claim create and verify audit events, got %d", claimAuditCount)
	}
}

func TestEditorCanPublishOwnDraftWithoutApproval(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "publish-owner@example.test", "owner correct horse password")
	editorLogin := seedAndLogin(t, server, db, "publishing-editor@example.test", "editor correct horse password")
	project := createTestProject(
		t,
		server,
		ownerLogin,
		`{"slug":"direct-editor-publish","name":"Direct Editor Publish","primaryDomain":"example.test"}`,
	)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'editor', 'active', CURRENT_TIMESTAMP)
	`, project.ID, editorLogin.userID); err != nil {
		t.Fatal(err)
	}
	category := createTestCategory(t, server, editorLogin, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(
		t,
		server,
		editorLogin,
		project.ID,
		`{"articleType":"guide","title":"Editor Draft","slug":"editor-draft","primaryCategoryId":"`+category.ID+`","html":"<p>Publish directly.</p>"}`,
	)

	saveRequest := newMemberMutationRequest(
		http.MethodPut,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID,
		`{"title":"Saved Directly","excerpt":"Latest saved copy","bodyDocument":{"type":"doc","content":[]},"html":"<p>Saved once, then published.</p>"}`,
		editorLogin,
	)
	saveResponse := mustTest(t, server, saveRequest)
	if saveResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected simple article save 200, got %d: %s", saveResponse.StatusCode, readBody(t, saveResponse))
	}
	var saved Envelope[store.AdminArticle]
	decodeJSONResponse(t, saveResponse, &saved)
	if saved.Data.LatestRevision == nil || saved.Data.LatestRevision.ID == article.LatestRevision.ID {
		t.Fatalf("expected PUT to store a new internal revision, got %#v", saved.Data.LatestRevision)
	}
	if saved.Data.Title != "Saved Directly" || saved.Data.Excerpt != "Latest saved copy" || saved.Data.HTML != "<p>Saved once, then published.</p>" {
		t.Fatalf("expected editable fields in save response, got %#v", saved.Data)
	}

	if _, err := server.store.CreateRevisionClaim(context.Background(), editorLogin.userID, project.ID, saved.Data.LatestRevision.ID, store.ClaimInput{
		ClaimText: "This unverified claim is advisory.", Importance: "material",
	}); err != nil {
		t.Fatalf("expected internal material claim fixture: %v", err)
	}

	publishRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/publish",
		"",
		editorLogin,
	)
	publishResponse := mustTest(t, server, publishRequest)
	if publishResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected empty-body direct publish 200, got %d: %s", publishResponse.StatusCode, readBody(t, publishResponse))
	}
	var publishedPayload Envelope[store.AdminArticle]
	decodeJSONResponse(t, publishResponse, &publishedPayload)
	published := publishedPayload.Data
	if published.PublicationState != "published" {
		t.Fatalf("expected direct publish to publish the draft, got publication=%q", published.PublicationState)
	}
	if published.Title != "Saved Directly" || published.Slug != "editor-draft" {
		t.Fatalf("expected empty publish input to use the latest save and current slug, got title=%q slug=%q", published.Title, published.Slug)
	}
	var decisions int
	if err := db.QueryRow(`
		SELECT COUNT(1) FROM approval_decisions WHERE project_id = ? AND revision_id = ?
	`, project.ID, saved.Data.LatestRevision.ID).Scan(&decisions); err != nil {
		t.Fatal(err)
	}
	if decisions != 0 {
		t.Fatalf("expected direct publication not to create an approval decision, got %d", decisions)
	}
	var finalizeEvents int
	if err := db.QueryRow(`
		SELECT COUNT(1) FROM audit_events
		WHERE project_id = ? AND action = 'revision.finalize_for_publication' AND target_id = ?
	`, project.ID, saved.Data.LatestRevision.ID).Scan(&finalizeEvents); err != nil {
		t.Fatal(err)
	}
	if finalizeEvents != 1 {
		t.Fatalf("expected one direct-publication finalization audit event, got %d", finalizeEvents)
	}
}

func TestDisclosuresAndCorrectionsReachPublishedJSON(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "owner correct password")
	writerLogin := seedAndLogin(t, server, db, "writer@example.test", "writer correct password")
	project := createTestProject(t, server, ownerLogin, `{"slug":"public-trust","name":"Public Trust"}`)
	category := createTestCategory(t, server, ownerLogin, project.ID, `{"slug":"updates","name":"Updates"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, writerLogin.userID); err != nil {
		t.Fatal(err)
	}

	article := createTestArticle(
		t,
		server,
		ownerLogin,
		project.ID,
		`{"articleType":"standard","title":"Trust Update","slug":"trust-update","primaryCategoryId":"`+category.ID+`","html":"<p>Visible update.</p>"}`,
	)
	revisionID := article.LatestRevision.ID
	publishTestArticle(t, server, ownerLogin, project.ID, article.ID, "trust-update")

	writerDisclosure := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/disclosures",
		`{"disclosureType":"affiliate","publicText":"Writer should not append public disclosure."}`,
		writerLogin,
	)
	writerDisclosureResponse := mustTest(t, server, writerDisclosure)
	if writerDisclosureResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer disclosure creation denial, got %d: %s", writerDisclosureResponse.StatusCode, readBody(t, writerDisclosureResponse))
	}

	disclosureRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/disclosures",
		`{"revisionId":"`+revisionID+`","disclosureType":"affiliate","publicText":"This article includes affiliate links."}`,
		ownerLogin,
	)
	disclosureResponse := mustTest(t, server, disclosureRequest)
	if disclosureResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected disclosure create 201, got %d: %s", disclosureResponse.StatusCode, readBody(t, disclosureResponse))
	}
	var disclosurePayload Envelope[store.Disclosure]
	decodeJSONResponse(t, disclosureResponse, &disclosurePayload)
	if disclosurePayload.Data.RevisionID != revisionID {
		t.Fatalf("expected revision-bound disclosure, got %#v", disclosurePayload.Data)
	}

	listDisclosuresRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/disclosures", nil)
	addCookies(listDisclosuresRequest, ownerLogin.cookies)
	listDisclosuresResponse := mustTest(t, server, listDisclosuresRequest)
	if listDisclosuresResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected disclosure list 200, got %d: %s", listDisclosuresResponse.StatusCode, readBody(t, listDisclosuresResponse))
	}
	var listedDisclosures ListEnvelope[store.Disclosure]
	decodeJSONResponse(t, listDisclosuresResponse, &listedDisclosures)
	if len(listedDisclosures.Data) != 1 || listedDisclosures.Data[0].ID != disclosurePayload.Data.ID {
		t.Fatalf("expected created disclosure in list, got %#v", listedDisclosures.Data)
	}

	correctionRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/corrections",
		`{"affectedRevisionId":"`+revisionID+`","publicNote":"Corrected an outdated benchmark date."}`,
		ownerLogin,
	)
	correctionResponse := mustTest(t, server, correctionRequest)
	if correctionResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected correction create 201, got %d: %s", correctionResponse.StatusCode, readBody(t, correctionResponse))
	}
	var correctionPayload Envelope[store.CorrectionNotice]
	decodeJSONResponse(t, correctionResponse, &correctionPayload)
	if correctionPayload.Data.AffectedRevisionID != revisionID {
		t.Fatalf("expected revision-bound correction, got %#v", correctionPayload.Data)
	}

	listCorrectionsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/corrections", nil)
	addCookies(listCorrectionsRequest, ownerLogin.cookies)
	listCorrectionsResponse := mustTest(t, server, listCorrectionsRequest)
	if listCorrectionsResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected correction list 200, got %d: %s", listCorrectionsResponse.StatusCode, readBody(t, listCorrectionsResponse))
	}
	var listedCorrections ListEnvelope[store.CorrectionNotice]
	decodeJSONResponse(t, listCorrectionsResponse, &listedCorrections)
	if len(listedCorrections.Data) != 1 || listedCorrections.Data[0].ID != correctionPayload.Data.ID {
		t.Fatalf("expected created correction in list, got %#v", listedCorrections.Data)
	}

	publishedRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/trust-update", nil)
	publishedRequest.Header.Set("X-Dev-Project-ID", project.ID)
	publishedResponse := mustTest(t, server, publishedRequest)
	if publishedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected published article 200, got %d: %s", publishedResponse.StatusCode, readBody(t, publishedResponse))
	}
	var published Envelope[store.PublishedPost]
	decodeJSONResponse(t, publishedResponse, &published)
	if !jsonContainsID(published.Data.Disclosures, disclosurePayload.Data.ID) {
		t.Fatalf("expected published JSON to include disclosure %q, got %#v", disclosurePayload.Data.ID, published.Data.Disclosures)
	}
	if !jsonContainsID(published.Data.Corrections, correctionPayload.Data.ID) {
		t.Fatalf("expected published JSON to include correction %q, got %#v", correctionPayload.Data.ID, published.Data.Corrections)
	}

	changesRequest := httptest.NewRequest(http.MethodGet, "/content/v1/changes", nil)
	changesRequest.Header.Set("X-Dev-Project-ID", project.ID)
	changesResponse := mustTest(t, server, changesRequest)
	if changesResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected changes list 200, got %d: %s", changesResponse.StatusCode, readBody(t, changesResponse))
	}
	var changes ListEnvelope[store.ChangeRecord]
	decodeJSONResponse(t, changesResponse, &changes)
	contentUpdates := 0
	for _, change := range changes.Data {
		if change.AggregateID == article.ID && change.Type == "content.updated" {
			contentUpdates++
		}
	}
	if contentUpdates < 2 {
		t.Fatalf("expected disclosure and correction content.updated events, got %#v", changes.Data)
	}

	var contentGeneration int64
	if err := db.QueryRow(`SELECT content_generation FROM projects WHERE id = ?`, project.ID).Scan(&contentGeneration); err != nil {
		t.Fatal(err)
	}
	if contentGeneration < 5 {
		t.Fatalf("expected public trust writes to advance content generation, got %d", contentGeneration)
	}
}

func TestAuthorManagementAuthorizationAndSharedProjectDirectory(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerALogin := seedAndLogin(t, server, db, "owner-a@example.test", "owner a correct password")
	ownerBLogin := seedAndLogin(t, server, db, "owner-b@example.test", "owner b correct password")
	editorLogin := seedAndLogin(t, server, db, "editor@example.test", "editor correct password")
	writerLogin := seedAndLogin(t, server, db, "writer@example.test", "writer correct password")
	projectA := createTestProject(t, server, ownerALogin, `{"slug":"authors-a","name":"Authors A"}`)
	projectB := createTestProject(t, server, ownerBLogin, `{"slug":"authors-b","name":"Authors B"}`)
	if _, err := db.Exec(`
		INSERT INTO assets(
		  id, project_id, object_key, filename, mime_type, byte_size, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "asset-project-b", projectB.ID, "authors/project-b.jpg", "project-b.jpg", "image/jpeg", 1024, ownerBLogin.userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES
		  (?, ?, 'editor', 'active', CURRENT_TIMESTAMP),
		  (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, projectA.ID, editorLogin.userID, projectA.ID, writerLogin.userID); err != nil {
		t.Fatal(err)
	}
	authorB := createTestAuthor(t, server, ownerBLogin, projectB.ID, `{"displayName":"Project B Author"}`)

	writerList := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectA.ID+"/authors", nil)
	addCookies(writerList, writerLogin.cookies)
	writerListResponse := mustTest(t, server, writerList)
	if writerListResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected writer author list 200, got %d: %s", writerListResponse.StatusCode, readBody(t, writerListResponse))
	}

	writerCreate := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectA.ID+"/authors",
		`{"displayName":"Writer Managed"}`,
		writerLogin,
	)
	writerCreateResponse := mustTest(t, server, writerCreate)
	if writerCreateResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer author creation to fail with 403, got %d: %s", writerCreateResponse.StatusCode, readBody(t, writerCreateResponse))
	}

	editorCreate := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectA.ID+"/authors",
		`{"displayName":"Editor Managed","loginUserId":"`+ownerALogin.userID+`"}`,
		editorLogin,
	)
	editorCreateResponse := mustTest(t, server, editorCreate)
	if editorCreateResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected editor login-linked author creation to fail with 403, got %d: %s", editorCreateResponse.StatusCode, readBody(t, editorCreateResponse))
	}

	crossProjectLogin := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectA.ID+"/authors",
		`{"displayName":"Wrong Login","loginUserId":"`+ownerBLogin.userID+`"}`,
		ownerALogin,
	)
	crossProjectLoginResponse := mustTest(t, server, crossProjectLogin)
	if crossProjectLoginResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected a shared author to link to a member from another project, got %d: %s", crossProjectLoginResponse.StatusCode, readBody(t, crossProjectLoginResponse))
	}

	linkedAuthor := createTestAuthor(t, server, ownerALogin, projectA.ID, `{"displayName":"Linked Author","loginUserId":"`+ownerALogin.userID+`"}`)
	editorGetLinked := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectA.ID+"/authors/"+linkedAuthor.ID, nil)
	addCookies(editorGetLinked, editorLogin.cookies)
	editorGetLinkedResponse := mustTest(t, server, editorGetLinked)
	if editorGetLinkedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected editor author detail 200, got %d: %s", editorGetLinkedResponse.StatusCode, readBody(t, editorGetLinkedResponse))
	}
	var editorLinkedDetail Envelope[store.Author]
	decodeJSONResponse(t, editorGetLinkedResponse, &editorLinkedDetail)
	if editorLinkedDetail.Data.LoginUserID != "" || editorLinkedDetail.Data.LoginEmail != "" {
		t.Fatalf("expected editor author detail to hide login metadata, got %#v", editorLinkedDetail.Data)
	}

	editorPatchLinked := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+projectA.ID+"/authors/"+linkedAuthor.ID,
		`{"displayName":"Editor Updated Linked Author"}`,
		editorLogin,
	)
	editorPatchLinkedResponse := mustTest(t, server, editorPatchLinked)
	if editorPatchLinkedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected editor author update without login metadata 200, got %d: %s", editorPatchLinkedResponse.StatusCode, readBody(t, editorPatchLinkedResponse))
	}
	var editorPatchedLinked Envelope[store.Author]
	decodeJSONResponse(t, editorPatchLinkedResponse, &editorPatchedLinked)
	if editorPatchedLinked.Data.LoginUserID != "" || editorPatchedLinked.Data.LoginEmail != "" {
		t.Fatalf("expected editor author update response to hide login metadata, got %#v", editorPatchedLinked.Data)
	}
	ownerGetLinked := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectA.ID+"/authors/"+linkedAuthor.ID, nil)
	addCookies(ownerGetLinked, ownerALogin.cookies)
	ownerGetLinkedResponse := mustTest(t, server, ownerGetLinked)
	if ownerGetLinkedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected owner author detail 200, got %d: %s", ownerGetLinkedResponse.StatusCode, readBody(t, ownerGetLinkedResponse))
	}
	var ownerLinkedDetail Envelope[store.Author]
	decodeJSONResponse(t, ownerGetLinkedResponse, &ownerLinkedDetail)
	if ownerLinkedDetail.Data.LoginUserID != ownerALogin.userID || ownerLinkedDetail.Data.DisplayName != "Editor Updated Linked Author" {
		t.Fatalf("expected editor update to preserve hidden login metadata, got %#v", ownerLinkedDetail.Data)
	}

	editorClearLogin := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+projectA.ID+"/authors/"+linkedAuthor.ID,
		`{"loginUserId":""}`,
		editorLogin,
	)
	editorClearLoginResponse := mustTest(t, server, editorClearLogin)
	if editorClearLoginResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected editor login unlink to fail with 403, got %d: %s", editorClearLoginResponse.StatusCode, readBody(t, editorClearLoginResponse))
	}

	crossProjectPhoto := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectA.ID+"/authors",
		`{"displayName":"Wrong Photo","photoAssetId":"asset-project-b"}`,
		ownerALogin,
	)
	crossProjectPhotoResponse := mustTest(t, server, crossProjectPhoto)
	if crossProjectPhotoResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected a shared author to use a photo asset from another project, got %d: %s", crossProjectPhotoResponse.StatusCode, readBody(t, crossProjectPhotoResponse))
	}

	crossProjectPatch := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+projectB.ID+"/authors/"+authorB.ID,
		`{"displayName":"Leaked"}`,
		ownerALogin,
	)
	crossProjectPatchResponse := mustTest(t, server, crossProjectPatch)
	if crossProjectPatchResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected a shared author to be editable from another project, got %d: %s", crossProjectPatchResponse.StatusCode, readBody(t, crossProjectPatchResponse))
	}
}

func TestProjectAPIKeyMutationsRequireRecentReauthentication(t *testing.T) {
	server, db := newAdminTestServer(t)
	password := "correct horse battery staple"
	login := seedAndLogin(t, server, db, "owner@example.test", password)
	project := createTestProject(t, server, login, `{"slug":"reauth","name":"Reauthentication"}`)
	existing := createTestAPIKey(t, server, login, project.ID, `{"environment":"production","name":"existing"}`)

	if _, err := db.Exec(`
		UPDATE sessions
		SET reauthenticated_at = datetime(CURRENT_TIMESTAMP, '-10 minutes')
		WHERE user_id = ?
	`, login.userID); err != nil {
		t.Fatal(err)
	}

	createRequest := newAPIKeyMutationRequest(
		t,
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/api-keys",
		`{"environment":"production","name":"requires reauthentication"}`,
		login,
	)
	createResponse := mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected stale reauthentication to fail with 403, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	for _, path := range []string{
		"/api/v1/projects/" + project.ID + "/api-keys/" + existing.Data.Key.ID + "/rotate",
		"/api/v1/projects/" + project.ID + "/api-keys/" + existing.Data.Key.ID + "/revoke",
	} {
		request := newAPIKeyMutationRequest(t, http.MethodPost, path, `{}`, login)
		response := mustTest(t, server, request)
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("expected stale reauthentication for %s to fail with 403, got %d: %s", path, response.StatusCode, readBody(t, response))
		}
	}
	assertContentCategoriesStatus(t, server, existing.Data.Secret, http.StatusOK)

	badReauth := newAPIKeyMutationRequest(t, http.MethodPost, "/api/v1/auth/reauthenticate", `{"password":"incorrect"}`, login)
	badReauthResponse := mustTest(t, server, badReauth)
	if badReauthResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected incorrect password to fail with 401, got %d", badReauthResponse.StatusCode)
	}

	reauth := newAPIKeyMutationRequest(t, http.MethodPost, "/api/v1/auth/reauthenticate", `{"password":"`+password+`"}`, login)
	reauthResponse := mustTest(t, server, reauth)
	if reauthResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected reauthentication 200, got %d: %s", reauthResponse.StatusCode, readBody(t, reauthResponse))
	}

	createRequest = newAPIKeyMutationRequest(
		t,
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/api-keys",
		`{"environment":"production","name":"reauthenticated"}`,
		login,
	)
	createResponse = mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected reauthenticated key creation 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
}

func TestExpiredProjectAPIKeyCannotBeCreatedOrRotated(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"expired-key","name":"Expired Key"}`)

	longName := newAPIKeyMutationRequest(
		t,
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/api-keys",
		`{"environment":"production","name":"`+strings.Repeat("k", 101)+`"}`,
		login,
	)
	longNameResponse := mustTest(t, server, longName)
	if longNameResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected an overlong key name to fail with 400, got %d: %s", longNameResponse.StatusCode, readBody(t, longNameResponse))
	}

	expiredCreate := newAPIKeyMutationRequest(
		t,
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/api-keys",
		`{"environment":"production","name":"already expired","expiresAt":"2000-01-01T00:00:00Z"}`,
		login,
	)
	expiredCreateResponse := mustTest(t, server, expiredCreate)
	if expiredCreateResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected past expiry to fail with 400, got %d: %s", expiredCreateResponse.StatusCode, readBody(t, expiredCreateResponse))
	}

	key := createTestAPIKey(t, server, login, project.ID, `{"environment":"production","name":"expired"}`)
	if _, err := db.Exec(`
		UPDATE project_api_keys
		SET expires_at = datetime(CURRENT_TIMESTAMP, '-1 minute')
		WHERE id = ?
	`, key.Data.Key.ID); err != nil {
		t.Fatal(err)
	}

	rotate := newAPIKeyMutationRequest(
		t,
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/api-keys/"+key.Data.Key.ID+"/rotate",
		`{}`,
		login,
	)
	rotateResponse := mustTest(t, server, rotate)
	if rotateResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected expired rotation to fail with 409, got %d: %s", rotateResponse.StatusCode, readBody(t, rotateResponse))
	}
}

func TestScheduledPublishFlow(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")

	project := createTestProject(t, server, login, `{"slug":"schedule","name":"Schedule Project","primaryDomain":"example.test"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"launches","name":"Launches"}`)

	articleRequest := `{
		"articleType":"standard",
		"title":"Scheduled Post",
		"slug":"scheduled-post",
		"primaryCategoryId":"` + category.ID + `",
		"excerpt":"A scheduled excerpt",
		"html":"<p>Scheduled body</p>"
	}`
	article := createTestArticle(t, server, login, project.ID, articleRequest)

	scheduleDraft := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/schedule",
		strings.NewReader(`{"slug":"scheduled-post","scheduledForUtc":"`+time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)+`"}`),
	)
	scheduleDraft.Header.Set("Content-Type", "application/json")
	scheduleDraft.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(scheduleDraft, login.cookies)
	scheduleResponse := mustTest(t, server, scheduleDraft)
	if scheduleResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected an editor to schedule a draft without separate approval, got %d: %s", scheduleResponse.StatusCode, readBody(t, scheduleResponse))
	}
	var scheduledPayload Envelope[store.AdminArticle]
	decodeJSONResponse(t, scheduleResponse, &scheduledPayload)
	if scheduledPayload.Data.PublicationState != "scheduled" {
		t.Fatalf("expected scheduling to retain the scheduled state, got publication=%q", scheduledPayload.Data.PublicationState)
	}

	beforePublishRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/scheduled-post", nil)
	beforePublishRequest.Header.Set("X-Dev-Project-ID", project.ID)
	beforePublishResponse := mustTest(t, server, beforePublishRequest)
	if beforePublishResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected scheduled article to remain hidden before worker publish, got %d", beforePublishResponse.StatusCode)
	}
	latest := createTestRevision(t, server, login, project.ID, article.ID, `{
		"title":"Updated Scheduled Post",
		"html":"<p>The latest saved body is published.</p>"
	}`)
	var scheduledRevisionID string
	if err := db.QueryRow(`
		SELECT published_revision_id
		FROM project_publications
		WHERE project_id = ? AND content_id = ?
	`, project.ID, article.ID).Scan(&scheduledRevisionID); err != nil {
		t.Fatal(err)
	}
	if scheduledRevisionID != latest.ID {
		t.Fatalf("expected the schedule to follow the latest publisher save %q, got %q", latest.ID, scheduledRevisionID)
	}

	published, err := store.New(db).PublishDueSchedules(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	if published != 1 {
		t.Fatalf("expected worker to publish one scheduled article, got %d", published)
	}

	publishedRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/scheduled-post", nil)
	publishedRequest.Header.Set("X-Dev-Project-ID", project.ID)
	publishedResponse := mustTest(t, server, publishedRequest)
	if publishedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected published article to be readable, got %d: %s", publishedResponse.StatusCode, readBody(t, publishedResponse))
	}
	var payload Envelope[store.PublishedPost]
	decodeJSONResponse(t, publishedResponse, &payload)
	if payload.Data.Title != "Updated Scheduled Post" {
		t.Fatalf("expected published title, got %q", payload.Data.Title)
	}
	if payload.Data.SEO.CanonicalURL != "https://example.test/blog/scheduled-post" {
		t.Fatalf("unexpected canonical URL %q", payload.Data.SEO.CanonicalURL)
	}
}

func TestPublicationTransitionsKeepVersionsAndScheduleStateSafe(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "transition-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"publication-transitions","name":"Publication Transitions","primaryDomain":"example.test"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, login, project.ID, `{"title":"State Guide","slug":"state-guide","primaryCategoryId":"`+category.ID+`","html":"<p>State body</p>"}`)
	publishTestArticle(t, server, login, project.ID, article.ID, "state-guide")
	var initialVersion int64
	if err := db.QueryRow(`SELECT publication_version FROM project_publications WHERE project_id = ? AND content_id = ?`, project.ID, article.ID).Scan(&initialVersion); err != nil {
		t.Fatal(err)
	}

	schedulePublished := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/schedule",
		`{"scheduledForUtc":"`+time.Now().Add(time.Hour).UTC().Format(time.RFC3339)+`"}`,
		login,
	)
	schedulePublishedResponse := mustTest(t, server, schedulePublished)
	if schedulePublishedResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected scheduling a live article to fail with 409, got %d: %s", schedulePublishedResponse.StatusCode, readBody(t, schedulePublishedResponse))
	}

	unpublishPath := "/api/v1/projects/" + project.ID + "/articles/" + article.ID + "/unpublish"
	unpublish := newMemberMutationRequest(http.MethodPost, unpublishPath, "", login)
	unpublishResponse := mustTest(t, server, unpublish)
	if unpublishResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected unpublish 200, got %d: %s", unpublishResponse.StatusCode, readBody(t, unpublishResponse))
	}
	var state string
	var version int64
	if err := db.QueryRow(`SELECT publication_state, publication_version FROM project_publications WHERE project_id = ? AND content_id = ?`, project.ID, article.ID).Scan(&state, &version); err != nil {
		t.Fatal(err)
	}
	if state != "unpublished" || version != initialVersion+1 {
		t.Fatalf("expected persisted unpublish version %d, got state=%q version=%d", initialVersion+1, state, version)
	}
	var generationAfterUnpublish int64
	var eventsAfterUnpublish int
	if err := db.QueryRow(`SELECT content_generation FROM projects WHERE id = ?`, project.ID).Scan(&generationAfterUnpublish); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events WHERE project_id = ? AND aggregate_id = ?`, project.ID, article.ID).Scan(&eventsAfterUnpublish); err != nil {
		t.Fatal(err)
	}

	noOpResponse := mustTest(t, server, newMemberMutationRequest(http.MethodPost, unpublishPath, "", login))
	if noOpResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected repeated unpublish to be idempotent, got %d: %s", noOpResponse.StatusCode, readBody(t, noOpResponse))
	}
	var noOpGeneration int64
	var noOpEvents int
	if err := db.QueryRow(`SELECT content_generation FROM projects WHERE id = ?`, project.ID).Scan(&noOpGeneration); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events WHERE project_id = ? AND aggregate_id = ?`, project.ID, article.ID).Scan(&noOpEvents); err != nil {
		t.Fatal(err)
	}
	if noOpGeneration != generationAfterUnpublish || noOpEvents != eventsAfterUnpublish {
		t.Fatalf("repeated unpublish emitted public work: generation %d→%d events %d→%d", generationAfterUnpublish, noOpGeneration, eventsAfterUnpublish, noOpEvents)
	}

	publishTestArticle(t, server, login, project.ID, article.ID, "state-guide")
	if err := db.QueryRow(`SELECT publication_version FROM project_publications WHERE project_id = ? AND content_id = ?`, project.ID, article.ID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != initialVersion+2 {
		t.Fatalf("expected republish version %d, got %d", initialVersion+2, version)
	}
	if response := mustTest(t, server, newMemberMutationRequest(http.MethodPost, unpublishPath, "", login)); response.StatusCode != http.StatusOK {
		t.Fatalf("expected second live unpublish 200, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var generationBeforeScheduleCancel int64
	var eventsBeforeScheduleCancel int
	if err := db.QueryRow(`SELECT content_generation FROM projects WHERE id = ?`, project.ID).Scan(&generationBeforeScheduleCancel); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events WHERE project_id = ? AND aggregate_id = ?`, project.ID, article.ID).Scan(&eventsBeforeScheduleCancel); err != nil {
		t.Fatal(err)
	}
	scheduleAfterUnpublish := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/schedule",
		`{"scheduledForUtc":"`+time.Now().Add(time.Hour).UTC().Format(time.RFC3339)+`"}`,
		login,
	)
	if response := mustTest(t, server, scheduleAfterUnpublish); response.StatusCode != http.StatusOK {
		t.Fatalf("expected scheduling an unpublished article 200, got %d: %s", response.StatusCode, readBody(t, response))
	}
	if response := mustTest(t, server, newMemberMutationRequest(http.MethodPost, unpublishPath, "", login)); response.StatusCode != http.StatusOK {
		t.Fatalf("expected scheduled cancellation 200, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var generationAfterScheduleCancel int64
	var eventsAfterScheduleCancel int
	if err := db.QueryRow(`SELECT publication_state, publication_version FROM project_publications WHERE project_id = ? AND content_id = ?`, project.ID, article.ID).Scan(&state, &version); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT content_generation FROM projects WHERE id = ?`, project.ID).Scan(&generationAfterScheduleCancel); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events WHERE project_id = ? AND aggregate_id = ?`, project.ID, article.ID).Scan(&eventsAfterScheduleCancel); err != nil {
		t.Fatal(err)
	}
	if state != "unpublished" || version != initialVersion+3 || generationAfterScheduleCancel != generationBeforeScheduleCancel || eventsAfterScheduleCancel != eventsBeforeScheduleCancel {
		t.Fatalf("schedule cancellation changed public state unexpectedly: state=%q version=%d generation=%d→%d events=%d→%d", state, version, generationBeforeScheduleCancel, generationAfterScheduleCancel, eventsBeforeScheduleCancel, eventsAfterScheduleCancel)
	}
}

func TestWriterSaveCancelsScheduleInsteadOfPublishingWithoutPermission(t *testing.T) {
	server, db := newAdminTestServer(t)
	owner := seedAndLogin(t, server, db, "schedule-owner@example.test", "correct horse battery staple")
	writer := seedAndLogin(t, server, db, "schedule-writer@example.test", "writer correct horse password")
	project := createTestProject(t, server, owner, `{"slug":"writer-schedule","name":"Writer Schedule","primaryDomain":"example.test"}`)
	if _, err := db.Exec(`INSERT INTO project_memberships(project_id, user_id, role, status, joined_at) VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)`, project.ID, writer.userID); err != nil {
		t.Fatal(err)
	}
	category := createTestCategory(t, server, owner, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, owner, project.ID, `{"title":"Scheduled Owner Draft","slug":"writer-schedule","primaryCategoryId":"`+category.ID+`","html":"<p>Owner body</p>"}`)
	scheduleRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/schedule",
		`{"scheduledForUtc":"`+time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)+`"}`,
		owner,
	)
	if response := mustTest(t, server, scheduleRequest); response.StatusCode != http.StatusOK {
		t.Fatalf("expected owner schedule 200, got %d: %s", response.StatusCode, readBody(t, response))
	}

	writerSave := newMemberMutationRequest(
		http.MethodPut,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID,
		`{"title":"Writer Update","bodyDocument":{"type":"doc","content":[]},"html":"<p>Writer body</p>"}`,
		writer,
	)
	writerSaveResponse := mustTest(t, server, writerSave)
	if writerSaveResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected writer save 200, got %d: %s", writerSaveResponse.StatusCode, readBody(t, writerSaveResponse))
	}
	var state string
	if err := db.QueryRow(`SELECT publication_state FROM project_publications WHERE project_id = ? AND content_id = ?`, project.ID, article.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "unpublished" {
		t.Fatalf("expected writer save to cancel schedule, got %q", state)
	}
	published, err := store.New(db).PublishDueSchedules(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	if published != 0 {
		t.Fatalf("expected cancelled writer schedule not to publish, got %d", published)
	}
	publicRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/writer-schedule", nil)
	publicRequest.Header.Set("X-Dev-Project-ID", project.ID)
	if response := mustTest(t, server, publicRequest); response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected writer content to remain private, got %d: %s", response.StatusCode, readBody(t, response))
	}
}

func TestDeleteArticleArchivesAndRemovesPublishedContent(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")

	project := createTestProject(t, server, login, `{"slug":"article-delete","name":"Article Delete","primaryDomain":"example.test"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, login, project.ID, `{
		"articleType":"guide",
		"title":"Archived Guide",
		"slug":"archived-guide",
		"primaryCategoryId":"`+category.ID+`",
		"excerpt":"A guide to archive",
		"html":"<p>Archive me once the flow is complete.</p>"
	}`)
	publishTestArticle(t, server, login, project.ID, article.ID, "archived-guide")

	var generationBeforeArchive int64
	var versionBeforeArchive int64
	if err := db.QueryRow(`
		SELECT project.content_generation, publication.publication_version
		FROM projects project
		JOIN project_publications publication ON publication.project_id = project.id
		WHERE project.id = ? AND publication.content_id = ?
	`, project.ID, article.ID).Scan(&generationBeforeArchive, &versionBeforeArchive); err != nil {
		t.Fatal(err)
	}

	deleteRequest := newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID,
		``,
		login,
	)
	deleteResponse := mustTest(t, server, deleteRequest)
	if deleteResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("expected delete article 204, got %d: %s", deleteResponse.StatusCode, readBody(t, deleteResponse))
	}

	articleRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/articles/"+article.ID, nil)
	addCookies(articleRequest, login.cookies)
	articleResponse := mustTest(t, server, articleRequest)
	if articleResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected archived article detail to return 404, got %d: %s", articleResponse.StatusCode, readBody(t, articleResponse))
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/articles", nil)
	addCookies(listRequest, login.cookies)
	listResponse := mustTest(t, server, listRequest)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected article list 200 after archive, got %d: %s", listResponse.StatusCode, readBody(t, listResponse))
	}
	var listPayload ListEnvelope[store.AdminArticle]
	decodeJSONResponse(t, listResponse, &listPayload)
	for _, listed := range listPayload.Data {
		if listed.ID == article.ID {
			t.Fatalf("archived article %s should not appear in admin list", article.ID)
		}
	}

	publishedRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/archived-guide", nil)
	publishedRequest.Header.Set("X-Dev-Project-ID", project.ID)
	publishedResponse := mustTest(t, server, publishedRequest)
	if publishedResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected archived article to disappear from content API, got %d: %s", publishedResponse.StatusCode, readBody(t, publishedResponse))
	}

	var archivedAt string
	var retiredAt string
	var publicationState string
	var scheduledFor string
	var generationAfterArchive int64
	var versionAfterArchive int64
	if err := db.QueryRow(`
		SELECT COALESCE(item.archived_at, ''), publication.publication_state,
		       COALESCE(publication.scheduled_for_utc, ''), COALESCE(publication.retired_at, ''),
		       project.content_generation, publication.publication_version
		FROM content_items item
		JOIN projects project ON project.id = item.project_id
		JOIN project_publications publication
		  ON publication.project_id = item.project_id AND publication.content_id = item.id
		WHERE item.project_id = ? AND item.id = ?
	`, project.ID, article.ID).Scan(
		&archivedAt,
		&publicationState,
		&scheduledFor,
		&retiredAt,
		&generationAfterArchive,
		&versionAfterArchive,
	); err != nil {
		t.Fatal(err)
	}
	if archivedAt == "" {
		t.Fatal("expected content_items.archived_at to be set")
	}
	if publicationState != "archived" {
		t.Fatalf("expected publication state archived, got %q", publicationState)
	}
	if scheduledFor != "" {
		t.Fatalf("expected archived publication to clear schedule, got %q", scheduledFor)
	}
	if retiredAt == "" {
		t.Fatal("expected archived publication retired_at to be set")
	}
	if generationAfterArchive <= generationBeforeArchive {
		t.Fatalf("expected content generation to advance from %d, got %d", generationBeforeArchive, generationAfterArchive)
	}
	if versionAfterArchive <= versionBeforeArchive {
		t.Fatalf("expected publication version to advance from %d, got %d", versionBeforeArchive, versionAfterArchive)
	}

	var archivedEvents int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM outbox_events
		WHERE project_id = ?
		  AND aggregate_id = ?
		  AND event_type = 'content.archived'
	`, project.ID, article.ID).Scan(&archivedEvents); err != nil {
		t.Fatal(err)
	}
	if archivedEvents != 1 {
		t.Fatalf("expected one content.archived outbox event, got %d", archivedEvents)
	}

	var archiveAudits int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND target_id = ?
		  AND action = 'content.archive'
	`, project.ID, article.ID).Scan(&archiveAudits); err != nil {
		t.Fatal(err)
	}
	if archiveAudits != 1 {
		t.Fatalf("expected one content.archive audit event, got %d", archiveAudits)
	}
}

func TestArticleListFiltersAndArchivedArticleRestore(t *testing.T) {
	server, db := newAdminTestServer(t)
	owner := seedAndLogin(t, server, db, "article-filter-owner@example.test", "correct horse battery staple")
	writer := seedAndLogin(t, server, db, "article-filter-writer@example.test", "another correct horse battery staple")
	project := createTestProject(t, server, owner, `{"slug":"article-filter","name":"Article Filter","primaryDomain":"example.test"}`)
	category := createTestCategory(t, server, owner, project.ID, `{"slug":"guides","name":"Guides"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, writer.userID); err != nil {
		t.Fatal(err)
	}

	archived := createTestArticle(t, server, owner, project.ID, `{
		"articleType":"guide",
		"title":"Needle Archive Guide",
		"slug":"needle-archive-guide",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>Archived body.</p>"
	}`)
	draft := createTestArticle(t, server, owner, project.ID, `{
		"articleType":"tutorial",
		"title":"Visible Draft",
		"slug":"visible-draft",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>Draft body.</p>"
	}`)
	secondDraft := createTestArticle(t, server, owner, project.ID, `{
		"articleType":"tutorial",
		"title":"Another Visible Draft",
		"slug":"another-visible-draft",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>Another draft body.</p>"
	}`)
	publishTestArticle(t, server, owner, project.ID, archived.ID, archived.Slug)
	archiveResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodDelete,
		"/api/v1/projects/"+project.ID+"/articles/"+archived.ID,
		``,
		owner,
	))
	if archiveResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("expected archive 204, got %d: %s", archiveResponse.StatusCode, readBody(t, archiveResponse))
	}

	filteredRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles?q=needle&publicationState=archived&includeArchived=true",
		nil,
	)
	addCookies(filteredRequest, owner.cookies)
	filteredResponse := mustTest(t, server, filteredRequest)
	if filteredResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected filtered article list 200, got %d: %s", filteredResponse.StatusCode, readBody(t, filteredResponse))
	}
	var filtered ListEnvelope[store.AdminArticle]
	decodeJSONResponse(t, filteredResponse, &filtered)
	if len(filtered.Data) != 1 || filtered.Data[0].ID != archived.ID || filtered.Data[0].ArchivedAt == "" {
		t.Fatalf("expected only the archived matching article, got %#v", filtered.Data)
	}

	draftRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles?publicationState=unpublished&limit=1",
		nil,
	)
	addCookies(draftRequest, owner.cookies)
	draftResponse := mustTest(t, server, draftRequest)
	var drafts ListEnvelope[store.AdminArticle]
	decodeJSONResponse(t, draftResponse, &drafts)
	if len(drafts.Data) != 1 || drafts.Meta.NextCursor == "" {
		t.Fatalf("expected the first filtered page and a cursor, got %#v", drafts)
	}
	secondPageRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles?publicationState=unpublished&limit=1&cursor="+url.QueryEscape(drafts.Meta.NextCursor),
		nil,
	)
	addCookies(secondPageRequest, owner.cookies)
	secondPageResponse := mustTest(t, server, secondPageRequest)
	var secondPage ListEnvelope[store.AdminArticle]
	decodeJSONResponse(t, secondPageResponse, &secondPage)
	if len(secondPage.Data) != 1 || secondPage.Data[0].ID == drafts.Data[0].ID {
		t.Fatalf("expected a distinct second filtered page, got first=%#v second=%#v", drafts.Data, secondPage.Data)
	}
	seenDrafts := map[string]bool{drafts.Data[0].ID: true, secondPage.Data[0].ID: true}
	if !seenDrafts[draft.ID] || !seenDrafts[secondDraft.ID] {
		t.Fatalf("expected both draft articles across pages, got %#v", seenDrafts)
	}

	literalWildcardRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/articles?q=%25", nil)
	addCookies(literalWildcardRequest, owner.cookies)
	literalWildcardResponse := mustTest(t, server, literalWildcardRequest)
	var literalWildcard ListEnvelope[store.AdminArticle]
	decodeJSONResponse(t, literalWildcardResponse, &literalWildcard)
	if len(literalWildcard.Data) != 0 {
		t.Fatalf("expected percent to be treated as a literal search character, got %#v", literalWildcard.Data)
	}

	invalidFilterRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles?publicationState=unknown",
		nil,
	)
	addCookies(invalidFilterRequest, owner.cookies)
	if response := mustTest(t, server, invalidFilterRequest); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected unsupported article filter 400, got %d: %s", response.StatusCode, readBody(t, response))
	}

	restorePath := "/api/v1/projects/" + project.ID + "/articles/" + archived.ID + "/restore"
	writerRestore := mustTest(t, server, newMemberMutationRequest(http.MethodPost, restorePath, `{}`, writer))
	if writerRestore.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer restore denial, got %d: %s", writerRestore.StatusCode, readBody(t, writerRestore))
	}
	ownerRestore := mustTest(t, server, newMemberMutationRequest(http.MethodPost, restorePath, `{}`, owner))
	if ownerRestore.StatusCode != http.StatusOK {
		t.Fatalf("expected owner restore 200, got %d: %s", ownerRestore.StatusCode, readBody(t, ownerRestore))
	}
	var restored Envelope[store.AdminArticle]
	decodeJSONResponse(t, ownerRestore, &restored)
	if restored.Data.ID != archived.ID || restored.Data.ArchivedAt != "" || restored.Data.PublicationState != "unpublished" {
		t.Fatalf("expected restored unpublished article, got %#v", restored.Data)
	}

	publicRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/"+archived.Slug, nil)
	publicRequest.Header.Set("X-Dev-Project-ID", project.ID)
	if response := mustTest(t, server, publicRequest); response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected restore to remain unpublished, got %d: %s", response.StatusCode, readBody(t, response))
	}

	var restoreAudits, restoreEvents int
	if err := db.QueryRow(`SELECT COUNT(1) FROM audit_events WHERE project_id = ? AND target_id = ? AND action = 'content.restore'`, project.ID, archived.ID).Scan(&restoreAudits); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(1) FROM outbox_events WHERE project_id = ? AND aggregate_id = ? AND event_type = 'content.restored'`, project.ID, archived.ID).Scan(&restoreEvents); err != nil {
		t.Fatal(err)
	}
	if restoreAudits != 1 || restoreEvents != 1 {
		t.Fatalf("expected one restore audit and event, got audits=%d events=%d", restoreAudits, restoreEvents)
	}
}

func TestArticleAutosaveRecoversDraftsAndRejectsConflicts(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "autosave-owner@example.test", "correct horse battery staple")
	writerLogin := seedAndLogin(t, server, db, "autosave-writer@example.test", "another correct horse battery staple")
	project := createTestProject(t, server, ownerLogin, `{"slug":"autosave","name":"Autosave Project","primaryDomain":"example.test"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, writerLogin.userID); err != nil {
		t.Fatal(err)
	}
	category := createTestCategory(t, server, ownerLogin, project.ID, `{"slug":"guides","name":"Guides"}`)
	tag := createTestTag(t, server, ownerLogin, project.ID, `{"slug":"autosave-tag","name":"Autosave Tag"}`)
	author := createTestAuthor(t, server, ownerLogin, project.ID, `{"slug":"autosave-author","displayName":"Autosave Author"}`)
	article := createTestArticle(t, server, ownerLogin, project.ID, `{
		"articleType":"guide",
		"title":"Autosave Guide",
		"slug":"autosave-guide",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>Published base</p>",
		"contributors":[{"authorId":"`+author.ID+`","role":"primary_author","position":0}]
	}`)
	baseRevisionID := article.LatestRevision.ID
	autosavePath := "/api/v1/projects/" + project.ID + "/articles/" + article.ID + "/autosave"
	autosaveBody := func(baseRevisionID string, expectedVersion int64, title string) string {
		t.Helper()
		body, err := json.Marshal(map[string]any{
			"baseRevisionId":  baseRevisionID,
			"expectedVersion": expectedVersion,
			"draft": map[string]any{
				"title":             title,
				"primaryCategoryId": category.ID,
				"tagIds":            []string{tag.ID},
				"contributors": []map[string]any{{
					"authorId": author.ID,
					"role":     "primary_author",
					"position": 0,
				}},
				"attributionEdited": true,
				"html":              "<p>Recovered working draft</p>",
				"bodyDocument": map[string]any{
					"type":    "doc",
					"content": []map[string]any{},
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}

	missingRequest := httptest.NewRequest(http.MethodGet, autosavePath, nil)
	addCookies(missingRequest, ownerLogin.cookies)
	missingResponse := mustTest(t, server, missingRequest)
	if missingResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected an absent autosave to return 404, got %d: %s", missingResponse.StatusCode, readBody(t, missingResponse))
	}
	missingVersionRequest := newMemberMutationRequest(
		http.MethodPut,
		autosavePath,
		`{"baseRevisionId":"`+baseRevisionID+`","draft":{"title":"Missing version"}}`,
		ownerLogin,
	)
	missingVersionResponse := mustTest(t, server, missingVersionRequest)
	if missingVersionResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected missing expectedVersion to return 400, got %d: %s", missingVersionResponse.StatusCode, readBody(t, missingVersionResponse))
	}

	createRequest := newMemberMutationRequest(http.MethodPut, autosavePath, autosaveBody(baseRevisionID, 0, "First autosave"), ownerLogin)
	createResponse := mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected autosave creation 200, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.ArticleAutosave]
	decodeJSONResponse(t, createResponse, &created)
	if created.Data.Version != 1 || created.Data.Stale || created.Data.Draft.Title != "First autosave" || !reflect.DeepEqual(created.Data.Draft.TagIDs, []string{tag.ID}) {
		t.Fatalf("unexpected created autosave: %#v", created.Data)
	}
	createdDocument, ok := created.Data.Draft.BodyDocument.(map[string]any)
	if !ok || createdDocument["type"] != "doc" || len(created.Data.Draft.Contributors) != 1 {
		t.Fatalf("expected the structured body and contributor fields to round trip, got draft=%#v", created.Data.Draft)
	}

	conflictRequest := newMemberMutationRequest(http.MethodPut, autosavePath, autosaveBody(baseRevisionID, 0, "Conflicting tab"), ownerLogin)
	conflictResponse := mustTest(t, server, conflictRequest)
	if conflictResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected stale autosave version to return 409, got %d: %s", conflictResponse.StatusCode, readBody(t, conflictResponse))
	}

	updateRequest := newMemberMutationRequest(http.MethodPut, autosavePath, autosaveBody(baseRevisionID, 1, "Second autosave"), ownerLogin)
	updateResponse := mustTest(t, server, updateRequest)
	if updateResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected autosave update 200, got %d: %s", updateResponse.StatusCode, readBody(t, updateResponse))
	}
	var updated Envelope[store.ArticleAutosave]
	decodeJSONResponse(t, updateResponse, &updated)
	if updated.Data.Version != 2 || updated.Data.Draft.Title != "Second autosave" {
		t.Fatalf("unexpected updated autosave: %#v", updated.Data)
	}

	writerGetRequest := httptest.NewRequest(http.MethodGet, autosavePath, nil)
	addCookies(writerGetRequest, writerLogin.cookies)
	writerGetResponse := mustTest(t, server, writerGetRequest)
	if writerGetResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected autosaves to be user scoped, got %d: %s", writerGetResponse.StatusCode, readBody(t, writerGetResponse))
	}

	newRevision := createTestRevision(t, server, writerLogin, project.ID, article.ID, `{
		"title":"A newer immutable revision",
		"html":"<p>Newer body</p>"
	}`)
	staleGetRequest := httptest.NewRequest(http.MethodGet, autosavePath, nil)
	addCookies(staleGetRequest, ownerLogin.cookies)
	staleGetResponse := mustTest(t, server, staleGetRequest)
	if staleGetResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected stale autosave recovery 200, got %d: %s", staleGetResponse.StatusCode, readBody(t, staleGetResponse))
	}
	var stale Envelope[store.ArticleAutosave]
	decodeJSONResponse(t, staleGetResponse, &stale)
	if !stale.Data.Stale || stale.Data.BaseRevisionID != baseRevisionID || stale.Data.Draft.Title != "Second autosave" {
		t.Fatalf("expected the original recoverable draft to be marked stale, got %#v", stale.Data)
	}

	staleBaseRequest := newMemberMutationRequest(http.MethodPut, autosavePath, autosaveBody(baseRevisionID, 2, "Must not overwrite"), ownerLogin)
	staleBaseResponse := mustTest(t, server, staleBaseRequest)
	if staleBaseResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected stale autosave base to return 409, got %d: %s", staleBaseResponse.StatusCode, readBody(t, staleBaseResponse))
	}

	deleteRequest := newMemberMutationRequest(http.MethodDelete, autosavePath, "", ownerLogin)
	deleteResponse := mustTest(t, server, deleteRequest)
	if deleteResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("expected autosave deletion 204, got %d: %s", deleteResponse.StatusCode, readBody(t, deleteResponse))
	}

	recreateRequest := newMemberMutationRequest(http.MethodPut, autosavePath, autosaveBody(newRevision.ID, 0, "Draft before revision save"), ownerLogin)
	recreateResponse := mustTest(t, server, recreateRequest)
	if recreateResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected autosave recreation 200, got %d: %s", recreateResponse.StatusCode, readBody(t, recreateResponse))
	}
	createTestRevision(t, server, ownerLogin, project.ID, article.ID, `{
		"title":"Committed autosave",
		"html":"<p>Committed body</p>"
	}`)
	afterRevisionRequest := httptest.NewRequest(http.MethodGet, autosavePath, nil)
	addCookies(afterRevisionRequest, ownerLogin.cookies)
	afterRevisionResponse := mustTest(t, server, afterRevisionRequest)
	if afterRevisionResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected revision creation to clear the actor's autosave, got %d: %s", afterRevisionResponse.StatusCode, readBody(t, afterRevisionResponse))
	}
}

func TestArticleSEOInputIsRevisionedAndControlsPublishedRobots(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "seo-input-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"seo-input","name":"SEO Input","primaryDomain":"seo.example.test"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, login, project.ID, `{
		"articleType":"guide",
		"title":"Editorial title",
		"slug":"seo-fields",
		"primaryCategoryId":"`+category.ID+`",
		"excerpt":"Editorial excerpt",
		"bodyDocument":{"type":"doc","content":[]},
		"html":"<p>SEO body</p>",
		"seo":{
			"title":"Search title",
			"description":"Search description",
			"robots":"noindex,nofollow",
			"openGraphTitle":"Social title",
			"openGraphDescription":"Social description",
			"openGraphImage":"https://cdn.example.test/social.png"
		}
	}`)
	detail := getTestRevisionDetail(t, db, project.ID, article.ID, article.LatestRevision.ID)
	seo, ok := detail.SEOSnapshot.(map[string]any)
	if !ok || seo["title"] != "Search title" || seo["robots"] != "noindex,nofollow" {
		t.Fatalf("unexpected revision SEO snapshot: %#v", detail.SEOSnapshot)
	}
	openGraph, ok := seo["openGraph"].(map[string]any)
	if !ok || openGraph["title"] != "Social title" || openGraph["image"] != "https://cdn.example.test/social.png" {
		t.Fatalf("unexpected Open Graph snapshot: %#v", seo["openGraph"])
	}
	updated := createTestRevision(t, server, login, project.ID, article.ID, `{
		"title":"Updated editorial title",
		"bodyDocument":{"type":"doc","content":[]},
		"html":"<p>Updated SEO body</p>"
	}`)
	updatedDetail := getTestRevisionDetail(t, db, project.ID, article.ID, updated.ID)
	updatedSEO, ok := updatedDetail.SEOSnapshot.(map[string]any)
	if !ok || updatedSEO["title"] != "Search title" || updatedSEO["robots"] != "noindex,nofollow" {
		t.Fatalf("an omitted SEO object must preserve the base revision snapshot: %#v", updatedDetail.SEOSnapshot)
	}

	publishTestArticle(t, server, login, project.ID, article.ID, "seo-fields")
	var robots string
	if err := db.QueryRow(`
		SELECT robots_directive
		FROM project_publications
		WHERE project_id = ? AND content_id = ?
	`, project.ID, article.ID).Scan(&robots); err != nil {
		t.Fatal(err)
	}
	if robots != "noindex,nofollow" {
		t.Fatalf("expected published robots directive from the revision, got %q", robots)
	}
}

func TestArticlePartialSavePreservesOmittedFieldsAndMergesSEO(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "partial-save@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"partial-save","name":"Partial Save"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, login, project.ID, `{
		"title":"Original title",
		"slug":"partial-save",
		"primaryCategoryId":"`+category.ID+`",
		"deck":"Original deck",
		"excerpt":"Original excerpt",
		"shortAnswer":"Original answer",
		"bodyDocument":{"type":"doc","content":[]},
		"html":"<p>Original body</p>",
		"seo":{
			"title":"Original search title",
			"description":"Original search description",
			"robots":"noindex,nofollow",
			"openGraphTitle":"Original social title",
			"openGraphDescription":"Original social description",
			"openGraphImage":"https://cdn.example.test/original.png"
		}
	}`)

	titleOnly := newMemberMutationRequest(
		http.MethodPut,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID,
		`{"title":"Renamed only"}`,
		login,
	)
	titleOnlyResponse := mustTest(t, server, titleOnly)
	if titleOnlyResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected title-only save 200, got %d: %s", titleOnlyResponse.StatusCode, readBody(t, titleOnlyResponse))
	}
	var renamed Envelope[store.AdminArticle]
	decodeJSONResponse(t, titleOnlyResponse, &renamed)
	if renamed.Data.Deck != "Original deck" || renamed.Data.Excerpt != "Original excerpt" || renamed.Data.ShortAnswer != "Original answer" || renamed.Data.HTML != "<p>Original body</p>" {
		t.Fatalf("title-only save lost editable fields: %#v", renamed.Data)
	}
	if renamed.Data.PrimaryCategoryID != category.ID || len(renamed.Data.Contributors) != len(article.Contributors) || !reflect.DeepEqual(renamed.Data.BodyDocument, article.BodyDocument) {
		t.Fatalf("title-only save lost structured relationships: %#v", renamed.Data)
	}
	if renamed.Data.SEO.Title != "Original search title" || renamed.Data.SEO.Robots != "noindex,nofollow" || renamed.Data.SEO.OpenGraphImage != "https://cdn.example.test/original.png" {
		t.Fatalf("title-only save lost SEO fields: %#v", renamed.Data.SEO)
	}

	clearAndMerge := newMemberMutationRequest(
		http.MethodPut,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID,
		`{"title":"Cleared metadata","deck":"","excerpt":"","shortAnswer":"","seo":{"description":"Updated description"}}`,
		login,
	)
	clearResponse := mustTest(t, server, clearAndMerge)
	if clearResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected explicit clearing save 200, got %d: %s", clearResponse.StatusCode, readBody(t, clearResponse))
	}
	var cleared Envelope[store.AdminArticle]
	decodeJSONResponse(t, clearResponse, &cleared)
	if cleared.Data.Deck != "" || cleared.Data.Excerpt != "" || cleared.Data.ShortAnswer != "" || cleared.Data.HTML != "<p>Original body</p>" {
		t.Fatalf("expected explicit metadata clearing without body loss, got %#v", cleared.Data)
	}
	if cleared.Data.SEO.Description != "Updated description" || cleared.Data.SEO.Title != "Original search title" || cleared.Data.SEO.Robots != "noindex,nofollow" || cleared.Data.SEO.OpenGraphImage != "https://cdn.example.test/original.png" {
		t.Fatalf("expected nested SEO merge, got %#v", cleared.Data.SEO)
	}

	mismatchedBody := newMemberMutationRequest(
		http.MethodPut,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID,
		`{"title":"Mismatched body","html":"<p>Only HTML</p>"}`,
		login,
	)
	mismatchedResponse := mustTest(t, server, mismatchedBody)
	if mismatchedResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected one-sided body save to fail with 400, got %d: %s", mismatchedResponse.StatusCode, readBody(t, mismatchedResponse))
	}
}

func TestArticleTagAssignmentsCreatePreserveReplaceAndClear(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "article-tags@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"article-tags","name":"Article Tags"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"guides","name":"Guides"}`)
	secondaryCategory := createTestCategory(t, server, login, project.ID, `{"slug":"reference","name":"Reference"}`)
	firstTag := createTestTag(t, server, login, project.ID, `{"slug":"technical-seo","name":"Technical SEO"}`)
	secondTag := createTestTag(t, server, login, project.ID, `{"slug":"content-strategy","name":"Content Strategy"}`)

	article := createTestArticle(t, server, login, project.ID, `{
		"title":"Tagged article",
		"slug":"tagged-article",
		"primaryCategoryId":"`+category.ID+`",
		"tagIds":["`+firstTag.ID+`"]
	}`)
	if !reflect.DeepEqual(article.TagIDs, []string{firstTag.ID}) || len(article.Tags) != 1 || article.Tags[0].ID != firstTag.ID {
		t.Fatalf("expected created article tags to round-trip, got ids=%#v tags=%#v", article.TagIDs, article.Tags)
	}
	topicID := "term-article-tags-topic"
	if _, err := db.Exec(`
		INSERT INTO taxonomy_terms(id, project_id, type, slug, name)
		VALUES (?, ?, 'topic', 'search-intent', 'Search Intent')
	`, topicID, project.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO article_taxonomy(project_id, content_id, taxonomy_term_id, is_primary, position)
		VALUES (?, ?, ?, 0, 20), (?, ?, ?, 0, 21)
	`, project.ID, article.ID, secondaryCategory.ID, project.ID, article.ID, topicID); err != nil {
		t.Fatal(err)
	}
	nonTagAssignments := []string{secondaryCategory.ID, topicID}
	assertArticleNonTagAssignments(t, db, project.ID, article.ID, nonTagAssignments)
	assertArticleTagAssignments(t, db, project.ID, article.ID, []string{firstTag.ID})
	assertRevisionSnapshotTags(t, db, project.ID, article.ID, []string{firstTag.ID})

	titleOnly := newMemberMutationRequest(
		http.MethodPut,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID,
		`{"title":"Still tagged"}`,
		login,
	)
	titleOnlyResponse := mustTest(t, server, titleOnly)
	if titleOnlyResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected title-only save 200, got %d: %s", titleOnlyResponse.StatusCode, readBody(t, titleOnlyResponse))
	}
	var preserved Envelope[store.AdminArticle]
	decodeJSONResponse(t, titleOnlyResponse, &preserved)
	if !reflect.DeepEqual(preserved.Data.TagIDs, []string{firstTag.ID}) {
		t.Fatalf("expected omitted tagIds to preserve tags, got %#v", preserved.Data.TagIDs)
	}
	assertArticleTagAssignments(t, db, project.ID, article.ID, []string{firstTag.ID})
	assertArticleNonTagAssignments(t, db, project.ID, article.ID, nonTagAssignments)

	replace := newMemberMutationRequest(
		http.MethodPut,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID,
		`{"title":"Retagged","tagIds":["`+secondTag.ID+`"]}`,
		login,
	)
	replaceResponse := mustTest(t, server, replace)
	if replaceResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected retag save 200, got %d: %s", replaceResponse.StatusCode, readBody(t, replaceResponse))
	}
	var retagged Envelope[store.AdminArticle]
	decodeJSONResponse(t, replaceResponse, &retagged)
	if !reflect.DeepEqual(retagged.Data.TagIDs, []string{secondTag.ID}) {
		t.Fatalf("expected replacement tags, got %#v", retagged.Data.TagIDs)
	}
	assertArticleTagAssignments(t, db, project.ID, article.ID, []string{secondTag.ID})
	assertArticleNonTagAssignments(t, db, project.ID, article.ID, nonTagAssignments)
	assertRevisionSnapshotTags(t, db, project.ID, article.ID, []string{secondTag.ID})

	clear := newMemberMutationRequest(
		http.MethodPut,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID,
		`{"title":"Untagged","tagIds":[]}`,
		login,
	)
	clearResponse := mustTest(t, server, clear)
	if clearResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected clear tags save 200, got %d: %s", clearResponse.StatusCode, readBody(t, clearResponse))
	}
	var cleared Envelope[store.AdminArticle]
	decodeJSONResponse(t, clearResponse, &cleared)
	if len(cleared.Data.TagIDs) != 0 || len(cleared.Data.Tags) != 0 {
		t.Fatalf("expected explicit empty tagIds to clear tags, got ids=%#v tags=%#v", cleared.Data.TagIDs, cleared.Data.Tags)
	}
	assertArticleTagAssignments(t, db, project.ID, article.ID, nil)
	assertArticleNonTagAssignments(t, db, project.ID, article.ID, nonTagAssignments)
	assertRevisionSnapshotTags(t, db, project.ID, article.ID, nil)

	tooManyTagIDs := make([]string, 101)
	for index := range tooManyTagIDs {
		tooManyTagIDs[index] = fmt.Sprintf("tag-%03d", index)
	}
	tooManyBody, err := json.Marshal(map[string]any{"title": "Too many tags", "tagIds": tooManyTagIDs})
	if err != nil {
		t.Fatal(err)
	}
	tooManyRequest := newMemberMutationRequest(
		http.MethodPut,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID,
		string(tooManyBody),
		login,
	)
	tooManyResponse := mustTest(t, server, tooManyRequest)
	if tooManyResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected more than 100 article tags to fail with 400, got %d: %s", tooManyResponse.StatusCode, readBody(t, tooManyResponse))
	}
}

func TestPublishCanonicalOmissionPreservesAndExplicitEmptyResets(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "canonical-owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"canonical-publish","name":"Canonical Publish","primaryDomain":"example.test","blogBasePath":"/stories"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, login, project.ID, `{"title":"Canonical Guide","slug":"canonical-guide","primaryCategoryId":"`+category.ID+`","html":"<p>Canonical body</p>"}`)
	path := "/api/v1/projects/" + project.ID + "/articles/" + article.ID + "/publish"

	customRequest := newMemberMutationRequest(http.MethodPost, path, `{"canonicalUrl":"https://external.example.test/custom"}`, login)
	customResponse := mustTest(t, server, customRequest)
	if customResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected custom canonical publish 200, got %d: %s", customResponse.StatusCode, readBody(t, customResponse))
	}
	var custom Envelope[store.AdminArticle]
	decodeJSONResponse(t, customResponse, &custom)
	if custom.Data.CanonicalURL != "https://external.example.test/custom" {
		t.Fatalf("unexpected custom canonical %q", custom.Data.CanonicalURL)
	}

	for _, body := range []string{"", `{"slug":"canonical-guide"}`} {
		preserveRequest := newMemberMutationRequest(http.MethodPost, path, body, login)
		preserveResponse := mustTest(t, server, preserveRequest)
		if preserveResponse.StatusCode != http.StatusOK {
			t.Fatalf("expected omitted canonical publish 200, got %d: %s", preserveResponse.StatusCode, readBody(t, preserveResponse))
		}
		var preserved Envelope[store.AdminArticle]
		decodeJSONResponse(t, preserveResponse, &preserved)
		if preserved.Data.CanonicalURL != "https://external.example.test/custom" {
			t.Fatalf("omitted canonical replaced custom URL with %q", preserved.Data.CanonicalURL)
		}
	}

	resetRequest := newMemberMutationRequest(http.MethodPost, path, `{"canonicalUrl":""}`, login)
	resetResponse := mustTest(t, server, resetRequest)
	if resetResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected canonical reset publish 200, got %d: %s", resetResponse.StatusCode, readBody(t, resetResponse))
	}
	var reset Envelope[store.AdminArticle]
	decodeJSONResponse(t, resetResponse, &reset)
	if reset.Data.CanonicalURL != "https://example.test/stories/canonical-guide" {
		t.Fatalf("expected generated self canonical, got %q", reset.Data.CanonicalURL)
	}

	invalidRequest := newMemberMutationRequest(http.MethodPost, path, `{"canonicalUrl":"javascript:alert(1)"}`, login)
	invalidResponse := mustTest(t, server, invalidRequest)
	if invalidResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid canonical 400, got %d: %s", invalidResponse.StatusCode, readBody(t, invalidResponse))
	}
}

func TestArticleSaveDefaultsBaseAndRejectsStaleToken(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{"slug":"revision-conflict","name":"Revision Conflict"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"conflicts","name":"Conflicts"}`)
	article := createTestArticle(t, server, login, project.ID, `{
		"title":"Revision One",
		"slug":"revision-conflict",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>Revision one</p>"
	}`)
	firstRevisionID := article.LatestRevision.ID

	defaultBaseRequest := newMemberMutationRequest(
		http.MethodPut,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID,
		`{"title":"Revision Two","bodyDocument":{"type":"doc","content":[]},"html":"<p>Revision two</p>"}`,
		login,
	)
	defaultBaseResponse := mustTest(t, server, defaultBaseRequest)
	if defaultBaseResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected the server to default the save token, got %d: %s", defaultBaseResponse.StatusCode, readBody(t, defaultBaseResponse))
	}
	var saved Envelope[store.AdminArticle]
	decodeJSONResponse(t, defaultBaseResponse, &saved)
	if saved.Data.LatestRevision == nil {
		t.Fatal("expected saved article to expose its latest save token")
	}
	secondRevision := *saved.Data.LatestRevision
	staleBaseRequest := newMemberMutationRequest(
		http.MethodPut,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID,
		`{"baseRevisionId":"`+firstRevisionID+`","title":"Stale Revision Three","bodyDocument":{"type":"doc","content":[]},"html":"<p>Stale</p>"}`,
		login,
	)
	staleBaseResponse := mustTest(t, server, staleBaseRequest)
	if staleBaseResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected stale base 409, got %d: %s", staleBaseResponse.StatusCode, readBody(t, staleBaseResponse))
	}

	thirdRevision := createTestRevision(t, server, login, project.ID, article.ID, `{
		"baseRevisionId":"`+secondRevision.ID+`",
		"title":"Revision Three",
		"html":"<p>Revision three</p>"
	}`)
	type saveSnapshot struct {
		ID             string
		BaseRevisionID string
	}
	rows, err := db.Query(`
		SELECT id, COALESCE(base_revision_id, '')
		FROM content_revisions
		WHERE project_id = ? AND content_id = ?
		ORDER BY revision_number DESC
		LIMIT 3
	`, project.ID, article.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var history []saveSnapshot
	for rows.Next() {
		var snapshot saveSnapshot
		if err := rows.Scan(&snapshot.ID, &snapshot.BaseRevisionID); err != nil {
			t.Fatal(err)
		}
		history = append(history, snapshot)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("expected three internal save snapshots after rejecting stale write, got %d", len(history))
	}
	if history[0].ID != thirdRevision.ID || history[0].BaseRevisionID != secondRevision.ID {
		t.Fatalf("unexpected third save lineage %#v", history[0])
	}
	if history[1].ID != secondRevision.ID || history[1].BaseRevisionID != firstRevisionID {
		t.Fatalf("unexpected second save lineage %#v", history[1])
	}
}

func TestRepublishEmitsDistinctUpdateAndOneHopSlugChangeEvents(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "publish-events@example.test", "correct horse battery staple")
	project := createTestProject(t, server, login, `{
		"slug":"publish-events",
		"name":"Publish Events",
		"primaryDomain":"example.test",
		"blogBasePath":"/insights"
	}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"news","name":"News"}`)
	article := createTestArticle(t, server, login, project.ID, `{
		"title":"First publication",
		"slug":"first-publication",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>First publication</p>"
	}`)
	publishTestArticle(t, server, login, project.ID, article.ID, "first-publication")

	createTestRevision(t, server, login, project.ID, article.ID, `{
		"title":"Updated publication",
		"html":"<p>Updated publication</p>"
	}`)
	publishTestArticle(t, server, login, project.ID, article.ID, "first-publication")
	publishTestArticle(t, server, login, project.ID, article.ID, "renamed-publication")

	type eventRecord struct {
		Type    string
		Payload string
	}
	rows, err := db.Query(`
		SELECT event_type, payload_json
		FROM outbox_events
		WHERE project_id = ? AND aggregate_id = ?
		ORDER BY created_at, rowid
	`, project.ID, article.ID)
	if err != nil {
		t.Fatal(err)
	}
	var events []eventRecord
	for rows.Next() {
		var event eventRecord
		if err := rows.Scan(&event.Type, &event.Payload); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 ||
		events[0].Type != "content.published" ||
		events[1].Type != "content.updated" ||
		events[2].Type != "content.slug_changed" {
		t.Fatalf("unexpected publication events %#v", events)
	}
	var slugPayload map[string]any
	if err := json.Unmarshal([]byte(events[2].Payload), &slugPayload); err != nil {
		t.Fatal(err)
	}
	if slugPayload["old_slug"] != "first-publication" ||
		slugPayload["new_slug"] != "renamed-publication" {
		t.Fatalf("unexpected slug event payload %#v", slugPayload)
	}
	var targetPath string
	if err := db.QueryRow(`
		SELECT target_path
		FROM slug_redirects
		WHERE project_id = ? AND source_path = '/insights/first-publication'
	`, project.ID).Scan(&targetPath); err != nil {
		t.Fatal(err)
	}
	if targetPath != "/insights/renamed-publication" {
		t.Fatalf("expected old slug to redirect directly to current slug, got %q", targetPath)
	}

	publishTestArticle(t, server, login, project.ID, article.ID, "first-publication")
	var oldSourceCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM slug_redirects
		WHERE project_id = ? AND source_path = '/insights/first-publication'
	`, project.ID).Scan(&oldSourceCount); err != nil {
		t.Fatal(err)
	}
	if oldSourceCount != 0 {
		t.Fatal("expected reverting a slug to remove the redirect whose source is now live")
	}
	if err := db.QueryRow(`
		SELECT target_path
		FROM slug_redirects
		WHERE project_id = ? AND source_path = '/insights/renamed-publication'
	`, project.ID).Scan(&targetPath); err != nil {
		t.Fatal(err)
	}
	if targetPath != "/insights/first-publication" {
		t.Fatalf("expected reverted slug history to remain one hop, got %q", targetPath)
	}
}

func TestArticleWorkflowUsesSinglePublication(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")

	project := createTestProject(t, server, login, `{
		"slug":"english-workflow",
		"name":"English Workflow",
		"primaryDomain":"example.test"
	}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, login, project.ID, `{
		"articleType":"guide",
		"title":"Original Guide",
		"slug":"english-guide",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>Original body</p>"
	}`)
	firstRevisionID := article.LatestRevision.ID
	publishTestArticle(t, server, login, project.ID, article.ID, "english-guide")

	secondRevision := createTestRevision(t, server, login, project.ID, article.ID, `{
		"title":"Updated Guide",
		"html":"<p>Updated body</p>"
	}`)
	publishTestArticle(t, server, login, project.ID, article.ID, "english-guide")

	var publicationCount int
	var publishedRevisionID string
	if err := db.QueryRow(`
		SELECT COUNT(*), MIN(published_revision_id)
		FROM project_publications
		WHERE project_id = ? AND content_id = ?
	`, project.ID, article.ID).Scan(&publicationCount, &publishedRevisionID); err != nil {
		t.Fatal(err)
	}
	if publicationCount != 1 || publishedRevisionID != secondRevision.ID {
		t.Fatalf("expected one publication updated to latest save %q (initial %q), got count=%d revision=%q", secondRevision.ID, firstRevisionID, publicationCount, publishedRevisionID)
	}

	englishRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/english-guide", nil)
	englishRequest.Header.Set("X-Dev-Project-ID", project.ID)
	englishResponse := mustTest(t, server, englishRequest)
	if englishResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected English article 200, got %d: %s", englishResponse.StatusCode, readBody(t, englishResponse))
	}
	var english Envelope[store.PublishedPost]
	decodeJSONResponse(t, englishResponse, &english)
	if english.Data.Title != "Updated Guide" {
		t.Fatalf("expected the latest publication, got title=%q", english.Data.Title)
	}
}

func TestCopyArticleToProjectCreatesIndependentAuditedDraft(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "copy-owner@example.test", "correct horse battery staple")
	sourceProject := createTestProject(t, server, login, `{"slug":"copy-source","name":"Copy Source","primaryDomain":"source.example.test"}`)
	destinationProject := createTestProject(t, server, login, `{"slug":"copy-destination","name":"Copy Destination","primaryDomain":"destination.example.test"}`)
	sourceCategory := createTestCategory(t, server, login, sourceProject.ID, `{"slug":"source","name":"Source"}`)
	destinationCategory := createTestCategory(t, server, login, destinationProject.ID, `{"slug":"destination","name":"Destination"}`)
	destinationAuthor := createTestAuthor(t, server, login, destinationProject.ID, `{"slug":"destination-author","displayName":"Destination Author"}`)
	sourceArticle := createTestArticle(t, server, login, sourceProject.ID, `{
		"articleType":"guide",
		"title":"Original draft",
		"slug":"copy-me",
		"primaryCategoryId":"`+sourceCategory.ID+`",
		"html":"<p>Original body</p>"
	}`)
	createTestRevision(t, server, login, sourceProject.ID, sourceArticle.ID, `{
		"title":"Selected source revision",
		"deck":"Selected deck",
		"excerpt":"Selected excerpt",
		"shortAnswer":"Selected answer",
		"bodyDocument":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Selected body"}]}]},
		"html":"<p>Selected body</p>"
	}`)
	newerRevision := createTestRevision(t, server, login, sourceProject.ID, sourceArticle.ID, `{
		"title":"Newer source revision",
		"html":"<p>Newer body</p>"
	}`)
	var sourceAuthorID string
	if err := db.QueryRow(`SELECT author_id FROM revision_contributors WHERE project_id = ? AND revision_id = ? AND role = 'primary_author'`, sourceProject.ID, newerRevision.ID).Scan(&sourceAuthorID); err != nil {
		t.Fatal(err)
	}

	copyRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+sourceProject.ID+"/articles/"+sourceArticle.ID+"/copy-to-project",
		`{
			"destinationProjectId":"`+destinationProject.ID+`",
			"primaryCategoryId":"`+destinationCategory.ID+`",
			"slug":"copied-guide",
			"canonicalDecision":"canonical_original",
			"canonicalOriginalUrl":"https://source.example.test/blog/copy-me",
			"contributorMappings":[{"sourceAuthorId":"`+sourceAuthorID+`","destinationAuthorId":"`+destinationAuthor.ID+`"}]
		}`,
		login,
	)
	copyResponse := mustTest(t, server, copyRequest)
	if copyResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected copy 201, got %d: %s", copyResponse.StatusCode, readBody(t, copyResponse))
	}
	var copied Envelope[store.AdminArticle]
	decodeJSONResponse(t, copyResponse, &copied)
	if copied.Data.ID == sourceArticle.ID {
		t.Fatal("expected a new destination article ID")
	}
	if copied.Data.ProjectID != destinationProject.ID ||
		copied.Data.OriginProjectID != sourceProject.ID ||
		copied.Data.OriginArticleID != sourceArticle.ID {
		t.Fatalf("unexpected copied ownership and origin %#v", copied.Data)
	}
	if copied.Data.Title != newerRevision.Title {
		t.Fatalf("expected the latest saved source content to be copied, got title %q", copied.Data.Title)
	}
	if copied.Data.PublicationState != "unpublished" {
		t.Fatalf("expected independent unpublished draft, got %#v", copied.Data)
	}
	if copied.Data.CanonicalPolicy != "canonical_original" ||
		copied.Data.CanonicalURL != "https://source.example.test/blog/copy-me" {
		t.Fatalf("unexpected canonical-original decision %#v", copied.Data)
	}
	if copied.Data.LatestRevision == nil || copied.Data.LatestRevision.ID == newerRevision.ID ||
		copied.Data.LatestRevision.RevisionNumber != 1 {
		t.Fatalf("expected independent first revision, got %#v", copied.Data.LatestRevision)
	}
	var copiedAuthorSnapshot string
	if err := db.QueryRow(`SELECT author_snapshot_json FROM content_revisions WHERE project_id = ? AND id = ?`, destinationProject.ID, copied.Data.LatestRevision.ID).Scan(&copiedAuthorSnapshot); err != nil {
		t.Fatal(err)
	}
	var copiedAuthors []store.Author
	if err := json.Unmarshal([]byte(copiedAuthorSnapshot), &copiedAuthors); err != nil {
		t.Fatal(err)
	}
	if len(copiedAuthors) != 1 || copiedAuthors[0].ID != destinationAuthor.ID {
		t.Fatalf("expected remapped immutable destination attribution, got %#v", copiedAuthors)
	}

	var copiedBody, copiedDeck, copiedExcerpt, copiedShortAnswer, baseRevisionID string
	var copiedRevisionCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM content_revisions
		WHERE project_id = ? AND content_id = ?
	`, destinationProject.ID, copied.Data.ID).Scan(&copiedRevisionCount); err != nil {
		t.Fatal(err)
	}
	if copiedRevisionCount != 1 {
		t.Fatalf("expected one independent destination revision, got %d", copiedRevisionCount)
	}
	if err := db.QueryRow(`
		SELECT sanitized_html, COALESCE(deck, ''), COALESCE(excerpt, ''),
		       COALESCE(short_answer, ''), COALESCE(base_revision_id, '')
		FROM content_revisions
		WHERE project_id = ? AND content_id = ?
	`, destinationProject.ID, copied.Data.ID).Scan(
		&copiedBody,
		&copiedDeck,
		&copiedExcerpt,
		&copiedShortAnswer,
		&baseRevisionID,
	); err != nil {
		t.Fatal(err)
	}
	if copiedBody != "<p>Newer body</p>" ||
		copiedDeck != "Selected deck" ||
		copiedExcerpt != "Selected excerpt" ||
		copiedShortAnswer != "Selected answer" ||
		baseRevisionID != "" {
		t.Fatalf("unexpected copied revision data: %q %q %q %q base=%q", copiedBody, copiedDeck, copiedExcerpt, copiedShortAnswer, baseRevisionID)
	}

	var originProjectID, originArticleID, taxonomyTermID string
	if err := db.QueryRow(`
		SELECT origin_project_id, origin_content_id
		FROM content_items
		WHERE project_id = ? AND id = ?
	`, destinationProject.ID, copied.Data.ID).Scan(&originProjectID, &originArticleID); err != nil {
		t.Fatal(err)
	}
	if originProjectID != sourceProject.ID || originArticleID != sourceArticle.ID {
		t.Fatalf("unexpected stored origin %q/%q", originProjectID, originArticleID)
	}
	if err := db.QueryRow(`
		SELECT taxonomy_term_id
		FROM article_taxonomy
		WHERE project_id = ? AND content_id = ? AND is_primary = 1
	`, destinationProject.ID, copied.Data.ID).Scan(&taxonomyTermID); err != nil {
		t.Fatal(err)
	}
	if taxonomyTermID != destinationCategory.ID {
		t.Fatalf("expected destination taxonomy %q, got %q", destinationCategory.ID, taxonomyTermID)
	}

	for _, audit := range []struct {
		projectID string
		action    string
		targetID  string
	}{
		{sourceProject.ID, "content.copy_from", sourceArticle.ID},
		{destinationProject.ID, "content.copy_to", copied.Data.ID},
	} {
		var metadataJSON string
		if err := db.QueryRow(`
			SELECT metadata_json
			FROM audit_events
			WHERE project_id = ? AND action = ? AND target_id = ?
		`, audit.projectID, audit.action, audit.targetID).Scan(&metadataJSON); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(metadataJSON, `"canonicalDecision":"canonical_original"`) ||
			!strings.Contains(metadataJSON, newerRevision.ID) {
			t.Fatalf("expected origin and canonical decision in audit metadata, got %s", metadataJSON)
		}
	}

	createTestRevision(t, server, login, destinationProject.ID, copied.Data.ID, `{
		"title":"Destination-only adaptation",
		"html":"<p>Destination-only body</p>"
	}`)
	var sourceRevisionCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM content_revisions
		WHERE project_id = ? AND content_id = ?
	`, sourceProject.ID, sourceArticle.ID).Scan(&sourceRevisionCount); err != nil {
		t.Fatal(err)
	}
	if sourceRevisionCount != 3 {
		t.Fatalf("destination edits must not alter source history; got %d source revisions", sourceRevisionCount)
	}

	adaptationRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+sourceProject.ID+"/articles/"+sourceArticle.ID+"/copy-to-project",
		`{
			"destinationProjectId":"`+destinationProject.ID+`",
			"primaryCategoryId":"`+destinationCategory.ID+`",
			"slug":"adapted-guide",
			"canonicalDecision":"material_adaptation",
			"contributorMappings":[{"sourceAuthorId":"`+sourceAuthorID+`","destinationAuthorId":"`+destinationAuthor.ID+`"}]
		}`,
		login,
	)
	adaptationResponse := mustTest(t, server, adaptationRequest)
	if adaptationResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected material-adaptation copy 201, got %d: %s", adaptationResponse.StatusCode, readBody(t, adaptationResponse))
	}
	var adaptation Envelope[store.AdminArticle]
	decodeJSONResponse(t, adaptationResponse, &adaptation)
	if adaptation.Data.CanonicalPolicy != "material_adaptation" ||
		adaptation.Data.CanonicalURL != "https://destination.example.test/blog/adapted-guide" {
		t.Fatalf("expected destination self canonical for adaptation, got %#v", adaptation.Data)
	}
}

func TestCopyArticleToProjectDerivesCanonicalAndRejectsUnsafeSourceReferences(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "copy-validation-owner@example.test", "correct horse battery staple")
	sourceProject := createTestProject(t, server, login, `{"slug":"validation-source","name":"Validation Source","primaryDomain":"source-validation.example.test"}`)
	destinationProject := createTestProject(t, server, login, `{"slug":"validation-destination","name":"Validation Destination","primaryDomain":"destination-validation.example.test"}`)
	sourceCategory := createTestCategory(t, server, login, sourceProject.ID, `{"slug":"source","name":"Source"}`)
	destinationCategory := createTestCategory(t, server, login, destinationProject.ID, `{"slug":"destination","name":"Destination"}`)
	destinationAuthor := createTestAuthor(t, server, login, destinationProject.ID, `{"slug":"destination-author","displayName":"Destination Author"}`)
	sourceArticle := createTestArticle(t, server, login, sourceProject.ID, `{
		"title":"Canonical source",
		"slug":"canonical-source",
		"primaryCategoryId":"`+sourceCategory.ID+`",
		"html":"<p>Safe body</p>"
	}`)
	var sourceAuthorID string
	if err := db.QueryRow(`SELECT author_id FROM revision_contributors WHERE project_id = ? AND revision_id = ? AND role = 'primary_author'`, sourceProject.ID, sourceArticle.LatestRevision.ID).Scan(&sourceAuthorID); err != nil {
		t.Fatal(err)
	}
	copyPath := "/api/v1/projects/" + sourceProject.ID + "/articles/" + sourceArticle.ID + "/copy-to-project"
	copyBody := func(slug, decision, canonicalField string) string {
		return `{
			"destinationProjectId":"` + destinationProject.ID + `",
			"primaryCategoryId":"` + destinationCategory.ID + `",
			"slug":"` + slug + `",
			"canonicalDecision":"` + decision + `",
			"contributorMappings":[{"sourceAuthorId":"` + sourceAuthorID + `","destinationAuthorId":"` + destinationAuthor.ID + `"}]` + canonicalField + `
		}`
	}

	unrelatedCanonical := mustTest(
		t,
		server,
		newMemberMutationRequest(
			http.MethodPost,
			copyPath,
			copyBody("unrelated-canonical", "canonical_original", `,"canonicalOriginalUrl":"https://unrelated.example.test/post"`),
			login,
		),
	)
	if unrelatedCanonical.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected unrelated canonical to return 400, got %d: %s", unrelatedCanonical.StatusCode, readBody(t, unrelatedCanonical))
	}

	derivedCanonical := mustTest(
		t,
		server,
		newMemberMutationRequest(
			http.MethodPost,
			copyPath,
			copyBody("derived-canonical", "canonical_original", ""),
			login,
		),
	)
	if derivedCanonical.StatusCode != http.StatusCreated {
		t.Fatalf("expected server-derived canonical copy 201, got %d: %s", derivedCanonical.StatusCode, readBody(t, derivedCanonical))
	}
	var copied Envelope[store.AdminArticle]
	decodeJSONResponse(t, derivedCanonical, &copied)
	if copied.Data.CanonicalURL != "https://source-validation.example.test/blog/canonical-source" {
		t.Fatalf("expected source canonical to be derived, got %q", copied.Data.CanonicalURL)
	}

	createTestRevision(t, server, login, sourceProject.ID, sourceArticle.ID, `{
		"title":"Body with source asset",
		"bodyDocument":{
			"type":"doc",
			"content":[{
				"type":"image",
				"attrs":{"assetId":"asset_source_project"}
			}]
		},
		"html":"<figure data-asset-id=\"asset_source_project\"></figure>"
	}`)
	projectScopedReference := mustTest(
		t,
		server,
		newMemberMutationRequest(
			http.MethodPost,
			copyPath,
			copyBody("unsafe-reference", "material_adaptation", ""),
			login,
		),
	)
	if projectScopedReference.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected project-scoped body reference to return 400, got %d: %s", projectScopedReference.StatusCode, readBody(t, projectScopedReference))
	}

	var destinationArticleCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM content_items
		WHERE project_id = ?
	`, destinationProject.ID).Scan(&destinationArticleCount); err != nil {
		t.Fatal(err)
	}
	if destinationArticleCount != 1 {
		t.Fatalf("expected only the valid derived-canonical copy, got %d destination articles", destinationArticleCount)
	}
}

func TestCopyArticleToProjectEnforcesBothProjectPermissionsAndScope(t *testing.T) {
	server, db := newAdminTestServer(t)
	owner := seedAndLogin(t, server, db, "copy-scope-owner@example.test", "correct horse battery staple")
	copier := seedAndLogin(t, server, db, "copy-scope-user@example.test", "another correct horse battery staple")
	sourceProject := createTestProject(t, server, owner, `{"slug":"scope-source","name":"Scope Source"}`)
	otherSourceProject := createTestProject(t, server, owner, `{"slug":"other-source","name":"Other Source"}`)
	destinationProject := createTestProject(t, server, owner, `{"slug":"scope-destination","name":"Scope Destination"}`)
	sourceCategory := createTestCategory(t, server, owner, sourceProject.ID, `{"slug":"source","name":"Source"}`)
	otherCategory := createTestCategory(t, server, owner, otherSourceProject.ID, `{"slug":"other","name":"Other"}`)
	destinationCategory := createTestCategory(t, server, owner, destinationProject.ID, `{"slug":"destination","name":"Destination"}`)
	sourceArticle := createTestArticle(t, server, owner, sourceProject.ID, `{
		"title":"Scoped source",
		"slug":"scoped-source",
		"primaryCategoryId":"`+sourceCategory.ID+`"
	}`)
	otherArticle := createTestArticle(t, server, owner, otherSourceProject.ID, `{
		"title":"Other source",
		"slug":"other-source",
		"primaryCategoryId":"`+otherCategory.ID+`"
	}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, sourceProject.ID, copier.userID); err != nil {
		t.Fatal(err)
	}

	copyBody := func() string {
		return `{
			"destinationProjectId":"` + destinationProject.ID + `",
			"primaryCategoryId":"` + destinationCategory.ID + `",
			"slug":"scoped-copy",
			"canonicalDecision":"material_adaptation"
		}`
	}
	copyPath := "/api/v1/projects/" + sourceProject.ID + "/articles/" + sourceArticle.ID + "/copy-to-project"

	noDestinationAccess := mustTest(t, server, newMemberMutationRequest(http.MethodPost, copyPath, copyBody(), copier))
	if noDestinationAccess.StatusCode != http.StatusNotFound {
		t.Fatalf("expected missing destination membership to return 404, got %d: %s", noDestinationAccess.StatusCode, readBody(t, noDestinationAccess))
	}

	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'reviewer', 'active', CURRENT_TIMESTAMP)
	`, destinationProject.ID, copier.userID); err != nil {
		t.Fatal(err)
	}
	reviewerDestination := mustTest(t, server, newMemberMutationRequest(http.MethodPost, copyPath, copyBody(), copier))
	if reviewerDestination.StatusCode != http.StatusForbidden {
		t.Fatalf("expected destination reviewer to be denied with 403, got %d: %s", reviewerDestination.StatusCode, readBody(t, reviewerDestination))
	}

	if _, err := db.Exec(`
		UPDATE project_memberships SET role = 'writer' WHERE project_id = ? AND user_id = ?
	`, destinationProject.ID, copier.userID); err != nil {
		t.Fatal(err)
	}
	crossProjectArticle := mustTest(
		t,
		server,
		newMemberMutationRequest(
			http.MethodPost,
			"/api/v1/projects/"+sourceProject.ID+"/articles/"+otherArticle.ID+"/copy-to-project",
			copyBody(),
			copier,
		),
	)
	if crossProjectArticle.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-project article/revision lookup to return 404, got %d: %s", crossProjectArticle.StatusCode, readBody(t, crossProjectArticle))
	}

	if _, err := db.Exec(`
		UPDATE project_memberships SET status = 'removed' WHERE project_id = ? AND user_id = ?
	`, sourceProject.ID, copier.userID); err != nil {
		t.Fatal(err)
	}
	noSourceAccess := mustTest(t, server, newMemberMutationRequest(http.MethodPost, copyPath, copyBody(), copier))
	if noSourceAccess.StatusCode != http.StatusNotFound {
		t.Fatalf("expected missing source membership to return 404, got %d: %s", noSourceAccess.StatusCode, readBody(t, noSourceAccess))
	}
}

type adminLoginResult struct {
	cookies   []*http.Cookie
	csrfToken string
	userID    string
}

func newAdminTestServer(t *testing.T) (*Server, *sql.DB) {
	return newAdminTestServerWithMailer(t, nil)
}

func newAdminTestServerWithMailer(t *testing.T, sender mailer.Sender) (*Server, *sql.DB) {
	t.Helper()
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "admin.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	webhookEncryptionKey := []byte("0123456789abcdef0123456789abcdef")
	server := New(Options{
		Config: config.Config{
			Env:                  "development",
			DevAuth:              true,
			AdminPublicURL:       "http://admin.example.test",
			WebhookEncryptionKey: webhookEncryptionKey,
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Mailer: sender,
		Store:  store.New(db, store.WithWebhookEncryptionKey(webhookEncryptionKey)),
	})
	return server, db
}

type recordingMailer struct {
	mu       sync.Mutex
	messages []mailer.Message
}

func (m *recordingMailer) Send(_ context.Context, message mailer.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, message)
	return nil
}

func (m *recordingMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *recordingMailer) message(t *testing.T, index int) mailer.Message {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		if index >= 0 && index < len(m.messages) {
			message := m.messages[index]
			m.mu.Unlock()
			return message
		}
		m.mu.Unlock()
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("expected password-reset email %d, got %d messages", index, m.count())
	return mailer.Message{}
}

func requestPasswordReset(t *testing.T, server *Server, email string, expectedStatus int) string {
	t.Helper()
	body, err := json.Marshal(forgotPasswordRequest{Email: email})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := mustTest(t, server, request)
	if response.StatusCode != expectedStatus {
		t.Fatalf("expected password-reset request %d, got %d: %s", expectedStatus, response.StatusCode, readBody(t, response))
	}
	return readBody(t, response)
}

func completePasswordReset(t *testing.T, server *Server, token, password string, expectedStatus int) {
	t.Helper()
	body, err := json.Marshal(resetPasswordRequest{Token: token, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := mustTest(t, server, request)
	if response.StatusCode != expectedStatus {
		t.Fatalf("expected password-reset completion %d, got %d: %s", expectedStatus, response.StatusCode, readBody(t, response))
	}
	_ = response.Body.Close()
}

func resetTokenFromMessage(t *testing.T, message mailer.Message) string {
	t.Helper()
	for _, line := range strings.Split(message.Text, "\n") {
		if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
			continue
		}
		resetURL, err := url.Parse(strings.TrimSpace(line))
		if err != nil {
			t.Fatal(err)
		}
		if token := resetURL.Query().Get("token"); token != "" {
			return token
		}
	}
	t.Fatalf("reset email did not contain a tokenized URL: %#v", message)
	return ""
}

func createTestProject(t *testing.T, server *Server, login adminLoginResult, body string) store.AdminProject {
	t.Helper()
	var requestBody map[string]any
	if err := json.Unmarshal([]byte(body), &requestBody); err != nil {
		t.Fatal(err)
	}
	encodedBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(encodedBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(request, login.cookies)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create project 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.AdminProject]
	decodeJSONResponse(t, response, &payload)
	return payload.Data
}

func createTestInvitation(t *testing.T, server *Server, login adminLoginResult, projectID, body string) store.ProjectMemberInvitation {
	t.Helper()
	request := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/invitations", body, login)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create invitation 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.ProjectMemberInvitation]
	decodeJSONResponse(t, response, &payload)
	return payload.Data
}

func acceptTestInvitation(t *testing.T, server *Server, token, password string, expectedStatus int) store.ProjectInvitationAcceptance {
	t.Helper()
	body, err := json.Marshal(invitationAcceptanceRequest{Password: password})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/invitations/"+token+"/accept", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := mustTest(t, server, request)
	if response.StatusCode != expectedStatus {
		t.Fatalf("expected invitation acceptance %d, got %d: %s", expectedStatus, response.StatusCode, readBody(t, response))
	}
	if expectedStatus != http.StatusOK {
		return store.ProjectInvitationAcceptance{}
	}
	var payload Envelope[store.ProjectInvitationAcceptance]
	decodeJSONResponse(t, response, &payload)
	return payload.Data
}

func assertInvitationRevoked(t *testing.T, db *sql.DB, token string) {
	t.Helper()
	var revokedAt string
	if err := db.QueryRow(`
		SELECT COALESCE(revoked_at, '')
		FROM invitations
		WHERE token_hash = ?
	`, security.TokenHash(token)).Scan(&revokedAt); err != nil {
		t.Fatal(err)
	}
	if revokedAt == "" {
		t.Fatal("expected invitation to be revoked")
	}
}

func newMemberMutationRequest(method, path, body string, login adminLoginResult) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(request, login.cookies)
	return request
}

func assertRecentReauthenticationRequired(t *testing.T, response *http.Response) {
	t.Helper()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected recent reauthentication failure, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Problem
	decodeJSONResponse(t, response, &payload)
	if payload.Title != "Recent reauthentication required" {
		t.Fatalf("expected recent reauthentication problem, got %#v", payload)
	}
}

func createTestCategory(t *testing.T, server *Server, login adminLoginResult, projectID, body string) store.TaxonomyTerm {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/categories", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(request, login.cookies)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create category 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.TaxonomyTerm]
	decodeJSONResponse(t, response, &payload)
	return payload.Data
}

func createTestTag(t *testing.T, server *Server, login adminLoginResult, projectID, body string) store.TaxonomyTerm {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/tags", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(request, login.cookies)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create tag 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.TaxonomyTerm]
	decodeJSONResponse(t, response, &payload)
	return payload.Data
}

func assertArticleTagAssignments(t *testing.T, db *sql.DB, projectID, articleID string, expected []string) {
	t.Helper()
	rows, err := db.Query(`
		SELECT assignment.taxonomy_term_id
		FROM article_taxonomy assignment
		JOIN taxonomy_terms term
		  ON term.project_id = assignment.project_id
		 AND term.id = assignment.taxonomy_term_id
		WHERE assignment.project_id = ?
		  AND assignment.content_id = ?
		  AND assignment.is_primary = 0
		  AND term.type = 'tag'
		ORDER BY assignment.position, assignment.taxonomy_term_id
	`, projectID, articleID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var actual []string
	for rows.Next() {
		var tagID string
		if err := rows.Scan(&tagID); err != nil {
			t.Fatal(err)
		}
		actual = append(actual, tagID)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("expected article tag assignments %#v, got %#v", expected, actual)
	}
}

func assertArticleNonTagAssignments(t *testing.T, db *sql.DB, projectID, articleID string, expected []string) {
	t.Helper()
	rows, err := db.Query(`
		SELECT assignment.taxonomy_term_id
		FROM article_taxonomy assignment
		JOIN taxonomy_terms term
		  ON term.project_id = assignment.project_id
		 AND term.id = assignment.taxonomy_term_id
		WHERE assignment.project_id = ?
		  AND assignment.content_id = ?
		  AND assignment.is_primary = 0
		  AND term.type <> 'tag'
		ORDER BY assignment.position, assignment.taxonomy_term_id
	`, projectID, articleID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var actual []string
	for rows.Next() {
		var termID string
		if err := rows.Scan(&termID); err != nil {
			t.Fatal(err)
		}
		actual = append(actual, termID)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("expected non-tag assignments %#v, got %#v", expected, actual)
	}
}

func assertRevisionSnapshotTags(t *testing.T, db *sql.DB, projectID, articleID string, expected []string) {
	t.Helper()
	if expected == nil {
		expected = []string{}
	}
	var raw string
	if err := db.QueryRow(`
		SELECT taxonomy_snapshot_json
		FROM content_revisions
		WHERE project_id = ? AND content_id = ?
		ORDER BY revision_number DESC
		LIMIT 1
	`, projectID, articleID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var snapshot store.PublishedTaxonomy
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		t.Fatal(err)
	}
	actual := make([]string, 0, len(snapshot.Tags))
	for _, tag := range snapshot.Tags {
		actual = append(actual, tag.ID)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("expected revision snapshot tags %#v, got %#v in %s", expected, actual, raw)
	}
}

func createTestAuthor(t *testing.T, server *Server, login adminLoginResult, projectID, body string) store.Author {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/authors", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(request, login.cookies)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create author 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.Author]
	decodeJSONResponse(t, response, &payload)
	return payload.Data
}

func findTaxonomyTerm(terms []store.TaxonomyTerm, termID string) store.TaxonomyTerm {
	for _, term := range terms {
		if term.ID == termID {
			return term
		}
	}
	return store.TaxonomyTerm{}
}

func createTestSeries(t *testing.T, server *Server, login adminLoginResult, projectID, body string) store.Series {
	t.Helper()
	request := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/series", body, login)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create series 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.Series]
	decodeJSONResponse(t, response, &payload)
	return payload.Data
}

func findSeries(items []store.Series, seriesID string) store.Series {
	for _, item := range items {
		if item.ID == seriesID {
			return item
		}
	}
	return store.Series{}
}

func jsonContainsID(value any, id string) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		fields, ok := item.(map[string]any)
		if ok && fields["id"] == id {
			return true
		}
	}
	return false
}

func createTestArticle(t *testing.T, server *Server, login adminLoginResult, projectID, body string) store.AdminArticle {
	t.Helper()
	var requestBody map[string]any
	if err := json.Unmarshal([]byte(body), &requestBody); err != nil {
		t.Fatal(err)
	}
	if _, contributorInputProvided := requestBody["contributors"]; !contributorInputProvided {
		articleSlug, _ := requestBody["slug"].(string)
		authorBody, err := json.Marshal(map[string]string{
			"slug":        "test-author-" + articleSlug,
			"displayName": "Test Author for " + articleSlug,
		})
		if err != nil {
			t.Fatal(err)
		}
		author := createTestAuthor(t, server, login, projectID, string(authorBody))
		requestBody["contributors"] = []map[string]any{{
			"authorId": author.ID,
			"role":     "primary_author",
			"position": 0,
		}}
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatal(err)
		}
		body = string(encoded)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/articles", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(request, login.cookies)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create article 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.AdminArticle]
	decodeJSONResponse(t, response, &payload)
	if payload.Data.LatestRevision == nil {
		t.Fatal("expected created article to include latest revision")
	}
	return payload.Data
}

func createTestRevision(t *testing.T, server *Server, login adminLoginResult, projectID, articleID, body string) store.AdminRevision {
	t.Helper()
	var requestBody map[string]any
	if err := json.Unmarshal([]byte(body), &requestBody); err != nil {
		t.Fatal(err)
	}
	if _, provided := requestBody["baseRevisionId"]; !provided {
		articleRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/articles/"+articleID, nil)
		addCookies(articleRequest, login.cookies)
		articleResponse := mustTest(t, server, articleRequest)
		if articleResponse.StatusCode != http.StatusOK {
			t.Fatalf("expected article lookup 200 before revision create, got %d: %s", articleResponse.StatusCode, readBody(t, articleResponse))
		}
		var articlePayload Envelope[store.AdminArticle]
		decodeJSONResponse(t, articleResponse, &articlePayload)
		if articlePayload.Data.LatestRevision == nil {
			t.Fatal("expected base revision")
		}
		requestBody["baseRevisionId"] = articlePayload.Data.LatestRevision.ID
	}
	if _, hasHTML := requestBody["html"]; hasHTML {
		if _, hasDocument := requestBody["bodyDocument"]; !hasDocument {
			requestBody["bodyDocument"] = map[string]any{"type": "doc", "content": []any{}}
		}
	}
	encoded, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatal(err)
	}
	body = string(encoded)
	request := newMemberMutationRequest(http.MethodPut, "/api/v1/projects/"+projectID+"/articles/"+articleID, body, login)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected article save 200, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.AdminArticle]
	decodeJSONResponse(t, response, &payload)
	if payload.Data.LatestRevision == nil {
		t.Fatal("expected article save to return its latest internal revision")
	}
	return *payload.Data.LatestRevision
}

type testRevisionDetail struct {
	ContentHash         string
	AuthorSnapshot      any
	ContributorSnapshot any
	SourceSnapshot      any
	ClaimSnapshot       any
	SEOSnapshot         any
}

func getTestRevisionDetail(t *testing.T, db *sql.DB, projectID, articleID, revisionID string) testRevisionDetail {
	t.Helper()
	var detail testRevisionDetail
	var authorJSON, contributorJSON, sourceJSON, claimJSON, seoJSON string
	if err := db.QueryRow(`
		SELECT content_hash, author_snapshot_json, contributor_snapshot_json,
		       source_snapshot_json, claim_snapshot_json, seo_snapshot_json
		FROM content_revisions
		WHERE project_id = ? AND content_id = ? AND id = ?
	`, projectID, articleID, revisionID).Scan(
		&detail.ContentHash, &authorJSON, &contributorJSON, &sourceJSON, &claimJSON, &seoJSON,
	); err != nil {
		t.Fatalf("expected internal revision fixture: %v", err)
	}
	detail.AuthorSnapshot = decodeTestSnapshot(t, authorJSON)
	detail.ContributorSnapshot = decodeTestSnapshot(t, contributorJSON)
	detail.SourceSnapshot = decodeTestSnapshot(t, sourceJSON)
	detail.ClaimSnapshot = decodeTestSnapshot(t, claimJSON)
	detail.SEOSnapshot = decodeTestSnapshot(t, seoJSON)
	return detail
}

func decodeTestSnapshot(t *testing.T, raw string) any {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("decode internal snapshot: %v", err)
	}
	return value
}

func decodeAuthorSnapshot(t *testing.T, snapshot any) []store.Author {
	t.Helper()
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var authors []store.Author
	if err := json.Unmarshal(raw, &authors); err != nil {
		t.Fatal(err)
	}
	return authors
}

func decodeContributorSnapshot(t *testing.T, snapshot any) []store.Contributor {
	t.Helper()
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var contributors []store.Contributor
	if err := json.Unmarshal(raw, &contributors); err != nil {
		t.Fatal(err)
	}
	return contributors
}

func publishTestArticle(t *testing.T, server *Server, login adminLoginResult, projectID, articleID, slug string) store.AdminArticle {
	t.Helper()
	request := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectID+"/articles/"+articleID+"/publish",
		`{"slug":"`+slug+`"}`,
		login,
	)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected publish article 200, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.AdminArticle]
	decodeJSONResponse(t, response, &payload)
	return payload.Data
}

func createTestAPIKey(t *testing.T, server *Server, login adminLoginResult, projectID, body string) Envelope[store.APIKeyWithSecret] {
	t.Helper()
	request := newAPIKeyMutationRequest(t, http.MethodPost, "/api/v1/projects/"+projectID+"/api-keys", body, login)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create API key 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.APIKeyWithSecret]
	decodeJSONResponse(t, response, &payload)
	return payload
}

func newAPIKeyMutationRequest(t *testing.T, method, path, body string, login adminLoginResult) *http.Request {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(request, login.cookies)
	return request
}

func assertContentCategoriesStatus(t *testing.T, server *Server, secret string, expected int) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/content/v1/categories", nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	response := mustTest(t, server, request)
	if response.StatusCode != expected {
		t.Fatalf("expected content categories status %d, got %d: %s", expected, response.StatusCode, readBody(t, response))
	}
}

func seedOwner(t *testing.T, db *sql.DB, email, password string) string {
	t.Helper()
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	user, err := store.New(db).BootstrapOwner(context.Background(), email, passwordHash)
	if err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func seedAndLogin(t *testing.T, server *Server, db *sql.DB, email, password string) adminLoginResult {
	t.Helper()
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := security.RandomID("usr")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO users(id, email_normalized, password_hash, status, email_verified_at, password_changed_at)
		VALUES (?, ?, ?, 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID, strings.ToLower(email), passwordHash)
	if err != nil {
		t.Fatal(err)
	}
	return adminLogin(t, server, email, password)
}

func adminLogin(t *testing.T, server *Server, email, password string) adminLoginResult {
	t.Helper()
	requestBody, err := json.Marshal(loginRequest{Email: email, Password: password})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[authResponse]
	decodeJSONResponse(t, response, &payload)
	return adminLoginResult{
		cookies:   response.Cookies(),
		csrfToken: payload.Data.CSRFToken,
		userID:    payload.Data.User.ID,
	}
}

func mustTest(t *testing.T, server *Server, request *http.Request) *http.Response {
	t.Helper()
	response, err := server.app.Test(request, fiber.TestConfig{Timeout: 15 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func addCookies(request *http.Request, cookies []*http.Cookie) {
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
}

func decodeJSONResponse(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatal(err)
	}
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func findProjectMember(members []store.AdminProjectMember, userID string) store.AdminProjectMember {
	for _, member := range members {
		if member.UserID == userID {
			return member
		}
	}
	return store.AdminProjectMember{}
}
