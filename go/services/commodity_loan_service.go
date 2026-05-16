package services

import (
	"context"
	"errors"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ErrLoanAlreadyOpen / ErrLoanAlreadyReturned alias the registry
// sentinels so apiserver code can compare against the services package
// without an extra import. Mirrors the ErrTagInUse pattern.
var (
	ErrLoanAlreadyOpen     = registry.ErrLoanAlreadyOpen
	ErrLoanAlreadyReturned = registry.ErrLoanAlreadyReturned
)

// ErrClosedLoanFieldImmutable signals that a PATCH targeted a field on
// a closed (returned) loan that is intentionally frozen for audit
// clarity. Issue #1511: borrower name/contact/note remain editable on
// closed loans (typo fixes, retrospective notes) but the date-of-record
// fields (due_back_at / returned_at) do not — once the loan is over
// those dates are "what actually happened" and editing them muddies the
// audit trail. Apiserver maps this sentinel to 422.
var ErrClosedLoanFieldImmutable = errx.NewSentinel("closed loan field is immutable")

// CommodityLoanService coordinates the lend-out lifecycle on top of the
// per-row CommodityLoanRegistry. Invariants enforced here (rather than
// at the SQL layer) so that the FE always sees a domain 409 instead of
// a Postgres uniqueness violation:
//
//   - At most one OPEN loan per commodity. Create rejects a second
//     concurrent open with ErrLoanAlreadyOpen + the existing loan.
//   - Return is one-shot. Marking a returned loan as returned again
//     yields ErrLoanAlreadyReturned (FE refresh + button hiding fixes
//     the UX; we deliberately don't paper over it with idempotency).
//
// Concurrency: there is no advisory lock for "at most one open" — two
// simultaneous POSTs racing on the same commodity could both observe no
// open row and both INSERT. We accept that gap because (1) the lend
// flow is a deliberate human action (no automation hammers it), (2)
// the second of two quick clicks is almost always a UX-level
// double-submit which the FE button-disable already prevents, and (3)
// the cure (a DB-level partial unique index keyed on commodity_id +
// `returned_at IS NULL`, plus its uniqueness violation translation) is
// disproportionately complex for the realistic risk. If the loan
// surface ever becomes API-driven (mobile app polling a "lend out"
// endpoint, agentic flow), revisit this.
type CommodityLoanService struct {
	factorySet *registry.FactorySet
	// eventService writes lent_out / returned / loan_updated audit
	// events into the per-commodity timeline (#1507). The event service
	// is best-effort internally — a failed event write logs but does not
	// roll back the loan operation.
	eventService *CommodityEventService
	// holdingChecker enforces the cross-kind invariant added by #1508:
	// a commodity cannot be lent out and in service simultaneously.
	holdingChecker *OpenHoldingChecker
}

func NewCommodityLoanService(factorySet *registry.FactorySet) *CommodityLoanService {
	return &CommodityLoanService{
		factorySet:     factorySet,
		eventService:   NewCommodityEventService(factorySet),
		holdingChecker: NewOpenHoldingChecker(factorySet),
	}
}

// StartLoan records a new open loan for the commodity.
//
// Returns:
//   - `created` — the newly persisted loan on success;
//   - `existing` — populated alongside ErrLoanAlreadyOpen when the
//     commodity already has an open loan, so the apiserver layer can
//     render a 409 that names the offending loan ("already lent to X
//     on Y") instead of stacking duplicates;
//   - `crossHolding` — populated alongside ErrCommodityAlreadyOut when
//     the commodity has an open SERVICE row (#1508 cross-kind
//     invariant). Mutually exclusive with `existing`.
//   - `err` — any other failure path, including registry / RLS /
//     postgres errors.
//
// `created`, `existing`, and `crossHolding` are mutually exclusive —
// exactly one is non-nil on a non-error response.
//
//revive:disable-next-line:function-result-limit // (created, existing, crossHolding, err) — collapsing existing+crossHolding loses the typed handler access, and a struct return makes the call sites worse.
func (s *CommodityLoanService) StartLoan(ctx context.Context, loan models.CommodityLoan) (created, existing *models.CommodityLoan, crossHolding *OpenHolding, err error) {
	loanReg, err := s.factorySet.CommodityLoanRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, nil, nil, errxtrace.Wrap("failed to create loan registry", err)
	}

	// #1554: bundles (Count > 1) cannot be lent — the row models a bag
	// of interchangeable units, not a single instance with a borrower.
	// Pre-fetching the commodity here is intentional: it costs one
	// extra round-trip but the alternative (translating a postgres
	// CHECK violation) would require a constraint we deliberately
	// don't ship (legacy rows are left alone per the issue's migration
	// policy).
	if err := EnsureCommodityTrackable(ctx, s.factorySet, loan.CommodityID); err != nil {
		return nil, nil, nil, err
	}

	// Guard against a second open loan on the same commodity. We do
	// not pre-validate that the commodity exists / is in this group —
	// RLS on commodity_loans + the FK on commodity_id (ON DELETE
	// CASCADE) handle that path: a non-existent / cross-group commodity
	// id surfaces as a postgres FK violation that translates to 4xx via
	// the standard renderEntityError path. Leaving the check out here
	// avoids an extra round-trip on the happy path.
	existing, err = loanReg.GetOpenForCommodity(ctx, loan.CommodityID)
	if err == nil && existing != nil {
		return nil, existing, nil, errxtrace.Wrap("commodity already has an open loan", ErrLoanAlreadyOpen)
	}
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		return nil, nil, nil, errxtrace.Wrap("failed to check existing open loan", err)
	}

	// Cross-kind invariant (#1508): a commodity cannot be lent out and
	// in service simultaneously. Pass HoldingKindLoan to skip the loan
	// check (already done above) — only blocks if a service is open.
	hold, err := s.holdingChecker.CheckCommodityFree(ctx, loan.CommodityID, HoldingKindLoan)
	if err != nil && !errors.Is(err, ErrCommodityAlreadyOut) {
		return nil, nil, nil, errxtrace.Wrap("failed to check cross-kind holding", err)
	}
	if errors.Is(err, ErrCommodityAlreadyOut) {
		return nil, nil, hold, err
	}

	// returned_at MUST be empty on create — even if the FE sent a
	// value (e.g. some misuse / future "import historical loan" path),
	// the lend flow always starts open. Stripping defensively.
	loan.ReturnedAt = nil
	loan.ReminderSentOverdue = false
	loan.ReminderSentDueSoon = false

	created, err = loanReg.Create(ctx, loan)
	if err != nil {
		return nil, nil, nil, errxtrace.Wrap("failed to create loan", err)
	}
	s.eventService.EmitLoanStarted(ctx, created)
	return created, nil, nil, nil
}

