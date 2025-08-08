package models_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestTenantAwareEntityID_GetTenantID(t *testing.T) {
	// Happy path tests
	t.Run("get tenant ID", func(t *testing.T) {
		c := qt.New(t)
		entity := &models.TenantAwareEntityID{
			TenantID: "test-tenant-123",
		}

		tenantID := entity.GetTenantID()
		c.Assert(tenantID, qt.Equals, "test-tenant-123")
	})

	t.Run("get empty tenant ID", func(t *testing.T) {
		c := qt.New(t)
		entity := &models.TenantAwareEntityID{}

		tenantID := entity.GetTenantID()
		c.Assert(tenantID, qt.Equals, "")
	})
}

func TestTenantAwareEntityID_SetTenantID(t *testing.T) {
	// Happy path tests
	t.Run("set tenant ID", func(t *testing.T) {
		c := qt.New(t)
		entity := &models.TenantAwareEntityID{}

		entity.SetTenantID("new-tenant-456")
		c.Assert(entity.TenantID, qt.Equals, "new-tenant-456")
	})

	t.Run("overwrite existing tenant ID", func(t *testing.T) {
		c := qt.New(t)
		entity := &models.TenantAwareEntityID{
			TenantID: "old-tenant",
		}

		entity.SetTenantID("new-tenant")
		c.Assert(entity.TenantID, qt.Equals, "new-tenant")
	})
}

func TestTenantAwareEntityID_GetID(t *testing.T) {
	// Happy path tests
	t.Run("get entity ID", func(t *testing.T) {
		c := qt.New(t)
		entity := &models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "entity-123"},
		}

		entityID := entity.GetID()
		c.Assert(entityID, qt.Equals, "entity-123")
	})
}

func TestTenantAwareEntityID_SetID(t *testing.T) {
	// Happy path tests
	t.Run("set entity ID", func(t *testing.T) {
		c := qt.New(t)
		entity := &models.TenantAwareEntityID{}

		entity.SetID("new-entity-456")
		c.Assert(entity.ID, qt.Equals, "new-entity-456")
	})
}

func TestWithTenantID(t *testing.T) {
	// Happy path tests
	t.Run("set tenant ID using helper function", func(t *testing.T) {
		c := qt.New(t)
		entity := &models.TenantAwareEntityID{}

		result := models.WithTenantID("helper-tenant", entity)
		c.Assert(result.GetTenantID(), qt.Equals, "helper-tenant")
		c.Assert(result, qt.Equals, entity) // Should return the same instance
	})
}

