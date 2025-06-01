package postgres

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/convert/fromschema"
	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/migration/schemadiff/types"
)

// Planner implements PostgreSQL-specific migration planning functionality.
//
// The Planner is responsible for converting schema differences into PostgreSQL-compatible
// AST nodes that can be rendered into executable SQL statements. It handles PostgreSQL-specific
// features like ENUM types, SERIAL columns, and proper dependency ordering.
//
// # Usage Example
//
//	planner := &postgres.Planner{}
//
//	// Schema differences from comparison
//	diff := &differtypes.SchemaDiff{
//		EnumsAdded:  []string{"user_status"},
//		TablesAdded: []string{"users"},
//	}
//
//	// Target schema from Go struct parsing
//	generated := &goschema.Database{
//		Enums: []goschema.Enum{
//			{Name: "user_status", Values: []string{"active", "inactive"}},
//		},
//		Tables: []goschema.Table{
//			{Name: "users", StructName: "User"},
//		},
//		Fields: []goschema.Field{
//			{Name: "id", Type: "SERIAL", StructName: "User", Primary: true},
//		},
//	}
//
//	// Generate migration AST nodes
//	nodes := planner.GenerateMigrationAST(diff, generated)
//
// # Thread Safety
//
// The Planner is stateless and safe for concurrent use across multiple goroutines.
// Each call to GenerateMigrationSQL operates independently without shared state.
type Planner struct {
}

func New() *Planner {
	return &Planner{}
}

