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
var _ registry.NativeLentOutFilterer = (*CommodityRegistry)(nil)

// SupportsNativeLentOutFilter marks this backend as resolving
// CommodityListOptions.LentOut via the EXISTS subquery on
// commodity_loans inside buildCommodityWhere — apiserver should skip
// the pre-resolve fetch and let the single query do the join.
func (*CommodityRegistry) SupportsNativeLentOutFilter() {}

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

	// Acquisition columns are server-managed (issue #1550 / #202): drop any
	// value the API caller smuggled in. The only legitimate writers are the
	// migration worker (via go/registry/internal/migrationops, NULL→value once)
	// and the signature-verified #534 backup restore, which reconstructs the
	// archived pair on a fresh row by signalling it through the trusted
	// WithRestoreAcquisition context seam. Absent that signal the pair is cleared
	// so it stays immutable for every user write.
	if price, currency, ok := registry.RestoreAcquisitionFromContext(ctx); ok {
		commodity.AcquisitionPrice = &price
		commodity.AcquisitionCurrency = &currency
	} else {
		commodity.AcquisitionPrice = nil
		commodity.AcquisitionCurrency = nil
	}

	reg := r.newSQLRegistry()

	createdCommodity, err := reg.Create(ctx, commodity, func(ctx context.Context, tx *sqlx.Tx) error {
		// Area is optional (issue #1986): only verify it exists when set.
		if commodity.AreaID != nil && *commodity.AreaID != "" {
			if _, err := r.getArea(ctx, tx, *commodity.AreaID); err != nil {
				return err
			}
		}
		// Auto-create / row-lock referenced tag rows inside this same tx
		// so a concurrent DeleteTag(force=true) on one of these slugs
		// can't leave us with an orphan JSONB reference at commit. See
		// ensureTagRowsInTx in tags.go for the full lock invariant.
		return ensureTagRowsInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID, r.createdByUserID, models.TagKindCommodity, []string(commodity.Tags))
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

// GetMany returns the commodities matching ids in unspecified order via a
// single `WHERE id = ANY($1)` round-trip. Missing / RLS-hidden ids are
// silently dropped — see the interface doc in registry.CommodityRegistry
// for the full contract. Empty ids returns (nil, nil) without opening a
// transaction; duplicate ids collapse server-side so a single commodity is
// returned once even if its id appears multiple times in the slice. RLS
// keeps the query group-scoped automatically for user-mode registries.
func (r *CommodityRegistry) GetMany(ctx context.Context, ids []string) ([]*models.Commodity, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var commodities []*models.Commodity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE id = ANY($1)`,
			r.tableNames.Commodities(),
		)
		rows, qerr := tx.QueryxContext(ctx, query, ids)
		if qerr != nil {
			return errxtrace.Wrap("failed to batch-fetch commodities", qerr)
		}
		defer rows.Close()
		for rows.Next() {
			var commodity models.Commodity
			if scanErr := rows.StructScan(&commodity); scanErr != nil {
				return errxtrace.Wrap("failed to scan commodity", scanErr)
			}
			cp := commodity
			commodities = append(commodities, &cp)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to batch-fetch commodities", err)
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

	whereClause, whereArgs := buildCommodityWhere(opts, string(r.tableNames.Commodities()), string(r.tableNames.CommodityLoans()))
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
// query plans for the common "no filter" path). commoditiesTable and
// loansTable carry the resolved table names so the LentOut filter's
// EXISTS subquery stays correct under a TableNames override (schema
// prefix, sharded suffix, test overrides) instead of hard-coding the
// default identifiers.
// commodityAreaCond builds the AreaID / Unassigned WHERE predicate (issue
// #1986). An explicit AreaID yields a `area_id = $idx` clause with an arg
// (hasArg=true); Unassigned alone yields `area_id IS NULL` with no arg
// (hasArg=false); neither yields an empty cond. AreaID wins when both are set.
func commodityAreaCond(opts registry.CommodityListOptions, idx int) (cond string, arg any, hasArg bool) {
	switch {
	case opts.AreaID != "":
		return fmt.Sprintf("area_id = $%d", idx), opts.AreaID, true
	case opts.Unassigned:
		return "area_id IS NULL", nil, false
	default:
		return "", nil, false
	}
}

// buildWarrantyStatusCond builds the computed-warranty-status disjunction for the
// WHERE clause, starting placeholders at startIdx. Returns the parenthesized
// condition (empty when no statuses are requested), the ordered args it consumed,
// and the next free placeholder index. Each status maps to a closed-form
// predicate on warranty_expires_at; multiple statuses are OR-ed, then AND-ed into
// the surrounding WHERE. Every non-`none` predicate guards against
// `warranty_expires_at = ”` alongside the NULL check — empty strings reach the
// column via the PDate zero value and would otherwise satisfy `expired`'s `<`
// test (since ” sorts below any ISO date), surfacing "no warranty" rows.
func buildWarrantyStatusCond(opts registry.CommodityListOptions, startIdx int) (string, []any, int) {
	if len(opts.WarrantyStatuses) == 0 {
		return "", nil, startIdx
	}
	now := opts.WarrantyNow
	if now.IsZero() {
		now = time.Now()
	}
	today := now.UTC().Format("2006-01-02")
	cutoff := now.UTC().AddDate(0, 0, models.WarrantyExpiringWindowDays).Format("2006-01-02")

	idx := startIdx
	var args []any
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
	if len(disj) == 0 {
		return "", nil, startIdx
	}
	return "(" + strings.Join(disj, " OR ") + ")", args, idx
}

func buildCommodityWhere(opts registry.CommodityListOptions, commoditiesTable, loansTable string) (string, []any) {
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
	if cond, arg, hasArg := commodityAreaCond(opts, idx); cond != "" {
		conds = append(conds, cond)
		if hasArg {
			args = append(args, arg)
			idx++
		}
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

	if cond, wargs, nextIdx := buildWarrantyStatusCond(opts, idx); cond != "" {
		conds = append(conds, cond)
		args = append(args, wargs...)
		idx = nextIdx
	}
	if opts.WarrantyExpiresBefore != "" {
		// Same empty-string defense as the warranty-status branches —
		// '' would lexicographically match `< cutoff` and pollute the
		// result with "no warranty" rows.
		conds = append(conds, fmt.Sprintf("(warranty_expires_at IS NOT NULL AND warranty_expires_at <> '' AND warranty_expires_at < $%d)", idx))
		args = append(args, opts.WarrantyExpiresBefore)
		// idx is unused below — LentOut doesn't add a parameter.
	}

	if c := buildLentOutCond(opts.LentOut, commoditiesTable, loansTable); c != "" {
		conds = append(conds, c)
	}

	if len(conds) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

// buildLentOutCond returns the EXISTS / NOT EXISTS subquery for the
// LentOut filter, or "" when the filter is inactive. Both table names
// are passed in so a TableNames override (schema prefix, sharded
// suffix, test stub) flows through to the correlated subquery's outer
// reference too — hard-coding "commodities.id" would silently break
// the join on any non-default deployment. Split out of
// buildCommodityWhere to keep the parent under the gocyclo threshold;
// the partial index `idx_commodity_loans_active` (returned_at IS NULL)
// keeps the subquery cheap on the storage side. RLS on commodity_loans
// constrains the inner SELECT to the caller's tenant+group automatically.
func buildLentOutCond(lentOut *bool, commoditiesTable, loansTable string) string {
	if lentOut == nil {
		return ""
	}
	op := "EXISTS"
	if !*lentOut {
		op = "NOT EXISTS"
	}
	return fmt.Sprintf(
		"%s (SELECT 1 FROM %s WHERE commodity_id = %s.id AND returned_at IS NULL)",
		op, loansTable, commoditiesTable,
	)
}

// buildCommodityOrder maps SortField to a SQL ORDER BY clause. The id tie
// breaker keeps page boundaries stable when the primary key has duplicate
// sort-field values (e.g. several commodities with the same name).
//
// Name sorts are case-insensitive (LOWER(name)) so "e2e-react-..." lands
// next to "Ergonomic Chair" instead of after every uppercase-starting
// commodity. Pre-#1658 the inventory was tiny (~11 rows) and the
// case-sensitive C-collation order didn't matter; with the enriched
// seed pushing the catalogue to ~36 rows, a lowercase-starting user
// commodity would otherwise jump to page 2 and out of any test's
// default-viewport assertions.
func buildCommodityOrder(opts registry.CommodityListOptions) string {
	field := opts.SortField
	if !field.IsValid() {
		field = registry.CommoditySortName
	}
	column := "name"
	caseInsensitive := false
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
		caseInsensitive = true
	}
	dir := "ASC"
	if opts.SortDesc {
		dir = "DESC"
	}
	// id tiebreaker mirrors the primary direction so paging stays stable
	// across duplicate-name rows AND matches the memory backend, where
	// SortStableFunc reverses the entire comparator (including the id
	// tiebreaker) on SortDesc=true. Without this, the two backends
	// disagree on the order of rows that share a sort-field value —
	// page boundaries jump between memory and postgres on the same query.
	if caseInsensitive {
		return fmt.Sprintf("ORDER BY LOWER(%s) %s, id %s", column, dir, dir)
	}
	return fmt.Sprintf("ORDER BY %s %s, id %s", column, dir, dir)
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
		// Area is optional (issue #1986): only verify it exists when set.
		if commodity.AreaID != nil && *commodity.AreaID != "" {
			if _, err := r.getArea(ctx, tx, *commodity.AreaID); err != nil {
				return err
			}
		}
		// Same orphan-prevention as in Create — an Update that adds new
		// tag slugs needs to grab the per-(group, slug) lock + upsert the
		// tag row before the JSONB column is rewritten by the surrounding
		// Update query.
		return ensureTagRowsInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID, r.createdByUserID, models.TagKindCommodity, []string(commodity.Tags))
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
