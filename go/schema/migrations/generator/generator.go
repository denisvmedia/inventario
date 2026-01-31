package generator

import (
	"context"
	"log/slog"
	"os"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/stokaro/ptah/config"
	"github.com/stokaro/ptah/core/goschema"
	"github.com/stokaro/ptah/dbschema"
	"github.com/stokaro/ptah/migration/generator"
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
	conn, err := dbschema.ConnectToDatabase(m.dbURL)
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
		GoEntitiesDir:  m.goEntitiesDir,
		DatabaseURL:    m.dbURL,
		DBConn:         conn,
		MigrationName:  migrationName,
		OutputDir:      migrationsDir,
		CompareOptions: config.WithAdditionalIgnoredExtensions("btree_gin", "pg_trgm"),
	}

	files, err := generator.GenerateMigration(opts)
	if err != nil {
		return nil, errxtrace.Wrap("failed to generate migration files", err)
	}

	// Check if no migration was needed (files will be nil when no changes detected)
	if files == nil {
		m.logger.Info("No schema changes detected - no migration files generated")
		return nil, nil
	}

	m.logger.Info("Migration files generated", "up_file", files.UpFile, "down_file", files.DownFile, "version", files.Version)

	return files, nil
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
