# Contributing to Inventario

Thanks for your interest in contributing! Inventario is a personal inventory
management system with a Go backend and a React frontend. This guide covers how
to set up your environment, the build/test/lint commands, the migration policy,
and the conventions we expect on pull requests.

By participating in this project you agree to abide by our
[Code of Conduct](CODE_OF_CONDUCT.md). Security issues must **not** be filed as
public issues — see [SECURITY.md](SECURITY.md) for the private disclosure
process.

## Where things live

- **Backend (`go/`)** — Go 1.26+ with a Chi router and PostgreSQL.
  - `go/models` — domain models with Ptah migration annotations.
  - `go/registry` — repository-pattern implementations (PostgreSQL, in-memory).
  - `go/apiserver` — HTTP API handlers and tenant-context middleware.
  - `go/services` — business logic.
  - `go/internal` — internal utilities (validation, error handling, logging).
  - `go/backup` — export/import functionality.
  - `go/schema/migrations/_sqldata` — **generated** SQL migrations (see below).
- **Frontend (`frontend/`)** — React 19 + TypeScript + Tailwind v4 + shadcn/ui.
  - `frontend/src/app` — application shell, providers, router.
  - `frontend/src/pages` — one folder per route.
  - `frontend/src/features` — feature slices (auth, group, commodity, file, …).
  - `frontend/src/components/ui` — shadcn primitives.
  - `frontend/src/types` — OpenAPI-generated DTOs plus hand-written types.
- **End-to-end tests (`e2e/`)** — Playwright tests for full user workflows.
- **Design mock (`design-mocks/`)** — read-only mirror of the upstream
  `inventario-design` repo. **Never edit anything under `design-mocks/`**; local
  changes are wiped on the next sync. It is the canonical visual contract for the
  frontend.

For the full architecture and developer guidance, read [AGENTS.md](AGENTS.md).
Before touching anything under `frontend/src/`, also read
[`devdocs/frontend/README.md`](devdocs/frontend/README.md).

## Prerequisites

