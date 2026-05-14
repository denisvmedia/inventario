package memory

import (
	"context"
	"sort"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.LoginEventRegistry = (*LoginEventRegistry)(nil)

type baseLoginEventRegistry = Registry[models.LoginEvent, *models.LoginEvent]

// LoginEventRegistry is the in-memory store for login_events. Mirrors
// the postgres implementation closely so behaviour is the same in
// tests and e2e — the only divergence is the lack of indexes (linear
// scans are fine for the row counts test fixtures touch).
type LoginEventRegistry struct {
	*baseLoginEventRegistry
}

func NewLoginEventRegistry() *LoginEventRegistry {
	return &LoginEventRegistry{
		baseLoginEventRegistry: NewRegistry[models.LoginEvent, *models.LoginEvent](),
	}
}

// Update is a no-op that returns the input unchanged — login_events is
// append-only by design, same shape as CommodityEventRegistry.Update. This
// keeps memory-mode behaviour aligned with the postgres registry so a
// stray Update call from a test or dev fixture can't mutate the audit
// trail.
func (r *LoginEventRegistry) Update(_ context.Context, event models.LoginEvent) (*models.LoginEvent, error) {
	return &event, nil
}

// Create inserts a new login_event. The write side runs out of any
// user context (we don't always have an authenticated user — failed
// logins for unknown emails for example) so we bypass the Registry's
// userID guard and write directly.
func (r *LoginEventRegistry) Create(_ context.Context, event models.LoginEvent) (*models.LoginEvent, error) {
	if event.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if event.Email == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}
	if event.Outcome == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Outcome"))
	}

	event.ID = uuid.New().String()
	if event.UUID == "" {
		event.UUID = uuid.New().String()
	}
	if event.Method == "" {
		event.Method = models.LoginMethodPassword
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	r.lock.Lock()
	r.items.Set(event.ID, &event)
	r.lock.Unlock()

	return &event, nil
}

// ListByUser returns the most recent login events for the user inside
// the tenant. Empty user OR empty tenant yields an empty list — NULL
// user_id rows (failed unknown-email attempts) are intentionally not
// returned by the user-facing list, and a tenant mismatch must never
// leak rows across tenants even when the user_id happens to match.
func (r *LoginEventRegistry) ListByUser(ctx context.Context, tenantID, userID string, limit int) ([]*models.LoginEvent, error) {
	if userID == "" || tenantID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}

	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	matched := make([]*models.LoginEvent, 0, len(all))
	for _, e := range all {
		if e.TenantID != tenantID {
			continue
		}
		if e.UserID != nil && *e.UserID == userID {
			matched = append(matched, e)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

// CountFailedSince returns the number of non-ok events for the user
// inside the tenant since `since`. Mirrors the postgres
// `outcome <> 'ok' AND tenant_id = $tenant` predicate.
func (r *LoginEventRegistry) CountFailedSince(ctx context.Context, tenantID, userID string, since time.Time) (int, error) {
	if userID == "" || tenantID == "" {
		return 0, nil
	}
	all, err := r.List(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range all {
		if e.TenantID != tenantID {
			continue
		}
		if e.UserID == nil || *e.UserID != userID {
			continue
		}
		if e.Outcome == models.LoginOutcomeOK {
			continue
		}
		if e.CreatedAt.Before(since) {
			continue
		}
		count++
	}
	return count, nil
}

// DeleteOlderThan removes login_events whose CreatedAt is strictly
// before cutoff. Returns the number of rows deleted so callers can
// log a single "N rows pruned" line per tick.
func (r *LoginEventRegistry) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int, error) {
	all, err := r.List(ctx)
	if err != nil {
		return 0, err
	}
	deleted := 0
	r.lock.Lock()
	for _, e := range all {
		if e.CreatedAt.Before(cutoff) {
			r.items.Delete(e.ID)
			deleted++
		}
	}
	r.lock.Unlock()
	return deleted, nil
}
