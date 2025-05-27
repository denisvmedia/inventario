package generic

import (
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/builders"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dialects/base"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/renderers"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

// Generator handles unknown dialects without applying dialect-specific transformations
type Generator struct {
	*base.Generator
	renderer *renderers.BaseRenderer
}

// New creates a new generic generator
func New(dialectName string) *Generator {
	return &Generator{
		Generator: base.NewGenerator(dialectName),
		renderer:  renderers.NewBaseRenderer(dialectName),
	}
}

// GenerateCreateTable generates CREATE TABLE SQL for unknown dialects using AST
func (g *Generator) GenerateCreateTable(table types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum) string {
	// Use the base generator's schema generation method
	schema := g.GenerateSchema([]types.TableDirective{table}, fields, indexes, enums)

	// Render using the base renderer by iterating through statements
	result, err := g.renderer.Render(schema)
	if err != nil {
		// Fallback to simple rendering if schema rendering fails
		createTableNode := g.Generator.GenerateCreateTable(table, fields, enums)
		simpleResult, _ := g.renderer.Render(createTableNode)
		return simpleResult
	}

	return result
}

// GenerateCreateTableWithEmbedded generates CREATE TABLE SQL for generic dialects with embedded field support
func (g *Generator) GenerateCreateTableWithEmbedded(table types.TableDirective, fields []types.SchemaField, indexes []types.SchemaIndex, enums []types.GlobalEnum, embeddedFields []types.EmbeddedField) string {
	// Process embedded fields to generate additional schema fields
	embeddedGeneratedFields := builders.ProcessEmbeddedFields(embeddedFields, fields, table.StructName)

	// Combine original fields with embedded-generated fields
	allFields := append(fields, embeddedGeneratedFields...)

	// Use the regular generic generation logic with the combined fields
	return g.GenerateCreateTable(table, allFields, indexes, enums)
}

// GenerateAlterStatements generates ALTER statements for unknown dialects using AST
func (g *Generator) GenerateAlterStatements(oldFields, newFields []types.SchemaField) string {
	// For now, return a simple comment indicating this is not yet implemented with AST
	// This would need to be implemented with proper ALTER TABLE AST nodes
	return "-- ALTER statements not yet implemented with AST for generic dialect\n"
}
