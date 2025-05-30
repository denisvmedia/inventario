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

	//migrator:schema:field name="category" type="VARCHAR(100)"
	Category string

	//migrator:schema:field name="price" type="DECIMAL(10,2)" not_null="true"
	Price float64

	//migrator:schema:field name="status" type="ENUM" enum="draft,active,discontinued" not_null="true" default="draft"
	Status string

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	UpdatedAt time.Time
}

//migrator:schema:index table="products" name="idx_products_name" columns="name"
//migrator:schema:index table="products" name="idx_products_category" columns="category"
//migrator:schema:index table="products" name="idx_products_status" columns="status"
