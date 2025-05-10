package inventario

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/denisvmedia/inventario/cmd/inventario/run"
	"github.com/denisvmedia/inventario/cmd/inventario/seed"
)

const (
	envPrefix = "INVENTARIO"
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
	viper.SetEnvPrefix(envPrefix)

	rootCmd.SetArgs(args)
	rootCmd.AddCommand(run.NewRunCommand())
	rootCmd.AddCommand(seed.NewSeedCommand())
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1) //revive:disable-line:deep-exit
	}
}
