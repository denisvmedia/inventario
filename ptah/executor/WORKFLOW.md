# Complete Database Schema Management Workflow

This document describes the complete workflow for managing database schemas with the **Ptah** schema management tool.

## Overview

Ptah supports a complete database schema management lifecycle:

1. **Generate** → Create SQL schema from Go entities
2. **Write** → Write schema to database (initial setup)
3. **Read** → Read and display current database schema
4. **Compare** → Compare current database with Go entities
5. **Migrate** → Generate migration SQL for differences
6. **Drop Schema** → Drop tables/enums from Go entities (DANGEROUS!)
7. **Drop All** → Drop ALL tables and enums in database (VERY DANGEROUS!)

## Commands

### 1. Generate Schema (SQL Output)

Generate SQL schema from Go entities without touching the database:

```bash
# Generate for all dialects
go run ./ptah/cmd generate --root-dir ./models

# Generate for specific dialect
go run ./ptah/cmd generate --root-dir ./models --dialect postgres
go run ./ptah/cmd generate --root-dir ./models --dialect mysql
go run ./ptah/cmd generate --root-dir ./models --dialect mariadb
```

**Output**: SQL statements printed to console

### 2. Write Schema to Database

Write the generated schema directly to a database:

```bash
go run ./ptah/cmd write-db --root-dir ./models --db-url postgres://user:pass@localhost/db
```

**Dry Run Mode** (preview changes without executing):

```bash
go run ./ptah/cmd write-db --root-dir ./models --db-url postgres://user:pass@localhost/db --dry-run
```

**Features**:
- ✅ Creates enums first (PostgreSQL requirement)
- ✅ Creates tables in dependency order
- ✅ Skips existing tables (safe)
- ✅ Transaction-based (all-or-nothing)
- ✅ Detailed progress output
- ✅ **Dry run support** - preview all SQL operations without executing them

### 3. Read Database Schema

Read and display the current database schema:

```bash
go run ./ptah/cmd read-db --db-url postgres://user:pass@localhost/db
```

**Output**: Complete schema information including tables, columns, constraints, indexes, and enums

### 4. Compare Schemas

Compare your Go entities with the current database schema:

```bash
go run ./ptah/cmd compare --root-dir ./models --db-url postgres://user:pass@localhost/db
```

**Output**: Detailed differences showing what needs to be added, removed, or modified

### 5. Generate Migration SQL

Generate migration SQL to update the database to match your entities:

```bash
go run ./ptah/cmd migrate --root-dir ./models --db-url postgres://user:pass@localhost/db
```

**Output**: SQL statements to apply the changes

### 6. Drop Schema (DANGEROUS!)

Drop tables and enums defined in your Go entities:

```bash
go run ./ptah/cmd drop-schema --root-dir ./models --db-url postgres://user:pass@localhost/db
```

**Dry Run Mode** (preview what would be dropped):

```bash
go run ./ptah/cmd drop-schema --root-dir ./models --db-url postgres://user:pass@localhost/db --dry-run
```

**Features**:
- ⚠️ Requires explicit confirmation (type 'YES') - **skipped in dry run mode**
- ✅ Only drops tables/enums defined in your Go entities
- ✅ Drops tables in reverse dependency order
- ✅ Drops enums after tables (PostgreSQL)
- ✅ Disables foreign key checks during operation (MySQL/MariaDB)
- ✅ Transaction-based (all-or-nothing)
- ✅ Detailed progress output
- ✅ **Dry run support** - preview all drop operations without executing them

**⚠️ WARNING**: This operation permanently deletes data and cannot be undone!

### 7. Drop All Tables (VERY DANGEROUS!)

Drop ALL tables and enums in the entire database:

```bash
go run ./ptah/cmd drop-all --db-url postgres://user:pass@localhost/db
```

**Dry Run Mode** (preview complete database cleanup):

```bash
go run ./ptah/cmd drop-all --db-url postgres://user:pass@localhost/db --dry-run
```

**Features**:
- 🚨 Requires double confirmation ('DELETE EVERYTHING' + 'YES I AM SURE') - **skipped in dry run mode**
- ✅ Drops ALL tables in the database (not just Go entities)
- ✅ Drops ALL enums in the database (PostgreSQL)
- ✅ Drops ALL sequences in the database (PostgreSQL)
- ✅ Queries database for complete object list
- ✅ Disables foreign key checks during operation (MySQL/MariaDB)
- ✅ Transaction-based (all-or-nothing)
- ✅ Detailed progress output
- ✅ **Dry run support** - preview complete database cleanup without executing

**🚨 EXTREME WARNING**: This operation completely empties the database and cannot be undone!

## Complete Workflow Example

### Step 1: Initial Setup

1. **Create your Go entities** with migrator directives:

```go
//migrator:schema:table name="users"
type User struct {
    //migrator:schema:field name="id" type="SERIAL" primary="true"
    ID int `json:"id"`

    //migrator:schema:field name="email" type="VARCHAR(255)" unique="true" not_null="true"
    Email string `json:"email"`

    //migrator:schema:field name="role" type="ENUM" enum="admin,user,guest" default="user"
    Role string `json:"role"`
}
```

2. **Generate and review the schema**:

```bash
go run ./ptah/cmd generate --root-dir ./models --dialect postgres
```

3. **Write the schema to your database**:

```bash
go run ./ptah/cmd write-db --root-dir ./models --db-url postgres://user:pass@localhost/mydb
```

### Step 2: Making Changes

1. **Update your Go entities** (e.g., add a new field):

