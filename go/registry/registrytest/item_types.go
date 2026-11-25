package registrytest

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// ItemTypeEnv is what RunItemTypeContract needs from a backend: registries
// of two groups of one tenant, each with an area items can be placed in, the
// contexts that carry each group's scope, and a service item type registry.
// Both groups exist and start without item types and without items.
type ItemTypeEnv struct {
	CtxA, CtxB                 context.Context
	TypesA, TypesB             registry.ItemTypeRegistry
	ServiceTypes               registry.ItemTypeRegistry
	CommoditiesA, CommoditiesB registry.CommodityRegistry
	AreaA, AreaB               string
	GroupA, GroupB             string
}

// RunItemTypeContract checks the behavior registry.ItemTypeRegistry
// promises: the built-in types are seeded once into a group that has none,
// slugs are made from names and never change, names are unique per group,
// item writes accept only the group's types, a type in use moves its items
// when it is deleted, the last type stays, and a group sees only its own
// types.
func RunItemTypeContract(t *testing.T, newEnv func(t *testing.T) ItemTypeEnv) {
	t.Helper()

	t.Run("list seeds the built-in types once", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		list, err := env.TypesA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Assert(itemTypeSlugs(list), qt.DeepEquals, builtinSlugs())
		for _, it := range list {
			c.Check(it.Name, qt.Equals, "", qt.Commentf("slug %s", it.Slug))
			c.Check(it.Icon, qt.Not(qt.Equals), "")
			c.Check(it.GroupID, qt.Equals, env.GroupA)
		}

		created, err := env.TypesA.EnsureBuiltins(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(created, qt.Equals, 0)
		again, err := env.TypesA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(again, qt.HasLen, len(models.BuiltinItemTypes))

		// Group B is untouched until something reads or writes it.
		count, err := env.TypesB.Count(env.CtxB)
		c.Assert(err, qt.IsNil)
		c.Check(count, qt.Equals, 0)
	})

	t.Run("a deleted built-in type is not seeded again", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		furniture := mustTypeBySlug(c, env.TypesA, env.CtxA, models.CommodityTypeFurniture)
		c.Assert(env.TypesA.Delete(env.CtxA, furniture.ID), qt.IsNil)

		created, err := env.TypesA.EnsureBuiltins(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(created, qt.Equals, 0)
		backfilled, err := env.ServiceTypes.BackfillBuiltins(env.CtxA)
		c.Assert(err, qt.IsNil)
		list, err := env.TypesA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(itemTypeSlugs(list), qt.Not(qt.Contains), models.CommodityTypeFurniture)
		c.Check(list, qt.HasLen, len(models.BuiltinItemTypes)-1)
		// The backfill seeded group B, which had none.
		c.Check(backfilled >= len(models.BuiltinItemTypes), qt.IsTrue, qt.Commentf("backfilled %d", backfilled))
		countB, err := env.TypesB.Count(env.CtxB)
		c.Assert(err, qt.IsNil)
		c.Check(countB, qt.Equals, len(models.BuiltinItemTypes))
	})

	t.Run("create makes a slug from the name and refuses a duplicate name", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		vehicle, err := env.TypesA.Create(env.CtxA, models.ItemType{Name: "  Vehicle ", Icon: "car"})
		c.Assert(err, qt.IsNil)
		c.Check(vehicle.ID, qt.Not(qt.Equals), "")
		c.Check(vehicle.Slug, qt.Equals, models.CommodityType("vehicle"))
		c.Check(vehicle.Name, qt.Equals, "Vehicle")
		c.Check(vehicle.Icon, qt.Equals, "car")

		// Creating the first custom type seeded the built-in ones too.
		list, err := env.TypesA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(list, qt.HasLen, len(models.BuiltinItemTypes)+1)
		c.Check(list[len(list)-1].ID, qt.Equals, vehicle.ID, qt.Commentf("custom types sort after the built-in ones"))

		for _, dup := range []string{"vehicle", "VEHICLE", " Vehicle\t"} {
			_, err := env.TypesA.Create(env.CtxA, models.ItemType{Name: dup})
			c.Check(err, qt.ErrorIs, registry.ErrAlreadyExists, qt.Commentf("name %q", dup))
		}
		// A built-in type that keeps its default name owns that name.
		_, err = env.TypesA.Create(env.CtxA, models.ItemType{Name: "electronics"})
		c.Check(err, qt.ErrorIs, registry.ErrAlreadyExists)

		// A name whose slug is taken, or is a built-in slug, gets a suffix.
		vehicles, err := env.TypesA.Create(env.CtxA, models.ItemType{Name: "Vehicle!"})
		c.Assert(err, qt.IsNil)
		c.Check(vehicles.Slug, qt.Equals, models.CommodityType("vehicle-2"))
		others, err := env.TypesA.Create(env.CtxA, models.ItemType{Name: "Other things"})
		c.Assert(err, qt.IsNil)
		c.Check(others.Slug, qt.Equals, models.CommodityType("other-things"))
		symbols, err := env.TypesA.Create(env.CtxA, models.ItemType{Name: "###"})
		c.Assert(err, qt.IsNil)
		c.Check(symbols.Slug, qt.Equals, models.CommodityType("type"))

		// The same name in another group is a different type.
		inB, err := env.TypesB.Create(env.CtxB, models.ItemType{Name: "Vehicle"})
		c.Assert(err, qt.IsNil)
		c.Check(inB.Slug, qt.Equals, models.CommodityType("vehicle"))

		got, err := env.TypesA.GetByName(env.CtxA, " vehicle ")
		c.Assert(err, qt.IsNil)
		c.Check(got.ID, qt.Equals, vehicle.ID)
		got, err = env.TypesA.GetByName(env.CtxA, "Appliance")
		c.Assert(err, qt.IsNil)
		c.Check(got.Slug, qt.Equals, models.CommodityTypeWhiteGoods)
		got, err = env.TypesA.GetBySlug(env.CtxA, "vehicle")
		c.Assert(err, qt.IsNil)
		c.Check(got.ID, qt.Equals, vehicle.ID)
	})

	t.Run("create keeps a requested slug when it is free", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		boat, err := env.TypesA.Create(env.CtxA, models.ItemType{Slug: "boat", Name: "Small craft"})
		c.Assert(err, qt.IsNil)
		c.Check(boat.Slug, qt.Equals, models.CommodityType("boat"))
		_, err = env.TypesA.Create(env.CtxA, models.ItemType{Slug: "boat", Name: "Dinghy"})
		c.Check(err, qt.ErrorIs, registry.ErrAlreadyExists)
	})

	t.Run("update renames without changing the slug", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		vehicle, err := env.TypesA.Create(env.CtxA, models.ItemType{Name: "Vehicle", Icon: "car"})
		c.Assert(err, qt.IsNil)
		_, err = env.TypesA.Create(env.CtxA, models.ItemType{Name: "Bicycle"})
		c.Assert(err, qt.IsNil)

		vehicle.Name = "BICYCLE"
		_, err = env.TypesA.Update(env.CtxA, *vehicle)
		c.Check(err, qt.ErrorIs, registry.ErrAlreadyExists)

		vehicle.Name = "Car"
		vehicle.Icon = "truck"
		vehicle.Slug = "smuggled"
		updated, err := env.TypesA.Update(env.CtxA, *vehicle)
		c.Assert(err, qt.IsNil)
		c.Check(updated.Name, qt.Equals, "Car")
		c.Check(updated.Icon, qt.Equals, "truck")
		c.Check(updated.Slug, qt.Equals, models.CommodityType("vehicle"))

		// A built-in type can take a name of its own and give it back.
		other := mustTypeBySlug(c, env.TypesA, env.CtxA, models.CommodityTypeOther)
		other.Name = "Misc"
		renamed, err := env.TypesA.Update(env.CtxA, *other)
		c.Assert(err, qt.IsNil)
		c.Check(renamed.Name, qt.Equals, "Misc")
		_, err = env.TypesA.Create(env.CtxA, models.ItemType{Name: "Other"})
		c.Assert(err, qt.IsNil, qt.Commentf("the default name is free once the built-in type has its own"))
	})

	t.Run("item writes take only the group's types", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		// A built-in slug seeds a group that has no types yet.
		_, err := env.CommoditiesB.Create(env.CtxB, itemTypeContractItem("Lamp", models.CommodityTypeOther, env.AreaB))
		c.Assert(err, qt.IsNil)
		countB, err := env.TypesB.Count(env.CtxB)
		c.Assert(err, qt.IsNil)
		c.Check(countB, qt.Equals, len(models.BuiltinItemTypes))

		vehicleB, err := env.TypesB.Create(env.CtxB, models.ItemType{Name: "Vehicle"})
		c.Assert(err, qt.IsNil)

		// Group A does not see group B's custom type.
		_, err = env.CommoditiesA.Create(env.CtxA, itemTypeContractItem("Car", vehicleB.Slug, env.AreaA))
		c.Check(err, qt.ErrorIs, registry.ErrUnknownItemType)
		_, err = env.TypesA.Get(env.CtxA, vehicleB.ID)
		c.Check(err, qt.ErrorIs, registry.ErrNotFound)

		vehicleA, err := env.TypesA.Create(env.CtxA, models.ItemType{Name: "Vehicle"})
		c.Assert(err, qt.IsNil)
		created, err := env.CommoditiesA.Create(env.CtxA, itemTypeContractItem("Car", vehicleA.Slug, env.AreaA))
		c.Assert(err, qt.IsNil)
		car, err := env.CommoditiesA.Get(env.CtxA, created.ID)
		c.Assert(err, qt.IsNil)

		car.Type = "spaceship"
		_, err = env.CommoditiesA.Update(env.CtxA, *car)
		c.Check(err, qt.ErrorIs, registry.ErrUnknownItemType)
		car.Type = models.CommodityTypeEquipment
		updated, err := env.CommoditiesA.Update(env.CtxA, *car)
		c.Assert(err, qt.IsNil)
		c.Check(updated.Type, qt.Equals, models.CommodityTypeEquipment)
	})

	runItemTypeDeleteContract(t, newEnv)
}