func TestTenantAwareModels_TenantID(t *testing.T) {
	// Test that all tenant-aware models properly implement TenantAware interface
	t.Run("location has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		location := &models.Location{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "test-tenant",
			},
			Name:    "Test Location",
			Address: "123 Test St",
		}

		c.Assert(location.GetTenantID(), qt.Equals, "test-tenant")
		location.SetTenantID("new-tenant")
		c.Assert(location.GetTenantID(), qt.Equals, "new-tenant")
	})

	t.Run("area has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		area := &models.Area{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "test-tenant",
			},
			Name:       "Test Area",
			LocationID: "location-123",
		}

		c.Assert(area.GetTenantID(), qt.Equals, "test-tenant")
		area.SetTenantID("new-tenant")
		c.Assert(area.GetTenantID(), qt.Equals, "new-tenant")
	})

	t.Run("commodity has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		commodity := &models.Commodity{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "test-tenant",
			},
			Name:      "Test Commodity",
			ShortName: "Test",
			Type:      models.CommodityTypeElectronics,
			AreaID:    "area-123",
			Status:    models.CommodityStatusInUse,
			Count:     1,
		}

		c.Assert(commodity.GetTenantID(), qt.Equals, "test-tenant")
		commodity.SetTenantID("new-tenant")
		c.Assert(commodity.GetTenantID(), qt.Equals, "new-tenant")
	})

	t.Run("file entity has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		file := &models.FileEntity{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "test-tenant",
			},
			Title: "Test File",
			Type:  models.FileTypeDocument,
		}

		c.Assert(file.GetTenantID(), qt.Equals, "test-tenant")
		file.SetTenantID("new-tenant")
		c.Assert(file.GetTenantID(), qt.Equals, "new-tenant")
	})

	t.Run("export has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		export := &models.Export{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "test-tenant",
			},
			Type:   models.ExportTypeFullDatabase,
			Status: models.ExportStatusPending,
		}

		c.Assert(export.GetTenantID(), qt.Equals, "test-tenant")
		export.SetTenantID("new-tenant")
		c.Assert(export.GetTenantID(), qt.Equals, "new-tenant")
	})

	t.Run("image has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		image := &models.Image{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "test-tenant",
			},
			CommodityID: "commodity-123",
		}

		c.Assert(image.GetTenantID(), qt.Equals, "test-tenant")
		image.SetTenantID("new-tenant")
		c.Assert(image.GetTenantID(), qt.Equals, "new-tenant")
	})

	t.Run("invoice has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		invoice := &models.Invoice{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "test-tenant",
			},
			CommodityID: "commodity-123",
		}

		c.Assert(invoice.GetTenantID(), qt.Equals, "test-tenant")
		invoice.SetTenantID("new-tenant")
		c.Assert(invoice.GetTenantID(), qt.Equals, "new-tenant")
	})

	t.Run("manual has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		manual := &models.Manual{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "test-tenant",
			},
			CommodityID: "commodity-123",
		}

		c.Assert(manual.GetTenantID(), qt.Equals, "test-tenant")
		manual.SetTenantID("new-tenant")
		c.Assert(manual.GetTenantID(), qt.Equals, "new-tenant")
	})

	t.Run("restore operation has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		restoreOp := &models.RestoreOperation{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "test-tenant",
			},
			ExportID:    "export-123",
			Description: "Test restore",
			Status:      models.RestoreStatusPending,
		}

		c.Assert(restoreOp.GetTenantID(), qt.Equals, "test-tenant")
		restoreOp.SetTenantID("new-tenant")
		c.Assert(restoreOp.GetTenantID(), qt.Equals, "new-tenant")
	})

	t.Run("restore step has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		restoreStep := &models.RestoreStep{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "test-tenant",
			},
			RestoreOperationID: "restore-op-123",
			Name:               "Test step",
			Result:             models.RestoreStepResultTodo,
		}

		c.Assert(restoreStep.GetTenantID(), qt.Equals, "test-tenant")
		restoreStep.SetTenantID("new-tenant")
		c.Assert(restoreStep.GetTenantID(), qt.Equals, "new-tenant")
	})

	t.Run("user has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "test-tenant",
			},
			Email: "test@example.com",
			Name:  "Test User",
			Role:  models.UserRoleUser,
		}

		c.Assert(user.GetTenantID(), qt.Equals, "test-tenant")
		user.SetTenantID("new-tenant")
		c.Assert(user.GetTenantID(), qt.Equals, "new-tenant")
	})
}

func TestTenantAwareInterface_Compliance(t *testing.T) {
	// Test that all models implement the required interfaces
	t.Run("models implement TenantAware interface", func(t *testing.T) {
		c := qt.New(t)

		// Test that models implement TenantAware
		var _ models.TenantAware = &models.Location{}
		var _ models.TenantAware = &models.Area{}
		var _ models.TenantAware = &models.Commodity{}
		var _ models.TenantAware = &models.FileEntity{}
		var _ models.TenantAware = &models.Export{}
		var _ models.TenantAware = &models.Image{}
		var _ models.TenantAware = &models.Invoice{}
		var _ models.TenantAware = &models.Manual{}
		var _ models.TenantAware = &models.RestoreOperation{}
		var _ models.TenantAware = &models.RestoreStep{}
		var _ models.TenantAware = &models.User{}

		// Test that models implement TenantAwareIDable
		var _ models.TenantAwareIDable = &models.Location{}
		var _ models.TenantAwareIDable = &models.Area{}
		var _ models.TenantAwareIDable = &models.Commodity{}
		var _ models.TenantAwareIDable = &models.FileEntity{}
		var _ models.TenantAwareIDable = &models.Export{}
		var _ models.TenantAwareIDable = &models.Image{}
		var _ models.TenantAwareIDable = &models.Invoice{}
		var _ models.TenantAwareIDable = &models.Manual{}
		var _ models.TenantAwareIDable = &models.RestoreOperation{}
		var _ models.TenantAwareIDable = &models.RestoreStep{}
		var _ models.TenantAwareIDable = &models.User{}

		c.Assert(true, qt.IsTrue) // If we get here, all interfaces are implemented correctly
	})
}
