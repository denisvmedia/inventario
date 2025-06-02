//go:buildx integration

package gonative_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/convert/fromschema"
	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/core/renderer"
)

// TestGenerateCreateTableFromStubs tests SQL generation from the sample entity files in stubs directory
func TestGenerateCreateTableFromStubs(t *testing.T) {
	c := qt.New(t)

	// Get the stubs directory
	currentDir, err := os.Getwd()
	c.Assert(err, qt.IsNil)
	stubsDir := filepath.Join(currentDir, "..", "..", "stubs")

	// List all .go files in the stubs directory
	files, err := filepath.Glob(filepath.Join(stubsDir, "*.go"))
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.Not(qt.HasLen), 0, qt.Commentf("No .go files found in stubs directory (%s)", stubsDir))

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			c := qt.New(t)

			// Parse the entity file
			database := goschema.ParseFile(file)

			// Verify that we got data from the file
			c.Assert(database.Tables, qt.Not(qt.HasLen), 0, qt.Commentf("No table directives found in %s", file))
			c.Assert(database.Fields, qt.Not(qt.HasLen), 0, qt.Commentf("No fields found in %s", file))

			// For each table defined in the file, generate SQL for both Postgres and MySQL
			for _, table := range database.Tables {
				t.Run(table.Name, func(t *testing.T) {
					c := qt.New(t)

					// Generate PostgreSQL SQL using the AST approach
					createTableNode := fromschema.FromTable(table, database.Fields, database.Enums, "postgres")
					pgSQL, err := renderer.RenderSQL("postgres", createTableNode)
					c.Assert(err, qt.IsNil)

					// Verify PostgreSQL SQL contains expected elements
					c.Assert(pgSQL, qt.Contains, fmt.Sprintf("-- POSTGRES TABLE: %s --", table.Name))
					c.Assert(pgSQL, qt.Contains, fmt.Sprintf("CREATE TABLE %s (", table.Name))

					// For PostgreSQL, enums are created separately, so generate them if they exist
					for _, enum := range database.Enums {
						enumNode := fromschema.FromEnum(enum)
						enumSQL, err := renderer.RenderSQL("postgres", enumNode)
						c.Assert(err, qt.IsNil)
						c.Assert(enumSQL, qt.Contains, fmt.Sprintf("CREATE TYPE %s AS ENUM", enum.Name))
					}

					// Generate MySQL SQL
					createTableNodeMySQL := fromschema.FromTable(table, database.Fields, database.Enums, "mysql")
					mySQL, err := renderer.RenderSQL("mysql", createTableNodeMySQL)
					c.Assert(err, qt.IsNil)

					// Verify MySQL SQL contains expected elements (may include table comment from overrides)
					c.Assert(mySQL, qt.Contains, fmt.Sprintf("-- MYSQL TABLE: %s", table.Name))
					c.Assert(mySQL, qt.Contains, fmt.Sprintf("CREATE TABLE %s (", table.Name))

					// Verify enums are handled in MySQL (they may be inlined or referenced by type name)
					for _, field := range database.Fields {
						if field.StructName == table.StructName {
							// Check if field is an enum
							for _, enum := range database.Enums {
								if field.Type == enum.Name {
									// MySQL may use either inline ENUM(...) or enum type name
									hasInlineEnum := strings.Contains(mySQL, fmt.Sprintf("%s ENUM(", field.Name))
									hasEnumType := strings.Contains(mySQL, fmt.Sprintf("%s %s", field.Name, enum.Name))
									c.Assert(hasInlineEnum || hasEnumType, qt.IsTrue,
										qt.Commentf("Expected field %s to have either inline ENUM or enum type %s", field.Name, enum.Name))
								}
							}
						}
					}

					// Verify MySQL SQL doesn't contain PostgreSQL-specific syntax
					c.Assert(mySQL, qt.Not(qt.Contains), "CREATE TYPE")

					// Generate MariaDB SQL (may differ from MySQL due to platform-specific overrides)
					createTableNodeMariaDB := fromschema.FromTable(table, database.Fields, database.Enums, "mariadb")
					mariaSQL, err := renderer.RenderSQL("mariadb", createTableNodeMariaDB)
					c.Assert(err, qt.IsNil)

					// Verify MariaDB SQL has correct header
					c.Assert(mariaSQL, qt.Contains, fmt.Sprintf("-- MARIADB TABLE: %s", table.Name))

					// Verify MariaDB SQL contains the table structure (but may have different constraints due to platform overrides)
					c.Assert(mariaSQL, qt.Contains, fmt.Sprintf("CREATE TABLE %s", table.Name))
					for _, field := range database.Fields {
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
