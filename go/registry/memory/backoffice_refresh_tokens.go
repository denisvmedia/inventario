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

var _ registry.BackofficeRefreshTokenRegistry = (*BackofficeRefreshTokenRegistry)(nil)

type baseBackofficeRefreshTokenRegistry = Registry[models.BackofficeRefreshToken, *models.BackofficeRefreshToken]

// BackofficeRefreshTokenRegistry is the in-memory implementation of
// the back-office refresh-token store (issue #1785, Phase 2). It mirrors
// the surface of the tenant-side memory RefreshTokenRegistry — same
// method semantics, same locking model — but is wired to BackofficeUser
// rather than (Tenant, User), so the two identity universes can't share
// a row.
type BackofficeRefreshTokenRegistry struct {
	*baseBackofficeRefreshTokenRegistry
}

// NewBackofficeRefreshTokenRegistry builds an empty in-memory store.
func NewBackofficeRefreshTokenRegistry() *BackofficeRefreshTokenRegistry {
	return &BackofficeRefreshTokenRegistry{
		baseBackofficeRefreshTokenRegistry: NewRegistry[models.BackofficeRefreshToken, *models.BackofficeRefreshToken](),
	}
}

// Create stamps id + created_at and inserts the row. The TokenHash and
// BackofficeUserID fields are required; both are rejected with
// ErrFieldRequired so tests that forget to populate them fail with the
// same shape as the tenant-side equivalent.
func (r *BackofficeRefreshTokenRegistry) Create(_ context.Context, token models.BackofficeRefreshToken) (*models.BackofficeRefreshToken, error) {
	if token.TokenHash == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TokenHash"))
	}
	if token.BackofficeUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}

	token.ID = uuid.New().String()
	if token.UUID == "" {
		token.UUID = uuid.New().String()
	}
	token.CreatedAt = time.Now()

	r.lock.Lock()
	r.items.Set(token.ID, &token)
	r.lock.Unlock()

	return &token, nil
}

// GetByHash returns the refresh-token row matching the supplied SHA-256
// hash (or ErrBackofficeRefreshTokenNotFound). Linear scan is fine for
// the in-memory backend — the production hot path is postgres, where the
// lookup is an indexed UNIQUE column.
func (r *BackofficeRefreshTokenRegistry) GetByHash(ctx context.Context, tokenHash string) (*models.BackofficeRefreshToken, error) {
	if tokenHash == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TokenHash"))
	}

	tokens, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range tokens {
		if t.TokenHash == tokenHash {
			return t, nil
		}
	}

	return nil, errxtrace.Classify(registry.ErrBackofficeRefreshTokenNotFound)
}

// Revoke marks a single row as revoked by id, gated on backofficeUserID
// so a stolen id can't be used to revoke someone else's session.
// Already-revoked rows are treated as idempotent successes.
func (r *BackofficeRefreshTokenRegistry) Revoke(_ context.Context, backofficeUserID, id string) error {
	if backofficeUserID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	now := time.Now()
	r.lock.Lock()
	defer r.lock.Unlock()
	t, ok := r.items.Get(id)
	if !ok || t.BackofficeUserID != backofficeUserID {
		return errxtrace.Classify(registry.ErrBackofficeRefreshTokenNotFound, errx.Attrs("entity_id", id))
	}
	if t.RevokedAt == nil {
		t.RevokedAt = &now
		r.items.Set(t.ID, t)
	}
	return nil
}

// ListActiveByBackofficeUserID returns all non-revoked, non-expired
// rows for the given back-office user ordered LastUsedAt desc with
// CreatedAt desc as the tiebreaker for never-used tokens.
func (r *BackofficeRefreshTokenRegistry) ListActiveByBackofficeUserID(ctx context.Context, backofficeUserID string) ([]*models.BackofficeRefreshToken, error) {
	if backofficeUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}

	tokens, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	out := make([]*models.BackofficeRefreshToken, 0, len(tokens))
	for _, t := range tokens {
		if t.BackofficeUserID != backofficeUserID {
			continue
		}
		if t.RevokedAt != nil {
			continue
		}
		if now.After(t.ExpiresAt) {
			continue
		}
		out = append(out, t)
	}

	sort.SliceStable(out, func(i, j int) bool {
		li, lj := out[i].LastUsedAt, out[j].LastUsedAt
		switch {
		case li != nil && lj != nil && !li.Equal(*lj):
			return li.After(*lj)
		case li != nil && lj == nil:
			return true
		case li == nil && lj != nil:
			return false
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// RevokeByBackofficeUserID marks every active row for the given user
// as revoked under a single write lock — same TOCTOU avoidance as the
// tenant-side equivalent.
func (r *BackofficeRefreshTokenRegistry) RevokeByBackofficeUserID(_ context.Context, backofficeUserID string) error {
	if backofficeUserID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}

	now := time.Now()
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		t := pair.Value
		if t.BackofficeUserID == backofficeUserID && t.RevokedAt == nil {
			t.RevokedAt = &now
			r.items.Set(t.ID, t)
		}
	}
	return nil
}

// DeleteExpired removes every row whose ExpiresAt is in the past.
func (r *BackofficeRefreshTokenRegistry) DeleteExpired(ctx context.Context) error {
	tokens, err := r.List(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	r.lock.Lock()
	for _, t := range tokens {
		if now.After(t.ExpiresAt) {
			r.items.Delete(t.ID)
		}
	}
	r.lock.Unlock()

	return nil
}
