package run

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-extras/go-kit/must"
	"github.com/jellydator/validation"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/backup/export"
	importpkg "github.com/denisvmedia/inventario/backup/import"
	"github.com/denisvmedia/inventario/backup/restore"
	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/internal/httpserver"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

type Command struct {
	command.Base

	config   Config
	dbConfig shared.DatabaseConfig
}

func New() *Command {
	c := &Command{}

	c.Base = command.NewBase(&cobra.Command{
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

FLAG DETAILS:

  --addr (default ":3333")
    Specifies the network address and port where the server will listen.
    Format: "[host]:port" (e.g., ":8080", "localhost:3333", "0.0.0.0:8080")
    Use ":0" to let the system choose an available port.

  --db-dsn (default "memory://")
    Database connection string supporting multiple backends:
    • PostgreSQL: "postgres://user:password@host:port/database?sslmode=disable"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runCommand()
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	shared.TryReadSection("run", &c.config)
	c.config.setDefaults()

	flags := c.Cmd().Flags()
	flags.StringVar(&c.config.Addr, "addr", c.config.Addr, "Bind address for the server")
	flags.StringVar(&c.config.UploadLocation, "upload-location", c.config.UploadLocation, "Location for the uploaded files")
	shared.RegisterLocalDatabaseFlags(c.Cmd(), &c.dbConfig)
	flags.IntVar(&c.config.MaxConcurrentExports, "max-concurrent-exports", c.config.MaxConcurrentExports, "Maximum number of concurrent export processes")
	flags.IntVar(&c.config.MaxConcurrentImports, "max-concurrent-imports", c.config.MaxConcurrentImports, "Maximum number of concurrent import processes")
	flags.StringVar(&c.config.JWTSecret, "jwt-secret", c.config.JWTSecret, "JWT secret for authentication (minimum 32 characters, auto-generated if not provided)")
	flags.StringVar(&c.config.FileSigningKey, "file-signing-key", c.config.FileSigningKey, "File signing key for secure file URLs (minimum 32 characters, auto-generated if not provided)")
	flags.StringVar(&c.config.FileURLExpiration, "file-url-expiration", c.config.FileURLExpiration, "File URL expiration duration (e.g., 15m, 1h, 30s)")
}

func (c *Command) runCommand() error {
	srv := &httpserver.APIServer{}
	bindAddr := c.config.Addr
	dsn := c.dbConfig.DBDSN

	// print out all environment variables that start with INVENTARIO_
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "INVENTARIO_") {
			slog.Info("Environment variable", "name", e)
		}
	}

	parsedDSN := must.Must(registry.Config(dsn).Parse())
	if parsedDSN.User != nil {
		parsedDSN.User = url.UserPassword("xxxxxx", "xxxxxx")
	}

	slog.Info("Starting server", "addr", bindAddr, "db-dsn", parsedDSN.String())

	var params apiserver.Params

	registrySetFn, ok := registry.GetRegistry(dsn)
	if !ok {
		slog.Error("Unknown registry", "dsn", dsn)
		return errors.New("unknown registry")
	}

	slog.Info("Selected database registry", "registry_type", fmt.Sprintf("%T", registrySetFn))

	factorySet, err := registrySetFn(registry.Config(dsn))
	if err != nil {
		slog.Error("Failed to setup registry", "error", err)
		return err
	}

	params.FactorySet = factorySet
	params.UploadLocation = c.config.UploadLocation
	params.EntityService = services.NewEntityService(factorySet, params.UploadLocation)
	params.DebugInfo = debug.NewInfo(dsn, params.UploadLocation)
	params.StartTime = time.Now()

	// Configure JWT secret from config/environment or generate a secure default
	jwtSecret, err := getJWTSecret(c.config.JWTSecret)
	if err != nil {
		slog.Error("Failed to configure JWT secret", "error", err)
		return err
	}

	// Configure file signing key from config/environment or generate a secure default
	fileSigningKey, err := getFileSigningKey(c.config.FileSigningKey)
	if err != nil {
		slog.Error("Failed to configure file signing key", "error", err)
		return err
	}

	// Parse file URL expiration duration
	fileURLExpiration, err := time.ParseDuration(c.config.FileURLExpiration)
	if err != nil {
		slog.Error("Failed to parse file URL expiration duration", "error", err, "duration", c.config.FileURLExpiration)
		return err
	}

	// Parse thumbnail slot duration and create thumbnail config
	thumbnailSlotDuration, err := time.ParseDuration(c.config.ThumbnailSlotDuration)
	if err != nil {
		slog.Error("Failed to parse thumbnail slot duration", "error", err, "duration", c.config.ThumbnailSlotDuration)
		return err
	}

	thumbnailConfig := services.ThumbnailGenerationConfig{
		MaxConcurrentPerUser: c.config.ThumbnailMaxConcurrentPerUser,
		RateLimitPerMinute:   c.config.ThumbnailRateLimitPerMinute,
		SlotDuration:         thumbnailSlotDuration,
	}

	params.JWTSecret = jwtSecret
	params.FileSigningKey = fileSigningKey
	params.FileURLExpiration = fileURLExpiration
	params.ThumbnailConfig = thumbnailConfig

	err = validation.Validate(params)
	if err != nil {
		slog.Error("Invalid server parameters", "error", err)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start export worker
	maxConcurrentExports := c.config.MaxConcurrentExports
	exportService := export.NewExportService(factorySet, params.UploadLocation)
	exportWorker := export.NewExportWorker(exportService, factorySet, maxConcurrentExports)
	exportWorker.Start(ctx)
	defer exportWorker.Stop()

	// Start restore worker
	restoreService := restore.NewRestoreService(factorySet, params.EntityService, params.UploadLocation)
	// Create a service registry set for the restore worker
	serviceRegistrySet := factorySet.CreateServiceRegistrySet()
	restoreWorker := restore.NewRestoreWorker(restoreService, serviceRegistrySet, params.UploadLocation)
	restoreWorker.Start(ctx)
	defer restoreWorker.Stop()

	// Start import worker
	maxConcurrentImports := c.config.MaxConcurrentImports
	importService := importpkg.NewImportService(factorySet, params.UploadLocation)
	importWorker := importpkg.NewImportWorker(importService, factorySet, maxConcurrentImports)
	importWorker.Start(ctx)
	defer importWorker.Stop()

	// Start thumbnail generation worker
	thumbnailWorker := services.NewThumbnailGenerationWorker(factorySet, params.UploadLocation, thumbnailConfig)
	thumbnailWorker.Start(ctx)
	defer thumbnailWorker.Stop()

	errCh := srv.Run(bindAddr, apiserver.APIServer(params, restoreWorker))

	// Wait for an interrupt signal (e.g., Ctrl+C)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigCh:
	case err := <-errCh:
		slog.Error("Failure during server startup", "error", err)
		return err
	}

	slog.Info("Shutting down server")
	err = srv.Shutdown()
	if err != nil {
		slog.Error("Failure during server shutdown", "error", err)
	}

	return err
}

