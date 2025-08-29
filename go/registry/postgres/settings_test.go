package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
)

func TestSettingsRegistry_Get_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Get user-aware settings registry
	settingsRegistry, err := registrySet.SettingsRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Test getting empty settings initially
	settings, err := settingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(settings.MainCurrency, qt.IsNil)
	c.Assert(settings.Theme, qt.IsNil)
	c.Assert(settings.ShowDebugInfo, qt.IsNil)
	c.Assert(settings.DefaultDateFormat, qt.IsNil)
}

func TestSettingsRegistry_Save_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name     string
		settings models.SettingsObject
	}{
		{
			name: "save all settings",
			settings: models.SettingsObject{
				MainCurrency:      stringPtr("USD"),
				Theme:             stringPtr("dark"),
				ShowDebugInfo:     boolPtr(true),
				DefaultDateFormat: stringPtr("2006-01-02"),
			},
		},
		{
			name: "save partial settings",
			settings: models.SettingsObject{
				MainCurrency: stringPtr("EUR"),
				Theme:        stringPtr("light"),
			},
		},
		{
			name: "save single setting",
			settings: models.SettingsObject{
				ShowDebugInfo: boolPtr(false),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()
			ctx = appctx.WithUser(ctx, &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})

			// Get user-aware settings registry
			settingsRegistry, err := registrySet.SettingsRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			// Save the settings
			err = settingsRegistry.Save(ctx, tc.settings)
			c.Assert(err, qt.IsNil)

			// Retrieve and verify the settings
			retrievedSettings, err := settingsRegistry.Get(ctx)
			c.Assert(err, qt.IsNil)

			if tc.settings.MainCurrency != nil {
				c.Assert(retrievedSettings.MainCurrency, qt.IsNotNil)
				c.Assert(*retrievedSettings.MainCurrency, qt.Equals, *tc.settings.MainCurrency)
			}
			if tc.settings.Theme != nil {
				c.Assert(retrievedSettings.Theme, qt.IsNotNil)
				c.Assert(*retrievedSettings.Theme, qt.Equals, *tc.settings.Theme)
			}
			if tc.settings.ShowDebugInfo != nil {
				c.Assert(retrievedSettings.ShowDebugInfo, qt.IsNotNil)
				c.Assert(*retrievedSettings.ShowDebugInfo, qt.Equals, *tc.settings.ShowDebugInfo)
			}
			if tc.settings.DefaultDateFormat != nil {
				c.Assert(retrievedSettings.DefaultDateFormat, qt.IsNotNil)
				c.Assert(*retrievedSettings.DefaultDateFormat, qt.Equals, *tc.settings.DefaultDateFormat)
			}
		})
	}
}

func TestSettingsRegistry_Save_UpdateExisting_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Get user-aware settings registry
	settingsRegistry, err := registrySet.SettingsRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Save initial settings
	initialSettings := models.SettingsObject{
		MainCurrency:      stringPtr("USD"),
		Theme:             stringPtr("dark"),
		ShowDebugInfo:     boolPtr(true),
		DefaultDateFormat: stringPtr("2006-01-02"),
	}
	err = settingsRegistry.Save(ctx, initialSettings)
	c.Assert(err, qt.IsNil)

	// Update settings
	updatedSettings := models.SettingsObject{
		MainCurrency:      stringPtr("EUR"),
		Theme:             stringPtr("light"),
		ShowDebugInfo:     boolPtr(false),
		DefaultDateFormat: stringPtr("02/01/2006"),
	}
	err = settingsRegistry.Save(ctx, updatedSettings)
	c.Assert(err, qt.IsNil)

	// Verify the updated settings
	retrievedSettings, err := settingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(*retrievedSettings.MainCurrency, qt.Equals, "EUR")
	c.Assert(*retrievedSettings.Theme, qt.Equals, "light")
	c.Assert(*retrievedSettings.ShowDebugInfo, qt.Equals, false)
	c.Assert(*retrievedSettings.DefaultDateFormat, qt.Equals, "02/01/2006")
}

