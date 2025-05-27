package constants

const (
	SepComma       = ","
	SepNL          = "\n"
	SepSpace       = " "
	SepTab         = "\t"
	SepSemicolon   = ";"
	SepCommaSpace  = ", "
	SepCommaNL     = ",\n"
	SepSemicolonNL = ";\n"
)

const (
	PartialTableCommentSQL = "-- %s TABLE: %s --" + SepNL
	PartialAlterCommentSQL = "-- ALTER statements: --" + SepNL

	PartialPostgresCreateEnumTypeSQL = `CREATE TYPE %s AS ENUM (%s);` + SepNL
	PartialMariaDBMysqlEnumSQL       = "ENUM(%s)"

	PartialCreateTableBeginSQL            = `CREATE TABLE %s (` + SepNL
	PartialConstraintForeignKeySQL        = `  CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s`
	PartialPrimaryKeySQL                  = " PRIMARY KEY"
	PartialUniqueSQL                      = " UNIQUE"
	PartialNotNull                        = " NOT NULL"
	PartialPrimaryKeyValueSQL             = `  PRIMARY KEY (%s)`
	PartialDefaultSQL                     = " DEFAULT '%s'"
	PartialDefaultFnSQL                   = " DEFAULT %s"
	PartialCheckSQL                       = " CHECK (%s)"
	PartialClosingBracketWithSemicolonSQL = ");"
	PartialClosingBracketSQL              = ")"

	PartialIndexCreateSQL = "CREATE"
	PartialIndexUniqueSQL = " UNIQUE"
	PartialIndexBodySQL   = " INDEX %s ON %s (%s);"

	PartialAlterColumnTypeSQL  = `ALTER TABLE %s ALTER COLUMN %s TYPE %s;` + SepNL
	PartialAlterDropNotNullSQL = `ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;` + SepNL
	PartialAlterSetNotNullSQL  = `ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;` + SepNL
	PartialAlterAddColumnSQL   = `ALTER TABLE %s ADD COLUMN %s %s;` + SepNL
)

const (
	PlatformTypePostgres = "postgres"
	PlatformTypeMySQL    = "mysql"
	PlatformTypeMariaDB  = "mariadb"
)
