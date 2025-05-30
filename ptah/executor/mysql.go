package executor

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/denisvmedia/inventario/ptah/schema/parser"
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
)

// MySQLReader reads schema information from MySQL/MariaDB databases
type MySQLReader struct {
	db     *sql.DB
	schema string
}

// NewMySQLReader creates a new MySQL schema reader
func NewMySQLReader(db *sql.DB, schema string) *MySQLReader {
	if schema == "" {
		schema = "information_schema"
	}
	return &MySQLReader{
		db:     db,
		schema: schema,
	}
}

// ReadSchema reads the complete schema from MySQL/MariaDB
func (r *MySQLReader) ReadSchema() (*parsertypes.DatabaseSchema, error) {
	schema := &parsertypes.DatabaseSchema{}

	// Get current database name
	var dbName string
	err := r.db.QueryRow("SELECT DATABASE()").Scan(&dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to get database name: %w", err)
	}

	// Read tables
	tables, err := r.readTables(dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to read tables: %w", err)
	}
	schema.Tables = tables

	// Read enums (MySQL stores them as column types)
	enums, err := r.readEnums(dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to read enums: %w", err)
	}
	schema.Enums = enums

	// Read indexes
	indexes, err := r.readIndexes(dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to read indexes: %w", err)
	}
	schema.Indexes = indexes

	// Read constraints
	constraints, err := r.readConstraints(dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to read constraints: %w", err)
	}
	schema.Constraints = constraints

	return schema, nil
}

