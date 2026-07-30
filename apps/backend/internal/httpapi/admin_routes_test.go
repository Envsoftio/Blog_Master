package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
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

	body := strings.NewReader(`{"slug":"Demo Project","name":"Demo Project","primaryDomain":"example.test","supportedLocales":["en","fr"]}`)
	createWithoutCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/projects", body)
	createWithoutCSRF.Header.Set("Content-Type", "application/json")
	addCookies(createWithoutCSRF, login.cookies)
	missingCSRFResponse := mustTest(t, server, createWithoutCSRF)
	if missingCSRFResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected create without CSRF to fail with 403, got %d", missingCSRFResponse.StatusCode)
	}

	body = strings.NewReader(`{"slug":"Demo Project","name":"Demo Project","primaryDomain":"example.test","supportedLocales":["en","fr"]}`)
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
	if otherProjectAfterRemovalResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected removal from one project to preserve access to another, got %d: %s", otherProjectAfterRemovalResponse.StatusCode, readBody(t, otherProjectAfterRemovalResponse))
	}
}

func TestMembershipAuthorizationAndCrossProjectScoping(t *testing.T) {
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
	if crossProjectResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-project member mutation to return 404, got %d: %s", crossProjectResponse.StatusCode, readBody(t, crossProjectResponse))
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
			response, err := server.app.Test(request, 15_000)
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

func TestSourcesClaimsAndApprovalGate(t *testing.T) {
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

	crossProjectClaim := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/revisions/"+revisionID+"/claims",
		`{"claimText":"Cross-project source should fail.","importance":"material","sourceIds":["`+projectBSourcePayload.Data.ID+`"]}`,
		ownerLogin,
	)
	crossProjectClaimResponse := mustTest(t, server, crossProjectClaim)
	if crossProjectClaimResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected cross-project source claim to fail with 400, got %d: %s", crossProjectClaimResponse.StatusCode, readBody(t, crossProjectClaimResponse))
	}

	claimRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/revisions/"+revisionID+"/claims",
		`{"claimText":"The benchmark improved conversion by 12%.","importance":"material","sourceIds":["`+source.ID+`"]}`,
		ownerLogin,
	)
	claimResponse := mustTest(t, server, claimRequest)
	if claimResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected claim create 201, got %d: %s", claimResponse.StatusCode, readBody(t, claimResponse))
	}
	var claimPayload Envelope[store.Claim]
	decodeJSONResponse(t, claimResponse, &claimPayload)
	claim := claimPayload.Data
	if claim.VerificationState != "unverified" || len(claim.SourceIDs) != 1 || claim.SourceIDs[0] != source.ID {
		t.Fatalf("unexpected claim payload %#v", claim)
	}

	listClaimRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/revisions/"+revisionID+"/claims", nil)
	addCookies(listClaimRequest, ownerLogin.cookies)
	listClaimResponse := mustTest(t, server, listClaimRequest)
	if listClaimResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected claim list 200, got %d: %s", listClaimResponse.StatusCode, readBody(t, listClaimResponse))
	}
	var listedClaims ListEnvelope[store.Claim]
	decodeJSONResponse(t, listClaimResponse, &listedClaims)
	if len(listedClaims.Data) != 1 || listedClaims.Data[0].ID != claim.ID {
		t.Fatalf("expected created claim in list, got %#v", listedClaims.Data)
	}

	unverifiedApproval := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/revisions/"+revisionID+"/approve", `{}`, ownerLogin)
	unverifiedApprovalResponse := mustTest(t, server, unverifiedApproval)
	if unverifiedApprovalResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected unverified material claim to block approval, got %d: %s", unverifiedApprovalResponse.StatusCode, readBody(t, unverifiedApprovalResponse))
	}

	writerVerify := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/claims/"+claim.ID+"/verify",
		`{"verificationState":"supported"}`,
		writerLogin,
	)
	writerVerifyResponse := mustTest(t, server, writerVerify)
	if writerVerifyResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer claim verification denial, got %d: %s", writerVerifyResponse.StatusCode, readBody(t, writerVerifyResponse))
	}

	verifyRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/claims/"+claim.ID+"/verify",
		`{"verificationState":"supported"}`,
		ownerLogin,
	)
	verifyResponse := mustTest(t, server, verifyRequest)
	if verifyResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected claim verify 200, got %d: %s", verifyResponse.StatusCode, readBody(t, verifyResponse))
	}
	var verifiedPayload Envelope[store.Claim]
	decodeJSONResponse(t, verifyResponse, &verifiedPayload)
	if verifiedPayload.Data.VerificationState != "supported" || verifiedPayload.Data.VerifiedBy != ownerLogin.userID {
		t.Fatalf("unexpected verified claim %#v", verifiedPayload.Data)
	}

	approved := approveTestRevision(t, server, ownerLogin, project.ID, revisionID)
	if approved.EditorialState != "approved" {
		t.Fatalf("expected approved revision, got %#v", approved)
	}
	secondApproval := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/revisions/"+revisionID+"/approve", `{}`, ownerLogin)
	secondApprovalResponse := mustTest(t, server, secondApproval)
	if secondApprovalResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected repeated approval to fail with 409, got %d: %s", secondApprovalResponse.StatusCode, readBody(t, secondApprovalResponse))
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/revisions/"+revisionID, nil)
	addCookies(detailRequest, ownerLogin.cookies)
	detailResponse := mustTest(t, server, detailRequest)
	if detailResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected revision detail 200, got %d: %s", detailResponse.StatusCode, readBody(t, detailResponse))
	}
	var detail Envelope[store.AdminRevisionDetail]
	decodeJSONResponse(t, detailResponse, &detail)
	if detail.Data.ContentHash != approved.ContentHash {
		t.Fatalf("expected repeated approval denial to preserve content hash %q, got %q", approved.ContentHash, detail.Data.ContentHash)
	}
	if !jsonContainsID(detail.Data.SourceSnapshot, source.ID) {
		t.Fatalf("expected source snapshot to include %q, got %#v", source.ID, detail.Data.SourceSnapshot)
	}
	if !jsonContainsID(detail.Data.ClaimSnapshot, claim.ID) {
		t.Fatalf("expected claim snapshot to include %q, got %#v", claim.ID, detail.Data.ClaimSnapshot)
	}

	lateClaim := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/revisions/"+revisionID+"/claims",
		`{"claimText":"Late change","importance":"normal"}`,
		ownerLogin,
	)
	lateClaimResponse := mustTest(t, server, lateClaim)
	if lateClaimResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected approved revision claim mutation to fail with 409, got %d: %s", lateClaimResponse.StatusCode, readBody(t, lateClaimResponse))
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
	approveTestRevision(t, server, ownerLogin, project.ID, revisionID)
	publishTestArticle(t, server, ownerLogin, project.ID, article.ID, revisionID, "trust-update")

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

