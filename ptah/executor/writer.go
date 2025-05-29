package executor

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/schema/builder"
	"github.com/denisvmedia/inventario/ptah/schema/meta"
)

// SchemaWriter interface for writing schemas to databases
type SchemaWriter interface {
	WriteSchema(result *builder.PackageParseResult) error
	DropSchema(result *builder.PackageParseResult) error
	DropAllTables() error
	ExecuteSQL(sql string) error
	BeginTransaction() error
	CommitTransaction() error
	RollbackTransaction() error
	CheckSchemaExists(result *builder.PackageParseResult) ([]string, error)
}

// PostgreSQLWriter writes schemas to PostgreSQL databases
type PostgreSQLWriter struct {
	db     *sql.DB
	tx     *sql.Tx
	schema string
}

// NewPostgreSQLWriter creates a new PostgreSQL schema writer
func NewPostgreSQLWriter(db *sql.DB, schema string) *PostgreSQLWriter {
	if schema == "" {
		schema = "public"
	}
	return &PostgreSQLWriter{
		db:     db,
		schema: schema,
	}
}

// WriteSchema writes the complete schema to the database
func (w *PostgreSQLWriter) WriteSchema(result *builder.PackageParseResult) error {
	// Start transaction
	if err := w.BeginTransaction(); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Rollback on error, commit on success
	defer func() {
		if w.tx != nil {
			w.RollbackTransaction()
		}
	}()

	// 1. Create enums first (PostgreSQL requires this)
	if err := w.writeEnums(result.Enums); err != nil {
		return fmt.Errorf("failed to write enums: %w", err)
	}

	// 2. Create tables in dependency order
	statements := GetOrderedCreateStatements(result, "postgres")
	for i, statement := range statements {
		fmt.Printf("Creating table %d/%d...\n", i+1, len(statements))
		if err := w.ExecuteSQL(statement); err != nil {
			return fmt.Errorf("failed to create table %d: %w", i+1, err)
		}
	}

	// 3. Commit transaction
	if err := w.CommitTransaction(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("Successfully created %d tables, %d enums\n", len(result.Tables), len(result.Enums))
	return nil
}

// writeEnums creates all enum types
func (w *PostgreSQLWriter) writeEnums(enums []meta.GlobalEnum) error {
	for _, enum := range enums {
		// Check if enum already exists
		var exists bool
		checkSQL := `
			SELECT EXISTS (
				SELECT 1 FROM pg_type t
				JOIN pg_namespace n ON n.oid = t.typnamespace
				WHERE t.typname = $1 AND n.nspname = $2
			)`

		err := w.tx.QueryRow(checkSQL, enum.Name, w.schema).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check if enum %s exists: %w", enum.Name, err)
		}

		if exists {
			fmt.Printf("Enum %s already exists, skipping...\n", enum.Name)
			continue
		}

		// Create enum
		values := make([]string, len(enum.Values))
		for i, v := range enum.Values {
			values[i] = "'" + v + "'"
		}

		createEnumSQL := fmt.Sprintf("CREATE TYPE %s AS ENUM (%s)",
			enum.Name, strings.Join(values, ", "))

		fmt.Printf("Creating enum: %s\n", enum.Name)
		if err := w.ExecuteSQL(createEnumSQL); err != nil {
			return fmt.Errorf("failed to create enum %s: %w", enum.Name, err)
		}
	}
	return nil
}

// ExecuteSQL executes a SQL statement
func (w *PostgreSQLWriter) ExecuteSQL(sql string) error {
	if w.tx == nil {
		return fmt.Errorf("no active transaction")
	}

	_, err := w.tx.Exec(sql)
	if err != nil {
		return fmt.Errorf("SQL execution failed: %w\nSQL: %s", err, sql)
	}
	return nil
}

// BeginTransaction starts a new transaction
func (w *PostgreSQLWriter) BeginTransaction() error {
	if w.tx != nil {
		return fmt.Errorf("transaction already active")
	}

	tx, err := w.db.Begin()
	if err != nil {
		return err
	}
	w.tx = tx
	return nil
}

// CommitTransaction commits the current transaction
func (w *PostgreSQLWriter) CommitTransaction() error {
	if w.tx == nil {
		return fmt.Errorf("no active transaction")
	}

	err := w.tx.Commit()
	w.tx = nil
	return err
}

// RollbackTransaction rolls back the current transaction
func (w *PostgreSQLWriter) RollbackTransaction() error {
	if w.tx == nil {
		return nil // No transaction to rollback
	}

	err := w.tx.Rollback()
	w.tx = nil
	return err
}

