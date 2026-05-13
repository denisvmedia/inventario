package postgres

import (
	"context"
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

var _ registry.GroupNotificationPrefRegistry = (*GroupNotificationPrefRegistry)(nil)

// GroupNotificationPrefRegistry persists per-user per-group notification
// opt-outs (issue #1648). Runs in service mode like GroupInviteAuditRegistry
// — the RLS policy on the table covers tenant isolation under
// inventario_app and grants bypass to inventario_background_worker for
// the warranty reminder sweep.
type GroupNotificationPrefRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewGroupNotificationPrefRegistry(dbx *sqlx.DB) *GroupNotificationPrefRegistry {
	return &GroupNotificationPrefRegistry{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
	}
}

func (r *GroupNotificationPrefRegistry) newSQLRegistry() *store.RLSRepository[models.GroupNotificationPref, *models.GroupNotificationPref] {
	return store.NewServiceSQLRegistry[models.GroupNotificationPref, *models.GroupNotificationPref](r.dbx, r.tableNames.GroupNotificationPrefs())
}

func (r *GroupNotificationPrefRegistry) Get(ctx context.Context, id string) (*models.GroupNotificationPref, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	var pref models.GroupNotificationPref
	err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &pref)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "GroupNotificationPref",
				"entity_id", id,
			))
		}
		return nil, errxtrace.Wrap("failed to get group notification pref", err)
	}
	return &pref, nil
}

func (r *GroupNotificationPrefRegistry) List(ctx context.Context) ([]*models.GroupNotificationPref, error) {
	var out []*models.GroupNotificationPref
	for p, err := range r.newSQLRegistry().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list group notification prefs", err)
		}
		out = append(out, &p)
	}
	return out, nil
}

func (r *GroupNotificationPrefRegistry) Count(ctx context.Context) (int, error) {
	count, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count group notification prefs", err)
	}
	return count, nil
}

func (r *GroupNotificationPrefRegistry) Create(ctx context.Context, pref models.GroupNotificationPref) (*models.GroupNotificationPref, error) {
	created, err := r.newSQLRegistry().Create(ctx, pref, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create group notification pref", err)
	}
	return &created, nil
}

func (r *GroupNotificationPrefRegistry) Update(ctx context.Context, pref models.GroupNotificationPref) (*models.GroupNotificationPref, error) {
	if pref.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.newSQLRegistry().Update(ctx, pref, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update group notification pref", err)
	}
	return &pref, nil
}

func (r *GroupNotificationPrefRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.newSQLRegistry().Delete(ctx, id, nil); err != nil {
		return errxtrace.Wrap("failed to delete group notification pref", err)
	}
	return nil
}

func (r *GroupNotificationPrefRegistry) ListByUserGroup(ctx context.Context, tenantID, groupID, userID string) ([]*models.GroupNotificationPref, error) {
	if tenantID == "" || groupID == "" || userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "tenant_id|group_id|user_id"))
	}
	var out []*models.GroupNotificationPref
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT id, uuid, tenant_id, group_id, user_id, category, enabled, created_at, updated_at
			 FROM %s
			 WHERE tenant_id = $1 AND group_id = $2 AND user_id = $3`,
			r.tableNames.GroupNotificationPrefs(),
		)
		rows, qerr := tx.QueryxContext(ctx, query, tenantID, groupID, userID)
		if qerr != nil {
			return qerr
		}
		defer rows.Close()
		for rows.Next() {
			var p models.GroupNotificationPref
			if scanErr := rows.StructScan(&p); scanErr != nil {
				return scanErr
			}
			out = append(out, &p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list group notification prefs by user/group", err)
	}
	return out, nil
}

// Upsert relies on the unique index on (tenant_id, group_id, user_id,
// category) for the conflict target. On conflict the existing row's
// `enabled` + `updated_at` get rewritten; otherwise a fresh row is
// inserted with a new id/uuid. The returned row is the post-write
// state so callers can echo it back without a follow-up SELECT.
func (r *GroupNotificationPrefRegistry) Upsert(ctx context.Context, pref models.GroupNotificationPref) (*models.GroupNotificationPref, error) {
	if pref.TenantID == "" || pref.GroupID == "" || pref.UserID == "" || pref.Category == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "tenant_id|group_id|user_id|category"))
	}
	if pref.GetID() == "" {
		pref.SetID(uuid.NewString())
	}
	now := time.Now().UTC()
	if pref.CreatedAt.IsZero() {
		pref.CreatedAt = now
	}
	pref.UpdatedAt = now

	var written models.GroupNotificationPref
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`INSERT INTO %s (id, uuid, tenant_id, group_id, user_id, category, enabled, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 ON CONFLICT (tenant_id, group_id, user_id, category)
			 DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = EXCLUDED.updated_at
			 RETURNING id, uuid, tenant_id, group_id, user_id, category, enabled, created_at, updated_at`,
			r.tableNames.GroupNotificationPrefs(),
		)
		row := tx.QueryRowxContext(ctx, query,
			pref.GetID(),
			pref.GetID(), // uuid mirrors id for new rows; ignored on conflict
			pref.TenantID,
			pref.GroupID,
			pref.UserID,
			pref.Category,
			pref.Enabled,
			pref.CreatedAt.UTC(),
			pref.UpdatedAt,
		)
		return row.StructScan(&written)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to upsert group notification pref", err)
	}
	return &written, nil
}
