package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/registry/registrytest"
	"go.5x5.cz/inventario/services"
)

func TestSavedViewContract_Postgres(t *testing.T) {
	registrytest.RunSavedViewContract(t, func(t *testing.T) registrytest.SavedViewEnv {
		c := qt.New(t)
		fx := newTagPGFixture(t)
		bob := mustCreateUser(c, context.Background(), fx.factorySet, fx.user.TenantID, "saved-views-bob@test-org.com")
		return registrytest.SavedViewEnv{
			Views:         fx.factorySet.SavedViewRegistryFactory,
			Tags:          fx.factorySet.TagRegistryFactory,
			Ctx:           fx.ctxA,
			OtherCtx:      tagCtxFor(bob, fx.groupAID),
			OtherGroupCtx: fx.ctxB,
		}
	})
}

// A member's saved views, shared ones included, go with the member, and so
// do the member's placements of other members' shared views: both user
// columns are NOT NULL foreign keys to users with NO ACTION, so a row left
// behind would block the final DELETE FROM users. Other members' placements
// of the purged member's shared views go with those views.
func TestUserPurger_Postgres_PurgesSavedViews(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := context.Background()
	bob := mustCreateUser(c, ctx, fx.factorySet, fx.user.TenantID, "saved-views-purge-bob@test-org.com")
	bobCtx := tagCtxFor(bob, fx.groupAID)

	views := fx.factorySet.SavedViewRegistryFactory
	pinned := &registry.SavedViewPlacementPatch{ShowInSidebar: new(true)}
	create := func(ctx context.Context, name string, shared bool) *models.SavedView {
		v, err := views.MustCreateUserRegistry(ctx).CreateChecked(ctx, models.SavedView{
			Name:         name,
			Icon:         models.SavedViewIconBookmark,
			Entity:       models.SavedViewEntityCommodities,
			QueryVersion: models.SavedViewQueryVersion,
			Shared:       shared,
		}, pinned, nil)
		c.Assert(err, qt.IsNil)
		return v
	}
	create(bobCtx, "Bob private", false)
	bobShared := create(bobCtx, "Bob shared", true)
	aliceView := create(fx.ctxA, "Alice", true)
	_, err := views.MustCreateUserRegistry(bobCtx).SetPlacement(bobCtx, aliceView.ID, *pinned)
	c.Assert(err, qt.IsNil)
	_, err = views.MustCreateUserRegistry(fx.ctxA).SetPlacement(fx.ctxA, bobShared.ID, *pinned)
	c.Assert(err, qt.IsNil)

	c.Assert(fx.factorySet.UserPurger.PurgeUserDependents(ctx, bob.TenantID, bob.ID), qt.IsNil)

	all, err := views.CreateServiceRegistry().List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 1)
	c.Assert(all[0].ID, qt.Equals, aliceView.ID)
	placements, err := views.MustCreateUserRegistry(fx.ctxA).Placements(fx.ctxA)
	c.Assert(err, qt.IsNil)
	c.Assert(placements, qt.HasLen, 1)
	c.Assert(placements[aliceView.ID].ShowInSidebar, qt.IsTrue)

	c.Assert(fx.factorySet.UserRegistry.Delete(ctx, bob.ID), qt.IsNil)
	_, err = fx.factorySet.UserRegistry.Get(ctx, bob.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// A tag rename rewrites the saved views of its group in the rename's own
// transaction, not over a second connection. So renames complete when the
// pool has no connection to spare: one rename with one connection, and two
// concurrent renames in two groups with two.
func TestTagService_Postgres_RenameRewritesViewsWithoutASecondConnection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		conns  int32
		groups int
	}{
		{name: "one rename, one connection", conns: 1, groups: 1},
		{name: "two concurrent renames, two connections", conns: 2, groups: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			fx := newTagPGFixture(t)
			type renamed struct {
				ctx    context.Context
				tag    *models.Tag
				viewID string
			}
			var renames []renamed
			for _, ctx := range []context.Context{fx.ctxA, fx.ctxB}[:tc.groups] {
				tag, err := fx.factorySet.TagRegistryFactory.MustCreateUserRegistry(ctx).Create(ctx, models.Tag{
					Kind: models.TagKindCommodity, Slug: "red", Label: "Red", Color: models.TagColorRed,
				})
				c.Assert(err, qt.IsNil)
				view, err := fx.factorySet.SavedViewRegistryFactory.MustCreateUserRegistry(ctx).Create(ctx, models.SavedView{
					Name:         "Red",
					Icon:         models.SavedViewIconBookmark,
					Entity:       models.SavedViewEntityCommodities,
					Query:        "tags=red",
					QueryVersion: models.SavedViewQueryVersion,
				})
				c.Assert(err, qt.IsNil)
				renames = append(renames, renamed{ctx: ctx, tag: tag, viewID: view.ID})
			}

			cfg, err := pgxpool.ParseConfig(skipIfNoPostgreSQL(t))
			c.Assert(err, qt.IsNil)
			cfg.MaxConns = tc.conns
			cfg.MinConns = 0
			pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
			c.Assert(err, qt.IsNil)
			dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
			tags := services.NewTagService(postgres.NewFactorySet(dbx))

			// On every exit the renames are canceled and waited for before
			// the pool closes, so a hung rename neither outlives the test
			// nor holds the pool open.
			stop, cancel := context.WithCancel(context.Background())
			var workers sync.WaitGroup
			defer func() {
				cancel()
				workers.Wait()
				_ = dbx.Close()
				pool.Close()
			}()
			done := make(chan error, len(renames))
			for _, r := range renames {
				workers.Go(func() {
					ctx, cancelRename := context.WithCancel(r.ctx)
					defer cancelRename()
					stopped := context.AfterFunc(stop, cancelRename)
					defer stopped()
					_, err := tags.RenameTag(ctx, r.tag.ID, "Crimson", "crimson", r.tag.Color)
					done <- err
				})
			}
			deadline := time.After(30 * time.Second)
			for range renames {
				select {
				case err := <-done:
					c.Assert(err, qt.IsNil)
				case <-deadline:
					c.Fatalf("a tag rename did not complete with %d pool connection(s)", tc.conns)
				}
			}

			for _, r := range renames {
				view, err := fx.factorySet.SavedViewRegistryFactory.MustCreateUserRegistry(r.ctx).Get(r.ctx, r.viewID)
				c.Assert(err, qt.IsNil)
				c.Assert(view.Query, qt.Equals, "tags=crimson")
			}
		})
	}
}

