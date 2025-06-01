package executor

import (
	"github.com/denisvmedia/inventario/ptah/core/goschema"
)

// createTestParseResult creates a minimal PackageParseResult for testing
func createTestParseResult() *goschema.Database {
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
