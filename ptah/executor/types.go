package executor

import (
	"github.com/denisvmedia/inventario/ptah/dbschema/types"
)

// SchemaReader interface for reading database schemas
type SchemaReader interface {
	ReadSchema() (*types.DBSchema, error)
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
