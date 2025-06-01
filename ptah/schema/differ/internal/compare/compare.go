package compare

import (
	"fmt"
	"sort"
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/schema/differ/differtypes"
	"github.com/denisvmedia/inventario/ptah/schema/differ/internal/normalize"
	"github.com/denisvmedia/inventario/ptah/schema/transform"
)

// TablesAndColumns performs comprehensive table and column comparison between generated and database schemas.
//
// This function is the core table comparison engine that identifies structural differences
// between the target schema (from Go struct annotations) and the current database schema.
// It handles table additions, removals, and detailed column-level modifications.
//
// # Comparison Process
//
// The function performs comparison in three phases:
//  1. **Table Discovery**: Creates lookup maps for efficient table comparison
//  2. **Table Diff Analysis**: Identifies added and removed tables
//  3. **Column Comparison**: For existing tables, performs detailed column analysis
//
// # Algorithm Complexity
//
// - Time Complexity: O(n + m + k) where n=generated tables, m=database tables, k=total columns
// - Space Complexity: O(n + m) for lookup maps
// - Optimized for large schemas with efficient map-based lookups
//
// # Embedded Field Handling
//
// The function properly handles embedded fields by delegating to TableColumns(),
// which processes embedded fields through the transform package to ensure generated
// fields are correctly compared against database columns.
//
// # Example Scenarios
//
// **New table detection**:
//   - Generated schema has "users" table
//   - Database schema doesn't have "users" table
//   - Result: "users" added to diff.TablesAdded
//
// **Removed table detection**:
//   - Database has "legacy_data" table
//   - Generated schema doesn't define "legacy_data"
//   - Result: "legacy_data" added to diff.TablesRemoved
//
// **Modified table detection**:
//   - Both schemas have "products" table
//   - Column structures differ (new columns, type changes, etc.)
//   - Result: TableDiff added to diff.TablesModified
//
// # Parameters
//
//   - generated: Target schema parsed from Go struct annotations
//   - database: Current database schema from executor introspection
//   - diff: SchemaDiff structure to populate with discovered differences
//
// # Side Effects
//
// Modifies the provided diff parameter by populating:
//   - diff.TablesAdded: Tables that need to be created
//   - diff.TablesRemoved: Tables that exist in database but not in target schema
//   - diff.TablesModified: Tables with structural differences
//
// # Output Consistency
//
// Results are sorted alphabetically for consistent output across multiple runs,
// ensuring deterministic migration generation and reliable testing.
func TablesAndColumns(generated *goschema.Database, database *dbschematypes.DBSchema, diff *differtypes.SchemaDiff) {
	// Create maps for quick lookup
	genTables := make(map[string]goschema.Table)
	for _, table := range generated.Tables {
		genTables[table.Name] = table
	}

	dbTables := make(map[string]dbschematypes.DBTable)
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
			tableDiff := TableColumns(genTable, dbTable, generated)
			if len(tableDiff.ColumnsAdded) > 0 || len(tableDiff.ColumnsRemoved) > 0 || len(tableDiff.ColumnsModified) > 0 {
				diff.TablesModified = append(diff.TablesModified, tableDiff)
			}
		}
	}

	// Sort for consistent output
	sort.Strings(diff.TablesAdded)
	sort.Strings(diff.TablesRemoved)
}

