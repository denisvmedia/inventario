package postgres

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
)

// PostgreSQLWriter writes schemas to PostgreSQL databases
type PostgreSQLWriter struct {
	db     *sql.DB
	tx     *sql.Tx
	schema string
	dryRun bool
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

// writeEnums creates all enum types
func (w *PostgreSQLWriter) writeEnums(enums []goschema.Enum) error { //nolint:unused // TODO: verify why this is not used
	for _, enum := range enums {
		// Check if enum already exists (skip in dry run mode)
		var exists bool
		if !w.dryRun {
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
				slog.Info("Enum already exists, skipping...", "enumName", enum.Name)
				continue
			}
		}

		// Create enum
		values := make([]string, len(enum.Values))
		for i, v := range enum.Values {
			values[i] = "'" + v + "'"
		}

		createEnumSQL := fmt.Sprintf("CREATE TYPE %s AS ENUM (%s)",
			enum.Name, strings.Join(values, ", "))

		slog.Info("Creating enum...", "enumName", enum.Name)
		if err := w.ExecuteSQL(createEnumSQL); err != nil {
			return fmt.Errorf("failed to create enum %s: %w", enum.Name, err)
		}
	}
	return nil
}

// ExecuteSQL executes a SQL statement
func (w *PostgreSQLWriter) ExecuteSQL(sqlExpr string) error {
	if w.dryRun {
		slog.Info("[DRY RUN] Would execute SQL", "sql", sqlExpr)
		return nil
	}

	if w.tx == nil {
		return fmt.Errorf("no active transaction")
	}

	_, err := w.tx.Exec(sqlExpr)
	if err != nil {
		return fmt.Errorf("SQL execution failed: %w\nSQL: %s", err, sqlExpr)
	}
	return nil
}

// BeginTransaction starts a new transaction
func (w *PostgreSQLWriter) BeginTransaction() error {
	if w.dryRun {
		slog.Info("[DRY RUN] Would begin transaction")
		return nil
	}

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
	if w.dryRun {
		slog.Info("[DRY RUN] Would commit transaction")
		return nil
	}

	if w.tx == nil {
		return fmt.Errorf("no active transaction")
	}

	err := w.tx.Commit()
	w.tx = nil
	return err
}

// RollbackTransaction rolls back the current transaction
func (w *PostgreSQLWriter) RollbackTransaction() error {
	if w.dryRun {
		slog.Info("[DRY RUN] Would rollback transaction")
		return nil
	}

	if w.tx == nil {
		return nil // No transaction to rollback
	}

	err := w.tx.Rollback()
	w.tx = nil
	return err
}

func (w *PostgreSQLWriter) collectAllObjects() (tables []string, enums []string, sequences []string, err error) { //revive:disable-line:function-result-limit // It's acceptable here
	if w.dryRun {
		// In dry run mode, simulate some tables/enums/sequences for demonstration
		tables = []string{"example_table1", "example_table2"}
		enums = []string{"example_enum1", "example_enum2"}
		sequences = []string{"example_table1_id_seq", "example_table2_id_seq"}
		return tables, enums, sequences, nil
	}

	// Get all tables in the schema
	tablesQuery := `
			SELECT table_name
			FROM information_schema.tables
			WHERE table_schema = $1 AND table_type = 'BASE TABLE'
			ORDER BY table_name`

	rows, err := w.db.Query(tablesQuery, w.schema)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
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
		return nil, nil, nil, fmt.Errorf("failed to query enums: %w", err)
	}
	defer enumRows.Close()

	for enumRows.Next() {
		var enumName string
		if err := enumRows.Scan(&enumName); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to scan enum name: %w", err)
		}
		enums = append(enums, enumName)
	}

	// Get all sequences in the schema
	sequencesQuery := `
			SELECT sequence_name
			FROM information_schema.sequences
			WHERE sequence_schema = $1
			ORDER BY sequence_name`

	seqRows, err := w.db.Query(sequencesQuery, w.schema)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to query sequences: %w", err)
	}
	defer seqRows.Close()

	for seqRows.Next() {
		var sequenceName string
		if err := seqRows.Scan(&sequenceName); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to scan sequence name: %w", err)
		}
		sequences = append(sequences, sequenceName)
	}

	return tables, enums, sequences, nil
}

// DropAllTables drops ALL tables and enums in the database schema (COMPLETE CLEANUP!)
func (w *PostgreSQLWriter) DropAllTables() error {
	slog.Warn("WARNING: This will drop ALL tables and enums in the database!")

	// Start transaction
	if err := w.BeginTransaction(); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Rollback on error, commit on success
	defer func() {
		if w.tx != nil {
			w.RollbackTransaction() // TODO: weird - it always rolls back??
		}
	}()

	tables, enums, sequences, err := w.collectAllObjects()
	if err != nil {
		return err
	}

	// Drop all tables with CASCADE to handle dependencies
	for _, tableName := range tables {
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS \"%s\" CASCADE", tableName)
		slog.Info("Dropping table...", "tableName", tableName)
		if err := w.ExecuteSQL(dropSQL); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", tableName, err)
		}
	}

	// Drop all enums
	for _, enumName := range enums {
		dropSQL := fmt.Sprintf("DROP TYPE IF EXISTS \"%s\" CASCADE", enumName)
		slog.Info("Dropping enum...", "enumName", enumName)
		if err := w.ExecuteSQL(dropSQL); err != nil {
			return fmt.Errorf("failed to drop enum %s: %w", enumName, err)
		}
	}

	// Drop all sequences
	for _, sequenceName := range sequences {
		dropSQL := fmt.Sprintf("DROP SEQUENCE IF EXISTS \"%s\" CASCADE", sequenceName)
		slog.Info("Dropping sequence...", "sequenceName", sequenceName)
		if err := w.ExecuteSQL(dropSQL); err != nil {
			return fmt.Errorf("failed to drop sequence %s: %w", sequenceName, err)
		}
	}

	// Commit transaction
	if err := w.CommitTransaction(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("All tables and enums dropped successfully!", "tables", len(tables), "enums", len(enums), "sequences", len(sequences))
	return nil
}

// SetDryRun enables or disables dry run mode
func (w *PostgreSQLWriter) SetDryRun(dryRun bool) {
	w.dryRun = dryRun
}

// IsDryRun returns whether dry run mode is enabled
func (w *PostgreSQLWriter) IsDryRun() bool {
	return w.dryRun
}
