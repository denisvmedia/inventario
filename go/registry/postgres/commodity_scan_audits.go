package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.CommodityScanAuditRegistry = (*CommodityScanAuditRegistry)(nil)

// CommodityScanAuditRegistry is the postgres-backed implementation of
// registry.CommodityScanAuditRegistry (#1720). It uses NonRLSRepository
// because the audit row is written by service code that may be
// finishing up after the request RLS context has already gone (e.g.
// on rate-limit reject before user identity was resolved into RLS) —
// the row carries explicit (tenant_id, user_id) columns so RLS can
// still apply to *reads* via the configured policies, but writes are
// intentionally bypassed.
type CommodityScanAuditRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewCommodityScanAuditRegistry constructs a CommodityScanAuditRegistry.
func NewCommodityScanAuditRegistry(dbx *sqlx.DB) *CommodityScanAuditRegistry {
	return &CommodityScanAuditRegistry{dbx: dbx, tableNames: store.DefaultTableNames}
}

func (r *CommodityScanAuditRegistry) newRepo() *store.NonRLSRepository[models.CommodityScanAudit, *models.CommodityScanAudit] {
	return store.NewSQLRegistry[models.CommodityScanAudit, *models.CommodityScanAudit](r.dbx, r.tableNames.CommodityScanAudits())
}

// Record persists a new commodity scan audit row.
func (r *CommodityScanAuditRegistry) Record(ctx context.Context, audit models.CommodityScanAudit) (*models.CommodityScanAudit, error) {
	if audit.UserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if audit.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if audit.Status == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Status"))
	}
	if audit.CreatedAt.IsZero() {
		audit.CreatedAt = time.Now()
	}
	created, err := r.newRepo().Create(ctx, audit, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to record commodity scan audit", err)
	}
	return &created, nil
}

// CountRecentForUser returns the number of audit rows for
// (tenantID, userID) created at or after since. The explicit tenant_id
// predicate is the contract guarantee that the memory implementation
// also enforces; postgres deployments can additionally rely on RLS via
// a user-scoped registry, but the predicate stays for parity so the
// caller doesn't need to know which mode the registry was built in.
func (r *CommodityScanAuditRegistry) CountRecentForUser(ctx context.Context, tenantID, userID string, since time.Time) (int, error) {
	if tenantID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	var count int
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE tenant_id = $1 AND user_id = $2 AND created_at >= $3`, r.tableNames.CommodityScanAudits())
	if err := r.dbx.GetContext(ctx, &count, query, tenantID, userID, since); err != nil {
		return 0, errxtrace.Wrap("count recent commodity scan audits", err)
	}
	return count, nil
}

// DeleteOlderThan removes every audit row whose created_at is before
// cutoff. The retention worker calls this on a schedule.
func (r *CommodityScanAuditRegistry) DeleteOlderThan(ctx context.Context, cutoff time.Time) error {
	return r.newRepo().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE created_at < $1`, r.tableNames.CommodityScanAudits())
		_, err := tx.ExecContext(ctx, query, cutoff)
		return err
	})
}
