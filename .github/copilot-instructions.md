This is a Go-based personal inventory management application with a Vue.js 3 + TypeScript frontend. It supports multiple
database backends (memory, BoltDB, PostgreSQL) and is designed for tracking personal items, their locations, and
associated metadata. Please follow these guidelines when contributing:

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
  - `go/registry/`: Data storage implementations (memory, boltdb, postgres)
  - `go/models/`: Data models and entity definitions
  - `go/apiserver/`: HTTP API server implementation
  - `go/internal/`: Internal packages (errkit, log, etc.)
- `frontend/`: Vue.js 3 + TypeScript frontend with SCSS styles
- `e2e/`: End-to-end tests using Playwright
- `docs/`: Swagger API documentation (auto-generated)
- `bin/`: Build output directory

## Key Guidelines

### Go Development
- `cd go` before operating on go code
- Use `github.com/denisvmedia/inventario/internal/errkit` for errors, but use std `errors` package for sentinel errors
- Use `github.com/denisvmedia/inventario/internal/log` for logging (never use std `log` package). Using `log/slog` is
   acceptable but internal log is preferred
- Use `github.com/frankban/quicktest` for tests, always imported with `qt` alias
- Write table-driven unit tests when possible, separating happy and unhappy paths
- Follow Go best practices and idiomatic patterns
- Write godoc comments for public APIs - balance verbosity with clarity
- Maintain existing code structure and organization
- Use dependency injection patterns where appropriate

### Frontend Development
- Use Vue.js 3 with TypeScript and Composition API
- Use SCSS for all styling - check `frontend/src/assets/*.scss` to avoid duplicating styles 
- Maintain consistent look and feel with existing views
- Follow Vue.js best practices and component patterns

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
  SWAG_VERSION=$(go list -m -f '{{.Version}}' github.com/swaggo/swag)
  go install github.com/swaggo/swag/cmd/swag@${SWAG_VERSION}
  swag init --output docs
  ```

### Database Support
- Support multiple backends: memory (default), BoltDB, PostgreSQL
- Use appropriate registry implementations in `go/registry/`
- Test database-specific code with appropriate test suites
- Squash SQL migrations, which belong to the same pull request, to have only 1 up and 1 down migration per PR

### Code Quality
- Ensure all files end with a newline (e,g, Go, TS, JS, MD files and other text files)
- Remove trailing whitespace (unless required by format)
- Follow consistent naming conventions
- Document complex logic appropriately
- Update existing documentation when making changes
