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

// CommodityServiceRegistryFactory creates CommodityServiceRegistry instances
// with proper context. Stores the base registry so all per-request
// registries share the same backing map (mirrors loans / tags / exports).
type CommodityServiceRegistryFactory struct {
	base *Registry[models.CommodityService, *models.CommodityService]
}

// CommodityServiceRegistry is the context-aware in-memory registry of services.
type CommodityServiceRegistry struct {
	*Registry[models.CommodityService, *models.CommodityService]

	userID string
}

var (
	_ registry.CommodityServiceRegistry        = (*CommodityServiceRegistry)(nil)
	_ registry.CommodityServiceRegistryFactory = (*CommodityServiceRegistryFactory)(nil)
)

func NewCommodityServiceRegistryFactory() *CommodityServiceRegistryFactory {
	return &CommodityServiceRegistryFactory{
		base: NewRegistry[models.CommodityService, *models.CommodityService](),
	}
}

func (f *CommodityServiceRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityServiceRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CommodityServiceRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityServiceRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}

	groupID := appctx.GroupIDFromContext(ctx)
	userRegistry := &Registry[models.CommodityService, *models.CommodityService]{
		items:   f.base.items,
		lock:    f.base.lock,
		userID:  user.ID,
		groupID: groupID,
	}

	return &CommodityServiceRegistry{
		Registry: userRegistry,
		userID:   user.ID,
	}, nil
}

func (f *CommodityServiceRegistryFactory) CreateServiceRegistry() registry.CommodityServiceRegistry {
	serviceRegistry := &Registry[models.CommodityService, *models.CommodityService]{
		items:  f.base.items,
		lock:   f.base.lock,
		userID: "",
	}

	return &CommodityServiceRegistry{
		Registry: serviceRegistry,
		userID:   "",
	}
}

func (r *CommodityServiceRegistry) Create(ctx context.Context, svc models.CommodityService) (*models.CommodityService, error) {
	now := time.Now()
	svc.CreatedAt = now
	svc.UpdatedAt = now
	created, err := r.Registry.CreateWithUser(ctx, svc)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create service", err)
	}
	return created, nil
}

func (r *CommodityServiceRegistry) Update(ctx context.Context, svc models.CommodityService) (*models.CommodityService, error) {
	svc.UpdatedAt = time.Now()
	updated, err := r.Registry.UpdateWithUser(ctx, svc)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update service", err)
	}
	return updated, nil
}

// ListByCommodity returns services for a single commodity, most-recent-first
// (sent_at desc, created_at desc as tiebreaker).
func (r *CommodityServiceRegistry) ListByCommodity(ctx context.Context, commodityID string) ([]*models.CommodityService, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*models.CommodityService, 0, len(all))
	for _, s := range all {
		if s.CommodityID == commodityID {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SentAt != out[j].SentAt {
			return out[i].SentAt > out[j].SentAt
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// GetOpenForCommodity returns the (at most one) open service row for the
// given commodity. Returns ErrNotFound if no open service exists. If
// multiple open services somehow exist (memory backend mid-test), returns
// the most recent — matching `SELECT ... ORDER BY sent_at DESC LIMIT 1`.
func (r *CommodityServiceRegistry) GetOpenForCommodity(ctx context.Context, commodityID string) (*models.CommodityService, error) {
	services, err := r.ListByCommodity(ctx, commodityID)
	if err != nil {
		return nil, err
	}
	for _, s := range services {
		if s.IsOpen() {
			return s, nil
		}
	}
	return nil, registry.ErrNotFound
}

func (r *CommodityServiceRegistry) ListPaginated(ctx context.Context, offset, limit int, opts registry.ServiceListOptions) ([]*models.CommodityService, int, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, 0, err
	}

	state := opts.State
	if state == "" {
		state = registry.ServiceStateAll
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	filtered := all[:0:0]
	for _, s := range all {
		switch state {
		case registry.ServiceStateAll:
			filtered = append(filtered, s)
		case registry.ServiceStateOpen:
			if s.IsOpen() {
				filtered = append(filtered, s)
			}
		case registry.ServiceStateOverdue:
			if s.IsOverdue(now) {
				filtered = append(filtered, s)
			}
		case registry.ServiceStateCompleted:
			if !s.IsOpen() {
				filtered = append(filtered, s)
			}
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].SentAt != filtered[j].SentAt {
			return filtered[i].SentAt > filtered[j].SentAt
		}
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}
	start := min(offset, total)
	end := min(start+limit, total)
	return filtered[start:end], total, nil
}

func (r *CommodityServiceRegistry) CountOpenByCommodity(ctx context.Context, commodityIDs []string) (map[string]int, error) {
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
	for _, s := range all {
		if !s.IsOpen() {
			continue
		}
		if _, ok := wanted[s.CommodityID]; !ok {
			continue
		}
		out[s.CommodityID]++
	}
	return out, nil
}
