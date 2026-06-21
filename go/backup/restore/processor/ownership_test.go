package processor

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore/security"
	"github.com/denisvmedia/inventario/backup/restore/types"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// ownershipFixture bundles everything the validateCommodityOwnershipInDB tests
// need: the factory set the processor reads through, the owner / attacker users,
// the owner's group context, and the UUID of a commodity owned by the owner.
type ownershipFixture struct {
	factorySet *registry.FactorySet
	owner      *models.User
	attacker   *models.User
	ownerCtx   context.Context
	// commodityUUID is the immutable UUID of the owner's commodity. The restore
	// path keys ownership lookups on this UUID (originalXMLID), so it is set
	// explicitly before Create and preserved by the memory registry.
	commodityUUID string
}

// newOwnershipFixture wires up two users (owner + attacker) in the same tenant,
// a location group owned by the owner, and one location / area / commodity
// created through the owner's user-aware registry set so CreatedByUserID is
// stamped to the owner. The commodity carries a known immutable UUID so the
// in-DB ownership lookup can find it by that UUID.
func newOwnershipFixture(t *testing.T) *ownershipFixture {
	t.Helper()

	ctx := context.Background()
	factorySet := memory.NewFactorySet()

	owner := must.Must(factorySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
		},
		Email: "owner@example.com",
		Name:  "Owner User",
	}))

	attacker := must.Must(factorySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
		},
		Email: "attacker@example.com",
		Name:  "Attacker User",
	}))

	slug := must.Must(models.GenerateGroupSlug())
	group := must.Must(factorySet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		Name:                "Test Group",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           owner.ID,
		GroupCurrency:       models.Currency("USD"),
	}))

	ownerCtx := appctx.WithGroup(appctx.WithUser(ctx, owner), group)
	ownerRegistrySet := must.Must(factorySet.CreateUserRegistrySet(ownerCtx))

	location := must.Must(ownerRegistrySet.LocationRegistry.Create(ownerCtx, models.Location{
		Name:    "Owner Location",
		Address: "123 Owner St",
	}))

	area := must.Must(ownerRegistrySet.AreaRegistry.Create(ownerCtx, models.Area{
		Name:       "Owner Area",
		LocationID: location.ID,
	}))

	// Set the immutable UUID explicitly. The memory registry overwrites the ID
	// with a fresh server-side value but preserves a caller-provided UUID, which
	// is exactly what the restore path relies on (commodity.UUID = originalID).
	commodityUUID := must.Must(models.GenerateGroupSlug())
	commodity := must.Must(ownerRegistrySet.CommodityRegistry.Create(ownerCtx, models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{UUID: commodityUUID},
		},
		AreaID:                 new(area.ID),
		Name:                   "Owner Commodity",
		ShortName:              "OC",
		Type:                   models.CommodityTypeElectronics,
		Status:                 models.CommodityStatusInUse,
		Count:                  1,
		OriginalPrice:          decimal.RequireFromString("100.00"),
		CurrentPrice:           decimal.RequireFromString("90.00"),
		OriginalPriceCurrency:  models.Currency("USD"),
		ConvertedOriginalPrice: decimal.Zero,
		PurchaseDate:           models.ToPDate("2023-01-01"),
	}))

	// Guard the fixture's central assumption: the memory registry must keep the
	// UUID we supplied so the ownership lookup keys line up with originalXMLID.
	qt.Assert(t, commodity.UUID, qt.Equals, commodityUUID)

	return &ownershipFixture{
		factorySet:    factorySet,
		owner:         owner,
		attacker:      attacker,
		ownerCtx:      ownerCtx,
		commodityUUID: commodityUUID,
	}
}

func newOwnershipProcessor(fx *ownershipFixture) *RestoreOperationProcessor {
	entityService := services.NewEntityService(fx.factorySet, "/tmp/restore-test")
	return NewRestoreOperationProcessor("test-op", fx.factorySet, entityService, "/tmp/restore-test", nil)
}

// emptyExisting returns an ExistingEntities snapshot with no pre-loaded
// commodities so the lookup falls through to the in-DB UUID map.
func emptyExisting() *types.ExistingEntities {
	return &types.ExistingEntities{Commodities: map[string]*models.Commodity{}}
}

// Case 1: the commodity is already present in the existing-entities snapshot, so
// the function short-circuits to nil without touching the DB.
func TestValidateCommodityOwnershipInDB_ExistingShortCircuit(t *testing.T) {
	c := qt.New(t)
	fx := newOwnershipFixture(t)
	proc := newOwnershipProcessor(fx)

	existing := &types.ExistingEntities{
		Commodities: map[string]*models.Commodity{
			fx.commodityUUID: {Name: "already known"},
		},
	}
	stats := &types.RestoreStats{}

	err := proc.validateCommodityOwnershipInDB(fx.ownerCtx, fx.commodityUUID, fx.owner, existing, stats)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0)
}

// Case 2: the original ID matches a DB commodity owned by the current user → nil.
func TestValidateCommodityOwnershipInDB_OwnedByCurrentUser(t *testing.T) {
	c := qt.New(t)
	fx := newOwnershipFixture(t)
	proc := newOwnershipProcessor(fx)

	stats := &types.RestoreStats{}

	err := proc.validateCommodityOwnershipInDB(fx.ownerCtx, fx.commodityUUID, fx.owner, emptyExisting(), stats)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0)
}

// Case 3: the original ID matches a DB commodity owned by a DIFFERENT user →
// ErrOwnershipViolation, with the error recorded on stats.
func TestValidateCommodityOwnershipInDB_OwnedByDifferentUser(t *testing.T) {
	c := qt.New(t)
	fx := newOwnershipFixture(t)
	proc := newOwnershipProcessor(fx)

	stats := &types.RestoreStats{}

	err := proc.validateCommodityOwnershipInDB(fx.ownerCtx, fx.commodityUUID, fx.attacker, emptyExisting(), stats)
	c.Assert(err, qt.ErrorIs, security.ErrOwnershipViolation)
	c.Assert(stats.ErrorCount, qt.Equals, 1)
	c.Assert(stats.Errors, qt.HasLen, 1)
}

// Case 4: the original ID matches no DB commodity → nil (treated as new).
func TestValidateCommodityOwnershipInDB_NoMatchingCommodity(t *testing.T) {
	c := qt.New(t)
	fx := newOwnershipFixture(t)
	proc := newOwnershipProcessor(fx)

	stats := &types.RestoreStats{}

	err := proc.validateCommodityOwnershipInDB(fx.ownerCtx, "no-such-uuid", fx.owner, emptyExisting(), stats)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0)
}
