package processor

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
)

// restore_strategy_test.go covers the location and area dispatch in dry-run —
// the decision table and the identity-adoption step. What it cannot reach is
// the write: whether the update lands on the row the dispatch chose, and
// whether the caller's map of what exists is refreshed so later references
// resolve to it. #2572.
//
// updateCommodity got this in #2571; these are the other two. The fixture is
// the same one, which already seeds a location and an area.

func TestUpdateLocationWritesAndRefreshesTheExistingMap(t *testing.T) {
	c := qt.New(t)

	p, ctx, area := restoreFixture(c)
	registrySet := must.Must(p.factorySet.CreateUserRegistrySet(ctx))
	stored := must.Must(registrySet.LocationRegistry.Get(ctx, area.LocationID))

	stats, existing, _ := newStrategyFixture()
	incoming := *stored
	incoming.Name = "Workshop (from backup)"
	incoming.Address = "2 Bench Rd"

	err := p.updateLocation(ctx, &incoming, nil, "uuid-loc-1", "Workshop", stats, existing,
		types.RestoreOptions{Strategy: types.RestoreStrategyMergeUpdate})
	c.Assert(err, qt.IsNil)

	c.Check(stats.UpdatedCount, qt.Equals, 1)
	c.Check(stats.LocationCount, qt.Equals, 1)

	reread := must.Must(registrySet.LocationRegistry.Get(ctx, stored.ID))
	c.Check(reread.Name, qt.Equals, "Workshop (from backup)")
	c.Check(reread.Address, qt.Equals, "2 Bench Rd")

	// Later rows in the same restore resolve their parent through this map,
	// so a stale entry here points every area at the row the backup
	// described rather than the one that was written.
	c.Assert(existing.Locations["uuid-loc-1"], qt.IsNotNil)
	c.Check(existing.Locations["uuid-loc-1"].ID, qt.Equals, stored.ID)
	c.Check(existing.Locations["uuid-loc-1"].Name, qt.Equals, "Workshop (from backup)")
}

func TestUpdateAreaWritesAndRefreshesTheExistingMap(t *testing.T) {
	c := qt.New(t)

	p, ctx, area := restoreFixture(c)
	registrySet := must.Must(p.factorySet.CreateUserRegistrySet(ctx))

	stats, existing, _ := newStrategyFixture()
	incoming := *area
	incoming.Name = "Shelf (from backup)"

	err := p.updateArea(ctx, &incoming, "uuid-area-1", stats, existing,
		types.RestoreOptions{Strategy: types.RestoreStrategyMergeUpdate})
	c.Assert(err, qt.IsNil)

	c.Check(stats.UpdatedCount, qt.Equals, 1)
	c.Check(stats.AreaCount, qt.Equals, 1)

	reread := must.Must(registrySet.AreaRegistry.Get(ctx, area.ID))
	c.Check(reread.Name, qt.Equals, "Shelf (from backup)")

	c.Assert(existing.Areas["uuid-area-1"], qt.IsNotNil)
	c.Check(existing.Areas["uuid-area-1"].ID, qt.Equals, area.ID)
	c.Check(existing.Areas["uuid-area-1"].Name, qt.Equals, "Shelf (from backup)")
}

// A dry run reports what it would do and touches nothing. A nil factorySet is
// the assertion: reaching the registry would panic rather than quietly write.
func TestUpdateLocationAndAreaDryRunWriteNothing(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	p := &RestoreOperationProcessor{}

	locStats, locExisting, _ := newStrategyFixture()
	err := p.updateLocation(ctx, &models.Location{Name: "Workshop"}, nil, "uuid-loc-1", "Workshop",
		locStats, locExisting, types.RestoreOptions{Strategy: types.RestoreStrategyMergeUpdate, DryRun: true})
	c.Assert(err, qt.IsNil)
	c.Check(locStats.UpdatedCount, qt.Equals, 1)
	c.Check(locStats.LocationCount, qt.Equals, 1)
	c.Check(locExisting.Locations, qt.HasLen, 0)

	areaStats, areaExisting, _ := newStrategyFixture()
	err = p.updateArea(ctx, &models.Area{Name: "Shelf"}, "uuid-area-1",
		areaStats, areaExisting, types.RestoreOptions{Strategy: types.RestoreStrategyMergeUpdate, DryRun: true})
	c.Assert(err, qt.IsNil)
	c.Check(areaStats.UpdatedCount, qt.Equals, 1)
	c.Check(areaStats.AreaCount, qt.Equals, 1)
	c.Check(areaExisting.Areas, qt.HasLen, 0)
}

// A location that fails to write records the failure against its own restore
// step, so the operator sees which location stopped rather than a bare error
// at the end of the run. The area path has no equivalent — noted rather than
// asserted, because that asymmetry is deliberate in the code.
func TestUpdateLocationRecordsAFailedStep(t *testing.T) {
	c := qt.New(t)

	p, ctx, _ := restoreFixture(c)
	p.restoreOperationID = "restore-op-1"

	stats, existing, _ := newStrategyFixture()

	// A location with no id cannot be updated: the registry has nothing to
	// match, which is the shape a corrupt backup takes.
	err := p.updateLocation(ctx, &models.Location{Name: "Ghost"}, nil, "uuid-loc-missing", "Ghost",
		stats, existing, types.RestoreOptions{Strategy: types.RestoreStrategyMergeUpdate})
	c.Assert(err, qt.IsNotNil)

	steps := must.Must(p.factorySet.RestoreStepRegistryFactory.CreateServiceRegistry().
		ListByRestoreOperation(ctx, "restore-op-1"))
	c.Assert(steps, qt.HasLen, 1)
	c.Check(steps[0].Name, qt.Equals, "Location: Ghost")
	c.Check(steps[0].Result, qt.Equals, models.RestoreStepResultError)
	c.Check(steps[0].Reason, qt.Not(qt.Equals), "")

	// A failed update is not counted as one.
	c.Check(stats.UpdatedCount, qt.Equals, 0)
	c.Check(existing.Locations, qt.HasLen, 0)
}
