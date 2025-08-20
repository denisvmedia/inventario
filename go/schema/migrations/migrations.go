package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// Embed migration files into the binary
// Note: This embeds all files in the migrations/source directory
//
//go:embed _sqldata/*
var embeddedMigrations embed.FS

// MigrationInfo contains information about an embedded migration
type MigrationInfo struct {
	Version  int64
	Name     string
	UpSQL    string
	DownSQL  string
	UpFile   string
	DownFile string
}

func HasEmbeddedMigrations() bool {
	matches, err := fs.Glob(embeddedMigrations, "_sqldata/*.sql")
	if err != nil {
		return false
	}
	return len(matches) > 0
}

func EmbeddedMigrationsFS() (fs.FS, error) {
	// skip _sqldata/ directory
	migFS, err := fs.Sub(embeddedMigrations, "_sqldata")
	if err != nil {
		return nil, err
	}
	return migFS, nil
}

func MigrationsFS(path string) fs.FS {
	return os.DirFS(path)
}

// ListMigrations returns a list of embedded migration files for debugging
func ListMigrations(fsys fs.FS) ([]string, error) {
	var files []string

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded migrations: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}

	sort.Strings(files)
	return files, nil
}
