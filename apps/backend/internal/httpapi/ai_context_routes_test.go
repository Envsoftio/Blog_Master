package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"seoblog/apps/backend/internal/store"
)

func TestVoiceProfilesAreVersionedValidatedAndProjectScoped(t *testing.T) {
	server, db := newAdminTestServer(t)
	owner := seedAndLogin(t, server, db, "voice-owner@example.test", "correct horse battery staple")
	writer := seedAndLogin(t, server, db, "voice-writer@example.test", "another correct horse battery staple")
	project := createTestProject(t, server, owner, `{
		"slug":"voice-project",
		"name":"Voice Project",
		"defaultLocale":"en",
		"supportedLocales":["en","en-GB"]
	}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, writer.userID); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/projects/" + project.ID + "/voice-profile"

	missingRequest := httptest.NewRequest(http.MethodGet, path, nil)
	addCookies(missingRequest, owner.cookies)
	if response := mustTest(t, server, missingRequest); response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected missing voice profile 404, got %d", response.StatusCode)
	}

	writerCreate := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		validVoiceProfileBody("en", "Direct and practical"),
		writer,
	))
	if writerCreate.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer voice-profile creation to fail with 403, got %d", writerCreate.StatusCode)
	}

	invalidCreate := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		`{"profile":{"audience":"Teams","writingExamples":[{"title":"Only one","excerpt":"This excerpt is deliberately long enough for validation."}]}}`,
		owner,
	))
	if invalidCreate.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected incomplete voice profile to fail with 400, got %d", invalidCreate.StatusCode)
	}

	firstResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		validVoiceProfileBody("en", "Direct and practical"),
		owner,
	))
	if firstResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected voice profile create 201, got %d: %s", firstResponse.StatusCode, readBody(t, firstResponse))
	}
	var first Envelope[store.VoiceProfile]
	decodeJSONResponse(t, firstResponse, &first)
	if first.Data.Version != 1 ||
		first.Data.Profile.Locale != "en" ||
		len(first.Data.Profile.WritingExamples) != 3 {
		t.Fatalf("unexpected first voice profile %#v", first.Data)
	}

	secondResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		validVoiceProfileBody("en-GB", "Calm, exact, and candid"),
		owner,
	))
	if secondResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected second voice profile create 201, got %d: %s", secondResponse.StatusCode, readBody(t, secondResponse))
	}
	var second Envelope[store.VoiceProfile]
	decodeJSONResponse(t, secondResponse, &second)
	if second.Data.Version != 2 || second.Data.Profile.Tone != "Calm, exact, and candid" {
		t.Fatalf("expected second voice version, got %#v", second.Data)
	}

	getRequest := httptest.NewRequest(http.MethodGet, path, nil)
	addCookies(getRequest, writer.cookies)
	getResponse := mustTest(t, server, getRequest)
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected member voice-profile read 200, got %d", getResponse.StatusCode)
	}
	if cacheControl := getResponse.Header.Get("Cache-Control"); cacheControl != "private, no-store" {
		t.Fatalf("expected private no-store voice profile, got %q", cacheControl)
	}
	var latest Envelope[store.VoiceProfile]
	decodeJSONResponse(t, getResponse, &latest)
	if latest.Data.ID != second.Data.ID {
		t.Fatalf("expected latest voice profile %q, got %q", second.Data.ID, latest.Data.ID)
	}

	if _, err := db.Exec(`
		UPDATE voice_profiles
		SET profile_json = '{}'
		WHERE project_id = ? AND id = ?
	`, project.ID, first.Data.ID); err == nil {
		t.Fatal("expected voice profile versions to be immutable")
	}

	unsupportedLocale := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		validVoiceProfileBody("fr", "Direct and practical"),
		owner,
	))
	if unsupportedLocale.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected unsupported voice locale to fail with 400, got %d", unsupportedLocale.StatusCode)
	}

	otherProject := createTestProject(t, server, owner, `{"slug":"other-voice","name":"Other Voice"}`)
	otherRequest := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+otherProject.ID+"/voice-profile", nil)
	addCookies(otherRequest, owner.cookies)
	if response := mustTest(t, server, otherRequest); response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected voice profiles to remain project-scoped, got %d", response.StatusCode)
	}

	var auditCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM audit_events
		WHERE project_id = ? AND action = 'voice_profile.create'
	`, project.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 2 {
		t.Fatalf("expected two voice-profile audit events, got %d", auditCount)
	}
}

func TestEvidencePacketsUseProjectSourcesAndImmutableApproval(t *testing.T) {
	server, db := newAdminTestServer(t)
	owner := seedAndLogin(t, server, db, "evidence-owner@example.test", "correct horse battery staple")
	writer := seedAndLogin(t, server, db, "evidence-writer@example.test", "another correct horse battery staple")
	project := createTestProject(t, server, owner, `{"slug":"evidence-project","name":"Evidence Project"}`)
	otherProject := createTestProject(t, server, owner, `{"slug":"other-evidence","name":"Other Evidence"}`)
	if _, err := db.Exec(`
		INSERT INTO project_memberships(project_id, user_id, role, status, joined_at)
		VALUES (?, ?, 'writer', 'active', CURRENT_TIMESTAMP)
	`, project.ID, writer.userID); err != nil {
		t.Fatal(err)
	}
	category := createTestCategory(t, server, owner, project.ID, `{"slug":"evidence","name":"Evidence"}`)
	article := createTestArticle(
		t,
		server,
		owner,
		project.ID,
		`{"articleType":"guide","title":"Evidence Guide","slug":"evidence-guide","primaryCategoryId":"`+category.ID+`","html":"<p>Evidence body.</p>"}`,
	)
	otherCategory := createTestCategory(t, server, owner, otherProject.ID, `{"slug":"other-evidence","name":"Other Evidence"}`)
	otherArticle := createTestArticle(
		t,
		server,
		owner,
		otherProject.ID,
		`{"articleType":"guide","title":"Other Evidence Guide","slug":"other-evidence-guide","primaryCategoryId":"`+otherCategory.ID+`","html":"<p>Other evidence body.</p>"}`,
	)
	if _, err := db.Exec(`
		INSERT INTO sources(id, project_id, title, url, source_type)
		VALUES
		  ('source-evidence', ?, 'Primary evidence', 'https://example.test/evidence', 'web'),
		  ('source-other', ?, 'Other evidence', 'https://example.test/other', 'web')
	`, project.ID, otherProject.ID); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/projects/" + project.ID + "/evidence-packets"
	body := validEvidencePacketBody(article.ID, "source-evidence", "A useful original thesis for the evidence-backed article.")

	firstResponse := mustTest(t, server, newMemberMutationRequest(http.MethodPost, path, body, writer))
	if firstResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected writer evidence create 201, got %d: %s", firstResponse.StatusCode, readBody(t, firstResponse))
	}
	var first Envelope[store.EvidencePacket]
	decodeJSONResponse(t, firstResponse, &first)
	if first.Data.Version != 1 ||
		first.Data.ContentID != article.ID ||
		first.Data.CreatedBy != writer.userID ||
		len(first.Data.Packet.SourceIDs) != 1 ||
		first.Data.Packet.CallToAction == "" ||
		first.Data.Packet.PublicationRecommendation != "ready" {
		t.Fatalf("unexpected first evidence packet %#v", first.Data)
	}

	crossSourceResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		validEvidencePacketBody(article.ID, "source-other", "Another useful original thesis for this article."),
		writer,
	))
	if crossSourceResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected cross-project evidence source to fail with 400, got %d", crossSourceResponse.StatusCode)
	}

	crossContentResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		validEvidencePacketBody(otherArticle.ID, "source-evidence", "Another useful original thesis for this article."),
		writer,
	))
	if crossContentResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected cross-project evidence content to fail with 400, got %d", crossContentResponse.StatusCode)
	}

	secondResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		validEvidencePacketBody(article.ID, "source-evidence", "A revised thesis with a clearer evidence-backed angle."),
		writer,
	))
	if secondResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected second evidence create 201, got %d: %s", secondResponse.StatusCode, readBody(t, secondResponse))
	}
	var second Envelope[store.EvidencePacket]
	decodeJSONResponse(t, secondResponse, &second)
	if second.Data.Version != 2 {
		t.Fatalf("expected evidence version 2, got %#v", second.Data)
	}

	firstPageRequest := httptest.NewRequest(
		http.MethodGet,
		path+"?contentId="+article.ID+"&limit=1",
		nil,
	)
	addCookies(firstPageRequest, writer.cookies)
	firstPageResponse := mustTest(t, server, firstPageRequest)
	if firstPageResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected evidence list 200, got %d: %s", firstPageResponse.StatusCode, readBody(t, firstPageResponse))
	}
	if cacheControl := firstPageResponse.Header.Get("Cache-Control"); cacheControl != "private, no-store" {
		t.Fatalf("expected private no-store evidence list, got %q", cacheControl)
	}
	var firstPage ListEnvelope[store.EvidencePacket]
	decodeJSONResponse(t, firstPageResponse, &firstPage)
	if len(firstPage.Data) != 1 ||
		firstPage.Data[0].ID != second.Data.ID ||
		firstPage.Meta.NextCursor == "" {
		t.Fatalf("expected newest evidence version and cursor, got %#v", firstPage)
	}

	secondPageRequest := httptest.NewRequest(
		http.MethodGet,
		path+"?contentId="+article.ID+"&limit=1&cursor="+firstPage.Meta.NextCursor,
		nil,
	)
	addCookies(secondPageRequest, writer.cookies)
	secondPageResponse := mustTest(t, server, secondPageRequest)
	var secondPage ListEnvelope[store.EvidencePacket]
	decodeJSONResponse(t, secondPageResponse, &secondPage)
	if len(secondPage.Data) != 1 || secondPage.Data[0].ID != first.Data.ID {
		t.Fatalf("expected stable evidence pagination, got %#v", secondPage.Data)
	}
	filterMismatchRequest := httptest.NewRequest(
		http.MethodGet,
		path+"?approvalState=approved&cursor="+firstPage.Meta.NextCursor,
		nil,
	)
	addCookies(filterMismatchRequest, writer.cookies)
	if response := mustTest(t, server, filterMismatchRequest); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected evidence cursor from another filter to fail with 400, got %d", response.StatusCode)
	}

	missingUniqueEvidence := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		sourceOnlyEvidencePacketBody(article.ID, "source-evidence", "ready"),
		writer,
	))
	if missingUniqueEvidence.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected source-only ready packet to fail with 400, got %d", missingUniqueEvidence.StatusCode)
	}

	needsEvidenceResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path,
		sourceOnlyEvidencePacketBody(article.ID, "source-evidence", "request_unique_evidence"),
		writer,
	))
	if needsEvidenceResponse.StatusCode != http.StatusCreated {
		t.Fatalf("expected evidence request packet 201, got %d: %s", needsEvidenceResponse.StatusCode, readBody(t, needsEvidenceResponse))
	}
	var needsEvidence Envelope[store.EvidencePacket]
	decodeJSONResponse(t, needsEvidenceResponse, &needsEvidence)
	if needsEvidence.Data.Version != 3 ||
		needsEvidence.Data.Packet.PublicationRecommendation != "request_unique_evidence" {
		t.Fatalf("unexpected needs-evidence packet %#v", needsEvidence.Data)
	}
	notReadyApproval := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path+"/"+needsEvidence.Data.ID+"/approve",
		`{}`,
		owner,
	))
	if notReadyApproval.StatusCode != http.StatusConflict {
		t.Fatalf("expected non-ready evidence approval to fail with 409, got %d", notReadyApproval.StatusCode)
	}

	writerApproval := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path+"/"+second.Data.ID+"/approve",
		`{}`,
		writer,
	))
	if writerApproval.StatusCode != http.StatusForbidden {
		t.Fatalf("expected writer evidence approval to fail with 403, got %d", writerApproval.StatusCode)
	}

	if _, err := db.Exec(`DELETE FROM sources WHERE project_id = ? AND id = 'source-evidence'`, project.ID); err != nil {
		t.Fatal(err)
	}
	staleSourceApproval := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path+"/"+second.Data.ID+"/approve",
		`{}`,
		owner,
	))
	if staleSourceApproval.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected missing evidence source to block approval with 400, got %d", staleSourceApproval.StatusCode)
	}
	if _, err := db.Exec(`
		INSERT INTO sources(id, project_id, title, url, source_type)
		VALUES ('source-evidence', ?, 'Primary evidence', 'https://example.test/evidence', 'web')
	`, project.ID); err != nil {
		t.Fatal(err)
	}

	approvalResponse := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path+"/"+second.Data.ID+"/approve",
		`{}`,
		owner,
	))
	if approvalResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected evidence approval 200, got %d: %s", approvalResponse.StatusCode, readBody(t, approvalResponse))
	}
	var approved Envelope[store.EvidencePacket]
	decodeJSONResponse(t, approvalResponse, &approved)
	if approved.Data.ApprovedBy != owner.userID || approved.Data.ApprovedAt == "" {
		t.Fatalf("unexpected evidence approval %#v", approved.Data)
	}
	repeatedApproval := mustTest(t, server, newMemberMutationRequest(
		http.MethodPost,
		path+"/"+second.Data.ID+"/approve",
		`{}`,
		owner,
	))
	if repeatedApproval.StatusCode != http.StatusConflict {
		t.Fatalf("expected repeated evidence approval to fail with 409, got %d", repeatedApproval.StatusCode)
	}

	approvedRequest := httptest.NewRequest(http.MethodGet, path+"?approvalState=approved", nil)
	addCookies(approvedRequest, owner.cookies)
	approvedResponse := mustTest(t, server, approvedRequest)
	var approvedPackets ListEnvelope[store.EvidencePacket]
	decodeJSONResponse(t, approvedResponse, &approvedPackets)
	if len(approvedPackets.Data) != 1 || approvedPackets.Data[0].ID != second.Data.ID {
		t.Fatalf("expected approved evidence filter, got %#v", approvedPackets.Data)
	}

	if _, err := db.Exec(`
		UPDATE evidence_packets
		SET packet_json = '{}'
		WHERE project_id = ? AND id = ?
	`, project.ID, second.Data.ID); err == nil {
		t.Fatal("expected evidence packet content to be immutable")
	}
	if _, err := db.Exec(`
		INSERT INTO evidence_packets(id, project_id, content_id, version, packet_json, created_by)
		VALUES ('cross-project-packet', ?, ?, 99, '{}', ?)
	`, project.ID, otherArticle.ID, owner.userID); err == nil {
		t.Fatal("expected database guard to reject cross-project evidence content")
	}
	if _, err := db.Exec(`
		INSERT INTO evidence_packets(id, project_id, content_id, version, packet_json, created_by)
		VALUES ('project-packet-one', ?, NULL, 1, '{}', ?)
	`, project.ID, owner.userID); err != nil {
		t.Fatalf("insert first project-scoped evidence version: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO evidence_packets(id, project_id, content_id, version, packet_json, created_by)
		VALUES ('project-packet-duplicate', ?, NULL, 1, '{}', ?)
	`, project.ID, owner.userID); err == nil {
		t.Fatal("expected duplicate project-scoped evidence version to fail")
	}

	var auditCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM audit_events
		WHERE project_id = ?
		  AND action IN ('evidence_packet.create', 'evidence_packet.approve')
	`, project.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 4 {
		t.Fatalf("expected three evidence creation audits and one approval audit, got %d", auditCount)
	}
}

func validVoiceProfileBody(locale, tone string) string {
	return `{"profile":{
		"audience":"Growth and product teams responsible for technical websites",
		"assumedKnowledge":"Comfortable with web publishing and basic search concepts",
		"brandPurpose":"Help teams publish accurate and genuinely useful technical guidance",
		"pointOfView":"Practical evidence matters more than promotional certainty",
		"tone":"` + tone + `",
		"formality":"Conversational professional",
		"humor":"Light and occasional",
		"preferredVocabulary":["evidence-backed","specific"],
		"productTerminology":{"CMS":"content platform"},
		"approvedProductFacts":["The platform keeps revision history."],
		"sentencePreferences":"Prefer clear active sentences with varied but controlled length",
		"paragraphPreferences":"Keep paragraphs focused on one idea and under five sentences",
		"avoidPhrases":["game-changing","revolutionary"],
		"prohibitedClaims":["guaranteed rankings"],
		"contentTypeStyles":{"guide":"Use ordered decisions and practical examples"},
		"writingExamples":[
		  {"title":"Migration guide","excerpt":"This example explains a difficult migration with precise steps and candid limitations."},
		  {"title":"Editorial review","excerpt":"This example evaluates editorial tradeoffs without hiding uncertainty or operational cost."},
		  {"title":"Technical comparison","excerpt":"This example compares alternatives against explicit criteria and cites the deciding evidence."}
		],
		"introductionRules":"Begin with the reader problem and the concrete outcome",
		"conclusionRules":"Summarize the decision and its important limitations",
		"callToActionRules":"Offer one relevant next action without manufactured urgency",
		"regionalSpelling":"Use the spelling conventions of the selected locale",
		"locale":"` + locale + `"
	}}`
}

func validEvidencePacketBody(contentID, sourceID, thesis string) string {
	return `{"contentId":"` + contentID + `","packet":{
		"humanBrief":"Explain the decision clearly for teams comparing implementation options.",
		"searchIntent":"Practical implementation comparison",
		"thesis":"` + thesis + `",
		"productFacts":[{"statement":"The workflow preserves immutable approved revisions.","sourceIds":["` + sourceID + `"]}],
		"subjectMatterNotes":["Operational simplicity matters for a small editorial team."],
		"firsthandObservations":["The review flow was exercised with representative draft content."],
		"sourceIds":["` + sourceID + `"],
		"customerEvidence":[],
		"measurements":["The focused integration suite completes in under two seconds locally."],
		"allowedClaims":["The workflow preserves revision history."],
		"prohibitedClaims":["Guaranteed search performance."],
		"limitations":["Provider execution remains a separate implementation slice."],
		"requiredInternalLinks":["/guides/editorial-review"],
		"callToAction":"Review the implementation checklist.",
		"publicationRecommendation":"ready"
	}}`
}

func sourceOnlyEvidencePacketBody(contentID, sourceID, recommendation string) string {
	return `{"contentId":"` + contentID + `","packet":{
		"humanBrief":"Explain the decision clearly for teams comparing implementation options.",
		"searchIntent":"Practical implementation comparison",
		"thesis":"A source-based thesis that still needs project-owned evidence.",
		"productFacts":[],
		"subjectMatterNotes":[],
		"firsthandObservations":[],
		"sourceIds":["` + sourceID + `"],
		"customerEvidence":[],
		"measurements":[],
		"allowedClaims":[],
		"prohibitedClaims":["Guaranteed search performance."],
		"limitations":["No project-owned evidence has been supplied."],
		"requiredInternalLinks":["/guides/editorial-review"],
		"callToAction":"Collect project-owned evidence before drafting.",
		"publicationRecommendation":"` + recommendation + `"
	}}`
}
