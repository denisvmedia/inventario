# Ptah Migrator

The Ptah Migrator provides versioned database migration capabilities with up/down directions, inspired by the PostgreSQL migrations from the registry package but adapted for the Ptah ecosystem.

## Features

- **Versioned Migrations**: Each migration has a unique version number and description
- **Up/Down Migrations**: Support for both applying and rolling back migrations
- **Transaction Safety**: Each migration runs in its own transaction
- **SQL File Support**: Migrations can be defined as SQL files
- **Go Function Support**: Migrations can also be defined as Go functions for complex logic
- **Multiple Database Support**: Works with PostgreSQL and MySQL through Ptah's executor package
- **Dry Run Mode**: Preview what migrations would do without actually applying them
- **Migration Status**: Check current migration state and pending migrations

## Migration File Structure

Migrations are stored with the following naming convention:

```
NNNNNNNNNN_description.up.sql    # Up migration
NNNNNNNNNN_description.down.sql  # Down migration
```

Where:
- `NNNNNNNNNN` is a 10-digit version number (e.g., `0000000001`)
- `description` is a snake_case description of the migration
- Each migration must have both `.up.sql` and `.down.sql` files

### Filesystem Requirements

The `RegisterMigrations` function accepts an `fs.FS` parameter where migrations should be located in the root directory. It's the caller's responsibility to prepare the filesystem correctly:

```go
// For embedded migrations, use a subdirectory
migrationsFS := must.Must(fs.Sub(GetMigrations(), "source"))
err := RegisterMigrations(migrator, migrationsFS)

// For directory on disk
migrationsFS := os.DirFS("/path/to/migrations")
err := RegisterMigrations(migrator, migrationsFS)

// For convenience, use helper functions
err := RegisterMigrationsFromEmbedded(migrator)  // Uses embedded source/ directory
err := RegisterMigrationsFromDirectory(migrator, "/path/to/migrations")
```

### Example Migration Files

**0000000001_create_users_table.up.sql:**
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
```

**0000000001_create_users_table.down.sql:**
```sql
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
```

## Command Line Interface

### Migrate Up
Apply all pending migrations:
```bash
go run ./cmd migrate-up --db-url postgres://user:pass@localhost/db --migrations-dir /path/to/migrations
```

With dry run:
```bash
go run ./cmd migrate-up --db-url postgres://user:pass@localhost/db --migrations-dir /path/to/migrations --dry-run
```

### Migrate Down
Roll back to a specific version:
```bash
go run ./cmd migrate-down --db-url postgres://user:pass@localhost/db --migrations-dir /path/to/migrations --target 5
```

With confirmation skip (dangerous!):
```bash
go run ./cmd migrate-down --db-url postgres://user:pass@localhost/db --migrations-dir /path/to/migrations --target 5 --confirm
```

### Migration Status
Check current migration status:
```bash
go run ./cmd migrate-status --db-url postgres://user:pass@localhost/db --migrations-dir /path/to/migrations
```

Verbose output:
```bash
go run ./cmd migrate-status --db-url postgres://user:pass@localhost/db --migrations-dir /path/to/migrations --verbose
```

JSON output:
```bash
go run ./cmd migrate-status --db-url postgres://user:pass@localhost/db --migrations-dir /path/to/migrations --json
```

## Programmatic Usage

### Basic Migration Execution

```go
package main

import (
    "context"
    "github.com/denisvmedia/inventario/ptah/executor"
    "github.com/denisvmedia/inventario/ptah/migrator"
)

func main() {
    // Connect to database
    conn, err := executor.ConnectToDatabase("postgres://user:pass@localhost/db")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    // Create filesystem from migrations directory
    migrationsFS := os.DirFS("/path/to/migrations")

    // Run all pending migrations
    err = migrator.RunMigrations(context.Background(), conn, migrationsFS)
    if err != nil {
        panic(err)
    }
}
```

### Custom Migration Registration

```go
// Register migrations from a custom filesystem
m := migrator.NewMigrator(conn)

// Option 1: Register from a directory on disk
err := migrator.RegisterMigrationsFromDirectory(m, "/path/to/migrations")

// Option 2: Register from a custom filesystem
customFS := os.DirFS("/custom/path")
err := migrator.RegisterMigrations(m, customFS)

// Option 3: Register from example migrations
exampleFS := migrator_examples.GetExampleMigrations()
migrationsFS := must.Must(fs.Sub(exampleFS, "migrations"))
err := migrator.RegisterMigrations(m, migrationsFS)

// Option 4: Register a Go-based migration
upFunc := func(ctx context.Context, conn *executor.DatabaseConnection) error {
    return conn.Writer().ExecuteSQL("CREATE TABLE test (id SERIAL PRIMARY KEY)")
}

downFunc := func(ctx context.Context, conn *executor.DatabaseConnection) error {
    return conn.Writer().ExecuteSQL("DROP TABLE test")
}

migrator.RegisterGoMigration(m, 1001, "Create test table", upFunc, downFunc)
```

### Migration Status Checking

```go
// Create filesystem from migrations directory
migrationsFS := os.DirFS("/path/to/migrations")

status, err := migrator.GetMigrationStatus(context.Background(), conn, migrationsFS)
if err != nil {
    panic(err)
}

fmt.Printf("Current version: %d\n", status.CurrentVersion)
fmt.Printf("Pending migrations: %d\n", len(status.PendingMigrations))

if status.HasPendingChanges {
    fmt.Println("Database needs migration!")
}
```

## Migration Table

The migrator automatically creates a `schema_migrations` table to track applied migrations:

```sql
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    description TEXT NOT NULL,
    applied_at TIMESTAMP NOT NULL
);
```

## Best Practices

1. **Always create both up and down migrations**: Every migration should be reversible
2. **Use descriptive names**: Make migration purposes clear from the filename
3. **Keep migrations small**: Each migration should make one focused change
4. **Test migrations**: Always test both up and down migrations before deploying
5. **Use transactions**: The migrator automatically wraps migrations in transactions
6. **Backup before rollbacks**: Down migrations can cause data loss
7. **Version numbers**: Use sequential version numbers or timestamps

## Safety Features

- **Transaction Wrapping**: Each migration runs in its own transaction
- **Rollback on Failure**: If a migration fails, the transaction is rolled back
- **Confirmation Prompts**: Down migrations require confirmation (unless `--confirm` is used)
- **Dry Run Mode**: Preview migrations without applying them
- **Validation**: Migrations are validated before execution

## Limitations

- **Query Support**: The current implementation has limited query support due to executor interface constraints
- **Version Tracking**: Current version tracking is simplified and may need enhancement for production use
- **Concurrent Migrations**: No built-in protection against concurrent migration execution

## Integration with Ptah

The migrator integrates seamlessly with Ptah's existing infrastructure:

- Uses Ptah's executor package for database connections
- Supports the same databases as Ptah (PostgreSQL, MySQL)
- Follows Ptah's transaction and error handling patterns
- Compatible with Ptah's dry-run functionality

## Future Enhancements

- Enhanced query support in executor interfaces
- Migration locking to prevent concurrent execution
- Migration dependency resolution
- Schema validation after migrations
- Migration performance metrics
- Web UI for migration management
