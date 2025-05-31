# Database Integration Tests

This directory contains comprehensive tests for the database executor functionality, including both unit tests and integration tests for MySQL and PostgreSQL databases.

## Test Files

### Unit Tests (No Database Required)
- `mysql_test.go` - Tests for MySQL schema reader functionality
- `postgres_test.go` - Tests for PostgreSQL schema reader functionality  
- `writer_test.go` - Tests for database schema writer functionality
- `connection_test.go` - Tests for database connection handling

### Integration Tests (Database Required)
The same test files contain integration tests that require actual database connections:
- MySQL integration tests (conditional on `MYSQL_TEST_DSN` environment variable)
- PostgreSQL integration tests (conditional on `POSTGRES_TEST_DSN` environment variable)

## Running Tests

### Unit Tests Only
To run only the unit tests (no database required):
```bash
cd ptah/executor
go test -v -run "TestNew|TestSchemaWriterInterface|TestTransactionMethods_NoConnection|TestUtilityMethods|TestparseEnumValues|TestenhanceTablesWithConstraints"
```

### Integration Tests with Docker Compose
The project includes a Docker Compose configuration with test databases. Use Go toolset directly:

#### 1. Start test databases:
```bash
docker-compose --profile test up -d postgres-test mysql-test
```

#### 2. Run integration tests with environment variables:
```powershell
# Set environment variables and run all integration tests
cd ptah/executor
$env:POSTGRES_TEST_DSN="postgres://inventario_test:test_password@localhost:5433/inventario_test?sslmode=disable"
$env:MYSQL_TEST_DSN="inventario_test:test_password@tcp(localhost:3308)/inventario_test"
go test -v -run "Integration"
```

#### 3. Run all tests (unit + integration):
```powershell
cd ptah/executor
$env:POSTGRES_TEST_DSN="postgres://inventario_test:test_password@localhost:5433/inventario_test?sslmode=disable"
$env:MYSQL_TEST_DSN="inventario_test:test_password@tcp(localhost:3308)/inventario_test"
go test -v
```

#### 4. Run specific integration tests:
```powershell
cd ptah/executor
$env:POSTGRES_TEST_DSN="postgres://inventario_test:test_password@localhost:5433/inventario_test?sslmode=disable"
$env:MYSQL_TEST_DSN="inventario_test:test_password@tcp(localhost:3308)/inventario_test"
go test -v -run "TestMySQLReader_ReadSchema_Integration"
```

#### 5. Stop databases when done:
```bash
docker-compose --profile test down
```

### Graceful Skipping
If environment variables are not set, integration tests will skip automatically:
```bash
cd ptah/executor
go test -v -run "Integration"
# Output: SKIP: TestMySQLReader_ReadSchema_Integration (0.00s)
#         mysql_test.go:134: Skipping MySQL tests: MYSQL_TEST_DSN environment variable not set
```

## Test Database Configuration

The test databases are configured in `docker-compose.yaml` with the `test` profile:

- **PostgreSQL Test**: 
  - Port: 5433
  - Database: `inventario_test`
  - User: `inventario_test`
  - Password: `test_password`

- **MySQL Test**:
  - Port: 3308  
  - Database: `inventario_test`
  - User: `inventario_test`
  - Password: `test_password`

## Test Coverage

### MySQL Tests (`mysql_test.go`)
- ✅ Constructor tests with different schema parameters
- ✅ Enum value parsing (happy and unhappy paths)
- ✅ Schema reading with real MySQL database
- ✅ Error handling with nil connections

### PostgreSQL Tests (`postgres_test.go`)
- ✅ Constructor tests with different schema parameters
- ✅ Schema reading with real PostgreSQL database
- ✅ Constraint enhancement logic
- ✅ Error handling with nil connections

### Writer Tests (`writer_test.go`)
- ✅ Constructor tests for both PostgreSQL and MySQL writers
- ✅ Interface compliance verification
- ✅ Transaction lifecycle management
- ✅ SQL parsing utility functions
- ✅ Schema existence checking
- ✅ Table dropping functionality
- ✅ Error handling without database connections

### Integration Test Features
- ✅ Real database schema creation and reading
- ✅ Transaction management (begin, commit, rollback)
- ✅ Table and constraint detection
- ✅ Enum handling (PostgreSQL)
- ✅ Index and constraint reading
- ✅ Schema cleanup (drop all tables)

## Environment Variables

The tests use these environment variables for conditional execution:

- `POSTGRES_TEST_DSN` - PostgreSQL connection string for integration tests
- `MYSQL_TEST_DSN` - MySQL connection string for integration tests

If these variables are not set, the integration tests will be automatically skipped with a clear message.

## Test Patterns

The tests follow the project's established patterns:

- **QuickTest Framework**: Using `qt.New(t)` and quicktest assertions
- **Table-Driven Tests**: For testing multiple scenarios efficiently
- **Happy/Unhappy Path Separation**: Clear separation of success and failure cases
- **Conditional Execution**: Integration tests skip gracefully when databases unavailable
- **Interface Compliance**: Explicit verification that types implement expected interfaces
- **Cleanup**: Proper cleanup of test data and database state

## Troubleshooting

### Tests Are Skipped
If integration tests are being skipped, ensure:
1. Docker is running
2. Test databases are started: `docker-compose --profile test up -d postgres-test mysql-test`
3. Environment variables are set correctly
4. Databases are healthy: `docker-compose --profile test ps`

### Connection Errors
If you get connection errors:
1. Check that the databases are running and healthy
2. Verify the connection strings match the docker-compose configuration
3. Ensure no other services are using the same ports (5433, 3308)

### Test Failures
If tests fail:
1. Check the test output for specific error messages
2. Ensure databases are in a clean state
3. Try restarting the test databases
4. Check Docker logs: `docker-compose --profile test logs postgres-test mysql-test`
