package ast

// NewPrimaryKeyConstraint creates a primary key constraint
func NewPrimaryKeyConstraint(columns ...string) *ConstraintNode {
	return &ConstraintNode{
		Type:    PrimaryKeyConstraint,
		Columns: columns,
	}
}

// NewUniqueConstraint creates a unique constraint
func NewUniqueConstraint(name string, columns ...string) *ConstraintNode {
	return &ConstraintNode{
		Type:    UniqueConstraint,
		Name:    name,
		Columns: columns,
	}
}

// NewForeignKeyConstraint creates a foreign key constraint
func NewForeignKeyConstraint(name string, columns []string, ref *ForeignKeyRef) *ConstraintNode {
	return &ConstraintNode{
		Type:      ForeignKeyConstraint,
		Name:      name,
		Columns:   columns,
		Reference: ref,
	}
}
