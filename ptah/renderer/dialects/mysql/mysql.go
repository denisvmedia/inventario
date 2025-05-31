package mysql

import (
	"fmt"

	"github.com/denisvmedia/inventario/ptah/platform"
	"github.com/denisvmedia/inventario/ptah/renderer"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/base"
	"github.com/denisvmedia/inventario/ptah/schema/ast"
	"github.com/denisvmedia/inventario/ptah/schema/differ/differtypes"
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
	"github.com/denisvmedia/inventario/ptah/schema/transform"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// Generator handles MySQL-specific SQL generation using AST
type Generator struct {
	*base.Generator
	renderer *renderer.MySQLRenderer
}

// New creates a new MySQL generator
func New() *Generator {
	return &Generator{
		Generator: base.NewGenerator(platform.MySQL),
		renderer:  renderer.NewMySQLRenderer(),
	}
}

// convertFieldToColumn converts a SchemaField to an AST ColumnNode for MySQL
func (g *Generator) convertFieldToColumn(field types.SchemaField, enums []types.GlobalEnum) *ast.ColumnNode {
	ftype := field.Type
	autoInc := field.AutoInc

	// Handle SERIAL types for MySQL by converting to INT and setting auto-increment
	if ftype == "SERIAL" {
		ftype = "INT"
		autoInc = true
	}

	// Check for platform-specific type override
	if dialectAttrs, ok := field.Overrides[platform.MySQL]; ok {
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

	if autoInc {
		column.SetAutoIncrement()
	}

	if field.Default != "" {
		column.SetDefault(field.Default)
	}

	if field.DefaultFn != "" {
		column.SetDefaultFunction(field.DefaultFn)
	}

	// Handle check constraint with platform-specific override
	checkConstraint := field.Check
	if dialectAttrs, ok := field.Overrides[platform.MySQL]; ok {
		if checkOverride, ok := dialectAttrs["check"]; ok {
			checkConstraint = checkOverride
		}
	}
	if checkConstraint != "" {
		column.SetCheck(checkConstraint)
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
	if dialectAttrs, ok := table.Overrides[platform.MySQL]; ok {
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

	// Add foreign key constraints
	for _, field := range fields {
		if field.StructName == table.StructName && field.Foreign != "" {
			// Parse foreign key reference
			refTable, refColumn := g.ParseForeignKeyReference(field.Foreign)

			// Create table-level foreign key constraint
			ref := &ast.ForeignKeyRef{
				Table:    refTable,
				Column:   refColumn,
				Name:     field.ForeignKeyName,
				OnDelete: "", // MySQL will use default behavior
				OnUpdate: "", // MySQL will use default behavior
			}
			constraint := ast.NewForeignKeyConstraint(field.ForeignKeyName, []string{field.Name}, ref)
			createTable.AddConstraint(constraint)
		}
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

// GenerateCreateTableWithEmbedded generates CREATE TABLE SQL for MySQL with embedded field support
func (g *Generator) GenerateCreateTableWithEmbedded(table types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum, embeddedFields []types.EmbeddedField) string {
	// Process embedded fields to generate additional schema fields
	embeddedGeneratedFields := transform.ProcessEmbeddedFields(embeddedFields, fields, table.StructName)

	// Combine original fields with embedded-generated fields
	allFields := append(fields, embeddedGeneratedFields...)

	// Use the regular MySQL generation logic with the combined fields
	return g.GenerateCreateTable(table, allFields, indexes, enums)
}

// GenerateMigrationSQL generates MySQL-specific migration SQL statements from schema differences.
//
// This method transforms the schema differences captured in the SchemaDiff into executable
// MySQL SQL statements that can be applied to bring the database schema in line with the target
// schema. The generated SQL follows MySQL-specific syntax and best practices.
//
// # Migration Order
//
// The SQL statements are generated in a specific order to avoid dependency conflicts:
//  1. Create new tables (MySQL handles enums inline, so no separate enum creation needed)
//  2. Modify existing tables (add/modify/remove columns)
//  3. Add new indexes
//  4. Remove indexes (safe operations)
//  5. Remove tables (dangerous - commented out by default)
//
// # MySQL-Specific Features
//
//   - Inline ENUM syntax within column definitions
//   - AUTO_INCREMENT for auto-increment functionality
//   - MySQL-specific ALTER TABLE syntax
//   - Support for MySQL table options (ENGINE, etc.)
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

	// MySQL doesn't need separate enum creation - enums are handled inline in table definitions

	// 1. Add new tables
	for _, tableName := range diff.TablesAdded {
		// Find the table in generated schema and create it
		for _, table := range generated.Tables {
			if table.Name == tableName {
				// Use the existing MySQL table generation logic
				createSQL := g.GenerateCreateTable(table, generated.Fields, generated.Indexes, generated.Enums)
				statements = append(statements, createSQL)
				break
			}
		}
	}

	// 2. Modify existing tables
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

					// Create enum map for MySQL enum handling
					enumMap := make(map[string][]string)
					for _, enum := range generated.Enums {
						enumMap[enum.Name] = enum.Values
					}

					// Generate ADD COLUMN statement using AST
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
				statements = append(statements, fmt.Sprintf("-- TODO: ALTER TABLE %s MODIFY COLUMN %s %s (%s);", tableDiff.TableName, colDiff.ColumnName, changeType, change))
			}
		}

		// Remove columns (dangerous!)
		for _, colName := range tableDiff.ColumnsRemoved {
			statements = append(statements, fmt.Sprintf("-- WARNING: ALTER TABLE %s DROP COLUMN %s; -- This will delete data!", tableDiff.TableName, colName))
		}
	}

	// 3. Add new indexes
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

	// 4. Remove indexes (safe operations)
	for _, indexName := range diff.IndexesRemoved {
		statements = append(statements, fmt.Sprintf("DROP INDEX %s;", indexName))
	}

	// 5. Remove tables (dangerous!)
	for _, tableName := range diff.TablesRemoved {
		statements = append(statements, fmt.Sprintf("-- WARNING: DROP TABLE %s; -- This will delete all data!", tableName))
	}

	// MySQL doesn't have separate enum types to remove - they're inline with tables

	return statements
}


