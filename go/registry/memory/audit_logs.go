package memory

import (
	"context"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.AuditLogRegistry = (*AuditLogRegistry)(nil)

type baseAuditLogRegistry = Registry[models.AuditLog, *models.AuditLog]

// AuditLogRegistry is an in-memory implementation of registry.AuditLogRegistry.
// It is used in tests and single-process development deployments.
type AuditLogRegistry struct {
	*baseAuditLogRegistry
}

// NewAuditLogRegistry creates a new in-memory AuditLogRegistry.
func NewAuditLogRegistry() *AuditLogRegistry {
	return &AuditLogRegistry{
		baseAuditLogRegistry: NewRegistry[models.AuditLog, *models.AuditLog](),
	}
}

// Create inserts a new audit log entry, generating an ID and setting the timestamp.
func (r *AuditLogRegistry) Create(ctx context.Context, entry models.AuditLog) (*models.AuditLog, error) {
	if entry.Action == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Action"))
	}

	entry.ID = uuid.New().String()
	entry.Timestamp = time.Now()

	r.lock.Lock()
	r.items.Set(entry.ID, &entry)
	r.lock.Unlock()

	return &entry, nil
}

// ListByUser returns all audit log entries for the given user.
func (r *AuditLogRegistry) ListByUser(ctx context.Context, userID string) ([]*models.AuditLog, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []*models.AuditLog
	for _, e := range all {
		if e.UserID != nil && *e.UserID == userID {
			result = append(result, e)
		}
	}
	return result, nil
}

// ListByTenant returns all audit log entries for the given tenant.
func (r *AuditLogRegistry) ListByTenant(ctx context.Context, tenantID string) ([]*models.AuditLog, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []*models.AuditLog
	for _, e := range all {
		if e.TenantID != nil && *e.TenantID == tenantID {
			result = append(result, e)
		}
	}
	return result, nil
}

// ListByAction returns all audit log entries matching the given action.
func (r *AuditLogRegistry) ListByAction(ctx context.Context, action string) ([]*models.AuditLog, error) {
	if action == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Action"))
	}

	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []*models.AuditLog
	for _, e := range all {
		if e.Action == action {
			result = append(result, e)
		}
	}
	return result, nil
}

// DeleteOlderThan removes all entries whose timestamp is before the given cutoff.
func (r *AuditLogRegistry) DeleteOlderThan(_ context.Context, cutoff time.Time) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.Timestamp.Before(cutoff) {
			r.items.Delete(pair.Key)
		}
	}
	return nil
}
