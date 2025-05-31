package executor

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestPostgreSQLWriter_DryRunMode(t *testing.T) {
	t.Run("SetDryRun and IsDryRun", func(t *testing.T) {
		c := qt.New(t)
		writer := NewPostgreSQLWriter(nil, "public")

		// Initially dry run should be false
		c.Assert(writer.IsDryRun(), qt.IsFalse)

		// Enable dry run
		writer.SetDryRun(true)
		c.Assert(writer.IsDryRun(), qt.IsTrue)

		// Disable dry run
		writer.SetDryRun(false)
		c.Assert(writer.IsDryRun(), qt.IsFalse)
	})

	t.Run("ExecuteSQL in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewPostgreSQLWriter(nil, "public")
		writer.SetDryRun(true)

		// In dry run mode, ExecuteSQL should not fail even without a database connection
		err := writer.ExecuteSQL("CREATE TABLE test (id SERIAL PRIMARY KEY)")
		c.Assert(err, qt.IsNil)
	})

	t.Run("BeginTransaction in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewPostgreSQLWriter(nil, "public")
		writer.SetDryRun(true)

		// In dry run mode, BeginTransaction should not fail even without a database connection
		err := writer.BeginTransaction()
		c.Assert(err, qt.IsNil)
	})

	t.Run("CommitTransaction in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewPostgreSQLWriter(nil, "public")
		writer.SetDryRun(true)

		// In dry run mode, CommitTransaction should not fail even without a database connection
		err := writer.CommitTransaction()
		c.Assert(err, qt.IsNil)
	})

	t.Run("RollbackTransaction in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewPostgreSQLWriter(nil, "public")
		writer.SetDryRun(true)

		// In dry run mode, RollbackTransaction should not fail even without a database connection
		err := writer.RollbackTransaction()
		c.Assert(err, qt.IsNil)
	})

	t.Run("WriteSchema in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewPostgreSQLWriter(nil, "public")
		writer.SetDryRun(true)

		result := createTestParseResult()

		// In dry run mode, WriteSchema should not fail even without a database connection
		err := writer.WriteSchema(result)
		c.Assert(err, qt.IsNil)
	})

	t.Run("DropSchema in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewPostgreSQLWriter(nil, "public")
		writer.SetDryRun(true)

		result := createTestParseResult()

		// In dry run mode, DropSchema should not fail even without a database connection
		err := writer.DropSchema(result)
		c.Assert(err, qt.IsNil)
	})

	t.Run("DropAllTables in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewPostgreSQLWriter(nil, "public")
		writer.SetDryRun(true)

		// In dry run mode, DropAllTables should not fail even without a database connection
		err := writer.DropAllTables()
		c.Assert(err, qt.IsNil)
	})
}

func TestMySQLWriter_DryRunMode(t *testing.T) {
	t.Run("SetDryRun and IsDryRun", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test")

		// Initially dry run should be false
		c.Assert(writer.IsDryRun(), qt.IsFalse)

		// Enable dry run
		writer.SetDryRun(true)
		c.Assert(writer.IsDryRun(), qt.IsTrue)

		// Disable dry run
		writer.SetDryRun(false)
		c.Assert(writer.IsDryRun(), qt.IsFalse)
	})

	t.Run("ExecuteSQL in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test")
		writer.SetDryRun(true)

		// In dry run mode, ExecuteSQL should not fail even without a database connection
		err := writer.ExecuteSQL("CREATE TABLE test (id INT AUTO_INCREMENT PRIMARY KEY)")
		c.Assert(err, qt.IsNil)
	})

	t.Run("BeginTransaction in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test")
		writer.SetDryRun(true)

		// In dry run mode, BeginTransaction should not fail even without a database connection
		err := writer.BeginTransaction()
		c.Assert(err, qt.IsNil)
	})

	t.Run("CommitTransaction in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test")
		writer.SetDryRun(true)

		// In dry run mode, CommitTransaction should not fail even without a database connection
		err := writer.CommitTransaction()
		c.Assert(err, qt.IsNil)
	})

	t.Run("RollbackTransaction in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test")
		writer.SetDryRun(true)

		// In dry run mode, RollbackTransaction should not fail even without a database connection
		err := writer.RollbackTransaction()
		c.Assert(err, qt.IsNil)
	})

	t.Run("WriteSchema in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test")
		writer.SetDryRun(true)

		result := createTestParseResult()

		// In dry run mode, WriteSchema should not fail even without a database connection
		err := writer.WriteSchema(result)
		c.Assert(err, qt.IsNil)
	})

	t.Run("DropSchema in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test")
		writer.SetDryRun(true)

		result := createTestParseResult()

		// In dry run mode, DropSchema should not fail even without a database connection
		err := writer.DropSchema(result)
		c.Assert(err, qt.IsNil)
	})

	t.Run("DropAllTables in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test")
		writer.SetDryRun(true)

		// In dry run mode, DropAllTables should not fail even without a database connection
		err := writer.DropAllTables()
		c.Assert(err, qt.IsNil)
	})

	t.Run("tableExists in dry run mode", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test")
		writer.SetDryRun(true)

		// In dry run mode, tableExists should return false to show all operations
		exists := writer.tableExists("any_table")
		c.Assert(exists, qt.IsFalse)
	})
}

