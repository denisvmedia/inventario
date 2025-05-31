package stubs

//migrator:schema:table name="primary_not_null_test"
type PrimaryNotNullTest struct {
	// Primary key with explicit not_null (should only show PRIMARY KEY)
	//migrator:schema:field name="id" type="SERIAL" primary not_null
	ID int `db:"id"`

	// Primary key with explicit not_null="true" (should only show PRIMARY KEY)
	//migrator:schema:field name="alt_id" type="INTEGER" primary="true" not_null="true"
	AltID int `db:"alt_id"`

	// Non-primary field with not_null (should show NOT NULL)
	//migrator:schema:field name="name" type="VARCHAR(255)" not_null
	Name string `db:"name"`

	// Non-primary field with unique and not_null (should show both)
	//migrator:schema:field name="email" type="VARCHAR(255)" unique not_null
	Email string `db:"email"`

	// Nullable field (should not show NOT NULL)
	//migrator:schema:field name="description" type="TEXT" nullable
	Description *string `db:"description"`
}
