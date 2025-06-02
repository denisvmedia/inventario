package planner_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/core/platform"
	"github.com/denisvmedia/inventario/ptah/migration/planner"
	"github.com/denisvmedia/inventario/ptah/migration/schemadiff/types"
)

func TestGetPlanner(t *testing.T) {
	tests := []struct {
		name    string
		dialect string
		wantErr bool
	}{
		{
			name:    "postgres planner",
			dialect: platform.Postgres,
			wantErr: false,
		},
		{
			name:    "mysql planner",
			dialect: platform.MySQL,
			wantErr: false,
		},
		{
			name:    "mariadb planner not implemented",
			dialect: platform.MariaDB,
			wantErr: true,
		},
		{
			name:    "unknown dialect",
			dialect: "unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			if tt.wantErr {
				defer func() {
					r := recover()
					c.Assert(r, qt.IsNotNil)
				}()
				planner.GetPlanner(tt.dialect)
				c.Assert(false, qt.IsTrue, qt.Commentf("Expected panic but none occurred"))
			} else {
				plannerInstance := planner.GetPlanner(tt.dialect)
				c.Assert(plannerInstance, qt.IsNotNil)
			}
		})
	}
}

func TestGenerateMigrationAST(t *testing.T) {
	tests := []struct {
		name      string
		dialect   string
		diff      *types.SchemaDiff
		generated *goschema.Database
		wantErr   bool
	}{
		{
			name:    "postgres migration generation",
			dialect: platform.Postgres,
			diff: &types.SchemaDiff{
				TablesAdded: []string{"users"},
			},
			generated: &goschema.Database{
				Tables: []goschema.Table{
					{Name: "users", StructName: "User"},
				},
				Fields: []goschema.Field{
					{Name: "id", Type: "SERIAL", StructName: "User", Primary: true},
				},
			},
			wantErr: false,
		},
		{
			name:    "mysql migration generation",
			dialect: platform.MySQL,
			diff: &types.SchemaDiff{
				TablesAdded: []string{"users"},
			},
			generated: &goschema.Database{
				Tables: []goschema.Table{
					{Name: "users", StructName: "User"},
				},
				Fields: []goschema.Field{
					{Name: "id", Type: "INT", StructName: "User", Primary: true, AutoInc: true},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			if tt.wantErr {
				defer func() {
					r := recover()
					c.Assert(r, qt.IsNotNil)
				}()
				planner.GenerateSchemaDiffAST(tt.diff, tt.generated, tt.dialect)
				c.Assert(false, qt.IsTrue, qt.Commentf("Expected panic but none occurred"))
			} else {
				nodes := planner.GenerateSchemaDiffAST(tt.diff, tt.generated, tt.dialect)
				c.Assert(nodes, qt.IsNotNil)
				c.Assert(len(nodes), qt.Equals, 1) // Should have one CREATE TABLE statement
			}
		})
	}
}
