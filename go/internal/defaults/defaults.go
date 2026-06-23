package defaults

import (
	"os"
	"path/filepath"
	"strings"
)

// Server contains default values for server configuration
type Server struct {
	Addr              string
	UploadLocation    string
	JWTSecret         string
	FileSigningKey    string
	FileURLExpiration string
}

// Database contains default values for database configuration
type Database struct {
	DSN string
}

// Workers contains default values for worker configuration
type Workers struct {
	MaxConcurrentExports             int
	MaxConcurrentImports             int
	MaxConcurrentRestores            int
	ExportPollInterval               string // Export worker poll interval (e.g., "10s")
	ImportPollInterval               string // Import worker poll interval (e.g., "10s")
	RestorePollInterval              string // Restore worker poll interval (e.g., "10s")
	RefreshTokenCleanupInterval      string // Refresh token cleanup interval (e.g., "1h")
	EmailVerificationCleanupInterval string // Email verification token cleanup interval (e.g., "1h")
	MagicLinkTokenCleanupInterval    string // Magic-link sign-in token cleanup interval (e.g., "1h")
	OperationSlotCleanupInterval     string // Operation-slot cleanup interval (e.g., "5m")
	GroupPurgeInterval               string // Group purge worker interval (e.g., "5m")
	WarrantyReminderInterval         string // Warranty reminder worker interval (e.g., "1h")
	StorageQuotaReminderInterval     string // Storage quota warning worker interval (e.g., "1h")
	LoanReminderInterval             string // Loan reminder worker interval (e.g., "1h")
	LoanReminderDueSoonDays          int    // Forward-looking window for the loan due-soon reminder (default 7)
	MaintenanceReminderInterval      string // Maintenance reminder worker interval (e.g., "1h")
	CurrencyMigrationInterval        string // Currency migration worker active-poll interval (e.g., "5s")
	BusinessMetricsInterval          string // Business-metrics collector interval (e.g., "60s")
	WorkerControlRefreshInterval     string // Worker soft-pause control poll interval (e.g., "10s")
}

// ThumbnailGeneration contains default values for thumbnail generation configuration
type ThumbnailGeneration struct {
	MaxConcurrentPerUser int    // Maximum simultaneous thumbnail generation jobs per user
	RateLimitPerMinute   int    // Maximum thumbnail generation requests per minute per user
	SlotDuration         string // Duration for which a concurrency slot is held (e.g., "30m")
	BatchSize            int    // Maximum thumbnail jobs processed per batch
	PollInterval         string // Thumbnail worker poll interval (e.g., "5s")
	CleanupInterval      string // Interval between completed-job cleanups (e.g., "5m")
	JobRetentionPeriod   string // How long completed thumbnail jobs are retained (e.g., "24h")
	JobBatchTimeout      string // Max wait for an in-flight batch before polling again (e.g., "30s")
	DetachedJobTimeout   string // Per-job timeout for detached thumbnail generation (e.g., "2m")
}

// Config contains all default configuration values
type Config struct {
	Server              Server
	Database            Database
	Workers             Workers
	ThumbnailGeneration ThumbnailGeneration
}

// getFileURL generates a file URL for the given path, similar to the function in run command
func getFileURL(path string) string {
	absPath, err := os.Getwd()
	if err != nil {
		// Fallback to relative path if we can't get working directory
		return "file://./" + path + "?create_dir=1"
	}

	absPath = filepath.ToSlash(filepath.Join(absPath, path))
	if strings.Contains(absPath, ":") {
		absPath = "/" + absPath // Ensure the drive letter is prefixed with a slash
	}
	return "file://" + absPath + "?create_dir=1"
}

