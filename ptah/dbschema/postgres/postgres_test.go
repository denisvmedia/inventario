package postgres

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/dbschema/internal/testutils"
	"github.com/denisvmedia/inventario/ptah/dbschema/types"
)

func TestNewPostgreSQLReader(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tests := []struct {
			name           string
			schema         string
			expectedSchema string
		}{
			{
				name:           "with custom schema",
				schema:         "test_schema",
				expectedSchema: "test_schema",
			},
			{
				name:           "with empty schema defaults to public",
				schema:         "",
				expectedSchema: "public",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				reader := NewPostgreSQLReader(nil, test.schema)
				c.Assert(reader, qt.IsNotNil)
				c.Assert(reader.schema, qt.Equals, test.expectedSchema)
				c.Assert(reader.db, qt.IsNil) // We passed nil for testing
			})
		}
	})
}

func TestPostgreSQLReader_ReadSchema_NoConnection(t *testing.T) {
	c := qt.New(t)

	// Test that reader can be created with nil database
	reader := NewPostgreSQLReader(nil, "public")
	c.Assert(reader, qt.IsNotNil)
	c.Assert(reader.schema, qt.Equals, "public")
	c.Assert(reader.db, qt.IsNil)

	// Note: We don't test ReadSchema() with nil db as it would panic
	// This is expected behavior - the reader requires a valid database connection
}

func TestPostgreSQLReader_enhanceTablesWithConstraints(t *testing.T) {
	c := qt.New(t)

	reader := NewPostgreSQLReader(nil, "public")

	// Create test data
	tables := []types.DBTable{
		{
			Name: "test_table",
			Columns: []types.DBColumn{
				{Name: "id", IsPrimaryKey: false, IsUnique: false},
				{Name: "email", IsPrimaryKey: false, IsUnique: false},
				{Name: "name", IsPrimaryKey: false, IsUnique: false},
			},
		},
	}

	constraints := []types.DBConstraint{
		{TableName: "test_table", ColumnName: "id", Type: "PRIMARY KEY"},
		{TableName: "test_table", ColumnName: "email", Type: "UNIQUE"},
	}

	// Test the enhancement
	reader.enhanceTablesWithConstraints(tables, constraints)

	// Verify results
	idCol := testutils.FindColumn(tables[0].Columns, "id")
	c.Assert(idCol.IsPrimaryKey, qt.IsTrue)
	c.Assert(idCol.IsUnique, qt.IsFalse)

	emailCol := testutils.FindColumn(tables[0].Columns, "email")
	c.Assert(emailCol.IsPrimaryKey, qt.IsFalse)
	c.Assert(emailCol.IsUnique, qt.IsTrue)

	nameCol := testutils.FindColumn(tables[0].Columns, "name")
	c.Assert(nameCol.IsPrimaryKey, qt.IsFalse)
	c.Assert(nameCol.IsUnique, qt.IsFalse)
}

func TestNewPostgreSQLWriter(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tests := []struct {
			name           string
			schema         string
			expectedSchema string
		}{
			{
				name:           "with custom schema",
				schema:         "test_schema",
				expectedSchema: "test_schema",
			},
			{
				name:           "with empty schema defaults to public",
				schema:         "",
				expectedSchema: "public",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c := qt.New(t)
				writer := NewPostgreSQLWriter(nil, test.schema)
				c.Assert(writer, qt.IsNotNil)
				c.Assert(writer.schema, qt.Equals, test.expectedSchema)
				c.Assert(writer.db, qt.IsNil) // We passed nil for testing
				c.Assert(writer.tx, qt.IsNil) // No transaction initially
			})
		}
	})
}

func TestPostgreSQLWriter_TransactionMethods_NoConnection(t *testing.T) {
	c := qt.New(t)
	writer := NewPostgreSQLWriter(nil, "public")

	t.Run("ExecuteSQL with no transaction", func(t *testing.T) {
		err := writer.ExecuteSQL("SELECT 1")
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Equals, "no active transaction")
	})

	t.Run("CommitTransaction with no transaction", func(t *testing.T) {
		err := writer.CommitTransaction()
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Equals, "no active transaction")
	})

	t.Run("RollbackTransaction with no transaction", func(t *testing.T) {
		err := writer.RollbackTransaction()
		c.Assert(err, qt.IsNil) // Should not error when no transaction
	})
}

func TestPostgreSQLWriter_SchemaWriterInterface(t *testing.T) {
	c := qt.New(t)
	writer := NewPostgreSQLWriter(nil, "public")
	var _ types.SchemaWriter = writer
	c.Assert(writer, qt.IsNotNil)
}
