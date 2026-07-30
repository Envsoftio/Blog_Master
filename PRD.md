# Product Requirements Document: Multi-Project SEO and LLM Blog Content Provider

| Field | Value |
|---|---|
| Status | Draft v1.9, ready for implementation alignment review |
| Version | 1.9 |
| Date | 2026-07-30 |
| Product | Headless blog CMS and versioned JSON content API |
| Primary users | Workspace owner, project owner/administrator, editor, reviewer, writer, landing-page developer |
| Direct consumers | Landing-project build, SSR, ISR and backend services |
| Indirect consumers | Visitors, search crawlers and answer engines through landing-project HTML |
| Object storage decision | Backblaze B2 Cloud Storage for media and SQLite backup data |
| Revision note | Aligned repository, command, Docker Compose and OpenAPI contract conventions with the checked-in monorepo; added implementation drift controls for Fiber, migrations and generated client artifacts |

## 1. Executive summary

This product is a dedicated, headless, multi-project blog content provider for managing the blogs of many landing-page projects from one secure backend. Human writers and administrators can research, draft, review, approve, schedule, publish, update and retire articles. AI assists with research, outlining, drafting, editing and quality checks, but it cannot approve or publish content.

Each landing-page project receives one or more project-scoped API keys. The project uses a key only from its server, build process, SSR or ISR layer to request published blog data as versioned JSON from the Go API. Landing projects never receive database credentials and never issue SQL against SQLite or libSQL. Public visitors and crawlers receive complete HTML from the landing project’s own canonical domain.

### Product terminology

- **Workspace:** the optional top-level account/organization that owns projects and global operators.
- **Project:** the tenant and authorization boundary inside this product. In MVP, one project normally represents one landing-page website, brand and canonical blog namespace.
- **Landing application:** the independently deployed external application that consumes a project’s published JSON and renders public pages.
- **Site/domain:** the public website and verified production/staging domains configured on a project; it is not a separate CMS tenant.
- **Project membership:** the many-to-many assignment connecting a human user to a project with a project-scoped role.
- **Project API key:** a project-owned service credential used by a trusted landing build, server, SSR or ISR process. It is not a human login or a database credential.

Implementation tables, cache keys, events and authorization checks shall use `project_id` for the tenant boundary. References to the public “site” mean the landing website/domain belonging to that project.

Every Article belongs to exactly one Project. There are no workspace-owned, unscoped or multi-project Article records. An Article’s `project_id` is required and immutable from creation through deletion.

### Hard product boundary

The platform is an editorial admin plus JSON content API. It does **not** host public blog pages.

The product itself has no public landing or marketing page. Its web entry point leads unauthenticated users to sign in and authenticated users to the administration application. All visitor-facing pages remain the responsibility of independently deployed landing applications.

The provider owns:

- Authentication, project isolation, project memberships and project API keys.
- Editorial content, revisions, authors, media metadata, sources and approvals.
- AI assistance and quality controls.
- Published JSON contracts, ETags, Redis caching and change events.
- JSON discovery and redirect manifests needed by landing projects.

Each landing project owns:

- Public URL routing and HTTP status codes.
- HTML rendering and page layout.
- Canonical/link/meta tags and JSON-LD injection.
- `robots.txt`, XML sitemap, RSS/Atom and optional `llms.txt`.
- Redirect execution.
- Its public CDN/page cache.
- IndexNow submission after its changed route is live.
- Visitor analytics, comments, newsletter capture and other site experiences.

Except for the Nuxt administration application, media delivery and health/documentation endpoints, content-provider responses are JSON. The Go API may include sanitized HTML as a JSON string and structured editor blocks as JSON, but it does not return a complete public blog webpage.

The recommended product architecture is:

- Go 1.25 or newer with Fiber v3 and Huma for the API.
- Nuxt for the administration application and rich editorial experience.
- SQLite in WAL mode, accessed through `database/sql` and generated `sqlc` queries.
- Redis for cache-aside article caching, request throttling and short-lived coordination.
- A durable SQLite transactional outbox for cache invalidation, webhooks and background delivery.
- Backblaze B2 Cloud Storage for media originals/derivatives and SQLite backups, using separate buckets and bucket-scoped application keys through its S3-compatible API.
- A CDN/reverse proxy in front of the provider API and media; each landing project operates its own page CDN.
- Provider-independent AI integrations with human review, evidence grounding and full internal provenance.
- Docker Compose for a reproducible local stack, including Nginx, Nuxt SSR, Go API, Go worker, Redis and local email capture.
- Nginx at the production origin, with PM2 supervising the Nuxt Nitro SSR server and the native Go API and worker binaries.

This is intentionally a modular monolith rather than a microservice system. It lives in one monorepo, but monorepo does not mean one process: shared Go domain code produces separate API and worker binaries, while Nuxt produces an independent Nitro SSR artifact. The three application processes share one release version and can be started locally or released together through one root command. The design should be straightforward to deploy and operate while preserving boundaries that allow Redis, the database, AI providers, email and object storage to be changed later.

## 2. Direct answers to the discovery questions

### 2.1 Should every landing page connect with an API key and query the database?

Partly. Every landing-page **project** should have one or more API keys, but each key must call a versioned Go Content API rather than connect directly to the database.

The correct path is:

```text
Landing project build/SSR/ISR
        |
        | Authorization: Bearer <project-scoped-api-key>
        v
Go Content API
        |
        | enforced project scope, published-only policy and cache
        v
Redis -> SQLite
```

Direct database access from landing projects is rejected because it would:

- Distribute database credentials across every project.
- Allow accidental access to drafts, users, prompts, audit logs and other projects.
- Bypass publication status, approval, cache invalidation and rate limits.
- Couple every landing page to internal tables and migrations.
- Make key revocation, audit, API versioning and response compatibility difficult.
- Prevent the backend from enforcing canonical URLs, locales and published revision selection.

A project may issue multiple named API keys per environment and consumer, for example `project-a-production-build`, `project-a-production-runtime` and `project-a-staging`. Every key maps to exactly one `project_id` and a published-read scope set. The backend derives the project from the key; it does not trust a client-supplied tenant identifier. Multiple keys permit independent revocation, usage tracking and overlapping rotation without interrupting every consumer.

The API key is secret only when used server-to-server. It must remain in private environment configuration and must never be embedded in browser JavaScript, HTML, `localStorage` or a `NUXT_PUBLIC_*` setting. A browser cannot keep an API key secret. A browser-facing public Content API is outside the initial scope; a landing project that needs runtime retrieval shall proxy it through its own server.

For SEO, the preferred integration is build-time generation, SSR or ISR. The provider returns JSON; the landing project turns it into article HTML, title, canonical URL, structured data and metadata in its initial page response. Crawlers must not need the provider API key, a cookie or client-side JavaScript to read an article.

Representative article response:

```json
{
  "data": {
    "id": "post_123",
    "articleType": "guide",
    "slug": "example-post",
    "locale": "en",
    "revision": 7,
    "title": "Example title",
    "deck": "A concise description.",
    "excerpt": "Summary used by listing pages.",
    "content": {
      "format": "tiptap-json",
      "document": {},
      "html": "<p>Sanitized article HTML.</p>",
      "tableOfContents": []
    },
    "authors": [],
    "editors": [],
    "reviewers": [],
    "taxonomy": {
      "primaryCategory": null,
      "categories": [],
      "tags": [],
      "series": null,
      "topics": []
    },
    "media": {
      "hero": null
    },
    "seo": {
      "title": "Example title",
      "description": "Search description",
      "canonicalUrl": "https://project.example/blog/example-post",
      "index": true,
      "openGraph": {},
      "structuredData": {}
    },
    "trust": {
      "sources": [],
      "disclosures": [],
      "corrections": []
    },
    "relatedArticles": [],
    "publishedAt": "2026-07-28T10:00:00Z",
    "modifiedAt": "2026-07-28T10:00:00Z"
  },
  "meta": {
    "projectId": "project_abc",
    "contentGeneration": 42,
    "etag": "revision-content-hash"
  }
}
```

### 2.2 How should Redis cache published, added and updated articles?

Redis will be a disposable cache, never the source of truth. SQLite remains authoritative.

The cache uses immutable revision bodies, versioned slug pointers and generation-based collection keys:

```text
blog:v1:body:{projectID}:{contentID}:{revisionID}
blog:v1:slug:{projectID}:{locale}:{slug}
blog:v1:list:{projectID}:{contentGeneration}:{normalizedQueryHash}
blog:v1:negative:{projectID}:{locale}:{slug}
```

Required behavior:

- A body key represents one immutable approved revision and can use a long TTL.
- A slug key points to the currently published content, revision and monotonic publication version.
- List, category, tag, author, related-content, redirect and discovery-manifest keys include the project’s content generation.
- Publishing, materially updating, unpublishing, restoring or changing a slug increments the project generation.
- Old generation keys become unreachable and expire; the application must never use Redis `KEYS` or an unbounded wildcard deletion.
- Slug pointer writes use a version guard so a slow request cannot restore an older revision after a publish.
- Cache TTLs include jitter to prevent many keys expiring at once.
- A missing post may be negatively cached for only 30–60 seconds.
- Go `singleflight` prevents multiple requests in one process rebuilding the same missing key.
- If Redis fails or times out, reads fall back to SQLite and publishing continues.
- Draft, preview, admin and AI responses are never stored in the published-content namespace.

Initial TTL policy, to be tuned from metrics:

```text
immutable published revision JSON    1–7 days plus about 10% jitter
slug/current-revision pointer        15–60 minutes plus explicit update
lists/taxonomy/discovery/feed data   5–15 minutes plus generation change
negative not-found result            30–60 seconds
```

The cached value is the final approved JSON envelope, including ETag/content hash, not a public webpage and not a database/editor object.

Publishing is handled with a transactional outbox:

1. In a SQLite transaction, select the approved revision, update the published revision pointer, increment publication version and content generation, and insert an outbox event.
2. Commit the transaction.
3. Perform best-effort immediate Redis pointer updates and warming.
4. The durable worker retries Redis invalidation/warming and landing-page revalidation.
5. The landing project acknowledges when the canonical route is live, then refreshes its own sitemap/RSS and submits relevant URL changes to IndexNow.

Redis Pub/Sub may be used as a fast notification, but not as the reliable event source because it provides at-most-once delivery. The durable outbox is responsible for retries, ordering and idempotency.

### 2.3 What is the complete blog structure?

The system separates project-owned stable article identity, immutable revisions and project publication:

```text
content_items
    Stable identity, content type and owning project

content_revisions
    Immutable body, metadata, sources and derived output

project_publications
    Project slug, canonical URL, state and published revision
```

This separation allows:

- A published revision to remain live while a new draft is being edited.
- Approval to be bound to an exact revision.
- Project-specific slugs, SEO metadata and canonical rules.
- Explicit cross-project copy/adaptation without sharing one mutable tenant record.
- Rollback to an earlier approved revision.
- Complete audit and correction history.

Every Article belongs to exactly one Project. Cross-project reuse creates an independently reviewed destination copy/fork and requires a canonical-original or material-adaptation decision. Detailed fields and page structure appear in Sections 8 and 9.

### 2.4 How will AI-assisted writing feel genuinely human?

The platform will not build an “AI detector evasion” or superficial “humanizer” feature. The product goal is authentic, expert-led writing with real human accountability.

The governing rule is:

> No unique input, no publication.

AI output becomes credible when it starts from a human brief, owned brand voice, primary sources, real product details, firsthand evidence, subject-matter review and meaningful editing. AI must not invent personal experience, customer stories, tests, quotes, statistics, credentials or citations.

Every AI-assisted publication requires:

- A named accountable human author or organizational author.
- A human-approved brief and angle.
- An evidence packet for factual content.
- Claim and source checks.
- A human editorial pass.
- Approval of the exact immutable revision.
- No unresolved placeholders or critical quality failures.

AI detector scores are not a publication gate. Quality is measured through factual accuracy, source validity, originality, specificity, usefulness, brand voice and human review.

### 2.5 How is the platform designed for SEO and LLMs?

The provider supplies the structured JSON needed for the landing project to implement characteristics that make content strong for search and useful for LLM retrieval and citation:

- Stable, crawlable, canonical public HTML.
- Original reporting, data, examples or expertise.
- Clear headings and concise answers.
- Precise claims connected to visible sources.
- Real authorship and reviewer information.
- Honest publication, modification and correction history.
- Useful tables, FAQs, media and transcripts where appropriate.
- Topic clusters, internal links and consistent entities.
- JSON discovery and redirect manifests from which the landing project produces XML sitemaps, RSS/Atom, redirects and IndexNow events.
- Accurate structured data that matches visible content.

There is no special guaranteed “LLM schema.” A landing project may generate `llms.txt` as an optional derivative for systems that choose to use it, but it is not a source of truth and Google currently says it ignores it. Conventional technical SEO, evidence and useful public pages remain the foundation. The JSON provider enables these outputs but does not serve the public crawler-facing files itself.

### 2.6 Why Fiber, and why was it not the initial conservative default?

Fiber is an acceptable production choice for this project. The earlier conservative default of chi was based on chi’s direct `net/http` compatibility, standard request cancellation semantics and broad middleware interoperability, not because Fiber is unsuitable.

Fiber v3 is now released and Huma supports it. Fiber offers a readable Express-like API, a substantial middleware ecosystem and strong performance. This PRD selects Fiber v3 with these constraints:

- Use native Fiber/Huma handlers instead of adapted `net/http` handlers where practical.
- Create explicit timeouts for SQLite, Redis, object storage, email and AI calls.
- Never retain `fiber.Ctx` or request-backed values after the handler returns.
- Run behind Nginx and Cloudflare, or an equivalent edge, for TLS, routing and modern HTTP support.
- Configure trusted proxies explicitly before trusting forwarded client IPs.
- Do not use prefork for this SQLite-backed, I/O-oriented CMS.
- Keep domain logic independent of Fiber so transport changes do not require a product rewrite.

Router benchmarks are not the main performance factor. CDN behavior, Redis, SQLite access, object storage, external AI calls and rendered response size will dominate.

### 2.7 How do projects, assigned users and API keys work?

`Project` is the tenant and security boundary. A project contains its own members, roles, authors, articles, taxonomy, media metadata, voice profile, integrations, API keys, audit history and published JSON namespace.

Human membership is many-to-many:

```text
users 1---* project_memberships *---1 projects
```

- One project may have many writers, editors, reviewers and administrators.
- One user may belong to many projects.
- The same user may hold a different role in each project.
- A workspace owner may administer all projects, but normal project access is deny-by-default and comes from an active membership.
- Public author profiles remain separate from login accounts so disabling a login does not erase historical bylines.

A project API key belongs to the project rather than to an individual user. Each project may create multiple keys for production, staging, build, SSR or rotation. The complete secret is shown once, only a cryptographic verifier is stored, and the key may be named, scoped, expired, revoked and audited independently.

The Content API resolves `project_id` from the credential before reading cache or SQLite. A path, query parameter or request body can narrow a request but can never expand or replace that project scope. Landing keys are published-read only and cannot access drafts, admin records, AI history or write endpoints.

### 2.8 Are Go and Nuxt separate projects or one workspace?

Use one Git monorepo/workspace containing two independent applications:

- `apps/backend`: one Go module producing API, worker and `admincli` binaries.
- `apps/admin`: one Nuxt admin application producing a Nitro Node SSR build for production.
- `contracts/openapi/openapi.yaml` and `packages/content-client`: generated OpenAPI and TypeScript integration artifacts.
- `infra`: Nginx, Docker development, PM2, Litestream, service definitions and infrastructure code.

The product codebase therefore has only two application areas: backend and frontend/admin. The API, worker and administrative CLI are separate executables produced by the same backend application, not separate product applications. External landing applications are API consumers and remain outside this repository.

Go and Nuxt use separate dependency graphs, tests and build outputs. Docker Compose runs them together for development, and one release command deploys their separate artifacts as one compatible release. Landing-page applications stay in separate repositories and consume the Content API or generated client. Section 11 defines the normative repository layout.

### 2.9 Can the whole workspace run and deploy with one command?

Yes, at the operator interface:

```text
task dev
  -> Docker Compose starts local Nginx, Nuxt SSR, Go API, Go worker, Redis and Mailpit

task deploy:prod RELEASE=<immutable-release-id>
  -> deploys the compatible Nuxt and Go artifacts, migrates safely and updates PM2
```

This does not combine Go and Nuxt into one executable. In production, Nginx is the only public origin; PM2 supervises Nuxt Nitro SSR, the Go API and the Go worker as three named processes. Redis and Litestream remain host or managed infrastructure services. One command is a safe orchestration interface over separate components, not a single-process architecture.

## 3. Product problem

Maintaining a separate blog backend, authoring workflow and SEO implementation for every landing-page project produces duplication, inconsistent quality and operational risk. Writers have no unified workflow, AI output may be published without sufficient evidence, and landing integrations can drift in fields, URLs, schema and caching.

The product must provide:

- One editorial system serving many independent project domains.
- Strong project isolation without a complex enterprise identity platform.
- A safe draft, review, approval and publication workflow.
- Fast and reliable delivery of published content.
- Search- and answer-engine-friendly page data.
- AI assistance that improves research and writing without bypassing human responsibility.
- Low operational complexity and a clear scaling path.

## 4. Goals

### 4.1 Product goals

- Manage multiple landing-page blogs from one administration application.
- Allow administrators and writers to create content manually or with AI assistance.
- Preserve a complete immutable revision, review, approval and audit history.
- Deliver only approved published revisions through a stable API.
- Produce versioned JSON data needed by landing projects for canonical public blog pages, archives, feeds, redirects and structured data.
- Make publishing, updating, unpublishing and rollback predictable and recoverable.
- Reduce repeated implementation work across landing projects through OpenAPI and generated SDKs.
- Improve organic discovery, crawlability, sourceworthiness and LLM citation potential.
- Start with an affordable SQLite-centered deployment without preventing a later database migration.

### 4.2 Engineering goals

- Modular monolith with explicit domain boundaries.
- Secure-by-default human sessions and project-scoped API keys.
- Cache-aside Redis with durable invalidation.
- Idempotent background jobs and webhooks.
- Testable authorization and tenant isolation.
- Observable publishing, caching, AI and delivery operations.
- Automated backups with tested restoration.

## 5. Non-goals

The initial product will not:

- Provide a public landing or marketing page for the CMS itself.
- Provide a separate internal folder system for articles; projects, hierarchical categories, workflow state, assignments and search provide the MVP organization model.
- Provide landing projects with direct SQL or database credentials.
- Host or render public blog HTML pages.
- Serve crawler-facing XML sitemap, RSS/Atom, `robots.txt` or `llms.txt` files.
- Execute visitor-facing redirects; it returns a redirect manifest for each landing project.
- Own visitor comments, newsletter subscription capture or landing-page analytics.
- Expose a secret-bearing Content API directly to browser JavaScript.
- Be a general-purpose page builder for complete marketing websites.
- Automatically publish AI-generated content without human approval.
- Promise search rankings, indexing or LLM citations.
- Optimize against AI-detection tools.
- Replace a customer data platform, email marketing suite or full analytics warehouse.
- Operate visitor memberships, paywalls, advertising inventory or monetization.
- Provide vector/semantic search, automated content-performance optimization or real-time collaborative editing in the committed delivery.
- Provide public themes or a customizable landing-page/page-builder runtime.
- Use microservices solely for organizational separation.
- Support active-active SQLite writers across several regions.
- Require social login, passwordless login or enterprise SSO in the first release.

### 5.1 Requirement priority convention

Unless explicitly marked optional or an uncommitted future consideration, every requirement using **shall** is a **MUST** for the single committed delivery.

