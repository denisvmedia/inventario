package executor

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq" // PostgreSQL driver

	"github.com/denisvmedia/inventario/ptah/schema/parser"
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// PostgreSQLReader reads schema from PostgreSQL databases
type PostgreSQLReader struct {
	db     *sql.DB
	schema string
}

// NewPostgreSQLReader creates a new PostgreSQL schema reader
func NewPostgreSQLReader(db *sql.DB, schema string) *PostgreSQLReader {
	if schema == "" {
		schema = "public"
	}
	return &PostgreSQLReader{
		db:     db,
		schema: schema,
	}
}

// ReadSchema reads the complete database schema
func (r *PostgreSQLReader) ReadSchema() (*parsertypes.DatabaseSchema, error) {
	schema := &parsertypes.DatabaseSchema{}

	// Read tables
	tables, err := r.readTables()
	if err != nil {
		return nil, fmt.Errorf("failed to read tables: %w", err)
	}
	schema.Tables = tables

	// Read enums
	enums, err := r.readEnums()
	if err != nil {
		return nil, fmt.Errorf("failed to read enums: %w", err)
	}
	schema.Enums = enums

	// Read indexes
	indexes, err := r.readIndexes()
	if err != nil {
		return nil, fmt.Errorf("failed to read indexes: %w", err)
	}
	schema.Indexes = indexes

	// Read constraints
	constraints, err := r.readConstraints()
	if err != nil {
		return nil, fmt.Errorf("failed to read constraints: %w", err)
	}
	schema.Constraints = constraints

	// Enhance tables with constraint information
	r.enhanceTablesWithConstraints(schema.Tables, schema.Constraints)

	return schema, nil
}

// readTables reads all tables and their columns
func (r *PostgreSQLReader) readTables() ([]parsertypes.Table, error) {
	// Read tables
	tablesQuery := `
		SELECT table_name, table_type,
		       COALESCE(obj_description(c.oid), '') as table_comment
		FROM information_schema.tables t
		LEFT JOIN pg_class c ON c.relname = t.table_name
		LEFT JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE t.table_schema = $1 AND (n.nspname = $1 OR n.nspname IS NULL)
		ORDER BY table_name`

	rows, err := r.db.Query(tablesQuery, r.schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []parsertypes.Table
	for rows.Next() {
		var table parsertypes.Table
		err := rows.Scan(&table.Name, &table.Type, &table.Comment)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table: %w", err)
		}

		// Read columns for this table
		columns, err := r.readColumns(table.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to read columns for table %s: %w", table.Name, err)
		}
		table.Columns = columns

		tables = append(tables, table)
	}

	return tables, nil
}

