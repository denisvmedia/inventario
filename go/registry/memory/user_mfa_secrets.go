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

var _ registry.UserMFASecretRegistry = (*UserMFASecretRegistry)(nil)

type baseUserMFASecretRegistry = Registry[models.UserMFASecret, *models.UserMFASecret]

type UserMFASecretRegistry struct {
	*baseUserMFASecretRegistry
}

func NewUserMFASecretRegistry() *UserMFASecretRegistry {
	return &UserMFASecretRegistry{
		baseUserMFASecretRegistry: NewRegistry[models.UserMFASecret, *models.UserMFASecret](),
	}
}

func (r *UserMFASecretRegistry) Create(ctx context.Context, mfa models.UserMFASecret) (*models.UserMFASecret, error) {
	if mfa.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if mfa.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if mfa.SecretEncrypted == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "SecretEncrypted"))
	}

	now := time.Now()
	mfa.ID = uuid.New().String()
	if mfa.UUID == "" {
		mfa.UUID = uuid.New().String()
	}
	mfa.CreatedAt = now
	mfa.UpdatedAt = now

	r.lock.Lock()
	defer r.lock.Unlock()
	// Enforce (tenant_id, user_id) uniqueness mirroring the postgres
	// unique index. The login flow always upserts via GetByUser first,
	// so a duplicate Create signals a bug rather than a race.
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		existing := pair.Value
		if existing.TenantID == mfa.TenantID && existing.UserID == mfa.UserID {
			return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("entity_type", "UserMFASecret"))
		}
	}
	r.items.Set(mfa.ID, &mfa)
	return &mfa, nil
}

func (r *UserMFASecretRegistry) GetByUser(_ context.Context, tenantID, userID string) (*models.UserMFASecret, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	r.lock.RLock()
	defer r.lock.RUnlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		mfa := pair.Value
		if mfa.TenantID == tenantID && mfa.UserID == userID {
			return mfa, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "UserMFASecret"))
}

// ConsumeBackupCodeAtomic mirrors the postgres implementation:
// the entire find-and-consume sequence runs under a single write
// lock so two concurrent /auth/login/mfa requests can't both win
// against the same code (#1645 review).
func (r *UserMFASecretRegistry) ConsumeBackupCodeAtomic(
	_ context.Context,
	tenantID, userID string,
	now time.Time,
	matchHash func(hash string) bool,
) (bool, error) {
	if tenantID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if matchHash == nil {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "matchHash"))
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		mfa := pair.Value
		if mfa.TenantID != tenantID || mfa.UserID != userID {
			continue
		}
		remaining := make([]string, 0, len(mfa.BackupCodesHashed))
		matched := false
		for _, hash := range mfa.BackupCodesHashed {
			if !matched && matchHash(hash) {
				matched = true
				continue
			}
			remaining = append(remaining, hash)
		}
		if !matched {
			return false, nil
		}
		mfa.BackupCodesHashed = remaining
		mfa.LastUsedAt = &now
		mfa.UpdatedAt = now
		// pair.Value is already a *UserMFASecret pointer in the map;
		// the mutation above is observable to subsequent reads.
		return true, nil
	}
	return false, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "UserMFASecret"))
}

// MarkTOTPStepUsedAtomic replicates the postgres CAS under the registry
// write lock: it bumps last_used_step to `step` only when the stored
// value is strictly less, returning whether this call won. The lock held
// for the read-compare-write makes two concurrent callers presenting the
// same code (same step) serialise — only the first wins, the second sees
// the already-advanced step and loses (#2124).
func (r *UserMFASecretRegistry) MarkTOTPStepUsedAtomic(_ context.Context, tenantID, userID string, step int64, now time.Time) (bool, error) {
	if tenantID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		mfa := pair.Value
		if mfa.TenantID != tenantID || mfa.UserID != userID {
			continue
		}
		if mfa.LastUsedStep >= step {
			// Replay (or stale) — the step was already consumed.
			return false, nil
		}
		mfa.LastUsedStep = step
		mfa.LastUsedAt = &now
		mfa.UpdatedAt = now
		return true, nil
	}
	return false, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "UserMFASecret"))
}

func (r *UserMFASecretRegistry) DeleteByUser(_ context.Context, tenantID, userID string) error {
	if tenantID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		mfa := pair.Value
		if mfa.TenantID == tenantID && mfa.UserID == userID {
			r.items.Delete(pair.Key)
			return nil
		}
	}
	return nil
}

func (r *UserMFASecretRegistry) Update(ctx context.Context, mfa models.UserMFASecret) (*models.UserMFASecret, error) {
	if mfa.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	if _, ok := r.items.Get(mfa.ID); !ok {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "UserMFASecret", "entity_id", mfa.ID))
	}
	mfa.UpdatedAt = time.Now()
	r.items.Set(mfa.ID, &mfa)
	return &mfa, nil
}
