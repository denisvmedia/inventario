package seeddata

import (
	"context"
	"fmt"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// seedTagSpec is the inline catalogue of tags the inventory commodities
// reference. Keep this list and the per-commodity Tags slices in
// seeddata_inventory.go in sync — a commodity that references a slug
// not present here would still work (the Tag row would auto-provision
// with DefaultTagColor on first reference) but the curated colors are
// what give the /tags page its visual identity.
type seedTagSpec struct {
	Slug  string
	Label string
	Color models.TagColor
}

// seedTagCatalogue is the curated set of tags the seed installs. Picked
// to overlap with the bundled commodity types and the bundled files'
// natural buckets: kitchen / electronics / outdoor for type-style tags,
// fragile / warranty-watch / loaned-out for cross-cutting "needs
// attention" labels, vintage / gift / work / seasonal as flavour.
//
// Slugs match the lower-snake-cased label so the auto-provision path
// (commodity write with unknown slug → create tag row with
// DefaultTagColor) would only collide if a stray hand-edited slug
// landed first. None of the seed commodities write unknown slugs, but
// the safeguard matters for any test that POSTs additional commodities
// against a seeded DB.
var seedTagCatalogue = []seedTagSpec{
	{Slug: "kitchen", Label: "Kitchen", Color: models.TagColorOrange},
	{Slug: "electronics", Label: "Electronics", Color: models.TagColorBlue},
	{Slug: "fragile", Label: "Fragile", Color: models.TagColorRed},
	{Slug: "vintage", Label: "Vintage", Color: models.TagColorAmber},
	{Slug: "work", Label: "Work", Color: models.TagColorBlue},
	{Slug: "outdoor", Label: "Outdoor", Color: models.TagColorGreen},
	{Slug: "seasonal", Label: "Seasonal", Color: models.TagColorAmber},
	{Slug: "warranty-watch", Label: "Warranty watch", Color: models.TagColorRed},
	{Slug: "loaned-out", Label: "Loaned out", Color: models.TagColorOrange},
	{Slug: "gift", Label: "Gift", Color: models.TagColorGreen},
}

// seedTags writes the curated tag catalogue into the current group.
// Idempotent on an empty group; called from the orchestrator only
// after the locations-count gate so the no-op rerun path doesn't reach
// here.
func seedTags(ctx context.Context, set *registry.Set, user *models.User, group *models.LocationGroup) error {
	now := time.Now()
	for _, spec := range seedTagCatalogue {
		tag := models.Tag{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID:        user.TenantID,
				GroupID:         group.ID,
				CreatedByUserID: user.ID,
			},
			Slug:      spec.Slug,
			Label:     spec.Label,
			Color:     spec.Color,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if _, err := set.TagRegistry.Create(ctx, tag); err != nil {
			return fmt.Errorf("create tag %s: %w", spec.Slug, err)
		}
	}
	return nil
}