func TestPreviewTokensExposeDraftRevisionOnly(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "owner correct password")
	project := createTestProject(t, server, login, `{"slug":"preview","name":"Preview Project"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"drafts","name":"Drafts"}`)
	article := createTestArticle(
		t,
		server,
		login,
		project.ID,
		`{"articleType":"standard","title":"Draft Preview","slug":"draft-preview","primaryCategoryId":"`+category.ID+`","html":"<p>Draft only.</p>"}`,
	)
	revisionID := article.LatestRevision.ID

	publicDraftRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/draft-preview?locale=en", nil)
	publicDraftRequest.Header.Set("X-Dev-Project-ID", project.ID)
	publicDraftResponse := mustTest(t, server, publicDraftRequest)
	if publicDraftResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected unpublished draft to be hidden from published content API, got %d: %s", publicDraftResponse.StatusCode, readBody(t, publicDraftResponse))
	}

	badTTL := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/preview-tokens",
		`{"articleId":"`+article.ID+`","revisionId":"`+revisionID+`","ttlMinutes":14}`,
		login,
	)
	badTTLResponse := mustTest(t, server, badTTL)
	if badTTLResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected too-short preview token ttl to fail with 400, got %d: %s", badTTLResponse.StatusCode, readBody(t, badTTLResponse))
	}

	createRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/preview-tokens",
		`{"articleId":"`+article.ID+`","revisionId":"`+revisionID+`","ttlMinutes":15}`,
		login,
	)
	createResponse := mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected preview token create 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.PreviewTokenWithSecret]
	decodeJSONResponse(t, createResponse, &created)
	if created.Data.Token.ID == "" || created.Data.Token.RevisionID != revisionID || !strings.HasPrefix(created.Data.Secret, "sbprev_") {
		t.Fatalf("unexpected preview token response %#v", created.Data)
	}

	var storedHashMatches int
	if err := db.QueryRow(`SELECT COUNT(1) FROM preview_tokens WHERE token_hash = ?`, security.TokenHash(created.Data.Secret)).Scan(&storedHashMatches); err != nil {
		t.Fatal(err)
	}
	if storedHashMatches != 1 {
		t.Fatalf("expected preview token hash to be stored once, got %d", storedHashMatches)
	}
	var rawSecretMatches int
	if err := db.QueryRow(`SELECT COUNT(1) FROM preview_tokens WHERE token_hash = ?`, created.Data.Secret).Scan(&rawSecretMatches); err != nil {
		t.Fatal(err)
	}
	if rawSecretMatches != 0 {
		t.Fatal("expected raw preview secret not to be stored")
	}
	var auditSecretMatches int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ? AND metadata_json LIKE '%' || ? || '%'
	`, project.ID, created.Data.Secret).Scan(&auditSecretMatches); err != nil {
		t.Fatal(err)
	}
	if auditSecretMatches != 0 {
		t.Fatal("expected raw preview secret not to appear in audit metadata")
	}

	previewTokenAsContentKey := httptest.NewRequest(http.MethodGet, "/content/v1/posts/draft-preview?locale=en", nil)
	previewTokenAsContentKey.Header.Set("Authorization", "Bearer "+created.Data.Secret)
	previewTokenAsContentKeyResponse := mustTest(t, server, previewTokenAsContentKey)
	if previewTokenAsContentKeyResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected preview token not to work as content API key, got %d: %s", previewTokenAsContentKeyResponse.StatusCode, readBody(t, previewTokenAsContentKeyResponse))
	}

	previewRequest := httptest.NewRequest(http.MethodGet, "/content/v1/preview/revisions/"+revisionID, nil)
	previewRequest.Header.Set("Authorization", "Bearer "+created.Data.Secret)
	previewResponse := mustTest(t, server, previewRequest)
	if previewResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected preview revision 200, got %d: %s", previewResponse.StatusCode, readBody(t, previewResponse))
	}
	if cacheControl := previewResponse.Header.Get("Cache-Control"); cacheControl != "private, no-store" {
		t.Fatalf("expected no-store preview cache control, got %q", cacheControl)
	}
	if robots := previewResponse.Header.Get("X-Robots-Tag"); robots != "noindex, nofollow" {
		t.Fatalf("expected preview robots header, got %q", robots)
	}
	var preview Envelope[store.PublishedPost]
	decodeJSONResponse(t, previewResponse, &preview)
	if preview.Data.Title != "Draft Preview" || !strings.Contains(preview.Data.Content.HTML, "Draft only.") {
		t.Fatalf("unexpected preview payload %#v", preview.Data)
	}
	if preview.Data.SEO.Index || preview.Data.SEO.Robots != "noindex,nofollow" {
		t.Fatalf("expected preview SEO to be noindex, got %#v", preview.Data.SEO)
	}

	wrongRevisionRequest := httptest.NewRequest(http.MethodGet, "/content/v1/preview/revisions/not-this-revision", nil)
	wrongRevisionRequest.Header.Set("Authorization", "Bearer "+created.Data.Secret)
	wrongRevisionResponse := mustTest(t, server, wrongRevisionRequest)
	if wrongRevisionResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected preview token to be bound to one revision, got %d: %s", wrongRevisionResponse.StatusCode, readBody(t, wrongRevisionResponse))
	}

	if _, err := db.Exec(`UPDATE preview_tokens SET expires_at = datetime(CURRENT_TIMESTAMP, '-1 minute') WHERE id = ?`, created.Data.Token.ID); err != nil {
		t.Fatal(err)
	}
	expiredRequest := httptest.NewRequest(http.MethodGet, "/content/v1/preview/revisions/"+revisionID, nil)
	expiredRequest.Header.Set("Authorization", "Bearer "+created.Data.Secret)
	expiredResponse := mustTest(t, server, expiredRequest)
	if expiredResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected expired preview token to fail with 401, got %d: %s", expiredResponse.StatusCode, readBody(t, expiredResponse))
	}

	revokeCreate := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/preview-tokens",
		`{"articleId":"`+article.ID+`","revisionId":"`+revisionID+`"}`,
		login,
	)
	revokeCreateResponse := mustTest(t, server, revokeCreate)
	if revokeCreateResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected second preview token create 201, got %d: %s", revokeCreateResponse.StatusCode, readBody(t, revokeCreateResponse))
	}
	var revocable Envelope[store.PreviewTokenWithSecret]
	decodeJSONResponse(t, revokeCreateResponse, &revocable)

	revokeRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/preview-tokens/"+revocable.Data.Token.ID+"/revoke",
		`{}`,
		login,
	)
	revokeResponse := mustTest(t, server, revokeRequest)
	if revokeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected preview token revoke 200, got %d: %s", revokeResponse.StatusCode, readBody(t, revokeResponse))
	}
	revokedRequest := httptest.NewRequest(http.MethodGet, "/content/v1/preview/revisions/"+revisionID, nil)
	revokedRequest.Header.Set("Authorization", "Bearer "+revocable.Data.Secret)
	revokedResponse := mustTest(t, server, revokedRequest)
	if revokedResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked preview token to fail with 401, got %d: %s", revokedResponse.StatusCode, readBody(t, revokedResponse))
	}
}

func TestAuthorManagementAuthorizationAndCrossProjectScoping(t *testing.T) {
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
	if crossProjectLoginResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected cross-project login link to fail with 400, got %d: %s", crossProjectLoginResponse.StatusCode, readBody(t, crossProjectLoginResponse))
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
	if crossProjectPhotoResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected cross-project photo asset to fail with 400, got %d: %s", crossProjectPhotoResponse.StatusCode, readBody(t, crossProjectPhotoResponse))
	}

	crossProjectPatch := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+projectB.ID+"/authors/"+authorB.ID,
		`{"displayName":"Leaked"}`,
		ownerALogin,
	)
	crossProjectPatchResponse := mustTest(t, server, crossProjectPatch)
	if crossProjectPatchResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-project author update to return 404, got %d: %s", crossProjectPatchResponse.StatusCode, readBody(t, crossProjectPatchResponse))
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
	revisionID := article.LatestRevision.ID

	scheduleBeforeApproval := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/schedule",
		strings.NewReader(`{"revisionId":"`+revisionID+`","slug":"scheduled-post","scheduledForUtc":"`+time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)+`"}`),
	)
	scheduleBeforeApproval.Header.Set("Content-Type", "application/json")
	scheduleBeforeApproval.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(scheduleBeforeApproval, login.cookies)
	scheduleBeforeApprovalResponse := mustTest(t, server, scheduleBeforeApproval)
	if scheduleBeforeApprovalResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected scheduling an unapproved revision to fail with 409, got %d", scheduleBeforeApprovalResponse.StatusCode)
	}

	approveRequest := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/revisions/"+revisionID+"/approve", strings.NewReader(`{}`))
	approveRequest.Header.Set("Content-Type", "application/json")
	approveRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(approveRequest, login.cookies)
	approveResponse := mustTest(t, server, approveRequest)
	if approveResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected approve revision 200, got %d: %s", approveResponse.StatusCode, readBody(t, approveResponse))
	}

	scheduleRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/schedule",
		strings.NewReader(`{"revisionId":"`+revisionID+`","slug":"scheduled-post","scheduledForUtc":"`+time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)+`"}`),
	)
	scheduleRequest.Header.Set("Content-Type", "application/json")
	scheduleRequest.Header.Set("X-CSRF-Token", login.csrfToken)
	addCookies(scheduleRequest, login.cookies)
	scheduleResponse := mustTest(t, server, scheduleRequest)
	if scheduleResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected schedule article 200, got %d: %s", scheduleResponse.StatusCode, readBody(t, scheduleResponse))
	}

	beforePublishRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/scheduled-post?locale=en", nil)
	beforePublishRequest.Header.Set("X-Dev-Project-ID", project.ID)
	beforePublishResponse := mustTest(t, server, beforePublishRequest)
	if beforePublishResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected scheduled article to remain hidden before worker publish, got %d", beforePublishResponse.StatusCode)
	}

	published, err := store.New(db).PublishDueSchedules(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	if published != 1 {
		t.Fatalf("expected worker to publish one scheduled article, got %d", published)
	}

	publishedRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/scheduled-post?locale=en", nil)
	publishedRequest.Header.Set("X-Dev-Project-ID", project.ID)
	publishedResponse := mustTest(t, server, publishedRequest)
	if publishedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected published article to be readable, got %d: %s", publishedResponse.StatusCode, readBody(t, publishedResponse))
	}
	var payload Envelope[store.PublishedPost]
	decodeJSONResponse(t, publishedResponse, &payload)
	if payload.Data.Title != "Scheduled Post" {
		t.Fatalf("expected published title, got %q", payload.Data.Title)
	}
	if payload.Data.SEO.CanonicalURL != "https://example.test/blog/scheduled-post" {
		t.Fatalf("unexpected canonical URL %q", payload.Data.SEO.CanonicalURL)
	}
}

func TestArticleRollbackRestoresApprovedRevision(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")

	project := createTestProject(t, server, login, `{"slug":"rollback","name":"Rollback Project","primaryDomain":"example.test"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, login, project.ID, `{
		"articleType":"guide",
		"title":"Original Guide",
		"slug":"rollback-guide",
		"primaryCategoryId":"`+category.ID+`",
		"excerpt":"Original excerpt",
		"html":"<p>Original body</p>"
	}`)
	firstRevisionID := article.LatestRevision.ID
	approveTestRevision(t, server, login, project.ID, firstRevisionID)
	publishTestArticle(t, server, login, project.ID, article.ID, firstRevisionID, "rollback-guide")

	secondRevision := createTestRevision(t, server, login, project.ID, article.ID, `{
		"title":"Updated Guide",
		"excerpt":"Updated excerpt",
		"html":"<p>Updated body</p>"
	}`)
	approveTestRevision(t, server, login, project.ID, secondRevision.ID)
	publishTestArticle(t, server, login, project.ID, article.ID, secondRevision.ID, "rollback-guide")

	draftRevision := createTestRevision(t, server, login, project.ID, article.ID, `{
		"title":"Unapproved Draft",
		"html":"<p>Draft body</p>"
	}`)
	rejectDraftRollback := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/rollback",
		`{"revisionId":"`+draftRevision.ID+`"}`,
		login,
	)
	rejectDraftRollbackResponse := mustTest(t, server, rejectDraftRollback)
	if rejectDraftRollbackResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected draft rollback to fail with 409, got %d: %s", rejectDraftRollbackResponse.StatusCode, readBody(t, rejectDraftRollbackResponse))
	}

	updatedRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/rollback-guide?locale=en", nil)
	updatedRequest.Header.Set("X-Dev-Project-ID", project.ID)
	updatedResponse := mustTest(t, server, updatedRequest)
	if updatedResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected updated article to be published, got %d: %s", updatedResponse.StatusCode, readBody(t, updatedResponse))
	}
	var updated Envelope[store.PublishedPost]
	decodeJSONResponse(t, updatedResponse, &updated)
	if updated.Data.Title != "Updated Guide" {
		t.Fatalf("expected updated title before rollback, got %q", updated.Data.Title)
	}

	var versionBeforeRollback int64
	if err := db.QueryRow(`
		SELECT publication_version
		FROM project_publications
		WHERE project_id = ? AND content_id = ?
	`, project.ID, article.ID).Scan(&versionBeforeRollback); err != nil {
		t.Fatal(err)
	}

	rejectRoutingChangeRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/rollback",
		`{"revisionId":"`+firstRevisionID+`","slug":"moved-guide","canonicalUrl":"https://example.test/blog/moved-guide"}`,
		login,
	)
	rejectRoutingChangeResponse := mustTest(t, server, rejectRoutingChangeRequest)
	if rejectRoutingChangeResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected rollback routing metadata to fail with 400, got %d: %s", rejectRoutingChangeResponse.StatusCode, readBody(t, rejectRoutingChangeResponse))
	}

	rollbackRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/rollback",
		`{"revisionId":"`+firstRevisionID+`"}`,
		login,
	)
	rollbackResponse := mustTest(t, server, rollbackRequest)
	if rollbackResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected rollback 200, got %d: %s", rollbackResponse.StatusCode, readBody(t, rollbackResponse))
	}
	var rollbackPayload Envelope[store.AdminArticle]
	decodeJSONResponse(t, rollbackResponse, &rollbackPayload)
	if rollbackPayload.Data.LatestRevision == nil || rollbackPayload.Data.LatestRevision.ID != draftRevision.ID {
		t.Fatalf("expected rollback to preserve newer draft history, got %#v", rollbackPayload.Data.LatestRevision)
	}

	restoredRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/rollback-guide?locale=en", nil)
	restoredRequest.Header.Set("X-Dev-Project-ID", project.ID)
	restoredResponse := mustTest(t, server, restoredRequest)
	if restoredResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected restored article to be readable, got %d: %s", restoredResponse.StatusCode, readBody(t, restoredResponse))
	}
	var restored Envelope[store.PublishedPost]
	decodeJSONResponse(t, restoredResponse, &restored)
	if restored.Data.Title != "Original Guide" || restored.Data.Revision != 1 {
		t.Fatalf("expected original revision after rollback, got title=%q revision=%d", restored.Data.Title, restored.Data.Revision)
	}

	var versionAfterRollback int64
	if err := db.QueryRow(`
		SELECT publication_version
		FROM project_publications
		WHERE project_id = ? AND content_id = ?
	`, project.ID, article.ID).Scan(&versionAfterRollback); err != nil {
		t.Fatal(err)
	}
	if versionAfterRollback <= versionBeforeRollback {
		t.Fatalf("expected rollback to advance publication version from %d, got %d", versionBeforeRollback, versionAfterRollback)
	}

	var restoredEvents int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM outbox_events
		WHERE project_id = ?
		  AND aggregate_id = ?
		  AND event_type = 'content.restored'
	`, project.ID, article.ID).Scan(&restoredEvents); err != nil {
		t.Fatal(err)
	}
	if restoredEvents != 1 {
		t.Fatalf("expected one content.restored outbox event, got %d", restoredEvents)
	}

	var rollbackAudits int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND target_id = (
		    SELECT id FROM project_publications WHERE project_id = ? AND content_id = ?
		  )
		  AND action = 'article.rollback'
	`, project.ID, project.ID, article.ID).Scan(&rollbackAudits); err != nil {
		t.Fatal(err)
	}
	if rollbackAudits != 1 {
		t.Fatalf("expected one article.rollback audit event, got %d", rollbackAudits)
	}
}

