// Package backup is the parent command group for signed `.inb` backup archive
// tooling (issue #534). Its subcommands operate directly on `.inb` files on
// disk and need only the backup signing key — never a database connection.
package backup

import (
	"github.com/spf13/cobra"
)

// New constructs the parent `backup` command and registers its subcommands.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Signed .inb backup archive tooling",
		Long: `Parent command for working with signed .inb backup archives.

Subcommands operate on .inb files directly and require only the backup signing
key (--backup-signing-key / INVENTARIO_RUN_BACKUP_SIGNING_KEY) — they do not
connect to a database.

Examples:
  inventario backup public-key
  inventario backup resign old.inb -o new.inb`,
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(newResignCmd())
	cmd.AddCommand(newPublicKeyCmd())
	return cmd
}
