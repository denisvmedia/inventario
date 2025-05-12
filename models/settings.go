package models

import (
	"encoding/json"
)

// Setting represents a system setting
type Setting struct {
	ID    string          `json:"id,omitempty"`
	Value json.RawMessage `json:"value"`
}

// GetID returns the ID of the setting
func (s *Setting) GetID() string {
	return s.ID
}

// SetID sets the ID of the setting
func (s *Setting) SetID(id string) {
	s.ID = id
}

// TLSConfig is removed as requested

// UIConfig represents UI configuration settings
type UIConfig struct {
	Theme            string `json:"theme"`
	ShowDebugInfo    bool   `json:"show_debug_info"`
	DefaultPageSize  int    `json:"default_page_size"`
	DefaultDateFormat string `json:"default_date_format"`
}

// SystemConfig represents system-wide configuration settings
type SystemConfig struct {
	UploadSizeLimit int64  `json:"upload_size_limit"`
	LogLevel        string `json:"log_level"`
	BackupEnabled   bool   `json:"backup_enabled"`
	BackupInterval  string `json:"backup_interval,omitempty"`
	BackupLocation  string `json:"backup_location,omitempty"`
	MainCurrency    string `json:"main_currency"`
}

// CurrencyConfig is removed as requested