- **MUST:** required to accept the single committed delivery.
- **SHOULD:** expected unless implementation evidence shows disproportionate cost or risk; any omission requires a recorded decision.
- **FUTURE:** not part of the committed delivery and not assigned to a second delivery phase.

Section 19 groups implementation work without creating separate releases. Security, tenant isolation, immutable approval, backup/restore, published-content correctness and “AI cannot publish” are delivery gates and cannot be deferred.

## 6. Users, roles and permissions

### 6.1 Personas

**Workspace owner**

Creates the workspace, bootstraps projects and manages workspace-level operators, security policy and destructive operations. A workspace role does not silently grant editorial access unless the documented owner policy allows it.

**Project owner**

Owns one project and manages its settings, memberships, API keys, integrations and destructive operations. Every active project must retain at least one owner.

**Project administrator**

Manages the assigned project’s taxonomy, authors, media, content, AI configuration, review, scheduling and publishing within granted permissions.

**Editor or reviewer**

Editors and reviewers are distinct project roles in the committed delivery. They review assigned content, comment, request changes, verify claims and approve exact revisions according to their permissions.

**Subject-matter reviewer**

Reviews assigned factual claims and evidence for a defined expertise area. This role may approve factual review but cannot manage users, keys or project configuration.

**Writer**

Creates and edits drafts, uploads media, uses approved AI tools, responds to comments and submits revisions for review. A writer cannot approve or publish.

**Landing-page developer**

Securely installs and consumes project API keys, implements canonical blog routes and configures revalidation receivers. Project owners or administrators create and rotate keys; a developer receives key-management access only through an explicit project role.

**Viewer or analyst**

Has read-only access to published content, workflow status and approved analytics for assigned projects. This is an uncommitted future consideration.

**Visitor or crawler**

Reads complete public pages from the landing project’s domain without authenticating to the CMS.

### 6.2 Permission model

Permissions are project-scoped and deny by default. Proposed permission names include:

```text
content.create
content.edit_own
content.edit_all
content.review
content.approve
content.publish
content.unpublish
content.archive
taxonomy.manage
media.manage
authors.manage
ai.use
ai.configure
project.manage
members.manage
api_keys.manage
audit.read
```

Default role mapping:

| Project role | Default capabilities |
|---|---|
| `project_owner` | All project capabilities, ownership transfer, API keys and destructive actions |
| `project_admin` | Project configuration, members, API keys, content, approval and publishing except ownership transfer |
| `editor` | Edit all content, coordinate workflow, review, approve and publish; no members, keys or destructive project actions |
| `reviewer` | Read drafts, comment, verify claims and approve; no publishing or project administration |
| `writer` | Create and edit permitted drafts, upload media, use approved AI tools and submit for review |
| `viewer` (future) | Read-only access to allowed project records and reports |

Authorization is checked in backend service methods, not only in routes or UI components. Every resource lookup includes the authorized `project_id`. Cross-project negative tests are mandatory.

A user may hold different roles on different projects. Committed project roles are `project_owner`, `project_admin`, `editor`, `reviewer` and `writer`; `viewer` is an uncommitted future consideration. Article assignment is separate from project membership. No global role is trusted from a cookie, browser request or project API key. If support impersonation is ever introduced, it shall require an explicit reason, short expiry, persistent banner and immutable audit trail.

## 7. Primary user journeys

### 7.1 Bootstrap and project setup

1. An operator creates the first owner using a one-time CLI command or expiring invitation.
2. The owner configures a workspace and creates its first project; project creation atomically creates the owner membership.
3. The owner sets domain, blog base path, locale, timezone, publisher, brand voice and SEO defaults.
4. The owner invites or assigns writers, editors and reviewers with project-specific roles.
5. The owner creates a named production API key and copies it once into the landing project’s secret store.
6. The landing developer validates the connection and configures a signed revalidation endpoint.

No default password or public bootstrap route is permitted.

### 7.2 Manual article workflow

1. A writer creates a content item and brief.
2. The writer creates or edits a draft revision.
3. Autosave preserves working changes without modifying the live revision.
4. The writer submits the revision for review.
5. The reviewer comments, requests changes or approves.
6. An administrator publishes immediately or schedules publication.
7. The JSON API and Redis update through the outbox flow, and the landing project receives a signed revalidation event.

### 7.3 AI-assisted article workflow

1. A human supplies the brief, purpose, audience, unique angle, constraints, evidence and CTA.
2. AI creates an evidence-aware outline with proposed claims and sources.
3. A human approves or changes the outline.
4. AI drafts sections as a new draft revision.
5. Automated checks identify unsupported claims, broken sources, duplication, filler and policy violations.
6. The writer adds original examples and performs an editorial rewrite.
7. A reviewer verifies claims and approves the exact revision.
8. An administrator publishes.

### 7.4 Updating published content

1. A user creates a new draft revision while the current approved revision remains public.
2. The revision includes a material change note and may preserve prior sources.
3. Review and approval run again.
4. Publishing atomically changes the public revision pointer.
5. The provider updates its JSON cache/manifests and notifies the landing project; the landing project updates its route, CDN, sitemap, feed and IndexNow state.

### 7.5 Slug change, unpublish and rollback

- A slug change creates a redirect record in the provider’s JSON redirect manifest; the landing project emits the permanent redirect.
- Unpublishing changes the JSON API and discovery manifest; the landing project returns the configured not-found/gone/replacement behavior and removes the URL from feeds and sitemaps.
- A replacement URL may be configured for a retired article.
- Rollback selects a previously approved revision and creates a new publication event; it does not erase history.

## 8. Functional requirements

### 8.1 Workspace and project management

- **FR-PROJECT-001:** The system shall support an optional workspace grouping and multiple projects within a workspace.
- **FR-PROJECT-002:** Each project shall have a stable internal ID, a unique workspace-scoped slug and a separate non-secret public identifier.
- **FR-PROJECT-003:** Each project shall configure its project name, primary domain, blog base path, locale, timezone and default language.
- **FR-PROJECT-004:** Each project shall configure publisher name, logo, URL and verified external identities.
- **FR-PROJECT-005:** Each project shall configure default SEO title patterns, social images and robots behavior.
- **FR-PROJECT-006:** Each project shall have a voice profile, topic boundaries, approved product facts, terminology and prohibited claims.
- **FR-PROJECT-007:** Each project shall support multiple named staging and production API keys without requiring one shared credential.
- **FR-PROJECT-008:** Staging and preview JSON shall default to `index: false` and shall never enter the production discovery manifest consumed for public sitemaps.
- **FR-PROJECT-009:** Authorized users shall be able to create, list, view, update, suspend, archive and, subject to dependency checks and retention, delete projects.
- **FR-PROJECT-010:** A project is the tenant/security boundary and shall own its articles, revisions, publications, taxonomy, authors, assets, sources, AI jobs, integrations and audit records.
- **FR-PROJECT-011:** In MVP, one project shall normally represent one landing-page website or brand with exactly one verified primary canonical domain and optional verified staging/alias domains.
- **FR-PROJECT-012:** Project creation and the creator’s `project_owner` membership shall commit atomically.
- **FR-PROJECT-013:** Suspending a project shall immediately deny its human sessions within that project, reject its API keys and stop new schedules/webhook deliveries without destroying published history.
- **FR-PROJECT-014:** Project archival or deletion shall produce a dependency report covering active keys, members, publications, schedules, redirects, assets, webhooks and pending jobs.
- **FR-PROJECT-015:** The Nuxt admin shall provide a project selector showing only projects the current user is authorized to access, and every screen shall retain an explicit selected-project context.
- **FR-PROJECT-016:** Workspace grouping shall not permit data access across projects unless an explicit workspace-level operator permission is checked and audited.
- **FR-PROJECT-017:** A project with retained Articles shall not be hard-deleted; Article archival/export/retention requirements must be resolved before project deletion can proceed.

### 8.2 Project memberships and roles

- **FR-MEMBER-001:** A project may have many human members, and a user may have active memberships in many projects.
- **FR-MEMBER-002:** A user may hold a different role in each project.
- **FR-MEMBER-003:** Project roles shall include `project_owner`, `project_admin`, `editor`, `reviewer` and `writer`; a `viewer` role remains an uncommitted future consideration that shall not require changing the membership model.
- **FR-MEMBER-004:** A project owner or authorized administrator shall be able to invite a new user or assign an existing user to a project role.
- **FR-MEMBER-005:** Users shall list and access only projects for which they hold an active membership, except for explicitly authorized and audited workspace operators.
- **FR-MEMBER-006:** Role changes and removals shall take effect immediately, revoke now-invalid access and create audit events.
- **FR-MEMBER-007:** Removing a user from one project shall not change their access to other projects or erase historical authorship, reviews, approvals or audit records.
- **FR-MEMBER-008:** The final active project owner shall not be removed or demoted until ownership is transferred.
- **FR-MEMBER-009:** Article assignment shall require project membership but shall remain separate from the membership record.
- **FR-MEMBER-010:** Writers shall not manage memberships, project settings or API keys.

### 8.3 Authentication and sessions

- **FR-AUTH-001:** Human accounts shall be invite-only; public registration is disabled.
- **FR-AUTH-002:** Passwords shall be stored with Argon2id using versioned parameters and unique salts.
- **FR-AUTH-003:** Authentication shall use opaque, server-side sessions rather than browser JWTs.
- **FR-AUTH-004:** The browser shall receive only a `Secure`, `HttpOnly`, `SameSite` session cookie.
- **FR-AUTH-005:** The backend shall store only a cryptographic hash of the session secret.
- **FR-AUTH-006:** Sessions shall have idle and absolute expiry and be revocable immediately.
- **FR-AUTH-007:** State-changing admin requests shall require session-bound CSRF protection and origin validation.
- **FR-AUTH-008:** Password reset and invitation responses shall not permit account enumeration.
- **FR-AUTH-009:** Invite and reset tokens shall be random, hash-only, single-use and expiring.
- **FR-AUTH-010:** Password changes, resets, user disabling and material permission changes shall revoke relevant sessions.
- **FR-AUTH-011:** Login, reset and invite operations shall be rate-limited by both source and account identity.
- **FR-AUTH-012:** The system shall support optional TOTP MFA for owners and administrators in the committed delivery.
- **FR-AUTH-013:** Single-factor passwords shall permit at least 64 characters and use a minimum length of 15 characters; arbitrary composition rules and scheduled password rotation are prohibited.
- **FR-AUTH-014:** New passwords should be checked against an offline or privacy-preserving breached/common-password list.
- **FR-AUTH-015:** Creating project API keys, changing credentials, transferring ownership and permanently deleting a project shall require recent reauthentication.
- **FR-AUTH-016:** Disabling a user shall immediately revoke their active sessions without deleting their historical authorship, comments, approvals or audit records.

### 8.4 Project API keys

- **FR-TOKEN-001:** Every API key shall be bound to exactly one project and one consumer environment.
- **FR-TOKEN-002:** A project may have multiple simultaneously active named keys per environment and consumer; no uniqueness constraint shall force all consumers to share one key.
- **FR-TOKEN-003:** Keys shall use explicit published-read scopes. The default profile may include `content:published:read`, `taxonomy:published:read`, `authors:published:read`, `discovery:read` and `redirects:read`.
- **FR-TOKEN-004:** A key secret shall be displayed only at creation and stored only as a cryptographic verifier; list responses shall return only prefix and non-secret metadata.
- **FR-TOKEN-005:** Keys shall support expiry, revocation, naming, last-used tracking and overlapping rotation.
- **FR-TOKEN-006:** Keys shall be accepted only in an authorization header over HTTPS.
- **FR-TOKEN-007:** Content keys shall not access drafts, previews, admin data, AI data or write actions.
- **FR-TOKEN-008:** Requests shall be rate-limited per key, project and source where appropriate.
- **FR-TOKEN-009:** Keys and their verifiers shall never appear in cache keys, analytics, logs, URLs or browser bundles.
- **FR-TOKEN-010:** The backend shall derive `project_id` exclusively from the verified key; a client-supplied project identifier shall never broaden or replace that scope.
- **FR-TOKEN-011:** Revoking one key shall not revoke or interrupt other valid keys belonging to the same project.
- **FR-TOKEN-012:** Key creation, rotation and revocation shall require `api_keys.manage`, recent human reauthentication and immutable audit events.
- **FR-TOKEN-013:** Project API keys shall remain in trusted landing server/build secret storage and shall never be placed in Nuxt public runtime configuration, browser JavaScript, HTML or `localStorage`.
- **FR-TOKEN-014:** Deployment environments shall use separate databases and credentials. Within one deployment, a key’s environment is a consumer/safety label; normal keys read that deployment’s approved published content, while draft preview uses a separate short-lived preview credential.

### 8.5 Authors and contributors

- **FR-AUTHOR-001:** A public author profile shall be separate from a login account and may optionally link to one.
- **FR-AUTHOR-002:** Profiles shall include display name, biography, image, credentials, expertise areas, profile URL and verified external identities.
- **FR-AUTHOR-003:** Revisions shall support multiple authors, editors, reviewers, photographers and other contributors.
- **FR-AUTHOR-004:** The system shall preserve the historical byline even if an account is later disabled.
- **FR-AUTHOR-005:** AI shall never be represented as a human author.
- **FR-AUTHOR-006:** An article shall support one ordered primary author, ordered co-authors and separately identified editors and expert reviewers.
- **FR-AUTHOR-007:** Public profiles shall additionally support slug, short and full biography, job title, organization, status and `sameAs` links.
- **FR-AUTHOR-008:** Published revisions shall retain an immutable author/contributor snapshot so later profile changes cannot silently rewrite historical attribution.

### 8.6 Content types and editorial structure

All article types shall use one flexible Article/Revision/Publication schema. A type changes its brief, suggested outline, evidence requirements, AI prompt template, editorial checklist and recommended landing treatment; it shall not create a separate database table.

Supported `article_type` values are:

- `standard`
- `guide`
- `tutorial`
- `comparison`
- `case_study`
- `research`
- `listicle`
- `news_update`
- `opinion`
- `reference`
- `glossary`
- `release_note`

The committed delivery shall provide dedicated briefs, AI prompts, editorial checklists and recommended structures for `standard`, `guide`, `tutorial` and `comparison`. The remaining article types shall use the shared Article schema and a generic configurable workflow until a dedicated type template is introduced.

A tutorial template should request prerequisites, ordered steps, validation and an expected result. A comparison template should require explicit criteria, evidence for claims, limitations and balanced pros/cons. `news_update` shall be used only where the project genuinely publishes time-sensitive news.

Supported body blocks shall include:

- Paragraph.
- H2–H4 heading with stable ID.
- Ordered and unordered list.
- Task/checklist.
- Quote with attribution.
- Inline code and code block with language.
- Accessible responsive table.
- Comparison table.
- Image, gallery and figure.
- Video or audio with transcript.
- Callout.
- Key-takeaways block.
- Step-by-step block.
- Pros-and-cons block.
- Reusable CTA.
- Link.
- Inline citation or footnote.
- FAQ item.
- Related-article reference.
- Embed from an allowlisted provider.
- Divider or section boundary.

- **FR-CONTENT-001:** A content item shall have a stable identity independent of slug and revision.
- **FR-CONTENT-002:** Every saved editorial version intended for review shall be an immutable revision.
- **FR-CONTENT-003:** The editor shall autosave working changes and recover from interruption.
- **FR-CONTENT-004:** The editor shall warn about concurrent edits; real-time multi-user merging is not required in MVP.
- **FR-CONTENT-005:** The canonical editable document shall use a versioned structured format such as TipTap/ProseMirror JSON.
- **FR-CONTENT-006:** Approval shall generate sanitized HTML, plain text, Markdown export, heading table of contents, word count, reading time and a content hash.
- **FR-CONTENT-007:** The renderer shall sanitize all rich content using an explicit allowlist.
- **FR-CONTENT-008:** A published content item may have an unpublished newer draft without changing the public output.
- **FR-CONTENT-009:** Import and export shall preserve content, metadata, media references, authors and redirects in documented formats.
- **FR-CONTENT-010:** The article title is the public page H1; the body schema shall reject an H1 block.
- **FR-CONTENT-011:** Raw HTML blocks shall be disabled by default. Any later exception requires a separately reviewed sanitizer and project policy.
- **FR-CONTENT-012:** Meaningful images shall require alt text; decorative images shall carry an explicit decorative flag and empty alt value.
- **FR-CONTENT-013:** Tables shall support header semantics and a landing rendering contract that remains usable on narrow screens.
- **FR-CONTENT-014:** FAQ questions and answers used in structured-data inputs shall also be visible in the article body.
- **FR-CONTENT-015:** Heading IDs shall remain stable across normal text edits and shall change only through an explicit anchor-edit action with collision checking.
- **FR-CONTENT-016:** The admin shall provide revision history and accessible side-by-side or inline diff for public fields and structured body changes.
- **FR-CONTENT-017:** Every Article shall have exactly one non-null `project_id`, assigned at creation and immutable afterward.
- **FR-CONTENT-018:** Article creation shall occur through an explicitly selected project and require an active membership with `content.create` for that project.
- **FR-CONTENT-019:** Every Article list, lookup, mutation, revision, publication and delete operation shall require an explicit authorized `project_id`; unscoped repository methods such as `GetArticle(id)` are prohibited.
- **FR-CONTENT-020:** Authors, taxonomy terms, media, sources, revisions, approvals, relationships and publications attached to an Article shall belong to the same project. Cross-project references shall fail validation and database constraints.
- **FR-CONTENT-021:** Copying an Article to another project shall create a new destination Article ID and independent revisions/publication state, record its origin and require access to both projects plus a new canonical/adaptation decision.
- **FR-CONTENT-022:** SQLite foreign keys and unique/composite constraints shall prevent a revision or publication under Project B from referencing an Article or revision owned by Project A.

### 8.7 Taxonomy, organization and discovery

- **FR-TAX-001:** MVP shall support project-scoped categories and tags.
- **FR-TAX-002:** Every published post shall have exactly one primary category; it may have secondary categories and multiple tags.
- **FR-TAX-003:** Taxonomy terms shall have project-specific slugs, descriptions, SEO fields and archive indexability.
- **FR-TAX-004:** The system shall support explicit pillar-to-cluster relationships.
- **FR-TAX-005:** The system shall detect likely topic duplication and keyword cannibalization before publication.
- **FR-TAX-006:** SQLite FTS5 shall support admin content search and may support public search for initial scale.
- **FR-TAX-007:** Search results shall respect project, locale, publication state and permission scope.
- **FR-TAX-008:** The system shall support project-scoped series with a stable slug, ordered article positions and pillar/supporting topic-cluster relationships.
- **FR-TAX-009:** Tags and categories shall support aliases and controlled merge operations with redirect/canonical handling where a public archive URL exists.
- **FR-TAX-010:** The admin shall report duplicate, near-duplicate and unused taxonomy terms.
- **FR-TAX-011:** Related articles shall support manual editorial selection in MVP with deterministic taxonomy-based suggestions; embedding-based recommendations are not required.
- **FR-TAX-012:** The system shall report orphaned published articles that have no inbound relationship from another article, category, series or landing navigation source known to the provider.
- **FR-TAX-013:** MVP categories shall support a parent/child hierarchy of no more than three levels, including the root category, so a project can organize articles into categories and subcategories. Each category may have at most one parent, and tags shall remain flat.
- **FR-TAX-014:** A category and its parent shall belong to the same project. Self-parenting, hierarchy cycles and cross-project parent references shall be rejected by service validation and database constraints where practical.
- **FR-TAX-015:** The admin shall provide a category-tree browser and hierarchical category selector, showing the full ancestor path when assigning or moving a category.
- **FR-TAX-016:** Category API responses shall expose parent, ancestor-breadcrumb and direct-child data. Published article queries for a category shall include articles assigned to descendant categories by default, while callers may explicitly request exact-category matches only.
- **FR-TAX-017:** Category slugs shall be unique within a project regardless of hierarchy level so stable `/categories/{slug}` API lookup remains unambiguous.

