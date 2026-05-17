package services

import (
	"context"
	"errors"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// SupplyLinkService is the per-commodity "supply links" (#1369) write
// path. Reads happen directly through SupplyLinkRegistry; the service
// exists for create / update / delete / reorder where we need to:
//
//   - assert the parent commodity exists in the current group context
//     (RLS would silently reject otherwise — 404 reads more cleanly);
//   - keep the commodity_id pinned to the URL path (defence against an
//     attacker swapping it in the body to attach a link to someone
//     else's item — RLS catches it, but rejecting at the service layer
//     means the FE sees a clean 404 instead of a CHECK violation);
//   - load the existing row on Update so callers can pass a sparse
//     patch (label / url / notes) without round-tripping the rest.
//
// The service deliberately does NOT cap "links per commodity" — the
// issue lists no such constraint, and households with weird appliances
// can have a handful per item. If pathological row counts ever surface
// we'll add the cap here, not in the registry.
type SupplyLinkService struct {
	factorySet *registry.FactorySet
}

// NewSupplyLinkService wires the service to the registry factories.
// Per-request registries are created inside each method off the
// request context, mirroring CommodityLoanService.
func NewSupplyLinkService(fs *registry.FactorySet) *SupplyLinkService {
	return &SupplyLinkService{factorySet: fs}
}

// Create attaches a new supply link to the commodity. The link.CommodityID
// must match the commodity in the request context — the apiserver layer
// pins it from the URL before calling.
func (s *SupplyLinkService) Create(ctx context.Context, link models.SupplyLink) (*models.SupplyLink, error) {
	if err := s.assertCommodityExists(ctx, link.CommodityID); err != nil {
		return nil, err
	}
	supplies, err := s.factorySet.SupplyLinkRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create supply link registry", err)
	}
	// Default sort_order to N (append) so the new row appears at the
	// bottom of the list. The FE relies on this for the "add link" CTA.
	if link.SortOrder == 0 {
		existing, lerr := supplies.ListByCommodity(ctx, link.CommodityID)
		if lerr != nil {
			return nil, errxtrace.Wrap("failed to list supply links", lerr)
		}
		link.SortOrder = len(existing)
	}
	created, err := supplies.Create(ctx, link)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create supply link", err)
	}
	return created, nil
}

// SupplyLinkPatch carries the fields a PATCH may mutate. Nil pointers
// mean "leave unchanged"; non-nil pointers (including empty strings)
// mean "set to this value". Sticking to pointer-presence semantics
// keeps the apiserver layer simple — no separate "clear" flags.
type SupplyLinkPatch struct {
	Label *string
	URL   *string
	Notes *string
}

// Update applies a sparse patch to the given supply link by id.
func (s *SupplyLinkService) Update(ctx context.Context, id string, patch SupplyLinkPatch) (*models.SupplyLink, error) {
	supplies, err := s.factorySet.SupplyLinkRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create supply link registry", err)
	}
	link, err := supplies.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if patch.Label != nil {
		link.Label = *patch.Label
	}
	if patch.URL != nil {
		link.URL = *patch.URL
	}
	if patch.Notes != nil {
		link.Notes = *patch.Notes
	}
	if err := link.ValidateWithContext(ctx); err != nil {
		return nil, err
	}
	updated, err := supplies.Update(ctx, *link)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update supply link", err)
	}
	return updated, nil
}

// Delete removes a supply link by id. Cascade from the parent commodity
// is handled by the DB (ON DELETE CASCADE on commodity_supply_links.commodity_id).
func (s *SupplyLinkService) Delete(ctx context.Context, id string) error {
	supplies, err := s.factorySet.SupplyLinkRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create supply link registry", err)
	}
	return supplies.Delete(ctx, id)
}

// Reorder applies a permutation: orderedIDs becomes the new visible
// order, densely renumbered 0..N-1. Returns ErrNotFound (surfaced as
// 404 by the apiserver) if any id does not belong to the commodity.
func (s *SupplyLinkService) Reorder(ctx context.Context, commodityID string, orderedIDs []string) error {
	if err := s.assertCommodityExists(ctx, commodityID); err != nil {
		return err
	}
	supplies, err := s.factorySet.SupplyLinkRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create supply link registry", err)
	}
	return supplies.ReorderForCommodity(ctx, commodityID, orderedIDs)
}

// assertCommodityExists returns ErrNotFound if the commodity is not
// visible under the current RLS context, so write paths fail with the
// same code regardless of whether the commodity is in another group
// (RLS hides it) or simply does not exist.
func (s *SupplyLinkService) assertCommodityExists(ctx context.Context, commodityID string) error {
	if commodityID == "" {
		return registry.ErrFieldRequired
	}
	commodities, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create commodity registry", err)
	}
	if _, gerr := commodities.Get(ctx, commodityID); gerr != nil {
		if errors.Is(gerr, registry.ErrNotFound) {
			return registry.ErrNotFound
		}
		return errxtrace.Wrap("failed to load parent commodity", gerr)
	}
	return nil
}