// DropSchema drops all tables and enums in the schema (DANGEROUS!)
func (w *PostgreSQLWriter) DropSchema(result *builder.PackageParseResult) error {
	fmt.Println("WARNING: This will drop all tables and enums!")

	// Start transaction
	if err := w.BeginTransaction(); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Rollback on error, commit on success
	defer func() {
		if w.tx != nil {
			w.RollbackTransaction()
		}
	}()

	// Drop tables in reverse dependency order
	tables := result.Tables
	for i := len(tables) - 1; i >= 0; i-- {
		table := tables[i]
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table.Name)
		fmt.Printf("Dropping table: %s\n", table.Name)
		if err := w.ExecuteSQL(dropSQL); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table.Name, err)
		}
	}

	// Drop enums
	for _, enum := range result.Enums {
		dropSQL := fmt.Sprintf("DROP TYPE IF EXISTS %s CASCADE", enum.Name)
		fmt.Printf("Dropping enum: %s\n", enum.Name)
		if err := w.ExecuteSQL(dropSQL); err != nil {
			return fmt.Errorf("failed to drop enum %s: %w", enum.Name, err)
		}
	}

	// Commit transaction
	if err := w.CommitTransaction(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("Successfully dropped %d tables, %d enums\n", len(result.Tables), len(result.Enums))
	return nil
}

