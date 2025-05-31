package generic

import (
	"fmt"

	"github.com/denisvmedia/inventario/ptah/renderer"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/base"
	"github.com/denisvmedia/inventario/ptah/schema/differ/differtypes"
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
	"github.com/denisvmedia/inventario/ptah/schema/transform"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// Generator handles unknown dialects without applying dialect-specific transformations
type Generator struct {
	*base.Generator
	renderer *renderer.BaseRenderer
}

// New creates a new generic generator
func New(dialectName string) *Generator {
	return &Generator{
		Generator: base.NewGenerator(dialectName),
		renderer:  renderer.NewBaseRenderer(dialectName),
	}
}

// GenerateCreateTable generates CREATE TABLE SQL for unknown dialects using AST
func (g *Generator) GenerateCreateTable(table types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum) string {
	// Use the base generator's schema generation method
	schema := g.GenerateSchema([]types.TableDirective{table}, fields, indexes, enums)

	// Render using the base renderer by iterating through statements
	result, err := g.renderer.Render(schema)
	if err != nil {
		// Fallback to simple rendering if schema rendering fails
		createTableNode := g.Generator.GenerateCreateTable(table, fields, enums)
		simpleResult, _ := g.renderer.Render(createTableNode)
		return simpleResult
	}

	return result
}

// GenerateCreateTableWithEmbedded generates CREATE TABLE SQL for generic dialects with embedded field support
func (g *Generator) GenerateCreateTableWithEmbedded(table types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum, embeddedFields []types.EmbeddedField) string {
	// Process embedded fields to generate additional schema fields
	embeddedGeneratedFields := transform.ProcessEmbeddedFields(embeddedFields, fields, table.StructName)

	// Combine original fields with embedded-generated fields
	allFields := append(fields, embeddedGeneratedFields...)

	// Use the regular generic generation logic with the combined fields
	return g.GenerateCreateTable(table, allFields, indexes, enums)
}

// GenerateAlterStatements generates ALTER statements for unknown dialects using AST
func (g *Generator) GenerateAlterStatements(oldFields, newFields []types.SchemaField) string {
	// For now, return a simple comment indicating this is not yet implemented with AST
	// This would need to be implemented with proper ALTER TABLE AST nodes
	return "-- ALTER statements not yet implemented with AST for generic dialect\n"
}

// GenerateMigrationSQL generates generic migration SQL statements from schema differences.
//
// This method provides a basic implementation for unknown database dialects that don't have
// specific dialect generators. It generates standard SQL statements that should work with
// most SQL databases, but may not be optimized for any particular database system.
//
// # Migration Order
//
// The SQL statements are generated in a basic order:
//  1. Create new tables (using generic SQL syntax)
//  2. Modify existing tables (basic ALTER TABLE statements)
//  3. Add new indexes (basic CREATE INDEX statements)
//  4. Remove indexes (basic DROP INDEX statements)
//  5. Remove tables (dangerous - commented out by default)
//
// # Generic Features
//
//   - Standard SQL syntax without dialect-specific optimizations
//   - Basic CREATE TABLE, ALTER TABLE, and INDEX statements
//   - Conservative approach with many operations commented out for safety
//   - Fallback implementation for unsupported database dialects
//
// # Parameters
//
//   - diff: The schema differences to be applied
//   - generated: The target schema parsed from Go struct annotations
//
// # Return Value
//
// Returns a slice of SQL statements as strings. Many statements may be commented out
// with TODO or WARNING prefixes, requiring manual review before execution.
func (g *Generator) GenerateMigrationSQL(diff *differtypes.SchemaDiff, generated *parsertypes.PackageParseResult) []string {
	var statements []string

	// Add a warning about using generic dialect
	statements = append(statements, fmt.Sprintf("-- WARNING: Using generic dialect '%s' - review all statements carefully!", g.GetDialectName()))
	statements = append(statements, "-- Many operations are commented out for safety and may need manual implementation.")
	statements = append(statements, "")

	// 1. Add new tables
	for _, tableName := range diff.TablesAdded {
		// Find the table in generated schema and create it
		for _, table := range generated.Tables {
			if table.Name == tableName {
				// Use the existing generic table generation logic
				createSQL := g.GenerateCreateTable(table, generated.Fields, generated.Indexes, generated.Enums)
				statements = append(statements, createSQL)
				break
			}
		}
	}

	// 2. Modify existing tables
	for _, tableDiff := range diff.TablesModified {
		statements = append(statements, fmt.Sprintf("-- Modify table: %s", tableDiff.TableName))

		// Add new columns (commented out for safety)
		for _, colName := range tableDiff.ColumnsAdded {
			statements = append(statements, fmt.Sprintf("-- TODO: ALTER TABLE %s ADD COLUMN %s ...; -- Define column type and constraints", tableDiff.TableName, colName))
		}

		// Modify existing columns (commented out for safety)
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

	// 3. Add new indexes (commented out for safety)
	for _, indexName := range diff.IndexesAdded {
		statements = append(statements, fmt.Sprintf("-- TODO: CREATE INDEX %s ON table_name (column_list); -- Define table and columns", indexName))
	}

	// 4. Remove indexes
	for _, indexName := range diff.IndexesRemoved {
		statements = append(statements, fmt.Sprintf("DROP INDEX %s;", indexName))
	}

	// 5. Remove tables (dangerous!)
	for _, tableName := range diff.TablesRemoved {
		statements = append(statements, fmt.Sprintf("-- WARNING: DROP TABLE %s; -- This will delete all data!", tableName))
	}

	// 6. Enum operations (commented out for generic dialect)
	for _, enumName := range diff.EnumsAdded {
		statements = append(statements, fmt.Sprintf("-- TODO: Create enum type %s; -- Enum syntax varies by database", enumName))
	}

	for _, enumName := range diff.EnumsRemoved {
		statements = append(statements, fmt.Sprintf("-- WARNING: Drop enum type %s; -- Enum syntax varies by database", enumName))
	}

	for _, enumDiff := range diff.EnumsModified {
		statements = append(statements, fmt.Sprintf("-- TODO: Modify enum type %s; -- Enum modification syntax varies by database", enumDiff.EnumName))
	}

	return statements
}
