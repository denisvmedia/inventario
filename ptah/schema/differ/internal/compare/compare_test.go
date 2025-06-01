package compare_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/ptr"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/schema/differ/differtypes"
	"github.com/denisvmedia/inventario/ptah/schema/differ/internal/compare"
)

func TestTablesAndColumns_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		generated *goschema.Database
		database  *dbschematypes.DBSchema
		expected  *differtypes.SchemaDiff
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
			database: &dbschematypes.DBSchema{
				Tables: []dbschematypes.DBTable{},
			},
			expected: &differtypes.SchemaDiff{
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
			database: &dbschematypes.DBSchema{
				Tables: []dbschematypes.DBTable{
					{Name: "old_table"},
				},
			},
			expected: &differtypes.SchemaDiff{
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
			database: &dbschematypes.DBSchema{
				Tables: []dbschematypes.DBTable{
					{
						Name: "users",
						Columns: []dbschematypes.DBColumn{
							{Name: "id", DataType: "integer", IsPrimaryKey: true},
						},
					},
				},
			},
			expected: &differtypes.SchemaDiff{
				TablesModified: []differtypes.TableDiff{
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
			database: &dbschematypes.DBSchema{
				Tables: []dbschematypes.DBTable{
					{
						Name: "users",
						Columns: []dbschematypes.DBColumn{
							{Name: "id", DataType: "integer", IsPrimaryKey: true},
							{Name: "legacy_field", DataType: "varchar"},
						},
					},
					{Name: "old_table"},
				},
			},
			expected: &differtypes.SchemaDiff{
				TablesAdded:   []string{"posts"},
				TablesRemoved: []string{"old_table"},
				TablesModified: []differtypes.TableDiff{
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

			diff := &differtypes.SchemaDiff{}
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
		database  *dbschematypes.DBSchema
		expected  *differtypes.SchemaDiff
	}{
		{
			name: "empty schemas",
			generated: &goschema.Database{
				Tables:         []goschema.Table{},
				Fields:         []goschema.Field{},
				EmbeddedFields: []goschema.EmbeddedField{},
			},
			database: &dbschematypes.DBSchema{
				Tables: []dbschematypes.DBTable{},
			},
			expected: &differtypes.SchemaDiff{},
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
			database: &dbschematypes.DBSchema{
				Tables: []dbschematypes.DBTable{},
			},
			expected: &differtypes.SchemaDiff{
				TablesAdded: []string{"users"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			diff := &differtypes.SchemaDiff{}
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
		dbTable   dbschematypes.DBTable
		generated *goschema.Database
		expected  differtypes.TableDiff
	}{
		{
			name:     "column added",
			genTable: goschema.Table{StructName: "User", Name: "users"},
			dbTable: dbschematypes.DBTable{
				Name: "users",
				Columns: []dbschematypes.DBColumn{
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
			expected: differtypes.TableDiff{
				TableName:    "users",
				ColumnsAdded: []string{"email"},
			},
		},
		{
			name:     "column removed",
			genTable: goschema.Table{StructName: "User", Name: "users"},
			dbTable: dbschematypes.DBTable{
				Name: "users",
				Columns: []dbschematypes.DBColumn{
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
			expected: differtypes.TableDiff{
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
	dbTable := dbschematypes.DBTable{
		Name: "users",
		Columns: []dbschematypes.DBColumn{
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

func TestTableColumns_UnhappyPath(t *testing.T) {
	tests := []struct {
		name      string
		genTable  goschema.Table
		dbTable   dbschematypes.DBTable
		generated *goschema.Database
		expected  differtypes.TableDiff
	}{
		{
			name:     "no fields for struct",
			genTable: goschema.Table{StructName: "User", Name: "users"},
			dbTable: dbschematypes.DBTable{
				Name:    "users",
				Columns: []dbschematypes.DBColumn{},
			},
			generated: &goschema.Database{
				Fields: []goschema.Field{
					{StructName: "Post", Name: "id", Type: "SERIAL", Primary: true}, // Different struct
				},
				EmbeddedFields: []goschema.EmbeddedField{},
			},
			expected: differtypes.TableDiff{
				TableName: "users",
			},
		},
		{
			name:     "empty database table",
			genTable: goschema.Table{StructName: "User", Name: "users"},
			dbTable: dbschematypes.DBTable{
				Name:    "users",
				Columns: []dbschematypes.DBColumn{},
			},
			generated: &goschema.Database{
				Fields: []goschema.Field{
					{StructName: "User", Name: "id", Type: "SERIAL", Primary: true},
				},
				EmbeddedFields: []goschema.EmbeddedField{},
			},
			expected: differtypes.TableDiff{
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
		dbCol    dbschematypes.DBColumn
		expected differtypes.ColumnDiff
	}{
		{
			name: "type change",
			genCol: goschema.Field{
				Name: "name",
				Type: "VARCHAR(255)",
			},
			dbCol: dbschematypes.DBColumn{
				Name:     "name",
				DataType: "TEXT",
			},
			expected: differtypes.ColumnDiff{
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
			dbCol: dbschematypes.DBColumn{
				Name:       "email",
				DataType:   "VARCHAR(255)",
				IsNullable: "YES",
			},
			expected: differtypes.ColumnDiff{
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
			dbCol: dbschematypes.DBColumn{
				Name:         "id",
				DataType:     "integer",
				IsPrimaryKey: false,
			},
			expected: differtypes.ColumnDiff{
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
			dbCol: dbschematypes.DBColumn{
				Name:     "email",
				DataType: "VARCHAR(255)",
				IsUnique: false,
			},
			expected: differtypes.ColumnDiff{
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
			dbCol: dbschematypes.DBColumn{
				Name:          "status",
				DataType:      "VARCHAR(50)",
				ColumnDefault: ptr.To("'inactive'"),
			},
			expected: differtypes.ColumnDiff{
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
			dbCol: dbschematypes.DBColumn{
				Name:       "name",
				DataType:   "VARCHAR(100)",
				IsNullable: "YES",
				IsUnique:   false,
			},
			expected: differtypes.ColumnDiff{
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
		dbCol    dbschematypes.DBColumn
		expected differtypes.ColumnDiff
	}{
		{
			name: "no changes",
			genCol: goschema.Field{
				Name:     "id",
				Type:     "SERIAL",
				Primary:  true,
				Nullable: false,
			},
			dbCol: dbschematypes.DBColumn{
				Name:         "id",
				DataType:     "integer",
				IsPrimaryKey: true,
				IsNullable:   "NO",
			},
			expected: differtypes.ColumnDiff{
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
			dbCol: dbschematypes.DBColumn{
				Name:            "id",
				DataType:        "integer",
				IsPrimaryKey:    true,
				IsAutoIncrement: true,
				ColumnDefault:   ptr.To("nextval('users_id_seq'::regclass)"),
			},
			expected: differtypes.ColumnDiff{
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
			dbCol: dbschematypes.DBColumn{
				Name:         "id",
				DataType:     "integer",
				IsPrimaryKey: true,
				IsNullable:   "NO",
			},
			expected: differtypes.ColumnDiff{
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
		database  *dbschematypes.DBSchema
		expected  *differtypes.SchemaDiff
	}{
		{
			name: "enum added",
			generated: &goschema.Database{
				Enums: []goschema.Enum{
					{Name: "status_enum", Values: []string{"active", "inactive"}},
				},
			},
			database: &dbschematypes.DBSchema{
				Enums: []dbschematypes.DBEnum{},
			},
			expected: &differtypes.SchemaDiff{
				EnumsAdded: []string{"status_enum"},
			},
		},
		{
			name: "enum removed",
			generated: &goschema.Database{
				Enums: []goschema.Enum{},
			},
			database: &dbschematypes.DBSchema{
				Enums: []dbschematypes.DBEnum{
					{Name: "old_enum", Values: []string{"value1", "value2"}},
				},
			},
			expected: &differtypes.SchemaDiff{
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
			database: &dbschematypes.DBSchema{
				Enums: []dbschematypes.DBEnum{
					{Name: "status_enum", Values: []string{"active", "inactive"}},
				},
			},
			expected: &differtypes.SchemaDiff{
				EnumsModified: []differtypes.EnumDiff{
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
			database: &dbschematypes.DBSchema{
				Enums: []dbschematypes.DBEnum{
					{Name: "status_enum", Values: []string{"active", "inactive", "deprecated"}},
					{Name: "old_enum", Values: []string{"value1"}},
				},
			},
			expected: &differtypes.SchemaDiff{
				EnumsAdded:   []string{"priority_enum"},
				EnumsRemoved: []string{"old_enum"},
				EnumsModified: []differtypes.EnumDiff{
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

			diff := &differtypes.SchemaDiff{}
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
		database  *dbschematypes.DBSchema
		expected  *differtypes.SchemaDiff
	}{
		{
			name: "empty schemas",
			generated: &goschema.Database{
				Enums: []goschema.Enum{},
			},
			database: &dbschematypes.DBSchema{
				Enums: []dbschematypes.DBEnum{},
			},
			expected: &differtypes.SchemaDiff{},
		},
		{
			name: "nil enums",
			generated: &goschema.Database{
				Enums: nil,
			},
			database: &dbschematypes.DBSchema{
				Enums: nil,
			},
			expected: &differtypes.SchemaDiff{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			diff := &differtypes.SchemaDiff{}
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
		dbEnum   dbschematypes.DBEnum
		expected differtypes.EnumDiff
	}{
		{
			name: "values added",
			genEnum: goschema.Enum{
				Name:   "status_enum",
				Values: []string{"active", "inactive", "pending", "archived"},
			},
			dbEnum: dbschematypes.DBEnum{
				Name:   "status_enum",
				Values: []string{"active", "inactive"},
			},
			expected: differtypes.EnumDiff{
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
			dbEnum: dbschematypes.DBEnum{
				Name:   "status_enum",
				Values: []string{"active", "inactive", "deprecated", "legacy"},
			},
			expected: differtypes.EnumDiff{
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
			dbEnum: dbschematypes.DBEnum{
				Name:   "priority_enum",
				Values: []string{"low", "medium", "urgent"},
			},
			expected: differtypes.EnumDiff{
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
		dbEnum   dbschematypes.DBEnum
		expected differtypes.EnumDiff
	}{
		{
			name: "no changes",
			genEnum: goschema.Enum{
				Name:   "status_enum",
				Values: []string{"active", "inactive"},
			},
			dbEnum: dbschematypes.DBEnum{
				Name:   "status_enum",
				Values: []string{"active", "inactive"},
			},
			expected: differtypes.EnumDiff{
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
			dbEnum: dbschematypes.DBEnum{
				Name:   "empty_enum",
				Values: []string{},
			},
			expected: differtypes.EnumDiff{
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
		database  *dbschematypes.DBSchema
		expected  *differtypes.SchemaDiff
	}{
		{
			name: "index added",
			generated: &goschema.Database{
				Indexes: []goschema.Index{
					{Name: "idx_user_email"},
				},
			},
			database: &dbschematypes.DBSchema{
				Indexes: []dbschematypes.DBIndex{},
			},
			expected: &differtypes.SchemaDiff{
				IndexesAdded: []string{"idx_user_email"},
			},
		},
		{
			name: "index removed",
			generated: &goschema.Database{
				Indexes: []goschema.Index{},
			},
			database: &dbschematypes.DBSchema{
				Indexes: []dbschematypes.DBIndex{
					{Name: "old_index", IsPrimary: false, IsUnique: false},
				},
			},
			expected: &differtypes.SchemaDiff{
				IndexesRemoved: []string{"old_index"},
			},
		},
		{
			name: "primary key index ignored",
			generated: &goschema.Database{
				Indexes: []goschema.Index{},
			},
			database: &dbschematypes.DBSchema{
				Indexes: []dbschematypes.DBIndex{
					{Name: "users_pkey", IsPrimary: true, IsUnique: false},
				},
			},
			expected: &differtypes.SchemaDiff{},
		},
		{
			name: "unique constraint index ignored",
			generated: &goschema.Database{
				Indexes: []goschema.Index{},
			},
			database: &dbschematypes.DBSchema{
				Indexes: []dbschematypes.DBIndex{
					{Name: "users_email_key", IsPrimary: false, IsUnique: true},
				},
			},
			expected: &differtypes.SchemaDiff{},
		},
		{
			name: "multiple index changes",
			generated: &goschema.Database{
				Indexes: []goschema.Index{
					{Name: "idx_user_email"},
					{Name: "idx_user_name"},
				},
			},
			database: &dbschematypes.DBSchema{
				Indexes: []dbschematypes.DBIndex{
					{Name: "idx_user_email", IsPrimary: false, IsUnique: false},
					{Name: "old_index", IsPrimary: false, IsUnique: false},
					{Name: "users_pkey", IsPrimary: true, IsUnique: false}, // Should be ignored
				},
			},
			expected: &differtypes.SchemaDiff{
				IndexesAdded:   []string{"idx_user_name"},
				IndexesRemoved: []string{"old_index"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			diff := &differtypes.SchemaDiff{}
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
		database  *dbschematypes.DBSchema
		expected  *differtypes.SchemaDiff
	}{
		{
			name: "empty schemas",
			generated: &goschema.Database{
				Indexes: []goschema.Index{},
			},
			database: &dbschematypes.DBSchema{
				Indexes: []dbschematypes.DBIndex{},
			},
			expected: &differtypes.SchemaDiff{},
		},
		{
			name: "nil indexes",
			generated: &goschema.Database{
				Indexes: nil,
			},
			database: &dbschematypes.DBSchema{
				Indexes: nil,
			},
			expected: &differtypes.SchemaDiff{},
		},
		{
			name: "only system indexes in database",
			generated: &goschema.Database{
				Indexes: []goschema.Index{},
			},
			database: &dbschematypes.DBSchema{
				Indexes: []dbschematypes.DBIndex{
					{Name: "users_pkey", IsPrimary: true, IsUnique: false},
					{Name: "users_email_key", IsPrimary: false, IsUnique: true},
				},
			},
			expected: &differtypes.SchemaDiff{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			diff := &differtypes.SchemaDiff{}
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
		dbCol    dbschematypes.DBColumn
		expected differtypes.ColumnDiff
	}{
		{
			name: "UDT name takes precedence over data type",
			genCol: goschema.Field{
				Name: "status",
				Type: "status_enum",
			},
			dbCol: dbschematypes.DBColumn{
				Name:     "status",
				DataType: "USER-DEFINED",
				UDTName:  "status_enum",
			},
			expected: differtypes.ColumnDiff{
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
			dbCol: dbschematypes.DBColumn{
				Name:            "id",
				DataType:        "integer",
				IsPrimaryKey:    true,
				IsAutoIncrement: false, // Not detected as auto increment
				ColumnDefault:   ptr.To("nextval('seq'::regclass)"),
			},
			expected: differtypes.ColumnDiff{
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
			dbCol: dbschematypes.DBColumn{
				Name:          "description",
				DataType:      "TEXT",
				ColumnDefault: nil, // NULL default
			},
			expected: differtypes.ColumnDiff{
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
	dbTable := dbschematypes.DBTable{
		Name: "users",
		Columns: []dbschematypes.DBColumn{
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

	database := &dbschematypes.DBSchema{
		Tables: []dbschematypes.DBTable{
			{Name: "zebra_old_table"},
			{Name: "alpha_old_table"},
		},
	}

	diff := &differtypes.SchemaDiff{}
	compare.TablesAndColumns(generated, database, diff)

	// Check that results are sorted alphabetically
	c.Assert(diff.TablesAdded, qt.DeepEquals, []string{"alpha_table", "zebra_table"})
	c.Assert(diff.TablesRemoved, qt.DeepEquals, []string{"alpha_old_table", "zebra_old_table"})
}

func TestColumnByName_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		diffs        []differtypes.ColumnDiff
		columnName   string
		expectedDiff *differtypes.ColumnDiff
	}{
		{
			name: "find existing column",
			diffs: []differtypes.ColumnDiff{
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
			expectedDiff: &differtypes.ColumnDiff{
				ColumnName: "email",
				Changes: map[string]string{
					"type":     "varchar -> text",
					"nullable": "true -> false",
				},
			},
		},
		{
			name: "find first column in slice",
			diffs: []differtypes.ColumnDiff{
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
			expectedDiff: &differtypes.ColumnDiff{
				ColumnName: "first_column",
				Changes: map[string]string{
					"type": "varchar -> text",
				},
			},
		},
		{
			name: "find last column in slice",
			diffs: []differtypes.ColumnDiff{
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
			expectedDiff: &differtypes.ColumnDiff{
				ColumnName: "last_column",
				Changes: map[string]string{
					"nullable": "true -> false",
				},
			},
		},
		{
			name: "find column with empty changes",
			diffs: []differtypes.ColumnDiff{
				{
					ColumnName: "unchanged_column",
					Changes:    map[string]string{},
				},
			},
			columnName: "unchanged_column",
			expectedDiff: &differtypes.ColumnDiff{
				ColumnName: "unchanged_column",
				Changes:    map[string]string{},
			},
		},
		{
			name: "find column with single change",
			diffs: []differtypes.ColumnDiff{
				{
					ColumnName: "status",
					Changes: map[string]string{
						"default": "'inactive' -> 'active'",
					},
				},
			},
			columnName: "status",
			expectedDiff: &differtypes.ColumnDiff{
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

			result := compare.ColumnByName(tt.diffs, tt.columnName)

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
		diffs      []differtypes.ColumnDiff
		columnName string
	}{
		{
			name: "column not found",
			diffs: []differtypes.ColumnDiff{
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
			diffs:      []differtypes.ColumnDiff{},
			columnName: "any_column",
		},
		{
			name:       "nil slice",
			diffs:      nil,
			columnName: "any_column",
		},
		{
			name: "empty column name search",
			diffs: []differtypes.ColumnDiff{
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
			diffs: []differtypes.ColumnDiff{
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

			result := compare.ColumnByName(tt.diffs, tt.columnName)

			c.Assert(result, qt.IsNil)
		})
	}
}

func TestColumnByName_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		diffs        []differtypes.ColumnDiff
		columnName   string
		expectedDiff *differtypes.ColumnDiff
	}{
		{
			name: "duplicate column names - returns first match",
			diffs: []differtypes.ColumnDiff{
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
			expectedDiff: &differtypes.ColumnDiff{
				ColumnName: "duplicate_name",
				Changes: map[string]string{
					"type": "varchar -> text",
				},
			},
		},
		{
			name: "column name with special characters",
			diffs: []differtypes.ColumnDiff{
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
			expectedDiff: &differtypes.ColumnDiff{
				ColumnName: "column-with-dash",
				Changes: map[string]string{
					"nullable": "true -> false",
				},
			},
		},
		{
			name: "column name with numbers",
			diffs: []differtypes.ColumnDiff{
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
			expectedDiff: &differtypes.ColumnDiff{
				ColumnName: "123column",
				Changes: map[string]string{
					"nullable": "true -> false",
				},
			},
		},
		{
			name: "single character column name",
			diffs: []differtypes.ColumnDiff{
				{
					ColumnName: "a",
					Changes: map[string]string{
						"type": "char -> varchar",
					},
				},
			},
			columnName: "a",
			expectedDiff: &differtypes.ColumnDiff{
				ColumnName: "a",
				Changes: map[string]string{
					"type": "char -> varchar",
				},
			},
		},
		{
			name: "very long column name",
			diffs: []differtypes.ColumnDiff{
				{
					ColumnName: "this_is_a_very_long_column_name_that_might_be_used_in_some_databases_with_descriptive_naming_conventions",
					Changes: map[string]string{
						"type": "text -> longtext",
					},
				},
			},
			columnName: "this_is_a_very_long_column_name_that_might_be_used_in_some_databases_with_descriptive_naming_conventions",
			expectedDiff: &differtypes.ColumnDiff{
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

			result := compare.ColumnByName(tt.diffs, tt.columnName)

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
	originalDiffs := []differtypes.ColumnDiff{
		{
			ColumnName: "test_column",
			Changes: map[string]string{
				"type": "varchar -> text",
			},
		},
	}

	result := compare.ColumnByName(originalDiffs, "test_column")

	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ColumnName, qt.Equals, "test_column")

	// Modify the returned pointer and verify it affects the original slice
	result.Changes["new_change"] = "old -> new"

	c.Assert(originalDiffs[0].Changes["new_change"], qt.Equals, "old -> new")
	c.Assert(len(originalDiffs[0].Changes), qt.Equals, 2)
}
