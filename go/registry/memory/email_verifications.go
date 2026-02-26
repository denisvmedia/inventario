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

var _ registry.EmailVerificationRegistry = (*EmailVerificationRegistry)(nil)

type baseEmailVerificationRegistry = Registry[models.EmailVerification, *models.EmailVerification]

// EmailVerificationRegistry is an in-memory implementation of registry.EmailVerificationRegistry.
type EmailVerificationRegistry struct {
	*baseEmailVerificationRegistry
}

// NewEmailVerificationRegistry creates a new in-memory EmailVerificationRegistry.
func NewEmailVerificationRegistry() *EmailVerificationRegistry {
	return &EmailVerificationRegistry{
		baseEmailVerificationRegistry: NewRegistry[models.EmailVerification, *models.EmailVerification](),
	}
}

// Create stores a new email verification record, generating an ID and CreatedAt timestamp.
func (r *EmailVerificationRegistry) Create(_ context.Context, ev models.EmailVerification) (*models.EmailVerification, error) {
	if ev.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if ev.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if ev.Token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}

	ev.ID = uuid.New().String()
	ev.CreatedAt = time.Now()

	r.lock.Lock()
	r.items.Set(ev.ID, &ev)
	r.lock.Unlock()

	return &ev, nil
}

// GetByToken returns the verification record matching the given token value.
func (r *EmailVerificationRegistry) GetByToken(ctx context.Context, token string) (*models.EmailVerification, error) {
	if token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, ev := range all {
		if ev.Token == token {
			return ev, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "EmailVerification"))
}

// GetByUserID returns all verification records belonging to the given user.
func (r *EmailVerificationRegistry) GetByUserID(ctx context.Context, userID string) ([]*models.EmailVerification, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	var result []*models.EmailVerification
	for _, ev := range all {
		if ev.UserID == userID {
			result = append(result, ev)
		}
	}
	return result, nil
}

// DeleteExpired removes all records whose ExpiresAt timestamp is in the past.
func (r *EmailVerificationRegistry) DeleteExpired(ctx context.Context) error {
	all, err := r.List(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	r.lock.Lock()
	for _, ev := range all {
		if now.After(ev.ExpiresAt) {
			r.items.Delete(ev.ID)
		}
	}
	r.lock.Unlock()
	return nil
}
