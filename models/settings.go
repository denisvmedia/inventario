package models

import (
	"encoding/json"
)

// Setting represents a system setting
type Setting struct {
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name"`
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

// UIConfig represents UI configuration settings
type UIConfig struct {
	Theme            string `json:"theme"`
	ShowDebugInfo    bool   `json:"show_debug_info"`
	DefaultPageSize  int    `json:"default_page_size"`
	DefaultDateFormat string `json:"default_date_format"`
}

// SystemConfig represents system-wide configuration settings
type SystemConfig struct {
	MainCurrency    Currency `json:"main_currency"`
}
