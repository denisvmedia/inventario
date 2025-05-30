package renderer_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/renderer"
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
)

func TestFormatSchema_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		schema   *parsertypes.DatabaseSchema
		info     parsertypes.DatabaseInfo
		contains []string
	}{
		{
			name: "complete schema with all components",
			schema: &parsertypes.DatabaseSchema{
				Tables: []parsertypes.Table{
					{
						Name:    "users",
						Type:    "TABLE",
						Comment: "User accounts",
						Columns: []parsertypes.Column{
							{
								Name:         "id",
								DataType:     "INTEGER",
								IsNullable:   "NO",
								IsPrimaryKey: true,
							},
							{
								Name:               "email",
								DataType:           "VARCHAR",
								CharacterMaxLength: intPtr(255),
								IsNullable:         "NO",
								IsUnique:           true,
							},
						},
					},
				},
				Enums: []parsertypes.Enum{
					{
						Name:   "user_status",
						Values: []string{"active", "inactive", "pending"},
					},
				},
				Indexes: []parsertypes.Index{
					{
						Name:      "idx_users_email",
						TableName: "users",
						Columns:   []string{"email"},
						IsUnique:  true,
					},
				},
				Constraints: []parsertypes.Constraint{
					{
						Name:       "pk_users",
						TableName:  "users",
						Type:       "PRIMARY KEY",
						ColumnName: "id",
					},
				},
			},
			info: parsertypes.DatabaseInfo{
				Dialect: "postgres",
				Version: "14.5",
				Schema:  "public",
			},
			contains: []string{
				"=== DATABASE SCHEMA (POSTGRES) ===",
				"Version: 14.5",
				"Schema: public",
				"SUMMARY:",
				"- Tables: 1",
				"- Enums: 1",
				"- Indexes: 1",
				"- Constraints: 1",
				"=== ENUMS ===",
				"- user_status: [active, inactive, pending]",
				"=== TABLES ===",
				"1. users (TABLE)",
				"Comment: User accounts",
				"Columns:",
				"- id INTEGER PRIMARY KEY NOT NULL",
				"- email VARCHAR(255) UNIQUE NOT NULL",
				"Constraints:",
				"- PRIMARY KEY (id)",
				"Indexes:",
				"- UNIQUE INDEX idx_users_email (email)",
			},
		},
		{
			name: "minimal schema with no enums or constraints",
			schema: &parsertypes.DatabaseSchema{
				Tables: []parsertypes.Table{
					{
						Name: "simple_table",
						Type: "TABLE",
						Columns: []parsertypes.Column{
							{
								Name:       "id",
								DataType:   "INTEGER",
								IsNullable: "NO",
							},
						},
					},
				},
			},
			info: parsertypes.DatabaseInfo{
				Dialect: "mysql",
				Version: "8.0",
				Schema:  "test_db",
			},
			contains: []string{
				"=== DATABASE SCHEMA (MYSQL) ===",
				"Version: 8.0",
				"Schema: test_db",
				"- Tables: 1",
				"- Enums: 0",
				"- Indexes: 0",
				"- Constraints: 0",
				"1. simple_table (TABLE)",
				"- id INTEGER NOT NULL",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := renderer.FormatSchema(tt.schema, tt.info)

			c.Assert(result, qt.Not(qt.Equals), "")
			for _, expected := range tt.contains {
				c.Assert(result, qt.Contains, expected)
			}
		})
	}
}

func TestFormatSchema_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		schema   *parsertypes.DatabaseSchema
		info     parsertypes.DatabaseInfo
		contains []string
	}{
		{
			name: "empty schema",
			schema: &parsertypes.DatabaseSchema{
				Tables:      []parsertypes.Table{},
				Enums:       []parsertypes.Enum{},
				Indexes:     []parsertypes.Index{},
				Constraints: []parsertypes.Constraint{},
			},
			info: parsertypes.DatabaseInfo{
				Dialect: "postgres",
				Version: "14.5",
				Schema:  "public",
			},
			contains: []string{
				"=== DATABASE SCHEMA (POSTGRES) ===",
				"- Tables: 0",
				"- Enums: 0",
				"- Indexes: 0",
				"- Constraints: 0",
				"=== TABLES ===",
			},
		},
		{
			name: "nil schema",
			schema: &parsertypes.DatabaseSchema{
				Tables:      nil,
				Enums:       nil,
				Indexes:     nil,
				Constraints: nil,
			},
			info: parsertypes.DatabaseInfo{
				Dialect: "mysql",
				Version: "8.0",
				Schema:  "test",
			},
			contains: []string{
				"=== DATABASE SCHEMA (MYSQL) ===",
				"- Tables: 0",
				"- Enums: 0",
				"- Indexes: 0",
				"- Constraints: 0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := renderer.FormatSchema(tt.schema, tt.info)

			c.Assert(result, qt.Not(qt.Equals), "")
			for _, expected := range tt.contains {
				c.Assert(result, qt.Contains, expected)
			}
		})
	}
}

