package stubs

//migrator:schema:table name="test_primary"
type TestPrimary struct {
	// Test simplified primary syntax
	//migrator:schema:field name="id" type="SERIAL" primary not_null
	ID int `db:"id"`

	// Test simplified unique syntax
	//migrator:schema:field name="email" type="VARCHAR(255)" unique not_null
	Email string `db:"email"`

	// Test simplified nullable syntax
	//migrator:schema:field name="description" type="TEXT" nullable
	Description *string `db:"description"`

	// Test simplified index syntax
	//migrator:schema:field name="username" type="VARCHAR(100)" unique not_null index
	Username string `db:"username"`
}
