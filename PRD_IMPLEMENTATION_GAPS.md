# PRD Implementation Gap Review

| Field | Value |
|---|---|
| Status | Active implementation plan |
| Based on | `PRD.md` v1.16 |
| Review date | 2026-08-01 |
| Scope | Fully implemented, partially implemented and not-started committed requirements |

## 1. Assessment

The repository has moved beyond the earlier checkpoint recorded in `PRD.md`. Fiber v3, Redis cache-aside reads, media/B2 processing, AI jobs, preview tokens, webhook delivery/replay, production release automation, versioned article autosave, a TipTap visual-editor foundation, optional publication metadata inputs and a bounded operational-observability slice now exist.

The product is not yet PRD-complete. The PRD defines unmarked `shall` requirements as mandatory and describes one committed delivery, so the remaining items below are delivery work rather than optional follow-up phases.

The product language is fixed to English, and language selection is not part of the committed product scope.

Status definitions used below:

- **Fully implemented:** the named bounded task is present and verified; this does not imply its broader product area is complete.
- **Partially implemented:** material behavior exists, but committed requirements remain in that area.
- **Not started:** no material checked-in implementation evidence was found in this review.

## 2. Not-started development

### 2.1 Dedicated editorial templates

- Dedicated briefs, recommended structures and checklists for `standard`, `guide`, `tutorial` and `comparison`.
- Article-type-specific AI prompts and evidence requirements.

### 2.3 Advanced taxonomy and discovery

- SQLite FTS5-backed project-scoped editorial search.
- Taxonomy aliases, controlled merges and taxonomy quality reports.
- Manual related-article and pillar/cluster relationship management.

### 2.4 Consumer integration data and external integrations

- Link checking for broken links, redirect chains and loops.
- A guarded, audited domain-configuration migration workflow.
- Reference landing integration and contract tests for HTML, JSON-LD, feed, sitemap and redirect rendering.

### 2.5 Content lifecycle automation

- Review-due and content-expiry reminders.
- Stale-content reporting and advanced correction review.
- Defined retention periods and guarded hard deletion.
- Provider-side editorial analytics and landing diagnostics.

### 2.6 HTTP mutation idempotency

- `Idempotency-Key` handling for publish and other retryable admin mutations.
- Documented API compatibility and deprecation policy.

### 2.7 Backup, disaster recovery and operations

- Declarative infrastructure for networking, Redis, B2, monitoring, backup services and secret references.

## 3. Fully implemented tasks

### 3.1 HTTP and contract alignment

- Fiber v3 and Huma are the active backend HTTP stack.
- OpenAPI generation and the server-only TypeScript content client are checked in and validated by the repository build flow.

### 3.2 Password and exact-revision approval security

- Password creation/reset paths enforce the PRD 15-character minimum.
- Non-owners cannot approve a revision they created.
- Project owners can self-approve only through explicit owner-controlled solo-owner mode.
- Approval decisions and audit events preserve self-approval state and the approved exact-revision content hash.

### 3.3 Native article/revision attribution slice

- Article creation and revision editing expose one reusable multi-role contributor editor for primary authors, ordered co-authors, editors, expert reviewers, photographers and other credits.
- Revision contributor inputs remain project-scoped, ordered and immutable after snapshotting; unchanged revision attribution is inherited without refreshing historical profiles.
- Approval now rejects an exact revision unless it contains exactly one accountable primary-author record with a complete immutable public snapshot.
- Public Content API bylines/contributor credits and author filtering use the approved revision snapshots.
- Cross-project copies require an explicit, complete mapping from every credited source author to an active destination-project author; roles and positions are retained while new immutable destination snapshots are created.

### 3.4 Article autosave and conflict-recovery slice

- A migration-backed autosave store persists a structured working draft per project, article and user without mutating immutable revisions.
- PUT writes require the exact base revision and expected autosave version; stale bases and another-tab versions return workflow conflicts rather than overwriting newer work.
- The article editor keeps its fast browser backup, restores the newest valid browser/server draft, exposes server save status, preserves stale work for manual reconciliation and can reload a newer another-tab draft.
- Autosaves remain project/reference scoped, are CSRF protected, have a bounded payload, and are cleared transactionally only for the user who creates the next immutable revision.
- Fiber routes, Huma/OpenAPI contracts and recovery/version regression tests cover the completed slice.

