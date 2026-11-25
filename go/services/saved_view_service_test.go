package services_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

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
	c.Assert(updated.View.ShowOnDashboard, qt.IsTrue)
	c.Assert(updated.View.ShowInSidebar, qt.IsFalse)
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
	broken, err := reg.Create(ctx, models.SavedView{
		Name: "Broken", Icon: models.SavedViewIconBookmark, Entity: models.SavedViewEntityCommodities,
		Query: "warranty_expires_before=2026-01-01", QueryVersion: models.SavedViewQueryVersion, ShowInSidebar: true,
	})
	c.Assert(err, qt.IsNil)
	future, err := reg.Create(ctx, models.SavedView{
		Name: "Future", Icon: models.SavedViewIconBookmark, Entity: models.SavedViewEntityCommodities,
		Query: "status=in_use", QueryVersion: models.SavedViewQueryVersion + 1, ShowInSidebar: true,
	})
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

func seedSavedViewCommodity(c *qt.C, f *savedViewFixture, ctx context.Context, name string, status models.CommodityStatus, tags ...string) {
	c.Helper()
	reg := f.factory.CommodityRegistryFactory.MustCreateUserRegistry(ctx)
	_, err := reg.Create(ctx, models.Commodity{
		Name:      name,
		ShortName: name,
		Type:      models.CommodityTypeOther,
		Status:    status,
		Count:     1,
		Tags:      tags,
	})
	c.Assert(err, qt.IsNil)
}

// The sidebar count runs the stored query through the list path, so it is the
// total the items list reports for the same query string.
func TestSavedViewService_CountsMatchTheList(t *testing.T) {
	c := qt.New(t)
	f := newSavedViewFixture()
	ctx := savedViewCtx("alice")
	alice := actor("alice", models.GroupRoleUser)
	seedSavedViewCommodity(c, f, ctx, "Vase", models.CommodityStatusInUse, "fragile")
	seedSavedViewCommodity(c, f, ctx, "Glass", models.CommodityStatusSold, "fragile")
	seedSavedViewCommodity(c, f, ctx, "Hammer", models.CommodityStatusInUse)

	fragile, err := f.svc.Create(ctx, alice, viewInput("Fragile", "tags=fragile", false))
	c.Assert(err, qt.IsNil)
	activeFragile, err := f.svc.Create(ctx, alice, viewInput("Active fragile", "tags=fragile&include_inactive=false", false))
	c.Assert(err, qt.IsNil)
	hidden, err := f.svc.Create(ctx, alice, services.SavedViewInput{Name: "Not in sidebar", Query: ""})
	c.Assert(err, qt.IsNil)

	set, err := f.factory.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	counts, err := f.svc.Counts(ctx, set)
	c.Assert(err, qt.IsNil)
	c.Assert(counts, qt.DeepEquals, map[string]int{fragile.View.ID: 2, activeFragile.View.ID: 1})
	_, counted := counts[hidden.View.ID]
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
