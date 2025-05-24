# PostgreSQL Registry Tests

This directory contains comprehensive unit tests for the PostgreSQL registry implementation. The tests cover all public methods and interfaces, error handling scenarios, database connection management, and transaction handling.

## Prerequisites

### PostgreSQL Database

The tests require a PostgreSQL database to be available. You can set this up in several ways:

#### Option 1: Local PostgreSQL Installation
1. Install PostgreSQL locally
2. Create a test database:
   ```sql
   CREATE DATABASE inventario_test;
   CREATE USER inventario_test WITH PASSWORD 'test_password';
   GRANT ALL PRIVILEGES ON DATABASE inventario_test TO inventario_test;
   ```

#### Option 2: Docker Container
```bash
docker run --name postgres-test -e POSTGRES_DB=inventario_test -e POSTGRES_USER=inventario_test -e POSTGRES_PASSWORD=test_password -p 5432:5432 -d postgres:15
```

#### Option 3: Docker Compose
Create a `docker-compose.test.yml` file:
```yaml
version: '3.8'
services:
  postgres-test:
    image: postgres:15
    environment:
      POSTGRES_DB: inventario_test
      POSTGRES_USER: inventario_test
      POSTGRES_PASSWORD: test_password
    ports:
      - "5432:5432"
    tmpfs:
      - /var/lib/postgresql/data
```

Then run:
```bash
docker-compose -f docker-compose.test.yml up -d
```

## Environment Configuration

Set the `POSTGRES_TEST_DSN` environment variable to point to your test database:

```bash
export POSTGRES_TEST_DSN="postgres://inventario_test:test_password@localhost:5432/inventario_test?sslmode=disable"
```

### Windows (PowerShell)
```powershell
$env:POSTGRES_TEST_DSN="postgres://inventario_test:test_password@localhost:5432/inventario_test?sslmode=disable"
```

### Windows (Command Prompt)
```cmd
set POSTGRES_TEST_DSN=postgres://inventario_test:test_password@localhost:5432/inventario_test?sslmode=disable
```

## Running the Tests

### Run All PostgreSQL Registry Tests
```bash
go test ./go/registry/postgresql/...
```

### Run Tests with Verbose Output
```bash
go test -v ./go/registry/postgresql/...
```

### Run Specific Test Files
```bash
# Test only location registry
go test -v ./go/registry/postgresql/ -run TestLocationRegistry

# Test only commodity registry
go test -v ./go/registry/postgresql/ -run TestCommodityRegistry

# Test only settings registry
go test -v ./go/registry/postgresql/ -run TestSettingsRegistry
```

### Run Tests with Coverage
```bash
go test -v -cover ./go/registry/postgresql/...
```

### Run Tests with Race Detection
```bash
go test -v -race ./go/registry/postgresql/...
```

## Test Structure

The tests are organized into separate files for each registry:

- `test_helpers_test.go` - Common test utilities and setup functions
- `locations_test.go` - Location registry tests
- `areas_test.go` - Area registry tests
- `commodities_test.go` - Commodity registry tests
- `images_test.go` - Image registry tests
- `invoices_test.go` - Invoice registry tests
- `manuals_test.go` - Manual registry tests
- `settings_test.go` - Settings registry tests
- `registry_test.go` - Integration tests for the complete registry set

### Test Categories

Each registry test file includes:

1. **Happy Path Tests** - Successful operations
   - Create, Read, Update, Delete operations
   - List and Count operations
   - Relationship management

2. **Unhappy Path Tests** - Error scenarios
   - Invalid input validation
   - Non-existent entity handling
   - Database constraint violations

3. **Integration Tests** - Cross-registry functionality
   - Foreign key constraints
   - Cascade deletions
   - Transaction isolation

## Test Features

### Database Availability Check
Tests automatically check if PostgreSQL is available and skip if not:
- Checks for `POSTGRES_TEST_DSN` environment variable
- Attempts to connect to the database
- Skips all tests if PostgreSQL is not accessible

### Test Isolation
Each test uses a clean database state:
- Tables are dropped and recreated for each test
- No test data persists between tests
- Each test gets a fresh schema

### Comprehensive Coverage
Tests cover:
- All CRUD operations for all entity types
- Error handling scenarios
- Database connection management
- Transaction handling
- Relationship management (AddImage, GetImages, etc.)
- Foreign key constraints and cascade deletions
- Concurrent operations
- Schema initialization

### Table-Driven Tests
Tests use table-driven approach with quicktest framework:
- Happy path and unhappy path scenarios are separated
- No if/else conditionals in test code
- Clear test case descriptions

## Troubleshooting

### Tests Are Skipped
If you see "Skipping PostgreSQL tests", ensure:
1. PostgreSQL is running and accessible
2. `POSTGRES_TEST_DSN` environment variable is set correctly
3. The database exists and is accessible with the provided credentials

### Connection Errors
Common connection issues:
- **"connection refused"** - PostgreSQL is not running
- **"database does not exist"** - Create the test database
- **"authentication failed"** - Check username/password
- **"SSL required"** - Add `?sslmode=disable` to DSN for local testing

### Permission Errors
Ensure the test user has sufficient privileges:
```sql
GRANT ALL PRIVILEGES ON DATABASE inventario_test TO inventario_test;
GRANT ALL ON SCHEMA public TO inventario_test;
```

### Schema Errors
If you encounter schema-related errors:
1. Ensure the database is empty before running tests
2. Check that the test user can create/drop tables
3. Verify PostgreSQL version compatibility (requires PostgreSQL 12+)

## Continuous Integration

For CI environments, you can use a temporary PostgreSQL instance:

### GitHub Actions Example
```yaml
services:
  postgres:
    image: postgres:15
    env:
      POSTGRES_DB: inventario_test
      POSTGRES_USER: inventario_test
      POSTGRES_PASSWORD: test_password
    options: >-
      --health-cmd pg_isready
      --health-interval 10s
      --health-timeout 5s
      --health-retries 5

steps:
  - name: Run PostgreSQL tests
    env:
      POSTGRES_TEST_DSN: postgres://inventario_test:test_password@localhost:5432/inventario_test?sslmode=disable
    run: go test -v ./go/registry/postgresql/...
```

## Performance Considerations

- Tests create and drop tables for each test, which may be slow for large test suites
- Consider using test-specific schemas instead of dropping tables for better performance
- Use connection pooling in the test setup for better resource utilization
- Run tests in parallel where possible (Go's default behavior)

## Contributing

When adding new tests:
1. Follow the existing naming conventions
2. Use table-driven tests for similar scenarios
3. Separate happy path and unhappy path tests
4. Add appropriate godoc comments for complex test scenarios
5. Ensure tests clean up after themselves
6. Test both success and error conditions
