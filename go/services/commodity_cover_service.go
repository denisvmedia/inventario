package services

import (
	"context"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// CoverSource enumerates the resolved cover photo's provenance. Today only
// FirstPhoto ships (issue #1451 option A); option B's `cover_file_id`
// override will surface as Explicit once the migration lands.
type CoverSource string

const (
	CoverSourceFirstPhoto CoverSource = "first_photo"
	CoverSourceExplicit   CoverSource = "explicit"
)

// ResolvedCover is the cover image picked for a single commodity, paired
// with its file id and the signed thumbnail URLs the FE renders. Source
// names which path produced it so the FE can later distinguish auto-pick
// from explicit-override.
type ResolvedCover struct {
	CommodityID string
	FileID      string
	Thumbnails  map[string]string
	Source      CoverSource
}

// CommodityCoverService resolves the cover image for one or more
// commodities. The current strategy is option (A): the earliest file
// (by `created_at`) with `linked_entity_type=commodity` /
// `linked_entity_id=<id>` / `category=photos`. Issue #1451 option (B)
// — `cover_file_id` override — slots in here once the migration lands.
type CommodityCoverService struct {
	signing *FileSigningService
}

// NewCommodityCoverService wires the cover resolver to the signing service
// it uses to mint thumbnail URLs.
func NewCommodityCoverService(signing *FileSigningService) *CommodityCoverService {
	return &CommodityCoverService{signing: signing}
}

// ResolveMany returns the cover image for each commodity in the input
// slice. Commodities without a usable photo are absent from the returned
// map (no zero-value entries). Errors from individual commodities are
// swallowed so the list endpoint never 500s on a partial failure — the
// FE already handles the missing-cover fallback.
//
// Resolution order per commodity (issue #1451):
//
//  1. Explicit override — `commodity.CoverFileID` (option B). The file
//     must still exist, belong to the commodity, and be an image; on
//     any failure the path falls through to (2) so a stale override
//     never blocks the auto-pick.
//  2. First photo by `created_at` ASC (option A — the default).
//
// Caller must supply the user id used to sign the URLs; an empty userID
// short-circuits to an empty result because the signed URL would be
// unverifiable.
func (s *CommodityCoverService) ResolveMany(ctx context.Context, fileReg registry.FileRegistry, commodities []*models.Commodity, userID string) map[string]ResolvedCover {
	out := make(map[string]ResolvedCover, len(commodities))
	if userID == "" || fileReg == nil || len(commodities) == 0 {
		return out
	}

	for _, c := range commodities {
		if c == nil || c.ID == "" {
			continue
		}
		cover, ok := s.resolveOne(ctx, fileReg, c, userID)
		if !ok {
			continue
		}
		out[c.ID] = cover
	}
	return out
}

// ResolveOne is the single-commodity convenience that the GET handler
// uses. Returns ok=false when no photo is attached or the URL signing
// fails — callers should fall back to the type emoji.
func (s *CommodityCoverService) ResolveOne(ctx context.Context, fileReg registry.FileRegistry, commodity *models.Commodity, userID string) (ResolvedCover, bool) {
	if userID == "" || fileReg == nil || commodity == nil || commodity.ID == "" {
		return ResolvedCover{}, false
	}
	return s.resolveOne(ctx, fileReg, commodity, userID)
}

func (s *CommodityCoverService) resolveOne(ctx context.Context, fileReg registry.FileRegistry, commodity *models.Commodity, userID string) (ResolvedCover, bool) {
	if commodity.CoverFileID != nil && *commodity.CoverFileID != "" {
		if cover, ok := s.signCover(commodity.ID, fetchExplicitCover(ctx, fileReg, commodity.ID, *commodity.CoverFileID), CoverSourceExplicit, userID); ok {
			return cover, true
		}
		// Fall through to first-photo if the override is unusable
		// (file deleted out from under us, type drifted, etc.). The
		// FK ON DELETE SET NULL keeps the row consistent on hard-deletes;
		// this branch covers the soft-delete / type-mismatch edge.
	}

	first, ok := pickFirstPhoto(ctx, fileReg, commodity.ID)
	if !ok {
		return ResolvedCover{}, false
	}
	return s.signCover(commodity.ID, first, CoverSourceFirstPhoto, userID)
}

// signCover builds a ResolvedCover from a file entity by minting signed
// thumbnail URLs. Returns ok=false on any signing failure or when the
// file is nil — both treated by callers as "skip and let the FE
// fallback render the emoji".
func (s *CommodityCoverService) signCover(commodityID string, file *models.FileEntity, source CoverSource, userID string) (ResolvedCover, bool) {
	if file == nil {
		return ResolvedCover{}, false
	}
	_, thumbnails, err := s.signing.GenerateSignedURLsWithThumbnails(file, userID)
	if err != nil || len(thumbnails) == 0 {
		return ResolvedCover{}, false
	}
	return ResolvedCover{
		CommodityID: commodityID,
		FileID:      file.ID,
		Thumbnails:  thumbnails,
		Source:      source,
	}, true
}

// fetchExplicitCover loads the file pointed at by `commodity.CoverFileID`
// and returns it only when it still belongs to this commodity and is an
// image. Anything else (not found, wrong commodity, non-image) returns
// nil so the caller can fall back to the first-photo path.
func fetchExplicitCover(ctx context.Context, fileReg registry.FileRegistry, commodityID, fileID string) *models.FileEntity {
	file, err := fileReg.Get(ctx, fileID)
	if err != nil || file == nil || file.File == nil {
		return nil
	}
	if file.LinkedEntityType != "commodity" || file.LinkedEntityID != commodityID {
		return nil
	}
	if file.Type != models.FileTypeImage {
		return nil
	}
	return file
}

// pickFirstPhoto picks the earliest `category=photos` file linked to the
// commodity. The legacy `linked_entity_meta="images"` query is used here
// because Search() filters by category but doesn't enforce the
// (commodity, images) meta — and we don't want to surface invoice-as-image
// uploads on the cover slot.
func pickFirstPhoto(ctx context.Context, fileReg registry.FileRegistry, commodityID string) (*models.FileEntity, bool) {
	files, err := fileReg.ListByLinkedEntityAndMeta(ctx, "commodity", commodityID, "images")
	if err != nil || len(files) == 0 {
		return nil, false
	}
	first := files[0]
	for _, f := range files[1:] {
		if f == nil || f.File == nil {
			continue
		}
		if first == nil || first.File == nil {
			first = f
			continue
		}
		if f.CreatedAt.Before(first.CreatedAt) {
			first = f
		}
	}
	if first == nil || first.File == nil {
		return nil, false
	}
	// Skip non-image rows — the legacy "images" bucket is officially
	// images-only but a defensive guard avoids surfacing the wrong file
	// on a card if a caller mis-classified an upload.
	if first.Type != models.FileTypeImage {
		return nil, false
	}
	return first, true
}
