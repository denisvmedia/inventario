package commonsql_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

// TestSettingsRegistry_Get_HappyPath tests successful settings retrieval scenarios.
func TestSettingsRegistry_Get_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Get default settings (should be empty)
	settings, err := registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(settings, qt.DeepEquals, models.SettingsObject{})
}

// TestSettingsRegistry_Save_HappyPath tests successful settings save scenarios.
func TestSettingsRegistry_Save_HappyPath(t *testing.T) {
	testCases := []struct {
		name     string
		settings models.SettingsObject
	}{
		{
			name: "basic settings",
			settings: models.SettingsObject{
				MainCurrency:      stringPtr("USD"),
				Theme:             stringPtr("dark"),
				ShowDebugInfo:     boolPtr(true),
				DefaultDateFormat: stringPtr("YYYY-MM-DD"),
			},
		},
		{
			name: "partial settings",
			settings: models.SettingsObject{
				MainCurrency: stringPtr("EUR"),
				Theme:        stringPtr("light"),
			},
		},
		{
			name: "only main currency",
			settings: models.SettingsObject{
				MainCurrency: stringPtr("GBP"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Save settings
			err := registrySet.SettingsRegistry.Save(ctx, tc.settings)
			c.Assert(err, qt.IsNil)

			// Retrieve and verify settings
			retrievedSettings, err := registrySet.SettingsRegistry.Get(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(retrievedSettings, qt.DeepEquals, tc.settings)
		})
	}
}

// TestSettingsRegistry_Save_UnhappyPath tests settings save error scenarios.
func TestSettingsRegistry_Save_UnhappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// First, set a main currency
	initialSettings := models.SettingsObject{
		MainCurrency: stringPtr("USD"),
	}
	err := registrySet.SettingsRegistry.Save(ctx, initialSettings)
	c.Assert(err, qt.IsNil)

	// Try to change the main currency (should fail)
	newSettings := models.SettingsObject{
		MainCurrency: stringPtr("EUR"),
	}
	err = registrySet.SettingsRegistry.Save(ctx, newSettings)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*main currency already set.*")

	// Verify original settings are unchanged
	retrievedSettings, err := registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedSettings, qt.DeepEquals, initialSettings)
}

// TestSettingsRegistry_Patch_HappyPath tests successful settings patch scenarios.
func TestSettingsRegistry_Patch_HappyPath(t *testing.T) {
	testCases := []struct {
		name        string
		configField string
		value       any
		expected    models.SettingsObject
	}{
		{
			name:        "set main currency",
			configField: "system.main_currency",
			value:       "USD",
			expected: models.SettingsObject{
				MainCurrency: stringPtr("USD"),
			},
		},
		{
			name:        "set theme",
			configField: "uiconfig.theme",
			value:       "dark",
			expected: models.SettingsObject{
				Theme: stringPtr("dark"),
			},
		},
		{
			name:        "set show debug info",
			configField: "uiconfig.show_debug_info",
			value:       true,
			expected: models.SettingsObject{
				ShowDebugInfo: boolPtr(true),
			},
		},
		{
			name:        "set default date format",
			configField: "uiconfig.default_date_format",
			value:       "DD/MM/YYYY",
			expected: models.SettingsObject{
				DefaultDateFormat: stringPtr("DD/MM/YYYY"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Patch settings
			err := registrySet.SettingsRegistry.Patch(ctx, tc.configField, tc.value)
			c.Assert(err, qt.IsNil)

			// Retrieve and verify settings
			retrievedSettings, err := registrySet.SettingsRegistry.Get(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(retrievedSettings, qt.DeepEquals, tc.expected)
		})
	}
}

// TestSettingsRegistry_Patch_UnhappyPath tests settings patch error scenarios.
func TestSettingsRegistry_Patch_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name        string
		configField string
		value       any
	}{
		{
			name:        "invalid config field",
			configField: "invalid.field",
			value:       "some_value",
		},
		{
			name:        "invalid main currency type",
			configField: "system.main_currency",
			value:       123, // Should be string
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Try to patch with invalid data
			err := registrySet.SettingsRegistry.Patch(ctx, tc.configField, tc.value)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestSettingsRegistry_Patch_MainCurrencyAlreadySet tests main currency change restriction.
func TestSettingsRegistry_Patch_MainCurrencyAlreadySet(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// First, set a main currency
	err := registrySet.SettingsRegistry.Patch(ctx, "system.main_currency", "USD")
	c.Assert(err, qt.IsNil)

	// Try to change the main currency (should fail)
	err = registrySet.SettingsRegistry.Patch(ctx, "system.main_currency", "EUR")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*main currency already set.*")

	// Verify original currency is unchanged
	retrievedSettings, err := registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(*retrievedSettings.MainCurrency, qt.Equals, "USD")

	// Setting the same currency should work
	err = registrySet.SettingsRegistry.Patch(ctx, "system.main_currency", "USD")
	c.Assert(err, qt.IsNil)
}

// TestSettingsRegistry_MultiplePatches tests multiple patch operations.
func TestSettingsRegistry_MultiplePatches(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Apply multiple patches
	err := registrySet.SettingsRegistry.Patch(ctx, "system.main_currency", "USD")
	c.Assert(err, qt.IsNil)

	err = registrySet.SettingsRegistry.Patch(ctx, "uiconfig.theme", "dark")
	c.Assert(err, qt.IsNil)

	err = registrySet.SettingsRegistry.Patch(ctx, "uiconfig.show_debug_info", true)
	c.Assert(err, qt.IsNil)

	err = registrySet.SettingsRegistry.Patch(ctx, "uiconfig.default_date_format", "DD/MM/YYYY")
	c.Assert(err, qt.IsNil)

	// Verify all settings are applied
	retrievedSettings, err := registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)

	expected := models.SettingsObject{
		MainCurrency:      stringPtr("USD"),
		Theme:             stringPtr("dark"),
		ShowDebugInfo:     boolPtr(true),
		DefaultDateFormat: stringPtr("DD/MM/YYYY"),
	}
	c.Assert(retrievedSettings, qt.DeepEquals, expected)
}

// TestSettingsRegistry_SaveAndPatch tests interaction between Save and Patch operations.
func TestSettingsRegistry_SaveAndPatch(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Save initial settings
	initialSettings := models.SettingsObject{
		MainCurrency: stringPtr("USD"),
		Theme:        stringPtr("light"),
	}
	err := registrySet.SettingsRegistry.Save(ctx, initialSettings)
	c.Assert(err, qt.IsNil)

	// Patch additional settings
	err = registrySet.SettingsRegistry.Patch(ctx, "uiconfig.show_debug_info", true)
	c.Assert(err, qt.IsNil)

	// Verify combined settings
	retrievedSettings, err := registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)

	expected := models.SettingsObject{
		MainCurrency:  stringPtr("USD"),
		Theme:         stringPtr("light"),
		ShowDebugInfo: boolPtr(true),
	}
	c.Assert(retrievedSettings, qt.DeepEquals, expected)

	// Try to save settings that would change main currency (should fail)
	newSettings := models.SettingsObject{
		MainCurrency: stringPtr("EUR"),
		Theme:        stringPtr("dark"),
	}
	err = registrySet.SettingsRegistry.Save(ctx, newSettings)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*main currency already set.*")

	// Verify settings are unchanged
	retrievedSettings, err = registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedSettings, qt.DeepEquals, expected)
}

// Helper functions for creating pointers to basic types
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
