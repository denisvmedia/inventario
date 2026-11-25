package services

import (
	"cmp"
	"context"
	"slices"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// InboxKind is the kind of record an inbox entry stands for. Together with
// the record's id it identifies the entry: two sources that flag the same
// record give one entry.
type InboxKind string

const (
	// InboxKindDraft is a server draft of a new item (#2812).
	InboxKindDraft InboxKind = "draft"
	// InboxKindCommodity is an item.
	InboxKindCommodity InboxKind = "commodity"
)

// InboxReason says why a record needs attention. A record can have several.
type InboxReason string

const (
	// InboxReasonDraft is an unfinished new item saved as a draft: it is not
	// an item until someone finishes it.
	InboxReasonDraft InboxReason = "draft"
	// InboxReasonItemDraft is an item marked as draft, which means its review
	// is not complete. Clearing the mark completes the review, also when the
	// item's purchase data stays unknown.
	InboxReasonItemDraft InboxReason = "item_draft"
)

// inboxReasonOrder lists the reasons in the order the inbox shows them. An
// entry is placed by its first reason, so the drafts come before the items
// that wait for review.
var inboxReasonOrder = []InboxReason{InboxReasonDraft, InboxReasonItemDraft}

// InboxReasons returns every reason, in display order.
func InboxReasons() []InboxReason {
	return slices.Clone(inboxReasonOrder)
}

// Valid reports whether r is a reason the inbox knows.
func (r InboxReason) Valid() bool {
	return slices.Contains(inboxReasonOrder, r)
}

// rank is r's place in inboxReasonOrder; a reason the order does not name
// goes last.
func (r InboxReason) rank() int {
	if i := slices.Index(inboxReasonOrder, r); i >= 0 {
		return i
	}
	return len(inboxReasonOrder)
}

// InboxEntry is one record that needs attention, with every reason it does.
type InboxEntry struct {
	Kind  InboxKind
	ID    string
	Title string
	// Reasons is never empty and holds each reason once, in display order.
	Reasons []InboxReason
	// UpdatedAt is when the record was last saved, when its source knows.
	UpdatedAt *time.Time
	// FileCount is the number of live files attached to a draft. Only the
	// entries of a page carry it (see InboxDetailer).
	FileCount int
}

// InboxSource finds one kind of unfinished work in the group of a registry
// set. A new kind of work (an intake session, an extraction waiting for
// review) is a new source; the service merges what the sources return.
type InboxSource interface {
	// Collect returns every record of the group that the source flags, in
	// the order the source lists them, with at least one reason each. A
	// record another source flags too is named by the same kind and id, so
	// the two become one entry.
	Collect(ctx context.Context, regSet *registry.Set) ([]InboxEntry, error)
}

// InboxDetailer is implemented by a source whose entries carry details that
// only a page of them needs, such as the file count of a draft. Detail fills
// them in on the entries of its kind and leaves the others alone.
type InboxDetailer interface {
	Detail(ctx context.Context, regSet *registry.Set, entries []*InboxEntry) error
}

// InboxCounts is how many records the inbox holds. Total counts each record
// once, whatever its number of reasons; Reasons counts, for each reason, the
// records that have it, so the reasons can add up to more than Total.
type InboxCounts struct {
	Total   int
	Reasons map[InboxReason]int
}

// InboxListOptions selects a page of the inbox.
type InboxListOptions struct {
	// Reason keeps the entries that have it; empty keeps every entry.
	Reason InboxReason
	Offset int
	Limit  int
}

// InboxPage is a page of the inbox.
type InboxPage struct {
	Entries []InboxEntry
	// Total is the number of entries the reason filter keeps.
	Total int
	// Counts describes the whole inbox, without the reason filter.
	Counts InboxCounts
}

// InboxService builds the inbox of a group (#2794): one list of the records
// that still need someone's attention, from every source of unfinished work.
// It only reads; each entry is cleared through the record's own endpoints.
//
// The inbox holds unfinished work, which people clear, so a source reads its
// records in full and the counts are exact rather than estimated from a page.
type InboxService struct {
	sources []InboxSource
}

// NewInboxService builds the service over sources. With none it uses
// DefaultInboxSources.
func NewInboxService(sources ...InboxSource) *InboxService {
	if len(sources) == 0 {
		sources = DefaultInboxSources()
	}
	return &InboxService{sources: sources}
}

// DefaultInboxSources returns the sources of the default inbox: the open
// server drafts and the items marked as draft. Missing information (no
// location, no photo, an unknown price) is not unfinished work and has no
// source here.
func DefaultInboxSources() []InboxSource {
	return []InboxSource{DraftInboxSource{}, ItemDraftInboxSource{}}
}

// Count returns the counts of the inbox of regSet's group.
func (s *InboxService) Count(ctx context.Context, regSet *registry.Set) (InboxCounts, error) {
	entries, err := s.collect(ctx, regSet)
	if err != nil {
		return InboxCounts{}, err
	}
	return countInbox(entries), nil
}

// List returns a page of the inbox of regSet's group.
func (s *InboxService) List(ctx context.Context, regSet *registry.Set, opts InboxListOptions) (InboxPage, error) {
	entries, err := s.collect(ctx, regSet)
	if err != nil {
		return InboxPage{}, err
	}
	page := InboxPage{Counts: countInbox(entries)}

	kept := entries
	if opts.Reason != "" {
		kept = slices.DeleteFunc(slices.Clone(entries), func(e InboxEntry) bool {
			return !slices.Contains(e.Reasons, opts.Reason)
		})
	}
	page.Total = len(kept)
	start := min(max(opts.Offset, 0), len(kept))
	end := len(kept)
	if opts.Limit > 0 {
		end = min(start+opts.Limit, len(kept))
	}
	page.Entries = slices.Clone(kept[start:end])

	refs := make([]*InboxEntry, len(page.Entries))
	for i := range page.Entries {
		refs[i] = &page.Entries[i]
	}
	for _, src := range s.sources {
		d, ok := src.(InboxDetailer)
		if !ok {
			continue
		}
		if err := d.Detail(ctx, regSet, refs); err != nil {
			return InboxPage{}, err
		}
	}
	return page, nil
}

type inboxKey struct {
	kind InboxKind
	id   string
}

// collect merges the sources' records into one entry per record and orders
// the entries by their first reason, keeping the sources' order within a
// reason.
func (s *InboxService) collect(ctx context.Context, regSet *registry.Set) ([]InboxEntry, error) {
	if regSet == nil {
		return nil, errxtrace.Wrap("inbox: registry set is required", registry.ErrFieldRequired)
	}
	index := map[inboxKey]int{}
	var merged []InboxEntry
	for _, src := range s.sources {
		entries, err := src.Collect(ctx, regSet)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if len(e.Reasons) == 0 {
				continue
			}
			key := inboxKey{kind: e.Kind, id: e.ID}
			if i, ok := index[key]; ok {
				merged[i] = mergeInboxEntries(merged[i], e)
				continue
			}
			e.Reasons = orderInboxReasons(e.Reasons)
			index[key] = len(merged)
			merged = append(merged, e)
		}
	}
	slices.SortStableFunc(merged, func(a, b InboxEntry) int {
		return cmp.Compare(a.Reasons[0].rank(), b.Reasons[0].rank())
	})
	return merged, nil
}

