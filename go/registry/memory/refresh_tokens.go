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

var _ registry.RefreshTokenRegistry = (*RefreshTokenRegistry)(nil)

type baseRefreshTokenRegistry = Registry[models.RefreshToken, *models.RefreshToken]

type RefreshTokenRegistry struct {
	*baseRefreshTokenRegistry
}

func NewRefreshTokenRegistry() *RefreshTokenRegistry {
	return &RefreshTokenRegistry{
		baseRefreshTokenRegistry: NewRegistry[models.RefreshToken, *models.RefreshToken](),
	}
}

func (r *RefreshTokenRegistry) Create(ctx context.Context, token models.RefreshToken) (*models.RefreshToken, error) {
	if token.TokenHash == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TokenHash"))
	}
	if token.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if token.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	token.ID = uuid.New().String()
	token.CreatedAt = time.Now()

	r.lock.Lock()
	r.items.Set(token.ID, &token)
	r.lock.Unlock()

	return &token, nil
}

// GetByTokenHash returns a refresh token by its SHA-256 hash.
func (r *RefreshTokenRegistry) GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	tokens, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range tokens {
		if t.TokenHash == tokenHash {
			return t, nil
		}
	}

	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "RefreshToken"))
}

// GetByUserID returns all refresh tokens for a user.
func (r *RefreshTokenRegistry) GetByUserID(ctx context.Context, userID string) ([]*models.RefreshToken, error) {
	tokens, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []*models.RefreshToken
	for _, t := range tokens {
		if t.UserID == userID {
			result = append(result, t)
		}
	}

	return result, nil
}

// ListActiveByUserID returns all non-revoked, non-expired refresh tokens
// for a user, ordered most-recently-used first. Tokens that have never
// been used fall back to CreatedAt for the ordering tiebreaker.
func (r *RefreshTokenRegistry) ListActiveByUserID(ctx context.Context, userID string) ([]*models.RefreshToken, error) {
	tokens, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	out := make([]*models.RefreshToken, 0, len(tokens))
	for _, t := range tokens {
		if t.RevokedAt != nil {
			continue
		}
		if now.After(t.ExpiresAt) {
			continue
		}
		out = append(out, t)
	}

	sort.SliceStable(out, func(i, j int) bool {
		// Active sessions sort: LastUsedAt desc, then CreatedAt desc.
		// Nil LastUsedAt sorts after any populated value (never-used
		// tokens land below recently-used ones).
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

// RevokeByUserID marks all refresh tokens for a user as revoked.
// A single write lock is held for the entire operation to avoid a TOCTOU
// race between listing tokens and updating them individually.
func (r *RefreshTokenRegistry) RevokeByUserID(_ context.Context, userID string) error {
	now := time.Now()
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		t := pair.Value
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &now
			r.items.Set(t.ID, t)
		}
	}
	return nil
}

// RevokeByID atomically revokes a refresh token by id only when it
// belongs to the supplied user. Returns ErrNotFound when no matching
// row exists so a user can't revoke someone else's session via a
// guessed id.
func (r *RefreshTokenRegistry) RevokeByID(_ context.Context, userID, id string) error {
	now := time.Now()
	r.lock.Lock()
	defer r.lock.Unlock()
	t, ok := r.items.Get(id)
	if !ok || t.UserID != userID {
		return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "RefreshToken", "entity_id", id))
	}
	if t.RevokedAt == nil {
		t.RevokedAt = &now
		r.items.Set(t.ID, t)
	}
	return nil
}

// RevokeAllExceptID revokes every refresh token for the user except
// the row whose id matches keepID. Pass an empty keepID to revoke
// every token.
func (r *RefreshTokenRegistry) RevokeAllExceptID(_ context.Context, userID, keepID string) error {
	now := time.Now()
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		t := pair.Value
		if t.UserID != userID || t.RevokedAt != nil {
			continue
		}
		if keepID != "" && t.ID == keepID {
			continue
		}
		t.RevokedAt = &now
		r.items.Set(t.ID, t)
	}
	return nil
}

// DeleteExpired removes all expired refresh tokens.
func (r *RefreshTokenRegistry) DeleteExpired(ctx context.Context) error {
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
