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

var _ registry.MagicLinkTokenRegistry = (*MagicLinkTokenRegistry)(nil)

type baseMagicLinkTokenRegistry = Registry[models.MagicLinkToken, *models.MagicLinkToken]

// MagicLinkTokenRegistry is an in-memory implementation of registry.MagicLinkTokenRegistry.
type MagicLinkTokenRegistry struct {
	*baseMagicLinkTokenRegistry
}

// NewMagicLinkTokenRegistry creates a new in-memory MagicLinkTokenRegistry.
func NewMagicLinkTokenRegistry() *MagicLinkTokenRegistry {
	return &MagicLinkTokenRegistry{
		baseMagicLinkTokenRegistry: NewRegistry[models.MagicLinkToken, *models.MagicLinkToken](),
	}
}

// Create stores a new magic-link token record, generating an ID and CreatedAt timestamp.
func (r *MagicLinkTokenRegistry) Create(_ context.Context, mlt models.MagicLinkToken) (*models.MagicLinkToken, error) {
	if mlt.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if mlt.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if mlt.Token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}

	mlt.ID = uuid.New().String()
	if mlt.UUID == "" {
		mlt.UUID = uuid.New().String()
	}
	mlt.CreatedAt = time.Now()

	r.lock.Lock()
	r.items.Set(mlt.ID, &mlt)
	r.lock.Unlock()

	return &mlt, nil
}

// GetByToken returns the magic-link record matching the given token value.
func (r *MagicLinkTokenRegistry) GetByToken(ctx context.Context, token string) (*models.MagicLinkToken, error) {
	if token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, mlt := range all {
		if mlt.Token == token {
			return mlt, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "MagicLinkToken"))
}

// DeleteByUserID removes all magic-link token records for the given user.
func (r *MagicLinkTokenRegistry) DeleteByUserID(ctx context.Context, userID string) error {
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	all, err := r.List(ctx)
	if err != nil {
		return err
	}
	r.lock.Lock()
	for _, mlt := range all {
		if mlt.UserID == userID {
			r.items.Delete(mlt.ID)
		}
	}
	r.lock.Unlock()
	return nil
}

// MarkClaimed atomically claims the magic-link token: under the registry's
// write lock it sets ClaimedAt only if it is still nil AND the token has not
// expired, returning whether this call performed the flip. The lock is the
// in-process equivalent of the postgres `claimed_at IS NULL AND expires_at >
// now` filter — two goroutines verifying the same token serialize, so exactly
// one observes the nil-and-live row and returns true while the loser (and an
// already-claimed, unknown, or expired token) returns false.
func (r *MagicLinkTokenRegistry) MarkClaimed(_ context.Context, token string) (bool, error) {
	if token == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	now := time.Now()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		mlt := pair.Value
		if mlt.Token != token {
			continue
		}
		if mlt.ClaimedAt != nil {
			// Already claimed — another caller won, or it was claimed earlier.
			return false, nil
		}
		if now.After(mlt.ExpiresAt) {
			// Expired tokens can never be burned (mirrors the postgres filter).
			return false, nil
		}
		mlt.ClaimedAt = &now
		return true, nil
	}
	// Token not found.
	return false, nil
}

// DeleteExpired removes all records whose ExpiresAt timestamp is in the past.
func (r *MagicLinkTokenRegistry) DeleteExpired(ctx context.Context) error {
	all, err := r.List(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	r.lock.Lock()
	for _, mlt := range all {
		if now.After(mlt.ExpiresAt) {
			r.items.Delete(mlt.ID)
		}
	}
	r.lock.Unlock()
	return nil
}
