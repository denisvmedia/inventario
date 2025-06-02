package compare_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/ptr"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/dbschema/types"
	"github.com/denisvmedia/inventario/ptah/migration/schemadiff/internal/compare"
	difftypes "github.com/denisvmedia/inventario/ptah/migration/schemadiff/types"
)

func TestTableColumns_UnhappyPath(t *testing.T) {
	tests := []struct {
		name      string
		genTable  goschema.Table
		dbTable   types.DBTable
		generated *goschema.Database
		expected  difftypes.TableDiff
	}{
		{
			name:     "no fields for struct",
			genTable: goschema.Table{StructName: "User", Name: "users"},
			dbTable: types.DBTable{
				Name:    "users",
				Columns: []types.DBColumn{},
			},
			generated: &goschema.Database{
				Fields: []goschema.Field{
					{StructName: "Post", Name: "id", Type: "SERIAL", Primary: true}, // Different struct
				},
				EmbeddedFields: []goschema.EmbeddedField{},
			},
			expected: difftypes.TableDiff{
				TableName: "users",
			},
		},
		{
			name:     "empty database table",
			genTable: goschema.Table{StructName: "User", Name: "users"},
			dbTable: types.DBTable{
				Name:    "users",
				Columns: []types.DBColumn{},
			},
			generated: &goschema.Database{
				Fields: []goschema.Field{
					{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
				},
				EmbeddedFields: []goschema.EmbeddedField{},
			},
			expected: difftypes.TableDiff{
				TableName:    "users",
				ColumnsAdded: []string{"id"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := compare.TableColumns(tt.genTable, tt.dbTable, tt.generated)

			c.Assert(result.TableName, qt.Equals, tt.expected.TableName)
			c.Assert(result.ColumnsAdded, qt.DeepEquals, tt.expected.ColumnsAdded)
			c.Assert(result.ColumnsRemoved, qt.DeepEquals, tt.expected.ColumnsRemoved)
		})
	}
}

func TestColumns_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		genCol   goschema.Field
		dbCol    types.DBColumn
		expected difftypes.ColumnDiff
	}{
		{
			name: "type change",
			genCol: goschema.Field{
				Name: "name",
				Type: "VARCHAR(255)",
			},
			dbCol: types.DBColumn{
				Name:     "name",
				DataType: "TEXT",
			},
			expected: difftypes.ColumnDiff{
				ColumnName: "name",
				Changes: map[string]string{
					"type": "text -> varchar",
				},
			},
		},
		{
			name: "nullable change",
			genCol: goschema.Field{
				Name:     "email",
				Type:     "VARCHAR(255)",
				Nullable: false,
			},
			dbCol: types.DBColumn{
				Name:       "email",
				DataType:   "VARCHAR(255)",
				IsNullable: "YES",
			},
			expected: difftypes.ColumnDiff{
				ColumnName: "email",
				Changes: map[string]string{
					"nullable": "true -> false",
				},
			},
		},
		{
			name: "primary key change",
			genCol: goschema.Field{
				Name:    "id",
				Type:    "SERIAL",
				Primary: true,
			},
			dbCol: types.DBColumn{
				Name:         "id",
				DataType:     "integer",
				IsPrimaryKey: false,
			},
			expected: difftypes.ColumnDiff{
				ColumnName: "id",
				Changes: map[string]string{
					"primary_key": "false -> true",
				},
			},
		},
		{
			name: "unique constraint change",
			genCol: goschema.Field{
				Name:   "email",
				Type:   "VARCHAR(255)",
				Unique: true,
			},
			dbCol: types.DBColumn{
				Name:     "email",
				DataType: "VARCHAR(255)",
				IsUnique: false,
			},
			expected: difftypes.ColumnDiff{
				ColumnName: "email",
				Changes: map[string]string{
					"unique": "false -> true",
				},
			},
		},
		{
			name: "default value change",
			genCol: goschema.Field{
				Name:    "status",
				Type:    "VARCHAR(50)",
				Default: "'active'",
			},
			dbCol: types.DBColumn{
				Name:          "status",
				DataType:      "VARCHAR(50)",
				ColumnDefault: ptr.To("'inactive'"),
			},
			expected: difftypes.ColumnDiff{
				ColumnName: "status",
				Changes: map[string]string{
					"default": "'inactive' -> 'active'",
				},
			},
		},
		{
			name: "multiple changes",
			genCol: goschema.Field{
				Name:     "name",
				Type:     "TEXT",
				Nullable: false,
				Unique:   true,
			},
			dbCol: types.DBColumn{
				Name:       "name",
				DataType:   "VARCHAR(100)",
				IsNullable: "YES",
				IsUnique:   false,
			},
			expected: difftypes.ColumnDiff{
				ColumnName: "name",
				Changes: map[string]string{
					"type":     "varchar -> text",
					"nullable": "true -> false",
					"unique":   "false -> true",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := compare.Columns(tt.genCol, tt.dbCol)

			c.Assert(result.ColumnName, qt.Equals, tt.expected.ColumnName)
			c.Assert(len(result.Changes), qt.Equals, len(tt.expected.Changes))
			for key, expectedValue := range tt.expected.Changes {
				c.Assert(result.Changes[key], qt.Equals, expectedValue)
			}
		})
	}
}

func TestColumns_UnhappyPath(t *testing.T) {
	tests := []struct {
		name     string
		genCol   goschema.Field
		dbCol    types.DBColumn
		expected difftypes.ColumnDiff
	}{
		{
			name: "no changes",
			genCol: goschema.Field{
				Name:     "id",
				Type:     "SERIAL",
				Primary:  true,
				Nullable: false,
			},
			dbCol: types.DBColumn{
				Name:         "id",
				DataType:     "integer",
				IsPrimaryKey: true,
				IsNullable:   "NO",
			},
			expected: difftypes.ColumnDiff{
				ColumnName: "id",
				Changes:    map[string]string{},
			},
		},
		{
			name: "auto increment column ignores default",
			genCol: goschema.Field{
				Name:    "id",
				Type:    "SERIAL",
				Primary: true,
				Default: "",
			},
			dbCol: types.DBColumn{
				Name:            "id",
				DataType:        "integer",
				IsPrimaryKey:    true,
				IsAutoIncrement: true,
				ColumnDefault:   ptr.To("nextval('users_id_seq'::regclass)"),
			},
			expected: difftypes.ColumnDiff{
				ColumnName: "id",
				Changes:    map[string]string{},
			},
		},
		{
			name: "primary key forces not null",
			genCol: goschema.Field{
				Name:     "id",
				Type:     "SERIAL",
				Primary:  true,
				Nullable: true, // This should be ignored for primary keys
			},
			dbCol: types.DBColumn{
				Name:         "id",
				DataType:     "integer",
				IsPrimaryKey: true,
				IsNullable:   "NO",
			},
			expected: difftypes.ColumnDiff{
				ColumnName: "id",
				Changes:    map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := compare.Columns(tt.genCol, tt.dbCol)

			c.Assert(result.ColumnName, qt.Equals, tt.expected.ColumnName)
			c.Assert(len(result.Changes), qt.Equals, len(tt.expected.Changes))
			for key, expectedValue := range tt.expected.Changes {
				c.Assert(result.Changes[key], qt.Equals, expectedValue)
			}
		})
	}
}

func TestEnums_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		generated *goschema.Database
		database  *types.DBSchema
		expected  *difftypes.SchemaDiff
	}{
		{
			name: "enum added",
			generated: &goschema.Database{
				Enums: []goschema.Enum{
					{Name: "status_enum", Values: []string{"active", "inactive"}},
				},
			},
			database: &types.DBSchema{
				Enums: []types.DBEnum{},
			},
			expected: &difftypes.SchemaDiff{
				EnumsAdded: []string{"status_enum"},
			},
		},
		{
			name: "enum removed",
			generated: &goschema.Database{
				Enums: []goschema.Enum{},
			},
			database: &types.DBSchema{
				Enums: []types.DBEnum{
					{Name: "old_enum", Values: []string{"value1", "value2"}},
				},
			},
			expected: &difftypes.SchemaDiff{
				EnumsRemoved: []string{"old_enum"},
			},
		},
		{
			name: "enum modified",
			generated: &goschema.Database{
				Enums: []goschema.Enum{
					{Name: "status_enum", Values: []string{"active", "inactive", "pending"}},
				},
			},
			database: &types.DBSchema{
				Enums: []types.DBEnum{
					{Name: "status_enum", Values: []string{"active", "inactive"}},
				},
			},
			expected: &difftypes.SchemaDiff{
				EnumsModified: []difftypes.EnumDiff{
					{
						EnumName:      "status_enum",
						ValuesAdded:   []string{"pending"},
						ValuesRemoved: nil,
					},
				},
			},
		},
		{
			name: "multiple enum changes",
			generated: &goschema.Database{
				Enums: []goschema.Enum{
					{Name: "status_enum", Values: []string{"active", "inactive"}},
					{Name: "priority_enum", Values: []string{"low", "medium", "high"}},
				},
			},
			database: &types.DBSchema{
				Enums: []types.DBEnum{
					{Name: "status_enum", Values: []string{"active", "inactive", "deprecated"}},
					{Name: "old_enum", Values: []string{"value1"}},
				},
			},
			expected: &difftypes.SchemaDiff{
				EnumsAdded:   []string{"priority_enum"},
				EnumsRemoved: []string{"old_enum"},
				EnumsModified: []difftypes.EnumDiff{
					{
						EnumName:      "status_enum",
						ValuesAdded:   nil,
						ValuesRemoved: []string{"deprecated"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			diff := &difftypes.SchemaDiff{}
			compare.Enums(tt.generated, tt.database, diff)

			c.Assert(diff.EnumsAdded, qt.DeepEquals, tt.expected.EnumsAdded)
			c.Assert(diff.EnumsRemoved, qt.DeepEquals, tt.expected.EnumsRemoved)
			c.Assert(len(diff.EnumsModified), qt.Equals, len(tt.expected.EnumsModified))

			for i, expectedEnumDiff := range tt.expected.EnumsModified {
				c.Assert(diff.EnumsModified[i].EnumName, qt.Equals, expectedEnumDiff.EnumName)
				c.Assert(diff.EnumsModified[i].ValuesAdded, qt.DeepEquals, expectedEnumDiff.ValuesAdded)
				c.Assert(diff.EnumsModified[i].ValuesRemoved, qt.DeepEquals, expectedEnumDiff.ValuesRemoved)
			}
		})
	}
}

func TestEnums_UnhappyPath(t *testing.T) {
	tests := []struct {
		name      string
		generated *goschema.Database
		database  *types.DBSchema
		expected  *difftypes.SchemaDiff
	}{
		{
			name: "empty schemas",
			generated: &goschema.Database{
				Enums: []goschema.Enum{},
			},
			database: &types.DBSchema{
				Enums: []types.DBEnum{},
			},
			expected: &difftypes.SchemaDiff{},
		},
		{
			name: "nil enums",
			generated: &goschema.Database{
				Enums: nil,
			},
			database: &types.DBSchema{
				Enums: nil,
			},
			expected: &difftypes.SchemaDiff{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			diff := &difftypes.SchemaDiff{}
			compare.Enums(tt.generated, tt.database, diff)

			c.Assert(diff.EnumsAdded, qt.DeepEquals, tt.expected.EnumsAdded)
			c.Assert(diff.EnumsRemoved, qt.DeepEquals, tt.expected.EnumsRemoved)
			c.Assert(len(diff.EnumsModified), qt.Equals, len(tt.expected.EnumsModified))
		})
	}
}

func TestEnumValues_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		genEnum  goschema.Enum
		dbEnum   types.DBEnum
		expected difftypes.EnumDiff
	}{
		{
			name: "values added",
			genEnum: goschema.Enum{
				Name:   "status_enum",
				Values: []string{"active", "inactive", "pending", "archived"},
			},
			dbEnum: types.DBEnum{
				Name:   "status_enum",
				Values: []string{"active", "inactive"},
			},
			expected: difftypes.EnumDiff{
				EnumName:      "status_enum",
				ValuesAdded:   []string{"archived", "pending"},
				ValuesRemoved: nil,
			},
		},
		{
			name: "values removed",
			genEnum: goschema.Enum{
				Name:   "status_enum",
				Values: []string{"active", "inactive"},
			},
			dbEnum: types.DBEnum{
				Name:   "status_enum",
				Values: []string{"active", "inactive", "deprecated", "legacy"},
			},
			expected: difftypes.EnumDiff{
				EnumName:      "status_enum",
				ValuesAdded:   nil,
				ValuesRemoved: []string{"deprecated", "legacy"},
			},
		},
		{
			name: "mixed changes",
			genEnum: goschema.Enum{
				Name:   "priority_enum",
				Values: []string{"low", "medium", "high", "critical"},
			},
			dbEnum: types.DBEnum{
				Name:   "priority_enum",
				Values: []string{"low", "medium", "urgent"},
			},
			expected: difftypes.EnumDiff{
				EnumName:      "priority_enum",
				ValuesAdded:   []string{"critical", "high"},
				ValuesRemoved: []string{"urgent"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := compare.EnumValues(tt.genEnum, tt.dbEnum)

			c.Assert(result.EnumName, qt.Equals, tt.expected.EnumName)
			c.Assert(result.ValuesAdded, qt.DeepEquals, tt.expected.ValuesAdded)
			c.Assert(result.ValuesRemoved, qt.DeepEquals, tt.expected.ValuesRemoved)
		})
	}
}

func TestEnumValues_UnhappyPath(t *testing.T) {
	tests := []struct {
		name     string
		genEnum  goschema.Enum
		dbEnum   types.DBEnum
		expected difftypes.EnumDiff
	}{
		{
			name: "no changes",
			genEnum: goschema.Enum{
				Name:   "status_enum",
				Values: []string{"active", "inactive"},
			},
			dbEnum: types.DBEnum{
				Name:   "status_enum",
				Values: []string{"active", "inactive"},
			},
			expected: difftypes.EnumDiff{
				EnumName:      "status_enum",
				ValuesAdded:   nil,
				ValuesRemoved: nil,
			},
		},
		{
			name: "empty enum values",
			genEnum: goschema.Enum{
				Name:   "empty_enum",
				Values: []string{},
			},
			dbEnum: types.DBEnum{
				Name:   "empty_enum",
				Values: []string{},
			},
			expected: difftypes.EnumDiff{
				EnumName:      "empty_enum",
				ValuesAdded:   nil,
				ValuesRemoved: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := compare.EnumValues(tt.genEnum, tt.dbEnum)

			c.Assert(result.EnumName, qt.Equals, tt.expected.EnumName)
			c.Assert(result.ValuesAdded, qt.DeepEquals, tt.expected.ValuesAdded)
			c.Assert(result.ValuesRemoved, qt.DeepEquals, tt.expected.ValuesRemoved)
		})
	}
}

func TestIndexes_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		generated *goschema.Database
		database  *types.DBSchema
		expected  *difftypes.SchemaDiff
	}{
		{
			name: "index added",
			generated: &goschema.Database{
				Indexes: []goschema.Index{
					{Name: "idx_user_email"},
				},
			},
			database: &types.DBSchema{
				Indexes: []types.DBIndex{},
			},
			expected: &difftypes.SchemaDiff{
				IndexesAdded: []string{"idx_user_email"},
			},
		},
		{
			name: "index removed",
			generated: &goschema.Database{
				Indexes: []goschema.Index{},
			},
			database: &types.DBSchema{
				Indexes: []types.DBIndex{
					{Name: "old_index", IsPrimary: false, IsUnique: false},
				},
			},
			expected: &difftypes.SchemaDiff{
				IndexesRemoved: []string{"old_index"},
			},
		},
		{
			name: "primary key index ignored",
			generated: &goschema.Database{
				Indexes: []goschema.Index{},
			},
			database: &types.DBSchema{
				Indexes: []types.DBIndex{
					{Name: "users_pkey", IsPrimary: true, IsUnique: false},
				},
			},
			expected: &difftypes.SchemaDiff{},
		},
		{
			name: "unique constraint index ignored",
			generated: &goschema.Database{
				Indexes: []goschema.Index{},
			},
			database: &types.DBSchema{
				Indexes: []types.DBIndex{
					{Name: "users_email_key", IsPrimary: false, IsUnique: true},
				},
			},
			expected: &difftypes.SchemaDiff{},
		},
		{
			name: "multiple index changes",
			generated: &goschema.Database{
				Indexes: []goschema.Index{
					{Name: "idx_user_email"},
					{Name: "idx_user_name"},
				},
			},
			database: &types.DBSchema{
				Indexes: []types.DBIndex{
					{Name: "idx_user_email", IsPrimary: false, IsUnique: false},
					{Name: "old_index", IsPrimary: false, IsUnique: false},
					{Name: "users_pkey", IsPrimary: true, IsUnique: false}, // Should be ignored
				},
			},
			expected: &difftypes.SchemaDiff{
				IndexesAdded:   []string{"idx_user_name"},
				IndexesRemoved: []string{"old_index"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			diff := &difftypes.SchemaDiff{}
			compare.Indexes(tt.generated, tt.database, diff)

			c.Assert(diff.IndexesAdded, qt.DeepEquals, tt.expected.IndexesAdded)
			c.Assert(diff.IndexesRemoved, qt.DeepEquals, tt.expected.IndexesRemoved)
		})
	}
}

