package migrate

import (
	"context"
	"fmt"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/internal/log"
	"github.com/denisvmedia/inventario/registry/migrations"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  `This command runs database migrations for the specified database.`,
	RunE:  migrateCommand,
}

const (
	dbDSNFlag = "db-dsn"
)

var migrateFlags = map[string]cobraflags.Flag{
	dbDSNFlag: &cobraflags.StringFlag{
		Name:  dbDSNFlag,
		Value: "",
		Usage: "Database DSN (required). Supported types: postgresql://, memory://, boltdb://",
	},
}

func NewMigrateCommand() *cobra.Command {
	cobraflags.RegisterMap(migrateCmd, migrateFlags)

	return migrateCmd
}

func migrateCommand(_ *cobra.Command, _ []string) error {
	dsn := migrateFlags[dbDSNFlag].GetString()

	if dsn == "" {
		return fmt.Errorf("database DSN is required")
	}

	log.WithField(dbDSNFlag, dsn).Info("Running migrations")

	// Run migrations using the standardized interface
	err := migrations.RunMigrations(context.Background(), dsn)
	if err != nil {
		log.WithError(err).Error("Failed to run migrations")
		return err
	}

	log.Info("Migrations completed successfully")
	return nil
}
