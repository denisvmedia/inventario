package stubs

import "time"

// This file demonstrates that embedded types can be defined separately
// from the tables that use them, but they need to be in the same file
// for the single-file parser to work correctly.

//migrator:schema:table name="embedded_example"
type EmbeddedExample struct {
	//migrator:schema:field name="id" type="INTEGER" primary="true" not_null="true"
	ID int `db:"id"`

	//migrator:schema:field name="name" type="VARCHAR(255)" not_null="true"
	Name string `db:"name"`
}

// Embedded type definitions
//
//migrator:schema:embed
type Timestamps struct {
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default="CURRENT_TIMESTAMP"
	CreatedAt time.Time `db:"created_at"`

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `db:"updated_at"`
}

//migrator:schema:embed
type AuditInfo struct {
	//migrator:schema:field name="by" type="TEXT"
	By string `db:"by"`

	//migrator:schema:field name="reason" type="TEXT"
	Reason string `db:"reason"`
}

//migrator:schema:embed
type Meta struct {
	Author string
	Source string
}

//migrator:schema:table name="users"
type User struct {
	//migrator:schema:field name="id" type="INTEGER" auto_increment="true" primary="true" not_null="true"
	ID int `db:"id"`

	//migrator:schema:field name="email" type="VARCHAR(255)" unique="true" not_null="true"
	Email string `db:"email"`
}
