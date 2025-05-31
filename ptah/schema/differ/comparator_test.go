package differ_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/ptr"

	"github.com/denisvmedia/inventario/ptah/schema/differ"
	"github.com/denisvmedia/inventario/ptah/schema/differ/differtypes"
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

func TestCompareSchemas_EmptySchemas(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables:         []types.TableDirective{},
		Fields:         []types.SchemaField{},
		Indexes:        []types.SchemaIndex{},
		Enums:          []types.GlobalEnum{},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables:  []parsertypes.Table{},
		Enums:   []parsertypes.Enum{},
		Indexes: []parsertypes.Index{},
	}

	diff := differ.CompareSchemas(generated, database)

	c.Assert(diff.HasChanges(), qt.IsFalse)
	c.Assert(diff.TablesAdded, qt.HasLen, 0)
	c.Assert(diff.TablesRemoved, qt.HasLen, 0)
	c.Assert(diff.TablesModified, qt.HasLen, 0)
	c.Assert(diff.EnumsAdded, qt.HasLen, 0)
	c.Assert(diff.EnumsRemoved, qt.HasLen, 0)
	c.Assert(diff.EnumsModified, qt.HasLen, 0)
	c.Assert(diff.IndexesAdded, qt.HasLen, 0)
	c.Assert(diff.IndexesRemoved, qt.HasLen, 0)
}

func TestCompareSchemas_TablesAdded(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables: []types.TableDirective{
			{StructName: "User", Name: "users"},
			{StructName: "Post", Name: "posts"},
		},
		Fields:         []types.SchemaField{},
		Indexes:        []types.SchemaIndex{},
		Enums:          []types.GlobalEnum{},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables:  []parsertypes.Table{},
		Enums:   []parsertypes.Enum{},
		Indexes: []parsertypes.Index{},
	}

	diff := differ.CompareSchemas(generated, database)

	c.Assert(diff.HasChanges(), qt.IsTrue)
	c.Assert(diff.TablesAdded, qt.HasLen, 2)
	c.Assert(diff.TablesAdded, qt.DeepEquals, []string{"posts", "users"}) // Should be sorted
	c.Assert(diff.TablesRemoved, qt.HasLen, 0)
	c.Assert(diff.TablesModified, qt.HasLen, 0)
}

func TestCompareSchemas_TablesRemoved(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables:         []types.TableDirective{},
		Fields:         []types.SchemaField{},
		Indexes:        []types.SchemaIndex{},
		Enums:          []types.GlobalEnum{},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables: []parsertypes.Table{
			{Name: "old_users"},
			{Name: "old_posts"},
		},
		Enums:   []parsertypes.Enum{},
		Indexes: []parsertypes.Index{},
	}

	diff := differ.CompareSchemas(generated, database)

	c.Assert(diff.HasChanges(), qt.IsTrue)
	c.Assert(diff.TablesAdded, qt.HasLen, 0)
	c.Assert(diff.TablesRemoved, qt.HasLen, 2)
	c.Assert(diff.TablesRemoved, qt.DeepEquals, []string{"old_posts", "old_users"}) // Should be sorted
	c.Assert(diff.TablesModified, qt.HasLen, 0)
}

func TestCompareSchemas_EnumsAdded(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables:  []types.TableDirective{},
		Fields:  []types.SchemaField{},
		Indexes: []types.SchemaIndex{},
		Enums: []types.GlobalEnum{
			{Name: "status_enum", Values: []string{"active", "inactive"}},
			{Name: "role_enum", Values: []string{"admin", "user"}},
		},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables:  []parsertypes.Table{},
		Enums:   []parsertypes.Enum{},
		Indexes: []parsertypes.Index{},
	}

	diff := differ.CompareSchemas(generated, database)

	c.Assert(diff.HasChanges(), qt.IsTrue)
	c.Assert(diff.EnumsAdded, qt.HasLen, 2)
	c.Assert(diff.EnumsAdded, qt.DeepEquals, []string{"role_enum", "status_enum"}) // Should be sorted
	c.Assert(diff.EnumsRemoved, qt.HasLen, 0)
	c.Assert(diff.EnumsModified, qt.HasLen, 0)
}

