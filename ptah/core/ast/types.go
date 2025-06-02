package ast

// TypeDefinition represents different types of type definitions that can be used in CREATE TYPE statements.
//
// This interface allows for various type definitions including enums, composite types,
// domains, and ranges. Each implementation provides the specific structure for its type.
type TypeDefinition interface {
	Node
	typeDefinition() // marker method to ensure type safety
}

// TypeOperation represents different types of operations that can be performed in ALTER TYPE statements.
//
// This interface extends the Node interface and includes a marker method to ensure
// type safety. All ALTER TYPE operations must implement both the visitor pattern
// and the marker method.
type TypeOperation interface {
	Node
	typeOperation() // marker method to ensure type safety
}

// EnumTypeDef represents an enum type definition for CREATE TYPE ... AS ENUM statements.
//
// This is used within CreateTypeNode to define enum types with a list of values.
type EnumTypeDef struct {
	// Values contains the list of allowed enum values
	Values []string
}

// NewEnumTypeDef creates a new enum type definition with the specified values.
//
// Example:
//
//	enumDef := NewEnumTypeDef("active", "inactive", "pending")
func NewEnumTypeDef(values ...string) *EnumTypeDef {
	return &EnumTypeDef{
		Values: values,
	}
}

// Accept implements the Node interface for EnumTypeDef.
func (td *EnumTypeDef) Accept(visitor Visitor) error {
	// This is typically handled by the parent CreateTypeNode's visitor
	return nil
}

// typeDefinition implements the marker method for type safety.
func (td *EnumTypeDef) typeDefinition() {}

// CompositeTypeDef represents a composite type definition for CREATE TYPE statements.
//
// Composite types define a structure with multiple fields, similar to a table
// but used as a data type. This is primarily supported by PostgreSQL.
type CompositeTypeDef struct {
	// Fields contains the list of fields in the composite type
	Fields []*CompositeField
}

// CompositeField represents a field in a composite type definition.
type CompositeField struct {
	// Name is the field name
	Name string
	// Type is the field data type
	Type string
	// Comment is an optional field comment
	Comment string
}

// NewCompositeTypeDef creates a new composite type definition with the specified fields.
//
// Example:
//
//	fields := []*CompositeField{
//		{Name: "street", Type: "TEXT"},
//		{Name: "city", Type: "TEXT"},
//		{Name: "zipcode", Type: "VARCHAR(10)"},
//	}
//	compositeDef := NewCompositeTypeDef(fields...)
func NewCompositeTypeDef(fields ...*CompositeField) *CompositeTypeDef {
	return &CompositeTypeDef{
		Fields: fields,
	}
}

// Accept implements the Node interface for CompositeTypeDef.
func (td *CompositeTypeDef) Accept(visitor Visitor) error {
	// This is typically handled by the parent CreateTypeNode's visitor
	return nil
}

// typeDefinition implements the marker method for type safety.
func (td *CompositeTypeDef) typeDefinition() {}

// DomainTypeDef represents a domain type definition for CREATE DOMAIN statements.
//
// Domains are essentially aliases for existing data types with optional constraints.
// This is primarily supported by PostgreSQL.
type DomainTypeDef struct {
	// BaseType is the underlying data type
	BaseType string
	// Nullable indicates whether the domain allows NULL values
	Nullable bool
	// Default contains the default value for the domain
	Default *DefaultValue
	// Check contains a check constraint expression
	Check string
}

// NewDomainTypeDef creates a new domain type definition with the specified base type.
//
// Example:
//
//	domainDef := NewDomainTypeDef("VARCHAR(255)").
//		SetNotNull().
//		SetCheck("VALUE ~ '^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'")
func NewDomainTypeDef(baseType string) *DomainTypeDef {
	return &DomainTypeDef{
		BaseType: baseType,
		Nullable: true, // Default to nullable
	}
}

// SetNotNull marks the domain as NOT NULL and returns the domain for chaining.
func (td *DomainTypeDef) SetNotNull() *DomainTypeDef {
	td.Nullable = false
	return td
}

// SetDefault sets a literal default value and returns the domain for chaining.
func (td *DomainTypeDef) SetDefault(value string) *DomainTypeDef {
	td.Default = &DefaultValue{Value: value}
	return td
}

