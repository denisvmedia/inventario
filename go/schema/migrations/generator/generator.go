package generator

import (
	"context"
	"log/slog"
	"os"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"ptah.run/config"
	"ptah.run/core/goschema"
	"ptah.run/dbschema"
	"ptah.run/engine/builtin"
	"ptah.run/migration/generator"
)

type Generator struct {
	dbURL         string
	goEntitiesDir string
	logger        *slog.Logger
}

func New(dbURL, goEntitiesDir string) (*Generator, error) {
	return &Generator{
		dbURL:         dbURL,
		goEntitiesDir: goEntitiesDir,
		logger:        slog.Default(),
	}, nil
}

func (m *Generator) SetLogger(logger *slog.Logger) *Generator {
	tmp := *m
	tmp.logger = logger
	return &tmp
}

// GenerateMigrationFiles generates timestamped migration files using Ptah's native generator
func (m *Generator) GenerateMigrationFiles(ctx context.Context, migrationName, migrationsDir string) (*generator.MigrationFiles, error) {
	m.logger.InfoContext(ctx, "Generating migration files", "schema_dir", m.goEntitiesDir, "migration_name", migrationName)

	// Ensure output directory exists
	err := os.MkdirAll(migrationsDir, 0o750)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create output directory", err)
	}

	// Connect to database first
	conn, err := dbschema.ConnectToDatabase(ctx, m.dbURL)
	if err != nil {
		return nil, errxtrace.Wrap("failed to connect to database", err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			m.logger.ErrorContext(ctx, "Failed to close database connection", "error", err)
		}
	}()

	// The runtime carries the feature services and codecs the generator
	// needs; Ptah requires one. The bundled one is what the ptah CLI uses.
	runtime, err := builtin.New()
	if err != nil {
		return nil, errxtrace.Wrap("failed to assemble the Ptah runtime", err)
	}

	// Use Ptah's native migration generator with database connection
	opts := generator.GenerateMigrationOptions{
		Runtime:       runtime,
		GoEntitiesDir: m.goEntitiesDir,
		DatabaseURL:   m.dbURL,
		DBConn:        conn,
		MigrationName: migrationName,
		OutputDir:     migrationsDir,
		// pg_trgm is provisioned by `inventario db bootstrap`, not declared to Ptah, so
		// it must be ignored here or the diff plans a DROP EXTENSION for it: Ptah treats
		// an undeclared extension present in the database as a removal. Ignoring excludes
		// it from both directions, which is what we want. See models/extensions.go.
		CompareOptions: config.WithAdditionalIgnoredExtensions("pg_trgm"),
	}

	files, err := generator.GenerateMigration(ctx, opts)
	if err != nil {
		return nil, errxtrace.Wrap("failed to generate migration files", err)
	}

	// Check if no migration was needed. Ptah returns a nil result, or one
	// carrying no pairs, when the diff came out empty.
	if files == nil || len(files.Files) == 0 {
		m.logger.InfoContext(ctx, "No schema changes detected - no migration files generated")
		return nil, nil
	}

	for _, pair := range files.Files {
		m.logger.InfoContext(ctx, "Migration files generated", "up_file", pair.UpFile, "down_file", pair.DownFile, "version", pair.Version)
	}

	return files, nil
}

// CheckPendingChanges checks whether there are any pending schema changes without writing
// any migration files. It runs the same diff logic as GenerateMigrationFiles but discards
// the output. Returns true if there are pending changes, false if the schema is in sync.
func (m *Generator) CheckPendingChanges(ctx context.Context) (bool, error) {
	// Use a temporary directory so we never touch the project's migrations folder.
	tmpDir, err := os.MkdirTemp("", "inventool-check-*")
	if err != nil {
		return false, errxtrace.Wrap("failed to create temp dir", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			m.logger.WarnContext(ctx, "Failed to remove temp dir", "path", tmpDir, "error", removeErr)
		}
	}()

	files, err := m.GenerateMigrationFiles(ctx, "check", tmpDir)
	if err != nil {
		return false, err
	}

	return files != nil, nil
}

// GenerateSchemaSQL generates complete schema SQL from Go annotations (for preview)
// TODO: implement
func (m *Generator) GenerateSchemaSQL(ctx context.Context) ([]string, error) {
	m.logger.InfoContext(ctx, "Generating schema SQL", "schema_dir", m.goEntitiesDir)

	annotations, err := builtin.Annotations()
	if err != nil {
		return nil, errxtrace.Wrap("failed to assemble the Ptah annotations", err)
	}

	// Parse Go entities from models directory
	goSchema, err := goschema.ParseDir(annotations, m.goEntitiesDir)
	if err != nil {
		return nil, errxtrace.Wrap("failed to parse Go schema", err)
	}

	m.logger.InfoContext(ctx, "Schema parsed successfully",
		"tables", len(goSchema.Tables),
		"fields", len(goSchema.Fields),
		"indexes", len(goSchema.Indexes),
		"enums", len(goSchema.Enums),
	)

	// For now, return a simple message indicating the schema was parsed
	// The actual SQL generation is handled by Ptah's migration generator
	m.logger.InfoContext(ctx, "Use migration generation for SQL output")
	return []string{"-- Schema parsed successfully"}, nil
}
