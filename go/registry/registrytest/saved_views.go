package registrytest

import (
	"context"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// SavedViewEnv is what RunSavedViewContract needs from a backend: one
// factory, two members of one empty group (Ctx for Alice, OtherCtx for Bob),
// and Alice in a second, empty group (OtherGroupCtx).
type SavedViewEnv struct {
	Views         registry.SavedViewRegistryFactory
	Ctx           context.Context
	OtherCtx      context.Context
	OtherGroupCtx context.Context
}

func newView(name, query string, shared bool) models.SavedView {
	return models.SavedView{
		Name:          name,
		Icon:          models.SavedViewIconBookmark,
		Entity:        models.SavedViewEntityCommodities,
		Query:         query,
		QueryVersion:  models.SavedViewQueryVersion,
		Shared:        shared,
		ShowInSidebar: true,
		// A caller-set order is ignored on create.
		SortOrder: 99,
	}
}

func viewNames(views []*models.SavedView) []string {
	out := make([]string, 0, len(views))
	for _, v := range views {
		out = append(out, v.Name)
	}
	return out
}

// RunSavedViewContract checks that a backend's SavedViewRegistry keeps the
// visibility rule, the order and the group-wide rewrite that
// registry.SavedViewRegistry describes. newEnv is called once per subtest.
func RunSavedViewContract(t *testing.T, newEnv func(t *testing.T) SavedViewEnv) {
	type fixture struct {
		env                         SavedViewEnv
		alice, bob, aliceOtherGroup registry.SavedViewRegistry
		mine, bobShared, bobPrivate *models.SavedView
		mineLater                   *models.SavedView
	}
	setup := func(t *testing.T) fixture {
		c := qt.New(t)
		env := newEnv(t)
		f := fixture{
			env:             env,
			alice:           env.Views.MustCreateUserRegistry(env.Ctx),
			bob:             env.Views.MustCreateUserRegistry(env.OtherCtx),
			aliceOtherGroup: env.Views.MustCreateUserRegistry(env.OtherGroupCtx),
		}
		var err error
		f.mine, err = f.alice.Create(env.Ctx, newView("Mine", "status=in_use", false))
		c.Assert(err, qt.IsNil)
		f.bobShared, err = f.bob.Create(env.OtherCtx, newView("Bob shared", "tags=fragile", true))
		c.Assert(err, qt.IsNil)
		f.bobPrivate, err = f.bob.Create(env.OtherCtx, newView("Bob private", "tags=fragile,red&tags_exclude=blue", false))
		c.Assert(err, qt.IsNil)
		f.mineLater, err = f.alice.Create(env.Ctx, newView("Mine later", "lent_out=true", false))
		c.Assert(err, qt.IsNil)
		return f
	}

	t.Run("create appends to the group and stamps the creator", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		c.Assert(f.mine.SortOrder, qt.Equals, 0)
		c.Assert(f.bobShared.SortOrder, qt.Equals, 1)
		c.Assert(f.bobPrivate.SortOrder, qt.Equals, 2)
		c.Assert(f.mineLater.SortOrder, qt.Equals, 3)
		c.Assert(f.mine.CreatedByUserID, qt.Not(qt.Equals), "")
		c.Assert(f.mine.CreatedByUserID, qt.Not(qt.Equals), f.bobShared.CreatedByUserID)
		c.Assert(f.mine.ID, qt.Not(qt.Equals), "")
		c.Assert(f.mine.UUID, qt.Not(qt.Equals), "")

		got, err := f.alice.Get(f.env.Ctx, f.mine.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Name, qt.Equals, "Mine")
		c.Assert(got.Query, qt.Equals, "status=in_use")
		c.Assert(got.ShowInSidebar, qt.IsTrue)
		c.Assert(got.GroupID, qt.Equals, f.mine.GroupID)
	})

	t.Run("a member sees their own views and the shared ones", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx

		views, err := f.alice.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(viewNames(views), qt.DeepEquals, []string{"Mine", "Bob shared", "Mine later"})
		n, err := f.alice.Count(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 3)

		views, err = f.bob.List(f.env.OtherCtx)
		c.Assert(err, qt.IsNil)
		c.Assert(viewNames(views), qt.DeepEquals, []string{"Bob shared", "Bob private"})

		_, err = f.alice.Get(ctx, f.bobPrivate.ID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		_, err = f.alice.Get(ctx, f.bobShared.ID)
		c.Assert(err, qt.IsNil)

		hijack := *f.bobPrivate
		hijack.Name = "Taken over"
		_, err = f.alice.Update(ctx, hijack)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		c.Assert(f.alice.Delete(ctx, f.bobPrivate.ID), qt.ErrorIs, registry.ErrNotFound)

		still, err := f.bob.Get(f.env.OtherCtx, f.bobPrivate.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(still.Name, qt.Equals, "Bob private")
	})

	t.Run("another group sees none of them", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		views, err := f.aliceOtherGroup.List(f.env.OtherGroupCtx)
		c.Assert(err, qt.IsNil)
		c.Assert(views, qt.HasLen, 0)
		_, err = f.aliceOtherGroup.Get(f.env.OtherGroupCtx, f.mine.ID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

		other, err := f.aliceOtherGroup.Create(f.env.OtherGroupCtx, newView("Elsewhere", "", false))
		c.Assert(err, qt.IsNil)
		c.Assert(other.SortOrder, qt.Equals, 0)
	})

	t.Run("update keeps the creator and the group", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		next := *f.mine
		next.Name = "Renamed"
		next.Shared = true
		next.CreatedByUserID = "someone-else"
		updated, err := f.alice.Update(ctx, next)
		c.Assert(err, qt.IsNil)
		c.Assert(updated.Name, qt.Equals, "Renamed")
		c.Assert(updated.CreatedByUserID, qt.Equals, f.mine.CreatedByUserID)

		got, err := f.bob.Get(f.env.OtherCtx, f.mine.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Name, qt.Equals, "Renamed")
		c.Assert(got.CreatedByUserID, qt.Equals, f.mine.CreatedByUserID)
		c.Assert(got.GroupID, qt.Equals, f.mine.GroupID)
	})

	t.Run("list orders by sort order, then name", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		tie := *f.mineLater
		tie.SortOrder = f.mine.SortOrder
		tie.Name = "a view"
		_, err := f.alice.Update(ctx, tie)
		c.Assert(err, qt.IsNil)
		views, err := f.alice.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(viewNames(views), qt.DeepEquals, []string{"a view", "Mine", "Bob shared"})
	})

	t.Run("reorder permutes the positions of the listed views only", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		c.Assert(f.alice.Reorder(ctx, []string{f.mineLater.ID, f.mine.ID}), qt.IsNil)

		views, err := f.alice.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(viewNames(views), qt.DeepEquals, []string{"Mine later", "Bob shared", "Mine"})
		shared, err := f.bob.Get(f.env.OtherCtx, f.bobShared.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(shared.SortOrder, qt.Equals, 1)
		private, err := f.bob.Get(f.env.OtherCtx, f.bobPrivate.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(private.SortOrder, qt.Equals, 2)
	})

	t.Run("reorder refuses an invisible or repeated view and changes nothing", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		err := f.alice.Reorder(ctx, []string{f.mineLater.ID, f.bobPrivate.ID})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		err = f.alice.Reorder(ctx, []string{f.mineLater.ID, f.mine.ID, f.mineLater.ID})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		err = f.alice.Reorder(ctx, []string{f.mineLater.ID, "no-such-view"})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

		views, err := f.alice.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(viewNames(views), qt.DeepEquals, []string{"Mine", "Bob shared", "Mine later"})
	})

	t.Run("rewrite reaches every view of the group", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		other, err := f.aliceOtherGroup.Create(f.env.OtherGroupCtx, newView("Elsewhere", "tags=fragile", false))
		c.Assert(err, qt.IsNil)

		n, err := f.alice.RewriteQueries(ctx, func(v models.SavedView) (string, bool) {
			if !strings.Contains(v.Query, "fragile") {
				return "", false
			}
			return strings.ReplaceAll(v.Query, "fragile", "delicate"), true
		})
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 2)

		private, err := f.bob.Get(f.env.OtherCtx, f.bobPrivate.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(private.Query, qt.Equals, "tags=delicate,red&tags_exclude=blue")
		shared, err := f.bob.Get(f.env.OtherCtx, f.bobShared.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(shared.Query, qt.Equals, "tags=delicate")
		mine, err := f.alice.Get(ctx, f.mine.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(mine.Query, qt.Equals, "status=in_use")
		untouched, err := f.aliceOtherGroup.Get(f.env.OtherGroupCtx, other.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(untouched.Query, qt.Equals, "tags=fragile")
	})

	t.Run("delete removes a visible view", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		c.Assert(f.alice.Delete(ctx, f.mine.ID), qt.IsNil)
		_, err := f.alice.Get(ctx, f.mine.ID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		c.Assert(f.alice.Delete(ctx, f.mine.ID), qt.ErrorIs, registry.ErrNotFound)
	})
}
