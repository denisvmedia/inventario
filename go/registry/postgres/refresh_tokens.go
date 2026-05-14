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

// newSQLRegistry returns an RLSRepository in service mode for the refresh_tokens table.
// It uses beginServiceTx under the hood, which explicitly sets the
// inventario_background_worker role before executing queries. This role is
// covered by the refresh_token_background_worker_access RLS policy, giving
// full cross-tenant access without requiring BYPASSRLS on the app user.
// This is required for auth flows (e.g. /auth/refresh, /auth/login) where
// no user/tenant context has been established in the database session yet.
func (r *RefreshTokenRegistry) newSQLRegistry() *store.RLSRepository[models.RefreshToken, *models.RefreshToken] {
	return store.NewServiceSQLRegistry[models.RefreshToken, *models.RefreshToken](r.dbx, r.tableNames.RefreshTokens())
}

func (r *RefreshTokenRegistry) Create(ctx context.Context, token models.RefreshToken) (*models.RefreshToken, error) {
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
	token.ID = uuid.New().String()
	if token.UUID == "" {
		token.UUID = uuid.New().String()
	}

	reg := r.newSQLRegistry()
	if err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		txReg := store.NewTxRegistry[models.RefreshToken](tx, r.tableNames.RefreshTokens())
		return txReg.Insert(ctx, token)
	}); err != nil {
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

// ListActiveByUserID returns refresh tokens for a user that are neither
// revoked nor expired, ordered by LastUsedAt desc (CreatedAt desc as the
// tiebreaker for never-used rows). Implemented as a single SQL query so
// pagination behaviour stays predictable even if a user accumulates many
// stale tokens before the retention sweep clears them.
func (r *RefreshTokenRegistry) ListActiveByUserID(ctx context.Context, userID string) ([]*models.RefreshToken, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	var tokens []*models.RefreshToken
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $2 `+
				`ORDER BY last_used_at DESC NULLS LAST, created_at DESC`,
			r.tableNames.RefreshTokens(),
		)
		return tx.SelectContext(ctx, &tokens, query, userID, time.Now())
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list active refresh tokens by user", err)
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

// RevokeByID revokes a single refresh token by id, gated on user_id so
// a user can't revoke someone else's session via a guessed id. Returns
// ErrNotFound when the (id, user_id) pair matches no row.
func (r *RefreshTokenRegistry) RevokeByID(ctx context.Context, userID, id string) error {
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		now := time.Now()
		query := fmt.Sprintf(
			`UPDATE %s SET revoked_at = $1 WHERE id = $2 AND user_id = $3 AND revoked_at IS NULL`,
			r.tableNames.RefreshTokens(),
		)
		res, err := tx.ExecContext(ctx, query, now, id, userID)
		if err != nil {
			return errxtrace.Wrap("failed to revoke refresh token by id", err)
		}
		// Distinguish "no matching row" from "row exists but is
		// already revoked". The UPDATE's WHERE includes
		// `revoked_at IS NULL`, so zero rows affected covers both
		// shapes — but only the genuinely-missing one should surface
		// as 404. An already-revoked row is treated as a successful
		// no-op (idempotent revoke), and a cross-user id correctly
		// 404s without revealing whether the id exists for someone
		// else.
		n, rerr := res.RowsAffected()
		if rerr != nil {
			// Driver couldn't report rows affected — treat as a
			// transient error rather than silently succeeding;
			// silent-success on a probe failure could hide that
			// the revoke didn't actually land.
			return errxtrace.Wrap("failed to read rows affected on revoke", rerr)
		}
		if n > 0 {
			return nil
		}
		var existing int
		probe := fmt.Sprintf(`SELECT 1 FROM %s WHERE id = $1 AND user_id = $2`, r.tableNames.RefreshTokens())
		perr := tx.GetContext(ctx, &existing, probe, id, userID)
		switch {
		case errors.Is(perr, sql.ErrNoRows):
			// Truly missing — surface as ErrNotFound.
			return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "RefreshToken", "entity_id", id))
		case perr != nil:
			// Anything else (connection drop, permission denied)
			// must NOT be swallowed — return so the handler 500s
			// rather than reporting a no-op revoke that didn't
			// happen.
			return errxtrace.Wrap("failed to probe refresh token existence", perr)
		}
		// Row exists and was already revoked — idempotent success.
		return nil
	})
}

// RevokeAllExceptID revokes every refresh token for the user except
// the row whose id matches keepID. Pass an empty keepID to revoke
// every token (equivalent to RevokeByUserID).
func (r *RefreshTokenRegistry) RevokeAllExceptID(ctx context.Context, userID, keepID string) error {
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		now := time.Now()
		var (
			query string
			args  []any
		)
		if keepID == "" {
			query = fmt.Sprintf(
				`UPDATE %s SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL`,
				r.tableNames.RefreshTokens(),
			)
			args = []any{now, userID}
		} else {
			query = fmt.Sprintf(
				`UPDATE %s SET revoked_at = $1 WHERE user_id = $2 AND id <> $3 AND revoked_at IS NULL`,
				r.tableNames.RefreshTokens(),
			)
			args = []any{now, userID, keepID}
		}
		_, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return errxtrace.Wrap("failed to revoke refresh tokens", err)
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
