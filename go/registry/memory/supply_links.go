package memory

import (
	"context"
	"sort"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// SupplyLinkRegistryFactory creates SupplyLinkRegistry instances with
// proper context (#1369). Mirrors the loan/service registry factories.
type SupplyLinkRegistryFactory struct {
	base *Registry[models.SupplyLink, *models.SupplyLink]
}

// SupplyLinkRegistry is the context-aware in-memory registry of supply links.
type SupplyLinkRegistry struct {
	*Registry[models.SupplyLink, *models.SupplyLink]
}

var (
	_ registry.SupplyLinkRegistry        = (*SupplyLinkRegistry)(nil)
	_ registry.SupplyLinkRegistryFactory = (*SupplyLinkRegistryFactory)(nil)
)

func NewSupplyLinkRegistryFactory() *SupplyLinkRegistryFactory {
	return &SupplyLinkRegistryFactory{
		base: NewRegistry[models.SupplyLink, *models.SupplyLink](),
	}
}

func (f *SupplyLinkRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.SupplyLinkRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *SupplyLinkRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.SupplyLinkRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}

	groupID := appctx.GroupIDFromContext(ctx)
	userRegistry := &Registry[models.SupplyLink, *models.SupplyLink]{
		items:   f.base.items,
		lock:    f.base.lock,
		userID:  user.ID,
		groupID: groupID,
	}

	return &SupplyLinkRegistry{Registry: userRegistry}, nil
}

func (f *SupplyLinkRegistryFactory) CreateServiceRegistry() registry.SupplyLinkRegistry {
	serviceRegistry := &Registry[models.SupplyLink, *models.SupplyLink]{
		items:  f.base.items,
		lock:   f.base.lock,
		userID: "",
	}
	return &SupplyLinkRegistry{Registry: serviceRegistry}
}

func (r *SupplyLinkRegistry) Create(ctx context.Context, link models.SupplyLink) (*models.SupplyLink, error) {
	now := time.Now()
	link.CreatedAt = now
	link.UpdatedAt = now
	created, err := r.Registry.CreateWithUser(ctx, link)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create supply link", err)
	}
	return created, nil
}

func (r *SupplyLinkRegistry) Update(ctx context.Context, link models.SupplyLink) (*models.SupplyLink, error) {
	link.UpdatedAt = time.Now()
	updated, err := r.Registry.UpdateWithUser(ctx, link)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update supply link", err)
	}
	return updated, nil
}

// ListByCommodity returns supply links for one commodity ordered by
// sort_order ASC, created_at ASC. Matches the postgres path.
func (r *SupplyLinkRegistry) ListByCommodity(ctx context.Context, commodityID string) ([]*models.SupplyLink, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*models.SupplyLink, 0, len(all))
	for _, l := range all {
		if l.CommodityID == commodityID {
			out = append(out, l)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// ReorderForCommodity densely renumbers sort_order = position for each
// id in orderedIDs (0..N-1). Ids not belonging to the commodity surface
// as ErrNotFound — mirrors the postgres path's all-or-nothing behaviour.
func (r *SupplyLinkRegistry) ReorderForCommodity(ctx context.Context, commodityID string, orderedIDs []string) error {
	if len(orderedIDs) == 0 {
		return nil
	}
	// Pre-validate every id belongs to commodityID. Failing fast avoids
	// a half-applied permutation when the second id is bad.
	for _, id := range orderedIDs {
		link, err := r.Get(ctx, id)
		if err != nil {
			return err
		}
		if link.CommodityID != commodityID {
			return registry.ErrNotFound
		}
	}
	now := time.Now()
	for i, id := range orderedIDs {
		link, err := r.Get(ctx, id)
		if err != nil {
			return err
		}
		link.SortOrder = i
		link.UpdatedAt = now
		if _, err := r.Registry.UpdateWithUser(ctx, *link); err != nil {
			return errxtrace.Wrap("failed to update supply link sort_order", err)
		}
	}
	return nil
}

// CountByCommodity returns the per-commodity supply link count. Mirrors
// the postgres path; missing ids map to 0.
func (r *SupplyLinkRegistry) CountByCommodity(ctx context.Context, commodityIDs []string) (map[string]int, error) {
	out := make(map[string]int, len(commodityIDs))
	for _, id := range commodityIDs {
		out[id] = 0
	}
	if len(commodityIDs) == 0 {
		return out, nil
	}
	wanted := make(map[string]struct{}, len(commodityIDs))
	for _, id := range commodityIDs {
		wanted[id] = struct{}{}
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, l := range all {
		if _, ok := wanted[l.CommodityID]; !ok {
			continue
		}
		out[l.CommodityID]++
	}
	return out, nil
}
