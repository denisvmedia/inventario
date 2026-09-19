package bootstrap

import (
	"context"
	"embed"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/template"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jackc/pgx/v5"

	"go.5x5.cz/inventario/schema/dsnutil"
)

//go:embed _sqldata/*.sql
var bootstrapFS embed.FS

// TemplateData holds the template variables for SQL migrations
type TemplateData struct {
	Username                    string
	UsernameForMigrations       string
	UsernameForBackgroundWorker string
}

// Service roles that 001_initial.sql creates and that the RLS policies are
// attached to. The application logs in as an ordinary user and reaches these
// with SET LOCAL ROLE, per operation.
const (
	roleApp        = "inventario_app"
	roleMigrator   = "inventario_migrator"
	roleBackground = "inventario_background_worker"
	roleAdmin      = "inventario_admin"
)

// A login named after a service role collapses into that role: CREATE USER runs
// first, so the later CREATE ROLE finds the name taken and skips, leaving one
// role that is both the login and the policy target.
//
// For the operational login that is an isolation failure. The app calls
// SET LOCAL ROLE inventario_app to drop to the tenant-scoped policies, but
// SET ROLE only narrows privileges when the target is a *different* role than
// the one holding the grants. A login already named inventario_app keeps every
// role granted to it — including inventario_background_worker, whose policies
// are USING (true) — and the tenant-isolation policy ORs with them. The switch
// becomes a no-op and every tenant's rows are visible.
//
// inventario_migrator is exempt for the migration login: that connection never
// switches roles and never serves user traffic, and every deployment manifest
// in this repo already names it that way.
var reservedNames = map[string][]string{
	"username":                       {roleApp, roleMigrator, roleBackground, roleAdmin},
	"username-for-migrations":        {roleApp, roleBackground, roleAdmin},
	"username-for-background-worker": {roleApp, roleMigrator, roleAdmin},
}

// ErrReservedLoginName is returned by Apply when a login name collides with one
// of the service roles bootstrap manages.
var ErrReservedLoginName = errx.NewSentinel("login name collides with a managed service role")

// Validate rejects login names that collide with a managed service role.
func (t TemplateData) Validate() error {
	for _, f := range []struct {
		flag, value string
	}{
		{"username", t.Username},
		{"username-for-migrations", t.UsernameForMigrations},
		{"username-for-background-worker", t.UsernameForBackgroundWorker},
	} {
		for _, reserved := range reservedNames[f.flag] {
			// Case-insensitively: the template interpolates the name unquoted, so
			// PostgreSQL folds INVENTARIO_APP to inventario_app and the collision
			// happens anyway. The template's own guards compare SQL string
			// literals, which do not fold, so they wave the spelling through.
			if !strings.EqualFold(f.value, reserved) {
				continue
			}
			return errxtrace.Wrap(
				fmt.Sprintf("--%s=%s: pick a plain login name such as `inventario`; "+
					"bootstrap grants it the service roles it needs",
					f.flag, f.value),
				ErrReservedLoginName,
			)
		}
	}
	return nil
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

	// Before the dry-run branch: --dry-run has to report the collision too.
	if err := args.Template.Validate(); err != nil {
		return err
	}

	// Get all SQL files from embedded filesystem
	files, err := m.getSQLFiles()
	if err != nil {
		return errxtrace.Wrap("failed to read bootstrap SQL files", err)
	}

	if len(files) == 0 {
		fmt.Fprintln(m.w, "No bootstrap migration files found")
		return nil
	}

	fmt.Fprintf(m.w, "Found bootstrap migration files: %v\n", files)

	if args.DryRun {
		return m.dryRun(files, args.Template)
	}

	// Connect to database. pgx.Connect (unlike pgxpool.ParseConfig) does not
	// strip pool_* keys from the DSN, so callers passing a pgxpool-shaped DSN
	// (e.g. POSTGRES_TEST_DSN) would otherwise have those params forwarded
	// to the server as unknown startup parameters. See dsnutil.StripPGXPoolParams.
	conn, err := pgx.Connect(ctx, dsnutil.StripPGXPoolParams(args.DSN))
	if err != nil {
		return errxtrace.Wrap("failed to connect to database", err)
	}
	defer func() {
		if closeErr := conn.Close(ctx); closeErr != nil {
			fmt.Fprintf(m.w, "Failed to close database connection: %v\n", closeErr)
		}
	}()

	// Apply each migration file
	for _, filename := range files {
		if err := m.applyMigrationFile(ctx, conn, filename, args.Template); err != nil {
			return errxtrace.Wrap("failed to apply migration file", err, errx.Attrs("filename", filename))
		}
	}

	fmt.Fprintf(m.w, "✅ All bootstrap migrations applied successfully\n")
	return nil
}

// getSQLFiles returns all SQL files from the embedded filesystem in alphabetical order
func (m *Migrator) getSQLFiles() ([]string, error) {
	entries, err := bootstrapFS.ReadDir("_sqldata")
	if err != nil {
		return nil, errxtrace.Wrap("failed to read bootstrap directory", err)
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
			return errxtrace.Wrap("failed to process file", err, errx.Attrs("filename", filename))
		}

		// Show a preview of the processed content (first few lines)
		lines := strings.Split(content, "\n")
		previewLines := min(len(lines), 5)

		fmt.Fprintln(m.w, "📝 Preview (first few lines):")
		for j := range previewLines {
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
		return errxtrace.Wrap("failed to read and process file", err)
	}

	// Execute the SQL content in a transaction
	tx, err := conn.Begin(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && rollbackErr != pgx.ErrTxClosed {
			fmt.Fprintf(m.w, "Failed to rollback transaction: %v\n", rollbackErr)
		}
	}()

	// Execute the SQL
	_, err = tx.Exec(ctx, content)
	if err != nil {
		return errxtrace.Wrap("failed to execute SQL", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return errxtrace.Wrap("failed to commit transaction", err)
	}

	fmt.Fprintln(m.w, "✅ Migration file applied successfully", filename)
	return nil
}

// readAndProcessFile reads a SQL file and processes template variables
func (m *Migrator) readAndProcessFile(filename string, templateData TemplateData) (string, error) {
	// Read the file content
	content, err := bootstrapFS.ReadFile(fmt.Sprintf("_sqldata/%s", filename))
	if err != nil {
		return "", errxtrace.Wrap("failed to read file", err)
	}

	// Process template variables
	tmpl, err := template.New(filename).Parse(string(content))
	if err != nil {
		return "", errxtrace.Wrap("failed to parse template", err)
	}

	var processed strings.Builder
	if err := tmpl.Execute(&processed, templateData); err != nil {
		return "", errxtrace.Wrap("failed to execute template", err)
	}

	return processed.String(), nil
}

// Print outputs all bootstrap migrations with template variables resolved
func (m *Migrator) Print(templateData TemplateData) error {
	// The printed SQL is meant to be handed to a DBA, so a collision has to
	// surface here rather than in the statements they end up running.
	if err := templateData.Validate(); err != nil {
		return err
	}

	// Get all SQL files from embedded filesystem
	files, err := m.getSQLFiles()
	if err != nil {
		return errxtrace.Wrap("failed to read bootstrap SQL files", err)
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
			return errxtrace.Wrap("failed to process file", err, errx.Attrs("filename", filename))
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
