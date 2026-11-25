package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestItemTypeContract_Postgres(t *testing.T) {
	registrytest.RunItemTypeContract(t, func(t *testing.T) registrytest.ItemTypeEnv {
		fx := newTagPGFixture(t)
		return registrytest.ItemTypeEnv{
			CtxA:         fx.ctxA,
			CtxB:         fx.ctxB,
			TypesA:       fx.groupASet.ItemTypeRegistry,
			TypesB:       fx.groupBSet.ItemTypeRegistry,
			ServiceTypes: fx.factorySet.ItemTypeRegistryFactory.CreateServiceRegistry(),
			CommoditiesA: fx.groupASet.CommodityRegistry,
			CommoditiesB: fx.groupBSet.CommodityRegistry,
			AreaA:        fx.areaAID,
			AreaB:        fx.areaBID,
			GroupA:       fx.groupAID,
			GroupB:       fx.groupBID,
		}
	})
}

// The group and tenant purges drop item types, which Delete alone could not
// do for a group's last one, and the ownership check sees the seeded rows
// as the group creator's.
func TestItemTypes_PurgeAndOwnership_Postgres(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	_, err := fx.groupASet.ItemTypeRegistry.EnsureBuiltins(fx.ctxA)
	c.Assert(err, qt.IsNil)
	_, err = fx.groupBSet.ItemTypeRegistry.EnsureBuiltins(fx.ctxB)
	c.Assert(err, qt.IsNil)

	list, err := fx.groupASet.ItemTypeRegistry.List(fx.ctxA)
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, len(models.BuiltinItemTypes))
	for _, it := range list {
		c.Check(it.CreatedByUserID, qt.Equals, fx.user.ID)
		c.Check(it.TenantID, qt.Equals, fx.user.TenantID)
	}

	c.Assert(fx.factorySet.GroupPurger.PurgeGroupDependents(t.Context(), fx.user.TenantID, fx.groupAID), qt.IsNil)
	countA, err := fx.groupASet.ItemTypeRegistry.Count(fx.ctxA)
	c.Assert(err, qt.IsNil)
	c.Check(countA, qt.Equals, 0)
	countB, err := fx.groupBSet.ItemTypeRegistry.Count(fx.ctxB)
	c.Assert(err, qt.IsNil)
	c.Check(countB, qt.Equals, len(models.BuiltinItemTypes))

	owns, err := fx.factorySet.UserContentOwnershipChecker.HasRetainedOwnedContent(t.Context(), fx.user.TenantID, fx.user.ID, []string{fx.groupAID, fx.groupBID})
	c.Assert(err, qt.IsNil)
	c.Check(owns, qt.IsFalse, qt.Commentf("item types in purged groups are not retained content"))
}
