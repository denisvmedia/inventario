---
description: Go review/edit guidance for GitHub Copilot in the Inventario repository. Tells Copilot to trust go.mod over its training data and not re-flag known false positives.
applyTo:
  - "go/**/*.go"
  - "go/go.mod"
  - "go/go.sum"
---

# Go instructions for Copilot

These instructions are **for GitHub Copilot only** (PR review bot + Copilot in IDE).
For human/agent contributors the source of truth is [`AGENTS.md`](../../AGENTS.md) at
the repo root and [`.claude/agents/inventario-backend.md`](../../.claude/agents/inventario-backend.md).
This file does not duplicate either of those — read them for the architectural
contract, multi-tenancy, migrations, registry pattern, error model, etc.

## Where the Go code lives

The Go module lives in **`go/`** (`./go/...` from the repository root).
This module is not at the repo root; the React frontend lives next to it under
`frontend/`. When you reason about imports, build flags, or `go vet`/`go test`
invocations, the working directory is `go/`, not the repo root.

## Trust `go.mod`, not your training data

Before you flag any line of Go as a "compile error", "won't build", "invalid
syntax", or "use the C-style form instead", **read `go/go.mod`** and use the
declared `go` directive as the language version baseline. Today that's
`go 1.26.x`; CI runs the matching toolchain. The full test suite runs on every
PR (`go-test*.yml`). If the suite is green and your knowledge of Go syntax says
the code shouldn't compile, **your knowledge is out of date — defer to the
build**. Do not suggest a rewrite.

The following are valid in this codebase and **must not be flagged**:

### `new(expr)` — Go 1.26 expression form

Since **Go 1.26** the `new` builtin's operand is an **expression**, not a type.
`new(x)` where `x` has type `T` allocates a `*T` and initializes the pointee to
the value of `x`. Examples that appear throughout this repo and are *not* bugs:

```go
newField.defaultValue = new(true)        // *bool pointing to true
newField.defaultValue = new(value)       // *int pointing to value
newField.minValue     = new(0)           // *int pointing to 0
m.users[user.ID]      = new(user)        // *models.User initialized from user
return new(yearsSince(born))             // *int initialized from the call result
```

This is **not** the old `new(T)` form returning a zero-valued pointer. Do not
"correct" it to `&x`, do not claim it "doesn't compile because the argument is
not a type", and do not propose `ptrTo(x)` helpers. The repo bans manufacturing
helpers like `ptrTo` / `toPtr` — the direct `new(expr)` form is the project
convention.

