# SEO Blog Content Hub — Product Requirements

**Version:** 2.0
**Status:** Active scope
**Product shape:** A small, secure, headless article manager

## 1. Product goal

Provide one focused place to write and publish SEO articles for multiple projects.

The core workflow is intentionally short:

1. Create an article.
2. Edit and save it.
3. Publish it now or schedule it.
4. Update, unpublish, archive, or restore it later.

The product does not include an editorial decision pipeline, version comparison, assignments, comments, decision gates, or rollback. Publishing is an explicit action performed by a permitted human.

## 2. Product principles

- Keep the common path visible and one click away.
- Expose one current editable article to writers and publishers.
- Keep publishing separate from saving.
- Do not make optional metadata block normal writing.
- Enforce project isolation on every read and write.
- Keep public delivery stable, cacheable, and secure.
- Preserve existing customer content during schema changes.
- Prefer understandable operations over enterprise workflow machinery.

## 3. Users and permissions

The supported project roles are:

| Role | Capabilities |
| --- | --- |
| Project owner | Full project, member, content, settings, and publishing control |
| Project administrator | Manage members except ownership, content, settings, and publishing |
| Editor | Create, edit, schedule, publish, unpublish, and organize content |
| Writer | Create and edit drafts; cannot publish or manage the project |

Permissions are project-scoped. A user may have a different role in each project.

No role is trusted from browser state alone. The server resolves membership for every protected request.

## 4. Information architecture

The administration UI contains:

- Dashboard
- Projects
- Content
- Calendar
- Taxonomy
- Series
- Media
- Authors
- AI workspace
- Members
- API keys
- Integrations
- Operations
- Audit log
- Project settings

The article editor contains three sections:

- **Write:** edit and save the current article.
- **Overview:** inspect identifiers, URL, author, taxonomy, and publication details.
- **Publish:** publish or unpublish with optional scheduling and URL settings.

## 5. Article lifecycle

An article has one of these publication states:

- `unpublished`
- `scheduled`
- `published`
- `archived`

### 5.1 Create

Creating an article requires:

- Article type
- Title
- URL slug
- Primary category
- Primary author
- Body

The new article starts as `unpublished`.

### 5.2 Save

Saving updates the article's current working content. It does not make content public.

The storage layer may keep immutable save and publication snapshots for concurrency,
recovery, audit, and reliable delivery. These are implementation details and are not
presented as a user-managed workflow.

The editor must:

- Autosave recoverable working content.
- Show saving, saved, and error states.
- Prevent silent overwrites from another browser tab.
- Allow an explicit reload when a concurrent save wins.
- Clear temporary autosave data after a successful full save.

### 5.3 Publish

Publishing uses the article’s current saved content.

The primary action is one button labeled **Publish article** or **Publish changes**.

The server must:

- Confirm the caller can publish for the project.
- Confirm the project and article are active.
- Confirm a title, slug, body, primary category, and primary author exist.
- Generate the canonical URL when none is supplied.
- Set the publication state to `published`.
- Record first-published and last-modified timestamps.
- Increment the project content generation.
- Insert a durable delivery event in the same transaction.
- Return the updated article.

Publishing does not require a separate state transition or decision record.

### 5.4 Schedule

An editor may choose a future date and time. Scheduling uses the same validations as publishing and sets the state to `scheduled`.

The worker publishes due articles atomically and emits the same delivery event as immediate publishing.

### 5.5 Unpublish

Unpublishing removes the article from public API results while keeping its editable content.

### 5.6 Archive and restore

Archiving hides the article from normal administration lists and public delivery. Restoring returns it as `unpublished`.

Archive and restore actions require confirmation and are recorded in the audit log.

## 6. Editor requirements

The editor supports:

- Paragraphs
- Headings H2–H4
- Bold, italic, underline, and strike-through
- Safe links
- Ordered and unordered lists
- Quotes and code blocks
- Tables
- Images with alt text
- Figures and captions
- Callouts, takeaways, steps, comparisons, calls to action, and FAQs
- Undo and redo

