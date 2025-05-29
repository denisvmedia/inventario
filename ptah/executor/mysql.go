package executor

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
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
func (r *MySQLReader) ReadSchema() (*DatabaseSchema, error) {
	schema := &DatabaseSchema{}

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
func (r *MySQLReader) readTables(dbName string) ([]Table, error) {
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

	var tables []Table
	for rows.Next() {
		var table Table
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
func (r *MySQLReader) readColumns(dbName, tableName string) ([]Column, error) {
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

	var columns []Column
	for rows.Next() {
		var col Column
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
func (r *MySQLReader) addConstraintInfo(dbName, tableName string, columns []Column) error {
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
func (r *MySQLReader) readEnums(dbName string) ([]Enum, error) {
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

	var enums []Enum
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
		enums = append(enums, Enum{
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
func (r *MySQLReader) readIndexes(dbName string) ([]Index, error) {
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

	var indexes []Index
	for rows.Next() {
		var index Index
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
func (r *MySQLReader) readConstraints(dbName string) ([]Constraint, error) {
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

	var constraints []Constraint
	for rows.Next() {
		var constraint Constraint
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