### 3.5 TipTap structured-editor foundation slice

- Article creation and revision editing now share a TipTap/ProseMirror visual editor instead of raw body-HTML textareas.
- The implemented surface covers paragraphs, H2–H4, bold/italic/underline/strike/code, safe links, ordered and unordered lists, quotes, code blocks, dividers, tables, figures/images and undo/redo.
- H1 remains unavailable; inserted links and images enforce the server URL policy, and the image flow requires alt text or an explicit decorative choice.
- Heading IDs are carried in editor JSON and HTML, generated collision-free when absent, and preserved during normal heading-text edits.
- The editor emits `tiptap-v1` JSON and matching HTML into immutable revision creation, server autosave and version-migrated browser recovery; renderer regression coverage verifies the resulting document/HTML contract.
- Specialized semantic blocks are supported end to end: task lists, attributed quotes, comparison tables, galleries, transcripts, related references and allowlisted embed references, with safe rendering and recovery parsing.
- Revision comparison exposes an accessible structured diff with changed-field summaries and a changed-only view.

### 3.6 Consumer-owned SEO boundary slice

- Published Content API responses expose approved article, author, taxonomy, media and optional SEO/social fields for downstream renderers.
- The provider does not generate `BlogPosting`, `NewsArticle`, `BreadcrumbList` or any other schema.org/JSON-LD graph.
- Each consuming project validates the optional canonical, robots and social inputs against its own routes and owns final HTML metadata, JSON-LD, sitemap/feed and redirect output.
- Store and client contract tests verify that SEO inputs remain available while generated JSON-LD is absent.

### 3.7 Landing integration data slice

- Published article responses expose primary-category ancestry so a landing project can generate its own breadcrumbs.
- Approved media snapshots expose a typed hero-media contract with safe URLs, explicit dimensions, accessibility metadata and only complete responsive variants; the Article image list also consumes these safe approved inputs.
- Public responses populate ordered series position plus published previous/next navigation, compact ordered related-article links and separate pillar/cluster topic relationships without recursively embedding article bodies.
- Preview responses use the same media and relationship shape while retaining their existing non-indexable policy.
- Store-unit and SQLite integration tests cover breadcrumb safety, responsive media filtering, series navigation and relationship output, and the server-only TypeScript client exposes the completed contract.

### 3.8 Backup and restore automation slice

- A pinned Litestream configuration continuously replicates SQLite to a dedicated B2 prefix with loopback metrics, a guarded control socket and remote deletion disabled in favor of reviewed provider lifecycle rules.
- Daily, monthly and pre-migration jobs synchronously advance the replica, restore into isolation, validate SQLite, upload separately prefixed SSE-B2/Object-Locked snapshots, download them with a distinct restore credential and record checksum/integrity evidence.
- Production deployments require the verified recovery-point command by default and cannot use the skip flag while the gate is enabled.
- A guarded restore tool supports immutable-snapshot and point-in-time continuous restores, requires recorded authorization, preserves an existing primary in a recoverable directory and leaves services stopped for operator validation.
- Systemd service/timer definitions, setup integration, CI contract checks and a clean-host/primary recovery runbook cover operation and rehearsal of the completed automation.

### 3.9 Operational observability and recovery slice

- The API and worker expose private Prometheus-compatible metrics with bounded labels for HTTP rate/errors/latency, cache outcomes, SQLite pressure/size, worker cycles and durable outbox, webhook, media, AI and email state.
- Structured Go logs carry service/environment plus safe request, actor/project, route, outcome, duration and error-category context without logging request bodies, credentials or tokens.
- A pinned Grafana Alloy configuration collects the loopback application/Litestream targets, embedded host and Redis exporters, selected PM2/system logs and credential-free runtime evidence for required PM2 processes, saved state, backups and releases.
- Checked-in managed alert rules cover availability, 5xx/auth/latency, the exact two-minute publication-outbox threshold, webhook/worker failures, SQLite/Redis pressure, provider failures/spend, PM2 state, disk/inodes, backup RPO and failed releases.
- A checked-in operations dashboard, setup integration, CI contract checks and operator runbooks cover deployment, alert testing and every incident class required by PRD Section 12.12.

## 4. Partially implemented areas

### 4.1 Projects and domains

Implemented: project CRUD, globally unique project slugs, status changes, dependency checks, project settings, memberships and project selection.

