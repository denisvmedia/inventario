package version

import (
	"github.com/spf13/cobra"

	"go.5x5.cz/inventario/internal/version"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display the current version, build information, and platform details.",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(version.String())
		},
	}
}
