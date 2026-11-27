package services_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/registrytest"
	"go.5x5.cz/inventario/services"
)

// saveComment returns an interleaved write that saves comment on the item,
// as an edit or a bulk change committed after the caller's read would.
func saveComment(comment string) func(context.Context, registry.CommodityRegistry, models.Commodity) error {
	return func(ctx context.Context, reg registry.CommodityRegistry, read models.Commodity) error {
		read.Comments = comment
		_, err := reg.Update(ctx, read)
		return err
	}
}

// An area deleted with the unlink strategy leaves its items area-less, and
// keeps a change saved on an item between the unlink's read and its write.
func TestUnlinkAndDeleteArea_KeepsAChangeSavedMeanwhile(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkFixture(c)
	item := fx.commodity(c, "Bike")
	fx.factorySet.CommodityRegistryFactory = registrytest.InterleaveCommodityWrite(
		fx.factorySet.CommodityRegistryFactory, item.ID, saveComment("saved meanwhile"))

	err := services.NewEntityService(fx.factorySet, fx.uploadLocation).UnlinkAndDeleteArea(fx.ctx, fx.areaID)
	c.Assert(err, qt.IsNil)

	got, err := fx.set.CommodityRegistry.Get(fx.ctx, item.ID)
	c.Assert(err, qt.IsNil)
	c.Check(got.AreaID, qt.IsNil)
	c.Check(got.Comments, qt.Equals, "saved meanwhile")
}

// An item moved to another area between the unlink's read and its write
// stays in that area.
func TestUnlinkAndDeleteArea_KeepsAnItemMovedMeanwhile(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkFixture(c)
	item := fx.commodity(c, "Bike")
	other, err := fx.set.AreaRegistry.Create(fx.ctx, models.Area{Name: "Garage", LocationID: mustArea(c, fx).LocationID})
	c.Assert(err, qt.IsNil)
	fx.factorySet.CommodityRegistryFactory = registrytest.InterleaveCommodityWrite(
		fx.factorySet.CommodityRegistryFactory, item.ID,
		func(ctx context.Context, reg registry.CommodityRegistry, read models.Commodity) error {
			read.AreaID = new(other.ID)
			_, err := reg.Update(ctx, read)
			return err
		})

	err = services.NewEntityService(fx.factorySet, fx.uploadLocation).UnlinkAndDeleteArea(fx.ctx, fx.areaID)
	c.Assert(err, qt.IsNil)

	got, err := fx.set.CommodityRegistry.Get(fx.ctx, item.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.AreaID, qt.IsNotNil)
	c.Check(*got.AreaID, qt.Equals, other.ID)
}

// Removing the cover photo from an item clears its cover and keeps a change
// saved on the item between the clear's read and its write.
func TestFileLinkService_Detach_KeepsAChangeSavedMeanwhile(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkFixture(c)
	item := fx.commodity(c, "Speaker")
	photo := fx.storedFile(c, "speaker.jpg", "image/jpeg", models.FileCategoryImages,
		models.FileLink{EntityType: models.FileLinkCommodity, EntityID: item.ID, Meta: models.FileLinkMetaImages})
	item.CoverFileID = new(photo.ID)
	_, err := fx.set.CommodityRegistry.Update(fx.ctx, *item)
	c.Assert(err, qt.IsNil)
	fx.factorySet.CommodityRegistryFactory = registrytest.InterleaveCommodityWrite(
		fx.factorySet.CommodityRegistryFactory, item.ID, saveComment("saved meanwhile"))

	_, err = services.NewFileLinkService(fx.factorySet).Detach(fx.ctx, photo.ID, photo.LinkTo(models.FileLinkCommodity, item.ID).ID)
	c.Assert(err, qt.IsNil)

	got, err := fx.set.CommodityRegistry.Get(fx.ctx, item.ID)
	c.Assert(err, qt.IsNil)
	c.Check(got.CoverFileID, qt.IsNil)
	c.Check(got.Comments, qt.Equals, "saved meanwhile")
}

// A write that loses to another one every time gives up after three reads
// with the conflict.
func TestRewriteCommodity_ReturnsTheConflictAfterThreeAttempts(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkFixture(c)
	item := fx.commodity(c, "Bike")

	reads := 0
	_, _, err := services.RewriteCommodity(fx.ctx, fx.set.CommodityRegistry, item.ID, nil, func(next *models.Commodity) bool {
		reads++
		// Another write lands after every read.
		bumped := *next
		bumped.Comments = next.Comments + "!"
		_, err := fx.set.CommodityRegistry.Update(fx.ctx, bumped)
		c.Assert(err, qt.IsNil)
		next.SerialNumber = "SN-1"
		return true
	})
	c.Assert(err, qt.ErrorIs, registry.ErrRevisionConflict)
	c.Check(reads, qt.Equals, 3)
}

func mustArea(c *qt.C, fx fileLinkFixture) *models.Area {
	c.Helper()
	area, err := fx.set.AreaRegistry.Get(fx.ctx, fx.areaID)
	c.Assert(err, qt.IsNil)
	return area
}
