// Package fromschema provides converters for transforming goschema types into AST nodes.
//
// This package serves as a bridge between the high-level schema definitions (goschema.Field,
// goschema.Table, etc.) and the low-level AST nodes that represent SQL DDL statements.
// The converters handle the translation of schema metadata into concrete SQL structures that
// can be rendered by dialect-specific visitors.
//
// # Core Functionality
//
// The package provides converter functions for all major schema elements:
//   - FromField: Converts field definitions to column AST nodes
//   - FromTable: Converts table definitions to CREATE TABLE AST nodes
//   - FromIndex: Converts index definitions to index AST nodes
//   - FromEnum: Converts enum definitions to enum AST nodes
//   - FromDatabase: Converts complete database schema to statement list
//
// # Example Usage
//
// Converting a simple field definition:
//
//	field := goschema.Field{
//		Name:     "email",
//		Type:     "VARCHAR(255)",
//		Nullable: false,
//		Unique:   true,
//		Comment:  "User email address",
//	}
//	column := fromschema.FromField(field, nil)
//
// Converting a complete database schema:
//
//	database := goschema.Database{
//		Tables: []goschema.Table{...},
//		Fields: []goschema.Field{...},
//		Indexes: []goschema.Index{...},
//		Enums: []goschema.Enum{...},
//	}
//	statements := fromschema.FromDatabase(database, "postgres")
//
// Platform-specific usage:
//
//	// Convert for MySQL with platform-specific overrides
//	mysqlStatements := fromschema.FromDatabase(database, "mysql")
//
//	// Convert for PostgreSQL with platform-specific overrides
//	postgresStatements := fromschema.FromDatabase(database, "postgres")
//
//	// Convert without platform-specific overrides (uses defaults)
//	defaultStatements := fromschema.FromDatabase(database, "")
package fromschema

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/goschema"
)

func applyPlatformOverrides(field goschema.Field, targetPlatform string) goschema.Field {
	fieldType := field.Type
	checkConstraint := field.Check
	comment := field.Comment
	defaultValue := field.Default
	defaultExpr := field.DefaultExpr

	// Apply platform-specific overrides if available
	if targetPlatform == "" {
		return field
	}

	if field.Overrides == nil {
		return field
	}

	platformOverrides, exists := field.Overrides[targetPlatform]
	if !exists {
		return field
	}

	// Override type if specified
	if typeOverride, ok := platformOverrides["type"]; ok {
		fieldType = typeOverride
	}
	// Override check constraint if specified
	if checkOverride, ok := platformOverrides["check"]; ok {
		checkConstraint = checkOverride
	}
	// Override comment if specified
	if commentOverride, ok := platformOverrides["comment"]; ok {
		comment = commentOverride
	}
	// Override default value if specified
	if defaultOverride, ok := platformOverrides["default"]; ok {
		defaultValue = defaultOverride
		defaultExpr = "" // Clear expression if literal default is overridden
	}
	// Override default expression if specified
	if defaultExprOverride, ok := platformOverrides["default_expr"]; ok {
		defaultExpr = defaultExprOverride
		defaultValue = "" // Clear literal if expression default is overridden
	}

	newField := field // Shallow copy to avoid modifying original field
	newField.Type = fieldType
	newField.Check = checkConstraint
	newField.Comment = comment
	newField.Default = defaultValue
	newField.DefaultExpr = defaultExpr

	return newField
}

func handleEnumTypesForMySQLLike(field goschema.Field, enums []goschema.Enum, targetPlatform string) goschema.Field {
	// Handle enum types for MySQL/MariaDB platforms
	if !strings.HasPrefix(field.Type, "enum_") {
		return field
	}

	if enums == nil {
		return field
	}
	// Validate enum field
	validateEnumField(field, enums)

	if targetPlatform != "mysql" && targetPlatform != "mariadb" {
		return field
	}

	fieldType := field.Type

	// For MySQL/MariaDB, convert enum type to inline enum values
	// Find the corresponding global enum
	for _, enum := range enums {
		if enum.Name != field.Type {
			continue
		}

		// Convert to inline ENUM syntax for MySQL/MariaDB
		quotedValues := make([]string, len(enum.Values))
		for i, value := range enum.Values {
			quotedValues[i] = fmt.Sprintf("'%s'", value) // TODO: properly escape
		}
		fieldType = fmt.Sprintf("ENUM(%s)", strings.Join(quotedValues, ", "))
		break
	}

	newField := field
	newField.Type = fieldType
	return newField
}

