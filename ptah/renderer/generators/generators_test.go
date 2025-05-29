package generators_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/platform"
	"github.com/denisvmedia/inventario/ptah/renderer/generators"
	"github.com/denisvmedia/inventario/ptah/schema/meta"
)

func TestGenerateCreateTable_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		table    meta.TableDirective
		fields   []meta.SchemaField
		indexes  []meta.SchemaIndex
		enums    []meta.GlobalEnum
		dialect  string
		contains []string // strings that should be present in output
	}{
		{
			name: "simple table with basic fields",
			table: meta.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "id",
					Type:       "SERIAL",
					Primary:    true,
				},
				{
					StructName: "User",
					Name:       "name",
					Type:       "VARCHAR(255)",
					Nullable:   false,
				},
			},
			dialect: "postgres",
			contains: []string{
				"-- POSTGRES TABLE: users --",
				"CREATE TABLE users (",
				"id SERIAL PRIMARY KEY",
				"name VARCHAR(255) NOT NULL",
			},
		},
		{
			name: "table with unique and nullable fields",
			table: meta.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
					Unique:     true,
					Nullable:   false,
				},
				{
					StructName: "User",
					Name:       "bio",
					Type:       "TEXT",
					Nullable:   true,
				},
			},
			dialect: "postgres",
			contains: []string{
				"email VARCHAR(255) UNIQUE NOT NULL",
				"bio TEXT",
			},
		},
		{
			name: "table with default values",
			table: meta.TableDirective{
				StructName: "Post",
				Name:       "posts",
			},
			fields: []meta.SchemaField{
				{
					StructName: "Post",
					Name:       "status",
					Type:       "VARCHAR(50)",
					Default:    "draft",
				},
				{
					StructName: "Post",
					Name:       "created_at",
					Type:       "TIMESTAMP",
					DefaultFn:  "NOW()",
				},
			},
			dialect: "postgres",
			contains: []string{
				"status VARCHAR(50) NOT NULL DEFAULT 'draft'",
				"created_at TIMESTAMP NOT NULL DEFAULT NOW()",
			},
		},
		{
			name: "table with check constraint",
			table: meta.TableDirective{
				StructName: "Product",
				Name:       "products",
			},
			fields: []meta.SchemaField{
				{
					StructName: "Product",
					Name:       "price",
					Type:       "DECIMAL(10,2)",
					Check:      "price > 0",
				},
			},
			dialect: "postgres",
			contains: []string{
				"price DECIMAL(10,2) NOT NULL CHECK (price > 0)",
			},
		},
		{
			name: "table with foreign key",
			table: meta.TableDirective{
				StructName: "Post",
				Name:       "posts",
			},
			fields: []meta.SchemaField{
				{
					StructName:     "Post",
					Name:           "user_id",
					Type:           "INT",
					Foreign:        "users(id)",
					ForeignKeyName: "fk_posts_user",
				},
			},
			dialect: "postgres",
			contains: []string{
				"user_id INT",
				"CONSTRAINT fk_posts_user FOREIGN KEY (user_id) REFERENCES users(id)",
			},
		},
		{
			name: "table with composite primary key",
			table: meta.TableDirective{
				StructName: "UserRole",
				Name:       "user_roles",
				PrimaryKey: []string{"user_id", "role_id"},
			},
			fields: []meta.SchemaField{
				{
					StructName: "UserRole",
					Name:       "user_id",
					Type:       "INT",
				},
				{
					StructName: "UserRole",
					Name:       "role_id",
					Type:       "INT",
				},
			},
			dialect: "postgres",
			contains: []string{
				"user_id INT",
				"role_id INT",
				"PRIMARY KEY (user_id, role_id)",
			},
		},
		{
			name: "table with index",
			table: meta.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
				},
			},
			indexes: []meta.SchemaIndex{
				{
					StructName: "User",
					Name:       "idx_users_email",
					Fields:     []string{"email"},
				},
			},
			dialect: "postgres",
			contains: []string{
				"CREATE INDEX idx_users_email ON users (email);",
			},
		},
		{
			name: "table with unique index",
			table: meta.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "username",
					Type:       "VARCHAR(100)",
				},
			},
			indexes: []meta.SchemaIndex{
				{
					StructName: "User",
					Name:       "idx_users_username_unique",
					Fields:     []string{"username"},
					Unique:     true,
				},
			},
			dialect: "postgres",
			contains: []string{
				"CREATE UNIQUE INDEX idx_users_username_unique ON users (username);",
			},
		},
		{
			name: "postgres table with enums",
			table: meta.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "status",
					Type:       "user_status_enum",
				},
			},
			enums: []meta.GlobalEnum{
				{
					Name:   "user_status_enum",
					Values: []string{"active", "inactive", "pending"},
				},
			},
			dialect: "postgres",
			contains: []string{
				"CREATE TYPE user_status_enum AS ENUM ('active', 'inactive', 'pending');",
				"status user_status_enum",
			},
		},
		{
			name: "mysql dialect without enums",
			table: meta.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "id",
					Type:       "INT AUTO_INCREMENT",
					Primary:    true,
				},
			},
			enums: []meta.GlobalEnum{
				{
					Name:   "user_status_enum",
					Values: []string{"active", "inactive"},
				},
			},
			dialect: "mysql",
			contains: []string{
				"-- MYSQL TABLE: users --",
				"id INT AUTO_INCREMENT PRIMARY KEY",
			},
		},
		{
			name: "table with type override",
			table: meta.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "id",
					Type:       "SERIAL",
					Primary:    true,
					Overrides: map[string]map[string]string{
						"mysql": {"type": "INT AUTO_INCREMENT"},
					},
				},
			},
			dialect: "mysql",
			contains: []string{
				"id INT AUTO_INCREMENT PRIMARY KEY",
			},
		},
		{
			name: "mysql table with enum column",
			table: meta.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "status",
					Type:       "user_status_enum",
					Nullable:   false,
				},
			},
			enums: []meta.GlobalEnum{
				{
					Name:   "user_status_enum",
					Values: []string{"active", "inactive", "pending"},
				},
			},
			dialect: "mysql",
			contains: []string{
				"-- MYSQL TABLE: users --",
				"status ENUM('active', 'inactive', 'pending') NOT NULL",
			},
		},
		{
			name: "mariadb table with enum column",
			table: meta.TableDirective{
				StructName: "Product",
				Name:       "products",
			},
			fields: []meta.SchemaField{
				{
					StructName: "Product",
					Name:       "category",
					Type:       "product_category",
					Nullable:   true,
				},
			},
			enums: []meta.GlobalEnum{
				{
					Name:   "product_category",
					Values: []string{"electronics", "clothing", "food", "books"},
				},
			},
			dialect: "mariadb",
			contains: []string{
				"-- MARIADB TABLE: products --",
				"category ENUM('electronics', 'clothing', 'food', 'books')",
			},
		},
		{
			name: "mysql table with multiple enum columns",
			table: meta.TableDirective{
				StructName: "Order",
				Name:       "orders",
			},
			fields: []meta.SchemaField{
				{
					StructName: "Order",
					Name:       "status",
					Type:       "order_status",
					Nullable:   false,
				},
				{
					StructName: "Order",
					Name:       "payment_method",
					Type:       "payment_method",
					Nullable:   false,
				},
			},
			enums: []meta.GlobalEnum{
				{
					Name:   "order_status",
					Values: []string{"pending", "processing", "shipped", "delivered", "cancelled"},
				},
				{
					Name:   "payment_method",
					Values: []string{"credit_card", "paypal", "bank_transfer", "cash"},
				},
			},
			dialect: "mysql",
			contains: []string{
				"status ENUM('pending', 'processing', 'shipped', 'delivered', 'cancelled') NOT NULL",
				"payment_method ENUM('credit_card', 'paypal', 'bank_transfer', 'cash') NOT NULL",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := generators.GenerateCreateTable(tt.table, tt.fields, tt.indexes, tt.enums, tt.dialect)

			for _, expected := range tt.contains {
				c.Assert(result, qt.Contains, expected, qt.Commentf("Expected to contain: %s", expected))
			}
		})
	}
}

