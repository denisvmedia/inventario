package services_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/commodityquery"
)

type savedViewFixture struct {
	factory *registry.FactorySet
	svc     *services.SavedViewService
}

func newSavedViewFixture() *savedViewFixture {
	fs := memory.NewFactorySet()
	return &savedViewFixture{factory: fs, svc: services.NewSavedViewService(fs)}
}

func savedViewCtx(userID string) context.Context {
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: "tenant-1",
		},
	})
	return appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "tenant-1",
		},
		GroupCurrency: "USD",
	})
}

func actor(userID string, role models.GroupRole) services.SavedViewActor {
	return services.SavedViewActor{UserID: userID, Role: role}
}

func viewInput(name, query string, shared bool) services.SavedViewInput {
	return services.SavedViewInput{Name: name, Query: query, Shared: shared, ShowInSidebar: true}
}

func TestSavedViewService_Create_StoresTheCanonicalQuery(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	ctx := savedViewCtx("alice")

	entry, err := f.svc.Create(ctx, actor("alice", models.GroupRoleViewer), services.SavedViewInput{
		Name:  "  Fragile things  ",
		Query: "?tags=Fragile&sort=name&include_inactive=false",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(entry.View.Name, qt.Equals, "Fragile things")
	c.Assert(entry.View.Query, qt.Equals, "include_inactive=false&sort=name&tags=fragile")
	c.Assert(entry.View.QueryVersion, qt.Equals, models.SavedViewQueryVersion)
	c.Assert(entry.View.Icon, qt.Equals, models.SavedViewIconBookmark)
	c.Assert(entry.View.Entity, qt.Equals, models.SavedViewEntityCommodities)
	c.Assert(entry.Owned, qt.IsTrue)
	c.Assert(entry.CanEdit, qt.IsTrue)
	c.Assert(entry.CanDelete, qt.IsTrue)
	c.Assert(entry.QueryError, qt.Equals, (*commodityquery.Error)(nil))
}

func TestSavedViewService_Create_RefusesAQueryTheListCannotShow(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	_, err := f.svc.Create(savedViewCtx("alice"), actor("alice", models.GroupRoleUser),
		viewInput("Old", "warranty_expires_before=2026-01-01", false))
	qErr, ok := errors.AsType[*commodityquery.Error](err)
	c.Assert(ok, qt.IsTrue)
	c.Assert(qErr.Param, qt.Equals, "warranty_expires_before")
}

func TestSavedViewService_Create_RefusesAnInvalidName(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	_, err := f.svc.Create(savedViewCtx("alice"), actor("alice", models.GroupRoleUser), viewInput("   ", "", false))
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "name")
}

func TestSavedViewService_Sharing(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	aliceCtx, bobCtx, carolCtx := savedViewCtx("alice"), savedViewCtx("bob"), savedViewCtx("carol")
	viewer := actor("alice", models.GroupRoleViewer)
	user := actor("alice", models.GroupRoleUser)
	bob := actor("bob", models.GroupRoleUser)
	admin := actor("carol", models.GroupRoleAdmin)

	// A viewer keeps personal views but cannot share one.
	_, err := f.svc.Create(aliceCtx, viewer, viewInput("Shared", "", true))
	c.Assert(err, qt.ErrorIs, services.ErrSavedViewForbidden)
	personal, err := f.svc.Create(aliceCtx, viewer, viewInput("Personal", "", false))
	c.Assert(err, qt.IsNil)
	_, err = f.svc.Update(aliceCtx, viewer, personal.View.ID, services.SavedViewPatch{Shared: new(true)})
	c.Assert(err, qt.ErrorIs, services.ErrSavedViewForbidden)

	shared, err := f.svc.Update(aliceCtx, user, personal.View.ID, services.SavedViewPatch{Shared: new(true)})
	c.Assert(err, qt.IsNil)
	c.Assert(shared.View.Shared, qt.IsTrue)

	// Another member sees the shared view but may neither edit nor delete it.
	seen, err := f.svc.Get(bobCtx, bob, personal.View.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(seen.Owned, qt.IsFalse)
	c.Assert(seen.CanEdit, qt.IsFalse)
	c.Assert(seen.CanDelete, qt.IsFalse)
	_, err = f.svc.Update(bobCtx, bob, personal.View.ID, services.SavedViewPatch{Name: new("Mine now")})
	c.Assert(err, qt.ErrorIs, services.ErrSavedViewForbidden)
	c.Assert(f.svc.Delete(bobCtx, bob, personal.View.ID), qt.ErrorIs, services.ErrSavedViewForbidden)
	c.Assert(f.svc.Reorder(bobCtx, bob, []string{personal.View.ID}), qt.ErrorIs, services.ErrSavedViewForbidden)

	// The creator, demoted to viewer, can no longer change what the group
	// sees, but can still delete the view.
	asViewer, err := f.svc.Get(aliceCtx, viewer, personal.View.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(asViewer.CanEdit, qt.IsFalse)
	c.Assert(asViewer.CanDelete, qt.IsTrue)

	// An admin may edit and delete any shared view.
	renamed, err := f.svc.Update(carolCtx, admin, personal.View.ID, services.SavedViewPatch{Name: new("Group view")})
	c.Assert(err, qt.IsNil)
	c.Assert(renamed.View.Name, qt.Equals, "Group view")
	c.Assert(renamed.View.CreatedByUserID, qt.Equals, "alice")
	c.Assert(f.svc.Delete(carolCtx, admin, personal.View.ID), qt.IsNil)
}

func TestSavedViewService_PersonalViewsArePrivate(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	admin := actor("carol", models.GroupRoleOwner)
	mine, err := f.svc.Create(savedViewCtx("alice"), actor("alice", models.GroupRoleUser), viewInput("Mine", "", false))
	c.Assert(err, qt.IsNil)

	// Not even an owner of the group sees another member's personal view.
	_, err = f.svc.Get(savedViewCtx("carol"), admin, mine.View.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	entries, err := f.svc.List(savedViewCtx("carol"), admin)
	c.Assert(err, qt.IsNil)
	c.Assert(entries, qt.HasLen, 0)
	c.Assert(f.svc.Delete(savedViewCtx("carol"), admin, mine.View.ID), qt.ErrorIs, registry.ErrNotFound)
}

func TestSavedViewService_Limits(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	aliceCtx := savedViewCtx("alice")
	alice := actor("alice", models.GroupRoleUser)
	for i := range models.MaxSavedViewsPerUser {
		_, err := f.svc.Create(aliceCtx, alice, viewInput(fmt.Sprintf("View %d", i), "", true))
		c.Assert(err, qt.IsNil)
	}
	_, err := f.svc.Create(aliceCtx, alice, viewInput("One too many", "", false))
	c.Assert(err, qt.ErrorIs, services.ErrSavedViewLimitReached)

	// The group already shares MaxSharedSavedViews views, so another member
	// may keep a personal view but not share it.
	bobCtx := savedViewCtx("bob")
	bob := actor("bob", models.GroupRoleUser)
	_, err = f.svc.Create(bobCtx, bob, viewInput("Shared", "", true))
	c.Assert(err, qt.ErrorIs, services.ErrSavedViewLimitReached)
	personal, err := f.svc.Create(bobCtx, bob, viewInput("Personal", "", false))
	c.Assert(err, qt.IsNil)
	_, err = f.svc.Update(bobCtx, bob, personal.View.ID, services.SavedViewPatch{Shared: new(true)})
	c.Assert(err, qt.ErrorIs, services.ErrSavedViewLimitReached)
}

func TestSavedViewService_Update_ReplacesTheQuery(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	ctx := savedViewCtx("alice")
	alice := actor("alice", models.GroupRoleViewer)
	created, err := f.svc.Create(ctx, alice, viewInput("View", "status=in_use", false))
	c.Assert(err, qt.IsNil)

	updated, err := f.svc.Update(ctx, alice, created.View.ID, services.SavedViewPatch{
		Query:           new("lent_out=1&sort=-registered_date"),
		Layout:          new(models.SavedViewLayoutGrid),
		Icon:            new(models.SavedViewIconStar),
		ShowOnDashboard: new(true),
		ShowInSidebar:   new(false),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(updated.View.Query, qt.Equals, "lent_out=true&sort=-registered_date")
	c.Assert(updated.View.Layout, qt.Equals, models.SavedViewLayoutGrid)
	c.Assert(updated.View.Icon, qt.Equals, models.SavedViewIconStar)
	c.Assert(updated.ShowOnDashboard, qt.IsTrue)
	c.Assert(updated.ShowInSidebar, qt.IsFalse)
	c.Assert(updated.View.Name, qt.Equals, "View")

	_, err = f.svc.Update(ctx, alice, created.View.ID, services.SavedViewPatch{Query: new("lent_out=false")})
	_, ok := errors.AsType[*commodityquery.Error](err)
	c.Assert(ok, qt.IsTrue)
}

// A stored query the contract no longer accepts is reported, not run: the view
// gets a query error and no count, so it never selects other items than it was
// saved with.
func TestSavedViewService_StoredQueryThatNoLongerParses(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	ctx := savedViewCtx("alice")
	reg := f.factory.SavedViewRegistryFactory.MustCreateUserRegistry(ctx)
	pinned := registry.SavedViewPlacementPatch{ShowInSidebar: new(true)}
	broken, err := reg.CreateChecked(ctx, models.SavedView{
		Name: "Broken", Icon: models.SavedViewIconBookmark, Entity: models.SavedViewEntityCommodities,
		Query: "warranty_expires_before=2026-01-01", QueryVersion: models.SavedViewQueryVersion,
	}, &pinned, nil)
	c.Assert(err, qt.IsNil)
	future, err := reg.CreateChecked(ctx, models.SavedView{
		Name: "Future", Icon: models.SavedViewIconBookmark, Entity: models.SavedViewEntityCommodities,
		Query: "status=in_use", QueryVersion: models.SavedViewQueryVersion + 1,
	}, &pinned, nil)
	c.Assert(err, qt.IsNil)

	entries, err := f.svc.List(ctx, actor("alice", models.GroupRoleUser))
	c.Assert(err, qt.IsNil)
	c.Assert(entries, qt.HasLen, 2)
	c.Assert(entries[0].QueryError, qt.IsNotNil)
	c.Assert(entries[0].QueryError.Param, qt.Equals, "warranty_expires_before")
	c.Assert(entries[1].QueryError, qt.IsNotNil)

	set, err := f.factory.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	counts, err := f.svc.Counts(ctx, set)
	c.Assert(err, qt.IsNil)
	_, counted := counts[broken.ID]
	c.Assert(counted, qt.IsFalse)
	_, counted = counts[future.ID]
	c.Assert(counted, qt.IsFalse)

	// Saving a new query brings the view up to date.
	fixed, err := f.svc.Update(ctx, actor("alice", models.GroupRoleUser), future.ID, services.SavedViewPatch{Query: new("status=in_use")})
	c.Assert(err, qt.IsNil)
	c.Assert(fixed.View.QueryVersion, qt.Equals, models.SavedViewQueryVersion)
	c.Assert(fixed.QueryError, qt.Equals, (*commodityquery.Error)(nil))
}

func seedSavedViewCommodity(c *qt.C, f *savedViewFixture, ctx context.Context, name string, status models.CommodityStatus, tags ...string) *models.Commodity {
	c.Helper()
	reg := f.factory.CommodityRegistryFactory.MustCreateUserRegistry(ctx)
	created, err := reg.Create(ctx, models.Commodity{
		Name:      name,
		ShortName: name,
		Type:      models.CommodityTypeOther,
		Status:    status,
		Count:     1,
		Tags:      tags,
	})
	c.Assert(err, qt.IsNil)
	return created
}

// The counts use the filter of the list path, so each is the total the items
// list reports for the same query string, items in Trash left out. They cover
// the views the member shows in the sidebar or on the dashboard, and only
// those.
func TestSavedViewService_CountsMatchTheList(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	ctx := savedViewCtx("alice")
	alice := actor("alice", models.GroupRoleUser)
	seedSavedViewCommodity(c, f, ctx, "Vase", models.CommodityStatusInUse, "fragile")
	seedSavedViewCommodity(c, f, ctx, "Glass", models.CommodityStatusSold, "fragile")
	seedSavedViewCommodity(c, f, ctx, "Hammer", models.CommodityStatusInUse)
	trashed := seedSavedViewCommodity(c, f, ctx, "Broken vase", models.CommodityStatusInUse, "fragile")
	_, err := f.factory.CommodityRegistryFactory.MustCreateUserRegistry(ctx).Trash(ctx, trashed.ID, "alice", time.Now())
	c.Assert(err, qt.IsNil)

	fragile, err := f.svc.Create(ctx, alice, viewInput("Fragile", "tags=fragile", false))
	c.Assert(err, qt.IsNil)
	activeFragile, err := f.svc.Create(ctx, alice, viewInput("Active fragile", "tags=fragile&include_inactive=false", false))
	c.Assert(err, qt.IsNil)
	dashboard, err := f.svc.Create(ctx, alice, services.SavedViewInput{Name: "On the dashboard", Query: "", ShowOnDashboard: true})
	c.Assert(err, qt.IsNil)
	hidden, err := f.svc.Create(ctx, alice, services.SavedViewInput{Name: "Shown nowhere", Query: ""})
	c.Assert(err, qt.IsNil)
	// Bob shares a view and shows it in his sidebar; Alice has not placed it.
	bobShared, err := f.svc.Create(savedViewCtx("bob"), actor("bob", models.GroupRoleUser), viewInput("Bob shared", "tags=fragile", true))
	c.Assert(err, qt.IsNil)

	set, err := f.factory.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	counts, err := f.svc.Counts(ctx, set)
	c.Assert(err, qt.IsNil)
	c.Assert(counts, qt.DeepEquals, map[string]int{fragile.View.ID: 2, activeFragile.View.ID: 1, dashboard.View.ID: 3})
	_, counted := counts[hidden.View.ID]
	c.Assert(counted, qt.IsFalse)
	_, counted = counts[bobShared.View.ID]
	c.Assert(counted, qt.IsFalse)

	for id, want := range counts {
		entry, err := f.svc.Get(ctx, alice, id)
		c.Assert(err, qt.IsNil)
		q, err := commodityquery.ParseSaved(entry.View.Query)
		c.Assert(err, qt.IsNil)
		page, err := commodityquery.List(ctx, set, q, 0, 50)
		c.Assert(err, qt.IsNil)
		c.Assert(page.Total, qt.Equals, want)
	}
}

// A view keeps the seller filter of the list in its canonical form, and its
// count is the total the list reports for it.
func TestSavedViewService_KeepsAndCountsTheSellerFilter(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	ctx := savedViewCtx("alice")
	alice := actor("alice", models.GroupRoleUser)
	sellers := f.factory.SellerRegistryFactory.MustCreateUserRegistry(ctx)
	shop, err := sellers.Create(ctx, models.Seller{Name: "Hardware shop"})
	c.Assert(err, qt.IsNil)
	market, err := sellers.Create(ctx, models.Seller{Name: "Market"})
	c.Assert(err, qt.IsNil)
	reg := f.factory.CommodityRegistryFactory.MustCreateUserRegistry(ctx)
	for name, seller := range map[string]string{"Drill": shop.ID, "Saw": shop.ID, "Lamp": market.ID, "Desk": ""} {
		item := models.Commodity{Name: name, ShortName: name, Type: models.CommodityTypeOther, Status: models.CommodityStatusInUse, Count: 1}
		if seller != "" {
			item.SellerID = new(seller)
		}
		_, err := reg.Create(ctx, item)
		c.Assert(err, qt.IsNil)
	}

	ids := []string{shop.ID, market.ID}
	slices.Sort(ids)
	both, err := f.svc.Create(ctx, alice, viewInput("Both shops", "seller_id="+ids[1]+"&seller_id="+ids[0], false))
	c.Assert(err, qt.IsNil)
	c.Assert(both.View.Query, qt.Equals, "seller_id="+ids[0]+"&seller_id="+ids[1])
	one, err := f.svc.Create(ctx, alice, viewInput("One shop", "seller_id="+shop.ID, false))
	c.Assert(err, qt.IsNil)

	set, err := f.factory.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	counts, err := f.svc.Counts(ctx, set)
	c.Assert(err, qt.IsNil)
	c.Assert(counts, qt.DeepEquals, map[string]int{both.View.ID: 3, one.View.ID: 2})
	q, err := commodityquery.ParseSaved(one.View.Query)
	c.Assert(err, qt.IsNil)
	page, err := commodityquery.List(ctx, set, q, 0, 50)
	c.Assert(err, qt.IsNil)
	c.Assert(page.Total, qt.Equals, 2)
}

// Renaming an item tag's slug carries every view of the group along,
// another member's personal view included.
func TestTagService_RenameTag_RewritesSavedViews(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	aliceCtx := savedViewCtx("alice")
	tags := services.NewTagService(f.factory)
	created, err := tags.EnsureTagsExist(aliceCtx, models.TagKindCommodity, []string{"fragile"})
	c.Assert(err, qt.IsNil)
	tag := created["fragile"]
	c.Assert(tag, qt.IsNotNil)

	bobView, err := f.svc.Create(savedViewCtx("bob"), actor("bob", models.GroupRoleUser), viewInput("Bob", "tags_exclude=fragile", false))
	c.Assert(err, qt.IsNil)
	aliceView, err := f.svc.Create(aliceCtx, actor("alice", models.GroupRoleUser), viewInput("Alice", "tags=fragile,red", false))
	c.Assert(err, qt.IsNil)

	_, err = tags.RenameTag(aliceCtx, tag.ID, "Delicate", "delicate", tag.Color)
	c.Assert(err, qt.IsNil)

	got, err := f.svc.Get(savedViewCtx("bob"), actor("bob", models.GroupRoleUser), bobView.View.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.View.Query, qt.Equals, "tags_exclude=delicate")
	got, err = f.svc.Get(aliceCtx, actor("alice", models.GroupRoleUser), aliceView.View.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.View.Query, qt.Equals, "tags=delicate%2Cred")

	// A label-only rename leaves the views alone.
	_, err = tags.RenameTag(aliceCtx, tag.ID, "Very delicate", "delicate", tag.Color)
	c.Assert(err, qt.IsNil)
	got, err = f.svc.Get(aliceCtx, actor("alice", models.GroupRoleUser), aliceView.View.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.View.Query, qt.Equals, "tags=delicate%2Cred")
}

func TestSavedViewService_Reorder(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	ctx := savedViewCtx("alice")
	alice := actor("alice", models.GroupRoleViewer)
	first, err := f.svc.Create(ctx, alice, viewInput("First", "", false))
	c.Assert(err, qt.IsNil)
	second, err := f.svc.Create(ctx, alice, viewInput("Second", "", false))
	c.Assert(err, qt.IsNil)

	c.Assert(f.svc.Reorder(ctx, alice, []string{second.View.ID, first.View.ID}), qt.IsNil)
	entries, err := f.svc.List(ctx, alice)
	c.Assert(err, qt.IsNil)
	c.Assert(entries, qt.HasLen, 2)
	c.Assert(entries[0].View.Name, qt.Equals, "Second")
	c.Assert(entries[1].View.Name, qt.Equals, "First")
}

// Concurrent renames of one tag from the same starting slug: the second
// renames from the slug the first left, read under the rename's lock, so the
// views follow the tag to its final slug.
func TestTagService_RenameTag_ConcurrentRenamesKeepViewsOnTheTag(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	ctx := savedViewCtx("alice")
	tags := services.NewTagService(f.factory)
	created, err := tags.EnsureTagsExist(ctx, models.TagKindCommodity, []string{"red"})
	c.Assert(err, qt.IsNil)
	tag := created["red"]
	view, err := f.svc.Create(ctx, actor("alice", models.GroupRoleUser), viewInput("Red", "tags=red", false))
	c.Assert(err, qt.IsNil)

	var wg sync.WaitGroup
	for _, slug := range []string{"crimson", "scarlet"} {
		wg.Go(func() {
			_, err := tags.RenameTag(ctx, tag.ID, slug, slug, tag.Color)
			c.Check(err, qt.IsNil)
		})
	}
	wg.Wait()

	final, err := f.factory.TagRegistryFactory.MustCreateUserRegistry(ctx).Get(ctx, tag.ID)
	c.Assert(err, qt.IsNil)
	got, err := f.svc.Get(ctx, actor("alice", models.GroupRoleUser), view.View.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.View.Query, qt.Equals, "tags="+final.Slug)
}

// Where a view shows is each member's own: sharing a view places it for
// nobody else, any member who sees a shared view may pin and unpin it, and an
// admin unpinning it changes only the admin's sidebar.
func TestSavedViewService_PlacementIsPerMember(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	aliceCtx, bobCtx, carolCtx := savedViewCtx("alice"), savedViewCtx("bob"), savedViewCtx("carol")
	alice := actor("alice", models.GroupRoleUser)
	bob := actor("bob", models.GroupRoleViewer)
	carol := actor("carol", models.GroupRoleAdmin)

	shared, err := f.svc.Create(aliceCtx, alice, services.SavedViewInput{Name: "Team", Shared: true, ShowInSidebar: true, ShowOnDashboard: true})
	c.Assert(err, qt.IsNil)
	c.Assert(shared.ShowInSidebar, qt.IsTrue)
	c.Assert(shared.ShowOnDashboard, qt.IsTrue)

	seen, err := f.svc.Get(bobCtx, bob, shared.View.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(seen.ShowInSidebar, qt.IsFalse)
	c.Assert(seen.ShowOnDashboard, qt.IsFalse)

	// A viewer who may not edit the view still places it for themselves.
	pinned, err := f.svc.Update(bobCtx, bob, shared.View.ID, services.SavedViewPatch{ShowInSidebar: new(true)})
	c.Assert(err, qt.IsNil)
	c.Assert(pinned.ShowInSidebar, qt.IsTrue)
	c.Assert(pinned.CanEdit, qt.IsFalse)
	// A change to the view itself in the same request is still refused, and
	// the placement in it is not applied.
	_, err = f.svc.Update(bobCtx, bob, shared.View.ID, services.SavedViewPatch{Name: new("Mine"), ShowOnDashboard: new(true)})
	c.Assert(err, qt.ErrorIs, services.ErrSavedViewForbidden)
	seen, err = f.svc.Get(bobCtx, bob, shared.View.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(seen.ShowOnDashboard, qt.IsFalse)

	// The admin pins and unpins it for the admin alone.
	_, err = f.svc.Update(carolCtx, carol, shared.View.ID, services.SavedViewPatch{ShowInSidebar: new(true)})
	c.Assert(err, qt.IsNil)
	_, err = f.svc.Update(carolCtx, carol, shared.View.ID, services.SavedViewPatch{ShowInSidebar: new(false)})
	c.Assert(err, qt.IsNil)
	for _, member := range []struct {
		ctx   context.Context
		actor services.SavedViewActor
		want  bool
	}{{aliceCtx, alice, true}, {bobCtx, bob, true}, {carolCtx, carol, false}} {
		got, err := f.svc.Get(member.ctx, member.actor, shared.View.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(got.ShowInSidebar, qt.Equals, member.want, qt.Commentf("member %s", member.actor.UserID))
	}

	// Another member's personal view cannot be placed: it does not exist for
	// them.
	personal, err := f.svc.Create(aliceCtx, alice, viewInput("Mine", "", false))
	c.Assert(err, qt.IsNil)
	_, err = f.svc.Update(bobCtx, bob, personal.View.ID, services.SavedViewPatch{ShowInSidebar: new(true)})
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// Five members race for the last shared slot: the limit is checked under the
// registry's write lock, so exactly one of them shares a view.
func TestSavedViewService_SharedLimitHoldsUnderConcurrency(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	aliceCtx := savedViewCtx("alice")
	alice := actor("alice", models.GroupRoleUser)
	for i := range models.MaxSharedSavedViews - 1 {
		_, err := f.svc.Create(aliceCtx, alice, viewInput(fmt.Sprintf("View %d", i), "", true))
		c.Assert(err, qt.IsNil)
	}
	var shared atomic.Int32
	var wg sync.WaitGroup
	for i := range 5 {
		wg.Go(func() {
			userID := fmt.Sprintf("member-%d", i)
			_, err := f.svc.Create(savedViewCtx(userID), actor(userID, models.GroupRoleUser), viewInput("Last slot", "", true))
			if err == nil {
				shared.Add(1)
				return
			}
			c.Check(err, qt.ErrorIs, services.ErrSavedViewLimitReached)
		})
	}
	wg.Wait()
	c.Assert(shared.Load(), qt.Equals, int32(1))
}

// An admin who unshares another member's view no longer sees it. The change
// is stored and the PATCH succeeds, placement fields and all, with the view
// as written; it does not fail on the placement it can no longer set.
func TestSavedViewService_AdminUnsharesAnotherMembersView(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	aliceCtx, carolCtx := savedViewCtx("alice"), savedViewCtx("carol")
	alice := actor("alice", models.GroupRoleUser)
	carol := actor("carol", models.GroupRoleAdmin)
	shared, err := f.svc.Create(aliceCtx, alice, services.SavedViewInput{Name: "Team", Shared: true, ShowInSidebar: true})
	c.Assert(err, qt.IsNil)
	_, err = f.svc.Update(carolCtx, carol, shared.View.ID, services.SavedViewPatch{ShowInSidebar: new(true)})
	c.Assert(err, qt.IsNil)

	// The manage page sends the placement fields with every edit.
	got, err := f.svc.Update(carolCtx, carol, shared.View.ID, services.SavedViewPatch{
		Name:            new("Team"),
		Shared:          new(false),
		ShowInSidebar:   new(true),
		ShowOnDashboard: new(false),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(got.View.Shared, qt.IsFalse)
	c.Assert(got.CanEdit, qt.IsFalse)
	c.Assert(got.CanDelete, qt.IsFalse)
	c.Assert(got.ShowInSidebar, qt.IsFalse)

	_, err = f.svc.Get(carolCtx, carol, shared.View.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	mine, err := f.svc.Get(aliceCtx, alice, shared.View.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(mine.View.Shared, qt.IsFalse)
	c.Assert(mine.ShowInSidebar, qt.IsTrue, qt.Commentf("the creator keeps their placement"))
}