- **Go** 1.26 or higher.
- **Node.js** 24.16.0 (managed via [Volta](https://volta.sh/); the version is
  pinned in `frontend/package.json`).
- **Git**.
- **PostgreSQL** for anything beyond the in-memory backend (user/tenant CLI
  commands, integration tests, and migrations require it).

## Building

Run from the repository root:

- `make build` — build both frontend and backend.
- `make build-frontend` — build the React frontend only.
- `make build-backend` — build the Go backend with the embedded frontend.
- `make build-backend-nofe` — build the Go backend without the frontend.

## Running locally

- `make run-backend` — run the backend server on `:3333`.
- `make run-frontend` — run the React (Vite) dev server.
- `make run-dev` — run both servers concurrently.

The default in-memory database loses data on restart. To seed development data,
start the server with `--enable-seed-endpoint` (off by default, never enable in
production) and then `curl -X POST http://localhost:3333/api/v1/seed`.

## Testing

- `make test` — run all tests (Go + frontend, excluding PostgreSQL).
- `make test-go` — run Go tests (excluding PostgreSQL registry tests).
- `make test-go-postgres` — run PostgreSQL registry tests (requires the
  `POSTGRES_TEST_DSN` environment variable).
- `make test-go-all` — run all Go tests including PostgreSQL.
- `make test-frontend` — run frontend unit tests with Vitest.
- `make test-e2e` — run the Playwright end-to-end tests.

Integration tests skip automatically when `POSTGRES_TEST_DSN` is unset; there is
no build tag to enable them.

## Linting and formatting

- `make lint` — run all linters (`lint-go` + `lint-frontend`).
- `make lint-go` — run `golangci-lint` on the Go code. Prefer
  `golangci-lint run --fix` to auto-correct import order and formatting; do not
  invoke `gci`/`gofmt`/`gofumpt` binaries directly.
- `make lint-frontend` — run ESLint (`eslint .`) on the frontend.
- Frontend formatting is checked separately by Prettier. ESLint does **not**
  catch Prettier drift, so run `npm run format:check` (or `npm run format` to
  fix) from `frontend/` before pushing frontend changes.
- `npm run typecheck` (from `frontend/`) — TypeScript type checking.

## Database migrations (important)

**Never hand-write migration SQL.** Schema migrations under
`go/schema/migrations/_sqldata/` are **generated** from the Go model annotations
(Ptah `//migrator:schema:*` tags). The CI schema-drift check regenerates from
the models and fails on any mismatch, so a freehand file will not pass review.

To add or change a migration:

1. Edit the Go model in `go/models/` — add fields, indexes, and RLS policies via
   `//migrator:schema:*` annotations.
2. Run `./scripts/generate-migration.sh <descriptive_name>`. It spins up an
   ephemeral Postgres container, applies every existing migration, diffs the
   live schema against your model annotations, and writes the resulting
   `<unix-timestamp>_<name>.up.sql` / `.down.sql` pair.
3. Review the generated SQL and commit both files. If the output looks wrong,
   fix the **model annotations** and regenerate — do not edit the SQL by hand,
   and do not rename a generated file to bump its timestamp prefix.

The generator picks its own timestamp (a real UTC Unix timestamp at or before
wall-clock now); a CI guard fails the build if any prefix is in the future.

**Exception:** some changes (data backfills, multi-step `ALTER`s, custom DDL)
cannot be expressed via annotations. In those rare cases, **stop and ask the
maintainer for explicit approval before writing SQL by hand.**

## Pull request conventions

- **Branch** off `master`; do not push directly to `master`.
- Keep each PR focused on a single change. Reference the related issue (for
  example, `Closes #1234`) in the PR description.
- Run the relevant `make` targets locally before opening a PR: at minimum
  `make lint` and `make test`, plus `npm run format:check` for frontend changes.
- If your change adds or modifies API endpoints, update the Swagger
  documentation and regenerate the OpenAPI-derived frontend types.
- If you change the database schema, follow the migration workflow above and
  ensure the schema-drift / `make lint-migrations` check passes.
- Write tests for new behavior; add or update unit, integration, and e2e tests
  as appropriate for the layer you touched.

## User documentation

The end-user docs site is an [Astro Starlight](https://starlight.astro.build)
project in [`docs/site/`](docs/site), deployed to GitHub Pages by
[`.github/workflows/docs.yml`](.github/workflows/docs.yml) (decided in #2146).
To add or edit a page, edit the Markdown under `docs/site/src/content/docs/`.
English is the default locale; `cs` / `ru` live under
`docs/site/src/content/docs/{cs,ru}/` and fall back to English when a page is
missing (so translating is additive). Screenshots live in
`docs/site/src/assets/screenshots/` and are embedded with relative paths.

The site is **versioned** (mike-style) and deployed via the GitHub Actions
Pages flow — **no `gh-pages` branch, no deploy history** (one copy). Each
version lives at its own sub-path:

- `master` builds to `/edge/` (URL `…/inventario/edge/`).
- The newest **N** `vX.Y.Z` tags build to `/vX.Y.Z/` (URL `…/inventario/vX.Y.Z/`);
  `N` is `MAX_DOC_TAG_VERSIONS` (default 10, set via a repo variable), and older
  tags are dropped.
- The site root `…/inventario/` is a generated redirect to the **latest
  release** (highest semver tag; currently `edge`, until the first tag exists).

Because the Pages artifact replaces the whole site each deploy, the workflow
rebuilds edge + the kept tags into one artifact per run; an in-page version
switcher lets readers move between them. The version is chosen by the
`DOCS_VERSION` env var (default `edge`), which drives the build's base path.

Preview locally with `cd docs/site && npm install && DOCS_VERSION=edge npm run dev`.

## Questions

If something here is unclear or out of date, open a regular issue (for
non-security topics) so we can fix the docs.
