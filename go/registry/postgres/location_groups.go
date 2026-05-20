package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.LocationGroupRegistry = (*LocationGroupRegistry)(nil)

type LocationGroupRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewLocationGroupRegistry(dbx *sqlx.DB) *LocationGroupRegistry {
	return &LocationGroupRegistry{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
	}
}

// newSQLRegistry returns an RLSRepository in service mode. location_groups has
// RLS enabled with a tenant-isolation policy on inventario_app, so running via
// the background-worker role (which has a bypass policy) is how we reach this
// tenant-scoped table from flows that manage tenant scoping in application
// code — the GroupService layer passes tenant_id explicitly to GetBySlug /
// ListByTenant / the slug-uniqueness check. Same pattern as RefreshTokenRegistry.
func (r *LocationGroupRegistry) newSQLRegistry() *store.RLSRepository[models.LocationGroup, *models.LocationGroup] {
	return store.NewServiceSQLRegistry[models.LocationGroup, *models.LocationGroup](r.dbx, r.tableNames.LocationGroups())
}

func (r *LocationGroupRegistry) Get(ctx context.Context, id string) (*models.LocationGroup, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var group models.LocationGroup
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &group)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "LocationGroup",
				"entity_id", id,
			))
		}
		return nil, errxtrace.Wrap("failed to get location group", err)
	}

	return &group, nil
}

func (r *LocationGroupRegistry) List(ctx context.Context) ([]*models.LocationGroup, error) {
	var groups []*models.LocationGroup

	reg := r.newSQLRegistry()

	for group, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list location groups", err)
		}
		groups = append(groups, &group)
	}

	return groups, nil
}

func (r *LocationGroupRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count location groups", err)
	}

	return count, nil
}

func (r *LocationGroupRegistry) Create(ctx context.Context, group models.LocationGroup) (*models.LocationGroup, error) {
	if group.Name == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}

	if group.Slug == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Slug"))
	}

	if group.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	// CreatedBy and UserID are NOT NULL columns (schema-level FK to users.id).
	// Without this check, a caller constructing a LocationGroup without
	// setting CreatedBy would silently insert an empty string and blow up
	// at the FK violation rather than here with a meaningful field_name.
	if group.CreatedBy == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "CreatedBy"))
	}

	reg := r.newSQLRegistry()

	createdGroup, err := reg.Create(ctx, group, func(ctx context.Context, tx *sqlx.Tx) error {
		// Scope the slug-uniqueness lookup to the (tenant_id, slug) unique
		// index directly instead of scanning every row that happens to match
		// the slug — much cheaper on a tenant with many groups.
		var existing models.LocationGroup
		txReg := store.NewTxRegistry[models.LocationGroup](tx, r.tableNames.LocationGroups())
		err := txReg.ScanOneByFields(ctx, []store.FieldValue{
			store.Pair("tenant_id", group.TenantID),
			store.Pair("slug", group.Slug),
		}, &existing)
		if err == nil {
			return errxtrace.Classify(registry.ErrSlugAlreadyExists, errx.Attrs("slug", group.Slug))
		}
		if !errors.Is(err, store.ErrNotFound) {
			return errxtrace.Wrap("failed to check for existing group", err)
		}
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to create location group", err)
	}

	return &createdGroup, nil
}

func (r *LocationGroupRegistry) Update(ctx context.Context, group models.LocationGroup) (*models.LocationGroup, error) {
	if group.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	if group.Name == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, group, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update location group", err)
	}

	return &group, nil
}

func (r *LocationGroupRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errxtrace.Wrap("failed to delete location group", err)
	}

	return nil
}

func (r *LocationGroupRegistry) GetBySlug(ctx context.Context, tenantID, slug string) (*models.LocationGroup, error) {
	if slug == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Slug"))
	}

	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	reg := r.newSQLRegistry()

	for group, err := range reg.ScanByField(ctx, store.Pair("slug", slug)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to get location group by slug", err)
		}
		if group.TenantID == tenantID {
			return &group, nil
		}
	}

	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
		"entity_type", "LocationGroup",
		"slug", slug,
	))
}

func (r *LocationGroupRegistry) ListByTenant(ctx context.Context, tenantID string) ([]*models.LocationGroup, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	var groups []*models.LocationGroup
	reg := r.newSQLRegistry()

	for group, err := range reg.ScanByField(ctx, store.Pair("tenant_id", tenantID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list location groups by tenant", err)
		}
		groups = append(groups, &group)
	}

	return groups, nil
}

