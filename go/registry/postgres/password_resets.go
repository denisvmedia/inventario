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

var _ registry.PasswordResetRegistry = (*PasswordResetRegistry)(nil)

// PasswordResetRegistry provides PostgreSQL-backed storage for password-reset records.
// It uses a NonRLSRepository because resets are resolved before a user session exists.
type PasswordResetRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewPasswordResetRegistry creates a new PasswordResetRegistry backed by the given database.
func NewPasswordResetRegistry(dbx *sqlx.DB) *PasswordResetRegistry {
	return &PasswordResetRegistry{dbx: dbx, tableNames: store.DefaultTableNames}
}

func (r *PasswordResetRegistry) newRepo() *store.NonRLSRepository[models.PasswordReset, *models.PasswordReset] {
	return store.NewSQLRegistry[models.PasswordReset, *models.PasswordReset](r.dbx, r.tableNames.PasswordResets())
}

// Create inserts a new password-reset record.
func (r *PasswordResetRegistry) Create(ctx context.Context, pr models.PasswordReset) (*models.PasswordReset, error) {
	if pr.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if pr.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if pr.Token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}
	pr.CreatedAt = time.Now()
	created, err := r.newRepo().Create(ctx, pr, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create password reset", err)
	}
	return &created, nil
}

// Get returns a password-reset record by ID.
func (r *PasswordResetRegistry) Get(ctx context.Context, id string) (*models.PasswordReset, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	var pr models.PasswordReset
	if err := r.newRepo().ScanOneByField(ctx, store.Pair("id", id), &pr); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "PasswordReset", "entity_id", id))
		}
		return nil, errxtrace.Wrap("failed to get password reset", err)
	}
	return &pr, nil
}

// List returns all password-reset records.
func (r *PasswordResetRegistry) List(ctx context.Context) ([]*models.PasswordReset, error) {
	var result []*models.PasswordReset
	for pr, err := range r.newRepo().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list password resets", err)
		}
		result = append(result, &pr)
	}
	return result, nil
}

// Update modifies an existing password-reset record.
func (r *PasswordResetRegistry) Update(ctx context.Context, pr models.PasswordReset) (*models.PasswordReset, error) {
	if pr.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.newRepo().Update(ctx, pr, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update password reset", err)
	}
	return &pr, nil
}

// Delete removes a password-reset record by ID.
func (r *PasswordResetRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.newRepo().Delete(ctx, id, nil); err != nil {
		return errxtrace.Wrap("failed to delete password reset", err)
	}
	return nil
}

// Count returns the total number of password-reset records.
func (r *PasswordResetRegistry) Count(ctx context.Context) (int, error) {
	count, err := r.newRepo().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count password resets", err)
	}
	return count, nil
}

// GetByToken returns the reset record matching the given token.
func (r *PasswordResetRegistry) GetByToken(ctx context.Context, token string) (*models.PasswordReset, error) {
	if token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}
	var pr models.PasswordReset
	if err := r.newRepo().ScanOneByField(ctx, store.Pair("token", token), &pr); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "PasswordReset"))
		}
		return nil, errxtrace.Wrap("failed to get password reset by token", err)
	}
	return &pr, nil
}

// GetByUserID returns all password-reset records belonging to the given user.
func (r *PasswordResetRegistry) GetByUserID(ctx context.Context, userID string) ([]*models.PasswordReset, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	var result []*models.PasswordReset
	for pr, err := range r.newRepo().ScanByField(ctx, store.Pair("user_id", userID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list password resets by user", err)
		}
		result = append(result, &pr)
	}
	return result, nil
}

// DeleteByUserID removes all password-reset records for the given user.
func (r *PasswordResetRegistry) DeleteByUserID(ctx context.Context, userID string) error {
	return r.newRepo().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE user_id = $1`, r.tableNames.PasswordResets())
		_, err := tx.ExecContext(ctx, query, userID)
		return err
	})
}

// DeleteExpired removes all records whose ExpiresAt timestamp is in the past.
func (r *PasswordResetRegistry) DeleteExpired(ctx context.Context) error {
	return r.newRepo().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE expires_at < $1`, r.tableNames.PasswordResets())
		_, err := tx.ExecContext(ctx, query, time.Now())
		return err
	})
}
