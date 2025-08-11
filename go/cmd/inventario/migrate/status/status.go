package status

import (
	"context"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/migrate/common"
)

// New creates the migrate status subcommand
func New(dsnFlag cobraflags.Flag) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current migration status",
		Long: `Display the current migration status including:
- Current database version
- Total number of migrations
- Number of pending migrations
- List of pending migrations (with --verbose)

Examples:
  inventario migrate status                # Show basic status
  inventario migrate status --verbose      # Show detailed status with pending migrations`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return migrateStatusCommand(cmd, args, dsnFlag)
		},
	}

	cmd.Flags().Bool("verbose", false, "Show detailed status information")

	return cmd
}

// migrateStatusCommand handles the migrate status subcommand
func migrateStatusCommand(cmd *cobra.Command, args []string, dsnFlag cobraflags.Flag) error {
	verbose, _ := cmd.Flags().GetBool("verbose")

	migrator, err := common.CreatePtahMigrator(dsnFlag.GetString())
	if err != nil {
		return err
	}

	return migrator.PrintMigrationStatus(context.Background(), verbose)
}
