package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

var _ registry.APITokenRegistry = (*APITokenRegistry)(nil)

// APITokenRegistry is the in-memory registry.APITokenRegistry. Rows are stored
// by value and copied on the way out, so a caller holding a returned pointer
// cannot change what the next lookup sees.
type APITokenRegistry struct {
	mu     sync.RWMutex
	tokens map[string]models.APIToken
}

// NewAPITokenRegistry returns an empty registry.
func NewAPITokenRegistry() *APITokenRegistry {
	return &APITokenRegistry{tokens: make(map[string]models.APIToken)}
}

func (r *APITokenRegistry) Create(_ context.Context, token models.APIToken) (*models.APIToken, error) {
	switch {
	case token.TokenHash == "":
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TokenHash"))
	case token.UserID == "":
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	case token.TenantID == "":
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	case token.Name == "":
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	case !token.Scope.IsValid():
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Scope"))
	}

	token.ID = uuid.New().String()
	token.UUID = uuid.New().String()
	token.CreatedAt = time.Now()
	token.LastUsedAt = nil
	token.RevokedAt = nil

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.tokens {
		if existing.TokenHash == token.TokenHash {
			return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("entity_type", "APIToken"))
		}
	}
	r.tokens[token.ID] = token
	return new(token), nil
}

func (r *APITokenRegistry) GetByTokenHash(_ context.Context, tokenHash string) (*models.APIToken, error) {
	if tokenHash == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TokenHash"))
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.tokens {
		if t.TokenHash == tokenHash {
			return new(t), nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "APIToken"))
}

func (r *APITokenRegistry) ListByUserID(_ context.Context, userID string) ([]*models.APIToken, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	r.mu.RLock()
	out := make([]*models.APIToken, 0)
	for _, t := range r.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			out = append(out, new(t))
		}
	}
	r.mu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (r *APITokenRegistry) CountActiveByUserID(_ context.Context, userID string, now time.Time) (int, error) {
	if userID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, t := range r.tokens {
		if t.UserID == userID && t.IsUsable(now) {
			count++
		}
	}
	return count, nil
}

func (r *APITokenRegistry) RevokeByID(_ context.Context, userID, id string) (*models.APIToken, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tokens[id]
	if !ok || t.UserID != userID {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "APIToken", "entity_id", id))
	}
	if t.RevokedAt == nil {
		t.RevokedAt = new(time.Now())
		r.tokens[id] = t
	}
	return new(t), nil
}

func (r *APITokenRegistry) RevokeAllByUserID(_ context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	revoked := 0
	for id, t := range r.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = new(now)
			r.tokens[id] = t
			revoked++
		}
	}
	return revoked, nil
}

func (r *APITokenRegistry) TouchLastUsed(_ context.Context, id string, at, staleBefore time.Time) (bool, error) {
	if id == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tokens[id]
	if !ok || t.RevokedAt != nil {
		return false, nil
	}
	if t.LastUsedAt != nil && !t.LastUsedAt.Before(staleBefore) {
		return false, nil
	}
	t.LastUsedAt = new(at)
	r.tokens[id] = t
	return true, nil
}

// DeleteByUserID hard-deletes every token the user owns. Memory-only: the
// postgres purgers delete by table name in SQL.
func (r *APITokenRegistry) DeleteByUserID(_ context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, t := range r.tokens {
		if t.UserID == userID {
			delete(r.tokens, id)
		}
	}
	return nil
}

// DeleteByTenantID hard-deletes every token in the tenant. Memory-only, like
// DeleteByUserID.
func (r *APITokenRegistry) DeleteByTenantID(_ context.Context, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, t := range r.tokens {
		if t.TenantID == tenantID {
			delete(r.tokens, id)
		}
	}
	return nil
}
