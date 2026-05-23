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

var _ registry.BackofficeUserMFASecretRegistry = (*BackofficeUserMFASecretRegistry)(nil)

// BackofficeUserMFASecretRegistry is the postgres-backed implementation
// of the per-back-office-user TOTP secret store (issue #1785, Phase 4).
// The underlying table has NO row-level security — same reasoning as
// backoffice_users / backoffice_refresh_tokens: back-office identities
// live OUTSIDE the tenant model, so RLS predicates that read
// get_current_*_id() would block the very calls that need to authenticate.
type BackofficeUserMFASecretRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewBackofficeUserMFASecretRegistry returns a postgres-backed registry
// using the default table-name set.
func NewBackofficeUserMFASecretRegistry(dbx *sqlx.DB) *BackofficeUserMFASecretRegistry {
	return NewBackofficeUserMFASecretRegistryWithTableNames(dbx, store.DefaultTableNames)
}

// NewBackofficeUserMFASecretRegistryWithTableNames lets tests override
// the table names (same pattern as every other postgres registry).
func NewBackofficeUserMFASecretRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *BackofficeUserMFASecretRegistry {
	return &BackofficeUserMFASecretRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *BackofficeUserMFASecretRegistry) newSQLRegistry() *store.NonRLSRepository[models.BackofficeUserMFASecret, *models.BackofficeUserMFASecret] {
	return store.NewSQLRegistry[models.BackofficeUserMFASecret, *models.BackofficeUserMFASecret](r.dbx, r.tableNames.BackofficeUserMFASecrets())
}

// Get returns the row for the given back-office user id, or
// ErrBackofficeMFASecretNotFound when no enrollment exists.
func (r *BackofficeUserMFASecretRegistry) Get(ctx context.Context, backofficeUserID string) (*models.BackofficeUserMFASecret, error) {
	if backofficeUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}

	var row models.BackofficeUserMFASecret
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE backoffice_user_id = $1`,
			r.tableNames.BackofficeUserMFASecrets(),
		)
		if err := tx.GetContext(ctx, &row, query, backofficeUserID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrBackofficeMFASecretNotFound, errx.Attrs("backoffice_user_id", backofficeUserID))
			}
			return errxtrace.Wrap("failed to get backoffice MFA secret", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Upsert atomically replaces (or inserts) the single row for the given
// back-office user. Implemented as DELETE + INSERT inside one tx because
// the natural unique key is backoffice_user_id (a regenerate-backup-codes
// call must keep the row id stable for downstream FK consumers, but
// today nothing FKs into backoffice_user_mfa_secrets so a churned id is
// safe). The whole replace runs under one transaction so a partial
// write is impossible.
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
	if secret.CreatedAt.IsZero() {
		secret.CreatedAt = now
	}
	secret.UpdatedAt = now

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Wipe any existing row for this back-office user first. The
		// unique index on backoffice_user_id makes a plain INSERT
		// fail on a regenerate call; DELETE + INSERT inside the tx
		// is the simplest atomic-replace pattern.
		delQuery := fmt.Sprintf(
			`DELETE FROM %s WHERE backoffice_user_id = $1`,
			r.tableNames.BackofficeUserMFASecrets(),
		)
		if _, err := tx.ExecContext(ctx, delQuery, secret.BackofficeUserID); err != nil {
			return errxtrace.Wrap("failed to clear existing backoffice MFA secret", err)
		}

		// Always mint server-side IDs for the fresh insert — same
		// reasoning as every other auth-sensitive registry.
		secret.ID = uuid.New().String()
		secret.UUID = uuid.New().String()

		txReg := store.NewTxRegistry[models.BackofficeUserMFASecret](tx, r.tableNames.BackofficeUserMFASecrets())
		if err := txReg.Insert(ctx, secret); err != nil {
			return errxtrace.Wrap("failed to insert backoffice MFA secret", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &secret, nil
}

// Delete removes the back-office user's MFA row idempotently — a
// missing row is not an error.
func (r *BackofficeUserMFASecretRegistry) Delete(ctx context.Context, backofficeUserID string) error {
	if backofficeUserID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}
	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`DELETE FROM %s WHERE backoffice_user_id = $1`,
			r.tableNames.BackofficeUserMFASecrets(),
		)
		if _, err := tx.ExecContext(ctx, query, backofficeUserID); err != nil {
			return errxtrace.Wrap("failed to delete backoffice MFA secret", err)
		}
		return nil
	})
}

// MarkEnabled stamps EnabledAt on the target row and bumps UpdatedAt.
func (r *BackofficeUserMFASecretRegistry) MarkEnabled(ctx context.Context, backofficeUserID string, at time.Time) error {
	if backofficeUserID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}
	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET enabled_at = $1, updated_at = now() WHERE backoffice_user_id = $2`,
			r.tableNames.BackofficeUserMFASecrets(),
		)
		res, err := tx.ExecContext(ctx, query, at, backofficeUserID)
		if err != nil {
			return errxtrace.Wrap("failed to mark backoffice MFA secret enabled", err)
		}
		affected, raErr := res.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read rows affected for backoffice MFA mark-enabled", raErr)
		}
		if affected == 0 {
			return errxtrace.Classify(registry.ErrBackofficeMFASecretNotFound, errx.Attrs("backoffice_user_id", backofficeUserID))
		}
		return nil
	})
}

