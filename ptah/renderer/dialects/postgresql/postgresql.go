package postgresql

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/platform"
	"github.com/denisvmedia/inventario/ptah/renderer"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/base"
	"github.com/denisvmedia/inventario/ptah/schema/ast"
	"github.com/denisvmedia/inventario/ptah/schema/differ/differtypes"
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
	"github.com/denisvmedia/inventario/ptah/schema/transform"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// Generator handles PostgreSQL-specific SQL generation using AST
type Generator struct {
	*base.Generator
	renderer *renderer.PostgreSQLRenderer
}

// New creates a new PostgreSQL generator
func New() *Generator {
	return &Generator{
		Generator: base.NewGenerator(platform.Postgres),
		renderer:  renderer.NewPostgreSQLRenderer(),
	}
}

// convertFieldToColumn converts a SchemaField to an AST ColumnNode for PostgreSQL
func (g *Generator) convertFieldToColumn(field types.SchemaField, enums []types.GlobalEnum) *ast.ColumnNode {
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
	if dialectAttrs, ok := field.Overrides[platform.Postgres]; ok {
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
func (g *Generator) convertTableDirectiveToAST(table types.TableDirective, fields []types.SchemaField, enums []types.GlobalEnum) *ast.CreateTableNode {
	createTable := ast.NewCreateTable(table.Name)

	// Set table comment
	if table.Comment != "" {
		createTable.Comment = table.Comment
	}

	// PostgreSQL doesn't support table-level options like MySQL ENGINE
	// So we ignore table.Overrides for PostgreSQL

	// Sort fields to ensure primary keys come first, then other fields
	var primaryFields, otherFields []types.SchemaField
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

// GenerateCreateTableWithoutEnums generates CREATE TABLE SQL for PostgreSQL without enum definitions
// This is used when enums are handled at the schema level to avoid duplication
func (g *Generator) GenerateCreateTableWithoutEnums(table types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum) string {
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
func (g *Generator) GenerateCreateTableWithEmbedded(table types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum, embeddedFields []types.EmbeddedField) string {
	// Process embedded fields to generate additional schema fields
	embeddedGeneratedFields := transform.ProcessEmbeddedFields(embeddedFields, fields, table.StructName)

	// Combine original fields with embedded-generated fields
	allFields := append(fields, embeddedGeneratedFields...)

	// Use the PostgreSQL generation logic without enum definitions (enums are handled at schema level)
	return g.GenerateCreateTableWithoutEnums(table, allFields, indexes, enums)
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

// GenerateMigrationSQL generates PostgreSQL-specific migration SQL statements from schema differences.
//
// This method transforms the schema differences captured in the SchemaDiff into executable
// PostgreSQL SQL statements that can be applied to bring the database schema in line with the target
// schema. The generated SQL follows PostgreSQL-specific syntax and best practices.
//
// # Migration Order
//
// The SQL statements are generated in a specific order to avoid dependency conflicts:
//  1. Create new enum types (required before tables that use them)
//  2. Modify existing enum types (add new values)
//  3. Create new tables
//  4. Modify existing tables (add/modify/remove columns)
//  5. Add new indexes
//  6. Remove indexes (safe operations)
//  7. Remove tables (dangerous - commented out by default)
//  8. Remove enum types (dangerous - commented out by default)
//
// # PostgreSQL-Specific Features
//
//   - Native ENUM types with CREATE TYPE and ALTER TYPE statements
//   - SERIAL columns for auto-increment functionality
//   - Proper handling of enum value limitations (cannot remove values easily)
//   - PostgreSQL-specific syntax for ALTER statements
//
// # Parameters
//
//   - diff: The schema differences to be applied
//   - generated: The target schema parsed from Go struct annotations
//
// # Return Value
//
// Returns a slice of SQL statements as strings. Each statement is a complete SQL
// command that can be executed independently. Comments and warnings are included
// as SQL comments (lines starting with "--").
func (g *Generator) GenerateMigrationSQL(diff *differtypes.SchemaDiff, generated *parsertypes.PackageParseResult) []string {
	var statements []string

	// 1. Add new enums first (PostgreSQL requires enum types to exist before tables use them)
	for _, enumName := range diff.EnumsAdded {
		for _, enum := range generated.Enums {
			if enum.Name == enumName {
				values := make([]string, len(enum.Values))
				for i, v := range enum.Values {
					values[i] = "'" + v + "'"
				}
				sql := fmt.Sprintf("CREATE TYPE %s AS ENUM (%s);", enum.Name, strings.Join(values, ", "))
				statements = append(statements, sql)
				break
			}
		}
	}

	// 2. Modify existing enums (add values only - PostgreSQL doesn't support removing enum values easily)
	for _, enumDiff := range diff.EnumsModified {
		for _, value := range enumDiff.ValuesAdded {
			sql := fmt.Sprintf("ALTER TYPE %s ADD VALUE '%s';", enumDiff.EnumName, value)
			statements = append(statements, sql)
		}
		// Note: PostgreSQL doesn't support removing enum values without recreating the enum
		if len(enumDiff.ValuesRemoved) > 0 {
			statements = append(statements, fmt.Sprintf("-- WARNING: Cannot remove enum values %v from %s without recreating the enum", enumDiff.ValuesRemoved, enumDiff.EnumName))
		}
	}

	// 3. Add new tables
	for _, tableName := range diff.TablesAdded {
		// Find the table in generated schema and create it
		for _, table := range generated.Tables {
			if table.Name == tableName {
				// Use the existing PostgreSQL table generation logic
				createSQL := g.GenerateCreateTableWithoutEnums(table, generated.Fields, generated.Indexes, generated.Enums)
				statements = append(statements, createSQL)
				break
			}
		}
	}

	// 4. Modify existing tables
	for _, tableDiff := range diff.TablesModified {
		statements = append(statements, fmt.Sprintf("-- Modify table: %s", tableDiff.TableName))

		// Add new columns
		for _, colName := range tableDiff.ColumnsAdded {
			// Find the field definition for this column
			for _, field := range generated.Fields {
				if field.Name == colName {
					column := g.convertFieldToColumn(field, generated.Enums)
					// Generate ADD COLUMN statement using AST
					alterNode := &ast.AlterTableNode{
						Name:       tableDiff.TableName,
						Operations: []ast.AlterOperation{&ast.AddColumnOperation{Column: column}},
					}
					result, err := g.renderer.Render(alterNode)
					if err != nil {
						statements = append(statements, fmt.Sprintf("-- ERROR: Failed to generate ADD COLUMN for %s.%s: %v", tableDiff.TableName, colName, err))
					} else {
						statements = append(statements, result)
					}
					break
				}
			}
		}

		// Modify existing columns
		for _, colDiff := range tableDiff.ColumnsModified {
			for changeType, change := range colDiff.Changes {
				statements = append(statements, fmt.Sprintf("-- TODO: ALTER TABLE %s ALTER COLUMN %s %s (%s);", tableDiff.TableName, colDiff.ColumnName, changeType, change))
			}
		}

		// Remove columns (dangerous!)
		for _, colName := range tableDiff.ColumnsRemoved {
			statements = append(statements, fmt.Sprintf("-- WARNING: ALTER TABLE %s DROP COLUMN %s; -- This will delete data!", tableDiff.TableName, colName))
		}
	}

	// 5. Add new indexes
	for _, indexName := range diff.IndexesAdded {
		// Find the index definition
		for _, idx := range generated.Indexes {
			if idx.Name == indexName {
				indexNode := ast.NewIndex(idx.Name, idx.StructName, idx.Fields...)
				if idx.Unique {
					indexNode.Unique = true
				}
				result, err := g.renderer.Render(indexNode)
				if err != nil {
					statements = append(statements, fmt.Sprintf("-- ERROR: Failed to generate CREATE INDEX for %s: %v", indexName, err))
				} else {
					statements = append(statements, result)
				}
				break
			}
		}
	}

	// 6. Remove indexes (safe operations)
	for _, indexName := range diff.IndexesRemoved {
		statements = append(statements, fmt.Sprintf("DROP INDEX IF EXISTS %s;", indexName))
	}

	// 7. Remove tables (dangerous!)
	for _, tableName := range diff.TablesRemoved {
		statements = append(statements, fmt.Sprintf("-- WARNING: DROP TABLE %s; -- This will delete all data!", tableName))
	}

	// 8. Remove enums (dangerous!)
	for _, enumName := range diff.EnumsRemoved {
		statements = append(statements, fmt.Sprintf("-- WARNING: DROP TYPE %s; -- Make sure no tables use this enum!", enumName))
	}

	return statements
}
