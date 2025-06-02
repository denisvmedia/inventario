package migrator

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Migration file naming pattern: NNNNNNNNNN_description.up.sql or NNNNNNNNNN_description.down.sql
var fileNameRe = regexp.MustCompile(`^(\d{10})_(.*).(down|up)(\.sql)$`)

// MigrationFile represents the parsed components of a migration file name
type MigrationFile struct {
	Version   int
	Name      string
	Direction string
	Extension string
}

// ParseMigrationFileName parses a migration filename into its components
// Expected format: NNNNNNNNNN_description.up.sql or NNNNNNNNNN_description.down.sql
// where NNNNNNNNNN is a 10-digit version number
func ParseMigrationFileName(filename string) (*MigrationFile, error) {
	matches := fileNameRe.FindStringSubmatch(filename)

	if matches == nil || len(matches) != 5 {
		return nil, errors.New("invalid migration file name format")
	}

	version, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, err
	}

	// Check if the name component is empty
	if matches[2] == "" {
		return nil, errors.New("migration name cannot be empty")
	}

	name := strings.ReplaceAll(matches[2], "_", " ")
	// Capitalize name
	name = cases.Title(language.English).String(name)

	direction := matches[3]
	extension := matches[4]

	return &MigrationFile{
		Version:   version,
		Name:      name,
		Direction: direction,
		Extension: extension,
	}, nil
}

// ValidateMigrationFileName validates that a filename follows the expected migration pattern
func ValidateMigrationFileName(filename string) bool {
	_, err := ParseMigrationFileName(filename)
	return err == nil
}

// GenerateMigrationFileName generates a migration filename from components
func GenerateMigrationFileName(version int, description, direction string) string {
	// Convert description to snake_case
	desc := strings.ToLower(description)
	desc = strings.ReplaceAll(desc, " ", "_")
	desc = regexp.MustCompile(`[^a-z0-9_]`).ReplaceAllString(desc, "")

	return fmt.Sprintf("%010d_%s.%s.sql", version, desc, direction)
}

// GetNextMigrationVersion generates the next migration version number
// This is a simple implementation that uses the current timestamp
func GetNextMigrationVersion() int {
	return int(time.Now().Unix())
}

// FormatTimestampForDatabase formats a timestamp for the specific database dialect
func FormatTimestampForDatabase(dialect string) string {
	now := time.Now()
	switch dialect {
	case "mysql", "mariadb":
		// MySQL/MariaDB expects format: 'YYYY-MM-DD HH:MM:SS'
		return now.Format("2006-01-02 15:04:05")
	case "postgres":
		// PostgreSQL accepts ISO 8601 format
		return now.Format(time.RFC3339)
	default:
		// Default to ISO 8601
		return now.Format(time.RFC3339)
	}
}

// SortMigrationFiles sorts migration files by version number
func SortMigrationFiles(files []MigrationFile) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].Version < files[j].Version
	})
}

// GroupMigrationFiles groups migration files by version, returning a map
// where each version maps to a struct containing up and down migration files
func GroupMigrationFiles(files []MigrationFile) map[int]MigrationPair {
	groups := make(map[int]MigrationPair)

	for _, file := range files {
		pair := groups[file.Version]
		switch file.Direction {
		case "up":
			pair.Up = &file
		case "down":
			pair.Down = &file
		}
		groups[file.Version] = pair
	}

	return groups
}

// MigrationPair represents a pair of up and down migration files for a version
type MigrationPair struct {
	Up   *MigrationFile
	Down *MigrationFile
}

// IsComplete returns true if both up and down migrations are present
func (mp MigrationPair) IsComplete() bool {
	return mp.Up != nil && mp.Down != nil
}

// HasUp returns true if the up migration is present
func (mp MigrationPair) HasUp() bool {
	return mp.Up != nil
}

// HasDown returns true if the down migration is present
func (mp MigrationPair) HasDown() bool {
	return mp.Down != nil
}

// GetVersion returns the version number (assumes both up and down have same version)
func (mp MigrationPair) GetVersion() int {
	if mp.Up != nil {
		return mp.Up.Version
	}
	if mp.Down != nil {
		return mp.Down.Version
	}
	return 0
}

// GetDescription returns the description (assumes both up and down have same description)
func (mp MigrationPair) GetDescription() string {
	if mp.Up != nil {
		return mp.Up.Name
	}
	if mp.Down != nil {
		return mp.Down.Name
	}
	return ""
}

// ValidateMigrationPairs validates that all migration pairs are complete
// Returns a list of versions that are missing either up or down migrations
func ValidateMigrationPairs(pairs map[int]MigrationPair) []int {
	var incomplete []int

	for version, pair := range pairs {
		if !pair.IsComplete() {
			incomplete = append(incomplete, version)
		}
	}

	sort.Ints(incomplete)
	return incomplete
}

// FindMigrationGaps finds gaps in migration version sequences
// Returns a list of missing version numbers in the sequence
func FindMigrationGaps(versions []int) []int {
	if len(versions) == 0 {
		return nil
	}

	sort.Ints(versions)
	var gaps []int

	for i := 1; i < len(versions); i++ {
		current := versions[i]
		previous := versions[i-1]

		// Check for gaps (this is a simple implementation)
		// In practice, you might want more sophisticated gap detection
		if current-previous > 1 {
			for v := previous + 1; v < current; v++ {
				gaps = append(gaps, v)
			}
		}
	}

	return gaps
}
