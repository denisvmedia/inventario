# Code map

Where each layer lives, the file to copy for each pattern, how to regenerate
derived files, and which files to read only in ranges. Paths are relative to
the repository root. The seller feature is the smallest complete vertical
slice (model, both registries, contract, handler, DTOs, page, hooks, i18n,
e2e, docs page), so most examples use it.

Find a symbol with `git grep -n` and read around the hit; this map tells you
where to start, not what to read in full.

## Backend (`go/`)

| Pattern | Canonical file | Notes |
| --- | --- | --- |
| Route registration | `go/apiserver/apiserver.go` | `APIServer()` builds the router; group-scoped routes hang under `/api/v1/g/{groupSlug}`. Find one with `git grep -n 'Route("/sellers"' go/apiserver/apiserver.go`. |
| API handler | `go/apiserver/sellers.go` | `Sellers()` returns the chi subrouter; each handler carries the swag comments (`// @Router`) that feed the OpenAPI spec. |
| Error to HTTP status | `go/apiserver/errors.go` | Sentinel errors mapped to JSON:API error responses. |
| JSON:API DTOs | `go/jsonapi/sellers.go` | Request and response envelopes with `Render`. |
| Model and schema | `go/models/seller.go` | `//ptah:schema:table`, `field`, `index` and `rls:*` annotations are the source of the migrations. |
| Registry interface | `go/registry/registry.go` | `type SellerRegistry interface`; factory interfaces and `FactorySet` in `go/registry/factories.go`. |
| Memory registry | `go/registry/memory/sellers.go` | Wired into the factory set in `go/registry/memory/memory.go`. |
| Postgres registry | `go/registry/postgres/sellers.go` | Wired in `go/registry/postgres/postgres.go`. |
| Shared registry contract | `go/registry/registrytest/sellers.go` | `RunSellerContract`, called by `go/registry/memory/sellers_test.go` and `go/registry/postgres/sellers_test.go`, so both backends pass the same cases. |
| Service | `go/services/commodity_loan_service.go` | Struct built from `*registry.FactorySet`, methods take `ctx` first. |
| Background worker | `go/services/trash_purge_worker.go` | Options pattern (`With...`), `Start`/`Stop` around a ticker loop, one `Sweep` per tick. |
| Worker wiring | `go/cmd/inventario/run/bootstrap/workers.go` | `StartTrashPurgeWorker`; called from `go/cmd/inventario/run/all/all.go` and listed in a worker group in `go/cmd/inventario/run/workers/workers.go` (group IDs in `selector.go`). Interval flags: `bootstrap/config.go`, `flags.go`, `durations.go`. |
| Export pipeline | `go/backup/export/` | Start with `README.md`. `worker.go` runs jobs, `jsonexport.go` and `inb_builder.go` write the signed `.inb` archive; the XML exporter sits behind the `legacy_xml_backup` build tag. |
| Import | `go/backup/import/` | Turns an uploaded archive into an export record; metadata only, no data restore. |
| Restore pipeline | `go/backup/restore/` | Start with `README.md`. `worker.go` and `service.go` run jobs; `processor/processor.go` is the entry, `processor_shared.go` the shared steps, and one `processor_<member>.go` per archive member. |
| Migrations | `go/schema/migrations/_sqldata/` | Generated `<ts>_<name>.up.sql`/`.down.sql` pairs plus `ptah.sum`; never edit by hand. |
| Schema artifact | `go/schema/schema.hcl` | Derived from the annotations; never edit by hand. |
| Postgres in tests | `go/internal/pgtest/pgtest.go` | Uses `POSTGRES_TEST_DSN` when set, otherwise starts an embedded server cached in `~/.embedded-postgres-go`. |
| CLI | `go/cmd/inventario/` | One directory per subcommand (`run`, `db`, `tenants`, `users`, ...). |
| OpenAPI notes | `devdocs/backend/openapi.md` | Conventions for the swag annotations. |

## Frontend (`frontend/src/`)

Read `devdocs/frontend/README.md` and the `frontend-work` skill before
changing anything here; `design-mocks/` is the read-only visual contract.

| Pattern | Canonical file | Notes |
| --- | --- | --- |
| Route table | `frontend/src/app/router.tsx` | Lazy page imports and `<Route>` entries. |
| Page | `frontend/src/pages/sellers/SellersListPage.tsx` | Test in `frontend/src/pages/sellers/__tests__/`. |
| Feature slice | `frontend/src/features/sellers/` | `api.ts` (calls through `@/lib/http`), `keys.ts` (query keys), `hooks.ts` (TanStack Query), `schemas.ts` (zod). |
| Feature components | `frontend/src/components/sellers/` | shadcn primitives live in `frontend/src/components/ui/`. |
| i18n strings | `frontend/src/i18n/locales/{en,cs,ru}/sellers.json` | A new namespace is registered in `frontend/src/i18n/i18next.config.ts`; extraction rules in `frontend/i18next.config.ts`; check with `npm run i18n:check`. |
| Test API mocks | `frontend/src/test/handlers/sellers.ts` | MSW handlers used by the Vitest suites. |
| Generated API types | `frontend/src/types/api.d.ts` | Generated from `go/docs/swagger.json`; re-exported as `Schema` from `frontend/src/types/index.ts`. Grep it, never read it. |

