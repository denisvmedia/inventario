package memory

import (
	"context"
	"slices"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// CommodityEventRegistryFactory creates CommodityEventRegistry instances with proper context.
type CommodityEventRegistryFactory struct {
	base *Registry[models.CommodityEvent, *models.CommodityEvent]
}

// CommodityEventRegistry is the in-memory append-only audit log for
// commodity state changes (#1450). Used in unit tests and single-process
// dev deployments; postgres is the production target.
type CommodityEventRegistry struct {
	*Registry[models.CommodityEvent, *models.CommodityEvent]
}

var (
	_ registry.CommodityEventRegistry        = (*CommodityEventRegistry)(nil)
	_ registry.CommodityEventRegistryFactory = (*CommodityEventRegistryFactory)(nil)
)

func NewCommodityEventRegistryFactory() *CommodityEventRegistryFactory {
	return &CommodityEventRegistryFactory{
		base: NewRegistry[models.CommodityEvent, *models.CommodityEvent](),
	}
}

func (f *CommodityEventRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityEventRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CommodityEventRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityEventRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	groupID := appctx.GroupIDFromContext(ctx)
	userRegistry := &Registry[models.CommodityEvent, *models.CommodityEvent]{
		items:   f.base.items,
		lock:    f.base.lock,
		userID:  user.ID,
		groupID: groupID,
	}
	return &CommodityEventRegistry{Registry: userRegistry}, nil
}

func (f *CommodityEventRegistryFactory) CreateServiceRegistry() registry.CommodityEventRegistry {
	serviceRegistry := &Registry[models.CommodityEvent, *models.CommodityEvent]{
		items: f.base.items,
		lock:  f.base.lock,
	}
	return &CommodityEventRegistry{Registry: serviceRegistry}
}

func (r *CommodityEventRegistry) Create(ctx context.Context, event models.CommodityEvent) (*models.CommodityEvent, error) {
	created, err := r.Registry.CreateWithUser(ctx, event)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity event", err)
	}
	return created, nil
}

// Update is a deliberate no-op so behavior matches the postgres
// implementation: events are append-only by contract. The generic
// memory.Registry.Update would otherwise overwrite the row, letting a
// memory-mode caller mutate history that the postgres backend would
// silently swallow. Returning the input untouched keeps the interface
// satisfiable without violating the append-only invariant.
func (r *CommodityEventRegistry) Update(_ context.Context, event models.CommodityEvent) (*models.CommodityEvent, error) {
	return &event, nil
}

// ListByCommodity returns paginated events for a single commodity newest-first.
// Filters by Kinds when supplied. Total reflects the filtered count.
func (r *CommodityEventRegistry) ListByCommodity(ctx context.Context, commodityID string, offset, limit int, opts registry.CommodityEventListOptions) ([]*models.CommodityEvent, int, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, 0, err
	}
	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}

	filtered := make([]*models.CommodityEvent, 0, len(all))
	for _, ev := range all {
		if ev == nil {
			continue
		}
		if ev.CommodityID != commodityID {
			continue
		}
		if len(opts.Kinds) > 0 && !slices.Contains(opts.Kinds, ev.Kind) {
			continue
		}
		filtered = append(filtered, ev)
	}

	// Newest-first; tie-break on id descending so the paginator is deterministic.
	slices.SortStableFunc(filtered, func(a, b *models.CommodityEvent) int {
		if a.OccurredAt.After(b.OccurredAt) {
			return -1
		}
		if a.OccurredAt.Before(b.OccurredAt) {
			return 1
		}
		switch {
		case a.GetID() > b.GetID():
			return -1
		case a.GetID() < b.GetID():
			return 1
		default:
			return 0
		}
	})

	total := len(filtered)
	start := min(offset, total)
	end := min(start+limit, total)
	return filtered[start:end], total, nil
}
