package boltdb_test

import (
	"encoding/json"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/boltdb"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

func setupTestDB(t *testing.T) (*bolt.DB, func()) {
	c := qt.New(t)

	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "boltdb-test-*")
	c.Assert(err, qt.IsNil)

	// Create a new database in the temporary directory
	db, err := dbx.NewDB(tempDir, "test.db").Open()
	c.Assert(err, qt.IsNil)

	// Return the database and a cleanup function
	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, cleanup
}

func TestSettingsRegistry_UIConfig(t *testing.T) {
	c := qt.New(t)

	// Setup test database
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a new instance of SettingsRegistry
	r := boltdb.NewSettingsRegistry(db)

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

	// Setup test database
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a new instance of SettingsRegistry
	r := boltdb.NewSettingsRegistry(db)

	// Create a test system config
	systemConfig := &models.SystemConfig{
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
	c.Assert(string(retrievedConfig.MainCurrency), qt.Equals, "EUR")

	// Remove the system config
	err = r.RemoveSystemConfig()
	c.Assert(err, qt.IsNil)

	// Try to get the system config again, should fail
	_, err = r.GetSystemConfig()
	c.Assert(err, qt.Not(qt.IsNil))
}

func TestSettingsRegistry_GenericSettings(t *testing.T) {
	c := qt.New(t)

	// Setup test database
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a new instance of SettingsRegistry
	r := boltdb.NewSettingsRegistry(db)

	v := struct {
		Test string `json:"test"`
	}{
		Test: "value",
	}

	// Create a test setting
	setting := models.Setting{
		Name:  "test_setting",
		Value: must.Must(json.Marshal(v)),
	}

	// Create the setting
	createdSetting, err := r.Create(setting)
	c.Assert(err, qt.IsNil)
	c.Assert(createdSetting.Name, qt.Equals, setting.Name)

	// Get the setting by ID
	retrievedSetting, err := r.Get(createdSetting.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedSetting.ID, qt.Equals, createdSetting.ID)
	c.Assert(retrievedSetting.Name, qt.Equals, setting.Name)
	c.Assert(string(retrievedSetting.Value), qt.JSONEquals, &v)

	// Get the setting by Name
	retrievedByNameSetting, err := r.GetByName(setting.Name)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedByNameSetting.ID, qt.Equals, createdSetting.ID)
	c.Assert(retrievedByNameSetting.Name, qt.Equals, setting.Name)
	c.Assert(string(retrievedByNameSetting.Value), qt.Equals, string(setting.Value))

	// Update the setting
	v = struct {
		Test string `json:"test"`
	}{
		Test: "updated",
	}
	retrievedSetting.Value = must.Must(json.Marshal(v))
	updatedSetting, err := r.Update(*retrievedSetting)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedSetting.ID, qt.Equals, createdSetting.ID)
	c.Assert(updatedSetting.Name, qt.Equals, setting.Name)
	c.Assert(string(updatedSetting.Value), qt.Equals, string(retrievedSetting.Value))

	// Get the updated setting by Name
	retrievedUpdatedSetting, err := r.GetByName(retrievedSetting.Name)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedUpdatedSetting.ID, qt.Equals, createdSetting.ID)
	c.Assert(retrievedUpdatedSetting.Name, qt.Equals, retrievedSetting.Name)
	c.Assert(string(retrievedUpdatedSetting.Value), qt.Equals, string(retrievedSetting.Value))

	// Delete the setting by Name
	err = r.DeleteByName(setting.Name)
	c.Assert(err, qt.IsNil)

	// Try to get the setting by ID again, should fail
	_, err = r.Get(createdSetting.ID)
	c.Assert(err, qt.Not(qt.IsNil))

	// Try to get the setting by Name again, should fail
	_, err = r.GetByName(setting.Name)
	c.Assert(err, qt.Not(qt.IsNil))
}