// GenerateMigrationAST generates PostgreSQL-specific migration AST statements from schema differences.
//
// This method transforms the schema differences captured in the SchemaDiff into executable
// PostgreSQL AST statements that can be applied to bring the database schema in line with the target
// schema. The generated AST follows PostgreSQL-specific syntax and best practices.
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
// # Examples
//
// Basic enum and table creation:
//
//	diff := &differtypes.SchemaDiff{
//		EnumsAdded:  []string{"user_status"},
//		TablesAdded: []string{"users"},
//	}
//
//	generated := &goschema.Database{
//		Enums: []goschema.Enum{
//			{Name: "user_status", Values: []string{"active", "inactive"}},
//		},
//		Tables: []goschema.Table{
//			{Name: "users", StructName: "User"},
//		},
//		Fields: []goschema.Field{
//			{Name: "id", Type: "SERIAL", StructName: "User", Primary: true},
//			{Name: "status", Type: "user_status", StructName: "User"},
//		},
//	}
//
//	nodes := planner.GenerateMigrationAST(diff, generated)
//	// Results in:
//	// 1. CREATE TYPE user_status AS ENUM ('active', 'inactive');
//	// 2. CREATE TABLE users (id SERIAL PRIMARY KEY, status user_status);
//
// Table modification with column changes:
//
//	diff := &differtypes.SchemaDiff{
//		TablesModified: []differtypes.TableDiff{
//			{
//				TableName:    "users",
//				ColumnsAdded: []string{"email"},
//				ColumnsModified: []differtypes.ColumnDiff{
//					{ColumnName: "name", Changes: map[string]string{"type": "VARCHAR(255)"}},
//				},
//			},
//		},
//	}
//	// Results in ALTER TABLE statements for adding and modifying columns
//
// # Return Value
//
// Returns a slice of AST nodes representing SQL statements. Each node can be rendered
// to SQL using a PostgreSQL-specific visitor. Comments and warnings are included
// as CommentNode instances for documentation and safety.
func (p *Planner) GenerateMigrationAST(diff *types.SchemaDiff, generated *goschema.Database) []ast.Node {
	var result []ast.Node

	// 1. Add new enums first (PostgreSQL requires enum types to exist before tables use them)
	for _, enumName := range diff.EnumsAdded {
		for _, enum := range generated.Enums {
			if enum.Name == enumName {
				values := make([]string, len(enum.Values))
				for i, v := range enum.Values {
					values[i] = "'" + v + "'"
				}

				enumNode := ast.NewEnum(enum.Name, enum.Values...)
				result = append(result, enumNode)
				break
			}
		}
	}

	// 2. Modify existing enums (add values only - PostgreSQL doesn't support removing enum values easily)
	for _, enumDiff := range diff.EnumsModified {
		astNode := ast.NewAlterType(enumDiff.EnumName)
		for _, value := range enumDiff.ValuesAdded {
			addEnumAst := ast.NewAddEnumValueOperation(value)
			astNode.AddOperation(addEnumAst)
		}
		result = append(result, astNode)

		// Note: PostgreSQL doesn't support removing enum values without recreating the enum
		if len(enumDiff.ValuesRemoved) > 0 {
			astCommentNode := ast.NewComment(fmt.Sprintf("WARNING: Cannot remove enum values %v from %s without recreating the enum", enumDiff.ValuesRemoved, enumDiff.EnumName))
			result = append(result, astCommentNode)
		}
	}

	// 3. Add new tables
	for _, tableName := range diff.TablesAdded {
		// Find the table in generated schema and create it
		for _, table := range generated.Tables {
			if table.Name == tableName {
				astNode := ast.NewCreateTable(tableName)
				for _, field := range generated.Fields {
					if field.StructName == table.StructName {
						columnNode := fromschema.FromField(field, generated.Enums, "postgres")
						astNode.AddColumn(columnNode)
					}
				}
				result = append(result, astNode)
				break
			}
		}
	}

	// 4. Modify existing tables
	for _, tableDiff := range diff.TablesModified {
		astCommentNode := ast.NewComment(fmt.Sprintf("Modify table: %s", tableDiff.TableName))
		result = append(result, astCommentNode)

		// Add new columns
		for _, colName := range tableDiff.ColumnsAdded {
			// Find the field definition for this column
			// We need to find the struct name that corresponds to this table name
			var targetField *goschema.Field
			var targetStructName string

			// First, find the struct name for this table
			for _, table := range generated.Tables {
				if table.Name == tableDiff.TableName {
					targetStructName = table.StructName
					break
				}
			}

			// Now find the field using the correct struct name
			for _, field := range generated.Fields {
				if field.StructName == targetStructName && field.Name == colName {
					targetField = &field
					break
				}
			}

			if targetField != nil {
				columnNode := fromschema.FromField(*targetField, generated.Enums, "postgres")
				// Generate ADD COLUMN statement using AST
				alterNode := &ast.AlterTableNode{
					Name:       tableDiff.TableName,
					Operations: []ast.AlterOperation{&ast.AddColumnOperation{Column: columnNode}},
				}
				result = append(result, alterNode)
			}
		}

		// Modify existing columns
		for _, colDiff := range tableDiff.ColumnsModified {
			// Find the target field definition for this column
			// We need to find the struct name that corresponds to this table name
			var targetField *goschema.Field
			var targetStructName string

			// First, find the struct name for this table
			for _, table := range generated.Tables {
				if table.Name == tableDiff.TableName {
					targetStructName = table.StructName
					break
				}
			}

			// Now find the field using the correct struct name
			for _, field := range generated.Fields {
				if field.StructName == targetStructName && field.Name == colDiff.ColumnName {
					targetField = &field
					break
				}
			}

			if targetField == nil {
				astCommentNode := ast.NewComment(fmt.Sprintf("ERROR: Could not find field definition for %s.%s (struct: %s)", tableDiff.TableName, colDiff.ColumnName, targetStructName))
				result = append(result, astCommentNode)
				continue
			}

			// Create a column definition with the target field properties
			columnNode := fromschema.FromField(*targetField, generated.Enums, "postgres")

			// Generate ALTER COLUMN statements using AST
			alterNode := &ast.AlterTableNode{
				Name:       tableDiff.TableName,
				Operations: []ast.AlterOperation{&ast.ModifyColumnOperation{Column: columnNode}},
			}
			result = append(result, alterNode)

			// Add a comment showing what changes are being made
			changesList := make([]string, 0, len(colDiff.Changes))
			for changeType, change := range colDiff.Changes {
				changesList = append(changesList, fmt.Sprintf("%s: %s", changeType, change))
			}
			astCommentNode := ast.NewComment(fmt.Sprintf("Modify column %s.%s: %s", tableDiff.TableName, colDiff.ColumnName, strings.Join(changesList, ", ")))
			result = append(result, astCommentNode)
		}

		// Remove columns (dangerous!)
		for _, colName := range tableDiff.ColumnsRemoved {
			// Generate DROP COLUMN statement using AST
			alterNode := &ast.AlterTableNode{
				Name:       tableDiff.TableName,
				Operations: []ast.AlterOperation{&ast.DropColumnOperation{ColumnName: colName}},
			}
			result = append(result, alterNode)
			astCommentNode := ast.NewComment(fmt.Sprintf("WARNING: Dropping column %s.%s - This will delete data!", tableDiff.TableName, colName))
			result = append(result, astCommentNode)
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
				if idx.Comment != "" {
					indexNode.Comment = idx.Comment
				}
				result = append(result, indexNode)
				break
			}
		}
	}

	// 6. Remove indexes (safe operations)
	for _, indexName := range diff.IndexesRemoved {
		dropIndexNode := ast.NewDropIndex(indexName).
			SetIfExists()
		result = append(result, dropIndexNode)
	}

	// 7. Remove tables (dangerous!)
	for _, tableName := range diff.TablesRemoved {
		dropTableNode := ast.NewDropTable(tableName).
			SetIfExists().
			SetCascade().
			SetComment("WARNING: This will delete all data!")

		result = append(result, dropTableNode)
	}

	// 8. Remove enums (dangerous!)
	for _, enumName := range diff.EnumsRemoved {
		dropTypeNode := ast.NewDropType(enumName).
			SetIfExists().
			SetCascade().
			SetComment("WARNING: Make sure no tables use this enum!")

		result = append(result, dropTypeNode)
	}

	return result
}