### 8.8 Review, approval and comments

- **FR-WORKFLOW-001:** Editorial state and publication state shall be stored separately.
- **FR-WORKFLOW-002:** Editorial states shall include draft, in review, changes requested and approved.
- **FR-WORKFLOW-003:** Publication states shall include unpublished, scheduled, published and archived.
- **FR-WORKFLOW-004:** A reviewer shall approve or reject an exact revision and content hash.
- **FR-WORKFLOW-005:** Editing an approved revision shall create a new draft and invalidate approval for the new content.
- **FR-WORKFLOW-006:** Review comments shall support resolution, reopening and links to a revision and, where possible, a document block.
- **FR-WORKFLOW-007:** A writer shall not approve or publish their own submitted work.
- **FR-WORKFLOW-008:** A project owner may self-approve only when the project’s explicit solo-owner mode is enabled. Project administrators and other roles shall not gain self-approval through this mode, and every self-approval shall preserve an explicit audit record.
- **FR-WORKFLOW-009:** Publication shall record approver, publisher, exact revision, timestamps and a change note.
- **FR-WORKFLOW-010:** The system shall display propagation status for Redis, CDN, webhook and landing-site revalidation.
- **FR-WORKFLOW-011:** Content shall support assignment, due dates, reviewer/SME assignment and notifications.
- **FR-WORKFLOW-012:** Comments shall support mentions without exposing users from projects the commenter cannot access.
- **FR-WORKFLOW-013:** Approval shall cover every public field—body, title, byline, sources, taxonomy, media, SEO, social and structured-data inputs. Changing any approved public field requires a new approval-bound revision or publication-metadata revision.
- **FR-WORKFLOW-014:** Reviewers shall be able to compare the submitted revision against its base revision and the currently published revision before approval.

### 8.9 Scheduling and lifecycle

- **FR-LIFE-001:** Publication may be immediate or scheduled in the project’s configured timezone and stored internally in UTC.
- **FR-LIFE-002:** Scheduling shall handle daylight-saving transitions deterministically.
- **FR-LIFE-003:** Scheduled jobs shall be idempotent and safe to retry.
- **FR-LIFE-004:** Posts shall support review-due and content-expiry reminders.
- **FR-LIFE-005:** Material updates shall require a change note and update visible modification data.
- **FR-LIFE-006:** Superficial edits shall not automatically change public `dateModified` or sitemap `lastmod`.
- **FR-LIFE-007:** Unpublish, archive, restore, redirect and delete shall be separate actions.
- **FR-LIFE-008:** Hard deletion shall require elevated permission and a defined retention period.
- **FR-LIFE-009:** A scheduled publication missed during downtime shall be claimed and processed idempotently after restart; schedules shall not rely on in-memory timers.
- **FR-LIFE-010:** Project timezone changes shall not silently reinterpret already-scheduled UTC publication times.
- **FR-LIFE-011:** Rollback shall select a previously approved revision through a new audited publication event and shall never erase intervening history.
- **FR-LIFE-012:** Public corrections shall be append-only notices linked to the affected article/revision, visible in published JSON and preserved across later edits.

### 8.10 Media

- **FR-MEDIA-001:** Media files shall be stored in a dedicated Backblaze B2 media bucket, not as SQLite blobs.
- **FR-MEDIA-002:** Uploads shall validate maximum size, detected MIME type and extension.
- **FR-MEDIA-003:** Images shall store width, height, focal point, alt text, caption, creator credit, license and source type.
- **FR-MEDIA-004:** The system shall generate or accept representative 1:1, 4:3 and 16:9 image variants.
- **FR-MEDIA-005:** The JSON contract shall return responsive variants and explicit dimensions; the landing renderer shall select suitable variants and lazy-load where appropriate.
- **FR-MEDIA-006:** AI-generated or materially AI-edited media shall have internal provenance metadata.
- **FR-MEDIA-007:** Deleting an asset in active use shall be blocked or require an explicit replacement.
- **FR-MEDIA-008:** Video and audio shall support captions or transcripts.
- **FR-MEDIA-009:** Upload validation shall bound decompressed pixel count, animation frames and archive expansion to prevent resource-exhaustion attacks.
- **FR-MEDIA-010:** SVG shall be rejected by default or processed through a dedicated sanitizer before publication.
- **FR-MEDIA-011:** Image processing shall remove EXIF/GPS metadata unless an explicit editorial policy preserves it.
- **FR-MEDIA-012:** Risk-appropriate document and archive uploads shall be malware scanned before becoming downloadable.

### 8.11 SEO and landing-rendering data

The provider returns SEO and discovery data as JSON. The landing project is responsible for converting that contract into public HTML, headers, crawler files and redirects.

- **FR-SEO-001:** Every publication JSON object shall include its intended stable canonical URL on the landing project’s verified domain.
- **FR-SEO-002:** The API shall return title, description, canonical URL, robots intent, social metadata and publication dates.
- **FR-SEO-003:** The API shall return verified inputs for `BlogPosting`/`Article`, `BreadcrumbList`, author profile and organization structured data.
- **FR-SEO-004:** Returned structured-data inputs shall never contain unsupported ratings, FAQs, claims or entities.
- **FR-SEO-005:** The JSON contract shall include canonical, locale, translation-group and hreflang relationships for the landing renderer.
- **FR-SEO-006:** The provider shall return a JSON discovery manifest containing the canonical indexable URLs and material `lastmod` data from which each landing project builds XML sitemap files.
- **FR-SEO-007:** The provider shall return ordered JSON feed data from which a landing project may build RSS and/or Atom.
- **FR-SEO-008:** Discovery-manifest `lastmod` and returned modification dates shall reflect material changes only.
- **FR-SEO-009:** The provider shall return previous-slug redirect records; the landing project shall emit one-hop permanent redirects.
- **FR-SEO-010:** Duplicate publication across project domains shall require either one declared canonical original or a materially distinct adaptation.
- **FR-SEO-011:** The provider shall return project crawler-policy configuration; the landing project owns its public `robots.txt`.
- **FR-SEO-012:** The provider may return optional JSON data for a landing project to produce `llms.txt`, but shall not serve or treat it as a ranking requirement.
- **FR-SEO-013:** The CMS shall notify the landing project of a change; the landing project shall submit IndexNow only after its changed URL is live or intentionally removed.
- **FR-SEO-014:** Preview and staging JSON shall be explicitly flagged non-indexable and shall not enter the production discovery manifest.
- **FR-SEO-015:** The JSON contract shall not include obsolete `meta keywords`.
- **FR-SEO-016:** Domain ownership shall be verified before it is returned as the production canonical domain.
- **FR-SEO-017:** Empty or thin taxonomy/archive entries shall be excluded from the discovery manifest or flagged `index: false`.
- **FR-SEO-018:** A link checker shall detect broken links, redirected internal links, redirect chains and loops and expose results to editors and landing integrations.
- **FR-SEO-019:** The landing integration guide and reference tests shall verify that returned SEO fields are rendered consistently in HTML, JSON-LD, sitemap/feed output and HTTP status/redirect behavior.
- **FR-SEO-020:** The published JSON shall expose explicit Open Graph title, description and image plus primary-image dimensions when supplied and approved.
- **FR-SEO-021:** Revisions may include editorially verified definitions and named-entity references for visible content and internal consistency; the system shall not fabricate entities or hidden LLM-only text.

### 8.12 JSON integration API

- **FR-API-001:** The API shall be versioned under stable route prefixes.
- **FR-API-002:** Huma shall generate OpenAPI 3.1 documentation and schemas.
- **FR-API-003:** List endpoints shall use bounded cursor pagination.
- **FR-API-004:** Filtering shall be allowlisted; arbitrary SQL-like filtering is prohibited.
- **FR-API-005:** Published responses shall include ETag and Last-Modified validators.
- **FR-API-006:** Conditional requests shall return `304 Not Modified` when appropriate.
- **FR-API-007:** Errors shall use a consistent problem-details schema without internal stack traces.
- **FR-API-008:** API changes shall follow a documented deprecation and compatibility policy.
- **FR-API-009:** Publish and other retryable mutations shall accept idempotency keys.
- **FR-API-010:** The project shall generate or publish a small TypeScript client for landing projects.
- **FR-API-011:** Successful Content API responses shall use `application/json` and a documented stable envelope.
- **FR-API-012:** The JSON contract shall expose both versioned structured body blocks and derived sanitized HTML unless a project policy disables one representation.
- **FR-API-013:** The API shall provide a changes cursor so landing projects can incrementally revalidate changed, redirected and removed routes.
- **FR-API-014:** Browser-facing access using a secret project API key is prohibited; landing projects shall call the API from a trusted server/build environment.
- **FR-API-015:** Article listing shall support allowlisted filters for category, tag, author, locale, article type, series when enabled, and publication date bounds with deterministic sorting.
- **FR-API-016:** Category, tag and author detail responses shall include or link to cursor-paginated published articles within the credential’s project.
- **FR-API-017:** The API shall support stable retrieval by slug and by non-secret article ID.
- **FR-API-018:** Related-article results shall identify whether a relationship is manually curated or deterministically suggested.
- **FR-API-019:** API key scope shall be resolved before cache lookup, and the selected project shall be part of every cache/database query even when an identifier is globally unique.

Initial protected integration routes:

```http
GET  /content/v1/posts
GET  /content/v1/posts/{slug}
GET  /content/v1/posts/by-id/{contentID}
GET  /content/v1/posts/{slug}/related
GET  /content/v1/categories
GET  /content/v1/categories/{slug}
GET  /content/v1/tags
GET  /content/v1/tags/{slug}
GET  /content/v1/authors
GET  /content/v1/authors/{slug}
GET  /content/v1/series
GET  /content/v1/series/{slug}
GET  /content/v1/feed-data
GET  /content/v1/discovery-manifest
GET  /content/v1/redirects
GET  /content/v1/changes?after={cursor}
HEAD /content/v1/posts/{slug}
```

`/series` routes are part of the committed delivery. List filters are allowlisted and cursor-paginated; arbitrary client expressions never become SQL.

There is no browser-public content route in MVP. A landing project may expose its own public JSON route or client-side widget endpoint after server-side retrieval, but that route belongs to the landing project and must never reveal the provider API key.

### 8.13 Preview

- **FR-PREVIEW-001:** Preview access shall be limited to one project, content item and revision.
- **FR-PREVIEW-002:** Preview credentials shall expire in 15–60 minutes and be revocable.
- **FR-PREVIEW-003:** The landing project’s server shall exchange or use the short-lived revision credential to request preview JSON; the raw provider credential shall not reach the browser.
- **FR-PREVIEW-004:** Preview JSON shall use `Cache-Control: private, no-store` and carry explicit `index: false`; the landing project shall emit `X-Robots-Tag: noindex, nofollow`.
- **FR-PREVIEW-005:** Preview and public caches shall use completely separate namespaces.

### 8.14 Redis and delivery

- **FR-CACHE-001:** Redis shall implement cache-aside reads for published content.
- **FR-CACHE-002:** Cache entries shall contain final published JSON representations, including only sanitized/approved fields, not editor records.
- **FR-CACHE-003:** Cache keys shall include schema version and project scope.
- **FR-CACHE-004:** Publish operations shall write SQLite first and update/invalidate cache only after commit.
- **FR-CACHE-005:** Immutable body, mutable pointer and generated list keys shall have separate TTL policies.
- **FR-CACHE-006:** Mutable pointers shall use monotonic publication-version guards.
- **FR-CACHE-007:** Redis failure shall degrade to SQLite without corrupting publication state.
- **FR-CACHE-008:** The application shall measure hit rate, miss rate, evictions, latency and database fallbacks.
- **FR-CACHE-009:** The provider shall invalidate its Redis JSON keys and, if its own JSON edge cache is enabled later, support exact URL or cache-tag purges. It shall not assume control of a landing project’s page CDN.
- **FR-CACHE-010:** Published JSON responses shall expose appropriate validators and cache directives; each landing project independently decides page-cache and stale-content policy.
- **FR-CACHE-011:** Changes to a public author profile, taxonomy term, reusable block, media record or project SEO/publisher setting shall increment the affected project generation and revalidate every dependent public representation.

### 8.15 Webhooks and integrations

- **FR-HOOK-001:** Each project shall configure one or more revalidation or deployment webhooks.
- **FR-HOOK-002:** Webhooks shall use HMAC signatures over timestamp, event ID and raw body.
- **FR-HOOK-003:** Receivers shall have enough information to reject stale or replayed requests.
- **FR-HOOK-004:** Deliveries shall be idempotent, retried with backoff and visible in the admin UI.
- **FR-HOOK-005:** Operators shall be able to replay a failed delivery safely.
- **FR-HOOK-006:** Redirect, publish, update, unpublish and restore shall produce distinct event types.
- **FR-HOOK-007:** The system shall integrate with a transactional email provider for invitations, reset and operational notifications.
- **FR-HOOK-008:** Webhook destinations shall be HTTPS, explicitly registered and validated against private, loopback, link-local and metadata-service addresses before every delivery and redirect.
- **FR-HOOK-009:** A staging environment shall be technically unable to purge, rebuild or notify production landing projects.

### 8.16 Audit and administrative operations

- **FR-AUDIT-001:** Security and editorial actions shall create append-only audit events.
- **FR-AUDIT-002:** Audit events shall include actor, project, action, target, revision, outcome, request ID and timestamp.
- **FR-AUDIT-003:** Audit records shall never contain passwords, session secrets, reset tokens, project API-key secrets, preview credentials or provider credentials.
- **FR-AUDIT-004:** Login success/failure, authorization failures, membership changes, API-key operations, approvals, publication, rollback and deletion shall be audited.
- **FR-AUDIT-005:** Owners shall be able to inspect and export audit records within the retention policy.

## 9. Detailed content and blog model

The product/API term is **Article**; `content_items` is the generalized internal storage name. Every Article belongs to exactly one Project. Its editable history is a sequence of immutable Revisions, and its public state is one Project Publication pointing to an exact approved Revision.

```text
Project
  └── Article (stable identity)
        ├── Revision 1
        ├── Revision 2
        └── Project Publication → approved Revision 2
```

The following invariants are mandatory:

- A project owns its articles, revisions, publications and dependent records.
- A published revision may remain live while a newer draft is edited.
- Approval binds to an exact revision and public-field hash.
- Publication changes a pointer through one transaction; it never mutates an approved revision.
- Cross-project reuse is an explicit copy/fork into the destination project, records its origin and requires its own review plus canonical-original/material-adaptation decision.
- A single Article row shall not be directly publishable into arbitrary projects.

Product-facing article field groups are:

| Group | Representative fields |
|---|---|
| Identity | Article ID, project ID, article type, locale, translation group, origin |
| Routing | Slug, canonical URL, prior slugs, retirement/replacement target |
| Presentation | Title, alternate title, deck, excerpt, short answer, hero media |
| Content | TipTap document, sanitized HTML, plain text, Markdown export, table of contents |
| Contributors | Primary author, ordered co-authors, editor, expert reviewer, other credits |
| Taxonomy | Primary category, secondary categories, tags, optional series/position, topic relationships |
| Publishing | Editorial state, publication state, schedule, approved/published revision |
| Dates | Created, first published, materially modified, review due, content expiry, retired |
| SEO/social | SEO title/description, canonical, robots, Open Graph fields, structured-data inputs |
| Trust | Sources, block citations, claims, methodology, disclosures, corrections |
| AI/internal | Brief, evidence packet, AI jobs/runs, assistance level, provenance and quality results |
| Operations | Revision number, content hash, publication version, cache generation and audit history |

### 9.1 Project

Required `projects` fields:

```text
id
workspace_id
slug
name
status
public_project_key
primary_domain
verified_domains_json
blog_base_path
default_locale
supported_locales
timezone
publisher_name
publisher_logo_asset_id
publisher_url
publisher_same_as_json
default_author_id
default_social_image_id
seo_title_pattern
default_robots_policy
voice_profile_id
content_generation
discovery_manifest_configuration
feed_data_configuration
landing_delivery_configuration
created_by
created_at
updated_at
archived_at
```

`status` includes active, suspended, archived and pending deletion. The public project key is an identifier, not an authentication secret.

### 9.2 Article

Stable `content_items` identity fields:

```text
id
project_id
article_type
translation_group_id
origin_project_id
origin_content_id
created_by
created_at
archived_at
```

`project_id` is immutable. Moving content across projects creates an explicit destination copy/fork; it does not update the tenant owner of the original Article.

The minimum database-enforced ownership skeleton is:

```sql
PRAGMA foreign_keys = ON;

CREATE TABLE content_items (
    id           TEXT PRIMARY KEY,
    project_id   TEXT NOT NULL,
    article_type TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE RESTRICT,
    UNIQUE (project_id, id)
);

CREATE TABLE content_revisions (
    id              TEXT PRIMARY KEY,
    project_id      TEXT NOT NULL,
    content_id      TEXT NOT NULL,
    revision_number INTEGER NOT NULL,
    created_at      TEXT NOT NULL,
    FOREIGN KEY (project_id, content_id)
      REFERENCES content_items(project_id, id) ON DELETE CASCADE,
    UNIQUE (project_id, content_id, id),
    UNIQUE (project_id, content_id, revision_number)
);

CREATE TABLE project_publications (
    id                    TEXT PRIMARY KEY,
    project_id            TEXT NOT NULL,
    content_id            TEXT NOT NULL,
    locale                TEXT NOT NULL,
    published_revision_id TEXT,
    FOREIGN KEY (project_id, content_id)
      REFERENCES content_items(project_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (project_id, content_id, published_revision_id)
      REFERENCES content_revisions(project_id, content_id, id),
    UNIQUE (project_id, content_id, locale)
);

CREATE TRIGGER content_items_project_immutable
BEFORE UPDATE OF project_id ON content_items
WHEN NEW.project_id <> OLD.project_id
BEGIN
    SELECT RAISE(ABORT, 'article project_id is immutable');
END;

CREATE INDEX idx_content_items_project_created
ON content_items(project_id, created_at, id);
```

The complete migration adds the remaining columns, but it shall preserve these ownership keys and foreign-key relationships. SQLite foreign keys must be enabled on every connection. The `sqlc` query layer shall expose project-scoped signatures such as `GetArticle(ctx, projectID, articleID)` and shall not expose an unscoped article lookup to handlers or services.

### 9.3 Content revision

Immutable revision fields:

```text
id
project_id
content_id
revision_number
base_revision_id
created_by_type
created_by_user_id
brief_id
title
alternate_title
deck
excerpt
short_answer
key_takeaways_json
body_document_json
sanitized_html
plain_text
markdown_export
table_of_contents_json
word_count
reading_time_seconds
locale
author_snapshot_json
contributor_snapshot_json
taxonomy_snapshot_json
source_snapshot_json
claim_snapshot_json
definition_snapshot_json
entity_snapshot_json
seo_snapshot_json
social_snapshot_json
media_snapshot_json
disclosure_snapshot_json
correction_summary_json
change_summary
content_hash
ai_assistance_level
ai_provenance_summary_json
created_at
```

Derived render fields are generated before approval so the reviewer sees the exact JSON/HTML representation that can become public.

### 9.4 Project publication

