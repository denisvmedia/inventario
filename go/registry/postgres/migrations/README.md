# PostgreSQL Database Migrations

This package provides a system for managing PostgreSQL database schema migrations for the Inventario application.

## What are Migrations?

Database migrations are a way to manage changes to your database schema over time. They allow you to:

1. Version your database schema
2. Apply changes incrementally
3. Roll back changes if needed
4. Keep track of which changes have been applied
5. Share schema changes with other developers

## How Migrations Work

The migration system uses a table called `schema_migrations` to track which migrations have been applied. Each migration has:

- A version number (integer)
- A description
- An "up" function that applies the migration
- A "down" function that reverts the migration

Migrations are applied in order of their version numbers.

## Creating a New Migration

To create a new migration, follow these steps:

1. Create a new file in the `migrations` package with a name that follows the pattern `NNN_description.go`, where `NNN` is the version number (e.g., `002_add_user_table.go`).

2. Implement your migration using the following template:

```go
package migrations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// YourMigrationName returns a migration that [describe what it does]
func YourMigrationName() *Migration {
	return &Migration{
		Version:     2, // Increment this number for each new migration
		Description: "Description of what this migration does",
		Up: func(ctx context.Context, pool *pgxpool.Pool) error {
			// SQL to apply the migration
			_, err := pool.Exec(ctx, `
				-- Your SQL statements here
				CREATE TABLE IF NOT EXISTS your_table (
					id TEXT PRIMARY KEY,
					name TEXT NOT NULL
				);
			`)
			return err
		},
		Down: func(ctx context.Context, pool *pgxpool.Pool) error {
			// SQL to revert the migration
			_, err := pool.Exec(ctx, `
				DROP TABLE IF EXISTS your_table;
			`)
			return err
		},
	}
}
```

3. Register your migration in the `register.go` file by adding it to the `RegisterMigrations` function:

```go
// RegisterMigrations registers all migrations with the migrator
func RegisterMigrations(migrator *Migrator) {
	// Register migrations in order
	migrator.Register(InitialSchemaMigration())
	migrator.Register(YourMigrationName()) // Add your migration here
	// Add more migrations here as needed
}
```

## Best Practices for Writing Migrations

1. **Make migrations idempotent**: Use `IF NOT EXISTS` and `IF EXISTS` clauses to ensure migrations can be run multiple times without errors.

2. **Keep migrations small**: Each migration should make a small, focused change to the schema.

3. **Always implement the Down function**: This allows you to roll back changes if needed.

4. **Use transactions**: The migration system automatically wraps each migration in a transaction, so if part of a migration fails, the entire migration is rolled back.

5. **Test your migrations**: Before committing a new migration, test it by running it against a test database.

6. **Version numbers**: Always increment the version number for each new migration. Use sequential numbers (1, 2, 3, etc.).

7. **Descriptive names**: Use descriptive names for your migration files and functions.

## Example: Adding a New Table

Here's an example of a migration that adds a new table:

```go
package migrations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AddUserTableMigration returns a migration that adds the users table
func AddUserTableMigration() *Migration {
	return &Migration{
		Version:     2,
		Description: "Add users table",
		Up: func(ctx context.Context, pool *pgxpool.Pool) error {
			_, err := pool.Exec(ctx, `
				CREATE TABLE IF NOT EXISTS users (
					id TEXT PRIMARY KEY,
					username TEXT NOT NULL UNIQUE,
					email TEXT NOT NULL UNIQUE,
					password_hash TEXT NOT NULL,
					created_at TIMESTAMP NOT NULL DEFAULT NOW(),
					updated_at TIMESTAMP NOT NULL DEFAULT NOW()
				);
			`)
			return err
		},
		Down: func(ctx context.Context, pool *pgxpool.Pool) error {
			_, err := pool.Exec(ctx, `
				DROP TABLE IF EXISTS users;
			`)
			return err
		},
	}
}
```

## Example: Modifying an Existing Table

Here's an example of a migration that modifies an existing table:

```go
package migrations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AddUserRoleMigration returns a migration that adds a role column to the users table
func AddUserRoleMigration() *Migration {
	return &Migration{
		Version:     3,
		Description: "Add role column to users table",
		Up: func(ctx context.Context, pool *pgxpool.Pool) error {
			_, err := pool.Exec(ctx, `
				ALTER TABLE users
				ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';
			`)
			return err
		},
		Down: func(ctx context.Context, pool *pgxpool.Pool) error {
			_, err := pool.Exec(ctx, `
				ALTER TABLE users
				DROP COLUMN IF EXISTS role;
			`)
			return err
		},
	}
}
```

## How Migrations Are Applied

Migrations are automatically applied when the application starts up. The migration system:

1. Checks which migrations have already been applied by querying the `schema_migrations` table
2. Applies any new migrations in order of their version numbers
3. Records each successful migration in the `schema_migrations` table

You can also manually run migrations using the `RunMigrations` function:

```go
err := migrations.RunMigrations(context.Background(), pool)
if err != nil {
    // Handle error
}
```

## Rolling Back Migrations

To roll back migrations to a specific version, you can use the `MigrateDown` method:

```go
migrator := migrations.NewMigrator(pool)
migrations.RegisterMigrations(migrator)
err := migrator.MigrateDown(context.Background(), targetVersion)
if err != nil {
    // Handle error
}
```

Where `targetVersion` is the version number you want to roll back to. All migrations with a version number higher than `targetVersion` will be rolled back.