// TableColumns performs detailed column-level comparison within a specific table.
//
// This function is responsible for the complex task of comparing column structures
// between a generated table definition and an existing database table. It handles
// embedded field processing, column mapping, and detailed property comparison.
//
// # Embedded Field Processing
//
// The function's most complex aspect is handling embedded fields:
//  1. **Field Expansion**: Uses transform.ProcessEmbeddedFields() to expand embedded structs
//  2. **Field Combination**: Merges original fields with embedded-generated fields
//  3. **Struct Filtering**: Only processes fields belonging to the target struct
//
// This ensures that embedded fields (like timestamps, audit info) are properly
// compared against their corresponding database columns.
//
// # Comparison Algorithm
//
// The function performs comparison in three phases:
//  1. **Column Discovery**: Creates lookup maps for efficient column comparison
//  2. **Addition/Removal Detection**: Identifies new and removed columns
//  3. **Modification Analysis**: Compares properties of existing columns
//
// # Example Scenarios
//
// **Embedded field handling**:
//
//	```go
//	type User struct {
//	    ID   int    `db:"id"`
//	    Name string `db:"name"`
//	    Timestamps // Embedded struct with CreatedAt, UpdatedAt
//	}
//	```
//	The function expands Timestamps fields and compares them against database columns.
//
// **Column addition detection**:
//   - Generated schema has "email" column
//   - Database table doesn't have "email" column
//   - Result: "email" added to TableDiff.ColumnsAdded
//
// **Column modification detection**:
//   - Both have "name" column
//   - Generated: VARCHAR(255), Database: VARCHAR(100)
//   - Result: ColumnDiff added to TableDiff.ColumnsModified
//
// # Parameters
//
//   - genTable: Generated table definition from Go struct annotations
//   - dbTable: Current database table structure from introspection
//   - generated: Complete parse result containing all fields and embedded field definitions
//
// # Return Value
//
// Returns a TableDiff containing:
//   - ColumnsAdded: New columns that need to be added
//   - ColumnsRemoved: Existing columns that should be removed
//   - ColumnsModified: Columns with property differences
//
// # Performance Considerations
//
// - Time Complexity: O(n + m + k) where n=generated columns, m=database columns, k=embedded fields
// - Space Complexity: O(n + m) for lookup maps
// - Embedded field processing adds overhead but is necessary for accurate comparison
//
// # Output Consistency
//
// Column lists are sorted alphabetically for deterministic output and reliable testing.
func TableColumns(genTable goschema.Table, dbTable dbschematypes.DBTable, generated *goschema.Database) differtypes.TableDiff {
	tableDiff := differtypes.TableDiff{TableName: genTable.Name}

	// Process embedded fields to get the complete field list (same as generators do)
	embeddedGeneratedFields := transform.ProcessEmbeddedFields(generated.EmbeddedFields, generated.Fields, genTable.StructName)

	// Combine original fields with embedded-generated fields
	allFields := append(generated.Fields, embeddedGeneratedFields...)

	// Create maps for quick lookup
	genColumns := make(map[string]goschema.Field)
	for _, field := range allFields {
		if field.StructName == genTable.StructName {
			genColumns[field.Name] = field
		}
	}

	dbColumns := make(map[string]dbschematypes.DBColumn)
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
			colDiff := Columns(genCol, dbCol)
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

// Columns performs detailed property-level comparison between a generated column and database column.
//
// This function is the most granular level of schema comparison, analyzing individual
// column properties to detect differences that require migration. It handles complex
// cross-database type normalization and property comparison logic.
//
// # Property Comparison Categories
//
// The function compares five main categories of column properties:
//  1. **Data Types**: Handles cross-database type normalization and comparison
//  2. **Nullability**: Considers primary key implications and explicit nullable settings
//  3. **Primary Key**: Compares primary key constraint status
//  4. **Uniqueness**: Compares unique constraint status
//  5. **Default Values**: Handles auto-increment special cases and type-specific normalization
//
// # Complex Logic Areas
//
// **Type Normalization**:
//   - Uses Type() to handle cross-database type variations
//   - Considers both DataType and UDTName from database introspection
//   - Handles PostgreSQL user-defined types vs standard types
//
// **Nullability Logic**:
//   - Primary key columns are always NOT NULL regardless of field definition
//   - Explicit nullable settings override default behavior
//   - Database "YES"/"NO" strings converted to boolean for comparison
//
// **Auto-increment Handling**:
//   - SERIAL columns have special default value handling
//   - Database shows sequence defaults, but entities expect empty defaults
//   - Prevents false positives for auto-increment columns
//
// # Example Comparisons
//
// **Type difference detection**:
//
//	```
//	Generated: VARCHAR(255)
//	Database:  VARCHAR(100)
//	Result:    Changes["type"] = "varchar -> varchar" (normalized)
//	```
//
// **Nullability change**:
//
//	```
//	Generated: nullable=false
//	Database:  nullable=true
//	Result:    Changes["nullable"] = "true -> false"
//	```
//
// **Primary key promotion**:
//
//	```
//	Generated: primary=true
//	Database:  primary=false
//	Result:    Changes["primary_key"] = "false -> true"
//	```
//
// **Default value normalization**:
//
//	```
//	Generated: default=""
//	Database:  default_expr="NULL"
//	Result:    No change (both normalize to empty string)
//	```
//
// # Parameters
//
//   - genCol: Generated column definition from Go struct field
//   - dbCol: Current database column from introspection
//
// # Return Value
//
// Returns a ColumnDiff with:
//   - ColumnName: Name of the column being compared
//   - Changes: Map of property changes in "old -> new" format
//
// # Cross-Database Considerations
//
// The function handles database-specific variations:
//   - **PostgreSQL**: UDT names, SERIAL types, native boolean types
//   - **MySQL/MariaDB**: TINYINT boolean representation, AUTO_INCREMENT
//   - **Type mapping**: Intelligent normalization for accurate comparison
func Columns(genCol goschema.Field, dbCol dbschematypes.DBColumn) differtypes.ColumnDiff {
	colDiff := differtypes.ColumnDiff{
		ColumnName: genCol.Name,
		Changes:    make(map[string]string),
	}

	// Compare data types (simplified)
	genType := normalize.Type(genCol.Type)
	dbType := normalize.Type(dbCol.DataType)
	if dbCol.UDTName != "" {
		dbType = normalize.Type(dbCol.UDTName)
	}

	if genType != dbType {
		colDiff.Changes["type"] = fmt.Sprintf("%s -> %s", dbType, genType)
	}

	// Compare nullable (primary keys are always NOT NULL regardless of the field definition)
	genNullable := genCol.Nullable
	if genCol.Primary {
		genNullable = false // Primary keys are always NOT NULL
	}
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
	if genDefault == "" {
		genDefault = genCol.DefaultExpr
	}
	dbDefault := ""
	if dbCol.ColumnDefault != nil {
		dbDefault = *dbCol.ColumnDefault
	}

	// For auto-increment/SERIAL columns, ignore default value differences
	// because the database will show the sequence default but the entity expects empty
	isAutoIncrement := dbCol.IsAutoIncrement || strings.Contains(strings.ToUpper(genCol.Type), "SERIAL")
	if !isAutoIncrement {
		normalizedDbDefault := normalize.DefaultValue(dbDefault, dbType)

		idxName := "default"
		if normalize.IsDefaultExpr(dbDefault) {
			idxName = "default_expr"
		}

		normalizeGenDefaultFn := normalize.DefaultValue(genDefault, "")

		if normalizeGenDefaultFn != normalizedDbDefault {
			colDiff.Changes[idxName] = fmt.Sprintf("%s -> %s", dbDefault, genDefault)
		}
	}

	return colDiff
}

// ColumnByName searches for a specific column difference by name within a slice of column diffs.
//
// This utility function provides efficient lookup of column differences by name, which is
// commonly needed when processing migration results or analyzing specific column changes.
// It performs a linear search through the provided slice and returns a pointer to the
// first matching ColumnDiff.
//
// # Search Algorithm
//
// The function uses a simple linear search with O(n) time complexity:
//  1. **Iteration**: Loops through each ColumnDiff in the provided slice
//  2. **Name Matching**: Compares ColumnName field with the target column name
//  3. **Early Return**: Returns immediately upon finding the first match
//  4. **Not Found**: Returns nil if no matching column is found
//
// # Use Cases
//
// **Migration Analysis**:
//   - Check if a specific column has changes before generating migration SQL
//   - Retrieve detailed change information for a particular column
//   - Validate expected changes in automated tests
//
// **Conditional Processing**:
//   - Apply different migration strategies based on specific column changes
//   - Skip certain operations if particular columns are not modified
//   - Generate warnings for potentially dangerous column modifications
//
// # Example Usage
//
// **Finding a specific column change**:
//
//	```go
//	tableDiff := compare.TableColumns(genTable, dbTable, generated)
//	emailDiff := compare.ColumnByName(tableDiff.ColumnsModified, "email")
//	if emailDiff != nil {
//	    if _, hasTypeChange := emailDiff.Changes["type"]; hasTypeChange {
//	        log.Println("Email column type is changing")
//	    }
//	}
//	```
//
// **Validation in tests**:
//
//	```go
//	result := compare.TableColumns(genTable, dbTable, generated)
//	nameDiff := compare.ColumnByName(result.ColumnsModified, "name")
//	assert.NotNil(t, nameDiff)
//	assert.Equal(t, "varchar -> text", nameDiff.Changes["type"])
//	```
//
// **Conditional migration logic**:
//
//	```go
//	for _, tableDiff := range schemaDiff.TablesModified {
//	    if pkDiff := compare.ColumnByName(tableDiff.ColumnsModified, "id"); pkDiff != nil {
//	        if _, hasPKChange := pkDiff.Changes["primary_key"]; hasPKChange {
//	            // Handle primary key changes with special care
//	            generatePrimaryKeyMigration(tableDiff.TableName, pkDiff)
//	        }
//	    }
//	}
//	```
//
// # Parameters
//
//   - diffs: Slice of ColumnDiff structures to search through
//   - columnName: Name of the column to find (case-sensitive exact match)
//
// # Return Value
//
// Returns a pointer to the first ColumnDiff with matching ColumnName, or nil if not found.
// The returned pointer references the original ColumnDiff in the slice, so modifications
// will affect the original data structure.
//
// # Performance Considerations
//
// - Time Complexity: O(n) where n is the number of column diffs
// - Space Complexity: O(1) - no additional memory allocation
// - For large numbers of columns, consider using a map-based lookup if called frequently
//
// # Thread Safety
//
// This function is read-only and thread-safe when used concurrently on the same data.
// However, if the underlying slice is being modified concurrently, appropriate
// synchronization is required.
//
// # Edge Cases
//
// - Empty slice: Returns nil immediately
// - Nil slice: Returns nil immediately (no panic)
// - Duplicate column names: Returns the first match encountered
// - Case sensitivity: Performs exact string matching (case-sensitive)
func ColumnByName(diffs []differtypes.ColumnDiff, columnName string) *differtypes.ColumnDiff {
	for _, diff := range diffs {
		if diff.ColumnName == columnName {
			return &diff
		}
	}
	return nil
}

// Enums performs comprehensive enum type comparison between generated and database schemas.
//
// This function handles the comparison of enum type definitions, which is particularly
// complex due to database-specific enum implementations and the challenges of enum
// value modification across different database systems.
//
// # Database-Specific Enum Handling
//
// **PostgreSQL**:
//   - Native ENUM types with CREATE TYPE statements
//   - Supports adding enum values but not removing them easily
//   - Enum values are stored in system catalogs
//
// **MySQL/MariaDB**:
//   - Inline ENUM syntax in column definitions
//   - Supports both adding and removing enum values
//   - Enum values are part of column type definition
//
// **SQLite**:
//   - No native enum support
//   - Uses CHECK constraints for enum-like behavior
//
// # Comparison Algorithm
//
// The function performs comparison in three phases:
//  1. **Enum Discovery**: Creates lookup maps for efficient enum comparison
//  2. **Addition/Removal Detection**: Identifies new and removed enum types
//  3. **Value Modification Analysis**: Compares enum values for existing types
//
// # Example Scenarios
//
// **New enum detection**:
//   - Generated schema defines "status_type" enum
//   - Database doesn't have "status_type" enum
//   - Result: "status_type" added to diff.EnumsAdded
//
// **Enum value addition**:
//   - Both have "priority_level" enum
//   - Generated: ["low", "medium", "high", "critical"]
//   - Database: ["low", "medium", "high"]
//   - Result: EnumDiff with ValuesAdded=["critical"]
//
// **Enum value removal** (problematic):
//   - Generated: ["active", "inactive"]
//   - Database: ["active", "inactive", "deprecated"]
//   - Result: EnumDiff with ValuesRemoved=["deprecated"]
//   - Note: May require manual intervention in PostgreSQL
//
// # Parameters
//
//   - generated: Target schema parsed from Go struct annotations
//   - database: Current database schema from executor introspection
//   - diff: SchemaDiff structure to populate with discovered differences
//
// # Side Effects
//
// Modifies the provided diff parameter by populating:
//   - diff.EnumsAdded: Enum types that need to be created
//   - diff.EnumsRemoved: Enum types that exist in database but not in target schema
//   - diff.EnumsModified: Enum types with value differences
//
// # Migration Considerations
//
// Enum modifications can be complex:
//   - Adding values is generally safe
//   - Removing values may require data migration
//   - PostgreSQL enum removal requires recreating the enum type
//   - MySQL enum changes require ALTER TABLE statements
//
// # Output Consistency
//
// Results are sorted alphabetically for consistent output across multiple runs.
func Enums(generated *goschema.Database, database *dbschematypes.DBSchema, diff *differtypes.SchemaDiff) {
	// Create maps for quick lookup
	genEnums := make(map[string]goschema.Enum)
	for _, enum := range generated.Enums {
		genEnums[enum.Name] = enum
	}

	dbEnums := make(map[string]dbschematypes.DBEnum)
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
			enumDiff := EnumValues(genEnum, dbEnum)
			if len(enumDiff.ValuesAdded) > 0 || len(enumDiff.ValuesRemoved) > 0 {
				diff.EnumsModified = append(diff.EnumsModified, enumDiff)
			}
		}
	}

	// Sort for consistent output
	sort.Strings(diff.EnumsAdded)
	sort.Strings(diff.EnumsRemoved)
}

