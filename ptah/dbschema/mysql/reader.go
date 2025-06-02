package mysql

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	coreparser "github.com/denisvmedia/inventario/ptah/core/parser"
	"github.com/denisvmedia/inventario/ptah/dbschema/types"
)

// Reader reads schema information from MySQL/MariaDB databases
type Reader struct {
	db     *sql.DB
	schema string
}

// NewMySQLReader creates a new MySQL schema reader
func NewMySQLReader(db *sql.DB, schema string) *Reader {
	if schema == "" {
		schema = "information_schema"
	}
	return &Reader{
		db:     db,
		schema: schema,
	}
}

// ReadSchema reads the complete schema from MySQL/MariaDB
func (r *Reader) ReadSchema() (*types.DBSchema, error) {
	schema := &types.DBSchema{}

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

// readTables reads all tables and their columns using SHOW CREATE TABLE and DDL parsing
func (r *Reader) readTables(dbName string) ([]types.DBTable, error) {
	// First, get just the table names
	tableNames, err := r.getTableNames(dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to get table names: %w", err)
	}

	var tables []types.DBTable
	for _, tableName := range tableNames {
		// Get DDL for this table using SHOW CREATE TABLE
		ddl, err := r.getTableDDL(tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get DDL for table %s: %w", tableName, err)
		}

		// Parse DDL using core parser
		table, err := r.parseTableFromDDL(ddl)
		if err != nil {
			return nil, fmt.Errorf("failed to parse DDL for table %s: %w", tableName, err)
		}

		tables = append(tables, table)
	}

	return tables, nil
}

// getTableNames fetches just the table names from the database
func (r *Reader) getTableNames(dbName string) ([]string, error) {
	query := `
		SELECT TABLE_NAME
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ?
		AND TABLE_TYPE IN ('BASE TABLE', 'VIEW')
		AND TABLE_NAME NOT IN ('schema_migrations')
		ORDER BY TABLE_NAME`

	rows, err := r.db.Query(query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, tableName)
	}

	return tableNames, nil
}

// getTableDDL gets the DDL for a specific table using SHOW CREATE TABLE
func (r *Reader) getTableDDL(tableName string) (string, error) {
	query := fmt.Sprintf("SHOW CREATE TABLE `%s`", tableName)

	var name, ddl string
	err := r.db.QueryRow(query).Scan(&name, &ddl)
	if err != nil {
		return "", fmt.Errorf("failed to get DDL for table %s: %w", tableName, err)
	}

	return ddl, nil
}

// parseTableFromDDL parses a table DDL using the core parser and converts to parsertypes.DBTable
func (r *Reader) parseTableFromDDL(ddl string) (types.DBTable, error) {
	// Parse DDL using core parser
	parser := coreparser.NewParser(ddl)
	statements, err := parser.Parse()
	if err != nil {
		return types.DBTable{}, fmt.Errorf("failed to parse DDL: %w", err)
	}

	if len(statements.Statements) == 0 {
		return types.DBTable{}, fmt.Errorf("no statements found in DDL")
	}

	// Should be a CREATE TABLE statement
	createTableNode, ok := statements.Statements[0].(*ast.CreateTableNode)
	if !ok {
		return types.DBTable{}, fmt.Errorf("expected CREATE TABLE statement, got %T", statements.Statements[0])
	}

	// Convert AST to parsertypes.DBTable
	return r.convertASTToTable(createTableNode), nil
}

func (r *Reader) applyColumnConstraint(table *types.DBTable, constraint *ast.ConstraintNode, primaryKeyColumns map[string]bool) {
	switch constraint.Type {
	case ast.PrimaryKeyConstraint:
		// Mark columns as primary key
		for _, colName := range constraint.Columns {
			colName = strings.Trim(colName, "`")
			for i := range table.Columns {
				if table.Columns[i].Name == colName {
					table.Columns[i].IsPrimaryKey = true
				}
			}
		}
	case ast.UniqueConstraint:
		// Mark columns as unique (only if single column unique constraint)
		// BUT skip if this column is already a primary key (primary keys are inherently unique)
		if len(constraint.Columns) == 1 {
			colName := strings.Trim(constraint.Columns[0], "`")
			if !primaryKeyColumns[colName] {
				for i := range table.Columns {
					if table.Columns[i].Name == colName {
						table.Columns[i].IsUnique = true
					}
				}
			}
		}
	}
}

