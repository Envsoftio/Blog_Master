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
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"seoblog/apps/backend/internal/config"
	"seoblog/apps/backend/internal/platform/database"
	"seoblog/apps/backend/internal/security"
	"seoblog/apps/backend/internal/store"
)

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

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID+"/authors", nil)
	addCookies(listRequest, login.cookies)
	listResponse := mustTest(t, server, listRequest)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected author list 200, got %d: %s", listResponse.StatusCode, readBody(t, listResponse))
	}
	var list ListEnvelope[store.Author]
	decodeJSONResponse(t, listResponse, &list)
	if len(list.Data) != 1 || list.Data[0].ID != author.ID {
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
		published.Data[0].PhotoAssetID != "asset-priya" {
		t.Fatalf("expected active author in content API, got %#v", published.Data)
	}

	patchRequest := newMemberMutationRequest(
		http.MethodPatch,
		"/api/v1/projects/"+project.ID+"/authors/"+author.ID,
		`{"displayName":"Priya S.","photoAssetId":"","status":"inactive","sameAs":["https://example.test/people/priya"]}`,
		login,
	)
	patchResponse := mustTest(t, server, patchRequest)
	if patchResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected author update 200, got %d: %s", patchResponse.StatusCode, readBody(t, patchResponse))
	}
	var patched Envelope[store.Author]
	decodeJSONResponse(t, patchResponse, &patched)
	if patched.Data.DisplayName != "Priya S." || patched.Data.Status != "inactive" || patched.Data.PhotoAssetID != "" {
		t.Fatalf("expected patched inactive author, got %#v", patched.Data)
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
	if !changeTypes["author.created"] || !changeTypes["author.updated"] {
		t.Fatalf("expected author create and update change events, got %#v", changes.Data)
	}

	var contentGeneration int64
	if err := db.QueryRow(`
		SELECT content_generation
		FROM projects
		WHERE id = ?
	`, project.ID).Scan(&contentGeneration); err != nil {
		t.Fatal(err)
	}
	if contentGeneration != 3 {
		t.Fatalf("expected author writes to advance content generation to 3, got %d", contentGeneration)
	}

	var auditCount int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM audit_events
		WHERE project_id = ?
		  AND target_id = ?
		  AND action IN ('author.create', 'author.update')
	`, project.ID, author.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 2 {
		t.Fatalf("expected author create and update audit events, got %d", auditCount)
	}
}

func TestAuthorManagementAuthorizationAndCrossProjectScoping(t *testing.T) {
	server, db := newAdminTestServer(t)
	ownerALogin := seedAndLogin(t, server, db, "owner-a@example.test", "owner a correct password")
	ownerBLogin := seedAndLogin(t, server, db, "owner-b@example.test", "owner b correct password")
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
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, projectA.ID, writerLogin.userID); err != nil {
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

type adminLoginResult struct {
	cookies   []*http.Cookie
	csrfToken string
	userID    string
}

func newAdminTestServer(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "admin.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	server := New(Options{
		Config: config.Config{Env: "development", DevAuth: true},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Store:  store.New(db),
	})
	return server, db
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
