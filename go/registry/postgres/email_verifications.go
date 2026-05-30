package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.EmailVerificationRegistry = (*EmailVerificationRegistry)(nil)

// EmailVerificationRegistry provides PostgreSQL-backed storage for email verification records.
// It uses a NonRLSRepository because verifications are resolved before a user session exists.
type EmailVerificationRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewEmailVerificationRegistry creates a new EmailVerificationRegistry backed by the given database.
func NewEmailVerificationRegistry(dbx *sqlx.DB) *EmailVerificationRegistry {
	return &EmailVerificationRegistry{dbx: dbx, tableNames: store.DefaultTableNames}
}

func (r *EmailVerificationRegistry) newRepo() *store.NonRLSRepository[models.EmailVerification, *models.EmailVerification] {
	return store.NewSQLRegistry[models.EmailVerification, *models.EmailVerification](r.dbx, r.tableNames.EmailVerifications())
}

// Create inserts a new email verification record.
func (r *EmailVerificationRegistry) Create(ctx context.Context, ev models.EmailVerification) (*models.EmailVerification, error) {
	if ev.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if ev.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if ev.Token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}
	ev.CreatedAt = time.Now()
	created, err := r.newRepo().Create(ctx, ev, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create email verification", err)
	}
	return &created, nil
}

// Get returns an email verification record by ID.
func (r *EmailVerificationRegistry) Get(ctx context.Context, id string) (*models.EmailVerification, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	var ev models.EmailVerification
	if err := r.newRepo().ScanOneByField(ctx, store.Pair("id", id), &ev); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "EmailVerification", "entity_id", id))
		}
		return nil, errxtrace.Wrap("failed to get email verification", err)
	}
	return &ev, nil
}

// List returns all email verification records.
func (r *EmailVerificationRegistry) List(ctx context.Context) ([]*models.EmailVerification, error) {
	var result []*models.EmailVerification
	for ev, err := range r.newRepo().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list email verifications", err)
		}
		result = append(result, &ev)
	}
	return result, nil
}

// Update modifies an existing email verification record.
func (r *EmailVerificationRegistry) Update(ctx context.Context, ev models.EmailVerification) (*models.EmailVerification, error) {
	if ev.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.newRepo().Update(ctx, ev, nil); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "EmailVerification", "entity_id", ev.GetID()))
		}
		return nil, errxtrace.Wrap("failed to update email verification", err)
	}
	return &ev, nil
}

// Delete removes an email verification record by ID.
func (r *EmailVerificationRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.newRepo().Delete(ctx, id, nil); err != nil {
		return errxtrace.Wrap("failed to delete email verification", err)
	}
	return nil
}

// Count returns the total number of email verification records.
func (r *EmailVerificationRegistry) Count(ctx context.Context) (int, error) {
	count, err := r.newRepo().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count email verifications", err)
	}
	return count, nil
}

// GetByToken returns the verification record matching the given token.
func (r *EmailVerificationRegistry) GetByToken(ctx context.Context, token string) (*models.EmailVerification, error) {
	if token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}
	var ev models.EmailVerification
	if err := r.newRepo().ScanOneByField(ctx, store.Pair("token", token), &ev); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "EmailVerification"))
		}
		return nil, errxtrace.Wrap("failed to get email verification by token", err)
	}
	return &ev, nil
}

// GetByUserID returns all verification records belonging to the given user.
func (r *EmailVerificationRegistry) GetByUserID(ctx context.Context, userID string) ([]*models.EmailVerification, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	var result []*models.EmailVerification
	for ev, err := range r.newRepo().ScanByField(ctx, store.Pair("user_id", userID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list email verifications by user", err)
		}
		result = append(result, &ev)
	}
	return result, nil
}

// MarkVerified atomically flips verified_at from NULL to the current time for
// the row matching token, returning whether this call won the claim. The
// `verified_at IS NULL` filter makes the write idempotent across concurrent
// requests: exactly one of them changes a row (rows-affected == 1 → true) and
// the rest — plus a non-existent or already-verified token — observe zero
// rows-affected → false. See the interface doc and #1005.
func (r *EmailVerificationRegistry) MarkVerified(ctx context.Context, token string) (bool, error) {
	if token == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}
	var claimed bool
	err := r.newRepo().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET verified_at = $1 WHERE token = $2 AND verified_at IS NULL`,
			r.tableNames.EmailVerifications(),
		)
		res, err := tx.ExecContext(ctx, query, time.Now(), token)
		if err != nil {
			return errxtrace.Wrap("failed to mark email verification as verified", err)
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return errxtrace.Wrap("failed to read rows affected", err)
		}
		claimed = rows > 0
		return nil
	})
	if err != nil {
		return false, err
	}
	return claimed, nil
}

// DeleteExpired removes all records whose ExpiresAt timestamp is in the past.
func (r *EmailVerificationRegistry) DeleteExpired(ctx context.Context) error {
	return r.newRepo().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE expires_at < $1`, r.tableNames.EmailVerifications())
		_, err := tx.ExecContext(ctx, query, time.Now())
		return err
	})
}
