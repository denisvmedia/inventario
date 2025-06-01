package postgresql_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/postgresql"
	"github.com/denisvmedia/inventario/ptah/schema/differ/differtypes"
)

func TestGenerator_New(t *testing.T) {
	c := qt.New(t)

	generator := postgresql.New()
	c.Assert(generator, qt.IsNotNil)
	c.Assert(generator.GetDialectName(), qt.Equals, "postgres")
}

func TestGenerator_GenerateCreateTable_BasicTable(t *testing.T) {
	c := qt.New(t)

	generator := postgresql.New()

	table := goschema.Table{
		StructName: "User",
		Name:       "users",
		Comment:    "User accounts table",
	}

	fields := []goschema.Field{
		{
			StructName: "User",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
		},
		{
			StructName: "User",
			Name:       "email",
			Type:       "VARCHAR(320)",
			Nullable:   false,
			Unique:     true,
		},
		{
			StructName: "User",
			Name:       "name",
			Type:       "VARCHAR(255)",
			Nullable:   false,
		},
		{
			StructName: "User",
			Name:       "created_at",
			Type:       "TIMESTAMP",
			Nullable:   false,
			Default:    "NOW()",
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "-- POSTGRES TABLE: users (User accounts table) --")
	c.Assert(result, qt.Contains, "CREATE TABLE users")
	c.Assert(result, qt.Contains, "id SERIAL PRIMARY KEY")
	c.Assert(result, qt.Contains, "email VARCHAR(320) UNIQUE NOT NULL")
	c.Assert(result, qt.Contains, "name VARCHAR(255) NOT NULL")
	c.Assert(result, qt.Contains, "created_at TIMESTAMP NOT NULL DEFAULT 'NOW()'")
}

func TestGenerator_GenerateCreateTable_WithEnums(t *testing.T) {
	c := qt.New(t)

	generator := postgresql.New()

	table := goschema.Table{
		StructName: "User",
		Name:       "users",
	}

	fields := []goschema.Field{
		{
			StructName: "User",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
		},
		{
			StructName: "User",
			Name:       "status",
			Type:       "user_status_enum",
			Nullable:   false,
		},
		{
			StructName: "User",
			Name:       "role",
			Type:       "user_role_enum",
			Nullable:   false,
			Default:    "user",
		},
	}

	enums := []goschema.Enum{
		{
			Name:   "user_status_enum",
			Values: []string{"active", "inactive", "pending"},
		},
		{
			Name:   "user_role_enum",
			Values: []string{"admin", "user", "guest"},
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, enums)

	// PostgreSQL should create enum types first
	c.Assert(result, qt.Contains, "CREATE TYPE user_status_enum AS ENUM ('active', 'inactive', 'pending');")
	c.Assert(result, qt.Contains, "CREATE TYPE user_role_enum AS ENUM ('admin', 'user', 'guest');")
	c.Assert(result, qt.Contains, "status user_status_enum NOT NULL")
	c.Assert(result, qt.Contains, "role user_role_enum NOT NULL DEFAULT 'user'")
}

func TestGenerator_GenerateCreateTable_WithIndexes(t *testing.T) {
	c := qt.New(t)

	generator := postgresql.New()

	table := goschema.Table{
		StructName: "User",
		Name:       "users",
	}

	fields := []goschema.Field{
		{
			StructName: "User",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
		},
		{
			StructName: "User",
			Name:       "email",
			Type:       "VARCHAR(320)",
			Nullable:   false,
		},
		{
			StructName: "User",
			Name:       "name",
			Type:       "VARCHAR(255)",
			Nullable:   false,
		},
	}

	indexes := []goschema.Index{
		{
			StructName: "User",
			Name:       "idx_users_email",
			Fields:     []string{"email"},
			Unique:     true,
		},
		{
			StructName: "User",
			Name:       "idx_users_name",
			Fields:     []string{"name"},
			Unique:     false,
		},
		{
			StructName: "Post", // Different struct - should be ignored
			Name:       "idx_posts_title",
			Fields:     []string{"title"},
		},
	}

	result := generator.GenerateCreateTable(table, fields, indexes, nil)

	c.Assert(result, qt.Contains, "CREATE UNIQUE INDEX idx_users_email ON users (email);")
	c.Assert(result, qt.Contains, "CREATE INDEX idx_users_name ON users (name);")
	c.Assert(result, qt.Not(qt.Contains), "idx_posts_title") // Should not include Post indexes
}

func TestGenerator_GenerateCreateTable_WithTypeOverrides(t *testing.T) {
	c := qt.New(t)

	generator := postgresql.New()

	table := goschema.Table{
		StructName: "User",
		Name:       "users",
	}

	fields := []goschema.Field{
		{
			StructName: "User",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
		},
		{
			StructName: "User",
			Name:       "data",
			Type:       "TEXT", // Default type
			Nullable:   true,
			Overrides: map[string]map[string]string{
				"postgres": {
					"type": "JSONB", // PostgreSQL-specific override
				},
			},
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "data JSONB")        // Should use the PostgreSQL override
	c.Assert(result, qt.Not(qt.Contains), "data TEXT") // Should not use the default type
}

func TestGenerator_GenerateCreateTable_CompositeKeys(t *testing.T) {
	c := qt.New(t)

	generator := postgresql.New()

	table := goschema.Table{
		StructName: "UserRole",
		Name:       "user_roles",
		PrimaryKey: []string{"user_id", "role_id"},
	}

	fields := []goschema.Field{
		{
			StructName: "UserRole",
			Name:       "user_id",
			Type:       "INTEGER",
			Nullable:   false,
		},
		{
			StructName: "UserRole",
			Name:       "role_id",
			Type:       "INTEGER",
			Nullable:   false,
		},
		{
			StructName: "UserRole",
			Name:       "assigned_at",
			Type:       "TIMESTAMP",
			Nullable:   false,
			Default:    "NOW()",
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "user_id INTEGER NOT NULL")
	c.Assert(result, qt.Contains, "role_id INTEGER NOT NULL")
	c.Assert(result, qt.Contains, "PRIMARY KEY (user_id, role_id)")
}

func TestGenerator_GenerateAlterStatements(t *testing.T) {
	c := qt.New(t)

	generator := postgresql.New()

	oldFields := []goschema.Field{
		{
			StructName: "User",
			Name:       "email",
			Type:       "TEXT",
			Nullable:   true,
		},
	}

	newFields := []goschema.Field{
		{
			StructName: "User",
			Name:       "email",
			Type:       "VARCHAR(320)",
			Nullable:   false,
		},
		{
			StructName: "User",
			Name:       "name",
			Type:       "VARCHAR(255)",
			Nullable:   false,
		},
	}

	result := generator.GenerateAlterStatements(oldFields, newFields)

	c.Assert(result, qt.Contains, "-- ALTER statements: --")
	c.Assert(result, qt.Contains, "ALTER TABLE User ADD COLUMN name VARCHAR(255) NOT NULL;")
	c.Assert(result, qt.Contains, "ALTER TABLE User ALTER COLUMN email TYPE VARCHAR(320);")
	c.Assert(result, qt.Contains, "ALTER TABLE User ALTER COLUMN email SET NOT NULL;")
}

func TestGenerator_GenerateCreateTable_IgnoresDifferentStructs(t *testing.T) {
	c := qt.New(t)

	generator := postgresql.New()

	table := goschema.Table{
		StructName: "User",
		Name:       "users",
	}

	fields := []goschema.Field{
		{
			StructName: "User",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
		},
		{
			StructName: "Post", // Different struct - should be ignored
			Name:       "title",
			Type:       "VARCHAR(255)",
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "id SERIAL PRIMARY KEY")
	c.Assert(result, qt.Not(qt.Contains), "title") // Should not include Post fields
}

func TestGenerator_GenerateCreateTable_WithEnumsInContext(t *testing.T) {
	c := qt.New(t)

	generator := postgresql.New()

	table := goschema.Table{
		StructName: "User",
		Name:       "users",
	}

	fields := []goschema.Field{
		{
			StructName: "User",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
		},
		{
			StructName: "User",
			Name:       "status",
			Type:       "user_status", // This should be recognized as an enum
			Nullable:   false,
		},
	}

	enums := []goschema.Enum{
		{
			Name:   "user_status",
			Values: []string{"active", "inactive", "suspended"},
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, enums)

	// Should contain the enum definition
	c.Assert(result, qt.Contains, "CREATE TYPE user_status AS ENUM ('active', 'inactive', 'suspended');")
	// Should use the enum type directly in the column
	c.Assert(result, qt.Contains, "status user_status NOT NULL")
}

func TestGenerator_GenerateMigrationSQL_AlterColumn(t *testing.T) {
	c := qt.New(t)

	generator := postgresql.New()

	// Create a mock schema diff with column modifications
	diff := &differtypes.SchemaDiff{
		TablesModified: []differtypes.TableDiff{
			{
				TableName: "users",
				ColumnsModified: []differtypes.ColumnDiff{
					{
						ColumnName: "email",
						Changes: map[string]string{
							"type":     "VARCHAR(255) -> VARCHAR(320)",
							"nullable": "true -> false",
						},
					},
				},
			},
		},
	}

	// Create generated schema with the target field
	generated := &goschema.Database{
		Fields: []goschema.Field{
			{
				StructName: "users",
				Name:       "email",
				Type:       "VARCHAR(320)",
				Nullable:   false,
			},
		},
	}

	result := generator.GenerateMigrationSQL(diff, generated)

	// Should contain ALTER COLUMN statements
	c.Assert(result, qt.HasLen, 3) // Comment + SQL
	c.Assert(result[0], qt.Contains, "-- Modify table: users")
	c.Assert(result[1], qt.Contains, "-- Modify column users.email:")
	c.Assert(result[2], qt.Contains, "ALTER TABLE users ALTER COLUMN email TYPE VARCHAR(320);")
	c.Assert(result[2], qt.Contains, "ALTER TABLE users ALTER COLUMN email SET NOT NULL;")
}

func TestGenerator_GenerateMigrationSQL_DropColumn(t *testing.T) {
	c := qt.New(t)

	generator := postgresql.New()

	// Create a mock schema diff with column removals
	diff := &differtypes.SchemaDiff{
		TablesModified: []differtypes.TableDiff{
			{
				TableName:      "users",
				ColumnsRemoved: []string{"old_field", "deprecated_column"},
			},
		},
	}

	generated := &goschema.Database{}

	result := generator.GenerateMigrationSQL(diff, generated)

	// Should contain DROP COLUMN statements with warnings
	c.Assert(result, qt.HasLen, 5) // 2 warnings + 2 SQL statements
	c.Assert(result[0], qt.Contains, "-- Modify table: users")
	c.Assert(result[1], qt.Contains, "-- WARNING: Dropping column users.old_field - This will delete data!")
	c.Assert(result[2], qt.Contains, "ALTER TABLE users DROP COLUMN old_field;")
	c.Assert(result[3], qt.Contains, "-- WARNING: Dropping column users.deprecated_column - This will delete data!")
	c.Assert(result[4], qt.Contains, "ALTER TABLE users DROP COLUMN deprecated_column;")
}
