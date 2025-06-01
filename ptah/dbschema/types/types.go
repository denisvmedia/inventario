package types

// DBSchema represents the complete schema read from a database
type DBSchema struct {
	Tables      []DBTable      `json:"tables"`
	Enums       []DBEnum       `json:"enums"`
	Indexes     []DBIndex      `json:"indexes"`
	Constraints []DBConstraint `json:"constraints"`
}

// DBTable represents a database table
type DBTable struct {
	Name    string     `json:"name"`
	Type    string     `json:"type"` // TABLE, VIEW, etc.
	Comment string     `json:"comment"`
	Columns []DBColumn `json:"columns"`
}

// DBColumn represents a database column
type DBColumn struct {
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

// DBEnum represents a database enum type (PostgreSQL)
type DBEnum struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// DBIndex represents a database index
type DBIndex struct {
	Name       string   `json:"name"`
	TableName  string   `json:"table_name"`
	Columns    []string `json:"columns"`
	IsUnique   bool     `json:"is_unique"`
	IsPrimary  bool     `json:"is_primary"`
	Definition string   `json:"definition"` // Full index definition
}

// DBConstraint represents a database constraint
type DBConstraint struct {
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

// DBInfo contains connection and metadata information
type DBInfo struct {
	Dialect string `json:"dialect"` // postgres, mysql, mariadb
	Version string `json:"version"`
	Schema  string `json:"schema"` // public, database name, etc.
	URL     string `json:"url"`    // database connection URL (for reference)
}

// SchemaReader interface for reading database schemas
type SchemaReader interface {
	ReadSchema() (*DBSchema, error)
}

// SchemaWriter interface for writing schemas to databases
type SchemaWriter interface {
	DropAllTables() error
	ExecuteSQL(sql string) error
	BeginTransaction() error
	CommitTransaction() error
	RollbackTransaction() error
	SetDryRun(dryRun bool)
	IsDryRun() bool
}
