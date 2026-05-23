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

var _ registry.BackofficeRefreshTokenRegistry = (*BackofficeRefreshTokenRegistry)(nil)

// BackofficeRefreshTokenRegistry is the postgres-backed implementation
// of the back-office refresh-token store (issue #1785, Phase 2). The
// underlying table has NO row-level security — the login flow runs
// before any DB session context is set, so an RLS predicate that read
// `get_current_*_id()` would block the very call that needs to look up
// its own row. The registry uses a NonRLSRepository for the same reason
// BackofficeUserRegistry does.
type BackofficeRefreshTokenRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewBackofficeRefreshTokenRegistry returns a postgres-backed registry
// using the default table-name set.
func NewBackofficeRefreshTokenRegistry(dbx *sqlx.DB) *BackofficeRefreshTokenRegistry {
	return NewBackofficeRefreshTokenRegistryWithTableNames(dbx, store.DefaultTableNames)
}

// NewBackofficeRefreshTokenRegistryWithTableNames lets tests override the
// table names (mirrors every other postgres registry in this package).
func NewBackofficeRefreshTokenRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *BackofficeRefreshTokenRegistry {
	return &BackofficeRefreshTokenRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *BackofficeRefreshTokenRegistry) newSQLRegistry() *store.NonRLSRepository[models.BackofficeRefreshToken, *models.BackofficeRefreshToken] {
	return store.NewSQLRegistry[models.BackofficeRefreshToken, *models.BackofficeRefreshToken](r.dbx, r.tableNames.BackofficeRefreshTokens())
}

// Create stamps id + uuid + created_at and inserts the row.
func (r *BackofficeRefreshTokenRegistry) Create(ctx context.Context, token models.BackofficeRefreshToken) (*models.BackofficeRefreshToken, error) {
	if token.TokenHash == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TokenHash"))
	}
	if token.BackofficeUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}

	token.CreatedAt = time.Now()
	token.ID = uuid.New().String()
	if token.UUID == "" {
		token.UUID = uuid.New().String()
	}

	reg := r.newSQLRegistry()
	if err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		txReg := store.NewTxRegistry[models.BackofficeRefreshToken](tx, r.tableNames.BackofficeRefreshTokens())
		return txReg.Insert(ctx, token)
	}); err != nil {
		return nil, errxtrace.Wrap("failed to insert backoffice refresh token", err)
	}

	return &token, nil
}

// Get returns a single row by id.
func (r *BackofficeRefreshTokenRegistry) Get(ctx context.Context, id string) (*models.BackofficeRefreshToken, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var token models.BackofficeRefreshToken
	reg := r.newSQLRegistry()
	if err := reg.ScanOneByField(ctx, store.Pair("id", id), &token); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrBackofficeRefreshTokenNotFound, errx.Attrs("entity_id", id))
		}
		return nil, errxtrace.Wrap("failed to get backoffice refresh token", err)
	}
	return &token, nil
}

// List walks the whole table. Used by tests; the production read paths
// always go through GetByHash / ListActiveByBackofficeUserID.
func (r *BackofficeRefreshTokenRegistry) List(ctx context.Context) ([]*models.BackofficeRefreshToken, error) {
	var tokens []*models.BackofficeRefreshToken
	reg := r.newSQLRegistry()

	for token, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list backoffice refresh tokens", err)
		}
		tokens = append(tokens, &token)
	}

	return tokens, nil
}

// Update rewrites a row by id. Currently used only by the refresh
// handler to bump LastUsedAt.
func (r *BackofficeRefreshTokenRegistry) Update(ctx context.Context, token models.BackofficeRefreshToken) (*models.BackofficeRefreshToken, error) {
	if token.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()
	if err := reg.Update(ctx, token, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update backoffice refresh token", err)
	}
	return &token, nil
}

// Delete removes a row by id.
func (r *BackofficeRefreshTokenRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()
	if err := reg.Delete(ctx, id, nil); err != nil {
		return errxtrace.Wrap("failed to delete backoffice refresh token", err)
	}
	return nil
}

// Count returns the total number of rows.
func (r *BackofficeRefreshTokenRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()
	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count backoffice refresh tokens", err)
	}
	return count, nil
}