// New returns the default configuration values
func New() Config {
	return Config{
		Server: Server{
			Addr:              ":3333",
			UploadLocation:    getFileURL("uploads"),
			JWTSecret:         "",    // Empty by default, will be generated if not provided
			FileSigningKey:    "",    // Empty by default, will be generated if not provided
			FileURLExpiration: "15m", // Default to 15 minutes for security
		},
		Database: Database{
			DSN: "memory://",
		},
		Workers: Workers{
			MaxConcurrentExports:             3,
			MaxConcurrentImports:             1,
			MaxConcurrentRestores:            1,
			ExportPollInterval:               "10s",
			ImportPollInterval:               "10s",
			RestorePollInterval:              "10s",
			RefreshTokenCleanupInterval:      "1h",
			EmailVerificationCleanupInterval: "1h",
			MagicLinkTokenCleanupInterval:    "1h",
			OperationSlotCleanupInterval:     "5m",
			GroupPurgeInterval:               "5m",
			WarrantyReminderInterval:         "1h",
			StorageQuotaReminderInterval:     "1h",
			LoanReminderInterval:             "1h",
			LoanReminderDueSoonDays:          7,
			MaintenanceReminderInterval:      "1h",
			CurrencyMigrationInterval:        "5s",
			BusinessMetricsInterval:          "60s",
			WorkerControlRefreshInterval:     "10s",
		},
		ThumbnailGeneration: ThumbnailGeneration{
			MaxConcurrentPerUser: 5,     // Maximum 5 simultaneous thumbnail generation jobs per user
			RateLimitPerMinute:   50,    // Maximum 50 thumbnail generation requests per minute per user
			SlotDuration:         "30m", // Hold concurrency slots for 30 minutes
			BatchSize:            10,    // Process up to 10 thumbnail jobs per batch
			PollInterval:         "5s",  // Check for new thumbnail jobs every 5 seconds
			CleanupInterval:      "5m",  // Cleanup completed jobs every 5 minutes
			JobRetentionPeriod:   "24h", // Keep completed jobs for 24 hours
			JobBatchTimeout:      "30s", // Wait up to 30 seconds for a batch before polling again
			DetachedJobTimeout:   "2m",  // Bound each detached thumbnail job
		},
	}
}

var defaultConfig = New()

// GetServerAddr returns the default server address
func GetServerAddr() string {
	return defaultConfig.Server.Addr
}

// GetUploadLocation returns the default upload location
func GetUploadLocation() string {
	return defaultConfig.Server.UploadLocation
}

// GetDatabaseDSN returns the default database DSN
func GetDatabaseDSN() string {
	return defaultConfig.Database.DSN
}

// GetMaxConcurrentExports returns the default max concurrent exports
func GetMaxConcurrentExports() int {
	return defaultConfig.Workers.MaxConcurrentExports
}

// GetMaxConcurrentImports returns the default max concurrent imports
func GetMaxConcurrentImports() int {
	return defaultConfig.Workers.MaxConcurrentImports
}

// GetJWTSecret returns the default JWT secret
func GetJWTSecret() string {
	return defaultConfig.Server.JWTSecret
}

// GetThumbnailMaxConcurrentPerUser returns the default max concurrent thumbnail generation jobs per user
func GetThumbnailMaxConcurrentPerUser() int {
	return defaultConfig.ThumbnailGeneration.MaxConcurrentPerUser
}

// GetThumbnailRateLimitPerMinute returns the default thumbnail generation rate limit per minute per user
func GetThumbnailRateLimitPerMinute() int {
	return defaultConfig.ThumbnailGeneration.RateLimitPerMinute
}

// GetThumbnailSlotDuration returns the default thumbnail generation slot duration
func GetThumbnailSlotDuration() string {
	return defaultConfig.ThumbnailGeneration.SlotDuration
}

// GetMaxConcurrentRestores returns the default max concurrent restores
func GetMaxConcurrentRestores() int {
	return defaultConfig.Workers.MaxConcurrentRestores
}

// GetExportPollInterval returns the default export worker poll interval
func GetExportPollInterval() string {
	return defaultConfig.Workers.ExportPollInterval
}

// GetImportPollInterval returns the default import worker poll interval
func GetImportPollInterval() string {
	return defaultConfig.Workers.ImportPollInterval
}

// GetRestorePollInterval returns the default restore worker poll interval
func GetRestorePollInterval() string {
	return defaultConfig.Workers.RestorePollInterval
}

// GetRefreshTokenCleanupInterval returns the default refresh token cleanup interval
func GetRefreshTokenCleanupInterval() string {
	return defaultConfig.Workers.RefreshTokenCleanupInterval
}

// GetEmailVerificationCleanupInterval returns the default email verification token cleanup interval
func GetEmailVerificationCleanupInterval() string {
	return defaultConfig.Workers.EmailVerificationCleanupInterval
}

