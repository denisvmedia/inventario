package inventario

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/denisvmedia/inventario/cmd/inventario/run"
)

var rootCmd = &cobra.Command{
	Use:   "inventario",
	Short: "Inventario application",
	Long:  `Inventario is a personal inventory application.`,
	Args:  cobra.NoArgs, // Disallow unknown subcommands
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(args ...string) {
	viper.AutomaticEnv()
	viper.SetEnvPrefix("INVENTARIO")

	rootCmd.SetArgs(args)
	rootCmd.AddCommand(run.NewRunCommand())
	err := rootCmd.Execute()
	if err != nil {
		// log.WithError(err).UserError("Failed to execute root command")
		os.Exit(1)
	}
}
