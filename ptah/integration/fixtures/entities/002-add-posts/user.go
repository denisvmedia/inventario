package entities

import "time"

//migrator:schema:table name="users"
type User struct {
	//migrator:schema:field name="id" type="SERIAL" primary="true"
	ID int64

	//migrator:schema:field name="email" type="VARCHAR(255)" not_null="true" unique="true"
	Email string

	//migrator:schema:field name="name" type="VARCHAR(255)" not_null="true"
	Name string

	//migrator:schema:field name="age" type="INTEGER"
	Age int

	//migrator:schema:field name="bio" type="TEXT"
	Bio string

	//migrator:schema:field name="active" type="BOOLEAN" not_null="true" default="true"
	Active bool

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	UpdatedAt time.Time
}

//migrator:schema:index table="users" name="idx_users_email" columns="email"
//migrator:schema:index table="users" name="idx_users_active" columns="active"
