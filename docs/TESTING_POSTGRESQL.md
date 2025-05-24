# PostgreSQL Testing Guide

This document explains how to run PostgreSQL tests for the Inventario application.

## Overview

The application has comprehensive unit tests for the PostgreSQL registry implementation. These tests are separated from the main test suite to avoid requiring PostgreSQL for basic development and CI.

## Test Structure

### Separate Test Workflows

1. **Main Go Tests** (`.github/workflows/go-test.yml`)
   - Runs all unit tests except PostgreSQL tests
   - No external dependencies required
   - Fast execution for basic CI/CD

2. **PostgreSQL Tests** (`.github/workflows/go-test-postgresql.yml`)
   - Runs only PostgreSQL registry tests
   - Uses PostgreSQL service container
   - Comprehensive database integration testing

### Test Scope

PostgreSQL tests are located in `go/registry/postgresql/` and include:
- All CRUD operations for all entity types
- Error handling scenarios
- Database connection management
- Transaction handling
- Relationship management
- Foreign key constraints and cascade deletions
- Concurrent operations
- Schema initialization

## Running Tests Locally

### Prerequisites

You need a PostgreSQL database for testing. You can set this up in several ways:

#### Option 1: Local PostgreSQL Installation

1. Install PostgreSQL locally
2. Create a test database:
   ```sql
   CREATE DATABASE inventario_test;
   CREATE USER inventario_test WITH PASSWORD 'test_password';
   GRANT ALL PRIVILEGES ON DATABASE inventario_test TO inventario_test;
   ```

#### Option 2: Docker PostgreSQL

```bash
# Start PostgreSQL container
docker run --name postgres-test -e POSTGRES_DB=inventario_test -e POSTGRES_USER=inventario_test -e POSTGRES_PASSWORD=test_password -p 5432:5432 -d postgres:15

# Stop and remove when done
docker stop postgres-test && docker rm postgres-test
```

#### Option 3: Automated Setup (Recommended)

Use the provided setup scripts for easy configuration:

**Windows (PowerShell):**
```powershell
# Using Docker (recommended)
.\scripts\setup-postgresql-test.ps1 -UseDocker

# Using local PostgreSQL
.\scripts\setup-postgresql-test.ps1
```

**Linux/macOS (Bash):**
```bash
# Using Docker (recommended)
./scripts/setup-postgresql-test.sh --use-docker

# Using local PostgreSQL
./scripts/setup-postgresql-test.sh
```

These scripts will automatically:
- Set up PostgreSQL (Docker or local)
- Create the test database and user
- Set the `POSTGRES_TEST_DSN` environment variable
- Provide instructions for running tests

### Environment Setup

Set the `POSTGRES_TEST_DSN` environment variable:

**Linux/macOS:**
```bash
export POSTGRES_TEST_DSN="postgres://inventario_test:test_password@localhost:5432/inventario_test?sslmode=disable"
```

**Windows (PowerShell):**
```powershell
$env:POSTGRES_TEST_DSN="postgres://inventario_test:test_password@localhost:5432/inventario_test?sslmode=disable"
```

**Windows (Command Prompt):**
```cmd
set POSTGRES_TEST_DSN=postgres://inventario_test:test_password@localhost:5432/inventario_test?sslmode=disable
```

### Running Tests

#### Using Make

```bash
# Run only PostgreSQL tests
make test-go-postgresql

# Run all Go tests (excluding PostgreSQL)
make test-go

# Run all Go tests (including PostgreSQL)
make test-go-all
```

#### Using Go directly

```bash
# Run only PostgreSQL tests
cd go
go test -v ./registry/postgresql/...

# Run all tests excluding PostgreSQL
cd go
go test -v ./... -skip="TestPostgreSQL"

# Run all tests including PostgreSQL
cd go
go test -v ./...
```

### Test Features

- **Automatic Skipping**: Tests automatically skip if PostgreSQL is not available
- **Clean State**: Each test uses a fresh database schema
- **Comprehensive Coverage**: Tests cover all registry operations and error scenarios
- **Table-Driven**: Uses quicktest framework with clear test case separation

## Continuous Integration

### GitHub Actions

The repository includes two separate workflows:

1. **go-test.yml**: Runs on every push/PR, excludes PostgreSQL tests
2. **go-test-postgresql.yml**: Runs on every push/PR, includes PostgreSQL service

Both workflows run in parallel, providing fast feedback for basic tests while ensuring PostgreSQL functionality is validated.

### Local CI Testing

You can simulate the CI environment locally:

```bash
# Test the main workflow (no PostgreSQL)
make test-go

# Test the PostgreSQL workflow (requires PostgreSQL)
export POSTGRES_TEST_DSN="postgres://inventario_test:test_password@localhost:5432/inventario_test?sslmode=disable"
make test-go-postgresql
```

## Troubleshooting

### Tests Are Skipped

If you see "Skipping PostgreSQL tests", ensure:
1. PostgreSQL is running and accessible
2. `POSTGRES_TEST_DSN` environment variable is set correctly
3. The database exists and is accessible with the provided credentials

### Common Connection Issues

- **"connection refused"**: PostgreSQL is not running
- **"database does not exist"**: Create the test database
- **"authentication failed"**: Check username/password
- **"SSL required"**: Add `?sslmode=disable` to DSN for local testing

### Permission Errors

Ensure the test user has sufficient privileges:
```sql
GRANT ALL PRIVILEGES ON DATABASE inventario_test TO inventario_test;
GRANT ALL ON SCHEMA public TO inventario_test;
```

## Development Workflow

1. **Regular Development**: Use `make test-go` for fast feedback
2. **PostgreSQL Changes**: Use `make test-go-postgresql` to test database logic
3. **Before Committing**: Use `make test-go-all` to run complete test suite
4. **CI Validation**: Both workflows run automatically on push/PR

This setup ensures that PostgreSQL functionality is thoroughly tested while keeping the development workflow fast and accessible.
