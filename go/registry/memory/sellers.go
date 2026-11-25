package memory

import (
	"context"
	"sync"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// SellerRegistryFactory creates SellerRegistry instances. All registries it
// creates share one backing map, and the commodity factory whose items point
// at the sellers: Delete unlinks them and ItemCounts counts them, standing in
// for the foreign key and the GROUP BY of the postgres registry.
type SellerRegistryFactory struct {
	base        *Registry[models.Seller, *models.Seller]
	commodities *CommodityRegistryFactory
	// writes serializes Create and Update, so the name check and the write
	// are one step, as the unique index makes them in postgres.
	writes *sync.Mutex
}

// SellerRegistry is the context-aware in-memory registry of sellers.
type SellerRegistry struct {
	*Registry[models.Seller, *models.Seller]

	all         *Registry[models.Seller, *models.Seller]
	commodities *CommodityRegistryFactory
	writes      *sync.Mutex
}

var (
	_ registry.SellerRegistry        = (*SellerRegistry)(nil)
	_ registry.SellerRegistryFactory = (*SellerRegistryFactory)(nil)
)

// NewSellerRegistryFactory creates the factory and attaches it to
// commodities, whose writes then check the seller they name.
func NewSellerRegistryFactory(commodities *CommodityRegistryFactory) *SellerRegistryFactory {
	f := &SellerRegistryFactory{
		base:        NewRegistry[models.Seller, *models.Seller](),
		commodities: commodities,
		writes:      &sync.Mutex{},
	}
	if commodities != nil {
		commodities.sellers = f
	}
	return f
}

func (f *SellerRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.SellerRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *SellerRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.SellerRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return f.newRegistry(user.ID, appctx.GroupIDFromContext(ctx)), nil
}

func (f *SellerRegistryFactory) CreateServiceRegistry() registry.SellerRegistry {
	return f.newRegistry("", "")
}

func (f *SellerRegistryFactory) newRegistry(userID, groupID string) *SellerRegistry {
	return &SellerRegistry{
		Registry: &Registry[models.Seller, *models.Seller]{
			items:   f.base.items,
			lock:    f.base.lock,
			userID:  userID,
			groupID: groupID,
		},
		all:         &Registry[models.Seller, *models.Seller]{items: f.base.items, lock: f.base.lock},
		commodities: f.commodities,
		writes:      f.writes,
	}
}

// nameTaken reports whether another seller of groupID has the name key of
// seller.
func (r *SellerRegistry) nameTaken(ctx context.Context, groupID string, seller *models.Seller) (bool, error) {
	all, err := r.all.List(ctx)
	if err != nil {
		return false, err
	}
	for _, other := range all {
		if other.GroupID == groupID && other.NameKey == seller.NameKey && other.ID != seller.ID {
			return true, nil
		}
	}
	return false, nil
}

func (r *SellerRegistry) groupFor(seller *models.Seller) string {
	if r.groupID != "" {
		return r.groupID
	}
	return seller.GroupID
}

func (r *SellerRegistry) Create(ctx context.Context, seller models.Seller) (*models.Seller, error) {
	seller.Normalize()
	r.writes.Lock()
	defer r.writes.Unlock()

	taken, err := r.nameTaken(ctx, r.groupFor(&seller), &seller)
	if err != nil {
		return nil, errxtrace.Wrap("failed to check seller name", err)
	}
	if taken {
		return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("name", seller.Name))
	}
	now := time.Now()
	seller.CreatedAt = now
	seller.UpdatedAt = now
	created, err := r.Registry.CreateWithUser(ctx, seller)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create seller", err)
	}
	return created, nil
}

func (r *SellerRegistry) Update(ctx context.Context, seller models.Seller) (*models.Seller, error) {
	seller.Normalize()
	r.writes.Lock()
	defer r.writes.Unlock()

	existing, err := r.Get(ctx, seller.ID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get seller", err)
	}
	taken, err := r.nameTaken(ctx, existing.GroupID, &seller)
	if err != nil {
		return nil, errxtrace.Wrap("failed to check seller name", err)
	}
	if taken {
		return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("name", seller.Name))
	}
	seller.CreatedAt = existing.CreatedAt
	seller.UpdatedAt = time.Now()
	updated, err := r.Registry.UpdateWithUser(ctx, seller)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update seller", err)
	}
	return updated, nil
}

// Delete removes the seller and leaves its items without one.
func (r *SellerRegistry) Delete(ctx context.Context, id string) error {
	if _, err := r.Get(ctx, id); err != nil {
		return errxtrace.Wrap("failed to get seller", err)
	}
	if err := r.Registry.Delete(ctx, id); err != nil {
		return errxtrace.Wrap("failed to delete seller", err)
	}
	if r.commodities == nil {
		return nil
	}
	base := r.commodities.baseCommodityRegistry
	base.lock.Lock()
	defer base.lock.Unlock()
	for pair := base.items.Oldest(); pair != nil; pair = pair.Next() {
		if c := pair.Value; c.SellerID != nil && *c.SellerID == id {
			c.SellerID = nil
		}
	}
	return nil
}

// List returns the sellers sorted by registry.SortSellers.
func (r *SellerRegistry) List(ctx context.Context) ([]*models.Seller, error) {
	sellers, err := r.Registry.List(ctx)
	if err != nil {
		return nil, err
	}
	registry.SortSellers(sellers)
	return sellers, nil
}

func (r *SellerRegistry) GetByName(ctx context.Context, name string) (*models.Seller, error) {
	key := models.SellerNameKey(name)
	sellers, err := r.Registry.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range sellers {
		if s.NameKey == key {
			return s, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "Seller", "name", name))
}

func (r *SellerRegistry) ItemCounts(_ context.Context) (map[string]int, error) {
	counts := map[string]int{}
	if r.commodities == nil {
		return counts, nil
	}
	base := r.commodities.baseCommodityRegistry
	base.lock.RLock()
	defer base.lock.RUnlock()
	for pair := base.items.Oldest(); pair != nil; pair = pair.Next() {
		c := pair.Value
		if c.SellerID == nil || c.IsTrashed() || (r.groupID != "" && c.GroupID != r.groupID) {
			continue
		}
		counts[*c.SellerID]++
	}
	return counts, nil
}
