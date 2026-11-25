package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/commodityquery"
)

// Limits on a bulk action on items. A request over one is refused before
// any item changes.
const (
	// MaxBulkQueryItems caps the items a query may select for a bulk action.
	// The items are changed one by one inside the request, so the cap bounds
	// how long the request runs.
	MaxBulkQueryItems = 500
	// MaxBulkTags caps each of the add and remove lists of a tag patch, the
	// same number a tag filter may name.
	MaxBulkTags = commodityquery.MaxTags
	// maxStatusNoteRunes is the length the item model allows a status note.
	maxStatusNoteRunes = 1024
)

// BulkSelection names the items of a bulk action: the ids in IDs, or the
// live items Query selects. With Query, ExpectedCount and ExpectedDigest are
// the number of items the caller saw it select and the commodityquery.Digest
// of their ids.
type BulkSelection struct {
	IDs            []string
	Query          *commodityquery.Query
	ExpectedCount  int
	ExpectedDigest string
}

// BulkSelectionChangedError reports that a query selects other items than
// the caller saw: a different number, or as many with at least one swapped.
// Acting on them would not act on what the user confirmed.
type BulkSelectionChangedError struct {
	Expected int
	Matched  int
}

func (e *BulkSelectionChangedError) Error() string {
	if e.Matched == e.Expected {
		return fmt.Sprintf("the query now selects other items than the %d the caller saw", e.Expected)
	}
	return fmt.Sprintf("the query now selects %d items, not %d", e.Matched, e.Expected)
}

// BulkTarget is one item of a bulk action. Row is the item as the selection
// read it, nil when the selection named it by id only. It names the item in
// a failure; an action reads the item again right before it writes it, so an
// edit saved after the selection was resolved is not overwritten.
type BulkTarget struct {
	ID  string
	Row *models.Commodity
}

func (t BulkTarget) name() string {
	if t.Row == nil {
		return ""
	}
	return t.Row.Name
}

// BulkTooManyError reports a query that selects more items than one bulk
// action takes.
type BulkTooManyError struct {
	Matched int
	Max     int
}

func (e *BulkTooManyError) Error() string {
	return fmt.Sprintf("the query selects %d items; a bulk action takes at most %d", e.Matched, e.Max)
}

// ResolveBulkSelection returns the items sel names, each once. A query is run
// through the item query contract (commodityquery), so it selects exactly the
// items the items list shows for it, in its order, and the targets carry the
// rows it read, for naming them. It may not select items in Trash, and it fails with
// BulkTooManyError or BulkSelectionChangedError instead of acting on a set
// the caller did not see. Ids are returned without rows.
func ResolveBulkSelection(ctx context.Context, set *registry.Set, sel BulkSelection) ([]BulkTarget, error) {
	if sel.Query == nil {
		ids := uniqueIDs(sel.IDs)
		targets := make([]BulkTarget, 0, len(ids))
		for _, id := range ids {
			targets = append(targets, BulkTarget{ID: id})
		}
		return targets, nil
	}
	if sel.Query.Trashed {
		return nil, &commodityquery.Error{
			Param:  commodityquery.ParamTrashed,
			Value:  "true",
			Reason: "bulk actions apply to live items only",
		}
	}
	page, err := commodityquery.List(ctx, set, *sel.Query, 0, MaxBulkQueryItems+1)
	if err != nil {
		return nil, err
	}
	if page.Total > MaxBulkQueryItems {
		return nil, &BulkTooManyError{Matched: page.Total, Max: MaxBulkQueryItems}
	}
	if page.Total != sel.ExpectedCount {
		return nil, &BulkSelectionChangedError{Expected: sel.ExpectedCount, Matched: page.Total}
	}
	ids := make([]string, 0, len(page.Commodities))
	targets := make([]BulkTarget, 0, len(page.Commodities))
	for _, c := range page.Commodities {
		ids = append(ids, c.ID)
		targets = append(targets, BulkTarget{ID: c.ID, Row: c})
	}
	if commodityquery.Digest(ids) != sel.ExpectedDigest {
		return nil, &BulkSelectionChangedError{Expected: sel.ExpectedCount, Matched: page.Total}
	}
	return targets, nil
}

func uniqueIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// CheckCommodityType reports whether an item may be given type t: nil, a
// validation.Error saying why not, or the error that kept it from checking.
// It is the one check of a type value outside the item model. Making the item
// types a per-group list (#2802) changes it together with
// models.CommodityType.Validate, which every item save runs, the bulk path
// included (Commodity.ValidateWithContext).
func CheckCommodityType(_ context.Context, t models.CommodityType) error {
	if !t.IsValid() {
		return validation.NewError("invalid_type", "invalid type")
	}
	return nil
}

