package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.TenantRegistry = (*TenantRegistry)(nil)

type TenantRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewTenantRegistry(dbx *sqlx.DB) *TenantRegistry {
	return NewTenantRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewTenantRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *TenantRegistry {
	return &TenantRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *TenantRegistry) newSQLRegistry() *store.NonRLSRepository[models.Tenant, *models.Tenant] {
	return store.NewSQLRegistry[models.Tenant, *models.Tenant](r.dbx, r.tableNames.Tenants())
}

func (r *TenantRegistry) Get(ctx context.Context, id string) (*models.Tenant, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var tenant models.Tenant
	reg := r.newSQLRegistry()
	err := reg.ScanOneByField(ctx, store.Pair("id", id), &tenant)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "Tenant",
				"entity_id", id,
			))
		}
		return nil, errxtrace.Wrap("failed to get entity", err)
	}

	return &tenant, nil
}

func (r *TenantRegistry) List(ctx context.Context) ([]*models.Tenant, error) {
	var tenants []*models.Tenant

	reg := r.newSQLRegistry()

	// Query the database for all tenants (atomic operation)
	for tenant, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list tenants", err)
		}
		tenants = append(tenants, &tenant)
	}

	return tenants, nil
}

func (r *TenantRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count tenants", err)
	}

	return count, nil
}

func (r *TenantRegistry) Create(ctx context.Context, tenant models.Tenant) (*models.Tenant, error) {
	if tenant.Name == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}

	if tenant.Slug == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Slug"))
	}

	// Default the registration mode to closed so INSERTs stay aligned with the
	// fail-closed schema default and never persist an empty/unknown mode.
	if tenant.RegistrationMode == "" {
		tenant.RegistrationMode = models.RegistrationModeClosed
	}

	// ID is now set automatically by NonRLSRepository.Create

	reg := r.newSQLRegistry()

	createdTenant, err := reg.Create(ctx, tenant, func(ctx context.Context, tx *sqlx.Tx) error {
		// Check if a tenant with the same slug already exists
		var existingTenant models.Tenant
		txReg := store.NewTxRegistry[models.Tenant](tx, r.tableNames.Tenants())
		err := txReg.ScanOneByField(ctx, store.Pair("slug", tenant.Slug), &existingTenant)
		if err == nil {
			return errxtrace.Classify(registry.ErrSlugAlreadyExists, errx.Attrs("slug", tenant.Slug))
		} else if !errors.Is(err, store.ErrNotFound) {
			return errxtrace.Wrap("failed to check for existing tenant", err)
		}
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to create tenant", err)
	}

	return &createdTenant, nil
}

func (r *TenantRegistry) Update(ctx context.Context, tenant models.Tenant) (*models.Tenant, error) {
	if tenant.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	if tenant.Name == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}

	if tenant.Slug == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Slug"))
	}

	if tenant.RegistrationMode == "" {
		tenant.RegistrationMode = models.RegistrationModeClosed
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, tenant, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update tenant", err)
	}

	return &tenant, nil
}

func (r *TenantRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errxtrace.Wrap("failed to delete tenant", err)
	}

	return nil
}

// GetDefault returns the tenant marked as the system default (is_default = true).
func (r *TenantRegistry) GetDefault(ctx context.Context) (*models.Tenant, error) {
	var tenant models.Tenant
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("is_default", true), &tenant)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "Tenant"))
		}
		return nil, errxtrace.Wrap("failed to get default tenant", err)
	}

	return &tenant, nil
}

// GetBySlug returns a tenant by its slug
func (r *TenantRegistry) GetBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	if slug == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Slug"))
	}

	var tenant models.Tenant
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("slug", slug), &tenant)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "Tenant",
				"slug", slug,
			))
		}
		return nil, errxtrace.Wrap("failed to get tenant by slug", err)
	}

	return &tenant, nil
}