func TestIndexes_UnhappyPath(t *testing.T) {
	tests := []struct {
		name      string
		generated *goschema.Database
		database  *types.DBSchema
		expected  *difftypes.SchemaDiff
	}{
		{
			name: "empty schemas",
			generated: &goschema.Database{
				Indexes: []goschema.Index{},
			},
			database: &types.DBSchema{
				Indexes: []types.DBIndex{},
			},
			expected: &difftypes.SchemaDiff{},
		},
		{
			name: "nil indexes",
			generated: &goschema.Database{
				Indexes: nil,
			},
			database: &types.DBSchema{
				Indexes: nil,
			},
			expected: &difftypes.SchemaDiff{},
		},
		{
			name: "only system indexes in database",
			generated: &goschema.Database{
				Indexes: []goschema.Index{},
			},
			database: &types.DBSchema{
				Indexes: []types.DBIndex{
					{Name: "users_pkey", IsPrimary: true, IsUnique: false},
					{Name: "users_email_key", IsPrimary: false, IsUnique: true},
				},
			},
			expected: &difftypes.SchemaDiff{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			diff := &difftypes.SchemaDiff{}
			compare.Indexes(tt.generated, tt.database, diff)

			c.Assert(diff.IndexesAdded, qt.DeepEquals, tt.expected.IndexesAdded)
			c.Assert(diff.IndexesRemoved, qt.DeepEquals, tt.expected.IndexesRemoved)
		})
	}
}

