package dbschema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

// SchemaDiff represents differences between two schemas
type SchemaDiff struct {
	TablesAdded    []string    `json:"tables_added"`
	TablesRemoved  []string    `json:"tables_removed"`
	TablesModified []TableDiff `json:"tables_modified"`
	EnumsAdded     []string    `json:"enums_added"`
	EnumsRemoved   []string    `json:"enums_removed"`
	EnumsModified  []EnumDiff  `json:"enums_modified"`
	IndexesAdded   []string    `json:"indexes_added"`
	IndexesRemoved []string    `json:"indexes_removed"`
}

// TableDiff represents differences in a table
type TableDiff struct {
	TableName       string       `json:"table_name"`
	ColumnsAdded    []string     `json:"columns_added"`
	ColumnsRemoved  []string     `json:"columns_removed"`
	ColumnsModified []ColumnDiff `json:"columns_modified"`
}

// ColumnDiff represents differences in a column
type ColumnDiff struct {
	ColumnName string            `json:"column_name"`
	Changes    map[string]string `json:"changes"` // field -> old_value -> new_value
}

// EnumDiff represents differences in an enum
type EnumDiff struct {
	EnumName      string   `json:"enum_name"`
	ValuesAdded   []string `json:"values_added"`
	ValuesRemoved []string `json:"values_removed"`
}

// CompareSchemas compares a generated schema with a database schema
func CompareSchemas(generated *migratorlib.PackageParseResult, database *DatabaseSchema) *SchemaDiff {
	diff := &SchemaDiff{}

	// Compare tables
	compareTablesAndColumns(generated, database, diff)

	// Compare enums
	compareEnums(generated, database, diff)

	// Compare indexes
	compareIndexes(generated, database, diff)

	return diff
}

// compareTablesAndColumns compares tables and their columns
func compareTablesAndColumns(generated *migratorlib.PackageParseResult, database *DatabaseSchema, diff *SchemaDiff) {
	// Create maps for quick lookup
	genTables := make(map[string]types.TableDirective)
	for _, table := range generated.Tables {
		genTables[table.Name] = table
	}

	dbTables := make(map[string]Table)
	for _, table := range database.Tables {
		dbTables[table.Name] = table
	}

	// Find added and removed tables
	for tableName := range genTables {
		if _, exists := dbTables[tableName]; !exists {
			diff.TablesAdded = append(diff.TablesAdded, tableName)
		}
	}

	for tableName := range dbTables {
		if _, exists := genTables[tableName]; !exists {
			diff.TablesRemoved = append(diff.TablesRemoved, tableName)
		}
	}

	// Find modified tables (compare columns)
	for tableName, genTable := range genTables {
		if dbTable, exists := dbTables[tableName]; exists {
			tableDiff := compareTableColumns(genTable, dbTable, generated)
			if len(tableDiff.ColumnsAdded) > 0 || len(tableDiff.ColumnsRemoved) > 0 || len(tableDiff.ColumnsModified) > 0 {
				diff.TablesModified = append(diff.TablesModified, tableDiff)
			}
		}
	}

	// Sort for consistent output
	sort.Strings(diff.TablesAdded)
	sort.Strings(diff.TablesRemoved)
}

// compareTableColumns compares columns within a table
func compareTableColumns(genTable types.TableDirective, dbTable Table, generated *migratorlib.PackageParseResult) TableDiff {
	tableDiff := TableDiff{TableName: genTable.Name}

	// Create maps for quick lookup
	genColumns := make(map[string]types.SchemaField)
	for _, field := range generated.Fields {
		if field.StructName == genTable.StructName {
			genColumns[field.Name] = field
		}
	}

	dbColumns := make(map[string]Column)
	for _, col := range dbTable.Columns {
		dbColumns[col.Name] = col
	}

	// Find added and removed columns
	for colName := range genColumns {
		if _, exists := dbColumns[colName]; !exists {
			tableDiff.ColumnsAdded = append(tableDiff.ColumnsAdded, colName)
		}
	}

	for colName := range dbColumns {
		if _, exists := genColumns[colName]; !exists {
			tableDiff.ColumnsRemoved = append(tableDiff.ColumnsRemoved, colName)
		}
	}

	// Find modified columns
	for colName, genCol := range genColumns {
		if dbCol, exists := dbColumns[colName]; exists {
			colDiff := compareColumns(genCol, dbCol)
			if len(colDiff.Changes) > 0 {
				tableDiff.ColumnsModified = append(tableDiff.ColumnsModified, colDiff)
			}
		}
	}

	// Sort for consistent output
	sort.Strings(tableDiff.ColumnsAdded)
	sort.Strings(tableDiff.ColumnsRemoved)

	return tableDiff
}