Raw HTML is disabled. Links and embeds must pass server-side allow-list validation.

Optional article fields include:

- Subtitle
- Excerpt
- Short answer
- SEO title
- Meta description
- Robots directive
- Open Graph title
- Open Graph description
- Open Graph image
- Canonical URL

Optional fields belong under clearly labeled expandable sections.

## 7. Projects

Each project owns its content, taxonomy, authors, media, keys, integrations, and settings.

Project settings include:

- Name and slug
- Primary domain
- Blog base path
- Timezone
- Publisher identity
- Default author
- Default social image
- SEO title pattern
- Default robots policy
- AI provider configuration

Project statuses are `active`, `suspended`, and `archived`.

Suspended or archived projects cannot create, save, schedule, or publish content.

## 8. Taxonomy, series, and authors

- Every publishable article has exactly one active primary category.
- Tags are optional and project-owned.
- Series membership is optional and ordered.
- Every publishable article has exactly one active primary author.
- Co-authors, editors, photographers, and other public credits are optional.
- Public author data is copied into the published payload so profile changes do not unexpectedly rewrite an already delivered byline until the article is published again.

Cross-project references are rejected.

## 9. Media

Media uploads use short-lived signed upload credentials.

The system records:

- File name
- Content type
- Size
- Checksum
- Width and height when applicable
- Alt text
- Caption
- Decorative flag
- Processing state

Only ready project-owned media may be inserted into an article.

## 10. AI assistance

AI is optional writing assistance, not a workflow gate.

Supported actions may include:

- Outline suggestions
- Draft suggestions
- Rewriting selected text
- Title and description suggestions
- Source discovery
- Quality hints

AI output is always a suggestion. A human must explicitly apply it and only a permitted human may publish.

Provider keys remain server-side and encrypted at rest. Provider failures must never prevent manual writing or publishing.

## 11. Public Content API

Public endpoints expose only `published` articles.

Required routes:

```text
GET /content/v1/posts
GET /content/v1/posts/{slug}
GET /content/v1/posts/by-id/{contentID}
GET /content/v1/categories
GET /content/v1/tags
GET /content/v1/series
GET /content/v1/authors
GET /content/v1/authors/{slug}
GET /content/v1/feed-data
GET /content/v1/discovery-manifest
```

Responses include:

- Stable article ID
- Slug and canonical URL
- Title and optional summaries
- Structured body and sanitized HTML
- Taxonomy
- Author credits
- SEO and social metadata
- Publication timestamps
- Public disclosures when present

Drafts, scheduled content, archived content, autosaves, admin data, and AI data never appear in public routes.

## 12. Administration API

The primary article routes are:

```text
GET    /api/v1/projects/{projectID}/articles
POST   /api/v1/projects/{projectID}/articles
GET    /api/v1/projects/{projectID}/articles/{articleID}
PUT    /api/v1/projects/{projectID}/articles/{articleID}
DELETE /api/v1/projects/{projectID}/articles/{articleID}

POST /api/v1/projects/{projectID}/articles/{articleID}/publish
POST /api/v1/projects/{projectID}/articles/{articleID}/schedule
POST /api/v1/projects/{projectID}/articles/{articleID}/unpublish
POST /api/v1/projects/{projectID}/articles/{articleID}/restore
```

The publish request may be empty. The server defaults to the current saved article, current slug, and generated canonical URL. Optional overrides are limited to slug and canonical URL.

Every mutation requires an authenticated session, project authorization, CSRF validation, and strict JSON decoding.

## 13. Data model

The administration product exposes one current article. Its conceptual fields are:

```text
articles
  id
  project_id
  article_type
  title
  slug
  deck
  excerpt
  short_answer
  body_document_json
  sanitized_html
  plain_text
  markdown_export
  table_of_contents_json
  taxonomy_snapshot_json
  author_snapshot_json
  contributor_snapshot_json
  seo_snapshot_json
  content_hash
  publication_state
  canonical_url
  scheduled_for_utc
  first_published_at
  materially_modified_at
  publication_version
  created_by
  created_at
  updated_at
  archived_at
```

