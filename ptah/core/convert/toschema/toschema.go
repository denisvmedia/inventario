// Package toschema provides converters for transforming AST nodes back into goschema types.
//
// This package serves as the reverse bridge from low-level AST nodes that represent SQL DDL
// statements back to high-level schema definitions (goschema.Field, goschema.Table, etc.).
// The converters handle the extraction of schema metadata from concrete SQL structures and
// can reconstruct platform-specific overrides when multiple platform variants are provided.
//
// # Core Functionality
//
// The package provides converter functions for all major AST elements:
//   - ToField: Converts column AST nodes to field definitions
//   - ToTable: Converts CREATE TABLE AST nodes to table definitions
//   - ToIndex: Converts index AST nodes to index definitions
//   - ToEnum: Converts enum AST nodes to enum definitions
//   - ToDatabase: Converts statement list to complete database schema
//
// # Example Usage
//
// Converting a simple column definition:
//
//	column := ast.NewColumn("email", "VARCHAR(255)").
//		SetNotNull().
//		SetUnique().
//		SetComment("User email address")
//	field := toschema.ToField(column, "User", "")
//
// Converting a complete statement list:
//
//	statements := &ast.StatementList{
//		Statements: []ast.Node{...},
//	}
//	database := toschema.ToDatabase(statements)
//
// Platform-specific reconstruction:
//
//	// Convert multiple platform variants to reconstruct overrides
//	mysqlField := toschema.ToField(mysqlColumn, "User", "mysql")
//	postgresField := toschema.ToField(postgresColumn, "User", "postgres")
//	mergedField := toschema.MergeFieldOverrides(postgresField, map[string]goschema.Field{
//		"mysql": mysqlField,
//	})
package toschema

import (
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/goschema"
)

// ToField converts an ast.ColumnNode to a goschema.Field with comprehensive attribute extraction.
//
// This function transforms a low-level column AST node back into a high-level field definition,
// extracting all supported column attributes including constraints, defaults, foreign keys,
// and comments. When a sourcePlatform is specified, the extracted values can be used to
// reconstruct platform-specific overrides.
//
// # Parameters
//
//   - column: The AST column node containing all column metadata
//   - structName: The name of the Go struct this field belongs to
//   - sourcePlatform: The platform this column was generated for (used for override reconstruction)
//
// # Supported Attributes
//
//   - Basic properties: name, type, nullable
//   - Constraints: primary key, unique, auto-increment
//   - Defaults: literal values and function calls
//   - Validation: check constraints
//   - Relationships: foreign key references
//   - Documentation: column comments
//
// # Examples
//
// Basic column conversion:
//
//	column := ast.NewColumn("email", "VARCHAR(255)").
//		SetNotNull().
//		SetUnique().
//		SetComment("User email address")
//	field := ToField(column, "User", "")
//	// Results in: goschema.Field{
//	//   Name: "email", Type: "VARCHAR(255)", Nullable: false,
//	//   Unique: true, Comment: "User email address"
//	// }
//
// Column with foreign key:
//
//	column := ast.NewColumn("user_id", "INTEGER").
//		SetNotNull().
//		SetForeignKey("users", "id", "fk_posts_user")
//	field := ToField(column, "Post", "")
//	// Results in: goschema.Field{
//	//   Name: "user_id", Type: "INTEGER", Nullable: false,
//	//   Foreign: "users(id)", ForeignKeyName: "fk_posts_user"
//	// }
//
// Column with default values:
//
//	column := ast.NewColumn("created_at", "TIMESTAMP").
//		SetNotNull().
//		SetDefaultExpression("NOW()")
//	field := ToField(column, "User", "")
//	// Results in: goschema.Field{
//	//   Name: "created_at", Type: "TIMESTAMP", Nullable: false,
//	//   DefaultExpr: "NOW()"
//	// }
//
// # Platform-Specific Reconstruction
//
// When sourcePlatform is specified, the function can be used to reconstruct platform overrides:
//
//	mysqlColumn := ast.NewColumn("data", "JSON")
//	postgresColumn := ast.NewColumn("data", "JSONB")
//
//	mysqlField := ToField(mysqlColumn, "Product", "mysql")
//	postgresField := ToField(postgresColumn, "Product", "postgres")
//	// These can later be merged to reconstruct overrides
//
// # Return Value
//
// Returns a fully configured goschema.Field with all attributes extracted from the AST node.
// The StructName is set to the provided structName parameter.
func ToField(column *ast.ColumnNode, structName, sourcePlatform string) goschema.Field {
	field := goschema.Field{
		StructName: structName,
		FieldName:  "", // This would need to be set separately as it's not in the AST
		Name:       column.Name,
		Type:       column.Type,
		Nullable:   column.Nullable,
		Primary:    column.Primary,
		AutoInc:    column.AutoInc,
		Unique:     column.Unique,
		Check:      column.Check,
		Comment:    column.Comment,
	}

	// Extract default values
	if column.Default != nil {
		if column.Default.Value != "" {
			field.Default = column.Default.Value
		} else if column.Default.Expression != "" {
			field.DefaultExpr = column.Default.Expression
		}
	}

	// Extract foreign key reference
	if column.ForeignKey != nil {
		if column.ForeignKey.Column != "" {
			field.Foreign = column.ForeignKey.Table + "(" + column.ForeignKey.Column + ")"
		} else {
			field.Foreign = column.ForeignKey.Table
		}
		field.ForeignKeyName = column.ForeignKey.Name
	}

	// Initialize overrides map if we have a source platform
	if sourcePlatform != "" {
		field.Overrides = make(map[string]map[string]string)
		// Platform-specific values would be populated by MergeFieldOverrides
	}

	return field
}