// compareColumns compares individual column properties
func compareColumns(genCol types.SchemaField, dbCol Column) ColumnDiff {
	colDiff := ColumnDiff{
		ColumnName: genCol.Name,
		Changes:    make(map[string]string),
	}

	// Compare data types (simplified)
	genType := normalizeType(genCol.Type)
	dbType := normalizeType(dbCol.DataType)
	if dbCol.UDTName != "" {
		dbType = normalizeType(dbCol.UDTName)
	}

	if genType != dbType {
		colDiff.Changes["type"] = fmt.Sprintf("%s -> %s", dbType, genType)
	}

	// Compare nullable
	genNullable := genCol.Nullable
	dbNullable := dbCol.IsNullable == "YES"
	if genNullable != dbNullable {
		colDiff.Changes["nullable"] = fmt.Sprintf("%t -> %t", dbNullable, genNullable)
	}

	// Compare primary key
	genPrimary := genCol.Primary
	dbPrimary := dbCol.IsPrimaryKey
	if genPrimary != dbPrimary {
		colDiff.Changes["primary_key"] = fmt.Sprintf("%t -> %t", dbPrimary, genPrimary)
	}

	// Compare unique
	genUnique := genCol.Unique
	dbUnique := dbCol.IsUnique
	if genUnique != dbUnique {
		colDiff.Changes["unique"] = fmt.Sprintf("%t -> %t", dbUnique, genUnique)
	}

	// Compare default values (simplified)
	genDefault := genCol.Default
	dbDefault := ""
	if dbCol.ColumnDefault != nil {
		dbDefault = *dbCol.ColumnDefault
	}
	if genDefault != dbDefault {
		colDiff.Changes["default"] = fmt.Sprintf("'%s' -> '%s'", dbDefault, genDefault)
	}

	return colDiff
}

// compareEnums compares enum types
func compareEnums(generated *migratorlib.PackageParseResult, database *DatabaseSchema, diff *SchemaDiff) {
	// Create maps for quick lookup
	genEnums := make(map[string]types.GlobalEnum)
	for _, enum := range generated.Enums {
		genEnums[enum.Name] = enum
	}

	dbEnums := make(map[string]Enum)
	for _, enum := range database.Enums {
		dbEnums[enum.Name] = enum
	}

	// Find added and removed enums
	for enumName := range genEnums {
		if _, exists := dbEnums[enumName]; !exists {
			diff.EnumsAdded = append(diff.EnumsAdded, enumName)
		}
	}

	for enumName := range dbEnums {
		if _, exists := genEnums[enumName]; !exists {
			diff.EnumsRemoved = append(diff.EnumsRemoved, enumName)
		}
	}

	// Find modified enums
	for enumName, genEnum := range genEnums {
		if dbEnum, exists := dbEnums[enumName]; exists {
			enumDiff := compareEnumValues(genEnum, dbEnum)
			if len(enumDiff.ValuesAdded) > 0 || len(enumDiff.ValuesRemoved) > 0 {
				diff.EnumsModified = append(diff.EnumsModified, enumDiff)
			}
		}
	}

	// Sort for consistent output
	sort.Strings(diff.EnumsAdded)
	sort.Strings(diff.EnumsRemoved)
}

// compareEnumValues compares enum values
func compareEnumValues(genEnum types.GlobalEnum, dbEnum Enum) EnumDiff {
	enumDiff := EnumDiff{EnumName: genEnum.Name}

	// Create sets for comparison
	genValues := make(map[string]bool)
	for _, value := range genEnum.Values {
		genValues[value] = true
	}

	dbValues := make(map[string]bool)
	for _, value := range dbEnum.Values {
		dbValues[value] = true
	}

	// Find added and removed values
	for value := range genValues {
		if !dbValues[value] {
			enumDiff.ValuesAdded = append(enumDiff.ValuesAdded, value)
		}
	}

	for value := range dbValues {
		if !genValues[value] {
			enumDiff.ValuesRemoved = append(enumDiff.ValuesRemoved, value)
		}
	}

	// Sort for consistent output
	sort.Strings(enumDiff.ValuesAdded)
	sort.Strings(enumDiff.ValuesRemoved)

	return enumDiff
}

// compareIndexes compares indexes (simplified)
func compareIndexes(generated *migratorlib.PackageParseResult, database *DatabaseSchema, diff *SchemaDiff) {
	// Create sets for comparison
	genIndexes := make(map[string]bool)
	for _, index := range generated.Indexes {
		genIndexes[index.Name] = true
	}

	dbIndexes := make(map[string]bool)
	for _, index := range database.Indexes {
		// Skip primary key indexes as they're handled with tables
		if !index.IsPrimary {
			dbIndexes[index.Name] = true
		}
	}

	// Find added and removed indexes
	for indexName := range genIndexes {
		if !dbIndexes[indexName] {
			diff.IndexesAdded = append(diff.IndexesAdded, indexName)
		}
	}

	for indexName := range dbIndexes {
		if !genIndexes[indexName] {
			diff.IndexesRemoved = append(diff.IndexesRemoved, indexName)
		}
	}

	// Sort for consistent output
	sort.Strings(diff.IndexesAdded)
	sort.Strings(diff.IndexesRemoved)
}

