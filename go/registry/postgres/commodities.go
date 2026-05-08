package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// CommodityRegistryFactory creates CommodityRegistry instances with proper context
type CommodityRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// CommodityRegistry is a context-aware registry that can only be created through the factory
type CommodityRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var _ registry.CommodityRegistry = (*CommodityRegistry)(nil)
var _ registry.CommodityRegistryFactory = (*CommodityRegistryFactory)(nil)

func NewCommodityRegistry(dbx *sqlx.DB) *CommodityRegistryFactory {
	return NewCommodityRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewCommodityRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *CommodityRegistryFactory {
	return &CommodityRegistryFactory{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// Factory methods implementing registry.CommodityRegistryFactory

func (f *CommodityRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CommodityRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user ID from context", err)
	}

	return &CommodityRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
		service:         false,
	}, nil
}

func (f *CommodityRegistryFactory) CreateServiceRegistry() registry.CommodityRegistry {
	return &CommodityRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *CommodityRegistry) Get(ctx context.Context, id string) (*models.Commodity, error) {
	slog.Debug("Getting commodity", "commodity_id", id, "created_by_user_id", r.createdByUserID, "tenant_id", r.tenantID, "service_mode", r.service)
	return r.get(ctx, id)
}

func (r *CommodityRegistry) Create(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	// ID, TenantID, and UserID are now set automatically by RLSRepository.Create

	// Acquisition columns are server-managed (issue #1550 / #202): drop
	// any value the API caller smuggled in. The migration worker is the
	// only legitimate writer (via go/registry/internal/migrationops),
	// and it only writes when the row's columns are still NULL.
	commodity.AcquisitionPrice = nil
	commodity.AcquisitionCurrency = nil

	reg := r.newSQLRegistry()

	createdCommodity, err := reg.Create(ctx, commodity, func(ctx context.Context, tx *sqlx.Tx) error {
		if _, err := r.getArea(ctx, tx, commodity.AreaID); err != nil {
			return err
		}
		// Auto-create / row-lock referenced tag rows inside this same tx
		// so a concurrent DeleteTag(force=true) on one of these slugs
		// can't leave us with an orphan JSONB reference at commit. See
		// ensureTagRowsInTx in tags.go for the full lock invariant.
		return ensureTagRowsInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID, r.createdByUserID, []string(commodity.Tags))
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity", err)
	}

	return &createdCommodity, nil
}

func (r *CommodityRegistry) GetByName(ctx context.Context, name string) (*models.Commodity, error) {
	var commodity models.Commodity
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("name", name), &commodity)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get commodity", err)
	}

	return &commodity, nil
}

func (r *CommodityRegistry) List(ctx context.Context) ([]*models.Commodity, error) {
	var commodities []*models.Commodity

	reg := r.newSQLRegistry()

	// Query the database for all commodities ordered by purchase date descending (most recent first).
	// NULL purchase dates are sorted last.
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf("SELECT * FROM %s ORDER BY purchase_date DESC NULLS LAST", r.tableNames.Commodities())
		rows, err := tx.QueryxContext(ctx, query)
		if err != nil {
			return errxtrace.Wrap("failed to list commodities", err)
		}
		defer rows.Close()

		for rows.Next() {
			var commodity models.Commodity
			if err := rows.StructScan(&commodity); err != nil {
				return errxtrace.Wrap("failed to scan commodity", err)
			}
			commodities = append(commodities, &commodity)
		}

		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodities", err)
	}

	return commodities, nil
}