// ListAdmin returns paginated, filtered, sorted location groups for the
// `/api/v1/admin/groups` listing (#1748) along with the per-row computed
// member_count from a correlated subquery on group_memberships.
//
// The endpoint crosses tenants by design — a system admin lists every
// tenant's groups. This registry runs in service mode as the
// inventario_background_worker role, which already carries `using=true`
// bypass policies on location_groups / group_memberships, so the
// cross-tenant read works regardless of RLS. The `SET LOCAL row_security =
// off` on the tx is defense-in-depth: it makes the intentional cross-tenant
// read explicit at the call site and, on the bypass role, is effectively a
// no-op rather than the primary guard. Same rationale as
// TenantRegistry.ListAdmin / UserRegistry.ListAdminByTenant.
//
// Total is post-filter, pre-pagination.
func (r *LocationGroupRegistry) ListAdmin(ctx context.Context, opts registry.AdminGroupListOptions) ([]*registry.AdminGroupListItem, int, error) {
	page := opts.Page
	if page <= 0 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage <= 0 {
		perPage = 50
	}

	sortField := opts.SortField
	if !sortField.IsValid() {
		sortField = registry.AdminGroupSortName
	}
	direction := "ASC"
	if opts.SortDesc {
		direction = "DESC"
	}

	groupsTable := r.tableNames.LocationGroups()
	membershipsTable := r.tableNames.GroupMemberships()

	args := make([]any, 0, 3)
	whereClauses := make([]string, 0, 3)
	if q := strings.TrimSpace(opts.Query); q != "" {
		args = append(args, "%"+q+"%")
		// The query arg is reused across name + slug.
		whereClauses = append(whereClauses, fmt.Sprintf("(g.name ILIKE $%d OR g.slug ILIKE $%d)", len(args), len(args)))
	}
	if t := strings.TrimSpace(opts.TenantID); t != "" {
		args = append(args, t)
		whereClauses = append(whereClauses, fmt.Sprintf("g.tenant_id = $%d", len(args)))
	}
	if s := strings.TrimSpace(opts.Status); s != "" {
		args = append(args, s)
		whereClauses = append(whereClauses, fmt.Sprintf("g.status = $%d", len(args)))
	}
	where := ""
	if len(whereClauses) > 0 {
		where = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	var (
		items []*registry.AdminGroupListItem
		total int
	)
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if _, execErr := tx.ExecContext(ctx, "SET LOCAL row_security = off"); execErr != nil {
			return errxtrace.Wrap("failed to disable row_security for admin group listing", execErr)
		}

		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s AS g %s", groupsTable, where)
		if scanErr := tx.QueryRowContext(ctx, countQuery, args...).Scan(&total); scanErr != nil {
			return errxtrace.Wrap("failed to count admin groups", scanErr)
		}

		limitPos := len(args) + 1
		offsetPos := len(args) + 2
		offset := (page - 1) * perPage

		// SECURITY: sortField is constrained to AdminGroupSortField via IsValid above,
		// direction is "ASC"/"DESC" literals, and table-names come from r.tableNames —
		// never user-supplied — so direct fmt.Sprintf interpolation is safe.
		pageQuery := fmt.Sprintf(`
			SELECT g.*,
				(SELECT COUNT(*) FROM %s AS m WHERE m.group_id = g.id) AS _member_count
			FROM %s AS g
			%s
			ORDER BY g.%s %s, g.id ASC
			LIMIT $%d OFFSET $%d`,
			membershipsTable, groupsTable, where, string(sortField), direction, limitPos, offsetPos,
		)
		pageArgs := append(append([]any{}, args...), perPage, offset)

		rows, err := tx.QueryxContext(ctx, pageQuery, pageArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to list admin groups", err)
		}
		defer rows.Close()

		for rows.Next() {
			var row struct {
				models.LocationGroup
				MemberCount int `db:"_member_count"`
			}
			if scanErr := rows.StructScan(&row); scanErr != nil {
				return errxtrace.Wrap("failed to scan admin group row", scanErr)
			}
			group := row.LocationGroup
			items = append(items, &registry.AdminGroupListItem{
				Group:       &group,
				MemberCount: row.MemberCount,
			})
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			return errxtrace.Wrap("failed during admin group row iteration", rowsErr)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetAdmin returns a single group detail row with the computed
// member_count plus the owning tenant (joined so the detail handler can
// render the tenant chip in one round-trip). Runs under
// `SET LOCAL row_security = off` for the same defense-in-depth
// cross-tenant rationale as ListAdmin.
func (r *LocationGroupRegistry) GetAdmin(ctx context.Context, groupID string) (*registry.AdminGroupDetail, error) {
	if groupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	var detail *registry.AdminGroupDetail
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if _, execErr := tx.ExecContext(ctx, "SET LOCAL row_security = off"); execErr != nil {
			return errxtrace.Wrap("failed to disable row_security for admin group detail", execErr)
		}
		var loadErr error
		detail, loadErr = r.loadAdminGroupDetailTx(ctx, tx, groupID)
		return loadErr
	})
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
			"entity_type", "LocationGroup",
			"entity_id", groupID,
		))
	}
	return detail, nil
}

