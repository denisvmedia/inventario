package generic

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/constants"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dialects/base"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

// Generator handles unknown dialects without applying dialect-specific transformations
type Generator struct {
	*base.Generator
}

// New creates a new generic generator
func New(dialectName string) *Generator {
	return &Generator{
		Generator: base.NewGenerator(dialectName),
	}
}

// GenerateCreateTable generates CREATE TABLE SQL for unknown dialects
func (g *Generator) GenerateCreateTable(table types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum) string {
	var buf strings.Builder

	// Add table comment
	buf.WriteString(g.GenerateTableComment(table.Name))

	// Start CREATE TABLE statement
	fmt.Fprintf(&buf, constants.PartialCreateTableBeginSQL, table.Name)

	var lines []string

	// Process fields
	for _, f := range fields {
		if f.StructName != table.StructName {
			continue
		}

		ftype := f.Type
		// Check for platform-specific type override
		if dialectAttrs, ok := f.Overrides[g.GetDialectName()]; ok {
			if typeOverride, ok := dialectAttrs["type"]; ok {
				ftype = typeOverride
			}
		}

		// For unknown dialects, use enum names directly without transformation
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

	// Add indexes
	buf.WriteString(g.GenerateIndexes(table, indexes))
	fmt.Fprintln(&buf)

	return buf.String()
}

// GenerateAlterStatements generates ALTER statements for unknown dialects
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
