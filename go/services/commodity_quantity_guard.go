package services

import (
	"context"
	"errors"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ErrCommodityNotTrackable is the cross-cutting sentinel returned when
// an operation that records per-instance state (lend, send-for-service,
// or warranty) is attempted against a bundle commodity (Count > 1).
//
// Issue #1554: Count > 1 models a bag of interchangeable units, not a
// single tracked instance. There is no single warranty / borrower /
// repair to attach to a "box of 12 screws" — the user splits the row
// into per-unit commodities when those events matter. Apiserver maps
// this sentinel to 422.
var ErrCommodityNotTrackable = errx.NewSentinel("commodity quantity > 1 forbids per-instance tracking")

// EnsureCommodityTrackable rejects with ErrCommodityNotTrackable when
// the referenced commodity has Count > 1. Used by StartLoan and
// StartService.
//
// Reads through the user-context registry so RLS applies — a commodity
// the caller cannot see (or that doesn't exist) is silently skipped
// here so the existing downstream error path stays intact: the loan /
// service create then fails with the canonical FK / not-found error
// the FE already handles. This mirrors the long-standing comment on
// CommodityLoanService.StartLoan ("RLS on commodity_loans + the FK on
// commodity_id handle that path").
func EnsureCommodityTrackable(ctx context.Context, factorySet *registry.FactorySet, commodityID string) error {
	reg, err := factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create commodity registry", err)
	}
	c, err := reg.Get(ctx, commodityID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			// Defer to the downstream FK / RLS error path. If the row is
			// genuinely missing, the loan / service insert below surfaces
			// the canonical 4xx; if it's an RLS isolation hit, exposing
			// "you can't track this" instead would also leak existence.
			return nil
		}
		return errxtrace.Wrap("failed to look up commodity", err)
	}
	if c == nil {
		return nil
	}
	if c.Count > 1 {
		return errxtrace.Wrap("commodity has count > 1", ErrCommodityNotTrackable)
	}
	return nil
}

// QuantityBumpBlockerKind enumerates the per-instance-state kinds that
// prevent a 1 → >1 quantity bump. Stable strings — they leak into the
// JSON:API error code field for FE introspection.
type QuantityBumpBlockerKind string

const (
	// QuantityBumpBlockerWarranty signals the row carries warranty data
	// (expiry date and/or notes) that would become meaningless on a
	// bundle row.
	QuantityBumpBlockerWarranty QuantityBumpBlockerKind = "warranty"
	// QuantityBumpBlockerLoan signals the row has an open loan row.
	QuantityBumpBlockerLoan QuantityBumpBlockerKind = "loan"
	// QuantityBumpBlockerService signals the row has an open service row.
	QuantityBumpBlockerService QuantityBumpBlockerKind = "service"
)

// QuantityBumpBlocker describes one reason a count=1 → count>1 update
// is being rejected. The `Detail` is a server-rendered message; the
// FE renders its own translated copy keyed off `Kind`.
type QuantityBumpBlocker struct {
	Kind   QuantityBumpBlockerKind
	Detail string
}

// CheckQuantityBumpBlockers inspects a count=1 commodity that's about
// to be bumped to count>1 and returns every per-instance-state record
// that needs to be cleared/closed first. Empty result means the bump
// is safe.
//
// Callers (apiserver commodity update path) invoke this only when the
// quantity actually crosses the 1 → >1 boundary; the model-level
// validator already rejects warranty fields on a fresh count>1 row
// without needing the cross-table query.
func CheckQuantityBumpBlockers(ctx context.Context, factorySet *registry.FactorySet, current *models.Commodity) ([]QuantityBumpBlocker, error) {
	if current == nil {
		return nil, nil
	}
	out := make([]QuantityBumpBlocker, 0, 3)

	if (current.WarrantyExpiresAt != nil && string(*current.WarrantyExpiresAt) != "") || current.WarrantyNotes != "" {
		out = append(out, QuantityBumpBlocker{
			Kind:   QuantityBumpBlockerWarranty,
			Detail: "warranty fields are set; clear them before increasing quantity",
		})
	}

	loanReg, err := factorySet.CommodityLoanRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create loan registry", err)
	}
	openLoan, err := loanReg.GetOpenForCommodity(ctx, current.ID)
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		return nil, errxtrace.Wrap("failed to look up open loan", err)
	}
	if openLoan != nil {
		out = append(out, QuantityBumpBlocker{
			Kind:   QuantityBumpBlockerLoan,
			Detail: "an open loan exists; mark it returned before increasing quantity",
		})
	}

	svcReg, err := factorySet.CommodityServiceRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create service registry", err)
	}
	openSvc, err := svcReg.GetOpenForCommodity(ctx, current.ID)
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		return nil, errxtrace.Wrap("failed to look up open service", err)
	}
	if openSvc != nil {
		out = append(out, QuantityBumpBlocker{
			Kind:   QuantityBumpBlockerService,
			Detail: "an open service row exists; mark it returned before increasing quantity",
		})
	}

	return out, nil
}
