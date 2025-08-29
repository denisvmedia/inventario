package bootstrap

import (
	"context"
	"embed"
	"fmt"
	"io"
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
	Username                    string
	UsernameForMigrations       string
	UsernameForBackgroundWorker string
}

// ApplyArgs contains arguments for applying bootstrap migrations
type ApplyArgs struct {
	DSN      string
	Template TemplateData
	DryRun   bool
}

// Migrator handles bootstrap database migrations
type Migrator struct {
	w io.Writer
}

// New creates a new bootstrap migrator
func New() *Migrator {
	return &Migrator{
		w: io.Discard,
	}
}

// WithWriter sets the output writer for logging
func (m *Migrator) WithWriter(w io.Writer) *Migrator {
	tmp := *m
	tmp.w = w
	return &tmp
}

// Apply executes all bootstrap migrations in alphabetical order
func (m *Migrator) Apply(ctx context.Context, args ApplyArgs) error {
	if args.DSN == "" {
		return fmt.Errorf("database DSN is required")
	}

	// Validate that this is a PostgreSQL DSN
	if !strings.HasPrefix(args.DSN, "postgres://") && !strings.HasPrefix(args.DSN, "postgresql://") {
		return fmt.Errorf("migrator: bootstrap migrations only support PostgreSQL databases")
	}

	// Get all SQL files from embedded filesystem
	files, err := m.getSQLFiles()
	if err != nil {
		return errkit.Wrap(err, "failed to read bootstrap SQL files")
	}

	if len(files) == 0 {
		fmt.Fprintln(m.w, "No bootstrap migration files found")
		return nil
	}

	fmt.Fprintf(m.w, "Found bootstrap migration files: %v\n", files)

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
			fmt.Fprintf(m.w, "Failed to close database connection: %v\n", closeErr)
		}
	}()

	// Apply each migration file
	for _, filename := range files {
		if err := m.applyMigrationFile(ctx, conn, filename, args.Template); err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to apply migration file %s", filename))
		}
	}

	fmt.Fprintf(m.w, "✅ All bootstrap migrations applied successfully\n")
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
	fmt.Fprintln(m.w, "🔍 [DRY RUN] Bootstrap migrations preview")
	fmt.Fprintf(m.w, "📋 Template variables: %+v\n", templateData)

	for i, filename := range files {
		fmt.Fprintf(m.w, "📄 [%d/%d] Would apply: %s\n", i+1, len(files), filename)

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

		fmt.Fprintln(m.w, "📝 Preview (first few lines):")
		for j := 0; j < previewLines; j++ {
			if strings.TrimSpace(lines[j]) != "" {
				fmt.Fprintf(m.w, "    %s\n", lines[j])
			}
		}
		if len(lines) > previewLines {
			fmt.Fprintf(m.w, "    ... (%d more lines)\n", len(lines)-previewLines)
		}
	}

	fmt.Fprintln(m.w, "✅ [DRY RUN] Preview completed successfully")
	return nil
}

// applyMigrationFile applies a single migration file
func (m *Migrator) applyMigrationFile(ctx context.Context, conn *pgx.Conn, filename string, templateData TemplateData) error {
	fmt.Fprintln(m.w, "📄 Applying migration file", filename)

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
			fmt.Fprintf(m.w, "Failed to rollback transaction: %v\n", rollbackErr)
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

	fmt.Fprintln(m.w, "✅ Migration file applied successfully", filename)
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
		fmt.Fprintln(m.w, "No bootstrap migration files found")
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
