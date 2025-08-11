# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Inventario is a comprehensive personal inventory management system built with Go backend and Vue.js frontend. The application supports multi-tenancy and provides enterprise-grade features for tracking personal belongings with hierarchical organization (Locations → Areas → Commodities → Files).

## Technology Stack

- **Backend**: Go 1.24+ with Chi router, PostgreSQL primary database with multi-database support
- **Frontend**: Vue.js 3, TypeScript, PrimeVue UI components, Pinia state management
- **Databases**: PostgreSQL (recommended), BoltDB (embedded), In-memory (testing)
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
- `make build-frontend` - Build Vue.js frontend only  
- `make build-backend` - Build Go backend with embedded frontend
- `make build-backend-nofe` - Build Go backend without frontend

### Testing
- `make test` - Run all tests (Go + frontend, excluding PostgreSQL)
- `make test-go` - Run Go tests (excluding PostgreSQL registry tests)
- `make test-go-postgres` - Run PostgreSQL registry tests (requires POSTGRES_TEST_DSN env var)
- `make test-go-all` - Run all Go tests including PostgreSQL
- `make test-frontend` - Run Vue.js tests with Vitest
- `make test-e2e` - Run Playwright end-to-end tests

### Linting
- `make lint` - Run all linters
- `make lint-go` - Run golangci-lint on Go code
- `make lint-frontend` - Run ESLint and Stylelint on frontend code

### Development Server
- `make run-backend` - Run backend server on :3333
- `make run-frontend` - Run Vue.js dev server
- `make run-dev` - Run both servers concurrently

### Database Operations
- `make seed-db` - Seed database with test data via API call
- For PostgreSQL: Set POSTGRES_TEST_DSN environment variable for testing

## Project Structure

### Backend (`/go`)
- `/models` - Domain models with Ptah migration annotations, including multi-tenant entities (Tenant, User)
- `/registry` - Repository pattern implementations (PostgreSQL, BoltDB, Memory)
- `/apiserver` - HTTP API handlers with Chi router, including tenant context middleware
- `/services` - Business logic services (file management, entity operations)
- `/internal` - Internal utilities (validation, error handling, logging)
- `/backup` - Export/import functionality with streaming support

### Frontend (`/frontend`)
- `/src/components` - Reusable Vue.js components with PrimeVue
- `/src/views` - Page-level components with hierarchical navigation
- `/src/stores` - Pinia stores for state management (including auth store)
- `/src/services` - API communication services with tenant-aware headers
- `/src/types` - TypeScript type definitions

### End-to-End Tests (`/e2e`)
- Playwright tests for complete user workflows
- Test data fixtures and setup utilities
- CRUD operation testing for all major entities

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
Structured error handling with `errkit` package:
- Context-aware validation using `jellydator/validation`
- Human-readable error messages
- Proper HTTP status code mapping

## Configuration

### Database Connection
Support for multiple database backends via DSN:
- `memory://` - In-memory (development/testing)
- `boltdb://path/to/file.db` - Embedded database
- `postgres://user:pass@host:port/db` - PostgreSQL (production)

### Multi-Tenant Configuration
- `--tenant-mode` - single, multi-header, multi-subdomain
- `--jwt-secret` - Required for multi-tenant authentication
- `--default-tenant-id` - For single-tenant mode

### File Storage Configuration
- `--upload-location` - Supports file://, s3://, azblob://, gs://

## Testing Strategy

### Unit Tests
- Table-driven tests using `frankban/quicktest`
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
- Vue.js with TypeScript and Composition API
- PrimeVue components for consistent UI

### Database Migrations
- Use Ptah struct annotations for schema definition
- All entities extend TenantAwareEntityID for multi-tenancy
- Foreign key constraints ensure data integrity

### API Design
- RESTful endpoints following JSON:API specification
- Tenant context middleware on all protected routes
- Swagger documentation for all endpoints

### Security
- JWT-based authentication with tenant validation
- Application-level tenant isolation
- Comprehensive input validation and sanitization
- File upload restrictions and MIME type validation

## Common Development Tasks

### Adding New Entity Types
1. Create model in `/go/models` with Ptah annotations
2. Add registry interface in `/go/registry/registry.go`
3. Implement in database-specific registries
4. Create API handlers in `/go/apiserver`
5. Add frontend service and components
6. Write tests for all layers

### Database Schema Changes
1. Update Ptah annotations in model structs
2. Run migrations: `./inventario migrate --db-dsn=<dsn>`
3. Test with `--dry-run` flag first
4. Update tests to reflect schema changes

### Frontend Component Development
1. Follow existing patterns in `/src/components`
2. Use PrimeVue components for consistency
3. Implement proper TypeScript types
4. Add to appropriate views with routing

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
- Database migrations run automatically on startup
- Supports rollback with `--rollback` flag
- Dry-run mode for testing: `--dry-run`