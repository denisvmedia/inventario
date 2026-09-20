package processor

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
)

// Restore has three strategies and they differ in the part a user cares
// about: whether a row already in the database is left alone, overwritten, or
// duplicated. #2114 N4 flagged the MERGE paths as untested, which is an
// uncomfortable place for a gap — the wrong branch here either loses an edit
// the user made since the backup or silently declines to restore it, and both
// look like success.
//
// These run in dry-run, which is what makes them cheap: every create and
// update path skips its registry work and only records the decision, so the
// dispatch is testable without a database. The dry-run report the UI shows is
// built from exactly these counters, so a test of the counters is also a test
// of what the user is told before they commit.

func newStrategyFixture() (*types.RestoreStats, *types.ExistingEntities, *types.IDMapping) {
	return &types.RestoreStats{},
		&types.ExistingEntities{
			Locations:   map[string]*models.Location{},
			Areas:       map[string]*models.Area{},
			Commodities: map[string]*models.Commodity{},
		},
		&types.IDMapping{
			Locations:   map[string]string{},
			Areas:       map[string]string{},
			Commodities: map[string]string{},
			Files:       map[string]string{},
		}
}

func dryRun(strategy types.RestoreStrategy) types.RestoreOptions {
	return types.RestoreOptions{Strategy: strategy, DryRun: true}
}

// TestLocationStrategy walks the whole decision table for locations. The row
// that matters most is merge_add against an existing location: the promise of
// "add what is missing" is that it touches nothing else, and a create there
// would duplicate the location under a new id.
func TestLocationStrategy(t *testing.T) {
	existingLoc := &models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: "db-loc-1"},
		},
		Name: "Garage (as stored)",
	}
	existingLoc.UUID = "uuid-loc-1"

	tests := []struct {
		name     string
		strategy types.RestoreStrategy
		existing *models.Location
		created  int
		updated  int
		skipped  int
	}{
		{"full replace, nothing there", types.RestoreStrategyFullReplace, nil, 1, 0, 0},
		{"full replace, already there", types.RestoreStrategyFullReplace, existingLoc, 1, 0, 0},
		{"merge add, nothing there", types.RestoreStrategyMergeAdd, nil, 1, 0, 0},
		{"merge add, already there", types.RestoreStrategyMergeAdd, existingLoc, 0, 0, 1},
		{"merge update, nothing there", types.RestoreStrategyMergeUpdate, nil, 1, 0, 0},
		{"merge update, already there", types.RestoreStrategyMergeUpdate, existingLoc, 0, 1, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			stats, existing, idMapping := newStrategyFixture()
			p := &RestoreOperationProcessor{}

			incoming := &models.Location{Name: "Garage (from backup)"}
			err := p.applyStrategyForLocationModel(context.Background(), incoming, tc.existing,
				"uuid-loc-1", "Garage", stats, existing, idMapping, dryRun(tc.strategy))

			c.Assert(err, qt.IsNil)
			c.Assert(stats.CreatedCount, qt.Equals, tc.created)
			c.Assert(stats.UpdatedCount, qt.Equals, tc.updated)
			c.Assert(stats.SkippedCount, qt.Equals, tc.skipped)
			// Whatever it did, it counted exactly one location — a restore
			// that reports the wrong total is how a user finds out too late.
			c.Assert(stats.LocationCount, qt.Equals, tc.created+tc.updated)
		})
	}
}

