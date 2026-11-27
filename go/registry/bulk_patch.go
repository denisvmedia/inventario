package registry

import (
	"slices"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
)

// PatchedCommodity returns stored with the columns CommodityRegistry.Patch
// writes taken from next: tags, type, status, status date, status note,
// sale price and area. Every other field keeps the stored value. It fails
// with ErrInvalidInput when next carries a tag that is neither on stored
// nor in addTags.
func PatchedCommodity(stored, next models.Commodity, addTags []string) (models.Commodity, error) {
	if err := checkPatchedTags(stored.Tags, next.Tags, addTags); err != nil {
		return models.Commodity{}, err
	}
	out := stored
	out.CustomFields = stored.CustomFields.Clone()
	out.Tags = slices.Clone(next.Tags)
	out.Type = next.Type
	out.Status = next.Status
	out.StatusDate = nil
	if next.StatusDate != nil {
		out.StatusDate = new(*next.StatusDate)
	}
	out.StatusNote = next.StatusNote
	out.SalePrice = nil
	if next.SalePrice != nil {
		out.SalePrice = new(*next.SalePrice)
	}
	out.AreaID = nil
	if next.AreaID != nil {
		out.AreaID = new(*next.AreaID)
	}
	out.NormalizeAreaID()
	return out, nil
}

// PatchedFileTags returns stored with the tags of next, the one column
// FileRegistry.PatchTags writes apart from the update time. It fails as
// PatchedCommodity does.
func PatchedFileTags(stored, next models.FileEntity, addTags []string) (models.FileEntity, error) {
	if err := checkPatchedTags(stored.Tags, next.Tags, addTags); err != nil {
		return models.FileEntity{}, err
	}
	out := stored
	out.Tags = slices.Clone(next.Tags)
	return out, nil
}

// checkPatchedTags refuses a patched tag list that adds a tag addTags does
// not name: a patch locks the tags it adds before the row, and a tag it
// did not lock could be deleted while the row is written.
func checkPatchedTags[S ~[]string](stored, next S, addTags []string) error {
	for _, t := range next {
		if !slices.Contains(stored, t) && !slices.Contains(addTags, t) {
			return errxtrace.Classify(ErrInvalidInput, errx.Attrs("tag", t, "reason", "the patch adds a tag it did not name"))
		}
	}
	return nil
}
