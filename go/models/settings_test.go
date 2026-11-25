package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

// TestSettingsObject_Set_HappyPath tests successful setting of fields.
func TestSettingsObject_Set_HappyPath(t *testing.T) {
	testCases := []struct {
		name        string
		field       string
		value       any
		expectedGet any
	}{
		{
			name:        "set theme",
			field:       "uiconfig.theme",
			value:       "dark",
			expectedGet: new("dark"),
		},
		{
			name:        "set show debug info",
			field:       "uiconfig.show_debug_info",
			value:       true,
			expectedGet: new(true),
		},
		{
			name:        "set default date format",
			field:       "uiconfig.default_date_format",
			value:       "YYYY-MM-DD",
			expectedGet: new("YYYY-MM-DD"),
		},
		{
			name:        "set show debug info false",
			field:       "uiconfig.show_debug_info",
			value:       false,
			expectedGet: new(false),
		},
		{
			name:        "set number format locale",
			field:       "appearance.number_format_locale",
			value:       "cs-CZ",
			expectedGet: new("cs-CZ"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			settings := &models.SettingsObject{}
			err := settings.Set(tc.field, tc.value)
			c.Assert(err, qt.IsNil)

			// Verify the value was set correctly
			value, err := settings.Get(tc.field)
			c.Assert(err, qt.IsNil)
			c.Assert(value, qt.DeepEquals, tc.expectedGet)
		})
	}
}

// TestSettingsObject_Set_UnhappyPath tests error cases for setting fields.
func TestSettingsObject_Set_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name          string
		field         string
		value         any
		expectedError string
	}{
		{
			name:          "non-existent field",
			field:         "nonexistent.field",
			value:         "value",
			expectedError: `cannot set field "nonexistent.field": no field with tag`,
		},
		{
			name:          "empty field name",
			field:         "",
			value:         "value",
			expectedError: `cannot set field "": no field with tag`,
		},
		{
			name:          "string into a toggle",
			field:         "notifications.warranty_expiry",
			value:         "yes",
			expectedError: `cannot set field "NotificationsWarrantyExpiry": .*value does not fit the field type`,
		},
		{
			name:          "text in a lead-time list",
			field:         "notifications.warranty_lead_times",
			value:         []any{float64(30), "soon"},
			expectedError: `cannot set field "NotificationsWarrantyLeadTimes": .*value does not fit the field type`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			settings := &models.SettingsObject{}
			err := settings.Set(tc.field, tc.value)
			c.Assert(err, qt.ErrorMatches, tc.expectedError)
		})
	}
}

// TestSettingsObject_Get_HappyPath tests successful getting of fields.
func TestSettingsObject_Get_HappyPath(t *testing.T) {
	testCases := []struct {
		name     string
		settings models.SettingsObject
		field    string
		expected any
	}{
		{
			name: "get theme",
			settings: models.SettingsObject{
				Theme: new("light"),
			},
			field:    "uiconfig.theme",
			expected: new("light"),
		},
		{
			name: "get show debug info true",
			settings: models.SettingsObject{
				ShowDebugInfo: new(true),
			},
			field:    "uiconfig.show_debug_info",
			expected: new(true),
		},
		{
			name: "get show debug info false",
			settings: models.SettingsObject{
				ShowDebugInfo: new(false),
			},
			field:    "uiconfig.show_debug_info",
			expected: new(false),
		},
		{
			name: "get default date format",
			settings: models.SettingsObject{
				DefaultDateFormat: new("DD/MM/YYYY"),
			},
			field:    "uiconfig.default_date_format",
			expected: new("DD/MM/YYYY"),
		},
		{
			name:     "get nil theme",
			settings: models.SettingsObject{},
			field:    "uiconfig.theme",
			expected: (*string)(nil),
		},
		{
			name:     "get nil show debug info",
			settings: models.SettingsObject{},
			field:    "uiconfig.show_debug_info",
			expected: (*bool)(nil),
		},
		{
			name:     "get nil default date format",
			settings: models.SettingsObject{},
			field:    "uiconfig.default_date_format",
			expected: (*string)(nil),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			value, err := tc.settings.Get(tc.field)
			c.Assert(err, qt.IsNil)
			c.Assert(value, qt.DeepEquals, tc.expected)
		})
	}
}

