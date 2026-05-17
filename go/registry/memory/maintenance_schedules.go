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

// MaintenanceScheduleRegistryFactory creates MaintenanceScheduleRegistry
// instances with proper context. Stores the base registry so all
// per-request registries share the same backing map (mirrors the loan /
// service pattern).
type MaintenanceScheduleRegistryFactory struct {
	base *Registry[models.MaintenanceSchedule, *models.MaintenanceSchedule]
}

// MaintenanceScheduleRegistry is the context-aware in-memory registry
// of maintenance schedules.
type MaintenanceScheduleRegistry struct {
	*Registry[models.MaintenanceSchedule, *models.MaintenanceSchedule]

	userID string
}

var (
	_ registry.MaintenanceScheduleRegistry        = (*MaintenanceScheduleRegistry)(nil)
	_ registry.MaintenanceScheduleRegistryFactory = (*MaintenanceScheduleRegistryFactory)(nil)
)

func NewMaintenanceScheduleRegistryFactory() *MaintenanceScheduleRegistryFactory {
	return &MaintenanceScheduleRegistryFactory{
		base: NewRegistry[models.MaintenanceSchedule, *models.MaintenanceSchedule](),
	}
}

func (f *MaintenanceScheduleRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.MaintenanceScheduleRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *MaintenanceScheduleRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.MaintenanceScheduleRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}

	groupID := appctx.GroupIDFromContext(ctx)
	userRegistry := &Registry[models.MaintenanceSchedule, *models.MaintenanceSchedule]{
		items:   f.base.items,
		lock:    f.base.lock,
		userID:  user.ID,
		groupID: groupID,
	}

	return &MaintenanceScheduleRegistry{
		Registry: userRegistry,
		userID:   user.ID,
	}, nil
}

func (f *MaintenanceScheduleRegistryFactory) CreateServiceRegistry() registry.MaintenanceScheduleRegistry {
	serviceRegistry := &Registry[models.MaintenanceSchedule, *models.MaintenanceSchedule]{
		items:  f.base.items,
		lock:   f.base.lock,
		userID: "",
	}

	return &MaintenanceScheduleRegistry{
		Registry: serviceRegistry,
		userID:   "",
	}
}

func (r *MaintenanceScheduleRegistry) Create(ctx context.Context, schedule models.MaintenanceSchedule) (*models.MaintenanceSchedule, error) {
	now := time.Now()
	schedule.CreatedAt = now
	schedule.UpdatedAt = now
	created, err := r.Registry.CreateWithUser(ctx, schedule)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create maintenance schedule", err)
	}
	return created, nil
}

func (r *MaintenanceScheduleRegistry) Update(ctx context.Context, schedule models.MaintenanceSchedule) (*models.MaintenanceSchedule, error) {
	schedule.UpdatedAt = time.Now()
	updated, err := r.Registry.UpdateWithUser(ctx, schedule)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update maintenance schedule", err)
	}
	return updated, nil
}

// ListByCommodity returns all schedules for the given commodity ordered
// by next_due_at ascending (title as tiebreaker for stable ordering).
func (r *MaintenanceScheduleRegistry) ListByCommodity(ctx context.Context, commodityID string) ([]*models.MaintenanceSchedule, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*models.MaintenanceSchedule, 0, len(all))
	for _, s := range all {
		if s.CommodityID == commodityID {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].NextDueAt != out[j].NextDueAt {
			return out[i].NextDueAt < out[j].NextDueAt
		}
		return out[i].Title < out[j].Title
	})
	return out, nil
}

func (r *MaintenanceScheduleRegistry) ListPaginated(ctx context.Context, offset, limit int, opts registry.MaintenanceListOptions) ([]*models.MaintenanceSchedule, int, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, 0, err
	}

	filtered := all[:0:0]
	for _, s := range all {
		if opts.EnabledOnly && !s.Enabled {
			continue
		}
		if opts.DueBefore != "" && string(s.NextDueAt) > opts.DueBefore {
			continue
		}
		filtered = append(filtered, s)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].NextDueAt != filtered[j].NextDueAt {
			return filtered[i].NextDueAt < filtered[j].NextDueAt
		}
		return filtered[i].Title < filtered[j].Title
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

func (r *MaintenanceScheduleRegistry) CountByCommodity(ctx context.Context, commodityIDs []string) (map[string]int, error) {
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
		if _, ok := wanted[s.CommodityID]; !ok {
			continue
		}
		out[s.CommodityID]++
	}
	return out, nil
}