func TestCompareSchemas_EnumsRemoved(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables:         []types.TableDirective{},
		Fields:         []types.SchemaField{},
		Indexes:        []types.SchemaIndex{},
		Enums:          []types.GlobalEnum{},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables: []parsertypes.Table{},
		Enums: []parsertypes.Enum{
			{Name: "old_status", Values: []string{"active", "inactive"}},
			{Name: "old_role", Values: []string{"admin", "user"}},
		},
		Indexes: []parsertypes.Index{},
	}

	diff := differ.CompareSchemas(generated, database)

	c.Assert(diff.HasChanges(), qt.IsTrue)
	c.Assert(diff.EnumsAdded, qt.HasLen, 0)
	c.Assert(diff.EnumsRemoved, qt.HasLen, 2)
	c.Assert(diff.EnumsRemoved, qt.DeepEquals, []string{"old_role", "old_status"}) // Should be sorted
	c.Assert(diff.EnumsModified, qt.HasLen, 0)
}

func TestCompareSchemas_EnumsModified(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables:  []types.TableDirective{},
		Fields:  []types.SchemaField{},
		Indexes: []types.SchemaIndex{},
		Enums: []types.GlobalEnum{
			{Name: "status_enum", Values: []string{"active", "inactive", "pending"}}, // Added "pending"
		},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables: []parsertypes.Table{},
		Enums: []parsertypes.Enum{
			{Name: "status_enum", Values: []string{"active", "inactive", "deleted"}}, // Has "deleted" instead of "pending"
		},
		Indexes: []parsertypes.Index{},
	}

	diff := differ.CompareSchemas(generated, database)

	c.Assert(diff.HasChanges(), qt.IsTrue)
	c.Assert(diff.EnumsAdded, qt.HasLen, 0)
	c.Assert(diff.EnumsRemoved, qt.HasLen, 0)
	c.Assert(diff.EnumsModified, qt.HasLen, 1)

	enumDiff := diff.EnumsModified[0]
	c.Assert(enumDiff.EnumName, qt.Equals, "status_enum")
	c.Assert(enumDiff.ValuesAdded, qt.DeepEquals, []string{"pending"})
	c.Assert(enumDiff.ValuesRemoved, qt.DeepEquals, []string{"deleted"})
}

func TestCompareSchemas_IndexesAddedAndRemoved(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables: []types.TableDirective{},
		Fields: []types.SchemaField{},
		Indexes: []types.SchemaIndex{
			{Name: "idx_user_email"},
			{Name: "idx_user_name"},
		},
		Enums:          []types.GlobalEnum{},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables: []parsertypes.Table{},
		Enums:  []parsertypes.Enum{},
		Indexes: []parsertypes.Index{
			{Name: "idx_user_email"},            // Exists in both
			{Name: "old_idx_user_phone"},        // Only in database
			{Name: "pk_users", IsPrimary: true}, // Primary key index - should be ignored
		},
	}

	diff := differ.CompareSchemas(generated, database)

	c.Assert(diff.HasChanges(), qt.IsTrue)
	c.Assert(diff.IndexesAdded, qt.HasLen, 1)
	c.Assert(diff.IndexesAdded, qt.DeepEquals, []string{"idx_user_name"})
	c.Assert(diff.IndexesRemoved, qt.HasLen, 1)
	c.Assert(diff.IndexesRemoved, qt.DeepEquals, []string{"old_idx_user_phone"})
}

func TestCompareSchemas_TablesModified_ColumnsAddedAndRemoved(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables: []types.TableDirective{
			{StructName: "User", Name: "users"},
		},
		Fields: []types.SchemaField{
			{StructName: "User", Name: "id", Type: "INTEGER", Primary: true},
			{StructName: "User", Name: "email", Type: "VARCHAR", Nullable: false}, // New field
			{StructName: "User", Name: "name", Type: "VARCHAR", Nullable: false},  // Existing field
		},
		Indexes:        []types.SchemaIndex{},
		Enums:          []types.GlobalEnum{},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables: []parsertypes.Table{
			{
				Name: "users",
				Columns: []parsertypes.Column{
					{Name: "id", DataType: "INTEGER", IsPrimaryKey: true, IsNullable: "NO"},
					{Name: "name", DataType: "VARCHAR", IsNullable: "NO"},
					{Name: "phone", DataType: "VARCHAR", IsNullable: "YES"}, // Field to be removed
				},
			},
		},
		Enums:   []parsertypes.Enum{},
		Indexes: []parsertypes.Index{},
	}

	diff := differ.CompareSchemas(generated, database)

	c.Assert(diff.HasChanges(), qt.IsTrue)
	c.Assert(diff.TablesModified, qt.HasLen, 1)

	tableDiff := diff.TablesModified[0]
	c.Assert(tableDiff.TableName, qt.Equals, "users")
	c.Assert(tableDiff.ColumnsAdded, qt.DeepEquals, []string{"email"})
	c.Assert(tableDiff.ColumnsRemoved, qt.DeepEquals, []string{"phone"})
	c.Assert(tableDiff.ColumnsModified, qt.HasLen, 0)
}

