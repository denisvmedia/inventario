package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	sqlb "github.com/huandu/go-sqlbuilder"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
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

var _ registry.NativePhotoFilterer = (*CommodityRegistry)(nil)

// SupportsNativePhotoFilter marks this backend as resolving
// CommodityListOptions.NoPhoto with the NOT EXISTS subquery of noPhotoExpr.
func (*CommodityRegistry) SupportsNativePhotoFilter() {}

// commodityTables names the tables the list's WHERE clause reads: the
// items, and the loans and file links its subqueries join.
type commodityTables struct {
	commodities string
	loans       string
	fileLinks   string
	files       string
}

func (r *CommodityRegistry) whereTables() commodityTables {
	return commodityTables{
		commodities: string(r.tableNames.Commodities()),
		loans:       string(r.tableNames.CommodityLoans()),
		fileLinks:   string(r.tableNames.FileLinks()),
		files:       string(r.tableNames.Files()),
	}
}

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
	requestedAssetNumber := prepareNewCommodity(ctx, &commodity)

	reg := r.newSQLRegistry()

	createdCommodity, err := reg.CreateWithPrepare(ctx, commodity, func(ctx context.Context, tx *sqlx.Tx, row *models.Commodity) error {
		return prepareNewCommodityInTx(ctx, tx, r.tableNames, r.tenantID, r.groupID, r.createdByUserID, row, requestedAssetNumber)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity", err)
	}

	return &createdCommodity, nil
}

// prepareNewCommodity applies the rules every new item gets before it is
// stored, and returns the asset number a backup restore asks for (0 for
// none). CommodityRegistry.Create and CommodityDraftRegistry.Promote both
// create items through it.
func prepareNewCommodity(ctx context.Context, commodity *models.Commodity) int {
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

	// Normalize an explicit empty area to nil so "" and NULL both mean
	// "unassigned" (#1986): a stored "" would miss the `area_id IS NULL`
	// filter and match no real area.
	commodity.NormalizeAreaID()
	commodity.NormalizeSellerID()

	// Every row written now tells an unknown price from a zero one.
	registry.NormalizeCommodityPrices(ctx, commodity)
	commodity.LegacyZeroPrices = false

	// The trash columns belong to Trash and Restore; a new item is live.
	commodity.ClearTrashState()

	// The asset number is server-managed: whatever the caller sent is
	// dropped and the number is assigned inside the insert transaction.
	// Only the backup restore may ask for a specific (archived) number.
	commodity.AssetNumber = nil
	requested, _ := registry.RestoreAssetNumberFromContext(ctx)
	return requested
}

// prepareNewCommodityInTx runs the steps of creating an item that belong in
// the insert transaction: the asset number, the type, area and seller checks
// and the tag rows.
// row already carries its ids, tenant, group and author.
func prepareNewCommodityInTx(ctx context.Context, tx *sqlx.Tx, tableNames store.TableNames, tenantID, groupID, userID string, row *models.Commodity, requestedAssetNumber int) error {
	// Taken first: every lock this transaction acquires after it
	// (tag rows) is one that no holder of a commodity row lock waits
	// for while holding it, so the order cannot deadlock with Update
	// or the backfill.
	if err := assignAssetNumberInTx(ctx, tx, tableNames.Commodities(), row, requestedAssetNumber); err != nil {
		return err
	}
	// The type must be one of the group's item types; the check locks it
	// against a concurrent delete until this insert commits.
	if err := checkCommodityTypeInTx(ctx, tx, tableNames, tenantID, groupID, row.Type); err != nil {
		return err
	}
	// Area is optional (issue #1986): only verify it exists when set.
	if row.AreaID != nil && *row.AreaID != "" {
		if _, err := getAreaInTx(ctx, tx, tableNames, *row.AreaID); err != nil {
			return err
		}
	}
	if err := checkSellerInTx(ctx, tx, tableNames, row.SellerID); err != nil {
		return err
	}
	// Auto-create / row-lock referenced tag rows inside this same tx
	// so a concurrent DeleteTag(force=true) on one of these slugs
	// can't leave us with an orphan JSONB reference at commit. See
	// ensureTagRowsInTx in tags.go for the full lock invariant.
	return ensureTagRowsInTx(ctx, tx, tableNames, tenantID, groupID, userID, models.TagKindCommodity, []string(row.Tags))
}

