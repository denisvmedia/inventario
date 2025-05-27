package migratorlib

import (
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/constants"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

// Type aliases for backward compatibility
type EmbeddedField = types.EmbeddedField
type SchemaField = types.SchemaField
type SchemaIndex = types.SchemaIndex
type TableDirective = types.TableDirective
type GlobalEnum = types.GlobalEnum

// Constant aliases for backward compatibility
const (
	SepComma       = constants.SepComma
	SepNL          = constants.SepNL
	SepSpace       = constants.SepSpace
	SepTab         = constants.SepTab
	SepSemicolon   = constants.SepSemicolon
	SepCommaSpace  = constants.SepCommaSpace
	SepCommaNL     = constants.SepCommaNL
	SepSemicolonNL = constants.SepSemicolonNL

	PartialTableCommentSQL                = constants.PartialTableCommentSQL
	PartialAlterCommentSQL                = constants.PartialAlterCommentSQL
	PartialPostgresCreateEnumTypeSQL      = constants.PartialPostgresCreateEnumTypeSQL
	PartialMariaDBMysqlEnumSQL            = constants.PartialMariaDBMysqlEnumSQL
	PartialCreateTableBeginSQL            = constants.PartialCreateTableBeginSQL
	PartialConstraintForeignKeySQL        = constants.PartialConstraintForeignKeySQL
	PartialPrimaryKeySQL                  = constants.PartialPrimaryKeySQL
	PartialUniqueSQL                      = constants.PartialUniqueSQL
	PartialNotNull                        = constants.PartialNotNull
	PartialPrimaryKeyValueSQL             = constants.PartialPrimaryKeyValueSQL
	PartialDefaultSQL                     = constants.PartialDefaultSQL
	PartialDefaultFnSQL                   = constants.PartialDefaultFnSQL
	PartialCheckSQL                       = constants.PartialCheckSQL
	PartialClosingBracketWithSemicolonSQL = constants.PartialClosingBracketWithSemicolonSQL
	PartialClosingBracketSQL              = constants.PartialClosingBracketSQL
	PartialIndexCreateSQL                 = constants.PartialIndexCreateSQL
	PartialIndexUniqueSQL                 = constants.PartialIndexUniqueSQL
	PartialIndexBodySQL                   = constants.PartialIndexBodySQL
	PartialAlterColumnTypeSQL             = constants.PartialAlterColumnTypeSQL
	PartialAlterDropNotNullSQL            = constants.PartialAlterDropNotNullSQL
	PartialAlterSetNotNullSQL             = constants.PartialAlterSetNotNullSQL
	PartialAlterAddColumnSQL              = constants.PartialAlterAddColumnSQL

	PlatformTypePostgres = constants.PlatformTypePostgres
	PlatformTypeMySQL    = constants.PlatformTypeMySQL
	PlatformTypeMariaDB  = constants.PlatformTypeMariaDB
)
