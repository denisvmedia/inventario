package memory

import (
	"cmp"
	"context"
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

// IntakeDocumentRegistryFactory creates IntakeDocumentRegistry instances
// (#2809). All registries it creates share one backing map. writes stands in
// for the unique index and the row locks of postgres.
type IntakeDocumentRegistryFactory struct {
	base   *Registry[models.IntakeDocument, *models.IntakeDocument]
	writes *sync.Mutex
	// drafts is set by CommodityDraftRegistryFactory.SetIntakeDocumentFactory.
	// Recording a draft reads whether it is promoted.
	drafts *CommodityDraftRegistryFactory
}

// IntakeDocumentRegistry is the context-aware in-memory registry of the
// documents other systems delivered.
type IntakeDocumentRegistry struct {
	*Registry[models.IntakeDocument, *models.IntakeDocument]
	factory *IntakeDocumentRegistryFactory
	service bool
}

var (
	_ registry.IntakeDocumentRegistry        = (*IntakeDocumentRegistry)(nil)
	_ registry.IntakeDocumentRegistryFactory = (*IntakeDocumentRegistryFactory)(nil)
)

func NewIntakeDocumentRegistryFactory() *IntakeDocumentRegistryFactory {
	return &IntakeDocumentRegistryFactory{
		base:   NewRegistry[models.IntakeDocument, *models.IntakeDocument](),
		writes: &sync.Mutex{},
	}
}

func (f *IntakeDocumentRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.IntakeDocumentRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *IntakeDocumentRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.IntakeDocumentRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &IntakeDocumentRegistry{
		Registry: &Registry[models.IntakeDocument, *models.IntakeDocument]{
			items:   f.base.items,
			lock:    f.base.lock,
			userID:  user.ID,
			groupID: appctx.GroupIDFromContext(ctx),
		},
		factory: f,
	}, nil
}

func (f *IntakeDocumentRegistryFactory) CreateServiceRegistry() registry.IntakeDocumentRegistry {
	return &IntakeDocumentRegistry{
		Registry: &Registry[models.IntakeDocument, *models.IntakeDocument]{
			items: f.base.items,
			lock:  f.base.lock,
		},
		factory: f,
		service: true,
	}
}

// promoted records the item a draft became on the document the draft came
// from, as the postgres promotion does in its transaction: the one that
// names the draft, or the one whose identity gives the key of a delivered
// draft and that names a session and no draft yet. The caller holds the
// draft factory's writes lock.
func (f *IntakeDocumentRegistryFactory) promoted(groupID, draftID, draftKey, commodityID string, now time.Time) {
	f.writes.Lock()
	defer f.writes.Unlock()
	f.base.lock.Lock()
	defer f.base.lock.Unlock()
	keyed := strings.HasPrefix(draftKey, models.IntakeDeliveredDraftKeyPrefix)
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		doc := pair.Value
		if doc.GroupID != groupID {
			continue
		}
		names := doc.DraftID != nil && *doc.DraftID == draftID
		unrecorded := doc.DraftID == nil && doc.SessionID != nil && keyed && doc.DraftClientKey() == draftKey
		if names || unrecorded {
			cp := *doc
			cp.DraftID = new(draftID)
			cp.CommodityID = new(commodityID)
			cp.UpdatedAt = now
			pair.Value = &cp
		}
	}
}

// groupsCreatedBy returns the groups with a document userID created in
// tenantID.
func (f *IntakeDocumentRegistryFactory) groupsCreatedBy(tenantID, userID string) []string {
	f.base.lock.RLock()
	defer f.base.lock.RUnlock()
	seen := map[string]bool{}
	var out []string
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		doc := pair.Value
		if doc.TenantID == tenantID && doc.CreatedByUserID == userID && !seen[doc.GroupID] {
			seen[doc.GroupID] = true
			out = append(out, doc.GroupID)
		}
	}
	return out
}

// reattributeCreator gives the documents userID created in tenantID to
// heirs[groupID], as the user purge does with group configuration, returns
// how many each group handed over, and deletes the ones whose group has no
// heir: that group leaves with the user.
func (f *IntakeDocumentRegistryFactory) reattributeCreator(tenantID, userID string, heirs map[string]string) map[string]int {
	f.writes.Lock()
	defer f.writes.Unlock()
	f.base.lock.Lock()
	defer f.base.lock.Unlock()
	var gone []string
	moved := map[string]int{}
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		doc := pair.Value
		if doc.TenantID != tenantID || doc.CreatedByUserID != userID {
			continue
		}
		if heir := heirs[doc.GroupID]; heir != "" {
			cp := *doc
			cp.CreatedByUserID = heir
			pair.Value = &cp
			moved[doc.GroupID]++
			continue
		}
		gone = append(gone, doc.ID)
	}
	for _, id := range gone {
		f.base.items.Delete(id)
	}
	return moved
}

func (r *IntakeDocumentRegistry) requireGroup() error {
	if r.service || r.groupID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	return nil
}

func intakeDocumentNotFound(id string) error {
	return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "intake_document", "entity_id", id))
}