func (r *CommodityRegistry) GetByName(ctx context.Context, name string) (*models.Commodity, error) {
	var commodity models.Commodity
	reg := r.newSQLRegistry()

	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf("SELECT * FROM %s WHERE name = $1 AND %s LIMIT 1", r.tableNames.Commodities(), liveRow)
		return scanOne(ctx, tx, &commodity, query, name)
	})
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
		query := fmt.Sprintf("SELECT * FROM %s WHERE %s ORDER BY purchase_date DESC NULLS LAST", r.tableNames.Commodities(), liveRow)
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
			`SELECT * FROM %s WHERE id = ANY($1) AND %s`,
			r.tableNames.Commodities(), liveRow,
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

// Count returns the number of live items.
func (r *CommodityRegistry) Count(ctx context.Context) (int, error) {
	var cnt int
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var err error
		cnt, err = countRows(ctx, tx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", r.tableNames.Commodities(), liveRow))
		return err
	})
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
	where := commodityWhere(opts, r.whereTables())

	var commodities []*models.Commodity
	var total int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		dataSB := newSelect().
			Select("*", totalCountColumn).
			From(string(r.tableNames.Commodities()))
		addWhere(dataSB, where)
		applyCommodityOrder(dataSB, opts)
		paginate(dataSB, offset, limit)
		dataQuery, dataArgs := dataSB.Build()

		rows, err := tx.QueryxContext(ctx, dataQuery, dataArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to list paginated commodities", err)
		}
		defer rows.Close()

		total = totalUnknown
		for rows.Next() {
			var row commodityPage
			if err := rows.StructScan(&row); err != nil {
				return errxtrace.Wrap("failed to scan commodity", err)
			}
			commodities = append(commodities, &row.Commodity)
			total = row.TotalCount
		}
		if err := rows.Err(); err != nil {
			return err
		}

		if total == totalUnknown {
			countSB := newSelect().
				Select("COUNT(*)").
				From(string(r.tableNames.Commodities()))
			countQuery, countArgs := addWhere(countSB, where).Build()
			total, err = countRows(ctx, tx, countQuery, countArgs...)
			if err != nil {
				return errxtrace.Wrap("failed to count commodities", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list paginated commodities", err)
	}

	return commodities, total, nil
}

// ListIDs returns the ids of at most limit commodities opts selects. The
// WHERE clause is the list's, so the ids are those of the rows ListPaginated
// returns for the same opts.
func (r *CommodityRegistry) ListIDs(ctx context.Context, opts registry.CommodityListOptions, limit int) ([]string, error) {
	if limit <= 0 {
		return []string{}, nil
	}
	where := commodityWhere(opts, r.whereTables())
	ids := make([]string, 0)
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		sb := newSelect().Select("id").From(string(r.tableNames.Commodities()))
		addWhere(sb, where)
		sb.Limit(limit)
		query, args := sb.Build()
		return tx.SelectContext(ctx, &ids, query, args...)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodity ids", err)
	}
	return ids, nil
}

// CountEach runs one COUNT(*) per entry of opts, with the WHERE clause of
// ListPaginated, in one transaction.
func (r *CommodityRegistry) CountEach(ctx context.Context, opts []registry.CommodityListOptions) ([]int, error) {
	counts := make([]int, len(opts))
	if len(opts) == 0 {
		return counts, nil
	}
	table := string(r.tableNames.Commodities())
	// One clock for the batch: the same warranty filter counts the same
	// items whichever side of midnight its query is built.
	now := time.Now()
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Under the default READ COMMITTED every COUNT would read its own
		// snapshot, and an item trashed between two of them would count in
		// both or in neither. The transaction has run only SET LOCAL so far,
		// so its isolation level can still change.
		if _, err := tx.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ"); err != nil {
			return errxtrace.Wrap("failed to set the count snapshot", err)
		}
		for i, o := range opts {
			if o.WarrantyNow.IsZero() {
				o.WarrantyNow = now
			}
			where := commodityWhere(o, r.whereTables())
			query, args := addWhere(newSelect().Select("COUNT(*)").From(table), where).Build()
			n, err := countRows(ctx, tx, query, args...)
			if err != nil {
				return errxtrace.Wrap("failed to count commodities", err)
			}
			counts[i] = n
		}
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count commodities", err)
	}
	return counts, nil
}

