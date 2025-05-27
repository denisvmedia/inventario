package migratorlib_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib"
)

// TestGenerateCreateTableFromStubs tests SQL generation from the sample entity files in stubs directory
func TestGenerateCreateTableFromStubs(t *testing.T) {
	c := qt.New(t)

	// Get the stubs directory
	currentDir, err := os.Getwd()
	c.Assert(err, qt.IsNil)
	stubsDir := filepath.Join(currentDir, "stubs")

	// List all .go files in the stubs directory
	files, err := filepath.Glob(filepath.Join(stubsDir, "*.go"))
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.Not(qt.HasLen), 0, qt.Commentf("No .go files found in stubs directory"))

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			c := qt.New(t)

			// Parse the entity file
			_, fields, indexes, tables, enums := migratorlib.ParseFile(file)

			// Verify that we got data from the file
			c.Assert(tables, qt.Not(qt.HasLen), 0, qt.Commentf("No table directives found in %s", file))
			c.Assert(fields, qt.Not(qt.HasLen), 0, qt.Commentf("No fields found in %s", file))

			// For each table defined in the file, generate SQL for both Postgres and MySQL
			for _, table := range tables {
				t.Run(table.Name, func(t *testing.T) {
					c := qt.New(t)

					// Generate PostgreSQL SQL
					pgSQL := migratorlib.GenerateCreateTable(table, fields, indexes, enums, "postgres")

					// Verify PostgreSQL SQL contains expected elements
					c.Assert(pgSQL, qt.Contains, fmt.Sprintf("-- POSTGRES TABLE: %s --", table.Name))
					c.Assert(pgSQL, qt.Contains, fmt.Sprintf("CREATE TABLE %s (", table.Name))

					// Check if we have enums and verify they're defined correctly in PostgreSQL
					for _, enum := range enums {
						c.Assert(pgSQL, qt.Contains, fmt.Sprintf("CREATE TYPE %s AS ENUM", enum.Name))
					}

					// Generate MySQL SQL
					mySQL := migratorlib.GenerateCreateTable(table, fields, indexes, enums, "mysql")

					// Verify MySQL SQL contains expected elements (may include table comment from overrides)
					c.Assert(mySQL, qt.Contains, fmt.Sprintf("-- MYSQL TABLE: %s", table.Name))
					c.Assert(mySQL, qt.Contains, fmt.Sprintf("CREATE TABLE %s (", table.Name))

					// Verify enums are inlined in MySQL
					for _, field := range fields {
						if field.StructName == table.StructName {
							// Check if field is an enum
							for _, enum := range enums {
								if field.Type == enum.Name {
									c.Assert(mySQL, qt.Contains, fmt.Sprintf("%s ENUM(", field.Name))
								}
							}
						}
					}

					// Verify MySQL SQL doesn't contain PostgreSQL-specific syntax
					c.Assert(mySQL, qt.Not(qt.Contains), "CREATE TYPE")

					// Generate MariaDB SQL (may differ from MySQL due to platform-specific overrides)
					mariaSQL := migratorlib.GenerateCreateTable(table, fields, indexes, enums, "mariadb")

					// Verify MariaDB SQL has correct header
					c.Assert(mariaSQL, qt.Contains, fmt.Sprintf("-- MARIADB TABLE: %s", table.Name))

					// Verify MariaDB SQL contains the table structure (but may have different constraints due to platform overrides)
					c.Assert(mariaSQL, qt.Contains, fmt.Sprintf("CREATE TABLE %s", table.Name))
					for _, field := range fields {
						if field.StructName == table.StructName {
							// Verify field is present (but don't check exact constraint syntax as it may differ)
							c.Assert(mariaSQL, qt.Contains, field.Name)
						}
					}
				})
			}
		})
	}
}
