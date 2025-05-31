// Package types defines the core data structures used throughout the Ptah schema migration system.
// These types represent the intermediate representation of database schema elements parsed from
// Go struct annotations and used for generating database-specific migration SQL.
package types

// EmbeddedField represents an embedded field in a Go struct that should be handled specially
// during schema generation. Embedded fields allow for composition and reuse of common field
// patterns across multiple tables.
//
// The EmbeddedField supports four different modes of handling:
//   - "inline": Injects the embedded struct's fields directly as separate columns
//   - "json": Serializes the entire embedded struct into a single JSON/JSONB column
//   - "relation": Creates a foreign key relationship to another table
//   - "skip": Completely ignores the embedded field during schema generation
//
// Usage in Go structs:
//
//	type User struct {
//	    ID int64
//	    //migrator:embedded mode="inline"
//	    Timestamps  // Results in: created_at, updated_at columns
//
//	    //migrator:embedded mode="json" name="metadata" type="JSONB"
//	    Meta UserMeta  // Results in: metadata JSONB column
//
//	    //migrator:embedded mode="relation" field="company_id" ref="companies(id)"
//	    Company Company  // Results in: company_id INTEGER + FK constraint
//	}
type EmbeddedField struct {
	StructName       string                       // The struct that contains this embedded field
	Mode             string                       // inline, json, relation, skip
	Prefix           string                       // For inline mode - prefix for field names
	Name             string                       // For json mode - column name
	Type             string                       // For json mode - column type (JSON/JSONB)
	Nullable         bool                         // Whether the field can be null
	Index            bool                         // Whether to create an index
	Field            string                       // For relation mode - foreign key field name
	Ref              string                       // For relation mode - reference table(column)
	OnDelete         string                       // For relation mode - ON DELETE action
	OnUpdate         string                       // For relation mode - ON UPDATE action
	Comment          string                       // Comment for the field/column
	EmbeddedTypeName string                       // The name of the embedded type (e.g., "Timestamps")
	Overrides        map[string]map[string]string // Platform-specific overrides
}

// SchemaField represents a database column/field definition parsed from Go struct field annotations.
// This is the core building block for table schema generation, containing all the metadata
// needed to generate appropriate CREATE TABLE column definitions for different database platforms.
//
// SchemaField is created by parsing //migrator:schema:field annotations from Go struct fields:
//
//	type Product struct {
//	    //migrator:schema:field name="id" type="SERIAL" primary="true"
//	    ID int64
//
//	    //migrator:schema:field name="name" type="VARCHAR(255)" not_null="true" unique="true"
//	    Name string
//
//	    //migrator:schema:field name="price" type="DECIMAL(10,2)" check="price > 0" default="0.00"
//	    Price float64
//
//	    //migrator:schema:field name="status" type="ENUM" enum="active,inactive" default="active"
//	    Status string
//
//	    //migrator:schema:field name="category_id" type="INTEGER" foreign="categories(id)"
//	    CategoryID int64
//	}
//
// The SchemaField supports platform-specific overrides through the Overrides field:
//
//	//migrator:schema:field name="id" type="SERIAL" platform.mysql.type="INT AUTO_INCREMENT"
//	ID int64
type SchemaField struct {
	StructName     string                       // Name of the Go struct this field belongs to
	FieldName      string                       // Name of the Go struct field
	Name           string                       // Database column name
	Type           string                       // Database column type (e.g., "VARCHAR(255)", "INTEGER")
	Nullable       bool                         // Whether the column allows NULL values
	Primary        bool                         // Whether this is a primary key column
	AutoInc        bool                         // Whether this column auto-increments
	Unique         bool                         // Whether this column has a unique constraint
	UniqueExpr     string                       // Custom unique constraint expression
	Default        string                       // Default value for the column
	DefaultExpr    string                       // Default expression (e.g., "NOW()", "UUID()", "CURRENT_TIMESTAMP", "1", "true")
	Foreign        string                       // Foreign key reference (e.g., "users(id)")
	ForeignKeyName string                       // Custom foreign key constraint name
	Enum           []string                     // Enum values for ENUM type fields
	Check          string                       // Check constraint expression
	Comment        string                       // Column comment
	Overrides      map[string]map[string]string // Platform-specific overrides (e.g., platform.mysql.type)
}