// ConsumeBackupCodeAtomic mirrors UserMFASecretRegistry.ConsumeBackupCodeAtomic:
// SELECT … FOR UPDATE acquires a row-level lock so two concurrent
// /backoffice/auth/login/mfa requests racing on the same backup code
// can't both observe the unconsumed list and both succeed.
func (r *BackofficeUserMFASecretRegistry) ConsumeBackupCodeAtomic(
	ctx context.Context,
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

	consumed := false
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var row models.BackofficeUserMFASecret
		selQuery := fmt.Sprintf(
			`SELECT * FROM %s WHERE backoffice_user_id = $1 FOR UPDATE`,
			r.tableNames.BackofficeUserMFASecrets(),
		)
		if err := tx.GetContext(ctx, &row, selQuery, backofficeUserID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrBackofficeMFASecretNotFound, errx.Attrs("backoffice_user_id", backofficeUserID))
			}
			return errxtrace.Wrap("failed to lock backoffice MFA secret", err)
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
			return nil
		}

		updQuery := fmt.Sprintf(
			`UPDATE %s SET backup_codes_hashed = $1, last_used_at = $2, updated_at = $3 WHERE id = $4`,
			r.tableNames.BackofficeUserMFASecrets(),
		)
		serialised := models.ValuerSlice[string](remaining)
		if _, err := tx.ExecContext(ctx, updQuery, serialised, now, now, row.ID); err != nil {
			return errxtrace.Wrap("failed to persist backoffice backup-code consumption", err)
		}
		consumed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return consumed, nil
}

// BumpLastUsedAt sets LastUsedAt to `now` after a successful TOTP
// verification (the backup-code path bumps it inside
// ConsumeBackupCodeAtomic instead).
func (r *BackofficeUserMFASecretRegistry) BumpLastUsedAt(ctx context.Context, backofficeUserID string, now time.Time) error {
	if backofficeUserID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}
	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET last_used_at = $1, updated_at = now() WHERE backoffice_user_id = $2`,
			r.tableNames.BackofficeUserMFASecrets(),
		)
		res, err := tx.ExecContext(ctx, query, now, backofficeUserID)
		if err != nil {
			return errxtrace.Wrap("failed to bump backoffice MFA last_used_at", err)
		}
		affected, raErr := res.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read rows affected for backoffice MFA bump", raErr)
		}
		if affected == 0 {
			return errxtrace.Classify(registry.ErrBackofficeMFASecretNotFound, errx.Attrs("backoffice_user_id", backofficeUserID))
		}
		return nil
	})
}
