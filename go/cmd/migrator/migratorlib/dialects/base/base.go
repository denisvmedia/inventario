package base

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/constants"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

// Generator provides common functionality for all dialect generators
type Generator struct {
	dialectName string
}

// NewGenerator creates a new base generator
func NewGenerator(dialectName string) *Generator {
	return &Generator{dialectName: dialectName}
}

// GetDialectName returns the name of the dialect
func (g *Generator) GetDialectName() string {
	return g.dialectName
}

// GenerateTableComment generates the table comment SQL
func (g *Generator) GenerateTableComment(tableName string) string {
	return fmt.Sprintf(constants.PartialTableCommentSQL, strings.ToUpper(g.dialectName), tableName)
}

// GenerateFieldLine generates a single field definition line
func (g *Generator) GenerateFieldLine(field types.SchemaField, fieldType string) string {
	nameAndType := fmt.Sprintf("  %s %s", field.Name, fieldType)
	line := nameAndType

	if field.Primary {
		line += constants.PartialPrimaryKeySQL
	} else {
		if field.Unique {
			line += constants.PartialUniqueSQL
		}
		if !field.Nullable {
			line += constants.PartialNotNull
		}
	}

	if field.Default != "" {
		line += fmt.Sprintf(constants.PartialDefaultSQL, field.Default)
	}
	if field.DefaultFn != "" {
		line += fmt.Sprintf(constants.PartialDefaultFnSQL, field.DefaultFn)
	}
	if field.Check != "" {
		line += fmt.Sprintf(constants.PartialCheckSQL, field.Check)
	}

	return line
}

// GenerateIndexes generates index creation SQL
func (g *Generator) GenerateIndexes(table types.TableDirective, indexes []types.SchemaIndex) string {
	var buf strings.Builder

	for _, idx := range indexes {
		if idx.StructName != table.StructName {
			continue
		}
		idxSQL := constants.PartialIndexCreateSQL
		if idx.Unique {
			idxSQL += constants.PartialIndexUniqueSQL
		}
		idxSQL += fmt.Sprintf(constants.PartialIndexBodySQL, idx.Name, table.Name, strings.Join(idx.Fields, constants.SepCommaSpace))
		fmt.Fprintln(&buf, idxSQL)
	}

	return buf.String()
}

// GenerateForeignKeys generates foreign key constraints
func (g *Generator) GenerateForeignKeys(table types.TableDirective, fields []types.SchemaField) []string {
	var fks []string

	for _, f := range fields {
		if f.StructName != table.StructName || f.Foreign == "" {
			continue
		}
		fk := fmt.Sprintf(constants.PartialConstraintForeignKeySQL, f.ForeignKeyName, f.Name, f.Foreign)
		fks = append(fks, fk)
	}

	return fks
}

// GeneratePrimaryKey generates primary key constraint for composite keys
func (g *Generator) GeneratePrimaryKey(table types.TableDirective) string {
	if len(table.PrimaryKey) > 1 {
		return fmt.Sprintf(constants.PartialPrimaryKeyValueSQL, strings.Join(table.PrimaryKey, constants.SepCommaSpace))
	}
	return ""
}

// QuoteList quotes a list of strings for SQL
func QuoteList(list []string) []string {
	res := make([]string, len(list))
	for i, v := range list {
		// Replace single quotes with doubled single quotes for SQL
		escaped := strings.ReplaceAll(v, "'", "''")
		res[i] = fmt.Sprintf("'%s'", escaped)
	}
	return res
}

// JoinSeps joins strings with separators (helper function)
func JoinSeps(list []string, sep string) string {
	return strings.Join(list, sep)
}
