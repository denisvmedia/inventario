package services

import (
	"context"
	"errors"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ErrServiceAlreadyOpen / ErrServiceAlreadyReturned alias the registry
// sentinels so apiserver code can compare against the services package
// without an extra import. Mirrors ErrLoanAlreadyOpen / ErrLoanAlreadyReturned.
var (
	ErrServiceAlreadyOpen     = registry.ErrServiceAlreadyOpen
	ErrServiceAlreadyReturned = registry.ErrServiceAlreadyReturned
)

// CommodityServiceService coordinates the in-service lifecycle on top of
// the per-row CommodityServiceRegistry. Sibling to CommodityLoanService —
// the design rationale (single open per commodity, service-level rather
// than DB-level invariant, no advisory locks, FE button-disable as the
// primary UX guard) is identical and documented at length on
// CommodityLoanService. The cross-kind invariant (#1508) lives in the
// shared OpenHoldingChecker so a future "third holding kind" can plug in
// without touching either of the existing services.
type CommodityServiceService struct {
	factorySet     *registry.FactorySet
	eventService   *CommodityEventService
	holdingChecker *OpenHoldingChecker
}

func NewCommodityServiceService(factorySet *registry.FactorySet) *CommodityServiceService {
	return &CommodityServiceService{
		factorySet:     factorySet,
		eventService:   NewCommodityEventService(factorySet),
		holdingChecker: NewOpenHoldingChecker(factorySet),
	}
}

// StartService records a new open service row for the commodity.
//
// Returns:
//   - `created` — the newly persisted service row on success;
//   - `existing` — populated alongside ErrServiceAlreadyOpen when the
//     commodity already has an open service row;
//   - `crossHolding` — populated alongside ErrCommodityAlreadyOut when
//     the commodity has an open LOAN. Mutually exclusive with `existing`;
//   - `err` — any other failure path.
//
// `created`, `existing`, and `crossHolding` are mutually exclusive on a
// non-error response.
//
//revive:disable-next-line:function-result-limit // (created, existing, crossHolding, err) — see the equivalent disable comment on CommodityLoanService.StartLoan.
func (s *CommodityServiceService) StartService(ctx context.Context, svc models.CommodityService) (created, existing *models.CommodityService, crossHolding *OpenHolding, err error) {
	svcReg, err := s.factorySet.CommodityServiceRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, nil, nil, errxtrace.Wrap("failed to create service registry", err)
	}

	// #1554: bundles (Count > 1) cannot be sent for service. Same
	// rationale + tradeoff as CommodityLoanService.StartLoan.
	if err := EnsureCommodityTrackable(ctx, s.factorySet, svc.CommodityID); err != nil {
		return nil, nil, nil, err
	}

	// Same-kind invariant: at most one open service row per commodity.
	existing, err = svcReg.GetOpenForCommodity(ctx, svc.CommodityID)
	if err == nil && existing != nil {
		return nil, existing, nil, errxtrace.Wrap("commodity already has an open service", ErrServiceAlreadyOpen)
	}
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		return nil, nil, nil, errxtrace.Wrap("failed to check existing open service", err)
	}

	// Cross-kind invariant (#1508): block if a loan is open. Pass
	// HoldingKindService so we don't double-check our own table.
	hold, err := s.holdingChecker.CheckCommodityFree(ctx, svc.CommodityID, HoldingKindService)
	if err != nil && !errors.Is(err, ErrCommodityAlreadyOut) {
		return nil, nil, nil, errxtrace.Wrap("failed to check cross-kind holding", err)
	}
	if errors.Is(err, ErrCommodityAlreadyOut) {
		return nil, nil, hold, err
	}

	// returned_at MUST be empty on create — defensively strip even if
	// an upstream caller sent one (e.g. import path). reminder_* flags
	// are worker-controlled.
	svc.ReturnedAt = nil
	svc.ReminderSentOverdue = false
	svc.ReminderSentDueSoon = false

	// Validate cost-pair + ISO 4217 + length caps via the model. The
	// JSON:API DTO already gates the create payload, but the same path
	// is reachable from CLI / import / agentic callers — running the
	// model validator here makes the invariant a single source of
	// truth instead of a DTO-only guarantee.
	if err := svc.ValidateWithContext(ctx); err != nil {
		return nil, nil, nil, errxtrace.Wrap("failed to validate service", err)
	}

	created, err = svcReg.Create(ctx, svc)
	if err != nil {
		return nil, nil, nil, errxtrace.Wrap("failed to create service", err)
	}
	s.eventService.EmitServiceStarted(ctx, created)
	return created, nil, nil, nil
}