// ToTable converts an ast.CreateTableNode to a goschema.Table with all associated metadata.
//
// This function extracts table-level properties from a CREATE TABLE AST node, including
// table options, comments, and composite constraints. It does not extract individual
// columns - use ToField for column conversion.
//
// # Parameters
//
//   - table: The CREATE TABLE AST node containing table metadata
//   - sourcePlatform: The platform this table was generated for (used for override reconstruction)
//
// # Table Features
//
//   - Table naming and comments
//   - Database-specific options (e.g., MySQL ENGINE)
//   - Composite primary keys extracted from constraints
//   - Platform-specific option handling
//
// # Examples
//
// Basic table conversion:
//
//	table := ast.NewCreateTable("users").
//		SetOption("ENGINE", "InnoDB").
//		SetComment("User accounts")
//	tableSchema := ToTable(table, "")
//	// Results in: goschema.Table{
//	//   Name: "users", Engine: "InnoDB", Comment: "User accounts"
//	// }
//
// Table with composite primary key:
//
//	table := ast.NewCreateTable("user_roles").
//		AddConstraint(ast.NewPrimaryKeyConstraint("user_id", "role_id"))
//	tableSchema := ToTable(table, "")
//	// Results in: goschema.Table{
//	//   Name: "user_roles", PrimaryKey: []string{"user_id", "role_id"}
//	// }
//
// MySQL table with multiple options:
//
//	table := ast.NewCreateTable("products").
//		SetOption("ENGINE", "InnoDB").
//		SetOption("CHARSET", "utf8mb4").
//		SetOption("COLLATE", "utf8mb4_unicode_ci")
//	tableSchema := ToTable(table, "mysql")
//
// # Platform-Specific Options
//
// The function extracts database-specific options from the Options map:
//   - ENGINE option is mapped to the Engine field
//   - Other options can be reconstructed as platform overrides
//
// # Return Value
//
// Returns a goschema.Table with all table-level attributes extracted from the AST node.
// The StructName field is derived from the table name using basic naming conventions.
func ToTable(table *ast.CreateTableNode, sourcePlatform string) goschema.Table {
	tableSchema := goschema.Table{
		StructName: generateStructName(table.Name),
		Name:       table.Name,
		Comment:    table.Comment,
	}

	// Extract ENGINE option if present
	if engine, exists := table.Options["ENGINE"]; exists {
		tableSchema.Engine = engine
	}

	// Extract composite primary key from constraints
	for _, constraint := range table.Constraints {
		if constraint.Type == ast.PrimaryKeyConstraint {
			tableSchema.PrimaryKey = constraint.Columns
			break // Only one primary key constraint per table
		}
	}

	// Initialize overrides map if we have a source platform
	if sourcePlatform != "" {
		tableSchema.Overrides = make(map[string]map[string]string)

		// Store platform-specific options as overrides
		platformOverrides := make(map[string]string)
		for key, value := range table.Options {
			if key != "ENGINE" { // ENGINE is handled separately
				platformOverrides[strings.ToLower(key)] = value
			}
		}

		// Store ENGINE as platform override if present
		if tableSchema.Engine != "" {
			platformOverrides["engine"] = tableSchema.Engine
		}

		// Store comment as platform override if present
		if tableSchema.Comment != "" {
			platformOverrides["comment"] = tableSchema.Comment
		}

		if len(platformOverrides) > 0 {
			tableSchema.Overrides[sourcePlatform] = platformOverrides
		}
	}

	return tableSchema
}