func TestColumns_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		genCol   goschema.Field
		dbCol    types.DBColumn
		expected difftypes.ColumnDiff
	}{
		{
			name: "UDT name takes precedence over data type",
			genCol: goschema.Field{
				Name: "status",
				Type: "status_enum",
			},
			dbCol: types.DBColumn{
				Name:     "status",
				DataType: "USER-DEFINED",
				UDTName:  "status_enum",
			},
			expected: difftypes.ColumnDiff{
				ColumnName: "status",
				Changes:    map[string]string{},
			},
		},
		{
			name: "SERIAL type detection for auto increment",
			genCol: goschema.Field{
				Name:    "id",
				Type:    "SERIAL",
				Primary: true,
				Default: "",
			},
			dbCol: types.DBColumn{
				Name:            "id",
				DataType:        "integer",
				IsPrimaryKey:    true,
				IsAutoIncrement: false, // Not detected as auto increment
				ColumnDefault:   ptr.To("nextval('seq'::regclass)"),
			},
			expected: difftypes.ColumnDiff{
				ColumnName: "id",
				Changes:    map[string]string{}, // Should ignore default due to SERIAL type
			},
		},
		{
			name: "null column default vs empty string",
			genCol: goschema.Field{
				Name:    "description",
				Type:    "TEXT",
				Default: "",
			},
			dbCol: types.DBColumn{
				Name:          "description",
				DataType:      "TEXT",
				ColumnDefault: nil, // NULL default
			},
			expected: difftypes.ColumnDiff{
				ColumnName: "description",
				Changes:    map[string]string{}, // Both should normalize to empty
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := compare.Columns(tt.genCol, tt.dbCol)

			c.Assert(result.ColumnName, qt.Equals, tt.expected.ColumnName)
			c.Assert(len(result.Changes), qt.Equals, len(tt.expected.Changes))
			for key, expectedValue := range tt.expected.Changes {
				c.Assert(result.Changes[key], qt.Equals, expectedValue)
			}
		})
	}
}

