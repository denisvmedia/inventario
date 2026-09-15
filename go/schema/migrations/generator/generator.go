package generator

import (
	"context"
	"log/slog"
	"os"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"ptah.run/config"
	"ptah.run/core/goschema"
	"ptah.run/dbschema"
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
	m.logger.Info("Generating migration files", "schema_dir", m.goEntitiesDir, "migration_name", migrationName)

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
			m.logger.Error("Failed to close database connection", "error", err)
		}
	}()

	// Use Ptah's native migration generator with database connection
	opts := generator.GenerateMigrationOptions{
		GoEntitiesDir: m.goEntitiesDir,
		DatabaseURL:   m.dbURL,
		DBConn:        conn,
		MigrationName: migrationName,
		OutputDir:     migrationsDir,
		// Extensions are provisioned by `inventario db bootstrap`, not declared to Ptah,
		// so they must be ignored here or the diff plans a DROP EXTENSION for each:
		// Ptah treats an undeclared extension present in the database as a removal.
		// Ignoring excludes them from both directions, which is what we want.
		//
		// This list only grows. A name stays here even once the extension is no longer
		// installed on new databases, because older databases still carry it and
		// removing the name is exactly what would hand them a DROP EXTENSION.
		// See models/extensions.go.
		CompareOptions: config.WithAdditionalIgnoredExtensions("btree_gin", "pg_trgm", "pgcrypto"),
	}

	files, err := generator.GenerateMigration(ctx, opts)
	if err != nil {
		return nil, errxtrace.Wrap("failed to generate migration files", err)
	}

	// Check if no migration was needed. Ptah returns a nil result, or one
	// carrying no pairs, when the diff came out empty.
	if files == nil || len(files.Files) == 0 {
		m.logger.Info("No schema changes detected - no migration files generated")
		return nil, nil
	}

	for _, pair := range files.Files {
		m.logger.Info("Migration files generated", "up_file", pair.UpFile, "down_file", pair.DownFile, "version", pair.Version)
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
			m.logger.Warn("Failed to remove temp dir", "path", tmpDir, "error", removeErr)
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
	m.logger.Info("Generating schema SQL", "schema_dir", m.goEntitiesDir)

	// Parse Go entities from models directory
	goSchema, err := goschema.ParseDir(m.goEntitiesDir)
	if err != nil {
		return nil, errxtrace.Wrap("failed to parse Go schema", err)
	}

	m.logger.Info("Schema parsed successfully",
		"tables", len(goSchema.Tables),
		"fields", len(goSchema.Fields),
		"indexes", len(goSchema.Indexes),
		"enums", len(goSchema.Enums),
		"extensions", len(goSchema.Extensions),
	)

	// For now, return a simple message indicating the schema was parsed
	// The actual SQL generation is handled by Ptah's migration generator
	m.logger.Info("Use migration generation for SQL output")
	return []string{"-- Schema parsed successfully"}, nil
}