Required constraints:

- Primary key on `id`.
- Unique key on `(project_id, id)`.
- Unique key on `(project_id, slug)` for non-archived articles.
- Foreign key from `project_id` to projects.
- Project-scoped foreign keys for taxonomy, authors, media, and related content.
- A database trigger preventing an article from changing projects.

The implementation may normalize these fields across project content, immutable save
snapshots, and publication records. That internal structure must not create extra
steps for users. Existing content must be preserved during every schema transition;
removing historical storage requires a separately planned migration and backfill.

## 14. Caching and delivery events

Published reads may use Redis cache-aside storage.

Cache keys include project and article identity. Project collection keys include a monotonic content generation value.

Publishing, unpublishing, changing a slug, archiving, and restoring must:

1. Commit article state and a durable outbox event in one SQLite transaction.
2. Let the worker invalidate affected cache entries.
3. Deliver signed webhooks with retry and replay support.

The Content API remains correct when Redis or webhook delivery is unavailable.

## 15. Authentication and security

- Passwords use Argon2id.
- Sessions use secure, HTTP-only, same-site cookies.
- State-changing browser requests require CSRF protection.
- Sensitive account actions require recent reauthentication.
- Invitations and password-reset tokens are single-use and expire.
- API keys are shown once and stored only as hashes.
- Secrets are never placed in URLs or logs.
- SQLite foreign keys are enabled on every connection.
- All repository methods are explicitly project-scoped.
- Public and administrative rate limits are separate.
- Security headers include CSP, HSTS in production, frame denial, and MIME sniffing protection.

## 16. Audit and operations

The audit log records:

- Authentication events
- Member and role changes
- API-key operations
- Article create, save, publish, schedule, unpublish, archive, and restore
- Project settings changes
- Integration and webhook operations

Audit records contain actor, project, action, target, outcome, request ID, and timestamp. They never contain passwords, session secrets, raw API keys, or provider credentials.

Operations include:

- Health and readiness endpoints
- Structured logs
- Prometheus-compatible metrics
- SQLite backups with tested restore procedures
- Redis treated as disposable cache
- Graceful worker shutdown
- Idempotent outbox processing

## 17. Accessibility and responsive behavior

- Meet WCAG 2.2 AA for core authentication, writing, and publishing tasks.
- Every input has a visible label.
- Keyboard focus is visible.
- Buttons have clear action text.
- Status is not communicated by color alone.
- The editor and publishing controls work at 320 px width.
- Destructive actions require confirmation and explain their effect.
- Success and failure feedback uses live regions where appropriate.

## 18. Performance targets

- Cached public article read: p95 under 100 ms.
- Uncached public article read: p95 under 300 ms under normal load.
- Admin list and article reads: p95 under 500 ms.
- Successful publish visible through the Content API within 5 seconds.
- Normal save feedback visible within 1 second on a healthy connection.

## 19. Acceptance criteria

- A writer can create, edit, and save an article but cannot publish it.
- An editor can create, edit, save, publish, schedule, and unpublish an article.
- Publishing succeeds with a single primary button and no decision step.
- Publishing the same article again updates the public content without creating duplicate publication rows.
- Existing databases can publish after upgrading without an `ON CONFLICT` constraint error.
- The Content API exposes only published articles.
- Project B cannot read or mutate Project A content by guessing identifiers.
- A slug collision returns a clear validation error.
- Unpublishing removes the article from public routes without deleting editable content.
- Archiving and restoring preserve article content.
- Public delivery remains correct during Redis or webhook outages.
- Admin type checks, backend tests, and production builds pass in CI.

## 20. Out of scope

- Multi-language content
- Multi-stage editorial workflow
- Content assignments and discussion threads
- Editorial decision gates
- Version comparison and rollback
- Enterprise identity federation
- Public website rendering
- Billing
- Native mobile applications