func TestTableColumns_EdgeCases(t *testing.T) {
	c := qt.New(t)

	// Test with column modifications
	genTable := goschema.Table{StructName: "User", Name: "users"}
	dbTable := types.DBTable{
		Name: "users",
		Columns: []types.DBColumn{
			{Name: "id", DataType: "integer", IsPrimaryKey: true},
			{Name: "name", DataType: "VARCHAR(100)", IsNullable: "YES"},
		},
	}

	generated := &goschema.Database{
		Fields: []goschema.Field{
			{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
			{StructName: "User", Name: "name", Type: "VARCHAR(255)", Nullable: false}, // Type and nullable change
		},
		EmbeddedFields: []goschema.EmbeddedField{},
	}

	result := compare.TableColumns(genTable, dbTable, generated)

	c.Assert(result.TableName, qt.Equals, "users")
	c.Assert(len(result.ColumnsModified), qt.Equals, 1)
	c.Assert(result.ColumnsModified[0].ColumnName, qt.Equals, "name")
	c.Assert(len(result.ColumnsModified[0].Changes), qt.Equals, 1) // Only nullable should change (types are both varchar)
	c.Assert(result.ColumnsModified[0].Changes["nullable"], qt.Equals, "true -> false")
}

func TestTablesAndColumns_SortingConsistency(t *testing.T) {
	c := qt.New(t)

	generated := &goschema.Database{
		Tables: []goschema.Table{
			{StructName: "User", Name: "zebra_table"},
			{StructName: "Post", Name: "alpha_table"},
		},
		Fields:         []goschema.Field{},
		EmbeddedFields: []goschema.EmbeddedField{},
	}

	database := &types.DBSchema{
		Tables: []types.DBTable{
			{Name: "zebra_old_table"},
			{Name: "alpha_old_table"},
		},
	}

	diff := &difftypes.SchemaDiff{}
	compare.TablesAndColumns(generated, database, diff)

	// Check that results are sorted alphabetically
	c.Assert(diff.TablesAdded, qt.DeepEquals, []string{"alpha_table", "zebra_table"})
	c.Assert(diff.TablesRemoved, qt.DeepEquals, []string{"alpha_old_table", "zebra_old_table"})
}

func TestColumnByName_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		diffs        []difftypes.ColumnDiff
		columnName   string
		expectedDiff *difftypes.ColumnDiff
	}{
		{
			name: "find existing column",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "id",
					Changes: map[string]string{
						"type": "integer -> bigint",
					},
				},
				{
					ColumnName: "email",
					Changes: map[string]string{
						"type":     "varchar -> text",
						"nullable": "true -> false",
					},
				},
				{
					ColumnName: "name",
					Changes: map[string]string{
						"unique": "false -> true",
					},
				},
			},
			columnName: "email",
			expectedDiff: &difftypes.ColumnDiff{
				ColumnName: "email",
				Changes: map[string]string{
					"type":     "varchar -> text",
					"nullable": "true -> false",
				},
			},
		},
		{
			name: "find first column in slice",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "first_column",
					Changes: map[string]string{
						"type": "varchar -> text",
					},
				},
				{
					ColumnName: "second_column",
					Changes: map[string]string{
						"nullable": "true -> false",
					},
				},
			},
			columnName: "first_column",
			expectedDiff: &difftypes.ColumnDiff{
				ColumnName: "first_column",
				Changes: map[string]string{
					"type": "varchar -> text",
				},
			},
		},
		{
			name: "find last column in slice",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "first_column",
					Changes: map[string]string{
						"type": "varchar -> text",
					},
				},
				{
					ColumnName: "last_column",
					Changes: map[string]string{
						"nullable": "true -> false",
					},
				},
			},
			columnName: "last_column",
			expectedDiff: &difftypes.ColumnDiff{
				ColumnName: "last_column",
				Changes: map[string]string{
					"nullable": "true -> false",
				},
			},
		},
		{
			name: "find column with empty changes",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "unchanged_column",
					Changes:    map[string]string{},
				},
			},
			columnName: "unchanged_column",
			expectedDiff: &difftypes.ColumnDiff{
				ColumnName: "unchanged_column",
				Changes:    map[string]string{},
			},
		},
		{
			name: "find column with single change",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "status",
					Changes: map[string]string{
						"default": "'inactive' -> 'active'",
					},
				},
			},
			columnName: "status",
			expectedDiff: &difftypes.ColumnDiff{
				ColumnName: "status",
				Changes: map[string]string{
					"default": "'inactive' -> 'active'",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := compare.SearchColumnByName(tt.diffs, tt.columnName)

			c.Assert(result, qt.IsNotNil)
			c.Assert(result.ColumnName, qt.Equals, tt.expectedDiff.ColumnName)
			c.Assert(len(result.Changes), qt.Equals, len(tt.expectedDiff.Changes))
			for key, expectedValue := range tt.expectedDiff.Changes {
				c.Assert(result.Changes[key], qt.Equals, expectedValue)
			}
		})
	}
}