func TestArticleRevisionHistoryAndDetailAreProjectScoped(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	otherLogin := seedAndLogin(t, server, db, "other@example.test", "another correct horse battery staple")

	project := createTestProject(t, server, login, `{"slug":"revision-history","name":"Revision History","primaryDomain":"example.test"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, login, project.ID, `{
		"articleType":"guide",
		"title":"Original Guide",
		"slug":"revision-history",
		"primaryCategoryId":"`+category.ID+`",
		"deck":"Original deck",
		"bodyDocument":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Original body"}]}]},
		"html":"<p>Original body</p>"
	}`)
	firstRevisionID := article.LatestRevision.ID
	approveTestRevision(t, server, login, project.ID, firstRevisionID)
	publishTestArticle(t, server, login, project.ID, article.ID, firstRevisionID, "revision-history")

	secondRevision := createTestRevision(t, server, login, project.ID, article.ID, `{
		"title":"Updated Guide",
		"deck":"Updated deck",
		"excerpt":"Updated excerpt",
		"bodyDocument":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Updated body"}]}]},
		"html":"<p>Updated body</p>"
	}`)

	historyRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/revisions?limit=1",
		nil,
	)
	addCookies(historyRequest, login.cookies)
	historyResponse := mustTest(t, server, historyRequest)
	if historyResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected revision history 200, got %d: %s", historyResponse.StatusCode, readBody(t, historyResponse))
	}
	var history ListEnvelope[store.AdminRevisionSummary]
	decodeJSONResponse(t, historyResponse, &history)
	if len(history.Data) != 1 || history.Data[0].ID != secondRevision.ID {
		t.Fatalf("expected latest revision first, got %#v", history.Data)
	}
	if history.Data[0].BaseRevisionID != firstRevisionID {
		t.Fatalf("expected base revision %q, got %q", firstRevisionID, history.Data[0].BaseRevisionID)
	}
	if history.Data[0].PublishedLocales == nil || len(history.Data[0].PublishedLocales) != 0 {
		t.Fatalf("expected a non-nil empty locale list for the draft revision, got %#v", history.Data[0].PublishedLocales)
	}
	if history.Meta.NextCursor == "" {
		t.Fatal("expected paginated revision history cursor")
	}

	nextRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/revisions?limit=1&cursor="+history.Meta.NextCursor,
		nil,
	)
	addCookies(nextRequest, login.cookies)
	nextResponse := mustTest(t, server, nextRequest)
	if nextResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected next revision page 200, got %d: %s", nextResponse.StatusCode, readBody(t, nextResponse))
	}
	var nextPage ListEnvelope[store.AdminRevisionSummary]
	decodeJSONResponse(t, nextResponse, &nextPage)
	if len(nextPage.Data) != 1 || nextPage.Data[0].ID != firstRevisionID {
		t.Fatalf("expected original revision on second page, got %#v", nextPage.Data)
	}
	if nextPage.Data[0].BaseRevisionID != "" {
		t.Fatalf("expected the first revision to have no base, got %q", nextPage.Data[0].BaseRevisionID)
	}
	if !reflect.DeepEqual(nextPage.Data[0].PublishedLocales, []string{"en"}) {
		t.Fatalf("expected current publication locale, got %#v", nextPage.Data[0].PublishedLocales)
	}

	detailRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/revisions/"+secondRevision.ID,
		nil,
	)
	addCookies(detailRequest, login.cookies)
	detailResponse := mustTest(t, server, detailRequest)
	if detailResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected revision detail 200, got %d: %s", detailResponse.StatusCode, readBody(t, detailResponse))
	}
	var detail Envelope[store.AdminRevisionDetail]
	decodeJSONResponse(t, detailResponse, &detail)
	if detail.Data.Title != "Updated Guide" || detail.Data.PlainText != "Updated body" {
		t.Fatalf("unexpected revision detail %#v", detail.Data)
	}
	document, ok := detail.Data.BodyDocument.(map[string]any)
	if !ok || document["type"] != "doc" {
		t.Fatalf("expected structured body document, got %#v", detail.Data.BodyDocument)
	}

	malformedCursorRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/revisions?cursor=not-base64",
		nil,
	)
	addCookies(malformedCursorRequest, login.cookies)
	malformedCursorResponse := mustTest(t, server, malformedCursorRequest)
	if malformedCursorResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected malformed cursor 400, got %d", malformedCursorResponse.StatusCode)
	}

	unknownCursorRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/revisions?cursor="+encodeCursor(idCursor{ID: "unknown-revision"}),
		nil,
	)
	addCookies(unknownCursorRequest, login.cookies)
	unknownCursorResponse := mustTest(t, server, unknownCursorRequest)
	if unknownCursorResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected unknown cursor 400, got %d", unknownCursorResponse.StatusCode)
	}

	unauthenticatedRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/revisions",
		nil,
	)
	unauthenticatedResponse := mustTest(t, server, unauthenticatedRequest)
	if unauthenticatedResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated history lookup 401, got %d", unauthenticatedResponse.StatusCode)
	}
	if contentType := unauthenticatedResponse.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/problem+json") {
		t.Fatalf("expected problem JSON content type, got %q", contentType)
	}

	otherHistoryRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/revisions",
		nil,
	)
	addCookies(otherHistoryRequest, otherLogin.cookies)
	otherHistoryResponse := mustTest(t, server, otherHistoryRequest)
	if otherHistoryResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected non-member history lookup to return 404, got %d", otherHistoryResponse.StatusCode)
	}

	crossArticleRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles/not-the-article/revisions/"+secondRevision.ID,
		nil,
	)
	addCookies(crossArticleRequest, login.cookies)
	crossArticleResponse := mustTest(t, server, crossArticleRequest)
	if crossArticleResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected mismatched article detail lookup to return 404, got %d", crossArticleResponse.StatusCode)
	}

	secondProject := createTestProject(t, server, login, `{"slug":"revision-history-two","name":"Revision History Two"}`)
	secondCategory := createTestCategory(t, server, login, secondProject.ID, `{"slug":"guides-two","name":"Guides Two"}`)
	secondArticle := createTestArticle(t, server, login, secondProject.ID, `{
		"title":"Second Project Article",
		"slug":"second-project-article",
		"primaryCategoryId":"`+secondCategory.ID+`",
		"html":"<p>Second project body</p>"
	}`)
	crossProjectHistoryRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+secondProject.ID+"/articles/"+article.ID+"/revisions",
		nil,
	)
	addCookies(crossProjectHistoryRequest, login.cookies)
	crossProjectHistoryResponse := mustTest(t, server, crossProjectHistoryRequest)
	if crossProjectHistoryResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-project article history lookup 404, got %d", crossProjectHistoryResponse.StatusCode)
	}
	crossProjectDetailRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+secondProject.ID+"/articles/"+secondArticle.ID+"/revisions/"+secondRevision.ID,
		nil,
	)
	addCookies(crossProjectDetailRequest, login.cookies)
	crossProjectDetailResponse := mustTest(t, server, crossProjectDetailRequest)
	if crossProjectDetailResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-project revision detail lookup 404, got %d", crossProjectDetailResponse.StatusCode)
	}

	if _, err := db.Exec(`UPDATE content_items SET archived_at = CURRENT_TIMESTAMP WHERE project_id = ? AND id = ?`, project.ID, article.ID); err != nil {
		t.Fatal(err)
	}
	archivedHistoryRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/revisions",
		nil,
	)
	addCookies(archivedHistoryRequest, login.cookies)
	archivedHistoryResponse := mustTest(t, server, archivedHistoryRequest)
	if archivedHistoryResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected archived article history lookup 404, got %d", archivedHistoryResponse.StatusCode)
	}
}

func TestCreateRevisionRejectsMissingAndStaleBase(t *testing.T) {
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

	missingBaseRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/revisions",
		`{"title":"Missing base","html":"<p>Missing base</p>"}`,
		login,
	)
	missingBaseResponse := mustTest(t, server, missingBaseRequest)
	if missingBaseResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected missing base 400, got %d: %s", missingBaseResponse.StatusCode, readBody(t, missingBaseResponse))
	}

	secondRevision := createTestRevision(t, server, login, project.ID, article.ID, `{
		"baseRevisionId":"`+firstRevisionID+`",
		"title":"Revision Two",
		"html":"<p>Revision two</p>"
	}`)
	staleBaseRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/revisions",
		`{"baseRevisionId":"`+firstRevisionID+`","title":"Stale Revision Three","html":"<p>Stale</p>"}`,
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
	historyRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/revisions?limit=3",
		nil,
	)
	addCookies(historyRequest, login.cookies)
	historyResponse := mustTest(t, server, historyRequest)
	if historyResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected history 200, got %d: %s", historyResponse.StatusCode, readBody(t, historyResponse))
	}
	var history ListEnvelope[store.AdminRevisionSummary]
	decodeJSONResponse(t, historyResponse, &history)
	if len(history.Data) != 3 {
		t.Fatalf("expected three revisions after rejecting stale write, got %d", len(history.Data))
	}
	if history.Data[0].ID != thirdRevision.ID || history.Data[0].BaseRevisionID != secondRevision.ID {
		t.Fatalf("unexpected third revision lineage %#v", history.Data[0])
	}
	if history.Data[1].ID != secondRevision.ID || history.Data[1].BaseRevisionID != firstRevisionID {
		t.Fatalf("unexpected second revision lineage %#v", history.Data[1])
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
	approveTestRevision(t, server, login, project.ID, article.LatestRevision.ID)
	publishTestArticle(t, server, login, project.ID, article.ID, article.LatestRevision.ID, "first-publication")

	secondRevision := createTestRevision(t, server, login, project.ID, article.ID, `{
		"title":"Updated publication",
		"html":"<p>Updated publication</p>"
	}`)
	approveTestRevision(t, server, login, project.ID, secondRevision.ID)
	publishTestArticle(t, server, login, project.ID, article.ID, secondRevision.ID, "first-publication")
	publishTestArticle(t, server, login, project.ID, article.ID, secondRevision.ID, "renamed-publication")

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

	publishTestArticle(t, server, login, project.ID, article.ID, secondRevision.ID, "first-publication")
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

func TestArticleRollbackTargetsPublicationLocale(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")

	project := createTestProject(t, server, login, `{
		"slug":"localized-rollback",
		"name":"Localized Rollback",
		"primaryDomain":"example.test",
		"supportedLocales":["en","fr"]
	}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, login, project.ID, `{
		"articleType":"guide",
		"title":"Original Guide",
		"slug":"english-guide",
		"locale":"en",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>Original body</p>"
	}`)
	firstRevisionID := article.LatestRevision.ID
	approveTestRevision(t, server, login, project.ID, firstRevisionID)
	publishTestArticleForLocale(t, server, login, project.ID, article.ID, firstRevisionID, "english-guide", "en")
	publishTestArticleForLocale(t, server, login, project.ID, article.ID, firstRevisionID, "guide-francais", "fr")

	secondRevision := createTestRevision(t, server, login, project.ID, article.ID, `{
		"title":"Updated Guide",
		"html":"<p>Updated body</p>"
	}`)
	approveTestRevision(t, server, login, project.ID, secondRevision.ID)
	publishTestArticleForLocale(t, server, login, project.ID, article.ID, secondRevision.ID, "english-guide", "en")
	publishTestArticleForLocale(t, server, login, project.ID, article.ID, secondRevision.ID, "guide-francais", "fr")

	type publicationRoute struct {
		slug       string
		canonical  string
		revisionID string
	}
	loadRoute := func(locale string) publicationRoute {
		t.Helper()
		var route publicationRoute
		if err := db.QueryRow(`
			SELECT slug, canonical_url, published_revision_id
			FROM project_publications
			WHERE project_id = ? AND content_id = ? AND locale = ?
		`, project.ID, article.ID, locale).Scan(&route.slug, &route.canonical, &route.revisionID); err != nil {
			t.Fatal(err)
		}
		return route
	}
	englishBefore := loadRoute("en")
	frenchBefore := loadRoute("fr")

	if _, err := db.Exec(`
		UPDATE project_publications
		SET updated_at = '2099-01-01 00:00:00'
		WHERE project_id = ? AND content_id = ? AND locale = 'en'
	`, project.ID, article.ID); err != nil {
		t.Fatal(err)
	}

	rollbackRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/rollback",
		`{"revisionId":"`+firstRevisionID+`","locale":"fr"}`,
		login,
	)
	rollbackResponse := mustTest(t, server, rollbackRequest)
	if rollbackResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected localized rollback 200, got %d: %s", rollbackResponse.StatusCode, readBody(t, rollbackResponse))
	}

	englishAfter := loadRoute("en")
	frenchAfter := loadRoute("fr")
	if englishAfter != englishBefore {
		t.Fatalf("expected English publication to remain unchanged, before=%#v after=%#v", englishBefore, englishAfter)
	}
	if frenchAfter.slug != frenchBefore.slug || frenchAfter.canonical != frenchBefore.canonical {
		t.Fatalf("expected French routing metadata to remain unchanged, before=%#v after=%#v", frenchBefore, frenchAfter)
	}
	if frenchAfter.revisionID != firstRevisionID {
		t.Fatalf("expected French publication to restore revision %q, got %q", firstRevisionID, frenchAfter.revisionID)
	}

	englishRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/english-guide?locale=en", nil)
	englishRequest.Header.Set("X-Dev-Project-ID", project.ID)
	englishResponse := mustTest(t, server, englishRequest)
	if englishResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected English article 200, got %d: %s", englishResponse.StatusCode, readBody(t, englishResponse))
	}
	var english Envelope[store.PublishedPost]
	decodeJSONResponse(t, englishResponse, &english)
	if english.Data.Title != "Updated Guide" {
		t.Fatalf("expected English publication to remain on updated revision, got %q", english.Data.Title)
	}

	frenchRequest := httptest.NewRequest(http.MethodGet, "/content/v1/posts/guide-francais?locale=fr", nil)
	frenchRequest.Header.Set("X-Dev-Project-ID", project.ID)
	frenchResponse := mustTest(t, server, frenchRequest)
	if frenchResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected French article 200, got %d: %s", frenchResponse.StatusCode, readBody(t, frenchResponse))
	}
	var french Envelope[store.PublishedPost]
	decodeJSONResponse(t, frenchResponse, &french)
	if french.Data.Title != "Original Guide" {
		t.Fatalf("expected French publication to restore original revision, got %q", french.Data.Title)
	}
}

