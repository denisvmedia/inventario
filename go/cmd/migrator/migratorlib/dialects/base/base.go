package base

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
	"github.com/denisvmedia/inventario/ptah/schema/builder"
	"github.com/denisvmedia/inventario/ptah/schema/meta"
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
func (g *Generator) GenerateColumn(field meta.SchemaField, fieldType string, enums []meta.GlobalEnum) *ast.ColumnNode {
	return builder.FromSchemaField(field, enums)
}

// GenerateCreateTable converts table directive and fields to a CreateTableNode
func (g *Generator) GenerateCreateTable(table meta.TableDirective, fields []meta.SchemaField, enums []meta.GlobalEnum) *ast.CreateTableNode {
	return builder.FromTableDirective(table, fields, enums)
}

// GenerateIndexes generates index AST nodes for a table
func (g *Generator) GenerateIndexes(table meta.TableDirective, indexes []meta.SchemaIndex) []*ast.IndexNode {
	var indexNodes []*ast.IndexNode

	for _, idx := range indexes {
		if idx.StructName != table.StructName {
			continue
		}
		indexNode := builder.FromSchemaIndex(idx)
		indexNodes = append(indexNodes, indexNode)
	}

	return indexNodes
}

// GenerateForeignKeyConstraints generates foreign key constraint nodes
func (g *Generator) GenerateForeignKeyConstraints(table meta.TableDirective, fields []meta.SchemaField) []*ast.ConstraintNode {
	var constraints []*ast.ConstraintNode

	for _, f := range fields {
		if f.StructName != table.StructName || f.Foreign == "" {
			continue
		}

		// Parse foreign key reference (assuming format "table(column)" or just "table")
		refTable, refColumn := g.ParseForeignKeyReference(f.Foreign)

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
func (g *Generator) GeneratePrimaryKeyConstraint(table meta.TableDirective) *ast.ConstraintNode {
	if len(table.PrimaryKey) > 1 {
		return ast.NewPrimaryKeyConstraint(table.PrimaryKey...)
	}
	return nil
}

// GenerateSchema generates a complete schema using the fluent API
func (g *Generator) GenerateSchema(tables []meta.TableDirective, fields []meta.SchemaField, indexes []meta.SchemaIndex, enums []meta.GlobalEnum) *ast.StatementList {
	return g.GenerateSchemaWithEmbedded(tables, fields, indexes, enums, nil)
}

// GenerateSchemaWithEmbedded generates a complete schema with embedded field support
func (g *Generator) GenerateSchemaWithEmbedded(tables []meta.TableDirective, fields []meta.SchemaField, indexes []meta.SchemaIndex, enums []meta.GlobalEnum, embeddedFields []meta.EmbeddedField) *ast.StatementList {
	schema := builder.NewSchema()

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

		// Process embedded fields first to generate additional schema fields
		embeddedGeneratedFields := builder.ProcessEmbeddedFields(embeddedFields, fields, table.StructName)

		// Combine original fields with embedded-generated fields
		allFields := append(fields, embeddedGeneratedFields...)

		// Sort fields to ensure primary keys come first, then other fields
		var primaryFields, otherFields []meta.SchemaField
		for _, field := range allFields {
			if field.StructName == table.StructName {
				if field.Primary {
					primaryFields = append(primaryFields, field)
				} else {
					otherFields = append(otherFields, field)
				}
			}
		}

		// Process primary key fields first, then other fields
		sortedFields := append(primaryFields, otherFields...)

		// Add columns
		for _, field := range sortedFields {
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
				refTable, refColumn := g.ParseForeignKeyReference(field.Foreign)
				columnBuilder.ForeignKey(refTable, refColumn, field.ForeignKeyName).End()
			} else {
				columnBuilder.End()
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

// GenerateCreateTableWithEmbedded generates CREATE TABLE SQL with embedded field support (base implementation)
func (g *Generator) GenerateCreateTableWithEmbedded(table meta.TableDirective, fields []meta.SchemaField, indexes []meta.SchemaIndex, enums []meta.GlobalEnum, embeddedFields []meta.EmbeddedField) string {
	// This is a base implementation that should be overridden by specific dialect generators
	// Return a simple string representation (this should be overridden by dialect-specific generators)
	return "-- Base implementation: use dialect-specific generator for proper SQL output"
}

// ParseForeignKeyReference parses foreign key reference string
// Supports formats: "table(column)" or just "table"
func (g *Generator) ParseForeignKeyReference(foreign string) (table, column string) {
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
