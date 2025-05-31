// Package transform provides converters for transforming schema types into AST nodes.
//
// This package serves as a bridge between the high-level schema definitions (types.SchemaField,
// types.TableDirective, etc.) and the low-level AST nodes that represent SQL DDL statements.
// The converters handle the translation of schema metadata into concrete SQL structures that
// can be rendered by dialect-specific visitors.
//
// # Core Functionality
//
// The package provides four main converter functions:
//   - FromSchemaField: Converts field definitions to column AST nodes
//   - FromTableDirective: Converts table definitions to CREATE TABLE AST nodes
//   - FromSchemaIndex: Converts index definitions to index AST nodes
//   - FromGlobalEnum: Converts enum definitions to enum AST nodes
//
// # Use Cases
//
// 1. **Migration Generation**: Convert parsed schema annotations into SQL DDL statements
// 2. **Schema Validation**: Transform schema definitions for validation and analysis
// 3. **Cross-Database Support**: Generate database-agnostic AST that can be rendered for different SQL dialects
// 4. **Schema Evolution**: Compare and transform schema definitions for migration planning
//
// # Example Usage
//
// Converting a simple field definition:
//
//	field := types.SchemaField{
//		Name:     "email",
//		Type:     "VARCHAR(255)",
//		Nullable: false,
//		Unique:   true,
//		Comment:  "User email address",
//	}
//	column := transform.FromSchemaField(field, nil)
//	// Results in: email VARCHAR(255) NOT NULL UNIQUE COMMENT 'User email address'
//
// Converting a table with multiple fields:
//
//	table := types.TableDirective{
//		StructName: "User",
//		Name:       "users",
//		Comment:    "Application users",
//	}
//	fields := []types.SchemaField{
//		{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
//		{StructName: "User", Name: "email", Type: "VARCHAR(255)", Nullable: false, Unique: true},
//	}
//	createTable := transform.FromTableDirective(table, fields, nil)
//	// Results in: CREATE TABLE users (id SERIAL PRIMARY KEY, email VARCHAR(255) NOT NULL UNIQUE) COMMENT 'Application users'
//
// # Enum Handling
//
// The package includes special handling for enum types. Fields with types starting with "enum_"
// are validated against global enum definitions to ensure consistency:
//
//	enum := types.GlobalEnum{
//		Name:   "enum_status",
//		Values: []string{"active", "inactive", "pending"},
//	}
//	field := types.SchemaField{
//		Name: "status",
//		Type: "enum_status",
//		Enum: []string{"active", "inactive"}, // Subset validation
//	}
//	column := transform.FromSchemaField(field, []types.GlobalEnum{enum})
//
// # Error Handling
//
// The converters are designed to be permissive and continue processing even when encountering
// validation issues. Enum validation warnings are logged but do not halt the conversion process.
// This allows for graceful handling of incomplete or evolving schema definitions.
package transform

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// FromSchemaField converts a SchemaField to a ColumnNode with comprehensive attribute mapping.
//
// This function transforms a high-level field definition into a concrete column AST node,
// handling all supported column attributes including constraints, defaults, foreign keys,
// and enum validation.
//
// # Parameters
//
//   - field: The schema field definition containing all column metadata
//   - enums: Global enum definitions used for enum type validation (can be nil)
//
// # Supported Attributes
//
//   - Basic properties: name, type, nullable
//   - Constraints: primary key, unique, auto-increment
//   - Defaults: literal values and function calls
//   - Validation: check constraints
//   - Relationships: foreign key references
//   - Documentation: column comments
//   - Enums: validation against global enum definitions
//
// # Examples
//
// Basic column with constraints:
//
//	field := types.SchemaField{
//		Name:     "id",
//		Type:     "SERIAL",
//		Primary:  true,
//		AutoInc:  true,
//		Comment:  "Primary key",
//	}
//	column := FromSchemaField(field, nil)
//	// Results in: id SERIAL PRIMARY KEY AUTO_INCREMENT COMMENT 'Primary key'
//
// Column with foreign key:
//
//	field := types.SchemaField{
//		Name:           "user_id",
//		Type:           "INTEGER",
//		Nullable:       false,
//		Foreign:        "users(id)",
//		ForeignKeyName: "fk_posts_user",
//	}
//	column := FromSchemaField(field, nil)
//	// Results in: user_id INTEGER NOT NULL REFERENCES users(id)
//
// Column with default value and check constraint:
//
//	field := types.SchemaField{
//		Name:     "price",
//		Type:     "DECIMAL(10,2)",
//		Nullable: false,
//		Default:  "0.00",
//		Check:    "price >= 0",
//		Comment:  "Product price in USD",
//	}
//	column := FromSchemaField(field, nil)
//	// Results in: price DECIMAL(10,2) NOT NULL DEFAULT 0.00 CHECK (price >= 0) COMMENT 'Product price in USD'
//
// Enum column with validation:
//
//	enum := types.GlobalEnum{
//		Name:   "enum_status",
//		Values: []string{"active", "inactive", "pending"},
//	}
//	field := types.SchemaField{
//		Name: "status",
//		Type: "enum_status",
//		Enum: []string{"active", "inactive"}, // Subset of global enum
//	}
//	column := FromSchemaField(field, []types.GlobalEnum{enum})
//	// Results in: status enum_status (with validation performed)
//
// # Enum Validation
//
// When a field type starts with "enum_", the function validates that:
//   - The referenced global enum exists
//   - Any field-specific enum values are a subset of the global enum values
//   - Warnings are logged for validation failures without stopping conversion
//
// # Return Value
//
// Returns a fully configured *ast.ColumnNode ready for SQL generation by dialect-specific visitors.
// The returned node contains all the attributes specified in the input field.
func FromSchemaField(field types.SchemaField, enums []types.GlobalEnum) *ast.ColumnNode {
	column := ast.NewColumn(field.Name, field.Type)

	// Validate enum type if field references an enum
	if isEnumType(field.Type) {
		validateEnumField(field, enums)
	}

	if !field.Nullable {
		column.SetNotNull()
	}

	if field.Primary {
		column.SetPrimary()
	}

	if field.Unique {
		column.SetUnique()
	}

	if field.AutoInc {
		column.SetAutoIncrement()
	}

	if field.Default != "" {
		column.SetDefault(field.Default)
	}

	if field.DefaultExpr != "" {
		column.SetDefaultExpression(field.DefaultExpr)
	}

	if field.Check != "" {
		column.SetCheck(field.Check)
	}

	if field.Comment != "" {
		column.SetComment(field.Comment)
	}

	if field.Foreign != "" {
		column.SetForeignKey(field.Foreign, "", field.ForeignKeyName)
	}

	return column
}

