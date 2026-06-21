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

var _ registry.BackofficeUserMFASecretRegistry = (*BackofficeUserMFASecretRegistry)(nil)

type baseBackofficeUserMFASecretRegistry = Registry[models.BackofficeUserMFASecret, *models.BackofficeUserMFASecret]

// BackofficeUserMFASecretRegistry is the in-memory implementation of the
// per-back-office-user TOTP secret store (issue #1785, Phase 4). Mirrors
// the postgres backend's "one row per back-office user" invariant via
// the in-memory upsert path so tests on the memory backend exercise the
// same semantics as production.
type BackofficeUserMFASecretRegistry struct {
	*baseBackofficeUserMFASecretRegistry
}

// NewBackofficeUserMFASecretRegistry constructs an empty in-memory MFA
// secret store. No RLS / tenant wiring is required — the back-office
// plane lives OUTSIDE the tenant model.
func NewBackofficeUserMFASecretRegistry() *BackofficeUserMFASecretRegistry {
	return &BackofficeUserMFASecretRegistry{
		baseBackofficeUserMFASecretRegistry: NewRegistry[models.BackofficeUserMFASecret, *models.BackofficeUserMFASecret](),
	}
}

// Get returns the row for the given back-office user id, or
// ErrBackofficeMFASecretNotFound when no enrollment exists.
func (r *BackofficeUserMFASecretRegistry) Get(_ context.Context, backofficeUserID string) (*models.BackofficeUserMFASecret, error) {
	if backofficeUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}
	r.lock.RLock()
	defer r.lock.RUnlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		row := pair.Value
		if row.BackofficeUserID == backofficeUserID {
			return row, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrBackofficeMFASecretNotFound, errx.Attrs("backoffice_user_id", backofficeUserID))
}

// Upsert atomically replaces (or inserts) the single row for the given
// back-office user. The whole replace runs under the registry's write
// lock so a partial write (secret persisted but backup codes failed)
// can't be observed by a concurrent reader.
func (r *BackofficeUserMFASecretRegistry) Upsert(ctx context.Context, secret models.BackofficeUserMFASecret) (*models.BackofficeUserMFASecret, error) {
	if secret.BackofficeUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}
	if secret.SecretEncrypted == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "SecretEncrypted"))
	}
	if err := secret.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("backoffice MFA secret failed model validation", err)
	}

	now := time.Now().UTC()
	r.lock.Lock()
	defer r.lock.Unlock()

	// Find existing row by backoffice_user_id; replace in-place so the
	// id stays stable across regenerate calls.
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		existing := pair.Value
		if existing.BackofficeUserID == secret.BackofficeUserID {
			// Preserve immutable id/uuid + CreatedAt across the upsert.
			secret.ID = existing.ID
			secret.UUID = existing.UUID
			secret.CreatedAt = existing.CreatedAt
			secret.UpdatedAt = now
			stored := secret
			r.items.Set(existing.ID, &stored)
			return &stored, nil
		}
	}

	// No existing row — insert a fresh one.
	secret.ID = uuid.New().String()
	if secret.UUID == "" {
		secret.UUID = uuid.New().String()
	}
	if secret.CreatedAt.IsZero() {
		secret.CreatedAt = now
	}
	secret.UpdatedAt = now
	stored := secret
	r.items.Set(stored.ID, &stored)
	return &stored, nil
}

// Delete removes the back-office user's MFA row idempotently. A missing
// row is not an error — matches the disable-is-noop contract.
func (r *BackofficeUserMFASecretRegistry) Delete(_ context.Context, backofficeUserID string) error {
	if backofficeUserID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		row := pair.Value
		if row.BackofficeUserID == backofficeUserID {
			r.items.Delete(pair.Key)
			return nil
		}
	}
	return nil
}

// MarkEnabled stamps EnabledAt on the target row and bumps UpdatedAt.
func (r *BackofficeUserMFASecretRegistry) MarkEnabled(_ context.Context, backofficeUserID string, at time.Time) error {
	if backofficeUserID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		row := pair.Value
		if row.BackofficeUserID == backofficeUserID {
			stamped := at
			row.EnabledAt = &stamped
			row.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return errxtrace.Classify(registry.ErrBackofficeMFASecretNotFound, errx.Attrs("backoffice_user_id", backofficeUserID))
}

// ConsumeBackupCodeAtomic mirrors the postgres implementation: the
// entire find-and-consume sequence runs under a single write lock so two
// concurrent /backoffice/auth/login/mfa requests can't both win against
// the same code.
func (r *BackofficeUserMFASecretRegistry) ConsumeBackupCodeAtomic(
	_ context.Context,
	backofficeUserID string,
	now time.Time,
	matchHash func(hash string) bool,
) (bool, error) {
	if backofficeUserID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}
	if matchHash == nil {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "matchHash"))
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		row := pair.Value
		if row.BackofficeUserID != backofficeUserID {
			continue
		}
		remaining := make([]string, 0, len(row.BackupCodesHashed))
		matched := false
		for _, hash := range row.BackupCodesHashed {
			if !matched && matchHash(hash) {
				matched = true
				continue
			}
			remaining = append(remaining, hash)
		}
		if !matched {
			return false, nil
		}
		row.BackupCodesHashed = remaining
		row.LastUsedAt = &now
		row.UpdatedAt = now
		return true, nil
	}
	return false, errxtrace.Classify(registry.ErrBackofficeMFASecretNotFound, errx.Attrs("backoffice_user_id", backofficeUserID))
}

// MarkTOTPStepUsedAtomic replicates the postgres CAS under the registry
// write lock: it bumps last_used_step to `step` only when the stored
// value is strictly less, returning whether this call won. The lock held
// for the read-compare-write serialises two concurrent callers presenting
// the same code (same step) — only the first wins (#2124).
func (r *BackofficeUserMFASecretRegistry) MarkTOTPStepUsedAtomic(_ context.Context, backofficeUserID string, step int64, now time.Time) (bool, error) {
	if backofficeUserID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		row := pair.Value
		if row.BackofficeUserID != backofficeUserID {
			continue
		}
		if row.LastUsedStep >= step {
			// Replay (or stale) — the step was already consumed.
			return false, nil
		}
		row.LastUsedStep = step
		stamped := now
		row.LastUsedAt = &stamped
		row.UpdatedAt = now
		return true, nil
	}
	// No row → the step could not be claimed. Mirror the postgres CAS, which
	// reports zero affected rows as (false, nil): a lost CAS (rejected like a
	// wrong code), not an infrastructure error — keeps memory and production
	// on one control flow (#2124).
	return false, nil
}