func TestCompareSchemas_TablesModified_ColumnsModified(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables: []types.TableDirective{
			{StructName: "User", Name: "users"},
		},
		Fields: []types.SchemaField{
			{StructName: "User", Name: "id", Type: "INTEGER", Primary: true},
			{StructName: "User", Name: "email", Type: "VARCHAR", Nullable: false, Unique: true},
			{StructName: "User", Name: "status", Type: "VARCHAR", Nullable: true, Default: "active"},
		},
		Indexes:        []types.SchemaIndex{},
		Enums:          []types.GlobalEnum{},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables: []parsertypes.Table{
			{
				Name: "users",
				Columns: []parsertypes.Column{
					{Name: "id", DataType: "INTEGER", IsPrimaryKey: true, IsNullable: "NO"},
					{Name: "email", DataType: "TEXT", IsNullable: "YES", IsUnique: false},                     // Type and nullable changed
					{Name: "status", DataType: "VARCHAR", IsNullable: "NO", ColumnDefault: ptr.To("pending")}, // Nullable and default changed
				},
			},
		},
		Enums:   []parsertypes.Enum{},
		Indexes: []parsertypes.Index{},
	}

	diff := differ.CompareSchemas(generated, database)

	c.Assert(diff.HasChanges(), qt.IsTrue)
	c.Assert(diff.TablesModified, qt.HasLen, 1)

	tableDiff := diff.TablesModified[0]
	c.Assert(tableDiff.TableName, qt.Equals, "users")
	c.Assert(tableDiff.ColumnsAdded, qt.HasLen, 0)
	c.Assert(tableDiff.ColumnsRemoved, qt.HasLen, 0)
	c.Assert(tableDiff.ColumnsModified, qt.HasLen, 2)

	// Check email column changes
	emailDiff := findColumnDiff(tableDiff.ColumnsModified, "email")
	c.Assert(emailDiff, qt.IsNotNil)
	c.Assert(emailDiff.Changes["type"], qt.Equals, "text -> varchar")
	c.Assert(emailDiff.Changes["nullable"], qt.Equals, "true -> false")
	c.Assert(emailDiff.Changes["unique"], qt.Equals, "false -> true")

	// Check status column changes
	statusDiff := findColumnDiff(tableDiff.ColumnsModified, "status")
	c.Assert(statusDiff, qt.IsNotNil)
	c.Assert(statusDiff.Changes["nullable"], qt.Equals, "false -> true")
	c.Assert(statusDiff.Changes["default"], qt.Equals, "'pending' -> 'active'")
}

// Helper function to find a column diff by name
func findColumnDiff(diffs []differtypes.ColumnDiff, columnName string) *differtypes.ColumnDiff {
	for _, diff := range diffs {
		if diff.ColumnName == columnName {
			return &diff
		}
	}
	return nil
}

