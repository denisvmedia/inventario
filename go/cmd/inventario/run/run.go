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
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/backup/export"
	importpkg "github.com/denisvmedia/inventario/backup/import"
	"github.com/denisvmedia/inventario/backup/restore"
	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/internal/httpserver"
	"github.com/denisvmedia/inventario/models"
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
  • Liveness Probe: http://localhost:3333/healthz
  • Readiness Probe: http://localhost:3333/readyz

Use /readyz for load balancer/orchestrator "can serve traffic" checks, and /healthz for basic process liveness.

The server runs until interrupted (Ctrl+C) and gracefully shuts down active connections.

SUBCOMMANDS:
  all        Start the API server and every background worker (default).
  apiserver  Start only the HTTP API server; background workers must run separately.
  workers    Start every background worker; no HTTP listener is opened.

Invoking "inventario run" without a subcommand is equivalent to "inventario run all"
and is kept for backward compatibility.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.runAll()
		},
	})

	c.registerFlags()
	c.registerSubcommands()

	return c
}

// registerSubcommands attaches the all/apiserver/workers subcommands to the
// `run` parent. All subcommands inherit the parent's PersistentFlags, so each
// accepts the full flag set and reads the same Config.
func (c *Command) registerSubcommands() {
	allCmd := &cobra.Command{
		Use:   "all",
		Short: "Start the API server and every background worker",
		Long: `Start the HTTP API server together with every background worker.

This is equivalent to invoking "inventario run" without a subcommand and is the
default, single-process mode.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.runAll()
		},
	}

	apiserverCmd := &cobra.Command{
		Use:   "apiserver",
		Short: "Start the HTTP API server only (no background workers)",
		Long: `Start only the HTTP API server. No background worker goroutines are started.

In this mode the API server uses a registry-backed RestoreStatusQuerier so it
can still enforce the "one active restore at a time" invariant. The matching
"inventario run workers" process (typically on another host) must be running
to actually process exports, imports, restores, thumbnails and token cleanup.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.runAPIServer()
		},
	}

	workersCmd := &cobra.Command{
		Use:     "workers",
		Aliases: []string{"worker"},
		Short:   "Start every background worker (no HTTP listener)",
		Long: `Start every background worker (export, import, restore, thumbnail generation
and refresh token cleanup) without opening the HTTP listener.

This mode is intended for split deployments where the API server runs as a
separate "inventario run apiserver" process.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.runWorkers()
		},
	}

	c.Cmd().AddCommand(allCmd, apiserverCmd, workersCmd)
}

func (c *Command) registerFlags() {
	shared.TryReadSection("run", &c.config)
	c.config.setDefaults()

	// Flags are registered as PersistentFlags on the `run` parent command so
	// that the `all`, `apiserver` and `workers` subcommands inherit them
	// automatically. Individual subcommands ignore flags they do not consume.
	flags := c.Cmd().PersistentFlags()
	flags.StringVar(&c.config.Addr, "addr", c.config.Addr, "Bind address for the server")
	flags.StringVar(&c.config.UploadLocation, "upload-location", c.config.UploadLocation, "Location for the uploaded files")
	shared.RegisterDatabaseFlags(c.Cmd(), &c.dbConfig)
	flags.IntVar(&c.config.MaxConcurrentExports, "max-concurrent-exports", c.config.MaxConcurrentExports, "Maximum number of concurrent export processes")
	flags.IntVar(&c.config.MaxConcurrentImports, "max-concurrent-imports", c.config.MaxConcurrentImports, "Maximum number of concurrent import processes")
	flags.IntVar(&c.config.MaxConcurrentRestores, "max-concurrent-restores", c.config.MaxConcurrentRestores, "Maximum number of concurrent restore processes")
	flags.StringVar(&c.config.ExportPollInterval, "export-poll-interval", c.config.ExportPollInterval, "Export worker poll interval (e.g., 10s, 30s)")
	flags.StringVar(&c.config.ImportPollInterval, "import-poll-interval", c.config.ImportPollInterval, "Import worker poll interval (e.g., 10s, 30s)")
	flags.StringVar(&c.config.RestorePollInterval, "restore-poll-interval", c.config.RestorePollInterval, "Restore worker poll interval (e.g., 10s, 30s)")
	flags.StringVar(&c.config.RefreshTokenCleanupInterval, "refresh-token-cleanup-interval", c.config.RefreshTokenCleanupInterval, "Interval between refresh token cleanup runs (e.g., 1h, 30m)")
	flags.IntVar(&c.config.ThumbnailBatchSize, "thumbnail-batch-size", c.config.ThumbnailBatchSize, "Maximum thumbnail jobs processed per batch")
	flags.StringVar(&c.config.ThumbnailPollInterval, "thumbnail-poll-interval", c.config.ThumbnailPollInterval, "Thumbnail worker poll interval (e.g., 5s, 10s)")
	flags.StringVar(&c.config.ThumbnailCleanupInterval, "thumbnail-cleanup-interval", c.config.ThumbnailCleanupInterval, "Interval between thumbnail job cleanup runs (e.g., 5m)")
	flags.StringVar(&c.config.ThumbnailJobRetentionPeriod, "thumbnail-job-retention-period", c.config.ThumbnailJobRetentionPeriod, "How long completed thumbnail jobs are retained (e.g., 24h)")
	flags.StringVar(&c.config.ThumbnailJobBatchTimeout, "thumbnail-job-batch-timeout", c.config.ThumbnailJobBatchTimeout, "How long the thumbnail worker waits for an in-flight batch before polling again (e.g., 30s)")
	flags.StringVar(&c.config.DetachedThumbnailJobTimeout, "detached-thumbnail-job-timeout", c.config.DetachedThumbnailJobTimeout, "Per-job timeout for detached thumbnail generation (e.g., 2m)")
	flags.StringVar(&c.config.JWTSecret, "jwt-secret", c.config.JWTSecret, "JWT secret for authentication (minimum 32 characters, auto-generated if not provided)")
	flags.StringVar(&c.config.FileSigningKey, "file-signing-key", c.config.FileSigningKey, "File signing key for secure file URLs (minimum 32 characters, auto-generated if not provided)")
	flags.StringVar(&c.config.FileURLExpiration, "file-url-expiration", c.config.FileURLExpiration, "File URL expiration duration (e.g., 15m, 1h, 30s)")
	flags.StringVar(&c.config.TokenBlacklistRedisURL, "token-blacklist-redis-url", c.config.TokenBlacklistRedisURL, "Redis URL for token blacklist (e.g., redis://localhost:6379/0); omit to use in-memory blacklist")
	flags.StringVar(&c.config.AuthRateLimitRedisURL, "auth-rate-limit-redis-url", c.config.AuthRateLimitRedisURL, "Redis URL for auth rate limiting/lockout (e.g., redis://localhost:6379/0); omit to use in-memory limiter")
	flags.BoolVar(&c.config.AuthRateLimitDisabled, "no-auth-rate-limit", c.config.AuthRateLimitDisabled, "Disable auth rate limiting entirely (for testing only — do not use in production)")
	flags.StringVar(&c.config.GlobalRateLimitRedisURL, "global-rate-limit-redis-url", c.config.GlobalRateLimitRedisURL, "Redis URL for global API rate limiting (e.g., redis://localhost:6379/0); omit to use in-memory limiter")
	flags.IntVar(&c.config.GlobalRateLimit, "global-rate-limit", c.config.GlobalRateLimit, "Global per-IP request limit for API endpoints")
	flags.StringVar(&c.config.GlobalRateWindow, "global-rate-window", c.config.GlobalRateWindow, "Global API rate limit window duration (e.g., 1h, 30m)")
	flags.BoolVar(&c.config.GlobalRateLimitDisabled, "no-global-rate-limit", c.config.GlobalRateLimitDisabled, "Disable global API rate limiting entirely (for testing only — do not use in production)")
	flags.StringVar(&c.config.GlobalRateTrustedProxies, "global-rate-trusted-proxies", c.config.GlobalRateTrustedProxies, "Comma-separated trusted proxy CIDRs/IPs used when resolving client IP for global rate limiting")
	flags.StringVar(&c.config.CSRFRedisURL, "csrf-redis-url", c.config.CSRFRedisURL, "Redis URL for CSRF token storage (e.g., redis://localhost:6379/0); omit to use in-memory storage")
	flags.StringVar(&c.config.AllowedOrigins, "allowed-origins", c.config.AllowedOrigins, "Comma-separated list of allowed CORS origins (e.g., https://example.com)")
	flags.StringVar(&c.config.RegistrationMode, "registration-mode", c.config.RegistrationMode, "Registration mode: open (anyone can register), approval (admin must approve), or closed (registration disabled)")
	flags.StringVar(&c.config.PublicURL, "public-url", c.config.PublicURL, "Public base URL used in transactional email links (e.g., https://inventario.example.com)")

	flags.StringVar(&c.config.EmailProvider, "email-provider", c.config.EmailProvider, "Email provider: stub, smtp, sendgrid, ses, mandrill, or mailchimp")
	flags.BoolVar(&c.config.LogEmailURLs, "log-email-urls", c.config.LogEmailURLs, "Log full verification/password-reset URLs in stub email mode (includes sensitive tokens; unsafe for shared logs)")
	flags.StringVar(&c.config.EmailFrom, "email-from", c.config.EmailFrom, "From address for transactional emails")
	flags.StringVar(&c.config.EmailReplyTo, "email-reply-to", c.config.EmailReplyTo, "Reply-To address for transactional emails")
	flags.StringVar(&c.config.EmailQueueRedisURL, "email-queue-redis-url", c.config.EmailQueueRedisURL, "Redis URL for email queue (recommended for production); omit to use in-memory queue")
	flags.IntVar(&c.config.EmailQueueWorkers, "email-queue-workers", c.config.EmailQueueWorkers, "Number of email queue workers")
	flags.IntVar(&c.config.EmailQueueMaxRetries, "email-queue-max-retries", c.config.EmailQueueMaxRetries, "Maximum number of retries per failed email")

	flags.StringVar(&c.config.SMTPHost, "smtp-host", c.config.SMTPHost, "SMTP host")
	flags.IntVar(&c.config.SMTPPort, "smtp-port", c.config.SMTPPort, "SMTP port")
	flags.StringVar(&c.config.SMTPUsername, "smtp-username", c.config.SMTPUsername, "SMTP username")
	flags.StringVar(&c.config.SMTPPassword, "smtp-password", c.config.SMTPPassword, "SMTP password")
	flags.BoolVar(&c.config.SMTPUseTLS, "smtp-use-tls", c.config.SMTPUseTLS, "Use STARTTLS for SMTP")

	flags.StringVar(&c.config.SendGridAPIKey, "sendgrid-api-key", c.config.SendGridAPIKey, "SendGrid API key")
	flags.StringVar(&c.config.SendGridBaseURL, "sendgrid-base-url", c.config.SendGridBaseURL, "SendGrid API base URL")

	flags.StringVar(&c.config.AWSRegion, "aws-region", c.config.AWSRegion, "AWS region for SES (e.g., us-east-1)")

	flags.StringVar(&c.config.MandrillAPIKey, "mandrill-api-key", c.config.MandrillAPIKey, "Mandrill/Mailchimp Transactional API key")
	flags.StringVar(&c.config.MandrillBaseURL, "mandrill-base-url", c.config.MandrillBaseURL, "Mandrill API base URL")
}

// runAll wires the API server, every background worker and the email
// lifecycle together as a composition of small, independently startable
// primitives. It is the behavior invoked by `inventario run` (bare) and by
// `inventario run all`.
func (c *Command) runAll() error {
	rs, err := c.buildRuntimeSetup()
	if err != nil {
		return err
	}
	defer rs.closeReadinessRedisPinger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopEmail := c.startEmailLifecycle(ctx, rs)
	defer stopEmail()

	stopExport := c.startExportWorker(ctx, rs)
	defer stopExport()

	restoreWorker, stopRestore := c.startRestoreWorker(ctx, rs)
	defer stopRestore()

	stopImport := c.startImportWorker(ctx, rs)
	defer stopImport()

	stopThumbnail := c.startThumbnailWorker(ctx, rs)
	defer stopThumbnail()

	stopRefreshTokenCleanup := c.startRefreshTokenCleanupWorker(ctx, rs)
	defer stopRefreshTokenCleanup()

	srv, errCh := c.startAPIServer(rs, restoreWorker)
	return waitForShutdown(srv, errCh)
}

// runAPIServer starts the HTTP API server without any background worker
// goroutines. It uses a registry-backed RestoreStatusQuerier so the API can
// still enforce the "one active restore at a time" invariant in deployments
// where the RestoreWorker runs in a separate process.
func (c *Command) runAPIServer() error {
	rs, err := c.buildRuntimeSetup()
	if err != nil {
		return err
	}
	defer rs.closeReadinessRedisPinger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopEmail := c.startEmailLifecycle(ctx, rs)
	defer stopEmail()

	restoreStatus := restore.NewRegistryStatusQuerier(rs.factorySet.CreateServiceRegistrySet())
	srv, errCh := c.startAPIServer(rs, restoreStatus)
	return waitForShutdown(srv, errCh)
}

// runWorkers starts every background worker without opening the HTTP listener.
// It blocks until the process receives SIGINT or SIGTERM.
func (c *Command) runWorkers() error {
	rs, err := c.buildRuntimeSetup()
	if err != nil {
		return err
	}
	defer rs.closeReadinessRedisPinger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopEmail := c.startEmailLifecycle(ctx, rs)
	defer stopEmail()

	stopExport := c.startExportWorker(ctx, rs)
	defer stopExport()

	_, stopRestore := c.startRestoreWorker(ctx, rs)
	defer stopRestore()

	stopImport := c.startImportWorker(ctx, rs)
	defer stopImport()

	stopThumbnail := c.startThumbnailWorker(ctx, rs)
	defer stopThumbnail()

	stopRefreshTokenCleanup := c.startRefreshTokenCleanupWorker(ctx, rs)
	defer stopRefreshTokenCleanup()

	slog.Info("Workers started; waiting for shutdown signal")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	slog.Info("Shutting down workers")
	return nil
}

// runtimeSetup aggregates the shared state produced by the run bootstrap:
// registry factory, API server parameters, email lifecycle and validated
// worker duration flags. It is built once by buildRuntimeSetup and fed into
// the per-subsystem start functions so they can be composed by run, run
// apiserver and run workers alike.
type runtimeSetup struct {
	dsn                       string
	factorySet                *registry.FactorySet
	params                    apiserver.Params
	emailLifecycle            emailServiceLifecycle
	workerDurations           workerDurations
	closeReadinessRedisPinger func()
}

// buildRuntimeSetup performs the non-goroutine preamble of run: it logs the
// startup context, resolves the registry factory, seeds the in-memory default
// tenant, builds the API server parameters and validates every duration-valued
// worker flag. On any failure, previously allocated external resources (Redis
// readiness clients) are released before the error is returned.
func (c *Command) buildRuntimeSetup() (*runtimeSetup, error) {
	dsn := c.dbConfig.DBDSN

	c.logInventarioEnv()
	c.logStartupInfo(dsn)

	factorySet, err := c.resolveFactorySet(dsn)
	if err != nil {
		return nil, err
	}

	c.seedMemoryDBDefaultTenant(dsn, factorySet)

	serverSetup, err := c.buildServerParams(factorySet, dsn)
	if err != nil {
		return nil, err
	}

	// Validate duration-valued worker flags up front so misconfiguration
	// fails fast without starting any background goroutines or the HTTP listener.
	durations, err := c.parseWorkerDurations()
	if err != nil {
		serverSetup.closeReadinessRedisPinger()
		return nil, err
	}

	return &runtimeSetup{
		dsn:                       dsn,
		factorySet:                factorySet,
		params:                    serverSetup.params,
		emailLifecycle:            serverSetup.emailLifecycle,
		workerDurations:           durations,
		closeReadinessRedisPinger: serverSetup.closeReadinessRedisPinger,
	}, nil
}

// logInventarioEnv emits every INVENTARIO_-prefixed environment variable name
// (values are intentionally omitted to avoid leaking secrets) to aid
// configuration troubleshooting.
func (c *Command) logInventarioEnv() {
	for _, e := range os.Environ() {
		name, _, _ := strings.Cut(e, "=")
		if strings.HasPrefix(name, "INVENTARIO_") {
			slog.Info("Environment variable", "name", name)
		}
	}
}

// logStartupInfo prints the bind address and the DSN with credentials masked.
func (c *Command) logStartupInfo(dsn string) {
	parsedDSN := must.Must(registry.Config(dsn).Parse())
	if parsedDSN.User != nil {
		parsedDSN.User = url.UserPassword("xxxxxx", "xxxxxx")
	}
	slog.Info("Starting server", "addr", c.config.Addr, "db-dsn", parsedDSN.String())
}

// resolveFactorySet selects the registry implementation that matches the DSN
// scheme and instantiates its factory set.
func (c *Command) resolveFactorySet(dsn string) (*registry.FactorySet, error) {
	registrySetFn, ok := registry.GetRegistry(dsn)
	if !ok {
		slog.Error("Unknown registry", "dsn", dsn)
		return nil, errors.New("unknown registry")
	}
	slog.Info("Selected database registry", "registry_type", fmt.Sprintf("%T", registrySetFn))

	factorySet, err := registrySetFn(registry.Config(dsn))
	if err != nil {
		slog.Error("Failed to setup registry", "error", err)
		return nil, err
	}
	return factorySet, nil
}

// seedMemoryDBDefaultTenant creates a default tenant in memory-db mode so
// PublicTenantMiddleware can resolve it without manual setup steps. Other
// backends are expected to have a tenant seeded via migrations or the CLI.
func (c *Command) seedMemoryDBDefaultTenant(dsn string, factorySet *registry.FactorySet) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(dsn)), "memory://") {
		return
	}
	defaultTenant := models.Tenant{
		Name:      "Default Tenant",
		Slug:      "default",
		Status:    models.TenantStatusActive,
		IsDefault: true,
	}
	if _, err := factorySet.TenantRegistry.Create(context.Background(), defaultTenant); err != nil {
		slog.Warn("Failed to seed default tenant in memory-db mode", "error", err)
	}
}

// startEmailLifecycle starts the configured email service (if any) and returns
// the matching stop function.
func (c *Command) startEmailLifecycle(ctx context.Context, rs *runtimeSetup) func() {
	rs.emailLifecycle.start(ctx)
	return rs.emailLifecycle.stop
}

// startExportWorker wires and starts the export worker and returns its stop
// function.
func (c *Command) startExportWorker(ctx context.Context, rs *runtimeSetup) func() {
	service := export.NewExportService(rs.factorySet, rs.params.UploadLocation)
	worker := export.NewExportWorker(
		service, rs.factorySet, c.config.MaxConcurrentExports,
		export.WithPollInterval(rs.workerDurations.exportPollInterval),
	)
	worker.Start(ctx)
	return worker.Stop
}

// startImportWorker wires and starts the import worker and returns its stop
// function.
func (c *Command) startImportWorker(ctx context.Context, rs *runtimeSetup) func() {
	service := importpkg.NewImportService(rs.factorySet, rs.params.UploadLocation)
	worker := importpkg.NewImportWorker(
		service, rs.factorySet, c.config.MaxConcurrentImports,
		importpkg.WithPollInterval(rs.workerDurations.importPollInterval),
	)
	worker.Start(ctx)
	return worker.Stop
}

// startRestoreWorker wires and starts the restore worker, returning both the
// worker (needed by the API server to satisfy the RestoreStatusQuerier
// interface) and its stop function.
func (c *Command) startRestoreWorker(ctx context.Context, rs *runtimeSetup) (*restore.RestoreWorker, func()) {
	service := restore.NewRestoreService(rs.factorySet, rs.params.EntityService, rs.params.UploadLocation)
	worker := restore.NewRestoreWorker(
		service, rs.factorySet.CreateServiceRegistrySet(), rs.params.UploadLocation,
		restore.WithPollInterval(rs.workerDurations.restorePollInterval),
		restore.WithMaxConcurrent(c.config.MaxConcurrentRestores),
	)
	worker.Start(ctx)
	return worker, worker.Stop
}

// startThumbnailWorker wires and starts the thumbnail generation worker and
// returns its stop function.
func (c *Command) startThumbnailWorker(ctx context.Context, rs *runtimeSetup) func() {
	worker := services.NewThumbnailGenerationWorker(
		rs.factorySet, rs.params.UploadLocation, rs.params.ThumbnailConfig,
		services.WithThumbnailPollInterval(rs.workerDurations.thumbnailPollInterval),
		services.WithThumbnailBatchSize(c.config.ThumbnailBatchSize),
		services.WithThumbnailCleanupInterval(rs.workerDurations.thumbnailCleanupInterval),
		services.WithThumbnailJobRetentionPeriod(rs.workerDurations.thumbnailJobRetentionPeriod),
		services.WithThumbnailJobBatchTimeout(rs.workerDurations.thumbnailJobBatchTimeout),
		services.WithDetachedThumbnailJobTimeout(rs.workerDurations.detachedThumbnailJobTimeout),
	)
	worker.Start(ctx)
	return worker.Stop
}

// startRefreshTokenCleanupWorker wires and starts the refresh token cleanup
// worker (which deletes expired tokens on the configured interval) and returns
// its stop function.
func (c *Command) startRefreshTokenCleanupWorker(ctx context.Context, rs *runtimeSetup) func() {
	worker := services.NewRefreshTokenCleanupWorker(
		rs.factorySet.RefreshTokenRegistry,
		services.WithRefreshTokenCleanupInterval(rs.workerDurations.refreshTokenCleanupInterval),
	)
	worker.Start(ctx)
	return worker.Stop
}

// startAPIServer starts the HTTP listener on the configured address and
// returns the server handle plus the error channel produced by httpserver.Run.
func (c *Command) startAPIServer(rs *runtimeSetup, restoreStatus apiserver.RestoreStatusQuerier) (*httpserver.APIServer, <-chan error) {
	srv := &httpserver.APIServer{}
	errCh := srv.Run(c.config.Addr, apiserver.APIServer(rs.params, restoreStatus))
	return srv, errCh
}

// waitForShutdown blocks until the API server reports a startup error or the
// process receives SIGINT/SIGTERM, then issues a graceful shutdown.
func waitForShutdown(srv *httpserver.APIServer, errCh <-chan error) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigCh:
	case err := <-errCh:
		slog.Error("Failure during server startup", "error", err)
		return err
	}

	slog.Info("Shutting down server")
	if err := srv.Shutdown(); err != nil {
		slog.Error("Failure during server shutdown", "error", err)
		return err
	}
	return nil
}

type serverSetup struct {
	params                    apiserver.Params
	emailLifecycle            emailServiceLifecycle
	closeReadinessRedisPinger func()
}

type workerDurations struct {
	exportPollInterval          time.Duration
	importPollInterval          time.Duration
	restorePollInterval         time.Duration
	refreshTokenCleanupInterval time.Duration
	thumbnailPollInterval       time.Duration
	thumbnailCleanupInterval    time.Duration
	thumbnailJobRetentionPeriod time.Duration
	thumbnailJobBatchTimeout    time.Duration
	detachedThumbnailJobTimeout time.Duration
}

func parseWorkerDuration(flagName, value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid --%s %q: %w", flagName, value, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid --%s %q: must be positive", flagName, value)
	}
	return d, nil
}

func (c *Command) parseWorkerDurations() (workerDurations, error) {
	var out workerDurations
	specs := []struct {
		flag  string
		value string
		dst   *time.Duration
	}{
		{"export-poll-interval", c.config.ExportPollInterval, &out.exportPollInterval},
		{"import-poll-interval", c.config.ImportPollInterval, &out.importPollInterval},
		{"restore-poll-interval", c.config.RestorePollInterval, &out.restorePollInterval},
		{"refresh-token-cleanup-interval", c.config.RefreshTokenCleanupInterval, &out.refreshTokenCleanupInterval},
		{"thumbnail-poll-interval", c.config.ThumbnailPollInterval, &out.thumbnailPollInterval},
		{"thumbnail-cleanup-interval", c.config.ThumbnailCleanupInterval, &out.thumbnailCleanupInterval},
		{"thumbnail-job-retention-period", c.config.ThumbnailJobRetentionPeriod, &out.thumbnailJobRetentionPeriod},
		{"thumbnail-job-batch-timeout", c.config.ThumbnailJobBatchTimeout, &out.thumbnailJobBatchTimeout},
		{"detached-thumbnail-job-timeout", c.config.DetachedThumbnailJobTimeout, &out.detachedThumbnailJobTimeout},
	}
	for _, spec := range specs {
		d, err := parseWorkerDuration(spec.flag, spec.value)
		if err != nil {
			slog.Error("Failed to parse worker duration", "flag", spec.flag, "error", err)
			return workerDurations{}, err
		}
		*spec.dst = d
	}
	return out, nil
}

func (c *Command) buildServerParams(factorySet *registry.FactorySet, dsn string) (_ serverSetup, err error) {
	params := apiserver.Params{
		FactorySet:     factorySet,
		UploadLocation: c.config.UploadLocation,
		StartTime:      time.Now(),
	}
	params.EntityService = services.NewEntityService(factorySet, params.UploadLocation)
	params.DebugInfo = debug.NewInfo(dsn, params.UploadLocation)

	// Configure JWT secret from config/environment or generate a secure default.
	jwtSecret, err := getJWTSecret(c.config.JWTSecret)
	if err != nil {
		slog.Error("Failed to configure JWT secret", "error", err)
		return serverSetup{}, err
	}

	// Configure file signing key from config/environment or generate a secure default.
	fileSigningKey, err := getFileSigningKey(c.config.FileSigningKey)
	if err != nil {
		slog.Error("Failed to configure file signing key", "error", err)
		return serverSetup{}, err
	}

	// Parse file URL expiration duration.
	fileURLExpiration, err := time.ParseDuration(c.config.FileURLExpiration)
	if err != nil {
		slog.Error("Failed to parse file URL expiration duration", "error", err, "duration", c.config.FileURLExpiration)
		return serverSetup{}, err
	}

	// Parse thumbnail slot duration and create thumbnail config.
	thumbnailSlotDuration, err := time.ParseDuration(c.config.ThumbnailSlotDuration)
	if err != nil {
		slog.Error("Failed to parse thumbnail slot duration", "error", err, "duration", c.config.ThumbnailSlotDuration)
		return serverSetup{}, err
	}

	params.JWTSecret = jwtSecret
	params.FileSigningKey = fileSigningKey
	params.FileURLExpiration = fileURLExpiration
	params.ThumbnailConfig = services.ThumbnailGenerationConfig{
		MaxConcurrentPerUser: c.config.ThumbnailMaxConcurrentPerUser,
		RateLimitPerMinute:   c.config.ThumbnailRateLimitPerMinute,
		SlotDuration:         thumbnailSlotDuration,
	}
	params.TokenBlacklister = services.NewTokenBlacklister(c.config.TokenBlacklistRedisURL)
	if c.config.AuthRateLimitDisabled {
		slog.Warn("Auth rate limiting is disabled via configuration — do not use this in production")
		params.AuthRateLimiter = services.NewNoOpAuthRateLimiter()
	} else {
		params.AuthRateLimiter = services.NewAuthRateLimiter(c.config.AuthRateLimitRedisURL)
	}
	if c.config.GlobalRateLimitDisabled {
		slog.Warn("Global API rate limiting is disabled via configuration — do not use this in production")
		params.GlobalRateLimiter = services.NewNoOpGlobalRateLimiter()
	} else {
		globalRateWindow, parseErr := time.ParseDuration(c.config.GlobalRateWindow)
		if parseErr != nil {
			slog.Error("Failed to parse global rate window duration", "error", parseErr, "duration", c.config.GlobalRateWindow)
			return serverSetup{}, parseErr
		}
		params.GlobalRateLimiter = services.NewGlobalRateLimiter(c.config.GlobalRateLimitRedisURL, c.config.GlobalRateLimit, globalRateWindow)
	}

	params.GlobalRateTrustedProxyNets, err = apiserver.ParseTrustedProxyCIDRs(c.config.GlobalRateTrustedProxies)
	if err != nil {
		slog.Error("Failed to parse global rate trusted proxies", "error", err)
		return serverSetup{}, err
	}

	params.CSRFService = services.NewCSRFService(c.config.CSRFRedisURL)
	params.RedisPinger = c.newReadinessRedisPinger()
	closeReadinessRedisPinger := func() {}
	if closer, ok := params.RedisPinger.(interface{ Close() error }); ok {
		closeReadinessRedisPinger = func() {
			if closeErr := closer.Close(); closeErr != nil {
				slog.Warn("Failed to close Redis readiness client(s)", "error", closeErr)
			}
		}
	}
	// Release Redis readiness clients on any failure path below. On success the
	// closer is returned in serverSetup and the caller owns its lifetime.
	defer func() {
		if err != nil {
			closeReadinessRedisPinger()
		}
	}()

	// Parse allowed origins (comma-separated) with fail-closed default.
	params.CORSConfig = apiserver.DefaultCORSConfig()
	params.CORSConfig.AllowedOrigins, err = apiserver.ParseAllowedOrigins(c.config.AllowedOrigins)
	if err != nil {
		slog.Error("Failed to parse allowed CORS origins", "error", err)
		return serverSetup{}, err
	}
	if len(params.CORSConfig.AllowedOrigins) == 0 {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(dsn)), "memory://") {
			params.CORSConfig.AllowedOrigins = apiserver.DefaultDevAllowedOrigins()
			slog.Warn("No CORS origins explicitly configured; using local development defaults in memory-db mode. Set --allowed-origins for custom values.")
		} else {
			slog.Warn("No CORS origins explicitly configured; cross-origin requests are denied. Set --allowed-origins to allow specific origins.")
		}
	}

	// Set registration mode from config (defaults to "open" when unset).
	params.RegistrationMode = models.RegistrationMode(c.config.RegistrationMode)
	params.PublicURL = strings.TrimSpace(c.config.PublicURL)
	if err = validateEmailPublicURLConfig(c.config.EmailProvider, params.PublicURL); err != nil {
		return serverSetup{}, err
	}

	emailLifecycle, err := c.buildEmailService()
	if err != nil {
		slog.Error("Failed to initialize email service", "error", err)
		return serverSetup{}, err
	}
	params.EmailService = emailLifecycle.service

	if err = validation.Validate(params); err != nil {
		slog.Error("Invalid server parameters", "error", err)
		return serverSetup{}, err
	}

	return serverSetup{
		params:                    params,
		emailLifecycle:            emailLifecycle,
		closeReadinessRedisPinger: closeReadinessRedisPinger,
	}, nil
}

type redisReadinessTarget struct {
	name   string
	client *redis.Client
}

type readinessRedisPinger struct {
	targets []redisReadinessTarget
}

func (p *readinessRedisPinger) Ping(ctx context.Context) error {
	for _, target := range p.targets {
		if err := target.client.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("%s dependency ping failed: %w", target.name, err)
		}
	}

	return nil
}

func (p *readinessRedisPinger) Close() error {
	closeErrs := make([]error, 0)
	for _, target := range p.targets {
		if err := target.client.Close(); err != nil {
			closeErrs = append(closeErrs, fmt.Errorf("%s dependency close failed: %w", target.name, err))
		}
	}
	if len(closeErrs) == 0 {
		return nil
	}
	return errors.Join(closeErrs...)
}

func (c *Command) newReadinessRedisPinger() apiserver.RedisPinger {
	type redisDependency struct {
		name string
		url  string
	}

	deps := make([]redisDependency, 0, 4)
	if redisURL := strings.TrimSpace(c.config.TokenBlacklistRedisURL); redisURL != "" {
		deps = append(deps, redisDependency{name: "token_blacklist", url: redisURL})
	}
	if !c.config.AuthRateLimitDisabled {
		if redisURL := strings.TrimSpace(c.config.AuthRateLimitRedisURL); redisURL != "" {
			deps = append(deps, redisDependency{name: "auth_rate_limit", url: redisURL})
		}
	}
	if !c.config.GlobalRateLimitDisabled {
		if redisURL := strings.TrimSpace(c.config.GlobalRateLimitRedisURL); redisURL != "" {
			deps = append(deps, redisDependency{name: "global_rate_limit", url: redisURL})
		}
	}
	if redisURL := strings.TrimSpace(c.config.CSRFRedisURL); redisURL != "" {
		deps = append(deps, redisDependency{name: "csrf", url: redisURL})
	}
	if len(deps) == 0 {
		return nil
	}
	groupedNamesByURL := make(map[string][]string, len(deps))
	orderedURLs := make([]string, 0, len(deps))
	for _, dep := range deps {
		if _, exists := groupedNamesByURL[dep.url]; !exists {
			orderedURLs = append(orderedURLs, dep.url)
		}
		groupedNamesByURL[dep.url] = append(groupedNamesByURL[dep.url], dep.name)
	}

	targets := make([]redisReadinessTarget, 0, len(orderedURLs))
	for _, redisURL := range orderedURLs {
		dependencyNames := groupedNamesByURL[redisURL]
		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			slog.Warn(
				"Invalid Redis URL for readiness check; Redis dependency checks will be skipped",
				"dependencies",
				strings.Join(dependencyNames, ","),
				"error",
				err,
			)
			continue
		}
		targets = append(targets, redisReadinessTarget{
			name:   strings.Join(dependencyNames, ","),
			client: redis.NewClient(opts),
		})
	}

	if len(targets) == 0 {
		return nil
	}

	return &readinessRedisPinger{targets: targets}
}

type emailServiceLifecycle struct {
	service services.EmailService
	start   func(context.Context)
	stop    func()
}

func normalizeEmailProvider(raw string) services.EmailProvider {
	provider := services.EmailProvider(strings.ToLower(strings.TrimSpace(raw)))
	if provider == "" {
		provider = services.EmailProviderStub
	}
	return provider
}

func (c *Command) buildEmailService() (emailServiceLifecycle, error) {
	provider := normalizeEmailProvider(c.config.EmailProvider)

	if provider == services.EmailProviderStub {
		svc := services.NewStubEmailService(services.WithLogEmailURLs(c.config.LogEmailURLs))
		return emailServiceLifecycle{
			service: svc,
			start:   func(context.Context) {},
			stop:    func() {},
		}, nil
	}

	asyncSvc, err := services.NewAsyncEmailService(services.EmailConfig{
		Provider:        provider,
		From:            c.config.EmailFrom,
		ReplyTo:         c.config.EmailReplyTo,
		QueueRedisURL:   c.config.EmailQueueRedisURL,
		QueueWorkers:    c.config.EmailQueueWorkers,
		QueueMaxRetry:   c.config.EmailQueueMaxRetries,
		SMTPHost:        c.config.SMTPHost,
		SMTPPort:        c.config.SMTPPort,
		SMTPUsername:    c.config.SMTPUsername,
		SMTPPassword:    c.config.SMTPPassword,
		SMTPUseTLS:      c.config.SMTPUseTLS,
		SendGridAPIKey:  c.config.SendGridAPIKey,
		SendGridBaseURL: c.config.SendGridBaseURL,
		AWSRegion:       c.config.AWSRegion,
		MandrillAPIKey:  c.config.MandrillAPIKey,
		MandrillBaseURL: c.config.MandrillBaseURL,
	})
	if err != nil {
		return emailServiceLifecycle{}, err
	}

	return emailServiceLifecycle{
		service: asyncSvc,
		start:   asyncSvc.Start,
		stop:    asyncSvc.Stop,
	}, nil
}

func validatePublicURLForTransactionalEmails(publicURL string) error {
	base := strings.TrimSpace(publicURL)
	if base == "" {
		return errors.New("public URL is required")
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("scheme and host are required")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	return nil
}

func validateEmailPublicURLConfig(provider, publicURL string) error {
	normalizedEmailProvider := normalizeEmailProvider(provider)

	switch normalizedEmailProvider {
	case services.EmailProviderStub:
		return nil
	case services.EmailProviderSMTP,
		services.EmailProviderSendGrid,
		services.EmailProviderSES,
		services.EmailProviderMandrill,
		services.EmailProviderMailchimp:
		if err := validatePublicURLForTransactionalEmails(publicURL); err != nil {
			return fmt.Errorf("invalid --public-url for email provider %q: %w", normalizedEmailProvider, err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported email provider: %q", normalizedEmailProvider)
	}
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