// readTables reads all tables and their columns
func (r *MySQLReader) readTables(dbName string) ([]parsertypes.Table, error) {
	query := `
		SELECT
			t.TABLE_NAME,
			t.TABLE_TYPE,
			COALESCE(t.TABLE_COMMENT, '') as TABLE_COMMENT
		FROM information_schema.TABLES t
		WHERE t.TABLE_SCHEMA = ?
		AND t.TABLE_TYPE IN ('BASE TABLE', 'VIEW')
		ORDER BY t.TABLE_NAME`

	rows, err := r.db.Query(query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []parsertypes.Table
	for rows.Next() {
		var table parsertypes.Table
		err := rows.Scan(&table.Name, &table.Type, &table.Comment)
		if err != nil {
			return nil, err
		}

		// Read columns for this table
		columns, err := r.readColumns(dbName, table.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to read columns for table %s: %w", table.Name, err)
		}
		table.Columns = columns

		tables = append(tables, table)
	}

	return tables, nil
}

// readColumns reads columns for a specific table
func (r *MySQLReader) readColumns(dbName, tableName string) ([]parsertypes.Column, error) {
	query := `
		SELECT
			c.COLUMN_NAME,
			c.DATA_TYPE,
			c.COLUMN_TYPE,
			c.IS_NULLABLE,
			c.COLUMN_DEFAULT,
			COALESCE(c.CHARACTER_MAXIMUM_LENGTH, 0) as CHARACTER_MAXIMUM_LENGTH,
			COALESCE(c.NUMERIC_PRECISION, 0) as NUMERIC_PRECISION,
			COALESCE(c.NUMERIC_SCALE, 0) as NUMERIC_SCALE,
			c.ORDINAL_POSITION,
			c.EXTRA
		FROM information_schema.COLUMNS c
		WHERE c.TABLE_SCHEMA = ? AND c.TABLE_NAME = ?
		ORDER BY c.ORDINAL_POSITION`

	rows, err := r.db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []parsertypes.Column
	for rows.Next() {
		var col parsertypes.Column
		var characterMaxLength, numericPrecision, numericScale int
		var columnDefault sql.NullString
		var extra string

		err := rows.Scan(
			&col.Name,
			&col.DataType,
			&col.ColumnType,
			&col.IsNullable,
			&columnDefault,
			&characterMaxLength,
			&numericPrecision,
			&numericScale,
			&col.OrdinalPosition,
			&extra,
		)
		if err != nil {
			return nil, err
		}

		// Handle nullable default
		if columnDefault.Valid {
			col.ColumnDefault = &columnDefault.String
		}

		// Set numeric fields
		if characterMaxLength > 0 {
			col.CharacterMaxLength = &characterMaxLength
		}
		if numericPrecision > 0 {
			col.NumericPrecision = &numericPrecision
		}
		if numericScale > 0 {
			col.NumericScale = &numericScale
		}

		// Detect auto increment
		col.IsAutoIncrement = strings.Contains(strings.ToLower(extra), "auto_increment")

		columns = append(columns, col)
	}

	// Add derived fields (primary key, unique) from constraints
	err = r.addConstraintInfo(dbName, tableName, columns)
	if err != nil {
		return nil, fmt.Errorf("failed to add constraint info: %w", err)
	}

	return columns, nil
}

// addConstraintInfo adds primary key and unique constraint information to columns
func (r *MySQLReader) addConstraintInfo(dbName, tableName string, columns []parsertypes.Column) error {
	// Get primary key information
	pkQuery := `
		SELECT COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = 'PRIMARY'
		ORDER BY ORDINAL_POSITION`

	pkRows, err := r.db.Query(pkQuery, dbName, tableName)
	if err != nil {
		return err
	}
	defer pkRows.Close()

	pkColumns := make(map[string]bool)
	for pkRows.Next() {
		var colName string
		if err := pkRows.Scan(&colName); err != nil {
			return err
		}
		pkColumns[colName] = true
	}

	// Get unique constraint information
	uniqueQuery := `
		SELECT COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.TABLE_CONSTRAINTS tc ON
			kcu.CONSTRAINT_NAME = tc.CONSTRAINT_NAME AND
			kcu.TABLE_SCHEMA = tc.TABLE_SCHEMA AND
			kcu.TABLE_NAME = tc.TABLE_NAME
		WHERE kcu.TABLE_SCHEMA = ? AND kcu.TABLE_NAME = ?
		AND tc.CONSTRAINT_TYPE = 'UNIQUE'`

	uniqueRows, err := r.db.Query(uniqueQuery, dbName, tableName)
	if err != nil {
		return err
	}
	defer uniqueRows.Close()

	uniqueColumns := make(map[string]bool)
	for uniqueRows.Next() {
		var colName string
		if err := uniqueRows.Scan(&colName); err != nil {
			return err
		}
		uniqueColumns[colName] = true
	}

	// Update column information
	for i := range columns {
		columns[i].IsPrimaryKey = pkColumns[columns[i].Name]
		columns[i].IsUnique = uniqueColumns[columns[i].Name]
	}

	return nil
}

// readEnums reads enum types from MySQL (stored as column types)
func (r *MySQLReader) readEnums(dbName string) ([]parsertypes.Enum, error) {
	query := `
		SELECT DISTINCT
			COLUMN_TYPE
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ?
		AND DATA_TYPE = 'enum'`

	rows, err := r.db.Query(query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var enums []parsertypes.Enum
	enumMap := make(map[string][]string)

	for rows.Next() {
		var columnType string
		err := rows.Scan(&columnType)
		if err != nil {
			return nil, err
		}

		// Parse enum values from column type like "enum('value1','value2','value3')"
		values := parseEnumValues(columnType)
		if len(values) > 0 {
			// Create a unique name for this enum based on its values
			enumName := fmt.Sprintf("enum_%s", strings.Join(values, "_"))
			enumMap[enumName] = values
		}
	}

	// Convert map to slice
	for name, values := range enumMap {
		enums = append(enums, parsertypes.Enum{
			Name:   name,
			Values: values,
		})
	}

	return enums, nil
}

// parseEnumValues parses enum values from MySQL column type
func parseEnumValues(columnType string) []string {
	// Remove "enum(" and ")" from the string
	if !strings.HasPrefix(columnType, "enum(") {
		return nil
	}

	valuesPart := strings.TrimPrefix(columnType, "enum(")
	valuesPart = strings.TrimSuffix(valuesPart, ")")

	// Split by comma and clean up quotes
	var values []string
	parts := strings.Split(valuesPart, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, "'\"")
		if part != "" {
			values = append(values, part)
		}
	}

	return values
}

// readIndexes reads all indexes
func (r *MySQLReader) readIndexes(dbName string) ([]parsertypes.Index, error) {
	query := `
		SELECT
			s.INDEX_NAME,
			s.TABLE_NAME,
			GROUP_CONCAT(s.COLUMN_NAME ORDER BY s.SEQ_IN_INDEX) as COLUMNS,
			s.NON_UNIQUE,
			s.INDEX_TYPE
		FROM information_schema.STATISTICS s
		WHERE s.TABLE_SCHEMA = ?
		GROUP BY s.INDEX_NAME, s.TABLE_NAME, s.NON_UNIQUE, s.INDEX_TYPE
		ORDER BY s.TABLE_NAME, s.INDEX_NAME`

	rows, err := r.db.Query(query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []parsertypes.Index
	for rows.Next() {
		var index parsertypes.Index
		var columnsStr string
		var nonUnique int
		var indexType string

		err := rows.Scan(&index.Name, &index.TableName, &columnsStr, &nonUnique, &indexType)
		if err != nil {
			return nil, err
		}

		index.Columns = strings.Split(columnsStr, ",")
		index.IsUnique = nonUnique == 0
		index.IsPrimary = index.Name == "PRIMARY"
		index.Definition = fmt.Sprintf("%s INDEX %s ON %s (%s)", indexType, index.Name, index.TableName, columnsStr)

		indexes = append(indexes, index)
	}

	return indexes, nil
}

// readConstraints reads all constraints
func (r *MySQLReader) readConstraints(dbName string) ([]parsertypes.Constraint, error) {
	query := `
		SELECT
			tc.CONSTRAINT_NAME,
			tc.TABLE_NAME,
			tc.CONSTRAINT_TYPE,
			COALESCE(kcu.COLUMN_NAME, '') as COLUMN_NAME,
			COALESCE(kcu.REFERENCED_TABLE_NAME, '') as REFERENCED_TABLE_NAME,
			COALESCE(kcu.REFERENCED_COLUMN_NAME, '') as REFERENCED_COLUMN_NAME,
			COALESCE(rc.DELETE_RULE, '') as DELETE_RULE,
			COALESCE(rc.UPDATE_RULE, '') as UPDATE_RULE
		FROM information_schema.TABLE_CONSTRAINTS tc
		LEFT JOIN information_schema.KEY_COLUMN_USAGE kcu ON
			tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME AND
			tc.TABLE_SCHEMA = kcu.TABLE_SCHEMA AND
			tc.TABLE_NAME = kcu.TABLE_NAME
		LEFT JOIN information_schema.REFERENTIAL_CONSTRAINTS rc ON
			tc.CONSTRAINT_NAME = rc.CONSTRAINT_NAME AND
			tc.TABLE_SCHEMA = rc.CONSTRAINT_SCHEMA
		WHERE tc.TABLE_SCHEMA = ?
		ORDER BY tc.TABLE_NAME, tc.CONSTRAINT_NAME`

	rows, err := r.db.Query(query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints []parsertypes.Constraint
	for rows.Next() {
		var constraint parsertypes.Constraint
		var referencedTable, referencedColumn, deleteRule, updateRule string
		err := rows.Scan(
			&constraint.Name,
			&constraint.TableName,
			&constraint.Type,
			&constraint.ColumnName,
			&referencedTable,
			&referencedColumn,
			&deleteRule,
			&updateRule,
		)
		if err != nil {
			return nil, err
		}

		// Set foreign key references if they exist
		if referencedTable != "" {
			constraint.ForeignTable = &referencedTable
		}
		if referencedColumn != "" {
			constraint.ForeignColumn = &referencedColumn
		}
		if deleteRule != "" {
			constraint.DeleteRule = &deleteRule
		}
		if updateRule != "" {
			constraint.UpdateRule = &updateRule
		}

		constraints = append(constraints, constraint)
	}

	return constraints, nil
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
func (w *MySQLWriter) WriteSchema(result *parsertypes.PackageParseResult) error {
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
	statements := parser.GetOrderedCreateStatements(result, "mysql")
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
func (w *MySQLWriter) CheckSchemaExists(result *parsertypes.PackageParseResult) ([]string, error) {
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
func (w *MySQLWriter) DropSchema(result *parsertypes.PackageParseResult) error {
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
