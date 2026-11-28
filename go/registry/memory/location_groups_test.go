package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

func TestLocationGroupRegistry_HiddenSections_RoundTrip(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	reg := memory.NewLocationGroupRegistry()

	created, err := reg.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Slug:                "abcdefghijklmnopqrstuv",
		Name:                "Stamps",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           "user-1",
		HiddenSections:      models.GroupSections{models.GroupSectionWarranties},
	})
	c.Assert(err, qt.IsNil)

	got, err := reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties})

	got.HiddenSections = models.GroupSections{models.GroupSectionLoans, models.GroupSectionUpcoming}
	_, err = reg.Update(ctx, *got)
	c.Assert(err, qt.IsNil)

	bySlug, err := reg.GetBySlug(ctx, "tenant-1", "abcdefghijklmnopqrstuv")
	c.Assert(err, qt.IsNil)
	c.Assert(bySlug.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionLoans, models.GroupSectionUpcoming})

	listed, err := reg.ListByTenant(ctx, "tenant-1")
	c.Assert(err, qt.IsNil)
	c.Assert(listed, qt.HasLen, 1)
	c.Assert(listed[0].HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionLoans, models.GroupSectionUpcoming})
}

func TestLocationGroupRegistry_SetHiddenSections(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	reg := memory.NewLocationGroupRegistry()

	created, err := reg.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Slug:                "abcdefghijklmnopqrstuv",
		Name:                "Books",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           "user-1",
	})
	c.Assert(err, qt.IsNil)

	// A rename stored after the caller read the group is kept.
	stale, err := reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	renamed := *stale
	renamed.Name = "Book collection"
	_, err = reg.Update(ctx, renamed)
	c.Assert(err, qt.IsNil)

	at := time.Now().Add(time.Minute)
	got, applied, err := reg.SetHiddenSections(ctx, created.ID, models.GroupSections{models.GroupSectionLoans, models.GroupSectionWarranties}, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	c.Assert(got.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties, models.GroupSectionLoans})
	stored, err := reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Name, qt.Equals, "Book collection")
	c.Assert(stored.UpdatedAt.Equal(at), qt.IsTrue)

	// A group whose deletion started is left as it is.
	pending := *stored
	pending.Status = models.LocationGroupStatusPendingDeletion
	_, err = reg.Update(ctx, pending)
	c.Assert(err, qt.IsNil)
	_, applied, err = reg.SetHiddenSections(ctx, created.ID, models.GroupSections{}, time.Now())
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsFalse)
	stored, err = reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties, models.GroupSectionLoans})

	_, _, err = reg.SetHiddenSections(ctx, "no-such-group", models.GroupSections{}, time.Now())
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// newWriteTestGroup stores an active group, then a change made from a copy
// read before it: the currency moves to EUR and a migration id is recorded.
// A write that started from the first read would undo that change.
func newWriteTestGroup(c *qt.C, reg *memory.LocationGroupRegistry) *models.LocationGroup {
	c.Helper()
	ctx := context.Background()
	created, err := reg.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Slug:                "abcdefghijklmnopqrstuv",
		Name:                "Books",
		Icon:                "book",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           "user-1",
		GroupCurrency:       "USD",
		HiddenSections:      models.GroupSections{models.GroupSectionWarranties},
	})
	c.Assert(err, qt.IsNil)
	migrated := *created
	migrated.GroupCurrency = "EUR"
	migrated.CurrencyMigrationID = new("migration-1")
	stored, err := reg.Update(ctx, migrated)
	c.Assert(err, qt.IsNil)
	return stored
}

func TestLocationGroupRegistry_UpdateDetails(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	reg := memory.NewLocationGroupRegistry()
	group := newWriteTestGroup(c, reg)

	at := time.Now().Add(time.Minute)
	got, applied, err := reg.UpdateDetails(ctx, group.ID, registry.GroupDetails{Name: "Book collection", Icon: "library", Description: "Shelves"}, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	stored, err := reg.Get(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	for _, g := range []*models.LocationGroup{got, stored} {
		c.Assert(g.Name, qt.Equals, "Book collection")
		c.Assert(g.Icon, qt.Equals, "library")
		c.Assert(g.Description, qt.Equals, "Shelves")
		c.Assert(g.UpdatedAt.Equal(at), qt.IsTrue)
		c.Assert(g.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties}, qt.Commentf("not given, so kept"))
		c.Assert(g.GroupCurrency, qt.Equals, models.Currency("EUR"))
		c.Assert(g.CurrencyMigrationID, qt.DeepEquals, new("migration-1"))
	}

	got, applied, err = reg.UpdateDetails(ctx, group.ID, registry.GroupDetails{Name: "Book collection", HiddenSections: &models.GroupSections{}}, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	c.Assert(got.HiddenSections, qt.DeepEquals, models.GroupSections{})
	c.Assert(got.Icon, qt.Equals, "")

	_, _, err = reg.UpdateDetails(ctx, group.ID, registry.GroupDetails{}, at)
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
	_, _, err = reg.UpdateDetails(ctx, "no-such-group", registry.GroupDetails{Name: "Books"}, at)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// A group whose deletion started is left as it is.
	_, applied, err = reg.MarkPendingDeletion(ctx, group.ID, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	got, applied, err = reg.UpdateDetails(ctx, group.ID, registry.GroupDetails{Name: "Renamed"}, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsFalse)
	c.Assert(got.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)
	stored, err = reg.Get(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Name, qt.Equals, "Book collection")
}

func TestLocationGroupRegistry_MarkPendingDeletion(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	reg := memory.NewLocationGroupRegistry()
	group := newWriteTestGroup(c, reg)

	at := time.Now().Add(time.Minute)
	got, applied, err := reg.MarkPendingDeletion(ctx, group.ID, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	c.Assert(got.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)
	stored, err := reg.Get(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)
	c.Assert(stored.UpdatedAt.Equal(at), qt.IsTrue)
	c.Assert(stored.GroupCurrency, qt.Equals, models.Currency("EUR"))

	got, applied, err = reg.MarkPendingDeletion(ctx, group.ID, time.Now())
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsFalse)
	c.Assert(got.UpdatedAt.Equal(at), qt.IsTrue, qt.Commentf("an already pending group is not written again"))

	_, _, err = reg.MarkPendingDeletion(ctx, "no-such-group", at)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestLocationGroupRegistry_SetCurrencyMigrationID(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	reg := memory.NewLocationGroupRegistry()
	group := newWriteTestGroup(c, reg)

	applied, err := reg.SetCurrencyMigrationID(ctx, group.ID, "migration-2")
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	stored, err := reg.Get(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.CurrencyMigrationID, qt.DeepEquals, new("migration-2"))
	c.Assert(stored.GroupCurrency, qt.Equals, models.Currency("EUR"))
	c.Assert(stored.UpdatedAt.Equal(group.UpdatedAt), qt.IsTrue, qt.Commentf("updated_at is not a column it writes"))

	_, err = reg.SetCurrencyMigrationID(ctx, "no-such-group", "migration-2")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	_, _, err = reg.MarkPendingDeletion(ctx, group.ID, time.Now())
	c.Assert(err, qt.IsNil)
	applied, err = reg.SetCurrencyMigrationID(ctx, group.ID, "migration-3")
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsFalse)
	stored, err = reg.Get(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.CurrencyMigrationID, qt.DeepEquals, new("migration-2"))
}
