package stubs

//migrator:schema:table name="categories" platform.mysql.engine="InnoDB" platform.mysql.comment="Product categories" platform.mariadb.engine="InnoDB" platform.mariadb.comment="Product categories"
type Category struct {
	//migrator:schema:field name="id" type="SERIAL" primary="true" platform.mysql.type="INT AUTO_INCREMENT" platform.mariadb.type="INT AUTO_INCREMENT"
	ID int64

	//migrator:schema:field name="name" type="VARCHAR(100)" not_null="true" unique="true"
	Name string

	//migrator:schema:field name="slug" type="VARCHAR(100)" not_null="true" unique="true"
	Slug string

	//migrator:schema:field name="description" type="TEXT" not_null="false"
	Description string

	//migrator:schema:field name="parent_id" type="INT" not_null="false" foreign="categories(id)" foreign_key_name="fk_category_parent"
	ParentID *int64

	//migrator:schema:field name="display_order" type="INT" not_null="true" default_expr="0"
	DisplayOrder int

	//migrator:schema:field name="visibility" type="enum_category_visibility" not_null="true" default="visible"
	Visibility string

	//migrator:schema:index name="idx_categories_parent" fields="parent_id"
	_ int
}

// The actual enum values that correspond to enum_category_visibility
//migrator:schema:field name="visibility" type="ENUM" enum="visible,hidden,featured"
