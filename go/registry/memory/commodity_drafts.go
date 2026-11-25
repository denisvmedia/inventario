package memory

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// CommodityDraftRegistryFactory creates CommodityDraftRegistry instances
// (#2812). All registries it creates share one backing map. Promotion needs
// the commodity and file registries, which postgres reaches with SQL in one
// transaction; the memory backend holds their factories instead.
type CommodityDraftRegistryFactory struct {
	base *Registry[models.CommodityDraft, *models.CommodityDraft]
	// writes serializes the read-check-write sequences (create by client
	// key, save, promote) the way the postgres row lock does.
	writes           *sync.Mutex
	commodityFactory *CommodityRegistryFactory
	fileFactory      *FileRegistryFactory
}

// CommodityDraftRegistry is the context-aware in-memory registry of drafts.
type CommodityDraftRegistry struct {
	*Registry[models.CommodityDraft, *models.CommodityDraft]
	factory *CommodityDraftRegistryFactory
}

var (
	_ registry.CommodityDraftRegistry        = (*CommodityDraftRegistry)(nil)
	_ registry.CommodityDraftRegistryFactory = (*CommodityDraftRegistryFactory)(nil)
)

func NewCommodityDraftRegistryFactory(commodityFactory *CommodityRegistryFactory, fileFactory *FileRegistryFactory) *CommodityDraftRegistryFactory {
	return &CommodityDraftRegistryFactory{
		base:             NewRegistry[models.CommodityDraft, *models.CommodityDraft](),
		writes:           &sync.Mutex{},
		commodityFactory: commodityFactory,
		fileFactory:      fileFactory,
	}
}

func (f *CommodityDraftRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityDraftRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CommodityDraftRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityDraftRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &CommodityDraftRegistry{
		Registry: &Registry[models.CommodityDraft, *models.CommodityDraft]{
			items:   f.base.items,
			lock:    f.base.lock,
			userID:  user.ID,
			groupID: appctx.GroupIDFromContext(ctx),
		},
		factory: f,
	}, nil
}

func (f *CommodityDraftRegistryFactory) CreateServiceRegistry() registry.CommodityDraftRegistry {
	return &CommodityDraftRegistry{
		Registry: &Registry[models.CommodityDraft, *models.CommodityDraft]{
			items: f.base.items,
			lock:  f.base.lock,
		},
		factory: f,
	}
}

// cloneDraftValues copies values through JSON, so the stored map shares
// nothing with the caller's and holds the types a JSONB round trip gives.
func cloneDraftValues(values models.CommodityDraftValues) (models.CommodityDraftValues, error) {
	if values == nil {
		return models.CommodityDraftValues{}, nil
	}
	raw, err := json.Marshal(map[string]any(values))
	if err != nil {
		return nil, err
	}
	out := models.CommodityDraftValues{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *CommodityDraftRegistry) requireGroup() error {
	if r.groupID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	return nil
}

func (r *CommodityDraftRegistry) newDraft(ctx context.Context, draft models.CommodityDraft) (*models.CommodityDraft, error) {
	values, err := cloneDraftValues(draft.Values)
	if err != nil {
		return nil, errxtrace.Wrap("failed to copy draft values", err)
	}
	now := time.Now()
	draft.Values = values
	draft.Status = models.CommodityDraftOpen
	draft.Revision = 1
	draft.PromotedCommodityID = nil
	draft.PromotedAt = nil
	draft.CreatedAt = now
	draft.UpdatedAt = now
	created, err := r.Registry.CreateWithUser(ctx, draft)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity draft", err)
	}
	return created, nil
}

// Create stores a new open draft. A client key the group already has is
// refused with ErrAlreadyExists, as the postgres unique index refuses it.
func (r *CommodityDraftRegistry) Create(ctx context.Context, draft models.CommodityDraft) (*models.CommodityDraft, error) {
	if err := r.requireGroup(); err != nil {
		return nil, err
	}
	r.factory.writes.Lock()
	defer r.factory.writes.Unlock()
	if _, err := r.findByClientKey(draft.ClientKey); err == nil {
		return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("field_name", "client_key"))
	}
	return r.newDraft(ctx, draft)
}

func (r *CommodityDraftRegistry) CreateOrGet(ctx context.Context, draft models.CommodityDraft) (*models.CommodityDraft, bool, error) {
	if err := r.requireGroup(); err != nil {
		return nil, false, err
	}
	r.factory.writes.Lock()
	defer r.factory.writes.Unlock()
	if existing, err := r.findByClientKey(draft.ClientKey); err == nil {
		return existing, false, nil
	}
	created, err := r.newDraft(ctx, draft)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

// findByClientKey returns a copy of the visible draft with the key.
func (r *CommodityDraftRegistry) findByClientKey(clientKey string) (*models.CommodityDraft, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		d := pair.Value
		if r.isItemVisible(d) && d.ClientKey == clientKey {
			return new(*d), nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "commodity_draft", "client_key", clientKey))
}

