package seed

import (
	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/internal/log"
	"github.com/denisvmedia/inventario/internal/seeddata"
	"github.com/denisvmedia/inventario/registry"
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed the database with example data",
	Long:  `This command seeds the database with example data.`,
	RunE:  seedCommand,
}

const (
	dbDSNFlag = "db-dsn"
)

var seedFlags = map[string]cobraflags.Flag{
	dbDSNFlag: &cobraflags.StringFlag{
		Name:  dbDSNFlag,
		Value: "memory://",
		Usage: "Database DSN",
	},
}

func NewSeedCommand() *cobra.Command {
	cobraflags.RegisterMap(seedCmd, seedFlags)

	return seedCmd
}

func seedCommand(_ *cobra.Command, _ []string) error {
	dsn := seedFlags[dbDSNFlag].GetString()
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


