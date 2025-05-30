package executor

import (
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
)

// SchemaReader interface for reading database schemas
type SchemaReader interface {
	ReadSchema() (*parsertypes.DatabaseSchema, error)
}

// SchemaWriter interface for writing schemas to databases
type SchemaWriter interface {
	WriteSchema(result *parsertypes.PackageParseResult) error
	DropSchema(result *parsertypes.PackageParseResult) error
	DropAllTables() error
	ExecuteSQL(sql string) error
	BeginTransaction() error
	CommitTransaction() error
	RollbackTransaction() error
	CheckSchemaExists(result *parsertypes.PackageParseResult) ([]string, error)
}