func TestArticleRollbackRequiresPublishedSource(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")

	project := createTestProject(t, server, login, `{"slug":"rollback-state","name":"Rollback State"}`)
	category := createTestCategory(t, server, login, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, login, project.ID, `{
		"articleType":"guide",
		"title":"Stateful Guide",
		"slug":"stateful-guide",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>Body</p>"
	}`)
	revisionID := article.LatestRevision.ID
	approveTestRevision(t, server, login, project.ID, revisionID)

	assertRollbackConflict := func() {
		t.Helper()
		request := newMemberMutationRequest(
			http.MethodPost,
			"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/rollback",
			`{"revisionId":"`+revisionID+`"}`,
			login,
		)
		response := mustTest(t, server, request)
		if response.StatusCode != http.StatusConflict {
			t.Fatalf("expected rollback conflict, got %d: %s", response.StatusCode, readBody(t, response))
		}
	}
	assertRollbackConflict()

	publishTestArticle(t, server, login, project.ID, article.ID, revisionID, "stateful-guide")
	unpublishRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/unpublish",
		`{}`,
		login,
	)
	unpublishResponse := mustTest(t, server, unpublishRequest)
	if unpublishResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected unpublish 200, got %d: %s", unpublishResponse.StatusCode, readBody(t, unpublishResponse))
	}
	assertRollbackConflict()

	var state string
	if err := db.QueryRow(`
		SELECT publication_state
		FROM project_publications
		WHERE project_id = ? AND content_id = ?
	`, project.ID, article.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "unpublished" {
		t.Fatalf("expected failed rollback to preserve unpublished state, got %q", state)
	}

	var rollbackAudits int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ? AND action = 'article.rollback'
	`, project.ID).Scan(&rollbackAudits); err != nil {
		t.Fatal(err)
	}
	if rollbackAudits != 0 {
		t.Fatalf("expected failed rollback attempts not to create success audits, got %d", rollbackAudits)
	}
}

func TestReviewCommentLifecycleAndScoping(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "owner@example.test", "correct horse battery staple")
	otherLogin := seedAndLogin(t, server, db, "other@example.test", "another correct horse battery staple")
	writerLogin := seedAndLogin(t, server, db, "writer@example.test", "writer correct horse battery staple")
	project := createTestProject(t, server, ownerLogin, `{"slug":"comments","name":"Comments Project"}`)
	otherProject := createTestProject(t, server, otherLogin, `{"slug":"other-comments","name":"Other Comments"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, writerLogin.userID); err != nil {
		t.Fatal(err)
	}
	category := createTestCategory(t, server, ownerLogin, project.ID, `{"slug":"guides","name":"Guides"}`)
	article := createTestArticle(t, server, ownerLogin, project.ID, `{
		"articleType":"guide",
		"title":"Reviewed Guide",
		"slug":"reviewed-guide",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>Draft body</p>"
	}`)

	createRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/comments",
		`{"revisionId":"`+article.LatestRevision.ID+`","blockId":"intro","body":"Please add source detail."}`,
		ownerLogin,
	)
	createResponse := mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected comment creation 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.ReviewComment]
	decodeJSONResponse(t, createResponse, &created)
	if created.Data.Status != "open" || created.Data.Body != "Please add source detail." || created.Data.RevisionID != article.LatestRevision.ID {
		t.Fatalf("expected open revision-scoped comment, got %#v", created.Data)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/comments", nil)
	addCookies(listRequest, ownerLogin.cookies)
	listResponse := mustTest(t, server, listRequest)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected comments list 200, got %d: %s", listResponse.StatusCode, readBody(t, listResponse))
	}
	var list ListEnvelope[store.ReviewComment]
	decodeJSONResponse(t, listResponse, &list)
	if len(list.Data) != 1 || list.Data[0].ID != created.Data.ID {
		t.Fatalf("expected created comment in list, got %#v", list.Data)
	}

	writerCommentRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/comments",
		`{"body":"I have addressed this feedback."}`,
		writerLogin,
	)
	writerCommentResponse := mustTest(t, server, writerCommentRequest)
	if writerCommentResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected writer comment creation 201, got %d: %s", writerCommentResponse.StatusCode, readBody(t, writerCommentResponse))
	}

	writerResolveRequest := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/comments/"+created.Data.ID+"/resolve", `{}`, writerLogin)
	writerResolveResponse := mustTest(t, server, writerResolveRequest)
	if writerResolveResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer comment resolution denial, got %d: %s", writerResolveResponse.StatusCode, readBody(t, writerResolveResponse))
	}

	resolveRequest := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/comments/"+created.Data.ID+"/resolve", `{}`, ownerLogin)
	resolveResponse := mustTest(t, server, resolveRequest)
	if resolveResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected resolve comment 200, got %d: %s", resolveResponse.StatusCode, readBody(t, resolveResponse))
	}
	var resolved Envelope[store.ReviewComment]
	decodeJSONResponse(t, resolveResponse, &resolved)
	if resolved.Data.Status != "resolved" || resolved.Data.ResolvedBy != ownerLogin.userID || resolved.Data.ResolvedAt == "" {
		t.Fatalf("expected resolved comment metadata, got %#v", resolved.Data)
	}

	writerReopenRequest := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/comments/"+created.Data.ID+"/reopen", `{}`, writerLogin)
	writerReopenResponse := mustTest(t, server, writerReopenRequest)
	if writerReopenResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer comment reopening denial, got %d: %s", writerReopenResponse.StatusCode, readBody(t, writerReopenResponse))
	}

	reopenRequest := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/comments/"+created.Data.ID+"/reopen", `{}`, ownerLogin)
	reopenResponse := mustTest(t, server, reopenRequest)
	if reopenResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected reopen comment 200, got %d: %s", reopenResponse.StatusCode, readBody(t, reopenResponse))
	}
	var reopened Envelope[store.ReviewComment]
	decodeJSONResponse(t, reopenResponse, &reopened)
	if reopened.Data.Status != "reopened" || reopened.Data.ResolvedBy != "" || reopened.Data.ResolvedAt != "" {
		t.Fatalf("expected reopened comment, got %#v", reopened.Data)
	}

	crossProjectCreate := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+otherProject.ID+"/articles/"+article.ID+"/comments",
		`{"body":"cross project"}`,
		otherLogin,
	)
	crossProjectCreateResponse := mustTest(t, server, crossProjectCreate)
	if crossProjectCreateResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-project article comment to return 404, got %d: %s", crossProjectCreateResponse.StatusCode, readBody(t, crossProjectCreateResponse))
	}

	crossProjectResolve := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+otherProject.ID+"/comments/"+created.Data.ID+"/resolve", `{}`, otherLogin)
	crossProjectResolveResponse := mustTest(t, server, crossProjectResolve)
	if crossProjectResolveResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-project comment resolve to return 404, got %d: %s", crossProjectResolveResponse.StatusCode, readBody(t, crossProjectResolveResponse))
	}

	invalidRevision := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/comments",
		`{"revisionId":"rev_missing","body":"bad revision"}`,
		ownerLogin,
	)
	invalidRevisionResponse := mustTest(t, server, invalidRevision)
	if invalidRevisionResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected invalid revision to return 404, got %d: %s", invalidRevisionResponse.StatusCode, readBody(t, invalidRevisionResponse))
	}

	unicodeBody := strings.Repeat("界", 4000)
	unicodeRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/comments",
		`{"body":"`+unicodeBody+`"}`,
		ownerLogin,
	)
	unicodeResponse := mustTest(t, server, unicodeRequest)
	if unicodeResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected 4000-character Unicode comment creation 201, got %d: %s", unicodeResponse.StatusCode, readBody(t, unicodeResponse))
	}

	tooLongRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/comments",
		`{"body":"`+unicodeBody+`界"}`,
		ownerLogin,
	)
	tooLongResponse := mustTest(t, server, tooLongRequest)
	if tooLongResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 4001-character Unicode comment rejection, got %d: %s", tooLongResponse.StatusCode, readBody(t, tooLongResponse))
	}

	suspendRequest := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/suspend", `{}`, ownerLogin)
	suspendResponse := mustTest(t, server, suspendRequest)
	if suspendResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected project suspension 200, got %d: %s", suspendResponse.StatusCode, readBody(t, suspendResponse))
	}

	suspendedCreateRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/comments",
		`{"body":"should not be created"}`,
		ownerLogin,
	)
	suspendedCreateResponse := mustTest(t, server, suspendedCreateRequest)
	if suspendedCreateResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected suspended-project comment creation conflict, got %d: %s", suspendedCreateResponse.StatusCode, readBody(t, suspendedCreateResponse))
	}

	suspendedResolveRequest := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+project.ID+"/comments/"+created.Data.ID+"/resolve", `{}`, ownerLogin)
	suspendedResolveResponse := mustTest(t, server, suspendedResolveRequest)
	if suspendedResolveResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected suspended-project comment resolution conflict, got %d: %s", suspendedResolveResponse.StatusCode, readBody(t, suspendedResolveResponse))
	}

	var auditCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND target_id = ?
		  AND action IN ('comment.create', 'comment.resolve', 'comment.reopen')
	`, project.ID, created.Data.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 3 {
		t.Fatalf("expected comment lifecycle audit events, got %d", auditCount)
	}
}

func TestReviewAssignmentCreationAndScoping(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerLogin := seedAndLogin(t, server, db, "assignment-owner@example.test", "correct horse battery staple")
	otherLogin := seedAndLogin(t, server, db, "assignment-other@example.test", "another correct horse battery staple")
	reviewerLogin := seedAndLogin(t, server, db, "assignment-reviewer@example.test", "reviewer correct horse password")
	writerLogin := seedAndLogin(t, server, db, "assignment-writer@example.test", "writer correct horse password")
	project := createTestProject(t, server, ownerLogin, `{"slug":"assignments","name":"Assignments Project"}`)
	otherProject := createTestProject(t, server, otherLogin, `{"slug":"other-assignments","name":"Other Assignments"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES
		  (?, ?, 'reviewer', 'active', CURRENT_TIMESTAMP),
		  (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, reviewerLogin.userID, project.ID, writerLogin.userID); err != nil {
		t.Fatal(err)
	}
	category := createTestCategory(t, server, ownerLogin, project.ID, `{"slug":"assignments","name":"Assignments"}`)
	article := createTestArticle(t, server, ownerLogin, project.ID, `{
		"articleType":"guide",
		"title":"Assigned Guide",
		"slug":"assigned-guide",
		"primaryCategoryId":"`+category.ID+`",
		"html":"<p>Draft body</p>"
	}`)

	candidateRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/review-assignees", nil)
	addCookies(candidateRequest, ownerLogin.cookies)
	candidateResponse := mustTest(t, server, candidateRequest)
	if candidateResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected review-assignee list 200, got %d: %s", candidateResponse.StatusCode, readBody(t, candidateResponse))
	}
	var candidates ListEnvelope[store.AdminProjectMember]
	decodeJSONResponse(t, candidateResponse, &candidates)
	if findProjectMember(candidates.Data, reviewerLogin.userID).UserID == "" {
		t.Fatalf("expected reviewer candidate in assignee list, got %#v", candidates.Data)
	}
	if findProjectMember(candidates.Data, writerLogin.userID).UserID != "" {
		t.Fatalf("writer should not be assignable for review, got %#v", candidates.Data)
	}
	firstCandidateRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/review-assignees?limit=1", nil)
	addCookies(firstCandidateRequest, ownerLogin.cookies)
	firstCandidateResponse := mustTest(t, server, firstCandidateRequest)
	var firstCandidates ListEnvelope[store.AdminProjectMember]
	decodeJSONResponse(t, firstCandidateResponse, &firstCandidates)
	if firstCandidateResponse.StatusCode != http.StatusOK || len(firstCandidates.Data) != 1 || firstCandidates.Meta.NextCursor == "" {
		t.Fatalf("unexpected first review-assignee page status=%d body=%#v", firstCandidateResponse.StatusCode, firstCandidates)
	}
	secondCandidateRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/review-assignees?limit=1&cursor="+firstCandidates.Meta.NextCursor,
		nil,
	)
	addCookies(secondCandidateRequest, ownerLogin.cookies)
	secondCandidateResponse := mustTest(t, server, secondCandidateRequest)
	var secondCandidates ListEnvelope[store.AdminProjectMember]
	decodeJSONResponse(t, secondCandidateResponse, &secondCandidates)
	if secondCandidateResponse.StatusCode != http.StatusOK ||
		len(secondCandidates.Data) != 1 ||
		secondCandidates.Data[0].UserID == firstCandidates.Data[0].UserID {
		t.Fatalf("unexpected second review-assignee page status=%d body=%#v", secondCandidateResponse.StatusCode, secondCandidates)
	}

	createRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/assignments",
		`{
			"revisionId":"`+article.LatestRevision.ID+`",
			"assignedTo":"`+reviewerLogin.userID+`",
			"assignmentType":"reviewer",
			"dueAt":"2026-08-01T10:00:00Z"
		}`,
		ownerLogin,
	)
	createResponse := mustTest(t, server, createRequest)
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected assignment creation 201, got %d: %s", createResponse.StatusCode, readBody(t, createResponse))
	}
	var created Envelope[store.ReviewAssignment]
	decodeJSONResponse(t, createResponse, &created)
	if created.Data.Status != "open" ||
		created.Data.ArticleID != article.ID ||
		created.Data.RevisionID != article.LatestRevision.ID ||
		created.Data.AssignedTo != reviewerLogin.userID ||
		created.Data.AssigneeRole != "reviewer" ||
		created.Data.AssignmentType != "reviewer" ||
		created.Data.DueAt != "2026-08-01 10:00:00" {
		t.Fatalf("unexpected created assignment %#v", created.Data)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/assignments", nil)
	addCookies(listRequest, ownerLogin.cookies)
	listResponse := mustTest(t, server, listRequest)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected assignments list 200, got %d: %s", listResponse.StatusCode, readBody(t, listResponse))
	}
	var list ListEnvelope[store.ReviewAssignment]
	decodeJSONResponse(t, listResponse, &list)
	if len(list.Data) != 1 || list.Data[0].ID != created.Data.ID || list.Data[0].AssigneeEmail != "assignment-reviewer@example.test" {
		t.Fatalf("expected created assignment in list, got %#v", list.Data)
	}

	duplicateCreateRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/assignments",
		`{"revisionId":"`+article.LatestRevision.ID+`","assignedTo":"`+reviewerLogin.userID+`","assignmentType":"reviewer"}`,
		ownerLogin,
	)
	duplicateCreateResponse := mustTest(t, server, duplicateCreateRequest)
	if duplicateCreateResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected duplicate assignment validation, got %d: %s", duplicateCreateResponse.StatusCode, readBody(t, duplicateCreateResponse))
	}

	completeRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/assignments/"+created.Data.ID+"/complete",
		`{}`,
		reviewerLogin,
	)
	completeResponse := mustTest(t, server, completeRequest)
	if completeResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected assignee completion 200, got %d: %s", completeResponse.StatusCode, readBody(t, completeResponse))
	}
	var completed Envelope[store.ReviewAssignment]
	decodeJSONResponse(t, completeResponse, &completed)
	if completed.Data.Status != "completed" || completed.Data.ClosedBy != reviewerLogin.userID || completed.Data.ClosedAt == "" {
		t.Fatalf("unexpected completed assignment %#v", completed.Data)
	}

	repeatCompleteRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/assignments/"+created.Data.ID+"/complete",
		`{}`,
		reviewerLogin,
	)
	repeatCompleteResponse := mustTest(t, server, repeatCompleteRequest)
	if repeatCompleteResponse.StatusCode != http.StatusConflict {
		t.Fatalf("expected repeated completion conflict, got %d: %s", repeatCompleteResponse.StatusCode, readBody(t, repeatCompleteResponse))
	}
	if _, err := db.Exec(`UPDATE review_assignments SET created_at = '2026-01-01 00:00:00' WHERE id = ?`, created.Data.ID); err != nil {
		t.Fatal(err)
	}

	reassignRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/assignments",
		`{"revisionId":"`+article.LatestRevision.ID+`","assignedTo":"`+reviewerLogin.userID+`","assignmentType":"reviewer"}`,
		ownerLogin,
	)
	reassignResponse := mustTest(t, server, reassignRequest)
	if reassignResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected reassignment after completion 201, got %d: %s", reassignResponse.StatusCode, readBody(t, reassignResponse))
	}
	var reassigned Envelope[store.ReviewAssignment]
	decodeJSONResponse(t, reassignResponse, &reassigned)

	reviewerCancelRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/assignments/"+reassigned.Data.ID+"/cancel",
		`{}`,
		reviewerLogin,
	)
	reviewerCancelResponse := mustTest(t, server, reviewerCancelRequest)
	if reviewerCancelResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected reviewer cancellation denial, got %d: %s", reviewerCancelResponse.StatusCode, readBody(t, reviewerCancelResponse))
	}

	cancelRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/assignments/"+reassigned.Data.ID+"/cancel",
		`{}`,
		ownerLogin,
	)
	cancelResponse := mustTest(t, server, cancelRequest)
	if cancelResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected manager cancellation 200, got %d: %s", cancelResponse.StatusCode, readBody(t, cancelResponse))
	}
	var cancelled Envelope[store.ReviewAssignment]
	decodeJSONResponse(t, cancelResponse, &cancelled)
	if cancelled.Data.Status != "cancelled" || cancelled.Data.ClosedBy != ownerLogin.userID || cancelled.Data.ClosedAt == "" {
		t.Fatalf("unexpected cancelled assignment %#v", cancelled.Data)
	}

	firstPageRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/assignments?limit=1", nil)
	addCookies(firstPageRequest, ownerLogin.cookies)
	firstPageResponse := mustTest(t, server, firstPageRequest)
	if firstPageResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected assignment first page 200, got %d: %s", firstPageResponse.StatusCode, readBody(t, firstPageResponse))
	}
	var firstPage ListEnvelope[store.ReviewAssignment]
	decodeJSONResponse(t, firstPageResponse, &firstPage)
	if len(firstPage.Data) != 1 || firstPage.Data[0].ID != reassigned.Data.ID || firstPage.Meta.NextCursor == "" || firstPage.Meta.OpenCount == nil || *firstPage.Meta.OpenCount != 0 {
		t.Fatalf("unexpected newest-first assignment page %#v", firstPage)
	}
	secondPageRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/assignments?limit=1&cursor="+firstPage.Meta.NextCursor,
		nil,
	)
	addCookies(secondPageRequest, ownerLogin.cookies)
	secondPageResponse := mustTest(t, server, secondPageRequest)
	if secondPageResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected assignment second page 200, got %d: %s", secondPageResponse.StatusCode, readBody(t, secondPageResponse))
	}
	var secondPage ListEnvelope[store.ReviewAssignment]
	decodeJSONResponse(t, secondPageResponse, &secondPage)
	if len(secondPage.Data) != 1 || secondPage.Data[0].ID != created.Data.ID || secondPage.Meta.NextCursor != "" || secondPage.Meta.OpenCount == nil || *secondPage.Meta.OpenCount != 0 {
		t.Fatalf("unexpected assignment second page %#v", secondPage)
	}

	var assignmentNotifications, suppressedNotifications int
	if err := db.QueryRow(`
		SELECT COUNT(1), SUM(CASE WHEN status = 'suppressed' THEN 1 ELSE 0 END)
		FROM review_assignment_notifications
		WHERE project_id = ?
		  AND recipient_user_id = ?
	`, project.ID, reviewerLogin.userID).Scan(&assignmentNotifications, &suppressedNotifications); err != nil {
		t.Fatal(err)
	}
	if assignmentNotifications != 2 || suppressedNotifications != 2 {
		t.Fatalf("expected closed assignments to retain suppressed notifications, total=%d suppressed=%d", assignmentNotifications, suppressedNotifications)
	}

	writerCreateRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/assignments",
		`{"assignedTo":"`+reviewerLogin.userID+`","assignmentType":"reviewer"}`,
		writerLogin,
	)
	writerCreateResponse := mustTest(t, server, writerCreateRequest)
	if writerCreateResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer assignment creation denial, got %d: %s", writerCreateResponse.StatusCode, readBody(t, writerCreateResponse))
	}

	reviewerCreateRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/assignments",
		`{"assignedTo":"`+reviewerLogin.userID+`","assignmentType":"reviewer"}`,
		reviewerLogin,
	)
	reviewerCreateResponse := mustTest(t, server, reviewerCreateRequest)
	if reviewerCreateResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected reviewer assignment creation denial, got %d: %s", reviewerCreateResponse.StatusCode, readBody(t, reviewerCreateResponse))
	}

	crossProjectArticle := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+otherProject.ID+"/articles/"+article.ID+"/assignments",
		`{"assignedTo":"`+otherLogin.userID+`","assignmentType":"reviewer"}`,
		otherLogin,
	)
	crossProjectArticleResponse := mustTest(t, server, crossProjectArticle)
	if crossProjectArticleResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-project article assignment to return 404, got %d: %s", crossProjectArticleResponse.StatusCode, readBody(t, crossProjectArticleResponse))
	}

	crossProjectAssignee := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/assignments",
		`{"assignedTo":"`+otherLogin.userID+`","assignmentType":"reviewer"}`,
		ownerLogin,
	)
	crossProjectAssigneeResponse := mustTest(t, server, crossProjectAssignee)
	if crossProjectAssigneeResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-project assignee to return 404, got %d: %s", crossProjectAssigneeResponse.StatusCode, readBody(t, crossProjectAssigneeResponse))
	}

	writerAssignee := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/assignments",
		`{"assignedTo":"`+writerLogin.userID+`","assignmentType":"reviewer"}`,
		ownerLogin,
	)
	writerAssigneeResponse := mustTest(t, server, writerAssignee)
	if writerAssigneeResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer reviewer-assignment denial, got %d: %s", writerAssigneeResponse.StatusCode, readBody(t, writerAssigneeResponse))
	}

	invalidRevision := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+project.ID+"/articles/"+article.ID+"/assignments",
		`{"revisionId":"rev_missing","assignedTo":"`+reviewerLogin.userID+`","assignmentType":"reviewer"}`,
		ownerLogin,
	)
	invalidRevisionResponse := mustTest(t, server, invalidRevision)
	if invalidRevisionResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected invalid assignment revision to return 404, got %d: %s", invalidRevisionResponse.StatusCode, readBody(t, invalidRevisionResponse))
	}

	crossProjectTransition := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+otherProject.ID+"/assignments/"+created.Data.ID+"/complete",
		`{}`,
		otherLogin,
	)
	crossProjectTransitionResponse := mustTest(t, server, crossProjectTransition)
	if crossProjectTransitionResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-project assignment transition to return 404, got %d: %s", crossProjectTransitionResponse.StatusCode, readBody(t, crossProjectTransitionResponse))
	}

	var auditCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND target_id = ?
		  AND action = 'assignment.create'
	`, project.ID, created.Data.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("expected assignment audit event, got %d", auditCount)
	}
	var transitionAuditCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND action IN ('assignment.completed', 'assignment.cancelled')
		  AND target_id IN (?, ?)
	`, project.ID, created.Data.ID, reassigned.Data.ID).Scan(&transitionAuditCount); err != nil {
		t.Fatal(err)
	}
	if transitionAuditCount != 2 {
		t.Fatalf("expected assignment transition audit events, got %d", transitionAuditCount)
	}
}