func TestCompareSchemas_WithEmbeddedFields(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables: []types.TableDirective{
			{StructName: "User", Name: "users"},
		},
		Fields: []types.SchemaField{
			{StructName: "User", Name: "id", Type: "INTEGER", Primary: true},
			{StructName: "User", Name: "name", Type: "VARCHAR", Nullable: false},
		},
		Indexes: []types.SchemaIndex{},
		Enums:   []types.GlobalEnum{},
		EmbeddedFields: []types.EmbeddedField{
			{
				StructName:       "User",
				Mode:             "inline",
				EmbeddedTypeName: "Timestamps",
			},
		},
	}

	// Mock the ProcessEmbeddedFields function behavior
	// In real scenario, this would be called by the comparator
	// For testing, we'll add the expected embedded fields to the Fields slice
	generated.Fields = append(generated.Fields, []types.SchemaField{
		{StructName: "User", Name: "created_at", Type: "TIMESTAMP", Nullable: false},
		{StructName: "User", Name: "updated_at", Type: "TIMESTAMP", Nullable: false},
	}...)

	database := &parsertypes.DatabaseSchema{
		Tables: []parsertypes.Table{
			{
				Name: "users",
				Columns: []parsertypes.Column{
					{Name: "id", DataType: "INTEGER", IsPrimaryKey: true, IsNullable: "NO"},
					{Name: "name", DataType: "VARCHAR", IsNullable: "NO"},
					{Name: "created_at", DataType: "TIMESTAMP", IsNullable: "NO"},
					// missing updated_at column
				},
			},
		},
		Enums:   []parsertypes.Enum{},
		Indexes: []parsertypes.Index{},
	}

	diff := differ.CompareSchemas(generated, database)

	c.Assert(diff.HasChanges(), qt.IsTrue)
	c.Assert(diff.TablesModified, qt.HasLen, 1)

	tableDiff := diff.TablesModified[0]
	c.Assert(tableDiff.TableName, qt.Equals, "users")
	c.Assert(tableDiff.ColumnsAdded, qt.DeepEquals, []string{"updated_at"})
	c.Assert(tableDiff.ColumnsRemoved, qt.HasLen, 0)
	c.Assert(tableDiff.ColumnsModified, qt.HasLen, 0)
}

func TestGenerateMigrationSQL_PostgreSQL(t *testing.T) {
	c := qt.New(t)

	diff := &differtypes.SchemaDiff{
		TablesAdded:   []string{"users"},
		TablesRemoved: []string{"old_table"},
		EnumsAdded:    []string{"status_enum"},
		EnumsRemoved:  []string{"old_enum"},
		EnumsModified: []differtypes.EnumDiff{
			{
				EnumName:      "role_enum",
				ValuesAdded:   []string{"moderator"},
				ValuesRemoved: []string{"guest"},
			},
		},
		IndexesAdded:   []string{"idx_user_email"},
		IndexesRemoved: []string{"old_index"},
	}

	generated := &parsertypes.PackageParseResult{
		Enums: []types.GlobalEnum{
			{Name: "status_enum", Values: []string{"active", "inactive"}},
		},
	}

	statements := differ.GenerateMigrationSQL(diff, generated, "postgres")

	c.Assert(statements, qt.Not(qt.HasLen), 0)

	// Check that enum creation is included
	found := false
	for _, stmt := range statements {
		if strings.Contains(stmt, "CREATE TYPE status_enum AS ENUM") {
			found = true
			c.Assert(stmt, qt.Contains, "'active'")
			c.Assert(stmt, qt.Contains, "'inactive'")
			break
		}
	}
	c.Assert(found, qt.IsTrue)

	// Check that enum modification is included
	found = false
	for _, stmt := range statements {
		if strings.Contains(stmt, "ALTER TYPE role_enum ADD VALUE 'moderator'") {
			found = true
			break
		}
	}
	c.Assert(found, qt.IsTrue)

	// Check that warnings for removed enum values are included
	found = false
	for _, stmt := range statements {
		if strings.Contains(stmt, "WARNING: Cannot remove enum values") && strings.Contains(stmt, "guest") {
			found = true
			break
		}
	}
	c.Assert(found, qt.IsTrue)

	// Check that index removal is included
	found = false
	for _, stmt := range statements {
		if strings.Contains(stmt, "DROP INDEX IF EXISTS old_index") {
			found = true
			break
		}
	}
	c.Assert(found, qt.IsTrue)

	// Check that table removal warning is included
	found = false
	for _, stmt := range statements {
		if strings.Contains(stmt, "WARNING: DROP TABLE old_table") {
			found = true
			break
		}
	}
	c.Assert(found, qt.IsTrue)

	// Check that enum removal warning is included
	found = false
	for _, stmt := range statements {
		if strings.Contains(stmt, "WARNING: DROP TYPE old_enum") {
			found = true
			break
		}
	}
	c.Assert(found, qt.IsTrue)
}

func TestGenerateMigrationSQL_NonPostgreSQL(t *testing.T) {
	c := qt.New(t)

	diff := &differtypes.SchemaDiff{
		EnumsAdded: []string{"status_enum"},
		EnumsModified: []differtypes.EnumDiff{
			{
				EnumName:    "role_enum",
				ValuesAdded: []string{"moderator"},
			},
		},
	}

	generated := &parsertypes.PackageParseResult{
		Enums: []types.GlobalEnum{
			{Name: "status_enum", Values: []string{"active", "inactive"}},
		},
	}

	statements := differ.GenerateMigrationSQL(diff, generated, "mysql")

	// For non-PostgreSQL dialects, enum operations should not generate SQL
	for _, stmt := range statements {
		c.Assert(stmt, qt.Not(qt.Contains), "CREATE TYPE")
		c.Assert(stmt, qt.Not(qt.Contains), "ALTER TYPE")
	}
}