// FromTableDirective converts a TableDirective to a CreateTableNode with all associated columns and constraints.
//
// This function creates a complete table definition by combining table metadata with its associated
// field definitions. It handles table-level properties, adds all matching columns, and creates
// composite constraints when needed.
//
// # Parameters
//
//   - table: The table directive containing table-level metadata
//   - fields: All schema fields; only those matching table.StructName are included
//   - enums: Global enum definitions passed to field conversion (can be nil)
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
//	table := types.TableDirective{
//		StructName: "User",
//		Name:       "users",
//		Comment:    "Application users",
//	}
//	fields := []types.SchemaField{
//		{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
//		{StructName: "User", Name: "email", Type: "VARCHAR(255)", Nullable: false, Unique: true},
//		{StructName: "User", Name: "created_at", Type: "TIMESTAMP", DefaultExpr: "NOW()"},
//	}
//	createTable := FromTableDirective(table, fields, nil)
//	// Results in: CREATE TABLE users (
//	//   id SERIAL PRIMARY KEY,
//	//   email VARCHAR(255) NOT NULL UNIQUE,
//	//   created_at TIMESTAMP DEFAULT NOW()
//	// ) COMMENT 'Application users'
//
// Table with composite primary key:
//
//	table := types.TableDirective{
//		StructName: "UserRole",
//		Name:       "user_roles",
//		PrimaryKey: []string{"user_id", "role_id"},
//		Comment:    "Many-to-many user roles",
//	}
//	fields := []types.SchemaField{
//		{StructName: "UserRole", Name: "user_id", Type: "INTEGER", Foreign: "users(id)"},
//		{StructName: "UserRole", Name: "role_id", Type: "INTEGER", Foreign: "roles(id)"},
//		{StructName: "UserRole", Name: "assigned_at", Type: "TIMESTAMP", DefaultExpr: "NOW()"},
//	}
//	createTable := FromTableDirective(table, fields, nil)
//	// Results in: CREATE TABLE user_roles (
//	//   user_id INTEGER REFERENCES users(id),
//	//   role_id INTEGER REFERENCES roles(id),
//	//   assigned_at TIMESTAMP DEFAULT NOW(),
//	//   PRIMARY KEY (user_id, role_id)
//	// ) COMMENT 'Many-to-many user roles'
//
// Table with database-specific options:
//
//	table := types.TableDirective{
//		StructName: "Product",
//		Name:       "products",
//		Engine:     "InnoDB",
//		Comment:    "Product catalog",
//	}
//	fields := []types.SchemaField{
//		{StructName: "Product", Name: "id", Type: "INT", Primary: true, AutoInc: true},
//		{StructName: "Product", Name: "name", Type: "VARCHAR(255)", Nullable: false},
//	}
//	createTable := FromTableDirective(table, fields, nil)
//	// Results in: CREATE TABLE products (
//	//   id INT PRIMARY KEY AUTO_INCREMENT,
//	//   name VARCHAR(255) NOT NULL
//	// ) ENGINE=InnoDB COMMENT 'Product catalog'
//
// # Field Filtering
//
// The function automatically filters the provided fields array to include only those
// where field.StructName matches table.StructName. This allows passing a complete
// field collection while processing multiple tables.
//
// # Composite Primary Keys
//
// When table.PrimaryKey contains multiple field names, a table-level PRIMARY KEY
// constraint is added. Individual field primary key flags are still respected
// for single-column primary keys.
//
// # Return Value
//
// Returns a fully configured *ast.CreateTableNode with all columns, constraints,
// and table options ready for SQL generation.
func FromTableDirective(table types.TableDirective, fields []types.SchemaField, enums []types.GlobalEnum) *ast.CreateTableNode {
	createTable := ast.NewCreateTable(table.Name)

	if table.Comment != "" {
		createTable.Comment = table.Comment
	}

	if table.Engine != "" {
		createTable.SetOption("ENGINE", table.Engine)
	}

	// Add columns
	for _, field := range fields {
		if field.StructName == table.StructName {
			column := FromSchemaField(field, enums)
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

// FromSchemaIndex converts a SchemaIndex to an IndexNode for database index creation.
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
//	index := types.SchemaIndex{
//		Name:       "idx_users_email",
//		StructName: "users",
//		Fields:     []string{"email"},
//		Comment:    "Index for email lookups",
//	}
//	indexNode := FromSchemaIndex(index)
//	// Results in: CREATE INDEX idx_users_email ON users (email) COMMENT 'Index for email lookups'
//
// Unique composite index:
//
//	index := types.SchemaIndex{
//		Name:       "idx_user_roles_unique",
//		StructName: "user_roles",
//		Fields:     []string{"user_id", "role_id"},
//		Unique:     true,
//		Comment:    "Ensure unique user-role combinations",
//	}
//	indexNode := FromSchemaIndex(index)
//	// Results in: CREATE UNIQUE INDEX idx_user_roles_unique ON user_roles (user_id, role_id)
//
// Performance index for queries:
//
//	index := types.SchemaIndex{
//		Name:       "idx_posts_user_created",
//		StructName: "posts",
//		Fields:     []string{"user_id", "created_at"},
//		Comment:    "Optimize user posts by creation date queries",
//	}
//	indexNode := FromSchemaIndex(index)
//	// Results in: CREATE INDEX idx_posts_user_created ON posts (user_id, created_at)
//
// # Use Cases
//
//   - Performance optimization for frequently queried columns
//   - Enforcing uniqueness constraints across multiple columns
//   - Supporting foreign key relationships
//   - Optimizing ORDER BY and WHERE clauses
//
// # Return Value
//
// Returns a fully configured *ast.IndexNode ready for SQL generation.
// The node contains the index name, target table, column list, and all specified options.
func FromSchemaIndex(index types.SchemaIndex) *ast.IndexNode {
	indexNode := ast.NewIndex(index.Name, index.StructName, index.Fields...)

	if index.Unique {
		indexNode.SetUnique()
	}

	if index.Comment != "" {
		indexNode.Comment = index.Comment
	}

	return indexNode
}

// FromGlobalEnum converts a GlobalEnum to an EnumNode for database enum type creation.
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
//	enum := types.GlobalEnum{
//		Name:   "status_type",
//		Values: []string{"active", "inactive", "pending"},
//	}
//	enumNode := FromGlobalEnum(enum)
//	// Results in: CREATE TYPE status_type AS ENUM ('active', 'inactive', 'pending')
//
// User role enum:
//
//	enum := types.GlobalEnum{
//		Name:   "user_role",
//		Values: []string{"admin", "moderator", "user", "guest"},
//	}
//	enumNode := FromGlobalEnum(enum)
//	// Results in: CREATE TYPE user_role AS ENUM ('admin', 'moderator', 'user', 'guest')
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
// Returns a *ast.EnumNode ready for SQL generation by dialect-specific visitors.
// The visitor implementation determines how the enum is rendered for each database type.
func FromGlobalEnum(enum types.GlobalEnum) *ast.EnumNode {
	return ast.NewEnum(enum.Name, enum.Values...)
}

// isEnumType checks if a field type represents an enum type.
//
// Enum types are identified by the "enum_" prefix in their type name.
// This convention allows the converter to distinguish between regular
// data types and enum references that require validation.
//
// # Parameters
//
//   - fieldType: The field type string to check
//
// # Examples
//
//	isEnumType("VARCHAR(255)")  // false
//	isEnumType("INTEGER")       // false
//	isEnumType("enum_status")   // true
//	isEnumType("enum_priority") // true
//
// # Return Value
//
// Returns true if the field type starts with "enum_", false otherwise.
func isEnumType(fieldType string) bool {
	return strings.HasPrefix(fieldType, "enum_")
}

// validateEnumField validates that an enum field references a valid global enum and its values are consistent.
//
// This function performs validation of enum field definitions against global enum definitions
// to ensure schema consistency. It checks that referenced enums exist and that any field-specific
// enum values are valid subsets of the global enum values.
//
// # Parameters
//
//   - field: The schema field with enum type to validate
//   - enums: Global enum definitions to validate against
//
// # Validation Rules
//
//  1. If the field type matches a global enum name, the global enum should exist
//  2. If the field has specific enum values, they must be a subset of the global enum values
//  3. Empty field enum values are ignored (field uses all global enum values)
//  4. Validation warnings are logged but do not stop processing
//
// # Examples
//
// Valid enum field (subset of global enum):
//
//	globalEnum := types.GlobalEnum{
//		Name:   "enum_status",
//		Values: []string{"active", "inactive", "pending", "archived"},
//	}
//	field := types.SchemaField{
//		Name: "status",
//		Type: "enum_status",
//		Enum: []string{"active", "inactive"}, // Valid subset
//	}
//	validateEnumField(field, []types.GlobalEnum{globalEnum}) // No warnings
//
// Invalid enum field (contains values not in global enum):
//
//	field := types.SchemaField{
//		Name: "status",
//		Type: "enum_status",
//		Enum: []string{"active", "invalid_value"}, // "invalid_value" not in global enum
//	}
//	validateEnumField(field, []types.GlobalEnum{globalEnum}) // Logs warning
//
// # Error Handling
//
// The function is designed to be non-fatal:
//   - Missing global enums are silently ignored (may be custom types)
//   - Invalid enum values generate warnings but don't halt processing
//   - This allows for graceful handling of incomplete schema definitions
//
// # Use Cases
//
//   - Schema validation during migration generation
//   - Development-time consistency checking
//   - Documentation of enum usage patterns
//   - Debugging enum-related schema issues
func validateEnumField(field types.SchemaField, enums []types.GlobalEnum) {
	// Find the corresponding global enum
	var globalEnum *types.GlobalEnum
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
				// Log warning or handle validation error
				// For now, we'll just continue without panicking
				fmt.Printf("Warning: enum field %s has value '%s' not found in global enum %s\n",
					field.Name, fieldValue, globalEnum.Name)
			}
		}
	}
}
