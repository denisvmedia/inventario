package postgresql

import (
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/ast"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/constants"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dialects/base"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/renderers"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

// Generator handles PostgreSQL-specific SQL generation using AST
type Generator struct {
	*base.Generator
	renderer *renderers.PostgreSQLRenderer
}

// New creates a new PostgreSQL generator
func New() *Generator {
	return &Generator{
		Generator: base.NewGenerator(constants.PlatformTypePostgres),
		renderer:  renderers.NewPostgreSQLRenderer(),
	}
}

// convertFieldToColumn converts a SchemaField to an AST ColumnNode for PostgreSQL
func (g *Generator) convertFieldToColumn(field types.SchemaField, enums []types.GlobalEnum) *ast.ColumnNode {
	ftype := field.Type

	// Check for platform-specific type override
	if dialectAttrs, ok := field.Overrides[constants.PlatformTypePostgres]; ok {
		if typeOverride, ok := dialectAttrs["type"]; ok {
			ftype = typeOverride
		}
	}

	// For PostgreSQL, enum types are used directly (they're defined separately)
	// No need to transform enum types - they're already correct

	// Create column node with the original type
	column := ast.NewColumn(field.Name, ftype)

	// Set column properties
	if field.Primary {
		column.SetPrimary()
	} else {
		if !field.Nullable {
			column.SetNotNull()
		}
		if field.Unique {
			column.SetUnique()
		}
	}

	if field.AutoInc {
		column.SetAutoIncrement()
	}

	if field.Default != "" {
		column.SetDefault(field.Default)
	}

	if field.DefaultFn != "" {
		column.SetDefaultFunction(field.DefaultFn)
	}

	if field.Check != "" {
		column.SetCheck(field.Check)
	}

	if field.Comment != "" {
		column.SetComment(field.Comment)
	}

	// Note: Foreign keys will be handled as table-level constraints
	// Don't set column-level foreign keys here

	return column
}

// convertTableDirectiveToAST converts a TableDirective to an AST CreateTableNode for PostgreSQL
func (g *Generator) convertTableDirectiveToAST(table types.TableDirective, fields []types.SchemaField, enums []types.GlobalEnum) *ast.CreateTableNode {
	createTable := ast.NewCreateTable(table.Name)

	// Set table comment
	if table.Comment != "" {
		createTable.Comment = table.Comment
	}

	// PostgreSQL doesn't support table-level options like MySQL ENGINE
	// So we ignore table.Overrides for PostgreSQL

	// Add columns
	for _, field := range fields {
		if field.StructName == table.StructName {
			column := g.convertFieldToColumn(field, enums)
			createTable.AddColumn(column)
		}
	}

	// Add composite primary key if specified
	if len(table.PrimaryKey) > 1 {
		constraint := ast.NewPrimaryKeyConstraint(table.PrimaryKey...)
		createTable.AddConstraint(constraint)
	}

	// Add foreign key constraints
	for _, field := range fields {
		if field.StructName == table.StructName && field.Foreign != "" {
			// Create table-level foreign key constraint
			ref := &ast.ForeignKeyRef{
				Table:  field.Foreign,
				Column: field.Name, // Assuming same column name in referenced table
				Name:   field.ForeignKeyName,
			}
			constraint := ast.NewForeignKeyConstraint(field.ForeignKeyName, []string{field.Name}, ref)
			createTable.AddConstraint(constraint)
		}
	}

	return createTable
}

// GenerateCreateTable generates CREATE TABLE SQL for PostgreSQL using AST
func (g *Generator) GenerateCreateTable(table types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum) string {
	// Convert table directive to AST
	createTableNode := g.convertTableDirectiveToAST(table, fields, enums)

	// Build a statement list with enums first, then table, then indexes
	var statements []ast.Node

	// Add enum definitions first (PostgreSQL requires this)
	for _, enum := range enums {
		enumNode := ast.NewEnum(enum.Name, enum.Values...)
		statements = append(statements, enumNode)
	}

	// Add the table
	statements = append(statements, createTableNode)

	// Add indexes
	for _, idx := range indexes {
		if idx.StructName == table.StructName {
			indexNode := ast.NewIndex(idx.Name, table.Name, idx.Fields...)
			if idx.Unique {
				indexNode.Unique = true
			}
			statements = append(statements, indexNode)
		}
	}

	// Create statement list and render using PostgreSQL renderer
	schemaAST := &ast.StatementList{Statements: statements}
	result, err := g.renderer.RenderSchema(schemaAST)
	if err != nil {
		// Fallback to error message if rendering fails
		return "-- Error rendering PostgreSQL schema: " + err.Error() + "\n"
	}

	return result
}

// GenerateAlterStatements generates ALTER statements for PostgreSQL using AST
func (g *Generator) GenerateAlterStatements(oldFields, newFields []types.SchemaField) string {
	// Group fields by table name
	tableOperations := make(map[string][]ast.AlterOperation)

	// Process each new field
	for _, newF := range newFields {
		found := false
		for _, oldF := range oldFields {
			if oldF.StructName == newF.StructName && oldF.Name == newF.Name {
				// Field exists, check for modifications
				if oldF.Type != newF.Type || oldF.Nullable != newF.Nullable {
					// PostgreSQL uses different syntax for modifying columns
					column := g.convertFieldToColumn(newF, nil)
					op := &ast.ModifyColumnOperation{Column: column}
					tableOperations[newF.StructName] = append(tableOperations[newF.StructName], op)
				}
				found = true
				break
			}
		}
		if !found {
			// New field, add it
			column := g.convertFieldToColumn(newF, nil)
			op := &ast.AddColumnOperation{Column: column}
			tableOperations[newF.StructName] = append(tableOperations[newF.StructName], op)
		}
	}

	// Build ALTER statements for each table
	var statements []ast.Node

	// Always add the comment, even if there are no operations
	statements = append(statements, ast.NewComment("ALTER statements: "))

	for tableName, operations := range tableOperations {
		alterNode := &ast.AlterTableNode{
			Name:       tableName,
			Operations: operations,
		}
		statements = append(statements, alterNode)
	}

	// If no operations, just return the comment
	if len(tableOperations) == 0 {
		return "-- ALTER statements: --\n\n"
	}

	// Render using PostgreSQL renderer
	schemaAST := &ast.StatementList{Statements: statements}
	result, err := g.renderer.RenderSchema(schemaAST)
	if err != nil {
		// Fallback to error message if rendering fails
		return "-- Error rendering PostgreSQL ALTER statements: " + err.Error() + "\n"
	}

	return result
}