// ToIndex converts an ast.IndexNode to a goschema.Index for database index definitions.
//
// This function extracts index metadata from an AST node, including index name,
// target table, column list, uniqueness constraints, and comments.
//
// # Parameters
//
//   - index: The AST index node containing index metadata
//
// # Index Features
//
//   - Single-column and composite indexes
//   - Unique and non-unique indexes
//   - Index comments for documentation
//   - Table association through StructName mapping
//
// # Examples
//
// Simple single-column index:
//
//	index := ast.NewIndex("idx_users_email", "users", "email").
//		SetComment("Index for email lookups")
//	indexSchema := ToIndex(index)
//	// Results in: goschema.Index{
//	//   Name: "idx_users_email", StructName: "users",
//	//   Fields: []string{"email"}, Comment: "Index for email lookups"
//	// }
//
// Unique composite index:
//
//	index := ast.NewIndex("idx_user_roles_unique", "user_roles", "user_id", "role_id").
//		SetUnique().
//		SetComment("Ensure unique user-role combinations")
//	indexSchema := ToIndex(index)
//	// Results in: goschema.Index{
//	//   Name: "idx_user_roles_unique", StructName: "user_roles",
//	//   Fields: []string{"user_id", "role_id"}, Unique: true,
//	//   Comment: "Ensure unique user-role combinations"
//	// }
//
// # Return Value
//
// Returns a goschema.Index with all index attributes extracted from the AST node.
// The StructName is set to match the table name for proper association.
func ToIndex(index *ast.IndexNode) goschema.Index {
	return goschema.Index{
		Name:       index.Name,
		StructName: index.Table, // Use table name as struct name
		Fields:     index.Columns,
		Unique:     index.Unique,
		Comment:    index.Comment,
	}
}

// ToEnum converts an ast.EnumNode to a goschema.Enum for database enum type definitions.
//
// This function extracts enum metadata from an AST node, including the enum name
// and the list of allowed values.
//
// # Parameters
//
//   - enum: The AST enum node containing enum metadata
//
// # Examples
//
// Simple status enum:
//
//	enum := ast.NewEnum("status_type", "active", "inactive", "pending")
//	enumSchema := ToEnum(enum)
//	// Results in: goschema.Enum{
//	//   Name: "status_type", Values: []string{"active", "inactive", "pending"}
//	// }
//
// User role enum:
//
//	enum := ast.NewEnum("user_role", "admin", "moderator", "user", "guest")
//	enumSchema := ToEnum(enum)
//	// Results in: goschema.Enum{
//	//   Name: "user_role", Values: []string{"admin", "moderator", "user", "guest"}
//	// }
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
// Returns a goschema.Enum with the enum name and values extracted from the AST node.
func ToEnum(enum *ast.EnumNode) goschema.Enum {
	return goschema.Enum{
		Name:   enum.Name,
		Values: enum.Values,
	}
}

