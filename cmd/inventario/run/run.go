package run

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/go-extras/go-kit/must"
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
	addrFlag           = "addr"
	uploadLocationFlag = "upload-location"
)

var runFlags = map[string]cobraflags.Flag{
	addrFlag: &cobraflags.StringFlag{
		Name:  addrFlag,
		Value: ":3333",
		Usage: "Bind address for the server",
	},
	uploadLocationFlag: &cobraflags.StringFlag{
		Name:  uploadLocationFlag,
		Value: "file://" + filepath.Join(filepath.ToSlash(must.Must(os.Getwd())), "uploads"),
		Usage: "Location for the uploaded files",
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

	var params apiserver.Params
	params.LocationRegistry = registry.NewMemoryLocationRegistry()
	params.AreaRegistry = registry.NewMemoryAreaRegistry(params.LocationRegistry)
	params.CommodityRegistry = registry.NewMemoryCommodityRegistry(params.AreaRegistry)
	params.UploadLocation = runFlags[uploadLocationFlag].GetString()

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
