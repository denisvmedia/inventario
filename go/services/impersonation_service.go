package services

import (
	"context"
	"sync"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/registry"
)

// ImpersonationSlot is the server-side "return slot" recorded when an
// admin starts an impersonation session (#1750). It is keyed by the
// impersonation access token's JTI and holds everything the
// `end` endpoint needs to restore the admin's original session without
// trusting any client-supplied state.
//
// The slot carries the raw refresh-token *value* the admin's session
// was using at start-time — not its hash — so `end` can re-set the
// httpOnly refresh cookie and mint a fresh access token for the admin.
// The raw value is held server-side only transiently: for the lifetime
// of the impersonation session and no longer (it is discarded the moment
// the session ends or expires). It is only ever re-emitted to the
// original operator's browser — and to no other client — via the
// httpOnly `refresh_token` cookie that `end` restores.
type ImpersonationSlot struct {
	// JTI is the impersonation access token's unique id — the slot key.
	JTI string
	// AdminUserID is the operator who initiated the impersonation.
	AdminUserID string
	// AdminTenantID is the operator's tenant. System admins are not
	// tenant-scoped for authorization, but the column is recorded so
	// the audit trail and the restored admin session stay coherent.
	AdminTenantID string
	// AdminRefreshTokenRaw is the raw refresh-token value the admin's
	// session held at start-time. Empty when the admin authenticated
	// without a refresh cookie (e.g. a pure-bearer test client) — in
	// that case `end` mints a brand-new admin session instead.
	AdminRefreshTokenRaw string
	// TargetUserID / TargetTenantID identify the impersonated user.
	TargetUserID   string
	TargetTenantID string
	// Reason is the optional operator-supplied justification.
	Reason string
	// StartedAt / ExpiresAt bound the impersonation session. ExpiresAt
	// mirrors the impersonation access token's exp claim.
	StartedAt time.Time
	ExpiresAt time.Time
}

// ImpersonationStore records and resolves impersonation return slots.
// The single implementation is in-memory: a slot lives for at most the
// impersonation TTL (≤ 30 min) and a process restart simply forces the
// admin to log in again — an acceptable trade-off that avoids a schema
// migration for short-lived ephemeral state. Multi-replica deployments
// must run a shared implementation; the interface exists so one can be
// dropped in without touching the handlers.
type ImpersonationStore interface {
	// Put records a new slot keyed by slot.JTI. An existing slot with
	// the same JTI is overwritten — JTIs are UUIDs so a collision is a
	// programming error, not a normal branch.
	Put(ctx context.Context, slot ImpersonationSlot) error

	// Get returns the slot for the given JTI. Returns registry.ErrNotFound
	// when no live slot exists (never recorded, already ended, or expired).
	Get(ctx context.Context, jti string) (ImpersonationSlot, error)

	// Delete removes the slot for the given JTI. Idempotent: deleting a
	// missing slot is not an error so a double `end` call is harmless.
	Delete(ctx context.Context, jti string) error
}

// InMemoryImpersonationStore is the default ImpersonationStore. Slots
// are pruned lazily on every access so an abandoned session (admin
// never calls `end`) cannot leak memory beyond one TTL window.
type InMemoryImpersonationStore struct {
	now func() time.Time

	mu    sync.Mutex
	slots map[string]ImpersonationSlot
}

// NewInMemoryImpersonationStore creates an empty in-memory store.
func NewInMemoryImpersonationStore() *InMemoryImpersonationStore {
	return &InMemoryImpersonationStore{
		now:   time.Now,
		slots: make(map[string]ImpersonationSlot),
	}
}

// Put records the slot keyed by slot.JTI, overwriting any existing slot
// with the same JTI. Expired slots are pruned first so the map stays
// bounded by the number of live impersonation sessions. See
// ImpersonationStore.Put.
func (s *InMemoryImpersonationStore) Put(_ context.Context, slot ImpersonationSlot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	s.slots[slot.JTI] = slot
	return nil
}

// Get returns the live slot for the given JTI, or registry.ErrNotFound
// when no slot exists — never recorded, already ended, or pruned because
// it expired. See ImpersonationStore.Get.
func (s *InMemoryImpersonationStore) Get(_ context.Context, jti string) (ImpersonationSlot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	slot, ok := s.slots[jti]
	if !ok {
		return ImpersonationSlot{}, errxtrace.Classify(registry.ErrNotFound)
	}
	return slot, nil
}

// Delete removes the slot for the given JTI. Idempotent: deleting a
// missing slot is a no-op, so a double `end` call is harmless. See
// ImpersonationStore.Delete.
func (s *InMemoryImpersonationStore) Delete(_ context.Context, jti string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.slots, jti)
	return nil
}

// pruneLocked evicts every slot whose ExpiresAt is in the past. The
// caller must hold s.mu. Called on Put/Get so memory stays bounded by
// the number of *live* impersonation sessions regardless of how many
// were abandoned without an explicit `end`.
func (s *InMemoryImpersonationStore) pruneLocked() {
	now := s.now()
	for jti, slot := range s.slots {
		if !slot.ExpiresAt.IsZero() && now.After(slot.ExpiresAt) {
			delete(s.slots, jti)
		}
	}
}
