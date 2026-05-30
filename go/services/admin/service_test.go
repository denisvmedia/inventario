package admin

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestService_RevokeSystemAdmin_MissingGrantRegistryAuditsResolvedSubject(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	factorySet := memory.NewFactorySet()
	user, err := factorySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Email:               "subject@example.com",
		Name:                "Subject User",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	svc := &Service{factorySet: factorySet}
	svc.factorySet.SystemAdminGrantRegistry = nil

	resultUser, hadFlag, err := svc.RevokeSystemAdmin(ctx, user.ID, false)
	c.Assert(resultUser, qt.IsNil)
	c.Assert(hadFlag, qt.IsFalse)
	c.Assert(err, qt.ErrorIs, registry.ErrInvalidConfig)

	entries, err := factorySet.AuditLogRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(entries, qt.HasLen, 1)

	entry := entries[0]
	c.Assert(entry.Action, qt.Equals, "admin.revoke_system_admin")
	c.Assert(entry.Success, qt.IsFalse)
	c.Assert(entry.UserID, qt.IsNil)
	c.Assert(entry.TenantID, qt.IsNotNil)
	c.Assert(*entry.TenantID, qt.Equals, user.TenantID)
	c.Assert(entry.EntityType, qt.IsNotNil)
	c.Assert(*entry.EntityType, qt.Equals, "user")
	c.Assert(entry.EntityID, qt.IsNotNil)
	c.Assert(*entry.EntityID, qt.Equals, user.ID)
	c.Assert(entry.ErrorMessage, qt.IsNotNil)
}
