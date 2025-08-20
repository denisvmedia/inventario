package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/common/version"
	"github.com/denisvmedia/inventario/cmd/inventool/db"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(args ...string) {
	var rootCmd = &cobra.Command{
		Use:   "inventool",
		Short: "Inventario Developer Tool",
		Long: `TODO: complete command docs

Use "inventool [command] --help" for detailed information about each command.`,
		Args: cobra.NoArgs, // Disallow unknown subcommands
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	rootCmd.SetArgs(args)
	rootCmd.AddCommand(version.New())
	rootCmd.AddCommand(db.New())
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1) //revive:disable-line:deep-exit
	}
}
