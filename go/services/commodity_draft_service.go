package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jellydator/validation"

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

// staleDiscardAfter is how long a discard may run before FinishStaleDiscards
// takes it as stopped. A discard takes seconds; it stops only when the server
// dies or the database fails half way.
const staleDiscardAfter = 10 * time.Minute

// staleDiscardBatch is how many stopped discards one FinishStaleDiscards call
// finishes at most.
const staleDiscardBatch = 10

// CommodityDraftServiceOption customizes a CommodityDraftService.
type CommodityDraftServiceOption func(*CommodityDraftService)

// WithCommodityDraftClock replaces the wall clock. Test-only.
func WithCommodityDraftClock(fn func() time.Time) CommodityDraftServiceOption {
	return func(s *CommodityDraftService) {
		if fn != nil {
			s.now = fn
		}
	}
}

// NewCommodityDraftService builds the service.
func NewCommodityDraftService(factorySet *registry.FactorySet, opts ...CommodityDraftServiceOption) *CommodityDraftService {
	s := &CommodityDraftService{
		factorySet: factorySet,
		events:     NewCommodityEventService(factorySet),
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
// second one, even when the type in its values was deleted since: the type
// is checked for a new draft only. A key whose draft is being discarded
// gets registry.ErrDraftDiscarded.
func (s *CommodityDraftService) Create(ctx context.Context, clientKey string, values models.CommodityDraftValues) (*models.CommodityDraft, bool, error) {
	draft := models.CommodityDraft{ClientKey: clientKey, Values: values}
	if err := draft.ValidateFields(ctx); err != nil {
		return nil, false, err
	}
	reg, err := s.drafts(ctx)
	if err != nil {
		return nil, false, err
	}
	existing, err := reg.GetByClientKey(ctx, clientKey)
	switch {
	case err == nil:
		if existing.IsDiscarding() {
			return nil, false, errxtrace.Classify(registry.ErrDraftDiscarded, errx.Attrs("entity_id", existing.ID))
		}
		return existing, false, nil
	case !errors.Is(err, registry.ErrNotFound):
		return nil, false, err
	}
	if err := s.checkValuesType(ctx, values); err != nil {
		return nil, false, err
	}
	// A concurrent create with the same key can still win between the
	// lookup and the insert; CreateOrGet then returns its draft.
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
	if err := s.checkValuesType(ctx, values); err != nil {
		return nil, err
	}
	reg, err := s.drafts(ctx)
	if err != nil {
		return nil, err
	}
	return reg.SaveValues(ctx, id, revision, values)
}

// checkValuesType checks that the type the values name, when they name one,
// is one of the group's item types (#2802), as an item's type must be. An
// unknown type is refused under values.type with ItemTypeUnknownCode, so the
// form can show it on its type field. The check is not held until the save:
// a type deleted after it leaves the draft naming a type the group no longer
// has, which the next save or the promotion refuses.
func (s *CommodityDraftService) checkValuesType(ctx context.Context, values models.CommodityDraftValues) error {
	t, _ := values["type"].(string)
	if t == "" {
		return nil
	}
	types, err := s.factorySet.ItemTypeRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create item type registry", err)
	}
	if err := CheckCommodityType(ctx, types, models.CommodityType(t)); err != nil {
		if verr, ok := errors.AsType[validation.Error](err); ok {
			return validation.Errors{"values": validation.Errors{"type": verr}}
		}
		return err
	}
	return nil
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
// normalized its tags. The item's type must be one of the group's item
// types: the registry checks it inside the promotion, seeding the built-in
// types first, and refuses an unknown one with registry.ErrUnknownItemType. promotionKey identifies this promotion: a repeat with
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
// finishes the work, and so does FinishStaleDiscards. The caller should pass
// a context the client cannot cancel (context.WithoutCancel), so a client
// that goes away does not stop the discard half way.
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

// FinishStaleDiscards finishes the discards in the group that began more than
// staleDiscardAfter ago and did not complete, at most staleDiscardBatch of
// them, and returns how many it finished. Until then such a draft keeps its
// files and its client key. A draft another discard finishes meanwhile
// counts as finished. A draft that fails is logged and the others are still
// finished, so one draft that keeps failing does not hold them up; the first
// failure is returned.
func (s *CommodityDraftService) FinishStaleDiscards(ctx context.Context) (int, error) {
	reg, err := s.drafts(ctx)
	if err != nil {
		return 0, err
	}
	stale, err := reg.ListDiscarding(ctx, s.now().Add(-staleDiscardAfter), staleDiscardBatch)
	if err != nil {
		return 0, errxtrace.Wrap("failed to list stopped discards", err)
	}
	finished := 0
	var firstErr error
	for _, draft := range stale {
		err := s.Discard(ctx, draft.ID)
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			slog.Error("Failed to finish a stopped draft discard", "draft_id", draft.ID, "error", err)
			if firstErr == nil {
				firstErr = errxtrace.Wrap("failed to finish a stopped discard", err, errx.Attrs("draft_id", draft.ID))
			}
			continue
		}
		finished++
	}
	return finished, firstErr
}