// merge_update adopts the existing row's identity rather than writing the
// backup's. Getting this wrong is not a cosmetic id difference: the update
// would target a row that does not exist, or create a second one, and every
// area and commodity already pointing at the live location would be orphaned.
func TestLocationMergeUpdateAdoptsTheExistingIdentity(t *testing.T) {
	c := qt.New(t)
	stats, existing, idMapping := newStrategyFixture()
	p := &RestoreOperationProcessor{}

	existingLoc := &models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: "db-loc-1"},
		},
		Name: "Garage (as stored)",
	}
	existingLoc.UUID = "uuid-loc-1"

	incoming := &models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: "id-from-the-backup"},
		},
		Name: "Garage (from backup)",
	}
	incoming.UUID = "uuid-from-the-backup"

	err := p.applyStrategyForLocationModel(context.Background(), incoming, existingLoc,
		"uuid-loc-1", "Garage", stats, existing, idMapping, dryRun(types.RestoreStrategyMergeUpdate))
	c.Assert(err, qt.IsNil)

	c.Assert(incoming.ID, qt.Equals, "db-loc-1",
		qt.Commentf("the update must target the row that exists, not the backup's id"))
	c.Assert(incoming.UUID, qt.Equals, "uuid-loc-1")
}

// The same table for areas.
func TestAreaStrategy(t *testing.T) {
	existingArea := &models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: "db-area-1"},
		},
		Name: "Shelf (as stored)",
	}
	existingArea.UUID = "uuid-area-1"

	tests := []struct {
		name     string
		strategy types.RestoreStrategy
		existing *models.Area
		created  int
		updated  int
		skipped  int
	}{
		{"full replace, already there", types.RestoreStrategyFullReplace, existingArea, 1, 0, 0},
		{"merge add, already there", types.RestoreStrategyMergeAdd, existingArea, 0, 0, 1},
		{"merge add, nothing there", types.RestoreStrategyMergeAdd, nil, 1, 0, 0},
		{"merge update, already there", types.RestoreStrategyMergeUpdate, existingArea, 0, 1, 0},
		{"merge update, nothing there", types.RestoreStrategyMergeUpdate, nil, 1, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			stats, existing, idMapping := newStrategyFixture()
			p := &RestoreOperationProcessor{}

			incoming := &models.Area{Name: "Shelf (from backup)"}
			err := p.applyStrategyForAreaModel(context.Background(), incoming, tc.existing,
				"uuid-area-1", stats, existing, idMapping, dryRun(tc.strategy))

			c.Assert(err, qt.IsNil)
			c.Assert(stats.CreatedCount, qt.Equals, tc.created)
			c.Assert(stats.UpdatedCount, qt.Equals, tc.updated)
			c.Assert(stats.SkippedCount, qt.Equals, tc.skipped)
			c.Assert(stats.AreaCount, qt.Equals, tc.created+tc.updated)
		})
	}
}

func TestAreaMergeUpdateAdoptsTheExistingIdentity(t *testing.T) {
	c := qt.New(t)
	stats, existing, idMapping := newStrategyFixture()
	p := &RestoreOperationProcessor{}

	existingArea := &models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: "db-area-1"},
		},
	}
	existingArea.UUID = "uuid-area-1"

	incoming := &models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: "id-from-the-backup"},
		},
	}
	incoming.UUID = "uuid-from-the-backup"

	err := p.applyStrategyForAreaModel(context.Background(), incoming, existingArea,
		"uuid-area-1", stats, existing, idMapping, dryRun(types.RestoreStrategyMergeUpdate))
	c.Assert(err, qt.IsNil)
	c.Assert(incoming.ID, qt.Equals, "db-area-1")
	c.Assert(incoming.UUID, qt.Equals, "uuid-area-1")
}

// An unknown strategy must do nothing at all rather than fall through to a
// default. A restore that silently picks a behaviour for a value it does not
// recognise is worse than one that declines.
func TestUnknownStrategyDoesNothing(t *testing.T) {
	c := qt.New(t)
	stats, existing, idMapping := newStrategyFixture()
	p := &RestoreOperationProcessor{}

	err := p.applyStrategyForLocationModel(context.Background(), &models.Location{}, nil,
		"uuid-loc-1", "Garage", stats, existing, idMapping,
		types.RestoreOptions{Strategy: types.RestoreStrategy("something-else"), DryRun: true})

	c.Assert(err, qt.IsNil)
	c.Assert(stats.CreatedCount, qt.Equals, 0)
	c.Assert(stats.UpdatedCount, qt.Equals, 0)
	c.Assert(stats.SkippedCount, qt.Equals, 0)
}