// ListAdmin returns paginated, filtered, sorted tenants for the
// `/api/v1/admin/tenants` listing (#1746) along with per-row computed
// user_count and group_count.
//
// The `tenants` table has no RLS enabled (it IS the tenant boundary), so
// the cross-tenant read works through the existing NonRLSRepository
// without a role switch — the correlated COUNT subqueries on the
// RLS-enabled `users` / `location_groups` tables rely on the connection
// role's bypass (table-owner or BYPASSRLS attribute) to produce the
// cross-tenant counts the admin UI needs.
//
// `SET LOCAL row_security = off` on the tx is a fail-loud guard: per
// the postgres semantics, queries against an RLS-protected table by a
// non-bypass role with row_security=off ERROR out rather than silently
// filtering. If the connection role's bypass is ever revoked this
// endpoint will start 5xx-ing loudly instead of returning honest-looking
// "0" counts that mask a misconfiguration. That matches the issue
// spec's "Endpoints bypass RLS via SET LOCAL row_security = off inside
// the handler's tx" — the bypass narrative is "fail loud on
// misconfiguration", not "produce rows the role couldn't otherwise see".
//
// Two queries are issued under the same tx so total + page rows stay
// consistent with one another. The COUNT/page split exists because
// applying the same correlated subqueries inside the count query is
// wasteful — `SELECT count(*)` on the filter-only predicate is enough.
func (r *TenantRegistry) ListAdmin(ctx context.Context, opts registry.AdminTenantListOptions) ([]*registry.AdminTenantListItem, int, error) {
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
		sortField = registry.AdminTenantSortName
	}
	direction := "ASC"
	if opts.SortDesc {
		direction = "DESC"
	}

	tenantsTable := r.tableNames.Tenants()
	usersTable := r.tableNames.Users()
	groupsTable := r.tableNames.LocationGroups()

	// Build optional WHERE clause for the search query.
	args := make([]any, 0, 2)
	where := ""
	if q := strings.TrimSpace(opts.Query); q != "" {
		args = append(args, "%"+q+"%")
		// $1 reused across three columns — keeps the binding simple.
		where = "WHERE (t.name ILIKE $1 OR t.slug ILIKE $1 OR t.domain ILIKE $1)"
	}

	var (
		items []*registry.AdminTenantListItem
		total int
	)
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// SET LOCAL row_security = off scopes the override to this tx.
		// Postgres semantics: with row_security=off, any non-bypass role
		// querying an RLS-protected table ERRORs instead of silently
		// filtering. That's the fail-loud guard — if this connection
		// role ever loses its bypass we want a 5xx, not a quietly empty
		// page. See the function godoc for the full reasoning.
		if _, execErr := tx.ExecContext(ctx, "SET LOCAL row_security = off"); execErr != nil {
			return errxtrace.Wrap("failed to disable row_security for admin tenant listing", execErr)
		}

		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s AS t %s", tenantsTable, where)
		if err := tx.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
			return errxtrace.Wrap("failed to count admin tenants", err)
		}

		// LIMIT/OFFSET use positional bindings that come after the
		// optional query arg, so derive their positions dynamically.
		limitPos := len(args) + 1
		offsetPos := len(args) + 2
		offset := (page - 1) * perPage

		// SECURITY: sortField is constrained to AdminTenantSortField via IsValid above,
		// direction is "ASC"/"DESC" literals, and table-names come from r.tableNames —
		// never user-supplied — so direct fmt.Sprintf interpolation is safe.
		pageQuery := fmt.Sprintf(`
			SELECT t.*,
				(SELECT COUNT(*) FROM %s AS u WHERE u.tenant_id = t.id) AS _user_count,
				(SELECT COUNT(*) FROM %s AS g WHERE g.tenant_id = t.id) AS _group_count
			FROM %s AS t
			%s
			ORDER BY t.%s %s, t.id ASC
			LIMIT $%d OFFSET $%d`,
			usersTable, groupsTable, tenantsTable, where, string(sortField), direction, limitPos, offsetPos,
		)
		pageArgs := append(append([]any{}, args...), perPage, offset)

		rows, err := tx.QueryxContext(ctx, pageQuery, pageArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to list admin tenants", err)
		}
		defer rows.Close()

		for rows.Next() {
			// Scan into a wide struct that embeds Tenant plus the
			// two correlated counts. Keeping the counts on the same
			// row means the page query stays a single round-trip.
			var row struct {
				models.Tenant
				UserCount  int `db:"_user_count"`
				GroupCount int `db:"_group_count"`
			}
			if scanErr := rows.StructScan(&row); scanErr != nil {
				return errxtrace.Wrap("failed to scan admin tenant row", scanErr)
			}
			tenant := row.Tenant
			items = append(items, &registry.AdminTenantListItem{
				Tenant:     &tenant,
				UserCount:  row.UserCount,
				GroupCount: row.GroupCount,
			})
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			return errxtrace.Wrap("failed during admin tenant row iteration", rowsErr)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetAdmin returns a single tenant detail row with the same computed
// counts the listing surfaces. Runs one tx with row_security=off plus
// COUNT subqueries instead of materialising the full user / group
// row sets — keeps the detail endpoint O(constant) regardless of
// tenant size.
func (r *TenantRegistry) GetAdmin(ctx context.Context, tenantID string) (*registry.AdminTenantListItem, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	tenantsTable := r.tableNames.Tenants()
	usersTable := r.tableNames.Users()
	groupsTable := r.tableNames.LocationGroups()

	var (
		tenant     models.Tenant
		userCount  int
		groupCount int
		found      bool
	)
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if _, execErr := tx.ExecContext(ctx, "SET LOCAL row_security = off"); execErr != nil {
			return errxtrace.Wrap("failed to disable row_security for admin tenant detail", execErr)
		}
		query := fmt.Sprintf(`
			SELECT t.*,
				(SELECT COUNT(*) FROM %s AS u WHERE u.tenant_id = t.id) AS _user_count,
				(SELECT COUNT(*) FROM %s AS g WHERE g.tenant_id = t.id) AS _group_count
			FROM %s AS t
			WHERE t.id = $1`,
			usersTable, groupsTable, tenantsTable,
		)
		var row struct {
			models.Tenant
			UserCount  int `db:"_user_count"`
			GroupCount int `db:"_group_count"`
		}
		scanErr := tx.QueryRowxContext(ctx, query, tenantID).StructScan(&row)
		if scanErr != nil {
			if errors.Is(scanErr, sql.ErrNoRows) {
				return nil
			}
			return errxtrace.Wrap("failed to load admin tenant detail", scanErr)
		}
		tenant = row.Tenant
		userCount = row.UserCount
		groupCount = row.GroupCount
		found = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
			"entity_type", "Tenant",
			"entity_id", tenantID,
		))
	}
	return &registry.AdminTenantListItem{
		Tenant:     &tenant,
		UserCount:  userCount,
		GroupCount: groupCount,
	}, nil
}

// GetByDomain returns a tenant by its domain
func (r *TenantRegistry) GetByDomain(ctx context.Context, domain string) (*models.Tenant, error) {
	if domain == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Domain"))
	}

	var tenant models.Tenant
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("domain", domain), &tenant)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "Tenant",
				"domain", domain,
			))
		}
		return nil, errxtrace.Wrap("failed to get tenant by domain", err)
	}

	return &tenant, nil
}