```text
id
project_id
content_id
locale
slug
canonical_url
canonical_policy
published_revision_id
publication_state
scheduled_for_utc
first_published_at
materially_modified_at
review_due_at
content_expires_at
unpublished_at
retired_at
replacement_url
publication_version
robots_directive
draft_seo_overrides_json
draft_social_overrides_json
published_metadata_revision_id
published_render_snapshot_hash
created_at
updated_at
```

Project-specific public metadata is versioned and approved with the publication. The mutable `project_publications` row points to an immutable published content revision and an immutable published metadata revision; it does not permit unreviewed public SEO or social changes in place.

### 9.5 Public author and contributor model

A public author is independent of a user login:

```text
authors(
  id, project_id, slug, display_name, short_bio, full_bio,
  photo_asset_id, job_title, organization, credentials_json,
  expertise_json, profile_url, external_profiles_json,
  same_as_json, status, created_at, updated_at
)

revision_contributors(
  project_id, revision_id, author_id, role, position, public_snapshot_json
)
```

`role` supports primary author, co-author, editor, expert reviewer, photographer and other credited contributor. Position preserves public ordering. Composite foreign keys shall require the revision and author to match `project_id`. A disabled login or inactive profile cannot erase an immutable published byline snapshot.

### 9.6 Taxonomy and content relationships

```text
taxonomy_terms(
  id, project_id, type, parent_id, slug, name, description,
  seo_json, indexability, canonical_term_id, status
)

article_taxonomy(
  project_id, content_id, taxonomy_term_id, is_primary, position
)

series(
  id, project_id, slug, name, description, seo_json, indexability
)

series_articles(
  project_id, series_id, content_id, position
)

content_relationships(
  project_id, source_content_id, target_content_id,
  relationship_type, origin, position
)
```

The committed delivery uses hierarchical categories/subcategories of no more than three levels, flat tags, manual related-article relationships, ordered series and pillar/supporting topic clusters. A category may have at most one same-project parent, category slugs are unique within the project and category writes must prevent cycles. Parent-category article queries include descendant-category assignments by default, with exact-category filtering available explicitly. Alias/merge history must preserve public archive redirects where applicable.

### 9.7 Sources, claims, disclosures and corrections

A source stores:

```text
id
project_id
title
publisher
author
url
publication_date
accessed_at
source_type
is_primary
archived_copy_reference
notes
```

A claim stores:

```text
id
project_id
revision_id
claim_text
block_id
importance
verification_state
verified_by
verified_at
```

`claim_sources` connects claims to supporting sources, while block citation marks connect visible text to the same source records. Composite foreign keys shall reject author, taxonomy, series, source, asset or relationship records whose `project_id` differs from the Article. Important factual claims must link to one or more sources or be identified as first-party evidence, opinion or an explicitly unsupported draft claim that blocks approval.

Public disclosures and corrections are first-class records:

```text
disclosures(
  id, project_id, content_id, revision_id, disclosure_type,
  public_text, created_by, created_at
)

correction_notices(
  id, project_id, content_id, affected_revision_id,
  public_note, corrected_by, corrected_at, supersedes_notice_id
)
```

Disclosure types include sponsorship, affiliate relationship, AI-assistance policy and methodology/limitations. Correction notices are append-only and remain available in the public history.

### 9.8 Landing-page JSON rendering contract

The provider’s standard article JSON supplies the landing project with data to render:

1. Breadcrumbs.
2. H1 headline.
3. Deck or direct answer.
4. Key takeaways.
5. Ordered authors, editor and expert reviewer.
6. Visible first-published and last-materially-updated dates.
7. Hero media with dimensions, alt text, caption and credit.
8. Generated table of contents with stable heading anchors.
9. Semantic body sections and structured blocks.
10. Responsive tables, code, figures, quotes, transcripts and callouts.
11. Visible source citations and methodology.
12. Disclosure and correction notices.
13. Manual/deterministic related articles and optional topic-cluster links.
14. Author biography.
15. Reusable CTA/component identifiers.
16. Accurate JSON-LD inputs and Open Graph metadata.
17. Optional editorial definitions and entity references that correspond to visible content.

Visitor comments and newsletter subscriptions are outside this provider’s scope. A landing project may use returned CTA/component identifiers to connect its own visitor-facing services.

### 9.9 Identity and operational records

Minimum identity and security tables:

```text
users(
  id, email_normalized, password_hash, status,
  email_verified_at, password_changed_at, created_at
)

project_memberships(
  project_id, user_id, role, status, invited_by, invited_at,
  joined_at, updated_at, removed_at,
  PRIMARY KEY(project_id, user_id)
)

sessions(
  token_hash, csrf_token_hash, user_id, created_at, last_seen_at,
  idle_expires_at, absolute_expires_at, revoked_at
)

invitations(
  token_hash, project_id, email_normalized, role, invited_by,
  expires_at, accepted_at
)

password_resets(
  token_hash, user_id, expires_at, used_at
)

project_api_keys(
  id, project_id, environment, name, token_prefix, token_hash,
  scopes, expires_at, last_used_at, created_by, created_at, revoked_at
)

preview_tokens(
  token_hash, project_id, content_id, revision_id, expires_at,
  exchanged_at, revoked_at
)
```

Minimum editorial and operational tables:

```text
authors
revision_contributors
taxonomy_terms
article_taxonomy
series
series_articles
content_relationships
review_assignments
review_comments
approval_decisions
publication_metadata_revisions
slug_redirects
assets
asset_variants
sources
claims
claim_sources
disclosures
correction_notices
ai_briefs
evidence_packets
ai_jobs
ai_runs
quality_check_results
outbox_events
outbox_deliveries
webhook_endpoints
webhook_attempts
audit_events
```

Raw session, invitation, reset, API-key and preview secrets are never stored. Hashing a high-entropy token is separate from password hashing: high-entropy opaque secrets may use SHA-256 or HMAC-SHA-256, while human passwords require Argon2id.

### 9.10 Blog feature ownership and delivery boundary

**Committed single-delivery capabilities owned by the provider**

- Projects, memberships, project roles and multiple named API keys.
- Public authors/contributors and immutable attribution snapshots.
- One Article schema with typed editorial templates.
- TipTap JSON, sanitized HTML, table of contents, word count and reading time.
- Revisions, diff, autosave, review comments, approval, scheduling, rollback and preview.
- Hierarchical categories/subcategories, flat tags, manual related articles and FTS5 admin search.
- Media metadata and Backblaze B2 asset storage.
- Human-managed sources, citations, claims, disclosures and correction notices.
- Project voice profiles, evidence packets and provider-neutral AI assistance for outlining, section drafting, rewriting, critique and metadata.
- AI quality gates, claim/source extraction, provenance, progress, cancellation, quotas, evaluation and mandatory human approval.
- Full multilingual editorial and translation workflow.
- Series, topic clusters and advanced cannibalization/orphan analysis.
- Search Console/Bing Webmaster imports, landing-delivery acknowledgements and optional IndexNow status reporting.
- Advanced stale-content/correction workflows, provider-side editorial analytics and landing-integration diagnostics.
- Optional TOTP MFA and refined editor/reviewer capabilities.
- SEO/social/structured-data inputs, redirect history, discovery/feed JSON and webhooks.
- Versioned published JSON, cursor pagination, ETag and Redis cache behavior.

**Uncommitted future considerations**

- Real-time collaborative editing.
- Vector/semantic search and embedding-based recommendations.
- Advanced performance/content-decay analytics and automated optimization suggestions.
- Additional enterprise identity, compliance and approval features.

**Landing-application responsibilities**

- Public HTML, themes, layout, archive/search UI and navigation.
- XML sitemap, RSS/Atom, `robots.txt`, optional `llms.txt` and HTTP redirects.
- Visitor comments, newsletter capture, memberships/paywalls, advertising and analytics.
- Public forms, consent management and other visitor-facing product behavior.

## 10. AI subsystem

### 10.1 AI product principles

- AI assists; humans own the article.
- AI cannot approve, schedule or publish.
- Sources and extracted web content are untrusted inputs.
- The system prefers primary and authoritative evidence.
- Unsupported certainty is a blocking quality issue.
- AI-generated content is always saved as a new draft revision.
- Provider, model, prompt version, inputs, sources, usage and output lineage are recorded internally.
- A provider outage must not prevent manual authoring, review or publishing.
- AI quality is not measured using an “AI detector” score.

### 10.2 Provider abstraction

The backend shall expose internal task-oriented interfaces rather than provider-specific calls:

```text
GenerateBriefQuestions
BuildResearchPlan
SummarizeSource
ProposeOutline
DraftSection
RewriteSelection
ExtractClaims
CritiqueDraft
SuggestInternalLinks
GenerateMetadata
GenerateImageBrief
```

Provider adapters may support commercial or self-hosted models. Model names shall be configuration, not domain logic. Each task defines:

- Required capabilities.
- Structured output schema.
- Maximum input/output budget.
- Timeout and retry policy.
- Primary and fallback provider.
- Data-retention eligibility.
- Whether web retrieval or other tools are permitted.

### 10.3 Project voice profile

Each project shall own a versioned voice profile containing:

- Audience and assumed knowledge.
- Brand purpose and point of view.
- Tone, formality and humour.
- Preferred vocabulary and product terminology.
- Sentence and paragraph preferences.
- Phrases, clichés, hype and claims to avoid.
- Style requirements by content type.
- Three or more owned, approved writing examples.
- Rules for introductions, conclusions and calls to action.
- Regional spelling and locale.

The system shall not imitate an unrelated living author or intentionally add errors to appear human.

### 10.4 Evidence packet

AI drafting should begin only after creation of an evidence packet containing:

- Human brief and search intent.
- Original thesis or useful angle.
- Product/service facts approved by the project.
- Subject-matter notes and firsthand observations.
- Primary and secondary sources.
- Customer evidence approved for publication.
- Screenshots, measurements or datasets where relevant.
- Allowed and prohibited claims.
- Known limitations, tradeoffs and uncertainty.
- Required internal links and CTA.

If no unique evidence exists, the workflow must request it or recommend not publishing.

### 10.5 AI workflow

1. **Brief validation:** identify missing audience, purpose, angle, evidence or CTA.
2. **Research plan:** propose questions and preferred source types.
3. **Source collection:** a human or controlled retrieval service adds sources.
4. **Source normalization:** extract metadata and bounded text, preserving source identity.
5. **Outline generation:** create headings, proposed claims, source mapping and planned original evidence.
6. **Human outline approval:** editing or explicit approval is required.
7. **Section drafting:** generate bounded sections from the approved outline and evidence.
8. **Claim extraction:** enumerate factual claims and connect them to sources.
9. **Quality critique:** check accuracy, unsupported claims, duplication, filler, structure, voice and SEO completeness.
10. **Human editing:** add experience, judgment, examples and narrative coherence.
11. **Subject review:** verify material claims, limitations and first-person assertions.
12. **Revision approval:** approve exact output and derived render.
13. **Publication:** only a permitted human action can select the revision for publication.

### 10.6 Automated quality checks

Blocking or warning checks shall include:

- Missing or invalid source for a material factual claim.
- Fabricated or unreachable URL.
- Unverified quotation.
- Unsupported statistic, superlative or comparative claim.
- Claim/source mismatch.
- Contradiction between sections.
- Cross-project near duplication.
- Existing-topic cannibalization.
- Excessive keyword repetition.
- Generic introduction or conclusion.
- Repeated article template or phrasing.
- Brand voice violations.
- Prohibited legal, medical, financial or product claims.
- Broken internal and external links.
- Missing image alt text or credit.
- Title, canonical, robots and structured-data mismatch.
- Misleading publication or modification dates.
- Unresolved placeholders such as `[SME example required]`.
- Hidden prompt text, scripts or unsafe markup in model output.

Warnings may be overridden only by a permitted reviewer with a recorded reason. Critical checks cannot be overridden without an administrator policy exception.

### 10.7 AI security

- Retrieved pages and documents are treated as data, never system instructions.
- The retrieval service shall allow only HTTP/HTTPS and block loopback, private networks, link-local ranges and cloud metadata endpoints.
- Redirect count, response size, content type and request duration shall be bounded.
- Model tools receive least privilege and never receive deployment secrets, session tokens or database credentials.
- Source HTML and model output are sanitized before rendering.
- Uploaded files are type-checked and scanned according to risk.
- Personally identifiable or confidential data is not sent to a provider unless project policy and provider terms allow it.
- Prompt and completion logging shall support redaction and configurable retention.
- Provider keys are stored in the deployment secret manager and never in project content.

### 10.8 AI provenance

Each `ai_run` shall record:

```text
id
project_id
content_id
revision_id
task_type
provider
model_identifier
prompt_template_version
voice_profile_version
evidence_packet_version
input_hash
output_hash
source_ids
started_by
started_at
completed_at
status
input_tokens
output_tokens
estimated_cost
error_category
```

Internal provenance is mandatory. Public disclosure of AI assistance is configurable by project policy and should be used when readers would reasonably expect it.

### 10.9 AI cost and reliability

- Set per-project daily and monthly budget limits.
- Show estimated cost before long research or generation tasks.
- Cap maximum sources, retrieved bytes and model tokens per job.
- Allow cancellation and expose job progress.
- Retry only safe, idempotent stages.
- Deduplicate identical jobs by input hash.
- Use fallback models only when they meet the task’s privacy and quality policy.
- Record provider latency, failure rate and cost.
- Never hold a web request open for the full lifecycle of a long research job; use an asynchronous job and progress updates.

### 10.10 AI evaluation

Before broad release, maintain a representative evaluation set by project and content type. Evaluate:

- Factual claim accuracy.
- Citation existence and claim support.
- Source quality.
- Brand voice adherence by human rubric.
- Originality and specificity.
- Cross-project duplication.
- Structural completeness.
- Human editing effort.
- Reviewer acceptance and change-request rate.
- Cost and latency per completed, approved article.

The evaluation process must test model or prompt changes before they become the default.

### 10.11 Prompt and output architecture

Prompts shall be assembled from separately versioned layers:

1. Platform safety and editorial policy.
2. Task-specific instruction template.
3. Project voice-profile version.
4. Content-type guidance.
5. Human brief.
6. Evidence-pack extracts.
7. Existing revision and internal-link context.
8. Exact structured-output schema.

External webpages, documents and transcripts shall be clearly delimited as untrusted evidence. Instructions found inside evidence never override platform policy, tool permissions or the requested output schema.

Drafting responses should use validated structured output containing:

```text
content_blocks
claims
source_references
assumptions
open_questions
needs_human_input
warnings
suggested_internal_links
```

One bounded schema-repair attempt is permitted when a provider returns invalid structured output. Repeated invalid output becomes a visible failed job and must not create or modify a content revision.

### 10.12 Claim verification model

A first-class claim record should include:

```text
id
revision_id
statement
claim_type
risk_level
time_sensitivity
source_ids
supporting_source_spans
verification_status
verified_by
verified_at
review_due_at
```

Verification states include unreviewed, supported, partially supported, unsupported, outdated, subject-expert-required and not applicable.

Direct quotations require an exact supporting span. Numerical claims require source, date and relevant context. Time-sensitive facts receive a review date. Removing or invalidating the only supporting source reopens the claim. A URL existing is not sufficient evidence that it supports a claim.

### 10.13 Allowed AI tools

AI may use narrowly scoped server-controlled tools to:

- Search the current project’s authorized content.
- Retrieve approved internal evidence.
- Fetch an allowlisted public URL through the protected retrieval service.
- Search the public web through an approved provider.
- Retrieve verified project facts, voice and terminology.
- Perform bounded calculations or conversions.
- Check URL status and canonical target.
- Suggest internal links.
- Inspect non-secret media metadata.

AI tools shall not publish or approve content, execute arbitrary SQL or shell commands, read secrets, send arbitrary email/webhooks, modify users or roles, or make unrestricted authenticated third-party requests. Tool calls have step, time and result-size limits and record sanitized inputs, output hashes, duration and parent AI run.

### 10.14 Job states, races and model change management

AI jobs use durable states:

```text
queued
running
needs_input
succeeded
failed
cancelled
budget_blocked
safety_blocked
```

Every job records the source revision/content hash. If a writer edits the draft while generation is running, the result remains an unapplied suggestion based on the older hash. The UI must require an explicit selection or merge and must never silently overwrite newer human work.

Provider/model deprecation, model-routing changes, core prompt changes, tool-permission changes and voice-profile schema changes require:

- Offline evaluation against a representative versioned test set.
- Security tests for malicious evidence and tool requests.
- A small canary rollout.
- Cost and quality comparison.
- A documented rollback to the prior model/prompt route.

A fallback provider may be used only if its privacy, retention, regional and capability policy was approved in advance. The system must record fallback and never silently route private inputs to a weaker data policy.

## 11. Technical architecture

### 11.1 Component view

```text
                          +----------------------+
                          | Nuxt Admin           |
                          | Nitro SSR server     |
                          +----------+-----------+
                                     |
                                     | same-origin session + CSRF
                                     v
+----------------+        +----------+-----------+        +----------------+
| Landing build  |------->| Go Fiber + Huma API |------->| SQLite WAL     |
| SSR / ISR      | API key| modular monolith    |        | source of truth|
+-------+--------+        +-----+----------+-----+        +--------+-------+
        |                       |          |                       |
        |                       |          +------------+          |
        |                       v                       |          |
        |                 +-----+-----+          +------v----------v-+
        |                 | Redis     |          | Durable worker    |
        |                 | cache     |          | SQLite outbox     |
        |                 +-----------+          +--+-----+-----+----+
        |                                            |     |     |
        v                                            |     |     |
+-------+--------+                                   |     |     |
| External      |<----- signed change/revalidate ----+     |     |
| landing app   |                                          |     |
| + page CDN    |                                          |     |
+-------+-------+                               Backblaze B2     AI/email
        |
        v
Landing-rendered HTML → visitors, search crawlers and answer engines
```

### 11.2 Repository and runtime structure

The product shall use one Git monorepo containing independent Go and Nuxt applications. The checked-in v1.9 layout and naming baseline is:

```text
seoblog/
├── apps/
│   ├── backend/
│   │   ├── cmd/
│   │   │   ├── api/main.go
│   │   │   ├── worker/main.go
│   │   │   └── admincli/main.go
│   │   ├── internal/
│   │   │   ├── config/
│   │   │   ├── httpapi/
│   │   │   ├── platform/database/migrations/
│   │   │   ├── security/
│   │   │   └── store/
│   │   ├── queries/
│   │   ├── sqlc.yaml
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   └── go.sum
│   └── admin/
│       ├── assets/css/
│       ├── layouts/
│       ├── pages/
│       ├── app.vue
│       ├── Dockerfile
│       ├── nuxt.config.ts
│       └── package.json
├── packages/
│   └── content-client/src/
├── contracts/
│   └── openapi/openapi.yaml
├── infra/
│   ├── nginx/
│   ├── pm2/
│   │   └── ecosystem.config.cjs
│   └── ...
├── pnpm-workspace.yaml
├── docker-compose.yml
├── Taskfile.yml
└── PRD.md
```

Normative boundaries:

