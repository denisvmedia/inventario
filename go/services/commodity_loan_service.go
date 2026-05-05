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
}

func NewCommodityLoanService(factorySet *registry.FactorySet) *CommodityLoanService {
	return &CommodityLoanService{factorySet: factorySet}
}

// StartLoan records a new open loan for the commodity. Returns
// ErrLoanAlreadyOpen along with the existing loan if one is already
// open — apiserver layer renders that as a 409 with the existing-loan
// payload so the FE can render "already lent to X on Y" instead of
// stacking duplicates.
func (s *CommodityLoanService) StartLoan(ctx context.Context, loan models.CommodityLoan) (*models.CommodityLoan, *models.CommodityLoan, error) {
	loanReg, err := s.factorySet.CommodityLoanRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, nil, errxtrace.Wrap("failed to create loan registry", err)
	}

	// Guard against a second open loan on the same commodity. We do
	// not pre-validate that the commodity exists / is in this group —
	// RLS on commodity_loans + the FK on commodity_id (ON DELETE
	// CASCADE) handle that path: a non-existent / cross-group commodity
	// id surfaces as a postgres FK violation that translates to 4xx via
	// the standard renderEntityError path. Leaving the check out here
	// avoids an extra round-trip on the happy path.
	existing, err := loanReg.GetOpenForCommodity(ctx, loan.CommodityID)
	if err == nil && existing != nil {
		return nil, existing, errxtrace.Wrap("commodity already has an open loan", ErrLoanAlreadyOpen)
	}
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		return nil, nil, errxtrace.Wrap("failed to check existing open loan", err)
	}

	// returned_at MUST be empty on create — even if the FE sent a
	// value (e.g. some misuse / future "import historical loan" path),
	// the lend flow always starts open. Stripping defensively.
	loan.ReturnedAt = nil
	loan.ReminderSentOverdue = false
	loan.ReminderSentDueSoon = false

	created, err := loanReg.Create(ctx, loan)
	if err != nil {
		return nil, nil, errxtrace.Wrap("failed to create loan", err)
	}
	return created, nil, nil
}

// UpdateLoan applies partial updates to an existing loan. Updatable
// fields: borrower_name (must stay non-empty if set), borrower_contact,
// borrower_note, due_back_at (clearable). Pass nil pointers to leave a
// field unchanged; pass an empty *string to clear borrower_contact /
// borrower_note. due_back_at is cleared by passing an empty
// models.PDate (a `null` in JSON, which deserialises to a non-nil
// pointer to an empty Date).
//
// lent_at and the borrower-name "first set" are intentionally NOT
// re-mutable here: changing the lend date after the fact creates audit
// confusion ("when did this actually leave?"); changing the borrower
// name after the fact loses history if a different borrower took the
// item next ("who has it now?" ambiguity). Replace the loan instead
// (return the old one + start a new one).
func (s *CommodityLoanService) UpdateLoan(ctx context.Context, id string, borrowerName, borrowerContact, borrowerNote *string, dueBackAt models.PDate, dueBackAtSet bool) (*models.CommodityLoan, error) {
	loanReg, err := s.factorySet.CommodityLoanRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create loan registry", err)
	}

	current, err := loanReg.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to look up loan", err)
	}

	updated := *current
	if borrowerName != nil {
		updated.BorrowerName = *borrowerName
	}
	if borrowerContact != nil {
		updated.BorrowerContact = *borrowerContact
	}
	if borrowerNote != nil {
		updated.BorrowerNote = *borrowerNote
	}
	if dueBackAtSet {
		// Caller signalled "this field appeared in the JSON." Empty
		// string in the supplied PDate clears the date; non-empty
		// replaces it.
		updated.DueBackAt = dueBackAt
	}

	final, err := loanReg.Update(ctx, updated)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update loan", err)
	}
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
	return final, nil
}