// EnumValues performs detailed value-level comparison between generated and database enum types.
//
// This function analyzes the specific values within an enum type to determine what
// changes are needed to bring the database enum in line with the generated enum
// definition. It uses set-based comparison for efficient value difference detection.
//
// # Algorithm Details
//
// The function uses a set-based approach for optimal performance:
//  1. **Set Creation**: Converts value slices to boolean maps for O(1) lookup
//  2. **Addition Detection**: Finds values in generated enum but not in database
//  3. **Removal Detection**: Finds values in database enum but not in generated
//  4. **Result Sorting**: Ensures deterministic output for consistent migrations
//
// # Example Scenarios
//
// **Value addition**:
//
//	```
//	Generated: ["draft", "published", "archived", "deleted"]
//	Database:  ["draft", "published", "archived"]
//	Result:    ValuesAdded=["deleted"], ValuesRemoved=[]
//	```
//
// **Value removal**:
//
//	```
//	Generated: ["active", "inactive"]
//	Database:  ["active", "inactive", "deprecated", "legacy"]
//	Result:    ValuesAdded=[], ValuesRemoved=["deprecated", "legacy"]
//	```
//
// **Mixed changes**:
//
//	```
//	Generated: ["pending", "approved", "rejected", "cancelled"]
//	Database:  ["pending", "approved", "denied"]
//	Result:    ValuesAdded=["rejected", "cancelled"], ValuesRemoved=["denied"]
//	```
//
// # Performance Characteristics
//
// - Time Complexity: O(n + m) where n=generated values, m=database values
// - Space Complexity: O(n + m) for the boolean maps
// - Optimized for large enum value sets with efficient set operations
//
// # Parameters
//
//   - genEnum: Generated enum definition from Go struct annotations
//   - dbEnum: Current database enum from introspection
//
// # Return Value
//
// Returns an EnumDiff containing:
//   - EnumName: Name of the enum being compared
//   - ValuesAdded: Values that need to be added to the database enum
//   - ValuesRemoved: Values that exist in database but not in generated enum
//
// # Migration Implications
//
// **Adding values**: Generally safe operation across all databases
// **Removing values**: May require careful consideration:
//   - Check if removed values are used in existing data
//   - PostgreSQL requires enum recreation for value removal
//   - MySQL allows value removal but may affect existing data
//
// # Output Consistency
//
// Value lists are sorted alphabetically to ensure deterministic migration
// generation and reliable testing across multiple runs.
func EnumValues(genEnum goschema.Enum, dbEnum dbschematypes.DBEnum) differtypes.EnumDiff {
	enumDiff := differtypes.EnumDiff{EnumName: genEnum.Name}

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

// Indexes performs index comparison between generated and database schemas with intelligent filtering.
//
// This function handles the comparison of database indexes, which requires careful
// filtering to avoid false positives from automatically generated indexes (primary
// keys, unique constraints) that are managed by the database system rather than
// explicitly defined in the schema.
//
// # Index Filtering Logic
//
// The function applies intelligent filtering to focus on user-defined indexes:
//
// **Generated Schema Indexes**:
//   - Includes all explicitly defined indexes from Go struct annotations
//   - These are indexes the developer intentionally created for performance
//
// **Database Schema Indexes**:
//   - Excludes primary key indexes (automatically created with PRIMARY KEY constraints)
//   - Excludes unique indexes (automatically created with UNIQUE constraints)
//   - Includes only manually created performance indexes
//
// This filtering prevents false positives where the system would suggest removing
// automatically generated indexes that are essential for constraint enforcement.
//
// # Example Scenarios
//
// **Performance index addition**:
//
//	```go
//	type User struct {
//	    Email string `db:"email" index:"idx_users_email"`
//	}
//	```
//	- Generated schema defines "idx_users_email"
//	- Database doesn't have this index
//	- Result: "idx_users_email" added to diff.IndexesAdded
//
// **Unused index removal**:
//   - Database has "idx_old_search" index
//   - Generated schema doesn't define this index
//   - Result: "idx_old_search" added to diff.IndexesRemoved
//
// **Automatic index filtering**:
//   - Database has "users_pkey" (primary key index)
//   - Database has "users_email_key" (unique constraint index)
//   - These are filtered out and not considered for removal
//
// # Algorithm Details
//
// 1. **Set Creation**: Converts index lists to boolean maps for O(1) lookup
// 2. **Filtering**: Applies database-side filtering for automatic indexes
// 3. **Comparison**: Performs set difference operations to find additions/removals
// 4. **Sorting**: Ensures deterministic output for consistent migrations
//
// # Parameters
//
//   - generated: Target schema parsed from Go struct annotations
//   - database: Current database schema from executor introspection
//   - diff: SchemaDiff structure to populate with discovered differences
//
// # Side Effects
//
// Modifies the provided diff parameter by populating:
//   - diff.IndexesAdded: Indexes that need to be created
//   - diff.IndexesRemoved: User-defined indexes that can be safely removed
//
// # Safety Considerations
//
// Index operations are generally safe:
//   - Adding indexes improves performance but doesn't affect data
//   - Removing indexes may impact query performance but doesn't cause data loss
//   - Primary key and unique constraint indexes are protected from removal
//
// # Performance Impact
//
// - Time Complexity: O(n + m) where n=generated indexes, m=database indexes
// - Space Complexity: O(n + m) for the boolean maps
// - Index operations can be expensive on large tables in production
func Indexes(generated *goschema.Database, database *dbschematypes.DBSchema, diff *differtypes.SchemaDiff) {
	// Create sets for comparison
	genIndexes := make(map[string]bool)
	for _, index := range generated.Indexes {
		genIndexes[index.Name] = true
	}

	dbIndexes := make(map[string]bool)
	for _, index := range database.Indexes {
		// Skip primary key indexes as they're handled with tables
		// Skip unique indexes as they're automatically created by UNIQUE constraints
		if !index.IsPrimary && !index.IsUnique {
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
