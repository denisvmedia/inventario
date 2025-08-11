package ptah

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/stokaro/ptah/migration/migrator"
)

// Embed migration files into the binary
// Note: This embeds all files in the migrations/source directory
//
//go:embed migrations/source/*
var embeddedMigrations embed.FS

//go:embed  migrations/custom/0000000000_permissions.sql
var permissionsSQL string

// EmbeddedMigrationInfo contains information about an embedded migration
type EmbeddedMigrationInfo struct {
	Version  int64
	Name     string
	UpSQL    string
	DownSQL  string
	UpFile   string
	DownFile string
}

// GetEmbeddedMigrations returns all embedded migration files
func GetEmbeddedMigrations() ([]EmbeddedMigrationInfo, error) {
	var migrations []EmbeddedMigrationInfo

	// Read all files from the embedded filesystem
	entries, err := fs.ReadDir(embeddedMigrations, "migrations/source")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded migrations directory: %w", err)
	}

	// Group files by version
	migrationMap := make(map[int64]*EmbeddedMigrationInfo)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Parse migration filename: {version}_{name}.{up|down}.sql
		parts := strings.Split(entry.Name(), "_")
		if len(parts) < 2 {
			continue
		}

		versionStr := parts[0]
		version, err := strconv.ParseInt(versionStr, 10, 64)
		if err != nil {
			continue
		}

		// Extract name and direction
		nameAndDirection := strings.Join(parts[1:], "_")
		if strings.HasSuffix(nameAndDirection, ".up.sql") {
			name := strings.TrimSuffix(nameAndDirection, ".up.sql")

			if migrationMap[version] == nil {
				migrationMap[version] = &EmbeddedMigrationInfo{
					Version: version,
					Name:    name,
				}
			}

			migrationMap[version].UpFile = entry.Name()

			// Read UP SQL content
			upPath := "migrations/source/" + entry.Name()
			upContent, err := fs.ReadFile(embeddedMigrations, upPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read UP migration %s: %w", entry.Name(), err)
			}
			migrationMap[version].UpSQL = string(upContent)
		} else if strings.HasSuffix(nameAndDirection, ".down.sql") {
			name := strings.TrimSuffix(nameAndDirection, ".down.sql")

			if migrationMap[version] == nil {
				migrationMap[version] = &EmbeddedMigrationInfo{
					Version: version,
					Name:    name,
				}
			}

			migrationMap[version].DownFile = entry.Name()

			// Read DOWN SQL content
			downPath := "migrations/source/" + entry.Name()
			downContent, err := fs.ReadFile(embeddedMigrations, downPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read DOWN migration %s: %w", entry.Name(), err)
			}
			migrationMap[version].DownSQL = string(downContent)
		}
	}

	// Convert map to sorted slice
	for _, migration := range migrationMap {
		migrations = append(migrations, *migration)
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// RegisterEmbeddedMigrations registers all embedded migrations with a Ptah migrator
func RegisterEmbeddedMigrations(ptahMigrator *migrator.Migrator) error {
	migrations, err := GetEmbeddedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get embedded migrations: %w", err)
	}

	fmt.Printf("Registering %d embedded migrations:\n", len(migrations)) //nolint:forbidigo // CLI output is OK

	for _, migration := range migrations {
		fmt.Printf("  - %d_%s (UP: %d bytes, DOWN: %d bytes)\n", //nolint:forbidigo // CLI output is OK
			migration.Version, migration.Name, len(migration.UpSQL), len(migration.DownSQL))

		// Create and register migration with Ptah migrator
		ptahMigration := migrator.CreateMigrationFromSQL(
			int(migration.Version),
			migration.Name,
			migration.UpSQL,
			migration.DownSQL,
		)
		ptahMigrator.Register(ptahMigration)
	}

	return nil
}

// HasEmbeddedMigrations checks if any migration files are embedded
func HasEmbeddedMigrations() bool {
	entries, err := fs.ReadDir(embeddedMigrations, "migrations/source")
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			return true
		}
	}

	return false
}

// ListEmbeddedMigrations returns a list of embedded migration files for debugging
func ListEmbeddedMigrations() ([]string, error) {
	var files []string

	entries, err := fs.ReadDir(embeddedMigrations, "migrations/source")
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

func PermissionsSQL() string {
	return permissionsSQL
}