Reference: [Go 1.26 release notes — `new`](https://tip.golang.org/doc/go1.26).

### Range over integers — Go 1.22

`for i := range n { ... }` where `n` is an `int` is valid since **Go 1.22**
(Feb 2024). It's equivalent to `for i := 0; i < n; i++`. Do not propose a
C-style rewrite.

### Range over functions — Go 1.23

`for k, v := range iterFunc { ... }` where `iterFunc` is a
`func(yield func(K, V) bool)` (i.e. `iter.Seq` / `iter.Seq2`) is valid since
**Go 1.23**.

### Generic type aliases — Go 1.24

`type Foo[T any] = Bar[T]` is valid since **Go 1.24**.

### Builtins `min`, `max`, `clear` — Go 1.21

Available everywhere. Do not suggest hand-rolled equivalents.

### `sync.WaitGroup.Go` — Go 1.25

`wg.Go(task)` (auto `Add`/`Done`) is valid since **Go 1.25**. The classic
`Add`/`Done`/`defer Done` pattern is also fine. Don't push contributors to
"the other form" just because you only know one.

If you spot a Go feature you don't recognize, **assume the toolchain knows
better than your training data**. Look it up in the Go release notes for the
version declared in `go.mod` before commenting.

## Repo-specific style — short version

These follow from `.golangci.yml`. They override anything in generic Go style
guides you may have been trained on.

- **Tests use `package <pkg>_test`** (external/black-box). `qtlint` enforces
  this. Do **not** suggest moving tests into the same package as the code
  under test — that contradicts the project rule.
- **Test framework is `github.com/frankban/quicktest`**, imported as `qt`
  (enforced by `importas`). Do **not** suggest `testify`, `assert`, or
  `require`.
- **Error stacktraces use `github.com/go-extras/errx/stacktrace`** imported as
  `errxtrace` (enforced by `importas`). Sentinels via `errx.NewSentinel(...)`.
  See `go/registry/errors.go` and `go/apiserver/errors.go` for the canonical
  pattern. The plain `errors` package is fine for `errors.Is` / `errors.As`.
- **Import groups via `gci`** in three sections: standard, default (third
  party), then `prefix(github.com/denisvmedia/inventario)`. Do not suggest
  `goimports`-flavored single-group layouts.
- **Banned imports:** `io/ioutil` (use `io` / `os`). Enforced by `depguard`.
- **`any`, not `interface{}`** — `revive use-any` is on. Don't suggest the
  reverse.
- **Comma-ok type assertions** required outside test code
  (`revive unchecked-type-assertion`).
- **Filename format** `^[_a-z][_a-z0-9]*\.go$` — no camelCase, no dashes.
  (Generated SQL migrations live under `schema/migrations/_sqldata/` and are
  `.sql`, so the Go filename rule never applies to them.)
- **Function limits** (`funlen`): 240 lines / 160 statements. Cognitive
  complexity ≤ 30 (`gocognit`). Cyclomatic ≤ 20 (`gocyclo`). Nested-if depth
  ≤ 6 (`nestif`). Line length ≤ 240 (`lll`). Test files are exempt from these.
- **`function-result-limit: 3`** — more than 3 return values? Wrap them in a
  struct.
- **Naked return** allowed only in functions ≤ 2 lines (`nakedret`).
- **`//nolint:` directives** must carry an explanation unless the lint is
  `errcheck` or `lll` (`nolintlint`). Don't suggest `//nolint:` without a
  reason comment.
- **Error strings:** `revive error-strings` is **disabled** in this repo —
  do **not** flag error messages for starting with uppercase or ending in a
  period. That's a deliberate project choice.
- **`gochecknoinits`** is enabled but **disabled for `cmd/...`**. Don't flag
  `init()` in CLI entry points.
- **`gosec`** suppressions: `G117` (false positive on struct field names
  containing "Secret"/"Token") and `G706` (structured `slog` logging is not
  log-injection-vulnerable) are project-wide exclusions. Don't re-raise them.
- **Structured logging** uses the standard library `log/slog` with key/value
  pairs: `slog.Error("Security violation", "user_id", user.ID, ...)`. Never the
  std `log` package.
- **Standard layout differs:** the module lives in `go/`, not at repo root.
  Sub-packages: `apiserver/` (HTTP handlers), `registry/` (data layer
  interface + memory/postgres implementations), `services/` (business
  logic), `models/` (domain models with `//migrator:schema:*` annotations),
  `internal/` (shared infra). There's no `pkg/` directory and no `cmd/` at
  the *root* — CLI commands live under `go/cmd/`. Don't suggest the
  standard `pkg/`/`cmd/`/`internal/` layout reorg.

## Multi-tenancy is load-bearing

This is a multi-tenant SaaS. **Never suggest reading `tenant_id` from request
bodies, query parameters, or headers**. It comes only from the authenticated
user's context (`appctx.UserFromContext(ctx).TenantID`), and the
`TenantMiddleware` validates that the user's tenant matches the resolved
tenant. Suggesting otherwise is a security regression. See
`go/apiserver/tenant_context.go` and `go/apiserver/jwt_middleware.go`.

Registries come in two flavors:

- **User registry** (`FactorySet.CreateUserRegistrySet(ctx)`) — RLS-scoped to
  the authenticated user. HTTP handlers use this.
- **Service registry** (`FactorySet.CreateServiceRegistrySet()`) — bypasses
  RLS. Background workers only. Never in an HTTP handler.

Don't propose switching one for the other without naming the security impact.

## Migrations are generated, not hand-written

Schema migrations under `go/schema/migrations/_sqldata/` are **generated** from
Go model annotations (`//migrator:schema:...`) by `./scripts/generate-migration.sh`.
CI's schema-drift check regenerates and fails on any mismatch. **Do not
suggest editing `.up.sql` / `.down.sql` files directly** — the correct fix is
to change the model annotations and regenerate. Hand-written SQL is allowed
only for cases annotations can't express (data backfills, multi-step ALTERs)
and that requires explicit human approval; Copilot should never recommend
hand-writing migration SQL as a normal workflow.

## Swagger / OpenAPI is paired with frontend codegen

Handler annotations live as `swag` comment blocks. Regeneration is
`make swagger` from the repo root (it runs both `swagger-backend` *and*
`codegen-frontend`). Don't suggest running just `swag init` directly — the
frontend types in `frontend/src/types/api.d.ts` need to stay in sync, and the
codegen CI gate will fail otherwise.

## Things Copilot has historically gotten wrong here

If you find yourself about to write any of these, **stop**:

- "This should use `goimports` ordering" — no, the project uses `gci` with
  three sections.
- "Move the test into the same package" — no, the project enforces external
  `_test` packages.
- "Replace `new(...)` with `&...`" — no, `new(expr)` is the project's
  preferred form on Go 1.26+.
- "Use `for i := 0; i < n; i++`" — no, range-over-int has been valid since
  Go 1.22.
- "Use `interface{}` instead of `any`" — no, the project mandates `any`.
- "Use `testify`/`assert`/`require`" — no, the project uses `quicktest` with
  the `qt` alias.
- "Read tenant from the request body/query" — no, that's a security bug; it
  comes from the auth context.
- "Edit the generated migration SQL" — no, edit the model annotation and
  regenerate.
- "Add a `ptrTo` / `toPtr` helper" — no, use `new(expr)` directly.
- "Lowercase the error message" — no, `revive error-strings` is disabled
  here.

## When in doubt

1. Read [`AGENTS.md`](../../AGENTS.md) for the project contract.
2. Read [`.claude/agents/inventario-backend.md`](../../.claude/agents/inventario-backend.md)
   for the operational playbook (lint chain, error mapping, registry
   contract, testing conventions).
3. Trust the CI build over your prior assumptions about Go syntax.
