package generic_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/generic"
)

func TestGenerator_New(t *testing.T) {
	tests := []struct {
		name        string
		dialectName string
	}{
		{"sqlite dialect", "sqlite"},
		{"custom dialect", "custom"},
		{"unknown dialect", "unknown"},
		{"empty dialect", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			generator := generic.New(tt.dialectName)
			c.Assert(generator, qt.IsNotNil)
			c.Assert(generator.GetDialectName(), qt.Equals, tt.dialectName)
		})
	}
}

func TestGenerator_GenerateCreateTable_BasicTable(t *testing.T) {
	c := qt.New(t)

	generator := generic.New("sqlite")

	table := goschema.Table{
		StructName: "User",
		Name:       "users",
		Comment:    "User accounts table",
	}

	fields := []goschema.Field{
		{
			StructName: "User",
			Name:       "id",
			Type:       "INTEGER",
			Primary:    true,
		},
		{
			StructName: "User",
			Name:       "email",
			Type:       "TEXT",
			Nullable:   false,
			Unique:     true,
		},
		{
			StructName: "User",
			Name:       "name",
			Type:       "TEXT",
			Nullable:   false,
		},
		{
			StructName: "User",
			Name:       "created_at",
			Type:       "DATETIME",
			Nullable:   false,
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "-- SQLITE TABLE: users (User accounts table) --")
	c.Assert(result, qt.Contains, "CREATE TABLE users")
	c.Assert(result, qt.Contains, "id INTEGER PRIMARY KEY")
	c.Assert(result, qt.Contains, "email TEXT NOT NULL UNIQUE")
	c.Assert(result, qt.Contains, "name TEXT NOT NULL")
	c.Assert(result, qt.Contains, "created_at DATETIME NOT NULL")
}

func TestGenerator_GenerateCreateTable_WithEnums(t *testing.T) {
	c := qt.New(t)

	generator := generic.New("sqlite")

	table := goschema.Table{
		StructName: "User",
		Name:       "users",
	}

	fields := []goschema.Field{
		{
			StructName: "User",
			Name:       "id",
			Type:       "INTEGER",
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

	// Generic dialect should use enum type names as-is (no transformation)
	c.Assert(result, qt.Contains, "status user_status_enum NOT NULL")
	c.Assert(result, qt.Contains, "role user_role_enum NOT NULL DEFAULT 'user'")
	// Should NOT contain PostgreSQL-style CREATE TYPE statements
	c.Assert(result, qt.Not(qt.Contains), "CREATE TYPE")
	// Should NOT contain MySQL-style inline ENUM definitions
	c.Assert(result, qt.Not(qt.Contains), "ENUM('active'")
}

func TestGenerator_GenerateCreateTable_WithIndexes(t *testing.T) {
	c := qt.New(t)

	generator := generic.New("sqlite")

	table := goschema.Table{
		StructName: "User",
		Name:       "users",
	}

	fields := []goschema.Field{
		{
			StructName: "User",
			Name:       "id",
			Type:       "INTEGER",
			Primary:    true,
		},
		{
			StructName: "User",
			Name:       "email",
			Type:       "TEXT",
			Nullable:   false,
		},
		{
			StructName: "User",
			Name:       "name",
			Type:       "TEXT",
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

	c.Assert(result, qt.Contains, "CREATE UNIQUE INDEX idx_users_email ON User (email);")
	c.Assert(result, qt.Contains, "CREATE INDEX idx_users_name ON User (name);")
	c.Assert(result, qt.Contains, "idx_posts_title") // Generic includes all indexes
}

func TestGenerator_GenerateCreateTable_CompositeKeys(t *testing.T) {
	c := qt.New(t)

	generator := generic.New("sqlite")

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
			Type:       "DATETIME",
			Nullable:   false,
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "user_id INTEGER NOT NULL")
	c.Assert(result, qt.Contains, "role_id INTEGER NOT NULL")
	c.Assert(result, qt.Contains, "PRIMARY KEY (user_id, role_id)")
}

func TestGenerator_GenerateAlterStatements(t *testing.T) {
	c := qt.New(t)

	generator := generic.New("sqlite")

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

	// Generic dialect should indicate that ALTER statements are not implemented
	c.Assert(result, qt.Contains, "-- ALTER statements not yet implemented with AST for generic dialect")
}

func TestGenerator_GenerateCreateTable_IgnoresDifferentStructs(t *testing.T) {
	c := qt.New(t)

	generator := generic.New("sqlite")

	table := goschema.Table{
		StructName: "User",
		Name:       "users",
	}

	fields := []goschema.Field{
		{
			StructName: "User",
			Name:       "id",
			Type:       "INTEGER",
			Primary:    true,
		},
		{
			StructName: "Post", // Different struct - should be ignored
			Name:       "title",
			Type:       "TEXT",
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "id INTEGER PRIMARY KEY")
	c.Assert(result, qt.Not(qt.Contains), "title") // Should not include Post fields
}

func TestGenerator_GenerateCreateTable_CustomDialectNames(t *testing.T) {
	tests := []struct {
		name        string
		dialectName string
		expected    string
	}{
		{
			name:        "sqlite dialect",
			dialectName: "sqlite",
			expected:    "-- SQLITE TABLE: users --",
		},
		{
			name:        "custom dialect",
			dialectName: "custom",
			expected:    "-- CUSTOM TABLE: users --",
		},
		{
			name:        "unknown dialect",
			dialectName: "unknown",
			expected:    "-- UNKNOWN TABLE: users --",
		},
		{
			name:        "cockroachdb dialect",
			dialectName: "cockroachdb",
			expected:    "-- COCKROACHDB TABLE: users --",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			generator := generic.New(tt.dialectName)

			table := goschema.Table{
				StructName: "User",
				Name:       "users",
			}

			fields := []goschema.Field{
				{
					StructName: "User",
					Name:       "id",
					Type:       "INTEGER",
					Primary:    true,
				},
			}

			result := generator.GenerateCreateTable(table, fields, nil, nil)

			c.Assert(result, qt.Contains, tt.expected)
		})
	}
}

func TestGenerator_GenerateCreateTable_NoDialectSpecificTransformations(t *testing.T) {
	c := qt.New(t)

	generator := generic.New("sqlite")

	table := goschema.Table{
		StructName: "User",
		Name:       "users",
		Engine:     "InnoDB", // Should be ignored for generic dialect
		Overrides: map[string]map[string]string{
			"mysql": {
				"engine": "MyISAM",
			},
			"postgres": {
				"tablespace": "custom",
			},
		},
	}

	fields := []goschema.Field{
		{
			StructName: "User",
			Name:       "id",
			Type:       "SERIAL", // Should remain as-is
			Primary:    true,
		},
		{
			StructName: "User",
			Name:       "data",
			Type:       "TEXT",
			Nullable:   true,
			Overrides: map[string]map[string]string{
				"mysql": {
					"type": "JSON",
				},
				"postgres": {
					"type": "JSONB",
				},
			},
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	// Generic dialect should not apply any dialect-specific transformations
	c.Assert(result, qt.Contains, "id SERIAL PRIMARY KEY")
	c.Assert(result, qt.Contains, "data TEXT")     // Should use original type, not overrides
	c.Assert(result, qt.Contains, "ENGINE=InnoDB") // Generic includes table engine
	c.Assert(result, qt.Not(qt.Contains), "JSON")  // Should not use MySQL override
	c.Assert(result, qt.Not(qt.Contains), "JSONB") // Should not use PostgreSQL override
}