func TestCompareSchemas_PrimaryKeyHandling(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables: []types.TableDirective{
			{StructName: "User", Name: "users"},
		},
		Fields: []types.SchemaField{
			{StructName: "User", Name: "id", Type: "INTEGER", Primary: true, Nullable: true}, // Primary key should override nullable
		},
		Indexes:        []types.SchemaIndex{},
		Enums:          []types.GlobalEnum{},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables: []parsertypes.Table{
			{
				Name: "users",
				Columns: []parsertypes.Column{
					{Name: "id", DataType: "INTEGER", IsPrimaryKey: true, IsNullable: "NO"},
				},
			},
		},
		Enums:   []parsertypes.Enum{},
		Indexes: []parsertypes.Index{},
	}

	diff := differ.CompareSchemas(generated, database)

	// Should not detect any changes because primary keys are always NOT NULL
	c.Assert(diff.HasChanges(), qt.IsFalse)
	c.Assert(diff.TablesModified, qt.HasLen, 0)
}

func TestCompareSchemas_UDTNameHandling(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables: []types.TableDirective{
			{StructName: "User", Name: "users"},
		},
		Fields: []types.SchemaField{
			{StructName: "User", Name: "status", Type: "status_enum", Nullable: false},
		},
		Indexes:        []types.SchemaIndex{},
		Enums:          []types.GlobalEnum{},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables: []parsertypes.Table{
			{
				Name: "users",
				Columns: []parsertypes.Column{
					{Name: "status", DataType: "USER-DEFINED", UDTName: "status_enum", IsNullable: "NO"},
				},
			},
		},
		Enums:   []parsertypes.Enum{},
		Indexes: []parsertypes.Index{},
	}

	diff := differ.CompareSchemas(generated, database)

	// Should not detect any changes because UDTName should be used for comparison
	c.Assert(diff.HasChanges(), qt.IsFalse)
	c.Assert(diff.TablesModified, qt.HasLen, 0)
}

func TestCompareSchemas_EmptyDefaultValues(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables: []types.TableDirective{
			{StructName: "User", Name: "users"},
		},
		Fields: []types.SchemaField{
			{StructName: "User", Name: "status", Type: "VARCHAR", Default: ""}, // Empty default
		},
		Indexes:        []types.SchemaIndex{},
		Enums:          []types.GlobalEnum{},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables: []parsertypes.Table{
			{
				Name: "users",
				Columns: []parsertypes.Column{
					{Name: "status", DataType: "VARCHAR", ColumnDefault: nil}, // NULL default
				},
			},
		},
		Enums:   []parsertypes.Enum{},
		Indexes: []parsertypes.Index{},
	}

	diff := differ.CompareSchemas(generated, database)

	c.Assert(diff.HasChanges(), qt.IsFalse)
	c.Assert(diff.TablesModified, qt.HasLen, 0)
}

