package memory_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestSettingsRegistry_UIConfig(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of SettingsRegistry
	r := memory.NewSettingsRegistry()

	// Create a test UI config
	uiConfig := &models.UIConfig{
		Theme:             "dark",
		ShowDebugInfo:     true,
		DefaultPageSize:   50,
		DefaultDateFormat: "YYYY-MM-DD",
	}

	// Set the UI config
	err := r.SetUIConfig(uiConfig)
	c.Assert(err, qt.IsNil)

	// Get the UI config
	retrievedConfig, err := r.GetUIConfig()
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedConfig, qt.DeepEquals, uiConfig)

	// Remove the UI config
	err = r.RemoveUIConfig()
	c.Assert(err, qt.IsNil)

	// Try to get the UI config again, should fail
	_, err = r.GetUIConfig()
	c.Assert(err, qt.Not(qt.IsNil))
}

func TestSettingsRegistry_SystemConfig(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of SettingsRegistry
	r := memory.NewSettingsRegistry()

	// Create a test system config
	systemConfig := &models.SystemConfig{
		UploadSizeLimit: 20971520, // 20MB
		LogLevel:        "debug",
		BackupEnabled:   true,
		BackupInterval:  "12h",
		BackupLocation:  "/backup",
		MainCurrency:    "EUR",
	}

	// Set the system config
	err := r.SetSystemConfig(systemConfig)
	c.Assert(err, qt.IsNil)

	// Get the system config
	retrievedConfig, err := r.GetSystemConfig()
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedConfig, qt.DeepEquals, systemConfig)

	// Verify the main currency
	c.Assert(retrievedConfig.MainCurrency, qt.Equals, "EUR")

	// Remove the system config
	err = r.RemoveSystemConfig()
	c.Assert(err, qt.IsNil)

	// Try to get the system config again, should fail
	_, err = r.GetSystemConfig()
	c.Assert(err, qt.Not(qt.IsNil))
}

func TestSettingsRegistry_GenericSettings(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of SettingsRegistry
	r := memory.NewSettingsRegistry()

	// Create a test setting
	setting := models.Setting{
		Value: []byte(`{"test": "value"}`),
	}

	// Create the setting
	createdSetting, err := r.Create(setting)
	c.Assert(err, qt.IsNil)
	c.Assert(createdSetting.ID, qt.Not(qt.Equals), "")

	// Get the setting
	retrievedSetting, err := r.Get(createdSetting.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedSetting.ID, qt.Equals, createdSetting.ID)
	c.Assert(string(retrievedSetting.Value), qt.Equals, string(setting.Value))

	// Update the setting
	setting.ID = createdSetting.ID
	setting.Value = []byte(`{"test": "updated"}`)
	updatedSetting, err := r.Update(setting)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedSetting.ID, qt.Equals, createdSetting.ID)
	c.Assert(string(updatedSetting.Value), qt.Equals, string(setting.Value))

	// Delete the setting
	err = r.Delete(createdSetting.ID)
	c.Assert(err, qt.IsNil)

	// Try to get the setting again, should fail
	_, err = r.Get(createdSetting.ID)
	c.Assert(err, qt.Not(qt.IsNil))
}
