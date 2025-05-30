package ast

// NewPrimaryKeyConstraint creates a table-level primary key constraint.
//
// This function creates a primary key constraint that spans one or more columns.
// For single-column primary keys, you can also use the SetPrimary() method on
// the column itself.
//
// Example:
//	// Single column primary key
//	pk := NewPrimaryKeyConstraint("id")
//	// Composite primary key
//	pk := NewPrimaryKeyConstraint("user_id", "role_id")
func NewPrimaryKeyConstraint(columns ...string) *ConstraintNode {
	return &ConstraintNode{
		Type:    PrimaryKeyConstraint,
		Columns: columns,
	}
}

// NewUniqueConstraint creates a table-level unique constraint with a name.
//
// This function creates a named unique constraint that can span multiple columns.
// The constraint name is useful for referencing the constraint later (e.g., for
// dropping it in migrations).
//
// Example:
//	// Single column unique constraint
//	unique := NewUniqueConstraint("uk_users_email", "email")
//	// Multi-column unique constraint
//	unique := NewUniqueConstraint("uk_users_name_company", "name", "company_id")
func NewUniqueConstraint(name string, columns ...string) *ConstraintNode {
	return &ConstraintNode{
		Type:    UniqueConstraint,
		Name:    name,
		Columns: columns,
	}
}

// NewForeignKeyConstraint creates a table-level foreign key constraint.
//
// This function creates a named foreign key constraint that references another
// table. The constraint can span multiple columns if both the source and target
// have composite keys.
//
// Example:
//	ref := &ForeignKeyRef{
//		Table:    "users",
//		Column:   "id",
//		OnDelete: "CASCADE",
//		Name:     "fk_orders_user",
//	}
//	fk := NewForeignKeyConstraint("fk_orders_user", []string{"user_id"}, ref)
func NewForeignKeyConstraint(name string, columns []string, ref *ForeignKeyRef) *ConstraintNode {
	return &ConstraintNode{
		Type:      ForeignKeyConstraint,
		Name:      name,
		Columns:   columns,
		Reference: ref,
	}
}
