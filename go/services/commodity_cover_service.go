package services

import (
	"context"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
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
//  2. Earliest cover-eligible file by `created_at` ASC among the files
//     linked to the commodity in its "images" bucket (option A).
//
// A file can be linked to several items, and each item resolves its cover
// through its own link: removing one item's link leaves the cover of every
// other item untouched.
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
// slice. Commodities without a usable photo, and items in Trash, are absent
// from the returned map (no zero-value entries). Read errors are swallowed
// so the list endpoint never 500s on a partial failure — the FE already
// handles the missing-cover fallback.
//
// Resolution order per commodity (issue #1451):
//
//  1. Explicit override — `commodity.CoverFileID` (option B). The file
//     must still exist, belong to the commodity, and be an image; on
//     any failure the path falls through to (2) so a stale override
//     never blocks the auto-pick.
//  2. First photo by `created_at` ASC (option A — the default).
//
// The whole slice costs at most two reads: one for the explicit covers and
// one for the first photos of the items still without a cover.
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

	live := make([]*models.Commodity, 0, len(commodities))
	var coverIDs []string
	for _, c := range commodities {
		if c == nil || c.ID == "" || c.IsTrashed() {
			continue
		}
		live = append(live, c)
		if id := explicitCoverID(c); id != "" {
			coverIDs = append(coverIDs, id)
		}
	}

	explicit := fetchExplicitCovers(ctx, fileReg, coverIDs)
	needFirst := make([]string, 0, len(live))
	for _, c := range live {
		if id := explicitCoverID(c); id != "" {
			if cover, ok := s.signCover(c.ID, explicitCoverFor(explicit[id], c.ID), CoverSourceExplicit, userID, binding); ok {
				out[c.ID] = cover
				continue
			}
			// Fall through to first-photo if the override is unusable
			// (file deleted out from under us, type drifted, etc.). The
			// FK ON DELETE SET NULL keeps the row consistent on
			// hard-deletes; this branch covers the soft-delete /
			// type-mismatch edge.
		}
		needFirst = append(needFirst, c.ID)
	}
	if len(needFirst) == 0 {
		return out
	}

	first, err := fileReg.FirstPhotos(ctx, needFirst)
	if err != nil {
		return out
	}
	for _, id := range needFirst {
		file := first[id]
		if !isCoverEligible(file) {
			continue
		}
		if cover, ok := s.signCover(id, file, CoverSourceFirstPhoto, userID, binding); ok {
			out[id] = cover
		}
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
	cover, ok := s.ResolveMany(ctx, fileReg, []*models.Commodity{commodity}, userID, binding)[commodity.ID]
	return cover, ok
}

// explicitCoverID returns the file id the item names as its cover, or "".
func explicitCoverID(c *models.Commodity) string {
	if c.CoverFileID == nil {
		return ""
	}
	return *c.CoverFileID
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
// PATCH endpoint applies: a cover photo is a file linked to the commodity
// that's BOTH `Type=image` (MIME-derived behaviour gate — drives
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

// fetchExplicitCovers loads the files the items name as their covers in one
// read, keyed by file id. A failed read returns an empty map, which sends
// every item to the first-photo path.
func fetchExplicitCovers(ctx context.Context, fileReg registry.FileRegistry, fileIDs []string) map[string]*models.FileEntity {
	out := make(map[string]*models.FileEntity, len(fileIDs))
	if len(fileIDs) == 0 {
		return out
	}
	files, err := fileReg.GetMany(ctx, fileIDs)
	if err != nil {
		return out
	}
	for _, f := range files {
		if f != nil {
			out[f.ID] = f
		}
	}
	return out
}

// explicitCoverFor returns the file an item names as its cover only when it
// is still attached to this commodity among its photos (link meta images,
// the attachments the No photo filter counts too) and is a usable cover
// photo (see isCoverEligible). Anything else (not found, wrong commodity,
// another bucket, non-image, non-images category) returns nil so the caller
// can fall back to the first-photo path.
func explicitCoverFor(file *models.FileEntity, commodityID string) *models.FileEntity {
	if file == nil {
		return nil
	}
	link := file.LinkTo(models.FileLinkCommodity, commodityID)
	if link == nil || link.Meta != models.FileLinkMetaImages {
		return nil
	}
	if !isCoverEligible(file) {
		return nil
	}
	return file
}