// find returns the stored document of the group with the identity. The caller
// holds the base lock.
func (r *IntakeDocumentRegistry) find(system, instance, documentID string) *models.IntakeDocument {
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		doc := pair.Value
		if doc.GroupID == r.groupID && doc.ExternalSystem == system && doc.ExternalInstance == instance && doc.ExternalDocumentID == documentID {
			return doc
		}
	}
	return nil
}

// Create stores the document; a group that already has its identity gets
// ErrAlreadyExists, as the unique index gives in postgres.
func (r *IntakeDocumentRegistry) Create(ctx context.Context, doc models.IntakeDocument) (*models.IntakeDocument, error) {
	stored, created, err := r.CreateOrGet(ctx, doc)
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("entity_type", "intake_document", "entity_id", stored.ID))
	}
	return stored, nil
}

func (r *IntakeDocumentRegistry) CreateOrGet(ctx context.Context, doc models.IntakeDocument) (*models.IntakeDocument, bool, error) {
	if err := r.requireGroup(); err != nil {
		return nil, false, err
	}
	if err := doc.ValidateFields(ctx); err != nil {
		return nil, false, errxtrace.Wrap("invalid intake document", err)
	}
	r.factory.writes.Lock()
	defer r.factory.writes.Unlock()

	r.lock.RLock()
	existing := r.find(doc.ExternalSystem, doc.ExternalInstance, doc.ExternalDocumentID)
	r.lock.RUnlock()
	if existing != nil {
		return new(*existing), false, nil
	}

	now := time.Now().UTC()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = doc.CreatedAt
	}
	stored, err := r.Registry.CreateWithUser(ctx, doc)
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to create intake document", err)
	}
	return new(*stored), true, nil
}

func (r *IntakeDocumentRegistry) GetByExternalID(_ context.Context, system, instance, documentID string) (*models.IntakeDocument, error) {
	if err := r.requireGroup(); err != nil {
		return nil, err
	}
	r.lock.RLock()
	defer r.lock.RUnlock()
	if doc := r.find(system, instance, documentID); doc != nil {
		return new(*doc), nil
	}
	return nil, intakeDocumentNotFound(documentID)
}

func (r *IntakeDocumentRegistry) RecordProgress(_ context.Context, id string, progress registry.IntakeDocumentProgress) (*models.IntakeDocument, error) {
	if err := r.requireGroup(); err != nil {
		return nil, err
	}
	// The draft factory's writes lock comes first, as in a promotion: a
	// promotion either finished, and the draft names its item, or starts
	// after this call and finds the draft recorded.
	var promotedItem *string
	if drafts := r.factory.drafts; progress.DraftID != nil && drafts != nil {
		drafts.writes.Lock()
		defer drafts.writes.Unlock()
		promotedItem = drafts.promotedItem(r.groupID, *progress.DraftID)
	}
	r.factory.writes.Lock()
	defer r.factory.writes.Unlock()
	r.lock.Lock()
	defer r.lock.Unlock()
	stored, ok := r.items.Get(id)
	if !ok || stored.GroupID != r.groupID {
		return nil, intakeDocumentNotFound(id)
	}
	// A copy takes the change: a reader may still hold the stored document.
	doc := new(*stored)
	setsDraft := doc.DraftID == nil && progress.DraftID != nil
	changed := false
	for _, f := range []struct {
		dst **string
		src *string
	}{
		{&doc.FileID, progress.FileID},
		{&doc.SessionID, progress.SessionID},
		{&doc.DraftID, progress.DraftID},
		{&doc.CommodityID, progress.CommodityID},
	} {
		if *f.dst == nil && f.src != nil {
			*f.dst = new(*f.src)
			changed = true
		}
	}
	if setsDraft && promotedItem != nil && doc.CommodityID == nil {
		doc.CommodityID = new(*promotedItem)
	}
	if changed {
		doc.UpdatedAt = time.Now().UTC()
	}
	r.items.Set(id, doc)
	return new(*doc), nil
}

func (r *IntakeDocumentRegistry) ListByDrafts(_ context.Context, draftIDs ...string) ([]*models.IntakeDocument, error) {
	return r.listBy(draftIDs, func(d *models.IntakeDocument) *string { return d.DraftID })
}

func (r *IntakeDocumentRegistry) ListByCommodities(_ context.Context, commodityIDs ...string) ([]*models.IntakeDocument, error) {
	return r.listBy(commodityIDs, func(d *models.IntakeDocument) *string { return d.CommodityID })
}

func (r *IntakeDocumentRegistry) listBy(ids []string, field func(*models.IntakeDocument) *string) ([]*models.IntakeDocument, error) {
	if err := r.requireGroup(); err != nil {
		return nil, err
	}
	out := []*models.IntakeDocument{}
	if len(ids) == 0 {
		return out, nil
	}
	r.lock.RLock()
	defer r.lock.RUnlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		doc := pair.Value
		if v := field(doc); doc.GroupID == r.groupID && v != nil && slices.Contains(ids, *v) {
			out = append(out, new(*doc))
		}
	}
	slices.SortStableFunc(out, func(a, b *models.IntakeDocument) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}