// UpdateFunc writes the view and the placement in one transaction: when the
// placement cannot be stored, the view change is rolled back too. A user who
// is not in the users table fails the placement's foreign key.
func TestSavedViewRegistry_Postgres_UpdateFuncRollsBackTheViewWithThePlacement(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	views := fx.factorySet.SavedViewRegistryFactory
	shared, err := views.MustCreateUserRegistry(fx.ctxA).Create(fx.ctxA, models.SavedView{
		Name:         "Team",
		Icon:         models.SavedViewIconBookmark,
		Entity:       models.SavedViewEntityCommodities,
		QueryVersion: models.SavedViewQueryVersion,
		Shared:       true,
	})
	c.Assert(err, qt.IsNil)

	ghost := *fx.user
	ghost.ID = "no-such-user"
	ghostCtx := tagCtxFor(&ghost, fx.groupAID)
	_, err = views.MustCreateUserRegistry(ghostCtx).UpdateFunc(ghostCtx, shared.ID,
		&registry.SavedViewPlacementPatch{ShowInSidebar: new(true)},
		func(v *models.SavedView, _ registry.SavedViewTally) error {
			v.Name = "Renamed"
			return nil
		})
	c.Assert(err, qt.ErrorMatches, `(?s).*failed to store saved view placement.*`)

	got, err := views.MustCreateUserRegistry(fx.ctxA).Get(fx.ctxA, shared.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Name, qt.Equals, "Team", qt.Commentf("the view change was kept without its placement"))
}
