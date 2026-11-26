package registrytest

import (
	"slices"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// RunMissingDataContract checks the missing-data filters of the item list
// (#2794): NoPhoto, NoPurchasePrice and NoPurchaseDate, and the
// FileRegistry.CommodityIDsWithPhotos read that backs NoPhoto on a backend
// that is not a registry.NativePhotoFilterer. A photo is what FirstPhotos
// would pick as a cover; an unknown price or date is missing and a known
// zero price is not (#2813). The counts of CountEach must equal the totals of
// ListPaginated for the same options. newEnv is called once per subtest and
// must return an empty group.
func RunMissingDataContract(t *testing.T, newEnv func(t *testing.T) TrashEnv) {
	t.Run("missing photo, purchase price and purchase date", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		ctx := env.Ctx
		at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

		complete := missingDataItem(c, env, "Complete", new(decimal.NewFromInt(10)), nil, "2024-01-01")
		free := missingDataItem(c, env, "Free", new(decimal.Zero), nil, "2024-02-01")
		gift := missingDataItem(c, env, "Gift", nil, nil, "")
		valued := missingDataItem(c, env, "Valued", nil, new(decimal.NewFromInt(50)), "2024-03-01")

		photo := func(name string, fileType models.FileType, category models.FileCategory, meta string, itemIDs ...string) *models.FileEntity {
			return coverContractFile(c, env, name, fileType, category, meta, at, itemIDs...)
		}
		// One photo shared by two items is a photo of both.
		_ = photo("complete-and-valued.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaImages, complete.ID, valued.ID)
		// The free item's only photo is in Trash.
		gone := photo("free.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaImages, free.ID)
		c.Assert(env.Files.Trash(ctx, gone.ID, "registrytest", "", at.Add(time.Hour)), qt.IsNil)
		// None of these is a photo of the gift: a document, an image filed
		// as a document, and an image in the documents bucket.
		_ = photo("gift-card.pdf", models.FileTypeDocument, models.FileCategoryImages, models.FileLinkMetaImages, gift.ID)
		_ = photo("gift-receipt.jpg", models.FileTypeImage, models.FileCategoryDocuments, models.FileLinkMetaImages, gift.ID)
		_ = photo("gift-box.jpg", models.FileTypeImage, models.FileCategoryImages, models.FileLinkMetaDocuments, gift.ID)

		withPhotos, err := env.Files.CommodityIDsWithPhotos(ctx)
		c.Assert(err, qt.IsNil)
		slices.Sort(withPhotos)
		want := []string{complete.ID, valued.ID}
		slices.Sort(want)
		c.Assert(withPhotos, qt.DeepEquals, want)

		base := registry.CommodityListOptions{IncludeInactive: true, PhotoCommodityIDs: withPhotos}
		cases := []struct {
			name string
			opts func(o registry.CommodityListOptions) registry.CommodityListOptions
			want []string
		}{
			{"no photo", func(o registry.CommodityListOptions) registry.CommodityListOptions {
				o.NoPhoto = true
				return o
			}, []string{"Free", "Gift"}},
			{"no purchase price", func(o registry.CommodityListOptions) registry.CommodityListOptions {
				o.NoPurchasePrice = true
				return o
			}, []string{"Gift", "Valued"}},
			{"no purchase date", func(o registry.CommodityListOptions) registry.CommodityListOptions {
				o.NoPurchaseDate = true
				return o
			}, []string{"Gift"}},
			{"no photo and no purchase price", func(o registry.CommodityListOptions) registry.CommodityListOptions {
				o.NoPhoto = true
				o.NoPurchasePrice = true
				return o
			}, []string{"Gift"}},
			{"no filter", func(o registry.CommodityListOptions) registry.CommodityListOptions {
				return o
			}, []string{"Complete", "Free", "Gift", "Valued"}},
		}
		opts := make([]registry.CommodityListOptions, 0, len(cases))
		for _, tc := range cases {
			o := tc.opts(base)
			opts = append(opts, o)
			items, total, err := env.Commodities.ListPaginated(ctx, 0, 50, o)
			c.Assert(err, qt.IsNil)
			names := make([]string, 0, len(items))
			for _, it := range items {
				names = append(names, it.Name)
			}
			c.Check(names, qt.DeepEquals, tc.want, qt.Commentf(tc.name))
			c.Check(total, qt.Equals, len(tc.want), qt.Commentf(tc.name))
		}
		counts, err := env.Commodities.CountEach(ctx, opts)
		c.Assert(err, qt.IsNil)
		for i, tc := range cases {
			c.Check(counts[i], qt.Equals, len(tc.want), qt.Commentf("count of %s", tc.name))
		}
	})
}

func missingDataItem(c *qt.C, env TrashEnv, name string, original, current *decimal.Decimal, purchased string) *models.Commodity {
	c.Helper()
	item := models.Commodity{
		Name:                  name,
		ShortName:             name,
		Type:                  models.CommodityTypeOther,
		AreaID:                new(env.AreaID),
		Count:                 1,
		Status:                models.CommodityStatusInUse,
		OriginalPrice:         original,
		OriginalPriceCurrency: "USD",
		CurrentPrice:          current,
	}
	if purchased != "" {
		item.PurchaseDate = models.ToPDate(models.Date(purchased))
	}
	created, err := env.Commodities.Create(env.Ctx, item)
	c.Assert(err, qt.IsNil)
	return created
}
