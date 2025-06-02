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
	Long:  `This command seeds the database with example data.`,
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
