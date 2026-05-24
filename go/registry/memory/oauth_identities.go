package memory

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.OAuthIdentityRegistry = (*OAuthIdentityRegistry)(nil)

type baseOAuthIdentityRegistry = Registry[models.OAuthIdentity, *models.OAuthIdentity]

// OAuthIdentityRegistry is an in-memory implementation of
// registry.OAuthIdentityRegistry mirroring the postgres semantics: global
// uniqueness on (provider, provider_user_id) is enforced on Create, and
// (tenantID, userID) is a defense-in-depth filter on the per-user reads.
type OAuthIdentityRegistry struct {
	*baseOAuthIdentityRegistry
}

// NewOAuthIdentityRegistry creates a new in-memory OAuthIdentityRegistry.
func NewOAuthIdentityRegistry() *OAuthIdentityRegistry {
	return &OAuthIdentityRegistry{
		baseOAuthIdentityRegistry: NewRegistry[models.OAuthIdentity, *models.OAuthIdentity](),
	}
}

// Create stores a new OAuth identity record, generating ID/UUID/LinkedAt.
// The (provider, provider_user_id) pair must be globally unique; a duplicate
// returns ErrAlreadyExists so the callback can distinguish "first-time link"
// from "already attached to some account" without an extra round-trip.
//
// The duplicate check + insert run under the same write lock so two
// concurrent Create calls cannot both pass the check and insert a clashing
// pair. The previous shape ran GetByProviderSubject outside the lock,
// which let a race insert two rows with the same global key.
func (r *OAuthIdentityRegistry) Create(_ context.Context, oi models.OAuthIdentity) (*models.OAuthIdentity, error) {
	if oi.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if oi.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if !oi.Provider.IsValid() {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Provider"))
	}
	if oi.ProviderUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ProviderUserID"))
	}

	oi.ID = uuid.New().String()
	if oi.UUID == "" {
		oi.UUID = uuid.New().String()
	}
	if oi.LinkedAt.IsZero() {
		oi.LinkedAt = time.Now()
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	// Duplicate check inside the lock so a concurrent Create can't slip a
	// clashing row between the check and the insert. We CANNOT call the
	// public GetByProviderSubject here — it acquires the same lock and
	// would deadlock.
	if existing := r.getByProviderSubjectNoLock(oi.Provider, oi.ProviderUserID); existing != nil {
		return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs(
			"entity_type", "OAuthIdentity",
			"provider", string(oi.Provider),
			"provider_user_id", oi.ProviderUserID,
		))
	}

	r.items.Set(oi.ID, &oi)
	return &oi, nil
}

// getByProviderSubjectNoLock is the lock-free variant used by Create's
// in-lock duplicate check. Callers MUST already hold r.lock.
func (r *OAuthIdentityRegistry) getByProviderSubjectNoLock(provider models.OAuthProvider, providerUserID string) *models.OAuthIdentity {
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		oi := pair.Value
		if oi.Provider == provider && oi.ProviderUserID == providerUserID {
			return oi
		}
	}
	return nil
}

// GetByProviderSubject looks up the row keyed by (provider, providerUserID).
func (r *OAuthIdentityRegistry) GetByProviderSubject(ctx context.Context, provider models.OAuthProvider, providerUserID string) (*models.OAuthIdentity, error) {
	if !provider.IsValid() {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Provider"))
	}
	if providerUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ProviderUserID"))
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, oi := range all {
		if oi.Provider == provider && oi.ProviderUserID == providerUserID {
			return oi, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "OAuthIdentity"))
}

// ListByUser returns every identity linked to userID within tenantID.
func (r *OAuthIdentityRegistry) ListByUser(ctx context.Context, tenantID, userID string) ([]*models.OAuthIdentity, error) {
	if tenantID == "" || userID == "" {
		return nil, nil
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	var result []*models.OAuthIdentity
	for _, oi := range all {
		if oi.TenantID == tenantID && oi.UserID == userID {
			result = append(result, oi)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Provider < result[j].Provider
	})
	return result, nil
}

// GetByUserAndProvider returns the single row keyed by (tenantID, userID, provider).
func (r *OAuthIdentityRegistry) GetByUserAndProvider(ctx context.Context, tenantID, userID string, provider models.OAuthProvider) (*models.OAuthIdentity, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if !provider.IsValid() {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Provider"))
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, oi := range all {
		if oi.TenantID == tenantID && oi.UserID == userID && oi.Provider == provider {
			return oi, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "OAuthIdentity"))
}

// DeleteByUserAndProvider removes (tenantID, userID, provider) idempotently.
func (r *OAuthIdentityRegistry) DeleteByUserAndProvider(ctx context.Context, tenantID, userID string, provider models.OAuthProvider) error {
	row, err := r.GetByUserAndProvider(ctx, tenantID, userID, provider)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			return nil
		}
		return err
	}
	r.lock.Lock()
	r.items.Delete(row.ID)
	r.lock.Unlock()
	return nil
}
