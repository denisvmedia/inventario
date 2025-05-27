package migrations

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var fileNameRe = regexp.MustCompile(`^(\d{10})_(.*).(down|up)(\.sql)$`)

// MigrationFile represents the parsed components of a migration file name
type MigrationFile struct {
	Version   int
	Name      string
	Direction string
	Extension string
}

// ParseMigrationFileName parses a migration filename into its components
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