// SetDefaultExpression sets a function as the default value and returns the domain for chaining.
func (td *DomainTypeDef) SetDefaultExpression(fn string) *DomainTypeDef {
	td.Default = &DefaultValue{Expression: fn}
	return td
}

// SetCheck sets a check constraint expression and returns the domain for chaining.
func (td *DomainTypeDef) SetCheck(expression string) *DomainTypeDef {
	td.Check = expression
	return td
}

// Accept implements the Node interface for DomainTypeDef.
func (td *DomainTypeDef) Accept(visitor Visitor) error {
	// This is typically handled by the parent CreateTypeNode's visitor
	return nil
}

// typeDefinition implements the marker method for type safety.
func (td *DomainTypeDef) typeDefinition() {}

// AddEnumValueOperation represents an ADD VALUE operation for ALTER TYPE statements.
//
// This operation adds a new value to an existing enum type. The position
// can be specified using BEFORE or AFTER clauses.
type AddEnumValueOperation struct {
	// Value is the new enum value to add
	Value string
	// Before specifies the existing value before which to insert (optional)
	Before string
	// After specifies the existing value after which to insert (optional)
	After string
}

// NewAddEnumValueOperation creates a new add enum value operation.
//
// Example:
//
//	op := NewAddEnumValueOperation("pending")
//	op := NewAddEnumValueOperation("archived").SetAfter("inactive")
func NewAddEnumValueOperation(value string) *AddEnumValueOperation {
	return &AddEnumValueOperation{
		Value: value,
	}
}

// SetBefore sets the BEFORE clause and returns the operation for chaining.
func (op *AddEnumValueOperation) SetBefore(value string) *AddEnumValueOperation {
	op.Before = value
	op.After = "" // Clear After if Before is set
	return op
}

// SetAfter sets the AFTER clause and returns the operation for chaining.
func (op *AddEnumValueOperation) SetAfter(value string) *AddEnumValueOperation {
	op.After = value
	op.Before = "" // Clear Before if After is set
	return op
}

// Accept implements the Node interface for AddEnumValueOperation.
func (op *AddEnumValueOperation) Accept(visitor Visitor) error {
	// This is typically handled by the parent AlterTypeNode's visitor
	return nil
}

// typeOperation implements the marker method for type safety.
func (op *AddEnumValueOperation) typeOperation() {}

// RenameEnumValueOperation represents a RENAME VALUE operation for ALTER TYPE statements.
//
// This operation renames an existing enum value to a new name.
type RenameEnumValueOperation struct {
	// OldValue is the current enum value name
	OldValue string
	// NewValue is the new enum value name
	NewValue string
}

// NewRenameEnumValueOperation creates a new rename enum value operation.
//
// Example:
//
//	op := NewRenameEnumValueOperation("old_status", "new_status")
func NewRenameEnumValueOperation(oldValue, newValue string) *RenameEnumValueOperation {
	return &RenameEnumValueOperation{
		OldValue: oldValue,
		NewValue: newValue,
	}
}

// Accept implements the Node interface for RenameEnumValueOperation.
func (op *RenameEnumValueOperation) Accept(visitor Visitor) error {
	// This is typically handled by the parent AlterTypeNode's visitor
	return nil
}

// typeOperation implements the marker method for type safety.
func (op *RenameEnumValueOperation) typeOperation() {}

// RenameTypeOperation represents a RENAME TO operation for ALTER TYPE statements.
//
// This operation renames the type itself to a new name.
type RenameTypeOperation struct {
	// NewName is the new type name
	NewName string
}

// NewRenameTypeOperation creates a new rename type operation.
//
// Example:
//
//	op := NewRenameTypeOperation("new_type_name")
func NewRenameTypeOperation(newName string) *RenameTypeOperation {
	return &RenameTypeOperation{
		NewName: newName,
	}
}

// Accept implements the Node interface for RenameTypeOperation.
func (op *RenameTypeOperation) Accept(visitor Visitor) error {
	// This is typically handled by the parent AlterTypeNode's visitor
	return nil
}

// typeOperation implements the marker method for type safety.
func (op *RenameTypeOperation) typeOperation() {}