// ValidateCommodityStatusTransition enforces the rule of a status change
// that needs the previous status, which the item model does not carry:
// leaving in_use requires a status date, and marking an item sold also a
// sale price. It returns nil when the change does not leave in_use or the
// fields are present. The rules that hold on every write (a sale price only
// when sold, a status date only when not in use) are the model's.
func ValidateCommodityStatusTransition(prev, next models.CommodityStatus, statusDate models.PDate, salePrice *decimal.Decimal) error {
	if prev != models.CommodityStatusInUse || next == models.CommodityStatusInUse {
		return nil
	}
	if statusDate == nil || string(*statusDate) == "" {
		return validation.Errors{
			"status_date": validation.NewError("status_date_required_on_transition",
				"status date is required when leaving in_use"),
		}
	}
	if next == models.CommodityStatusSold && salePrice == nil {
		return validation.Errors{
			"sale_price": validation.NewError("sale_price_required_for_sold",
				"sale price is required when marking as sold"),
		}
	}
	return nil
}

// CommodityPatchInput is a bulk patch as a caller writes it. A nil member
// leaves that part of the items unchanged.
type CommodityPatchInput struct {
	// AddTags are tag slugs or the text of new tags; RemoveTags are slugs.
	AddTags    []string
	RemoveTags []string
	Type       *string
	Status     *CommodityStatusInput
}

// CommodityStatusInput is the status part of a CommodityPatchInput.
type CommodityStatusInput struct {
	Value string
	Date  string
	Note  string
}

// CommodityPatch is a checked bulk patch, ready to apply to items.
type CommodityPatch struct {
	// AddTags and RemoveTags are tag slugs; no slug is in both.
	AddTags    []string
	RemoveTags []string
	Type       *models.CommodityType
	Status     *CommodityStatusChange
	// addInput is the add list as written, so a tag EnsureTags creates gets
	// the label the user typed.
	addInput []string
}

// CommodityStatusChange sets an item's status with the date and note of the
// change. Date is nil for in_use.
type CommodityStatusChange struct {
	Status models.CommodityStatus
	Date   models.PDate
	Note   string
}

// ParseCommodityPatch checks in and returns the patch it describes. It has
// no side effects: tags to add that do not exist yet are created by
// CommodityBulkService.EnsureTags. The error is a validation.Errors keyed by
// tags, type, status, status_date and status_note.
func ParseCommodityPatch(ctx context.Context, in CommodityPatchInput) (CommodityPatch, error) {
	var patch CommodityPatch
	errs := validation.Errors{}

	add, remove, err := parseTagLists(in.AddTags, in.RemoveTags)
	if err != nil {
		errs["tags"] = err
	}
	patch.AddTags, patch.RemoveTags, patch.addInput = add, remove, in.AddTags

	if in.Type != nil {
		t := models.CommodityType(*in.Type)
		if err := CheckCommodityType(ctx, t); err != nil {
			if _, ok := errors.AsType[validation.Error](err); !ok {
				return CommodityPatch{}, err
			}
			errs["type"] = err
		}
		patch.Type = &t
	}

	if in.Status != nil {
		change, statusErrs := parseStatusChange(*in.Status)
		maps.Copy(errs, statusErrs)
		patch.Status = change
	}

	if len(errs) > 0 {
		return CommodityPatch{}, errs
	}
	return patch, nil
}

// parseTagLists checks the add and remove lists of a tag patch and returns
// them as unique slugs.
func parseTagLists(addRaw, removeRaw []string) (add, remove []string, err error) {
	if add, err = patchSlugs(addRaw); err != nil {
		return nil, nil, err
	}
	if remove, err = patchSlugs(removeRaw); err != nil {
		return nil, nil, err
	}
	if slices.ContainsFunc(add, func(s string) bool { return slices.Contains(remove, s) }) {
		return nil, nil, validation.NewError("tag_added_and_removed", "a tag cannot be both added and removed")
	}
	return add, remove, nil
}