// mergeInboxEntries joins two sources' entries for the same record.
func mergeInboxEntries(a, b InboxEntry) InboxEntry {
	a.Reasons = orderInboxReasons(append(slices.Clone(a.Reasons), b.Reasons...))
	if a.Title == "" {
		a.Title = b.Title
	}
	if b.UpdatedAt != nil && (a.UpdatedAt == nil || b.UpdatedAt.After(*a.UpdatedAt)) {
		a.UpdatedAt = b.UpdatedAt
	}
	a.FileCount = max(a.FileCount, b.FileCount)
	return a
}

// orderInboxReasons drops repeated reasons and sorts the rest into display
// order; reasons the order does not know keep their order after the known
// ones.
func orderInboxReasons(reasons []InboxReason) []InboxReason {
	out := make([]InboxReason, 0, len(reasons))
	for _, r := range reasons {
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	slices.SortStableFunc(out, func(a, b InboxReason) int { return cmp.Compare(a.rank(), b.rank()) })
	return out
}

func countInbox(entries []InboxEntry) InboxCounts {
	counts := InboxCounts{Total: len(entries), Reasons: map[InboxReason]int{}}
	for _, e := range entries {
		for _, r := range e.Reasons {
			counts.Reasons[r]++
		}
	}
	return counts
}

// inboxReadPage is how many records a source reads per registry call.
const inboxReadPage = 500

// DraftInboxSource flags every open server draft of the group (#2812). A
// draft being discarded or already promoted is not open, so it is not in the
// inbox.
type DraftInboxSource struct{}

// Collect returns the open drafts, most recently saved first.
func (DraftInboxSource) Collect(ctx context.Context, regSet *registry.Set) ([]InboxEntry, error) {
	var out []InboxEntry
	for offset := 0; ; {
		drafts, total, err := regSet.CommodityDraftRegistry.ListOpen(ctx, offset, inboxReadPage)
		if err != nil {
			return nil, errxtrace.Wrap("inbox: list drafts", err)
		}
		for _, d := range drafts {
			out = append(out, InboxEntry{
				Kind:      InboxKindDraft,
				ID:        d.ID,
				Title:     d.Values.Title(),
				Reasons:   []InboxReason{InboxReasonDraft},
				UpdatedAt: new(d.UpdatedAt),
			})
		}
		offset += len(drafts)
		if len(drafts) == 0 || offset >= total {
			return out, nil
		}
	}
}

// Detail fills in the file count of the drafts among entries, in one read.
func (DraftInboxSource) Detail(ctx context.Context, regSet *registry.Set, entries []*InboxEntry) error {
	var ids []string
	for _, e := range entries {
		if e.Kind == InboxKindDraft {
			ids = append(ids, e.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	counts, err := regSet.FileRegistry.CountLinkedTo(ctx, models.FileLinkDraft, ids)
	if err != nil {
		return errxtrace.Wrap("inbox: count draft files", err)
	}
	for _, e := range entries {
		if e.Kind == InboxKindDraft {
			e.FileCount = counts[e.ID]
		}
	}
	return nil
}

// ItemDraftInboxSource flags every live item of the group that is marked as
// draft, whatever its status. Items in Trash are left out.
type ItemDraftInboxSource struct{}

// Collect returns the items marked as draft, the most recently added first
// (by asset number, which the registry gives out in creation order).
func (ItemDraftInboxSource) Collect(ctx context.Context, regSet *registry.Set) ([]InboxEntry, error) {
	opts := registry.CommodityListOptions{IncludeInactive: true, DraftsOnly: true}
	var items []*models.Commodity
	for offset := 0; ; {
		page, total, err := regSet.CommodityRegistry.ListPaginated(ctx, offset, inboxReadPage, opts)
		if err != nil {
			return nil, errxtrace.Wrap("inbox: list draft items", err)
		}
		items = append(items, page...)
		offset += len(page)
		if len(page) == 0 || offset >= total {
			break
		}
	}
	slices.SortStableFunc(items, func(a, b *models.Commodity) int {
		switch {
		case a.AssetNumber != nil && b.AssetNumber != nil && *a.AssetNumber != *b.AssetNumber:
			return cmp.Compare(*b.AssetNumber, *a.AssetNumber)
		case a.AssetNumber != nil && b.AssetNumber == nil:
			return -1
		case a.AssetNumber == nil && b.AssetNumber != nil:
			return 1
		}
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
	out := make([]InboxEntry, 0, len(items))
	for _, c := range items {
		out = append(out, InboxEntry{
			Kind:    InboxKindCommodity,
			ID:      c.ID,
			Title:   c.Name,
			Reasons: []InboxReason{InboxReasonItemDraft},
		})
	}
	return out, nil
}
