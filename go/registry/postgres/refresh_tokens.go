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

var _ registry.RefreshTokenRegistry = (*RefreshTokenRegistry)(nil)

type RefreshTokenRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewRefreshTokenRegistry(dbx *sqlx.DB) *RefreshTokenRegistry {
	return NewRefreshTokenRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewRefreshTokenRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *RefreshTokenRegistry {
	return &RefreshTokenRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// newSQLRegistry returns a NonRLSRepository for the refresh_tokens table.
// This registry operates on a DB connection that either has BYPASSRLS or
// uses the superuser role, intentionally skipping the row-level security
// policies that filter by tenant_id/user_id. This is required for auth
// flows (e.g. /auth/refresh, /auth/login) where no user context has been
// established yet in the database session.
func (r *RefreshTokenRegistry) newSQLRegistry() *store.NonRLSRepository[models.RefreshToken, *models.RefreshToken] {
	return store.NewSQLRegistry[models.RefreshToken](r.dbx, r.tableNames.RefreshTokens())
}

func (r *RefreshTokenRegistry) Create(ctx context.Context, token models.RefreshToken) (_ *models.RefreshToken, err error) {
	if token.TokenHash == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TokenHash"))
	}
	if token.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if token.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	token.CreatedAt = time.Now()

	tx, txErr := r.dbx.Beginx()
	if txErr != nil {
		return nil, errxtrace.Wrap("failed to begin transaction", txErr)
	}
	defer func() {
		err = errors.Join(err, store.RollbackOrCommit(tx, err))
	}()

	txReg := store.NewTxRegistry[models.RefreshToken](tx, r.tableNames.RefreshTokens())
	token.ID = uuid.New().String()
	if err = txReg.Insert(ctx, token); err != nil {
		return nil, errxtrace.Wrap("failed to insert refresh token", err)
	}

	return &token, nil
}

func (r *RefreshTokenRegistry) Get(ctx context.Context, id string) (*models.RefreshToken, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var token models.RefreshToken
	reg := r.newSQLRegistry()
	err := reg.ScanOneByField(ctx, store.Pair("id", id), &token)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "RefreshToken", "entity_id", id))
		}
		return nil, errxtrace.Wrap("failed to get refresh token", err)
	}

	return &token, nil
}

func (r *RefreshTokenRegistry) List(ctx context.Context) ([]*models.RefreshToken, error) {
	var tokens []*models.RefreshToken
	reg := r.newSQLRegistry()

	for token, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list refresh tokens", err)
		}
		tokens = append(tokens, &token)
	}

	return tokens, nil
}

func (r *RefreshTokenRegistry) Update(ctx context.Context, token models.RefreshToken) (*models.RefreshToken, error) {
	if token.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()
	err := reg.Update(ctx, token, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update refresh token", err)
	}

	return &token, nil
}

func (r *RefreshTokenRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errxtrace.Wrap("failed to delete refresh token", err)
	}

	return nil
}

func (r *RefreshTokenRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()
	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count refresh tokens", err)
	}
	return count, nil
}

// GetByTokenHash returns a refresh token by its SHA-256 hash.
func (r *RefreshTokenRegistry) GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	if tokenHash == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TokenHash"))
	}

	var token models.RefreshToken
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE token_hash = $1`, r.tableNames.RefreshTokens())
		err := tx.GetContext(ctx, &token, query, tokenHash)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "RefreshToken"))
			}
			return errxtrace.Wrap("failed to get refresh token by hash", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &token, nil
}

// GetByUserID returns all refresh tokens for a user.
func (r *RefreshTokenRegistry) GetByUserID(ctx context.Context, userID string) ([]*models.RefreshToken, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	var tokens []*models.RefreshToken
	reg := r.newSQLRegistry()

	for token, err := range reg.ScanByField(ctx, store.Pair("user_id", userID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list refresh tokens by user", err)
		}
		tokens = append(tokens, &token)
	}

	return tokens, nil
}

// RevokeByUserID marks all refresh tokens for a user as revoked.
func (r *RefreshTokenRegistry) RevokeByUserID(ctx context.Context, userID string) error {
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		now := time.Now()
		query := fmt.Sprintf(
			`UPDATE %s SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL`,
			r.tableNames.RefreshTokens(),
		)
		_, err := tx.ExecContext(ctx, query, now, userID)
		if err != nil {
			return errxtrace.Wrap("failed to revoke refresh tokens by user", err)
		}
		return nil
	})
}

// DeleteExpired removes all expired refresh tokens.
func (r *RefreshTokenRegistry) DeleteExpired(ctx context.Context) error {
	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE expires_at < $1`, r.tableNames.RefreshTokens())
		_, err := tx.ExecContext(ctx, query, time.Now())
		if err != nil {
			return errxtrace.Wrap("failed to delete expired refresh tokens", err)
		}
		return nil
	})
}