```go
//migrator:schema:table name="users"
type User struct {
    //migrator:schema:field name="id" type="SERIAL" primary="true"
    ID int `json:"id"`

    //migrator:schema:field name="email" type="VARCHAR(255)" unique="true" not_null="true"
    Email string `json:"email"`

    //migrator:schema:field name="role" type="ENUM" enum="admin,user,guest,moderator" default="user"
    Role string `json:"role"`

    // NEW FIELD
    //migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="NOW()"
    CreatedAt time.Time `json:"created_at"`
}
```

2. **Compare with the database**:

```bash
go run ./ptah/cmd compare --root-dir ./models --db-url postgres://user:pass@localhost/mydb
```

**Example Output**:
```
=== SCHEMA DIFFERENCES DETECTED ===

SUMMARY: 2 changes detected
- Tables: +0 -0 ~1
- Enums: +0 -0 ~1

🔧 TABLES TO MODIFY:
  ~ users
    + Column: created_at

🔧 ENUMS TO MODIFY:
  ~ enum_user_role
    + Value: moderator
```

3. **Generate migration SQL**:

```bash
go run ./ptah/cmd migrate --root-dir ./models --db-url postgres://user:pass@localhost/mydb
```

**Example Output**:
```sql
-- Migration generated from schema differences
-- Generated on: 2024-01-15 10:30:00
-- Source: ./models
-- Target: postgres://user:***@localhost/mydb

ALTER TYPE enum_user_role ADD VALUE 'moderator';
-- TODO: ALTER TABLE users ADD COLUMN created_at TIMESTAMP NOT NULL DEFAULT NOW();

Generated 2 migration statements.
⚠️  Review the SQL carefully before executing!
```

4. **Apply the migration** (manually for now):

```sql
-- Copy the generated SQL and execute it in your database
ALTER TYPE enum_user_role ADD VALUE 'moderator';
ALTER TABLE users ADD COLUMN created_at TIMESTAMP NOT NULL DEFAULT NOW();
```

### Step 3: Verification

1. **Read the updated schema**:

```bash
go run ./ptah/cmd read-db --db-url postgres://user:pass@localhost/mydb
```

2. **Compare again** (should show no differences):

```bash
go run ./ptah/cmd compare --root-dir ./models --db-url postgres://user:pass@localhost/mydb
```

**Expected Output**:
```
=== NO SCHEMA CHANGES DETECTED ===
The database schema matches your entity definitions.
```

## Dry Run Mode

All destructive operations support **dry run mode** for safe preview of changes:

### What is Dry Run Mode?

Dry run mode allows you to preview exactly what SQL operations would be executed without actually making any changes to your database. This is especially useful for:

- **Testing configurations** before applying to production
- **Reviewing changes** in CI/CD pipelines
- **Learning** what operations each command performs
- **Debugging** schema generation issues

### Commands Supporting Dry Run

| Command | Dry Run Flag | Description |
|---------|-------------|-------------|
| `write-db` | `--dry-run` | Preview schema creation without executing |
| `drop-schema` | `--dry-run` | Preview table/enum drops without executing |
| `drop-all` | `--dry-run` | Preview complete database cleanup without executing |

### Dry Run Output Format

When dry run mode is enabled, you'll see:

```
[DRY RUN] Would begin transaction
[DRY RUN] Would execute SQL: CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest')
[DRY RUN] Would execute SQL: CREATE TABLE users (id SERIAL PRIMARY KEY, ...)
[DRY RUN] Would commit transaction
✅ [DRY RUN] Schema operations completed successfully!
```

### Safety Features in Dry Run Mode

- **No database connections** for actual writes
- **No confirmations required** - dangerous operations show preview without prompts
- **Complete SQL preview** - see exactly what would be executed
- **Transaction simulation** - shows transaction boundaries
- **Error-free exploration** - safe to run against any database

## Advanced Features

### Transaction Safety

The `write-db` command uses database transactions:
- ✅ All changes are applied atomically
- ✅ If any error occurs, all changes are rolled back
- ✅ Database remains in consistent state

### Existing Schema Detection

The `write-db` command safely handles existing schemas:
- ✅ Skips tables that already exist
- ✅ Shows warnings about existing tables
- ✅ Suggests using `compare` or `migrate` for updates

### Password Security

All commands automatically mask passwords in output:
- ✅ `postgres://user:***@localhost/db` (password hidden)
- ✅ Safe for logs and screenshots

### Error Handling

Comprehensive error messages for common issues:
- ✅ Connection failures
- ✅ Permission errors
- ✅ Invalid entity definitions
- ✅ SQL execution errors

## Supported Database Features

### PostgreSQL ✅
- Tables, views, columns
- Primary keys, foreign keys, unique constraints
- Check constraints
- Indexes (regular, unique, primary)
- Enum types with values
- Auto-increment (SERIAL) detection
- Comments and metadata

### MySQL/MariaDB ✅
- Tables, views, columns
- Primary keys, foreign keys, unique constraints
- Check constraints (MySQL 8.0+)
- Indexes (regular, unique, primary)
- Enum types with values
- Auto-increment detection
- Full schema reading and writing
- Schema comparison and migration generation

## Best Practices

1. **Always review migration SQL** before executing
2. **Test migrations on a copy** of your production database first
3. **Backup your database** before applying migrations
4. **Use version control** for your Go entity definitions
5. **Document breaking changes** in your migration comments

## Troubleshooting

### Connection Issues
- Ensure database server is running
- Check connection parameters (host, port, credentials)
- Verify database exists and user has permissions

### Schema Parsing Issues
- Check migrator directive syntax
- Ensure all required fields are specified
- Verify enum values are comma-separated

### Migration Issues
- Review generated SQL carefully
- Test on development database first
- Consider data migration needs for breaking changes

## Future Enhancements

- [ ] Automatic migration execution with confirmation
- [ ] Migration versioning and history
- [ ] Data migration support
- [ ] Schema validation and linting
- [ ] Migration rollback generation
