package services

import (
	"context"
	"log/slog"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// CommodityEventService writes append-only audit rows for commodity state
// changes (issue #1450). Handlers call EmitCreated / EmitUpdated /
// EmitDeleted around their CRUD operations; the service owns all the
// "what changed" diffing so a future caller (CLI, batch importer, restore
// flow) gets the same timeline shape without re-implementing the diff.
//
// Emit failures are logged but never propagated to the HTTP path: a
// successful CRUD must not 500 because the audit write hiccuped. Losing
// an event row is far less disruptive than losing the corresponding
// state change. If the loss rate ever matters, swap log for retry.
type CommodityEventService struct {
	factorySet *registry.FactorySet
}

// NewCommodityEventService binds the service to a FactorySet so it can
// build a per-request, RLS-scoped event registry on each emit.
func NewCommodityEventService(fs *registry.FactorySet) *CommodityEventService {
	return &CommodityEventService{factorySet: fs}
}

// EmitCreated records a "created" event with the initial fields snapshot.
// before is null on this kind by definition; after holds the user-relevant
// fields the timeline UI renders.
func (s *CommodityEventService) EmitCreated(ctx context.Context, after *models.Commodity) {
	if s == nil || after == nil {
		return
	}
	s.emit(ctx, after.ID, models.CommodityEventKindCreated, nil, snapshotCreated(after))
}

// EmitUpdated diffs before vs after and emits one or more events. Specific
// kinds (status_changed, moved, price_changed, cover_changed) take
// precedence over the generic "updated" kind: the timeline reads better
// when the user sees "moved Living Room → Storage Unit" rather than a
// vague "edited". If multiple aspects shifted in one write, multiple
// events emit — one per detected aspect — so the timeline stays sparse
// and per-row meaningful.
//
// When no meaningful field changed (e.g. saving the same row, refreshing
// last_modified_date with no actual edit), nothing emits.
func (s *CommodityEventService) EmitUpdated(ctx context.Context, before, after *models.Commodity) {
	if s == nil || before == nil || after == nil {
		return
	}

	emittedSpecific := false

	if before.Status != after.Status {
		s.emit(ctx, after.ID, models.CommodityEventKindStatusChanged,
			models.CommodityEventPayload{"status": string(before.Status)},
			models.CommodityEventPayload{"status": string(after.Status)},
		)
		emittedSpecific = true
	}

	if before.AreaID != after.AreaID {
		s.emit(ctx, after.ID, models.CommodityEventKindMoved,
			models.CommodityEventPayload{"area_id": before.AreaID},
			models.CommodityEventPayload{"area_id": after.AreaID},
		)
		emittedSpecific = true
	}

	if priceChanged(before, after) {
		s.emit(ctx, after.ID, models.CommodityEventKindPriceChanged,
			models.CommodityEventPayload{
				"original_price":           decimalString(before.OriginalPrice),
				"original_price_currency":  string(before.OriginalPriceCurrency),
				"converted_original_price": decimalString(before.ConvertedOriginalPrice),
				"current_price":            decimalString(before.CurrentPrice),
			},
			models.CommodityEventPayload{
				"original_price":           decimalString(after.OriginalPrice),
				"original_price_currency":  string(after.OriginalPriceCurrency),
				"converted_original_price": decimalString(after.ConvertedOriginalPrice),
				"current_price":            decimalString(after.CurrentPrice),
			},
		)
		emittedSpecific = true
	}

	if !ptrEq(before.CoverFileID, after.CoverFileID) {
		s.emit(ctx, after.ID, models.CommodityEventKindCoverChanged,
			models.CommodityEventPayload{"cover_file_id": ptrString(before.CoverFileID)},
			models.CommodityEventPayload{"cover_file_id": ptrString(after.CoverFileID)},
		)
		emittedSpecific = true
	}

	if emittedSpecific {
		return
	}

	if !genericUpdateChanged(before, after) {
		return
	}

	// Generic "updated" — emit a sparse before/after snapshot of the
	// fields the timeline UI knows how to format. The user-relevant
	// subset is a deliberate choice: storing the full row would
	// reintroduce the "log bloat" the issue calls out.
	s.emit(ctx, after.ID, models.CommodityEventKindUpdated,
		snapshotForUpdate(before),
		snapshotForUpdate(after),
	)
}

// EmitDeleted records a "deleted" event right before the hard delete.
// after is null on this kind. The row CASCADES away when the parent
// commodity is dropped — the event has audit value only within the same
// request (or for a future cross-commodity feed once the table is lifted
// to entity_events).
func (s *CommodityEventService) EmitDeleted(ctx context.Context, before *models.Commodity) {
	if s == nil || before == nil {
		return
	}
	s.emit(ctx, before.ID, models.CommodityEventKindDeleted, snapshotCreated(before), nil)
}

// EmitLoanStarted records a "lent_out" event when a new loan opens. The
// after payload carries the borrower-facing fields the timeline UI
// renders ("Lent out to X on Y, due back Z"); before is null since this
// is the first observation of the loan.
func (s *CommodityEventService) EmitLoanStarted(ctx context.Context, loan *models.CommodityLoan) {
	if s == nil || loan == nil {
		return
	}
	s.emit(ctx, loan.CommodityID, models.CommodityEventKindLentOut,
		nil,
		snapshotLoanLifecycle(loan),
	)
}

// EmitLoanReturned records a "returned" event when an open loan closes.
// after carries the returned_at + identifying fields so the timeline can
// render "Marked returned on Z" without joining back to commodity_loans.
// before is null on this kind — the loan's identity hasn't changed,
// only its terminal state, and surfacing a sparse diff would just be
// noise.
func (s *CommodityEventService) EmitLoanReturned(ctx context.Context, loan *models.CommodityLoan) {
	if s == nil || loan == nil {
		return
	}
	s.emit(ctx, loan.CommodityID, models.CommodityEventKindReturned,
		nil,
		snapshotLoanLifecycle(loan),
	)
}

// EmitLoanUpdated records a "loan_updated" event when one or more
// mutable loan fields change (borrower_contact / borrower_note /
// due_back_at). When nothing actually changed the call is a no-op —
// same gate as EmitUpdated for commodities, so idempotent PATCHes don't
// pollute the timeline.
//
// borrower_name is included in the snapshot for traceability even
// though the service rejects renames; that way the FE can render a
// human-readable "X's loan updated" line without a join.
func (s *CommodityEventService) EmitLoanUpdated(ctx context.Context, before, after *models.CommodityLoan) {
	if s == nil || before == nil || after == nil {
		return
	}
	if !loanFieldsChanged(before, after) {
		return
	}
	s.emit(ctx, after.CommodityID, models.CommodityEventKindLoanUpdated,
		snapshotLoanDiff(before),
		snapshotLoanDiff(after),
	)
}

// emit is the shared write path. Construction of the registry per-call is
// cheap (it's just a wrapper around the shared dbx) and keeps the call
// fully RLS-scoped to the current request's tenant + group + user.
func (s *CommodityEventService) emit(ctx context.Context, commodityID string, kind models.CommodityEventKind, before, after models.CommodityEventPayload) {
	reg, err := s.factorySet.CommodityEventRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		slog.WarnContext(ctx, "commodity event: failed to build registry", "err", err, "kind", kind, "commodity_id", commodityID)
		return
	}

	event := models.CommodityEvent{
		CommodityID: commodityID,
		Kind:        kind,
		OccurredAt:  time.Now().UTC(),
		Before:      before,
		After:       after,
	}
	if _, err := reg.Create(ctx, event); err != nil {
		// Wrap-and-log only — see the package-level comment on why we
		// don't propagate.
		slog.WarnContext(ctx, "commodity event: failed to write",
			"err", errxtrace.Wrap("commodity event write", err),
			"kind", kind,
			"commodity_id", commodityID,
		)
	}
}

