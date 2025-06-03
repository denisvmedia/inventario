package seed

import (
	"fmt"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/debug/seeddata"
	"github.com/denisvmedia/inventario/internal/log"
	"github.com/denisvmedia/inventario/registry"
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed the database with example data",
	Long: `Seed populates your Inventario database with sample data to help you get started
quickly. This is particularly useful for development, testing, or when you want to
explore the application's features with realistic example data.

The seeding process creates sample inventory items, locations, categories, and other
data structures that demonstrate the full capabilities of the application. This allows
you to immediately begin exploring features without manually creating initial data.

USAGE EXAMPLES:

  Seed with default in-memory database:
    inventario seed

  Seed a PostgreSQL database:
    inventario seed --db-dsn="postgres://user:pass@localhost/inventario"

  Seed a local BoltDB database:
    inventario seed --db-dsn="boltdb://./inventario.db"

  Preview what would be seeded (dry-run mode):
    inventario seed --dry-run --db-dsn="postgres://user:pass@localhost/inventario"

FLAG DETAILS:

  --db-dsn (default "memory://")
    Database connection string. Must match the same database you plan to use
    with the 'run' command. Supported formats:
    • PostgreSQL: "postgres://user:password@host:port/database?sslmode=disable"
    • BoltDB: "boltdb://path/to/database.db"
    • In-memory: "memory://" (useful for quick testing)

  --dry-run (default false)
    When enabled, shows what data would be created without making actual changes.
    Note: Dry-run mode is not yet fully implemented and will return an error.
    This flag is reserved for future functionality.

SAMPLE DATA INCLUDES:
  • Sample locations (rooms, storage areas, containers)
  • Example inventory categories (electronics, books, tools, etc.)
  • Various inventory items with different attributes
  • Realistic metadata and relationships between items

PREREQUISITES:
  • Database must exist and be accessible
  • Database schema must be up-to-date (run 'inventario migrate' first)
  • For PostgreSQL, ensure the user has INSERT permissions
  • For file-based databases, ensure write permissions to the directory

WORKFLOW:
  1. Create/prepare your database
  2. Run migrations: inventario migrate --db-dsn="your-database-url"  
  3. Seed with data: inventario seed --db-dsn="your-database-url"
  4. Start the server: inventario run --db-dsn="your-database-url"

WARNING: Seeding may create duplicate data if run multiple times on the same database.
Consider backing up your database before seeding if it contains important data.`,
	RunE:  seedCommand,
}

const (
	dbDSNFlag  = "db-dsn"
	dryRunFlag = "dry-run"
)

var seedFlags = map[string]cobraflags.Flag{
	dbDSNFlag: &cobraflags.StringFlag{
		Name:  dbDSNFlag,
		Value: "memory://",
		Usage: "Database DSN",
	},
	dryRunFlag: &cobraflags.BoolFlag{
		Name:  dryRunFlag,
		Value: false,
		Usage: "Show what data would be seeded without making actual changes",
	},
}

func NewSeedCommand() *cobra.Command {
	cobraflags.RegisterMap(seedCmd, seedFlags)

	return seedCmd
}

func seedCommand(_ *cobra.Command, _ []string) error {
	dsn := seedFlags[dbDSNFlag].GetString()
	dryRun := seedFlags[dryRunFlag].GetBool()

	if dryRun {
		// log.WithFields(log.Fields{
		//	dbDSNFlag: dsn,
		// }).Info("[DRY RUN] Would seed database")
		// fmt.Println("⚠️  [DRY RUN] Seed dry run mode is not yet fully implemented.")
		// fmt.Println("⚠️  This would seed the database with example data.")
		// fmt.Println("⚠️  The seed data would include sample locations, areas, and commodities.")
		return fmt.Errorf("dry run mode not yet implemented")
	}

	log.WithFields(log.Fields{
		dbDSNFlag: dsn,
	}).Info("Seeding database")

	registrySetFn, ok := registry.GetRegistry(dsn)
	if !ok {
		log.WithField("dsn", dsn).Fatal("Unknown registry")
		return nil
	}

	registrySet, err := registrySetFn(registry.Config(dsn))
	if err != nil {
		log.WithError(err).Fatal("Failed to setup registry")
		return nil
	}

	err = seeddata.SeedData(registrySet)
	if err != nil {
		log.WithError(err).Fatal("Failed to seed data")
		return nil
	}

	log.Info("Database seeded successfully")
	return nil
}