- `apps/backend` is one Go module with one `go.mod`; a `go.work` file is unnecessary until the repository contains multiple independent Go modules.
- `cmd/api`, `cmd/worker` and `cmd/admincli` compile as separate binaries and reuse packages under `internal`.
- Runtime SQLite migrations used by the binaries live under `apps/backend/internal/platform/database/migrations` and are embedded by the database package. Any duplicate migration directory must be generated from, or reconciled with, that authoritative source before production release.
- The current foundation may keep coarse packages such as `internal/httpapi` and `internal/store` while requirements stabilize. When the codebase is split into finer domain packages, Section 11.3 defines the intended package boundaries.
- The PNPM workspace manages only the Nuxt application and TypeScript packages; it does not manage Go dependencies.
- A root task runner coordinates builds, tests, migrations, contract generation, Docker development and the versioned production release without merging dependency graphs.
- `task dev` shall start the complete development stack through the repository's canonical Docker Compose entry point, currently `docker-compose.yml`; `task deploy:prod RELEASE=<immutable-release-id>` shall release both application artifacts through the controlled production deployment script.
- Huma's OpenAPI output is the API contract source. The generated contract is stored at `contracts/openapi/openapi.yaml`, and the protected API also exposes `/openapi.yaml` while running. `task contracts:generate` and the generated `packages/content-client` package shall be checked in or validated together, and CI shall fail on contract/client drift.
- Nuxt shall contain no direct SQLite access, authoritative authorization or duplicated core Nitro API implementation. It calls the Go admin API.
- Nuxt SSR may perform presentation-layer session bootstrap and call the Go API over the host-private network, but Go remains authoritative for authentication, authorization, validation, persistence and publication.
- Landing applications remain in separate repositories and consume the versioned Content API or generated content client.
- One monorepo/release version does not imply one runtime process or one deployable artifact.

### 11.3 Backend package structure

Proposed internal Go domain packages:

```text
identity        users, invitations, passwords and sessions
authorization   memberships, roles and permissions
projects        tenant settings, domains, policies, voice and integrations
content         items, revisions, structured documents
editorial       review, comments and approvals
publication     schedules, published pointers and rollback
taxonomy        categories, tags, topics and series
media           assets, variants, credits and storage
seo             canonical rules, schema inputs, discovery/feed data and redirect manifests
delivery        published JSON DTOs, ETags and cache behavior
integrations    project API keys, landing revalidation webhooks and delivery acknowledgements
ai              providers, evidence, jobs, provenance and evaluations
audit           security/editorial audit events
jobs            outbox processing, retries and job status
```

These packages share one Go module, source tree, database and versioned release. The API, worker and CLI are separate binaries that reuse them and interact through explicit service interfaces and transactions.

### 11.4 Fiber and Huma

Fiber v3 is selected for routing and middleware. Huma provides:

- OpenAPI 3.1 generation.
- Request and response schemas.
- Consistent validation.
- Problem response formatting.
- An easier generated client path.
- Some insulation between transport and domain services.

All database, AI, Redis and object-storage calls receive explicit service-level contexts and timeouts. Background work receives copied immutable identifiers and never a retained Fiber request context.

Implementation alignment gate: the checked-in foundation currently imports `github.com/gofiber/fiber/v2`. Before production acceptance, the backend shall either migrate to Fiber v3 with the compatible Huma adapter and regenerate contract/client artifacts, or record an explicit architecture decision changing the selected HTTP stack. Until that gate is resolved, requirements that name Fiber v3 describe the selected target rather than proof that the current code has completed the migration.

### 11.5 Nuxt versus HTMX

Nuxt is selected for the admin application because the product needs:

- A rich structured editor.
- Revision comparison.
- Autosave and conflict warnings.
- Comment and approval interfaces.
- AI generation progress and streaming feedback.
- Drag/drop media management.
- Complex filters and dashboards.
- Accessible light and dark themes.

HTMX would be suitable for a simpler operational console, but the editor and review experience would require increasing amounts of custom client state. Nuxt provides a clearer long-term UI architecture. Public landing projects are independent and may use Nuxt, another SSR framework or a static generator.

The admin Nuxt application shall use Nitro SSR in production behind Nginx. SSR may render the application shell, resolve the current session and preload presentation data, while browser hydration provides the rich editor experience. This does not make Nuxt a second backend: all authoritative permissions, workflow transitions, persistence, AI job control and Content API behavior remain in Go. Admin SSR pages are non-indexable and `no-store`; only fingerprinted client assets receive long-lived caching.

### 11.6 SQLite decision

Recommended initial database stack:

- SQLite in WAL mode.
- `database/sql`.
- `sqlc` for typed, visible SQL.
- A migration tool such as Goose.
- A CGO-free driver such as `modernc.org/sqlite`, subject to load testing.
- Foreign keys enabled.
- Busy timeout configured.
- Short write transactions.
- Continuous WAL-aware backup to a dedicated Backblaze B2 backup bucket through Litestream's S3-compatible replica adapter.

PocketBase is not selected because it is a complete backend framework with its own auth, data model, routing and administrative concepts. Combining it with Fiber would duplicate responsibilities. PocketBase remains a valid alternative only if the team chooses to let PocketBase replace most of the custom backend.

SQLite is appropriate while the product has a single primary writer host and moderate administrative write traffic. CDN and Redis absorb published reads. Redis does not make SQLite multi-writer.

Migration triggers include:

- Requirement for active-active writers across hosts.
- Sustained lock contention despite short transactions.
- High availability requiring automatic database failover.
- A serverless deployment without persistent local disk.
- Reporting workloads that interfere with transactional work.

At that point, evaluate the current Turso remote/sync offering or migrate the repository layer to PostgreSQL. Landing projects still call the API; database tokens are never distributed to them.

### 11.7 Redis decision

Use `go-redis/v9` behind a small application cache interface.

Redis responsibilities:

- Published representation cache.
- Listing/feed-data/discovery-manifest cache.
- Login and integration rate limits.
- Short coordination locks when more than one API process exists.
- Optional non-durable fast invalidation notifications.

Redis shall not be:

- The canonical article store.
- The only event queue.
- The authoritative session store in the initial design.
- Shared between an evicting cache and future durable business queues.

Recommended eviction policy for a cache-only instance is `allkeys-lfu` or, after observation, `allkeys-lru`. Every cache key must have a TTL.

### 11.8 Durable outbox

`outbox_events` is written in the same SQLite transaction as the business state change. Events include:

```text
event_id
event_type
project_id
content_id
revision_id
publication_version
old_slug
new_slug
payload_json
created_at
available_at
attempt_count
completed_at
```

Delivery destinations track independent attempts so a successful Redis update does not hide a failed landing webhook. Consumers are idempotent by event ID and version. Out-of-order events re-read current SQLite state before updating mutable cache pointers.

### 11.9 Durable job runner

Requested long-running work such as AI generation, image processing, link checking and scheduled publishing uses a durable `jobs` table rather than Redis or in-memory timers.

Minimum job fields:

```text
id
job_type
project_id
payload_json
idempotency_key
status
priority
attempt_count
max_attempts
next_attempt_at
locked_by
locked_until
created_at
started_at
completed_at
last_error_code
last_error_safe_message
trace_id
```

Workers atomically claim a lease, heartbeat long work and reclaim an expired lease after a crash. Transient failures use bounded exponential backoff with jitter; permanent validation, authorization and safety failures are not retried indefinitely. Exhausted jobs enter an admin-visible dead-letter state with safe manual replay.

Only one effective scheduler/outbox lease may process a logical job at a time, even if more API/worker processes are introduced. Server clocks must be synchronized because leases, schedules, webhook signatures and expiring credentials depend on accurate time.

### 11.10 Recommended technology stack

| Layer | Selected technology | Reason |
|---|---|---|
| Backend language | Go 1.25+ | Simple deployment, concurrency and strong operational tooling |
| HTTP/API | Fiber v3 + Huma | Productive routing/middleware plus OpenAPI 3.1 contracts |
| Admin application | Nuxt + TypeScript with Nitro Node SSR | Rich stateful editorial UI, server-rendered bootstrap and Vue ecosystem |
| UI system | Nuxt UI | Accessible component system with built-in dark-mode support |
| Rich editor | Nuxt UI Editor/TipTap/ProseMirror | Structured versioned JSON and extensible editorial blocks |
| Database access | `database/sql` + `sqlc` | Explicit SQL with generated typed Go code |
| Migrations | Goose or equivalent | Versioned, reviewable schema change workflow |
| SQLite driver | `modernc.org/sqlite` initially | CGO-free deployment, subject to benchmark/compatibility tests |
| Cache/rate limit | Redis + `go-redis/v9` | Shared cache-aside and throttling primitives |
| Full-text search | SQLite FTS5 | Adequate initial editorial/public search without another service |
| Durable jobs/events | SQLite jobs + transactional outbox | Atomic state and recoverable delivery without Redis durability assumptions |
| Media | Backblaze B2, CDN and asynchronous derivatives | Durable scalable binaries outside SQLite |
| Backup | Litestream to Backblaze B2 plus independent SQLite snapshots | Continuous recovery points and tested restore workflow |
| Edge/TLS | Cloudflare plus Nginx | CDN/WAF at edge and established origin TLS, proxy and routing controls |
| Observability | Structured logs, OpenTelemetry metrics/traces and managed error reporting | Correlated operational visibility without a large self-hosted stack |
| Browser tests | Playwright | End-to-end authentication, editing, review and publication tests |
| Unit/integration tests | Go test, Vitest and real SQLite/Redis integration tests | Domain, UI and infrastructure behavior |
| Local runtime | Docker Engine plus Docker Compose v2 | One-command, reproducible development stack |
| Production application supervisor | PM2 in fork mode | Supervision, restart policy, release-specific process declaration and startup restoration |
| Production host services | systemd or managed services | Nginx, Redis and Litestream are infrastructure rather than PM2 applications |
| Infrastructure | OpenTofu/Terraform | Reproducible small deployment without Kubernetes |
| AI | Provider-neutral adapters and JSON-schema outputs | Model choice, privacy policy and fallback remain configurable |

Exact dependency versions shall be pinned during implementation and updated through automated compatibility/security checks. Current model names and hosted-service pricing shall remain configuration rather than requirements.

### 11.11 Admin UX and visual design

The admin UI should be calm, readable and editorial rather than resembling a generic database console.

- Use Nuxt UI’s neutral design tokens and accessible primitives.
- Support system, light and dark theme preferences, with an explicit user override.
- Keep article text at a comfortable 65–80 character measure in editing and preview modes.
- Use typography and spacing to communicate hierarchy; do not rely only on color for workflow status.
- Preserve visible keyboard focus, labelled validation errors and skip/navigation landmarks.
- Keep autosave, revision, environment and publication status continuously visible.
- Separate writing controls from AI suggestions so generated text cannot appear to be silently authored by the user.
- Show source/claim status next to the relevant content without making the default writing surface excessively noisy.
- Provide distraction-reduced writing mode and a structured metadata/SEO side panel.
- Offer side-by-side or inline revision diff with screen-reader-compatible change summaries.
- Warn clearly before navigating away with unapplied local changes or an unresolved merge.

Primary screens:

```text
Project list, creation, selector and lifecycle status
Dashboard and operational alerts
Content library and calendar
Structured article editor
Revision history and diff
Review comments, claims and approvals
AI brief/evidence/jobs panel
Media library
Authors and taxonomy
Project SEO/brand/crawler settings
Project members, roles and invitations
Project API keys, webhooks and delivery status
Audit, backup and system health
```

## 12. Infrastructure and deployment

### 12.1 Recommended MVP topology

Use one primary region and one persistent Linux host for MVP. A starting size of approximately 2 vCPU, 4 GB RAM and 60–100 GB local SSD/NVMe is reasonable, but load and media-processing tests must confirm it before production:

```text
Cloudflare DNS/CDN/WAF or equivalent
          |
Cloudflare Tunnel or restricted HTTPS origin
          |
        Nginx
          |
          +-- cms.example.com UI routes --> PM2: Nuxt Nitro SSR
          +-- cms.example.com/api/* -----> PM2: Go API
          +-- content.example.com/* -----> PM2: Go API

        PM2
          +-- Nuxt Nitro SSR process
          +-- Go API process
          +-- Go worker process

        Host/private services
          +-- Redis cache
          +-- Litestream backup agent
          +-- local persistent SQLite filesystem

External services:
  Backblaze B2 media bucket
  Transactional email
  AI model providers
  Error/metric monitoring
  Separate Backblaze B2 database-backup bucket
```

The Nuxt Nitro server, API and worker are separate PM2-supervised processes from one pinned release. The API and worker are distinct native Go binaries built from the same Go module and run on the same host/filesystem while sharing the local SQLite database. The admin CLI is a third on-demand Go binary for owner bootstrap, migrations and controlled maintenance. Nuxt never opens SQLite. Do not place the SQLite file on NFS or an eventually consistent network volume.

The public availability of landing pages should primarily depend on each project’s CDN-rendered output, not continuous availability of the CMS. A previously rendered article remains available even during CMS maintenance.

Redis may run privately on the same VM for MVP because it contains only disposable cache and rate-limit state. It shall have a memory limit, no public port and an explicit eviction policy. Move Redis to a managed same-region private service with TLS/ACLs before horizontally scaling application hosts.

Do not use Kubernetes for MVP. It adds orchestration complexity without removing SQLite’s single-primary constraint. Docker Compose is the development runtime only. In production, PM2 supervises application processes; systemd or managed-service controls supervise Nginx, Redis and Litestream. PM2 shall not be used to launch Redis or Litestream.

Recommended hostnames:

```text
cms.example.com       Nuxt admin and authenticated /api routes
content.example.com   project Content API
assets.example.com    immutable public media
status.example.com    optional externally hosted status page
```

Admin HTML and user-specific SSR responses are `no-store`; hashed Nuxt client assets may be cached immutably. API-key-authenticated JSON content and media use separate policies. Only Nginx or the private tunnel should receive public HTTP traffic. PM2 ports bind to loopback/private interfaces, and SQLite and Redis must never be externally reachable.

### 12.2 Environment separation

Provide development, staging and production environments with:

- Separate SQLite databases.
- Separate project API keys.
- Separate Backblaze B2 media and database-backup buckets for each environment.
- Separate Redis namespaces or instances.
- Separate webhook targets and signing secrets.
- Separate AI budgets and, where needed, provider keys.
- Clearly visible environment banners in the admin UI.
- `noindex` and blocked public discovery for non-production environments.

Production data must not be copied into development without explicit redaction.

### 12.3 Packaging and deployment

- Build reproducible native Go binaries and the Nuxt Nitro Node SSR artifact in CI or a controlled build environment; do not compile unreviewed source on the production host.
- Use Docker images and Docker Compose for development and integration testing. Production runs the versioned application artifacts under PM2 rather than running the application stack through Docker Compose.
- Keep Nuxt `ssr: true`. SSR is used for the admin presentation layer, authenticated route bootstrap and server runtime configuration; it does not move domain logic, SQL or authoritative authorization out of Go.
- Run migrations as an explicit deployment step with a pre-migration backup.
- Prefer backward-compatible expand/migrate/contract schema changes.
- Gracefully stop HTTP and worker processes, allowing transactions and jobs to finish or return safely to the queue.
- Expose liveness and readiness endpoints.
- Verify database access, Redis fallback, required Backblaze B2 access and migration version in readiness checks without making optional AI providers a hard dependency.
- Maintain a fast rollback procedure for the application; database rollback must use tested forward fixes or restoration rather than destructive ad hoc commands.
- Drain workers before migrations, stop new lease claims, and safely complete or release running jobs.
- Run migration tests both from an empty database and from the previous production schema.
- Pin artifacts by version/digest and generate a software bill of materials.

#### Release artifacts and host layout

The release bundle contains independently runnable, mutually compatible artifacts:

```text
backend-api          native Go API binary
backend-worker       native Go worker binary
backend-admincli     on-demand bootstrap/migration/maintenance binary
admin/.output/       versioned Nuxt Nitro Node SSR build
ecosystem.config.cjs versioned PM2 application declaration
nginx/seoblog.conf   reviewed Nginx virtual-host configuration
openapi.yaml         generated API contract
content-client       generated versioned TypeScript package
release.json         release ID, commit SHA, build metadata and checksums
```

All artifacts receive the same immutable release ID and commit SHA. They are separate runtime outputs but are tested, released and normally rolled back as one compatibility unit. A production host uses a layout equivalent to:

```text
/srv/seoblog/
├── releases/
│   ├── 2026-07-29.1/
│   │   ├── bin/seoblog-api
│   │   ├── bin/seoblog-worker
│   │   ├── bin/seoblog-admincli
│   │   ├── admin/.output/
│   │   ├── ecosystem.config.cjs
│   │   └── release.json
│   └── ...
├── shared/
│   └── data/seoblog.db
├── current -> releases/2026-07-29.1
└── previous -> releases/<previous-release>
```

The SQLite database and other mutable state live outside a release directory. Switching the `current` symlink must never switch, copy or delete the authoritative database. Database migrations should be embedded in the `admincli` binary or shipped as checksummed release data.

Nginx provides the public origin and routes:

| Request | Upstream | Required behavior |
|---|---|---|
| `cms.example.com/api/*` | Go API on `127.0.0.1:8080` | Same-origin session/CSRF behavior; admin responses `no-store` |
| `cms.example.com/*` | Nuxt Nitro on `127.0.0.1:3000` | SSR UI; support streaming and WebSocket upgrades used by development/runtime features |
| `content.example.com/content/v1/*` | Go API on `127.0.0.1:8080` | Project-key JSON API with distinct limits and cache headers |
| `assets.example.com/*` | Media CDN/B2 delivery path | Immutable media policy; do not route large media through Nuxt |

Nginx terminates or receives trusted TLS, applies body/header/time limits and forwards normalized proxy headers. Fiber and Nuxt shall trust forwarded client information only from the known Nginx/edge addresses. Nuxt may call the Go API through its loopback URL during SSR and may forward only the request data needed for the user session. It shall not receive project API keys, AI-provider credentials, B2 backup credentials or database access.

#### Docker Compose local development

Local development shall run through one root command:

```text
task dev
  -> docker compose up --build
```

The default Compose project contains:

| Service | Purpose | Local exposure and state |
|---|---|---|
| `nginx` | Production-like same-origin routing | Browser entry point, default `http://localhost:8088` |
| `admin` | Nuxt SSR application | Internal port 3000; source/HMR support may be provided by Compose or documented direct dev mode |
| `api` | Go API | Internal port 8080; shared SQLite volume |
| `worker` | Go worker | No public port; shared SQLite volume |
| `redis` | Disposable cache and rate-limit state | Compose-private only; any local volume is convenience state and may be reset |
| `mailpit` | Captures invitations and password-reset email | Web UI bound to loopback; SMTP is Compose-private |

An optional `backup` profile may run Litestream against a dedicated development/test target. It shall never receive production B2 credentials. AI, transactional email and B2 integrations use explicit development accounts or deterministic test adapters; Docker startup must not silently call production services.

The Compose design shall:

- Use a named local Docker volume such as `seoblog_sqlite` mounted only into the API, worker and optional backup container. This avoids host bind-mount filesystem differences for SQLite WAL and locks.
- Keep the database path identical in API and worker configuration and keep it on one Docker host; never use NFS or a network volume.
- Use one shared backend development image with different API and worker commands, while keeping them as independent processes.
- Run application containers as non-root users, use `init: true` or equivalent signal forwarding and implement health checks.
- Support either source-mounted hot reload through Docker Compose or an equivalent documented direct development loop using `task backend:dev`, `task backend:worker` and `pnpm --filter @seoblog/admin dev`; use named dependency/build caches where source mounts are enabled so routine restarts do not reinstall every dependency.
- Support Nuxt HMR/WebSocket proxying through the local Nginx route.
- Bind any optional direct debug ports to loopback only. Redis and SQLite are never exposed.
- Keep `.env` untracked, provide a non-secret `.env.example` and fail clearly when required development values are absent.
- Use a stable Compose project name so volumes are not accidentally shared with staging or an unrelated checkout.