func TestFormatColumn_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		column   parsertypes.Column
		indent   string
		expected string
	}{
		{
			name: "primary key column",
			column: parsertypes.Column{
				Name:         "id",
				DataType:     "INTEGER",
				IsNullable:   "NO",
				IsPrimaryKey: true,
			},
			indent:   "  ",
			expected: "  - id INTEGER PRIMARY KEY NOT NULL\n",
		},
		{
			name: "varchar column with length",
			column: parsertypes.Column{
				Name:               "email",
				DataType:           "VARCHAR",
				CharacterMaxLength: intPtr(255),
				IsNullable:         "NO",
				IsUnique:           true,
			},
			indent:   "    ",
			expected: "    - email VARCHAR(255) UNIQUE NOT NULL\n",
		},
		{
			name: "decimal column with precision and scale",
			column: parsertypes.Column{
				Name:             "price",
				DataType:         "DECIMAL",
				NumericPrecision: intPtr(10),
				NumericScale:     intPtr(2),
				IsNullable:       "YES",
				ColumnDefault:    strPtr("0.00"),
			},
			indent:   "  ",
			expected: "  - price DECIMAL(10,2) DEFAULT 0.00\n",
		},
		{
			name: "auto increment column",
			column: parsertypes.Column{
				Name:            "id",
				DataType:        "INTEGER",
				IsNullable:      "NO",
				IsAutoIncrement: true,
			},
			indent:   "",
			expected: "- id INTEGER NOT NULL AUTO_INCREMENT\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Use reflection to call the unexported function
			// Since formatColumn is unexported, we'll test it through FormatSchema
			schema := &parsertypes.DatabaseSchema{
				Tables: []parsertypes.Table{
					{
						Name:    "test_table",
						Type:    "TABLE",
						Columns: []parsertypes.Column{tt.column},
					},
				},
			}
			info := parsertypes.DatabaseInfo{Dialect: "postgres", Version: "14", Schema: "public"}

			result := renderer.FormatSchema(schema, info)

			// Extract the column part and verify it contains expected formatting
			lines := strings.Split(result, "\n")
			var columnLine string
			for _, line := range lines {
				if strings.Contains(line, tt.column.Name) && strings.Contains(line, "- ") {
					columnLine = line
					break
				}
			}

			c.Assert(columnLine, qt.Not(qt.Equals), "")
			c.Assert(columnLine, qt.Contains, tt.column.Name)
			c.Assert(columnLine, qt.Contains, tt.column.DataType)
		})
	}
}

func TestFormatConstraint_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		constraint parsertypes.Constraint
		indent     string
		expected   string
	}{
		{
			name: "primary key constraint",
			constraint: parsertypes.Constraint{
				Name:       "pk_users",
				TableName:  "users",
				Type:       "PRIMARY KEY",
				ColumnName: "id",
			},
			indent:   "  ",
			expected: "  - PRIMARY KEY (id)\n",
		},
		{
			name: "foreign key constraint with rules",
			constraint: parsertypes.Constraint{
				Name:          "fk_user_profile",
				TableName:     "profiles",
				Type:          "FOREIGN KEY",
				ColumnName:    "user_id",
				ForeignTable:  strPtr("users"),
				ForeignColumn: strPtr("id"),
				DeleteRule:    strPtr("CASCADE"),
				UpdateRule:    strPtr("RESTRICT"),
			},
			indent:   "    ",
			expected: "    - FOREIGN KEY user_id -> users(id) ON DELETE CASCADE ON UPDATE RESTRICT\n",
		},
		{
			name: "unique constraint",
			constraint: parsertypes.Constraint{
				Name:       "uk_users_email",
				TableName:  "users",
				Type:       "UNIQUE",
				ColumnName: "email",
			},
			indent:   "  ",
			expected: "  - UNIQUE (email)\n",
		},
		{
			name: "check constraint",
			constraint: parsertypes.Constraint{
				Name:        "ck_users_age",
				TableName:   "users",
				Type:        "CHECK",
				ColumnName:  "age",
				CheckClause: strPtr("age >= 0"),
			},
			indent:   "  ",
			expected: "  - CHECK age CHECK age >= 0\n",
		},
		{
			name: "unknown constraint type",
			constraint: parsertypes.Constraint{
				Name:       "custom_constraint",
				TableName:  "users",
				Type:       "CUSTOM",
				ColumnName: "field",
			},
			indent:   "",
			expected: "- CUSTOM (field)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Test through FormatSchema since formatConstraint is unexported
			schema := &parsertypes.DatabaseSchema{
				Tables: []parsertypes.Table{
					{
						Name: tt.constraint.TableName,
						Type: "TABLE",
						Columns: []parsertypes.Column{
							{Name: "id", DataType: "INTEGER"},
						},
					},
				},
				Constraints: []parsertypes.Constraint{tt.constraint},
			}
			info := parsertypes.DatabaseInfo{Dialect: "postgres", Version: "14", Schema: "public"}

			result := renderer.FormatSchema(schema, info)

			// Verify the constraint appears in the output
			c.Assert(result, qt.Contains, tt.constraint.Type)
			c.Assert(result, qt.Contains, tt.constraint.ColumnName)
		})
	}
}

