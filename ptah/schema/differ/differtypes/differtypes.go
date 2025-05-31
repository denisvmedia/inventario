package differtypes

// SchemaDiff represents comprehensive differences between two database schemas.
//
// This structure captures all types of schema changes that can occur between a target
// schema (generated from Go struct annotations) and an existing database schema.
// It provides a complete picture of what needs to be modified to bring the database
// schema in line with the application's expected schema.
//
// # Structure Organization
//
// The diff is organized by database object type for clear categorization:
//   - Tables: New, removed, and modified table structures
//   - Enums: New, removed, and modified enum types
//   - Indexes: New and removed database indexes
//
// # JSON Serialization
//
// All fields are JSON-serializable for integration with external tools,
// CI/CD pipelines, and migration management systems.
//
// # Example Usage
//
//	diff := &SchemaDiff{
//		TablesAdded: []string{"users", "posts"},
//		TablesModified: []TableDiff{
//			{TableName: "products", ColumnsAdded: []string{"price", "category"}},
//		},
//		EnumsAdded: []string{"status_type"},
//	}
//
//	if diff.HasChanges() {
//		fmt.Printf("Found %d new tables\n", len(diff.TablesAdded))
//	}
type SchemaDiff struct {
	// TablesAdded contains names of tables that exist in the target schema
	// but not in the current database schema
	TablesAdded []string `json:"tables_added"`

	// TablesRemoved contains names of tables that exist in the current database
	// but not in the target schema (potentially dangerous - data loss)
	TablesRemoved []string `json:"tables_removed"`

	// TablesModified contains detailed information about tables that exist in both
	// schemas but have structural differences (columns, constraints, etc.)
	TablesModified []TableDiff `json:"tables_modified"`

	// EnumsAdded contains names of enum types that exist in the target schema
	// but not in the current database schema
	EnumsAdded []string `json:"enums_added"`

	// EnumsRemoved contains names of enum types that exist in the current database
	// but not in the target schema (potentially dangerous - may break existing data)
	EnumsRemoved []string `json:"enums_removed"`

	// EnumsModified contains detailed information about enum types that exist in both
	// schemas but have different values (additions/removals)
	EnumsModified []EnumDiff `json:"enums_modified"`

	// IndexesAdded contains names of indexes that exist in the target schema
	// but not in the current database schema
	IndexesAdded []string `json:"indexes_added"`

	// IndexesRemoved contains names of indexes that exist in the current database
	// but not in the target schema (safe operation - no data loss)
	IndexesRemoved []string `json:"indexes_removed"`
}

// HasChanges returns true if the diff contains any schema changes requiring migration.
//
// This method provides a quick way to determine if any migration actions are needed
// without having to check each individual diff category. It's commonly used in
// CI/CD pipelines and automated deployment systems to decide whether to generate
// and apply migrations.
//
// # Return Value
//
// Returns true if any of the following conditions are met:
//   - New tables need to be created
//   - Existing tables need to be removed
//   - Existing tables have structural modifications
//   - New enum types need to be created
//   - Existing enum types need to be removed
//   - Existing enum types have value modifications
//   - New indexes need to be created
//   - Existing indexes need to be removed
//
// # Example Usage
//
//	diff := CompareSchemas(generated, database)
//	if diff.HasChanges() {
//		log.Println("Schema changes detected, generating migration...")
//		statements := diff.GenerateMigrationSQL(generated, "postgres")
//		// Apply migration statements...
//	} else {
//		log.Println("No schema changes detected")
//	}
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

// TableDiff represents structural differences within a specific database table.
//
// This structure captures all types of changes that can occur to a table's structure,
// including column additions, removals, and modifications. It provides detailed
// information needed to generate appropriate ALTER TABLE statements.
//
// # Example Usage
//
//	tableDiff := TableDiff{
//		TableName: "users",
//		ColumnsAdded: []string{"email", "created_at"},
//		ColumnsRemoved: []string{"legacy_field"},
//		ColumnsModified: []ColumnDiff{
//			{ColumnName: "name", Changes: map[string]string{"type": "VARCHAR(100) -> VARCHAR(255)"}},
//		},
//	}
type TableDiff struct {
	// TableName is the name of the table being modified
	TableName string `json:"table_name"`

	// ColumnsAdded contains names of columns that need to be added to the table
	ColumnsAdded []string `json:"columns_added"`

	// ColumnsRemoved contains names of columns that need to be removed from the table
	// (potentially dangerous - may cause data loss)
	ColumnsRemoved []string `json:"columns_removed"`

	// ColumnsModified contains detailed information about columns that exist in both
	// schemas but have different properties (type, constraints, defaults, etc.)
	ColumnsModified []ColumnDiff `json:"columns_modified"`
}

// ColumnDiff represents specific property changes within a database column.
//
// This structure captures the detailed differences between the current column
// definition and the target column definition. Each change is represented as
// a key-value pair showing the transition from old value to new value.
//
// # Change Types
//
// Common change types include:
//   - "type": Data type changes (e.g., "VARCHAR(100) -> VARCHAR(255)")
//   - "nullable": Nullability changes (e.g., "true -> false")
//   - "primary_key": Primary key constraint changes (e.g., "false -> true")
//   - "unique": Unique constraint changes (e.g., "false -> true")
//   - "default": Default value changes (e.g., "'old' -> 'new'")
//
// # Example Usage
//
//	columnDiff := ColumnDiff{
//		ColumnName: "email",
//		Changes: map[string]string{
//			"type": "VARCHAR(100) -> VARCHAR(255)",
//			"nullable": "true -> false",
//			"unique": "false -> true",
//		},
//	}
type ColumnDiff struct {
	// ColumnName is the name of the column being modified
	ColumnName string `json:"column_name"`

	// Changes maps change types to their old->new value transitions
	// Format: "change_type" -> "old_value -> new_value"
	Changes map[string]string `json:"changes"`
}

// EnumDiff represents changes to enum type values.
//
// This structure captures modifications to enum types, specifically the addition
// and removal of enum values. It's important to note that not all databases
// support enum value removal without recreating the entire enum type.
//
// # Database Limitations
//
//   - **PostgreSQL**: Supports adding enum values but not removing them without recreating the enum
//   - **MySQL/MariaDB**: Supports both adding and removing enum values with ALTER TABLE
//   - **SQLite**: No native enum support - uses CHECK constraints
//
// # Example Usage
//
//	enumDiff := EnumDiff{
//		EnumName: "status_type",
//		ValuesAdded: []string{"pending", "archived"},
//		ValuesRemoved: []string{"deprecated"},
//	}
type EnumDiff struct {
	// EnumName is the name of the enum type being modified
	EnumName string `json:"enum_name"`

	// ValuesAdded contains enum values that need to be added to the enum type
	ValuesAdded []string `json:"values_added"`

	// ValuesRemoved contains enum values that need to be removed from the enum type
	// (may not be supported by all databases - see database limitations above)
	ValuesRemoved []string `json:"values_removed"`
}
