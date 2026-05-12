package models_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestPlanByID(t *testing.T) {
	c := qt.New(t)

	c.Run("free returns Free", func(c *qt.C) {
		p := models.PlanByID("free")
		c.Assert(p.ID, qt.Equals, "free")
		c.Assert(p.Name, qt.Equals, "Free")
		c.Assert(p.MaxItems, qt.IsNotNil)
		c.Assert(*p.MaxItems, qt.Equals, 500)
		c.Assert(p.AllowsRestore, qt.IsFalse)
		c.Assert(p.AllowsAPIAccess, qt.IsFalse)
	})

	c.Run("pro returns Pro", func(c *qt.C) {
		p := models.PlanByID("pro")
		c.Assert(p.ID, qt.Equals, "pro")
		c.Assert(p.Name, qt.Equals, "Pro")
		// Pro is uncapped on item / location counts (mock parity).
		c.Assert(p.MaxItems, qt.IsNil)
		c.Assert(p.MaxLocations, qt.IsNil)
		c.Assert(p.MaxStorageBytes, qt.IsNotNil)
		c.Assert(p.AllowsRestore, qt.IsTrue)
		c.Assert(p.AllowsAPIAccess, qt.IsTrue)
	})

	c.Run("unlimited returns Unlimited", func(c *qt.C) {
		p := models.PlanByID("unlimited")
		c.Assert(p.ID, qt.Equals, "unlimited")
		c.Assert(p.Name, qt.Equals, "Unlimited")
		c.Assert(p.MaxItems, qt.IsNil)
		c.Assert(p.MaxLocations, qt.IsNil)
		c.Assert(p.MaxStorageBytes, qt.IsNil)
	})

	c.Run("unknown id degrades to Unlimited", func(c *qt.C) {
		// The fallback exists so every plan-reading request path stays
		// renderable when tenants.plan_id has a value not in the
		// in-code catalogue (operator hand-edit, partial migration).
		// `unlimited` is the safer default — the alternative would be
		// to silently apply `free` caps to a paying tenant.
		p := models.PlanByID("nonexistent-plan-id")
		c.Assert(p.ID, qt.Equals, "unlimited")
	})

	c.Run("empty id degrades to Unlimited", func(c *qt.C) {
		// Memory-mode tenants created before this migration ran have an
		// empty PlanID. The fallback keeps them on `unlimited` rather
		// than rejecting the request.
		p := models.PlanByID("")
		c.Assert(p.ID, qt.Equals, "unlimited")
	})
}

func TestPlansCatalogOrder(t *testing.T) {
	c := qt.New(t)

	plans := models.Plans()
	c.Assert(plans, qt.HasLen, 3)
	// Order is load-bearing for the eventual FE catalogue page: free
	// first, then the natural upgrade path.
	c.Assert(plans[0].ID, qt.Equals, "free")
	c.Assert(plans[1].ID, qt.Equals, "pro")
	c.Assert(plans[2].ID, qt.Equals, "unlimited")
}
