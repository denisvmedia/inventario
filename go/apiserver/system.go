package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/internal/version"
	"github.com/denisvmedia/inventario/models"
)

type systemAPI struct {
	debugInfo *debug.Info
	startTime time.Time
}

// SystemInfo contains comprehensive system information
type SystemInfo struct {
	// Version information
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`

	// System information
	DatabaseBackend    string `json:"database_backend"`
	FileStorageBackend string `json:"file_storage_backend"`
	OperatingSystem    string `json:"operating_system"`

	// Runtime metrics
	Uptime        string `json:"uptime"`
	MemoryUsage   string `json:"memory_usage"`
	NumGoroutines int    `json:"num_goroutines"`
	NumCPU        int    `json:"num_cpu"`

	// Settings information
	Settings models.SettingsObject `json:"settings"`
}

// getSystemInfo returns comprehensive system information.
// @Summary Get system information
// @Description get comprehensive system information including version, runtime metrics, and settings
// @Tags system
// @Accept  json
// @Produce  json
// @Success 200 {object} SystemInfo "OK"
// @Router /system [get]
func (api *systemAPI) getSystemInfo(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	// Get user-aware settings registry from context
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	settingsRegistry := regSet.SettingsRegistry

	// Get version information
	buildInfo := version.Get()

	// Get runtime metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Calculate uptime
	uptime := time.Since(api.startTime)

	// Format memory usage (in MB)
	memoryUsageMB := float64(memStats.Alloc) / 1024 / 1024

	// Get current settings
	settings, err := settingsRegistry.Get(r.Context())
	if err != nil {
		// If settings can't be loaded, use empty settings but don't fail
		settings = models.SettingsObject{}
	}

	systemInfo := SystemInfo{
		// Version information
		Version:   buildInfo.Version,
		Commit:    buildInfo.Commit,
		BuildDate: buildInfo.Date,
		GoVersion: buildInfo.GoVersion,
		Platform:  buildInfo.Platform,

		// System information from debug info
		DatabaseBackend:    api.debugInfo.DatabaseDriver,
		FileStorageBackend: api.debugInfo.FileStorageDriver,
		OperatingSystem:    api.debugInfo.OperatingSystem,

		// Runtime metrics
		Uptime:        formatDuration(uptime),
		MemoryUsage:   fmt.Sprintf("%.1f MB", memoryUsageMB),
		NumGoroutines: runtime.NumGoroutine(),
		NumCPU:        runtime.NumCPU(),

		// Settings
		Settings: settings,
	}

	// Set the content type to application/json
	w.Header().Set("Content-Type", "application/json")

	// Return the system information
	if err := json.NewEncoder(w).Encode(systemInfo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%d day%s %d hour%s", days, pluralize(days), hours, pluralize(hours))
	}
	if hours > 0 {
		return fmt.Sprintf("%d hour%s %d minute%s", hours, pluralize(hours), minutes, pluralize(minutes))
	}
	if minutes > 0 {
		return fmt.Sprintf("%d minute%s %d second%s", minutes, pluralize(minutes), seconds, pluralize(seconds))
	}
	return fmt.Sprintf("%d second%s", seconds, pluralize(seconds))
}

// pluralize returns "s" if the value is not 1, empty string otherwise
func pluralize(value int) string {
	if value == 1 {
		return ""
	}
	return "s"
}

// System returns a handler for system information.
func System(debugInfo *debug.Info, startTime time.Time) func(r chi.Router) {
	api := &systemAPI{
		debugInfo: debugInfo,
		startTime: startTime,
	}

	return func(r chi.Router) {
		r.Get("/", api.getSystemInfo) // GET /system
	}
}
