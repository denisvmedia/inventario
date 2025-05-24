package boltdb_test

import (
	"context"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/boltdb"
)

func TestSettingsRegistry(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	tempDir := c.TempDir()

	// Create a test database
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := bolt.Open(dbPath, 0o600, nil)
	c.Assert(err, qt.IsNil)
	defer func() {
		err = db.Close()
		c.Assert(err, qt.IsNil)
	}()

	// Create a settings registry
	settingsRegistry := boltdb.NewSettingsRegistry(db)

	// Test Get with no settings
	settings, err := settingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(settings, qt.Equals, models.SettingsObject{})

	// Test Save
	theme := "dark"
	showDebugInfo := true
	testSettings := models.SettingsObject{
		Theme:         &theme,
		ShowDebugInfo: &showDebugInfo,
	}
	err = settingsRegistry.Save(ctx, testSettings)
	c.Assert(err, qt.IsNil)

	// Test Get after Save
	retrievedSettings, err := settingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedSettings, qt.DeepEquals, testSettings)

	// Test Patch
	newTheme := "light"
	err = settingsRegistry.Patch(ctx, "uiconfig.theme", newTheme)
	c.Assert(err, qt.IsNil)
	newCurrency := "USD"
	err = settingsRegistry.Patch(ctx, "system.main_currency", newCurrency)
	c.Assert(err, qt.IsNil)

	// Test Get after Patch
	retrievedSettings, err = settingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(*retrievedSettings.Theme, qt.Equals, newTheme)
	c.Assert(*retrievedSettings.ShowDebugInfo, qt.Equals, showDebugInfo)
	c.Assert(*retrievedSettings.MainCurrency, qt.Equals, newCurrency)
}
