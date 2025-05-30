package entities

import "time"

//migrator:schema:table name="categories"
type Category struct {
	//migrator:schema:field name="id" type="SERIAL" primary="true"
	ID int64

	//migrator:schema:field name="name" type="VARCHAR(255)" not_null="true" unique="true"
	Name string

	//migrator:schema:field name="description" type="TEXT"
	Description string

	//migrator:schema:field name="parent_id" type="BIGINT"
	ParentID *int64

	//migrator:schema:field name="sort_order" type="INTEGER" not_null="true" default="0"
	SortOrder int

	//migrator:schema:field name="is_active" type="BOOLEAN" not_null="true" default="true"
	IsActive bool

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	UpdatedAt time.Time
}

//migrator:schema:index table="categories" name="idx_categories_name" columns="name"
//migrator:schema:index table="categories" name="idx_categories_parent_id" columns="parent_id"
//migrator:schema:index table="categories" name="idx_categories_sort_order" columns="sort_order"
//migrator:schema:index table="categories" name="idx_categories_active" columns="is_active"
//migrator:schema:foreign_key table="categories" name="fk_categories_parent_id" columns="parent_id" ref_table="categories" ref_columns="id" on_delete="SET NULL"
//migrator:schema:check_constraint table="categories" name="chk_categories_sort_order_non_negative" condition="sort_order >= 0"