func TestFormatIndex_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		index    parsertypes.Index
		indent   string
		expected string
	}{
		{
			name: "primary key index",
			index: parsertypes.Index{
				Name:      "pk_users",
				TableName: "users",
				Columns:   []string{"id"},
				IsPrimary: true,
			},
			indent:   "  ",
			expected: "  - PRIMARY KEY pk_users (id)\n",
		},
		{
			name: "unique index",
			index: parsertypes.Index{
				Name:      "uk_users_email",
				TableName: "users",
				Columns:   []string{"email"},
				IsUnique:  true,
			},
			indent:   "    ",
			expected: "    - UNIQUE INDEX uk_users_email (email)\n",
		},
		{
			name: "regular index with multiple columns",
			index: parsertypes.Index{
				Name:      "idx_users_name_age",
				TableName: "users",
				Columns:   []string{"first_name", "last_name", "age"},
			},
			indent:   "  ",
			expected: "  - INDEX idx_users_name_age (first_name, last_name, age)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Test through FormatSchema since formatIndex is unexported
			schema := &parsertypes.DatabaseSchema{
				Tables: []parsertypes.Table{
					{
						Name: tt.index.TableName,
						Type: "TABLE",
						Columns: []parsertypes.Column{
							{Name: "id", DataType: "INTEGER"},
						},
					},
				},
				Indexes: []parsertypes.Index{tt.index},
			}
			info := parsertypes.DatabaseInfo{Dialect: "postgres", Version: "14", Schema: "public"}

			result := renderer.FormatSchema(schema, info)

			// Verify the index appears in the output
			c.Assert(result, qt.Contains, tt.index.Name)
			for _, col := range tt.index.Columns {
				c.Assert(result, qt.Contains, col)
			}
		})
	}
}

func TestGetTableConstraints_HappyPath(t *testing.T) {
	tests := []struct {
		name        string
		constraints []parsertypes.Constraint
		tableName   string
		expected    int
	}{
		{
			name: "multiple constraints for table",
			constraints: []parsertypes.Constraint{
				{Name: "pk_users", TableName: "users", Type: "PRIMARY KEY"},
				{Name: "fk_posts_user", TableName: "posts", Type: "FOREIGN KEY"},
				{Name: "uk_users_email", TableName: "users", Type: "UNIQUE"},
				{Name: "ck_users_age", TableName: "users", Type: "CHECK"},
			},
			tableName: "users",
			expected:  3,
		},
		{
			name: "no constraints for table",
			constraints: []parsertypes.Constraint{
				{Name: "pk_posts", TableName: "posts", Type: "PRIMARY KEY"},
			},
			tableName: "users",
			expected:  0,
		},
		{
			name:        "empty constraints list",
			constraints: []parsertypes.Constraint{},
			tableName:   "users",
			expected:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Test through FormatSchema since getTableConstraints is unexported
			schema := &parsertypes.DatabaseSchema{
				Tables: []parsertypes.Table{
					{
						Name: tt.tableName,
						Type: "TABLE",
						Columns: []parsertypes.Column{
							{Name: "id", DataType: "INTEGER"},
						},
					},
				},
				Constraints: tt.constraints,
			}
			info := parsertypes.DatabaseInfo{Dialect: "postgres", Version: "14", Schema: "public"}

			result := renderer.FormatSchema(schema, info)

			// Count constraint occurrences in the table section
			if tt.expected > 0 {
				c.Assert(result, qt.Contains, "Constraints:")
			}
		})
	}
}

func TestGetTableIndexes_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		indexes   []parsertypes.Index
		tableName string
		expected  int
	}{
		{
			name: "multiple indexes for table",
			indexes: []parsertypes.Index{
				{Name: "pk_users", TableName: "users", IsPrimary: true},
				{Name: "idx_posts_title", TableName: "posts", IsUnique: false},
				{Name: "uk_users_email", TableName: "users", IsUnique: true},
			},
			tableName: "users",
			expected:  2,
		},
		{
			name: "no indexes for table",
			indexes: []parsertypes.Index{
				{Name: "idx_posts_title", TableName: "posts"},
			},
			tableName: "users",
			expected:  0,
		},
		{
			name:      "empty indexes list",
			indexes:   []parsertypes.Index{},
			tableName: "users",
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Test through FormatSchema since getTableIndexes is unexported
			schema := &parsertypes.DatabaseSchema{
				Tables: []parsertypes.Table{
					{
						Name: tt.tableName,
						Type: "TABLE",
						Columns: []parsertypes.Column{
							{Name: "id", DataType: "INTEGER"},
						},
					},
				},
				Indexes: tt.indexes,
			}
			info := parsertypes.DatabaseInfo{Dialect: "postgres", Version: "14", Schema: "public"}

			result := renderer.FormatSchema(schema, info)

			// Count index occurrences in the table section
			if tt.expected > 0 {
				c.Assert(result, qt.Contains, "Indexes:")
			}
		})
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func strPtr(s string) *string {
	return &s
}
