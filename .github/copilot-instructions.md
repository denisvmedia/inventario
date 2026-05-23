# Inventario Copilot instructions

This is a Go-based personal inventory management application with a React 19 + TypeScript frontend (Tailwind v4
+ shadcn/ui). It supports multiple database backends (memory, PostgreSQL) and is designed for tracking personal
items, their locations, and associated metadata. Please follow these guidelines when contributing:

## Code Standards

### Required Before Each Commit
- Run `make lint-go && make test-go` before committing any Go changes to ensure proper code formatting, linting and testing
- Run `make lint-frontend` before committing any frontend changes

### Development Flow
- Test: `make test` (runs both Go and frontend tests)
- Test backend only: `make test-go` (runs both Go tests)
- Test backend only with PostgreSQL: `make test-go-postgres` (runs both Go tests including PostgreSQL, requires env var
  in format: `POSTGRES_TEST_DSN=postgres://user:password@localhost:5432/inventario_test?sslmode=disable`)
- Build: `make build` (builds both backend and frontend)
- Build backend only: `make build-backend`
- Build frontend only: `make build-frontend`
- End-to-end tests: `make test-e2e` (run only when asked explicitely)
- Seed DB with test data: `make seed-db` (requires app running on localhost:3333)

## Repository Structure
- `go/`: Backend Go code and main application entry point
  - `go/registry/`: Data storage implementations (memory, postgres)
  - `go/models/`: Data models and entity definitions
  - `go/apiserver/`: HTTP API server implementation
  - `go/internal/`: Internal packages (log, errormarshal, etc.)
- `frontend/`: React 19 + TypeScript frontend (Tailwind v4 + shadcn/ui)
- `e2e/`: End-to-end tests using Playwright
- `docs/`: Swagger API documentation (auto-generated)
- `bin/`: Build output directory

## Key Guidelines

### Go Development
- `cd go` before operating on go code
- Use `github.com/go-extras/errx` (and `github.com/go-extras/errx/stacktrace` imported as `errxtrace`) for error wrapping and stack traces. Define sentinel errors with `errx.NewSentinel(...)` (see `go/services/errors.go`); reach for the std `errors` package only for `errors.Is` / `errors.As` checks.
- Use `github.com/denisvmedia/inventario/internal/log` for logging (never use std `log` package). Using `log/slog` is
   acceptable but internal log is preferred
- Use `github.com/frankban/quicktest` for tests, always imported with `qt` alias
- Write table-driven unit tests when possible, separating happy and unhappy paths
- Follow Go best practices and idiomatic patterns
- Write godoc comments for public APIs - balance verbosity with clarity
- Maintain existing code structure and organization
- Use dependency injection patterns where appropriate

### Go version and modern syntax (review-time guidance)

**This module targets Go 1.26+** (`go/go.mod` declares `go 1.26.0`; CI runs Go 1.26.x).
Do **not** flag the following as compile errors or "doesn't compile in Go" — they are valid:

- **Range over integer** — `for i := range n { ... }` where `n` is an `int` is valid since
  **Go 1.22** (Feb 2024). Equivalent to `for i := 0; i < n; i++`. Don't tell authors to
  rewrite it to a C-style for loop.
  See: <https://go.dev/blog/go1.22> ("range over integers").
- **Range over function** — `for k, v := range iterFunc { ... }` where `iterFunc` is a
  `func(yield func(K, V) bool)` (i.e. `iter.Seq` / `iter.Seq2`) is valid since **Go 1.23**.
- **Builtins** `min`, `max`, `clear` are available since Go 1.21.
- **Generic type aliases** (`type Foo[T any] = Bar[T]`) are valid since Go 1.24.
- **`sync.WaitGroup.Go`** — `wg.Go(task)` (auto `Add`/`Done`) is valid since **Go 1.25**.
  The classic `Add`/`Done` pattern is also fine; don't push contributors to one form
  over the other.
- **`new` takes an expression, not just a type** — since **Go 1.26**, `new(x)` where
  `x` has type `T` allocates a `*T` initialized to the value of `x`. Examples in this
  repo: `new(true)` (→ `*bool`), `new(value)` (→ `*int`), `m.users[user.ID] = new(user)`
  (→ `*models.User` initialized from `user`). This is **not** the legacy `new(T)`
  zero-value form — do **not** "fix" it to `&x`, do **not** claim "the argument is not
  a type", and do **not** propose `ptrTo` / `toPtr` helpers (the project bans those).
  Reference: <https://tip.golang.org/doc/go1.26>.

**Before claiming Go code "does not compile":** the project's `go-test*.yml` workflows
build and run the full test suite on every PR. If those are green and you're flagging a
compile error, you're wrong — your knowledge of Go syntax is outdated. Defer to the
build, not to memory. **The Go language version in `go/go.mod` is authoritative; read
it before commenting on syntax. Treat any Go-version references in these instructions
as feature minimums only, not as the exact project patch version.**

For deeper Go-specific review guidance see [`.github/instructions/go.instructions.md`](instructions/go.instructions.md).

### Frontend Development
- Use React 19 with TypeScript
- Use Tailwind v4 + shadcn/ui for styling — see `frontend/src/index.css` for tokens
- Maintain consistent look and feel with existing pages
- Follow React best practices and component patterns

### Testing Requirements
- Write unit tests for new Go functionality using quicktest
- Run `make test-go` for Go tests (excludes PostgreSQL by default)
- For PostgreSQL tests: set `POSTGRES_TEST_DSN` (e.g. `POSTGRES_TEST_DSN=postgres://user:password@localhost:5432/inventario_test?sslmode=disable`)
   environment variable and run `make test-go-postgres` (PostgreSQL must be running according to the env var)
- Run `make test-frontend` for frontend tests
- Consider writing e2e tests for complex user flows (in `e2e/` directory)
- All tests must pass before committing

### API Documentation
- When changing Go API entities, regenerate Swagger docs:
  ```bash
  go tool swag init --output docs
  ```

### Database Support
- Support multiple backends: memory (default), PostgreSQL
- Use appropriate registry implementations in `go/registry/`
- Test database-specific code with appropriate test suites
- Squash SQL migrations, which belong to the same pull request, to have only 1 up and 1 down migration per PR

### Code Quality
- Ensure all files end with a newline (e,g, Go, TS, JS, MD files and other text files)
- Remove trailing whitespace (unless required by format)
- Follow consistent naming conventions
- Document complex logic appropriately
- Update existing documentation when making changes
