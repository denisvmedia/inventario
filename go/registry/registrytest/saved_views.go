package registrytest

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
		Name:         name,
		Icon:         models.SavedViewIconBookmark,
		Entity:       models.SavedViewEntityCommodities,
		Query:        query,
		QueryVersion: models.SavedViewQueryVersion,
		Shared:       shared,
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
// visibility rule, the order, the per-member placements, the checked writes
// and the group-wide rewrite that registry.SavedViewRegistry describes.
// newEnv is called once per subtest.
func RunSavedViewContract(t *testing.T, newEnv func(t *testing.T) SavedViewEnv) {
	setup := func(t *testing.T) savedViewFixture {
		c := qt.New(t)
		env := newEnv(t)
		f := savedViewFixture{
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

	runSavedViewBasics(t, setup)
	runSavedViewCheckedWrites(t, setup)
	runSavedViewPlacements(t, setup)
	runSavedViewOrderAndRewrite(t, setup)
}

// savedViewFixture holds Alice's and Bob's registries in one group, Alice's
// in a second group, and four views: Alice's two personal ones and Bob's
// shared and personal ones, created in this order.
type savedViewFixture struct {
	env                         SavedViewEnv
	alice, bob, aliceOtherGroup registry.SavedViewRegistry
	mine, bobShared, bobPrivate *models.SavedView
	mineLater                   *models.SavedView
}

// runSavedViewBasics checks create, visibility and the plain update.
func runSavedViewBasics(t *testing.T, setup func(*testing.T) savedViewFixture) {
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

	t.Run("update keeps the stored position", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		moved := *f.mineLater
		moved.SortOrder = -1
		moved.Name = "Moved?"
		updated, err := f.alice.Update(ctx, moved)
		c.Assert(err, qt.IsNil)
		c.Assert(updated.SortOrder, qt.Equals, f.mineLater.SortOrder)
		views, err := f.alice.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(viewNames(views), qt.DeepEquals, []string{"Mine", "Bob shared", "Moved?"})
	})
}

// runSavedViewCheckedWrites checks the writes that run under the group's
// write lock with the tally.
func runSavedViewCheckedWrites(t *testing.T, setup func(*testing.T) savedViewFixture) {
	t.Run("create checked hands the tally to check under the lock", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		var seen registry.SavedViewTally
		created, err := f.alice.CreateChecked(ctx, newView("Checked", "", false),
			&registry.SavedViewPlacementPatch{ShowOnDashboard: new(true)},
			func(t registry.SavedViewTally) error {
				seen = t
				return nil
			})
		c.Assert(err, qt.IsNil)
		// Alice created two views; Bob shares one.
		c.Assert(seen, qt.Equals, registry.SavedViewTally{Own: 2, Shared: 1})
		c.Assert(created.SortOrder, qt.Equals, 4)
		c.Assert(created.CreatedByUserID, qt.Equals, f.mine.CreatedByUserID)

		placements, err := f.alice.Placements(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(placements[created.ID].ShowOnDashboard, qt.IsTrue)
		c.Assert(placements[created.ID].ShowInSidebar, qt.IsFalse)

		refused := errors.New("over the limit")
		_, err = f.alice.CreateChecked(ctx, newView("Refused", "", false), nil, func(registry.SavedViewTally) error {
			return refused
		})
		c.Assert(err, qt.ErrorIs, refused)
		views, err := f.alice.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(viewNames(views), qt.DeepEquals, []string{"Mine", "Bob shared", "Mine later", "Checked"})
	})

	t.Run("checked creates do not pass a limit together", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		full := errors.New("full")
		// Alice has two views; the check lets her have three.
		var created atomic.Int32
		var wg sync.WaitGroup
		for range 6 {
			wg.Go(func() {
				_, err := f.alice.CreateChecked(ctx, newView("Racing", "", false), nil, func(t registry.SavedViewTally) error {
					if t.Own >= 3 {
						return full
					}
					return nil
				})
				if err == nil {
					created.Add(1)
				}
			})
		}
		wg.Wait()
		c.Assert(created.Load(), qt.Equals, int32(1))
	})

	t.Run("a rewrite waits for an update func and lands on top of it", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx, bobCtx := f.env.Ctx, f.env.OtherCtx
		done := make(chan struct{})
		var rewritten atomic.Bool
		_, err := f.alice.UpdateFunc(ctx, f.mine.ID, func(v *models.SavedView, _ registry.SavedViewTally) error {
			go func() {
				defer close(done)
				_, err := f.bob.RewriteQueries(bobCtx, func(v models.SavedView) (string, bool) {
					return "status=sold", v.Name == "Renamed under the lock"
				})
				if err == nil {
					rewritten.Store(true)
				}
			}()
			// Without the lock the rewrite would land here and the write
			// below would put the old query back.
			time.Sleep(100 * time.Millisecond)
			v.Name = "Renamed under the lock"
			return nil
		})
		c.Assert(err, qt.IsNil)
		<-done
		c.Assert(rewritten.Load(), qt.IsTrue)
		got, err := f.alice.Get(ctx, f.mine.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Name, qt.Equals, "Renamed under the lock")
		c.Assert(got.Query, qt.Equals, "status=sold")
	})

	t.Run("update func writes what mutate leaves and keeps the managed fields", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		updated, err := f.alice.UpdateFunc(ctx, f.mineLater.ID, func(v *models.SavedView, tally registry.SavedViewTally) error {
			c.Check(tally, qt.Equals, registry.SavedViewTally{Own: 2, Shared: 1})
			v.Name = "Renamed"
			v.SortOrder = -1
			v.CreatedByUserID = "someone-else"
			v.GroupID = "another-group"
			return nil
		})
		c.Assert(err, qt.IsNil)
		c.Assert(updated.Name, qt.Equals, "Renamed")
		c.Assert(updated.SortOrder, qt.Equals, f.mineLater.SortOrder)
		c.Assert(updated.CreatedByUserID, qt.Equals, f.mineLater.CreatedByUserID)
		c.Assert(updated.GroupID, qt.Equals, f.mineLater.GroupID)
		got, err := f.alice.Get(ctx, f.mineLater.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Name, qt.Equals, "Renamed")
		c.Assert(got.SortOrder, qt.Equals, f.mineLater.SortOrder)

		refused := errors.New("not yours")
		_, err = f.alice.UpdateFunc(ctx, f.mine.ID, func(v *models.SavedView, _ registry.SavedViewTally) error {
			v.Name = "Never stored"
			return refused
		})
		c.Assert(err, qt.ErrorIs, refused)
		got, err = f.alice.Get(ctx, f.mine.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Name, qt.Equals, "Mine")

		_, err = f.alice.UpdateFunc(ctx, f.bobPrivate.ID, func(*models.SavedView, registry.SavedViewTally) error {
			return nil
		})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})
}

