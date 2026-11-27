package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
)

// -- Helpers ---------------------------------------------------------------

// newTestGroup returns a minimal valid LocationGroup for the caller's tenant
// and user. It is NOT persisted — call Create(ctx, ...) yourself.
func newTestGroup(c *qt.C, tenantID, userID, name string) models.LocationGroup {
	c.Helper()
	slug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	return models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Name:                name,
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           userID,
	}
}

// -- Create ----------------------------------------------------------------

func TestLocationGroupRegistry_Create_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "Shiny New Group"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.Name, qt.Equals, "Shiny New Group")
	c.Assert(created.Status, qt.Equals, models.LocationGroupStatusActive)
}

func TestLocationGroupRegistry_Create_MissingFields(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	valid := newTestGroup(c, user.TenantID, user.ID, "x")
	cases := []struct {
		name string
		mut  func(*models.LocationGroup)
	}{
		{"name empty", func(g *models.LocationGroup) { g.Name = "" }},
		{"slug empty", func(g *models.LocationGroup) { g.Slug = "" }},
		{"tenant empty", func(g *models.LocationGroup) { g.TenantID = "" }},
		{"created_by empty", func(g *models.LocationGroup) { g.CreatedBy = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			g := valid
			tc.mut(&g)
			_, err := registrySet.LocationGroupRegistry.Create(ctx, g)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
		})
	}
}

func TestLocationGroupRegistry_Create_DuplicateSlug(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	first := newTestGroup(c, user.TenantID, user.ID, "First")
	_, err := registrySet.LocationGroupRegistry.Create(ctx, first)
	c.Assert(err, qt.IsNil)

	// Same slug in the same tenant — rejected by the (tenant_id, slug) unique
	// index (the pre-insert check surfaces it as ErrSlugAlreadyExists).
	second := first
	second.Name = "Second"
	_, err = registrySet.LocationGroupRegistry.Create(ctx, second)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorIs, registry.ErrSlugAlreadyExists)
}

// -- Get / GetBySlug -------------------------------------------------------

func TestLocationGroupRegistry_Get_And_GetBySlug(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "Lookup Me"))
	c.Assert(err, qt.IsNil)

	byID, err := registrySet.LocationGroupRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(byID.Name, qt.Equals, "Lookup Me")

	bySlug, err := registrySet.LocationGroupRegistry.GetBySlug(ctx, user.TenantID, created.Slug)
	c.Assert(err, qt.IsNil)
	c.Assert(bySlug.ID, qt.Equals, created.ID)

	_, err = registrySet.LocationGroupRegistry.Get(ctx, "no-such-id")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	_, err = registrySet.LocationGroupRegistry.GetBySlug(ctx, user.TenantID, "no-such-slug")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// -- ListByTenant ----------------------------------------------------------

func TestLocationGroupRegistry_ListByTenant(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	// The test fixture creates one group; add two more.
	for _, name := range []string{"Alpha", "Beta"} {
		_, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, name))
		c.Assert(err, qt.IsNil)
	}

	groups, err := registrySet.LocationGroupRegistry.ListByTenant(ctx, user.TenantID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(groups) >= 3, qt.IsTrue)
	for _, g := range groups {
		c.Assert(g.TenantID, qt.Equals, user.TenantID)
	}
}

// -- Update / Delete -------------------------------------------------------

func TestLocationGroupRegistry_Update_And_Delete(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "Renamable"))
	c.Assert(err, qt.IsNil)

	created.Name = "Renamed"
	created.Icon = "📦"
	updated, err := registrySet.LocationGroupRegistry.Update(ctx, *created)
	c.Assert(err, qt.IsNil)
	c.Assert(updated.Name, qt.Equals, "Renamed")
	c.Assert(updated.Icon, qt.Equals, "📦")

	err = registrySet.LocationGroupRegistry.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	_, err = registrySet.LocationGroupRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// -- Hidden sections -------------------------------------------------------

