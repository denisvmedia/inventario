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
	MaxConcurrentExports        int
	MaxConcurrentImports        int
	MaxConcurrentRestores       int
	ExportPollInterval          string // Export worker poll interval (e.g., "10s")
	ImportPollInterval          string // Import worker poll interval (e.g., "10s")
	RestorePollInterval         string // Restore worker poll interval (e.g., "10s")
	RefreshTokenCleanupInterval string // Refresh token cleanup interval (e.g., "1h")
	GroupPurgeInterval          string // Group purge worker interval (e.g., "5m")
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
			MaxConcurrentExports:        3,
			MaxConcurrentImports:        1,
			MaxConcurrentRestores:       1,
			ExportPollInterval:          "10s",
			ImportPollInterval:          "10s",
			RestorePollInterval:         "10s",
			RefreshTokenCleanupInterval: "1h",
			GroupPurgeInterval:          "5m",
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

// GetGroupPurgeInterval returns the default interval between group purge sweeps.
func GetGroupPurgeInterval() string {
	return defaultConfig.Workers.GroupPurgeInterval
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
