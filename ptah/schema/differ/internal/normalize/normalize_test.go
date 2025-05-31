package normalize_test

import (
	"testing"

	"github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/schema/differ/internal/normalize"
)

func TestType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// VARCHAR variations
		{"varchar lowercase", "varchar", "varchar"},
		{"varchar uppercase", "VARCHAR", "varchar"},
		{"varchar with size", "VARCHAR(255)", "varchar"},
		{"varchar with small size", "varchar(100)", "varchar"},
		{"varchar2 oracle style", "VARCHAR2", "varchar"},
		{"varchar2 with size", "VARCHAR2(4000)", "varchar"},

		// TEXT variations
		{"text lowercase", "text", "text"},
		{"text uppercase", "TEXT", "text"},
		{"longtext mysql", "LONGTEXT", "text"},
		{"mediumtext mysql", "MEDIUMTEXT", "text"},
		{"tinytext mysql", "TINYTEXT", "text"},

		// Integer variations
		{"int lowercase", "int", "integer"},
		{"int uppercase", "INT", "integer"},
		{"integer full", "INTEGER", "integer"},
		{"bigint", "BIGINT", "integer"},
		{"smallint", "SMALLINT", "integer"},
		{"mediumint mysql", "MEDIUMINT", "integer"},

		// SERIAL types (PostgreSQL auto-increment)
		{"serial lowercase", "serial", "integer"},
		{"serial uppercase", "SERIAL", "integer"},
		{"bigserial", "BIGSERIAL", "integer"},

		// Boolean variations
		{"bool lowercase", "bool", "boolean"},
		{"bool uppercase", "BOOL", "boolean"},
		{"boolean full", "BOOLEAN", "boolean"},
		{"tinyint mysql boolean", "TINYINT", "boolean"},
		{"tinyint with size", "TINYINT(1)", "boolean"},

		// Timestamp variations
		{"timestamp lowercase", "timestamp", "timestamp"},
		{"timestamp uppercase", "TIMESTAMP", "timestamp"},
		{"timestamp with timezone", "TIMESTAMP WITH TIME ZONE", "timestamp"},
		{"timestamp without timezone", "TIMESTAMP WITHOUT TIME ZONE", "timestamp"},

		// Decimal variations
		{"decimal lowercase", "decimal", "decimal"},
		{"decimal uppercase", "DECIMAL", "decimal"},
		{"decimal with precision", "DECIMAL(10,2)", "decimal"},
		{"numeric", "NUMERIC", "decimal"},
		{"numeric with precision", "NUMERIC(5,2)", "decimal"},

		// Unrecognized types (should return as-is, lowercased)
		{"enum type", "ENUM('a','b','c')", "enum('a','b','c')"},
		{"json type", "JSON", "json"},
		{"uuid type", "UUID", "uuid"},
		{"custom type", "MyCustomType", "mycustomtype"},

		// Edge cases
		{"empty string", "", ""},
		{"mixed case complex", "VarChar(255)", "varchar"},
		{"multiple keywords", "UNSIGNED BIGINT", "integer"},
		{"partial match", "varchar_custom", "varchar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := quicktest.New(t)
			result := normalize.Type(tt.input)
			c.Assert(result, quicktest.Equals, tt.expected)
		})
	}
}

func TestDefaultValue(t *testing.T) {
	tests := []struct {
		name         string
		defaultValue string
		typeName     string
		expected     string
	}{
		// Empty and NULL handling
		{"empty string", "", "varchar", ""},
		{"null uppercase", "NULL", "varchar", ""},
		{"null lowercase", "null", "varchar", ""},
		{"null mixed case", "Null", "varchar", ""},

		// Quote removal
		{"single quotes", "'hello'", "varchar", "hello"},
		{"double quotes", "\"world\"", "varchar", "world"},
		{"single quotes with spaces", "' hello world '", "varchar", " hello world "},
		{"double quotes with spaces", "\" test value \"", "varchar", " test value "},
		{"no quotes", "plain_value", "varchar", "plain_value"},

		// Boolean normalization for boolean types
		{"boolean true string", "true", "boolean", "true"},
		{"boolean false string", "false", "boolean", "false"},
		{"boolean one", "1", "boolean", "true"},
		{"boolean zero", "0", "boolean", "false"},
		{"boolean TRUE uppercase", "TRUE", "boolean", "true"},
		{"boolean FALSE uppercase", "FALSE", "boolean", "false"},
		{"boolean quoted true", "'true'", "boolean", "true"},
		{"boolean quoted one", "'1'", "boolean", "true"},
		{"boolean quoted zero", "\"0\"", "boolean", "false"},

		// Boolean values for non-boolean types (should not be normalized)
		{"one for integer", "1", "integer", "1"},
		{"zero for varchar", "0", "varchar", "0"},
		{"true for text", "true", "text", "true"},

		// Complex cases
		{"quoted null", "'NULL'", "varchar", "NULL"},
		{"double quoted null", "\"NULL\"", "varchar", "NULL"},
		{"nested quotes", "\"'value'\"", "varchar", "value"},
		{"special characters", "'special@#$%'", "varchar", "special@#$%"},

		// Edge cases for boolean normalization
		{"unrecognized boolean value", "maybe", "boolean", "maybe"},
		{"numeric string for boolean", "123", "boolean", "123"},
		{"empty boolean", "", "boolean", ""},

		// Additional quote scenarios
		{"only single quote", "'", "varchar", ""},
		{"only double quote", "\"", "varchar", ""},
		{"mixed quotes start", "'value\"", "varchar", "value"},
		{"mixed quotes end", "\"value'", "varchar", "value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := quicktest.New(t)
			result := normalize.DefaultValue(tt.defaultValue, tt.typeName)
			c.Assert(result, quicktest.Equals, tt.expected)
		})
	}
}

func BenchmarkType(b *testing.B) {
	testCases := []string{
		"VARCHAR(255)",
		"BIGINT",
		"BOOLEAN",
		"TIMESTAMP WITH TIME ZONE",
		"DECIMAL(10,2)",
		"ENUM('a','b','c')",
	}

	for _, tc := range testCases {
		b.Run(tc, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				normalize.Type(tc)
			}
		})
	}
}

func BenchmarkDefaultValue(b *testing.B) {
	testCases := []struct {
		defaultValue string
		typeName     string
	}{
		{"'hello world'", "varchar"},
		{"1", "boolean"},
		{"NULL", "integer"},
		{"\"complex 'nested' value\"", "text"},
	}

	for _, tc := range testCases {
		b.Run(tc.defaultValue+"_"+tc.typeName, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				normalize.DefaultValue(tc.defaultValue, tc.typeName)
			}
		})
	}
}
