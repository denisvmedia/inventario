package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestCustomFieldContract_Postgres(t *testing.T) {
	registrytest.RunCustomFieldContract(t, func(t *testing.T) registrytest.CustomFieldEnv {
		fx := newTagPGFixture(t)
		return registrytest.CustomFieldEnv{
			CtxA:         fx.ctxA,
			CtxB:         fx.ctxB,
			FieldsA:      fx.groupASet.CustomFieldRegistry,
			FieldsB:      fx.groupBSet.CustomFieldRegistry,
			TypesA:       fx.groupASet.ItemTypeRegistry,
			CommoditiesA: fx.groupASet.CommodityRegistry,
			CommoditiesB: fx.groupBSet.CommodityRegistry,
			AreaA:        fx.areaAID,
			AreaB:        fx.areaBID,
		}
	})
}

// The group purge drops a group's custom fields, their values and template
// entries; the ownership check sees a field as its author's content until
// its group is purged.
func TestCustomFields_PurgeAndOwnership_Postgres(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	other, err := fx.groupASet.ItemTypeRegistry.GetBySlug(fx.ctxA, models.CommodityTypeOther)
	if err != nil {
		_, err = fx.groupASet.ItemTypeRegistry.EnsureBuiltins(fx.ctxA)
		c.Assert(err, qt.IsNil)
		other, err = fx.groupASet.ItemTypeRegistry.GetBySlug(fx.ctxA, models.CommodityTypeOther)
	}
	c.Assert(err, qt.IsNil)
	field, err := fx.groupASet.CustomFieldRegistry.Create(fx.ctxA, models.CustomFieldDefinition{
		Name: "Serial", Type: models.CustomFieldTypeText,
		Templates: []models.CustomFieldTemplate{{ItemTypeID: other.ID}},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.groupASet.CommodityRegistry.Create(fx.ctxA, models.Commodity{
		Name: "Thing", ShortName: "Thing", Type: models.CommodityTypeOther, AreaID: new(fx.areaAID),
		Count: 1, Status: models.CommodityStatusInUse,
		CustomFields: models.CustomFieldValues{field.ID: []byte(`"S-1"`)},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.groupBSet.CustomFieldRegistry.Create(fx.ctxB, models.CustomFieldDefinition{Name: "Serial", Type: models.CustomFieldTypeText})
	c.Assert(err, qt.IsNil)

	owns, err := fx.factorySet.UserContentOwnershipChecker.HasRetainedOwnedContent(t.Context(), fx.user.TenantID, fx.user.ID, nil)
	c.Assert(err, qt.IsNil)
	c.Check(owns, qt.IsTrue)

	c.Assert(fx.factorySet.GroupPurger.PurgeGroupDependents(t.Context(), fx.user.TenantID, fx.groupAID), qt.IsNil)
	countA, err := fx.groupASet.CustomFieldRegistry.Count(fx.ctxA)
	c.Assert(err, qt.IsNil)
	c.Check(countA, qt.Equals, 0)
	countB, err := fx.groupBSet.CustomFieldRegistry.Count(fx.ctxB)
	c.Assert(err, qt.IsNil)
	c.Check(countB, qt.Equals, 1)

	c.Assert(fx.factorySet.TenantPurger.PurgeTenantDependents(t.Context(), fx.user.TenantID), qt.IsNil)
	all, err := fx.factorySet.CustomFieldRegistryFactory.CreateServiceRegistry().List(t.Context())
	c.Assert(err, qt.IsNil)
	c.Check(all, qt.HasLen, 0)
}