// DropAllTables drops ALL tables and enums in the database schema (COMPLETE CLEANUP!)
func (w *PostgreSQLWriter) DropAllTables() error {
	fmt.Println("WARNING: This will drop ALL tables and enums in the database!")

	// Start transaction
	if err := w.BeginTransaction(); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Rollback on error, commit on success
	defer func() {
		if w.tx != nil {
			w.RollbackTransaction()
		}
	}()

	// Get all tables in the schema
	tablesQuery := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = $1 AND table_type = 'BASE TABLE'
		ORDER BY table_name`

	rows, err := w.db.Query(tablesQuery, w.schema)
	if err != nil {
		return fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	// Drop all tables with CASCADE to handle dependencies
	for _, tableName := range tables {
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", tableName)
		fmt.Printf("Dropping table: %s\n", tableName)
		if err := w.ExecuteSQL(dropSQL); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", tableName, err)
		}
	}

	// Get all custom types (enums) in the schema
	enumsQuery := `
		SELECT typname
		FROM pg_type t
		JOIN pg_namespace n ON t.typnamespace = n.oid
		WHERE n.nspname = $1 AND t.typtype = 'e'
		ORDER BY typname`

	enumRows, err := w.db.Query(enumsQuery, w.schema)
	if err != nil {
		return fmt.Errorf("failed to query enums: %w", err)
	}
	defer enumRows.Close()

	var enums []string
	for enumRows.Next() {
		var enumName string
		if err := enumRows.Scan(&enumName); err != nil {
			return fmt.Errorf("failed to scan enum name: %w", err)
		}
		enums = append(enums, enumName)
	}

	// Drop all enums
	for _, enumName := range enums {
		dropSQL := fmt.Sprintf("DROP TYPE IF EXISTS %s CASCADE", enumName)
		fmt.Printf("Dropping enum: %s\n", enumName)
		if err := w.ExecuteSQL(dropSQL); err != nil {
			return fmt.Errorf("failed to drop enum %s: %w", enumName, err)
		}
	}

	// Get all sequences in the schema and drop them
	sequencesQuery := `
		SELECT sequence_name
		FROM information_schema.sequences
		WHERE sequence_schema = $1
		ORDER BY sequence_name`

	seqRows, err := w.db.Query(sequencesQuery, w.schema)
	if err != nil {
		return fmt.Errorf("failed to query sequences: %w", err)
	}
	defer seqRows.Close()

	var sequences []string
	for seqRows.Next() {
		var sequenceName string
		if err := seqRows.Scan(&sequenceName); err != nil {
			return fmt.Errorf("failed to scan sequence name: %w", err)
		}
		sequences = append(sequences, sequenceName)
	}

	// Drop all sequences
	for _, sequenceName := range sequences {
		dropSQL := fmt.Sprintf("DROP SEQUENCE IF EXISTS %s CASCADE", sequenceName)
		fmt.Printf("Dropping sequence: %s\n", sequenceName)
		if err := w.ExecuteSQL(dropSQL); err != nil {
			return fmt.Errorf("failed to drop sequence %s: %w", sequenceName, err)
		}
	}

	// Commit transaction
	if err := w.CommitTransaction(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("Successfully dropped %d tables, %d enums, %d sequences\n", len(tables), len(enums), len(sequences))
	return nil
}

// CheckSchemaExists checks if any tables from the schema already exist
func (w *PostgreSQLWriter) CheckSchemaExists(result *builder.PackageParseResult) ([]string, error) {
	var existingTables []string

	for _, table := range result.Tables {
		var exists bool
		checkSQL := `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = $1 AND table_name = $2
			)`

		err := w.db.QueryRow(checkSQL, w.schema, table.Name).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("failed to check if table %s exists: %w", table.Name, err)
		}

		if exists {
			existingTables = append(existingTables, table.Name)
		}
	}

	return existingTables, nil
}

// MySQLWriter writes schemas to MySQL/MariaDB databases
type MySQLWriter struct {
	db     *sql.DB
	tx     *sql.Tx
	schema string
}

// NewMySQLWriter creates a new MySQL schema writer
func NewMySQLWriter(db *sql.DB, schema string) *MySQLWriter {
	return &MySQLWriter{
		db:     db,
		schema: schema,
	}
}

// WriteSchema writes the complete schema to the database
func (w *MySQLWriter) WriteSchema(result *builder.PackageParseResult) error {
	// Start transaction
	if err := w.BeginTransaction(); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Rollback on error
	defer func() {
		if w.tx != nil {
			w.RollbackTransaction()
		}
	}()

	// MySQL doesn't have separate enum types like PostgreSQL
	// Enums are defined inline in column definitions

	// Get existing tables to avoid conflicts
	existingTables, err := w.CheckSchemaExists(result)
	if err != nil {
		return fmt.Errorf("failed to check existing schema: %w", err)
	}
	existingTableMap := make(map[string]bool)
	for _, table := range existingTables {
		existingTableMap[table] = true
	}

	// Create tables in dependency order
	statements := GetOrderedCreateStatements(result, "mysql")
	for i, statement := range statements {
		fmt.Printf("Creating table %d/%d...\n", i+1, len(statements))

		// Split the statement into individual SQL commands
		sqlCommands := w.splitSQLStatements(statement)
		for _, sqlCmd := range sqlCommands {
			// Skip CREATE TABLE statements for existing tables
			if w.isCreateTableStatement(sqlCmd) {
				tableName := w.extractTableNameFromCreateTable(sqlCmd)
				if existingTableMap[tableName] {
					fmt.Printf("Table %s already exists, skipping CREATE TABLE...\n", tableName)
					continue
				}
			}

			// Skip CREATE INDEX statements for non-existent tables
			if w.isCreateIndexStatement(sqlCmd) {
				tableName := w.extractTableNameFromCreateIndex(sqlCmd)
				if !w.tableExists(tableName) {
					fmt.Printf("Table %s doesn't exist, skipping CREATE INDEX...\n", tableName)
					continue
				}
			}

			if err := w.ExecuteSQL(sqlCmd); err != nil {
				return fmt.Errorf("failed to execute SQL for table %d: %w", i+1, err)
			}
		}
	}

	// Commit transaction
	if err := w.CommitTransaction(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("Successfully created %d tables\n", len(result.Tables))
	return nil
}

// ExecuteSQL executes a SQL statement
func (w *MySQLWriter) ExecuteSQL(sql string) error {
	if w.tx == nil {
		return fmt.Errorf("no active transaction")
	}

	_, err := w.tx.Exec(sql)
	if err != nil {
		return fmt.Errorf("SQL execution failed: %w\nSQL: %s", err, sql)
	}
	return nil
}

// BeginTransaction starts a new transaction
func (w *MySQLWriter) BeginTransaction() error {
	if w.tx != nil {
		return fmt.Errorf("transaction already active")
	}

	tx, err := w.db.Begin()
	if err != nil {
		return err
	}
	w.tx = tx
	return nil
}

// CommitTransaction commits the current transaction
func (w *MySQLWriter) CommitTransaction() error {
	if w.tx == nil {
		return fmt.Errorf("no active transaction")
	}

	err := w.tx.Commit()
	w.tx = nil
	return err
}

// RollbackTransaction rolls back the current transaction
func (w *MySQLWriter) RollbackTransaction() error {
	if w.tx == nil {
		return nil // No transaction to rollback
	}

	err := w.tx.Rollback()
	w.tx = nil
	return err
}

// CheckSchemaExists checks if any tables from the schema already exist
func (w *MySQLWriter) CheckSchemaExists(result *builder.PackageParseResult) ([]string, error) {
	var existingTables []string

	for _, table := range result.Tables {
		var exists bool
		checkSQL := `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = DATABASE() AND table_name = ?
			)`

		err := w.db.QueryRow(checkSQL, table.Name).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("failed to check if table %s exists: %w", table.Name, err)
		}

		if exists {
			existingTables = append(existingTables, table.Name)
		}
	}

	return existingTables, nil
}

// DropSchema drops all tables in the schema (DANGEROUS!)
func (w *MySQLWriter) DropSchema(result *builder.PackageParseResult) error {
	fmt.Println("WARNING: This will drop all tables!")

	// Start transaction
	if err := w.BeginTransaction(); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Rollback on error, commit on success
	defer func() {
		if w.tx != nil {
			w.RollbackTransaction()
		}
	}()

	// Disable foreign key checks to avoid dependency issues
	if err := w.ExecuteSQL("SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return fmt.Errorf("failed to disable foreign key checks: %w", err)
	}

	// Drop tables in reverse dependency order
	tables := result.Tables
	for i := len(tables) - 1; i >= 0; i-- {
		table := tables[i]
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s", table.Name)
		fmt.Printf("Dropping table: %s\n", table.Name)
		if err := w.ExecuteSQL(dropSQL); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table.Name, err)
		}
	}

	// Re-enable foreign key checks
	if err := w.ExecuteSQL("SET FOREIGN_KEY_CHECKS = 1"); err != nil {
		return fmt.Errorf("failed to re-enable foreign key checks: %w", err)
	}

	// Commit transaction
	if err := w.CommitTransaction(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("Successfully dropped %d tables\n", len(result.Tables))
	return nil
}

// DropAllTables drops ALL tables in the database (COMPLETE CLEANUP!)
func (w *MySQLWriter) DropAllTables() error {
	fmt.Println("WARNING: This will drop ALL tables in the database!")

	// Start transaction
	if err := w.BeginTransaction(); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Rollback on error, commit on success
	defer func() {
		if w.tx != nil {
			w.RollbackTransaction()
		}
	}()

	// Disable foreign key checks to avoid dependency issues
	if err := w.ExecuteSQL("SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return fmt.Errorf("failed to disable foreign key checks: %w", err)
	}

	// Get all tables in the current database
	tablesQuery := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
		ORDER BY table_name`

	rows, err := w.db.Query(tablesQuery)
	if err != nil {
		return fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	// Drop all tables
	for _, tableName := range tables {
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)
		fmt.Printf("Dropping table: %s\n", tableName)
		if err := w.ExecuteSQL(dropSQL); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", tableName, err)
		}
	}

	// Re-enable foreign key checks
	if err := w.ExecuteSQL("SET FOREIGN_KEY_CHECKS = 1"); err != nil {
		return fmt.Errorf("failed to re-enable foreign key checks: %w", err)
	}

	// Commit transaction
	if err := w.CommitTransaction(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("Successfully dropped %d tables\n", len(tables))
	return nil
}