// GetMagicLinkTokenCleanupInterval returns the default magic-link sign-in token cleanup interval
func GetMagicLinkTokenCleanupInterval() string {
	return defaultConfig.Workers.MagicLinkTokenCleanupInterval
}

// GetOperationSlotCleanupInterval returns the default operation-slot cleanup interval
func GetOperationSlotCleanupInterval() string {
	return defaultConfig.Workers.OperationSlotCleanupInterval
}

// GetGroupPurgeInterval returns the default interval between group purge sweeps.
func GetGroupPurgeInterval() string {
	return defaultConfig.Workers.GroupPurgeInterval
}

// GetWarrantyReminderInterval returns the default interval between
// warranty reminder sweeps.
func GetWarrantyReminderInterval() string {
	return defaultConfig.Workers.WarrantyReminderInterval
}

// GetStorageQuotaReminderInterval returns the default interval
// between storage quota warning sweeps (#1585).
func GetStorageQuotaReminderInterval() string {
	return defaultConfig.Workers.StorageQuotaReminderInterval
}

// GetLoanReminderInterval returns the default interval between loan
// reminder sweeps (#1509). Mirrors warranty / storage quota: hourly
// cadence is the right granularity for a date-window scan.
func GetLoanReminderInterval() string {
	return defaultConfig.Workers.LoanReminderInterval
}

// GetLoanReminderDueSoonDays returns the default forward-looking
// window for the due-soon kind. Configurable per-user is explicitly
// out of scope for v1 (issue #1509 "Out of scope: Per-loan custom
// reminder cadence").
func GetLoanReminderDueSoonDays() int {
	return defaultConfig.Workers.LoanReminderDueSoonDays
}

// GetMaintenanceReminderInterval returns the default interval
// between maintenance reminder sweeps (#1368).
func GetMaintenanceReminderInterval() string {
	return defaultConfig.Workers.MaintenanceReminderInterval
}

// GetCurrencyMigrationInterval returns the default active-poll interval
// for the currency migration worker. The worker switches to a 1m idle
// cadence when no pending rows exist, so this is the latency-sensitive
// knob the operator tunes.
func GetCurrencyMigrationInterval() string {
	return defaultConfig.Workers.CurrencyMigrationInterval
}

// GetBusinessMetricsInterval returns the default interval between
// business-metrics collection sweeps (#843). 60s is frequent enough for
// installation-wide gauges (tenants/users/storage move slowly) without
// adding meaningful aggregate-query load.
func GetBusinessMetricsInterval() string {
	return defaultConfig.Workers.BusinessMetricsInterval
}

// GetWorkerControlRefreshInterval returns the default poll interval for
// the background-worker soft-pause controller (#1308). 10s keeps a
// pause/resume flip visible within a few seconds while adding negligible
// DB load (one indexed List per interval).
func GetWorkerControlRefreshInterval() string {
	return defaultConfig.Workers.WorkerControlRefreshInterval
}

// GetThumbnailBatchSize returns the default thumbnail worker batch size
func GetThumbnailBatchSize() int {
	return defaultConfig.ThumbnailGeneration.BatchSize
}

// GetThumbnailPollInterval returns the default thumbnail worker poll interval
func GetThumbnailPollInterval() string {
	return defaultConfig.ThumbnailGeneration.PollInterval
}

// GetThumbnailCleanupInterval returns the default interval between completed-job cleanups
func GetThumbnailCleanupInterval() string {
	return defaultConfig.ThumbnailGeneration.CleanupInterval
}

// GetThumbnailJobRetentionPeriod returns the default retention period for completed thumbnail jobs
func GetThumbnailJobRetentionPeriod() string {
	return defaultConfig.ThumbnailGeneration.JobRetentionPeriod
}

// GetThumbnailJobBatchTimeout returns the default batch timeout for the thumbnail worker
func GetThumbnailJobBatchTimeout() string {
	return defaultConfig.ThumbnailGeneration.JobBatchTimeout
}

// GetDetachedThumbnailJobTimeout returns the default per-job timeout for detached thumbnail generation
func GetDetachedThumbnailJobTimeout() string {
	return defaultConfig.ThumbnailGeneration.DetachedJobTimeout
}
