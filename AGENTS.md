# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) and other coding agents when working with code in this repository.

## Project Overview

Inventario is a comprehensive personal inventory management system built with Go backend and React frontend. The application supports multi-tenancy and provides enterprise-grade features for tracking personal belongings with hierarchical organization (Locations → Areas → Commodities → Files).

## Technology Stack

- **Backend**: Go 1.26+ with Chi router, PostgreSQL primary database with multi-database support
- **Frontend**: React 19, TypeScript, Tailwind v4, shadcn/ui, TanStack Query, react-hook-form, react-i18next
- **Databases**: PostgreSQL (recommended), In-memory (testing)
- **File Storage**: Go Cloud Development Kit (local, S3, Azure Blob, Google Cloud)
- **Schema Management**: Ptah migrations with Go struct annotations
- **Multi-tenancy**: Row-Level Security (RLS) with application-level isolation

## Core Architecture

### Data Model Hierarchy
```
Locations (Top-level containers, e.g., "Home", "Office")
├── Areas (Sub-containers, e.g., "Living Room", "Storage")
    └── Commodities (Individual items with comprehensive metadata)
        ├── Images (Visual documentation)
        ├── Invoices (Purchase documentation)
        ├── Manuals (Product documentation)
        └── Files (Generic file attachments)
```

### Multi-Tenant Support
The system implements enterprise-grade multi-tenancy with:
- Tenant and User models with proper authentication
- Application-level tenant isolation enforced through middleware
- Tenant context management throughout the request lifecycle
- Database foreign key constraints ensuring data integrity

## Development Commands

### Building
- `make build` - Build both frontend and backend
- `make build-frontend` - Build React frontend only
- `make build-backend` - Build Go backend with embedded frontend
- `make build-backend-nofe` - Build Go backend without frontend
- `make build-inventool` - Build InvenTool CLI tool

### Testing
- `make test` - Run all tests (Go + frontend, excluding PostgreSQL)
- `make test-go` - Run Go tests (excluding PostgreSQL registry tests)
- `make test-go-postgres` - Run PostgreSQL registry tests (requires POSTGRES_TEST_DSN env var)
- `make test-go-all` - Run all Go tests including PostgreSQL
- `make test-frontend` - Run React unit tests with Vitest
- `make test-e2e` - Run Playwright end-to-end tests

### Linting
- `make lint` - Run all linters
- `make lint-go` - Run golangci-lint on Go code
- `make lint-frontend` - Run ESLint on frontend code (`eslint .`)

### Development Server
- `make run-backend` - Run backend server on :3333
- `make run-frontend` - Run React (Vite) dev server
- `make run-dev` - Run both servers concurrently

### Database Operations
- **MIGRATIONS — DEFAULT IS GENERATED, HAND-WRITTEN REQUIRES EXPLICIT USER APPROVAL.** Schema migrations live in `go/schema/migrations/_sqldata/` as `<version>_<name>.up.sql` / `.down.sql` pairs. Those files are **generated** from the Go model annotations (Ptah `//migrator:schema:...` tags) by the schema-drift generator; the CI schema-drift check regenerates from models and exits non-zero on any mismatch, so a freehand file will fail before merge regardless of how clean it looks. To add or change a migration:
  1. Edit the Go model in `go/models/` — add fields with `//migrator:schema:field`, indexes with `//migrator:schema:index`, RLS policies with `//migrator:schema:rls:policy`, etc.
  2. Run `./scripts/generate-migration.sh <descriptive_name>`. It spins up an ephemeral Postgres container, applies every existing migration, diffs the live schema against your model annotations, and writes the resulting UP/DOWN pair to `_sqldata/`.
  3. The generator picks its own timestamp (a real UTC Unix timestamp ≤ wall-clock now). A `TestEmbeddedMigrations_VersionNotInFuture` CI guard fails the build if any prefix exceeds the wall-clock; never invent a "fake-future" prefix to dodge collisions, just rebase + regenerate so the next real-time second is picked.
  4. Review the generated SQL, run `go test ./schema/migrations/...` locally, then commit both files.
  If the generator output looks wrong, fix the **model annotations** and regenerate — don't edit the SQL by hand.
  **Hand-written migrations exception.** Some changes can't be expressed via model annotations (data backfills, view refactors, multi-step `ALTER`s, one-off custom DDL). In those rare cases — **STOP and ask the user for explicit permission before writing SQL by hand**. Do not improvise. The user will tell you whether to handcraft the migration or land it as a follow-up using a different approach (e.g. a runtime backfill worker). The CI check still has to pass afterward, so the user's approval includes the trade-off of suppressing drift detection for that specific file.
