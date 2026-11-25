//go:build !legacy_xml_backup

package backup_test

import (
	"archive/tar"
	"encoding/json"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
)

// addReviewedCommodity stores a non-draft commodity with the given purchase
// data in the fixture's area and returns its UUID.
func (f *inbFixture) addReviewedCommodity(c *qt.C, com models.Commodity) string {
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	com.TenantGroupAwareEntityID = models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID}
	com.ShortName = com.Name
	com.Type = models.CommodityTypeOther
	com.AreaID = new(f.areaID)
	com.Count = 1
	com.Status = models.CommodityStatusInUse
	return must.Must(comReg.Create(f.ctx, com)).UUID
}

// TestINBRoundTrip_KeepsPriceKnowledge exports and restores the issue's four
// acquisitions and checks that an unknown price comes back unknown and a zero
// price comes back zero.
func TestINBRoundTrip_KeepsPriceKnowledge(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	want := map[string]models.Commodity{
		"Gift": {
			Name:                  "Gift",
			OriginalPriceCurrency: "USD",
		},
		"Free": {
			Name:                  "Free",
			OriginalPrice:         new(decimal.Zero),
			OriginalPriceCurrency: "USD",
			CurrentPrice:          new(decimal.Zero),
			PurchaseDate:          models.ToPDate("2024-02-01"),
		},
		"Appraised": {
			Name:                  "Appraised",
			OriginalPriceCurrency: "USD",
			CurrentPrice:          new(decimal.RequireFromString("250.5")),
		},
		"Imported": {
			Name:                   "Imported",
			OriginalPrice:          new(decimal.NewFromInt(100)),
			OriginalPriceCurrency:  "EUR",
			ConvertedOriginalPrice: new(decimal.NewFromInt(110)),
			PurchaseDate:           models.ToPDate("2024-03-01"),
		},
	}
	uuids := make(map[string]string, len(want))
	for name, com := range want {
		uuids[name] = f.addReviewedCommodity(c, com)
	}

	blobKey, archive := f.runExport(c, signer)

	// In the archive an unknown price is an absent member and a zero is "0".
	_, jsons := innerMembers(c, archive)
	var doc map[string]any
	for name, body := range jsons {
		if strings.HasPrefix(name, "location-") {
			c.Assert(json.Unmarshal(body, &doc), qt.IsNil)
		}
	}
	c.Assert(doc, qt.IsNotNil)
	exported := map[string]map[string]any{}
	for _, raw := range doc["commodities"].([]any) {
		com := raw.(map[string]any)
		exported[com["name"].(string)] = com
	}
	for _, member := range []string{"originalPrice", "convertedOriginalPrice", "currentPrice", "purchaseDate"} {
		_, present := exported["Gift"][member]
		c.Assert(present, qt.IsFalse, qt.Commentf("%s", member))
	}
	c.Assert(exported["Free"]["originalPrice"], qt.Equals, "0")
	c.Assert(exported["Free"]["currentPrice"], qt.Equals, "0")

	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	for name, expected := range want {
		restored := f.commodityByUUID(c, uuids[name])
		c.Assert(models.SamePrice(restored.OriginalPrice, expected.OriginalPrice), qt.IsTrue, qt.Commentf("%s original: %v", name, restored.OriginalPrice))
		c.Assert(models.SamePrice(restored.ConvertedOriginalPrice, expected.ConvertedOriginalPrice), qt.IsTrue, qt.Commentf("%s converted: %v", name, restored.ConvertedOriginalPrice))
		c.Assert(models.SamePrice(restored.CurrentPrice, expected.CurrentPrice), qt.IsTrue, qt.Commentf("%s current: %v", name, restored.CurrentPrice))
		c.Assert(restored.OriginalPriceCurrency, qt.Equals, expected.OriginalPriceCurrency)
		c.Assert(restored.PurchaseDate.Equal(expected.PurchaseDate), qt.IsTrue, qt.Commentf("%s purchase date", name))
		c.Assert(restored.Draft, qt.IsFalse)
	}
}

