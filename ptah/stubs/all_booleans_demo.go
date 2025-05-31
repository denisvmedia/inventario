package stubs

// Demonstration of all boolean attributes working with simplified syntax

//migrator:schema:table name="boolean_demo"
type BooleanDemo struct {
	// Primary key with simplified syntax
	//migrator:schema:field name="id" type="SERIAL" primary not_null
	ID int `db:"id"`

	// Unique field with index
	//migrator:schema:field name="email" type="VARCHAR(255)" unique not_null index
	Email string `db:"email"`

	// Auto increment field (alternative syntax)
	//migrator:schema:field name="sequence_id" type="INTEGER" auto_increment unique
	SequenceID int `db:"sequence_id"`

	// Nullable field (explicitly nullable)
	//migrator:schema:field name="description" type="TEXT" nullable
	Description *string `db:"description"`

	// Boolean field with default
	//migrator:schema:field name="is_active" type="BOOLEAN" not_null default_expr="true"
	IsActive bool `db:"is_active"`

	// Boolean field following naming pattern (automatically detected as boolean)
	//migrator:schema:field name="has_permission" type="BOOLEAN" not_null default_expr="false"
	HasPermission bool `db:"has_permission"`

	// Field with all boolean attributes combined
	//migrator:schema:field name="special_code" type="VARCHAR(50)" unique not_null index
	SpecialCode string `db:"special_code"`

	// Mixed syntax (old and new combined)
	//migrator:schema:field name="status" type="VARCHAR(20)" not_null="true" unique index default="pending"
	Status string `db:"status"`
}