Remaining: guarded primary/alias/staging domain configuration changes, complete publisher/default-media settings and safe domain migration.

### 4.2 Authors and contributors

Implemented: public author profile CRUD, optional login-account links, ordered project-scoped contributor inputs, a reusable multi-role contributor editor on article creation and revision editing, immutable inherited revision snapshots, public byline/contributor population, author filtering, approval-hash binding, and approval-time enforcement of exactly one accountable primary author.

Remaining: none in this bounded attribution slice.

### 4.3 Structured editing and autosave

Implemented: a shared TipTap visual surface, common semantic blocks and formatting, structured `tiptap-v1` JSON plus HTML emission, allowlist sanitization, derived HTML/Markdown/plain text/table of contents, persistent heading IDs during normal edits, collision-checked explicit anchor editing, project-scoped ready-image and citation pickers, callout/takeaway/steps/pros-and-cons/CTA/FAQ blocks, task lists, attributed quotes, comparison tables, galleries, transcripts, related references, allowlisted embed references, accessible revision comparison, version-migrated local recovery, versioned server autosave, stale-base detection and explicit another-tab conflict recovery.

Remaining: none in this bounded structured-editor and autosave slice.

### 4.4 Review and approval

Implemented: submission, change requests, comments, assignments, notifications, exact-revision decisions, quality gates, publishing, scheduling and rollback. Exact-revision creator checks now prohibit self-approval for non-owners, require explicit owner-only solo-owner opt-in for owner self-approval, and persist the self-approval flag and approved content hash in decision and audit records.

Remaining: cover every public field in approval, record complete approver/publisher/change-note data, mentions and end-to-end propagation status.

### 4.5 Taxonomy and series

Implemented: three-level categories, tags, primary-category enforcement, hierarchy responses, category redirects, series creation and public series routes.

Remaining: secondary categories/tags in article editing, ordered series membership, relationship editing, related-content population, merge/alias operations and quality reports.

### 4.6 Sources and trust

Implemented: source CRUD, revision claims, verification, disclosures, corrections, approval blocking, immutable source/claim snapshots and public JSON exposure.

Remaining: inline citation/footnote editing, source-health checks, archived evidence handling, automated claim extraction and richer correction tooling.

### 4.7 Media and B2

Implemented: signed uploads, MIME/extension/pixel/frame validation, SVG rejection, processing jobs, standard image variants and reference-aware deletion.

Remaining: metadata/focal-point editing, article and hero attachment, transcript support and production malware scanning.

### 4.8 AI subsystem

Implemented: voice profiles, evidence packets, immutable input snapshots, outline/draft/quality jobs, worker retries, cancellation, provenance and token/cost recording.

Remaining: rewrite/critique/metadata tasks, multiple-provider routing and fallback, quotas and enforced budgets, evaluation/canary workflows, automatic source extraction and safe proposal-to-draft application.

### 4.9 Consumer metadata inputs and Content API

Implemented: protected versioned routes, cursor pagination, allowlisted filters, previews, validators, redirects, feed data, discovery manifest, changes cursor, optional SEO/social inputs, typed responsive hero media, ordered series navigation, populated related/topic relationship links, publisher cache invalidation, OpenAPI and a server-only TypeScript client. JSON-LD generation is intentionally excluded and belongs to consuming projects.

Remaining: publisher logo/verified identity inputs, optional crawler-policy inputs, thin-archive hints and reference consumer-renderer tests.

### 4.10 Redis and delivery

Implemented: generation-scoped cache-aside reads, negative caching, TTL jitter, singleflight and SQLite fallback.

Remaining: immutable-body/mutable-pointer separation, monotonic pointer writes, cache warming and explicit invalidation delivery.

### 4.11 Webhooks and email

Implemented: HMAC signing, destination validation, retries, attempt visibility, replay and event fan-out.

Remaining: bounce monitoring and broader operational notifications.

### 4.12 Release and audit operations

Implemented: CI, checksummed release artifacts, PM2, Nginx, deployment locking, migrations, health checks, application rollback and project audit views.

Remaining: reboot rehearsals, database-compatible rollback evidence and failure-injection tests. Backup gates, isolated restoration, clean-host recovery, structured application logs, operational metrics, alerts and incident runbooks are implemented.

## 5. Implementation order