func TestCompareSchemas_ComplexScenario(t *testing.T) {
	c := qt.New(t)

	generated := &parsertypes.PackageParseResult{
		Tables: []types.TableDirective{
			{StructName: "User", Name: "users"},
			{StructName: "Post", Name: "posts"}, // New table
		},
		Fields: []types.SchemaField{
			// Users table
			{StructName: "User", Name: "id", Type: "INTEGER", Primary: true},
			{StructName: "User", Name: "email", Type: "VARCHAR", Nullable: false, Unique: true},
			{StructName: "User", Name: "status", Type: "user_status", Nullable: false, Default: "active"},
			// Posts table
			{StructName: "Post", Name: "id", Type: "INTEGER", Primary: true},
			{StructName: "Post", Name: "title", Type: "VARCHAR", Nullable: false},
		},
		Indexes: []types.SchemaIndex{
			{Name: "idx_user_email"},
			{Name: "idx_post_title"},
		},
		Enums: []types.GlobalEnum{
			{Name: "user_status", Values: []string{"active", "inactive", "suspended"}}, // Modified enum
			{Name: "post_status", Values: []string{"draft", "published"}},              // New enum
		},
		EmbeddedFields: []types.EmbeddedField{},
	}

	database := &parsertypes.DatabaseSchema{
		Tables: []parsertypes.Table{
			{
				Name: "users",
				Columns: []parsertypes.Column{
					{Name: "id", DataType: "INTEGER", IsPrimaryKey: true, IsNullable: "NO"},
					{Name: "email", DataType: "TEXT", IsNullable: "YES", IsUnique: false},                         // Type and constraints changed
					{Name: "status", DataType: "user_status", IsNullable: "NO", ColumnDefault: ptr.To("pending")}, // Default changed
					{Name: "phone", DataType: "VARCHAR", IsNullable: "YES"},                                       // Column to be removed
				},
			},
			{
				Name: "old_logs", // Table to be removed
				Columns: []parsertypes.Column{
					{Name: "id", DataType: "INTEGER", IsPrimaryKey: true, IsNullable: "NO"},
				},
			},
		},
		Enums: []parsertypes.Enum{
			{Name: "user_status", Values: []string{"active", "inactive", "deleted"}}, // "suspended" added, "deleted" removed
			{Name: "old_priority", Values: []string{"low", "high"}},                  // Enum to be removed
		},
		Indexes: []parsertypes.Index{
			{Name: "idx_user_email"},     // Exists in both
			{Name: "old_idx_user_phone"}, // Index to be removed
		},
	}

	diff := differ.CompareSchemas(generated, database)

	c.Assert(diff.HasChanges(), qt.IsTrue)

	// Check tables
	c.Assert(diff.TablesAdded, qt.DeepEquals, []string{"posts"})
	c.Assert(diff.TablesRemoved, qt.DeepEquals, []string{"old_logs"})
	c.Assert(diff.TablesModified, qt.HasLen, 1)

	// Check table modifications
	tableDiff := diff.TablesModified[0]
	c.Assert(tableDiff.TableName, qt.Equals, "users")
	c.Assert(tableDiff.ColumnsAdded, qt.HasLen, 0)
	c.Assert(tableDiff.ColumnsRemoved, qt.DeepEquals, []string{"phone"})
	c.Assert(tableDiff.ColumnsModified, qt.HasLen, 2)

	// Check column modifications
	emailDiff := findColumnDiff(tableDiff.ColumnsModified, "email")
	c.Assert(emailDiff, qt.IsNotNil)
	c.Assert(emailDiff.Changes["type"], qt.Equals, "text -> varchar")
	c.Assert(emailDiff.Changes["nullable"], qt.Equals, "true -> false")
	c.Assert(emailDiff.Changes["unique"], qt.Equals, "false -> true")

	statusDiff := findColumnDiff(tableDiff.ColumnsModified, "status")
	c.Assert(statusDiff, qt.IsNotNil)
	c.Assert(statusDiff.Changes["default"], qt.Equals, "'pending' -> 'active'")

	// Check enums
	c.Assert(diff.EnumsAdded, qt.DeepEquals, []string{"post_status"})
	c.Assert(diff.EnumsRemoved, qt.DeepEquals, []string{"old_priority"})
	c.Assert(diff.EnumsModified, qt.HasLen, 1)

	enumDiff := diff.EnumsModified[0]
	c.Assert(enumDiff.EnumName, qt.Equals, "user_status")
	c.Assert(enumDiff.ValuesAdded, qt.DeepEquals, []string{"suspended"})
	c.Assert(enumDiff.ValuesRemoved, qt.DeepEquals, []string{"deleted"})

	// Check indexes
	c.Assert(diff.IndexesAdded, qt.DeepEquals, []string{"idx_post_title"})
	c.Assert(diff.IndexesRemoved, qt.DeepEquals, []string{"old_idx_user_phone"})
}