// CountTags counts the tags of the commodities opts selects in one query: a
// LEFT JOIN LATERAL over the tags array yields one row per (commodity, tag),
// and one row with a NULL slug for a commodity without tags, so grouping by
// the slug gives every tag count plus, under NULL, the untagged count. The
// WHERE clause is the list's, so the counts describe the same commodities.
func (r *CommodityRegistry) CountTags(ctx context.Context, opts registry.CommodityListOptions) (registry.CommodityTagCounts, error) {
	commodities := string(r.tableNames.Commodities())
	where := commodityWhere(opts, r.whereTables())
	sb := commodityTagCountsSelect(commodities)
	query, args := addWhere(sb, where).Build()

	counts := registry.CommodityTagCounts{Tags: map[string]int{}}
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return errxtrace.Wrap("failed to count commodity tags", err)
		}
		defer rows.Close()
		for rows.Next() {
			var slug sql.NullString
			var n int
			if err := rows.Scan(&slug, &n); err != nil {
				return errxtrace.Wrap("failed to scan commodity tag count", err)
			}
			if slug.Valid {
				counts.Tags[slug.String] = n
			} else {
				counts.Untagged = n
			}
		}
		return rows.Err()
	})
	if err != nil {
		return registry.CommodityTagCounts{}, errxtrace.Wrap("failed to count commodity tags", err)
	}
	return counts, nil
}

// ListBrands returns the brands of the group's commodities, see
// registry.CountCommodityBrands. The database groups and counts; the order
// and the limit are applied in Go, so both backends sort alike.
func (r *CommodityRegistry) ListBrands(ctx context.Context, query string, limit int) ([]registry.CommodityBrandCount, error) {
	if limit <= 0 {
		return []registry.CommodityBrandCount{}, nil
	}
	sb := newSelect().Select("brand", "COUNT(*)").From(string(r.tableNames.Commodities()))
	// Items in Trash do not count, as on every other list.
	where := []string{liveRow, sb.NotEqual("brand", "")}
	if q := strings.TrimSpace(query); q != "" {
		where = append(where, sb.ILike("brand", containsPattern(q)))
	}
	sb.Where(where...).GroupBy("brand")
	sqlQuery, args := sb.Build()

	brands := []registry.CommodityBrandCount{}
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		rows, err := tx.QueryContext(ctx, sqlQuery, args...)
		if err != nil {
			return errxtrace.Wrap("failed to list commodity brands", err)
		}
		defer rows.Close()
		for rows.Next() {
			var b registry.CommodityBrandCount
			if err := rows.Scan(&b.Brand, &b.Count); err != nil {
				return errxtrace.Wrap("failed to scan commodity brand", err)
			}
			brands = append(brands, b)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodity brands", err)
	}
	registry.SortCommodityBrands(brands)
	return brands[:min(limit, len(brands))], nil
}

// commodityTagCountsSelect is the SELECT of CountTags without its WHERE.
// COUNT(DISTINCT id) keeps a slug stored twice on one row from counting
// twice.
func commodityTagCountsSelect(commodities string) *sqlb.SelectBuilder {
	sb := newSelect().
		Select("t.slug", "COUNT(DISTINCT "+commodities+".id)").
		From(commodities)
	sb.JoinWithOption(sqlb.LeftJoin,
		"LATERAL jsonb_array_elements_text(COALESCE("+commodities+".tags, '[]'::jsonb)) AS t(slug)",
		"true")
	sb.GroupBy("t.slug")
	return sb
}

