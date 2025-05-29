package meta

// EmbeddedField represents an embedded field in a struct
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

// SchemaField represents a database field
type SchemaField struct {
	StructName     string
	FieldName      string
	Name           string
	Type           string
	Nullable       bool
	Primary        bool
	AutoInc        bool
	Unique         bool
	UniqueExpr     string
	Default        string
	DefaultFn      string
	Foreign        string
	ForeignKeyName string
	Enum           []string
	Check          string
	Comment        string
	Overrides      map[string]map[string]string
}

// SchemaIndex represents a database index
type SchemaIndex struct {
	StructName string
	Name       string
	Fields     []string
	Unique     bool
	Comment    string
}

// TableDirective represents a table configuration
type TableDirective struct {
	StructName string
	Name       string
	Engine     string
	Comment    string
	PrimaryKey []string
	Checks     []string
	CustomSQL  string
	Overrides  map[string]map[string]string
}

// GlobalEnum represents a global enum definition
type GlobalEnum struct {
	Name   string
	Values []string
}
