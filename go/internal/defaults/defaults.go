package defaults

import (
	"os"
	"path/filepath"
	"strings"
)

// Server contains default values for server configuration
type Server struct {
	Addr           string
	UploadLocation string
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

// Config contains all default configuration values
type Config struct {
	Server   Server
	Database Database
	Workers  Workers
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
			Addr:           ":3333",
			UploadLocation: getFileURL("uploads"),
		},
		Database: Database{
			DSN: "memory://",
		},
		Workers: Workers{
			MaxConcurrentExports: 3,
			MaxConcurrentImports: 3,
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
