package stubs

//migrator:schema:table name="auto_increment_test"
type AutoIncrementTest struct {
	// Test auto_increment with INTEGER type (should become SERIAL in PostgreSQL)
	//migrator:schema:field name="id" type="INTEGER" primary auto_increment
	ID int `db:"id"`

	// Test auto_increment with BIGINT type (should become BIGSERIAL in PostgreSQL)
	//migrator:schema:field name="big_id" type="BIGINT" auto_increment unique
	BigID int64 `db:"big_id"`

	// Test auto_increment with SMALLINT type (should become SMALLSERIAL in PostgreSQL)
	//migrator:schema:field name="small_id" type="SMALLINT" auto_increment
	SmallID int16 `db:"small_id"`

	// Regular field without auto_increment
	//migrator:schema:field name="name" type="VARCHAR(255)" not_null
	Name string `db:"name"`

	// Test auto_increment with default INT type
	//migrator:schema:field name="sequence_num" type="INT" auto_increment
	SequenceNum int `db:"sequence_num"`
}
