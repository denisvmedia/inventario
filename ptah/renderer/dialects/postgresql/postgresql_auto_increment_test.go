package postgresql

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
)

func TestAutoIncrementConversion(t *testing.T) {
	c := qt.New(t)
	generator := New()

	tests := []struct {
		name         string
		field        goschema.Field
		expectedType string
		description  string
	}{
		{
			name: "INTEGER with auto_increment becomes SERIAL",
			field: goschema.Field{
				Name:    "id",
				Type:    "INTEGER",
				AutoInc: true,
				Primary: true,
			},
			expectedType: "SERIAL",
			description:  "INTEGER + auto_increment should become SERIAL",
		},
		{
			name: "INT with auto_increment becomes SERIAL",
			field: goschema.Field{
				Name:    "sequence_id",
				Type:    "INT",
				AutoInc: true,
			},
			expectedType: "SERIAL",
			description:  "INT + auto_increment should become SERIAL",
		},
		{
			name: "BIGINT with auto_increment becomes BIGSERIAL",
			field: goschema.Field{
				Name:    "big_id",
				Type:    "BIGINT",
				AutoInc: true,
			},
			expectedType: "BIGSERIAL",
			description:  "BIGINT + auto_increment should become BIGSERIAL",
		},
		{
			name: "SMALLINT with auto_increment becomes SMALLSERIAL",
			field: goschema.Field{
				Name:    "small_id",
				Type:    "SMALLINT",
				AutoInc: true,
			},
			expectedType: "SMALLSERIAL",
			description:  "SMALLINT + auto_increment should become SMALLSERIAL",
		},
		{
			name: "VARCHAR without auto_increment stays VARCHAR",
			field: goschema.Field{
				Name:    "name",
				Type:    "VARCHAR(255)",
				AutoInc: false,
			},
			expectedType: "VARCHAR(255)",
			description:  "Non-auto-increment fields should keep their original type",
		},
		{
			name: "Custom type with auto_increment becomes SERIAL",
			field: goschema.Field{
				Name:    "custom_id",
				Type:    "CUSTOM_TYPE",
				AutoInc: true,
			},
			expectedType: "SERIAL",
			description:  "Unknown types with auto_increment should default to SERIAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			column := generator.convertFieldToColumn(tt.field, nil)
			c.Assert(column.Type, qt.Equals, tt.expectedType, qt.Commentf(tt.description))
		})
	}
}

func TestAutoIncrementInCreateTable(t *testing.T) {
	c := qt.New(t)
	generator := New()

	table := goschema.Table{
		StructName: "TestTable",
		Name:       "test_table",
	}

	fields := []goschema.Field{
		{
			StructName: "TestTable",
			Name:       "id",
			Type:       "INTEGER",
			AutoInc:    true,
			Primary:    true,
		},
		{
			StructName: "TestTable",
			Name:       "big_id",
			Type:       "BIGINT",
			AutoInc:    true,
			Unique:     true,
		},
		{
			StructName: "TestTable",
			Name:       "name",
			Type:       "VARCHAR(255)",
			AutoInc:    false,
			Nullable:   false,
		},
	}

	sql := generator.GenerateCreateTable(table, fields, nil, nil)

	// Verify that auto_increment fields are converted to SERIAL types
	c.Assert(strings.Contains(sql, "id SERIAL PRIMARY KEY"), qt.IsTrue,
		qt.Commentf("INTEGER + auto_increment + primary should generate 'id SERIAL PRIMARY KEY'"))

	c.Assert(strings.Contains(sql, "big_id BIGSERIAL UNIQUE"), qt.IsTrue,
		qt.Commentf("BIGINT + auto_increment + unique should generate 'big_id BIGSERIAL UNIQUE'"))

	c.Assert(strings.Contains(sql, "name VARCHAR(255) NOT NULL"), qt.IsTrue,
		qt.Commentf("Regular field should remain unchanged"))

	// Verify that AUTO_INCREMENT keyword is NOT present in PostgreSQL output
	c.Assert(strings.Contains(sql, "AUTO_INCREMENT"), qt.IsFalse,
		qt.Commentf("PostgreSQL output should not contain AUTO_INCREMENT keyword"))
}

func TestPlatformSpecificOverride(t *testing.T) {
	c := qt.New(t)
	generator := New()

	// Test that platform-specific overrides take precedence over auto_increment conversion
	field := goschema.Field{
		Name:    "id",
		Type:    "INTEGER",
		AutoInc: true,
		Primary: true,
		Overrides: map[string]map[string]string{
			"postgres": {
				"type": "UUID",
			},
		},
	}

	column := generator.convertFieldToColumn(field, nil)

	// Platform override should take precedence over auto_increment conversion
	c.Assert(column.Type, qt.Equals, "UUID",
		qt.Commentf("Platform-specific type override should take precedence over auto_increment conversion"))
}