// ServiceUpdate is the per-field patch payload for UpdateService. Same
// pointer convention as LoanUpdate ("nil = leave unchanged"); same
// "clearing optional dates not supported via PATCH" caveat — to drop
// expected_return_at, delete the row and start a fresh one.
//
// Cost is patched as a pair: setting CostAmount without CostCurrency (or
// vice versa) fails validation. Clearing the cost (set to nil/zero) is
// supported by passing both fields explicitly.
type ServiceUpdate struct {
	ProviderName     *string
	ProviderContact  *string
	Reason           *string
	ExpectedReturnAt models.PDate
	// CostAmount: non-nil sets, nil leaves unchanged. Must be paired
	// with CostCurrency on the same call.
	CostAmount *decimal.Decimal
	// CostCurrency: non-nil sets, nil leaves unchanged. Must be paired
	// with CostAmount on the same call. Plain string (not Currency)
	// because empty-string "unset" trips Currency's auto-Validatable;
	// ISO 4217 is enforced at the model layer when non-empty.
	CostCurrency *string
}

// UpdateService applies partial updates to an existing service row. See
// ServiceUpdate for per-field semantics.
//
// sent_at and provider_name's "first set" are intentionally NOT
// re-mutable here for the same audit-clarity reasons spelled out on
// UpdateLoan: changing the send date or the workshop after the fact
// muddles the timeline. Replace the row instead.
func (s *CommodityServiceService) UpdateService(ctx context.Context, id string, patch ServiceUpdate) (*models.CommodityService, error) {
	svcReg, err := s.factorySet.CommodityServiceRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create service registry", err)
	}

	current, err := svcReg.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to look up service", err)
	}

	updated := *current
	if patch.ProviderName != nil {
		updated.ProviderName = *patch.ProviderName
	}
	if patch.ProviderContact != nil {
		updated.ProviderContact = *patch.ProviderContact
	}
	if patch.Reason != nil {
		updated.Reason = *patch.Reason
	}
	if patch.ExpectedReturnAt != nil {
		updated.ExpectedReturnAt = patch.ExpectedReturnAt
	}
	// Cost is paired — caller must set BOTH or NEITHER on a single
	// patch. The model's ValidateWithContext re-checks the pair invariant.
	if patch.CostAmount != nil {
		updated.CostAmount = *patch.CostAmount
	}
	if patch.CostCurrency != nil {
		updated.CostCurrency = *patch.CostCurrency
	}

	// Re-validate the patched row. Catches non-ISO currency, an unset
	// pair, length-cap violations on the patched fields, etc.
	if err := updated.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("failed to validate service patch", err)
	}

	final, err := svcReg.Update(ctx, updated)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update service", err)
	}
	s.eventService.EmitServiceUpdated(ctx, current, final)
	return final, nil
}

// MarkReturned closes out a service row. returnedAt defaults to today
// (server clock, YYYY-MM-DD). Returns ErrServiceAlreadyReturned if the
// row is already closed. Optional finalCost lets the caller record the
// repair bill on the same call as the return — common workflow ("I
// picked it up and the bill was X").
//
// finalCurrency is plain *string for the same reason ServiceUpdate's
// CostCurrency is — see that type for the rationale.
func (s *CommodityServiceService) MarkReturned(ctx context.Context, id string, returnedAt models.PDate, finalCost *decimal.Decimal, finalCurrency *string) (*models.CommodityService, error) {
	svcReg, err := s.factorySet.CommodityServiceRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create service registry", err)
	}

	current, err := svcReg.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to look up service", err)
	}
	if !current.IsOpen() {
		return nil, errxtrace.Wrap("service already returned", ErrServiceAlreadyReturned)
	}

	if returnedAt == nil || *returnedAt == "" {
		today := models.Date(time.Now().Format("2006-01-02"))
		returnedAt = &today
	}

	updated := *current
	updated.ReturnedAt = returnedAt
	if finalCost != nil {
		updated.CostAmount = *finalCost
	}
	if finalCurrency != nil {
		updated.CostCurrency = *finalCurrency
	}

	// Re-validate so a final-cost pair patched on return-time obeys the
	// same ISO 4217 + pair invariant as create / update.
	if err := updated.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("failed to validate service final cost", err)
	}

	final, err := svcReg.Update(ctx, updated)
	if err != nil {
		return nil, errxtrace.Wrap("failed to mark service returned", err)
	}
	s.eventService.EmitServiceReturned(ctx, final)
	return final, nil
}