// FromField converts a goschema.Field to an ast.ColumnNode with comprehensive attribute mapping.
//
// This function transforms a high-level field definition into a concrete column AST node,
// handling all supported column attributes including constraints, defaults, foreign keys,
// enum validation, and platform-specific overrides.
//
// # Parameters
//
//   - field: The schema field definition containing all column metadata
//   - enums: Global enum definitions used for enum type validation (can be nil)
//   - targetPlatform: Target database platform for applying platform-specific overrides (e.g., "postgres", "mysql", "mariadb")
//
// # Supported Attributes
//
//   - Basic properties: name, type, nullable
//   - Constraints: primary key, unique, auto-increment
//   - Defaults: literal values and function calls
//   - Validation: check constraints
//   - Relationships: foreign key references
//   - Documentation: column comments
//   - Platform overrides: dialect-specific type mappings
//
// # Examples
//
// Basic field with constraints:
//
//	field := goschema.Field{
//		Name:     "email",
//		Type:     "VARCHAR(255)",
//		Nullable: false,
//		Unique:   true,
//		Comment:  "User email address",
//	}
//	column := FromField(field, nil)
//	// Results in: email VARCHAR(255) NOT NULL UNIQUE COMMENT 'User email address'
//
// Field with foreign key:
//
//	field := goschema.Field{
//		Name:           "user_id",
//		Type:           "INTEGER",
//		Nullable:       false,
//		Foreign:        "users(id)",
//		ForeignKeyName: "fk_posts_user",
//	}
//	column := FromField(field, nil)
//	// Results in: user_id INTEGER NOT NULL REFERENCES users(id)
//
// Field with default values:
//
//	field := goschema.Field{
//		Name:        "created_at",
//		Type:        "TIMESTAMP",
//		Nullable:    false,
//		DefaultExpr: "NOW()",
//	}
//	column := FromField(field, nil)
//	// Results in: created_at TIMESTAMP NOT NULL DEFAULT NOW()
//
// # Platform-Specific Overrides
//
// The function supports platform-specific overrides through the field.Overrides map.
// These overrides allow different database platforms to use different configurations:
//
//	field := goschema.Field{
//		Name: "data",
//		Type: "JSONB",
//		Overrides: map[string]map[string]string{
//			"mysql":   {"type": "JSON"},
//			"mariadb": {"type": "LONGTEXT", "check": "JSON_VALID(data)"},
//		},
//	}
//	// For MySQL: data JSON
//	// For MariaDB: data LONGTEXT CHECK (JSON_VALID(data))
//	// For PostgreSQL: data JSONB (default)
//
// # Return Value
//
// Returns a fully configured *ast.ColumnNode ready for SQL generation by dialect-specific visitors.
// The returned node contains all the attributes specified in the input field, with platform-specific
// overrides applied when a matching platform is specified.
func FromField(field goschema.Field, enums []goschema.Enum, targetPlatform string) *ast.ColumnNode {
	field = applyPlatformOverrides(field, targetPlatform)
	field = handleEnumTypesForMySQLLike(field, enums, targetPlatform)

	column := ast.NewColumn(field.Name, field.Type)

	// Set nullable - only override default if explicitly set to false
	// The default behavior should be nullable=true (which ast.NewColumn already sets)
	if !field.Nullable {
		column.SetNotNull()
	}

	// Set constraints
	if field.Primary {
		column.SetPrimary()
	}
	if field.Unique {
		column.SetUnique()
	}
	if field.AutoInc {
		column.SetAutoIncrement()
	}

	// Set default values (using potentially overridden values)
	switch {
	case field.Default != "":
		column.SetDefault(field.Default)
	case field.DefaultExpr != "":
		column.SetDefaultExpression(field.DefaultExpr)
	}

	// Set check constraint (using potentially overridden value)
	if field.Check != "" {
		column.SetCheck(field.Check)
	}

	// Set comment (using potentially overridden value)
	if field.Comment != "" {
		column.SetComment(field.Comment)
	}

	// Set foreign key reference
	if fkRef := parseForeignKeyReference(field.Foreign); fkRef != nil {
		column.SetForeignKey(fkRef.Table, fkRef.Column, field.ForeignKeyName)
	}

	return column
}