// applyTags removes the tags in remove from tags and appends those in add
// that are not there yet, keeping the order of the tags already present. It
// reports whether the list changed.
func applyTags[S ~[]string](tags S, add, remove []string) (S, bool) {
	out := make(S, 0, len(tags)+len(add))
	for _, t := range tags {
		if !slices.Contains(remove, t) && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	for _, t := range add {
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	if slices.Equal(out, tags) {
		return tags, false
	}
	return out, true
}

// patchSlugs normalizes a tag list of a patch into unique slugs. An entry
// that is not a tag is refused rather than dropped, so the request does not
// report success for a tag it never applied.
func patchSlugs(raw []string) ([]string, error) {
	if len(raw) > MaxBulkTags {
		return nil, validation.NewError("too_many_tags", fmt.Sprintf("at most %d tags can be added or removed at once", MaxBulkTags))
	}
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		slug := models.NormalizeTagSlug(r)
		if slug == "" || utf8.RuneCountInString(slug) > models.MaxTagSlugRunes {
			return nil, validation.NewError("invalid_tag", fmt.Sprintf("%q is not a tag", r))
		}
		if !slices.Contains(out, slug) {
			out = append(out, slug)
		}
	}
	return out, nil
}

func parseStatusChange(in CommodityStatusInput) (*CommodityStatusChange, validation.Errors) {
	status := models.CommodityStatus(in.Value)
	errs := validation.Errors{}
	switch {
	case !status.IsValid():
		errs["status"] = validation.NewError("invalid_status", "invalid status")
	case status == models.CommodityStatusSold:
		errs["status"] = validation.NewError("bulk_sold_unsupported",
			"sold needs a sale price for each item; mark items sold one at a time")
	case status == models.CommodityStatusInUse:
		if in.Date != "" {
			errs["status_date"] = validation.NewError("status_forbids_status_date",
				"status date cannot be set when status is in_use")
		}
		if in.Note != "" {
			errs["status_note"] = validation.NewError("status_forbids_status_note",
				"status note cannot be set when status is in_use")
		}
	default:
		if in.Date == "" {
			errs["status_date"] = validation.NewError("status_date_required_on_transition",
				"status date is required when leaving in_use")
		} else if _, err := time.Parse(time.DateOnly, in.Date); err != nil {
			errs["status_date"] = validation.NewError("invalid_date", "must be a date as YYYY-MM-DD")
		}
		if utf8.RuneCountInString(in.Note) > maxStatusNoteRunes {
			errs["status_note"] = validation.NewError("status_note_too_long",
				fmt.Sprintf("must be at most %d characters", maxStatusNoteRunes))
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	change := &CommodityStatusChange{Status: status, Note: in.Note}
	if in.Date != "" {
		change.Date = models.ToPDate(models.Date(in.Date))
	}
	return change, nil
}

// Apply returns c with the patch applied and whether anything changed. Tags
// are removed first and added after, keeping the order of the tags an item
// already has; a status change replaces the status date and note and clears
// the sale price, which only a sold item has. An item already in the target
// status keeps its date and note.
func (p CommodityPatch) Apply(c models.Commodity) (models.Commodity, bool) {
	changed := false
	if len(p.AddTags) > 0 || len(p.RemoveTags) > 0 {
		if tags, tagsChanged := applyTags(c.Tags, p.AddTags, p.RemoveTags); tagsChanged {
			c.Tags = tags
			changed = true
		}
	}
	if p.Type != nil && c.Type != *p.Type {
		c.Type = *p.Type
		changed = true
	}
	if p.Status != nil && c.Status != p.Status.Status {
		c.Status = p.Status.Status
		c.StatusDate = nil
		if p.Status.Date != nil {
			c.StatusDate = models.ToPDate(*p.Status.Date)
		}
		c.StatusNote = p.Status.Note
		c.SalePrice = nil
		changed = true
	}
	return c, changed
}

// BulkOutcome is the result of a bulk action, item by item. Every id of the
// action is in exactly one of Succeeded, Skipped and Failed; Notes adds to
// succeeded ids.
type BulkOutcome struct {
	Succeeded []string
	Skipped   []BulkSkip
	Failed    []BulkFailure
	Notes     []BulkNote
}

// BulkNote is something to know about an item a bulk action changed.
type BulkNote struct {
	ID   string
	Name string
	// Code is BulkNoteFilesKept.
	Code string
}

// BulkNoteFilesKept: the item went to Trash, but moving its files there
// failed, so some of them stayed live.
const BulkNoteFilesKept = "files_kept"

// BulkSkip is an item a bulk action left unchanged because it already had
// what the action sets.
type BulkSkip struct {
	ID   string
	Name string
}

// BulkFailure is an item a bulk action could not change. Name is empty when
// the item could not be read.
type BulkFailure struct {
	ID   string
	Name string
	Err  error
}

func (o *BulkOutcome) fail(id, name string, err error) {
	o.Failed = append(o.Failed, BulkFailure{ID: id, Name: name, Err: err})
}

// CommodityBulkService applies one change to many items. Each item goes
// through the checks, the write and the history event of a single change of
// the same kind; an item that fails does not stop the others, and the outcome
// says what happened to each.
type CommodityBulkService struct {
	tags   *TagService
	events *CommodityEventService
	trash  *TrashService
}

// NewCommodityBulkService builds a CommodityBulkService on the services that
// own tags, item history and Trash.
func NewCommodityBulkService(tags *TagService, events *CommodityEventService, trash *TrashService) *CommodityBulkService {
	return &CommodityBulkService{tags: tags, events: events, trash: trash}
}

// EnsureTags creates the tags patch adds that the group does not have yet,
// with the label as typed, the way saving a single item does.
func (s *CommodityBulkService) EnsureTags(ctx context.Context, patch *CommodityPatch) error {
	if len(patch.addInput) == 0 {
		return nil
	}
	slugs, err := s.tags.NormalizeAndEnsureSlugs(ctx, models.TagKindCommodity, patch.addInput)
	if err != nil {
		return err
	}
	patch.AddTags = slugs
	return nil
}

// Update applies patch to every target, reading each item right before it
// writes it. An item the patch does not change is skipped. The context must
// carry the group currency, which the item validation reads.
func (s *CommodityBulkService) Update(ctx context.Context, items registry.CommodityRegistry, targets []BulkTarget, patch CommodityPatch) BulkOutcome {
	var out BulkOutcome
	for i, t := range targets {
		if err := ctx.Err(); err != nil {
			failRest(&out, targets[i:], err)
			break
		}
		before, err := items.Get(ctx, t.ID)
		if err != nil {
			out.fail(t.ID, t.name(), err)
			continue
		}
		after, changed := patch.Apply(*before)
		if !changed {
			out.Skipped = append(out.Skipped, BulkSkip{ID: t.ID, Name: before.Name})
			continue
		}
		if err := ValidateCommodityStatusTransition(before.Status, after.Status, after.StatusDate, after.SalePrice); err != nil {
			out.fail(t.ID, before.Name, err)
			continue
		}
		if err := after.ValidateWithContext(ctx); err != nil {
			out.fail(t.ID, before.Name, err)
			continue
		}
		updated, err := items.Update(ctx, after)
		if err != nil {
			out.fail(t.ID, before.Name, err)
			continue
		}
		s.events.EmitUpdated(ctx, before, updated)
		out.Succeeded = append(out.Succeeded, t.ID)
	}
	return out
}

// Move puts every target into the area areaID, which the caller has checked
// exists, reading each item right before it writes it. An item already
// there is skipped.
func (s *CommodityBulkService) Move(ctx context.Context, items registry.CommodityRegistry, targets []BulkTarget, areaID string) BulkOutcome {
	var out BulkOutcome
	for i, t := range targets {
		if err := ctx.Err(); err != nil {
			failRest(&out, targets[i:], err)
			break
		}
		before, err := items.Get(ctx, t.ID)
		if err != nil {
			out.fail(t.ID, t.name(), err)
			continue
		}
		if before.AreaID != nil && *before.AreaID == areaID {
			out.Skipped = append(out.Skipped, BulkSkip{ID: t.ID, Name: before.Name})
			continue
		}
		after := *before
		after.AreaID = new(areaID)
		updated, err := items.Update(ctx, after)
		if err != nil {
			out.fail(t.ID, before.Name, err)
			continue
		}
		s.events.EmitUpdated(ctx, before, updated)
		out.Succeeded = append(out.Succeeded, t.ID)
	}
	return out
}

// Delete moves every target to Trash, as deleting one item does: with the
// files no other entity uses and a deleted event in its history. An item
// that went to Trash although moving its files failed is reported as
// succeeded, with a BulkNoteFilesKept note, since restoring it is how the
// user takes the delete back.
func (s *CommodityBulkService) Delete(ctx context.Context, targets []BulkTarget) BulkOutcome {
	var out BulkOutcome
	for i, t := range targets {
		if err := ctx.Err(); err != nil {
			failRest(&out, targets[i:], err)
			break
		}
		trashed, err := s.trash.TrashCommodity(ctx, t.ID)
		switch {
		case err == nil:
			out.Succeeded = append(out.Succeeded, t.ID)
		case trashed != nil:
			slog.WarnContext(ctx, "bulk delete: item went to Trash, its files did not", "commodity_id", t.ID, "err", err)
			out.Succeeded = append(out.Succeeded, t.ID)
			out.Notes = append(out.Notes, BulkNote{ID: t.ID, Name: trashed.Name, Code: BulkNoteFilesKept})
		default:
			out.fail(t.ID, t.name(), err)
		}
	}
	return out
}

func failRest(out *BulkOutcome, targets []BulkTarget, err error) {
	for _, t := range targets {
		out.fail(t.ID, t.name(), err)
	}
}