func TestColumnByName_UnhappyPath(t *testing.T) {
	tests := []struct {
		name       string
		diffs      []difftypes.ColumnDiff
		columnName string
	}{
		{
			name: "column not found",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "id",
					Changes: map[string]string{
						"type": "integer -> bigint",
					},
				},
				{
					ColumnName: "email",
					Changes: map[string]string{
						"nullable": "true -> false",
					},
				},
			},
			columnName: "nonexistent_column",
		},
		{
			name:       "empty slice",
			diffs:      []difftypes.ColumnDiff{},
			columnName: "any_column",
		},
		{
			name:       "nil slice",
			diffs:      nil,
			columnName: "any_column",
		},
		{
			name: "empty column name search",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "id",
					Changes: map[string]string{
						"type": "integer -> bigint",
					},
				},
			},
			columnName: "",
		},
		{
			name: "case sensitive search - wrong case",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "Email",
					Changes: map[string]string{
						"type": "varchar -> text",
					},
				},
			},
			columnName: "email", // lowercase, should not match "Email"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := compare.SearchColumnByName(tt.diffs, tt.columnName)

			c.Assert(result, qt.IsNil)
		})
	}
}

func TestColumnByName_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		diffs        []difftypes.ColumnDiff
		columnName   string
		expectedDiff *difftypes.ColumnDiff
	}{
		{
			name: "duplicate column names - returns first match",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "duplicate_name",
					Changes: map[string]string{
						"type": "varchar -> text",
					},
				},
				{
					ColumnName: "duplicate_name",
					Changes: map[string]string{
						"nullable": "true -> false",
					},
				},
			},
			columnName: "duplicate_name",
			expectedDiff: &difftypes.ColumnDiff{
				ColumnName: "duplicate_name",
				Changes: map[string]string{
					"type": "varchar -> text",
				},
			},
		},
		{
			name: "column name with special characters",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "column_with_underscore",
					Changes: map[string]string{
						"type": "varchar -> text",
					},
				},
				{
					ColumnName: "column-with-dash",
					Changes: map[string]string{
						"nullable": "true -> false",
					},
				},
				{
					ColumnName: "column.with.dots",
					Changes: map[string]string{
						"unique": "false -> true",
					},
				},
			},
			columnName: "column-with-dash",
			expectedDiff: &difftypes.ColumnDiff{
				ColumnName: "column-with-dash",
				Changes: map[string]string{
					"nullable": "true -> false",
				},
			},
		},
		{
			name: "column name with numbers",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "column123",
					Changes: map[string]string{
						"type": "integer -> bigint",
					},
				},
				{
					ColumnName: "123column",
					Changes: map[string]string{
						"nullable": "true -> false",
					},
				},
			},
			columnName: "123column",
			expectedDiff: &difftypes.ColumnDiff{
				ColumnName: "123column",
				Changes: map[string]string{
					"nullable": "true -> false",
				},
			},
		},
		{
			name: "single character column name",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "a",
					Changes: map[string]string{
						"type": "char -> varchar",
					},
				},
			},
			columnName: "a",
			expectedDiff: &difftypes.ColumnDiff{
				ColumnName: "a",
				Changes: map[string]string{
					"type": "char -> varchar",
				},
			},
		},
		{
			name: "very long column name",
			diffs: []difftypes.ColumnDiff{
				{
					ColumnName: "this_is_a_very_long_column_name_that_might_be_used_in_some_databases_with_descriptive_naming_conventions",
					Changes: map[string]string{
						"type": "text -> longtext",
					},
				},
			},
			columnName: "this_is_a_very_long_column_name_that_might_be_used_in_some_databases_with_descriptive_naming_conventions",
			expectedDiff: &difftypes.ColumnDiff{
				ColumnName: "this_is_a_very_long_column_name_that_might_be_used_in_some_databases_with_descriptive_naming_conventions",
				Changes: map[string]string{
					"type": "text -> longtext",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := compare.SearchColumnByName(tt.diffs, tt.columnName)

			if tt.expectedDiff == nil {
				c.Assert(result, qt.IsNil)
			} else {
				c.Assert(result, qt.IsNotNil)
				c.Assert(result.ColumnName, qt.Equals, tt.expectedDiff.ColumnName)
				c.Assert(len(result.Changes), qt.Equals, len(tt.expectedDiff.Changes))
				for key, expectedValue := range tt.expectedDiff.Changes {
					c.Assert(result.Changes[key], qt.Equals, expectedValue)
				}
			}
		})
	}
}

