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
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID:        "test-tenant",
				CreatedByUserID: "test-user",
			},
			Name:    "Test Location",
			Address: "123 Test St",
		}

		c.Assert(location.GetTenantID(), qt.Equals, "test-tenant")
		location.SetTenantID("new-tenant")
		c.Assert(location.GetTenantID(), qt.Equals, "new-tenant")

		c.Assert(location.GetCreatedByUserID(), qt.Equals, "test-user")
		location.SetCreatedByUserID("new-user")
		c.Assert(location.GetCreatedByUserID(), qt.Equals, "new-user")
	})

	t.Run("area has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		area := &models.Area{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID:        "test-tenant",
				CreatedByUserID: "test-user",
			},
			Name:       "Test Area",
			LocationID: "location-123",
		}

		c.Assert(area.GetTenantID(), qt.Equals, "test-tenant")
		area.SetTenantID("new-tenant")
		c.Assert(area.GetTenantID(), qt.Equals, "new-tenant")

		c.Assert(area.GetCreatedByUserID(), qt.Equals, "test-user")
		area.SetCreatedByUserID("new-user")
		c.Assert(area.GetCreatedByUserID(), qt.Equals, "new-user")
	})

	t.Run("commodity has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		commodity := &models.Commodity{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID:        "test-tenant",
				CreatedByUserID: "test-user",
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

		c.Assert(commodity.GetCreatedByUserID(), qt.Equals, "test-user")
		commodity.SetCreatedByUserID("new-user")
		c.Assert(commodity.GetCreatedByUserID(), qt.Equals, "new-user")
	})

	t.Run("file entity has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		file := &models.FileEntity{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
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
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID: "test-tenant",
			},
			Type:   models.ExportTypeFullDatabase,
			Status: models.ExportStatusPending,
		}

		c.Assert(export.GetTenantID(), qt.Equals, "test-tenant")
		export.SetTenantID("new-tenant")
		c.Assert(export.GetTenantID(), qt.Equals, "new-tenant")
	})

	t.Run("restore operation has tenant ID", func(t *testing.T) {
		c := qt.New(t)
		restoreOp := &models.RestoreOperation{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
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
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
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
		var _ models.TenantAware = &models.RestoreOperation{}
		var _ models.TenantAware = &models.RestoreStep{}
		var _ models.TenantAware = &models.User{}

		// Test that models implement TenantAwareIDable
		var _ models.TenantAwareIDable = &models.Location{}
		var _ models.TenantAwareIDable = &models.Area{}
		var _ models.TenantAwareIDable = &models.Commodity{}
		var _ models.TenantAwareIDable = &models.FileEntity{}
		var _ models.TenantAwareIDable = &models.Export{}
		var _ models.TenantAwareIDable = &models.RestoreOperation{}
		var _ models.TenantAwareIDable = &models.RestoreStep{}
		var _ models.TenantAwareIDable = &models.User{}

		c.Assert(true, qt.IsTrue) // If we get here, all interfaces are implemented correctly
	})
}

func TestUserAwareInterface_Compliance(t *testing.T) {
	// Test that all models implement the user-aware or group-aware interfaces
	t.Run("models implement correct awareness interfaces", func(t *testing.T) {
		c := qt.New(t)

		// Data models implement CreatedByUserAware (not UserAware)
		var _ models.CreatedByUserAware = &models.Location{}
		var _ models.CreatedByUserAware = &models.Area{}
		var _ models.CreatedByUserAware = &models.Commodity{}
		var _ models.CreatedByUserAware = &models.FileEntity{}
		var _ models.CreatedByUserAware = &models.Export{}
		var _ models.CreatedByUserAware = &models.RestoreOperation{}
		var _ models.CreatedByUserAware = &models.RestoreStep{}

		// User no longer implements UserAware — users.user_id was a
		// legacy self-FK dropped by issue #1289 Gap B. The row's own ID is
		// authoritative, and access control lives in group_memberships.

		// Data models implement TenantGroupAware
		var _ models.TenantGroupAware = &models.Location{}
		var _ models.TenantGroupAware = &models.Area{}
		var _ models.TenantGroupAware = &models.Commodity{}
		var _ models.TenantGroupAware = &models.FileEntity{}
		var _ models.TenantGroupAware = &models.Export{}
		var _ models.TenantGroupAware = &models.RestoreOperation{}
		var _ models.TenantGroupAware = &models.RestoreStep{}

		// User no longer implements TenantUserAware — see comment on
		// UserAware above.

		// Data models implement TenantGroupAwareIDable
		var _ models.TenantGroupAwareIDable = &models.Location{}
		var _ models.TenantGroupAwareIDable = &models.Area{}
		var _ models.TenantGroupAwareIDable = &models.Commodity{}
		var _ models.TenantGroupAwareIDable = &models.FileEntity{}
		var _ models.TenantGroupAwareIDable = &models.Export{}
		var _ models.TenantGroupAwareIDable = &models.RestoreOperation{}
		var _ models.TenantGroupAwareIDable = &models.RestoreStep{}

		// User implements TenantAwareIDable (not TenantUserAwareIDable —
		// users.user_id was dropped in #1289 Gap B).
		var _ models.TenantAwareIDable = &models.User{}

		c.Assert(true, qt.IsTrue) // If we get here, all interfaces are implemented correctly
	})
}
