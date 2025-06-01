package postgres

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/dbschema/types"
)

// Reader reads schema from PostgreSQL databases
type Reader struct {
	db     *sql.DB
	schema string
}

// NewPostgreSQLReader creates a new PostgreSQL schema reader
func NewPostgreSQLReader(db *sql.DB, schema string) *Reader {
	if schema == "" {
		schema = "public"
	}
	return &Reader{
		db:     db,
		schema: schema,
	}
}

// ReadSchema reads the complete database schema
func (r *Reader) ReadSchema() (*types.DBSchema, error) {
	schema := &types.DBSchema{}

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

	// Enhance tables with primary key information from indexes
	r.enhanceTablesWithIndexes(schema.Tables, schema.Indexes)

	return schema, nil
}

// readTables reads all tables and their columns
func (r *Reader) readTables() ([]types.DBTable, error) {
	// Read tables, excluding system tables like schema_migrations
	tablesQuery := `
		SELECT table_name, table_type,
		       COALESCE(obj_description(c.oid), '') as table_comment
		FROM information_schema.tables t
		LEFT JOIN pg_class c ON c.relname = t.table_name
		LEFT JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE t.table_schema = $1 AND (n.nspname = $1 OR n.nspname IS NULL)
		AND t.table_name NOT IN ('schema_migrations')
		ORDER BY table_name`

	rows, err := r.db.Query(tablesQuery, r.schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []types.DBTable
	for rows.Next() {
		var table types.DBTable
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
func (r *Reader) readColumns(tableName string) ([]types.DBColumn, error) {
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

	var columns []types.DBColumn
	for rows.Next() {
		var col types.DBColumn
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
func (r *Reader) readEnums() ([]types.DBEnum, error) {
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

	var enums []types.DBEnum
	for name, values := range enumMap {
		enums = append(enums, types.DBEnum{
			Name:   name,
			Values: values,
		})
	}

	return enums, nil
}

// readIndexes reads all indexes
func (r *Reader) readIndexes() ([]types.DBIndex, error) {
	indexesQuery := `
		SELECT
			n.nspname as schemaname,
			t.relname as tablename,
			i.relname as indexname,
			pg_get_indexdef(i.oid) as indexdef,
			ix.indisprimary,
			ix.indisunique
		FROM pg_index ix
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = $1
		AND t.relname NOT IN ('schema_migrations')
		ORDER BY t.relname, i.relname`

	rows, err := r.db.Query(indexesQuery, r.schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()

	var indexes []types.DBIndex
	for rows.Next() {
		var schemaName, tableName, indexName, indexDef string
		var isPrimary, isUnique bool
		err := rows.Scan(&schemaName, &tableName, &indexName, &indexDef, &isPrimary, &isUnique)
		if err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}

		// Parse index definition to extract columns and properties
		index := types.DBIndex{
			Name:       indexName,
			TableName:  tableName,
			Definition: indexDef,
			IsUnique:   isUnique,
			IsPrimary:  isPrimary,
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
func (r *Reader) readConstraints() ([]types.DBConstraint, error) {
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
			AND tc.table_name = kcu.table_name
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
		AND tc.table_name NOT IN ('schema_migrations')
		ORDER BY tc.table_name, tc.constraint_type, tc.constraint_name`

	rows, err := r.db.Query(constraintsQuery, r.schema)
	if err != nil {
		return nil, fmt.Errorf("failed to query constraints: %w", err)
	}
	defer rows.Close()

	var constraints []types.DBConstraint
	for rows.Next() {
		var constraint types.DBConstraint
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
func (r *Reader) enhanceTablesWithConstraints(tables []types.DBTable, constraints []types.DBConstraint) {
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

// enhanceTablesWithIndexes adds primary key information from indexes
func (r *Reader) enhanceTablesWithIndexes(tables []types.DBTable, indexes []types.DBIndex) {
	// For auto-increment integer columns (originally SERIAL), automatically set them as primary keys
	// This is a PostgreSQL-specific behavior where SERIAL columns become auto-increment integers and are typically primary keys
	for i := range tables {
		for j := range tables[i].Columns {
			col := &tables[i].Columns[j]

			// If it's an auto-increment integer column, assume it's a primary key
			// PostgreSQL converts SERIAL to integer with auto-increment
			if col.IsAutoIncrement && (strings.Contains(strings.ToLower(col.DataType), "int") ||
				strings.Contains(strings.ToLower(col.UDTName), "int")) {
				col.IsPrimaryKey = true
			}
		}
	}
}