func TestSettingsRegistry_Patch_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name         string
		settingName  string
		settingValue any
		verifyFunc   func(*qt.C, models.SettingsObject)
	}{
		{
			name:         "patch main currency",
			settingName:  "system.main_currency",
			settingValue: "USD",
			verifyFunc: func(c *qt.C, settings models.SettingsObject) {
				c.Assert(settings.MainCurrency, qt.IsNotNil)
				c.Assert(*settings.MainCurrency, qt.Equals, "USD")
			},
		},
		{
			name:         "patch theme",
			settingName:  "uiconfig.theme",
			settingValue: "dark",
			verifyFunc: func(c *qt.C, settings models.SettingsObject) {
				c.Assert(settings.Theme, qt.IsNotNil)
				c.Assert(*settings.Theme, qt.Equals, "dark")
			},
		},
		{
			name:         "patch show debug info",
			settingName:  "uiconfig.show_debug_info",
			settingValue: true,
			verifyFunc: func(c *qt.C, settings models.SettingsObject) {
				c.Assert(settings.ShowDebugInfo, qt.IsNotNil)
				c.Assert(*settings.ShowDebugInfo, qt.Equals, true)
			},
		},
		{
			name:         "patch default date format",
			settingName:  "uiconfig.default_date_format",
			settingValue: "2006-01-02",
			verifyFunc: func(c *qt.C, settings models.SettingsObject) {
				c.Assert(settings.DefaultDateFormat, qt.IsNotNil)
				c.Assert(*settings.DefaultDateFormat, qt.Equals, "2006-01-02")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			// Patch the setting
			err := registrySet.SettingsRegistry.Patch(ctx, tc.settingName, tc.settingValue)
			c.Assert(err, qt.IsNil)

			// Retrieve and verify the settings
			retrievedSettings, err := registrySet.SettingsRegistry.Get(ctx)
			c.Assert(err, qt.IsNil)

			tc.verifyFunc(c, retrievedSettings)
		})
	}
}

func TestSettingsRegistry_Patch_UpdateExisting_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Set initial value
	err := registrySet.SettingsRegistry.Patch(ctx, "uiconfig.theme", "dark")
	c.Assert(err, qt.IsNil)

	// Update the value
	err = registrySet.SettingsRegistry.Patch(ctx, "uiconfig.theme", "light")
	c.Assert(err, qt.IsNil)

	// Verify the updated value
	retrievedSettings, err := registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedSettings.Theme, qt.IsNotNil)
	c.Assert(*retrievedSettings.Theme, qt.Equals, "light")
}

