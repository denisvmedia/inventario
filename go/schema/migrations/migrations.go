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

//// GetEmbeddedMigrations returns all embedded migration files
//func GetEmbeddedMigrations() ([]MigrationInfo, error) {
//	var migrations []MigrationInfo
//
//	// Read all files from the embedded filesystem
//	entries, err := fs.ReadDir(embeddedMigrations, "migrations/source")
//	if err != nil {
//		return nil, fmt.Errorf("failed to read embedded migrations directory: %w", err)
//	}
//
//	// Group files by version
//	migrationMap := make(map[int64]*MigrationInfo)
//
//	for _, entry := range entries {
//		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
//			continue
//		}
//
//		// Parse migration filename: {version}_{name}.{up|down}.sql
//		parts := strings.Split(entry.Name(), "_")
//		if len(parts) < 2 {
//			continue
//		}
//
//		versionStr := parts[0]
//		version, err := strconv.ParseInt(versionStr, 10, 64)
//		if err != nil {
//			continue
//		}
//
//		// Extract name and direction
//		nameAndDirection := strings.Join(parts[1:], "_")
//		if strings.HasSuffix(nameAndDirection, ".up.sql") {
//			name := strings.TrimSuffix(nameAndDirection, ".up.sql")
//
//			if migrationMap[version] == nil {
//				migrationMap[version] = &MigrationInfo{
//					Version: version,
//					Name:    name,
//				}
//			}
//
//			migrationMap[version].UpFile = entry.Name()
//
//			// Read UP SQL content
//			upPath := "migrations/source/" + entry.Name()
//			upContent, err := fs.ReadFile(embeddedMigrations, upPath)
//			if err != nil {
//				return nil, fmt.Errorf("failed to read UP migration %s: %w", entry.Name(), err)
//			}
//			migrationMap[version].UpSQL = string(upContent)
//		} else if strings.HasSuffix(nameAndDirection, ".down.sql") {
//			name := strings.TrimSuffix(nameAndDirection, ".down.sql")
//
//			if migrationMap[version] == nil {
//				migrationMap[version] = &MigrationInfo{
//					Version: version,
//					Name:    name,
//				}
//			}
//
//			migrationMap[version].DownFile = entry.Name()
//
//			// Read DOWN SQL content
//			downPath := "migrations/source/" + entry.Name()
//			downContent, err := fs.ReadFile(embeddedMigrations, downPath)
//			if err != nil {
//				return nil, fmt.Errorf("failed to read DOWN migration %s: %w", entry.Name(), err)
//			}
//			migrationMap[version].DownSQL = string(downContent)
//		}
//	}
//
//	// Convert map to sorted slice
//	for _, migration := range migrationMap {
//		migrations = append(migrations, *migration)
//	}
//
//	// Sort by version
//	sort.Slice(migrations, func(i, j int) bool {
//		return migrations[i].Version < migrations[j].Version
//	})
//
//	return migrations, nil
//}

//
//func NewGenerator(dbURL string) (*generator.Generator, error) {
//
//	return generator.New(dbURL, goEntitiesFS, goEntitiesDir)
//}
