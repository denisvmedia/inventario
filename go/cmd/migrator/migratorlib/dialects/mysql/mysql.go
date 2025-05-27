package mysql

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/constants"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dialects/base"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

// Generator handles MySQL-specific SQL generation
type Generator struct {
	*base.Generator
}

// New creates a new MySQL generator
func New() *Generator {
	return &Generator{
		Generator: base.NewGenerator(constants.PlatformTypeMySQL),
	}
}

// processFieldType processes field type for MySQL, handling enums appropriately
func (g *Generator) processFieldType(field types.SchemaField, enums []types.GlobalEnum) string {
	ftype := field.Type

	// Check for platform-specific type override
	if dialectAttrs, ok := field.Overrides[constants.PlatformTypeMySQL]; ok {
		if typeOverride, ok := dialectAttrs["type"]; ok {
			ftype = typeOverride
		}
	}

	// Check if the field type is an enum and transform it for MySQL
	for _, en := range enums {
		if ftype == en.Name {
			// MySQL defines enum inline
			if len(en.Values) > 0 {
				return fmt.Sprintf(constants.PartialMariaDBMysqlEnumSQL, strings.Join(base.QuoteList(en.Values), constants.SepCommaSpace))
			}
			break
		}
	}

	return ftype
}

// generateTableOptions generates MySQL-specific table options
func (g *Generator) generateTableOptions(table types.TableDirective) string {
	dialectAttrs, ok := table.Overrides[constants.PlatformTypeMySQL]
	if !ok {
		return ""
	}

	var options []string

	// Handle ENGINE option
	if engine, ok := dialectAttrs["engine"]; ok {
		options = append(options, fmt.Sprintf("ENGINE=%s", engine))
	}

	// Handle COMMENT option
	if comment, ok := dialectAttrs["comment"]; ok {
		options = append(options, fmt.Sprintf("COMMENT='%s'", comment))
	}

	// Add any other platform-specific options
	for k, v := range dialectAttrs {
		if k != "engine" && k != "comment" && k != "type" {
			options = append(options, fmt.Sprintf("%s=%s", k, v))
		}
	}

	if len(options) > 0 {
		return constants.SepSpace + strings.Join(options, constants.SepSpace)
	}

	return ""
}

// GenerateCreateTable generates CREATE TABLE SQL for MySQL
func (g *Generator) GenerateCreateTable(table types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum) string {
	var buf strings.Builder

	// Add table comment
	buf.WriteString(g.GenerateTableComment(table.Name))

	// MySQL doesn't need separate enum type definitions - they're inline

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

	// Close table definition with MySQL-specific options
	fmt.Fprintln(&buf, strings.Join(lines, constants.SepCommaNL))

	// Add MySQL-specific table options
	tableOptions := g.generateTableOptions(table)
	if tableOptions != "" {
		fmt.Fprint(&buf, constants.PartialClosingBracketSQL)
		fmt.Fprint(&buf, tableOptions)
		fmt.Fprintln(&buf, constants.SepSemicolon)
	} else {
		fmt.Fprintln(&buf, constants.PartialClosingBracketWithSemicolonSQL)
	}

	// Add indexes
	buf.WriteString(g.GenerateIndexes(table, indexes))
	fmt.Fprintln(&buf)

	return buf.String()
}

// GenerateAlterStatements generates ALTER statements for MySQL
func (g *Generator) GenerateAlterStatements(oldFields, newFields []types.SchemaField) string {
	var buf strings.Builder

	buf.WriteString(constants.PartialAlterCommentSQL)
	for _, newF := range newFields {
		found := false
		for _, oldF := range oldFields {
			if oldF.StructName == newF.StructName && oldF.Name == newF.Name {
				if oldF.Type != newF.Type {
					// MySQL uses MODIFY COLUMN instead of ALTER COLUMN TYPE
					fmt.Fprintf(&buf, "ALTER TABLE %s MODIFY COLUMN %s %s;\n", newF.StructName, newF.Name, newF.Type)
				}
				if oldF.Nullable != newF.Nullable {
					if newF.Nullable {
						// MySQL doesn't have a direct DROP NOT NULL, need to MODIFY the column
						fmt.Fprintf(&buf, "ALTER TABLE %s MODIFY COLUMN %s %s NULL;\n", newF.StructName, newF.Name, newF.Type)
					} else {
						fmt.Fprintf(&buf, "ALTER TABLE %s MODIFY COLUMN %s %s NOT NULL;\n", newF.StructName, newF.Name, newF.Type)
					}
				}
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(&buf, "ALTER TABLE %s ADD COLUMN %s %s;\n", newF.StructName, newF.Name, newF.Type)
		}
	}
	fmt.Fprintln(&buf)

	return buf.String()
}