func TestGenerateCreateTable_UnhappyPath(t *testing.T) {
	tests := []struct {
		name        string
		table       meta.TableDirective
		fields      []meta.SchemaField
		indexes     []meta.SchemaIndex
		enums       []meta.GlobalEnum
		dialect     string
		notContains []string // strings that should NOT be present in output
	}{
		{
			name: "fields from different struct are ignored",
			table: meta.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "id",
					Type:       "SERIAL",
					Primary:    true,
				},
				{
					StructName: "Post", // Different struct
					Name:       "title",
					Type:       "VARCHAR(255)",
				},
			},
			dialect: "postgres",
			notContains: []string{
				"title VARCHAR(255)",
			},
		},
		{
			name: "indexes from different struct are ignored",
			table: meta.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
				},
			},
			indexes: []meta.SchemaIndex{
				{
					StructName: "Post", // Different struct
					Name:       "idx_posts_title",
					Fields:     []string{"title"},
				},
			},
			dialect: "postgres",
			notContains: []string{
				"idx_posts_title",
			},
		},
		{
			name: "foreign key without foreign field is ignored",
			table: meta.TableDirective{
				StructName: "Post",
				Name:       "posts",
			},
			fields: []meta.SchemaField{
				{
					StructName:     "Post",
					Name:           "user_id",
					Type:           "INT",
					Foreign:        "", // Empty foreign key
					ForeignKeyName: "fk_posts_user",
				},
			},
			dialect: "postgres",
			notContains: []string{
				"CONSTRAINT fk_posts_user",
			},
		},
		{
			name: "enums not generated for non-postgres dialects",
			table: meta.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "status",
					Type:       "user_status_enum",
				},
			},
			enums: []meta.GlobalEnum{
				{
					Name:   "user_status_enum",
					Values: []string{"active", "inactive"},
				},
			},
			dialect: "mysql",
			notContains: []string{
				"CREATE TYPE user_status_enum AS ENUM",
			},
		},
		{
			name: "non-existent enum reference",
			table: meta.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "status",
					Type:       "non_existent_enum", // No matching enum definition
				},
			},
			enums: []meta.GlobalEnum{
				{
					Name:   "user_status_enum", // Different name
					Values: []string{"active", "inactive"},
				},
			},
			dialect: "mysql",
			notContains: []string{
				"status ENUM('active', 'inactive')",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := generators.GenerateCreateTable(tt.table, tt.fields, tt.indexes, tt.enums, tt.dialect)

			for _, notExpected := range tt.notContains {
				c.Assert(result, qt.Not(qt.Contains), notExpected, qt.Commentf("Expected NOT to contain: %s", notExpected))
			}
		})
	}
}

