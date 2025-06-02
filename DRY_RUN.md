# Dry Run Mode Documentation

This document describes the dry run functionality implemented across all write operations in both the Ptah schema management tool and the main Inventario application.

## Overview

Dry run mode allows you to preview exactly what operations would be performed without actually executing them. This provides a safe way to:

- Test configurations before applying to production
- Review changes in CI/CD pipelines  
- Learn what operations each command performs
- Debug schema generation issues
- Validate SQL generation without database changes

## Supported Commands

### Ptah Schema Management Tool

All destructive Ptah commands support the `--dry-run` flag:

#### 1. Write Schema (`write-db`)

**Normal execution:**
```bash
go run ./ptah/cmd write-db --root-dir ./models --db-url postgres://user:pass@localhost/db
```

**Dry run mode:**
```bash
go run ./ptah/cmd write-db --root-dir ./models --db-url postgres://user:pass@localhost/db --dry-run
```

**What it shows:**
- All CREATE TABLE statements that would be executed
- All CREATE TYPE (enum) statements for PostgreSQL
- Transaction begin/commit operations
- Table creation order and dependencies

#### 2. Drop Schema (`drop-schema`)

**Normal execution:**
```bash
go run ./ptah/cmd drop-schema --root-dir ./models --db-url postgres://user:pass@localhost/db
```

**Dry run mode:**
```bash
go run ./ptah/cmd drop-schema --root-dir ./models --db-url postgres://user:pass@localhost/db --dry-run
```

**What it shows:**
- All DROP TABLE statements that would be executed
- All DROP TYPE statements for PostgreSQL
- Drop order (reverse dependency order)
- **No confirmation prompts** - safe preview without user interaction

#### 3. Drop All Tables (`drop-all`)

**Normal execution:**
```bash
go run ./ptah/cmd drop-all --db-url postgres://user:pass@localhost/db
```

**Dry run mode:**
```bash
go run ./ptah/cmd drop-all --db-url postgres://user:pass@localhost/db --dry-run
```

**What it shows:**
- All tables that would be dropped from the database
- All enums that would be dropped (PostgreSQL)
- All sequences that would be dropped (PostgreSQL)
- **No confirmation prompts** - safe preview of complete database cleanup

### Inventario Application Commands

#### 4. Database Migrations (`migrate`)

**Normal execution:**
```bash
go run . migrate --db-dsn postgres://user:pass@localhost/db
```

**Dry run mode:**
```bash
go run . migrate --db-dsn postgres://user:pass@localhost/db --dry-run
```

**Current status:** Partially implemented - shows informational message about dry run mode not being fully implemented yet.

#### 5. Database Seeding (`seed`)

**Normal execution:**
```bash
go run . seed --db-dsn postgres://user:pass@localhost/db
```

**Dry run mode:**
```bash
go run . seed --db-dsn postgres://user:pass@localhost/db --dry-run
```

**Current status:** Partially implemented - shows informational message about what data would be seeded.

## Implementation Details

### Core Components Modified

1. **SchemaWriter Interface** (`ptah/executor/types.go`)
   - Added `SetDryRun(bool)` method
   - Added `IsDryRun() bool` method

2. **PostgreSQL Writer** (`ptah/executor/postgres.go`)
   - Added `dryRun` field to struct
   - Modified all write methods to check dry run mode
   - SQL execution shows preview instead of executing

3. **MySQL Writer** (`ptah/executor/mysql.go`)
   - Added `dryRun` field to struct
   - Modified all write methods to check dry run mode
   - SQL execution shows preview instead of executing

4. **Command Implementations**
   - Added `--dry-run` flag to all write commands
   - Modified command logic to set dry run mode on writers
   - Updated output messages to indicate dry run mode

### Dry Run Output Format

When dry run mode is enabled, all operations are prefixed with `[DRY RUN]`:

```
[DRY RUN] Would write schema from ./models to database postgres://user:***@localhost/db
=== DRY RUN: WRITE SCHEMA TO DATABASE ===

Connected to postgres database successfully!
[DRY RUN] Would begin transaction
Creating enum: user_role
[DRY RUN] Would execute SQL: CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest')
Creating table 1/2...
[DRY RUN] Would execute SQL: CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    role user_role DEFAULT 'user'
)
Creating table 2/2...
[DRY RUN] Would execute SQL: CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    title VARCHAR(255) NOT NULL
)
[DRY RUN] Would commit transaction
✅ [DRY RUN] Schema operations completed successfully!
```

### Safety Features

1. **No Database Writes**: In dry run mode, no actual SQL is executed against the database
2. **No Confirmations**: Dangerous operations skip confirmation prompts in dry run mode
3. **Complete Preview**: Shows exactly what SQL would be executed
4. **Transaction Simulation**: Shows transaction boundaries and operations
5. **Error-Free**: Safe to run against any database without risk

## Usage Examples

### Testing Schema Changes

Before applying schema changes to production:

```bash
# Preview what would be created
go run ./ptah/cmd write-db --root-dir ./models --db-url postgres://prod-user:pass@prod-db/myapp --dry-run

# If output looks correct, run without --dry-run
go run ./ptah/cmd write-db --root-dir ./models --db-url postgres://prod-user:pass@prod-db/myapp
```

### CI/CD Integration

In your CI pipeline, always run dry run first:

```yaml
- name: Preview Schema Changes
  run: |
    go run ./ptah/cmd write-db --root-dir ./models --db-url $DATABASE_URL --dry-run

- name: Apply Schema Changes
  run: |
    go run ./ptah/cmd write-db --root-dir ./models --db-url $DATABASE_URL
  if: github.ref == 'refs/heads/main'
```

### Learning and Debugging

Use dry run to understand what operations are performed:

```bash
# See what tables would be created
go run ./ptah/cmd write-db --root-dir ./models --db-url postgres://localhost/test --dry-run

# See what would be dropped
go run ./ptah/cmd drop-schema --root-dir ./models --db-url postgres://localhost/test --dry-run
```

## Future Enhancements

1. **Full Migration Dry Run**: Complete implementation of dry run mode for database migrations
2. **Seed Data Preview**: Show actual seed data that would be inserted
3. **Diff Output**: Show before/after comparisons in dry run mode
4. **JSON Output**: Machine-readable dry run output for tooling integration
5. **Rollback Preview**: Show what rollback operations would look like

## Best Practices

1. **Always test first**: Use dry run mode before applying changes to important databases
2. **Review output carefully**: Check that the generated SQL matches your expectations
3. **Use in CI/CD**: Integrate dry run checks into your deployment pipeline
4. **Document changes**: Save dry run output as documentation of schema changes
5. **Validate against multiple databases**: Test dry run against different database types if you support multiple