// GetByHash returns the row whose token_hash matches.
func (r *BackofficeRefreshTokenRegistry) GetByHash(ctx context.Context, tokenHash string) (*models.BackofficeRefreshToken, error) {
	if tokenHash == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TokenHash"))
	}

	var token models.BackofficeRefreshToken
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE token_hash = $1`, r.tableNames.BackofficeRefreshTokens())
		err := tx.GetContext(ctx, &token, query, tokenHash)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrBackofficeRefreshTokenNotFound)
			}
			return errxtrace.Wrap("failed to get backoffice refresh token by hash", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &token, nil
}

// Revoke marks a single row as revoked, gated on backofficeUserID so a
// guessed id can't be used to revoke a session that belongs to someone
// else. Returns ErrBackofficeRefreshTokenNotFound when no row matches
// the (id, backofficeUserID) pair, mirroring RefreshTokenRegistry.RevokeByID's
// not-found semantics.
func (r *BackofficeRefreshTokenRegistry) Revoke(ctx context.Context, backofficeUserID, id string) error {
	if backofficeUserID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		now := time.Now()
		query := fmt.Sprintf(
			`UPDATE %s SET revoked_at = $1 WHERE id = $2 AND backoffice_user_id = $3 AND revoked_at IS NULL`,
			r.tableNames.BackofficeRefreshTokens(),
		)
		res, err := tx.ExecContext(ctx, query, now, id, backofficeUserID)
		if err != nil {
			return errxtrace.Wrap("failed to revoke backoffice refresh token", err)
		}
		n, rerr := res.RowsAffected()
		if rerr != nil {
			return errxtrace.Wrap("failed to read rows affected on backoffice refresh token revoke", rerr)
		}
		if n > 0 {
			return nil
		}
		// Distinguish "no matching row" from "already revoked" so an
		// idempotent revoke succeeds while a cross-user id correctly
		// 404s — same shape as RefreshTokenRegistry.RevokeByID.
		var existing int
		probe := fmt.Sprintf(`SELECT 1 FROM %s WHERE id = $1 AND backoffice_user_id = $2`, r.tableNames.BackofficeRefreshTokens())
		perr := tx.GetContext(ctx, &existing, probe, id, backofficeUserID)
		switch {
		case errors.Is(perr, sql.ErrNoRows):
			return errxtrace.Classify(registry.ErrBackofficeRefreshTokenNotFound, errx.Attrs("entity_id", id))
		case perr != nil:
			return errxtrace.Wrap("failed to probe backoffice refresh token existence", perr)
		}
		// Row exists and was already revoked — idempotent success.
		return nil
	})
}

// ListActiveByBackofficeUserID returns active rows ordered LastUsedAt
// desc (CreatedAt desc tiebreaker for never-used tokens).
func (r *BackofficeRefreshTokenRegistry) ListActiveByBackofficeUserID(ctx context.Context, backofficeUserID string) ([]*models.BackofficeRefreshToken, error) {
	if backofficeUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}

	var tokens []*models.BackofficeRefreshToken
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE backoffice_user_id = $1 AND revoked_at IS NULL AND expires_at > $2 `+
				`ORDER BY last_used_at DESC NULLS LAST, created_at DESC`,
			r.tableNames.BackofficeRefreshTokens(),
		)
		return tx.SelectContext(ctx, &tokens, query, backofficeUserID, time.Now())
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list active backoffice refresh tokens", err)
	}
	return tokens, nil
}

// RevokeByBackofficeUserID marks every active row for the user as revoked.
func (r *BackofficeRefreshTokenRegistry) RevokeByBackofficeUserID(ctx context.Context, backofficeUserID string) error {
	if backofficeUserID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "BackofficeUserID"))
	}

	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		now := time.Now()
		query := fmt.Sprintf(
			`UPDATE %s SET revoked_at = $1 WHERE backoffice_user_id = $2 AND revoked_at IS NULL`,
			r.tableNames.BackofficeRefreshTokens(),
		)
		if _, err := tx.ExecContext(ctx, query, now, backofficeUserID); err != nil {
			return errxtrace.Wrap("failed to revoke backoffice refresh tokens by user", err)
		}
		return nil
	})
}

// DeleteExpired removes all rows whose expires_at is in the past.
func (r *BackofficeRefreshTokenRegistry) DeleteExpired(ctx context.Context) error {
	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE expires_at < $1`, r.tableNames.BackofficeRefreshTokens())
		if _, err := tx.ExecContext(ctx, query, time.Now()); err != nil {
			return errxtrace.Wrap("failed to delete expired backoffice refresh tokens", err)
		}
		return nil
	})
}
