package bootstrap

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"text/template"

	"github.com/jackc/pgx/v5"

	"github.com/denisvmedia/inventario/internal/errkit"
)

//go:embed _sqldata/*.sql
var bootstrapFS embed.FS

// TemplateData holds the template variables for SQL migrations
type TemplateData struct {
	Username              string
	UsernameForMigrations string
}

// ApplyArgs contains arguments for applying bootstrap migrations
type ApplyArgs struct {
	DSN      string
	Template TemplateData
	DryRun   bool
}

// Migrator handles bootstrap database migrations
type Migrator struct {
	logger *slog.Logger
}

// New creates a new bootstrap migrator
func New() *Migrator {
	return &Migrator{
		logger: slog.Default(),
	}
}

// Apply executes all bootstrap migrations in alphabetical order
func (m *Migrator) Apply(ctx context.Context, args ApplyArgs) error {
	if args.DSN == "" {
		return fmt.Errorf("database DSN is required")
	}

	// Validate that this is a PostgreSQL DSN
	if !strings.HasPrefix(args.DSN, "postgres://") && !strings.HasPrefix(args.DSN, "postgresql://") {
		return fmt.Errorf("bootstrap migrations only support PostgreSQL databases")
	}

	// Get all SQL files from embedded filesystem
	files, err := m.getSQLFiles()
	if err != nil {
		return errkit.Wrap(err, "failed to read bootstrap SQL files")
	}

	if len(files) == 0 {
		m.logger.Info("No bootstrap migration files found")
		return nil
	}

	m.logger.Info("Found bootstrap migration files", "count", len(files), "files", files)

	if args.DryRun {
		return m.dryRun(files, args.Template)
	}

	// Connect to database
	conn, err := pgx.Connect(ctx, args.DSN)
	if err != nil {
		return errkit.Wrap(err, "failed to connect to database")
	}
	defer func() {
		if closeErr := conn.Close(ctx); closeErr != nil {
			m.logger.Error("Failed to close database connection", "error", closeErr)
		}
	}()

	// Apply each migration file
	for _, filename := range files {
		if err := m.applyMigrationFile(ctx, conn, filename, args.Template); err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to apply migration file %s", filename))
		}
	}

	m.logger.Info("✅ All bootstrap migrations applied successfully")
	return nil
}

// getSQLFiles returns all SQL files from the embedded filesystem in alphabetical order
func (m *Migrator) getSQLFiles() ([]string, error) {
	entries, err := bootstrapFS.ReadDir("_sqldata")
	if err != nil {
		return nil, errkit.Wrap(err, "failed to read bootstrap directory")
	}

	var sqlFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			sqlFiles = append(sqlFiles, entry.Name())
		}
	}

	// Sort files alphabetically to ensure consistent execution order
	sort.Strings(sqlFiles)

	return sqlFiles, nil
}

// dryRun shows what would be executed without actually running the migrations
func (m *Migrator) dryRun(files []string, templateData TemplateData) error {
	m.logger.Info("🔍 [DRY RUN] Bootstrap migrations preview")
	m.logger.Info("📋 Template variables", "username", templateData.Username, "usernameForMigrations", templateData.UsernameForMigrations)

	for i, filename := range files {
		m.logger.Info(fmt.Sprintf("📄 [%d/%d] Would apply: %s", i+1, len(files), filename))

		// Read and process the file content
		content, err := m.readAndProcessFile(filename, templateData)
		if err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to process file %s", filename))
		}

		// Show a preview of the processed content (first few lines)
		lines := strings.Split(content, "\n")
		previewLines := 5
		if len(lines) < previewLines {
			previewLines = len(lines)
		}

		m.logger.Info("📝 Preview (first few lines):")
		for j := 0; j < previewLines; j++ {
			if strings.TrimSpace(lines[j]) != "" {
				m.logger.Info(fmt.Sprintf("    %s", lines[j]))
			}
		}
		if len(lines) > previewLines {
			m.logger.Info(fmt.Sprintf("    ... (%d more lines)", len(lines)-previewLines))
		}
	}

	m.logger.Info("✅ [DRY RUN] Preview completed successfully")
	return nil
}

// applyMigrationFile applies a single migration file
func (m *Migrator) applyMigrationFile(ctx context.Context, conn *pgx.Conn, filename string, templateData TemplateData) error {
	m.logger.Info("📄 Applying migration file", "filename", filename)

	// Read and process the file content
	content, err := m.readAndProcessFile(filename, templateData)
	if err != nil {
		return errkit.Wrap(err, "failed to read and process file")
	}

	// Execute the SQL content in a transaction
	tx, err := conn.Begin(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && rollbackErr != pgx.ErrTxClosed {
			m.logger.Error("Failed to rollback transaction", "error", rollbackErr)
		}
	}()

	// Execute the SQL
	_, err = tx.Exec(ctx, content)
	if err != nil {
		return errkit.Wrap(err, "failed to execute SQL")
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return errkit.Wrap(err, "failed to commit transaction")
	}

	m.logger.Info("✅ Migration file applied successfully", "filename", filename)
	return nil
}

// readAndProcessFile reads a SQL file and processes template variables
func (m *Migrator) readAndProcessFile(filename string, templateData TemplateData) (string, error) {
	// Read the file content
	content, err := bootstrapFS.ReadFile(fmt.Sprintf("_sqldata/%s", filename))
	if err != nil {
		return "", errkit.Wrap(err, "failed to read file")
	}

	// Process template variables
	tmpl, err := template.New(filename).Parse(string(content))
	if err != nil {
		return "", errkit.Wrap(err, "failed to parse template")
	}

	var processed strings.Builder
	if err := tmpl.Execute(&processed, templateData); err != nil {
		return "", errkit.Wrap(err, "failed to execute template")
	}

	return processed.String(), nil
}

// Print outputs all bootstrap migrations with template variables resolved
func (m *Migrator) Print(templateData TemplateData) error {
	// Get all SQL files from embedded filesystem
	files, err := m.getSQLFiles()
	if err != nil {
		return errkit.Wrap(err, "failed to read bootstrap SQL files")
	}

	if len(files) == 0 {
		m.logger.Info("No bootstrap migration files found")
		return nil
	}

	// Process and print each migration file
	for i, filename := range files {
		if i > 0 {
			fmt.Println() // Add blank line between files
		}

		fmt.Printf("-- ========================================\n")
		fmt.Printf("-- Bootstrap Migration File: %s\n", filename)
		fmt.Printf("-- ========================================\n\n")

		// Read and process the file content
		content, err := m.readAndProcessFile(filename, templateData)
		if err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to process file %s", filename))
		}

		// Print the processed content
		fmt.Print(content)

		// Ensure content ends with newline
		if !strings.HasSuffix(content, "\n") {
			fmt.Println()
		}
	}

	return nil
}
