package entities

import "time"

//migrator:schema:table name="products"
type Product struct {
	//migrator:schema:field name="id" type="SERIAL" primary="true"
	ID int64

	//migrator:schema:field name="name" type="VARCHAR(255)" not_null="true"
	Name string

	//migrator:schema:field name="description" type="TEXT"
	Description string

	//migrator:schema:field name="price" type="DECIMAL(10,2)" not_null="true"
	Price float64

	//migrator:schema:field name="active" type="BOOLEAN" not_null="true" default="true"
	Active bool

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	UpdatedAt time.Time
}
