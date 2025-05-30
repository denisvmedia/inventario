package parsertypes

import (
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// PackageParseResult contains all parsed entities from the entire project.
//
// This struct aggregates all database schema information discovered during the recursive
// parsing process. It includes all entity types, their relationships, and dependency
// information needed for proper migration generation.
//
// The result is processed to:
//   - Remove duplicates that may occur when entities are defined in multiple files
//   - Build dependency graphs based on foreign key relationships
//   - Sort tables in topological order to ensure proper creation sequence
//
// Fields:
//   - Tables: All table directives found in the project
//   - Fields: All field definitions with their database mappings
//   - Indexes: All index definitions for database optimization
//   - Enums: Global enum definitions that can be referenced by fields
//   - EmbeddedFields: Fields from embedded structs with their relation modes
//   - Dependencies: Dependency graph mapping table names to their dependencies
type PackageParseResult struct {
	Tables         []types.TableDirective
	Fields         []types.SchemaField
	Indexes        []types.SchemaIndex
	Enums          []types.GlobalEnum
	EmbeddedFields []types.EmbeddedField
	Dependencies   map[string][]string // table -> list of tables it depends on
}

// DatabaseSchema represents the complete schema read from a database
type DatabaseSchema struct {
	Tables      []Table      `json:"tables"`
	Enums       []Enum       `json:"enums"`
	Indexes     []Index      `json:"indexes"`
	Constraints []Constraint `json:"constraints"`
}

// Table represents a database table
type Table struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"` // TABLE, VIEW, etc.
	Comment string   `json:"comment"`
	Columns []Column `json:"columns"`
}

// Column represents a database column
type Column struct {
	Name               string  `json:"name"`
	DataType           string  `json:"data_type"`
	UDTName            string  `json:"udt_name"`             // For PostgreSQL enum types
	ColumnType         string  `json:"column_type"`          // For MySQL ENUM syntax
	IsNullable         string  `json:"is_nullable"`          // YES/NO
	ColumnDefault      *string `json:"column_default"`       // Can be NULL
	CharacterMaxLength *int    `json:"character_max_length"` // For VARCHAR, etc.
	NumericPrecision   *int    `json:"numeric_precision"`    // For DECIMAL, etc.
	NumericScale       *int    `json:"numeric_scale"`        // For DECIMAL, etc.
	OrdinalPosition    int     `json:"ordinal_position"`
	IsAutoIncrement    bool    `json:"is_auto_increment"` // Derived field
	IsPrimaryKey       bool    `json:"is_primary_key"`    // Derived field
	IsUnique           bool    `json:"is_unique"`         // Derived field
}

// Enum represents a database enum type (PostgreSQL)
type Enum struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// Index represents a database index
type Index struct {
	Name       string   `json:"name"`
	TableName  string   `json:"table_name"`
	Columns    []string `json:"columns"`
	IsUnique   bool     `json:"is_unique"`
	IsPrimary  bool     `json:"is_primary"`
	Definition string   `json:"definition"` // Full index definition
}

// Constraint represents a database constraint
type Constraint struct {
	Name          string  `json:"name"`
	TableName     string  `json:"table_name"`
	Type          string  `json:"type"` // PRIMARY KEY, FOREIGN KEY, UNIQUE, CHECK
	ColumnName    string  `json:"column_name"`
	ForeignTable  *string `json:"foreign_table"`  // For foreign keys
	ForeignColumn *string `json:"foreign_column"` // For foreign keys
	DeleteRule    *string `json:"delete_rule"`    // CASCADE, RESTRICT, etc.
	UpdateRule    *string `json:"update_rule"`    // CASCADE, RESTRICT, etc.
	CheckClause   *string `json:"check_clause"`   // For CHECK constraints
}

// DatabaseInfo contains connection and metadata information
type DatabaseInfo struct {
	Dialect string `json:"dialect"` // postgres, mysql, mariadb
	Version string `json:"version"`
	Schema  string `json:"schema"` // public, database name, etc.
	URL     string `json:"url"`     // database connection URL (for reference)
}