## End-to-end tests (`e2e/`)

- Example spec: `e2e/tests/sellers.spec.ts`.
- Config: `e2e/playwright.config.ts`; helpers in `e2e/utils/`, `e2e/fixtures/`
  and `e2e/setup/`.
- How to run and debug: the `inventario-e2e` skill.

## Documentation site (`docs/site/`)

- Example page: `docs/site/src/content/docs/sellers.md`.
- Sidebar: `docs/site/astro.config.mjs`; splash cards:
  `docs/site/src/content/docs/index.mdx`; screenshots:
  `docs/site/src/assets/screenshots/`.
- Build check: `cd docs/site && DOCS_VERSION=edge npm run build`.

## Regenerating derived files

| What | Command | Writes |
| --- | --- | --- |
| OpenAPI spec and frontend types | `make swagger` | Runs `make swagger-backend` (`cd go && go tool swag init --output docs`) and `make codegen-frontend` (`cd frontend && npm run codegen`). |
| OpenAPI spec only | `make swagger-backend` | `go/docs/swagger.json`, `go/docs/swagger.yaml` and `go/docs/docs.go` |
| Frontend types only | `cd frontend && npm run codegen` | `frontend/src/types/api.d.ts` from `go/docs/swagger.json` |
| Migration | `./scripts/generate-migration.sh <name>` | A new pair in `go/schema/migrations/_sqldata/` and a refreshed `ptah.sum`. Needs a local Docker daemon and Go. |
| `ptah.sum` | Refreshed only by `./scripts/generate-migration.sh`, as its last step | Never rehash it to cover an edited, renamed or rebased migration. After a rebase, regenerate the migration so it sorts after the ones already merged (AGENTS.md, "Database Migrations"). |
| Schema artifact | `make schema-hcl` | `go/schema/schema.hcl`, through `scripts/generate-schema-hcl.sh` |

CI fails on a stale result of each: Swagger Docs Sync, Frontend Codegen
Drift, Migrations Lint and Schema Artifact Sync.

## Large files: read in ranges

Measured with `wc -l`. Outline first (`grep -nE '^(func|type) '` for Go,
`grep -nE '^(export|function) '` for TypeScript), then read around the part
you need.

| Lines | File |
| ---: | --- |
| 21356 | `frontend/src/types/api.d.ts` (generated: grep only) |
| 4087 | `go/registry/registry.go` |
| 3913 | `frontend/src/components/items/CommodityFormDialog.tsx` |
| 2669 | `frontend/src/pages/commodities/CommodityDetailPage.tsx` |
| 2629 | `go/apiserver/auth_test.go` |
| 2077 | `go/apiserver/auth.go` |
| 2014 | `frontend/src/pages/commodities/CommoditiesListPage.tsx` |
| 1789 | `e2e/tests/groups.spec.ts` |
| 1743 | `go/backup/export/inb_builder.go` |
| 1576 | `go/services/orphan_file_gc_worker.go` |
| 1557 | `frontend/src/components/items/AiScanStep.tsx` |
| 1510 | `go/backup/restore/processor/processor.go` |
| 1485 | `go/backup/export/service_legacy_xml.go` |
| 1482 | `frontend/src/pages/SettingsPage.tsx` |
| 1345 | `go/registry/postgres/commodities.go` |
| 1318 | `go/registry/memory/commodities.go` |
| 1242 | `go/apiserver/groups.go` |
| 1238 | `go/services/admin/service.go` |
| 1230 | `go/apiserver/files.go` |
| 1226 | `go/registry/postgres/files.go` |
| 1225 | `go/services/group_service.go` |
| 1201 | `go/backup/restore/processor/processor_shared.go` |
| 1133 | `go/apiserver/apiserver.go` |

The test files next to these are about as large; treat them the same way.

## Agent scripts

The `agent-workflow` skill (`.agents/skills/agent-workflow/SKILL.md`) ships
the scripts agents use while iterating. They act on the checkout of the
current directory and refuse the main checkout, so `cd` into the worktree in
the same command (`cd .claude/worktrees/<name> && <script>`):

- `.agents/skills/agent-workflow/scripts/worktree-setup.sh`: disk, npm,
  Playwright and Go module checks for a fresh worktree; below 3 GiB free it
  cleans up rebuildable caches first.
- `.agents/skills/agent-workflow/scripts/lint-changed.sh`: linters on what
  the branch changed.
- `.agents/skills/agent-workflow/scripts/test-changed.sh`: tests of the
  touched Go packages, their direct importers, and related Vitest files.