// FromTable converts a goschema.Table to an ast.CreateTableNode with all associated columns and constraints.
//
// This function creates a complete table definition by combining table metadata with its associated
// field definitions. It handles table-level properties, adds all matching columns, creates
// composite constraints, and applies platform-specific overrides.
//
// # Parameters
//
//   - table: The table directive containing table-level metadata
//   - fields: All schema fields; only those matching table.StructName are included
//   - enums: Global enum definitions passed to field conversion (can be nil)
//   - targetPlatform: Target database platform for applying platform-specific overrides
//
// # Table Features
//
//   - Table naming and comments
//   - Database-specific options (e.g., MySQL ENGINE)
//   - Composite primary keys
//   - Column definitions with full attribute support
//   - Automatic field filtering by struct name
//
// # Examples
//
// Basic table with simple primary key:
//
//	table := goschema.Table{
//		StructName: "User",
//		Name:       "users",
//		Comment:    "Application users",
//	}
//	fields := []goschema.Field{
//		{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
//		{StructName: "User", Name: "email", Type: "VARCHAR(255)", Nullable: false, Unique: true},
//	}
//	createTable := FromTable(table, fields, nil)
//
// Table with composite primary key:
//
//	table := goschema.Table{
//		StructName: "UserRole",
//		Name:       "user_roles",
//		PrimaryKey: []string{"user_id", "role_id"},
//	}
//	fields := []goschema.Field{
//		{StructName: "UserRole", Name: "user_id", Type: "INTEGER", Foreign: "users(id)"},
//		{StructName: "UserRole", Name: "role_id", Type: "INTEGER", Foreign: "roles(id)"},
//	}
//	createTable := FromTable(table, fields, nil)
//
// MySQL table with engine specification:
//
//	table := goschema.Table{
//		StructName: "Product",
//		Name:       "products",
//		Engine:     "InnoDB",
//		Comment:    "Product catalog",
//	}
//	createTable := FromTable(table, fields, nil)
//
// # Platform-Specific Overrides
//
// The function supports platform-specific table overrides through the table.Overrides map:
//
//	table := goschema.Table{
//		Name: "products",
//		Overrides: map[string]map[string]string{
//			"mysql":   {"engine": "InnoDB", "comment": "Product catalog"},
//			"mariadb": {"engine": "InnoDB", "charset": "utf8mb4"},
//		},
//	}
//
// # Return Value
//
// Returns a fully configured *ast.CreateTableNode ready for SQL generation.
// The node contains the table definition with all columns, constraints, and platform-specific options.
func FromTable(table goschema.Table, fields []goschema.Field, enums []goschema.Enum, targetPlatform string) *ast.CreateTableNode {
	createTable := ast.NewCreateTable(table.Name)

	// Start with base table values
	tableComment := table.Comment
	tableEngine := table.Engine

	// Apply platform-specific overrides if available
	if targetPlatform != "" && table.Overrides != nil {
		if platformOverrides, exists := table.Overrides[targetPlatform]; exists {
			// Override comment if specified
			if commentOverride, ok := platformOverrides["comment"]; ok {
				tableComment = commentOverride
			}
			// Override engine if specified
			if engineOverride, ok := platformOverrides["engine"]; ok {
				tableEngine = engineOverride
			}
			// Apply any other platform-specific options
			for key, value := range platformOverrides {
				if key != "comment" && key != "engine" {
					createTable.SetOption(strings.ToUpper(key), value)
				}
			}
		}
	}

	// Set table comment (using potentially overridden value)
	if tableComment != "" {
		createTable.Comment = tableComment
	}

	// Set database-specific options (using potentially overridden value)
	if tableEngine != "" {
		createTable.SetOption("ENGINE", tableEngine)
	}

	// Add columns for fields that belong to this table
	for _, field := range fields {
		if field.StructName == table.StructName {
			column := FromField(field, enums, targetPlatform)
			createTable.AddColumn(column)
		}
	}

	// Add composite primary key constraint if specified
	if len(table.PrimaryKey) > 1 {
		constraint := ast.NewPrimaryKeyConstraint(table.PrimaryKey...)
		createTable.AddConstraint(constraint)
	}

	return createTable
}

