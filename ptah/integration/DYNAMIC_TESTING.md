# Dynamic Integration Testing with Versioned Entities

This document describes the new dynamic integration testing approach for the Ptah migration library, which uses versioned entity fixtures instead of pre-bundled SQL migrations.

## Overview

The dynamic testing approach provides more realistic and comprehensive testing by:

1. **Testing the Full Workflow**: Entity parsing → Schema generation → Migration generation → Application
2. **Using Realistic Evolution**: Versioned entities represent real-world schema evolution patterns
3. **Eliminating Manual SQL**: No need to manually write migration SQL files
4. **Better Coverage**: Tests various migration scenarios and edge cases

## Versioned Entity Structure

```
ptah/integration/fixtures/entities/
├── 000-initial/          # Basic entities (User, Product)
│   ├── user.go
│   └── product.go
├── 001-add-fields/       # Added fields (age, bio, description, category)
│   ├── user.go
│   └── product.go
├── 002-add-posts/        # Added Post entity with foreign keys
│   ├── user.go
│   ├── product.go
│   └── post.go
└── 003-add-enums/        # Added enum fields for status
    ├── user.go
    ├── product.go
    └── post.go
```

## Entity Evolution Examples

### Version 000-initial: Basic Entities
```go
//migrator:schema:table name="users"
type User struct {
    //migrator:schema:field name="id" type="SERIAL" primary="true"
    ID int64
    
    //migrator:schema:field name="email" type="VARCHAR(255)" not_null="true" unique="true"
    Email string
    
    //migrator:schema:field name="name" type="VARCHAR(255)" not_null="true"
    Name string
}
```

### Version 001-add-fields: Additional Fields
```go
type User struct {
    // ... existing fields ...
    
    //migrator:schema:field name="age" type="INTEGER"
    Age int
    
    //migrator:schema:field name="bio" type="TEXT"
    Bio string
    
    //migrator:schema:field name="active" type="BOOLEAN" not_null="true" default="true"
    Active bool
}
```

### Version 003-add-enums: Enum Support
```go
type User struct {
    // ... existing fields ...
    
    //migrator:schema:field name="status" type="ENUM" enum="active,inactive,suspended" not_null="true" default="active"
    Status string
}
```

## Dynamic Test Scenarios

### 1. Basic Evolution (`dynamic_basic_evolution`)
Tests sequential evolution through all versions: 000 → 001 → 002 → 003

### 2. Skip Versions (`dynamic_skip_versions`)
Tests non-sequential migration: 000 → 002 → 003 (skipping 001)

### 3. Idempotency (`dynamic_idempotency`)
Tests applying the same version multiple times (should be no-op)

### 4. Partial Apply (`dynamic_partial_apply`)
Tests applying to specific version, then continuing to final version

### 5. Schema Diff (`dynamic_schema_diff`)
Tests schema diff generation between versions without applying

### 6. Migration SQL Generation (`dynamic_migration_sql_generation`)
Tests SQL generation from entity changes

## VersionedEntityManager API

```go
// Create manager with fixtures filesystem
vem, err := NewVersionedEntityManager(fixturesFS)
defer vem.Cleanup()

// Load entities from specific version
err = vem.LoadEntityVersion("001-add-fields")

// Generate schema from current entities
schema, err := vem.GenerateSchemaFromEntities()

// Generate migration SQL comparing with database
statements, err := vem.GenerateMigrationSQL(ctx, conn)

// Apply migration from current entities
err = vem.ApplyMigrationFromEntities(ctx, conn, "Add user fields")

// Migrate to specific version (load + apply)
err = vem.MigrateToVersion(ctx, conn, "002-add-posts", "Add posts table")
```

## Running Dynamic Tests

The dynamic scenarios are automatically included in all test runs:

```bash
# Run all tests (includes dynamic scenarios)
docker-compose --profile test run --rm ptah-tester

# Run only dynamic scenarios
docker-compose --profile test run --rm ptah-tester --scenarios=dynamic_basic_evolution,dynamic_idempotency

# Run specific dynamic scenario
docker-compose --profile test run --rm ptah-tester --scenarios=dynamic_basic_evolution --databases=postgres
```

## Benefits Over Static Migrations

1. **Realistic Testing**: Tests actual ptah functionality instead of just applying pre-made SQL
2. **Maintainable**: Entity files are easier to understand and modify than SQL
3. **Comprehensive**: Tests the full pipeline from entities to applied migrations
4. **Flexible**: Easy to add new evolution scenarios by creating new version directories
5. **Self-Documenting**: Entity evolution tells a clear story of schema changes

## Adding New Test Scenarios

1. **Create New Entity Version**: Add new directory under `fixtures/entities/`
2. **Define Evolution**: Create entity files showing the desired changes
3. **Add Test Scenario**: Create new test function in `scenarios_dynamic.go`
4. **Register Scenario**: Add to `GetDynamicScenarios()` function

Example:
```go
{
    Name:        "dynamic_new_scenario",
    Description: "Test new migration pattern",
    TestFunc:    testDynamicNewScenario,
}
```

This approach provides much more realistic and comprehensive testing of the Ptah migration library's core functionality.
