package boltdb_test

import (
	"context"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/boltdb"
)

func TestExportRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create temporary database
	tempDB, err := os.CreateTemp("", "test_exports_*.db")
	c.Assert(err, qt.IsNil)
	defer os.Remove(tempDB.Name())
	tempDB.Close()

	db, err := bolt.Open(tempDB.Name(), 0600, nil)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	reg := boltdb.NewExportRegistry(db)

	// Test data
	now := models.Date(time.Now().Format("2006-01-02"))
	selectedItems := models.ValuerSlice[models.ExportSelectedItem]{
		{
			ID:   "location1",
			Type: models.ExportSelectedItemTypeLocation,
			Name: "Test Location",
		},
	}

	export := models.Export{
		Type:            models.ExportTypeSelectedItems,
		Status:          models.ExportStatusPending,
		IncludeFileData: true,
		SelectedItems:   selectedItems,
		Description:     "Test export",
		CreatedDate:     &now,
	}

	ctx := context.Background()

	// Create export
	created, err := reg.Create(ctx, export)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.Type, qt.Equals, models.ExportTypeSelectedItems)
	c.Assert(created.Status, qt.Equals, models.ExportStatusPending)
	c.Assert(created.Description, qt.Equals, "Test export")
	c.Assert(len(created.SelectedItems), qt.Equals, 1)
	c.Assert(created.SelectedItems[0].ID, qt.Equals, "location1")

	// Get export
	retrieved, err := reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved.ID, qt.Equals, created.ID)
	c.Assert(retrieved.Type, qt.Equals, models.ExportTypeSelectedItems)
	c.Assert(len(retrieved.SelectedItems), qt.Equals, 1)
}

func TestExportRegistry_Update(t *testing.T) {
	c := qt.New(t)

	// Create temporary database
	tempDB, err := os.CreateTemp("", "test_exports_*.db")
	c.Assert(err, qt.IsNil)
	defer os.Remove(tempDB.Name())
	tempDB.Close()

	db, err := bolt.Open(tempDB.Name(), 0600, nil)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	reg := boltdb.NewExportRegistry(db)

	// Test data
	now := models.Date(time.Now().Format("2006-01-02"))
	export := models.Export{
		Type:            models.ExportTypeSelectedItems,
		Status:          models.ExportStatusPending,
		IncludeFileData: true,
		Description:     "Test export",
		CreatedDate:     &now,
	}

	ctx := context.Background()

	// Create export
	created, err := reg.Create(ctx, export)
	c.Assert(err, qt.IsNil)

	// Update export
	created.Status = models.ExportStatusCompleted
	created.FilePath = "/path/to/export.xml"
	completedDate := models.Date(time.Now().Format("2006-01-02"))
	created.CompletedDate = &completedDate

	updated, err := reg.Update(ctx, *created)
	c.Assert(err, qt.IsNil)
	c.Assert(updated.Status, qt.Equals, models.ExportStatusCompleted)
	c.Assert(updated.FilePath, qt.Equals, "/path/to/export.xml")
	c.Assert(updated.CompletedDate, qt.IsNotNil)
}

func TestExportRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create temporary database
	tempDB, err := os.CreateTemp("", "test_exports_*.db")
	c.Assert(err, qt.IsNil)
	defer os.Remove(tempDB.Name())
	tempDB.Close()

	db, err := bolt.Open(tempDB.Name(), 0600, nil)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	reg := boltdb.NewExportRegistry(db)

	// Test data
	now := models.Date(time.Now().Format("2006-01-02"))
	export := models.Export{
		Type:            models.ExportTypeSelectedItems,
		Status:          models.ExportStatusPending,
		IncludeFileData: true,
		Description:     "Test export",
		CreatedDate:     &now,
	}

	ctx := context.Background()

	// Create export
	created, err := reg.Create(ctx, export)
	c.Assert(err, qt.IsNil)

	// Soft delete export
	err = reg.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	// Verify it's still accessible via Get (soft deleted)
	retrieved, err := reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved.IsDeleted(), qt.IsTrue)

	// Verify it's not in the regular list
	exports, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(exports), qt.Equals, 0)

	// Verify it's in the deleted list
	deletedExports, err := reg.ListDeleted(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(deletedExports), qt.Equals, 1)
	c.Assert(deletedExports[0].ID, qt.Equals, created.ID)

	// Hard delete export
	err = reg.HardDelete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	// Verify it's completely gone
	_, err = reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNotNil)
}

func TestExportRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)

	// Create temporary database
	tempDB, err := os.CreateTemp("", "test_exports_*.db")
	c.Assert(err, qt.IsNil)
	defer os.Remove(tempDB.Name())
	tempDB.Close()

	db, err := bolt.Open(tempDB.Name(), 0600, nil)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	reg := boltdb.NewExportRegistry(db)
	ctx := context.Background()

	// Test missing description
	export := models.Export{
		Type:   models.ExportTypeSelectedItems,
		Status: models.ExportStatusPending,
		// Description is missing
	}

	_, err = reg.Create(ctx, export)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "Description")

	// Test missing type
	export = models.Export{
		Description: "Test export",
		Status:      models.ExportStatusPending,
		// Type is missing
	}

	_, err = reg.Create(ctx, export)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "Type")
}

func TestExportRegistry_Create_DefaultValues(t *testing.T) {
	c := qt.New(t)

	// Create temporary database
	tempDB, err := os.CreateTemp("", "test_exports_*.db")
	c.Assert(err, qt.IsNil)
	defer os.Remove(tempDB.Name())
	tempDB.Close()

	db, err := bolt.Open(tempDB.Name(), 0600, nil)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	reg := boltdb.NewExportRegistry(db)
	ctx := context.Background()

	// Test default date and status setting
	export := models.Export{
		Type:        models.ExportTypeSelectedItems,
		Description: "Test export",
		// CreatedDate and Status are not set
	}

	created, err := reg.Create(ctx, export)
	c.Assert(err, qt.IsNil)
	c.Assert(created.CreatedDate, qt.IsNotNil)
	c.Assert(created.Status, qt.Equals, models.ExportStatusPending)
}
