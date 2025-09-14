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
	MaxConcurrentExports int
	MaxConcurrentImports int
}

// ThumbnailGeneration contains default values for thumbnail generation configuration
type ThumbnailGeneration struct {
	MaxConcurrentPerUser int    // Maximum simultaneous thumbnail generation jobs per user
	RateLimitPerMinute   int    // Maximum thumbnail generation requests per minute per user
	SlotDuration         string // Duration for which a concurrency slot is held (e.g., "30m")
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
			MaxConcurrentExports: 3,
			MaxConcurrentImports: 1,
		},
		ThumbnailGeneration: ThumbnailGeneration{
			MaxConcurrentPerUser: 5,     // Maximum 5 simultaneous thumbnail generation jobs per user
			RateLimitPerMinute:   50,    // Maximum 50 thumbnail generation requests per minute per user
			SlotDuration:         "30m", // Hold concurrency slots for 30 minutes
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
