package memory

import (
	"context"
	"sync"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.CommodityScanAuditRegistry = (*CommodityScanAuditRegistry)(nil)

// CommodityScanAuditRegistry is the in-memory implementation of
// registry.CommodityScanAuditRegistry. It is used in unit tests and the
// memory:// backend; the postgres implementation lives in the postgres
// package.
type CommodityScanAuditRegistry struct {
	mu    sync.RWMutex
	items map[string]*models.CommodityScanAudit
}

// NewCommodityScanAuditRegistry constructs an empty registry.
func NewCommodityScanAuditRegistry() *CommodityScanAuditRegistry {
	return &CommodityScanAuditRegistry{
		items: make(map[string]*models.CommodityScanAudit),
	}
}

// Record stores a new audit entry, generating ID/UUID and CreatedAt
// when the caller did not preset them.
func (r *CommodityScanAuditRegistry) Record(_ context.Context, audit models.CommodityScanAudit) (*models.CommodityScanAudit, error) {
	if audit.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if audit.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if audit.Status == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Status"))
	}

	audit.ID = uuid.New().String()
	if audit.UUID == "" {
		audit.UUID = uuid.New().String()
	}
	if audit.CreatedAt.IsZero() {
		audit.CreatedAt = time.Now()
	}

	r.mu.Lock()
	stored := audit
	r.items[stored.ID] = &stored
	r.mu.Unlock()

	return &stored, nil
}

// CountRecentForUser counts audit rows for (tenantID, userID) created
// at or after since.
func (r *CommodityScanAuditRegistry) CountRecentForUser(_ context.Context, tenantID, userID string, since time.Time) (int, error) {
	if tenantID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, item := range r.items {
		if item.TenantID != tenantID {
			continue
		}
		if item.UserID != userID {
			continue
		}
		if item.CreatedAt.Before(since) {
			continue
		}
		count++
	}
	return count, nil
}

// DeleteOlderThan removes every audit row older than cutoff.
func (r *CommodityScanAuditRegistry) DeleteOlderThan(_ context.Context, cutoff time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, item := range r.items {
		if item.CreatedAt.Before(cutoff) {
			delete(r.items, id)
		}
	}
	return nil
}
