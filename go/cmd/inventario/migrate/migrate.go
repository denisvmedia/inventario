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
	dbDSNFlag  = "db-dsn"
	dryRunFlag = "dry-run"
)

var migrateFlags = map[string]cobraflags.Flag{
	dbDSNFlag: &cobraflags.StringFlag{
		Name:  dbDSNFlag,
		Value: "",
		Usage: "Database DSN (required). Supported types: postgres://, memory://, boltdb://",
	},
	dryRunFlag: &cobraflags.BoolFlag{
		Name:  dryRunFlag,
		Value: false,
		Usage: "Show what migrations would be executed without making actual changes",
	},
}

func NewMigrateCommand() *cobra.Command {
	cobraflags.RegisterMap(migrateCmd, migrateFlags)

	return migrateCmd
}

func migrateCommand(_ *cobra.Command, _ []string) error {
	dsn := migrateFlags[dbDSNFlag].GetString()
	dryRun := migrateFlags[dryRunFlag].GetBool()

	if dsn == "" {
		return fmt.Errorf("database DSN is required")
	}

	if dryRun {
		// log.WithField(dbDSNFlag, dsn).Info("[DRY RUN] Would run migrations")
		// fmt.Println("⚠️  [DRY RUN] Migration dry run mode is not yet fully implemented.")
		// fmt.Println("⚠️  This would run migrations against the database.")
		// fmt.Println("⚠️  For now, use the ptah tool 'migrate' command for dry run migration SQL generation.")
		return fmt.Errorf("dry run mode is not yet implemented")
	} else {
		log.WithField(dbDSNFlag, dsn).Info("Running migrations")
	}

	// Run migrations using the standardized interface
	err := migrations.RunMigrations(context.Background(), dsn)
	if err != nil {
		log.WithError(err).Error("Failed to run migrations")
		return err
	}

	log.Info("Migrations completed successfully")
	return nil
}