Required root tasks are:

```text
task dev              build and start the complete stack
task dev:logs         follow all application logs
task dev:down         stop containers without deleting the SQLite volume
task dev:reset        require explicit confirmation, then recreate local data
task test:integration run an isolated Compose project and disposable test volume
```

The local Nginx route sends `/api/*` directly to Go and all remaining admin UI routes to Nuxt, matching production ownership. A developer may address the Go or Nuxt port directly only for debugging. Docker Compose is not the production deployment mechanism.

#### Production supervision with PM2

PM2 supervises exactly three application processes for the MVP:

1. `seoblog-admin`: the Nuxt Nitro Node server.
2. `seoblog-api`: the native Go HTTP API binary.
3. `seoblog-worker`: the native Go jobs/outbox worker binary.

Nginx, Redis and Litestream are controlled by systemd or by their managed-service provider. PM2 must not be treated as a general infrastructure orchestrator. All PM2 processes run under one dedicated, non-root deployment user, and PM2 commands during deploy and recovery run as that same user so `PM2_HOME`, the saved process list and logs do not diverge.

The versioned `infra/pm2/ecosystem.config.cjs` should be equivalent to:

```js
const releaseRoot = '/srv/seoblog/current'

module.exports = {
  apps: [
    {
      name: 'seoblog-admin',
      namespace: 'seoblog',
      cwd: releaseRoot,
      script: './admin/.output/server/index.mjs',
      interpreter: 'node',
      exec_mode: 'fork',
      instances: 1,
      watch: false,
      autorestart: true,
      restart_delay: 2000,
      min_uptime: '10s',
      max_restarts: 10,
      kill_timeout: 15000,
      max_memory_restart: '768M',
      env_production: {
        NODE_ENV: 'production',
        NITRO_HOST: '127.0.0.1',
        NITRO_PORT: '3000',
        NUXT_API_INTERNAL_BASE_URL: 'http://127.0.0.1:8080'
      }
    },
    {
      name: 'seoblog-api',
      namespace: 'seoblog',
      cwd: releaseRoot,
      script: './bin/seoblog-api',
      interpreter: 'none',
      exec_mode: 'fork',
      instances: 1,
      watch: false,
      autorestart: true,
      restart_delay: 2000,
      min_uptime: '10s',
      max_restarts: 10,
      kill_timeout: 15000,
      max_memory_restart: '1G',
      env_production: {
        APP_ENV: 'production',
        SEOBLOG_CONFIG_FILE: '/etc/seoblog/production.env'
      }
    },
    {
      name: 'seoblog-worker',
      namespace: 'seoblog',
      cwd: releaseRoot,
      script: './bin/seoblog-worker',
      interpreter: 'none',
      exec_mode: 'fork',
      instances: 1,
      watch: false,
      autorestart: true,
      restart_delay: 2000,
      min_uptime: '10s',
      max_restarts: 10,
      kill_timeout: 60000,
      max_memory_restart: '1G',
      env_production: {
        APP_ENV: 'production',
        SEOBLOG_CONFIG_FILE: '/etc/seoblog/production.env'
      }
    }
  ]
}
```

This declaration is illustrative; memory and shutdown values require load tests. Secret values shall not be committed to the ecosystem file. The protected configuration file or secret injection mechanism holds the values, and its permissions restrict it to the application user. Nuxt public runtime configuration must contain no secrets.

Use PM2 `fork` mode and one instance per process initially. PM2 cluster mode is useful primarily for Node network workloads and must not be used to multiply Go API/worker processes without an explicit SQLite write-contention, scheduling and connection-pool design. Nuxt alone may later use multiple cluster instances after memory/load testing, because it owns no SQLite state.

The API handles `SIGINT` and `SIGTERM`, stops accepting new requests and completes a bounded graceful shutdown. The worker stops claiming new leases, checkpoints or releases retryable work and exits before `kill_timeout`. Nuxt also performs bounded graceful shutdown. Do not enable production file watching.

Host provisioning shall pin a supported Node.js release and PM2 version, run PM2 startup integration for the dedicated application user and persist the verified process list with `pm2 save`. PM2 stdout/stderr logs need structured application output, rotation, retention and disk-usage alerts through `pm2-logrotate` or the host log rotation system. The deployment must never run `pm2 delete all` or otherwise affect unrelated PM2 applications.

#### One-command production application release

The operator interface shall be:

```text
task deploy:prod RELEASE=<immutable-release-id>
```

The command deploys the Nuxt SSR artifact and all Go artifacts as one release, but it does not hide the required safety stages:

1. Fetch or upload a CI-built, checksummed release and verify its release manifest, supported target architecture and required runtime versions.
2. Extract into a new `/srv/seoblog/releases/<release-id>` directory without changing `current`.
3. Validate configuration, free disk, database ownership, migration compatibility, B2 backup access and PM2/Nginx preconditions.
4. Stop new worker lease claims, allow bounded active work to finish and verify the worker is drained.
5. Create and verify a pre-migration SQLite recovery point in the dedicated B2 backup path.
6. Run the new admin CLI migration exactly once under a deployment lock.
7. Record the old `current` target as `previous`, then atomically replace `current`; never place mutable SQLite data inside either release.
8. Run `pm2 startOrRestart ecosystem.config.cjs --env production --update-env --only "seoblog-admin,seoblog-api,seoblog-worker"` so unrelated PM2 applications are untouched.
9. Check the API readiness endpoint, Nuxt SSR health, Nginx routes, a project-scoped Content API smoke request and worker lease processing.
10. Persist the healthy PM2 process list with `pm2 save`, record the release result and prune old releases only under the retention policy.

Normal application releases do not require an Nginx restart. If a reviewed Nginx configuration changed, the deployment first installs it through narrowly scoped privilege, runs `nginx -t`, and performs a graceful reload only after validation.

Failure before the symlink switch leaves the old release active. Failure after the switch restores the previous application symlink and restarts the three named PM2 processes. Database migrations must remain backward compatible with the previous application or use a tested forward fix/restoration plan; the script shall never improvise a destructive down migration.

`task deploy:prod` assumes the host, Nginx, PM2 startup service, Redis, Litestream, directories, protected configuration and least-privilege deploy access were provisioned already. Infrastructure changes remain a separately reviewed `task infra:plan`/`task infra:apply` workflow. PM2’s Git-based deployment feature is not the default: production consumes prebuilt immutable artifacts rather than pulling and compiling arbitrary branch state on the host.

With one PM2 instance, `startOrRestart` may cause a short origin interruption. Existing landing pages should remain available from their own CDN. If truly zero-downtime admin/API releases become necessary, add a health-checked blue/green port switch in Nginx rather than assuming PM2 cluster mode makes the SQLite-backed Go service horizontally safe.

### 12.4 JSON API and HTTP caching

Published JSON responses should use:

- Strong ETag derived from the published revision/content hash.
- Last-Modified based on material publication state.
- Conditional `GET` and `HEAD`.
- A short consumer freshness period.
- Landing-side caching appropriate to its rendering strategy.
- `stale-while-revalidate`.
- `stale-if-error` where safe.

Authorization-protected integration responses may not be cached by a shared CDN by default. Redis is the provider’s JSON cache, while the principal public CDN cache is the fully rendered page owned by the landing project. MVP does not expose a browser-public provider endpoint.

### 12.5 Object storage and media delivery

- Use a dedicated private Backblaze B2 media bucket with default SSE-B2 encryption enabled; deliver approved media through a CDN with an authenticated private origin or an explicitly reviewed public-derivative policy.
- Access B2 from the Go service through its HTTPS S3-compatible API. The configured endpoint shall be `https://s3.<region>.backblazeb2.com`, and the endpoint and region shall match the bucket exactly.
- Upload directly with short-lived presigned `PUT` URLs where practical, using unpredictable content-addressed object keys, declared size limits and integrity checks. Do not design around browser form-based presigned `POST`, which B2's S3-compatible API does not support.
- Use B2 file-version history and reviewed lifecycle rules for recovery and cleanup.
- Use a dedicated transformation service or controlled image worker for resizing and format conversion.
- Serve assets through a CDN with immutable content-addressed names.
- Store database references and metadata, not binary blobs.
- Store originals privately and expose only verified derivatives according to policy.
- Clean abandoned multipart uploads and unreferenced temporary objects after a grace period.
- Do not depend on per-object ACLs or object tagging; B2's S3-compatible access control is bucket-oriented and does not implement all AWS S3 features.
- Keep media originals/derivatives and SQLite backups in separate B2 buckets. The media application key shall have no access to the backup bucket.
- Use distinct bucket-scoped B2 application keys for upload/processing, CDN origin reads, backup writes and restore reads. Never use the account master key in the running application.

### 12.6 Backup and disaster recovery

Minimum requirements:

- Continuously replicate SQLite and WAL-aware state with a pinned, current Litestream release to a dedicated private Backblaze B2 backup bucket using Litestream's S3-compatible replica type.
- Configure the exact B2 endpoint and region (`s3.<region>.backblazeb2.com`); use a Litestream release that supports automatic B2 endpoint compatibility and validate backup integrity in staging before production.
- Create daily verified database snapshots.
- Retain daily backups for at least 30 days and monthly backups according to business/legal policy.
- Back up configuration needed to reconstruct projects, secret references and deployment.
- Keep recoverable B2 file-version history and lifecycle settings for media objects according to the media-retention policy.
- Redis cache requires no backup; it must be fully rebuildable.
- Test restoration at least quarterly into an isolated environment.
- Document who can authorize restore and how DNS/CDN/webhook behavior is handled during recovery.
- Never create a live backup using a simple filesystem copy of only the `.db` file; use a WAL-aware replicator, SQLite Online Backup API or `VACUUM INTO`.
- Restore the latest backup automatically to an isolated database each day and run `PRAGMA quick_check`; run a full clean-host application recovery drill at least quarterly.
- Enable B2 Object Lock on the backup bucket and apply reviewed governance or compliance retention to independent daily/monthly snapshot objects. Object Lock cannot be disabled after it is enabled on a bucket, so retention mode and duration must be tested and approved first.
- Keep the continuous Litestream replica and immutable snapshots in separate prefixes or separate B2 backup buckets. Object Lock and lifecycle policies must not break Litestream compaction or the contiguous restore chain.
- Use separate least-privilege, bucket- or prefix-scoped B2 application keys for backup writing and restoration. The normal API/media credentials shall not list, read, write or delete backup objects.
- Give the running backup process only the B2 capabilities Litestream actually requires. Keep immutable snapshot deletion/retention controls outside the application credential, and prefer B2 lifecycle rules for approved cleanup.
- Enable default SSE-B2 encryption before the first upload to every private backup bucket. If customer-managed encryption is required, separately validate an SSE-C or client-side-encrypted snapshot design and its key-recovery procedure before adoption.

Proposed initial recovery objectives:

- Database recovery point objective: 5 minutes.
- Initial database recovery time objective: 4 hours, with a 1-hour target after recovery automation is proven.
- Published landing pages should remain available from their CDN during a CMS database recovery where cached/deployed.

### 12.7 Secrets

- Store deployment and provider secrets in the platform secret manager or protected environment injection.
- Do not store secrets in Git, Nuxt public configuration, SQLite content fields or logs.
- Separate secrets by environment.
- Support rotation for session signing/CSRF material, project API keys, webhook secrets, Backblaze B2 application keys, email and AI providers.
- Maintain a break-glass owner recovery procedure with audit.

### 12.8 Infrastructure as code and operations

Once production hosting is selected, infrastructure shall be reproducible through Terraform, OpenTofu or an equivalent declarative tool. The repository should include:

- Network and firewall rules.
- DNS and CDN configuration where supported.
- Nginx virtual hosts, the PM2 ecosystem declaration and systemd/managed-service definitions for Nginx, Redis, Litestream and PM2 startup restoration.
- Persistent volume declaration.
- Redis configuration.
- Backblaze B2 media/backup buckets, application-key scopes, SSE-B2 encryption, Object Lock and lifecycle policies.
- Monitoring and alert definitions.
- Backup configuration.
- Environment-specific secret references, not secret values.

### 12.9 Scaling path

Scale in this order:

1. Improve rendered-page CDN hit rate.
2. Optimize Redis hit rate and response size.
3. Add indexes and improve SQLite queries.
4. Independently tune CPU, memory, concurrency and service limits for the existing API and worker processes on the same host.
5. Increase primary host resources and storage performance.
6. Move read-heavy analytics away from the transactional database.
7. If multiple writer hosts or automatic DB failover are required, migrate to a suitable network database.

Do not introduce multiple services or regions before metrics identify the bottleneck.

Before a second application host is introduced, move Redis to shared managed infrastructure and implement one effective job/scheduler lease. Never mount one SQLite WAL database over a network filesystem for multiple hosts.

As a later resilience optimization, publication may materialize immutable public artifacts:

```text
published/{projectID}/revisions/{revisionID}.json
published/{projectID}/manifests/{contentGeneration}.json
```

These artifacts can provide a last-known-good content source through Backblaze B2/CDN while the CMS origin is recovering. They remain derived output and do not replace SQLite.

### 12.10 Network and host security

- Prefer an outbound-only tunnel or firewall the origin to known CDN addresses.
- Keep Redis and SQLite on private/local interfaces only.
- Use key-based SSH through a private administrative network such as WireGuard or Tailscale; disable public password login.
- Run services as non-root users and restrict database/secret file permissions.
- Configure Nginx as the proxy-header boundary and configure Fiber/Nitro trusted-proxy ranges narrowly before using forwarded IPs for audit or rate limits.
- Set request-body, header, upload and connection limits at both proxy and application layers.
- Restrict CORS to exact required origins; server-to-server integration does not require browser CORS.
- Apply timeouts and bounded retries to every outbound provider call.
- Validate the resolved destination again after every redirect for webhooks, imported media, link checks and AI retrieval.

### 12.11 Email and domain operations

The transactional email domain shall configure SPF, DKIM and DMARC. The product shall monitor bounces and provider failures and provide a safe invitation/reset resend flow without revealing account existence.

Domain activation shall verify ownership before changing canonical URLs, discovery-manifest data or the crawler policy returned to a landing project. A domain-change workflow shall:

- Confirm ownership of the new domain.
- Generate old-domain/old-path redirect requirements.
- Update canonical and webhook targets atomically where possible.
- Prevent redirect loops.
- Revalidate sitemaps, feeds, structured data and project configuration.
- Preserve an audit record and rollback plan.

### 12.12 Operational runbooks

Runbooks are required before production for:

- Lost or compromised human session.
- Leaked project API key, preview credential or webhook secret.
- Incorrect or harmful publication and emergency rollback.
- Stale CDN or failed landing revalidation.
- Redis outage or cache poisoning concern.
- SQLite busy storm, corruption, disk-full or host loss.
- Failed migration.
- Failed release before/after the `current` symlink switch and verified previous-release recovery.
- PM2 daemon/process-list loss, wrong-user `PM2_HOME`, restart loop, log growth or failure after host reboot.
- Nuxt SSR, Go API or worker graceful-shutdown timeout.
- Invalid Nginx configuration, failed graceful reload or unhealthy loopback upstream.
- Backup restoration and primary-host replacement.
- Outbox/dead-letter backlog.
- AI provider outage, prompt/model rollback or unexpected spend.
- Transactional email outage.
- Domain or TLS incident.

## 13. Security, privacy and compliance

### 13.1 Security controls

- HTTPS everywhere with HSTS after domain validation.
- Secure session cookie, CSRF protection and exact admin origins.
- Argon2id passwords and optional owner/admin MFA.
- Server-side deny-by-default authorization.
- Project-scoped repository queries and negative isolation tests.
- Rate limits for login, reset, invite, AI, uploads and integration API.
- CSP, frame protection, MIME sniffing protection and safe referrer policy.
- Rich-text and model-output sanitization.
- SSR payload allowlisting so cookies, authorization headers, internal service URLs, private runtime configuration and error details are never serialized into the browser hydration payload.
- Parameterized SQL generated through `sqlc`.
- HMAC-signed, replay-resistant webhooks.
- Strict upload size/type policies.
- SSRF protection for AI and media URL retrieval.
- Safe embed allowlists and SVG rejection/sanitization.
- Dependency scanning, pinned updates and a regular patch process.
- Encrypted backups and restricted database filesystem permissions.
- Audit records and security alerts.
- Automated negative tests for tenant isolation, stored XSS, CSRF, SSRF, privilege escalation, token leakage and webhook replay.

### 13.2 Data classification

Classify at minimum:

- **Public:** published articles, public authors and public media.
- **Internal:** drafts, review comments, AI prompts, evidence packets and analytics.
- **Sensitive:** user emails, session records, invitation/reset records and audit metadata.
- **Secret:** raw session secrets, project API keys, webhook secrets and provider credentials.

Raw secrets shall never be persisted after issuance where a hash is sufficient.

### 13.3 Privacy and retention

- Define retention separately for audit events, failed AI inputs, prompt/completion logs, deleted users and soft-deleted content.
- Allow an administrator to export or delete personal account data subject to audit and legal retention.
- Minimize IP and user-agent storage; retain only what is needed for security.
- Do not send private drafts or personal data to AI providers without an explicit processing policy.
- Newsletter and comment features require separate consent, unsubscribe, moderation and deletion requirements.
- Analytics integrations must respect project/landing-site consent policy and applicable privacy law.

### 13.4 High-stakes content

Medical, legal, financial or safety-sensitive content requires a project-level policy, qualified reviewer and stronger source rules. AI must not independently provide or approve high-stakes advice.

## 14. Non-functional requirements

The following are proposed initial targets and should be confirmed after expected project and traffic volumes are known.

### 14.1 Availability and resilience

- **NFR-AVAIL-001:** The landing integration contract shall support last-known-good rendering so temporary provider downtime does not remove already-rendered landing pages.
- **NFR-AVAIL-002:** Monthly Content API availability target shall be 99.9%, excluding announced maintenance.
- **NFR-AVAIL-003:** Monthly admin availability target shall be 99.5%.
- **NFR-AVAIL-004:** Redis failure shall not prevent authoritative reads or publication.
- **NFR-AVAIL-005:** AI, email or landing-webhook failure shall not corrupt content or block manual editing.

### 14.2 Performance

- **NFR-PERF-001:** Cached Content API p95 origin latency shall be below 200 ms under the agreed baseline load.
- **NFR-PERF-002:** Uncached SQLite Content API p95 latency shall be below 500 ms for standard post and listing queries.
- **NFR-PERF-003:** Standard admin reads shall reach usable UI state in under 2 seconds on a typical broadband connection.
- **NFR-PERF-004:** Redis cache hit rate for published article/list JSON traffic should exceed 90% after warm-up.
- **NFR-PERF-005:** API response size shall be bounded; large bodies and media shall not be returned in list endpoints.

### 14.3 Publication propagation

- **NFR-PUB-001:** The Content API shall expose the new revision within 5 seconds of a successful publish under normal operation.
- **NFR-PUB-002:** Webhook delivery shall begin within 5 seconds.
- **NFR-PUB-003:** A compatible SSR/ISR landing project should expose the changed page within 60 seconds; static host build times are measured separately.
- **NFR-PUB-004:** The admin UI shall show partial failures rather than claiming complete propagation.
- **NFR-PUB-005:** Ninety-nine percent of retryable outbox/webhook deliveries should complete within 5 minutes.
- **NFR-PUB-006:** The oldest pending publication outbox event shall alert at 2 minutes and page an operator at an agreed critical threshold.

### 14.4 Accessibility and usability