// TestMapTypeToSQL_EnumHandling tests the fix for the ENUM(”) bug
// This test verifies that fields with empty enum values are not treated as enums
func TestMapTypeToSQL_EnumHandling(t *testing.T) {

	tests := []struct {
		name        string
		fieldType   string
		enumValues  []string
		dialect     string
		expected    string
		description string
	}{
		{
			name:        "boolean_with_nil_enum",
			fieldType:   "BOOLEAN",
			enumValues:  nil,
			dialect:     "mysql",
			expected:    "BOOLEAN",
			description: "BOOLEAN field with nil enum values should remain BOOLEAN",
		},
		{
			name:        "boolean_with_empty_enum_slice",
			fieldType:   "BOOLEAN",
			enumValues:  []string{},
			dialect:     "mysql",
			expected:    "BOOLEAN",
			description: "BOOLEAN field with empty enum slice should remain BOOLEAN",
		},
		{
			name:        "boolean_with_empty_string_in_enum",
			fieldType:   "BOOLEAN",
			enumValues:  []string{""},
			dialect:     "mysql",
			expected:    "BOOLEAN",
			description: "BOOLEAN field with empty string in enum slice should remain BOOLEAN (this was the bug)",
		},
		{
			name:        "varchar_with_empty_string_in_enum",
			fieldType:   "VARCHAR(100)",
			enumValues:  []string{""},
			dialect:     "mysql",
			expected:    "VARCHAR(100)",
			description: "VARCHAR field with empty string in enum slice should remain VARCHAR",
		},
		{
			name:        "varchar_with_multiple_empty_strings",
			fieldType:   "VARCHAR(255)",
			enumValues:  []string{"", "", ""},
			dialect:     "mysql",
			expected:    "VARCHAR(255)",
			description: "VARCHAR field with multiple empty strings should remain VARCHAR",
		},
		{
			name:        "valid_enum_with_values",
			fieldType:   "enum_status",
			enumValues:  []string{"active", "inactive"},
			dialect:     "mysql",
			expected:    "ENUM('active', 'inactive')",
			description: "Valid enum with actual values should work correctly",
		},
		{
			name:        "enum_with_mixed_empty_and_valid",
			fieldType:   "enum_status",
			enumValues:  []string{"", "active", "", "inactive", ""},
			dialect:     "mysql",
			expected:    "ENUM('active', 'inactive')",
			description: "Enum with mixed empty and valid values should filter out empty values",
		},
		{
			name:        "enum_type_prefix_with_empty_values",
			fieldType:   "enum_user_status",
			enumValues:  []string{""},
			dialect:     "mysql",
			expected:    "enum_user_status",
			description: "Type starting with enum_ but having only empty values should return type as-is",
		},
		{
			name:        "postgres_enum_handling",
			fieldType:   "enum_status",
			enumValues:  []string{"active", "inactive"},
			dialect:     "postgres",
			expected:    "enum_status",
			description: "PostgreSQL should return enum type name as-is",
		},
		{
			name:        "mariadb_enum_handling",
			fieldType:   "enum_status",
			enumValues:  []string{"active", "inactive"},
			dialect:     "mariadb",
			expected:    "ENUM('active', 'inactive')",
			description: "MariaDB should handle enums like MySQL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			// We need to test the MapTypeToSQL function through the public API
			// Create a simple schema diff and generate SQL to test the function indirectly
			generated := &parsertypes.PackageParseResult{
				Tables: []types.TableDirective{
					{StructName: "TestTable", Name: "test_table"},
				},
				Fields: []types.SchemaField{
					{
						StructName: "TestTable",
						Name:       "test_field",
						Type:       tt.fieldType,
						Enum:       tt.enumValues,
						Nullable:   false,
					},
				},
				Enums: []types.GlobalEnum{
					{Name: "enum_status", Values: []string{"active", "inactive"}},
				},
			}

			database := &parsertypes.DatabaseSchema{
				Tables: []parsertypes.Table{},
				Enums:  []parsertypes.Enum{},
			}

			diff := differ.CompareSchemas(generated, database)
			statements := differ.GenerateMigrationSQL(diff, generated, tt.dialect)

			// Find the CREATE TABLE statement
			var createTableSQL string
			for _, stmt := range statements {
				if strings.Contains(stmt, "CREATE TABLE test_table") {
					createTableSQL = stmt
					break
				}
			}

			c.Assert(createTableSQL, qt.Not(qt.Equals), "", qt.Commentf("CREATE TABLE statement not found"))
			c.Assert(createTableSQL, qt.Contains, tt.expected, qt.Commentf(tt.description))

			// Specifically check that we don't have the ENUM('') bug
			if tt.fieldType != "enum_status" && !strings.HasPrefix(tt.fieldType, "enum_") {
				c.Assert(createTableSQL, qt.Not(qt.Contains), "ENUM('')", qt.Commentf("Should not contain ENUM('')"))
			}
		})
	}
}