// TestSettingsObject_Get_UnhappyPath tests error cases for getting fields.
func TestSettingsObject_Get_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name          string
		field         string
		expectedError string
	}{
		{
			name:          "non-existent field",
			field:         "nonexistent.field",
			expectedError: `no field with configfield tag "nonexistent.field" found`,
		},
		{
			name:          "empty field name",
			field:         "",
			expectedError: `no field with configfield tag "" found`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			settings := &models.SettingsObject{}
			value, err := settings.Get(tc.field)
			c.Assert(err, qt.ErrorMatches, tc.expectedError)
			c.Assert(value, qt.IsNil)
		})
	}
}

// TestSettingsObject_SetAndGet_Integration tests setting and getting values in combination.
func TestSettingsObject_SetAndGet_Integration(t *testing.T) {
	c := qt.New(t)

	settings := &models.SettingsObject{}

	// Set multiple values
	err := settings.Set("uiconfig.theme", "dark")
	c.Assert(err, qt.IsNil)

	err = settings.Set("uiconfig.show_debug_info", true)
	c.Assert(err, qt.IsNil)

	err = settings.Set("uiconfig.default_date_format", "MM/DD/YYYY")
	c.Assert(err, qt.IsNil)

	// Verify all values
	theme, err := settings.Get("uiconfig.theme")
	c.Assert(err, qt.IsNil)
	c.Assert(theme, qt.DeepEquals, new("dark"))

	showDebugInfo, err := settings.Get("uiconfig.show_debug_info")
	c.Assert(err, qt.IsNil)
	c.Assert(showDebugInfo, qt.DeepEquals, new(true))

	dateFormat, err := settings.Get("uiconfig.default_date_format")
	c.Assert(err, qt.IsNil)
	c.Assert(dateFormat, qt.DeepEquals, new("MM/DD/YYYY"))

	// Verify the struct fields directly
	c.Assert(settings.Theme, qt.DeepEquals, new("dark"))
	c.Assert(settings.ShowDebugInfo, qt.DeepEquals, new(true))
	c.Assert(settings.DefaultDateFormat, qt.DeepEquals, new("MM/DD/YYYY"))
}

// TestSettingsObject_OverwriteValues tests overwriting existing values.
func TestSettingsObject_OverwriteValues(t *testing.T) {
	c := qt.New(t)

	settings := &models.SettingsObject{
		Theme:             new("light"),
		ShowDebugInfo:     new(false),
		DefaultDateFormat: new("YYYY-MM-DD"),
	}

	// Overwrite values
	err := settings.Set("uiconfig.theme", "dark")
	c.Assert(err, qt.IsNil)

	err = settings.Set("uiconfig.show_debug_info", true)
	c.Assert(err, qt.IsNil)

	err = settings.Set("uiconfig.default_date_format", "DD-MM-YYYY")
	c.Assert(err, qt.IsNil)

	// Verify new values
	c.Assert(settings.Theme, qt.DeepEquals, new("dark"))
	c.Assert(settings.ShowDebugInfo, qt.DeepEquals, new(true))
	c.Assert(settings.DefaultDateFormat, qt.DeepEquals, new("DD-MM-YYYY"))
}

// TestSettingsObject_TypeConversion tests type conversion scenarios.
func TestSettingsObject_TypeConversion(t *testing.T) {
	testCases := []struct {
		name     string
		field    string
		setValue any
		expected any
	}{
		{
			name:     "bool to pointer bool",
			field:    "uiconfig.show_debug_info",
			setValue: true,
			expected: new(true),
		},
		{
			name:     "pointer string to pointer string",
			field:    "uiconfig.theme",
			setValue: new("dark"),
			expected: new("dark"),
		},
		{
			name:     "pointer bool to pointer bool",
			field:    "uiconfig.show_debug_info",
			setValue: new(false),
			expected: new(false),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			settings := &models.SettingsObject{}
			err := settings.Set(tc.field, tc.setValue)
			c.Assert(err, qt.IsNil)

			value, err := settings.Get(tc.field)
			c.Assert(err, qt.IsNil)
			c.Assert(value, qt.DeepEquals, tc.expected)
		})
	}
}