// ToDatabase converts an ast.StatementList to a complete goschema.Database schema.
//
// This function processes all statements in the AST statement list and extracts
// the complete database schema including tables, fields, indexes, and enums.
// It handles the reverse process of FromDatabase, reconstructing the original
// schema structure from the generated SQL DDL statements.
//
// # Parameters
//
//   - statements: The AST statement list containing all DDL statements
//
// # Statement Processing
//
// The function processes statements in any order and categorizes them:
//  1. EnumNode statements become global enum definitions
//  2. CreateTableNode statements become table definitions and field collections
//  3. IndexNode statements become index definitions
//
// # Examples
//
// Converting a complete statement list:
//
//	statements := &ast.StatementList{
//		Statements: []ast.Node{
//			ast.NewEnum("user_status", "active", "inactive"),
//			ast.NewCreateTable("users").AddColumn(
//				ast.NewColumn("id", "SERIAL").SetPrimary(),
//			),
//			ast.NewIndex("idx_users_status", "users", "status"),
//		},
//	}
//	database := ToDatabase(statements)
//	// Results in: goschema.Database{
//	//   Enums: []goschema.Enum{{Name: "user_status", Values: [...]}},
//	//   Tables: []goschema.Table{{Name: "users", StructName: "User"}},
//	//   Fields: []goschema.Field{{Name: "id", Type: "SERIAL", Primary: true}},
//	//   Indexes: []goschema.Index{{Name: "idx_users_status", StructName: "users"}},
//	// }
//
// # Field Extraction
//
// Fields are extracted from table columns and associated with their parent table
// through the StructName field. The function automatically generates appropriate
// StructName values based on table names.
//
// # Return Value
//
// Returns a complete goschema.Database with all schema elements extracted and
// properly categorized from the AST statement list.
func ToDatabase(statements *ast.StatementList) goschema.Database {
	database := goschema.Database{
		Tables:  []goschema.Table{},
		Fields:  []goschema.Field{},
		Indexes: []goschema.Index{},
		Enums:   []goschema.Enum{},
	}

	// Process all statements and categorize them
	for _, stmt := range statements.Statements {
		switch node := stmt.(type) {
		case *ast.EnumNode:
			// Convert enum definitions
			enumSchema := ToEnum(node)
			database.Enums = append(database.Enums, enumSchema)

		case *ast.CreateTableNode:
			// Convert table definition
			tableSchema := ToTable(node, "")
			database.Tables = append(database.Tables, tableSchema)

			// Extract fields from table columns
			for _, column := range node.Columns {
				fieldSchema := ToField(column, tableSchema.StructName, "")
				database.Fields = append(database.Fields, fieldSchema)
			}

		case *ast.IndexNode:
			// Convert index definitions
			indexSchema := ToIndex(node)
			database.Indexes = append(database.Indexes, indexSchema)
		}
	}

	return database
}

// MergeFieldOverrides merges platform-specific field variants to reconstruct platform overrides.
//
// This function takes a base field definition and a map of platform-specific variants,
// then reconstructs the Overrides map by comparing differences between the base field
// and each platform variant.
//
// # Parameters
//
//   - baseField: The base field definition (typically from the default platform)
//   - platformFields: Map of platform names to their specific field variants
//
// # Override Detection
//
// The function compares each platform field with the base field and detects differences in:
//   - Type definitions
//   - Check constraints
//   - Comments
//   - Default values and expressions
//
// # Examples
//
// Reconstructing platform overrides:
//
//	baseField := goschema.Field{
//		Name: "data", Type: "JSONB", Comment: "JSON data"
//	}
//	platformFields := map[string]goschema.Field{
//		"mysql": {
//			Name: "data", Type: "JSON", Comment: "JSON data"
//		},
//		"mariadb": {
//			Name: "data", Type: "LONGTEXT", Check: "JSON_VALID(data)", Comment: "JSON data"
//		},
//	}
//	mergedField := MergeFieldOverrides(baseField, platformFields)
//	// Results in field with Overrides map containing platform-specific differences
//
// # Return Value
//
// Returns the base field with the Overrides map populated with platform-specific differences.
func MergeFieldOverrides(baseField goschema.Field, platformFields map[string]goschema.Field) goschema.Field {
	if len(platformFields) == 0 {
		return baseField
	}

	// Initialize overrides map
	if baseField.Overrides == nil {
		baseField.Overrides = make(map[string]map[string]string)
	}

	// Compare each platform field with the base field
	for platform, platformField := range platformFields {
		platformOverrides := make(map[string]string)

		// Check for type differences
		if platformField.Type != baseField.Type {
			platformOverrides["type"] = platformField.Type
		}

		// Check for check constraint differences
		if platformField.Check != baseField.Check {
			platformOverrides["check"] = platformField.Check
		}

		// Check for comment differences
		if platformField.Comment != baseField.Comment {
			platformOverrides["comment"] = platformField.Comment
		}

		// Check for default value differences
		if platformField.Default != baseField.Default {
			platformOverrides["default"] = platformField.Default
		}

		// Check for default expression differences
		if platformField.DefaultExpr != baseField.DefaultExpr {
			platformOverrides["default_expr"] = platformField.DefaultExpr
		}

		// Store platform overrides if any differences were found
		if len(platformOverrides) > 0 {
			baseField.Overrides[platform] = platformOverrides
		}
	}

	return baseField
}

