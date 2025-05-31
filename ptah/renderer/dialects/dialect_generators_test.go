package dialects_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/platform"
	"github.com/denisvmedia/inventario/ptah/renderer/generators"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

func TestDialectGenerators_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		dialect  string
		contains []string
	}{
		{
			name:    "postgresql generator creates enum types",
			dialect: "postgres",
			contains: []string{
				"-- POSTGRES TABLE: users --",
				"CREATE TYPE user_status_enum AS ENUM ('active', 'inactive', 'pending');",
				"status user_status_enum NOT NULL",
			},
		},
		{
			name:    "mysql generator creates inline enums",
			dialect: "mysql",
			contains: []string{
				"-- MYSQL TABLE: users (User accounts) --",
				"status ENUM('active', 'inactive', 'pending') NOT NULL",
				"ENGINE=InnoDB",
			},
		},
		{
			name:    "mariadb generator creates inline enums",
			dialect: "mariadb",
			contains: []string{
				"-- MARIADB TABLE: users (User accounts) --",
				"status ENUM('active', 'inactive', 'pending') NOT NULL",
				"ENGINE=InnoDB",
			},
		},
		{
			name:    "unknown dialect uses generic generator",
			dialect: "sqlite",
			contains: []string{
				"-- SQLITE TABLE: users --",
				"status user_status_enum NOT NULL",
			},
		},
	}

	// Common test data
	table := types.TableDirective{
		StructName: "User",
		Name:       "users",
		Overrides: map[string]map[string]string{
			"mysql": {
				"engine":  "InnoDB",
				"comment": "User accounts",
			},
			"mariadb": {
				"engine":  "InnoDB",
				"comment": "User accounts",
			},
		},
	}

	fields := []types.SchemaField{
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
	}

	enums := []types.GlobalEnum{
		{
			Name:   "user_status_enum",
			Values: []string{"active", "inactive", "pending"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			generator := generators.GetDialectGenerator(tt.dialect)
			result := generator.GenerateCreateTable(table, fields, nil, enums)

			for _, expected := range tt.contains {
				c.Assert(result, qt.Contains, expected, qt.Commentf("Expected to contain: %s", expected))
			}
		})
	}
}

func TestDialectGenerators_AlterStatements(t *testing.T) {
	tests := []struct {
		name     string
		dialect  string
		contains []string
	}{
		{
			name:    "postgresql alter statements",
			dialect: "postgres",
			contains: []string{
				"ALTER TABLE User ALTER COLUMN email TYPE VARCHAR(255);",
				"ALTER TABLE User ADD COLUMN name TEXT NOT NULL;",
			},
		},
		{
			name:    "mysql alter statements use MODIFY",
			dialect: "mysql",
			contains: []string{
				"ALTER TABLE User MODIFY COLUMN email VARCHAR(255) NOT NULL;",
				"ALTER TABLE User ADD COLUMN name TEXT NOT NULL;",
			},
		},
		{
			name:    "mariadb alter statements use MODIFY",
			dialect: "mariadb",
			contains: []string{
				"ALTER TABLE User MODIFY COLUMN email VARCHAR(255) NOT NULL;",
				"ALTER TABLE User ADD COLUMN name TEXT NOT NULL;",
			},
		},
	}

	oldFields := []types.SchemaField{
		{
			StructName: "User",
			Name:       "email",
			Type:       "TEXT",
		},
	}

	newFields := []types.SchemaField{
		{
			StructName: "User",
			Name:       "email",
			Type:       "VARCHAR(255)",
		},
		{
			StructName: "User",
			Name:       "name",
			Type:       "TEXT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			generator := generators.GetDialectGenerator(tt.dialect)
			result := generator.GenerateAlterStatements(oldFields, newFields)

			for _, expected := range tt.contains {
				c.Assert(result, qt.Contains, expected, qt.Commentf("Expected to contain: %s", expected))
			}
		})
	}
}

func TestDialectGenerators_GetDialectName(t *testing.T) {
	tests := []struct {
		dialect      string
		expectedName string
	}{
		{"postgres", "postgres"},
		{"mysql", "mysql"},
		{"mariadb", "mariadb"},
		{"sqlite", "sqlite"},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			c := qt.New(t)

			generator := generators.GetDialectGenerator(tt.dialect)
			c.Assert(generator.GetDialectName(), qt.Equals, tt.expectedName)
		})
	}
}

func TestDialectGenerators_BackwardCompatibility(t *testing.T) {
	// Test that the old API still works exactly the same
	table := types.TableDirective{
		StructName: "User",
		Name:       "users",
	}

	fields := []types.SchemaField{
		{
			StructName: "User",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
		},
	}

	// Test GenerateCreateTable backward compatibility
	result := generators.GenerateCreateTable(table, fields, nil, nil, "postgres")
	c := qt.New(t)
	c.Assert(result, qt.Contains, "-- POSTGRES TABLE: users --")
	c.Assert(result, qt.Contains, "id SERIAL PRIMARY KEY")

	// Test GenerateAlterStatements backward compatibility
	oldFields := []types.SchemaField{}
	newFields := []types.SchemaField{
		{
			StructName: "User",
			Name:       "email",
			Type:       "VARCHAR(255)",
		},
	}

	alterResult := generators.GenerateAlterStatements(oldFields, newFields, platform.Postgres)
	c.Assert(alterResult, qt.Contains, "ALTER TABLE User ADD COLUMN email VARCHAR(255) NOT NULL;")
}
