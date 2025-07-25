package run

import (
	"context"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-extras/cobraflags"
	"github.com/go-extras/go-kit/must"
	"github.com/jellydator/validation"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/backup/export"
	importpkg "github.com/denisvmedia/inventario/backup/import"
	"github.com/denisvmedia/inventario/backup/restore"
	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/internal/defaults"
	"github.com/denisvmedia/inventario/internal/httpserver"
	"github.com/denisvmedia/inventario/internal/log"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the application server",
	Long: `Run starts the Inventario application server, providing a web-based interface
for managing your personal inventory. The server hosts both the API endpoints and
the frontend interface, allowing you to access your inventory through a web browser.

The server supports multiple database backends and provides RESTful API endpoints
for all inventory operations. File uploads are handled through configurable storage
locations that can be local filesystem paths or cloud storage URLs.

USAGE EXAMPLES:

  Basic development server (in-memory database):
    inventario run

  Production server with PostgreSQL:
    inventario run --addr=":8080" --db-dsn="postgres://user:pass@localhost/inventario"

  Custom upload location:
    inventario run --upload-location="file:///var/lib/inventario/uploads?create_dir=1"

  Local development with persistent database:
    inventario run --db-dsn="boltdb://./inventario.db" --upload-location="file://./uploads?create_dir=1"

FLAG DETAILS:

  --addr (default ":3333")
    Specifies the network address and port where the server will listen.
    Format: "[host]:port" (e.g., ":8080", "localhost:3333", "0.0.0.0:8080")
    Use ":0" to let the system choose an available port.

  --db-dsn (default "memory://")
    Database connection string supporting multiple backends:
    • PostgreSQL: "postgres://user:password@host:port/database?sslmode=disable"
    • BoltDB: "boltdb://path/to/database.db"
    • In-memory: "memory://" (data lost on restart, useful for testing)

  --upload-location (default "file://./uploads?create_dir=1")
    Specifies where uploaded files are stored. Supports:
    • Local filesystem: "file:///absolute/path?create_dir=1"
    • Relative path: "file://./relative/path?create_dir=1"
    • The "create_dir=1" parameter creates the directory if it doesn't exist

PREREQUISITES:
  • Database must be migrated before first run: "inventario migrate --db-dsn=..."
  • For production use, ensure the database and upload directory have proper permissions

SERVER ENDPOINTS:
  Once running, the server provides:
  • Web Interface: http://localhost:3333 (or your specified address)
  • API Documentation: http://localhost:3333/api/docs (Swagger UI)
  • Health Check: http://localhost:3333/api/health

The server runs until interrupted (Ctrl+C) and gracefully shuts down active connections.`,
	RunE: runCommand,
}

const (
	addrFlag                 = "addr"
	uploadLocationFlag       = "upload-location"
	dbDSNFlag                = "db-dsn"
	maxConcurrentExportsFlag = "max-concurrent-exports"
	maxConcurrentImportsFlag = "max-concurrent-imports"
)

var runFlags = map[string]cobraflags.Flag{
	addrFlag: &cobraflags.StringFlag{
		Name:  addrFlag,
		Value: defaults.GetServerAddr(),
		Usage: "Bind address for the server",
	},
	uploadLocationFlag: &cobraflags.StringFlag{
		Name:  uploadLocationFlag,
		Value: defaults.GetUploadLocation(),
		Usage: "Location for the uploaded files",
	},
	dbDSNFlag: &cobraflags.StringFlag{
		Name:  dbDSNFlag,
		Value: defaults.GetDatabaseDSN(),
		Usage: "Database DSN",
	},
	maxConcurrentExportsFlag: &cobraflags.IntFlag{
		Name:  maxConcurrentExportsFlag,
		Value: defaults.GetMaxConcurrentExports(),
		Usage: "Maximum number of concurrent export processes",
	},
	maxConcurrentImportsFlag: &cobraflags.IntFlag{
		Name:  maxConcurrentImportsFlag,
		Value: defaults.GetMaxConcurrentImports(),
		Usage: "Maximum number of concurrent import processes",
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

	if configFile := viper.ConfigFileUsed(); configFile != "" {
		log.WithField("config_file", configFile).Debug("Configuration file loaded")
	}
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
	params.EntityService = services.NewEntityService(registrySet, params.UploadLocation)
	params.DebugInfo = debug.NewInfo(dsn, params.UploadLocation)

	err = validation.Validate(params)
	if err != nil {
		log.WithError(err).Error("Invalid server parameters")
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start export worker
	maxConcurrentExports := runFlags[maxConcurrentExportsFlag].GetInt()
	exportService := export.NewExportService(registrySet, params.UploadLocation)
	exportWorker := export.NewExportWorker(exportService, registrySet, maxConcurrentExports)
	exportWorker.Start(ctx)
	defer exportWorker.Stop()

	// Start restore worker
	restoreService := restore.NewRestoreService(registrySet, params.EntityService, params.UploadLocation)
	restoreWorker := restore.NewRestoreWorker(restoreService, registrySet, params.UploadLocation)
	restoreWorker.Start(ctx)
	defer restoreWorker.Stop()

	// Start import worker
	maxConcurrentImports := runFlags[maxConcurrentImportsFlag].GetInt()
	importService := importpkg.NewImportService(registrySet, params.UploadLocation)
	importWorker := importpkg.NewImportWorker(importService, registrySet, maxConcurrentImports)
	importWorker.Start(ctx)
	defer importWorker.Stop()

	errCh := srv.Run(bindAddr, apiserver.APIServer(params, restoreWorker))

	// Wait for an interrupt signal (e.g., Ctrl+C)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	select {
	case <-c:
	case err := <-errCh:
		log.WithError(err).Error("Failure during server startup")
		return err
	}

	log.Info("Shutting down server")
	err = srv.Shutdown()
	if err != nil {
		log.WithError(err).Error("Failure during server shutdown")
	}

	return err
}