func (r *CommodityDraftRegistry) GetByClientKey(_ context.Context, clientKey string) (*models.CommodityDraft, error) {
	return r.findByClientKey(clientKey)
}

func (r *CommodityDraftRegistry) Update(ctx context.Context, draft models.CommodityDraft) (*models.CommodityDraft, error) {
	values, err := cloneDraftValues(draft.Values)
	if err != nil {
		return nil, errxtrace.Wrap("failed to copy draft values", err)
	}
	draft.Values = values
	draft.UpdatedAt = time.Now()
	updated, err := r.Registry.UpdateWithUser(ctx, draft)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update commodity draft", err)
	}
	return updated, nil
}

func (r *CommodityDraftRegistry) ListOpen(ctx context.Context, offset, limit int) ([]*models.CommodityDraft, int, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, 0, err
	}
	open := make([]*models.CommodityDraft, 0, len(all))
	for _, d := range all {
		if d.IsOpen() {
			open = append(open, d)
		}
	}
	slices.SortStableFunc(open, func(a, b *models.CommodityDraft) int {
		if c := b.UpdatedAt.Compare(a.UpdatedAt); c != 0 {
			return c
		}
		return strings.Compare(b.ID, a.ID)
	})
	total := len(open)
	offset = min(max(offset, 0), total)
	end := min(offset+max(limit, 0), total)
	return open[offset:end], total, nil
}

// checkOpenRevision mirrors the postgres check.
func checkOpenRevision(draft *models.CommodityDraft, revision int) error {
	if !draft.IsOpen() {
		return errxtrace.Classify(registry.ErrDraftPromoted, errx.Attrs("entity_id", draft.ID))
	}
	if draft.Revision != revision {
		return errxtrace.Classify(registry.ErrDraftRevisionConflict,
			errx.Attrs("entity_id", draft.ID, "revision", draft.Revision, "expected_revision", revision))
	}
	return nil
}

func (r *CommodityDraftRegistry) SaveValues(ctx context.Context, id string, revision int, values models.CommodityDraftValues) (*models.CommodityDraft, error) {
	r.factory.writes.Lock()
	defer r.factory.writes.Unlock()
	draft, err := r.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to save commodity draft", err)
	}
	if err := checkOpenRevision(draft, revision); err != nil {
		return nil, errxtrace.Wrap("failed to save commodity draft", err)
	}
	draft.Values = values
	draft.Revision++
	return r.Update(ctx, *draft)
}

// Promote mirrors the postgres transaction step by step. The memory backend
// has no transaction: a failure after the item is created leaves the item
// and an open draft, which the dev and test uses of this backend accept.
func (r *CommodityDraftRegistry) Promote(ctx context.Context, id string, revision int, commodity models.Commodity) (registry.DraftPromotion, error) {
	if err := r.requireGroup(); err != nil {
		return registry.DraftPromotion{}, err
	}
	r.factory.writes.Lock()
	defer r.factory.writes.Unlock()

	draft, err := r.Get(ctx, id)
	if err != nil {
		return registry.DraftPromotion{}, errxtrace.Wrap("failed to promote commodity draft", err)
	}
	if !draft.IsOpen() {
		return registry.DraftPromotion{Draft: draft}, nil
	}
	if err := checkOpenRevision(draft, revision); err != nil {
		return registry.DraftPromotion{}, errxtrace.Wrap("failed to promote commodity draft", err)
	}

	commodities, err := r.factory.commodityFactory.CreateUserRegistry(ctx)
	if err != nil {
		return registry.DraftPromotion{}, errxtrace.Wrap("failed to create commodity registry", err)
	}
	item, err := commodities.Create(ctx, commodity)
	if err != nil {
		return registry.DraftPromotion{}, errxtrace.Wrap("failed to create commodity", err)
	}

	files, err := r.factory.fileFactory.createUserFileRegistry(ctx)
	if err != nil {
		return registry.DraftPromotion{}, errxtrace.Wrap("failed to create file registry", err)
	}
	files.moveLinks(models.FileLinkDraft, draft.ID, models.FileLinkCommodity, item.ID)

	now := time.Now()
	draft.Status = models.CommodityDraftPromoted
	draft.PromotedCommodityID = new(item.ID)
	draft.PromotedAt = new(now)
	draft.Values = models.CommodityDraftValues{}
	draft.Revision++
	updated, err := r.Update(ctx, *draft)
	if err != nil {
		return registry.DraftPromotion{}, errxtrace.Wrap("failed to mark the draft promoted", err)
	}
	return registry.DraftPromotion{Draft: updated, Commodity: item}, nil
}
