package memory

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.BackofficeUserRegistry = (*BackofficeUserRegistry)(nil)

type baseBackofficeUserRegistry = Registry[models.BackofficeUser, *models.BackofficeUser]

// BackofficeUserRegistry is the in-memory implementation of the
// platform-operator identity store (issue #1785). Mirrors the postgres
// backend's invariants — lowercased email storage and platform-wide
// email uniqueness — so tests that use the memory backend exercise the
// same semantics as production.
type BackofficeUserRegistry struct {
	*baseBackofficeUserRegistry
}

// NewBackofficeUserRegistry builds an empty in-memory back-office user
// store. No RLS / tenant wiring is required because the table is
// cross-cutting infrastructure that lives OUTSIDE the tenant model.
func NewBackofficeUserRegistry() *BackofficeUserRegistry {
	return &BackofficeUserRegistry{
		baseBackofficeUserRegistry: NewRegistry[models.BackofficeUser, *models.BackofficeUser](),
	}
}

// Create validates required fields, lowercases the email, and rejects
// duplicates platform-wide. CreatedAt / UpdatedAt are stamped at insert
// time so callers don't need to populate them. Returns ErrFieldRequired,
// ErrInvalidBackofficeRole, or ErrBackofficeEmailAlreadyExists when
// applicable.
func (r *BackofficeUserRegistry) Create(ctx context.Context, user models.BackofficeUser) (*models.BackofficeUser, error) {
	if err := r.validateForCreate(user); err != nil {
		return nil, err
	}
	user.Email = normaliseBackofficeEmail(user.Email)

	// Email uniqueness check has to happen under the write lock or two
	// concurrent Creates with the same email could both pass the
	// existence check. Using the underlying Registry's exposed lock keeps
	// the contention scope tight.
	r.lock.Lock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.Email == user.Email {
			r.lock.Unlock()
			return nil, errxtrace.Classify(registry.ErrBackofficeEmailAlreadyExists, errx.Attrs("email", user.Email))
		}
	}
	r.lock.Unlock()

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	if user.LastLoginAt != nil && user.LastLoginAt.IsZero() {
		user.LastLoginAt = nil
	}

	return r.baseBackofficeUserRegistry.Create(ctx, user)
}

// Get returns the row by id. Translates the generic ErrNotFound to the
// back-office-specific sentinel so callers don't have to discriminate
// at the call site.
func (r *BackofficeUserRegistry) Get(ctx context.Context, id string) (*models.BackofficeUser, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	user, err := r.baseBackofficeUserRegistry.Get(ctx, id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", id))
		}
		return nil, err
	}
	return user, nil
}

// GetByEmail performs a case-insensitive lookup by lowercased email.
func (r *BackofficeUserRegistry) GetByEmail(ctx context.Context, email string) (*models.BackofficeUser, error) {
	if email == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}
	normalised := normaliseBackofficeEmail(email)

	users, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.Email == normalised {
			return u, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("email", normalised))
}

// Update revalidates required fields, lowercases the email, and rejects
// email collisions with OTHER rows. Preserves CreatedAt / PasswordHash
// / LastLoginAt from the persisted row so a partial-struct update from
// the future HTTP layer can't clobber them by accident.
//
// PasswordHash is intentionally NOT required on the input — Update
// restores it from the persisted row. Callers that want to change the
// hash must go through SetPasswordHash.
func (r *BackofficeUserRegistry) Update(ctx context.Context, user models.BackofficeUser) (*models.BackofficeUser, error) {
	if user.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.validateForUpdate(user); err != nil {
		return nil, err
	}
	user.Email = normaliseBackofficeEmail(user.Email)

	r.lock.Lock()
	existing, ok := r.items.Get(user.ID)
	if !ok {
		r.lock.Unlock()
		return nil, errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", user.ID))
	}
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.ID != user.ID && pair.Value.Email == user.Email {
			r.lock.Unlock()
			return nil, errxtrace.Classify(registry.ErrBackofficeEmailAlreadyExists, errx.Attrs("email", user.Email))
		}
	}
	// Preserve immutable / write-path-isolated fields.
	user.CreatedAt = existing.CreatedAt
	user.PasswordHash = existing.PasswordHash
	user.LastLoginAt = existing.LastLoginAt
	user.UpdatedAt = time.Now()
	r.lock.Unlock()

	return r.baseBackofficeUserRegistry.Update(ctx, user)
}

// Delete is delegated to the base registry. Idempotent semantics match
// the rest of the memory backend.
func (r *BackofficeUserRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	return r.baseBackofficeUserRegistry.Delete(ctx, id)
}

// SetPasswordHash overwrites only password_hash + bumps updated_at.
// Intentionally separate from Update so the bcrypt hash can never leak
// through a generic full-row update.
func (r *BackofficeUserRegistry) SetPasswordHash(_ context.Context, id, hash string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if hash == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "PasswordHash"))
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	existing, ok := r.items.Get(id)
	if !ok {
		return errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", id))
	}
	existing.PasswordHash = hash
	existing.UpdatedAt = time.Now()
	r.items.Set(id, existing)
	return nil
}

// UpdateLastLogin stamps last_login_at and bumps updated_at.
func (r *BackofficeUserRegistry) UpdateLastLogin(_ context.Context, id string, at time.Time) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	existing, ok := r.items.Get(id)
	if !ok {
		return errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", id))
	}
	stamped := at
	existing.LastLoginAt = &stamped
	existing.UpdatedAt = time.Now()
	r.items.Set(id, existing)
	return nil
}

// SetActive flips is_active and bumps updated_at.
func (r *BackofficeUserRegistry) SetActive(_ context.Context, id string, active bool) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	existing, ok := r.items.Get(id)
	if !ok {
		return errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", id))
	}
	existing.IsActive = active
	existing.UpdatedAt = time.Now()
	r.items.Set(id, existing)
	return nil
}

// validateForCreate enforces the full required-field invariants for an
// insert: email, name, password_hash, and a valid role. Role validation
// returns the registry-typed sentinel instead of a validation.Error so
// callers can branch on a single identity.
func (r *BackofficeUserRegistry) validateForCreate(user models.BackofficeUser) error {
	if err := validateCommonBackofficeFields(user); err != nil {
		return err
	}
	if user.PasswordHash == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "PasswordHash"))
	}
	return nil
}

// validateForUpdate is the relaxed variant: it skips the PasswordHash
// check because Update restores the persisted hash from the stored row
// (callers must use SetPasswordHash to change the hash). Everything
// else remains required.
func (r *BackofficeUserRegistry) validateForUpdate(user models.BackofficeUser) error {
	return validateCommonBackofficeFields(user)
}

// validateCommonBackofficeFields covers the invariants shared by Create
// and Update — every column except the password hash, which only Create
// requires.
func validateCommonBackofficeFields(user models.BackofficeUser) error {
	if strings.TrimSpace(user.Email) == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}
	if strings.TrimSpace(user.Name) == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}
	if !user.Role.IsValid() {
		return errxtrace.Classify(registry.ErrInvalidBackofficeRole, errx.Attrs("role", string(user.Role)))
	}
	return nil
}

// normaliseBackofficeEmail lowercases + trims the email so case variants
// collapse to a single row. Mirrors postgres's behaviour where the
// registry layer does the same normalisation before INSERT / SELECT.
func normaliseBackofficeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
