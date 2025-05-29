package stubs

//migrator:schema:table name="products" platform.mysql.engine="InnoDB" platform.mysql.comment="Product catalog" platform.mariadb.engine="InnoDB" platform.mariadb.comment="Product catalog"
type Product struct {
	//migrator:schema:field name="id" type="SERIAL" primary="true" platform.mysql.type="INT AUTO_INCREMENT" platform.mariadb.type="INT AUTO_INCREMENT"
	ID int64

	//migrator:schema:field name="sku" type="VARCHAR(50)" not_null="true" unique="true"
	SKU string

	//migrator:schema:field name="name" type="VARCHAR(255)" not_null="true"
	Name string

	//migrator:schema:field name="description" type="TEXT" not_null="false"
	Description string

	//migrator:schema:field name="price" type="DECIMAL(10,2)" not_null="true" check="price > 0"
	Price float64

	//migrator:schema:field name="status" type="ENUM" enum="active,inactive,discontinued,out_of_stock" not_null="true" default="active"
	Status string

	//migrator:schema:field name="category_id" type="INT" not_null="true" foreign="categories(id)" foreign_key_name="fk_product_category"
	CategoryID int64

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="NOW()"
	CreatedAt string

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="false"
	UpdatedAt string

	//migrator:schema:field name="in_stock" type="BOOLEAN" not_null="true" default="true"
	InStock bool

	//migrator:schema:index name="idx_products_category" fields="category_id"
	_ int
}