// normalizeType normalizes type names for comparison
func normalizeType(typeName string) string {
	// Convert common type variations to standard forms
	typeName = strings.ToLower(typeName)

	switch {
	case strings.Contains(typeName, "varchar"):
		return "varchar"
	case strings.Contains(typeName, "text"):
		return "text"
	case strings.Contains(typeName, "serial"):
		return "integer"
	case strings.Contains(typeName, "int"):
		return "integer"
	case strings.Contains(typeName, "bool"):
		return "boolean"
	case strings.Contains(typeName, "timestamp"):
		return "timestamp"
	case strings.Contains(typeName, "decimal") || strings.Contains(typeName, "numeric"):
		return "decimal"
	default:
		return typeName
	}
}

// HasChanges returns true if the diff contains any changes
func (d *SchemaDiff) HasChanges() bool {
	return len(d.TablesAdded) > 0 ||
		len(d.TablesRemoved) > 0 ||
		len(d.TablesModified) > 0 ||
		len(d.EnumsAdded) > 0 ||
		len(d.EnumsRemoved) > 0 ||
		len(d.EnumsModified) > 0 ||
		len(d.IndexesAdded) > 0 ||
		len(d.IndexesRemoved) > 0
}

// GenerateMigrationSQL generates SQL statements to apply the schema differences
func (d *SchemaDiff) GenerateMigrationSQL(generated *migratorlib.PackageParseResult, dialect string) []string {
	var statements []string

	// 1. Add new enums first
	for _, enumName := range d.EnumsAdded {
		for _, enum := range generated.Enums {
			if enum.Name == enumName {
				if dialect == "postgres" {
					values := make([]string, len(enum.Values))
					for i, v := range enum.Values {
						values[i] = "'" + v + "'"
					}
					sql := fmt.Sprintf("CREATE TYPE %s AS ENUM (%s);", enum.Name, strings.Join(values, ", "))
					statements = append(statements, sql)
				}
				break
			}
		}
	}

	// 2. Modify existing enums (add values only - PostgreSQL doesn't support removing enum values easily)
	for _, enumDiff := range d.EnumsModified {
		if dialect == "postgres" {
			for _, value := range enumDiff.ValuesAdded {
				sql := fmt.Sprintf("ALTER TYPE %s ADD VALUE '%s';", enumDiff.EnumName, value)
				statements = append(statements, sql)
			}
			// Note: PostgreSQL doesn't support removing enum values without recreating the enum
			if len(enumDiff.ValuesRemoved) > 0 {
				statements = append(statements, fmt.Sprintf("-- WARNING: Cannot remove enum values %v from %s without recreating the enum", enumDiff.ValuesRemoved, enumDiff.EnumName))
			}
		}
	}

	// 3. Add new tables
	for _, tableName := range d.TablesAdded {
		// Find the table in generated schema and create it
		for _, table := range generated.Tables {
			if table.Name == tableName {
				// Get the create statement for this table
				statements = append(statements, fmt.Sprintf("-- Create table: %s", tableName))
				// Note: This would need the full table creation logic
				// For now, we'll add a placeholder
				statements = append(statements, fmt.Sprintf("-- TODO: CREATE TABLE %s (...);", tableName))
				break
			}
		}
	}

	// 4. Modify existing tables
	for _, tableDiff := range d.TablesModified {
		statements = append(statements, fmt.Sprintf("-- Modify table: %s", tableDiff.TableName))

		// Add new columns
		for _, colName := range tableDiff.ColumnsAdded {
			statements = append(statements, fmt.Sprintf("-- TODO: ALTER TABLE %s ADD COLUMN %s ...;", tableDiff.TableName, colName))
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
	for _, indexName := range d.IndexesAdded {
		statements = append(statements, fmt.Sprintf("-- TODO: CREATE INDEX %s ON ...;", indexName))
	}

	// 6. Remove indexes
	for _, indexName := range d.IndexesRemoved {
		statements = append(statements, fmt.Sprintf("DROP INDEX IF EXISTS %s;", indexName))
	}

	// 7. Remove tables (dangerous!)
	for _, tableName := range d.TablesRemoved {
		statements = append(statements, fmt.Sprintf("-- WARNING: DROP TABLE %s; -- This will delete all data!", tableName))
	}

	// 8. Remove enums (dangerous!)
	for _, enumName := range d.EnumsRemoved {
		statements = append(statements, fmt.Sprintf("-- WARNING: DROP TYPE %s; -- Make sure no tables use this enum!", enumName))
	}

	return statements
}
