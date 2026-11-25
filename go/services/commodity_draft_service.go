package services

import (
	"context"
	"errors"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// CommodityDraftService runs the lifecycle of a draft (#2812): create, save,
// promote to an item, discard. The files of a draft are ordinary files
// attached through file_links with entity type "draft"; FileLinkService
// attaches and detaches them.
type CommodityDraftService struct {
	factorySet *registry.FactorySet
	events     *CommodityEventService
	now        func() time.Time
}

// NewCommodityDraftService builds the service.
func NewCommodityDraftService(factorySet *registry.FactorySet) *CommodityDraftService {
	return &CommodityDraftService{
		factorySet: factorySet,
		events:     NewCommodityEventService(factorySet),
		now:        time.Now,
	}
}

func (s *CommodityDraftService) drafts(ctx context.Context) (registry.CommodityDraftRegistry, error) {
	reg, err := s.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create draft registry", err)
	}
	return reg, nil
}

// Create stores a new draft, or returns the group's draft with the same
// client key unchanged (created=false). A repeated create after a lost
// response therefore gives the client the draft it already made, never a
// second one. A key whose draft is being discarded gets
// registry.ErrDraftDiscarded.
func (s *CommodityDraftService) Create(ctx context.Context, clientKey string, values models.CommodityDraftValues) (*models.CommodityDraft, bool, error) {
	draft := models.CommodityDraft{ClientKey: clientKey, Values: values}
	if err := draft.ValidateFields(ctx); err != nil {
		return nil, false, err
	}
	reg, err := s.drafts(ctx)
	if err != nil {
		return nil, false, err
	}
	stored, created, err := reg.CreateOrGet(ctx, draft)
	if err != nil {
		return nil, false, err
	}
	if stored.IsDiscarding() {
		return nil, false, errxtrace.Classify(registry.ErrDraftDiscarded, errx.Attrs("entity_id", stored.ID))
	}
	return stored, created, nil
}

// Save replaces the values of an open draft. revision is the revision the
// client loaded; a save from a stale tab fails with
// registry.ErrDraftRevisionConflict.
func (s *CommodityDraftService) Save(ctx context.Context, id string, revision int, values models.CommodityDraftValues) (*models.CommodityDraft, error) {
	if err := models.ValidateCommodityDraftValues(values); err != nil {
		return nil, err
	}
	reg, err := s.drafts(ctx)
	if err != nil {
		return nil, err
	}
	return reg.SaveValues(ctx, id, revision, values)
}

// Files returns the live files attached to the draft, newest first.
func (s *CommodityDraftService) Files(ctx context.Context, id string) ([]*models.FileEntity, error) {
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file registry", err)
	}
	files, err := fileReg.ListByLinkedEntity(ctx, models.FileLinkDraft, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list draft files", err)
	}
	return files, nil
}

// FileCounts returns how many live files each draft has. Drafts without
// files are absent.
func (s *CommodityDraftService) FileCounts(ctx context.Context, ids []string) (map[string]int, error) {
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file registry", err)
	}
	counts, err := fileReg.CountLinkedTo(ctx, models.FileLinkDraft, ids)
	if err != nil {
		return nil, errxtrace.Wrap("failed to count draft files", err)
	}
	return counts, nil
}

// Promote turns an open draft into an item, moving its files to the item,
// and records the item's "created" event. The caller validated the item and
// normalized its tags. promotionKey identifies this promotion: a repeat with
// the same key after a lost response creates nothing and returns the item
// the draft became with created=false; another key gets
// registry.ErrDraftPromoted, as does the same key when that item no longer
// exists.
func (s *CommodityDraftService) Promote(ctx context.Context, id string, revision int, promotionKey string, commodity models.Commodity) (*models.Commodity, bool, error) {
	reg, err := s.drafts(ctx)
	if err != nil {
		return nil, false, err
	}
	result, err := reg.Promote(ctx, id, revision, promotionKey, commodity)
	if err != nil {
		return nil, false, err
	}
	if result.Commodity != nil {
		s.events.EmitCreated(ctx, result.Commodity)
		return result.Commodity, true, nil
	}
	draft := result.Draft
	if draft.PromotedCommodityID == nil {
		return nil, false, errxtrace.Classify(registry.ErrDraftPromoted, errx.Attrs("entity_id", id))
	}
	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to create commodity registry", err)
	}
	existing, err := comReg.Get(ctx, *draft.PromotedCommodityID)
	if errors.Is(err, registry.ErrNotFound) {
		return nil, false, errxtrace.Classify(registry.ErrDraftPromoted,
			errx.Attrs("entity_id", id, "commodity_id", *draft.PromotedCommodityID))
	}
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to get the promoted commodity", err)
	}
	return existing, false, nil
}

// Discard deletes an open draft. Each of its files that no live entity
// still uses goes to Trash, so it can be restored as a standalone file until
// the Trash retention runs out; a file another item, area, location or draft
// uses only loses its link to this draft. A file whose other links are to
// items in Trash goes to Trash with the first of them, so restoring that item
// brings the file back.
//
// It runs in two phases so that it cannot interleave with a promotion or an
// attachment. MarkDiscarding first takes the draft out of play under its row
// lock: from then on a promotion, a save or an attachment is refused, and a
// promotion that won the lock makes the discard fail with
// registry.ErrDraftPromoted instead. The files are then sent to Trash or
// released, each by a conditional update that a link added meanwhile
// overrides, and the row is deleted last. A discard that stopped half way
// leaves a draft being discarded, which no list shows; discarding it again
// finishes the work.
func (s *CommodityDraftService) Discard(ctx context.Context, id string) error {
	reg, err := s.drafts(ctx)
	if err != nil {
		return err
	}
	userID, err := actorID(ctx)
	if err != nil {
		return err
	}
	if _, err := reg.MarkDiscarding(ctx, id); err != nil {
		return errxtrace.Wrap("failed to begin the discard", err)
	}
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create file registry", err)
	}

	files, err := fileReg.ListByLinkedEntity(ctx, models.FileLinkDraft, id)
	if err != nil {
		return errxtrace.Wrap("failed to list draft files", err)
	}
	now := s.now().UTC()
	for _, file := range files {
		if _, err := fileReg.TrashUnlessHeld(ctx, file.ID, models.FileLinkDraft, id, userID, now); err != nil {
			return errxtrace.Wrap("failed to move draft file to Trash", err, errx.Attrs("file_id", file.ID))
		}
	}
	if _, err := fileReg.RemoveLinksToEntity(ctx, models.FileLinkDraft, id); err != nil {
		return errxtrace.Wrap("failed to remove draft file links", err)
	}
	if err := reg.DeleteDiscarding(ctx, id); err != nil {
		return errxtrace.Wrap("failed to delete draft", err)
	}
	return nil
}