// runSavedViewPlacements checks that placement is per member.
func runSavedViewPlacements(t *testing.T, setup func(*testing.T) savedViewFixture) {
	t.Run("each member places the views they see on their own", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx, bobCtx := f.env.Ctx, f.env.OtherCtx

		// Sharing a view places it for nobody else.
		placements, err := f.alice.Placements(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(placements, qt.HasLen, 0)

		p, err := f.alice.SetPlacement(ctx, f.bobShared.ID, registry.SavedViewPlacementPatch{ShowInSidebar: new(true)})
		c.Assert(err, qt.IsNil)
		c.Assert(p.ShowInSidebar, qt.IsTrue)
		c.Assert(p.ShowOnDashboard, qt.IsFalse)
		p, err = f.alice.SetPlacement(ctx, f.bobShared.ID, registry.SavedViewPlacementPatch{ShowOnDashboard: new(true)})
		c.Assert(err, qt.IsNil)
		c.Assert(p.ShowInSidebar, qt.IsTrue, qt.Commentf("a nil field keeps the stored value"))
		c.Assert(p.ShowOnDashboard, qt.IsTrue)
		_, err = f.bob.SetPlacement(bobCtx, f.bobShared.ID, registry.SavedViewPlacementPatch{ShowInSidebar: new(false)})
		c.Assert(err, qt.IsNil)

		placements, err = f.alice.Placements(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(placements, qt.HasLen, 1)
		c.Assert(placements[f.bobShared.ID].ShowInSidebar, qt.IsTrue)
		placements, err = f.bob.Placements(bobCtx)
		c.Assert(err, qt.IsNil)
		c.Assert(placements[f.bobShared.ID].ShowInSidebar, qt.IsFalse)

		_, err = f.alice.SetPlacement(ctx, f.bobPrivate.ID, registry.SavedViewPlacementPatch{ShowInSidebar: new(true)})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		_, err = f.alice.SetPlacement(ctx, "no-such-view", registry.SavedViewPlacementPatch{ShowInSidebar: new(true)})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	})

	t.Run("a view that stops being shared leaves only its creator's placement", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx, bobCtx := f.env.Ctx, f.env.OtherCtx
		_, err := f.alice.SetPlacement(ctx, f.bobShared.ID, registry.SavedViewPlacementPatch{ShowInSidebar: new(true)})
		c.Assert(err, qt.IsNil)
		_, err = f.bob.SetPlacement(bobCtx, f.bobShared.ID, registry.SavedViewPlacementPatch{ShowOnDashboard: new(true)})
		c.Assert(err, qt.IsNil)

		_, err = f.bob.UpdateFunc(bobCtx, f.bobShared.ID, func(v *models.SavedView, _ registry.SavedViewTally) error {
			v.Shared = false
			return nil
		})
		c.Assert(err, qt.IsNil)
		placements, err := f.alice.Placements(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(placements, qt.HasLen, 0)

		// Shared again, it is in nobody else's sidebar until they put it there.
		_, err = f.bob.UpdateFunc(bobCtx, f.bobShared.ID, func(v *models.SavedView, _ registry.SavedViewTally) error {
			v.Shared = true
			return nil
		})
		c.Assert(err, qt.IsNil)
		placements, err = f.alice.Placements(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(placements, qt.HasLen, 0)
		placements, err = f.bob.Placements(bobCtx)
		c.Assert(err, qt.IsNil)
		c.Assert(placements[f.bobShared.ID].ShowOnDashboard, qt.IsTrue)
	})

	t.Run("deleting a view deletes every placement of it", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx, bobCtx := f.env.Ctx, f.env.OtherCtx
		_, err := f.alice.SetPlacement(ctx, f.bobShared.ID, registry.SavedViewPlacementPatch{ShowInSidebar: new(true)})
		c.Assert(err, qt.IsNil)
		c.Assert(f.bob.Delete(bobCtx, f.bobShared.ID), qt.IsNil)
		placements, err := f.alice.Placements(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(placements, qt.HasLen, 0)
	})
}

// runSavedViewOrderAndRewrite checks reorder, the group-wide rewrite and
// delete.
func runSavedViewOrderAndRewrite(t *testing.T, setup func(*testing.T) savedViewFixture) {
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