func TestGenerateAlterStatements_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		oldFields []meta.SchemaField
		newFields []meta.SchemaField
		contains  []string
	}{
		{
			name: "add new column",
			oldFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "id",
					Type:       "SERIAL",
				},
			},
			newFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "id",
					Type:       "SERIAL",
				},
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
				},
			},
			contains: []string{
				"-- ALTER statements: --",
				"ALTER TABLE User ADD COLUMN email VARCHAR(255) NOT NULL;",
			},
		},
		{
			name: "change column type",
			oldFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "name",
					Type:       "VARCHAR(100)",
				},
			},
			newFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "name",
					Type:       "TEXT",
				},
			},
			contains: []string{
				"ALTER TABLE User ALTER COLUMN name TYPE TEXT;",
			},
		},
		{
			name: "change nullable to not null",
			oldFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
					Nullable:   true,
				},
			},
			newFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
					Nullable:   false,
				},
			},
			contains: []string{
				"ALTER TABLE User ALTER COLUMN email SET NOT NULL;",
			},
		},
		{
			name: "change not null to nullable",
			oldFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "bio",
					Type:       "TEXT",
					Nullable:   false,
				},
			},
			newFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "bio",
					Type:       "TEXT",
					Nullable:   true,
				},
			},
			contains: []string{
				"ALTER TABLE User ALTER COLUMN bio DROP NOT NULL;",
			},
		},
		{
			name: "multiple changes",
			oldFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "name",
					Type:       "VARCHAR(100)",
					Nullable:   true,
				},
			},
			newFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "name",
					Type:       "TEXT",
					Nullable:   false,
				},
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
				},
			},
			contains: []string{
				"ALTER TABLE User ALTER COLUMN name TYPE TEXT;",
				"ALTER TABLE User ALTER COLUMN name SET NOT NULL;",
				"ALTER TABLE User ADD COLUMN email VARCHAR(255) NOT NULL;",
			},
		},
		{
			name: "no changes needed",
			oldFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "id",
					Type:       "SERIAL",
					Nullable:   false,
				},
			},
			newFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "id",
					Type:       "SERIAL",
					Nullable:   false,
				},
			},
			contains: []string{
				"-- ALTER statements: --",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := generators.GenerateAlterStatements(tt.oldFields, tt.newFields, platform.PlatformTypePostgres)

			for _, expected := range tt.contains {
				c.Assert(result, qt.Contains, expected, qt.Commentf("Expected to contain: %s", expected))
			}
		})
	}
}

