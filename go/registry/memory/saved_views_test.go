package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/registrytest"
)

func memberCtx(userID, groupID string) context.Context {
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: "tenant-1",
		},
	})
	return appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: groupID},
			TenantID: "tenant-1",
		},
		Slug: groupID,
	})
}

func TestSavedViewContract_Memory(t *testing.T) {
	registrytest.RunSavedViewContract(t, func(*testing.T) registrytest.SavedViewEnv {
		fs := memory.NewFactorySet()
		return registrytest.SavedViewEnv{
			Views:         fs.SavedViewRegistryFactory,
			Tags:          fs.TagRegistryFactory,
			Ctx:           memberCtx("alice", "group-1"),
			OtherCtx:      memberCtx("bob", "group-1"),
			OtherGroupCtx: memberCtx("alice", "group-2"),
		}
	})
}

// Purging a member deletes the views they created, every placement of those
// views, and the member's placements of the views of others.
func TestUserPurger_Memory_PurgesSavedViewsAndPlacements(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()
	aliceCtx, bobCtx := memberCtx("alice", "group-1"), memberCtx("bob", "group-1")
	pinned := registry.SavedViewPlacementPatch{ShowInSidebar: new(true)}
	create := func(ctx context.Context, name string) *models.SavedView {
		v, err := fs.SavedViewRegistryFactory.MustCreateUserRegistry(ctx).CreateChecked(ctx, models.SavedView{
			Name: name, Icon: models.SavedViewIconBookmark, Entity: models.SavedViewEntityCommodities,
			QueryVersion: models.SavedViewQueryVersion, Shared: true,
		}, &pinned, nil)
		c.Assert(err, qt.IsNil)
		return v
	}
	bobView := create(bobCtx, "Bob")
	aliceView := create(aliceCtx, "Alice")
	alice := fs.SavedViewRegistryFactory.MustCreateUserRegistry(aliceCtx)
	bob := fs.SavedViewRegistryFactory.MustCreateUserRegistry(bobCtx)
	_, err := alice.SetPlacement(aliceCtx, bobView.ID, pinned)
	c.Assert(err, qt.IsNil)
	_, err = bob.SetPlacement(bobCtx, aliceView.ID, pinned)
	c.Assert(err, qt.IsNil)

	c.Assert(fs.UserPurger.PurgeUserDependents(context.Background(), "tenant-1", "bob"), qt.IsNil)

	views, err := alice.List(aliceCtx)
	c.Assert(err, qt.IsNil)
	c.Assert(views, qt.HasLen, 1)
	c.Assert(views[0].ID, qt.Equals, aliceView.ID)
	placements, err := alice.Placements(aliceCtx)
	c.Assert(err, qt.IsNil)
	c.Assert(placements, qt.HasLen, 1)
	c.Assert(placements[aliceView.ID].ShowInSidebar, qt.IsTrue)
	placements, err = bob.Placements(bobCtx)
	c.Assert(err, qt.IsNil)
	c.Assert(placements, qt.HasLen, 0)
}