// splitSQLStatements splits a multi-statement SQL string into individual statements
func (w *MySQLWriter) splitSQLStatements(sql string) []string {
	// Split by semicolon and filter out empty statements
	var statements []string
	parts := strings.Split(sql, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Skip empty parts and comments
		if part != "" && !strings.HasPrefix(part, "--") {
			statements = append(statements, part)
		}
	}

	return statements
}

// isCreateTableStatement checks if a SQL statement is a CREATE TABLE statement
func (w *MySQLWriter) isCreateTableStatement(sql string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "CREATE TABLE")
}

// isCreateIndexStatement checks if a SQL statement is a CREATE INDEX statement
func (w *MySQLWriter) isCreateIndexStatement(sql string) bool {
	return strings.Contains(strings.ToUpper(strings.TrimSpace(sql)), "CREATE") &&
		strings.Contains(strings.ToUpper(strings.TrimSpace(sql)), "INDEX")
}

// extractTableNameFromCreateTable extracts table name from CREATE TABLE statement
func (w *MySQLWriter) extractTableNameFromCreateTable(sql string) string {
	// Simple regex to extract table name from "CREATE TABLE tablename ("
	parts := strings.Fields(strings.TrimSpace(sql))
	if len(parts) >= 3 && strings.ToUpper(parts[0]) == "CREATE" && strings.ToUpper(parts[1]) == "TABLE" {
		return strings.TrimSuffix(parts[2], "(")
	}
	return ""
}

// extractTableNameFromCreateIndex extracts table name from CREATE INDEX statement
func (w *MySQLWriter) extractTableNameFromCreateIndex(sql string) string {
	// Look for "ON tablename" pattern
	parts := strings.Fields(strings.TrimSpace(sql))
	for i, part := range parts {
		if strings.ToUpper(part) == "ON" && i+1 < len(parts) {
			return strings.TrimSuffix(parts[i+1], "(")
		}
	}
	return ""
}

// tableExists checks if a table exists in the database
func (w *MySQLWriter) tableExists(tableName string) bool {
	var exists bool
	checkSQL := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = DATABASE() AND table_name = ?
		)`

	err := w.db.QueryRow(checkSQL, tableName).Scan(&exists)
	return err == nil && exists
}
