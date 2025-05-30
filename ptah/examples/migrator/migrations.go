package migrator_examples

import (
	"embed"
)

//go:embed migrations
var exampleMigrations embed.FS

// GetExampleMigrations returns the embedded example migrations filesystem
func GetExampleMigrations() embed.FS {
	return exampleMigrations
}
