package planner

import (
	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/core/platform"
	"github.com/denisvmedia/inventario/ptah/core/renderer"
	"github.com/denisvmedia/inventario/ptah/core/sqlutil"
	"github.com/denisvmedia/inventario/ptah/migration/planner/dialects/mysql"
	"github.com/denisvmedia/inventario/ptah/migration/planner/dialects/postgres"
	"github.com/denisvmedia/inventario/ptah/migration/schemadiff/types"
)

type DialectGenerator interface {
	GenerateMigrationAST(diff *types.SchemaDiff, generated *goschema.Database) []ast.Node
}

func GetPlanner(dialect string) DialectGenerator {
	switch dialect {
	case platform.Postgres:
		return postgres.New()
	case platform.MySQL:
		return mysql.New()
	case platform.MariaDB:
		panic("not implemented")
	default:
		// For unknown dialects, use a generic generator that doesn't apply dialect-specific transformations
		panic("not implemented")
	}
}

func GenerateSchemaDiffAST(diff *types.SchemaDiff, generated *goschema.Database, dialect string) []ast.Node {
	planner := GetPlanner(dialect)
	return planner.GenerateMigrationAST(diff, generated)
}

func GenerateSchemaDiffSQLStatements(diff *types.SchemaDiff, generated *goschema.Database, dialect string) []string {
	output := GenerateSchemaDiffSQL(diff, generated, dialect)
	statements := sqlutil.SplitSQLStatements(output)
	return statements
}

func GenerateSchemaDiffSQL(diff *types.SchemaDiff, generated *goschema.Database, dialect string) string {
	astNodes := GenerateSchemaDiffAST(diff, generated, dialect)
	output, err := renderer.RenderSQL(dialect, astNodes...)
	if err != nil {
		panic(err)
	}
	return output
}
