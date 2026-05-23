package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.SystemAdminGrantRegistry = (*SystemAdminGrantRegistry)(nil)

// SystemAdminGrantRegistry is the in-memory implementation of the
// system-admin grant store (#1784). It guards all mutating operations
// with a single per-registry mutex: the postgres impl serialises via
// pg_advisory_xact_lock('system_admin_mutations'), so the in-memory
// equivalent must serialise too — otherwise tests that exercise the
// last-admin invariant under concurrent revokes would race and
// occasionally let the grant set drop to zero.
type SystemAdminGrantRegistry struct {
	lock sync.Mutex
	// items is keyed by grant ID. The unique invariant lives on user_id
	// instead (mirroring the SQL unique index); a tiny secondary map
	// keeps the lookup constant-time without an O(N) scan.
	items    map[string]*models.SystemAdminGrant
	byUserID map[string]string // user_id -> grant id
	nowFn    func() time.Time
	uuidFn   func() string
}

// NewSystemAdminGrantRegistry creates a new in-memory SystemAdminGrantRegistry.
func NewSystemAdminGrantRegistry() *SystemAdminGrantRegistry {
	return &SystemAdminGrantRegistry{
		items:    make(map[string]*models.SystemAdminGrant),
		byUserID: make(map[string]string),
		nowFn:    func() time.Time { return time.Now().UTC() },
		uuidFn:   func() string { return uuid.New().String() },
	}
}

// Exists returns true when the user has a grant row. Hot path —
// called from RequireSystemAdmin on every /api/v1/admin/* request.
func (r *SystemAdminGrantRegistry) Exists(_ context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	_, ok := r.byUserID[userID]
	return ok, nil
}

// Grant inserts a grant row. Idempotent: when the user is already a
// system admin, returns (true, nil) and does not mutate the row.
func (r *SystemAdminGrantRegistry) Grant(_ context.Context, userID string, grantedBy *string) (hadGrant bool, err error) {
	if userID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	if _, ok := r.byUserID[userID]; ok {
		return true, nil
	}

	// Deep-copy grantedBy so the caller can mutate the variable they
	// passed in (or even re-use the same address across calls) without
	// rippling into the registry's stored row. The postgres impl gets
	// this for free via INSERT param marshalling — the memory backend
	// must do it explicitly.
	var storedGrantedBy *string
	if grantedBy != nil {
		copyOf := *grantedBy
		storedGrantedBy = &copyOf
	}

	grant := &models.SystemAdminGrant{
		EntityID: models.EntityID{
			ID:   r.uuidFn(),
			UUID: r.uuidFn(),
		},
		UserID:    userID,
		GrantedBy: storedGrantedBy,
		GrantedAt: r.nowFn(),
	}
	r.items[grant.ID] = grant
	r.byUserID[userID] = grant.ID
	return false, nil
}

// RevokeAtomic removes the grant row, serialising against concurrent
// revokes via the registry mutex. With allowZero=false, refuses to
// drop the last remaining grant — returns ErrLastSystemAdmin. The
// memory backend has no transactions; the mutex is the equivalent
// boundary the postgres impl gets from pg_advisory_xact_lock.
//
//revive:disable-next-line:flag-parameter
func (r *SystemAdminGrantRegistry) RevokeAtomic(_ context.Context, userID string, allowZero bool) (hadGrant bool, err error) {
	if userID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	grantID, ok := r.byUserID[userID]
	if !ok {
		// Idempotent: no grant, nothing to do.
		return false, nil
	}

	if !allowZero && len(r.items) <= 1 {
		return true, errxtrace.Classify(registry.ErrLastSystemAdmin, errx.Attrs("user_id", userID))
	}

	delete(r.items, grantID)
	delete(r.byUserID, userID)
	return true, nil
}

// List returns every grant row, ordered by (granted_at ASC, user_id ASC).
// The user_id secondary key gives callers a stable, deterministic
// iteration order when two grants share a granted_at timestamp — without
// it, fast-fired CLI grants could tie on `now()` resolution and the
// rendered list would shuffle between calls. The postgres impl applies
// the same composite ORDER BY so both backends agree.
func (r *SystemAdminGrantRegistry) List(_ context.Context) ([]*models.SystemAdminGrant, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	out := make([]*models.SystemAdminGrant, 0, len(r.items))
	for _, g := range r.items {
		// Defensive copy so a caller mutating the returned slice can't
		// corrupt registry state. The struct copy duplicates value
		// fields but `GrantedBy *string` still aliases the registry's
		// pointer — copy the pointee too so a caller writing through
		// the returned pointer can't reach the stored row.
		cp := *g
		if g.GrantedBy != nil {
			copyOf := *g.GrantedBy
			cp.GrantedBy = &copyOf
		}
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].GrantedAt.Equal(out[j].GrantedAt) {
			return out[i].UserID < out[j].UserID
		}
		return out[i].GrantedAt.Before(out[j].GrantedAt)
	})
	return out, nil
}
