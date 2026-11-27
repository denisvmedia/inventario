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

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

var _ registry.APITokenRegistry = (*APITokenRegistry)(nil)

// APITokenRegistry stores personal API tokens. Every query runs as
// inventario_background_worker (service mode): authentication resolves a token
// by digest before any user context is set, which the inventario_app policy
// would refuse. Each statement that touches a user's rows names the user_id
// itself, so the wider role does not widen what a caller can reach.
type APITokenRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewAPITokenRegistry returns a registry on the default table names.
func NewAPITokenRegistry(dbx *sqlx.DB) *APITokenRegistry {
	return NewAPITokenRegistryWithTableNames(dbx, store.DefaultTableNames)
}

// NewAPITokenRegistryWithTableNames returns a registry on custom table names.
func NewAPITokenRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *APITokenRegistry {
	return &APITokenRegistry{dbx: dbx, tableNames: tableNames}
}

func (r *APITokenRegistry) newSQLRegistry() *store.RLSRepository[models.APIToken, *models.APIToken] {
	return store.NewServiceSQLRegistry[models.APIToken, *models.APIToken](r.dbx, r.tableNames.APITokens())
}

func (r *APITokenRegistry) Create(ctx context.Context, token models.APIToken) (*models.APIToken, error) {
	switch {
	case token.TokenHash == "":
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TokenHash"))
	case token.UserID == "":
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	case token.TenantID == "":
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	case token.Name == "":
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	case !token.Scope.IsValid():
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Scope"))
	}

	token.ID = uuid.New().String()
	token.UUID = uuid.New().String()
	token.CreatedAt = time.Now()
	token.LastUsedAt = nil
	token.RevokedAt = nil

	reg := r.newSQLRegistry()
	if err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if token.GroupID != nil {
			if err := r.checkGroupMember(ctx, tx, token); err != nil {
				return err
			}
		}
		txReg := store.NewTxRegistry[models.APIToken](tx, r.tableNames.APITokens())
		return txReg.Insert(ctx, token)
	}); err != nil {
		return nil, errxtrace.Wrap("failed to insert api token", err)
	}
	return &token, nil
}

// checkGroupMember takes the group's membership lock and fails with
// ErrNotAGroupMember unless the token's user is still a member, so the insert
// cannot land after a removal that already revoked the group's tokens.
func (r *APITokenRegistry) checkGroupMember(ctx context.Context, tx *sqlx.Tx, token models.APIToken) error {
	if err := lockGroupMemberships(ctx, tx, *token.GroupID); err != nil {
		return err
	}
	var member bool
	query := fmt.Sprintf(
		`SELECT EXISTS (SELECT 1 FROM %s WHERE tenant_id = $1 AND group_id = $2 AND member_user_id = $3)`,
		r.tableNames.GroupMemberships())
	if err := tx.GetContext(ctx, &member, query, token.TenantID, *token.GroupID, token.UserID); err != nil {
		return errxtrace.Wrap("failed to check group membership for api token", err)
	}
	if !member {
		return errxtrace.Classify(registry.ErrNotAGroupMember, errx.Attrs("group_id", *token.GroupID))
	}
	return nil
}

func (r *APITokenRegistry) GetByTokenHash(ctx context.Context, tokenHash string) (*models.APIToken, error) {
	if tokenHash == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TokenHash"))
	}

	var token models.APIToken
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE token_hash = $1`, r.tableNames.APITokens())
		if err := tx.GetContext(ctx, &token, query, tokenHash); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "APIToken"))
			}
			return errxtrace.Wrap("failed to get api token by hash", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *APITokenRegistry) ListByUserID(ctx context.Context, userID string) ([]*models.APIToken, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	tokens := make([]*models.APIToken, 0)
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE user_id = $1 AND revoked_at IS NULL ORDER BY created_at DESC, id`,
			r.tableNames.APITokens(),
		)
		return tx.SelectContext(ctx, &tokens, query, userID)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list api tokens by user", err)
	}
	return tokens, nil
}

func (r *APITokenRegistry) CountActiveByUserID(ctx context.Context, userID string, now time.Time) (int, error) {
	if userID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	var count int
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s WHERE user_id = $1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > $2)`,
			r.tableNames.APITokens(),
		)
		return tx.GetContext(ctx, &count, query, userID, now)
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to count api tokens", err)
	}
	return count, nil
}

func (r *APITokenRegistry) RevokeByID(ctx context.Context, userID, id string) (*models.APIToken, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var token models.APIToken
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// COALESCE keeps the first revocation time on a repeated revoke, so
		// the call is idempotent and still returns the stored row.
		query := fmt.Sprintf(
			`UPDATE %s SET revoked_at = COALESCE(revoked_at, $1) WHERE id = $2 AND user_id = $3 RETURNING *`,
			r.tableNames.APITokens(),
		)
		if err := tx.GetContext(ctx, &token, query, time.Now(), id, userID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "APIToken", "entity_id", id))
			}
			return errxtrace.Wrap("failed to revoke api token", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *APITokenRegistry) RevokeAllByUserID(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	var revoked int64
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL`,
			r.tableNames.APITokens(),
		)
		res, err := tx.ExecContext(ctx, query, time.Now(), userID)
		if err != nil {
			return errxtrace.Wrap("failed to revoke api tokens by user", err)
		}
		revoked, err = res.RowsAffected()
		if err != nil {
			return errxtrace.Wrap("failed to read rows affected on api token revoke", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int(revoked), nil
}

func (r *APITokenRegistry) TouchLastUsed(ctx context.Context, id string, at, staleBefore time.Time) (bool, error) {
	if id == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var touched int64
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET last_used_at = $1 WHERE id = $2 AND revoked_at IS NULL AND (last_used_at IS NULL OR last_used_at < $3)`,
			r.tableNames.APITokens(),
		)
		res, err := tx.ExecContext(ctx, query, at, id, staleBefore)
		if err != nil {
			return errxtrace.Wrap("failed to update api token last_used_at", err)
		}
		touched, err = res.RowsAffected()
		if err != nil {
			return errxtrace.Wrap("failed to read rows affected on api token touch", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return touched > 0, nil
}
