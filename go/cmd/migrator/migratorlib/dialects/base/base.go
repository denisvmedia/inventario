package base

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/ast"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/builders"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

// Generator provides common functionality for all dialect generators using AST-based approach
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

// GenerateTableComment generates a table comment AST node
func (g *Generator) GenerateTableComment(tableName string) *ast.CommentNode {
	commentText := fmt.Sprintf("-- %s TABLE: %s --", strings.ToUpper(g.dialectName), tableName)
	return ast.NewComment(commentText)
}

// GenerateColumn converts a SchemaField to a ColumnNode using AST builders
func (g *Generator) GenerateColumn(field types.SchemaField, fieldType string, enums []types.GlobalEnum) *ast.ColumnNode {
	return builders.FromSchemaField(field, enums)
}

// GenerateCreateTable converts table directive and fields to a CreateTableNode
func (g *Generator) GenerateCreateTable(table types.TableDirective, fields []types.SchemaField, enums []types.GlobalEnum) *ast.CreateTableNode {
	return builders.FromTableDirective(table, fields, enums)
}

// GenerateIndexes generates index AST nodes for a table
func (g *Generator) GenerateIndexes(table types.TableDirective, indexes []types.SchemaIndex) []*ast.IndexNode {
	var indexNodes []*ast.IndexNode

	for _, idx := range indexes {
		if idx.StructName != table.StructName {
			continue
		}
		indexNode := builders.FromSchemaIndex(idx)
		indexNodes = append(indexNodes, indexNode)
	}

	return indexNodes
}

// GenerateForeignKeyConstraints generates foreign key constraint nodes
func (g *Generator) GenerateForeignKeyConstraints(table types.TableDirective, fields []types.SchemaField) []*ast.ConstraintNode {
	var constraints []*ast.ConstraintNode

	for _, f := range fields {
		if f.StructName != table.StructName || f.Foreign == "" {
			continue
		}

		// Parse foreign key reference (assuming format "table(column)" or just "table")
		refTable, refColumn := g.parseForeignKeyReference(f.Foreign)
		
		ref := &ast.ForeignKeyRef{
			Table:  refTable,
			Column: refColumn,
			Name:   f.ForeignKeyName,
		}

		constraint := ast.NewForeignKeyConstraint(f.ForeignKeyName, []string{f.Name}, ref)
		constraints = append(constraints, constraint)
	}

	return constraints
}

// GeneratePrimaryKeyConstraint generates primary key constraint for composite keys
func (g *Generator) GeneratePrimaryKeyConstraint(table types.TableDirective) *ast.ConstraintNode {
	if len(table.PrimaryKey) > 1 {
		return ast.NewPrimaryKeyConstraint(table.PrimaryKey...)
	}
	return nil
}

// GenerateSchema generates a complete schema using the fluent API
func (g *Generator) GenerateSchema(tables []types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum) *ast.StatementList {
	schema := builders.NewSchema()

	// Add comment for the schema
	schema.Comment(fmt.Sprintf("%s Database Schema", strings.ToUpper(g.dialectName)))

	// Add global enums first (for PostgreSQL)
	for _, enum := range enums {
		schema.Enum(enum.Name, enum.Values...)
	}

	// Add tables
	for _, table := range tables {
		tableBuilder := schema.Table(table.Name)

		if table.Comment != "" {
			tableBuilder.Comment(table.Comment)
		}

		if table.Engine != "" {
			tableBuilder.Engine(table.Engine)
		}

		// Add columns
		for _, field := range fields {
			if field.StructName == table.StructName {
				columnBuilder := tableBuilder.Column(field.Name, field.Type)

				if !field.Nullable {
					columnBuilder.NotNull()
				}

				if field.Primary {
					columnBuilder.Primary()
				}

				if field.Unique {
					columnBuilder.Unique()
				}

				if field.AutoInc {
					columnBuilder.AutoIncrement()
				}

				if field.Default != "" {
					columnBuilder.Default(field.Default)
				}

				if field.DefaultFn != "" {
					columnBuilder.DefaultFunction(field.DefaultFn)
				}

				if field.Check != "" {
					columnBuilder.Check(field.Check)
				}

				if field.Comment != "" {
					columnBuilder.Comment(field.Comment)
				}

				if field.Foreign != "" {
					refTable, refColumn := g.parseForeignKeyReference(field.Foreign)
					columnBuilder.ForeignKey(refTable, refColumn, field.ForeignKeyName).End()
				} else {
					columnBuilder.End()
				}
			}
		}

		// Add composite primary key if specified
		if len(table.PrimaryKey) > 1 {
			tableBuilder.PrimaryKey(table.PrimaryKey...)
		}

		tableBuilder.End()
	}

	// Add indexes
	for _, idx := range indexes {
		indexBuilder := schema.Index(idx.Name, idx.StructName, idx.Fields...)
		
		if idx.Unique {
			indexBuilder.Unique()
		}

		if idx.Comment != "" {
			indexBuilder.Comment(idx.Comment)
		}

		indexBuilder.End()
	}

	return schema.Build()
}

// parseForeignKeyReference parses foreign key reference string
// Supports formats: "table(column)" or just "table"
func (g *Generator) parseForeignKeyReference(foreign string) (table, column string) {
	if strings.Contains(foreign, "(") {
		// Format: "table(column)"
		parts := strings.Split(foreign, "(")
		table = strings.TrimSpace(parts[0])
		column = strings.TrimSpace(strings.TrimSuffix(parts[1], ")"))
	} else {
		// Format: "table" - assume primary key column
		table = strings.TrimSpace(foreign)
		column = "id" // Default assumption
	}
	return table, column
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
