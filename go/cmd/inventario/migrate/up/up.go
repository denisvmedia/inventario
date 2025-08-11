package up

import (
	"context"
	"fmt"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/migrate/common"
	"github.com/denisvmedia/inventario/registry/ptah"
)

const (
	dryRunFlag = "dry-run"
)

var flags = map[string]cobraflags.Flag{
	dryRunFlag: &cobraflags.BoolFlag{
		Name:  dryRunFlag,
		Usage: "Show what would be dropped without executing",
	},
}

// New creates the migrate up subcommand
func New(dsnFlag cobraflags.Flag) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		Long: `Apply all pending database migrations to bring the schema up to date.

Each migration runs in its own transaction, so if any migration fails,
it will be rolled back and the migration process will stop.

Examples:
  inventario migrate up                    # Apply all pending migrations
  inventario migrate up --dry-run          # Preview what would be applied`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return migrateUpCommand(cmd, args, dsnFlag)
		},
	}

	// Register cobraflags which automatically handles viper binding and persistent flags
	cobraflags.RegisterMap(cmd, flags)
	common.RegisterOperationalUserFlag(cmd)

	return cmd
}

// migrateUpCommand handles the migrate up subcommand
func migrateUpCommand(cmd *cobra.Command, args []string, dsnFlag cobraflags.Flag) error {
	dryRun := flags[dryRunFlag].GetBool()
	dsn := dsnFlag.GetString()
	opUser, _ := cmd.Flags().GetString("operational-user")
	operationalUser := common.GetOperationalUser(opUser, dsn)

	migratorArgs := ptah.MigrateArgs{
		OperationalUser: operationalUser,
		DryRun:          dryRun,
	}

	migrator, err := common.CreatePtahMigrator(dsn)
	if err != nil {
		return err
	}

	fmt.Println("=== MIGRATE UP ===") //nolint:forbidigo // CLI output is OK
	fmt.Printf("Database: %s\n", dsn) //nolint:forbidigo // CLI output is OK
	fmt.Println()                     //nolint:forbidigo // CLI output is OK

	return migrator.MigrateUp(context.Background(), migratorArgs)
}
