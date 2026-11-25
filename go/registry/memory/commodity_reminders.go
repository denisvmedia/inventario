package memory

import (
	"context"
	"sort"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// CommodityReminderRegistryFactory creates CommodityReminderRegistry
// instances. All registries it creates share one backing map.
type CommodityReminderRegistryFactory struct {
	base *Registry[models.CommodityReminder, *models.CommodityReminder]
}

// CommodityReminderRegistry is the context-aware in-memory registry of item
// reminders.
type CommodityReminderRegistry struct {
	*Registry[models.CommodityReminder, *models.CommodityReminder]
}

var (
	_ registry.CommodityReminderRegistry        = (*CommodityReminderRegistry)(nil)
	_ registry.CommodityReminderRegistryFactory = (*CommodityReminderRegistryFactory)(nil)
)

func NewCommodityReminderRegistryFactory() *CommodityReminderRegistryFactory {
	return &CommodityReminderRegistryFactory{
		base: NewRegistry[models.CommodityReminder, *models.CommodityReminder](),
	}
}

func (f *CommodityReminderRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityReminderRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CommodityReminderRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityReminderRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}

	return &CommodityReminderRegistry{
		Registry: &Registry[models.CommodityReminder, *models.CommodityReminder]{
			items:   f.base.items,
			lock:    f.base.lock,
			userID:  user.ID,
			groupID: appctx.GroupIDFromContext(ctx),
		},
	}, nil
}

func (f *CommodityReminderRegistryFactory) CreateServiceRegistry() registry.CommodityReminderRegistry {
	return &CommodityReminderRegistry{
		Registry: &Registry[models.CommodityReminder, *models.CommodityReminder]{
			items: f.base.items,
			lock:  f.base.lock,
		},
	}
}

func (r *CommodityReminderRegistry) Create(ctx context.Context, reminder models.CommodityReminder) (*models.CommodityReminder, error) {
	now := time.Now()
	reminder.CreatedAt = now
	reminder.UpdatedAt = now
	created, err := r.Registry.CreateWithUser(ctx, reminder)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity reminder", err)
	}
	return created, nil
}

func (r *CommodityReminderRegistry) Update(ctx context.Context, reminder models.CommodityReminder) (*models.CommodityReminder, error) {
	reminder.UpdatedAt = time.Now()
	updated, err := r.Registry.UpdateWithUser(ctx, reminder)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update commodity reminder", err)
	}
	return updated, nil
}

// ListByCommodity returns the reminders of one item ordered by due_on, with
// the title as a tiebreaker.
func (r *CommodityReminderRegistry) ListByCommodity(ctx context.Context, commodityID string) ([]*models.CommodityReminder, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*models.CommodityReminder, 0, len(all))
	for _, rem := range all {
		if rem.CommodityID == commodityID {
			out = append(out, rem)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DueOn != out[j].DueOn {
			return out[i].DueOn < out[j].DueOn
		}
		return out[i].Title < out[j].Title
	})
	return out, nil
}
