package registrytest

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// SavedViewEnv is what RunSavedViewContract needs from a backend: the saved
// view factory and the tag factory whose renames rewrite the views, two
// members of one empty group (Ctx for Alice, OtherCtx for Bob), and Alice in
// a second, empty group (OtherGroupCtx).
type SavedViewEnv struct {
	Views         registry.SavedViewRegistryFactory
	Tags          registry.TagRegistryFactory
	Ctx           context.Context
	OtherCtx      context.Context
	OtherGroupCtx context.Context
}

// newTag creates an item tag in the group of ctx.
func newTag(c *qt.C, env SavedViewEnv, ctx context.Context, slug string) *models.Tag {
	c.Helper()
	return newTagOfKind(c, env, ctx, models.TagKindCommodity, slug)
}

func newTagOfKind(c *qt.C, env SavedViewEnv, ctx context.Context, kind models.TagKind, slug string) *models.Tag {
	c.Helper()
	tag, err := env.Tags.MustCreateUserRegistry(ctx).Create(ctx, models.Tag{
		Kind:  kind,
		Slug:  slug,
		Label: slug,
		Color: models.TagColorBlue,
	})
	c.Assert(err, qt.IsNil)
	return tag
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

// replaceTag is a SavedViewTagRewrite for the plain queries of these tests:
// it renames from wherever it is a whole value of tags or tags_exclude.
func replaceTag(view models.SavedView, from, to string) (string, bool) {
	params := strings.Split(view.Query, "&")
	changed := false
	for i, p := range params {
		key, value, ok := strings.Cut(p, "=")
		if !ok || (key != "tags" && key != "tags_exclude") {
			continue
		}
		slugs := strings.Split(value, ",")
		for j, slug := range slugs {
			if slug == from {
				slugs[j] = to
				changed = true
			}
		}
		params[i] = key + "=" + strings.Join(slugs, ",")
	}
	return strings.Join(params, "&"), changed
}

func viewNames(views []*models.SavedView) []string {
	out := make([]string, 0, len(views))
	for _, v := range views {
		out = append(out, v.Name)
	}
	return out
}

// RunSavedViewContract checks that a backend's SavedViewRegistry keeps the
// visibility rule, the order, the per-member placements and the checked
// writes that registry.SavedViewRegistry describes, and that a tag rename
// rewrites the views as registry.TagRegistry.RenameAtomic describes.
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
	runSavedViewOrder(t, setup)
	runSavedViewTagRenames(t, setup)
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
		errs := make([]error, 6)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Go(func() {
				_, errs[i] = f.alice.CreateChecked(ctx, newView("Racing", "", false), nil, func(t registry.SavedViewTally) error {
					if t.Own >= 3 {
						return full
					}
					return nil
				})
			})
		}
		wg.Wait()
		var created int
		for _, err := range errs {
			if err == nil {
				created++
				continue
			}
			c.Assert(err, qt.ErrorIs, full, qt.Commentf("a create failed for another reason than the limit"))
		}
		c.Assert(created, qt.Equals, 1)
		views, err := f.alice.List(ctx)
		c.Assert(err, qt.IsNil)
		racing := 0
		for _, v := range views {
			if v.Name == "Racing" {
				racing++
			}
		}
		c.Assert(racing, qt.Equals, 1, qt.Commentf("a refused create stored its view"))
	})

	t.Run("a tag rename waits for an update func and lands on top of it", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx, bobCtx := f.env.Ctx, f.env.OtherCtx
		tag := newTag(c, f.env, ctx, "sold-items")
		done := make(chan error, 1)
		finishedEarly := false
		_, err := f.alice.UpdateFunc(ctx, f.mine.ID, nil, func(v *models.SavedView, _ registry.SavedViewTally) error {
			go func() {
				_, _, err := f.env.Tags.MustCreateUserRegistry(bobCtx).RenameAtomic(bobCtx, tag.ID, "", "sold", "",
					func(v models.SavedView, _, _ string) (string, bool) {
						return "status=sold", v.Name == "Renamed under the lock"
					})
				done <- err
			}()
			// Without the lock the rewrite would land here and the write
			// below would put the old query back.
			time.Sleep(100 * time.Millisecond)
			select {
			case err := <-done:
				finishedEarly = true
				done <- err
			default:
			}
			v.Name = "Renamed under the lock"
			return nil
		})
		c.Assert(err, qt.IsNil)
		c.Assert(finishedEarly, qt.IsFalse, qt.Commentf("the rename finished while the update held the view"))
		c.Assert(<-done, qt.IsNil)
		got, err := f.alice.Get(ctx, f.mine.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Name, qt.Equals, "Renamed under the lock")
		c.Assert(got.Query, qt.Equals, "status=sold")
	})

	t.Run("update func writes what mutate leaves and keeps the managed fields", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		updated, err := f.alice.UpdateFunc(ctx, f.mineLater.ID, nil, func(v *models.SavedView, tally registry.SavedViewTally) error {
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
		_, err = f.alice.UpdateFunc(ctx, f.mine.ID, nil, func(v *models.SavedView, _ registry.SavedViewTally) error {
			v.Name = "Never stored"
			return refused
		})
		c.Assert(err, qt.ErrorIs, refused)
		got, err = f.alice.Get(ctx, f.mine.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Name, qt.Equals, "Mine")

		_, err = f.alice.UpdateFunc(ctx, f.bobPrivate.ID, nil, func(*models.SavedView, registry.SavedViewTally) error {
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

		_, err = f.bob.UpdateFunc(bobCtx, f.bobShared.ID, nil, func(v *models.SavedView, _ registry.SavedViewTally) error {
			v.Shared = false
			return nil
		})
		c.Assert(err, qt.IsNil)
		placements, err := f.alice.Placements(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(placements, qt.HasLen, 0)

		// Shared again, it is in nobody else's sidebar until they put it there.
		_, err = f.bob.UpdateFunc(bobCtx, f.bobShared.ID, nil, func(v *models.SavedView, _ registry.SavedViewTally) error {
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

	t.Run("update func stores the placement with the view", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx, bobCtx := f.env.Ctx, f.env.OtherCtx
		updated, err := f.alice.UpdateFunc(ctx, f.bobShared.ID,
			&registry.SavedViewPlacementPatch{ShowOnDashboard: new(true)},
			func(v *models.SavedView, _ registry.SavedViewTally) error {
				v.Name = "Renamed by Alice"
				return nil
			})
		c.Assert(err, qt.IsNil)
		c.Assert(updated.Name, qt.Equals, "Renamed by Alice")
		placements, err := f.alice.Placements(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(placements[f.bobShared.ID].ShowOnDashboard, qt.IsTrue)
		c.Assert(placements[f.bobShared.ID].ShowInSidebar, qt.IsFalse, qt.Commentf("a nil field is false on insert"))

		// A refused mutate stores neither.
		refused := errors.New("refused")
		_, err = f.alice.UpdateFunc(ctx, f.bobShared.ID,
			&registry.SavedViewPlacementPatch{ShowOnDashboard: new(false)},
			func(*models.SavedView, registry.SavedViewTally) error { return refused })
		c.Assert(err, qt.ErrorIs, refused)
		placements, err = f.alice.Placements(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(placements[f.bobShared.ID].ShowOnDashboard, qt.IsTrue)

		// A view the user no longer sees once written gets no placement:
		// Alice unshares Bob's view.
		_, err = f.alice.UpdateFunc(ctx, f.bobShared.ID,
			&registry.SavedViewPlacementPatch{ShowInSidebar: new(true)},
			func(v *models.SavedView, _ registry.SavedViewTally) error {
				v.Shared = false
				return nil
			})
		c.Assert(err, qt.IsNil)
		placements, err = f.alice.Placements(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(placements, qt.HasLen, 0)
		views, err := f.env.Views.CreateServiceRegistry().List(bobCtx)
		c.Assert(err, qt.IsNil)
		for _, v := range views {
			if v.ID == f.bobShared.ID {
				c.Assert(v.Shared, qt.IsFalse)
			}
		}
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

// runSavedViewOrder checks reorder and delete.
func runSavedViewOrder(t *testing.T, setup func(*testing.T) savedViewFixture) {
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

	t.Run("a list never sees a reorder half done", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		forward := []string{"Mine", "Bob shared", "Mine later"}
		backward := []string{"Mine later", "Bob shared", "Mine"}
		done := make(chan struct{})
		reordered := make(chan error, 1)
		go func() {
			defer close(reordered)
			ids := [][]string{
				{f.mineLater.ID, f.bobShared.ID, f.mine.ID},
				{f.mine.ID, f.bobShared.ID, f.mineLater.ID},
			}
			for i := 0; ; i++ {
				select {
				case <-done:
					return
				default:
				}
				if err := f.alice.Reorder(ctx, ids[i%2]); err != nil {
					reordered <- err
					return
				}
			}
		}()
		for range 200 {
			views, err := f.alice.List(ctx)
			if err != nil {
				close(done)
				c.Fatal(err)
			}
			names := viewNames(views)
			if !slices.Equal(names, forward) && !slices.Equal(names, backward) {
				close(done)
				c.Fatalf("list returned %v, neither the order before nor after a reorder", names)
			}
		}
		close(done)
		c.Assert(<-reordered, qt.IsNil)
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

// runSavedViewTagRenames checks that a tag rename rewrites the views of its
// group in the rename itself.
func runSavedViewTagRenames(t *testing.T, setup func(*testing.T) savedViewFixture) {
	t.Run("a tag rename rewrites every view of the group", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		other, err := f.aliceOtherGroup.Create(f.env.OtherGroupCtx, newView("Elsewhere", "tags=fragile", false))
		c.Assert(err, qt.IsNil)
		tag := newTag(c, f.env, ctx, "fragile")

		var calls []string
		_, from, err := f.env.Tags.MustCreateUserRegistry(ctx).RenameAtomic(ctx, tag.ID, "", "delicate", "",
			func(v models.SavedView, from, to string) (string, bool) {
				calls = append(calls, from+">"+to)
				return replaceTag(v, from, to)
			})
		c.Assert(err, qt.IsNil)
		c.Assert(from, qt.Equals, "fragile")
		// Every view of the group, Bob's personal one included.
		c.Assert(calls, qt.DeepEquals, []string{"fragile>delicate", "fragile>delicate", "fragile>delicate", "fragile>delicate"})

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

	t.Run("a rename that keeps the slug, or renames a file tag, leaves the views alone", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		tags := f.env.Tags.MustCreateUserRegistry(ctx)
		called := false
		rewrite := func(models.SavedView, string, string) (string, bool) {
			called = true
			return "status=sold", true
		}
		item := newTag(c, f.env, ctx, "fragile")
		_, _, err := tags.RenameAtomic(ctx, item.ID, "Fragile things", "", "", rewrite)
		c.Assert(err, qt.IsNil)
		_, _, err = tags.RenameAtomic(ctx, item.ID, "", "fragile", "", rewrite)
		c.Assert(err, qt.IsNil)
		file := newTagOfKind(c, f.env, ctx, models.TagKindFile, "red")
		_, _, err = tags.RenameAtomic(ctx, file.ID, "", "crimson", "", rewrite)
		c.Assert(err, qt.IsNil)
		c.Assert(called, qt.IsFalse)
	})

	t.Run("a rename of a tag that does not exist changes nothing", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx := f.env.Ctx
		called := false
		_, _, err := f.env.Tags.MustCreateUserRegistry(ctx).RenameAtomic(ctx, "no-such-tag", "", "sold", "",
			func(models.SavedView, string, string) (string, bool) {
				called = true
				return "status=sold", true
			})
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		c.Assert(called, qt.IsFalse)
		got, err := f.alice.Get(ctx, f.mine.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Query, qt.Equals, "status=in_use")
	})

	// Tag A goes from red to crimson while tag B goes from scarlet to red.
	// If B could take red before A's views moved to crimson, A's rewrite
	// would then also catch B's views and move them to crimson.
	t.Run("a rename that takes a freed slug waits until the views have followed the first tag", func(t *testing.T) {
		c := qt.New(t)
		f := setup(t)
		ctx, bobCtx := f.env.Ctx, f.env.OtherCtx
		red := newTag(c, f.env, ctx, "red")
		scarlet := newTag(c, f.env, ctx, "scarlet")
		redView, err := f.alice.Create(ctx, newView("Red", "tags=red", false))
		c.Assert(err, qt.IsNil)
		scarletView, err := f.bob.Create(bobCtx, newView("Scarlet", "tags=scarlet", true))
		c.Assert(err, qt.IsNil)

		entered, release := make(chan struct{}), make(chan struct{})
		var hold, unhold sync.Once
		releaseOnce := func() { unhold.Do(func() { close(release) }) }
		// A test that stops early must not leave the first rename waiting.
		defer releaseOnce()
		first := make(chan error, 1)
		go func() {
			_, _, err := f.env.Tags.MustCreateUserRegistry(ctx).RenameAtomic(ctx, red.ID, "", "crimson", "",
				func(v models.SavedView, from, to string) (string, bool) {
					hold.Do(func() {
						close(entered)
						<-release
					})
					return replaceTag(v, from, to)
				})
			first <- err
		}()
		select {
		case <-entered:
		case err := <-first:
			c.Fatalf("the first rename returned without rewriting a view: %v", err)
		case <-time.After(30 * time.Second):
			c.Fatal("the first rename never reached the view rewrite")
		}
		second := make(chan error, 1)
		go func() {
			_, _, err := f.env.Tags.MustCreateUserRegistry(bobCtx).RenameAtomic(bobCtx, scarlet.ID, "", "red", "", replaceTag)
			second <- err
		}()
		// Give the second rename time to take red if nothing stops it.
		time.Sleep(200 * time.Millisecond)
		held, heldErr := f.env.Tags.MustCreateUserRegistry(ctx).Get(ctx, scarlet.ID)
		releaseOnce()
		c.Assert(<-first, qt.IsNil)
		c.Assert(<-second, qt.IsNil)
		c.Assert(heldErr, qt.IsNil)
		c.Assert(held.Slug, qt.Equals, "scarlet",
			qt.Commentf("the second rename took red while views still named it for the first tag"))

		got, err := f.alice.Get(ctx, redView.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Query, qt.Equals, "tags=crimson")
		got, err = f.bob.Get(bobCtx, scarletView.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.Query, qt.Equals, "tags=red")
	})
}