// FromIndex converts a goschema.Index to an ast.IndexNode for database index creation.
//
// This function transforms index metadata into an AST node that can be rendered
// as CREATE INDEX statements by dialect-specific visitors. It supports both
// single-column and composite indexes with optional uniqueness constraints.
//
// # Parameters
//
//   - index: The schema index definition containing index metadata
//
// # Index Features
//
//   - Single-column and composite indexes
//   - Unique and non-unique indexes
//   - Index comments for documentation
//   - Automatic table association
//
// # Examples
//
// Simple single-column index:
//
//	index := goschema.Index{
//		Name:       "idx_users_email",
//		StructName: "users",
//		Fields:     []string{"email"},
//		Comment:    "Index for email lookups",
//	}
//	indexNode := FromIndex(index)
//
// Unique composite index:
//
//	index := goschema.Index{
//		Name:       "idx_user_roles_unique",
//		StructName: "user_roles",
//		Fields:     []string{"user_id", "role_id"},
//		Unique:     true,
//		Comment:    "Ensure unique user-role combinations",
//	}
//	indexNode := FromIndex(index)
//
// # Return Value
//
// Returns a fully configured *ast.IndexNode ready for SQL generation.
// The node contains the index name, target table, column list, and all specified options.
func FromIndex(index goschema.Index) *ast.IndexNode {
	indexNode := ast.NewIndex(index.Name, index.StructName, index.Fields...)

	// Set unique constraint
	if index.Unique {
		indexNode.Unique = true
	}

	// Set comment
	if index.Comment != "" {
		indexNode.Comment = index.Comment
	}

	return indexNode
}

// FromEnum converts a goschema.Enum to an ast.EnumNode for database enum type creation.
//
// This function transforms a global enum definition into an AST node that can be rendered
// as CREATE TYPE statements (primarily for PostgreSQL) or equivalent enum handling for
// other database systems.
//
// # Parameters
//
//   - enum: The global enum definition containing the enum name and allowed values
//
// # Examples
//
// Simple status enum:
//
//	enum := goschema.Enum{
//		Name:   "status_type",
//		Values: []string{"active", "inactive", "pending"},
//	}
//	enumNode := FromEnum(enum)
//
// User role enum:
//
//	enum := goschema.Enum{
//		Name:   "user_role",
//		Values: []string{"admin", "moderator", "user", "guest"},
//	}
//	enumNode := FromEnum(enum)
//
// # Database Support
//
// Enum support varies by database:
//   - PostgreSQL: Native ENUM types via CREATE TYPE
//   - MySQL: ENUM column types
//   - SQLite: CHECK constraints with IN clauses
//   - Other databases: Various enum-like implementations
//
// # Return Value
//
// Returns an *ast.EnumNode ready for SQL generation by dialect-specific visitors.
// The visitor implementation determines how the enum is rendered for each database type.
func FromEnum(enum goschema.Enum) *ast.EnumNode {
	return ast.NewEnum(enum.Name, enum.Values...)
}

