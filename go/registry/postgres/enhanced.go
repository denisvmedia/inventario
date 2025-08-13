package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// EnhancedPostgreSQLRegistry implements the enhanced registry interface with PostgreSQL-specific features
type EnhancedPostgreSQLRegistry struct {
	*registry.Set
	pool         *pgxpool.Pool
	sqlxDB       *sqlx.DB
	capabilities registry.DatabaseCapabilities
}

// NewEnhancedPostgreSQLRegistry creates a new enhanced PostgreSQL registry
func NewEnhancedPostgreSQLRegistry(pool *pgxpool.Pool, sqlxDB *sqlx.DB) *EnhancedPostgreSQLRegistry {
	baseSet := NewRegistrySet(sqlxDB)
	capabilities := registry.CapabilityMatrix["postgres"]

	return &EnhancedPostgreSQLRegistry{
		Set:          baseSet,
		pool:         pool,
		sqlxDB:       sqlxDB,
		capabilities: capabilities,
	}
}

// GetCapabilities returns the PostgreSQL capabilities
func (r *EnhancedPostgreSQLRegistry) GetCapabilities() registry.DatabaseCapabilities {
	return r.capabilities
}

// FullTextSearch performs PostgreSQL full-text search on commodities
func (r *EnhancedPostgreSQLRegistry) FullTextSearch(ctx context.Context, query string, options ...registry.SearchOption) ([]*models.Commodity, error) {
	opts := &registry.SearchOptions{Limit: 100, Offset: 0}
	for _, opt := range options {
		opt(opts)
	}

	sql := `
		SELECT c.*, ts_rank(search_vector, plainto_tsquery($1)) as rank
		FROM commodities c
		WHERE search_vector @@ plainto_tsquery($1)
		ORDER BY rank DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, sql, query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to execute full-text search")
	}
	defer rows.Close()

	var commodities []*models.Commodity
	for rows.Next() {
		var commodity models.Commodity
		var rank float64

		err := rows.Scan(
			&commodity.ID,
			&commodity.Name,
			&commodity.ShortName,
			&commodity.Type,
			&commodity.AreaID,
			&commodity.Count,
			&commodity.OriginalPrice,
			&commodity.OriginalPriceCurrency,
			&commodity.ConvertedOriginalPrice,
			&commodity.CurrentPrice,
			&commodity.SerialNumber,
			&commodity.ExtraSerialNumbers,
			&commodity.PartNumbers,
			&commodity.Tags,
			&commodity.Status,
			&commodity.PurchaseDate,
			&commodity.RegisteredDate,
			&commodity.LastModifiedDate,
			&commodity.URLs,
			&commodity.Comments,
			&commodity.Draft,
			&rank,
		)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan commodity")
		}

		commodities = append(commodities, &commodity)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating over rows")
	}

	return commodities, nil
}

// JSONBQuery performs JSONB queries on specified table and field
func (r *EnhancedPostgreSQLRegistry) JSONBQuery(ctx context.Context, table string, jsonbField string, query map[string]any) ([]any, error) {
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal query")
	}

	sql := fmt.Sprintf("SELECT * FROM %s WHERE %s @> $1", table, jsonbField)

	rows, err := r.pool.Query(ctx, sql, queryJSON)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to execute JSONB query")
	}
	defer rows.Close()

	var results []any
	for rows.Next() {
		// This would need to be customized based on the table structure
		// For now, return a generic map
		values, err := rows.Values()
		if err != nil {
			return nil, errkit.Wrap(err, "failed to get row values")
		}
		results = append(results, values)
	}

	return results, nil
}

// BulkUpsert performs bulk upsert operations
func (r *EnhancedPostgreSQLRegistry) BulkUpsert(ctx context.Context, entities []any) error {
	// This would need to be implemented based on entity types
	// For now, return a placeholder implementation
	return fmt.Errorf("bulk upsert not yet implemented for %d entities", len(entities))
}

// CreateAdvancedIndex creates PostgreSQL-specific indexes
func (r *EnhancedPostgreSQLRegistry) CreateAdvancedIndex(ctx context.Context, spec registry.IndexSpec) error {
	var sql string

	switch spec.Type {
	case "gin_jsonb":
		sql = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s USING GIN (%s)",
			spec.Name, spec.Table, spec.Column)
	case "gist_tsvector":
		sql = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s USING GIST (%s)",
			spec.Name, spec.Table, spec.Column)
	case "btree_partial":
		sql = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s) WHERE %s",
			spec.Name, spec.Table, spec.Column, spec.Condition)
	case "gin_array":
		sql = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s USING GIN (%s)",
			spec.Name, spec.Table, spec.Column)
	default:
		return fmt.Errorf("unsupported index type: %s", spec.Type)
	}

	_, err := r.pool.Exec(ctx, sql)
	return errkit.Wrap(err, "failed to create advanced index")
}

// Enhanced registry accessors
func (r *EnhancedPostgreSQLRegistry) EnhancedCommodityRegistry() registry.EnhancedCommodityRegistry {
	return &EnhancedPostgreSQLCommodityRegistry{
		base:         r.Set.CommodityRegistry,
		pool:         r.pool,
		sqlxDB:       r.sqlxDB,
		capabilities: r.capabilities,
	}
}

func (r *EnhancedPostgreSQLRegistry) EnhancedAreaRegistry() registry.EnhancedAreaRegistry {
	// For now, return a fallback implementation
	// TODO: Implement PostgreSQL-specific area registry
	fallback := registry.NewFallbackRegistry(r.Set, "postgres")
	return fallback.EnhancedAreaRegistry()
}

func (r *EnhancedPostgreSQLRegistry) EnhancedLocationRegistry() registry.EnhancedLocationRegistry {
	// For now, return a fallback implementation
	// TODO: Implement PostgreSQL-specific location registry
	fallback := registry.NewFallbackRegistry(r.Set, "postgres")
	return fallback.EnhancedLocationRegistry()
}

func (r *EnhancedPostgreSQLRegistry) EnhancedFileRegistry() registry.EnhancedFileRegistry {
	// For now, return a fallback implementation
	// TODO: Implement PostgreSQL-specific file registry
	fallback := registry.NewFallbackRegistry(r.Set, "postgres")
	return fallback.EnhancedFileRegistry()
}

// EnhancedPostgreSQLCommodityRegistry implements enhanced commodity operations
type EnhancedPostgreSQLCommodityRegistry struct {
	base         registry.CommodityRegistry
	pool         *pgxpool.Pool
	sqlxDB       *sqlx.DB
	capabilities registry.DatabaseCapabilities
}

// Implement CommodityRegistry interface by delegating to base
func (r *EnhancedPostgreSQLCommodityRegistry) Create(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	return r.base.Create(ctx, commodity)
}

func (r *EnhancedPostgreSQLCommodityRegistry) Get(ctx context.Context, id string) (*models.Commodity, error) {
	return r.base.Get(ctx, id)
}

func (r *EnhancedPostgreSQLCommodityRegistry) List(ctx context.Context) ([]*models.Commodity, error) {
	return r.base.List(ctx)
}

func (r *EnhancedPostgreSQLCommodityRegistry) Update(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	return r.base.Update(ctx, commodity)
}

func (r *EnhancedPostgreSQLCommodityRegistry) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}

func (r *EnhancedPostgreSQLCommodityRegistry) Count(ctx context.Context) (int, error) {
	return r.base.Count(ctx)
}

func (r *EnhancedPostgreSQLCommodityRegistry) AddImage(ctx context.Context, commodityID, imageID string) error {
	return r.base.AddImage(ctx, commodityID, imageID)
}

func (r *EnhancedPostgreSQLCommodityRegistry) GetImages(ctx context.Context, commodityID string) ([]string, error) {
	return r.base.GetImages(ctx, commodityID)
}

func (r *EnhancedPostgreSQLCommodityRegistry) DeleteImage(ctx context.Context, commodityID, imageID string) error {
	return r.base.DeleteImage(ctx, commodityID, imageID)
}

func (r *EnhancedPostgreSQLCommodityRegistry) AddManual(ctx context.Context, commodityID, manualID string) error {
	return r.base.AddManual(ctx, commodityID, manualID)
}

func (r *EnhancedPostgreSQLCommodityRegistry) GetManuals(ctx context.Context, commodityID string) ([]string, error) {
	return r.base.GetManuals(ctx, commodityID)
}

func (r *EnhancedPostgreSQLCommodityRegistry) DeleteManual(ctx context.Context, commodityID, manualID string) error {
	return r.base.DeleteManual(ctx, commodityID, manualID)
}

func (r *EnhancedPostgreSQLCommodityRegistry) AddInvoice(ctx context.Context, commodityID, invoiceID string) error {
	return r.base.AddInvoice(ctx, commodityID, invoiceID)
}

func (r *EnhancedPostgreSQLCommodityRegistry) GetInvoices(ctx context.Context, commodityID string) ([]string, error) {
	return r.base.GetInvoices(ctx, commodityID)
}

func (r *EnhancedPostgreSQLCommodityRegistry) DeleteInvoice(ctx context.Context, commodityID, invoiceID string) error {
	return r.base.DeleteInvoice(ctx, commodityID, invoiceID)
}

// Enhanced methods with PostgreSQL-specific implementations
func (r *EnhancedPostgreSQLCommodityRegistry) SearchByTags(ctx context.Context, tags []string, operator registry.TagOperator) ([]*models.Commodity, error) {
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal tags")
	}

	var sql string
	switch operator {
	case registry.TagOperatorAND:
		sql = "SELECT * FROM commodities WHERE tags @> $1"
	case registry.TagOperatorOR:
		sql = "SELECT * FROM commodities WHERE tags && $1"
	default:
		return nil, fmt.Errorf("unsupported tag operator: %s", operator)
	}

	var commodities []*models.Commodity
	err = r.sqlxDB.SelectContext(ctx, &commodities, sql, tagsJSON)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to search by tags")
	}

	return commodities, nil
}

func (r *EnhancedPostgreSQLCommodityRegistry) FindSimilar(ctx context.Context, commodityID string, threshold float64) ([]*models.Commodity, error) {
	// PostgreSQL similarity search using trigram similarity
	sql := `
		SELECT c.*, similarity(c.name, ref.name) as sim
		FROM commodities c, commodities ref
		WHERE ref.id = $1
		AND c.id != $1
		AND similarity(c.name, ref.name) > $2
		ORDER BY sim DESC
		LIMIT 10
	`

	var commodities []*models.Commodity
	err := r.sqlxDB.SelectContext(ctx, &commodities, sql, commodityID, threshold)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to find similar commodities")
	}

	return commodities, nil
}

func (r *EnhancedPostgreSQLCommodityRegistry) FullTextSearch(ctx context.Context, query string, options ...registry.SearchOption) ([]*models.Commodity, error) {
	opts := &registry.SearchOptions{Limit: 100, Offset: 0}
	for _, opt := range options {
		opt(opts)
	}

	sql := `
		SELECT c.*, ts_rank(search_vector, plainto_tsquery($1)) as rank
		FROM commodities c
		WHERE search_vector @@ plainto_tsquery($1)
		ORDER BY rank DESC
		LIMIT $2 OFFSET $3
	`

	var commodities []*models.Commodity
	err := r.sqlxDB.SelectContext(ctx, &commodities, sql, query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to execute full-text search")
	}

	return commodities, nil
}

func (r *EnhancedPostgreSQLCommodityRegistry) AggregateByArea(ctx context.Context, groupBy []string) ([]registry.AggregationResult, error) {
	sql := `
		SELECT
			area_id,
			COUNT(*) as count,
			AVG(COALESCE(converted_original_price, original_price)) as avg_price,
			SUM(COALESCE(converted_original_price, original_price)) as total_price
		FROM commodities
		WHERE draft = false
		GROUP BY area_id
		ORDER BY count DESC
	`

	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to aggregate by area")
	}
	defer rows.Close()

	var results []registry.AggregationResult
	for rows.Next() {
		var areaID string
		var count int
		var avgPrice, totalPrice *float64

		err := rows.Scan(&areaID, &count, &avgPrice, &totalPrice)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan aggregation result")
		}

		result := registry.AggregationResult{
			GroupBy: map[string]any{"area_id": areaID},
			Count:   count,
			Avg:     make(map[string]float64),
			Sum:     make(map[string]float64),
		}

		if avgPrice != nil {
			result.Avg["price"] = *avgPrice
		}
		if totalPrice != nil {
			result.Sum["price"] = *totalPrice
		}

		results = append(results, result)
	}

	return results, nil
}

func (r *EnhancedPostgreSQLCommodityRegistry) CountByStatus(ctx context.Context) (map[string]int, error) {
	sql := "SELECT status, COUNT(*) FROM commodities GROUP BY status"

	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to count by status")
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var status string
		var count int

		err := rows.Scan(&status, &count)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan status count")
		}

		result[status] = count
	}

	return result, nil
}

func (r *EnhancedPostgreSQLCommodityRegistry) CountByType(ctx context.Context) (map[string]int, error) {
	sql := "SELECT type, COUNT(*) FROM commodities GROUP BY type"

	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to count by type")
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var commodityType string
		var count int

		err := rows.Scan(&commodityType, &count)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan type count")
		}

		result[commodityType] = count
	}

	return result, nil
}

func (r *EnhancedPostgreSQLCommodityRegistry) FindByPriceRange(ctx context.Context, minPrice, maxPrice float64, currency string) ([]*models.Commodity, error) {
	sql := `
		SELECT * FROM commodities
		WHERE COALESCE(converted_original_price, original_price) BETWEEN $1 AND $2
		AND (original_price_currency = $3 OR $3 = '')
		ORDER BY COALESCE(converted_original_price, original_price)
	`

	var commodities []*models.Commodity
	err := r.sqlxDB.SelectContext(ctx, &commodities, sql, minPrice, maxPrice, currency)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to find by price range")
	}

	return commodities, nil
}

func (r *EnhancedPostgreSQLCommodityRegistry) FindByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Commodity, error) {
	sql := `
		SELECT * FROM commodities
		WHERE purchase_date BETWEEN $1 AND $2
		ORDER BY purchase_date DESC
	`

	var commodities []*models.Commodity
	err := r.sqlxDB.SelectContext(ctx, &commodities, sql, startDate, endDate)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to find by date range")
	}

	return commodities, nil
}

func (r *EnhancedPostgreSQLCommodityRegistry) FindBySerialNumbers(ctx context.Context, serialNumbers []string) ([]*models.Commodity, error) {
	serialJSON, err := json.Marshal(serialNumbers)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal serial numbers")
	}

	sql := `
		SELECT * FROM commodities
		WHERE serial_number = ANY($1::text[])
		OR extra_serial_numbers ?| $1::text[]
		ORDER BY name
	`

	var commodities []*models.Commodity
	err = r.sqlxDB.SelectContext(ctx, &commodities, sql, serialJSON)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to find by serial numbers")
	}

	return commodities, nil
}
