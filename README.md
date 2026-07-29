# SEO Blog CMS

Headless multi-project blog CMS and versioned JSON content API.

This repository follows the PRD structure:

- `apps/backend`: Go API, worker and admin CLI.
- `apps/admin`: Nuxt administration frontend.
- `contracts`: generated OpenAPI and shared contracts.
- `packages/content-client`: TypeScript client for landing projects.
- `infra`: Docker, Nginx, PM2 and deployment support.

The CMS does not provide a public landing page. Unauthenticated web users go to sign in; public blog HTML is rendered by external landing applications that consume the Content API from a trusted server/build environment.

## Local Development

Go and PNPM are required for direct application commands. Docker Compose is the preferred full-stack runtime.

```bash
task dev
```

Default backend settings:

- API address: `:8080`
- SQLite path: `./seoblog.db`
- Environment: `development`

Useful checks:

```bash
task backend:test
task contracts:generate
pnpm --filter @seoblog/content-client build
pnpm --filter @seoblog/admin build
```

Create the first admin owner with the one-time CLI command after migrations:

```bash
cd apps/backend
cp .env.example .env
# Edit .env and set SEOBLOG_BOOTSTRAP_PASSWORD to a strong 8+ character password.
go run ./cmd/admincli bootstrap-owner -email owner@example.com
```

Scheduled publishing requires the API and worker to run against the same database. In local direct mode, start both from `apps/backend`:

```bash
go run ./cmd/api
go run ./cmd/worker
```

The admin Articles page can create a category, create an article, approve its latest revision and schedule or publish it. The project Authors page can create and update public byline profiles while inactive authors remain hidden from the Content API. The project Members page can create one-time invitation links, update project roles and remove members while retaining at least one active owner. Invited users accept the link to activate their account, and reissuing or removing an invitation invalidates its older tokens. The project API keys page can create, rotate and revoke server-side landing credentials, with the raw secret shown once. The project Audit page lists project-scoped security and editorial events without exposing one-time secrets or stored verifiers. Due scheduled articles remain hidden from the Content API until the worker promotes them to `published`.

The protected Content API is documented at `/docs` and `/openapi.yaml` while the API process is running. Project API keys are server-only credentials and the TypeScript client refuses browser-side construction.