// priceChanged reports whether any of the four price-related fields shifted.
// Decimal equality uses Decimal.Equal so 17500.00 == 17500 doesn't false-positive.
func priceChanged(before, after *models.Commodity) bool {
	return !before.OriginalPrice.Equal(after.OriginalPrice) ||
		before.OriginalPriceCurrency != after.OriginalPriceCurrency ||
		!before.ConvertedOriginalPrice.Equal(after.ConvertedOriginalPrice) ||
		!before.CurrentPrice.Equal(after.CurrentPrice)
}

// genericUpdateChanged reports whether any user-meaningful field changed
// outside the specific-kind fields already handled above. Excludes
// last_modified_date because the registry refreshes it on every write
// regardless of intent — counting it would emit "updated" events with
// no visible diff.
func genericUpdateChanged(before, after *models.Commodity) bool {
	if before.Name != after.Name ||
		before.ShortName != after.ShortName ||
		before.Type != after.Type ||
		before.Count != after.Count ||
		before.SerialNumber != after.SerialNumber ||
		before.Comments != after.Comments ||
		before.Draft != after.Draft ||
		!equalPDate(before.PurchaseDate, after.PurchaseDate) ||
		!equalPDate(before.RegisteredDate, after.RegisteredDate) {
		return true
	}
	if !sliceStringsEqual([]string(before.ExtraSerialNumbers), []string(after.ExtraSerialNumbers)) ||
		!sliceStringsEqual([]string(before.PartNumbers), []string(after.PartNumbers)) ||
		!sliceStringsEqual([]string(before.Tags), []string(after.Tags)) {
		return true
	}
	if !urlsEqual(before.URLs, after.URLs) {
		return true
	}
	return false
}

