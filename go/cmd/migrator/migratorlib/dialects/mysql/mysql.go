package mysql

import (
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/ast"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/constants"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dialects/base"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/renderers"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

// Generator handles MySQL-specific SQL generation using AST
type Generator struct {
	*base.Generator
	renderer *renderers.MySQLRenderer
}

// New creates a new MySQL generator
func New() *Generator {
	return &Generator{
		Generator: base.NewGenerator(constants.PlatformTypeMySQL),
		renderer:  renderers.NewMySQLRenderer(),
	}
}

// convertFieldToColumn converts a SchemaField to an AST ColumnNode for MySQL
func (g *Generator) convertFieldToColumn(field types.SchemaField, enums []types.GlobalEnum) *ast.ColumnNode {
	ftype := field.Type

	// Check for platform-specific type override
	if dialectAttrs, ok := field.Overrides[constants.PlatformTypeMySQL]; ok {
		if typeOverride, ok := dialectAttrs["type"]; ok {
			ftype = typeOverride
		}
	}

	// Keep the original enum type name - the renderer will handle MySQL-specific enum formatting
	// No need to transform enum types here, let the AST and renderer handle it

	// Create column node with the original type (enum names will be handled by renderer)
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

	// Handle foreign key
	if field.Foreign != "" {
		column.SetForeignKey(field.Foreign, field.Name, field.ForeignKeyName)
	}

	return column
}

// convertTableDirectiveToAST converts a TableDirective to an AST CreateTableNode for MySQL
func (g *Generator) convertTableDirectiveToAST(table types.TableDirective, fields []types.SchemaField, enums []types.GlobalEnum) *ast.CreateTableNode {
	createTable := ast.NewCreateTable(table.Name)

	// Set table comment
	if table.Comment != "" {
		createTable.Comment = table.Comment
	}

	// Handle MySQL-specific table options
	if dialectAttrs, ok := table.Overrides[constants.PlatformTypeMySQL]; ok {
		// Handle ENGINE option
		if engine, ok := dialectAttrs["engine"]; ok {
			createTable.SetOption("ENGINE", engine)
		}

		// Handle COMMENT option (if not already set from table.Comment)
		if comment, ok := dialectAttrs["comment"]; ok && createTable.Comment == "" {
			createTable.Comment = comment
		}

		// Add any other platform-specific options
		for k, v := range dialectAttrs {
			if k != "engine" && k != "comment" && k != "type" {
				createTable.SetOption(k, v)
			}
		}
	}

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

	return createTable
}

// GenerateCreateTable generates CREATE TABLE SQL for MySQL using AST
func (g *Generator) GenerateCreateTable(table types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum) string {
	// Convert table directive to AST
	createTableNode := g.convertTableDirectiveToAST(table, fields, enums)

	// Build a statement list manually
	var statements []ast.Node
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

	// Create enum map for renderer
	enumMap := make(map[string][]string)
	for _, enum := range enums {
		enumMap[enum.Name] = enum.Values
	}

	// Use the enhanced renderer method that handles enums properly
	if len(enums) > 0 {
		result, err := g.renderSchemaWithEnums(&ast.StatementList{Statements: statements}, enumMap)
		if err != nil {
			return "-- Error rendering MySQL schema with enums: " + err.Error() + "\n"
		}
		return result
	}

	// Create statement list and render normally if no enums
	schemaAST := &ast.StatementList{Statements: statements}
	result, err := g.renderer.RenderSchema(schemaAST)
	if err != nil {
		// Fallback to error message if rendering fails
		return "-- Error rendering MySQL schema: " + err.Error() + "\n"
	}

	return result
}

// renderSchemaWithEnums renders a schema using the MySQL renderer's enum support
func (g *Generator) renderSchemaWithEnums(statements *ast.StatementList, enumMap map[string][]string) (string, error) {
	g.renderer.Reset()

	// MySQL doesn't need separate enum definitions, so we render everything in order
	for _, stmt := range statements.Statements {
		// Skip enum nodes as MySQL handles enums inline
		if _, ok := stmt.(*ast.EnumNode); ok {
			continue
		}

		// Use enhanced rendering for CREATE TABLE with enum support
		if createTable, ok := stmt.(*ast.CreateTableNode); ok {
			if err := g.renderer.VisitCreateTableWithEnums(createTable, enumMap); err != nil {
				return "", err
			}
		} else {
			if err := stmt.Accept(g.renderer); err != nil {
				return "", err
			}
		}
	}

	return g.renderer.GetOutput(), nil
}

// GenerateAlterStatements generates ALTER statements for MySQL using AST
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
					// MySQL uses MODIFY COLUMN for both type and nullability changes
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

	for tableName, operations := range tableOperations {
		alterNode := &ast.AlterTableNode{
			Name:       tableName,
			Operations: operations,
		}
		statements = append(statements, alterNode)
	}

	// Render using MySQL renderer
	schemaAST := &ast.StatementList{Statements: statements}
	result, err := g.renderer.RenderSchema(schemaAST)
	if err != nil {
		// Fallback to error message if rendering fails
		return "-- Error rendering MySQL ALTER statements: " + err.Error() + "\n"
	}

	return result
}
