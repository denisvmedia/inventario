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

var _ registry.AuditLogRegistry = (*AuditLogRegistry)(nil)

// AuditLogRegistry provides PostgreSQL-backed storage for audit log entries.
// It uses a NonRLSRepository because audit logs are system-wide records not
// subject to per-user or per-tenant Row-Level Security.
type AuditLogRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewAuditLogRegistry creates a new AuditLogRegistry backed by the given database.
func NewAuditLogRegistry(dbx *sqlx.DB) *AuditLogRegistry {
	return NewAuditLogRegistryWithTableNames(dbx, store.DefaultTableNames)
}

// NewAuditLogRegistryWithTableNames creates a new AuditLogRegistry with custom table names.
func NewAuditLogRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *AuditLogRegistry {
	return &AuditLogRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *AuditLogRegistry) newSQLRegistry() *store.NonRLSRepository[models.AuditLog, *models.AuditLog] {
	return store.NewSQLRegistry[models.AuditLog, *models.AuditLog](r.dbx, r.tableNames.AuditLogs())
}

// Create inserts a new audit log entry. The ID is generated automatically.
func (r *AuditLogRegistry) Create(ctx context.Context, entry models.AuditLog) (*models.AuditLog, error) {
	if entry.Action == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Action"))
	}

	entry.Timestamp = time.Now()

	reg := r.newSQLRegistry()
	created, err := reg.Create(ctx, entry, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create audit log entry", err)
	}

	return &created, nil
}

// Get returns a single audit log entry by its ID.
func (r *AuditLogRegistry) Get(ctx context.Context, id string) (*models.AuditLog, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var entry models.AuditLog
	reg := r.newSQLRegistry()
	err := reg.ScanOneByField(ctx, store.Pair("id", id), &entry)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "AuditLog", "entity_id", id))
		}
		return nil, errxtrace.Wrap("failed to get audit log entry", err)
	}

	return &entry, nil
}

// List returns all audit log entries.
func (r *AuditLogRegistry) List(ctx context.Context) ([]*models.AuditLog, error) {
	var entries []*models.AuditLog
	reg := r.newSQLRegistry()

	for entry, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list audit log entries", err)
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

// Update modifies an existing audit log entry (rarely needed; included for interface compliance).
func (r *AuditLogRegistry) Update(ctx context.Context, entry models.AuditLog) (*models.AuditLog, error) {
	if entry.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()
	if err := reg.Update(ctx, entry, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update audit log entry", err)
	}

	return &entry, nil
}

// Delete removes an audit log entry by ID.
func (r *AuditLogRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()
	if err := reg.Delete(ctx, id, nil); err != nil {
		return errxtrace.Wrap("failed to delete audit log entry", err)
	}

	return nil
}

// Count returns the total number of audit log entries.
func (r *AuditLogRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()
	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count audit log entries", err)
	}
	return count, nil
}

// ListByUser returns all audit log entries for the specified user.
func (r *AuditLogRegistry) ListByUser(ctx context.Context, userID string) ([]*models.AuditLog, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	var entries []*models.AuditLog
	reg := r.newSQLRegistry()
	for entry, err := range reg.ScanByField(ctx, store.Pair("user_id", userID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list audit log entries by user", err)
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

// ListByTenant returns all audit log entries for the specified tenant.
func (r *AuditLogRegistry) ListByTenant(ctx context.Context, tenantID string) ([]*models.AuditLog, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	var entries []*models.AuditLog
	reg := r.newSQLRegistry()
	for entry, err := range reg.ScanByField(ctx, store.Pair("tenant_id", tenantID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list audit log entries by tenant", err)
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

// ListByAction returns all audit log entries matching the given action string.
func (r *AuditLogRegistry) ListByAction(ctx context.Context, action string) ([]*models.AuditLog, error) {
	if action == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Action"))
	}

	var entries []*models.AuditLog
	reg := r.newSQLRegistry()
	for entry, err := range reg.ScanByField(ctx, store.Pair("action", action)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list audit log entries by action", err)
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

// DeleteOlderThan removes all audit log entries with a timestamp before cutoff.
func (r *AuditLogRegistry) DeleteOlderThan(ctx context.Context, cutoff time.Time) error {
	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE timestamp < $1`, r.tableNames.AuditLogs())
		_, err := tx.ExecContext(ctx, query, cutoff)
		if err != nil {
			return errxtrace.Wrap("failed to delete old audit log entries", err)
		}
		return nil
	})
}