func TestSettingsRegistry_Patch_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name         string
		settingName  string
		settingValue any
	}{
		{
			name:         "invalid setting name",
			settingName:  "invalid.setting",
			settingValue: "value",
		},
		{
			name:         "empty setting name",
			settingName:  "",
			settingValue: "value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			err := registrySet.SettingsRegistry.Patch(ctx, tc.settingName, tc.settingValue)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestSettingsRegistry_MixedOperations_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Start with empty settings
	settings, err := registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(settings.MainCurrency, qt.IsNil)

	// Patch individual settings
	err = registrySet.SettingsRegistry.Patch(ctx, "system.main_currency", "USD")
	c.Assert(err, qt.IsNil)
	err = registrySet.SettingsRegistry.Patch(ctx, "uiconfig.theme", "dark")
	c.Assert(err, qt.IsNil)

	// Verify patched settings
	settings, err = registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(settings.MainCurrency, qt.IsNotNil)
	c.Assert(*settings.MainCurrency, qt.Equals, "USD")
	c.Assert(settings.Theme, qt.IsNotNil)
	c.Assert(*settings.Theme, qt.Equals, "dark")

	// Save complete settings object (should update existing)
	newSettings := models.SettingsObject{
		MainCurrency:      stringPtr("EUR"),
		Theme:             stringPtr("light"),
		ShowDebugInfo:     boolPtr(true),
		DefaultDateFormat: stringPtr("2006-01-02"),
	}
	err = registrySet.SettingsRegistry.Save(ctx, newSettings)
	c.Assert(err, qt.IsNil)

	// Verify all settings are updated
	settings, err = registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(*settings.MainCurrency, qt.Equals, "EUR")
	c.Assert(*settings.Theme, qt.Equals, "light")
	c.Assert(*settings.ShowDebugInfo, qt.Equals, true)
	c.Assert(*settings.DefaultDateFormat, qt.Equals, "2006-01-02")

	// Patch one setting again
	err = registrySet.SettingsRegistry.Patch(ctx, "uiconfig.show_debug_info", false)
	c.Assert(err, qt.IsNil)

	// Verify only the patched setting changed
	settings, err = registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(*settings.MainCurrency, qt.Equals, "EUR")
	c.Assert(*settings.Theme, qt.Equals, "light")
	c.Assert(*settings.ShowDebugInfo, qt.Equals, false)
	c.Assert(*settings.DefaultDateFormat, qt.Equals, "2006-01-02")
}

func TestSettingsRegistry_Save_EmptySettings_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Save empty settings object
	emptySettings := models.SettingsObject{}
	err := registrySet.SettingsRegistry.Save(ctx, emptySettings)
	c.Assert(err, qt.IsNil)

	// Verify settings remain empty
	retrievedSettings, err := registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedSettings.MainCurrency, qt.IsNil)
	c.Assert(retrievedSettings.Theme, qt.IsNil)
	c.Assert(retrievedSettings.ShowDebugInfo, qt.IsNil)
	c.Assert(retrievedSettings.DefaultDateFormat, qt.IsNil)
}

func TestSettingsRegistry_Patch_ComplexValues_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name         string
		settingName  string
		settingValue any
		verifyFunc   func(*qt.C, models.SettingsObject)
	}{
		{
			name:         "patch with boolean false",
			settingName:  "uiconfig.show_debug_info",
			settingValue: false,
			verifyFunc: func(c *qt.C, settings models.SettingsObject) {
				c.Assert(settings.ShowDebugInfo, qt.IsNotNil)
				c.Assert(*settings.ShowDebugInfo, qt.Equals, false)
			},
		},
		{
			name:         "patch with boolean true",
			settingName:  "uiconfig.show_debug_info",
			settingValue: true,
			verifyFunc: func(c *qt.C, settings models.SettingsObject) {
				c.Assert(settings.ShowDebugInfo, qt.IsNotNil)
				c.Assert(*settings.ShowDebugInfo, qt.Equals, true)
			},
		},
		{
			name:         "patch with empty string",
			settingName:  "uiconfig.theme",
			settingValue: "",
			verifyFunc: func(c *qt.C, settings models.SettingsObject) {
				c.Assert(settings.Theme, qt.IsNotNil)
				c.Assert(*settings.Theme, qt.Equals, "")
			},
		},
		{
			name:         "patch with special characters",
			settingName:  "uiconfig.default_date_format",
			settingValue: "dd/MM/yyyy HH:mm:ss",
			verifyFunc: func(c *qt.C, settings models.SettingsObject) {
				c.Assert(settings.DefaultDateFormat, qt.IsNotNil)
				c.Assert(*settings.DefaultDateFormat, qt.Equals, "dd/MM/yyyy HH:mm:ss")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			err := registrySet.SettingsRegistry.Patch(ctx, tc.settingName, tc.settingValue)
			c.Assert(err, qt.IsNil)

			retrievedSettings, err := registrySet.SettingsRegistry.Get(ctx)
			c.Assert(err, qt.IsNil)

			tc.verifyFunc(c, retrievedSettings)
		})
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
