package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.UserMFASecretRegistry = (*UserMFASecretRegistry)(nil)

type UserMFASecretRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewUserMFASecretRegistry(dbx *sqlx.DB) *UserMFASecretRegistry {
	return NewUserMFASecretRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewUserMFASecretRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *UserMFASecretRegistry {
	return &UserMFASecretRegistry{dbx: dbx, tableNames: tableNames}
}

// newSQLRegistry uses service mode (RLS bypass) because the registry is
// consumed during the password-step of login, before any tenant/user
// RLS context has been established on the connection.
func (r *UserMFASecretRegistry) newSQLRegistry() *store.RLSRepository[models.UserMFASecret, *models.UserMFASecret] {
	return store.NewServiceSQLRegistry[models.UserMFASecret, *models.UserMFASecret](r.dbx, r.tableNames.UserMFASecrets())
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
	mfa.CreatedAt = now
	mfa.UpdatedAt = now
	// Always mint server-side IDs — mirrors refresh_tokens / users /
	// every other auth-sensitive registry. Trusting a caller-provided
	// id would let a stolen handler context plant arbitrary primary
	// keys; the unique (tenant_id, user_id) index already prevents
	// double enrollment.
	mfa.ID = uuid.New().String()
	mfa.UUID = uuid.New().String()

	reg := r.newSQLRegistry()
	if err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		txReg := store.NewTxRegistry[models.UserMFASecret](tx, r.tableNames.UserMFASecrets())
		return txReg.Insert(ctx, mfa)
	}); err != nil {
		return nil, errxtrace.Wrap("failed to insert user_mfa_secret", err)
	}

	return &mfa, nil
}

func (r *UserMFASecretRegistry) Get(ctx context.Context, id string) (*models.UserMFASecret, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	var row models.UserMFASecret
	reg := r.newSQLRegistry()
	err := reg.ScanOneByField(ctx, store.Pair("id", id), &row)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "UserMFASecret", "entity_id", id))
		}
		return nil, errxtrace.Wrap("failed to get user_mfa_secret", err)
	}
	return &row, nil
}

func (r *UserMFASecretRegistry) List(ctx context.Context) ([]*models.UserMFASecret, error) {
	var rows []*models.UserMFASecret
	reg := r.newSQLRegistry()
	for row, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list user_mfa_secrets", err)
		}
		rows = append(rows, &row)
	}
	return rows, nil
}

func (r *UserMFASecretRegistry) Update(ctx context.Context, mfa models.UserMFASecret) (*models.UserMFASecret, error) {
	if mfa.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	mfa.UpdatedAt = time.Now()
	reg := r.newSQLRegistry()
	if err := reg.Update(ctx, mfa, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update user_mfa_secret", err)
	}
	return &mfa, nil
}

func (r *UserMFASecretRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	reg := r.newSQLRegistry()
	if err := reg.Delete(ctx, id, nil); err != nil {
		return errxtrace.Wrap("failed to delete user_mfa_secret", err)
	}
	return nil
}

func (r *UserMFASecretRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()
	n, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count user_mfa_secrets", err)
	}
	return n, nil
}

// GetByUser returns the (at most one) row for the (tenant, user) pair,
// or registry.ErrNotFound when the user has never enrolled.
func (r *UserMFASecretRegistry) GetByUser(ctx context.Context, tenantID, userID string) (*models.UserMFASecret, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	var row models.UserMFASecret
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE tenant_id = $1 AND user_id = $2`, r.tableNames.UserMFASecrets())
		if err := tx.GetContext(ctx, &row, query, tenantID, userID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "UserMFASecret"))
			}
			return errxtrace.Wrap("failed to get user_mfa_secret by user", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ConsumeBackupCodeAtomic removes the matching bcrypt hash from the
// row's backup_codes_hashed JSONB array under a single transaction.
// SELECT … FOR UPDATE acquires a row-level lock so two concurrent
// /auth/login/mfa requests racing on the same backup code can't both
// observe the unconsumed list and both succeed (#1645 review).
//
// The bcrypt compare is cheap relative to the lock hold time but we
// keep it inside the transaction so the failed-match case still
// releases the lock promptly when the closure returns false.
func (r *UserMFASecretRegistry) ConsumeBackupCodeAtomic(
	ctx context.Context,
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

	consumed := false
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var row models.UserMFASecret
		selQuery := fmt.Sprintf(
			`SELECT * FROM %s WHERE tenant_id = $1 AND user_id = $2 FOR UPDATE`,
			r.tableNames.UserMFASecrets(),
		)
		if err := tx.GetContext(ctx, &row, selQuery, tenantID, userID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "UserMFASecret"))
			}
			return errxtrace.Wrap("failed to lock user_mfa_secret", err)
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
			// Releasing the lock without an UPDATE is intentional —
			// nothing changed; the caller maps this to a 401.
			return nil
		}

		updQuery := fmt.Sprintf(
			`UPDATE %s SET backup_codes_hashed = $1, last_used_at = $2, updated_at = $3 WHERE id = $4`,
			r.tableNames.UserMFASecrets(),
		)
		serialized := models.ValuerSlice[string](remaining)
		if _, err := tx.ExecContext(ctx, updQuery, serialized, now, now, row.ID); err != nil {
			return errxtrace.Wrap("failed to persist backup-code consumption", err)
		}
		consumed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return consumed, nil
}

// DeleteByUser removes the user's MFA row idempotently — a missing row
// is not an error, since the disable flow is a no-op for non-enrolled users.
func (r *UserMFASecretRegistry) DeleteByUser(ctx context.Context, tenantID, userID string) error {
	if tenantID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE tenant_id = $1 AND user_id = $2`, r.tableNames.UserMFASecrets())
		if _, err := tx.ExecContext(ctx, query, tenantID, userID); err != nil {
			return errxtrace.Wrap("failed to delete user_mfa_secret by user", err)
		}
		return nil
	})
}