// LoanUpdate is the per-field patch payload for UpdateLoan. Each
// pointer field uses the standard "nil = leave unchanged, non-nil =
// set to this value" convention. DueBackAt is the one tri-state
// field — see issue #1513.
//
// To clear DueBackAt the caller sets ClearDueBackAt=true (and
// usually leaves DueBackAt nil; if both are set, ClearDueBackAt
// wins because the explicit clear intent should not be silently
// shadowed by a stale pointer left in the patch). Picking a
// parallel bool over a `**Date` keeps call sites legible and dodges
// revive's flag-parameter rule (the bool lives in the patch struct,
// not in the function signature).
type LoanUpdate struct {
	BorrowerName    *string
	BorrowerContact *string
	BorrowerNote    *string
	// DueBackAt: non-nil sets, nil + ClearDueBackAt=false leaves
	// unchanged, nil + ClearDueBackAt=true clears the column.
	DueBackAt      models.PDate
	ClearDueBackAt bool
}

// UpdateLoan applies partial updates to an existing loan. See
// LoanUpdate for per-field semantics.
//
// Mutability matrix:
//   - borrower_name / borrower_contact / borrower_note: mutable on
//     both open and closed loans. The mid-loan ambiguity "if you
//     change the borrower name on an open loan, who actually has the
//     item now?" was raised when this surface was first designed, but
//     the FE (EditLoanDialog) has always exposed these fields as
//     editable and the BE has always accepted them. The right way to
//     model "a different person took the item" is to return the open
//     loan and start a new one — name-edit on an open loan is treated
//     as a typo fix, same as on a closed one.
//   - due_back_at: mutable on OPEN loans (including #1513's clear-to-
//     null). Frozen on CLOSED loans (#1511) — date-of-record once the
//     loan is over; rejected with ErrClosedLoanFieldImmutable.
//   - lent_at and returned_at: NOT touched by this path. lent_at has
//     no PATCH field at all (changing the lend date after the fact is
//     audit confusion). returned_at flips via MarkReturned, not here.
//
// Date corrections on closed loans require delete-and-recreate; that
// path replaces the row entirely, so the original created_at and the
// row's audit history are lost — accept that as the cost of an
// incorrect lend_at / due_back_at / returned_at, since #1511's whole
// motivation was avoiding delete-and-recreate for the typo / late-note
// cases this allowlist covers.
func (s *CommodityLoanService) UpdateLoan(ctx context.Context, id string, patch LoanUpdate) (*models.CommodityLoan, error) {
	loanReg, err := s.factorySet.CommodityLoanRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create loan registry", err)
	}

	current, err := loanReg.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to look up loan", err)
	}

	// Closed-loan gate (#1511). due_back_at is date-of-record on a
	// returned loan and must not change. Reject both the "set new date"
	// and "clear the date" intents so the FE's disabled-with-tooltip
	// affordance matches the wire-level invariant.
	if !current.IsOpen() && (patch.DueBackAt != nil || patch.ClearDueBackAt) {
		return nil, errxtrace.Wrap("due_back_at cannot be edited on a closed loan", ErrClosedLoanFieldImmutable)
	}

	updated := *current
	if patch.BorrowerName != nil {
		updated.BorrowerName = *patch.BorrowerName
	}
	if patch.BorrowerContact != nil {
		updated.BorrowerContact = *patch.BorrowerContact
	}
	if patch.BorrowerNote != nil {
		updated.BorrowerNote = *patch.BorrowerNote
	}
	switch {
	case patch.ClearDueBackAt:
		updated.DueBackAt = nil
	case patch.DueBackAt != nil:
		updated.DueBackAt = patch.DueBackAt
	}

	final, err := loanReg.Update(ctx, updated)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update loan", err)
	}
	s.eventService.EmitLoanUpdated(ctx, current, final)
	return final, nil
}

// MarkReturned closes out a loan. returnedAt defaults to today (server
// clock, YYYY-MM-DD). Returns ErrLoanAlreadyReturned if the row is
// already closed.
func (s *CommodityLoanService) MarkReturned(ctx context.Context, id string, returnedAt models.PDate) (*models.CommodityLoan, error) {
	loanReg, err := s.factorySet.CommodityLoanRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create loan registry", err)
	}

	current, err := loanReg.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to look up loan", err)
	}
	if !current.IsOpen() {
		return nil, errxtrace.Wrap("loan already returned", ErrLoanAlreadyReturned)
	}

	if returnedAt == nil || *returnedAt == "" {
		today := models.Date(time.Now().Format("2006-01-02"))
		returnedAt = &today
	}

	updated := *current
	updated.ReturnedAt = returnedAt

	final, err := loanReg.Update(ctx, updated)
	if err != nil {
		return nil, errxtrace.Wrap("failed to mark loan returned", err)
	}
	s.eventService.EmitLoanReturned(ctx, final)
	return final, nil
}
