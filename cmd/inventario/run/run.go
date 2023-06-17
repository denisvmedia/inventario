package run

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/jellydator/validation"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/cobraflags"
	"github.com/denisvmedia/inventario/internal/httpserver"
	"github.com/denisvmedia/inventario/internal/log"
	"github.com/denisvmedia/inventario/registry"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the application server",
	Long:  `This command runs the application server.`,
	RunE:  runCommand,
}

const (
	addrFlag = "addr"
)

var runFlags = map[string]cobraflags.Flag{
	addrFlag: &cobraflags.StringFlag{
		Name:  addrFlag,
		Value: ":3333",
		Usage: "Bind address for the server",
	},
}

func NewRunCommand() *cobra.Command {
	cobraflags.RegisterMap(runCmd, runFlags)

	return runCmd
}

func runCommand(_ *cobra.Command, _ []string) error {
	srv := &httpserver.APIServer{}
	bindAddr := runFlags[addrFlag].GetString()
	log.WithField(addrFlag, bindAddr).Info("Starting server")

	params := apiserver.Params{}
	params.LocationRegistry = registry.NewMemoryLocationRegistry()
	params.AreaRegistry = registry.NewMemoryAreaRegistry(params.LocationRegistry)
	params.CommodityRegistry = registry.NewMemoryCommodityRegistry(params.AreaRegistry)

	err := validation.Validate(params)
	if err != nil {
		log.WithError(err).Error("Invalid server parameters")
		return err
	}
	srv.Run(bindAddr, apiserver.APIServer(params))

	// Wait for an interrupt signal (e.g., Ctrl+C)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Info("Shutting down server")
	err = srv.Shutdown()
	if err != nil {
		log.WithError(err).Error("Failure during server shutdown")
	}

	return err
}