// readColumns reads all columns for a specific table
func (r *PostgreSQLReader) readColumns(tableName string) ([]parsertypes.Column, error) {
	columnsQuery := `
		SELECT
			column_name,
			data_type,
			udt_name,
			is_nullable,
			column_default,
			character_maximum_length,
			numeric_precision,
			numeric_scale,
			ordinal_position
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`

	rows, err := r.db.Query(columnsQuery, r.schema, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []parsertypes.Column
	for rows.Next() {
		var col parsertypes.Column
		err := rows.Scan(
			&col.Name,
			&col.DataType,
			&col.UDTName,
			&col.IsNullable,
			&col.ColumnDefault,
			&col.CharacterMaxLength,
			&col.NumericPrecision,
			&col.NumericScale,
			&col.OrdinalPosition,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}

		// Detect auto increment (SERIAL types)
		if col.ColumnDefault != nil {
			defaultVal := *col.ColumnDefault
			col.IsAutoIncrement = strings.Contains(defaultVal, "nextval(") &&
				strings.Contains(defaultVal, "_seq")
		}

		columns = append(columns, col)
	}

	return columns, nil
}

// readEnums reads all enum types
func (r *PostgreSQLReader) readEnums() ([]parsertypes.Enum, error) {
	enumsQuery := `
		SELECT
			t.typname AS enum_name,
			e.enumlabel AS enum_value,
			e.enumsortorder
		FROM pg_type t
		JOIN pg_enum e ON t.oid = e.enumtypid
		JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = $1
		ORDER BY t.typname, e.enumsortorder`

	rows, err := r.db.Query(enumsQuery, r.schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query enums: %w", err)
	}
	defer rows.Close()

	enumMap := make(map[string][]string)
	for rows.Next() {
		var enumName, enumValue string
		var sortOrder int
		err := rows.Scan(&enumName, &enumValue, &sortOrder)
		if err != nil {
			return nil, fmt.Errorf("failed to scan enum: %w", err)
		}

		enumMap[enumName] = append(enumMap[enumName], enumValue)
	}

	var enums []parsertypes.Enum
	for name, values := range enumMap {
		enums = append(enums, parsertypes.Enum{
			Name:   name,
			Values: values,
		})
	}

	return enums, nil
}

// readIndexes reads all indexes
func (r *PostgreSQLReader) readIndexes() ([]parsertypes.Index, error) {
	indexesQuery := `
		SELECT
			schemaname,
			tablename,
			indexname,
			indexdef
		FROM pg_indexes
		WHERE schemaname = $1
		ORDER BY tablename, indexname`

	rows, err := r.db.Query(indexesQuery, r.schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()

	var indexes []parsertypes.Index
	for rows.Next() {
		var schemaName, tableName, indexName, indexDef string
		err := rows.Scan(&schemaName, &tableName, &indexName, &indexDef)
		if err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}

		// Parse index definition to extract columns and properties
		index := parsertypes.Index{
			Name:       indexName,
			TableName:  tableName,
			Definition: indexDef,
			IsUnique:   strings.Contains(strings.ToUpper(indexDef), "UNIQUE"),
			IsPrimary:  strings.Contains(indexName, "_pkey"),
		}

		// Extract column names from index definition (simplified parsing)
		if strings.Contains(indexDef, "(") && strings.Contains(indexDef, ")") {
			start := strings.Index(indexDef, "(") + 1
			end := strings.LastIndex(indexDef, ")")
			if start < end {
				columnsStr := indexDef[start:end]
				columns := strings.Split(columnsStr, ",")
				for i, col := range columns {
					columns[i] = strings.TrimSpace(col)
				}
				index.Columns = columns
			}
		}

		indexes = append(indexes, index)
	}

	return indexes, nil
}

// readConstraints reads all constraints
func (r *PostgreSQLReader) readConstraints() ([]parsertypes.Constraint, error) {
	constraintsQuery := `
		SELECT
			tc.table_name,
			tc.constraint_name,
			tc.constraint_type,
			COALESCE(kcu.column_name, ''),
			COALESCE(ccu.table_name, ''),
			COALESCE(ccu.column_name, ''),
			COALESCE(rc.delete_rule, ''),
			COALESCE(rc.update_rule, ''),
			COALESCE(cc.check_clause, '')
		FROM information_schema.table_constraints AS tc
		LEFT JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		LEFT JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		LEFT JOIN information_schema.referential_constraints AS rc
			ON tc.constraint_name = rc.constraint_name
			AND tc.table_schema = rc.constraint_schema
		LEFT JOIN information_schema.check_constraints AS cc
			ON tc.constraint_name = cc.constraint_name
			AND tc.table_schema = cc.constraint_schema
		WHERE tc.table_schema = $1
		ORDER BY tc.table_name, tc.constraint_type, tc.constraint_name`

	rows, err := r.db.Query(constraintsQuery, r.schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query constraints: %w", err)
	}
	defer rows.Close()

	var constraints []parsertypes.Constraint
	for rows.Next() {
		var constraint parsertypes.Constraint
		var foreignTable, foreignColumn, deleteRule, updateRule, checkClause string

		err := rows.Scan(
			&constraint.TableName,
			&constraint.Name,
			&constraint.Type,
			&constraint.ColumnName,
			&foreignTable,
			&foreignColumn,
			&deleteRule,
			&updateRule,
			&checkClause,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan constraint: %w", err)
		}

		// Set optional fields
		if foreignTable != "" {
			constraint.ForeignTable = &foreignTable
		}
		if foreignColumn != "" {
			constraint.ForeignColumn = &foreignColumn
		}
		if deleteRule != "" {
			constraint.DeleteRule = &deleteRule
		}
		if updateRule != "" {
			constraint.UpdateRule = &updateRule
		}
		if checkClause != "" {
			constraint.CheckClause = &checkClause
		}

		constraints = append(constraints, constraint)
	}

	return constraints, nil
}

// enhanceTablesWithConstraints adds constraint information to table columns
func (r *PostgreSQLReader) enhanceTablesWithConstraints(tables []parsertypes.Table, constraints []parsertypes.Constraint) {
	// Create maps for quick lookup
	primaryKeys := make(map[string]map[string]bool)
	uniqueKeys := make(map[string]map[string]bool)

	for _, constraint := range constraints {
		if constraint.Type == "PRIMARY KEY" {
			if primaryKeys[constraint.TableName] == nil {
				primaryKeys[constraint.TableName] = make(map[string]bool)
			}
			primaryKeys[constraint.TableName][constraint.ColumnName] = true
		}
		if constraint.Type == "UNIQUE" {
			if uniqueKeys[constraint.TableName] == nil {
				uniqueKeys[constraint.TableName] = make(map[string]bool)
			}
			uniqueKeys[constraint.TableName][constraint.ColumnName] = true
		}
	}

	// Update table columns with constraint information
	for i := range tables {
		for j := range tables[i].Columns {
			col := &tables[i].Columns[j]
			tableName := tables[i].Name

			if primaryKeys[tableName] != nil && primaryKeys[tableName][col.Name] {
				col.IsPrimaryKey = true
			}
			if uniqueKeys[tableName] != nil && uniqueKeys[tableName][col.Name] {
				col.IsUnique = true
			}
		}
	}
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
func (w *PostgreSQLWriter) WriteSchema(result *parsertypes.PackageParseResult) error {
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
	statements := parser.GetOrderedCreateStatements(result, "postgres")
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
func (w *PostgreSQLWriter) writeEnums(enums []types.GlobalEnum) error {
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
func (w *PostgreSQLWriter) DropSchema(result *parsertypes.PackageParseResult) error {
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
func (w *PostgreSQLWriter) CheckSchemaExists(result *parsertypes.PackageParseResult) ([]string, error) {
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