func TestLocationGroupRegistry_HiddenSections_RoundTrip(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	// A group written without the setting hides nothing, and reads back an
	// empty list rather than null.
	created, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "Plain"))
	c.Assert(err, qt.IsNil)
	got, err := registrySet.LocationGroupRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.HiddenSections, qt.DeepEquals, models.GroupSections{})

	got.HiddenSections = models.GroupSections{models.GroupSectionWarranties, models.GroupSectionLoans}
	_, err = registrySet.LocationGroupRegistry.Update(ctx, *got)
	c.Assert(err, qt.IsNil)

	reread, err := registrySet.LocationGroupRegistry.GetBySlug(ctx, user.TenantID, created.Slug)
	c.Assert(err, qt.IsNil)
	c.Assert(reread.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties, models.GroupSectionLoans})

	listed, err := registrySet.LocationGroupRegistry.ListByTenant(ctx, user.TenantID)
	c.Assert(err, qt.IsNil)
	var found bool
	for _, g := range listed {
		if g.ID == created.ID {
			found = true
			c.Assert(g.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties, models.GroupSectionLoans})
		}
	}
	c.Assert(found, qt.IsTrue)

	// A group created with the setting keeps it.
	withSections := newTestGroup(c, user.TenantID, user.ID, "Stamps")
	withSections.HiddenSections = models.GroupSections{models.GroupSectionWarranties}
	stamps, err := registrySet.LocationGroupRegistry.Create(ctx, withSections)
	c.Assert(err, qt.IsNil)
	got, err = registrySet.LocationGroupRegistry.Get(ctx, stamps.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties})
}

func TestLocationGroupRegistry_SetHiddenSections(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)
	groups := registrySet.LocationGroupRegistry

	created, err := groups.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "Books"))
	c.Assert(err, qt.IsNil)

	// A rename that commits after the caller read the group is kept: the
	// write touches the hidden sections and updated_at only.
	stale, err := groups.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	renamed := *stale
	renamed.Name = "Book collection"
	_, err = groups.Update(ctx, renamed)
	c.Assert(err, qt.IsNil)

	at := time.Now().Add(time.Minute).UTC().Truncate(time.Microsecond)
	got, applied, err := groups.SetHiddenSections(ctx, created.ID, models.GroupSections{models.GroupSectionLoans, models.GroupSectionWarranties}, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	c.Assert(got.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties, models.GroupSectionLoans})
	c.Assert(got.Name, qt.Equals, "Book collection")

	stored, err := groups.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Name, qt.Equals, "Book collection")
	c.Assert(stored.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties, models.GroupSectionLoans})
	c.Assert(stored.UpdatedAt.Equal(at), qt.IsTrue, qt.Commentf("updated_at %s, want %s", stored.UpdatedAt, at))

	// A group whose deletion started is left as it is.
	pending := *stored
	pending.Status = models.LocationGroupStatusPendingDeletion
	_, err = groups.Update(ctx, pending)
	c.Assert(err, qt.IsNil)
	got, applied, err = groups.SetHiddenSections(ctx, created.ID, models.GroupSections{}, time.Now())
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsFalse)
	c.Assert(got.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)
	stored, err = groups.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)
	c.Assert(stored.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties, models.GroupSectionLoans})

	_, _, err = groups.SetHiddenSections(ctx, "no-such-group", models.GroupSections{}, time.Now())
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// newWriteTestGroup stores an active group, then a change made from a copy
// read before it: the currency moves to EUR. A write that started from the
// first read would undo that change.
func newWriteTestGroup(c *qt.C, set *registry.Set) *models.LocationGroup {
	c.Helper()
	ctx := context.Background()
	user := getTestUser(c, set)
	seed := newTestGroup(c, user.TenantID, user.ID, "Books")
	seed.Icon = "book"
	seed.GroupCurrency = "USD"
	seed.HiddenSections = models.GroupSections{models.GroupSectionWarranties}
	created, err := set.LocationGroupRegistry.Create(ctx, seed)
	c.Assert(err, qt.IsNil)
	migrated := *created
	migrated.GroupCurrency = "EUR"
	stored, err := set.LocationGroupRegistry.Update(ctx, migrated)
	c.Assert(err, qt.IsNil)
	return stored
}

