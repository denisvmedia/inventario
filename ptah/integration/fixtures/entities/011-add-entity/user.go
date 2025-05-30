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

	//migrator:schema:field name="user_age" type="SMALLINT"
	UserAge int16

	//migrator:schema:field name="description" type="VARCHAR(500)"
	Description string

	//migrator:schema:field name="status" type="ENUM" enum="active,inactive,suspended" not_null="true" default="active"
	Status string

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	UpdatedAt time.Time
}

//migrator:schema:index table="users" name="idx_users_status" columns="status"
//migrator:schema:index table="users" name="idx_users_name_email" columns="name,email"
//migrator:schema:index table="users" name="idx_users_description" columns="description"
