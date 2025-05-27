package migratorlib_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib"
	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/types"
)

func TestGenerateCreateTable_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		table    types.TableDirective
		fields   []types.SchemaField
		indexes  []types.SchemaIndex
		enums    []types.GlobalEnum
		dialect  string
		contains []string // strings that should be present in output
	}{
		{
			name: "simple table with basic fields",
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
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
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
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
			table: types.TableDirective{
				StructName: "Post",
				Name:       "posts",
			},
			fields: []types.SchemaField{
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
			table: types.TableDirective{
				StructName: "Product",
				Name:       "products",
			},
			fields: []types.SchemaField{
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
			table: types.TableDirective{
				StructName: "Post",
				Name:       "posts",
			},
			fields: []types.SchemaField{
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
			table: types.TableDirective{
				StructName: "UserRole",
				Name:       "user_roles",
				PrimaryKey: []string{"user_id", "role_id"},
			},
			fields: []types.SchemaField{
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
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
				},
			},
			indexes: []types.SchemaIndex{
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
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "username",
					Type:       "VARCHAR(100)",
				},
			},
			indexes: []types.SchemaIndex{
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
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "status",
					Type:       "user_status_enum",
				},
			},
			enums: []types.GlobalEnum{
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
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "id",
					Type:       "INT AUTO_INCREMENT",
					Primary:    true,
				},
			},
			enums: []types.GlobalEnum{
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
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
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
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "status",
					Type:       "user_status_enum",
					Nullable:   false,
				},
			},
			enums: []types.GlobalEnum{
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
			table: types.TableDirective{
				StructName: "Product",
				Name:       "products",
			},
			fields: []types.SchemaField{
				{
					StructName: "Product",
					Name:       "category",
					Type:       "product_category",
					Nullable:   true,
				},
			},
			enums: []types.GlobalEnum{
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
			table: types.TableDirective{
				StructName: "Order",
				Name:       "orders",
			},
			fields: []types.SchemaField{
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
			enums: []types.GlobalEnum{
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

			result := migratorlib.GenerateCreateTable(tt.table, tt.fields, tt.indexes, tt.enums, tt.dialect)

			for _, expected := range tt.contains {
				c.Assert(result, qt.Contains, expected, qt.Commentf("Expected to contain: %s", expected))
			}
		})
	}
}

func TestGenerateCreateTable_UnhappyPath(t *testing.T) {
	tests := []struct {
		name        string
		table       types.TableDirective
		fields      []types.SchemaField
		indexes     []types.SchemaIndex
		enums       []types.GlobalEnum
		dialect     string
		notContains []string // strings that should NOT be present in output
	}{
		{
			name: "fields from different struct are ignored",
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
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
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
				},
			},
			indexes: []types.SchemaIndex{
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
			table: types.TableDirective{
				StructName: "Post",
				Name:       "posts",
			},
			fields: []types.SchemaField{
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
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "status",
					Type:       "user_status_enum",
				},
			},
			enums: []types.GlobalEnum{
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
			table: types.TableDirective{
				StructName: "User",
				Name:       "users",
			},
			fields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "status",
					Type:       "non_existent_enum", // No matching enum definition
				},
			},
			enums: []types.GlobalEnum{
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

			result := migratorlib.GenerateCreateTable(tt.table, tt.fields, tt.indexes, tt.enums, tt.dialect)

			for _, notExpected := range tt.notContains {
				c.Assert(result, qt.Not(qt.Contains), notExpected, qt.Commentf("Expected NOT to contain: %s", notExpected))
			}
		})
	}
}

func TestGenerateAlterStatements_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		oldFields []types.SchemaField
		newFields []types.SchemaField
		contains  []string
	}{
		{
			name: "add new column",
			oldFields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "id",
					Type:       "SERIAL",
				},
			},
			newFields: []types.SchemaField{
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
				"ALTER TABLE User ADD COLUMN email VARCHAR(255);",
			},
		},
		{
			name: "change column type",
			oldFields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "name",
					Type:       "VARCHAR(100)",
				},
			},
			newFields: []types.SchemaField{
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
			oldFields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
					Nullable:   true,
				},
			},
			newFields: []types.SchemaField{
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
			oldFields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "bio",
					Type:       "TEXT",
					Nullable:   false,
				},
			},
			newFields: []types.SchemaField{
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
			oldFields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "name",
					Type:       "VARCHAR(100)",
					Nullable:   true,
				},
			},
			newFields: []types.SchemaField{
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
				"ALTER TABLE User ADD COLUMN email VARCHAR(255);",
			},
		},
		{
			name: "no changes needed",
			oldFields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "id",
					Type:       "SERIAL",
					Nullable:   false,
				},
			},
			newFields: []types.SchemaField{
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

			result := migratorlib.GenerateAlterStatements(tt.oldFields, tt.newFields)

			for _, expected := range tt.contains {
				c.Assert(result, qt.Contains, expected, qt.Commentf("Expected to contain: %s", expected))
			}
		})
	}
}

func TestGenerateAlterStatements_UnhappyPath(t *testing.T) {
	tests := []struct {
		name        string
		oldFields   []types.SchemaField
		newFields   []types.SchemaField
		notContains []string
	}{
		{
			name: "fields from different structs don't generate cross-struct changes",
			oldFields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "name",
					Type:       "VARCHAR(100)",
				},
			},
			newFields: []types.SchemaField{
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
			oldFields: []types.SchemaField{
				{
					StructName: "User",
					Name:       "email",
					Type:       "VARCHAR(255)",
					Nullable:   true,
				},
			},
			newFields: []types.SchemaField{
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
			newFields: []types.SchemaField{
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

			result := migratorlib.GenerateAlterStatements(tt.oldFields, tt.newFields)

			for _, notExpected := range tt.notContains {
				c.Assert(result, qt.Not(qt.Contains), notExpected, qt.Commentf("Expected NOT to contain: %s", notExpected))
			}
		})
	}
}

// Test behavior with dialects that don't explicitly support enums
func TestGenerateCreateTableWithUnsupportedDialect(t *testing.T) {
	c := qt.New(t)

	table := types.TableDirective{
		StructName: "User",
		Name:       "users",
	}

	fields := []types.SchemaField{
		{
			StructName: "User",
			Name:       "status",
			Type:       "enum_user_status",
			Nullable:   false,
		},
	}

	enums := []types.GlobalEnum{
		{
			Name:   "enum_user_status",
			Values: []string{"active", "inactive"},
		},
	}

	// Test with a made-up dialect
	sql := migratorlib.GenerateCreateTable(table, fields, nil, enums, "sqlite")

	// Verify it doesn't contain enum-specific syntax
	c.Assert(sql, qt.Not(qt.Contains), "CREATE TYPE")
	c.Assert(sql, qt.Not(qt.Contains), "ENUM(")

	// Verify it uses the enum name directly
	c.Assert(sql, qt.Contains, "status enum_user_status")
}

// Test generating alter statements for enums
func TestGenerateAlterStatementsWithEnums(t *testing.T) {
	c := qt.New(t)

	oldFields := []types.SchemaField{
		{
			StructName: "Product",
			Name:       "status",
			Type:       "enum_product_status",
			Nullable:   false,
		},
	}

	newFields := []types.SchemaField{
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

	alterSQL := migratorlib.GenerateAlterStatements(oldFields, newFields)

	// Verify alter statements contain expected changes
	c.Assert(alterSQL, qt.Contains, "ALTER TABLE Product ALTER COLUMN status TYPE enum_product_status_v2;")
	c.Assert(alterSQL, qt.Contains, "ALTER TABLE Product ADD COLUMN new_status enum_product_status;")
}
