package testutils

import (
	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/dbschema/types"
)

// CreateTestParseResult creates a minimal PackageParseResult for testing
func CreateTestParseResult() *goschema.Database {
	return &goschema.Database{
		Tables: []goschema.Table{
			{Name: "test_table", StructName: "TestTable"},
		},
		Fields: []goschema.Field{
			{Name: "id", Type: "int", StructName: "TestTable"},
			{Name: "name", Type: "string", StructName: "TestTable"},
		},
		Indexes: []goschema.Index{},
		Enums: []goschema.Enum{
			{Name: "test_status", Values: []string{"active", "inactive"}},
		},
		EmbeddedFields: []goschema.EmbeddedField{},
	}
}

func FindColumn(columns []types.DBColumn, name string) *types.DBColumn {
	for i := range columns {
		if columns[i].Name == name {
			return &columns[i]
		}
	}
	return nil
}
