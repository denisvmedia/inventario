# Ptah Migration Library Integration Tests

This directory contains comprehensive integration tests for the Ptah migration library. The tests validate migration functionality across multiple database backends including PostgreSQL, MySQL, and MariaDB.

## Overview

The integration test suite covers all aspects of the migration system as outlined in the integration test plan:

### 🧱 Basic Functionality
- Apply incremental migrations
- Roll back migrations
- Upgrade to specific version
- Check current version
- Generate desired schema
- Read actual DB schema
- Dry-run support
- Operation planning
- Schema diff
- Failure diagnostics

### 🔁 Idempotency
- Re-apply already applied migrations
- Run migrate up when database is already up-to-date

### 🔀 Concurrency
- Launch parallel migrate up processes
- Ensure locking prevents double-apply

### 🧪 Partial Failure Recovery
- Handle multi-step migrations with intentional failures
- Validate recovery and rollback capabilities

### ⏱ Timestamp Verification
- Check that `applied_at` timestamps are stored correctly

### 📂 Manual Patch Detection
- Detect manual schema changes via schema diff

### 🔒 Permission Restrictions
- Test behavior with limited database privileges

### 🧹 Cleanup Support
- Drop all tables and re-run from empty state

## Architecture

### Components

- **`framework.go`** - Core test framework with TestRunner and DatabaseHelper
- **`reporter.go`** - Report generation in multiple formats (TXT, JSON, HTML)
- **`scenarios.go`** - Basic test scenarios implementation
- **`scenarios_advanced.go`** - Advanced test scenarios (concurrency, idempotency)
- **`scenarios_misc.go`** - Miscellaneous test scenarios (timestamps, permissions)

### Test Fixtures

- **`fixtures/migrations/basic/`** - Standard migration set for testing
- **`fixtures/migrations/failing/`** - Migrations with intentional failures
- **`fixtures/migrations/partial_failure/`** - Multi-step migrations with failures
- **`fixtures/entities/`** - Go entity definitions for schema generation tests

## Running Tests

All integration tests are designed to run using Docker Compose, which provides isolated database environments and ensures consistent test execution across different systems.

### Basic Usage

```bash
# Run all tests with default settings (text report)
docker-compose --profile test run --rm ptah-tester

# Run with HTML report (recommended for detailed analysis)
docker-compose --profile test run --rm ptah-tester --report=html

# Run with JSON report (good for CI/CD integration)
docker-compose --profile test run --rm ptah-tester --report=json

# Enable verbose output for debugging
docker-compose --profile test run --rm ptah-tester --verbose
```

### Scenario Selection

```bash
# Run specific scenarios only
docker-compose --profile test run --rm ptah-tester --scenarios=apply_incremental_migrations,rollback_migrations

# Run basic functionality tests
docker-compose --profile test run --rm ptah-tester --scenarios=apply_incremental_migrations,upgrade_to_specific_version,check_current_version

# Run idempotency tests
docker-compose --profile test run --rm ptah-tester --scenarios=idempotency_reapply,idempotency_up_to_date

# Run failure recovery tests
docker-compose --profile test run --rm ptah-tester --scenarios=failure_diagnostics,partial_failure_recovery
```

### Database Selection

```bash
# Test against PostgreSQL only
docker-compose --profile test run --rm ptah-tester --databases=postgres

# Test against MySQL only
docker-compose --profile test run --rm ptah-tester --databases=mysql

# Test against MariaDB only
docker-compose --profile test run --rm ptah-tester --databases=mariadb

# Test against specific combination
docker-compose --profile test run --rm ptah-tester --databases=postgres,mysql
```

### Combined Options

```bash
# Comprehensive test with detailed reporting
docker-compose --profile test run --rm ptah-tester --report=html --verbose

# Quick smoke test
docker-compose --profile test run --rm ptah-tester --scenarios=apply_incremental_migrations --databases=postgres --report=txt

# CI/CD friendly execution
docker-compose --profile test run --rm ptah-tester --report=json --databases=postgres,mysql,mariadb
```

## Command Line Options

- `--report` - Report format: `txt`, `json`, or `html` (default: `txt`)
- `--databases` - Comma-separated list of databases to test (default: `postgres,mysql,mariadb`)
- `--scenarios` - Comma-separated list of specific scenarios to run (default: all)
- `--verbose` - Enable verbose output

Reports are automatically saved to `/app/reports` inside the container and mapped to `./integration/reports` on the host.

## Report Formats

### Text Report
Plain text format suitable for CI/CD pipelines and console output.

### JSON Report
Machine-readable format for integration with other tools and systems.

### HTML Report
Rich, interactive report with:
- Visual progress indicators
- Detailed test results
- Error highlighting
- Responsive design
- Summary statistics

## Database Requirements

### PostgreSQL
- Version: 16+
- Required permissions: CREATE, DROP, SELECT, INSERT, UPDATE, DELETE
- Default schema: `public`

### MySQL
- Version: 8+
- Required permissions: CREATE, DROP, SELECT, INSERT, UPDATE, DELETE
- Authentication: `mysql_native_password`

### MariaDB
- Version: 10.11+
- Required permissions: CREATE, DROP, SELECT, INSERT, UPDATE, DELETE
- Compatible with MySQL driver

## Test Data

The integration tests use controlled test data:

- **Users table**: Basic user information with email uniqueness
- **Posts table**: Blog posts with foreign key to users
- **Comments table**: Comments with foreign keys to posts and users

This schema provides sufficient complexity to test:
- Primary keys and auto-increment
- Foreign key constraints
- Unique constraints
- Indexes
- Different data types
- Cascading deletes

## Continuous Integration

The integration tests are designed to run in CI/CD environments using Docker Compose:

```yaml
# Example GitHub Actions workflow
- name: Run Integration Tests
  run: |
    docker-compose --profile test run --rm ptah-tester --report=json

# Example GitLab CI
test:integration:
  script:
    - docker-compose --profile test run --rm ptah-tester --report=json
  artifacts:
    reports:
      junit: integration/reports/*.json

# Example with specific database testing
test:postgres:
  script:
    - docker-compose --profile test run --rm ptah-tester --databases=postgres --report=json
```

## Troubleshooting

### Database Connection Issues
- Verify database URLs are correct
- Check that databases are running and accessible
- Ensure proper permissions are granted

### Test Failures
- Check the generated reports for detailed error messages
- Verify test fixtures are properly structured
- Ensure database schemas are clean before running tests

### Performance Issues
- Consider running tests against fewer databases
- Use specific scenario selection for faster feedback
- Check database resource allocation

## Contributing

When adding new test scenarios:

1. Add the scenario function to the appropriate `scenarios_*.go` file
2. Register it in the `GetAllScenarios()` function
3. Create any necessary test fixtures
4. Update this README with scenario documentation
5. Test against all supported databases

## Future Enhancements

- [ ] Performance benchmarking scenarios
- [ ] Large-scale migration testing
- [ ] Cross-database migration compatibility
- [ ] Schema validation scenarios
- [ ] Backup and restore testing
