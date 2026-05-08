package services

import (
	"context"
	"errors"
	"time"

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
// set to this value" convention.
//
// **Clearing `DueBackAt` is NOT supported.** JSON `null` and an omitted
// field both decode to a nil *Date with `omitempty`, so the handler
// can't surface "user wants to clear this" via the wire format today.
// To remove a due date, delete the loan and create a fresh one —
// preserves a clean audit history. (Wrapping the patch in a struct
// rather than threading a parallel `dueBackAtSet bool` parameter
// keeps the call site readable and dodges revive's
// flag-parameter rule.)
type LoanUpdate struct {
	BorrowerName    *string
	BorrowerContact *string
	BorrowerNote    *string
	// DueBackAt: non-nil sets, nil leaves unchanged. There's no
	// "clear" sentinel — see the type-level comment above.
	DueBackAt models.PDate
}

// UpdateLoan applies partial updates to an existing loan. See
// LoanUpdate for per-field semantics.
//
// lent_at and the borrower-name "first set" are intentionally NOT
// re-mutable here: changing the lend date after the fact creates audit
// confusion ("when did this actually leave?"); changing the borrower
// name after the fact loses history if a different borrower took the
// item next ("who has it now?" ambiguity). Replace the loan instead
// (return the old one + start a new one).
func (s *CommodityLoanService) UpdateLoan(ctx context.Context, id string, patch LoanUpdate) (*models.CommodityLoan, error) {
	loanReg, err := s.factorySet.CommodityLoanRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create loan registry", err)
	}

	current, err := loanReg.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to look up loan", err)
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
	if patch.DueBackAt != nil {
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