func TestColumnByName_PointerBehavior(t *testing.T) {
	c := qt.New(t)

	// Test that the returned pointer references the original data
	originalDiffs := []difftypes.ColumnDiff{
		{
			ColumnName: "test_column",
			Changes: map[string]string{
				"type": "varchar -> text",
			},
		},
	}

	result := compare.SearchColumnByName(originalDiffs, "test_column")

	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ColumnName, qt.Equals, "test_column")

	// Modify the returned pointer and verify it affects the original slice
	result.Changes["new_change"] = "old -> new"

	c.Assert(originalDiffs[0].Changes["new_change"], qt.Equals, "old -> new")
	c.Assert(len(originalDiffs[0].Changes), qt.Equals, 2)
}

func TestTablesAndColumns_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		generated *goschema.Database
		database  *types.DBSchema
		expected  *difftypes.SchemaDiff
	}{
		{
			name: "new table added",
			generated: &goschema.Database{
				Tables: []goschema.Table{
					{StructName: "User", Name: "users"},
				},
				Fields: []goschema.Field{
					{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
				},
				EmbeddedFields: []goschema.EmbeddedField{},
			},
			database: &types.DBSchema{
				Tables: []types.DBTable{},
			},
			expected: &difftypes.SchemaDiff{
				TablesAdded: []string{"users"},
			},
		},
		{
			name: "table removed",
			generated: &goschema.Database{
				Tables:         []goschema.Table{},
				Fields:         []goschema.Field{},
				EmbeddedFields: []goschema.EmbeddedField{},
			},
			database: &types.DBSchema{
				Tables: []types.DBTable{
					{Name: "old_table"},
				},
			},
			expected: &difftypes.SchemaDiff{
				TablesRemoved: []string{"old_table"},
			},
		},
		{
			name: "table modified - column added",
			generated: &goschema.Database{
				Tables: []goschema.Table{
					{StructName: "User", Name: "users"},
				},
				Fields: []goschema.Field{
					{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
					{StructName: "User", Name: "email", Type: "VARCHAR(255)", Nullable: false},
				},
				EmbeddedFields: []goschema.EmbeddedField{},
			},
			database: &types.DBSchema{
				Tables: []types.DBTable{
					{
						Name: "users",
						Columns: []types.DBColumn{
							{Name: "id", DataType: "integer", IsPrimaryKey: true},
						},
					},
				},
			},
			expected: &difftypes.SchemaDiff{
				TablesModified: []difftypes.TableDiff{
					{
						TableName:    "users",
						ColumnsAdded: []string{"email"},
					},
				},
			},
		},
		{
			name: "multiple changes",
			generated: &goschema.Database{
				Tables: []goschema.Table{
					{StructName: "User", Name: "users"},
					{StructName: "Post", Name: "posts"},
				},
				Fields: []goschema.Field{
					{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
					{StructName: "Post", Name: "id", Type: "SERIAL", Primary: true},
				},
				EmbeddedFields: []goschema.EmbeddedField{},
			},
			database: &types.DBSchema{
				Tables: []types.DBTable{
					{
						Name: "users",
						Columns: []types.DBColumn{
							{Name: "id", DataType: "integer", IsPrimaryKey: true},
							{Name: "legacy_field", DataType: "varchar"},
						},
					},
					{Name: "old_table"},
				},
			},
			expected: &difftypes.SchemaDiff{
				TablesAdded:   []string{"posts"},
				TablesRemoved: []string{"old_table"},
				TablesModified: []difftypes.TableDiff{
					{
						TableName:      "users",
						ColumnsRemoved: []string{"legacy_field"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			diff := &difftypes.SchemaDiff{}
			compare.TablesAndColumns(tt.generated, tt.database, diff)

			c.Assert(diff.TablesAdded, qt.DeepEquals, tt.expected.TablesAdded)
			c.Assert(diff.TablesRemoved, qt.DeepEquals, tt.expected.TablesRemoved)
			c.Assert(len(diff.TablesModified), qt.Equals, len(tt.expected.TablesModified))

			for i, expectedTableDiff := range tt.expected.TablesModified {
				c.Assert(diff.TablesModified[i].TableName, qt.Equals, expectedTableDiff.TableName)
				c.Assert(diff.TablesModified[i].ColumnsAdded, qt.DeepEquals, expectedTableDiff.ColumnsAdded)
				c.Assert(diff.TablesModified[i].ColumnsRemoved, qt.DeepEquals, expectedTableDiff.ColumnsRemoved)
			}
		})
	}
}

func TestTablesAndColumns_UnhappyPath(t *testing.T) {
	tests := []struct {
		name      string
		generated *goschema.Database
		database  *types.DBSchema
		expected  *difftypes.SchemaDiff
	}{
		{
			name: "empty schemas",
			generated: &goschema.Database{
				Tables:         []goschema.Table{},
				Fields:         []goschema.Field{},
				EmbeddedFields: []goschema.EmbeddedField{},
			},
			database: &types.DBSchema{
				Tables: []types.DBTable{},
			},
			expected: &difftypes.SchemaDiff{},
		},
		{
			name: "nil embedded fields",
			generated: &goschema.Database{
				Tables: []goschema.Table{
					{StructName: "User", Name: "users"},
				},
				Fields: []goschema.Field{
					{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
				},
				EmbeddedFields: nil,
			},
			database: &types.DBSchema{
				Tables: []types.DBTable{},
			},
			expected: &difftypes.SchemaDiff{
				TablesAdded: []string{"users"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			diff := &difftypes.SchemaDiff{}
			compare.TablesAndColumns(tt.generated, tt.database, diff)

			c.Assert(diff.TablesAdded, qt.DeepEquals, tt.expected.TablesAdded)
			c.Assert(diff.TablesRemoved, qt.DeepEquals, tt.expected.TablesRemoved)
			c.Assert(len(diff.TablesModified), qt.Equals, len(tt.expected.TablesModified))
		})
	}
}

func TestTableColumns_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		genTable  goschema.Table
		dbTable   types.DBTable
		generated *goschema.Database
		expected  difftypes.TableDiff
	}{
		{
			name:     "column added",
			genTable: goschema.Table{StructName: "User", Name: "users"},
			dbTable: types.DBTable{
				Name: "users",
				Columns: []types.DBColumn{
					{Name: "id", DataType: "integer", IsPrimaryKey: true},
				},
			},
			generated: &goschema.Database{
				Fields: []goschema.Field{
					{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
					{StructName: "User", Name: "email", Type: "VARCHAR(255)", Nullable: false},
				},
				EmbeddedFields: []goschema.EmbeddedField{},
			},
			expected: difftypes.TableDiff{
				TableName:    "users",
				ColumnsAdded: []string{"email"},
			},
		},
		{
			name:     "column removed",
			genTable: goschema.Table{StructName: "User", Name: "users"},
			dbTable: types.DBTable{
				Name: "users",
				Columns: []types.DBColumn{
					{Name: "id", DataType: "integer", IsPrimaryKey: true},
					{Name: "legacy_field", DataType: "varchar"},
				},
			},
			generated: &goschema.Database{
				Fields: []goschema.Field{
					{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
				},
				EmbeddedFields: []goschema.EmbeddedField{},
			},
			expected: difftypes.TableDiff{
				TableName:      "users",
				ColumnsRemoved: []string{"legacy_field"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := compare.TableColumns(tt.genTable, tt.dbTable, tt.generated)

			c.Assert(result.TableName, qt.Equals, tt.expected.TableName)
			c.Assert(result.ColumnsAdded, qt.DeepEquals, tt.expected.ColumnsAdded)
			c.Assert(result.ColumnsRemoved, qt.DeepEquals, tt.expected.ColumnsRemoved)
		})
	}
}

func TestTableColumns_WithEmbeddedFields(t *testing.T) {
	c := qt.New(t)

	genTable := goschema.Table{StructName: "User", Name: "users"}
	dbTable := types.DBTable{
		Name: "users",
		Columns: []types.DBColumn{
			{Name: "id", DataType: "integer", IsPrimaryKey: true},
		},
	}

	generated := &goschema.Database{
		Fields: []goschema.Field{
			{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
			{StructName: "Timestamps", Name: "created_at", Type: "TIMESTAMP", Nullable: false},
			{StructName: "Timestamps", Name: "updated_at", Type: "TIMESTAMP", Nullable: false},
		},
		EmbeddedFields: []goschema.EmbeddedField{
			{
				StructName:       "User",
				Mode:             "inline",
				EmbeddedTypeName: "Timestamps",
			},
		},
	}

	result := compare.TableColumns(genTable, dbTable, generated)

	c.Assert(result.TableName, qt.Equals, "users")
	c.Assert(result.ColumnsAdded, qt.DeepEquals, []string{"created_at", "updated_at"})
}