// SchemaIndex represents a database index definition parsed from Go struct annotations.
// Indexes are used to improve query performance and enforce uniqueness constraints
// on one or more columns.
//
// SchemaIndex is created by parsing //migrator:schema:index annotations:
//
//	type User struct {
//	    //migrator:schema:field name="id" type="SERIAL" primary="true"
//	    ID int64
//
//	    //migrator:schema:field name="email" type="VARCHAR(255)" not_null="true"
//	    Email string
//
//	    //migrator:schema:field name="status" type="VARCHAR(50)"
//	    Status string
//
//	    // Single column index
//	    //migrator:schema:index name="idx_users_email" fields="email" unique="true"
//	    _ int
//
//	    // Multi-column index
//	    //migrator:schema:index name="idx_users_email_status" fields="email,status"
//	    _ int
//	}
type SchemaIndex struct {
	StructName string   // Name of the Go struct this index belongs to
	Name       string   // Index name (e.g., "idx_users_email")
	Fields     []string // Column names included in the index
	Unique     bool     // Whether this is a unique index
	Comment    string   // Index comment/description
}

// TableDirective represents a database table configuration parsed from Go struct annotations.
// This defines the overall table properties and metadata that will be used to generate
// CREATE TABLE statements.
//
// TableDirective is created by parsing //migrator:schema:table annotations:
//
//	//migrator:schema:table name="users" comment="User accounts table"
//	type User struct {
//	    //migrator:schema:field name="id" type="SERIAL" primary="true"
//	    ID int64
//
//	    //migrator:schema:field name="email" type="VARCHAR(255)" not_null="true"
//	    Email string
//	}
//
// Platform-specific configurations can be specified using overrides:
//
//	//migrator:schema:table name="products" platform.mysql.engine="InnoDB" platform.mysql.comment="Product catalog"
//	type Product struct {
//	    // ... fields
//	}
//
// Composite primary keys can be defined using the primary_key attribute:
//
//	//migrator:schema:table name="user_roles" primary_key="user_id,role_id"
//	type UserRole struct {
//	    //migrator:schema:field name="user_id" type="INTEGER" foreign="users(id)"
//	    UserID int64
//
//	    //migrator:schema:field name="role_id" type="INTEGER" foreign="roles(id)"
//	    RoleID int64
//	}
type TableDirective struct {
	StructName string                       // Name of the Go struct this table represents
	Name       string                       // Database table name
	Engine     string                       // Storage engine (MySQL/MariaDB specific, e.g., "InnoDB")
	Comment    string                       // Table comment/description
	PrimaryKey []string                     // Composite primary key column names
	Checks     []string                     // Table-level check constraints
	CustomSQL  string                       // Custom SQL to append to CREATE TABLE
	Overrides  map[string]map[string]string // Platform-specific overrides
}

// GlobalEnum represents a global enumeration type definition that can be shared across
// multiple tables and fields. Global enums are automatically generated when ENUM type
// fields are defined in struct annotations.
//
// What makes an enum "global":
// Global enums are database-level type definitions (particularly in PostgreSQL) that can be
// referenced by multiple tables and columns. Unlike inline enum constraints, global enums:
//   - Are created once as a database type (CREATE TYPE ... AS ENUM in PostgreSQL)
//   - Can be reused across multiple tables and columns
//   - Provide better type safety and consistency
//   - Allow for easier maintenance when enum values need to be modified
//
// How global enums are created:
// When you define a field with type="ENUM" and enum values, Ptah automatically generates
// a global enum with a standardized name pattern: "enum_{struct_name}_{field_name}":
//
//	type User struct {
//	    //migrator:schema:field name="status" type="ENUM" enum="active,inactive,suspended" default="active"
//	    Status string  // Creates global enum: "enum_user_status"
//	}
//
//	type Post struct {
//	    //migrator:schema:field name="status" type="ENUM" enum="draft,published,archived" default="draft"
//	    Status string  // Creates global enum: "enum_post_status"
//	}
//
// Database platform differences:
//   - PostgreSQL: Creates actual ENUM types (CREATE TYPE enum_user_status AS ENUM ('active', 'inactive'))
//   - MySQL/MariaDB: Uses ENUM column type with values (status ENUM('active', 'inactive'))
//   - SQLite: Uses CHECK constraints with IN clauses (status TEXT CHECK (status IN ('active', 'inactive')))
//
// Example of generated SQL:
//
//	PostgreSQL:
//	  CREATE TYPE enum_user_status AS ENUM ('active', 'inactive', 'suspended');
//	  CREATE TABLE users (status enum_user_status DEFAULT 'active');
//
//	MySQL:
//	  CREATE TABLE users (status ENUM('active', 'inactive', 'suspended') DEFAULT 'active');
type GlobalEnum struct {
	Name   string   // The generated enum type name (e.g., "enum_user_status")
	Values []string // The allowed enum values (e.g., ["active", "inactive", "suspended"])
}