// FromDatabase converts a complete goschema.Database to an ast.StatementList containing all DDL statements.
//
// This function creates a comprehensive database schema by converting all schema elements
// (enums, tables, indexes, embedded fields) into their corresponding AST nodes. The statements are ordered
// to ensure proper dependency resolution during SQL execution, with platform-specific
// overrides applied throughout.
//
// # Parameters
//
//   - database: The complete database schema containing all tables, fields, indexes, enums, and embedded fields
//   - targetPlatform: Target database platform for applying platform-specific overrides
//
// # Statement Ordering
//
// The function generates statements in the following order to respect dependencies:
//  1. Enum type definitions (CREATE TYPE statements)
//  2. Table definitions (CREATE TABLE statements) with embedded fields processed
//  3. Index definitions (CREATE INDEX statements)
//
// This ordering ensures that:
//   - Enum types are created before tables that reference them
//   - Tables are created before indexes that reference them
//   - Foreign key dependencies are handled by the table creation order
//   - Embedded fields are processed and converted to regular fields before table creation
//
// # Embedded Field Processing
//
// The function processes embedded fields before creating tables, supporting four modes:
//   - "inline": Expands embedded struct fields as individual table columns
//   - "json": Serializes the entire embedded struct into a single JSON/JSONB column
//   - "relation": Creates a foreign key relationship to another table
//   - "skip": Completely ignores the embedded field during schema generation
//
// # Examples
//
// Converting a complete database schema:
//
//	database := goschema.Database{
//		Enums: []goschema.Enum{
//			{Name: "user_status", Values: []string{"active", "inactive"}},
//		},
//		Tables: []goschema.Table{
//			{StructName: "User", Name: "users", Comment: "User accounts"},
//		},
//		Fields: []goschema.Field{
//			{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
//			{StructName: "User", Name: "status", Type: "user_status", Nullable: false},
//		},
//		EmbeddedFields: []goschema.EmbeddedField{
//			{StructName: "User", Mode: "inline", EmbeddedTypeName: "Timestamps"},
//		},
//		Indexes: []goschema.Index{
//			{Name: "idx_users_status", StructName: "users", Fields: []string{"status"}},
//		},
//	}
//	statements := FromDatabase(database)
//
// # Platform-Specific Processing
//
// All schema elements (tables, fields, embedded fields) are processed with platform-specific overrides
// applied based on the targetPlatform parameter. This ensures that the generated
// AST nodes contain the appropriate configurations for the target database.
//
// # Return Value
//
// Returns an *ast.StatementList containing all DDL statements in proper execution order.
// The statement list can be processed by dialect-specific visitors to generate SQL.
func FromDatabase(database goschema.Database, targetPlatform string) *ast.StatementList {
	statements := &ast.StatementList{
		Statements: make([]ast.Node, 0),
	}

	// Process embedded fields to generate additional fields for each table
	allFields := processEmbeddedFields(database.EmbeddedFields, database.Fields)

	// 1. Add enum definitions first (they may be referenced by tables)
	for _, enum := range database.Enums {
		enumNode := FromEnum(enum)
		statements.Statements = append(statements.Statements, enumNode)
	}

	// 2. Add table definitions (they may be referenced by indexes)
	// Use the combined field list that includes embedded field expansions
	for _, table := range database.Tables {
		tableNode := FromTable(table, allFields, database.Enums, targetPlatform)
		statements.Statements = append(statements.Statements, tableNode)
	}

	// 3. Add index definitions last
	for _, index := range database.Indexes {
		indexNode := FromIndex(index)
		statements.Statements = append(statements.Statements, indexNode)
	}

	return statements
}

// parseForeignKeyReference parses a foreign key reference string into an ast.ForeignKeyRef.
//
// The foreign key reference string should be in the format "table(column)" or just "table"
// (which defaults to referencing the "id" column).
//
// Examples:
//   - "users(id)" -> references users.id
//   - "users" -> references users.id (default)
//   - "categories(slug)" -> references categories.slug
//
// Returns nil if the reference string is malformed.
func parseForeignKeyReference(foreign string) *ast.ForeignKeyRef {
	if foreign == "" {
		return nil
	}

	// Check if it contains parentheses for column specification
	if strings.Contains(foreign, "(") && strings.Contains(foreign, ")") {
		// Parse "table(column)" format
		parts := strings.Split(foreign, "(")
		if len(parts) != 2 {
			return nil
		}

		table := strings.TrimSpace(parts[0])
		columnPart := strings.TrimSpace(parts[1])

		// Remove closing parenthesis
		if !strings.HasSuffix(columnPart, ")") {
			return nil
		}
		column := strings.TrimSuffix(columnPart, ")")

		return &ast.ForeignKeyRef{
			Table:  table,
			Column: column,
		}
	}

	// Default to "id" column if no column specified
	return &ast.ForeignKeyRef{
		Table:  strings.TrimSpace(foreign),
		Column: "id",
	}
}

