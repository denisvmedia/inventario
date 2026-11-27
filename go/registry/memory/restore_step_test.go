package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

// The restore worker writes steps through the service registry, which fills
// in no tenant or group. Postgres refuses such a step, so memory does too.
func TestRestoreStepRegistry_ServiceCreateRequiresTenantAndGroup(t *testing.T) {
	tests := []struct {
		name string
		ids  models.TenantGroupAwareEntityID
	}{
		{name: "no tenant", ids: models.TenantGroupAwareEntityID{GroupID: "group-1", CreatedByUserID: "user-1"}},
		{name: "no group", ids: models.TenantGroupAwareEntityID{TenantID: "tenant-1", CreatedByUserID: "user-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			reg := memory.NewFactorySet().RestoreStepRegistryFactory.CreateServiceRegistry()

			_, err := reg.Create(context.Background(), models.RestoreStep{
				TenantGroupAwareEntityID: tt.ids,
				RestoreOperationID:       "restore-1",
				Name:                     "Initializing restore",
				Result:                   models.RestoreStepResultInProgress,
			})
			c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)

			steps, err := reg.ListByRestoreOperation(context.Background(), "restore-1")
			c.Assert(err, qt.IsNil)
			c.Assert(steps, qt.HasLen, 0)
		})
	}
}

func TestRestoreStepRegistry_ServiceCreateStoresTheOwner(t *testing.T) {
	c := qt.New(t)
	reg := memory.NewFactorySet().RestoreStepRegistryFactory.CreateServiceRegistry()

	created, err := reg.Create(context.Background(), models.RestoreStep{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-1", GroupID: "group-1", CreatedByUserID: "user-1"},
		RestoreOperationID:       "restore-1",
		Name:                     "Initializing restore",
		Result:                   models.RestoreStepResultInProgress,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created.TenantID, qt.Equals, "tenant-1")
	c.Assert(created.GroupID, qt.Equals, "group-1")
}