// commodityWhere assembles the WHERE clause for the filtered list queries.
// The table names are passed in so a TableNames override flows through to the
// correlated subqueries' outer reference too — hard-coding "commodities.id"
// would silently break the join on any non-default deployment.
//
// The count and the data query share one clause, so the total always describes
// the page.
func commodityWhere(opts registry.CommodityListOptions, tables commodityTables) *sqlb.WhereClause {
	cond := sqlb.NewCond()
	var exprs []string
	add := func(expr string) {
		exprs = append(exprs, expr)
	}

	// Live items, or with opts.Trashed the items in Trash; never both.
	if opts.Trashed {
		add(trashedRow)
	} else {
		add(liveRow)
	}

	// Default view: hide drafts unless caller asked to see them.
	if !opts.IncludeInactive {
		add(cond.Equal("draft", false))
		// Implicit status='in_use' applies only when the caller hasn't
		// chosen specific statuses — see the equivalent comment in the
		// memory implementation for the full rationale.
		if len(opts.Statuses) == 0 {
			add(cond.Equal("status", string(models.CommodityStatusInUse)))
		}
	}
	if opts.DraftsOnly {
		add(cond.Equal("draft", true))
	}

	if len(opts.Types) > 0 {
		add(cond.In("type", anyStrings(opts.Types)...))
	}
	if len(opts.Statuses) > 0 {
		add(cond.In("status", anyStrings(opts.Statuses)...))
	}
	add(commodityAreaExpr(cond, opts))
	if len(opts.SellerIDs) > 0 {
		add(cond.In("seller_id", anyStrings(opts.SellerIDs)...))
	}
	for _, term := range registry.SplitSearchTerms(opts.Search) {
		add(searchTermExpr(cond, term, opts.SearchTagLabels[term]))
	}
	exprs = append(exprs, tagFilterExprs(cond, opts.Tags)...)
	add(warrantyStatusExpr(cond, opts))
	if opts.WarrantyExpiresBefore != "" {
		// Same empty-string defense as the warranty-status branches —
		// '' would lexicographically match `< cutoff` and pollute the
		// result with "no warranty" rows.
		add(cond.And(
			"warranty_expires_at IS NOT NULL",
			"warranty_expires_at <> ''",
			cond.LessThan("warranty_expires_at", opts.WarrantyExpiresBefore),
		))
	}
	add(lentOutExpr(opts.LentOut, tables.commodities, tables.loans))
	if opts.NoPhoto {
		add(noPhotoExpr(cond, tables))
	}
	if opts.NoPurchasePrice {
		add(cond.IsNull("original_price"))
	}
	if opts.NoPurchaseDate {
		// An empty string is the PDate zero value, the same defense as
		// the warranty date above.
		add(cond.Or(cond.IsNull("purchase_date"), cond.Equal("purchase_date", "")))
	}

	return andWhere(cond, exprs...)
}

// searchTermExpr is the SQL form of one term of registry.MatchCommoditySearch.
// The generated search_text column joins the text fields of
// registry.CommoditySearchTextFields, tag slugs included, with U+001F, and a
// term never holds a control character, so one escaped ILIKE against it
// equals "one of those fields contains the term"; the trigram index
// commodities_search_text_trgm_idx serves it. The fields of
// registry.CommodityColumnSearchFields get an ILIKE each on their own
// trigram-indexed column. A tag found by its label arrives as a pre-resolved
// slug list, matched with ?| on the GIN-indexed tags column, and a term that
// reads as an asset ID also matches the number.
func searchTermExpr(cond *sqlb.Cond, term string, labelSlugs []string) string {
	pattern := containsPattern(term)
	alternatives := []string{"search_text ILIKE " + cond.Var(pattern)}
	for _, field := range registry.CommodityColumnSearchFields {
		alternatives = append(alternatives, string(field)+" ILIKE "+cond.Var(pattern))
	}
	if len(labelSlugs) > 0 {
		alternatives = append(alternatives, "tags ?| "+cond.Var(labelSlugs)+"::text[]")
	}
	if n, ok := models.ParseAssetID(term); ok {
		alternatives = append(alternatives, cond.Equal("asset_number", n))
	}
	return cond.Or(alternatives...)
}

// tagFilterExprs states registry.CommodityTagFilter.Matches in SQL, one
// AND-ed expression per part. The ?& and ?| operators use the GIN index on
// tags. A NULL tags column is read as an empty list, the way
// TagRegistry.GetStats counts untagged items, so an item without tags passes
// an exclusion instead of vanishing on a NULL.
func tagFilterExprs(cond *sqlb.Cond, f registry.CommodityTagFilter) []string {
	var exprs []string
	if f.Untagged {
		exprs = append(exprs, "jsonb_array_length(COALESCE(tags, '[]'::jsonb)) = 0")
	}
	if len(f.Include) > 0 {
		op := "?&"
		if f.Mode == registry.TagMatchAny {
			op = "?|"
		}
		exprs = append(exprs, "tags "+op+" "+cond.Var(f.Include)+"::text[]")
	}
	if len(f.Exclude) > 0 {
		exprs = append(exprs, "NOT (COALESCE(tags, '[]'::jsonb) ?| "+cond.Var(f.Exclude)+"::text[])")
	}
	return exprs
}