func TestGenerateAlterStatements_UnhappyPath(t *testing.T) {
	tests := []struct {
		name        string
		oldFields   []meta.SchemaField
		newFields   []meta.SchemaField
		notContains []string
	}{
		{
			name: "fields from different structs don't generate cross-struct changes",
			oldFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "name",
					Type:       "VARCHAR(100)",
				},
			},
			newFields: []meta.SchemaField{
				{
					StructName: "Post", // Different struct
					Name:       "name",
					Type:       "TEXT",
				},
			},
			notContains: []string{
				"ALTER TABLE User ALTER COLUMN name TYPE TEXT;",
				"ALTER TABLE Post ALTER COLUMN name TYPE TEXT;",
			},
		},
		{
			name: "identical fields don't generate unnecessary changes",
			oldFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
					Nullable:   true,
				},
			},
			newFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
					Nullable:   true,
				},
			},
			notContains: []string{
				"ALTER TABLE User ALTER COLUMN email TYPE VARCHAR(255);",
				"ALTER TABLE User ALTER COLUMN email SET NOT NULL;",
				"ALTER TABLE User ALTER COLUMN email DROP NOT NULL;",
			},
		},
		{
			name:      "empty old fields don't generate alter statements",
			oldFields: nil,
			newFields: []meta.SchemaField{
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
				},
			},
			notContains: []string{
				"ALTER TABLE User ALTER COLUMN email TYPE VARCHAR(255);",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := generators.GenerateAlterStatements(tt.oldFields, tt.newFields, platform.PlatformTypePostgres)

			for _, notExpected := range tt.notContains {
				c.Assert(result, qt.Not(qt.Contains), notExpected, qt.Commentf("Expected NOT to contain: %s", notExpected))
			}
		})
	}
}

// Test behavior with dialects that don't explicitly support enums
func TestGenerateCreateTableWithUnsupportedDialect(t *testing.T) {
	c := qt.New(t)

	table := meta.TableDirective{
		StructName: "User",
		Name:       "users",
	}

	fields := []meta.SchemaField{
		{
			StructName: "User",
			Name:       "status",
			Type:       "enum_user_status",
			Nullable:   false,
		},
	}

	enums := []meta.GlobalEnum{
		{
			Name:   "enum_user_status",
			Values: []string{"active", "inactive"},
		},
	}

	// Test with a made-up dialect
	sql := generators.GenerateCreateTable(table, fields, nil, enums, "sqlite")

	// Verify it doesn't contain enum-specific syntax
	c.Assert(sql, qt.Not(qt.Contains), "CREATE TYPE")
	c.Assert(sql, qt.Not(qt.Contains), "ENUM(")

	// Verify it uses the enum name directly
	c.Assert(sql, qt.Contains, "status enum_user_status")
}

// Test generating alter statements for enums
func TestGenerateAlterStatementsWithEnums(t *testing.T) {
	c := qt.New(t)

	oldFields := []meta.SchemaField{
		{
			StructName: "Product",
			Name:       "status",
			Type:       "enum_product_status",
			Nullable:   false,
		},
	}

	newFields := []meta.SchemaField{
		{
			StructName: "Product",
			Name:       "status",
			Type:       "enum_product_status_v2", // Changed enum type
			Nullable:   false,
		},
		{
			StructName: "Product",
			Name:       "new_status", // New enum field
			Type:       "enum_product_status",
			Nullable:   true,
		},
	}

	alterSQL := generators.GenerateAlterStatements(oldFields, newFields, platform.PlatformTypePostgres)

	// Verify alter statements contain expected changes
	c.Assert(alterSQL, qt.Contains, "ALTER TABLE Product ALTER COLUMN status TYPE enum_product_status_v2;")
	c.Assert(alterSQL, qt.Contains, "ALTER TABLE Product ADD COLUMN new_status enum_product_status")
}
