package postgres_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// -- Helpers ---------------------------------------------------------------

// newTestGroup returns a minimal valid LocationGroup for the caller's tenant
// and user. It is NOT persisted — call Create(ctx, ...) yourself.
func newTestGroup(c *qt.C, tenantID, userID, name string) models.LocationGroup {
	c.Helper()
	slug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	return models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			UserID:   userID,
		},
		Name:      name,
		Slug:      slug,
		Status:    models.LocationGroupStatusActive,
		CreatedBy: userID,
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
		{"user_id empty", func(g *models.LocationGroup) { g.UserID = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			g := valid
			tc.mut(&g)
			_, err := registrySet.LocationGroupRegistry.Create(ctx, g)
			c.Assert(err, qt.IsNotNil)
			c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
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
	c.Assert(errors.Is(err, registry.ErrSlugAlreadyExists), qt.IsTrue)
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
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)

	_, err = registrySet.LocationGroupRegistry.GetBySlug(ctx, user.TenantID, "no-such-slug")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
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
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}