func TestCopyArticleToProjectCreatesIndependentAuditedDraft(t *testing.T) {
	server, db := newAdminTestServer(t)
	login := seedAndLogin(t, server, db, "copy-owner@example.test", "correct horse battery staple")
	sourceProject := createTestProject(t, server, login, `{"slug":"copy-source","name":"Copy Source","primaryDomain":"source.example.test"}`)
	destinationProject := createTestProject(t, server, login, `{"slug":"copy-destination","name":"Copy Destination","primaryDomain":"destination.example.test"}`)
	sourceCategory := createTestCategory(t, server, login, sourceProject.ID, `{"slug":"source","name":"Source"}`)
	destinationCategory := createTestCategory(t, server, login, destinationProject.ID, `{"slug":"destination","name":"Destination"}`)
	sourceArticle := createTestArticle(t, server, login, sourceProject.ID, `{
		"articleType":"guide",
		"title":"Original draft",
		"slug":"copy-me",
		"primaryCategoryId":"`+sourceCategory.ID+`",
		"html":"<p>Original body</p>"
	}`)
	selectedRevision := createTestRevision(t, server, login, sourceProject.ID, sourceArticle.ID, `{
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

	copyRequest := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+sourceProject.ID+"/articles/"+sourceArticle.ID+"/copy-to-project",
		`{
			"destinationProjectId":"`+destinationProject.ID+`",
			"sourceRevisionId":"`+selectedRevision.ID+`",
			"primaryCategoryId":"`+destinationCategory.ID+`",
			"slug":"copied-guide",
			"locale":"en",
			"canonicalDecision":"canonical_original",
			"canonicalOriginalUrl":"https://source.example.test/blog/copy-me"
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
	if copied.Data.Title != selectedRevision.Title || copied.Data.Title == newerRevision.Title {
		t.Fatalf("expected exact selected revision to be copied, got title %q", copied.Data.Title)
	}
	if copied.Data.EditorialState != "draft" || copied.Data.PublicationState != "unpublished" {
		t.Fatalf("expected independent unpublished draft, got %#v", copied.Data)
	}
	if copied.Data.CanonicalPolicy != "canonical_original" ||
		copied.Data.CanonicalURL != "https://source.example.test/blog/copy-me" {
		t.Fatalf("unexpected canonical-original decision %#v", copied.Data)
	}
	if copied.Data.LatestRevision == nil || copied.Data.LatestRevision.ID == selectedRevision.ID ||
		copied.Data.LatestRevision.RevisionNumber != 1 {
		t.Fatalf("expected independent first revision, got %#v", copied.Data.LatestRevision)
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
	if copiedBody != "<p>Selected body</p>" ||
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
			!strings.Contains(metadataJSON, selectedRevision.ID) {
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
			"sourceRevisionId":"`+newerRevision.ID+`",
			"primaryCategoryId":"`+destinationCategory.ID+`",
			"slug":"adapted-guide",
			"canonicalDecision":"material_adaptation"
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
	sourceArticle := createTestArticle(t, server, login, sourceProject.ID, `{
		"title":"Canonical source",
		"slug":"canonical-source",
		"primaryCategoryId":"`+sourceCategory.ID+`",
		"html":"<p>Safe body</p>"
	}`)
	copyPath := "/api/v1/projects/" + sourceProject.ID + "/articles/" + sourceArticle.ID + "/copy-to-project"
	copyBody := func(revisionID, slug, decision, canonicalField string) string {
		return `{
			"destinationProjectId":"` + destinationProject.ID + `",
			"sourceRevisionId":"` + revisionID + `",
			"primaryCategoryId":"` + destinationCategory.ID + `",
			"slug":"` + slug + `",
			"canonicalDecision":"` + decision + `"` + canonicalField + `
		}`
	}

	unrelatedCanonical := mustTest(
		t,
		server,
		newMemberMutationRequest(
			http.MethodPost,
			copyPath,
			copyBody(sourceArticle.LatestRevision.ID, "unrelated-canonical", "canonical_original", `,"canonicalOriginalUrl":"https://unrelated.example.test/post"`),
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
			copyBody(sourceArticle.LatestRevision.ID, "derived-canonical", "canonical_original", ""),
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

	referencedRevision := createTestRevision(t, server, login, sourceProject.ID, sourceArticle.ID, `{
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
			copyBody(referencedRevision.ID, "unsafe-reference", "material_adaptation", ""),
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

	copyBody := func(article store.AdminArticle) string {
		return `{
			"destinationProjectId":"` + destinationProject.ID + `",
			"sourceRevisionId":"` + article.LatestRevision.ID + `",
			"primaryCategoryId":"` + destinationCategory.ID + `",
			"slug":"scoped-copy",
			"canonicalDecision":"material_adaptation"
		}`
	}
	copyPath := "/api/v1/projects/" + sourceProject.ID + "/articles/" + sourceArticle.ID + "/copy-to-project"

	noDestinationAccess := mustTest(t, server, newMemberMutationRequest(http.MethodPost, copyPath, copyBody(sourceArticle), copier))
	if noDestinationAccess.StatusCode != http.StatusNotFound {
		t.Fatalf("expected missing destination membership to return 404, got %d: %s", noDestinationAccess.StatusCode, readBody(t, noDestinationAccess))
	}

	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'reviewer', 'active', CURRENT_TIMESTAMP)
	`, destinationProject.ID, copier.userID); err != nil {
		t.Fatal(err)
	}
	reviewerDestination := mustTest(t, server, newMemberMutationRequest(http.MethodPost, copyPath, copyBody(sourceArticle), copier))
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
			copyBody(otherArticle),
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
	noSourceAccess := mustTest(t, server, newMemberMutationRequest(http.MethodPost, copyPath, copyBody(sourceArticle), copier))
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
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(body))
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
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatal(err)
		}
		body = string(encoded)
	}
	request := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/articles/"+articleID+"/revisions", body, login)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create revision 201, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.AdminRevision]
	decodeJSONResponse(t, response, &payload)
	return payload.Data
}

func approveTestRevision(t *testing.T, server *Server, login adminLoginResult, projectID, revisionID string) store.AdminRevision {
	t.Helper()
	request := newMemberMutationRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/revisions/"+revisionID+"/approve", `{}`, login)
	response := mustTest(t, server, request)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected approve revision 200, got %d: %s", response.StatusCode, readBody(t, response))
	}
	var payload Envelope[store.AdminRevision]
	decodeJSONResponse(t, response, &payload)
	return payload.Data
}

func publishTestArticle(t *testing.T, server *Server, login adminLoginResult, projectID, articleID, revisionID, slug string) store.AdminArticle {
	t.Helper()
	return publishTestArticleForLocale(t, server, login, projectID, articleID, revisionID, slug, "")
}

func publishTestArticleForLocale(t *testing.T, server *Server, login adminLoginResult, projectID, articleID, revisionID, slug, locale string) store.AdminArticle {
	t.Helper()
	localeField := ""
	if locale != "" {
		localeField = `,"locale":"` + locale + `"`
	}
	request := newMemberMutationRequest(
		http.MethodPost,
		"/api/v1/projects/"+projectID+"/articles/"+articleID+"/publish",
		`{"revisionId":"`+revisionID+`","slug":"`+slug+`"`+localeField+`}`,
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
	response, err := server.app.Test(request, 15_000)
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
