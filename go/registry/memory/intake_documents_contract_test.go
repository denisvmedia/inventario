package memory_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestIntakeDocumentContract_Memory(t *testing.T) {
	registrytest.RunIntakeDocumentContract(t, newMemoryIntakeDocumentEnv)
}

// newMemoryIntakeDocumentEnv returns the registries of one empty group with
// an area to promote drafts into.
func newMemoryIntakeDocumentEnv(t *testing.T) registrytest.IntakeDocumentEnv {
	c := qt.New(t)
	ctx := appctx.WithUser(t.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "user-1"}, TenantID: "tenant-1"},
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "group-1"}, TenantID: "tenant-1"},
		Slug:                "group-1",
		GroupCurrency:       "USD",
	})
	fs := memory.NewFactorySet()
	set, err := fs.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	loc, err := set.LocationRegistry.Create(ctx, models.Location{Name: "Home", Address: "addr"})
	c.Assert(err, qt.IsNil)
	area, err := set.AreaRegistry.Create(ctx, models.Area{Name: "Shed", LocationID: loc.GetID()})
	c.Assert(err, qt.IsNil)
	return registrytest.IntakeDocumentEnv{
		Ctx:       ctx,
		Documents: set.IntakeDocumentRegistry,
		Service:   fs.IntakeDocumentRegistryFactory.CreateServiceRegistry(),
		Drafts:    set.CommodityDraftRegistry,
		AreaID:    area.GetID(),
	}
}

// The purgers treat delivered documents as the postgres ones do: account
// deletion hands the documents the user's token delivered to another member
// of their group and deletes the ones of a group without one, the group
// purge takes the group's, the tenant purge the rest.
func TestIntakeDocumentPurges_Memory(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()
	type stored struct {
		ctx context.Context
		id  string
	}
	seed := func(userID, groupID string) stored {
		ctx := appctx.WithUser(t.Context(), &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: userID}, TenantID: "tenant-1"},
		})
		ctx = appctx.WithGroup(ctx, &models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: groupID}, TenantID: "tenant-1"},
			Slug:                groupID,
			GroupCurrency:       "USD",
		})
		reg, err := fs.IntakeDocumentRegistryFactory.CreateUserRegistry(ctx)
		c.Assert(err, qt.IsNil)
		doc, _, err := reg.CreateOrGet(ctx, models.IntakeDocument{
			ExternalSystem: "paperless-ngx", ExternalDocumentID: userID, PayloadHash: "h",
		})
		c.Assert(err, qt.IsNil)
		return stored{ctx: ctx, id: doc.ID}
	}
	service := fs.IntakeDocumentRegistryFactory.CreateServiceRegistry()
	exists := func(s stored) bool {
		_, err := service.Get(t.Context(), s.id)
		if errors.Is(err, registry.ErrNotFound) {
			return false
		}
		c.Assert(err, qt.IsNil)
		return true
	}

	for _, m := range []struct {
		user string
		role models.GroupRole
	}{{"user-1", models.GroupRoleOwner}, {"user-3", models.GroupRoleUser}} {
		_, err := fs.GroupMembershipRegistry.Create(t.Context(), models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
			GroupID:             "group-c",
			MemberUserID:        m.user,
			Role:                m.role,
		})
		c.Assert(err, qt.IsNil)
	}

	leaving := seed("user-1", "group-a")
	staying := seed("user-2", "group-a")
	other := seed("user-2", "group-b")
	shared := seed("user-1", "group-c")

	c.Assert(fs.UserPurger.PurgeUserDependents(t.Context(), "tenant-1", "user-1"), qt.IsNil)
	c.Check(exists(leaving), qt.IsFalse)
	c.Check(exists(staying), qt.IsTrue)
	handedOver, err := service.Get(t.Context(), shared.id)
	c.Assert(err, qt.IsNil)
	c.Check(handedOver.CreatedByUserID, qt.Equals, "user-3")

	c.Assert(fs.GroupPurger.PurgeGroupDependents(t.Context(), "tenant-1", "group-a"), qt.IsNil)
	c.Check(exists(staying), qt.IsFalse)
	c.Check(exists(other), qt.IsTrue)

	c.Assert(fs.TenantPurger.PurgeTenantDependents(t.Context(), "tenant-1"), qt.IsNil)
	c.Check(exists(other), qt.IsFalse)
}
