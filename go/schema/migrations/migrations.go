package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strconv"
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

// MaxVersion returns the highest migration version found in the provided
// migration filesystem, or 0 if none are present. The filesystem must follow
// ptah's naming convention: `<unix-seconds>_<description>.up.sql`. Used to
// compare an app/migrator binary's embedded migrations against the version
// recorded in `schema_migrations` so we can detect when a stale binary
// (for example the migrate container in a multi-image docker-compose stack —
// see #1655) left the DB behind the binary actually running.
func MaxVersion(fsys fs.FS) (int64, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return 0, fmt.Errorf("failed to read migration filesystem: %w", err)
	}

	var max int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Only look at .up.sql to avoid double-counting (.down.sql carries
		// the same version prefix).
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			continue
		}
		v, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			// Not a versioned migration file — silently skip rather than
			// fail the whole comparison.
			continue
		}
		if v > max {
			max = v
		}
	}

	return max, nil
}
