# Migration Generator Validation Integration Test

This document describes the `migration_generator_validation` integration test scenario that validates the correctness of the ptah migration generator.

## Overview

The migration generator validation test ensures that:

1. **Migration Generation**: The generator correctly creates migrations from entity definitions
2. **Schema Consistency**: Database schema matches goschema output after applying migrations  
3. **Schema Diff Validation**: schemadiff reports no differences between expected and actual schemas
4. **Forward Migrations**: Migrations can be applied sequentially (000 → 001 → 002)
5. **Rollback Migrations**: Migrations can be rolled back in reverse order (002 → 001 → 000 → empty)

## Test Steps

### Step 1: Initial Migration (000-initial)
- Load entity definitions from `fixtures/entities/000-initial`
- Generate and apply the initial migration
- Validate database schema matches goschema output
- Ensure schemadiff reports no differences

### Step 2: Add Fields (001-add-fields)  
- Load entity definitions from `fixtures/entities/001-add-fields`
- Generate migration on top of the applied 000-initial migration
- Apply the migration (UP)
- Validate database schema matches goschema output
- Ensure schemadiff reports no differences

### Step 3: Add Posts (002-add-posts)
- Load entity definitions from `fixtures/entities/002-add-posts`
- Generate migration on top of the applied 001-add-fields migration
- Apply the migration (UP)
- Validate database schema matches goschema output
- Ensure schemadiff reports no differences

### Step 4: Rollback to Step 2 (001-add-fields)
- Generate rollback migration to reach 001-add-fields state
- Apply the rollback migration
- Validate database schema matches goschema output for 001-add-fields

### Step 5: Rollback to Step 1 (000-initial)
- Generate rollback migration to reach 000-initial state
- Apply the rollback migration  
- Validate database schema matches goschema output for 000-initial

### Step 6: Rollback to Empty State
- Drop all tables to return to empty database state
- Validate database schema is empty (no tables)

## Running the Test

### Prerequisites

Set up a test database connection using environment variables:

```bash
# PostgreSQL
export POSTGRES_TEST_URL="postgres://user:password@localhost:5432/test_db"

# MySQL  
export MYSQL_TEST_URL="mysql://user:password@localhost:3306/test_db"
```

### Run Specific Test

```bash
# Run only the migration generator validation test
go run ./cmd/integration-test run migration_generator_validation

# Run with verbose output
go run ./cmd/integration-test run migration_generator_validation --verbose

# Run with specific database
POSTGRES_TEST_URL="postgres://..." go run ./cmd/integration-test run migration_generator_validation
```

### Run Unit Tests

```bash
# Run the unit tests for the validation functions
go test ./integration -run TestMigrationGeneratorValidation -v
go test ./integration -run TestValidateSchemaConsistency -v  
go test ./integration -run TestValidateEmptySchema -v
```

## Test Architecture

The test uses the following components:

- **VersionedEntityManager**: Manages loading different entity versions from fixtures
- **Migration Generator**: Generates migrations dynamically from entity definitions
- **Schema Diff**: Compares expected vs actual database schemas
- **Migrator**: Applies generated migrations to the database

## Validation Functions

### `validateSchemaConsistency()`
- Loads entities for a specific version
- Generates expected schema from entities using goschema
- Reads actual database schema using database introspection
- Compares schemas using schemadiff
- Fails if any differences are detected

### `rollbackToVersion()`
- Loads target version entities
- Generates migration SQL to reach target state
- Applies the rollback migration if changes are needed

### `validateEmptySchema()`
- Reads current database schema
- Ensures no tables exist in the database

## Expected Behavior

✅ **Success Criteria:**
- All migration steps complete without errors
- Schema consistency validation passes at each step
- Rollback operations work correctly
- Final empty state validation passes

❌ **Failure Scenarios:**
- Migration generation fails
- Schema differences detected after applying migrations
- Rollback operations fail
- Database not properly cleaned up

## Troubleshooting

### Common Issues

1. **Database Connection**: Ensure test database URL is correctly set
2. **Permissions**: Database user needs CREATE/DROP table permissions
3. **Clean State**: Test requires starting with an empty database
4. **Fixtures**: Entity fixtures must be present in `fixtures/entities/` directory

### Debug Output

Enable verbose logging to see detailed migration SQL:

```bash
go run ./cmd/integration-test run migration_generator_validation --verbose
```

This will show:
- Generated migration SQL statements
- Schema comparison results
- Step-by-step validation progress