- **NFR-A11Y-001:** The admin interface shall target WCAG 2.2 AA.
- **NFR-A11Y-002:** All workflows shall be keyboard accessible.
- **NFR-A11Y-003:** Light and dark themes shall meet text and control contrast requirements.
- **NFR-A11Y-004:** The editor shall preserve semantic headings and provide alt-text guidance.
- **NFR-A11Y-005:** Destructive operations shall require clear confirmation and explain recovery behavior.

### 14.5 Capacity assumptions

Initial design validation should cover at least:

- 100 projects.
- 100,000 total content items and their revisions.
- 50 concurrent admin users.
- 1,000 publish/update events in an hour during a migration or campaign.
- 1 million Content API requests per month before CDN-rendered page traffic.
- 100 MB maximum source document only if asynchronous processing and storage policies allow it; standard image limits shall be much lower.

These are design test targets, not guaranteed contractual limits.

### 14.6 Development and deployment

- **NFR-DEPLOY-001:** `task dev` shall start a working local Nginx, Nuxt SSR, Go API, Go worker, Redis and email-capture stack through Docker Compose without requiring globally installed Go or PNPM.
- **NFR-DEPLOY-002:** Stopping the normal development stack shall preserve its named SQLite volume; any volume-destructive reset shall require explicit confirmation.
- **NFR-DEPLOY-003:** Production Nginx shall be the only public application ingress and shall route only to loopback/private Nuxt and Go ports.
- **NFR-DEPLOY-004:** PM2 shall supervise exactly one Nuxt SSR, one Go API and one Go worker instance for the MVP in fork mode; Nginx, Redis and Litestream shall not run under PM2.
- **NFR-DEPLOY-005:** Production artifacts shall be built before reaching the host, checksummed and addressed by one immutable release ID; the production host shall not deploy mutable branch state.
- **NFR-DEPLOY-006:** PM2 process declarations shall contain no secret values, run as one dedicated non-root user, persist across reboot and have bounded restart, memory, shutdown and log-retention policies.
- **NFR-DEPLOY-007:** `task deploy:prod RELEASE=<id>` shall release the compatible Nuxt and Go outputs together and enforce preflight, worker drain, verified backup, migration lock, atomic activation, health checks and release recording.
- **NFR-DEPLOY-008:** A failed application activation shall restore the prior compatible release without deleting data or running an unapproved destructive down migration.
- **NFR-DEPLOY-009:** Nuxt SSR shall remain a presentation layer; direct SQLite access, final authorization decisions and duplicated write-domain APIs are prohibited.

## 15. Observability

### 15.1 Logs

Use structured JSON logs with:

- Timestamp.
- Severity.
- Service and environment.
- Request or job ID.
- Project ID.
- Actor user/API-key ID where safe.
- Route or job type.
- Outcome and duration.
- Sanitized error category.

Do not log authorization headers, cookies, passwords, raw tokens, reset links, provider keys or unredacted confidential prompts.

### 15.2 Metrics

Track:

- HTTP request rate, errors and latency.
- Nginx upstream errors and latency split between Nuxt SSR and the Go API.
- PM2 process state, unexpected restart count, uptime, memory and event-loop health for Nuxt.
- Authentication failures and rate-limit decisions.
- SQLite query latency, busy errors, WAL/checkpoint health and database size.
- Redis latency, hits, misses, evictions, memory and fallback count.
- Outbox age, pending events, retries and dead-letter events.
- Webhook latency, attempts and failure rate.
- Publish-to-API and publish-to-landing propagation time.
- Backup age, success and restore verification.
- Media processing failures.
- AI job latency, token use, cost, cancellation, provider error and reviewer acceptance.
- Email delivery failure and bounce rate.

### 15.3 Alerts

Alert on:

- Elevated 5xx or authorization error rate.
- Sustained SQLite busy/locked errors.
- Disk space or inode exhaustion.
- Redis eviction surge or unavailability.
- Outbox oldest-event age over threshold.
- Repeated webhook failure.
- Backup age beyond RPO.
- Unexpected AI spend or provider error spike.
- Certificate or domain validation problems.
- Any required PM2 process offline/restarting, saved process-list mismatch or failed post-reboot health check.
- Nginx upstream failure or sustained Nuxt SSR readiness failure.
- Failed or partially activated production release.
- Discovery/feed-data manifest generation or landing-delivery failure.

## 16. Commonly forgotten requirements

The following requirements are explicitly included because they are frequently discovered too late.

| Forgotten area | Risk if omitted | Required treatment |
|---|---|---|
| Article project ownership | Cross-project disclosure or orphaned data | Non-null immutable `project_id`, scoped queries and composite foreign keys |
| Old slugs | Lost backlinks and 404s | Permanent redirect history and collision checks |
| Unpublish behavior | Stale search results or accidental deletion | JSON removal/tombstone plus landing 404/410/replacement and sitemap removal |
| Approved revision binding | Unreviewed edits become public | Approval tied to revision ID and content hash |
| Published edit isolation | Live page changes during drafting | New draft while old revision remains live |
| Scheduling timezone/DST | Posts publish at the wrong time | Store UTC plus project timezone and test DST gaps/duplicates |
| Cache race after publish | Old content reappears | Monotonic version guards and immutable body keys |
| Partial propagation | CMS says “published” while site is stale | Destination-level status and retry UI |
| Webhook replay/order | Duplicate or stale builds | HMAC, timestamps, event IDs and publication versions |
| Preview indexing | Draft leaks into search | `no-store`, `noindex`, short-lived revision token |
| Staging indexing | Test domain competes with production | `index: false` and exclusion from production discovery manifest |
| Duplicate cross-project content | Cannibalization and unclear canonical | Explicit destination copy plus canonical original or material adaptation requirement |
| Honest update dates | False freshness signals | Material-change rule and visible change notes |
| Author departure | Broken bylines or deleted history | Public author snapshot independent of login status |
| Corrections | Trust and legal risk | Correction note, history and correction policy |
| Media rights | Copyright exposure | Creator, license, source and usage metadata |
| Image dimensions/variants | Poor layout and rich-result eligibility | Explicit dimensions and representative variants |
| Asset deletion | Broken articles | Usage graph and guarded delete |
| Broken external sources | Unsupported claims | Periodic link check and source status |
| Source content changes | Evidence becomes unverifiable | Access date and optional archive reference where lawful |
| AI hallucinated citations | Reputational damage | Claim/source verification and blocking quality gate |
| AI prompt injection | Data or secret exposure | Treat sources as untrusted; least-privilege tools |
| AI provider retention | Confidential draft leakage | Provider policy and per-project data controls |
| AI cost runaway | Unexpected bills | Per-project budgets, caps, estimates and alerts |
| Manual fallback | Provider outage stops writers | All core workflows work without AI |
| Email deliverability | Users cannot join or reset | Verified sender, bounce monitoring and resend flow |
| First-owner bootstrap | Default admin vulnerability | One-time CLI or expiring invite |
| Session/cache coupling | Redis flush logs everyone out | SQLite-authoritative sessions |
| Autosave conflicts | Writers overwrite each other | Revision/version check and conflict warning |
| Browser back/navigation | Draft loss | Local working buffer plus server autosave recovery |
| Idempotency | Duplicate publish or media jobs | Idempotency keys and event IDs |
| Job cancellation | Expensive jobs continue invisibly | Cancellation state and cooperative checks |
| Migration failure | Deployment corrupts service | Pre-migration backup and tested recovery |
| Mutable production build | Server receives different code than CI tested | Prebuilt checksummed artifacts and immutable release ID |
| Database inside release | Symlink rollback selects or loses the wrong database | Keep SQLite in `/srv/seoblog/shared/data`, outside every release |
| Wrong PM2 user/home | Deploy sees a different process list or reboot restores nothing | One dedicated user for startup, deploy, save and recovery |
| Unsaved PM2 state | Applications stay down after host reboot | Verified startup integration, `pm2 save` and reboot test |
| PM2 log growth | Root disk fills and SQLite fails | Structured logs, rotation, retention and disk alerts |
| PM2 cluster over SQLite | Duplicate workers and write contention | One fork-mode API and worker; scale only after explicit redesign |
| Nuxt becomes a second backend | Authorization and validation drift | SSR presentation/BFF only; all authoritative domain APIs remain in Go |
| SSR hydration leakage | Cookie, secret or internal configuration reaches browser HTML | Allowlisted serialized state and automated secret/payload tests |
| Nginx route collision | `/api` reaches Nuxt or admin HTML reaches Go | Exact route precedence and end-to-end routing tests |
| Invalid Nginx release | A configuration reload takes down ingress | Versioned config, `nginx -t`, graceful reload and previous config |
| Local volume deletion | Developer loses drafts/test fixtures | Normal down preserves volume; confirmed `dev:reset` is the only reset path |
| Backup without restore test | False sense of safety | Quarterly isolated restoration |
| Media backup mismatch | DB restores with missing images | B2 file-version history and database-to-object consistency checks |
| Storage exhaustion | SQLite and uploads fail | Disk alerts, quotas and cleanup policy |
| Reserved URL collisions | Blog route conflicts | Reserved-slug registry and pre-publish validation |
| Orphaned articles | Valuable pages are undiscoverable | Inbound relationship report and landing-navigation verification |
| Taxonomy sprawl | Duplicate/thin archives | Aliases, merge workflow and unused-term reporting |
| H1/raw HTML body blocks | Broken semantics or stored XSS | Title-only H1 and raw HTML disabled by default |
| FAQ/schema mismatch | Misleading structured data | FAQ answers must be visible in the rendered article |
| Wide/inaccessible tables | Poor mobile and assistive use | Header semantics and responsive rendering contract |
| Contributor ordering | Incorrect byline/credit | Primary author plus ordered co-authors and explicit roles |
| Series ordering | Unstable navigation | Persistent unique position within a project series |
| Pagination stability | Duplicate/missing list results | Cursor pagination with deterministic sort |
| Locale/hreflang mistakes | Wrong regional pages | Translation group validation and reciprocal hreflang |
| API deprecation | Landing projects break | Versioning, changelog and migration window |
| Data portability | Vendor lock-in and project exit risk | Full documented export including redirects/media metadata |
| Audit retention | No incident evidence or privacy excess | Defined retention and protected export |
| Analytics consent | Privacy/compliance exposure | Project/landing-site consent and minimal default tracking |
| Accessibility | Editor and output exclude users | WCAG AA and semantic-output validation |
| Dependency lifecycle | Known vulnerabilities | Automated scanning and regular upgrade policy |
| Domain ownership | Bad canonical or webhook target | Verify domains and restrict redirect/webhook URLs |
| SSRF through webhooks | Backend reaches private services | URL validation, private-network blocks and allowlists |
| Content security policy | Rich content enables XSS | Sanitization plus CSP and safe embeds |
| Search crawler vs training crawler | Unintended crawler policy | Separate, documented robots controls |
| Legal disclosures | Affiliate/sponsored content risk | Visible disclosure blocks and per-project policy |
| Content review expiry | Old facts remain live | Review-due dates, reminders and stale-content reports |
| Two simultaneous publishers | A stale revision wins | Transactional version check and one monotonic publication pointer |
| Missed scheduled job | Downtime permanently skips publication | Durable UTC schedule claimed after restart |
| Server clock drift | Bad schedules, signatures and expiry | NTP synchronization and clock-health monitoring |
| Writer edits during AI run | AI overwrites newer human work | Bind job to source hash and require explicit merge |
| Silent AI fallback | Private content reaches weaker provider policy | Pre-approved fallback policies and visible provenance |
| Provider/model deprecation | Generation suddenly breaks or changes quality | Versioned routing, evaluation, canary and rollback |
| Project/domain deletion | Active keys/jobs continue operating | Dependency report, delayed deletion and credential revocation |
| Final project owner removal | Unmanageable tenant | Ownership transfer required before removal or demotion |
| Shared or unrotatable API key | One leak interrupts every consumer | Multiple named keys and overlapping rotation |
| Email DNS | Invites/resets land in spam | SPF, DKIM, DMARC and bounce monitoring |
| Upload decompression bomb | Worker memory/disk exhaustion | Pixel/frame/archive expansion limits |
| SVG active content | Stored XSS through media | Reject or dedicated sanitization |
| Domain change | Canonicals, redirects and feeds diverge | Verified migration workflow and one-hop redirects |
| Backup credentials | App compromise deletes recovery data | Separate least-privilege/locked backup storage |

### 16.1 Mandatory edge-case test matrix

Implementation test plans shall include:

- A Project-B route receives an Article ID, revision ID, author ID, taxonomy ID, source ID or asset ID that belongs to Project A.
- A database write attempts to attach a Project-A revision to a Project-B publication.
- An import or background job omits `project_id` or tries to change an existing Article’s project.
- A cross-project copy is requested by a user who can read the source but cannot create in the destination, and vice versa.
- Two editors save changes based on the same draft version.
- Two administrators attempt to publish different revisions simultaneously.
- The SQLite publish commits but Redis, CDN or the landing webhook fails.
- A stale worker processes an older event after a newer event.
- Redis is flushed, evicts hot keys or becomes unreachable.
- The service is offline at a scheduled publication time.
- A schedule falls inside a daylight-saving gap or duplicate hour.
- A project timezone changes after posts have been scheduled.
- Slugs differ only by case, punctuation, percent encoding or Unicode normalization.
- A proposed redirect creates a loop or a multi-hop chain.
- An author, tag, CTA or asset is removed while referenced by a published revision.
- A project domain changes while old URLs are still indexed.
- A project API key or preview URL leaks and is revoked.
- A writer edits the source revision while an AI job is running.
- An AI provider times out after streaming a partial result.
- SQLite returns busy, the disk fills or the process crashes during a publication transaction.
- A landing revalidation fails after the Content API already exposes the new revision.
- An article is unpublished while browsers and CDN nodes retain cached copies.
- A project deletion is requested while it has active keys, memberships, schedules, webhooks and published URLs.
- The final project owner attempts to remove or demote themselves while another ownership transfer races.
- Two keys for one project remain active during rotation and only the intended old key is revoked.
- A backup restores successfully but contains pending outbox deliveries from before the failure.
- `task dev:down` and a subsequent `task dev` preserve local SQLite data, while `task dev:reset` requires confirmation and removes only the intended Compose volume.
- Local Nginx sends `/api/*` to Go, sends UI routes and HMR upgrades to Nuxt and never exposes Redis.
- The Docker stack starts with Redis unavailable and the Go API degrades safely instead of corrupting or refusing authoritative SQLite operations.
- A deployment fails before activation, after migration and after the `current` symlink switch; each case follows its documented recovery path.
- A migration is exercised with the previous release still serving traffic and proves the required backward compatibility.
- PM2 is invoked as the wrong operating-system user, its saved process list is absent and the host reboots; monitoring and the runbook detect and recover each case.
- A proposed Nginx configuration is invalid; `nginx -t` blocks activation and the current ingress remains available.
- Nuxt SSR, the Go API and the worker each exceed their graceful-shutdown window or enter a restart loop without exhausting disk through logs.

## 17. Suggested API and event contracts

### 17.1 Human authentication

```http
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/auth/me
GET  /api/v1/auth/csrf
POST /api/v1/auth/forgot-password
POST /api/v1/auth/reset-password
POST /api/v1/invitations/{token}/accept
```

### 17.2 Projects, memberships and API keys

```http
GET    /api/v1/projects
POST   /api/v1/projects
GET    /api/v1/projects/{projectID}
PATCH  /api/v1/projects/{projectID}
POST   /api/v1/projects/{projectID}/suspend
POST   /api/v1/projects/{projectID}/archive
GET    /api/v1/projects/{projectID}/deletion-impact
DELETE /api/v1/projects/{projectID}

GET    /api/v1/projects/{projectID}/members
POST   /api/v1/projects/{projectID}/invitations
PATCH  /api/v1/projects/{projectID}/members/{userID}
DELETE /api/v1/projects/{projectID}/members/{userID}

GET    /api/v1/projects/{projectID}/api-keys
POST   /api/v1/projects/{projectID}/api-keys
POST   /api/v1/projects/{projectID}/api-keys/{keyID}/rotate
POST   /api/v1/projects/{projectID}/api-keys/{keyID}/revoke
```

Creating or rotating a key returns its raw secret once. Later list/get responses expose only its prefix, label, scopes, environment, timestamps and revocation state.

### 17.3 Project articles

```http
GET    /api/v1/projects/{projectID}/articles
POST   /api/v1/projects/{projectID}/articles
GET    /api/v1/projects/{projectID}/articles/{articleID}
GET    /api/v1/projects/{projectID}/articles/{articleID}/revisions
GET    /api/v1/projects/{projectID}/articles/{articleID}/revisions/{revisionID}
POST   /api/v1/projects/{projectID}/articles/{articleID}/revisions
POST   /api/v1/projects/{projectID}/revisions/{revisionID}/submit
POST   /api/v1/projects/{projectID}/revisions/{revisionID}/request-changes
POST   /api/v1/projects/{projectID}/revisions/{revisionID}/approve
POST   /api/v1/projects/{projectID}/articles/{articleID}/publish
POST   /api/v1/projects/{projectID}/articles/{articleID}/schedule
POST   /api/v1/projects/{projectID}/articles/{articleID}/unpublish
POST   /api/v1/projects/{projectID}/articles/{articleID}/rollback
POST   /api/v1/projects/{projectID}/articles/{articleID}/copy-to-project
```

Every handler resolves the membership/permission for `{projectID}` first and calls a project-scoped service/repository method. If `{articleID}` exists under another project, the response is the same not-found result as a nonexistent ID; it shall not reveal the other project’s existence. `copy-to-project` requires source-read permission, destination-create permission and returns a new destination Article ID.

Revision creation includes the exact `baseRevisionId` loaded by the editor. If a newer revision has already been committed, the API returns a conflict instead of recording false lineage or silently overwriting concurrent work.

### 17.4 AI jobs

```http
POST /api/v1/projects/{projectID}/ai/jobs
GET  /api/v1/projects/{projectID}/ai/jobs/{jobID}
POST /api/v1/projects/{projectID}/ai/jobs/{jobID}/cancel
GET  /api/v1/projects/{projectID}/ai/jobs/{jobID}/events
```

AI results create proposals or draft revisions; there is no AI publish endpoint.

### 17.5 Event types

```text
content.published
content.updated
content.unpublished
content.restored
content.slug_changed
content.redirect_created
content.archived
project.settings_changed
project.discovery_manifest_changed
```

Each publication event includes `event_id`, `project_id`, `content_id`, `revision_id`, `publication_version`, canonical URL, prior URL where relevant and event time.

## 18. Acceptance criteria

### 18.1 Authentication and isolation

- An invited user can set a password, log in, remain logged in through a secure cookie and log out.
- Reset links are single-use and revoke existing sessions.
- Creating a project atomically gives its creator the active `project_owner` membership.
- A project owner can assign several writers, editors and reviewers.
- One user can be a writer in Project A and an editor in Project B.
- A user cannot list or access an unassigned project, including through guessed IDs.
- Removing Project A membership immediately denies A while preserving the user’s Project B access and historical Project A attribution.
- The final active project owner cannot remove or demote themselves until ownership is transferred.
- A writer cannot approve, publish, manage members or create project API keys.
- A Project-A user or key cannot retrieve any Project-B record, including by guessed IDs.
- Requesting a Project-A Article ID through a Project-B admin route returns the same not-found response as an unknown ID and reveals no Project-A metadata.
- Two production keys for one project can work simultaneously, and revoking one does not invalidate the other.
- The Content API derives project scope exclusively from the key and never from an untrusted request selector.
- Raw keys in URLs, SQLite, logs, Redis keys, analytics and browser bundles are detected in automated tests or reviews.

### 18.2 Editorial workflow

