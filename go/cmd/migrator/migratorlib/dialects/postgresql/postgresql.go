package postgresql

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/constants"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dialects/base"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

// Generator handles PostgreSQL-specific SQL generation
type Generator struct {
	*base.Generator
}

// New creates a new PostgreSQL generator
func New() *Generator {
	return &Generator{
		Generator: base.NewGenerator(constants.PlatformTypePostgres),
	}
}

// generateEnumTypes generates CREATE TYPE statements for PostgreSQL enums
func (g *Generator) generateEnumTypes(enums []types.GlobalEnum) string {
	var buf strings.Builder

	for _, en := range enums {
		fmt.Fprintf(&buf, constants.PartialPostgresCreateEnumTypeSQL, en.Name, base.JoinSeps(base.QuoteList(en.Values), constants.SepCommaSpace))
	}

	return buf.String()
}

// processFieldType processes field type for PostgreSQL, handling enums and overrides
func (g *Generator) processFieldType(field types.SchemaField, enums []types.GlobalEnum) string {
	ftype := field.Type

	// Check for platform-specific type override
	if dialectAttrs, ok := field.Overrides[constants.PlatformTypePostgres]; ok {
		if typeOverride, ok := dialectAttrs["type"]; ok {
			ftype = typeOverride
		}
	}

	// For PostgreSQL, enum types are used directly (they're defined separately)
	// No need to transform enum types - they're already correct

	return ftype
}

// GenerateCreateTable generates CREATE TABLE SQL for PostgreSQL
func (g *Generator) GenerateCreateTable(table types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum) string {
	var buf strings.Builder

	// Add table comment
	buf.WriteString(g.GenerateTableComment(table.Name))

	// Add enum type definitions first
	buf.WriteString(g.generateEnumTypes(enums))

	// Start CREATE TABLE statement
	fmt.Fprintf(&buf, constants.PartialCreateTableBeginSQL, table.Name)

	var lines []string

	// Process fields
	for _, f := range fields {
		if f.StructName != table.StructName {
			continue
		}

		ftype := g.processFieldType(f, enums)
		line := g.GenerateFieldLine(f, ftype)
		lines = append(lines, line)
	}

	// Add composite primary key if needed
	if pk := g.GeneratePrimaryKey(table); pk != "" {
		lines = append(lines, pk)
	}

	// Add foreign keys
	fks := g.GenerateForeignKeys(table, fields)
	lines = append(lines, fks...)

	// Close table definition
	fmt.Fprintln(&buf, strings.Join(lines, constants.SepCommaNL))
	fmt.Fprintln(&buf, constants.PartialClosingBracketWithSemicolonSQL)

	// PostgreSQL doesn't support table-level options like MySQL ENGINE
	// So we ignore table.Overrides for PostgreSQL

	// Add indexes
	buf.WriteString(g.GenerateIndexes(table, indexes))
	fmt.Fprintln(&buf)

	return buf.String()
}

// GenerateAlterStatements generates ALTER statements for PostgreSQL
func (g *Generator) GenerateAlterStatements(oldFields, newFields []types.SchemaField) string {
	var buf strings.Builder

	buf.WriteString(constants.PartialAlterCommentSQL)
	for _, newF := range newFields {
		found := false
		for _, oldF := range oldFields {
			if oldF.StructName == newF.StructName && oldF.Name == newF.Name {
				if oldF.Type != newF.Type {
					fmt.Fprintf(&buf, constants.PartialAlterColumnTypeSQL, newF.StructName, newF.Name, newF.Type)
				}
				if oldF.Nullable != newF.Nullable {
					if newF.Nullable {
						fmt.Fprintf(&buf, constants.PartialAlterDropNotNullSQL, newF.StructName, newF.Name)
					} else {
						fmt.Fprintf(&buf, constants.PartialAlterSetNotNullSQL, newF.StructName, newF.Name)
					}
				}
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(&buf, constants.PartialAlterAddColumnSQL, newF.StructName, newF.Name, newF.Type)
		}
	}
	fmt.Fprintln(&buf)

	return buf.String()
}
