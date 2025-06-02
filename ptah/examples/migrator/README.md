# Migrator Examples

This directory contains example migrations and usage patterns for the Ptah migrator.

## Example Migrations

The `migrations/` directory contains sample migration files that demonstrate:

1. **0000000001_initial_schema** - Basic table creation
2. **0000000002_add_users_table** - Table with indexes and constraints
3. **0000000003_add_user_profile_fields** - ALTER TABLE operations

## Usage

### Using Example Migrations

```go
package main

import (
    "context"
    "io/fs"
    
    "github.com/go-extras/go-kit/must"
    
    "github.com/denisvmedia/inventario/ptah/executor"
    "github.com/denisvmedia/inventario/ptah/migrator"
    migrator_examples "github.com/denisvmedia/inventario/ptah/examples/migrator"
)

func main() {
    // Connect to database
    conn, err := executor.ConnectToDatabase("postgres://user:pass@localhost/db")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    // Create migrator
    m := migrator.NewMigrator(conn)

    // Register example migrations
    exampleFS := migrator_examples.GetExampleMigrations()
    migrationsFS := must.Must(fs.Sub(exampleFS, "migrations"))
    
    err = migrator.RegisterMigrations(m, migrationsFS)
    if err != nil {
        panic(err)
    }

    // Run migrations
    err = m.MigrateUp(context.Background())
    if err != nil {
        panic(err)
    }
}
```

### Creating Your Own Migrations

1. Create a directory for your migrations
2. Add `.up.sql` and `.down.sql` files following the naming convention
3. Use `os.DirFS()` or embed them in your application

```go
// From directory
migrationsFS := os.DirFS("/path/to/your/migrations")
err := migrator.RegisterMigrations(m, migrationsFS)

// From embedded filesystem
//go:embed your_migrations
var yourMigrations embed.FS

migrationsFS := must.Must(fs.Sub(yourMigrations, "your_migrations"))
err := migrator.RegisterMigrations(m, migrationsFS)
```

## Migration File Naming

Follow this pattern:
- `NNNNNNNNNN_description.up.sql` - Forward migration
- `NNNNNNNNNN_description.down.sql` - Rollback migration

Where:
- `NNNNNNNNNN` is a 10-digit version number (e.g., `0000000001`)
- `description` is a snake_case description

## Best Practices

1. **Always create both up and down migrations**
2. **Test migrations on a copy of production data**
3. **Keep migrations small and focused**
4. **Use transactions (handled automatically by the migrator)**
5. **Review down migrations carefully - they can cause data loss**

## Command Line Usage

You can also use these examples with the command line tools by pointing them to the migrations directory:

```bash
# Note: You would need to extract the migrations to a directory first
# as the CLI tools expect a directory path, not an embedded filesystem

go run ./cmd migrate-status --db-url postgres://user:pass@localhost/db
go run ./cmd migrate-up --db-url postgres://user:pass@localhost/db
go run ./cmd migrate-down --db-url postgres://user:pass@localhost/db --target 1
```