- A writer can create, autosave, submit and revise an article.
- Creating an Article under Project A persists a non-null `project_id = A`; no Article can be created without an authorized selected project.
- Attempts through the API, repository layer or direct migration test to change an Article’s `project_id` are rejected.
- SQLite rejects a Project-B revision, publication, author, taxonomy, source, media or relationship reference to a Project-A Article.
- Copying an Article from Project A to Project B creates a new Article ID, independent revision/publication history and an audited origin/canonical decision.
- A reviewer can request changes and approve an exact revision.
- A project owner can self-approve only after solo-owner mode is explicitly enabled; project administrators, editors, reviewers and writers cannot use that exception.
- Editing after approval creates a new unapproved revision.
- Publishing exposes only the approved revision.
- Rollback restores an earlier approved revision without deleting history.
- Publishing is rejected unless the article has exactly one primary category.
- Category creation and moves reject a fourth hierarchy level, duplicate project-wide category slug, self-parenting, cycles and cross-project parents.
- A parent-category article query includes descendant-category assignments by default, while exact-category mode excludes them.
- The committed delivery exposes dedicated briefs, AI prompts and editorial checklists for standard articles, guides, tutorials and comparisons; every other supported type remains usable through the generic workflow.
- The body cannot contain H1 or raw HTML blocks under the default policy.
- Public FAQ structured-data inputs correspond to visible FAQ content.
- Public citations, disclosures and correction notices survive revision and rollback history correctly.

### 18.3 Delivery and cache

- A project API key returns only its project’s published posts.
- The project-key JSON API never returns a draft.
- Publish, update, slug change and unpublish correctly update direct-post, listing, feed-data, discovery and redirect JSON results.
- Redis loss causes a temporary performance reduction, not data loss or incorrect authorization.
- An old in-flight cache fill cannot overwrite a newer published pointer.
- Failed landing webhooks are visible and safely retryable.

### 18.4 SEO and LLM readiness

- The provider returns all required content/SEO data as JSON, and the reference landing integration renders complete article HTML and metadata in the initial response.
- Canonical, robots, dates, authors, images and JSON-LD match visible content.
- Old slugs redirect permanently.
- Staging and preview URLs are non-indexable.
- Sitemaps include only canonical indexable publications and use honest `lastmod`.
- Duplicate cross-project publishing requires an explicit canonical decision.
- Sources and material claims are visible or appropriately documented.

### 18.5 AI

- AI cannot publish through API or UI permissions.
- Every AI output records provider/model/prompt/evidence provenance.
- Unsupported material claims and invented citations block approval.
- Unresolved placeholders block approval.
- AI provider failure leaves manual editing and publication operational.
- Per-project budgets and cancellation work.
- Human reviewers can see the evidence and quality-check results before approval.

### 18.6 Operations

- Automated backup completes within the defined RPO.
- A documented isolated restore test succeeds.
- Outbox, webhook, Redis, AI and backup failures produce observable alerts.
- Secrets do not appear in structured logs.
- A deployment can be rolled back without destructive database commands.
- From a clean checkout, `task dev` starts Nginx, Nuxt, API, worker, Redis and local email services; normal shutdown/restart preserves the development database.
- One immutable release command deploys compatible Nuxt SSR and Go artifacts, validates them through Nginx and records the release result.
- An injected post-activation failure restores the previous compatible application release without deleting or replacing SQLite data.
- A production-like reboot restores the three named PM2 applications under the intended non-root user and leaves Nginx, Redis and Litestream healthy.
- Nginx routing tests prove that admin UI, admin API and Content API traffic reach only their intended upstreams.

### 18.7 Production release gates

Production launch requires evidence that:

- Tenant isolation and every role boundary pass automated positive and negative tests.
- Project creation, membership changes, last-owner protection and multi-key rotation pass concurrency and authorization tests.
- Article ownership tests prove non-null/immutable `project_id`, project-scoped repository signatures and composite foreign-key rejection of cross-project revisions, publications and dependencies.
- Publication/cache failure tests always converge on the latest committed revision.
- Preview and draft content cannot enter project-key results, production feed-data or discovery manifests.
- Representative structured data, canonicals, dates, redirects and locales validate against rendered pages.
- Redis loss produces slower service rather than incorrect or unavailable authoritative content.
- A clean-host backup restoration meets the declared RPO/RTO or has a formally accepted exception.
- Jobs/outbox delivery is retryable, idempotent, lease-safe and safe when processed out of order.
- Security testing covers stored/reflected XSS, CSRF, SSRF, upload parsing, token storage, privilege escalation and webhook replay.
- The admin UI meets the accessibility target for login, editing, review and publish workflows.
- Load testing represents the capacity assumptions and includes publication bursts.
- Dashboards, alerts and the operational runbooks in Section 12.12 are usable by the responsible operator.
- Email domain authentication and invitation/password-reset delivery have been tested.
- Docker Compose onboarding, named-volume preservation/reset, HMR through local Nginx and Redis-failure behavior pass on every supported developer operating system.
- The checked-in HTTP stack matches the selected Fiber v3 + Huma decision, or an approved architecture decision records and tests the accepted deviation.
- `task contracts:generate` produces `contracts/openapi/openapi.yaml`, and the TypeScript content client is regenerated or validated from the same contract without drift.
- A clean-host rehearsal verifies PM2 startup/save, process ownership, graceful shutdown, bounded logs, Nginx routing, deployment locking, exact-process rollback and recovery after reboot.
- The production release is checksummed, the pre-migration B2 recovery point is verified and no build occurs from mutable source on the host.
- Nuxt SSR has no database credential or direct domain-write path, and the public Nginx configuration exposes no PM2 or Redis port.

## 19. Single committed delivery

The product shall be implemented and accepted as one complete delivery. The workstreams below may be sequenced internally for engineering dependency management, but they are not separate product phases or independently deferred releases.

### Current implementation checkpoint

As of PRD v1.9, the checked-in foundation includes root Taskfile orchestration, Docker Compose services for Nginx, Nuxt, Go API, Go worker, Redis and Mailpit, an `admincli` for migrations/bootstrap/OpenAPI generation, embedded SQLite migrations, a Nuxt admin shell with project-scoped pages, invite/session/member/API-key/audit flows, article/category/author/series workflow slices, scheduled publication worker behavior, project-scoped revision history and comparison, article rollback, audited cross-project article copying with an explicit canonical/adaptation decision through the admin API and admin UI, protected Content API routes and a server-only TypeScript content client. This checkpoint is implementation evidence, not a scope reduction.

Still-required committed scope includes richer structured editing, autosave conflict handling, media/B2 processing, source/claim/disclosure/correction workflows, AI evidence/jobs/provenance, preview tokens, webhook delivery/replay, Redis cache-aside behavior, full SEO/discovery polish, production release automation, backup/restore automation, observability and the Fiber v3 alignment gate unless an approved architecture decision changes it.

### Foundation workstream

- Monorepo with independent Go backend and Nuxt admin applications, one Go module, generated contract/client and root Taskfile orchestration.
- Docker Compose development stack with Nginx, Nuxt, API, worker, Redis, Mailpit, health checks and a persistent local SQLite volume.
- Nuxt Nitro SSR shell and design system with light/dark themes.
- SQLite, migrations, `sqlc` and backups.
- Project tenant/workspace grouping model.
- Project-owned Article keys, immutable ownership constraints and cross-project negative test harness.
- Invite-only accounts, sessions, CSRF, project memberships and project-scoped roles.
- Audit framework.
- CI-built immutable release bundle, production Nginx routes, PM2 ecosystem/startup/logging, safe one-command deployment and basic observability.

### Complete product workstream

- Authors, content items, revisions and structured editor.
- Autosave, conflict warning, revision diff and rollback.
- Hierarchical categories/subcategories, flat tags, manual related articles and media.
- Review, change requests, approval and publication.
- Scheduling, preview and basic review-due/content-expiry reminders.
- Human-managed sources, citations, claims, disclosures and correction notices.
- Project voice profiles and AI evidence packets built on the human-managed source model.
- AI outline, section draft, rewrite, critique and metadata assistance.
- Automated claim/source extraction, quality gates and internal provenance.
- AI job progress, cancellation, project quotas, budget controls and evaluation.
- Internal-link and content-cannibalization suggestions.
- Multiple named project API keys and the protected Content API.
- Landing revalidation webhook.
- Redis cache-aside and durable outbox.
- Versioned article JSON, canonical/JSON-LD inputs, redirect records, discovery manifest and feed data.
- Search Console/Bing Webmaster data imports.
- Landing delivery acknowledgements and optional IndexNow status reporting.
- Advanced stale-content and correction workflows.
- Multilingual editorial UI and translation workflow.
- Series, topic clusters and advanced relationship analysis.
- Configurable editor/reviewer permission refinements.
- Optional MFA.
- More advanced provider-side editorial analytics and landing integration diagnostics.

## 20. Success metrics

Product metrics:

- Median time from approved brief to approved article.
- Percentage of drafts approved without a second major rewrite.
- Percentage of published articles with complete author, source and structured-data fields.
- Stale-content review completion rate.
- Cross-project duplication/cannibalization warnings resolved before publication.
- Organic impressions, indexed canonical pages and search conversions by project.
- Bing/other available citation metrics and referral traffic from answer engines.

Reliability metrics:

- Content API availability and latency.
- Cache hit rate.
- Publish-to-landing propagation time.
- Outbox and webhook failure rate.
- Backup freshness and restoration success.
- Authentication abuse and cross-project authorization test results.

AI metrics:

- Claim-verification pass rate.
- Broken or fabricated citation rate.
- Human reviewer acceptance/change-request rate.
- Human editing time and meaningful edit rate.
- Cost and latency per approved article.
- Provider failure and fallback rate.

Rankings and LLM citations are observed outcomes, not guaranteed product acceptance criteria.

## 21. Risks and mitigations

| Risk | Mitigation |
|---|---|
| AI produces plausible but false content | Evidence packets, claim extraction, human verification and blocking checks |
| Many low-value pages damage sites | No auto-publish, unique-input rule, duplication detection and editorial quotas |
| Project API key leaks from browser code | Server-only integration, landing-side proxy for runtime reads and secret scanning |
| SQLite write contention | Short transactions, WAL, one primary host, metrics and migration triggers |
| Cache returns stale revision | Versioned pointers, generation keys, durable outbox and TTL |
| Landing build fails after CMS publish | Propagation status, retries, CDN stale content and manual replay |
| Same content appears on several domains | Required canonical-original or material-adaptation decision |
| Media copyright complaint | Source/license/credit metadata and takedown workflow |
| Admin account takeover | Secure sessions, rate limits, strong passwords, optional MFA and audit |
| Nuxt SSR becomes a second source of business logic | Strict presentation/BFF boundary, no DB credential and contract tests against the Go API |
| PM2 restarts the wrong or stale release | Dedicated user, versioned ecosystem file, atomic `current` link, health checks and saved-list verification |
| Release migration leaves application incompatible | Expand-compatible migrations, verified B2 recovery point and previous-release/forward-fix runbook |
| Provider or infrastructure lock-in | Service interfaces, portable content export and provider-independent AI jobs |
| Unexpected AI spend | Project budgets, estimates, hard caps and alerts |
| Backup exists but is unusable | Quarterly restore drill and recorded recovery outcome |

## 22. Open decisions

The following require business or deployment input before implementation:

- Whether the initial product supports one workspace containing many projects or multiple independent customer workspaces.
- Expected number of projects, articles, monthly publishes and Content API requests.
- Production hosting provider and primary region.
- Whether Redis is managed or self-hosted.
- Image-transformation and media-CDN implementation; Backblaze B2 is selected as authoritative media storage.
- Transactional email provider and sender domain.
- Initial AI providers, data-retention requirements and monthly budget.
- Required locales and translation workflow.
- Default public AI-assistance disclosure policy.
- Audit, deleted-content and AI-log retention periods.
- Which landing frameworks need maintained reference adapters or examples.
- Required legal/compliance regimes based on customers and operating regions.

## 23. Recommended decisions

Unless an open decision changes the constraints, the implementation should proceed with:

- Go Fiber v3 plus Huma.
- Nuxt admin with Nitro SSR and TipTap/ProseMirror.
- One monorepo containing independent Go backend and Nuxt admin applications; one Go module, a PNPM workspace for frontend/client packages, separate API/worker/CLI/Nitro SSR artifacts and landing applications in separate repositories.
- Docker Compose as the local development runtime, with one root `task dev` command and a named local SQLite volume.
- Nginx as the production ingress, routing the Nuxt admin SSR server and Go APIs without exposing their loopback ports.
- PM2 in fork mode supervising one Nuxt Nitro SSR process, one native Go API process and one native Go worker process; systemd or managed services supervise Nginx, Redis and Litestream.
- One immutable, prebuilt release deployed through `task deploy:prod`, with worker drain, verified B2 recovery point, migration lock, atomic symlink activation, health checks and previous-application rollback.
- SQLite WAL plus `sqlc`; no PocketBase.
- Redis cache-aside with a SQLite transactional outbox.
- Server-side opaque sessions; no browser JWT.
- Project as the tenant/security boundary, with many-to-many project memberships and per-project roles.
- Every Article belongs to exactly one Project through a required immutable `project_id`; revisions, publications and dependencies are protected by project-scoped queries and composite foreign keys.
- Multiple named, project- and environment-bound API keys, separated by consumer and supporting overlap during rotation.
- One Article schema with typed templates, TipTap JSON as the editable source and both structured blocks and sanitized HTML in the published JSON contract.
- Dedicated workflows for standard articles, guides, tutorials and comparisons; other supported article types initially use the shared generic workflow.
- No separate article folders in the committed delivery.
- Hierarchical categories/subcategories with a three-level maximum, project-unique category slugs, one required primary category and descendant-inclusive parent filtering; flat tags, manual related articles, series and topic clusters are included in the committed delivery; vector recommendations remain an uncommitted future consideration.
- Project-owner self-approval only through explicitly enabled solo-owner mode.
- A headless, JSON-only Content API consumed through SSR/SSG/ISR, with canonical HTML owned by each project domain.
- Manual and provider-independent AI-assisted writing in the single committed delivery, with evidence grounding and mandatory human approval.
- Separate Backblaze B2 buckets for media and SQLite backups, with independent bucket-scoped application keys, SSE-B2, lifecycle rules and backup-snapshot Object Lock.
- One primary region and host for the initial SQLite deployment.
- CDN-first public delivery and explicit restore/scale triggers.

## 24. Research references

- [Fiber v3 documentation](https://docs.gofiber.io/)
- [Fiber context behavior](https://docs.gofiber.io/guide/go-context/)
- [Fiber session and cookie guidance](https://docs.gofiber.io/blog/fiber-v3-sessions-cookies/)
- [Huma Fiber adapter](https://pkg.go.dev/github.com/danielgtaylor/huma/v2/adapters/humafiber)
- [Huma OpenAPI generation](https://huma.rocks/features/openapi-generation/)
- [Go server module layout](https://go.dev/doc/modules/layout)
- [chi documentation](https://github.com/go-chi/chi)
- [Nuxt UI](https://ui.nuxt.com/)
- [Nuxt 4 directory structure](https://nuxt.com/docs/4.x/directory-structure/nuxt)
- [Nuxt Node/Nitro deployment and PM2](https://nuxt.com/docs/3.x/getting-started/deployment)
- [Nuxt UI components and editor](https://ui.nuxt.com/docs/components/)
- [TipTap Nuxt integration](https://tiptap.dev/docs/editor/getting-started/install/nuxt)
- [Docker Compose development specification](https://docs.docker.com/reference/compose-file/develop/)
- [Docker Compose file-watch workflow](https://docs.docker.com/compose/how-tos/file-watch/)
- [Docker Compose named volumes](https://docs.docker.com/reference/compose-file/volumes/)
- [Docker Compose dependency readiness](https://docs.docker.com/compose/how-tos/startup-order/)
- [Nginx HTTP reverse-proxy module](https://nginx.org/en/docs/http/ngx_http_proxy_module.html)
- [Nginx WebSocket proxying](https://nginx.org/en/docs/http/websocket.html)
- [PM2 ecosystem application declaration](https://pm2.keymetrics.io/docs/usage/application-declaration/)
- [PM2 startup restoration and process saving](https://pm2.keymetrics.io/docs/usage/startup/)
- [PM2 graceful shutdown](https://pm2.keymetrics.io/docs/usage/signals-clean-restart/)
- [PM2 log management](https://pm2.keymetrics.io/docs/usage/log-management/)
- [PM2 support for native binaries and non-Node runtimes](https://pm2.keymetrics.io/docs/usage/bun-deno/)
- [PM2 deployment lifecycle](https://pm2.keymetrics.io/docs/usage/deployment/)
- [SQLite write-ahead logging](https://www.sqlite.org/wal.html)
- [SQLite Online Backup API](https://www.sqlite.org/backup.html)
- [Litestream restore and integrity checking](https://litestream.io/reference/restore/)
- [Litestream replication to Backblaze B2](https://litestream.io/guides/backblaze/)
- [Litestream S3-compatible provider guidance](https://litestream.io/guides/s3-compatible/)
- [Backblaze B2 S3-compatible API](https://www.backblaze.com/docs/cloud-storage-s3-compatible-api)
- [Backblaze B2 application keys](https://www.backblaze.com/docs/cloud-storage-application-keys)
- [Backblaze B2 server-side encryption](https://www.backblaze.com/docs/cloud-storage-server-side-encryption)
- [Backblaze B2 Object Lock](https://www.backblaze.com/docs/cloud-storage-object-lock)
- [Private Backblaze B2 delivery through Cloudflare](https://www.backblaze.com/docs/cloud-storage-deliver-private-backblaze-b2-content-through-cloudflare-cdn)
- [PocketBase as a Go framework](https://pocketbase.io/docs/use-as-framework/)
- [Current Turso Go SDK guidance](https://docs.turso.tech/sdk/go/reference)
- [Redis cache-aside guidance for Go](https://redis.io/docs/latest/develop/use-cases/cache-aside/go/)
- [Redis Pub/Sub delivery behavior](https://redis.io/docs/latest/develop/pubsub/)
- [Redis security guidance](https://redis.io/docs/latest/operate/oss_and_stack/management/security/)
- [Transactional outbox pattern](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/transactional-outbox.html)
- [Cloudflare connectivity options](https://developers.cloudflare.com/cloudflare-one/networks/connectivity-options/)
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/)
- [OWASP REST Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/REST_Security_Cheat_Sheet.html)
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [OWASP Forgot Password Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html)
- [OWASP CSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)
- [OWASP Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html)
- [OWASP Multi-Tenant Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Multi_Tenant_Security_Cheat_Sheet.html)
- [NIST digital identity and password guidance](https://pages.nist.gov/800-63-4/sp800-63b.html)
- [Google guidance for AI features and search](https://developers.google.com/search/docs/fundamentals/ai-optimization-guide)
- [Google guidance for generative AI content](https://developers.google.com/search/docs/fundamentals/using-gen-ai-content)
- [Google helpful content guidance](https://developers.google.com/search/docs/fundamentals/creating-helpful-content)
- [Google spam policies](https://developers.google.com/search/docs/essentials/spam-policies)
- [Google Article structured data](https://developers.google.com/search/docs/appearance/structured-data/article)
- [Google sitemap guidance](https://developers.google.com/search/docs/crawling-indexing/sitemaps/overview)
- [IndexNow documentation](https://www.indexnow.org/documentation)
- [Bing AI Performance announcement](https://blogs.bing.com/webmaster/February-2026/Introducing-AI-Performance-in-Bing-Webmaster-Tools-Public-Preview)
