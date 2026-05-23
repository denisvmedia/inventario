package services

import (
	"context"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// CoverSource enumerates the resolved cover photo's provenance.
// FirstPhoto = the auto-pick path (issue #1451 option A). Explicit = the
// `commodities.cover_file_id` override (option B). The FE renders both
// the same way; the value is wire-level metadata so the star-toggle UI
// can distinguish "user-pinned this" from "first photo by default".
type CoverSource string

const (
	CoverSourceFirstPhoto CoverSource = "first_photo"
	CoverSourceExplicit   CoverSource = "explicit"
)

// ResolvedCover is the cover image picked for a single commodity, paired
// with its file id and the signed thumbnail URLs the FE renders. Source
// names which path produced it so the FE can light up the right star
// state ("auto" vs "explicit").
type ResolvedCover struct {
	CommodityID string
	FileID      string
	Thumbnails  map[string]string
	Source      CoverSource
}

// CommodityCoverService resolves the cover image for one or more
// commodities. Resolution order:
//
//  1. Explicit `commodities.cover_file_id` (option B) — when set and the
//     pointed-at file is still cover-eligible (linked to this commodity,
//     `Type=image`, `Category=images`).
//  2. Earliest cover-eligible file by `created_at` ASC under
//     `linked_entity_type=commodity` / `linked_entity_meta=images`
//     (option A).
//
// A stale or no-longer-eligible explicit override falls through to (2)
// rather than blanking out the slot, so a deleted-image race never
// flashes the emoji fallback while the user is mid-flow.
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
// unverifiable. `binding` couples the produced URLs to the caller's
// browser session (see ExtractSessionBinding).
func (s *CommodityCoverService) ResolveMany(ctx context.Context, fileReg registry.FileRegistry, commodities []*models.Commodity, userID string, binding SessionBinding) map[string]ResolvedCover {
	out := make(map[string]ResolvedCover, len(commodities))
	if userID == "" || fileReg == nil || len(commodities) == 0 {
		return out
	}

	for _, c := range commodities {
		if c == nil || c.ID == "" {
			continue
		}
		cover, ok := s.resolveOne(ctx, fileReg, c, userID, binding)
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
func (s *CommodityCoverService) ResolveOne(ctx context.Context, fileReg registry.FileRegistry, commodity *models.Commodity, userID string, binding SessionBinding) (ResolvedCover, bool) {
	if userID == "" || fileReg == nil || commodity == nil || commodity.ID == "" {
		return ResolvedCover{}, false
	}
	return s.resolveOne(ctx, fileReg, commodity, userID, binding)
}

func (s *CommodityCoverService) resolveOne(ctx context.Context, fileReg registry.FileRegistry, commodity *models.Commodity, userID string, binding SessionBinding) (ResolvedCover, bool) {
	if commodity.CoverFileID != nil && *commodity.CoverFileID != "" {
		if cover, ok := s.signCover(commodity.ID, fetchExplicitCover(ctx, fileReg, commodity.ID, *commodity.CoverFileID), CoverSourceExplicit, userID, binding); ok {
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
	return s.signCover(commodity.ID, first, CoverSourceFirstPhoto, userID, binding)
}

// signCover builds a ResolvedCover from a file entity by minting signed
// thumbnail URLs. Returns ok=false on any signing failure or when the
// file is nil — both treated by callers as "skip and let the FE
// fallback render the emoji".
func (s *CommodityCoverService) signCover(commodityID string, file *models.FileEntity, source CoverSource, userID string, binding SessionBinding) (ResolvedCover, bool) {
	if file == nil {
		return ResolvedCover{}, false
	}
	_, thumbnails, err := s.signing.GenerateSignedURLsWithThumbnails(file, userID, binding)
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

// isCoverEligible enforces the same write-side / read-side invariant the
// PATCH endpoint applies: a cover photo is a `linked_entity_type=commodity`
// row that's BOTH `Type=image` (MIME-derived behaviour gate — drives
// thumbnail generation) AND categorised as `images` (the user-meaningful
// bucket). Tightened from a Type-only check after the Copilot review on
// #1504 — a JPEG mis-uploaded as `category=invoices` would otherwise
// sneak past as a cover.
func isCoverEligible(file *models.FileEntity) bool {
	if file == nil || file.File == nil {
		return false
	}
	if file.Type != models.FileTypeImage {
		return false
	}
	return file.Category == models.FileCategoryImages
}

// fetchExplicitCover loads the file pointed at by `commodity.CoverFileID`
// and returns it only when it still belongs to this commodity and is a
// usable cover photo (see isCoverEligible). Anything else (not found,
// wrong commodity, non-image, non-images category) returns nil so the
// caller can fall back to the first-photo path.
func fetchExplicitCover(ctx context.Context, fileReg registry.FileRegistry, commodityID, fileID string) *models.FileEntity {
	file, err := fileReg.Get(ctx, fileID)
	if err != nil || file == nil {
		return nil
	}
	if file.LinkedEntityType != "commodity" || file.LinkedEntityID != commodityID {
		return nil
	}
	if !isCoverEligible(file) {
		return nil
	}
	return file
}

// pickFirstPhoto picks the earliest cover-eligible file linked to the
// commodity. The legacy `linked_entity_meta="images"` bucket scopes the
// query (officially images-only); isCoverEligible enforces the actual
// invariant in case a caller mis-classified an upload. Filtering happens
// BEFORE the earliest-by-CreatedAt pick so a non-image row at the top of
// the bucket can't shadow a valid photo uploaded a second later.
func pickFirstPhoto(ctx context.Context, fileReg registry.FileRegistry, commodityID string) (*models.FileEntity, bool) {
	files, err := fileReg.ListByLinkedEntityAndMeta(ctx, "commodity", commodityID, "images")
	if err != nil || len(files) == 0 {
		return nil, false
	}
	var first *models.FileEntity
	for _, f := range files {
		if !isCoverEligible(f) {
			continue
		}
		if first == nil || f.CreatedAt.Before(first.CreatedAt) {
			first = f
		}
	}
	if first == nil {
		return nil, false
	}
	return first, true
}