// commodityAreaExpr builds the AreaID / Unassigned predicate (issue #1986).
// AreaID wins when both are set; neither yields an empty string.
func commodityAreaExpr(cond *sqlb.Cond, opts registry.CommodityListOptions) string {
	switch {
	case opts.AreaID != "":
		return cond.Equal("area_id", opts.AreaID)
	case opts.Unassigned:
		return cond.IsNull("area_id")
	default:
		return ""
	}
}

// warrantyStatusExpr builds the computed-warranty-status disjunction, empty
// when no statuses are requested. Each status maps to a closed-form predicate
// on warranty_expires_at. Every non-`none` predicate guards against an empty
// `warranty_expires_at` alongside the NULL check — empty strings reach the
// column via the PDate zero value and would otherwise satisfy `expired`'s `<`
// test, an empty string sorting below any ISO date, surfacing "no warranty"
// rows.
func warrantyStatusExpr(cond *sqlb.Cond, opts registry.CommodityListOptions) string {
	if len(opts.WarrantyStatuses) == 0 {
		return ""
	}
	now := opts.WarrantyNow
	if now.IsZero() {
		now = time.Now()
	}
	today := now.UTC().Format("2006-01-02")
	cutoff := now.UTC().AddDate(0, 0, models.WarrantyExpiringWindowDays).Format("2006-01-02")

	var disj []string
	for _, s := range opts.WarrantyStatuses {
		switch s {
		case registry.WarrantyStatusFilterNone:
			disj = append(disj, cond.Or(
				cond.IsNull("warranty_expires_at"),
				cond.Equal("warranty_expires_at", ""),
			))
		case registry.WarrantyStatusFilterExpired:
			disj = append(disj, cond.And(
				"warranty_expires_at IS NOT NULL",
				"warranty_expires_at <> ''",
				cond.LessThan("warranty_expires_at", today),
			))
		case registry.WarrantyStatusFilterExpiring:
			disj = append(disj, cond.And(
				"warranty_expires_at <> ''",
				cond.GreaterEqualThan("warranty_expires_at", today),
				cond.LessEqualThan("warranty_expires_at", cutoff),
			))
		case registry.WarrantyStatusFilterActive:
			disj = append(disj, cond.And(
				"warranty_expires_at <> ''",
				cond.GreaterThan("warranty_expires_at", cutoff),
			))
		}
	}
	if len(disj) == 0 {
		return ""
	}
	return cond.Or(disj...)
}

