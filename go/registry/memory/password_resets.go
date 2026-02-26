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

var _ registry.PasswordResetRegistry = (*PasswordResetRegistry)(nil)

type basePasswordResetRegistry = Registry[models.PasswordReset, *models.PasswordReset]

// PasswordResetRegistry is an in-memory implementation of registry.PasswordResetRegistry.
type PasswordResetRegistry struct {
	*basePasswordResetRegistry
}

// NewPasswordResetRegistry creates a new in-memory PasswordResetRegistry.
func NewPasswordResetRegistry() *PasswordResetRegistry {
	return &PasswordResetRegistry{
		basePasswordResetRegistry: NewRegistry[models.PasswordReset, *models.PasswordReset](),
	}
}

// Create stores a new password-reset record, generating an ID and CreatedAt timestamp.
func (r *PasswordResetRegistry) Create(_ context.Context, pr models.PasswordReset) (*models.PasswordReset, error) {
	if pr.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if pr.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if pr.Token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}

	pr.ID = uuid.New().String()
	pr.CreatedAt = time.Now()

	r.lock.Lock()
	r.items.Set(pr.ID, &pr)
	r.lock.Unlock()

	return &pr, nil
}

// GetByToken returns the reset record matching the given token value.
func (r *PasswordResetRegistry) GetByToken(ctx context.Context, token string) (*models.PasswordReset, error) {
	if token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, pr := range all {
		if pr.Token == token {
			return pr, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "PasswordReset"))
}

// GetByUserID returns all password-reset records belonging to the given user.
func (r *PasswordResetRegistry) GetByUserID(ctx context.Context, userID string) ([]*models.PasswordReset, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	var result []*models.PasswordReset
	for _, pr := range all {
		if pr.UserID == userID {
			result = append(result, pr)
		}
	}
	return result, nil
}

// DeleteByUserID removes all password-reset records for the given user.
func (r *PasswordResetRegistry) DeleteByUserID(ctx context.Context, userID string) error {
	records, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}
	r.lock.Lock()
	for _, pr := range records {
		r.items.Delete(pr.ID)
	}
	r.lock.Unlock()
	return nil
}

// DeleteExpired removes all records whose ExpiresAt timestamp is in the past.
func (r *PasswordResetRegistry) DeleteExpired(ctx context.Context) error {
	all, err := r.List(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	r.lock.Lock()
	for _, pr := range all {
		if now.After(pr.ExpiresAt) {
			r.items.Delete(pr.ID)
		}
	}
	r.lock.Unlock()
	return nil
}