// TestSettingsObject_EmptyValues tests setting empty/zero values.
func TestSettingsObject_EmptyValues(t *testing.T) {
	c := qt.New(t)

	settings := &models.SettingsObject{}

	// Set false boolean
	err := settings.Set("uiconfig.show_debug_info", false)
	c.Assert(err, qt.IsNil)
	c.Assert(settings.ShowDebugInfo, qt.DeepEquals, new(false))

	// Verify via Get method
	debugInfo, err := settings.Get("uiconfig.show_debug_info")
	c.Assert(err, qt.IsNil)
	c.Assert(debugInfo, qt.DeepEquals, new(false))
}

// TestSettingsObject_LeadTimesFromJSON covers the shapes encoding/json
// produces for the reminder lead times (#2805).
func TestSettingsObject_LeadTimesFromJSON(t *testing.T) {
	c := qt.New(t)
	settings := &models.SettingsObject{}

	c.Assert(settings.Set("notifications.warranty_lead_times", []any{float64(90), float64(30)}), qt.IsNil)
	c.Assert(settings.NotificationsWarrantyLeadTimes, qt.DeepEquals, []int{90, 30})
	c.Assert(settings.Set("notifications.loan_lead_time", float64(10)), qt.IsNil)
	c.Assert(settings.NotificationsLoanLeadTime, qt.DeepEquals, new(10))

	// null clears a value back to the default.
	c.Assert(settings.Set("notifications.warranty_lead_times", nil), qt.IsNil)
	c.Assert(settings.NotificationsWarrantyLeadTimes, qt.IsNil)
}

func TestSettingsObject_ValidateWithContext(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		settings models.SettingsObject
		wantErr  string
	}{
		{name: "nothing set", settings: models.SettingsObject{}},
		{name: "valid lead times", settings: models.SettingsObject{
			NotificationsWarrantyLeadTimes:    []int{90, 30, 1},
			NotificationsMaintenanceLeadTimes: []int{365},
			NotificationsLoanLeadTime:         new(60),
		}},
		{name: "zero is not a warranty lead time", settings: models.SettingsObject{NotificationsWarrantyLeadTimes: []int{30, 0}}, wantErr: ".*between 1 and 365 days.*"},
		{name: "beyond a year", settings: models.SettingsObject{NotificationsMaintenanceLeadTimes: []int{366}}, wantErr: ".*between 1 and 365 days.*"},
		{name: "duplicates", settings: models.SettingsObject{NotificationsWarrantyLeadTimes: []int{7, 7}}, wantErr: ".*must not repeat.*"},
		{name: "too many", settings: models.SettingsObject{NotificationsWarrantyLeadTimes: []int{1, 2, 3, 4, 5, 6}}, wantErr: ".*at most 5.*"},
		{name: "empty list", settings: models.SettingsObject{NotificationsWarrantyLeadTimes: []int{}}, wantErr: ".*at least one.*"},
		{name: "loan lead time of zero", settings: models.SettingsObject{NotificationsLoanLeadTime: new(0)}, wantErr: ".*notificationsLoanLeadTime.*"},
		{name: "loan lead time too long", settings: models.SettingsObject{NotificationsLoanLeadTime: new(61)}, wantErr: ".*notificationsLoanLeadTime.*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			err := tt.settings.ValidateWithContext(ctx)
			if tt.wantErr == "" {
				c.Assert(err, qt.IsNil)
				return
			}
			c.Assert(err, qt.ErrorMatches, tt.wantErr)
		})
	}
}