// ListByGroup returns every commodity in (tenant_id, group_id), regardless
// of draft / status. The currency-migration service (#202) needs the full
// row set — ListPaginated's default filters would otherwise hide drafts and
// archived rows from the conversion. Service-mode callers (the worker)
// pass tenantID + groupID explicitly because they bypass RLS; user-mode
// callers should still pass the same values they were created with so the
// query is executed under the same RLS view they already see.
func (r *CommodityRegistry) ListByGroup(ctx context.Context, tenantID, groupID string) ([]*models.Commodity, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if groupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	var commodities []*models.Commodity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE tenant_id = $1 AND group_id = $2 ORDER BY id ASC`,
			r.tableNames.Commodities(),
		)
		rows, qerr := tx.QueryxContext(ctx, query, tenantID, groupID)
		if qerr != nil {
			return errxtrace.Wrap("failed to list commodities by group", qerr)
		}
		defer rows.Close()
		for rows.Next() {
			var commodity models.Commodity
			if scanErr := rows.StructScan(&commodity); scanErr != nil {
				return errxtrace.Wrap("failed to scan commodity", scanErr)
			}
			commodities = append(commodities, &commodity)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodities by group", err)
	}
	return commodities, nil
}

func (r *CommodityRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count commodities", err)
	}

	return cnt, nil
}

// ListPaginated returns a paginated list of commodities along with the total count.
// The opts parameter narrows the result via dynamic WHERE/ORDER BY clauses;
// see registry.CommodityListOptions for the field-by-field semantics. The
// total reflects the filtered count (post-WHERE, pre-LIMIT).
func (r *CommodityRegistry) ListPaginated(ctx context.Context, offset, limit int, opts registry.CommodityListOptions) ([]*models.Commodity, int, error) {
	// Normalize pagination parameters to prevent negative SQL OFFSET/LIMIT errors.
	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}

	whereClause, whereArgs := buildCommodityWhere(opts)
	orderClause := buildCommodityOrder(opts)

	var commodities []*models.Commodity
	var total int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, r.tableNames.Commodities(), whereClause)
		if err := tx.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
			return errxtrace.Wrap("failed to count commodities", err)
		}

		dataArgs := append([]any{}, whereArgs...)
		dataArgs = append(dataArgs, limit, offset)
		dataQuery := fmt.Sprintf(`
			SELECT * FROM %s
			%s
			%s
			LIMIT $%d OFFSET $%d`,
			r.tableNames.Commodities(),
			whereClause,
			orderClause,
			len(whereArgs)+1, len(whereArgs)+2,
		)

		rows, err := tx.QueryxContext(ctx, dataQuery, dataArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to list paginated commodities", err)
		}
		defer rows.Close()

		for rows.Next() {
			var commodity models.Commodity
			if err := rows.StructScan(&commodity); err != nil {
				return errxtrace.Wrap("failed to scan commodity", err)
			}
			commodities = append(commodities, &commodity)
		}

		return rows.Err()
	})
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list paginated commodities", err)
	}

	return commodities, total, nil
}

// buildCommodityWhere assembles the WHERE clause + args for filtered list
// queries. Returns ("", nil) when opts is the zero value, so the caller's
// SQL stays identical to the pre-filtering era (avoiding a regression in
// query plans for the common "no filter" path).
func buildCommodityWhere(opts registry.CommodityListOptions) (string, []any) {
	var conds []string
	var args []any
	idx := 1

	// Default view: hide drafts unless caller asked to see them.
	if !opts.IncludeInactive {
		conds = append(conds, fmt.Sprintf("draft = $%d", idx))
		args = append(args, false)
		idx++
		// Implicit status='in_use' applies only when the caller hasn't
		// chosen specific statuses — see the equivalent comment in the
		// memory implementation for the full rationale.
		if len(opts.Statuses) == 0 {
			conds = append(conds, fmt.Sprintf("status = $%d", idx))
			args = append(args, string(models.CommodityStatusInUse))
			idx++
		}
	}

	if len(opts.Types) > 0 {
		placeholders := make([]string, len(opts.Types))
		for i, t := range opts.Types {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, string(t))
			idx++
		}
		conds = append(conds, fmt.Sprintf("type IN (%s)", strings.Join(placeholders, ", ")))
	}
	if len(opts.Statuses) > 0 {
		placeholders := make([]string, len(opts.Statuses))
		for i, s := range opts.Statuses {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, string(s))
			idx++
		}
		conds = append(conds, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ", ")))
	}
	if opts.AreaID != "" {
		conds = append(conds, fmt.Sprintf("area_id = $%d", idx))
		args = append(args, opts.AreaID)
		idx++
	}
	if q := strings.TrimSpace(opts.Search); q != "" {
		// LOWER() + LIKE rather than ILIKE so the existing functional
		// index (commodities_name_lower_idx) is hit. ILIKE bypasses
		// the index on Postgres < 14 because the planner can't see
		// the case-folded form.
		conds = append(conds, fmt.Sprintf("(LOWER(name) LIKE $%d OR LOWER(short_name) LIKE $%d)", idx, idx))
		args = append(args, "%"+strings.ToLower(q)+"%")
		idx++
	}

	if len(opts.WarrantyStatuses) > 0 {
		now := opts.WarrantyNow
		if now.IsZero() {
			now = time.Now()
		}
		today := now.UTC().Format("2006-01-02")
		cutoff := now.UTC().AddDate(0, 0, models.WarrantyExpiringWindowDays).Format("2006-01-02")
		// Each status maps to a closed-form predicate on warranty_expires_at.
		// Multiple statuses are OR-ed together, then the whole disjunction
		// joins the surrounding WHERE with AND. Date strings are zero-padded
		// ISO so lexicographic comparison matches chronological order.
		//
		// Every non-`none` predicate guards against `warranty_expires_at = ''`
		// alongside the NULL check — empty strings reach the column via the
		// PDate zero value and would otherwise satisfy `expired`'s `<` test
		// (because '' is lexicographically less than any ISO date), which
		// would surface "no warranty" rows under the Expired tab.
		var disj []string
		for _, s := range opts.WarrantyStatuses {
			switch s {
			case registry.WarrantyStatusFilterNone:
				disj = append(disj, "(warranty_expires_at IS NULL OR warranty_expires_at = '')")
			case registry.WarrantyStatusFilterExpired:
				disj = append(disj, fmt.Sprintf("(warranty_expires_at IS NOT NULL AND warranty_expires_at <> '' AND warranty_expires_at < $%d)", idx))
				args = append(args, today)
				idx++
			case registry.WarrantyStatusFilterExpiring:
				disj = append(disj, fmt.Sprintf("(warranty_expires_at <> '' AND warranty_expires_at >= $%d AND warranty_expires_at <= $%d)", idx, idx+1))
				args = append(args, today, cutoff)
				idx += 2
			case registry.WarrantyStatusFilterActive:
				disj = append(disj, fmt.Sprintf("(warranty_expires_at <> '' AND warranty_expires_at > $%d)", idx))
				args = append(args, cutoff)
				idx++
			}
		}
		if len(disj) > 0 {
			conds = append(conds, "("+strings.Join(disj, " OR ")+")")
		}
	}
	if opts.WarrantyExpiresBefore != "" {
		// Same empty-string defense as the warranty-status branches —
		// '' would lexicographically match `< cutoff` and pollute the
		// result with "no warranty" rows.
		conds = append(conds, fmt.Sprintf("(warranty_expires_at IS NOT NULL AND warranty_expires_at <> '' AND warranty_expires_at < $%d)", idx))
		args = append(args, opts.WarrantyExpiresBefore)
		// idx is the last branch; the trailing increment would be ineffassign-flagged.
	}

	if len(conds) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

// buildCommodityOrder maps SortField to a SQL ORDER BY clause. The id tie
// breaker keeps page boundaries stable when the primary key has duplicate
// sort-field values (e.g. several commodities with the same name).
func buildCommodityOrder(opts registry.CommodityListOptions) string {
	field := opts.SortField
	if !field.IsValid() {
		field = registry.CommoditySortName
	}
	column := "name"
	switch field {
	case registry.CommoditySortRegisteredDate:
		column = "registered_date"
	case registry.CommoditySortPurchaseDate:
		column = "purchase_date"
	case registry.CommoditySortCurrentPrice:
		column = "current_price"
	case registry.CommoditySortOriginalPrice:
		column = "original_price"
	case registry.CommoditySortCount:
		column = "count"
	case registry.CommoditySortName:
		column = "name"
	}
	dir := "ASC"
	if opts.SortDesc {
		dir = "DESC"
	}
	return fmt.Sprintf("ORDER BY %s %s, id ASC", column, dir)
}

func (r *CommodityRegistry) Update(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	// Pre-fetch the existing row so we can copy the server-managed
	// acquisition columns onto the entity — RLSGroupRepository.Update
	// writes the entity verbatim, so we cannot rely on the surrounding
	// repository to "skip" these. One extra read per Update is fine at
	// personal-inventory scale and keeps the write-once invariant on
	// the columns intact even if the API silently re-serialised the
	// commodity into the payload.
	if existing, err := r.get(ctx, commodity.GetID()); err == nil && existing != nil {
		commodity.AcquisitionPrice = clonePtrDecimal(existing.AcquisitionPrice)
		commodity.AcquisitionCurrency = clonePtrCurrency(existing.AcquisitionCurrency)
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, commodity, func(ctx context.Context, tx *sqlx.Tx, dbCommodity models.Commodity) error {
		if _, err := r.getArea(ctx, tx, commodity.AreaID); err != nil {
			return err
		}
		// Same orphan-prevention as in Create — an Update that adds new
		// tag slugs needs to grab the per-(group, slug) lock + upsert the
		// tag row before the JSONB column is rewritten by the surrounding
		// Update query.
		return ensureTagRowsInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID, r.createdByUserID, []string(commodity.Tags))
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to update commodity", err)
	}

	return &commodity, nil
}

// clonePtrDecimal returns a heap-allocated copy of d (or nil). Used to
// preserve server-managed acquisition columns across Update calls.
func clonePtrDecimal(d *decimal.Decimal) *decimal.Decimal {
	if d == nil {
		return nil
	}
	cp := *d
	return &cp
}

// clonePtrCurrency mirrors clonePtrDecimal for *Currency pointers.
func clonePtrCurrency(c *models.Currency) *models.Currency {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}

func (r *CommodityRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, nil)
	return err
}

func (r *CommodityRegistry) newSQLRegistry() *store.RLSGroupRepository[models.Commodity, *models.Commodity] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.Commodity](r.dbx, r.tableNames.Commodities())
	}
	return store.NewGroupAwareSQLRegistry[models.Commodity](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.Commodities())
}

func (r *CommodityRegistry) get(ctx context.Context, id string) (*models.Commodity, error) {
	slog.Debug("Getting commodity", "commodity_id", id, "created_by_user_id", r.createdByUserID, "tenant_id", r.tenantID, "service_mode", r.service)

	var commodity models.Commodity
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &commodity)
	if err != nil {
		// Add debug logging for RLS issues
		slog.Debug("Commodity not found - possible RLS issue",
			"commodity_id", id,
			"created_by_user_id", r.createdByUserID,
			"tenant_id", r.tenantID,
			"service_mode", r.service,
		)
		return nil, errxtrace.Wrap("failed to get commodity", err)
	}

	return &commodity, nil
}

func (r *CommodityRegistry) getArea(ctx context.Context, tx *sqlx.Tx, areaID string) (*models.Area, error) {
	var area models.Area
	areaReg := store.NewTxRegistry[models.Area](tx, r.tableNames.Areas())
	err := areaReg.ScanOneByField(ctx, store.Pair("id", areaID), &area)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get area", err)
	}

	return &area, nil
}

// Legacy file-related methods (GetImages/GetManuals/GetInvoices) were removed
// under #1421 alongside the `images`/`invoices`/`manuals` SQL tables they
// queried. Use the unified FileRegistry filtered by linked_entity_meta.
