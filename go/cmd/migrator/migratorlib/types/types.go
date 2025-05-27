package types

// EmbeddedField represents an embedded field in a struct
type EmbeddedField struct {
	StructName string
	Mode       string
	Prefix     string
	Name       string
	Type       string
	Nullable   bool
	Index      bool
	Field      string
	Ref        string
	OnDelete   string
	OnUpdate   string
	Comment    string
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
