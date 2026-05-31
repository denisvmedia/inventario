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

var _ registry.MagicLinkTokenRegistry = (*MagicLinkTokenRegistry)(nil)

// MagicLinkTokenRegistry provides PostgreSQL-backed storage for magic-link
// sign-in tokens. It uses a NonRLSRepository because tokens are resolved
// before a user session exists.
type MagicLinkTokenRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewMagicLinkTokenRegistry creates a new MagicLinkTokenRegistry backed by the given database.
func NewMagicLinkTokenRegistry(dbx *sqlx.DB) *MagicLinkTokenRegistry {
	return &MagicLinkTokenRegistry{dbx: dbx, tableNames: store.DefaultTableNames}
}

func (r *MagicLinkTokenRegistry) newRepo() *store.NonRLSRepository[models.MagicLinkToken, *models.MagicLinkToken] {
	return store.NewSQLRegistry[models.MagicLinkToken, *models.MagicLinkToken](r.dbx, r.tableNames.MagicLinkTokens())
}

// Create inserts a new magic-link token record.
func (r *MagicLinkTokenRegistry) Create(ctx context.Context, mlt models.MagicLinkToken) (*models.MagicLinkToken, error) {
	if mlt.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if mlt.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if mlt.Token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}
	mlt.CreatedAt = time.Now()
	created, err := r.newRepo().Create(ctx, mlt, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create magic link token", err)
	}
	return &created, nil
}

// Get returns a magic-link token record by ID.
func (r *MagicLinkTokenRegistry) Get(ctx context.Context, id string) (*models.MagicLinkToken, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	var mlt models.MagicLinkToken
	if err := r.newRepo().ScanOneByField(ctx, store.Pair("id", id), &mlt); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "MagicLinkToken", "entity_id", id))
		}
		return nil, errxtrace.Wrap("failed to get magic link token", err)
	}
	return &mlt, nil
}

// List returns all magic-link token records.
func (r *MagicLinkTokenRegistry) List(ctx context.Context) ([]*models.MagicLinkToken, error) {
	var result []*models.MagicLinkToken
	for mlt, err := range r.newRepo().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list magic link tokens", err)
		}
		result = append(result, &mlt)
	}
	return result, nil
}

// Update modifies an existing magic-link token record.
func (r *MagicLinkTokenRegistry) Update(ctx context.Context, mlt models.MagicLinkToken) (*models.MagicLinkToken, error) {
	if mlt.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.newRepo().Update(ctx, mlt, nil); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "MagicLinkToken", "entity_id", mlt.GetID()))
		}
		return nil, errxtrace.Wrap("failed to update magic link token", err)
	}
	return &mlt, nil
}

// Delete removes a magic-link token record by ID.
func (r *MagicLinkTokenRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.newRepo().Delete(ctx, id, nil); err != nil {
		return errxtrace.Wrap("failed to delete magic link token", err)
	}
	return nil
}

// Count returns the total number of magic-link token records.
func (r *MagicLinkTokenRegistry) Count(ctx context.Context) (int, error) {
	count, err := r.newRepo().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count magic link tokens", err)
	}
	return count, nil
}

// GetByToken returns the magic-link record matching the given token.
func (r *MagicLinkTokenRegistry) GetByToken(ctx context.Context, token string) (*models.MagicLinkToken, error) {
	if token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}
	var mlt models.MagicLinkToken
	if err := r.newRepo().ScanOneByField(ctx, store.Pair("token", token), &mlt); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "MagicLinkToken"))
		}
		return nil, errxtrace.Wrap("failed to get magic link token by token", err)
	}
	return &mlt, nil
}

// DeleteByUserID removes all magic-link token records for the given user.
func (r *MagicLinkTokenRegistry) DeleteByUserID(ctx context.Context, userID string) error {
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	return r.newRepo().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE user_id = $1`, r.tableNames.MagicLinkTokens())
		_, err := tx.ExecContext(ctx, query, userID)
		return err
	})
}

// MarkClaimed atomically flips claimed_at from NULL to the current time for
// the row matching token, returning whether this call won the claim. The
// `claimed_at IS NULL AND expires_at > $1` filter makes the write idempotent
// across concurrent requests and folds the expiry check in: exactly one of N
// callers carrying the same live token changes a row (rows-affected == 1 →
// true), and the rest — plus a non-existent, already-claimed, or expired token
// — observe zero rows-affected → false. See the interface doc.
func (r *MagicLinkTokenRegistry) MarkClaimed(ctx context.Context, token string) (bool, error) {
	if token == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}
	var claimed bool
	err := r.newRepo().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET claimed_at = $1 WHERE token = $2 AND claimed_at IS NULL AND expires_at > $1`,
			r.tableNames.MagicLinkTokens(),
		)
		now := time.Now()
		res, err := tx.ExecContext(ctx, query, now, token)
		if err != nil {
			return errxtrace.Wrap("failed to mark magic link token as claimed", err)
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
func (r *MagicLinkTokenRegistry) DeleteExpired(ctx context.Context) error {
	return r.newRepo().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE expires_at < $1`, r.tableNames.MagicLinkTokens())
		_, err := tx.ExecContext(ctx, query, time.Now())
		return err
	})
}