// loadAdminGroupDetailTx loads the admin group detail row (group +
// member_count + tenant chip) within the supplied tx. It is the shared
// detail-shaping query used by both GetAdmin and MarkPendingDeletionAdmin
// so the post-delete row carries exactly the shape the detail handler
// renders without a second round-trip. Returns (nil, nil) when the group
// id doesn't exist — callers map that to ErrNotFound. The caller is
// responsible for the `SET LOCAL row_security = off` on the tx.
func (r *LocationGroupRegistry) loadAdminGroupDetailTx(ctx context.Context, tx *sqlx.Tx, groupID string) (*registry.AdminGroupDetail, error) {
	groupsTable := r.tableNames.LocationGroups()
	membershipsTable := r.tableNames.GroupMemberships()
	tenantsTable := r.tableNames.Tenants()

	query := fmt.Sprintf(`
		SELECT g.*,
			(SELECT COUNT(*) FROM %s AS m WHERE m.group_id = g.id) AS _member_count,
			t.id AS _tenant_id, t.name AS _tenant_name, t.slug AS _tenant_slug
		FROM %s AS g
		JOIN %s AS t ON t.id = g.tenant_id
		WHERE g.id = $1`,
		membershipsTable, groupsTable, tenantsTable,
	)
	var row struct {
		models.LocationGroup
		MemberCount int    `db:"_member_count"`
		TenantID    string `db:"_tenant_id"`
		TenantName  string `db:"_tenant_name"`
		TenantSlug  string `db:"_tenant_slug"`
	}
	scanErr := tx.QueryRowxContext(ctx, query, groupID).StructScan(&row)
	if scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errxtrace.Wrap("failed to load admin group detail", scanErr)
	}
	group := row.LocationGroup
	return &registry.AdminGroupDetail{
		Group:       &group,
		MemberCount: row.MemberCount,
		Tenant: &models.Tenant{
			EntityID: models.EntityID{ID: row.TenantID},
			Name:     row.TenantName,
			Slug:     row.TenantSlug,
		},
	}, nil
}

// MarkPendingDeletionAdmin flips a group to pending_deletion for the
// cross-tenant admin soft-delete (#1748). The status-transition logic is
// identical to GroupService.InitiateGroupDeletion — Status set to
// pending_deletion and UpdatedAt bumped — so the existing
// group_purge_worker finishes the hard-delete with no parallel code path.
//
// The whole read-decide-write-reload runs inside one background-worker tx
// so two concurrent admin deletes can't both observe `active` and race,
// and the returned detail row is the committed post-transition state. The
// in-tx reload (instead of a follow-up GetAdmin) closes a TOCTOU window:
// between the soft-delete commit and a separate re-fetch the
// group_purge_worker could hard-delete the now-pending row, turning a
// successful DELETE into a spurious 404. Idempotent: an already-pending
// group returns (detail, true, nil) without re-writing the row.
func (r *LocationGroupRegistry) MarkPendingDeletionAdmin(ctx context.Context, groupID string) (*registry.AdminGroupDetail, bool, error) {
	if groupID == "" {
		return nil, false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	groupsTable := r.tableNames.LocationGroups()

	var (
		alreadyPending bool
		detail         *registry.AdminGroupDetail
	)
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		var currentStatus string
		selectQuery := fmt.Sprintf("SELECT status FROM %s WHERE id = $1 FOR UPDATE", groupsTable)
		scanErr := tx.QueryRowContext(ctx, selectQuery, groupID).Scan(&currentStatus)
		if scanErr != nil {
			if errors.Is(scanErr, sql.ErrNoRows) {
				return nil
			}
			return errxtrace.Wrap("failed to load location group for soft-delete", scanErr)
		}
		if currentStatus == string(models.LocationGroupStatusPendingDeletion) {
			alreadyPending = true
		} else {
			updateQuery := fmt.Sprintf("UPDATE %s SET status = $1, updated_at = $2 WHERE id = $3", groupsTable)
			if _, execErr := tx.ExecContext(ctx, updateQuery,
				string(models.LocationGroupStatusPendingDeletion), time.Now(), groupID); execErr != nil {
				return errxtrace.Wrap("failed to mark location group pending_deletion", execErr)
			}
		}
		// Reload the post-transition detail row in the SAME tx — the row is
		// still pinned by the FOR UPDATE lock above, so this reflects the
		// committed state and cannot race the purge worker.
		var loadErr error
		detail, loadErr = r.loadAdminGroupDetailTx(ctx, tx, groupID)
		return loadErr
	})
	if err != nil {
		return nil, false, err
	}
	if detail == nil {
		return nil, false, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
			"entity_type", "LocationGroup",
			"entity_id", groupID,
		))
	}
	return detail, alreadyPending, nil
}
