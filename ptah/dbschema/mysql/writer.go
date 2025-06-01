package mysql

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
)

// Writer writes schemas to MySQL/MariaDB databases
type Writer struct {
	db     *sql.DB
	tx     *sql.Tx
	schema string
	dryRun bool
}

// NewMySQLWriter creates a new MySQL schema writer
func NewMySQLWriter(db *sql.DB, schema string) *Writer {
	return &Writer{
		db:     db,
		schema: schema,
	}
}

// ExecuteSQL executes a SQL statement
func (w *Writer) ExecuteSQL(sql string) error {
	if w.dryRun {
		slog.Info("[DRY RUN] Would execute SQL", "sql", sql)
		return nil
	}

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
func (w *Writer) BeginTransaction() error {
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
func (w *Writer) CommitTransaction() error {
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
func (w *Writer) RollbackTransaction() error {
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

// DropAllTables drops ALL tables in the database (COMPLETE CLEANUP!)
func (w *Writer) DropAllTables() error {
	slog.Info("WARNING: This will drop ALL tables in the database!")

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

	var tables []string

	if w.dryRun {
		// In dry run mode, simulate some tables for demonstration
		tables = []string{"example_table1", "example_table2", "example_table3"}
	} else {
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

		for rows.Next() {
			var tableName string
			if err := rows.Scan(&tableName); err != nil {
				return fmt.Errorf("failed to scan table name: %w", err)
			}
			tables = append(tables, tableName)
		}
	}

	// Drop all tables
	for _, tableName := range tables {
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName)
		slog.Info("Dropping table", "tableName", tableName)
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

	slog.Info("Successfully dropped tables", "count", len(tables))
	return nil
}

// isCreateTableStatement checks if a SQL statement is a CREATE TABLE statement
func (w *Writer) isCreateTableStatement(sql string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "CREATE TABLE")
}

// isCreateIndexStatement checks if a SQL statement is a CREATE INDEX statement
func (w *Writer) isCreateIndexStatement(sql string) bool {
	return strings.Contains(strings.ToUpper(strings.TrimSpace(sql)), "CREATE") &&
		strings.Contains(strings.ToUpper(strings.TrimSpace(sql)), "INDEX")
}

// extractTableNameFromCreateTable extracts table name from CREATE TABLE statement
func (w *Writer) extractTableNameFromCreateTable(sql string) string {
	// Simple regex to extract table name from "CREATE TABLE tablename ("
	parts := strings.Fields(strings.TrimSpace(sql))
	if len(parts) >= 3 && strings.ToUpper(parts[0]) == "CREATE" && strings.ToUpper(parts[1]) == "TABLE" {
		return strings.TrimSuffix(parts[2], "(")
	}
	return ""
}

// extractTableNameFromCreateIndex extracts table name from CREATE INDEX statement
func (w *Writer) extractTableNameFromCreateIndex(sql string) string {
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
func (w *Writer) tableExists(tableName string) bool { //nolint:unused // TODO: verify why this is not used
	if w.dryRun {
		// In dry run mode, assume table doesn't exist to show all operations
		return false
	}

	var exists bool
	checkSQL := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = DATABASE() AND table_name = ?
		)`

	err := w.db.QueryRow(checkSQL, tableName).Scan(&exists)
	return err == nil && exists
}

// SetDryRun enables or disables dry run mode
func (w *Writer) SetDryRun(dryRun bool) {
	w.dryRun = dryRun
}

// IsDryRun returns whether dry run mode is enabled
func (w *Writer) IsDryRun() bool {
	return w.dryRun
}