func TestLocationGroupRegistry_UpdateDetails(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	groups := registrySet.LocationGroupRegistry
	group := newWriteTestGroup(c, registrySet)

	at := time.Now().Add(time.Minute).UTC().Truncate(time.Microsecond)
	got, applied, err := groups.UpdateDetails(ctx, group.ID, registry.GroupDetails{Name: "Book collection", Icon: "library", Description: "Shelves"}, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	stored, err := groups.Get(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	for _, g := range []*models.LocationGroup{got, stored} {
		c.Assert(g.Name, qt.Equals, "Book collection")
		c.Assert(g.Icon, qt.Equals, "library")
		c.Assert(g.Description, qt.Equals, "Shelves")
		c.Assert(g.UpdatedAt.Equal(at), qt.IsTrue, qt.Commentf("updated_at %s, want %s", g.UpdatedAt, at))
		c.Assert(g.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties}, qt.Commentf("not given, so kept"))
		c.Assert(g.GroupCurrency, qt.Equals, models.Currency("EUR"))
	}

	got, applied, err = groups.UpdateDetails(ctx, group.ID, registry.GroupDetails{Name: "Book collection", HiddenSections: &models.GroupSections{}}, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	c.Assert(got.HiddenSections, qt.DeepEquals, models.GroupSections{})
	c.Assert(got.Icon, qt.Equals, "")

	_, _, err = groups.UpdateDetails(ctx, group.ID, registry.GroupDetails{}, at)
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
	_, _, err = groups.UpdateDetails(ctx, "no-such-group", registry.GroupDetails{Name: "Books"}, at)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// A group whose deletion started is left as it is.
	_, applied, err = groups.MarkPendingDeletion(ctx, group.ID, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	got, applied, err = groups.UpdateDetails(ctx, group.ID, registry.GroupDetails{Name: "Renamed"}, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsFalse)
	c.Assert(got.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)
	stored, err = groups.Get(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Name, qt.Equals, "Book collection")
}

func TestLocationGroupRegistry_MarkPendingDeletion(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	groups := registrySet.LocationGroupRegistry
	group := newWriteTestGroup(c, registrySet)

	at := time.Now().Add(time.Minute).UTC().Truncate(time.Microsecond)
	got, applied, err := groups.MarkPendingDeletion(ctx, group.ID, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	c.Assert(got.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)
	stored, err := groups.Get(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)
	c.Assert(stored.UpdatedAt.Equal(at), qt.IsTrue, qt.Commentf("updated_at %s, want %s", stored.UpdatedAt, at))
	c.Assert(stored.GroupCurrency, qt.Equals, models.Currency("EUR"))

	got, applied, err = groups.MarkPendingDeletion(ctx, group.ID, time.Now())
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsFalse)
	c.Assert(got.UpdatedAt.Equal(at), qt.IsTrue, qt.Commentf("an already pending group is not written again"))

	_, _, err = groups.MarkPendingDeletion(ctx, "no-such-group", at)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestLocationGroupRegistry_SetCurrencyMigrationID(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	groups := registrySet.LocationGroupRegistry
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	worker := postgres.NewCurrencyMigrationRegistry(sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")).CreateServiceRegistry()
	startMigration := func() *models.CurrencyMigration {
		m, err := registrySet.CurrencyMigrationRegistry.Create(ctx, models.CurrencyMigration{
			FromCurrency: "USD",
			ToCurrency:   "EUR",
			ExchangeRate: decimal.NewFromFloat(0.92),
		})
		c.Assert(err, qt.IsNil)
		return m
	}

	// The worker finished the migration before its id was recorded.
	finished := startMigration()
	groupID := finished.GroupID
	now := time.Now().UTC()
	c.Assert(worker.UpdateStatus(ctx, finished.ID, registry.CurrencyMigrationStatusPatch{Status: models.CurrencyMigrationStatusRunning, StartedAt: &now}), qt.IsNil)
	c.Assert(worker.UpdateStatus(ctx, finished.ID, registry.CurrencyMigrationStatusPatch{Status: models.CurrencyMigrationStatusCompleted, CompletedAt: &now}), qt.IsNil)
	applied, err := groups.SetCurrencyMigrationID(ctx, groupID, finished.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsFalse)
	stored, err := groups.Get(ctx, groupID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.CurrencyMigrationID, qt.IsNil)

	// A pending migration is recorded, and nothing else is written.
	pending := startMigration()
	before := stored
	applied, err = groups.SetCurrencyMigrationID(ctx, groupID, pending.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	stored, err = groups.Get(ctx, groupID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.CurrencyMigrationID, qt.DeepEquals, new(pending.ID))
	c.Assert(stored.Name, qt.Equals, before.Name)
	c.Assert(stored.UpdatedAt.Equal(before.UpdatedAt), qt.IsTrue)

	_, err = groups.SetCurrencyMigrationID(ctx, "no-such-group", pending.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// A group whose deletion started is left as it is.
	cleared := *stored
	cleared.CurrencyMigrationID = nil
	_, err = groups.Update(ctx, cleared)
	c.Assert(err, qt.IsNil)
	_, _, err = groups.MarkPendingDeletion(ctx, groupID, time.Now())
	c.Assert(err, qt.IsNil)
	applied, err = groups.SetCurrencyMigrationID(ctx, groupID, pending.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsFalse)
	stored, err = groups.Get(ctx, groupID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.CurrencyMigrationID, qt.IsNil)
}