// lentOutExpr returns the EXISTS / NOT EXISTS subquery for the LentOut filter,
// or "" when the filter is inactive. The partial index
// `idx_commodity_loans_active` (returned_at IS NULL) keeps the subquery cheap;
// RLS on commodity_loans constrains the inner SELECT to the caller's
// tenant+group automatically.
func lentOutExpr(lentOut *bool, commoditiesTable, loansTable string) string {
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

// noPhotoExpr keeps the items without a photo: no live image file linked in
// the item's images bucket, the files FileRegistry.FirstPhotos picks a cover
// from. Row-level security on the file tables keeps the subquery to the
// caller's group.
func noPhotoExpr(cond *sqlb.Cond, tables commodityTables) string {
	return fmt.Sprintf(
		"NOT EXISTS (SELECT 1 FROM %s l JOIN %s f ON f.id = l.file_id "+
			"WHERE l.entity_id = %s.id AND l.entity_type = %s AND l.meta = %s "+
			"AND f.deleted_at IS NULL AND f.type = %s AND f.category = %s)",
		tables.fileLinks, tables.files, tables.commodities,
		cond.Var(models.FileLinkCommodity), cond.Var(models.FileLinkMetaImages),
		cond.Var(string(models.FileTypeImage)), cond.Var(string(models.FileCategoryImages)),
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
func applyCommodityOrder(sb *sqlb.SelectBuilder, opts registry.CommodityListOptions) {
	field := opts.SortField
	if !field.IsValid() {
		field = registry.CommoditySortName
	}
	column := "name"
	caseInsensitive := false
	// Dates and prices are NULL when unknown. NULLS LAST keeps them after
	// every known value in both directions (PostgreSQL's default puts them
	// first on DESC); the memory backend orders them the same way.
	nullable := false
	switch field {
	case registry.CommoditySortRegisteredDate:
		column = "registered_date"
		nullable = true
	case registry.CommoditySortPurchaseDate:
		column = "purchase_date"
		nullable = true
	case registry.CommoditySortCurrentPrice:
		column = "current_price"
		nullable = true
	case registry.CommoditySortOriginalPrice:
		column = "original_price"
		nullable = true
	case registry.CommoditySortCount:
		column = "count"
	case registry.CommoditySortName:
		column = "name"
		caseInsensitive = true
	}
	// id tiebreaker mirrors the primary direction so paging stays stable
	// across duplicate-name rows AND matches the memory backend, where
	// SortStableFunc reverses the entire comparator (including the id
	// tiebreaker) on SortDesc=true. Without this, the two backends
	// disagree on the order of rows that share a sort-field value —
	// page boundaries jump between memory and postgres on the same query.
	expr := column
	if caseInsensitive {
		expr = "LOWER(" + column + ")"
	}
	nulls := ""
	if nullable {
		nulls = " NULLS LAST"
	}
	// Per-column direction, not sb.Desc(): that appends one keyword after the
	// whole list, which would leave the leading column ascending.
	if opts.SortDesc {
		sb.OrderBy(expr + " DESC" + nulls).OrderByDesc("id")
	} else {
		sb.OrderBy(expr + " ASC" + nulls).OrderByAsc("id")
	}
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
		commodity.UnsetEchoedLegacyZeros(existing)
	}

	// Normalize an explicit empty area to nil (#1986) — un-assigning via an
	// empty string persists as NULL, consistent with the IS NULL filter.
	commodity.NormalizeAreaID()
	commodity.NormalizeSellerID()

	// The caller's prices tell unknown from zero, whatever the row held.
	registry.NormalizeCommodityPrices(ctx, &commodity)
	commodity.LegacyZeroPrices = false

	reg := r.newSQLRegistry()

	err := reg.UpdateWithPrepare(ctx, commodity, func(ctx context.Context, tx *sqlx.Tx, stored models.Commodity, row *models.Commodity) error {
		// A trashed item is read-only until it is restored. stored was read
		// without a lock, so the check is made again under one. The trash
		// columns themselves belong to Trash and Restore.
		if err := lockLiveRow(ctx, tx, r.tableNames.Commodities(), "commodity", stored.ID); err != nil {
			return err
		}
		row.ClearTrashState()
		// The asset number is server-managed: carry the stored one over.
		// Read under a row lock so a backfill numbering this row at the
		// same moment is either seen or waits for this update to finish.
		if err := keepAssetNumberInTx(ctx, tx, r.tableNames.Commodities(), row); err != nil {
			return err
		}
		commodity.AssetNumber = row.AssetNumber
		// A changed type must be one of the group's item types. "Changed"
		// compares with the type the row holds under the lock, not with
		// stored: a type delete may have moved the item to another type
		// after stored was read, and an update that kept the old type would
		// then write back a type the group no longer has. The new type's
		// row is locked after the item's, and a type delete locks the
		// deleted type before the items it moves; an update never waits for
		// the type it leaves, so the orders cannot deadlock.
		current, err := lockedCommodityType(ctx, tx, r.tableNames.Commodities(), stored.ID)
		if err != nil {
			return err
		}
		if row.Type != current {
			if err := checkCommodityTypeInTx(ctx, tx, r.tableNames, stored.TenantID, stored.GroupID, row.Type); err != nil {
				return err
			}
		}
		// Area is optional (issue #1986): only verify it exists when set.
		if commodity.AreaID != nil && *commodity.AreaID != "" {
			if _, err := r.getArea(ctx, tx, *commodity.AreaID); err != nil {
				return err
			}
		}
		if err := r.checkSeller(ctx, tx, commodity.SellerID); err != nil {
			return err
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

// lockedCommodityType returns the type of the commodity id, which the
// caller's transaction has already locked.
func lockedCommodityType(ctx context.Context, tx *sqlx.Tx, table store.TableName, id string) (models.CommodityType, error) {
	var current models.CommodityType
	if err := tx.GetContext(ctx, &current, fmt.Sprintf(`SELECT type FROM %s WHERE id = $1`, table), id); err != nil {
		return "", errxtrace.Wrap("failed to read commodity type", err, errx.Attrs("commodity_id", id))
	}
	return current, nil
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

// Delete removes ONLY the commodity row. Its DB children (loans, services,
// supply-links) CASCADE and cover_file_id is ON DELETE SET NULL, so the row
// delete itself is FK-safe — but the files LINKED to the commodity
// (file_links with entity_type 'commodity') are NOT cascaded, and their
// physical blobs are not touched here.
//
// INVARIANT (#2121): user-initiated commodity deletes MUST go through
// services.EntityService.DeleteCommodityRecursive, which drops this row and then
// runs FileService.DetachEntityFiles: a file attached only to this commodity is
// deleted (row + best-effort blob), a file other entities still use loses just
// this commodity's link. Calling this bare row delete directly leaves the links
// dangling and every file attached only here orphaned in the bucket. The only
// legitimate direct callers are tests and the group/tenant purgers, which sweep
// all of the (tenant, group)'s blobs separately before dropping rows.
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

	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1 AND %s LIMIT 1", r.tableNames.Commodities(), liveRow)
		return scanOne(ctx, tx, &commodity, query, id)
	})
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
	return getAreaInTx(ctx, tx, r.tableNames, areaID)
}

// getAreaInTx reads an area in tx; ErrNotFound when it is not visible.
func getAreaInTx(ctx context.Context, tx *sqlx.Tx, tableNames store.TableNames, areaID string) (*models.Area, error) {
	var area models.Area
	areaReg := store.NewTxRegistry[models.Area](tx, tableNames.Areas())
	err := areaReg.ScanOneByField(ctx, store.Pair("id", areaID), &area)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get area", err)
	}

	return &area, nil
}

// checkSeller verifies that sellerID, when set, names a seller the
// transaction can see. The transaction runs under the group's row-level
// security, so a seller of another group is not found, which the foreign key
// alone would not catch.
func (r *CommodityRegistry) checkSeller(ctx context.Context, tx *sqlx.Tx, sellerID *string) error {
	return checkSellerInTx(ctx, tx, r.tableNames, sellerID)
}

// checkSellerInTx is checkSeller for code that has no CommodityRegistry,
// such as promoting a draft.
func checkSellerInTx(ctx context.Context, tx *sqlx.Tx, tableNames store.TableNames, sellerID *string) error {
	if sellerID == nil {
		return nil
	}
	var seller models.Seller
	sellerReg := store.NewTxRegistry[models.Seller](tx, tableNames.Sellers())
	if err := sellerReg.ScanOneByField(ctx, store.Pair("id", *sellerID), &seller); err != nil {
		return errxtrace.Wrap("failed to get seller", err)
	}
	return nil
}

// Legacy file-related methods (GetImages/GetManuals/GetInvoices) were removed
// under #1421 alongside the `images`/`invoices`/`manuals` SQL tables they
// queried. Use the unified FileRegistry filtered by link meta.

// commodityPage carries the window function's total alongside the row (#1032).
type commodityPage struct {
	models.Commodity
	TotalCount int `db:"total_count"`
}

// GetTrashed returns a trashed item by id.
func (r *CommodityRegistry) GetTrashed(ctx context.Context, id string) (*models.Commodity, error) {
	var commodity models.Commodity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1 AND %s LIMIT 1", r.tableNames.Commodities(), trashedRow)
		return scanOne(ctx, tx, &commodity, query, id)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to get trashed commodity", err)
	}
	return &commodity, nil
}

// ListTrashed returns a page of the group's trashed items, most recently
// deleted first.
func (r *CommodityRegistry) ListTrashed(ctx context.Context, offset, limit int) ([]*models.Commodity, int, error) {
	var commodities []*models.Commodity
	var total int
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		dataSB := newSelect().Select("*", totalCountColumn).From(string(r.tableNames.Commodities()))
		dataSB.Where(trashedRow).OrderByDesc("deleted_at").OrderByDesc("id")
		dataQuery, dataArgs := paginate(dataSB, offset, limit).Build()
		rows, err := scanAll[commodityPage](ctx, tx, dataQuery, dataArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to list trashed commodities", err)
		}
		total = totalUnknown
		for _, row := range rows {
			commodities = append(commodities, &row.Commodity)
			total = row.TotalCount
		}
		if total == totalUnknown {
			total, err = countRows(ctx, tx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", r.tableNames.Commodities(), trashedRow))
			if err != nil {
				return errxtrace.Wrap("failed to count trashed commodities", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list trashed commodities", err)
	}
	return commodities, total, nil
}

// ListTrashedBefore returns up to limit items deleted before cutoff, oldest
// first. On a service registry it spans every tenant and group.
func (r *CommodityRegistry) ListTrashedBefore(ctx context.Context, cutoff time.Time, limit int) ([]*models.Commodity, error) {
	if limit <= 0 {
		return nil, nil
	}
	var commodities []*models.Commodity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE %s AND deleted_at < $1 ORDER BY deleted_at ASC, id ASC LIMIT $2`,
			r.tableNames.Commodities(), trashedRow)
		var err error
		commodities, err = scanAll[models.Commodity](ctx, tx, query, cutoff.UTC(), limit)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list expired trashed commodities", err)
	}
	return commodities, nil
}

// Trash moves a live item to Trash in one conditional UPDATE, so two
// concurrent deletes trash it once and the second one gets ErrNotFound.
func (r *CommodityRegistry) Trash(ctx context.Context, id, deletedBy string, at time.Time) (*models.Commodity, error) {
	var commodity models.Commodity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET deleted_at = $2, deleted_by = $3, deleted_from_area_id = area_id
			 WHERE id = $1 AND %s RETURNING *`,
			r.tableNames.Commodities(), liveRow)
		return scanOne(ctx, tx, &commodity, query, id, at.UTC(), deletedBy)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to trash commodity", err, errx.Attrs("commodity_id", id))
	}
	return &commodity, nil
}

// SetDraft sets or clears the draft mark of a live item, under the row lock
// that Update takes too.
func (r *CommodityRegistry) SetDraft(ctx context.Context, id string, draft bool, check func(models.Commodity) error) (*models.Commodity, error) {
	var commodity models.Commodity
	table := r.tableNames.Commodities()
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := lockLiveRow(ctx, tx, table, "commodity", id); err != nil {
			return err
		}
		if err := scanOne(ctx, tx, &commodity, fmt.Sprintf(`SELECT * FROM %s WHERE id = $1`, table), id); err != nil {
			return err
		}
		commodity.Draft = draft
		if check != nil {
			if err := check(commodity); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET draft = $2 WHERE id = $1`, table), id, draft)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to set the draft mark", err, errx.Attrs("commodity_id", id))
	}
	return &commodity, nil
}

// Restore makes a trashed item live again.
func (r *CommodityRegistry) Restore(ctx context.Context, id string) (*models.Commodity, error) {
	var commodity models.Commodity
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`UPDATE %s SET deleted_at = NULL, deleted_by = NULL, deleted_from_area_id = NULL
			 WHERE id = $1 AND %s RETURNING *`,
			r.tableNames.Commodities(), trashedRow)
		return scanOne(ctx, tx, &commodity, query, id)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to restore commodity", err, errx.Attrs("commodity_id", id))
	}
	return &commodity, nil
}

// PurgeTrashed deletes a trashed item's row. The condition on deleted_at lives
// in the DELETE, so a concurrent restore wins and the purge deletes nothing.
// It holds the group's currency-migration lock while it deletes: a running
// migration lists every item of the group, trashed ones included, and then
// updates each listed row, so a row deleted in between would fail it.
func (r *CommodityRegistry) PurgeTrashed(ctx context.Context, id string) (bool, error) {
	var deleted bool
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var groupID string
		query := fmt.Sprintf(`SELECT group_id FROM %s WHERE id = $1 AND %s`, r.tableNames.Commodities(), trashedRow)
		if err := tx.GetContext(ctx, &groupID, query, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		if err := lockGroupCurrencyMigration(ctx, tx, groupID); err != nil {
			return err
		}
		query = fmt.Sprintf(`DELETE FROM %s WHERE id = $1 AND %s`, r.tableNames.Commodities(), trashedRow)
		var err error
		deleted, err = execAffected(ctx, tx, query, id)
		return err
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to purge commodity", err, errx.Attrs("commodity_id", id))
	}
	return deleted, nil
}
