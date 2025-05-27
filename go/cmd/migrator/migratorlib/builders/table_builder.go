package builders

import (
	"strings"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/ast"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

// TableBuilder provides a fluent API for building CREATE TABLE statements
type TableBuilder struct {
	table *ast.CreateTableNode
}

// NewTable creates a new table builder
func NewTable(name string) *TableBuilder {
	return &TableBuilder{
		table: ast.NewCreateTable(name),
	}
}

// Comment sets the table comment
func (tb *TableBuilder) Comment(comment string) *TableBuilder {
	tb.table.Comment = comment
	return tb
}

// Engine sets the table engine (MySQL/MariaDB specific)
func (tb *TableBuilder) Engine(engine string) *TableBuilder {
	tb.table.SetOption("ENGINE", engine)
	return tb
}

// Option sets a custom table option
func (tb *TableBuilder) Option(key, value string) *TableBuilder {
	tb.table.SetOption(key, value)
	return tb
}

// Column adds a column using a fluent column builder
func (tb *TableBuilder) Column(name, dataType string) *ColumnBuilder {
	column := ast.NewColumn(name, dataType)
	tb.table.AddColumn(column)
	return &ColumnBuilder{
		column: column,
		table:  tb,
	}
}

// PrimaryKey adds a composite primary key constraint
func (tb *TableBuilder) PrimaryKey(columns ...string) *TableBuilder {
	constraint := ast.NewPrimaryKeyConstraint(columns...)
	tb.table.AddConstraint(constraint)
	return tb
}

// Unique adds a unique constraint
func (tb *TableBuilder) Unique(name string, columns ...string) *TableBuilder {
	constraint := ast.NewUniqueConstraint(name, columns...)
	tb.table.AddConstraint(constraint)
	return tb
}

// ForeignKey adds a foreign key constraint
func (tb *TableBuilder) ForeignKey(name string, columns []string, refTable, refColumn string) *ForeignKeyBuilder {
	ref := &ast.ForeignKeyRef{
		Table:  refTable,
		Column: refColumn,
		Name:   name,
	}
	constraint := ast.NewForeignKeyConstraint(name, columns, ref)
	tb.table.AddConstraint(constraint)

	return &ForeignKeyBuilder{
		ref:   ref,
		table: tb,
	}
}

// Build returns the completed CREATE TABLE AST node
func (tb *TableBuilder) Build() *ast.CreateTableNode {
	return tb.table
}

// ColumnBuilder provides a fluent API for building column definitions
type ColumnBuilder struct {
	column *ast.ColumnNode
	table  *TableBuilder
}

// Primary marks the column as primary key
func (cb *ColumnBuilder) Primary() *ColumnBuilder {
	cb.column.SetPrimary()
	return cb
}

// NotNull marks the column as NOT NULL
func (cb *ColumnBuilder) NotNull() *ColumnBuilder {
	cb.column.SetNotNull()
	return cb
}

// Nullable marks the column as nullable (default)
func (cb *ColumnBuilder) Nullable() *ColumnBuilder {
	cb.column.Nullable = true
	return cb
}

// Unique marks the column as UNIQUE
func (cb *ColumnBuilder) Unique() *ColumnBuilder {
	cb.column.SetUnique()
	return cb
}

// AutoIncrement marks the column as auto-incrementing
func (cb *ColumnBuilder) AutoIncrement() *ColumnBuilder {
	cb.column.SetAutoIncrement()
	return cb
}

// Default sets a literal default value
func (cb *ColumnBuilder) Default(value string) *ColumnBuilder {
	cb.column.SetDefault(value)
	return cb
}

// DefaultFunction sets a function as default value
func (cb *ColumnBuilder) DefaultFunction(fn string) *ColumnBuilder {
	cb.column.SetDefaultFunction(fn)
	return cb
}

// Check sets a check constraint
func (cb *ColumnBuilder) Check(expression string) *ColumnBuilder {
	cb.column.SetCheck(expression)
	return cb
}

// Comment sets a column comment
func (cb *ColumnBuilder) Comment(comment string) *ColumnBuilder {
	cb.column.SetComment(comment)
	return cb
}

// ForeignKey sets a foreign key reference
func (cb *ColumnBuilder) ForeignKey(table, column, name string) *ForeignKeyBuilder {
	cb.column.SetForeignKey(table, column, name)

	ref := cb.column.ForeignKey
	return &ForeignKeyBuilder{
		ref:   ref,
		table: cb.table,
	}
}

// End returns to the table builder
func (cb *ColumnBuilder) End() *TableBuilder {
	return cb.table
}

// ForeignKeyBuilder provides a fluent API for building foreign key constraints
type ForeignKeyBuilder struct {
	ref   *ast.ForeignKeyRef
	table *TableBuilder
}

// OnDelete sets the ON DELETE action
func (fkb *ForeignKeyBuilder) OnDelete(action string) *ForeignKeyBuilder {
	fkb.ref.OnDelete = action
	return fkb
}

// OnUpdate sets the ON UPDATE action
func (fkb *ForeignKeyBuilder) OnUpdate(action string) *ForeignKeyBuilder {
	fkb.ref.OnUpdate = action
	return fkb
}

// End returns to the table builder
func (fkb *ForeignKeyBuilder) End() *TableBuilder {
	return fkb.table
}

// IndexBuilder provides a fluent API for building CREATE INDEX statements
type IndexBuilder struct {
	index *ast.IndexNode
}

// NewIndex creates a new index builder
func NewIndex(name, table string, columns ...string) *IndexBuilder {
	return &IndexBuilder{
		index: ast.NewIndex(name, table, columns...),
	}
}

// Unique marks the index as unique
func (ib *IndexBuilder) Unique() *IndexBuilder {
	ib.index.SetUnique()
	return ib
}

// Type sets the index type (e.g., BTREE, HASH)
func (ib *IndexBuilder) Type(indexType string) *IndexBuilder {
	ib.index.Type = indexType
	return ib
}

// Comment sets the index comment
func (ib *IndexBuilder) Comment(comment string) *IndexBuilder {
	ib.index.Comment = comment
	return ib
}

// Build returns the completed CREATE INDEX AST node
func (ib *IndexBuilder) Build() *ast.IndexNode {
	return ib.index
}

// SchemaBuilder provides a fluent API for building complete database schemas
type SchemaBuilder struct {
	statements []ast.Node
}

// NewSchema creates a new schema builder
func NewSchema() *SchemaBuilder {
	return &SchemaBuilder{
		statements: make([]ast.Node, 0),
	}
}

// Comment adds a comment to the schema
func (sb *SchemaBuilder) Comment(text string) *SchemaBuilder {
	sb.statements = append(sb.statements, ast.NewComment(text))
	return sb
}

// Enum adds an enum definition (PostgreSQL)
func (sb *SchemaBuilder) Enum(name string, values ...string) *SchemaBuilder {
	sb.statements = append(sb.statements, ast.NewEnum(name, values...))
	return sb
}

// Table adds a table definition and returns a table builder that can return to schema
func (sb *SchemaBuilder) Table(name string) *SchemaTableBuilder {
	tb := NewTable(name)
	// We'll add the table to statements when End() is called
	return &SchemaTableBuilder{
		TableBuilder: tb,
		schema:       sb,
	}
}

// Index adds an index definition and returns an index builder that can return to schema
func (sb *SchemaBuilder) Index(name, table string, columns ...string) *SchemaIndexBuilder {
	ib := NewIndex(name, table, columns...)
	// We'll add the index to statements when End() is called
	return &SchemaIndexBuilder{
		IndexBuilder: ib,
		schema:       sb,
	}
}

// Build returns the completed schema as a statement list
func (sb *SchemaBuilder) Build() *ast.StatementList {
	return &ast.StatementList{
		Statements: sb.statements,
	}
}

// SchemaTableBuilder wraps TableBuilder and allows returning to schema
type SchemaTableBuilder struct {
	*TableBuilder
	schema *SchemaBuilder
}

// Column adds a column using a fluent column builder that can return to schema table
func (stb *SchemaTableBuilder) Column(name, dataType string) *SchemaColumnBuilder {
	column := ast.NewColumn(name, dataType)
	stb.TableBuilder.table.AddColumn(column)
	return &SchemaColumnBuilder{
		column:      column,
		schemaTable: stb,
	}
}

// End completes the table definition and returns to the schema builder
func (stb *SchemaTableBuilder) End() *SchemaBuilder {
	stb.schema.statements = append(stb.schema.statements, stb.TableBuilder.Build())
	return stb.schema
}

// SchemaColumnBuilder wraps column building and allows returning to schema table
type SchemaColumnBuilder struct {
	column      *ast.ColumnNode
	schemaTable *SchemaTableBuilder
}

// Primary marks the column as primary key
func (scb *SchemaColumnBuilder) Primary() *SchemaColumnBuilder {
	scb.column.SetPrimary()
	return scb
}

// NotNull marks the column as NOT NULL
func (scb *SchemaColumnBuilder) NotNull() *SchemaColumnBuilder {
	scb.column.SetNotNull()
	return scb
}

// Nullable marks the column as nullable (default)
func (scb *SchemaColumnBuilder) Nullable() *SchemaColumnBuilder {
	scb.column.Nullable = true
	return scb
}

// Unique marks the column as UNIQUE
func (scb *SchemaColumnBuilder) Unique() *SchemaColumnBuilder {
	scb.column.SetUnique()
	return scb
}

// AutoIncrement marks the column as auto-incrementing
func (scb *SchemaColumnBuilder) AutoIncrement() *SchemaColumnBuilder {
	scb.column.SetAutoIncrement()
	return scb
}

// Default sets a literal default value
func (scb *SchemaColumnBuilder) Default(value string) *SchemaColumnBuilder {
	scb.column.SetDefault(value)
	return scb
}

// DefaultFunction sets a function as default value
func (scb *SchemaColumnBuilder) DefaultFunction(fn string) *SchemaColumnBuilder {
	scb.column.SetDefaultFunction(fn)
	return scb
}

// Check sets a check constraint
func (scb *SchemaColumnBuilder) Check(expression string) *SchemaColumnBuilder {
	scb.column.SetCheck(expression)
	return scb
}

// Comment sets a column comment
func (scb *SchemaColumnBuilder) Comment(comment string) *SchemaColumnBuilder {
	scb.column.SetComment(comment)
	return scb
}

// ForeignKey sets a foreign key reference
func (scb *SchemaColumnBuilder) ForeignKey(table, column, name string) *SchemaForeignKeyBuilder {
	scb.column.SetForeignKey(table, column, name)

	ref := scb.column.ForeignKey
	return &SchemaForeignKeyBuilder{
		ref:         ref,
		schemaTable: scb.schemaTable,
	}
}

// End returns to the schema table builder
func (scb *SchemaColumnBuilder) End() *SchemaTableBuilder {
	return scb.schemaTable
}

// SchemaForeignKeyBuilder provides a fluent API for building foreign key constraints in schema context
type SchemaForeignKeyBuilder struct {
	ref         *ast.ForeignKeyRef
	schemaTable *SchemaTableBuilder
}

// OnDelete sets the ON DELETE action
func (sfkb *SchemaForeignKeyBuilder) OnDelete(action string) *SchemaForeignKeyBuilder {
	sfkb.ref.OnDelete = action
	return sfkb
}

// OnUpdate sets the ON UPDATE action
func (sfkb *SchemaForeignKeyBuilder) OnUpdate(action string) *SchemaForeignKeyBuilder {
	sfkb.ref.OnUpdate = action
	return sfkb
}

// End returns to the schema table builder
func (sfkb *SchemaForeignKeyBuilder) End() *SchemaTableBuilder {
	return sfkb.schemaTable
}

// SchemaIndexBuilder wraps IndexBuilder and allows returning to schema
type SchemaIndexBuilder struct {
	*IndexBuilder
	schema *SchemaBuilder
}

// End completes the index definition and returns to the schema builder
func (sib *SchemaIndexBuilder) End() *SchemaBuilder {
	sib.schema.statements = append(sib.schema.statements, sib.IndexBuilder.Build())
	return sib.schema
}

// Converter functions to build AST from existing types

// FromSchemaField converts a SchemaField to a ColumnNode
func FromSchemaField(field types.SchemaField, enums []types.GlobalEnum) *ast.ColumnNode {
	column := ast.NewColumn(field.Name, field.Type)

	if !field.Nullable {
		column.SetNotNull()
	}

	if field.Primary {
		column.SetPrimary()
	}

	if field.Unique {
		column.SetUnique()
	}

	if field.AutoInc {
		column.SetAutoIncrement()
	}

	if field.Default != "" {
		column.SetDefault(field.Default)
	}

	if field.DefaultFn != "" {
		column.SetDefaultFunction(field.DefaultFn)
	}

	if field.Check != "" {
		column.SetCheck(field.Check)
	}

	if field.Comment != "" {
		column.SetComment(field.Comment)
	}

	if field.Foreign != "" {
		column.SetForeignKey(field.Foreign, "", field.ForeignKeyName)
	}

	return column
}

// FromTableDirective converts a TableDirective to a CreateTableNode
func FromTableDirective(table types.TableDirective, fields []types.SchemaField, enums []types.GlobalEnum) *ast.CreateTableNode {
	createTable := ast.NewCreateTable(table.Name)

	if table.Comment != "" {
		createTable.Comment = table.Comment
	}

	if table.Engine != "" {
		createTable.SetOption("ENGINE", table.Engine)
	}

	// Add columns
	for _, field := range fields {
		if field.StructName == table.StructName {
			column := FromSchemaField(field, enums)
			createTable.AddColumn(column)
		}
	}

	// Add composite primary key if specified
	if len(table.PrimaryKey) > 1 {
		constraint := ast.NewPrimaryKeyConstraint(table.PrimaryKey...)
		createTable.AddConstraint(constraint)
	}

	return createTable
}

// ProcessEmbeddedFields processes embedded fields and generates corresponding schema fields
func ProcessEmbeddedFields(embeddedFields []types.EmbeddedField, allFields []types.SchemaField, structName string) []types.SchemaField {
	var generatedFields []types.SchemaField

	for _, embedded := range embeddedFields {
		if embedded.StructName != structName {
			continue
		}

		switch embedded.Mode {
		case "inline":
			// Find fields from the embedded type and add them with optional prefix
			for _, field := range allFields {
				if field.StructName == embedded.EmbeddedTypeName {
					newField := field
					newField.StructName = structName

					// Apply prefix if specified
					if embedded.Prefix != "" {
						newField.Name = embedded.Prefix + field.Name
					}

					generatedFields = append(generatedFields, newField)
				}
			}

		case "json":
			// Create a single JSON/JSONB column
			columnName := embedded.Name
			if columnName == "" {
				columnName = strings.ToLower(embedded.EmbeddedTypeName) + "_data"
			}

			columnType := embedded.Type
			if columnType == "" {
				columnType = "JSONB" // Default to JSONB
			}

			generatedFields = append(generatedFields, types.SchemaField{
				StructName: structName,
				FieldName:  embedded.EmbeddedTypeName,
				Name:       columnName,
				Type:       columnType,
				Nullable:   embedded.Nullable,
				Comment:    embedded.Comment,
				Overrides:  embedded.Overrides, // Pass through platform-specific overrides
			})

		case "relation":
			// Create a foreign key field
			if embedded.Field == "" || embedded.Ref == "" {
				continue // Skip if required fields are missing
			}

			// Parse the reference to get the type
			refType := "INTEGER" // Default type
			if strings.Contains(embedded.Ref, "VARCHAR") || strings.Contains(embedded.Ref, "TEXT") {
				refType = "VARCHAR(36)" // Assume UUID if not integer
			}

			generatedFields = append(generatedFields, types.SchemaField{
				StructName:     structName,
				FieldName:      embedded.EmbeddedTypeName,
				Name:           embedded.Field,
				Type:           refType,
				Nullable:       embedded.Nullable,
				Foreign:        embedded.Ref,
				ForeignKeyName: "fk_" + strings.ToLower(structName) + "_" + strings.ToLower(embedded.Field),
				Comment:        embedded.Comment,
			})

		case "skip":
			// Do nothing - skip this embedded field
			continue

		default:
			// Default to inline mode if no mode specified
			for _, field := range allFields {
				if field.StructName == embedded.EmbeddedTypeName {
					newField := field
					newField.StructName = structName
					generatedFields = append(generatedFields, newField)
				}
			}
		}
	}

	return generatedFields
}

// FromSchemaIndex converts a SchemaIndex to an IndexNode
func FromSchemaIndex(index types.SchemaIndex) *ast.IndexNode {
	indexNode := ast.NewIndex(index.Name, index.StructName, index.Fields...)

	if index.Unique {
		indexNode.SetUnique()
	}

	if index.Comment != "" {
		indexNode.Comment = index.Comment
	}

	return indexNode
}
