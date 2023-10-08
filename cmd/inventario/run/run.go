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
	"github.com/denisvmedia/inventario/registry/boltdb"
	"github.com/denisvmedia/inventario/registry/memory"
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
	dbDSN              = "db-dsn"
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
	dbDSN: &cobraflags.StringFlag{
		Name:  dbDSN,
		Value: "memory://",
	},
}

func NewRunCommand() *cobra.Command {
	cobraflags.RegisterMap(runCmd, runFlags)

	return runCmd
}

func registerDBBackends() {
	boltdb.Register()
	memory.Register()
}

func runCommand(_ *cobra.Command, _ []string) error {
	registerDBBackends()

	srv := &httpserver.APIServer{}
	bindAddr := runFlags[addrFlag].GetString()
	dsn := runFlags[dbDSN].GetString()
	log.WithFields(log.Fields{
		addrFlag: bindAddr,
		dbDSN:    dsn,
	}).Info("Starting server")

	var params apiserver.Params

	registrySetFn, ok := registry.GetRegistry(dsn)
	if !ok {
		log.WithField("dsn", dsn).Fatal("Unknown registry")
		return nil
	}

	registrySet, err := registrySetFn(registry.Config(dsn))
	if err != nil {
		log.WithError(err).Fatal("Failed to setup registry")
		return nil
	}

	params.RegistrySet = registrySet
	params.UploadLocation = runFlags[uploadLocationFlag].GetString()

	err = validation.Validate(params)
	if err != nil {
		log.WithError(err).Error("Invalid server parameters")
		return err
	}
	errCh := srv.Run(bindAddr, apiserver.APIServer(params))

	// Wait for an interrupt signal (e.g., Ctrl+C)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	select {
	case <-c:
	case err := <-errCh:
		log.WithError(err).Error("Failure during server startup")
		os.Exit(1)
		return nil
	}

	log.Info("Shutting down server")
	err = srv.Shutdown()
	if err != nil {
		log.WithError(err).Error("Failure during server shutdown")
	}

	return err
}
