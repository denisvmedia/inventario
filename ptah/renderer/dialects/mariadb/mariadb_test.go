package mariadb_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/renderer/dialects/mariadb"
	"github.com/denisvmedia/inventario/ptah/schema/meta"
)

func TestGenerator_New(t *testing.T) {
	c := qt.New(t)

	generator := mariadb.New()
	c.Assert(generator, qt.IsNotNil)
	c.Assert(generator.GetDialectName(), qt.Equals, "mariadb")
}

func TestGenerator_GenerateCreateTable_BasicTable(t *testing.T) {
	c := qt.New(t)

	generator := mariadb.New()

	table := meta.TableDirective{
		StructName: "User",
		Name:       "users",
		Comment:    "User accounts table",
		Engine:     "InnoDB",
	}

	fields := []meta.SchemaField{
		{
			StructName: "User",
			Name:       "id",
			Type:       "INT",
			Primary:    true,
			AutoInc:    true,
		},
		{
			StructName: "User",
			Name:       "email",
			Type:       "VARCHAR(320)",
			Nullable:   false,
			Unique:     true,
		},
		{
			StructName: "User",
			Name:       "name",
			Type:       "VARCHAR(255)",
			Nullable:   false,
		},
		{
			StructName: "User",
			Name:       "created_at",
			Type:       "TIMESTAMP",
			Nullable:   false,
			Default:    "CURRENT_TIMESTAMP",
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "-- MARIADB TABLE: users (User accounts table) --")
	c.Assert(result, qt.Contains, "CREATE TABLE users")
	c.Assert(result, qt.Contains, "id INT PRIMARY KEY AUTO_INCREMENT")
	c.Assert(result, qt.Contains, "email VARCHAR(320) NOT NULL UNIQUE")
	c.Assert(result, qt.Contains, "name VARCHAR(255) NOT NULL")
	c.Assert(result, qt.Contains, "created_at TIMESTAMP NOT NULL DEFAULT 'CURRENT_TIMESTAMP'")
}

func TestGenerator_GenerateCreateTable_WithInlineEnums(t *testing.T) {
	c := qt.New(t)

	generator := mariadb.New()

	table := meta.TableDirective{
		StructName: "User",
		Name:       "users",
		Engine:     "InnoDB",
	}

	fields := []meta.SchemaField{
		{
			StructName: "User",
			Name:       "id",
			Type:       "INT",
			Primary:    true,
			AutoInc:    true,
		},
		{
			StructName: "User",
			Name:       "status",
			Type:       "user_status_enum",
			Nullable:   false,
		},
		{
			StructName: "User",
			Name:       "role",
			Type:       "user_role_enum",
			Nullable:   false,
			Default:    "user",
		},
	}

	enums := []meta.GlobalEnum{
		{
			Name:   "user_status_enum",
			Values: []string{"active", "inactive", "pending"},
		},
		{
			Name:   "user_role_enum",
			Values: []string{"admin", "user", "guest"},
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, enums)

	// MariaDB should create inline ENUM types (similar to MySQL)
	c.Assert(result, qt.Contains, "status ENUM('active', 'inactive', 'pending') NOT NULL")
	c.Assert(result, qt.Contains, "role ENUM('admin', 'user', 'guest') NOT NULL DEFAULT 'user'")
	// Should NOT contain PostgreSQL-style CREATE TYPE statements
	c.Assert(result, qt.Not(qt.Contains), "CREATE TYPE")
}

func TestGenerator_GenerateCreateTable_WithIndexes(t *testing.T) {
	c := qt.New(t)

	generator := mariadb.New()

	table := meta.TableDirective{
		StructName: "User",
		Name:       "users",
		Engine:     "InnoDB",
	}

	fields := []meta.SchemaField{
		{
			StructName: "User",
			Name:       "id",
			Type:       "INT",
			Primary:    true,
			AutoInc:    true,
		},
		{
			StructName: "User",
			Name:       "email",
			Type:       "VARCHAR(320)",
			Nullable:   false,
		},
		{
			StructName: "User",
			Name:       "name",
			Type:       "VARCHAR(255)",
			Nullable:   false,
		},
	}

	indexes := []meta.SchemaIndex{
		{
			StructName: "User",
			Name:       "idx_users_email",
			Fields:     []string{"email"},
			Unique:     true,
		},
		{
			StructName: "User",
			Name:       "idx_users_name",
			Fields:     []string{"name"},
			Unique:     false,
		},
		{
			StructName: "Post", // Different struct - should be ignored
			Name:       "idx_posts_title",
			Fields:     []string{"title"},
		},
	}

	result := generator.GenerateCreateTable(table, fields, indexes, nil)

	c.Assert(result, qt.Contains, "CREATE UNIQUE INDEX idx_users_email ON users (email);")
	c.Assert(result, qt.Contains, "CREATE INDEX idx_users_name ON users (name);")
	c.Assert(result, qt.Not(qt.Contains), "idx_posts_title") // Should not include Post indexes
}

func TestGenerator_GenerateCreateTable_WithTypeOverrides(t *testing.T) {
	c := qt.New(t)

	generator := mariadb.New()

	table := meta.TableDirective{
		StructName: "User",
		Name:       "users",
		Engine:     "InnoDB",
	}

	fields := []meta.SchemaField{
		{
			StructName: "User",
			Name:       "id",
			Type:       "SERIAL",
			Primary:    true,
		},
		{
			StructName: "User",
			Name:       "data",
			Type:       "TEXT", // Default type
			Nullable:   true,
			Overrides: map[string]map[string]string{
				"mariadb": {
					"type": "JSON", // MariaDB-specific override
				},
			},
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "data JSON")         // Should use the MariaDB override
	c.Assert(result, qt.Not(qt.Contains), "data TEXT") // Should not use the default type
}

func TestGenerator_GenerateCreateTable_WithTableOverrides(t *testing.T) {
	c := qt.New(t)

	generator := mariadb.New()

	table := meta.TableDirective{
		StructName: "User",
		Name:       "users",
		Overrides: map[string]map[string]string{
			"mariadb": {
				"engine":  "Aria",
				"comment": "MariaDB user table",
				"charset": "utf8mb4",
			},
		},
	}

	fields := []meta.SchemaField{
		{
			StructName: "User",
			Name:       "id",
			Type:       "INT",
			Primary:    true,
			AutoInc:    true,
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "-- MARIADB TABLE: users (MariaDB user table) --")
	c.Assert(result, qt.Contains, "ENGINE=Aria")
	c.Assert(result, qt.Contains, "charset=utf8mb4")
}

func TestGenerator_GenerateCreateTable_WithCheckOverrides(t *testing.T) {
	c := qt.New(t)

	generator := mariadb.New()

	table := meta.TableDirective{
		StructName: "Product",
		Name:       "products",
	}

	fields := []meta.SchemaField{
		{
			StructName: "Product",
			Name:       "price",
			Type:       "DECIMAL(10,2)",
			Check:      "price > 0", // Default check
			Overrides: map[string]map[string]string{
				"mariadb": {
					"check": "price >= 0", // MariaDB-specific override
				},
			},
		},
		{
			StructName: "Product",
			Name:       "quantity",
			Type:       "INTEGER",
			Check:      "quantity > 5", // Default check
			Overrides: map[string]map[string]string{
				"mysql": {
					"check": "quantity > 0", // MySQL fallback
				},
				"mariadb": {
					"check": "quantity >= 0", // MariaDB-specific override (should take precedence)
				},
			},
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "price DECIMAL(10,2) NOT NULL CHECK (price >= 0)") // Should use MariaDB override
	c.Assert(result, qt.Not(qt.Contains), "CHECK (price > 0)")                       // Should not use default
	c.Assert(result, qt.Contains, "quantity INTEGER NOT NULL CHECK (quantity >= 0)") // Should use MariaDB override
	c.Assert(result, qt.Not(qt.Contains), "CHECK (quantity > 5)")                    // Should not use default
	c.Assert(result, qt.Not(qt.Contains), "CHECK (quantity > 0)")                    // Should not use MySQL fallback
}

func TestGenerator_GenerateCreateTable_CompositeKeys(t *testing.T) {
	c := qt.New(t)

	generator := mariadb.New()

	table := meta.TableDirective{
		StructName: "UserRole",
		Name:       "user_roles",
		PrimaryKey: []string{"user_id", "role_id"},
		Engine:     "InnoDB",
	}

	fields := []meta.SchemaField{
		{
			StructName: "UserRole",
			Name:       "user_id",
			Type:       "INT",
			Nullable:   false,
		},
		{
			StructName: "UserRole",
			Name:       "role_id",
			Type:       "INT",
			Nullable:   false,
		},
		{
			StructName: "UserRole",
			Name:       "assigned_at",
			Type:       "TIMESTAMP",
			Nullable:   false,
			Default:    "CURRENT_TIMESTAMP",
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "user_id INT NOT NULL")
	c.Assert(result, qt.Contains, "role_id INT NOT NULL")
	c.Assert(result, qt.Contains, "PRIMARY KEY (user_id, role_id)")
}

func TestGenerator_GenerateAlterStatements(t *testing.T) {
	c := qt.New(t)

	generator := mariadb.New()

	oldFields := []meta.SchemaField{
		{
			StructName: "User",
			Name:       "email",
			Type:       "TEXT",
			Nullable:   true,
		},
	}

	newFields := []meta.SchemaField{
		{
			StructName: "User",
			Name:       "email",
			Type:       "VARCHAR(320)",
			Nullable:   false,
		},
		{
			StructName: "User",
			Name:       "name",
			Type:       "VARCHAR(255)",
			Nullable:   false,
		},
	}

	result := generator.GenerateAlterStatements(oldFields, newFields)

	// MariaDB should generate appropriate ALTER statements (similar to MySQL)
	c.Assert(result, qt.Contains, "-- ALTER statements: --")
	c.Assert(result, qt.Contains, "ALTER TABLE User ADD COLUMN name VARCHAR(255) NOT NULL;")
	c.Assert(result, qt.Contains, "ALTER TABLE User MODIFY COLUMN email VARCHAR(320) NOT NULL;")
}

func TestGenerator_GenerateCreateTable_IgnoresDifferentStructs(t *testing.T) {
	c := qt.New(t)

	generator := mariadb.New()

	table := meta.TableDirective{
		StructName: "User",
		Name:       "users",
		Engine:     "InnoDB",
	}

	fields := []meta.SchemaField{
		{
			StructName: "User",
			Name:       "id",
			Type:       "INT",
			Primary:    true,
			AutoInc:    true,
		},
		{
			StructName: "Post", // Different struct - should be ignored
			Name:       "title",
			Type:       "VARCHAR(255)",
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "id INT PRIMARY KEY AUTO_INCREMENT")
	c.Assert(result, qt.Not(qt.Contains), "title") // Should not include Post fields
}

func TestGenerator_GenerateCreateTable_MariaDBSpecificFeatures(t *testing.T) {
	c := qt.New(t)

	generator := mariadb.New()

	table := meta.TableDirective{
		StructName: "Log",
		Name:       "logs",
		Engine:     "Aria",
		Overrides: map[string]map[string]string{
			"mariadb": {
				"row_format":    "DYNAMIC",
				"page_checksum": "1",
			},
		},
	}

	fields := []meta.SchemaField{
		{
			StructName: "Log",
			Name:       "id",
			Type:       "BIGINT",
			Primary:    true,
			AutoInc:    true,
		},
		{
			StructName: "Log",
			Name:       "message",
			Type:       "TEXT",
			Nullable:   false,
		},
	}

	result := generator.GenerateCreateTable(table, fields, nil, nil)

	c.Assert(result, qt.Contains, "row_format=DYNAMIC")
	c.Assert(result, qt.Contains, "page_checksum=1")
}
