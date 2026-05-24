package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.OAuthIdentityRegistry = (*OAuthIdentityRegistry)(nil)

// pgUniqueViolationCode is the SQLSTATE Postgres returns when a UNIQUE
// constraint trips. Used to map the (provider, provider_user_id) collision
// onto registry.ErrAlreadyExists so the OAuth callback can branch on the
// classified sentinel rather than parsing error strings.
const pgUniqueViolationCode = "23505"

// OAuthIdentityRegistry provides PostgreSQL-backed storage for OAuth
// identity records. It uses a NonRLSRepository because the callback
// resolves identities before any user session exists; the
// `oauth_identity_background_worker_access` RLS policy on the table
// covers the read path.
type OAuthIdentityRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewOAuthIdentityRegistry creates a new OAuthIdentityRegistry backed by
// the given database.
func NewOAuthIdentityRegistry(dbx *sqlx.DB) *OAuthIdentityRegistry {
	return &OAuthIdentityRegistry{dbx: dbx, tableNames: store.DefaultTableNames}
}

func (r *OAuthIdentityRegistry) newRepo() *store.NonRLSRepository[models.OAuthIdentity, *models.OAuthIdentity] {
	return store.NewSQLRegistry[models.OAuthIdentity, *models.OAuthIdentity](r.dbx, r.tableNames.UserOAuthIdentities())
}

// Create inserts a new OAuth identity row. Maps Postgres unique-constraint
// violations on (provider, provider_user_id) onto registry.ErrAlreadyExists.
func (r *OAuthIdentityRegistry) Create(ctx context.Context, oi models.OAuthIdentity) (*models.OAuthIdentity, error) {
	if oi.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if oi.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if !oi.Provider.IsValid() {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Provider"))
	}
	if oi.ProviderUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ProviderUserID"))
	}
	if oi.LinkedAt.IsZero() {
		oi.LinkedAt = time.Now()
	}
	created, err := r.newRepo().Create(ctx, oi, nil)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {
			return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs(
				"entity_type", "OAuthIdentity",
				"provider", string(oi.Provider),
				"provider_user_id", oi.ProviderUserID,
			))
		}
		return nil, errxtrace.Wrap("failed to create OAuth identity", err)
	}
	return &created, nil
}

// Get returns an OAuth identity record by ID.
func (r *OAuthIdentityRegistry) Get(ctx context.Context, id string) (*models.OAuthIdentity, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	var oi models.OAuthIdentity
	if err := r.newRepo().ScanOneByField(ctx, store.Pair("id", id), &oi); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "OAuthIdentity", "entity_id", id))
		}
		return nil, errxtrace.Wrap("failed to get OAuth identity", err)
	}
	return &oi, nil
}

// List returns all OAuth identity records.
func (r *OAuthIdentityRegistry) List(ctx context.Context) ([]*models.OAuthIdentity, error) {
	var result []*models.OAuthIdentity
	for oi, err := range r.newRepo().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list OAuth identities", err)
		}
		result = append(result, &oi)
	}
	return result, nil
}

// Update modifies an existing OAuth identity record.
func (r *OAuthIdentityRegistry) Update(ctx context.Context, oi models.OAuthIdentity) (*models.OAuthIdentity, error) {
	if oi.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.newRepo().Update(ctx, oi, nil); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "OAuthIdentity", "entity_id", oi.GetID()))
		}
		return nil, errxtrace.Wrap("failed to update OAuth identity", err)
	}
	return &oi, nil
}

// Delete removes an OAuth identity record by ID.
func (r *OAuthIdentityRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.newRepo().Delete(ctx, id, nil); err != nil {
		return errxtrace.Wrap("failed to delete OAuth identity", err)
	}
	return nil
}

// Count returns the total number of OAuth identity records.
func (r *OAuthIdentityRegistry) Count(ctx context.Context) (int, error) {
	count, err := r.newRepo().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count OAuth identities", err)
	}
	return count, nil
}

// GetByProviderSubject returns the row keyed by (provider, providerUserID).
func (r *OAuthIdentityRegistry) GetByProviderSubject(ctx context.Context, provider models.OAuthProvider, providerUserID string) (*models.OAuthIdentity, error) {
	if !provider.IsValid() {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Provider"))
	}
	if providerUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ProviderUserID"))
	}
	var oi models.OAuthIdentity
	query := fmt.Sprintf(
		`SELECT * FROM %s WHERE provider = $1 AND provider_user_id = $2 LIMIT 1`,
		r.tableNames.UserOAuthIdentities(),
	)
	if err := r.dbx.GetContext(ctx, &oi, query, string(provider), providerUserID); err != nil {
		if isNoRowsErr(err) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "OAuthIdentity"))
		}
		return nil, errxtrace.Wrap("failed to look up OAuth identity by provider subject", err)
	}
	return &oi, nil
}

// ListByUser returns every identity linked to userID within tenantID.
func (r *OAuthIdentityRegistry) ListByUser(ctx context.Context, tenantID, userID string) ([]*models.OAuthIdentity, error) {
	if tenantID == "" || userID == "" {
		return nil, nil
	}
	query := fmt.Sprintf(
		`SELECT * FROM %s WHERE tenant_id = $1 AND user_id = $2 ORDER BY provider ASC`,
		r.tableNames.UserOAuthIdentities(),
	)
	var rows []*models.OAuthIdentity
	if err := r.dbx.SelectContext(ctx, &rows, query, tenantID, userID); err != nil {
		return nil, errxtrace.Wrap("failed to list OAuth identities for user", err)
	}
	return rows, nil
}

// GetByUserAndProvider returns the (tenantID, userID, provider) row.
func (r *OAuthIdentityRegistry) GetByUserAndProvider(ctx context.Context, tenantID, userID string, provider models.OAuthProvider) (*models.OAuthIdentity, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if !provider.IsValid() {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Provider"))
	}
	var oi models.OAuthIdentity
	query := fmt.Sprintf(
		`SELECT * FROM %s WHERE tenant_id = $1 AND user_id = $2 AND provider = $3 LIMIT 1`,
		r.tableNames.UserOAuthIdentities(),
	)
	if err := r.dbx.GetContext(ctx, &oi, query, tenantID, userID, string(provider)); err != nil {
		if isNoRowsErr(err) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "OAuthIdentity"))
		}
		return nil, errxtrace.Wrap("failed to look up OAuth identity by user/provider", err)
	}
	return &oi, nil
}

// DeleteByUserAndProvider removes (tenantID, userID, provider) idempotently.
func (r *OAuthIdentityRegistry) DeleteByUserAndProvider(ctx context.Context, tenantID, userID string, provider models.OAuthProvider) error {
	if tenantID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if !provider.IsValid() {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Provider"))
	}
	query := fmt.Sprintf(
		`DELETE FROM %s WHERE tenant_id = $1 AND user_id = $2 AND provider = $3`,
		r.tableNames.UserOAuthIdentities(),
	)
	if _, err := r.dbx.ExecContext(ctx, query, tenantID, userID, string(provider)); err != nil {
		return errxtrace.Wrap("failed to delete OAuth identity", err)
	}
	return nil
}

// isNoRowsErr reports whether err is a database-level "no rows" signal.
// sqlx normalises both pgx's no-rows and lib/pq's no-rows into
// sql.ErrNoRows when GetContext returns; the helper keeps the call sites
// terse.
func isNoRowsErr(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