// snapshotCreated captures the fields the FE renders for a "created" or
// "deleted" event. Sparse on purpose — anything not shown in the timeline
// is dead weight in JSONB.
func snapshotCreated(c *models.Commodity) models.CommodityEventPayload {
	return models.CommodityEventPayload{
		"name":     c.Name,
		"area_id":  c.AreaID,
		"status":   string(c.Status),
		"type":     string(c.Type),
		"draft":    c.Draft,
		"count":    c.Count,
		"currency": string(c.OriginalPriceCurrency),
	}
}

// snapshotForUpdate captures the fields the timeline UI renders for a
// generic "updated" diff. The subset is deliberately narrower than
// snapshotCreated — we only persist what the FE will actually format,
// otherwise a later schema rev that changes a non-displayed field would
// trigger empty-looking "updated" rows.
func snapshotForUpdate(c *models.Commodity) models.CommodityEventPayload {
	return models.CommodityEventPayload{
		"name":         c.Name,
		"short_name":   c.ShortName,
		"type":         string(c.Type),
		"count":        c.Count,
		"comments":     c.Comments,
		"draft":        c.Draft,
		"serial":       c.SerialNumber,
		"tags":         []string(c.Tags),
		"extra_serial": []string(c.ExtraSerialNumbers),
		"part_numbers": []string(c.PartNumbers),
	}
}

// decimalString turns a decimal.Decimal into its plain text representation —
// JSONB doesn't have a native decimal type and float coercion would lose
// precision (the prices the user is auditing are exact).
func decimalString(d decimal.Decimal) string {
	return d.String()
}

// ptrEq compares two *string values for equality treating nil and "" as
// distinct (matches the BE's treatment of CoverFileID where the override
// is the difference between unset and explicitly cleared).
func ptrEq(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ptrString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// equalPDate compares two PDate pointers safely. PDate's underlying type is
// a string, so direct comparison after dereference works.
func equalPDate(a, b models.PDate) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return string(*a) == string(*b)
}

func sliceStringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// snapshotLoanLifecycle captures the loan fields the timeline renders
// for `lent_out` and `returned` events. Sparse on purpose — the FE only
// formats the listed keys, and storing more would invite a schema-rev
// trap (an unused key sticks in JSONB forever). Empty-string optionals
// are dropped so the JSON object stays minimal.
func snapshotLoanLifecycle(l *models.CommodityLoan) models.CommodityEventPayload {
	p := models.CommodityEventPayload{
		"loan_id":       l.ID,
		"borrower_name": l.BorrowerName,
		"lent_at":       string(l.LentAt),
	}
	if l.BorrowerContact != "" {
		p["borrower_contact"] = l.BorrowerContact
	}
	if l.BorrowerNote != "" {
		p["borrower_note"] = l.BorrowerNote
	}
	if l.DueBackAt != nil && *l.DueBackAt != "" {
		p["due_back_at"] = string(*l.DueBackAt)
	}
	if l.ReturnedAt != nil && *l.ReturnedAt != "" {
		p["returned_at"] = string(*l.ReturnedAt)
	}
	return p
}

// snapshotLoanDiff captures the fields the timeline renders for a
// `loan_updated` diff. borrower_name is preserved so the FE can build
// "X's loan updated" copy without a separate join, even though the
// service rejects rename mutations.
func snapshotLoanDiff(l *models.CommodityLoan) models.CommodityEventPayload {
	p := models.CommodityEventPayload{
		"loan_id":          l.ID,
		"borrower_name":    l.BorrowerName,
		"borrower_contact": l.BorrowerContact,
		"borrower_note":    l.BorrowerNote,
	}
	if l.DueBackAt != nil {
		p["due_back_at"] = string(*l.DueBackAt)
	} else {
		p["due_back_at"] = ""
	}
	return p
}

// loanFieldsChanged reports whether any of the mutable loan fields
// shifted between before and after. Mirrors the EmitUpdated diff gate
// for commodities — saves a no-op PATCH from polluting the timeline.
func loanFieldsChanged(before, after *models.CommodityLoan) bool {
	if before.BorrowerName != after.BorrowerName ||
		before.BorrowerContact != after.BorrowerContact ||
		before.BorrowerNote != after.BorrowerNote {
		return true
	}
	return !equalPDate(before.DueBackAt, after.DueBackAt)
}

// urlsEqual compares two URL slices by their string forms — the order
// matters because the user controls it via the form.
func urlsEqual(a, b models.ValuerSlice[*models.URL]) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] == nil && b[i] == nil {
			continue
		}
		if a[i] == nil || b[i] == nil {
			return false
		}
		if a[i].String() != b[i].String() {
			return false
		}
	}
	return true
}
