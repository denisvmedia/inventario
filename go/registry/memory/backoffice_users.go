package memory

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

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
//
// The email uniqueness check AND the insert happen under a single
// write-lock acquisition — the previous shape released the lock between
// the check and the underlying Registry.Create's own re-acquisition,
// leaving a race window where two concurrent calls with the same email
// could both observe "no collision". Holding the lock across both ops
// matches the postgres backend's "check + insert in one tx" contract.
func (r *BackofficeUserRegistry) Create(ctx context.Context, user models.BackofficeUser) (*models.BackofficeUser, error) {
	if err := r.validateForCreate(user); err != nil {
		return nil, err
	}
	// Defence-in-depth: re-run the model's full validation so format/
	// length constraints (EmailPattern, max lengths, closed-set role)
	// fail closed even if a future caller bypasses Service.Bootstrap.
	// The registry's bespoke validateForCreate runs first so the existing
	// ErrFieldRequired / ErrInvalidBackofficeRole sentinels keep their
	// identity for callers that branch on them.
	if err := user.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("backoffice user failed model validation", err)
	}
	user.Email = normaliseBackofficeEmail(user.Email)

	// Stamp in UTC to match the postgres backend (which uses
	// time.Now().UTC() / Postgres now() returning UTC-equivalent values)
	// and the rest of the memory registries — keeps cross-backend
	// timestamps comparable and independent of the host timezone.
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = now
	}
	if user.LastLoginAt != nil && user.LastLoginAt.IsZero() {
		user.LastLoginAt = nil
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.Email == user.Email {
			return nil, errxtrace.Classify(registry.ErrBackofficeEmailAlreadyExists, errx.Attrs("email", user.Email))
		}
	}

	// Mint server-side ID + UUID inline so the whole "check uniqueness,
	// assign id, insert" sequence runs under the single lock. Mirrors
	// the base memory Registry's Create body — duplicated here so we
	// don't have to release the lock to call into it.
	user.ID = uuid.New().String()
	if user.UUID == "" {
		user.UUID = uuid.New().String()
	}
	stored := user
	r.items.Set(stored.ID, &stored)

	return &stored, nil
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
// Whitespace-only input is treated as empty so a stray "   " from a
// caller doesn't fall through to a no-rows lookup.
func (r *BackofficeUserRegistry) GetByEmail(ctx context.Context, email string) (*models.BackofficeUser, error) {
	if strings.TrimSpace(email) == "" {
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
//
// The cross-row email uniqueness check AND the persisted write happen
// under a single write-lock acquisition — the previous shape released
// the lock between the check and a delegated call to the underlying
// Registry.Update, leaving a race window where two concurrent updates
// could both observe "no collision". Holding the lock across both ops
// matches the postgres backend's "check + UPDATE in one tx" contract.
func (r *BackofficeUserRegistry) Update(ctx context.Context, user models.BackofficeUser) (*models.BackofficeUser, error) {
	if user.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.validateForUpdate(user); err != nil {
		return nil, err
	}
	// Defence-in-depth: re-run the model's full validation so format/
	// length constraints (EmailPattern, max lengths, closed-set role)
	// fail closed even when callers bypass the service layer. The
	// registry's bespoke validateForUpdate runs first so the existing
	// ErrFieldRequired / ErrInvalidBackofficeRole sentinels keep their
	// identity. PasswordHash isn't part of the public Update surface,
	// so model validation runs on a copy with an opaque non-empty hash
	// substituted in — the on-disk hash is restored further down.
	validateUser := user
	if validateUser.PasswordHash == "" {
		validateUser.PasswordHash = "validation-placeholder"
	}
	if err := validateUser.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("backoffice user failed model validation", err)
	}
	user.Email = normaliseBackofficeEmail(user.Email)

	r.lock.Lock()
	defer r.lock.Unlock()

	existing, ok := r.items.Get(user.ID)
	if !ok {
		return nil, errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", user.ID))
	}
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.ID != user.ID && pair.Value.Email == user.Email {
			return nil, errxtrace.Classify(registry.ErrBackofficeEmailAlreadyExists, errx.Attrs("email", user.Email))
		}
	}
	// Preserve immutable / write-path-isolated fields, then perform the
	// store update inline so the whole "check uniqueness, mutate" sequence
	// runs under the single lock acquisition.
	user.CreatedAt = existing.CreatedAt
	user.PasswordHash = existing.PasswordHash
	user.LastLoginAt = existing.LastLoginAt
	user.UpdatedAt = time.Now().UTC()
	// UUID is immutable after creation — overwrite whatever the caller
	// supplied with the persisted value (mirrors the base Registry's
	// Update behaviour for UUIDable rows).
	user.UUID = existing.UUID
	stored := user
	r.items.Set(stored.ID, &stored)

	return &stored, nil
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
// through a generic full-row update. Whitespace-only input is rejected
// so a caller can't accidentally wipe the hash via a stray "   " — the
// previous shape accepted any non-zero string and a blanked-out hash
// would lock the back-office user out of every plane indefinitely
// (carries Phase 1 deferred review comment cid 3292613046).
func (r *BackofficeUserRegistry) SetPasswordHash(_ context.Context, id, hash string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if strings.TrimSpace(hash) == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "PasswordHash"))
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	existing, ok := r.items.Get(id)
	if !ok {
		return errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", id))
	}
	existing.PasswordHash = hash
	existing.UpdatedAt = time.Now().UTC()
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
	existing.UpdatedAt = time.Now().UTC()
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
	existing.UpdatedAt = time.Now().UTC()
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
