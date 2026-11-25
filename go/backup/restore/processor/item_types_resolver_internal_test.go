package processor

// White-box: drives itemTypeResolver directly. Archives without item types in
// the manifest (formats before 3.4 and the XML backup, which builds only
// under the legacy_xml_backup tag) reach resolve and nothing else, and no
// exported API builds such an archive.

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
)

// A restore into a full group gives the items of a type it cannot create the
// type Other and counts that once per archived type, whether the type came
// from the manifest or only from the items.
func TestItemTypeResolver_FullGroupCountsEachMissingTypeOnce(t *testing.T) {
	c := qt.New(t)
	p, ctx, _ := restoreFixture(c)

	resolver := must.Must(p.itemTypeResolver(ctx, types.RestoreOptions{Strategy: types.RestoreStrategyMergeAdd}))
	reg := resolver.reg
	for i := must.Must(reg.Count(ctx)); i < models.MaxItemTypesPerGroup; i++ {
		must.Must(reg.Create(ctx, models.ItemType{Name: fmt.Sprintf("Kind %d", i)}))
	}

	stats := &types.RestoreStats{}
	c.Assert(resolver.importArchived(ctx, []types.INBItemType{{Slug: "boat", Name: "Boat", Icon: "ship"}}, stats), qt.IsNil)
	c.Check(stats.ErrorCount, qt.Equals, 1)

	for _, slug := range []models.CommodityType{"vehicle", "vehicle", "boat", models.CommodityTypeElectronics} {
		got, err := resolver.resolve(ctx, slug, stats)
		c.Assert(err, qt.IsNil)
		if slug == models.CommodityTypeElectronics {
			c.Check(got, qt.Equals, models.CommodityTypeElectronics)
			continue
		}
		c.Check(got, qt.Equals, models.CommodityTypeOther, qt.Commentf("slug %s", slug))
	}
	c.Check(stats.ErrorCount, qt.Equals, 2, qt.Commentf("errors: %v", stats.Errors))
	c.Check(stats.Errors[1], qt.Contains, "item type vehicle:")
	c.Check(must.Must(reg.Count(ctx)), qt.Equals, models.MaxItemTypesPerGroup)
}
