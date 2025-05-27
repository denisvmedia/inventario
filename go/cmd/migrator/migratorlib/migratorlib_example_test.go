package migratorlib_test

import (
	"fmt"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

// ExampleGenerateCreateTable demonstrates how to generate SQL for different databases
func ExampleGenerateCreateTable() {
	// Sample data
	productTable := types.TableDirective{
		StructName: "Product",
		Name:       "products",
	}

	fields := []types.SchemaField{
		{
			StructName: "Product",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
		},
		{
			StructName: "Product",
			Name:       "name",
			Type:       "VARCHAR(255)",
			Nullable:   false,
		},
		{
			StructName: "Product",
			Name:       "status",
			Type:       "enum_product_status",
			Nullable:   false,
		},
	}

	indexes := []types.SchemaIndex{
		{
			StructName: "Product",
			Name:       "idx_products_name",
			Fields:     []string{"name"},
		},
	}

	enums := []types.GlobalEnum{
		{
			Name:   "enum_product_status",
			Values: []string{"active", "inactive", "discontinued"},
		},
	}

	// Generate for PostgreSQL
	pgSQL := migratorlib.GenerateCreateTable(productTable, fields, indexes, enums, "postgres")
	fmt.Println("PostgreSQL SQL:")
	fmt.Print(pgSQL)

	// Generate for MySQL
	mySQL := migratorlib.GenerateCreateTable(productTable, fields, indexes, enums, "mysql")
	fmt.Println("MySQL SQL:")
	fmt.Print(mySQL)

	// Output:
	// PostgreSQL SQL:
	// -- POSTGRES TABLE: products --
	// CREATE TYPE enum_product_status AS ENUM ('active', 'inactive', 'discontinued');
	// CREATE TABLE products (
	//   id SERIAL PRIMARY KEY,
	//   name VARCHAR(255) NOT NULL,
	//   status enum_product_status NOT NULL
	// );
	// CREATE INDEX idx_products_name ON products (name);
	//
	// MySQL SQL:
	// -- MYSQL TABLE: products --
	// CREATE TABLE products (
	//   id SERIAL PRIMARY KEY,
	//   name VARCHAR(255) NOT NULL,
	//   status ENUM('active', 'inactive', 'discontinued') NOT NULL
	// );
	// CREATE INDEX idx_products_name ON products (name);
	//
}

// Example_platformSpecificAttributes demonstrates how to use platform-specific attributes
func Example_platformSpecificAttributes() {
	// Sample data for a table with platform-specific attributes
	productTable := types.TableDirective{
		StructName: "Product",
		Name:       "products",
		Overrides: map[string]map[string]string{
			"mysql": {
				"engine":  "InnoDB",
				"comment": "Product catalog",
				"charset": "utf8mb4",
			},
			"mariadb": {
				"engine":  "InnoDB",
				"comment": "Product catalog",
				"charset": "utf8mb4",
			},
		},
	}

	fields := []types.SchemaField{
		{
			StructName: "Product",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
			Overrides: map[string]map[string]string{
				"mysql": {
					"type": "INT AUTO_INCREMENT",
				},
				"mariadb": {
					"type": "INT AUTO_INCREMENT",
				},
			},
		},
		{
			StructName: "Product",
			Name:       "name",
			Type:       "VARCHAR(255)",
			Nullable:   false,
		},
	}

	// Generate for MySQL with platform-specific attributes
	mySQL := migratorlib.GenerateCreateTable(productTable, fields, nil, nil, "mysql")
	fmt.Println("MySQL SQL with platform-specific attributes:")
	fmt.Print(mySQL)

	// Generate for PostgreSQL (platform-specific attributes are ignored)
	pgSQL := migratorlib.GenerateCreateTable(productTable, fields, nil, nil, "postgres")
	fmt.Println("PostgreSQL SQL (platform-specific attributes ignored):")
	fmt.Print(pgSQL)

	// Output:
	// MySQL SQL with platform-specific attributes:
	// -- MYSQL TABLE: products (Product catalog) --
	// CREATE TABLE products (
	//   id INT AUTO_INCREMENT PRIMARY KEY,
	//   name VARCHAR(255) NOT NULL
	// ); ENGINE=InnoDB charset=utf8mb4
	// PostgreSQL SQL (platform-specific attributes ignored):
	// -- POSTGRES TABLE: products --
	// CREATE TABLE products (
	//   id SERIAL PRIMARY KEY,
	//   name VARCHAR(255) NOT NULL
	// );
}