- `curl -X POST http://localhost:3333/api/v1/seed` - Seed the database with test data (POST only; the route is gated off by default since #2039; run the server with `--enable-seed-endpoint` / `INVENTARIO_RUN_ENABLE_SEED_ENDPOINT=true` first)
- `./inventario tenants create` - Create tenants for initial setup
- `./inventario tenants list` - List all tenants with filtering
- `./inventario tenants get <id-or-slug>` - Get detailed tenant information
- `./inventario tenants update <id-or-slug>` - Update tenant properties
- `./inventario tenants delete <id-or-slug>` - Delete tenants with confirmation
- `./inventario users create` - Create users for initial setup
- `./inventario users list` - List all users with filtering
- `./inventario users get <id-or-email>` - Get detailed user information
- `./inventario users update <id-or-email>` - **Not yet implemented** (placeholder; prints a message and exits without changes)
- `./inventario users delete <id-or-email>` - Delete users with confirmation
- For PostgreSQL: Set POSTGRES_TEST_DSN environment variable for testing

## Project Structure

### Backend (`/go`)
- `/models` - Domain models with Ptah migration annotations, including multi-tenant entities (Tenant, User)
- `/registry` - Repository pattern implementations (PostgreSQL, Memory)
- `/apiserver` - HTTP API handlers with Chi router, including tenant context middleware
- `/services` - Business logic services (file management, entity operations)
- `/internal` - Internal utilities (validation, error handling, logging)
- `/backup` - Export/import functionality with streaming support

### Frontend (`/frontend`)
- `/src/app` — application shell, providers, router
- `/src/pages` — one folder per route
- `/src/features` — feature slices (auth, group, commodity, file, tag, …)
- `/src/components/ui` — shadcn primitives (Radix-based)
- `/src/lib` — `cn()`, env, http, auth-storage, group-context
- `/src/hooks` — cross-feature hooks
- `/src/i18n` — react-i18next config
- `/src/types` — OpenAPI-generated DTOs + hand-written types
- `/src/test` — Vitest setup + shared fixtures

### Frontend developer standards

The frontend stack is React 19 + TypeScript + Tailwind v4 + shadcn/ui. The full developer guide is being rewritten under [#1424](https://github.com/denisvmedia/inventario/issues/1424); current docs live in [`devdocs/frontend/`](devdocs/frontend/).

### End-to-End Tests (`/e2e`)
- Playwright tests for complete user workflows
- Test data fixtures and setup utilities
- CRUD operation testing for all major entities

### Design mock (`/design-mocks`)
- Read-only mirror of upstream `github.com/denisvmedia/inventario-design`.
- Maintained by a separate sync tool — local edits will be overwritten on the next sync, so they are forbidden.
- Same stack as `frontend/` (React 19, Tailwind v4 OKLCH, shadcn/ui new-york, Lucide, Sonner, Recharts, RHF + Zod).
- 19 views in `design-mocks/src/views/` plus `UIShowcaseView.tsx` (1379-line catalog of every UI primitive: buttons, badges, alerts, cards, tabs, forms, menus, tables, charts, typography, color tokens, empty states, etc.).

## Design contract & frontend workflow

`design-mocks/` is the **canonical visual contract** for `frontend/`. Treat it as the source of truth for layout, spacing, color usage, component composition, and interaction patterns.

### Read-only rule (no exceptions)

- **Never edit, create, delete, or move files inside `design-mocks/`.** It is a mirror of `github.com/denisvmedia/inventario-design` synchronized by an external tool; any local change is wiped on the next sync.
- Do not commit changes to `design-mocks/`. Do not propose drive-by fixes there. If you spot a bug *in the mock itself*, report it to the user — fixes go through the upstream repo, not this one.
- If a tool or refactor would touch a file under `design-mocks/`, stop and check with the user first.

### Mandatory pre-flight before touching `frontend/`

Before reading, modifying, or designing anything under `frontend/src/`:

1. Read [`devdocs/frontend/README.md`](devdocs/frontend/README.md) — it indexes 17 docs covering coding standards, components, forms, data, i18n, routing, accessibility, testing, performance, screenshots, PR checklist, and the design-language brief. Follow the doc that matches the task.
2. Locate the corresponding mock in `design-mocks/src/views/` (pages) or `design-mocks/src/components/` (shared components).
3. If the page/component is **not** present in the mock, fall back to [`design-mocks/src/views/UIShowcaseView.tsx`](design-mocks/src/views/UIShowcaseView.tsx) — it catalogues every UI primitive (buttons, badges, alerts, cards, tabs, forms, menus, tables, charts, typography, color tokens, empty states) and is the canonical fallback for surfaces the mock omits.

### Default = 1:1 fidelity

Match the mock exactly unless there is an explicit, recorded reason to diverge.

- **If the agent wants to deviate:** stop, surface the reason to the user, propose alternatives, wait for explicit approval. Never deviate unilaterally.
- **If the user requests a deviation:** accept it, but first explain the consequences (visual drift from the mock, future review/maintenance friction) and confirm they understand the tradeoff.
- **Every accepted deviation must be logged** in [`devdocs/frontend/design-deviations.md`](devdocs/frontend/design-deviations.md) in the matching domain section, using the entry format documented at the top of that file. Without a log entry, the change is not finished.

### Mandatory reading

- [`devdocs/frontend/README.md`](devdocs/frontend/README.md) — frontend operating manual (always).
- [`devdocs/frontend/design-deviations.md`](devdocs/frontend/design-deviations.md) — current deviation log (read before designing a surface; append after landing one).
- [`design-mocks/src/views/UIShowcaseView.tsx`](design-mocks/src/views/UIShowcaseView.tsx) — UI primitive catalog (consult when the mock is silent).

## Key Patterns and Conventions

### Registry Pattern
All data access uses the registry pattern with interfaces:
```go
type Registry[T any] interface {
    Create(context.Context, T) (*T, error)
    Get(ctx context.Context, id string) (*T, error)
    List(context.Context) ([]*T, error)
    Update(context.Context, T) (*T, error)
    Delete(ctx context.Context, id string) error
    Count(context.Context) (int, error)
}
```

### Multi-Tenancy Context
All operations are tenant-aware through context propagation:
- Tenant ID extracted from JWT tokens or headers
- Context middleware ensures proper tenant isolation
- Database queries automatically filtered by tenant_id

### File Management
Uses Go Cloud Development Kit for storage abstraction:
- Supports local, S3, Azure, and Google Cloud storage
- File metadata stored in database with blob storage for content
- In-app viewers for images (with zoom) and PDFs

### Error Handling
Structured error handling with `github.com/go-extras/errx` and `github.com/go-extras/errx/stacktrace` (`errxtrace`):
- Context-aware validation using `jellydator/validation`
- Human-readable error messages
- Proper HTTP status code mapping

## Configuration

### Database Connection
Support for multiple database backends via DSN:
- `memory://` - In-memory (development/testing)
- `postgres://user:pass@host:port/db` - PostgreSQL (production)

### Authentication Configuration
- `--jwt-secret` - JWT secret for authentication (minimum 32 characters; auto-generated if not provided)

### File Storage Configuration
- `--upload-location` - Supports file://, s3://, azblob://, gs://
- `--max-upload-bytes` - Maximum size of a single uploaded file in bytes (default 1 GiB / `1073741824`); `0` or negative disables the limit

## Testing Strategy

### Unit Tests
- Table-driven tests using `frankban/quicktest` aliased as `qt`
- Comprehensive model validation testing
- Registry pattern testing with mock implementations

### Integration Tests  
- Multi-tenant isolation testing
- Database transaction testing
- File upload and management testing

### End-to-End Tests
- Complete user workflows from login to data management
- Cross-browser testing with Playwright
- Multi-tenant data isolation verification

## Development Best Practices

### Code Style
- Follow Go conventions with golangci-lint
- React 19 with TypeScript (strict)
- shadcn/ui components (Radix primitives) for consistent UI

### Git Worktrees
- **All git worktrees MUST be created inside the main project under `.claude/worktrees/<name>/`.** This is a hard requirement, not a suggestion — never create a worktree as a sibling directory of the repository (e.g. `../inventario-1748`) or anywhere else outside `.claude/worktrees/`.
- Example: `git worktree add .claude/worktrees/issue-1748-admin-groups -b feat/1748-admin-groups-endpoints master`.
- Keeping every worktree under `.claude/worktrees/` ensures tooling (SocratiCode indexing, linters, the file watcher) resolves a single canonical project root and that stray worktrees are never indexed or linted as if they were separate projects.

### Pointer Allocation Style
- When code needs a pointer copy of an existing value, prefer the direct `new(value)` form at the use site (for example, `m.users[user.ID] = new(user)`) instead of copy-then-address patterns through intermediate locals.
- Do not introduce `ptrTo`, `toPtr`, or similar helper wrappers whose only purpose is manufacturing pointers; prefer the direct allocation form unless the helper adds real behavior beyond pointer construction.
- **Go 1.26+ extends the `new` builtin's operand from a type to an expression** — this is the spec change that makes the form above legal. Quoting the [Go 1.26 release notes](https://tip.golang.org/doc/go1.26): *"The built-in `new` function, which creates a new variable, now allows its operand to be an expression, specifying the initial value of the variable."* If `expr` has type `T`, then `new(expr)` allocates a `T`, **initializes it to the value of `expr`**, and returns a `*T`. Concrete examples from this codebase:
  - `new(changedAt)` where `changedAt` is a `time.Time` parameter → returns a `*time.Time` pointing to a copy of `changedAt` (NOT a zero-valued pointer).
  - `m.users[user.ID] = new(user)` where `user` is a `models.User` value → stores a `*models.User` initialized from `user`.
  - `Age: new(yearsSince(born))` from the official spec example → returns a `*int` initialized to the call's result.
- **Common misreading to avoid:** `new(x)` where `x` looks like a value is *not* the legacy `new(T)` form returning a zero-valued pointer; it is the Go 1.26 expression form returning a pointer initialized with `x`. The project requires Go 1.26+ (see Technology Stack), so this form is always intended.

### Database Migrations
- Use Ptah struct annotations for schema definition
- All entities extend TenantAwareEntityID for multi-tenancy
- Foreign key constraints ensure data integrity
- **Never hand-write migration SQL.** New tables/columns/indexes are declared as `//migrator:schema:*` annotations on the model struct; the migration files in `go/schema/migrations/_sqldata/` are *generated* from those annotations by `inventool db migrations generate` against an ephemeral postgres. Use the wrapper script:
  ```
  ./scripts/generate-migration.sh <migration_name>
  ```
  The script spins a throwaway postgres container, applies all existing migrations, then diffs the model annotations against the live schema and writes a timestamped `<ts>_<name>.up.sql` / `.down.sql` pair. Hand-written SQL drifts from the model and `make lint-migrations` (CI gate) will fail.
- Migration filenames are timestamped (`<unix-seconds>_<snake_case>.{up,down}.sql`); the script picks the timestamp.
- **Migration version MUST be a real Unix timestamp in UTC ≤ the current `date -u +%s`.** Never invent a "fake-future" prefix to dodge a collision: the migrator treats version IDs as monotonic real timestamps, and a value greater than wall-clock now will break audit reasoning ("when did this migration land?") and any tooling that filters by `created_at`. If the generator's output collides with another in-flight migration, rebase + regenerate (it will pick the next real second). Renaming a generated migration to a value greater than `now` is the worst possible workaround — always pick a real timestamp ≤ now that preserves cross-migration ordering against already-merged migrations. A CI guard fails the build if any file under `go/schema/migrations/_sqldata/` carries a version prefix > now.
- After generating, `make lint-migrations` (requires `POSTGRES_TEST_DSN`) must report no pending schema changes.

### API Design
- RESTful endpoints following JSON:API specification
- Tenant context middleware on all protected routes
- Swagger documentation for all endpoints

### Security
- JWT-based authentication with tenant validation
- Application-level tenant isolation
- Comprehensive input validation and sanitization
- File upload restrictions and MIME type validation
- **Bcrypt cost in tests: `bcrypt.MinCost` (4), never `bcrypt.DefaultCost`.** Production password hashing stays at `DefaultCost` (10, ~80ms per op). In tests this multiplies out — `-race` adds ~10x overhead and a single apiserver test binary can blow the per-package 10-minute panic timeout when many fixtures seed users. The fix:
  - Production code paths that hash tenant-user passwords (`models.User.SetPassword`) and tenant MFA backup codes (`services.MFAService.GenerateBackupCodes`) take their cost from a package-level variable.
  - Tests register `models.SetBcryptCostForTesting(t, bcrypt.MinCost)` and `services.SetBackupCodeBcryptCostForTesting(t, bcrypt.MinCost)` via `TestMain` or at the top of each test. A `bcrypt_init_test.go` per package is the canonical landing site. When `t` is non-nil, the helpers restore the previous value via `t.Cleanup` to keep overrides scoped.
  - Never call `bcrypt.GenerateFromPassword(..., bcrypt.DefaultCost)` in a `*_test.go` file. A package missing the `bcrypt_init_test.go` override that creates many users will silently double the test wall-clock under `-race`.

## Common Development Tasks

### Adding New Entity Types
1. Create model in `/go/models` with Ptah annotations
2. Add registry interface in `/go/registry/registry.go`
3. Implement in database-specific registries
4. Create API handlers in `/go/apiserver`
5. Add frontend service and components
6. Write tests for all layers

### Database Schema Changes
1. Update `//migrator:schema:*` annotations on the model struct (table, fields, indexes, RLS policies).
2. Generate the migration with `./scripts/generate-migration.sh <name>` — never hand-write the SQL.
3. Apply locally: `./inventario db migrate up --db-dsn=<dsn>` (use `--dry-run` first if unsure).
4. Update tests to reflect schema changes.

### Frontend Component Development
1. Follow existing patterns in `frontend/src/components` and `frontend/src/features`
2. Use shadcn/ui (Radix) primitives via `frontend/src/components/ui`
3. Implement proper TypeScript types
4. Add to appropriate routes/pages

## Deployment

### Single Binary Deployment
- Built binary includes embedded frontend assets
- Supports multiple database backends
- Environment variable or CLI configuration

### Docker Deployment
- Multi-stage Dockerfile for production
- Development and test compose configurations
- Health checks and proper signal handling

### Migration Strategy
- Dry-run mode for testing: `--dry-run`

## Repo-level skills

Repository-scoped Claude Code skills live under `.claude/skills/`. They activate automatically when their description matches the task — but you should also read them directly when starting related work, since they encode workflows that AGENTS.md only summarizes.

- [`.claude/skills/frontend-work/SKILL.md`](.claude/skills/frontend-work/SKILL.md) — orchestrates any task touching `frontend/`: pre-flight reading, design-mock fidelity, deviation logging, and the optional post-change screenshot/Issue-comment flow. Activates on any change inside `frontend/src/`.
- [`.claude/skills/screenshot-review/SKILL.md`](.claude/skills/screenshot-review/SKILL.md) — captures local screenshots via `e2e/screenshots.mjs`, reviews them visually for regressions, and (on explicit user request only) publishes them to an `assets/screenshots-<issue>` branch via `e2e/push-screenshots.sh` so they can be embedded in an Issue comment.
- [`.claude/skills/inventario-e2e/SKILL.md`](.claude/skills/inventario-e2e/SKILL.md) — runs and debugs Playwright e2e tests locally; not for unit/Vitest tests.