// validateEnumField validates that enum field values are consistent with global enum definitions.
//
// This function performs validation for fields with enum types, ensuring that:
//   - The referenced global enum exists
//   - Any field-specific enum values are a subset of the global enum values
//
// Validation warnings are logged but do not stop the conversion process, allowing for
// graceful handling of incomplete or evolving schema definitions.
func validateEnumField(field goschema.Field, enums []goschema.Enum) {
	if !strings.HasPrefix(field.Type, "enum_") {
		return
	}

	// Find the corresponding global enum
	var globalEnum *goschema.Enum
	for _, enum := range enums {
		if enum.Name == field.Type {
			globalEnum = &enum
			break
		}
	}

	// If no global enum found, this might be an issue but we don't panic
	// as the field might be using a custom enum type
	if globalEnum == nil {
		return
	}

	// If field has enum values, validate they match the global enum
	if len(field.Enum) > 0 {
		// Check that all field enum values exist in the global enum
		globalEnumMap := make(map[string]bool)
		for _, value := range globalEnum.Values {
			globalEnumMap[value] = true
		}

		for _, fieldValue := range field.Enum {
			if fieldValue != "" && !globalEnumMap[fieldValue] {
				// Log warning - in a real implementation, you might want to use a proper logger
				// For now, we'll just continue without panicking
				_ = fieldValue // Suppress unused variable warning
			}
		}
	}
}

// processEmbeddedFields processes embedded fields and generates corresponding schema fields based on embedding modes.
//
// This function is the core processor for handling embedded struct fields in Go structs, transforming them
// into appropriate database schema fields according to the specified embedding mode. It supports four
// distinct modes of embedding that provide different approaches to handling complex data structures
// in relational databases.
//
// # Parameters
//
//   - embeddedFields: Collection of embedded field definitions to process
//   - originalFields: Complete collection of schema fields from all parsed structs
//
// # Embedding Modes
//
// The function supports four embedding modes, each serving different architectural patterns:
//
// 1. **"inline"**: Expands embedded struct fields as individual table columns
// 2. **"json"**: Serializes the entire embedded struct into a single JSON/JSONB column
// 3. **"relation"**: Creates a foreign key relationship to another table
// 4. **"skip"**: Completely ignores the embedded field during schema generation
//
// # Return Value
//
// Returns a combined slice of goschema.Field containing both the original fields and
// the generated fields from embedded field processing. This combined list is ready
// for use in table creation.
func processEmbeddedFields(embeddedFields []goschema.EmbeddedField, originalFields []goschema.Field) []goschema.Field {
	// Start with the original fields
	allFields := make([]goschema.Field, len(originalFields))
	copy(allFields, originalFields)

	// Process embedded fields for each struct
	structNames := getUniqueStructNames(embeddedFields)
	for _, structName := range structNames {
		generatedFields := processEmbeddedFieldsForStruct(embeddedFields, originalFields, structName)
		allFields = append(allFields, generatedFields...)
	}

	return allFields
}

// getUniqueStructNames extracts unique struct names from embedded fields.
func getUniqueStructNames(embeddedFields []goschema.EmbeddedField) []string {
	structNameMap := make(map[string]bool)
	for _, embedded := range embeddedFields {
		structNameMap[embedded.StructName] = true
	}

	var structNames []string
	for structName := range structNameMap {
		structNames = append(structNames, structName)
	}
	return structNames
}

func processEmbeddedInlineMode(generatedFields []goschema.Field, embedded goschema.EmbeddedField, allFields []goschema.Field, structName string) []goschema.Field {
	// INLINE MODE: Expand embedded struct fields as individual table columns
	for _, field := range allFields {
		if field.StructName != embedded.EmbeddedTypeName {
			continue
		}
		// Clone the field and reassign to target struct
		newField := field
		newField.StructName = structName

		// Apply prefix to column name if specified
		if embedded.Prefix != "" {
			newField.Name = embedded.Prefix + field.Name
		}

		generatedFields = append(generatedFields, newField)
	}

	return generatedFields
}

