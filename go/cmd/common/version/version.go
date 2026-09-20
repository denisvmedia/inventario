package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"go.5x5.cz/inventario/internal/version"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display the current version, build information, and platform details.",
		Run: func(cmd *cobra.Command, _ []string) {
			// Not cmd.Println: cobra's Print family writes to OutOrStderr, so
			// the version — this command's whole answer — would land on stderr
			// and `inventario version > file` would leave the file empty.
			fmt.Fprintln(cmd.OutOrStdout(), version.String())
		},
	}
}
