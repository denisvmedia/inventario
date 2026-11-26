package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

var _ registry.PersonalDataExportRegistry = (*PersonalDataExportRegistry)(nil)

// PersonalDataExportRegistry is the in-memory registry.PersonalDataExportRegistry.
// Rows are stored by value and copied on the way out.
type PersonalDataExportRegistry struct {
	mu      sync.Mutex
	exports map[string]models.PersonalDataExport
}

// NewPersonalDataExportRegistry returns an empty registry.
func NewPersonalDataExportRegistry() *PersonalDataExportRegistry {
	return &PersonalDataExportRegistry{exports: make(map[string]models.PersonalDataExport)}
}

func validatePersonalDataExportOwner(export models.PersonalDataExport) error {
	switch {
	case export.UserID == "":
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	case export.TenantID == "":
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	case export.ExpiresAt.IsZero():
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ExpiresAt"))
	}
	return nil
}

func (r *PersonalDataExportRegistry) CreateUnlessRequestedSince(_ context.Context, export models.PersonalDataExport, since time.Time) (created, blocking *models.PersonalDataExport, err error) {
	if err := validatePersonalDataExportOwner(export); err != nil {
		return nil, nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	var latest *models.PersonalDataExport
	for _, e := range r.exports {
		if e.UserID != export.UserID || !e.CreatedAt.After(since) {
			continue
		}
		if latest == nil || e.CreatedAt.After(latest.CreatedAt) {
			latest = new(e)
		}
	}
	if latest != nil {
		return nil, latest, nil
	}

	export.ID = uuid.New().String()
	export.UUID = uuid.New().String()
	if export.CreatedAt.IsZero() {
		export.CreatedAt = time.Now()
	}
	export.Status = models.PersonalDataExportStatusPending
	r.exports[export.ID] = export
	return new(export), nil, nil
}

func (r *PersonalDataExportRegistry) Get(_ context.Context, id string) (*models.PersonalDataExport, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.exports[id]
	if !ok {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "PersonalDataExport", "entity_id", id))
	}
	return new(e), nil
}

func (r *PersonalDataExportRegistry) GetByUser(_ context.Context, userID, id string) (*models.PersonalDataExport, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.exports[id]
	if !ok || e.UserID != userID {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "PersonalDataExport", "entity_id", id))
	}
	return new(e), nil
}

func (r *PersonalDataExportRegistry) ListByUserID(_ context.Context, userID string) ([]*models.PersonalDataExport, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	return r.list(func(e *models.PersonalDataExport) bool { return e.UserID == userID }, func(a, b *models.PersonalDataExport) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(a.ID, b.ID))
	}), nil
}

func (r *PersonalDataExportRegistry) ListPending(_ context.Context) ([]*models.PersonalDataExport, error) {
	return r.list(func(e *models.PersonalDataExport) bool { return e.Status == models.PersonalDataExportStatusPending }, func(a, b *models.PersonalDataExport) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID, b.ID))
	}), nil
}

func (r *PersonalDataExportRegistry) ListExpired(_ context.Context, now time.Time, limit int) ([]*models.PersonalDataExport, error) {
	out := r.list(func(e *models.PersonalDataExport) bool { return e.IsExpired(now) }, func(a, b *models.PersonalDataExport) int {
		return cmp.Or(a.ExpiresAt.Compare(b.ExpiresAt), cmp.Compare(a.ID, b.ID))
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *PersonalDataExportRegistry) list(keep func(*models.PersonalDataExport) bool, order func(a, b *models.PersonalDataExport) int) []*models.PersonalDataExport {
	r.mu.Lock()
	out := make([]*models.PersonalDataExport, 0)
	for _, e := range r.exports {
		if keep(&e) {
			out = append(out, new(e))
		}
	}
	r.mu.Unlock()
	slices.SortFunc(out, order)
	return out
}

func (r *PersonalDataExportRegistry) ClaimPending(_ context.Context, id string) (bool, error) {
	if id == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.exports[id]
	if !ok || e.Status != models.PersonalDataExportStatusPending {
		return false, nil
	}
	e.Status = models.PersonalDataExportStatusInProgress
	r.exports[id] = e
	return true, nil
}

func (r *PersonalDataExportRegistry) Update(_ context.Context, export models.PersonalDataExport) (*models.PersonalDataExport, error) {
	if export.ID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if !export.Status.IsValid() {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Status"))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.exports[export.ID]
	if !ok {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "PersonalDataExport", "entity_id", export.ID))
	}
	stored.Status = export.Status
	stored.BlobKey = export.BlobKey
	stored.SizeBytes = export.SizeBytes
	stored.GroupCount = export.GroupCount
	stored.ItemCount = export.ItemCount
	stored.FileCount = export.FileCount
	stored.ErrorMessage = export.ErrorMessage
	stored.CompletedAt = export.CompletedAt
	stored.ExpiresAt = export.ExpiresAt
	r.exports[export.ID] = stored
	return new(stored), nil
}

func (r *PersonalDataExportRegistry) Delete(_ context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.exports, id)
	return nil
}

// DeleteByUserID hard-deletes every export of the user. Memory-only: the
// postgres purgers delete by table name in SQL.
func (r *PersonalDataExportRegistry) DeleteByUserID(_ context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, e := range r.exports {
		if e.UserID == userID {
			delete(r.exports, id)
		}
	}
	return nil
}

// DeleteByTenantID hard-deletes every export in the tenant. Memory-only, like
// DeleteByUserID.
func (r *PersonalDataExportRegistry) DeleteByTenantID(_ context.Context, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, e := range r.exports {
		if e.TenantID == tenantID {
			delete(r.exports, id)
		}
	}
	return nil
}