// convertASTToTable converts an AST CreateTableNode to parsertypes.DBTable
func (r *Reader) convertASTToTable(node *ast.CreateTableNode) types.DBTable {
	table := types.DBTable{
		Name:    strings.Trim(node.Name, "`"), // Remove backticks
		Type:    "BASE TABLE",                 // Default for regular tables
		Comment: "",                           // Will be extracted from options if present
	}

	// Convert columns
	for _, astCol := range node.Columns {
		// Convert boolean nullable to string format expected by parsertypes
		isNullable := "YES"
		if !astCol.Nullable {
			isNullable = "NO"
		}

		col := types.DBColumn{
			Name:            strings.Trim(astCol.Name, "`"),
			DataType:        astCol.Type,
			ColumnType:      astCol.Type, // For MySQL, these are often the same
			IsNullable:      isNullable,
			OrdinalPosition: len(table.Columns) + 1,
			IsAutoIncrement: astCol.AutoInc,
			IsPrimaryKey:    astCol.Primary,
			IsUnique:        astCol.Unique,
		}

		// Handle default values
		if astCol.Default != nil {
			if astCol.Default.Expression != "" {
				col.ColumnDefault = &astCol.Default.Expression
			} else {
				col.ColumnDefault = &astCol.Default.Value
			}
		}

		// Handle character length, precision, scale if available
		// These would need to be parsed from the type string for MySQL
		// For now, we'll leave them as nil since the AST doesn't provide them directly

		table.Columns = append(table.Columns, col)
	}

	// Handle table-level constraints to update column flags
	primaryKeyColumns := make(map[string]bool)

	// First pass: identify primary key columns
	for _, constraint := range node.Constraints {
		if constraint.Type != ast.PrimaryKeyConstraint {
			continue
		}
		for _, colName := range constraint.Columns {
			colName = strings.Trim(colName, "`")
			primaryKeyColumns[colName] = true
		}
	}

	// Second pass: apply constraints
	for _, constraint := range node.Constraints {
		r.applyColumnConstraint(&table, constraint, primaryKeyColumns)
	}

	// Extract table comment from options
	for key, value := range node.Options {
		if strings.ToUpper(key) == "COMMENT" {
			table.Comment = strings.Trim(value, "'\"")
		}
	}

	return table
}

// readEnums reads enum types from MySQL (stored as column types)
func (r *Reader) readEnums(dbName string) ([]types.DBEnum, error) {
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

	var enums []types.DBEnum
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
		enums = append(enums, types.DBEnum{
			Name:   name,
			Values: values,
		})
	}

	return enums, nil
}

// readIndexes reads all indexes
func (r *Reader) readIndexes(dbName string) ([]types.DBIndex, error) {
	query := `
		SELECT
			s.INDEX_NAME,
			s.TABLE_NAME,
			GROUP_CONCAT(s.COLUMN_NAME ORDER BY s.SEQ_IN_INDEX) as COLUMNS,
			s.NON_UNIQUE,
			s.INDEX_TYPE
		FROM information_schema.STATISTICS s
		WHERE s.TABLE_SCHEMA = ?
		AND s.TABLE_NAME NOT IN ('schema_migrations')
		GROUP BY s.INDEX_NAME, s.TABLE_NAME, s.NON_UNIQUE, s.INDEX_TYPE
		ORDER BY s.TABLE_NAME, s.INDEX_NAME`

	rows, err := r.db.Query(query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []types.DBIndex
	for rows.Next() {
		var index types.DBIndex
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
func (r *Reader) readConstraints(dbName string) ([]types.DBConstraint, error) {
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
		AND tc.TABLE_NAME NOT IN ('schema_migrations')
		ORDER BY tc.TABLE_NAME, tc.CONSTRAINT_NAME`

	rows, err := r.db.Query(query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints []types.DBConstraint
	for rows.Next() {
		var constraint types.DBConstraint
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
