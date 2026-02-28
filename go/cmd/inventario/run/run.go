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
	flags.StringVar(&c.config.TokenBlacklistRedisURL, "token-blacklist-redis-url", c.config.TokenBlacklistRedisURL, "Redis URL for token blacklist (e.g., redis://localhost:6379/0); omit to use in-memory blacklist")
	flags.StringVar(&c.config.AuthRateLimitRedisURL, "auth-rate-limit-redis-url", c.config.AuthRateLimitRedisURL, "Redis URL for auth rate limiting/lockout (e.g., redis://localhost:6379/0); omit to use in-memory limiter")
	flags.BoolVar(&c.config.AuthRateLimitDisabled, "no-auth-rate-limit", c.config.AuthRateLimitDisabled, "Disable auth rate limiting entirely (for testing only — do not use in production)")
	flags.StringVar(&c.config.CSRFRedisURL, "csrf-redis-url", c.config.CSRFRedisURL, "Redis URL for CSRF token storage (e.g., redis://localhost:6379/0); omit to use in-memory storage")
	flags.StringVar(&c.config.AllowedOrigins, "allowed-origins", c.config.AllowedOrigins, "Comma-separated list of allowed CORS origins (e.g., https://example.com); leave empty in development for AllowAll")
	flags.StringVar(&c.config.RegistrationMode, "registration-mode", c.config.RegistrationMode, "Registration mode: open (anyone can register), approval (admin must approve), or closed (registration disabled)")
	flags.StringVar(&c.config.PublicURL, "public-url", c.config.PublicURL, "Public base URL used in transactional email links (e.g., https://inventario.example.com)")

	flags.StringVar(&c.config.EmailProvider, "email-provider", c.config.EmailProvider, "Email provider: stub, smtp, sendgrid, ses, mandrill, or mailchimp")
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
	params.TokenBlacklister = services.NewTokenBlacklister(c.config.TokenBlacklistRedisURL)
	if c.config.AuthRateLimitDisabled {
		slog.Warn("Auth rate limiting is disabled via configuration — do not use this in production")
		params.AuthRateLimiter = services.NewNoOpAuthRateLimiter()
	} else {
		params.AuthRateLimiter = services.NewAuthRateLimiter(c.config.AuthRateLimitRedisURL)
	}

	params.CSRFService = services.NewCSRFService(c.config.CSRFRedisURL)

	// Parse allowed origins (comma-separated). An empty value means AllowAll (dev mode).
	if c.config.AllowedOrigins != "" {
		for origin := range strings.SplitSeq(c.config.AllowedOrigins, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				params.AllowedOrigins = append(params.AllowedOrigins, origin)
			}
		}
	}

	// Set registration mode from config (defaults to "open" when unset).
	params.RegistrationMode = models.RegistrationMode(c.config.RegistrationMode)
	params.PublicURL = strings.TrimSpace(c.config.PublicURL)

	if err := validateEmailPublicURLConfig(c.config.EmailProvider, params.PublicURL); err != nil {
		return err
	}

	emailService, err := services.NewAsyncEmailService(services.EmailConfig{
		Provider:        services.EmailProvider(c.config.EmailProvider),
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
		slog.Error("Failed to initialize email service", "error", err)
		return err
	}
	params.EmailService = emailService

	err = validation.Validate(params)
	if err != nil {
		slog.Error("Invalid server parameters", "error", err)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emailService.Start(ctx)
	defer emailService.Stop()

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

	// Start refresh token cleanup worker (deletes expired tokens every hour)
	refreshTokenCleanupWorker := services.NewRefreshTokenCleanupWorker(factorySet.RefreshTokenRegistry)
	refreshTokenCleanupWorker.Start(ctx)
	defer refreshTokenCleanupWorker.Stop()

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
	normalizedEmailProvider := services.EmailProvider(strings.ToLower(strings.TrimSpace(provider)))
	if normalizedEmailProvider == "" {
		normalizedEmailProvider = services.EmailProviderStub
	}

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