1. **Approval and password security**
   - Enforce creator/self-approval restrictions.
   - Add an explicit owner-only solo-owner approval setting.
   - Persist and audit self-approval decisions.
   - Enforce the 15-character password minimum.
2. **Author attribution**
   - Completed: ordered contributor inputs, immutable snapshots, native article/revision editing, public bylines and approval-time primary-author enforcement.
   - Completed: explicit cross-project copy contributor remapping with destination-owned snapshots.
3. **Structured editor and autosave**
   - Completed: replace body HTML textareas with the shared TipTap visual-editor foundation.
   - Completed: server-backed structured drafts, browser fallback and optimistic conflict recovery.
   - Completed: callout, takeaway, steps, pros/cons, CTA and FAQ blocks; explicit collision-checked anchor controls; project media and citation pickers.
   - Completed: task lists, attributed quotes, comparison tables, galleries, transcripts, related references, allowlisted embeds and accessible structured diff.
4. **Consumer-owned SEO contract**
   - Completed: preserve optional SEO/social inputs without rendering crawler-facing output.
   - Completed: expose author, taxonomy, media, series and relationship data needed by consumer-owned renderers; generated JSON-LD is intentionally absent.
5. **Backup and restore**
   - Completed: make verified recovery points and restore automation production gates.
6. **Observability and runbooks**
   - Completed: delivery metrics, managed alert/dashboard definitions and operational recovery documentation.
7. **Dedicated editorial templates**
   - Remaining: add dedicated briefs, recommended structures, checklists, AI prompts and evidence requirements for `standard`, `guide`, `tutorial` and `comparison`.

## 6. Current implementation progress

- [x] Gap review recorded in a standalone PRD implementation document.
- [x] Approval and solo-owner enforcement.
- [x] Password-policy alignment.
- [x] Native article/revision author attribution, multi-role editor, public bylines and approval gate.
- [x] Server-backed article autosave, browser fallback and conflict recovery.
- [x] TipTap visual-editor foundation with structured draft preservation and stable IDs during normal heading edits.
- [x] Optional SEO/social inputs with immutable bylines and publisher cache invalidation; no provider-generated JSON-LD.
- [x] Cross-project copy contributor remapping.
- [x] Callout/takeaway/steps/pros-cons/CTA/FAQ blocks, explicit anchor controls and project media/citation pickers.
- [x] Remaining specialized semantic blocks and accessible structured diff.
- [x] Consumer-owned SEO boundary reflected in the published contract.
- [x] Backup and restore automation.
- [x] Observability, alerts and runbooks.

## 7. Verification baseline

At the start of this implementation pass:

- All Go tests passed.
- Nuxt type checking passed.
- Nuxt production build passed.
- The TypeScript content-client build passed.

## 8. Current verification

After enforcing the consumer-owned SEO boundary:

- Store tests pass for published article/entity data, dates, bylines, taxonomy, publisher inputs, safe media URLs and advisory SEO/social fields without generated schema.org output.
- Public HTTP regression coverage passes for immutable author/entity data and publisher-setting content-generation invalidation.
- The TypeScript content-client build passes with typed advisory SEO inputs and no generated JSON-LD contract.
- The full Go test suite passes, including the loopback SMTP integration tests when run outside the restricted workspace sandbox.
- `git diff --check` reports no whitespace errors.

After the backup and restore automation pass:

- Backup shell syntax and policy-contract checks pass, including a behavioral guarded restore against a temporary SQLite database.
- The full Go test suite passes.
- `git diff --check` reports no whitespace errors.

After the operational observability and recovery pass:

- Focused Go tests pass for bounded Prometheus exposition, durable delivery gauges and label-cardinality controls.
- The observability contract test passes for the runtime evidence exporter, dashboard JSON, loopback scrape policy, protected credential references and required alert definitions.
- Shell syntax checks pass for VPS provisioning and observability automation.
- The full Go test suite passes.
- `git diff --check` reports no whitespace errors.

After workspace removal:

- Projects are the sole tenant boundary, project slugs are globally unique and project creation atomically creates the owner membership.
- Workspace persistence, migrations, API routes, OpenAPI paths, client methods and Nuxt administration UI are removed.
- Project-membership isolation regression coverage and the full Go test suite pass against the simplified fresh schema.
- Nuxt type checking, the Nuxt production build and the TypeScript content-client build pass.
