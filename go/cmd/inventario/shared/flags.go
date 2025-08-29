package shared

import (
	"github.com/go-extras/go-kit/ptr"
	"github.com/spf13/cobra"
)

func RegisterBootstrapFlags(cmd *cobra.Command, username, usernameForMigrations, usernameForBackgroundWorker *string) {
	flags := cmd.Flags()
	flags.StringVar(username, "username", ptr.From(username), "Operational database username")
	flags.StringVar(usernameForMigrations, "username-for-migrations", ptr.From(usernameForMigrations), "Database username for migrations")
	flags.StringVar(usernameForBackgroundWorker, "username-for-background-worker", ptr.From(usernameForBackgroundWorker), "Database username for background worker")
}

func RegisterDryRunFlag(cmd *cobra.Command, dryRun *bool) {
	cmd.Flags().BoolVar(dryRun, "dry-run", ptr.From(dryRun), "Show what would be executed without running")
}