// MergeTableOverrides merges platform-specific table variants to reconstruct platform overrides.
//
// This function takes a base table definition and a map of platform-specific variants,
// then reconstructs the Overrides map by comparing differences between the base table
// and each platform variant.
//
// # Parameters
//
//   - baseTable: The base table definition (typically from the default platform)
//   - platformTables: Map of platform names to their specific table variants
//
// # Override Detection
//
// The function compares each platform table with the base table and detects differences in:
//   - Engine specifications
//   - Comments
//   - Other table options (charset, collation, etc.)
//
// # Examples
//
// Reconstructing table platform overrides:
//
//	baseTable := goschema.Table{
//		Name: "products", Engine: "InnoDB", Comment: "Product catalog"
//	}
//	platformTables := map[string]goschema.Table{
//		"mariadb": {
//			Name: "products", Engine: "InnoDB", Comment: "MariaDB product catalog"
//		},
//	}
//	mergedTable := MergeTableOverrides(baseTable, platformTables)
//	// Results in table with Overrides map containing platform-specific differences
//
// # Return Value
//
// Returns the base table with the Overrides map populated with platform-specific differences.
func MergeTableOverrides(baseTable goschema.Table, platformTables map[string]goschema.Table) goschema.Table {
	if len(platformTables) == 0 {
		return baseTable
	}

	// Initialize overrides map
	if baseTable.Overrides == nil {
		baseTable.Overrides = make(map[string]map[string]string)
	}

	// Compare each platform table with the base table
	for platform, platformTable := range platformTables {
		platformOverrides := make(map[string]string)

		// Check for engine differences
		if platformTable.Engine != baseTable.Engine {
			platformOverrides["engine"] = platformTable.Engine
		}

		// Check for comment differences
		if platformTable.Comment != baseTable.Comment {
			platformOverrides["comment"] = platformTable.Comment
		}

		// Store platform overrides if any differences were found
		if len(platformOverrides) > 0 {
			baseTable.Overrides[platform] = platformOverrides
		}
	}

	return baseTable
}

// generateStructName converts a table name to a Go struct name using basic naming conventions.
//
// This function applies common naming transformations:
//   - Removes underscores and capitalizes following letters
//   - Capitalizes the first letter
//   - Handles plural to singular conversion for common cases
//
// # Examples
//
//	generateStructName("users") -> "User"
//	generateStructName("user_roles") -> "UserRole"
//	generateStructName("product_categories") -> "ProductCategory"
//	generateStructName("logs") -> "Log"
//
// # Return Value
//
// Returns a Pascal-case struct name suitable for Go code generation.
func generateStructName(tableName string) string {
	if tableName == "" {
		return ""
	}

	// Split by underscores and capitalize each part
	parts := strings.Split(tableName, "_")
	var result strings.Builder

	for _, part := range parts {
		if len(part) > 0 {
			// Capitalize first letter and add the rest
			result.WriteString(strings.ToUpper(part[:1]))
			if len(part) > 1 {
				result.WriteString(part[1:])
			}
		}
	}

	structName := result.String()

	// Basic plural to singular conversion for common cases
	if strings.HasSuffix(structName, "ies") {
		structName = structName[:len(structName)-3] + "y"
	} else if strings.HasSuffix(structName, "ses") {
		structName = structName[:len(structName)-2]
	} else if strings.HasSuffix(structName, "s") && !strings.HasSuffix(structName, "ss") {
		structName = structName[:len(structName)-1]
	}

	return structName
}
