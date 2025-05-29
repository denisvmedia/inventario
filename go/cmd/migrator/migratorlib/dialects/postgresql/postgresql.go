package postgresql

import (
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dialects/base"
	"github.com/denisvmedia/inventario/ptah/platform"
	"github.com/denisvmedia/inventario/ptah/schema/ast"
	"github.com/denisvmedia/inventario/ptah/schema/builder"
	"github.com/denisvmedia/inventario/ptah/schema/meta"
	"github.com/denisvmedia/inventario/ptah/schema/renderer"
)

// Generator handles PostgreSQL-specific SQL generation using AST
type Generator struct {
	*base.Generator
	renderer *renderer.PostgreSQLRenderer
}

// New creates a new PostgreSQL generator
func New() *Generator {
	return &Generator{
		Generator: base.NewGenerator(platform.PlatformTypePostgres),
		renderer:  renderer.NewPostgreSQLRenderer(),
	}
}

// convertFieldToColumn converts a SchemaField to an AST ColumnNode for PostgreSQL
func (g *Generator) convertFieldToColumn(field meta.SchemaField, enums []meta.GlobalEnum) *ast.ColumnNode {
	ftype := field.Type

	// Handle auto-increment for PostgreSQL by converting to SERIAL types (before platform overrides)
	if field.AutoInc {
		switch ftype {
		case "INTEGER", "INT":
			ftype = "SERIAL"
		case "BIGINT":
			ftype = "BIGSERIAL"
		case "SMALLINT":
			ftype = "SMALLSERIAL"
		default:
			// For other types, default to SERIAL
			ftype = "SERIAL"
		}
	}

	// Check for platform-specific type override (takes precedence over auto-increment conversion)
	if dialectAttrs, ok := field.Overrides[platform.PlatformTypePostgres]; ok {
		if typeOverride, ok := dialectAttrs["type"]; ok {
			ftype = typeOverride
		}
	}

	// For PostgreSQL, enum types are used directly (they're defined separately)
	// No need to transform enum types - they're already correct

	// Create column node with the converted type
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

	// Auto-increment is handled by type conversion to SERIAL above
	// PostgreSQL doesn't use AUTO_INCREMENT keyword

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
func (g *Generator) convertTableDirectiveToAST(table meta.TableDirective, fields []meta.SchemaField, enums []meta.GlobalEnum) *ast.CreateTableNode {
	createTable := ast.NewCreateTable(table.Name)

	// Set table comment
	if table.Comment != "" {
		createTable.Comment = table.Comment
	}

	// PostgreSQL doesn't support table-level options like MySQL ENGINE
	// So we ignore table.Overrides for PostgreSQL

	// Sort fields to ensure primary keys come first, then other fields
	var primaryFields, otherFields []meta.SchemaField
	for _, field := range fields {
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
		column := g.convertFieldToColumn(field, enums)
		createTable.AddColumn(column)
	}

	// Add composite primary key if specified
	if len(table.PrimaryKey) > 1 {
		constraint := ast.NewPrimaryKeyConstraint(table.PrimaryKey...)
		createTable.AddConstraint(constraint)
	}

	// Add foreign key constraints
	for _, field := range fields {
		if field.StructName == table.StructName && field.Foreign != "" {
			// Parse foreign key reference
			refTable, refColumn := g.ParseForeignKeyReference(field.Foreign)

			// Create table-level foreign key constraint
			ref := &ast.ForeignKeyRef{
				Table:  refTable,
				Column: refColumn,
				Name:   field.ForeignKeyName,
			}
			constraint := ast.NewForeignKeyConstraint(field.ForeignKeyName, []string{field.Name}, ref)
			createTable.AddConstraint(constraint)
		}
	}

	return createTable
}

// GenerateCreateTable generates CREATE TABLE SQL for PostgreSQL using AST
// Note: This method should not be used for package-level generation as it includes enum definitions.
// Use GenerateCreateTableWithoutEnums for package-level generation where enums are handled separately.
func (g *Generator) GenerateCreateTable(table meta.TableDirective, fields []meta.SchemaField, indexes []meta.SchemaIndex, enums []meta.GlobalEnum) string {
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

// GenerateCreateTableWithoutEnums generates CREATE TABLE SQL for PostgreSQL without enum definitions
// This is used when enums are handled at the schema level to avoid duplication
func (g *Generator) GenerateCreateTableWithoutEnums(table meta.TableDirective, fields []meta.SchemaField, indexes []meta.SchemaIndex, enums []meta.GlobalEnum) string {
	// Convert table directive to AST
	createTableNode := g.convertTableDirectiveToAST(table, fields, enums)

	// Build a statement list with just table and indexes (no enums)
	var statements []ast.Node

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

// GenerateCreateTableWithEmbedded generates CREATE TABLE SQL for PostgreSQL with embedded field support
// This method is used by the package-migrator and does not include enum definitions to avoid duplication
func (g *Generator) GenerateCreateTableWithEmbedded(table meta.TableDirective, fields []meta.SchemaField, indexes []meta.SchemaIndex, enums []meta.GlobalEnum, embeddedFields []meta.EmbeddedField) string {
	// Process embedded fields to generate additional schema fields
	embeddedGeneratedFields := builder.ProcessEmbeddedFields(embeddedFields, fields, table.StructName)

	// Combine original fields with embedded-generated fields
	allFields := append(fields, embeddedGeneratedFields...)

	// Use the PostgreSQL generation logic without enum definitions (enums are handled at schema level)
	return g.GenerateCreateTableWithoutEnums(table, allFields, indexes, enums)
}

// GenerateAlterStatements generates ALTER statements for PostgreSQL using AST
func (g *Generator) GenerateAlterStatements(oldFields, newFields []meta.SchemaField) string {
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