// TestINBRestore_PriceMeaningByFormatVersion checks how an archive's absent
// and zero prices are read. Before 3.1 the exporter left out every zero price,
// so an absent purchase price is restored as the zero it stood for, while an
// absent or zero current or converted price is restored as unknown (zero meant
// "not set" there). From 3.1 an absent price is unknown and "0" is a zero.
func TestINBRestore_PriceMeaningByFormatVersion(t *testing.T) {
	tests := []struct {
		name          string
		version       string
		original      string
		current       string
		wantOriginal  *decimal.Decimal
		wantCurrent   *decimal.Decimal
		wantConverted *decimal.Decimal
	}{
		{name: "3.0 absent prices", version: "3.0", wantOriginal: new(decimal.Zero)},
		{name: "3.0 known prices", version: "3.0", original: "12", current: "10", wantOriginal: new(decimal.NewFromInt(12)), wantCurrent: new(decimal.NewFromInt(10))},
		{name: "2.0 absent prices", version: "2.0", wantOriginal: new(decimal.Zero)},
		{name: "3.1 absent prices", version: "3.1"},
		{name: "3.1 zero prices", version: "3.1", original: "0", current: "0", wantOriginal: new(decimal.Zero), wantCurrent: new(decimal.Zero)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			signer := testSigner(c)
			f := newInbFixture(c)

			doc := inbDocForFixture(c, f, func(com *types.INBCommodity) {
				com.Draft = false
				com.OriginalPrice = tt.original
				com.CurrentPrice = tt.current
			})
			docJSON := must.Must(json.Marshal(doc))
			manifest := must.Must(json.Marshal(map[string]string{"version": tt.version}))
			archive := signedArchiveFromInnerTar(c, signer, func(tw *tar.Writer) {
				writeMember(c, tw, &tar.Header{Name: "manifest.json", Mode: 0o600, Typeflag: tar.TypeReg}, manifest)
				writeMember(c, tw, &tar.Header{Name: "location-home.json", Mode: 0o600, Typeflag: tar.TypeReg}, docJSON)
			})

			bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
			key := "t/tenant-a/restores/price-meaning.inb"
			c.Assert(bucket.WriteAll(f.ctx, key, archive, nil), qt.IsNil)
			c.Assert(bucket.Close(), qt.IsNil)

			final, err := restoreInb(c, f, signer, key)
			c.Assert(err, qt.IsNil)
			c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

			restored := f.commodityByUUID(c, doc.Commodities[0].ID)
			c.Assert(models.SamePrice(restored.OriginalPrice, tt.wantOriginal), qt.IsTrue, qt.Commentf("original: %v", restored.OriginalPrice))
			c.Assert(models.SamePrice(restored.CurrentPrice, tt.wantCurrent), qt.IsTrue, qt.Commentf("current: %v", restored.CurrentPrice))
			c.Assert(models.SamePrice(restored.ConvertedOriginalPrice, tt.wantConverted), qt.IsTrue, qt.Commentf("converted: %v", restored.ConvertedOriginalPrice))
		})
	}
}

func TestINBFormatVersion_TellsUnknownFromZero(t *testing.T) {
	c := qt.New(t)
	c.Assert(types.PriceReadingFor(export.INBFormatVersion), qt.Equals, types.PricesKeepZero)
	c.Assert(types.PriceReadingFor("3.2"), qt.Equals, types.PricesKeepZero)
	c.Assert(types.PriceReadingFor("4.0"), qt.Equals, types.PricesKeepZero)
	c.Assert(types.PriceReadingFor("3.0"), qt.Equals, types.PricesOmitZero)
	c.Assert(types.PriceReadingFor("2.1"), qt.Equals, types.PricesOmitZero)
	c.Assert(types.PriceReadingFor(""), qt.Equals, types.PricesOmitZero)
}