func TestSchemaWriterInterface_DryRunMethods(t *testing.T) {
	t.Run("PostgreSQL writer implements dry run methods", func(t *testing.T) {
		c := qt.New(t)
		writer := NewPostgreSQLWriter(nil, "public")

		// Verify the writer implements the SchemaWriter interface including dry run methods
		var schemaWriter SchemaWriter = writer
		c.Assert(schemaWriter, qt.IsNotNil)

		// Test that dry run methods are available
		schemaWriter.SetDryRun(true)
		c.Assert(schemaWriter.IsDryRun(), qt.IsTrue)
	})

	t.Run("MySQL writer implements dry run methods", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test")

		// Verify the writer implements the SchemaWriter interface including dry run methods
		var schemaWriter SchemaWriter = writer
		c.Assert(schemaWriter, qt.IsNotNil)

		// Test that dry run methods are available
		schemaWriter.SetDryRun(true)
		c.Assert(schemaWriter.IsDryRun(), qt.IsTrue)
	})
}

func TestDryRunMode_NoActualDatabaseOperations(t *testing.T) {
	t.Run("PostgreSQL dry run performs no actual operations", func(t *testing.T) {
		c := qt.New(t)
		writer := NewPostgreSQLWriter(nil, "public")
		writer.SetDryRun(true)

		result := createTestParseResult()

		// All these operations should succeed without a database connection in dry run mode
		err := writer.BeginTransaction()
		c.Assert(err, qt.IsNil)

		err = writer.ExecuteSQL("CREATE TYPE test_enum AS ENUM ('value1', 'value2')")
		c.Assert(err, qt.IsNil)

		err = writer.ExecuteSQL("CREATE TABLE test (id SERIAL PRIMARY KEY)")
		c.Assert(err, qt.IsNil)

		err = writer.WriteSchema(result)
		c.Assert(err, qt.IsNil)

		err = writer.DropSchema(result)
		c.Assert(err, qt.IsNil)

		err = writer.DropAllTables()
		c.Assert(err, qt.IsNil)

		err = writer.CommitTransaction()
		c.Assert(err, qt.IsNil)

		err = writer.RollbackTransaction()
		c.Assert(err, qt.IsNil)
	})

	t.Run("MySQL dry run performs no actual operations", func(t *testing.T) {
		c := qt.New(t)
		writer := NewMySQLWriter(nil, "test")
		writer.SetDryRun(true)

		result := createTestParseResult()

		// All these operations should succeed without a database connection in dry run mode
		err := writer.BeginTransaction()
		c.Assert(err, qt.IsNil)

		err = writer.ExecuteSQL("CREATE TABLE test (id INT AUTO_INCREMENT PRIMARY KEY)")
		c.Assert(err, qt.IsNil)

		err = writer.WriteSchema(result)
		c.Assert(err, qt.IsNil)

		err = writer.DropSchema(result)
		c.Assert(err, qt.IsNil)

		err = writer.DropAllTables()
		c.Assert(err, qt.IsNil)

		err = writer.CommitTransaction()
		c.Assert(err, qt.IsNil)

		err = writer.RollbackTransaction()
		c.Assert(err, qt.IsNil)
	})
}