// runItemTypeDeleteContract covers DeleteReassigning: the in-use check, the
// move of the items, and the last type of a group.
func runItemTypeDeleteContract(t *testing.T, newEnv func(t *testing.T) ItemTypeEnv) {
	t.Helper()

	t.Run("delete refuses a type in use and moves items when told where", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		vehicle, err := env.TypesA.Create(env.CtxA, models.ItemType{Name: "Vehicle"})
		c.Assert(err, qt.IsNil)
		car, err := env.CommoditiesA.Create(env.CtxA, itemTypeContractItem("Car", vehicle.Slug, env.AreaA))
		c.Assert(err, qt.IsNil)
		bike, err := env.CommoditiesA.Create(env.CtxA, itemTypeContractItem("Bike", vehicle.Slug, env.AreaA))
		c.Assert(err, qt.IsNil)
		_, err = env.CommoditiesA.Trash(env.CtxA, bike.ID, "registrytest", time.Now())
		c.Assert(err, qt.IsNil)
		_, err = env.CommoditiesA.Create(env.CtxA, itemTypeContractItem("Lamp", models.CommodityTypeOther, env.AreaA))
		c.Assert(err, qt.IsNil)

		counts, err := env.TypesA.ItemCounts(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(counts[vehicle.Slug], qt.Equals, registry.ItemTypeUsage{Live: 1, Trashed: 1})
		c.Check(counts[models.CommodityTypeOther], qt.Equals, registry.ItemTypeUsage{Live: 1})

		// Group B's counts do not include group A's items.
		countsB, err := env.TypesB.ItemCounts(env.CtxB)
		c.Assert(err, qt.IsNil)
		c.Check(countsB, qt.HasLen, 0)

		err = env.TypesA.Delete(env.CtxA, vehicle.ID)
		c.Check(err, qt.ErrorIs, registry.ErrItemTypeInUse)

		_, err = env.TypesA.DeleteReassigning(env.CtxA, vehicle.ID, vehicle.ID)
		c.Check(err, qt.ErrorIs, registry.ErrInvalidInput, qt.Commentf("a type cannot take its own items"))
		otherB, err := env.TypesB.Create(env.CtxB, models.ItemType{Name: "Elsewhere"})
		c.Assert(err, qt.IsNil)
		_, err = env.TypesA.DeleteReassigning(env.CtxA, vehicle.ID, otherB.ID)
		c.Check(err, qt.ErrorIs, registry.ErrInvalidInput, qt.Commentf("another group's type is not a target"))

		equipment := mustTypeBySlug(c, env.TypesA, env.CtxA, models.CommodityTypeEquipment)
		moved, err := env.TypesA.DeleteReassigning(env.CtxA, vehicle.ID, equipment.ID)
		c.Assert(err, qt.IsNil)
		c.Check(moved, qt.Equals, 2, qt.Commentf("the item in Trash moves too"))

		_, err = env.TypesA.Get(env.CtxA, vehicle.ID)
		c.Check(err, qt.ErrorIs, registry.ErrNotFound)
		got, err := env.CommoditiesA.Get(env.CtxA, car.ID)
		c.Assert(err, qt.IsNil)
		c.Check(got.Type, qt.Equals, models.CommodityTypeEquipment)
		counts, err = env.TypesA.ItemCounts(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(counts[models.CommodityTypeEquipment], qt.Equals, registry.ItemTypeUsage{Live: 1, Trashed: 1})
		_, gone := counts[vehicle.Slug]
		c.Check(gone, qt.IsFalse)

		// An unused type goes without a target.
		clothes := mustTypeBySlug(c, env.TypesA, env.CtxA, models.CommodityTypeClothes)
		moved, err = env.TypesA.DeleteReassigning(env.CtxA, clothes.ID, "")
		c.Assert(err, qt.IsNil)
		c.Check(moved, qt.Equals, 0)
	})

	t.Run("the last type stays", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		list, err := env.TypesA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		for _, it := range list[1:] {
			c.Assert(env.TypesA.Delete(env.CtxA, it.ID), qt.IsNil)
		}
		err = env.TypesA.Delete(env.CtxA, list[0].ID)
		c.Check(err, qt.ErrorIs, registry.ErrLastItemType)

		count, err := env.TypesA.Count(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(count, qt.Equals, 1)
	})
}

func itemTypeContractItem(name string, slug models.CommodityType, areaID string) models.Commodity {
	return models.Commodity{
		Name:      name,
		ShortName: name,
		Type:      slug,
		AreaID:    new(areaID),
		Count:     1,
		Status:    models.CommodityStatusInUse,
	}
}

func mustTypeBySlug(c *qt.C, reg registry.ItemTypeRegistry, ctx context.Context, slug models.CommodityType) *models.ItemType {
	c.Helper()
	_, err := reg.EnsureBuiltins(ctx)
	c.Assert(err, qt.IsNil, qt.Commentf("seed item types"))
	t, err := reg.GetBySlug(ctx, slug)
	c.Assert(err, qt.IsNil)
	return t
}

func itemTypeSlugs(types []*models.ItemType) []models.CommodityType {
	out := make([]models.CommodityType, 0, len(types))
	for _, t := range types {
		out = append(out, t.Slug)
	}
	return out
}

func builtinSlugs() []models.CommodityType {
	out := make([]models.CommodityType, 0, len(models.BuiltinItemTypes))
	for _, b := range models.BuiltinItemTypes {
		out = append(out, b.Slug)
	}
	return out
}