// getJWTSecret retrieves JWT secret from config/environment or generates a secure default
func getJWTSecret(configSecret string) ([]byte, error) {
	// Use the secret from config (which includes environment variables via cleanenv)
	if configSecret != "" {
		// If provided as hex string, decode it
		if decoded, err := hex.DecodeString(configSecret); err == nil && len(decoded) >= 32 {
			slog.Info("Using JWT secret from configuration (hex decoded)")
			return decoded, nil
		}
		// If provided as plain string and long enough, use it directly
		if len(configSecret) >= 32 {
			slog.Info("Using JWT secret from configuration")
			return []byte(configSecret), nil
		}
		slog.Warn("Configured JWT secret is too short (minimum 32 characters), generating random secret")
	}

	// Generate a secure random secret
	slog.Warn("No JWT secret configured, generating random secret")
	slog.Warn("For production use, set INVENTARIO_RUN_JWT_SECRET environment variable or jwt-secret in config file with a secure 32+ byte secret")

	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	if err != nil {
		return nil, err
	}

	slog.Info("Generated random JWT secret (hex)", "secret", hex.EncodeToString(secret))
	slog.Info("Save this secret to INVENTARIO_RUN_JWT_SECRET environment variable or config file for consistent authentication across restarts")

	return secret, nil
}

// getFileSigningKey retrieves file signing key from config/environment or generates a secure default
func getFileSigningKey(configKey string) ([]byte, error) {
	// Use the key from config (which includes environment variables via cleanenv)
	if configKey != "" {
		// If provided as hex string, decode it
		if decoded, err := hex.DecodeString(configKey); err == nil && len(decoded) >= 32 {
			slog.Info("Using file signing key from configuration (hex decoded)")
			return decoded, nil
		}
		// If provided as plain string and long enough, use it directly
		if len(configKey) >= 32 {
			slog.Info("Using file signing key from configuration")
			return []byte(configKey), nil
		}
		slog.Warn("Configured file signing key is too short (minimum 32 characters), generating random key")
	}

	// Generate a secure random key
	slog.Warn("No file signing key configured, generating random key")
	slog.Warn("For production use, set INVENTARIO_RUN_FILE_SIGNING_KEY environment variable or file-signing-key in config file with a secure 32+ byte key")

	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}

	slog.Info("Generated random file signing key (hex)", "key", hex.EncodeToString(key))
	slog.Info("Save this key to INVENTARIO_RUN_FILE_SIGNING_KEY environment variable or config file for consistent file URL signing across restarts")

	return key, nil
}
