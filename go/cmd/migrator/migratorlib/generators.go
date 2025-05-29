package migratorlib

import (
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dialects/generic"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dialects/mariadb"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dialects/mysql"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dialects/postgresql"
	"github.com/denisvmedia/inventario/ptah/platform"
	"github.com/denisvmedia/inventario/ptah/schema/meta"
)

// DialectGenerator defines the interface for database-specific SQL generation
type DialectGenerator interface {
	// GenerateCreateTable generates CREATE TABLE SQL for the specific dialect
	GenerateCreateTable(table meta.TableDirective, fields []meta.SchemaField, indexes []meta.SchemaIndex, enums []meta.GlobalEnum) string

	// GenerateCreateTableWithEmbedded generates CREATE TABLE SQL with embedded field support
	GenerateCreateTableWithEmbedded(table meta.TableDirective, fields []meta.SchemaField, indexes []meta.SchemaIndex, enums []meta.GlobalEnum, embeddedFields []meta.EmbeddedField) string

	// GenerateAlterStatements generates ALTER statements for the specific dialect
	GenerateAlterStatements(oldFields, newFields []meta.SchemaField) string

	// GetDialectName returns the name of the dialect
	GetDialectName() string
}

// GetDialectGenerator returns the appropriate dialect generator for the given dialect name
func GetDialectGenerator(dialect string) DialectGenerator {
	switch dialect {
	case platform.PlatformTypePostgres:
		return postgresql.New()
	case platform.PlatformTypeMySQL:
		return mysql.New()
	case platform.PlatformTypeMariaDB:
		return mariadb.New()
	default:
		// For unknown dialects, use a generic generator that doesn't apply dialect-specific transformations
		return generic.New(dialect)
	}
}

// GenerateCreateTable generates CREATE TABLE SQL for the given dialect (backward compatibility function)
func GenerateCreateTable(table meta.TableDirective, fields []meta.SchemaField, indexes []meta.SchemaIndex, enums []meta.GlobalEnum, dialect string) string {
	generator := GetDialectGenerator(dialect)
	return generator.GenerateCreateTable(table, fields, indexes, enums)
}

// GenerateCreateTableWithEmbedded generates CREATE TABLE SQL with embedded field support
func GenerateCreateTableWithEmbedded(table meta.TableDirective, fields []meta.SchemaField, indexes []meta.SchemaIndex, enums []meta.GlobalEnum, embeddedFields []meta.EmbeddedField, dialect string) string {
	generator := GetDialectGenerator(dialect)
	return generator.GenerateCreateTableWithEmbedded(table, fields, indexes, enums, embeddedFields)
}

// GenerateAlterStatements generates ALTER statements for the given dialect (backward compatibility function)
func GenerateAlterStatements(oldFields, newFields []meta.SchemaField) string {
	// For backward compatibility, use PostgreSQL dialect as default
	// In the future, this function should accept a dialect parameter
	generator := GetDialectGenerator(platform.PlatformTypePostgres)
	return generator.GenerateAlterStatements(oldFields, newFields)
}
