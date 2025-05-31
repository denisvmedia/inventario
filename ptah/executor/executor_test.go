package executor

import (
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// createTestParseResult creates a minimal PackageParseResult for testing
func createTestParseResult() *parsertypes.PackageParseResult {
	return &parsertypes.PackageParseResult{
		Tables: []types.TableDirective{
			{Name: "test_table", StructName: "TestTable"},
		},
		Fields: []types.SchemaField{
			{Name: "id", Type: "int", StructName: "TestTable"},
			{Name: "name", Type: "string", StructName: "TestTable"},
		},
		Indexes: []types.SchemaIndex{},
		Enums: []types.GlobalEnum{
			{Name: "test_status", Values: []string{"active", "inactive"}},
		},
		EmbeddedFields: []types.EmbeddedField{},
	}
}
