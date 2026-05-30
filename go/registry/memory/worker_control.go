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

var _ registry.WorkerControlRegistry = (*WorkerControlRegistry)(nil)

// WorkerControlRegistry is the in-memory implementation of the
// background-worker soft-pause control store (#1308). All operations are
// guarded by a single per-registry mutex: the postgres impl serialises
// the pause/resume upsert via the unique (worker_type) index, so the
// in-memory equivalent must serialise too to keep the idempotency
// contract race-free. nowFn/uuidFn are injectable so tests get
// deterministic timestamps and IDs.
type WorkerControlRegistry struct {
	lock sync.Mutex
	// items is keyed by worker_type (the natural key), mirroring the SQL
	// unique index — at most one control row per worker type.
	items  map[string]*models.WorkerControl
	nowFn  func() time.Time
	uuidFn func() string
}

// NewWorkerControlRegistry creates a new in-memory WorkerControlRegistry.
func NewWorkerControlRegistry() *WorkerControlRegistry {
	return &WorkerControlRegistry{
		items:  make(map[string]*models.WorkerControl),
		nowFn:  func() time.Time { return time.Now().UTC() },
		uuidFn: func() string { return uuid.New().String() },
	}
}

// NewWorkerControlRegistryForTesting builds a registry with injected
// clock and id generators so tests get deterministic paused_at /
// updated_at timestamps and stable ids. Production code must use
// NewWorkerControlRegistry.
func NewWorkerControlRegistryForTesting(nowFn func() time.Time, uuidFn func() string) *WorkerControlRegistry {
	return &WorkerControlRegistry{
		items:  make(map[string]*models.WorkerControl),
		nowFn:  nowFn,
		uuidFn: uuidFn,
	}
}

// List returns every worker_control row ordered by worker_type. Returns
// defensive copies so a caller mutating the slice can't corrupt registry
// state.
func (r *WorkerControlRegistry) List(_ context.Context) ([]*models.WorkerControl, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	out := make([]*models.WorkerControl, 0, len(r.items))
	for _, wc := range r.items {
		out = append(out, cloneWorkerControl(wc))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].WorkerType < out[j].WorkerType
	})
	return out, nil
}

// Pause idempotently marks workerType paused. pausedBy/reason are stored
// as NULL when empty. Re-pausing an already-paused type updates
// paused_by/reason but PRESERVES the original paused_at — same semantics
// as the postgres CASE expression.
func (r *WorkerControlRegistry) Pause(_ context.Context, workerType, pausedBy, reason string) (*models.WorkerControl, error) {
	if workerType == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired)
	}
	// Defence-in-depth: the handler/CLI already reject unknown types, but
	// the registry is the storage authority — never write a control row
	// for a worker that doesn't exist.
	if !models.WorkerType(workerType).IsValid() {
		return nil, errxtrace.Classify(registry.ErrInvalidInput, errx.Attrs("worker_type", workerType))
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	now := r.nowFn()

	existing, ok := r.items[workerType]
	if ok && existing.Paused {
		// Already paused: update by/reason, preserve the original
		// paused_at, bump updated_at.
		existing.PausedBy = optionalString(pausedBy)
		existing.Reason = optionalString(reason)
		existing.UpdatedAt = now
		return cloneWorkerControl(existing), nil
	}

	pausedAt := now
	wc := &models.WorkerControl{
		EntityID: models.EntityID{
			ID:   r.uuidFn(),
			UUID: r.uuidFn(),
		},
		WorkerType: models.WorkerType(workerType),
		Paused:     true,
		PausedBy:   optionalString(pausedBy),
		PausedAt:   &pausedAt,
		Reason:     optionalString(reason),
		UpdatedAt:  now,
	}
	if ok {
		// A row existed but was not paused — reuse its identity so the
		// id/uuid stay stable across pause/resume cycles, mirroring the
		// postgres upsert which keeps the same row.
		wc.EntityID = existing.EntityID
	}
	r.items[workerType] = wc
	return cloneWorkerControl(wc), nil
}

// Resume idempotently marks workerType not paused, clearing
// paused_at/paused_by/reason. When no row exists it is a no-op and
// returns a synthetic not-paused WorkerControl (no row created).
func (r *WorkerControlRegistry) Resume(_ context.Context, workerType string) (*models.WorkerControl, error) {
	if workerType == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired)
	}
	if !models.WorkerType(workerType).IsValid() {
		return nil, errxtrace.Classify(registry.ErrInvalidInput, errx.Attrs("worker_type", workerType))
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	existing, ok := r.items[workerType]
	if !ok {
		// Already running — synthesize a not-paused state without
		// creating a row (matches the postgres no-op behaviour).
		return &models.WorkerControl{WorkerType: models.WorkerType(workerType)}, nil
	}

	existing.Paused = false
	existing.PausedBy = nil
	existing.PausedAt = nil
	existing.Reason = nil
	existing.UpdatedAt = r.nowFn()
	return cloneWorkerControl(existing), nil
}

// optionalString maps an empty string to a nil *string (stored as NULL)
// and a non-empty value to a fresh pointer the caller can't alias.
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	cp := s
	return &cp
}

// cloneWorkerControl deep-copies a control row, duplicating the nullable
// pointer fields so a caller writing through the returned pointer can't
// reach the stored row.
func cloneWorkerControl(wc *models.WorkerControl) *models.WorkerControl {
	cp := *wc
	if wc.PausedBy != nil {
		v := *wc.PausedBy
		cp.PausedBy = &v
	}
	if wc.PausedAt != nil {
		v := *wc.PausedAt
		cp.PausedAt = &v
	}
	if wc.Reason != nil {
		v := *wc.Reason
		cp.Reason = &v
	}
	return &cp
}
