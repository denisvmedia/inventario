package run

import (
	"context"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/go-extras/cobraflags"
	"github.com/go-extras/go-kit/must"
	"github.com/jellydator/validation"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/export"
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
	addrFlag               = "addr"
	uploadLocationFlag     = "upload-location"
	dbDSNFlag              = "db-dsn"
	exportDirFlag          = "export-dir"
	maxConcurrentExportsFlag = "max-concurrent-exports"
)

func getFileURL(path string) string {
	absPath := filepath.ToSlash(filepath.Join(must.Must(os.Getwd()), path))
	if strings.Contains(absPath, ":") {
		absPath = "/" + absPath // Ensure the drive letter is prefixed with a slash
	}
	return "file://" + absPath + "?create_dir=1"
}

var runFlags = map[string]cobraflags.Flag{
	addrFlag: &cobraflags.StringFlag{
		Name:  addrFlag,
		Value: ":3333",
		Usage: "Bind address for the server",
	},
	uploadLocationFlag: &cobraflags.StringFlag{
		Name:  uploadLocationFlag,
		Value: getFileURL("uploads"),
		Usage: "Location for the uploaded files",
	},
	dbDSNFlag: &cobraflags.StringFlag{
		Name:  dbDSNFlag,
		Value: "memory://",
		Usage: "Database DSN",
	},
	exportDirFlag: &cobraflags.StringFlag{
		Name:  exportDirFlag,
		Value: "exports",
		Usage: "Directory to store export files",
	},
	maxConcurrentExportsFlag: &cobraflags.IntFlag{
		Name:  maxConcurrentExportsFlag,
		Value: 3,
		Usage: "Maximum number of concurrent export processes",
	},
}

func NewRunCommand() *cobra.Command {
	cobraflags.RegisterMap(runCmd, runFlags)

	return runCmd
}

func runCommand(_ *cobra.Command, _ []string) error {
	srv := &httpserver.APIServer{}
	bindAddr := runFlags[addrFlag].GetString()
	dsn := runFlags[dbDSNFlag].GetString()
	parsedDSN := must.Must(registry.Config(dsn).Parse())
	if parsedDSN.User != nil {
		parsedDSN.User = url.UserPassword("xxxxxx", "xxxxxx")
	}

	log.WithFields(log.Fields{
		addrFlag:  bindAddr,
		dbDSNFlag: parsedDSN.String(),
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
	params.DebugInfo = debug.NewInfo(dsn, params.UploadLocation)

	err = validation.Validate(params)
	if err != nil {
		log.WithError(err).Error("Invalid server parameters")
		return err
	}

	// Start export worker
	exportDir := runFlags[exportDirFlag].GetString()
	maxConcurrentExports := runFlags[maxConcurrentExportsFlag].GetInt()
	exportService := export.NewExportService(registrySet, exportDir, params.UploadLocation)
	exportWorker := export.NewExportWorker(exportService, registrySet, maxConcurrentExports)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	exportWorker.Start(ctx)
	defer exportWorker.Stop()

	errCh := srv.Run(bindAddr, apiserver.APIServer(params))

	// Wait for an interrupt signal (e.g., Ctrl+C)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	select {
	case <-c:
	case err := <-errCh:
		log.WithError(err).Error("Failure during server startup")
		os.Exit(1) //revive:disable-line:deep-exit
		return nil
	}

	log.Info("Shutting down server")
	err = srv.Shutdown()
	if err != nil {
		log.WithError(err).Error("Failure during server shutdown")
	}

	return err
}
