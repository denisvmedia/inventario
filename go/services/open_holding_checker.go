package services

import (
	"context"
	"errors"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/registry"
)

// HoldingKind names a kind of "holding" — a state where a commodity is
// temporarily out of the user's possession. Loans (#1452) and services
// (#1508) are both holdings; future kinds (in-transit, pawned, ...)
// would extend this enum.
type HoldingKind string

const (
	HoldingKindLoan    HoldingKind = "loan"
	HoldingKindService HoldingKind = "service"
)

// OpenHolding is the projection of any open holding — loan, service,
// future kind — that the cross-kind invariant cares about. Carries just
// enough to render an actionable 409 on the FE (kind + id + lent
// borrower / service provider) without leaking the whole row across
// abstraction boundaries.
//
// Service callers translate the wrapped sentinel ErrCommodityAlreadyOut
// into a domain 409 and attach the OpenHolding payload so the FE can
// render "already at Apple Service since 2026-03-12" or "already lent
// to X" without a follow-up GET.
type OpenHolding struct {
	Kind HoldingKind
	ID   string
	// PartyName is the user-facing label of the other side of the
	// holding — borrower for a loan, provider for a service.
	PartyName string
}

// OpenHoldingChecker enforces the cross-kind invariant: a commodity is
// out at most for ONE reason at a time. Callers (CommodityLoanService.
// StartLoan, CommodityServiceService.StartService, future kinds) consult
// the checker before persisting their own row.
//
// Implementation note: the checker queries each kind-specific registry
// individually rather than maintaining a denormalised "holdings" table.
// At small N (loans + services per commodity), the round-trip cost is
// negligible compared to the schema-flag-day cost a single table would
// impose — see the issue's "single-table generalization?" open question.
//
// Concurrency: same race-window caveat as the per-kind invariants. Two
// simultaneous POSTs (one StartLoan, one StartService) on the same
// commodity could both observe no open holding and both INSERT. Same
// "deliberate human action, FE button hide is the primary UX guard"
// reasoning as documented on CommodityLoanService.
type OpenHoldingChecker struct {
	factorySet *registry.FactorySet
}

// NewOpenHoldingChecker binds the checker to a FactorySet so it can build
// per-request, RLS-scoped registries on each call.
func NewOpenHoldingChecker(factorySet *registry.FactorySet) *OpenHoldingChecker {
	return &OpenHoldingChecker{factorySet: factorySet}
}

// CheckCommodityFree reports whether the commodity has any OPEN holding
// (loan or service). If excludeKind is non-empty, that kind is skipped —
// callers use this to ignore "their own" kind ("I'm starting a loan;
// only block me if a service is open"). Returns:
//
//   - (nil, nil) — commodity is free, the holding-creating call should
//     proceed.
//   - (holding, ErrCommodityAlreadyOut) — a different open holding kind
//     exists. Caller wraps the wrapped sentinel into a domain 409.
//   - (nil, err) — registry failure.
func (c *OpenHoldingChecker) CheckCommodityFree(ctx context.Context, commodityID string, excludeKind HoldingKind) (*OpenHolding, error) {
	if c == nil || commodityID == "" {
		return nil, nil
	}

	if excludeKind != HoldingKindLoan {
		loanReg, err := c.factorySet.CommodityLoanRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to create loan registry", err)
		}
		openLoan, err := loanReg.GetOpenForCommodity(ctx, commodityID)
		if err == nil && openLoan != nil {
			return &OpenHolding{
				Kind:      HoldingKindLoan,
				ID:        openLoan.ID,
				PartyName: openLoan.BorrowerName,
			}, errxtrace.Wrap("commodity is already out (open loan)", ErrCommodityAlreadyOut)
		}
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			return nil, errxtrace.Wrap("failed to check open loan", err)
		}
	}

	if excludeKind != HoldingKindService {
		svcReg, err := c.factorySet.CommodityServiceRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to create service registry", err)
		}
		openSvc, err := svcReg.GetOpenForCommodity(ctx, commodityID)
		if err == nil && openSvc != nil {
			return &OpenHolding{
				Kind:      HoldingKindService,
				ID:        openSvc.ID,
				PartyName: openSvc.ProviderName,
			}, errxtrace.Wrap("commodity is already out (open service)", ErrCommodityAlreadyOut)
		}
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			return nil, errxtrace.Wrap("failed to check open service", err)
		}
	}

	return nil, nil
}

// ErrCommodityAlreadyOut is the cross-kind sentinel re-exported here so
// service-package consumers can compare against it without an extra
// import. Mirrors the ErrLoanAlreadyOpen / ErrLoanAlreadyReturned aliases.
var ErrCommodityAlreadyOut = registry.ErrCommodityAlreadyOut