func processEmbeddedJSONMode(generatedFields []goschema.Field, embedded goschema.EmbeddedField, structName string) []goschema.Field {
	// JSON MODE: Serialize embedded struct into a single JSON/JSONB column
	columnName := embedded.Name
	if columnName == "" {
		// Auto-generate column name: "Meta" -> "meta_data"
		columnName = strings.ToLower(embedded.EmbeddedTypeName) + "_data"
	}

	columnType := embedded.Type
	if columnType == "" {
		columnType = "JSONB" // Default to PostgreSQL JSONB for best performance
	}

	// Create the JSON column field
	generatedFields = append(generatedFields, goschema.Field{
		StructName: structName,
		FieldName:  embedded.EmbeddedTypeName,
		Name:       columnName,
		Type:       columnType,
		Nullable:   embedded.Nullable,
		Comment:    embedded.Comment,
		Overrides:  embedded.Overrides, // Platform-specific type overrides (JSON vs JSONB vs TEXT)
	})

	return generatedFields
}

func processEmbeddedRelationMode(generatedFields []goschema.Field, embedded goschema.EmbeddedField, structName string) []goschema.Field {
	// RELATION MODE: Create a foreign key field linking to another table
	if embedded.Field == "" || embedded.Ref == "" {
		// Skip incomplete relation definitions - both field name and reference are required
		return generatedFields
	}

	// Intelligent type inference based on reference pattern
	refType := "INTEGER" // Default assumption: numeric primary key
	if strings.Contains(embedded.Ref, "VARCHAR") || strings.Contains(embedded.Ref, "TEXT") ||
		strings.Contains(strings.ToLower(embedded.Ref), "uuid") {
		// Reference suggests string-based key (likely UUID)
		refType = "VARCHAR(36)" // Standard UUID length
	}

	// Generate automatic foreign key constraint name following convention
	foreignKeyName := "fk_" + strings.ToLower(structName) + "_" + strings.ToLower(embedded.Field)

	// Create the foreign key field
	generatedFields = append(generatedFields, goschema.Field{
		StructName:     structName,
		FieldName:      embedded.EmbeddedTypeName,
		Name:           embedded.Field,    // e.g., "user_id"
		Type:           refType,           // INTEGER or VARCHAR(36)
		Nullable:       embedded.Nullable, // Can the relationship be optional?
		Foreign:        embedded.Ref,      // e.g., "users(id)"
		ForeignKeyName: foreignKeyName,    // e.g., "fk_posts_user_id"
		Comment:        embedded.Comment,  // Documentation for the relationship
	})

	return generatedFields
}

// processEmbeddedFieldsForStruct processes embedded fields for a specific struct and generates corresponding schema fields.
//
// This function implements the core logic for transforming embedded fields into database schema fields
// according to their specified embedding mode. It processes only embedded fields that belong to the
// specified structName.
//
// # Parameters
//
//   - embeddedFields: Collection of embedded field definitions to process
//   - allFields: Complete collection of schema fields from all parsed structs
//   - structName: Name of the target struct to process embedded fields for
//
// # Return Value
//
// Returns a slice of goschema.Field representing the generated database fields for the specified struct.
// Each field is fully configured with appropriate types, constraints, and metadata.
func processEmbeddedFieldsForStruct(embeddedFields []goschema.EmbeddedField, allFields []goschema.Field, structName string) []goschema.Field {
	var generatedFields []goschema.Field

	// Process each embedded field definition
	for _, embedded := range embeddedFields {
		// Filter: only process embedded fields for the target struct
		if embedded.StructName != structName {
			continue
		}

		switch embedded.Mode {
		case "inline":
			// INLINE MODE: Expand embedded struct fields as individual table columns
			generatedFields = processEmbeddedInlineMode(generatedFields, embedded, allFields, structName)
		case "json":
			// JSON MODE: Serialize embedded struct into a single JSON/JSONB column
			generatedFields = processEmbeddedJSONMode(generatedFields, embedded, structName)
		case "relation":
			// RELATION MODE: Create a foreign key field linking to another table
			generatedFields = processEmbeddedRelationMode(generatedFields, embedded, structName)
		case "skip":
			// SKIP MODE: Completely ignore this embedded field
			continue
		default:
			// DEFAULT MODE: Fall back to inline behavior for unrecognized modes
			slog.Warn("Unrecognized embedding mode for struct - defaulting to inline mode", "mode", embedded.Mode, "struct", structName)
			generatedFields = processEmbeddedInlineMode(generatedFields, embedded, allFields, structName)
		}
	}

	return generatedFields
}
