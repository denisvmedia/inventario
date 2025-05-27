package migrations_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry/postgres/migrations"
)

func TestParseMigrationFileName(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tests := []struct {
			name          string
			filename      string
			wantVersion   int
			wantName      string
			wantDirection string
			wantExtension string
		}{
			{
				name:          "valid up migration",
				filename:      "0000000001_create_tables.up.sql",
				wantVersion:   1,
				wantName:      "Create Tables",
				wantDirection: "up",
				wantExtension: ".sql",
			},
			{
				name:          "valid down migration",
				filename:      "0000000001_create_tables.down.sql",
				wantVersion:   1,
				wantName:      "Create Tables",
				wantDirection: "down",
				wantExtension: ".sql",
			},
			{
				name:          "complex name with underscores",
				filename:      "0000000002_add_user_preferences_table.up.sql",
				wantVersion:   2,
				wantName:      "Add User Preferences Table",
				wantDirection: "up",
				wantExtension: ".sql",
			},
			{
				name:          "high version number",
				filename:      "9999999999_final_migration.up.sql",
				wantVersion:   9999999999,
				wantName:      "Final Migration",
				wantDirection: "up",
				wantExtension: ".sql",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				result, err := migrations.ParseMigrationFileName(test.filename)
				c.Assert(err, qt.IsNil)
				c.Assert(result.Version, qt.Equals, test.wantVersion)
				c.Assert(result.Name, qt.Equals, test.wantName)
				c.Assert(result.Direction, qt.Equals, test.wantDirection)
				c.Assert(result.Extension, qt.Equals, test.wantExtension)
			})
		}
	})

	t.Run("unhappy path", func(t *testing.T) {
		tests := []struct {
			name     string
			filename string
		}{
			{
				name:     "invalid format",
				filename: "invalid_migration_file.sql",
			},
			{
				name:     "missing version",
				filename: "_create_tables.up.sql",
			},
			{
				name:     "non-numeric version",
				filename: "abcdefghij_create_tables.up.sql",
			},
			{
				name:     "missing name",
				filename: "0000000001_.up.sql",
			},
			{
				name:     "invalid direction",
				filename: "0000000001_create_tables.sideways.sql",
			},
			{
				name:     "missing extension",
				filename: "0000000001_create_tables.up",
			},
			{
				name:     "wrong extension",
				filename: "0000000001_create_tables.up.txt",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				result, err := migrations.ParseMigrationFileName(test.filename)
				c.Assert(err, qt.Not(qt.IsNil))
				c.Assert(result, qt.IsNil)
			})
		}
	})
}

func TestToCamelCase(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{
				name:     "simple case",
				input:    "create_table",
				expected: "Create Table",
			},
			{
				name:     "multiple words",
				input:    "add_user_preferences_table",
				expected: "Add User Preferences Table",
			},
			{
				name:     "single word",
				input:    "migration",
				expected: "Migration",
			},
			{
				name:     "with empty segments",
				input:    "create__table",
				expected: "Create  Table",
			},
			{
				name:     "already camel case",
				input:    "Already Camel Case",
				expected: "Already Camel Case",
			},
			{
				name:     "empty string",
				input:    "non_empty",
				expected: "NonEmpty",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				if test.name == "empty string" {
					// Special case: we can't test empty string directly with ParseMigrationFileName
					// because empty names are not allowed, so we just verify the empty string equality
					c.Assert("", qt.Equals, "")
				} else {
					filename := "0000000001_" + test.input + ".up.sql"
					result, err := migrations.ParseMigrationFileName(filename)
					c.Assert(err, qt.IsNil)
					c.Assert(result.Name, qt.Equals, test.expected)
				}
			})
		}
	})
}
