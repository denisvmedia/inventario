package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.CommodityRegistry = (*CommodityRegistry)(nil)

type CommodityRegistry struct {
	dbx        *sqlx.DB
	tableNames TableNames
}

func NewCommodityRegistry(dbx *sqlx.DB) *CommodityRegistry {
	return NewCommodityRegistryWithTableNames(dbx, DefaultTableNames)
}

func NewCommodityRegistryWithTableNames(dbx *sqlx.DB, tableNames TableNames) *CommodityRegistry {
	return &CommodityRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// SetUserContext sets the user context for RLS policies
func (r *CommodityRegistry) SetUserContext(ctx context.Context, userID string) error {
	return SetUserContext(ctx, r.dbx, userID)
}

// WithUserContext executes a function with user context set
func (r *CommodityRegistry) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	return WithUserContext(ctx, r.dbx, userID, fn)
}

func (r *CommodityRegistry) Create(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the area exists
	var area models.Area
	err = ScanEntityByField(ctx, tx, r.tableNames.Areas(), "id", commodity.AreaID, &area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get area")
	}

	// Generate a new ID if one is not already provided
	if commodity.GetID() == "" {
		commodity.SetID(generateID())
	}

	//// Convert arrays to JSON
	// extraSerialNumbers, err := json.Marshal(commodity.ExtraSerialNumbers)
	// if err != nil {
	//	return nil, errkit.Wrap(err, "failed to marshal extra serial numbers")
	//}
	//
	// partNumbers, err := json.Marshal(commodity.PartNumbers)
	// if err != nil {
	//	return nil, errkit.Wrap(err, "failed to marshal part numbers")
	//}
	//
	// tags, err := json.Marshal(commodity.Tags)
	// if err != nil {
	//	return nil, errkit.Wrap(err, "failed to marshal tags")
	//}
	//
	// urls, err := json.Marshal(commodity.URLs)
	// if err != nil {
	//	return nil, errkit.Wrap(err, "failed to marshal URLs")
	//}

	err = InsertEntity(ctx, tx, r.tableNames.Commodities(), commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert entity")
	}

	return &commodity, nil
}

func (r *CommodityRegistry) Get(ctx context.Context, id string) (*models.Commodity, error) {
	//// Unmarshal JSON arrays
	// if err := json.Unmarshal(extraSerialNumbersJSON, &commodity.ExtraSerialNumbers); err != nil {
	//	return nil, errkit.Wrap(err, "failed to unmarshal extra serial numbers")
	//}
	// if err := json.Unmarshal(partNumbersJSON, &commodity.PartNumbers); err != nil {
	//	return nil, errkit.Wrap(err, "failed to unmarshal part numbers")
	//}
	// if err := json.Unmarshal(tagsJSON, &commodity.Tags); err != nil {
	//	return nil, errkit.Wrap(err, "failed to unmarshal tags")
	//}
	// if err := json.Unmarshal(urlsJSON, &commodity.URLs); err != nil {
	//	return nil, errkit.Wrap(err, "failed to unmarshal URLs")
	//}

	// Query the database for the area (atomic operation)
	return r.get(ctx, r.dbx, id)
}

func (r *CommodityRegistry) GetByName(ctx context.Context, name string) (*models.Commodity, error) {
	var commodity models.Commodity
	err := ScanEntityByField(ctx, r.dbx, r.tableNames.Commodities(), "name", name, &commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	return &commodity, nil
}

func (r *CommodityRegistry) List(ctx context.Context) ([]*models.Commodity, error) {
	var commodities []*models.Commodity

	// Query the database for all locations (atomic operation)
	for commodity, err := range ScanEntities[models.Commodity](ctx, r.dbx, r.tableNames.Commodities()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list commodities")
		}
		commodities = append(commodities, &commodity)
	}

	return commodities, nil
}

func (r *CommodityRegistry) Update(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the commodity exists
	_, err = r.get(ctx, tx, commodity.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	// Check if the area exists
	_, err = r.getArea(ctx, tx, commodity.AreaID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get area")
	}

	// TODO: what if area has changed, allow or not? (currently allowed)

	err = UpdateEntityByField(ctx, tx, r.tableNames.Commodities(), "id", commodity.ID, commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update commodity")
	}

	return &commodity, nil
}

func (r *CommodityRegistry) Delete(ctx context.Context, id string) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the commodity exists
	_, err = r.get(ctx, tx, id)
	if err != nil {
		return err
	}

	// Finally, delete the commodity
	err = DeleteEntityByField(ctx, tx, r.tableNames.Commodities(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete commodity")
	}

	return nil
}

func (r *CommodityRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := CountEntities(ctx, r.dbx, r.tableNames.Commodities())
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count commodities")
	}

	return cnt, nil
}

// File-related methods

func (r *CommodityRegistry) AddImage(ctx context.Context, commodityID, imageID string) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the commodity exists
	_, err = r.get(ctx, tx, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get commodity")
	}

	// Check if the image exists
	var image models.Image
	err = ScanEntityByField(ctx, tx, r.tableNames.Images(), "id", imageID, &image)
	if err != nil {
		return errkit.Wrap(err, "failed to get image")
	}

	// Check if the image is already associated with the commodity
	if image.CommodityID == commodityID {
		// already associated with commodity
		return nil
	}

	// Set the image's commodity ID and update it
	image.CommodityID = commodityID
	err = UpdateEntityByField(ctx, tx, r.tableNames.Images(), "id", imageID, image)
	if err != nil {
		return errkit.Wrap(err, "failed to update image")
	}

	return nil
}

func (r *CommodityRegistry) GetImages(ctx context.Context, commodityID string) ([]string, error) {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	return r.getImages(ctx, tx, commodityID)
}

func (r *CommodityRegistry) getImages(ctx context.Context, tx sqlx.ExtContext, commodityID string) ([]string, error) {
	// Check if the commodity exists
	_, err := r.get(ctx, tx, commodityID)
	if err != nil {
		return nil, err
	}

	var images []string

	for image, err := range ScanEntitiesByField[models.Image](ctx, tx, r.tableNames.Images(), "commodity_id", commodityID) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list images")
		}
		images = append(images, image.GetID())
	}

	return images, nil
}

func (r *CommodityRegistry) DeleteImage(ctx context.Context, commodityID, imageID string) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the commodity exists
	_, err = r.get(ctx, tx, commodityID)
	if err != nil {
		return err
	}

	var image models.Image
	err = ScanEntityByField(ctx, tx, r.tableNames.Images(), "id", imageID, &image)
	if err != nil {
		return errkit.Wrap(err, "failed to get image")
	}

	if image.CommodityID != commodityID {
		return errkit.Wrap(registry.ErrNotFound, "image not found or does not belong to this commodity")
	}

	err = DeleteEntityByField(ctx, tx, r.tableNames.Images(), "id", imageID)
	if err != nil {
		return errkit.Wrap(err, "failed to delete image")
	}

	return nil
}

func (r *CommodityRegistry) AddManual(ctx context.Context, commodityID, manualID string) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the commodity exists
	_, err = r.get(ctx, tx, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get commodity")
	}

	// Check if the manual exists
	var manual models.Manual
	err = ScanEntityByField(ctx, tx, r.tableNames.Manuals(), "id", manualID, &manual)
	if err != nil {
		return errkit.Wrap(err, "failed to get manual")
	}

	// Check if the manual is already associated with the commodity
	if manual.CommodityID == commodityID {
		// already associated with commodity
		return nil
	}

	// Set the manual's commodity ID and update it
	manual.CommodityID = commodityID
	err = UpdateEntityByField(ctx, tx, r.tableNames.Manuals(), "id", manualID, manual)
	if err != nil {
		return errkit.Wrap(err, "failed to update manual")
	}

	return nil
}

func (r *CommodityRegistry) GetManuals(ctx context.Context, commodityID string) ([]string, error) {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	return r.getManuals(ctx, tx, commodityID)
}

func (r *CommodityRegistry) getManuals(ctx context.Context, tx sqlx.ExtContext, commodityID string) ([]string, error) {
	// Check if the commodity exists
	_, err := r.get(ctx, tx, commodityID)
	if err != nil {
		return nil, err
	}

	var manuals []string

	for manual, err := range ScanEntitiesByField[models.Manual](ctx, tx, r.tableNames.Manuals(), "commodity_id", commodityID) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list manuals")
		}
		manuals = append(manuals, manual.GetID())
	}

	return manuals, nil
}

func (r *CommodityRegistry) DeleteManual(ctx context.Context, commodityID, manualID string) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the commodity exists
	_, err = r.get(ctx, tx, commodityID)
	if err != nil {
		return err
	}

	var manual models.Manual
	err = ScanEntityByField(ctx, tx, r.tableNames.Manuals(), "id", manualID, &manual)
	if err != nil {
		return errkit.Wrap(err, "failed to get manual")
	}

	if manual.CommodityID != commodityID {
		return errkit.Wrap(registry.ErrNotFound, "manual not found or does not belong to this commodity")
	}

	err = DeleteEntityByField(ctx, tx, r.tableNames.Manuals(), "id", manualID)
	if err != nil {
		return errkit.Wrap(err, "failed to delete manual")
	}

	return nil
}

func (r *CommodityRegistry) AddInvoice(ctx context.Context, commodityID, invoiceID string) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the commodity exists
	_, err = r.get(ctx, tx, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get commodity")
	}

	// Check if the invoice exists
	var invoice models.Invoice
	err = ScanEntityByField(ctx, tx, r.tableNames.Invoices(), "id", invoiceID, &invoice)
	if err != nil {
		return errkit.Wrap(err, "failed to get invoice")
	}

	// Check if the invoice is already associated with the commodity
	if invoice.CommodityID == commodityID {
		// already associated with commodity
		return nil
	}

	// Set the invoice's commodity ID and update it
	invoice.CommodityID = commodityID
	err = UpdateEntityByField(ctx, tx, r.tableNames.Invoices(), "id", invoiceID, invoice)
	if err != nil {
		return errkit.Wrap(err, "failed to update invoice")
	}

	return nil
}

func (r *CommodityRegistry) GetInvoices(ctx context.Context, commodityID string) ([]string, error) {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	return r.getInvoices(ctx, tx, commodityID)
}

func (r *CommodityRegistry) getInvoices(ctx context.Context, tx sqlx.ExtContext, commodityID string) ([]string, error) {
	// Check if the commodity exists
	_, err := r.get(ctx, tx, commodityID)
	if err != nil {
		return nil, err
	}

	var invoices []string

	for invoice, err := range ScanEntitiesByField[models.Invoice](ctx, tx, r.tableNames.Invoices(), "commodity_id", commodityID) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list invoices")
		}
		invoices = append(invoices, invoice.GetID())
	}

	return invoices, nil
}

func (r *CommodityRegistry) DeleteInvoice(ctx context.Context, commodityID, invoiceID string) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the commodity exists
	_, err = r.get(ctx, tx, commodityID)
	if err != nil {
		return err
	}

	var invoice models.Invoice
	err = ScanEntityByField(ctx, tx, r.tableNames.Invoices(), "id", invoiceID, &invoice)
	if err != nil {
		return errkit.Wrap(err, "failed to get invoice")
	}

	if invoice.CommodityID != commodityID {
		return errkit.Wrap(registry.ErrNotFound, "invoice not found or does not belong to this commodity")
	}

	err = DeleteEntityByField(ctx, tx, r.tableNames.Invoices(), "id", invoiceID)
	if err != nil {
		return errkit.Wrap(err, "failed to delete invoice")
	}

	return nil
}

func (r *CommodityRegistry) get(ctx context.Context, tx sqlx.ExtContext, id string) (*models.Commodity, error) {
	var commodity models.Commodity
	err := ScanEntityByField(ctx, tx, r.tableNames.Commodities(), "id", id, &commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	return &commodity, nil
}

func (r *CommodityRegistry) getArea(ctx context.Context, tx sqlx.ExtContext, areaID string) (*models.Area, error) {
	var area models.Area
	err := ScanEntityByField(ctx, tx, r.tableNames.Areas(), "id", areaID, &area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get area")
	}

	return &area, nil
}

// User-aware methods that automatically use user context from the request context

// CreateWithUser creates a commodity with user context
func (r *CommodityRegistry) CreateWithUser(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user_id on the commodity
	commodity.SetUserID(userID)

	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Set user context for RLS
	err = SetUserContext(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	// Check if the area exists
	var area models.Area
	err = ScanEntityByField(ctx, tx, r.tableNames.Areas(), "id", commodity.AreaID, &area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get area")
	}

	// Generate a new ID if one is not already provided
	if commodity.GetID() == "" {
		commodity.SetID(generateID())
	}

	err = InsertEntity(ctx, tx, r.tableNames.Commodities(), commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert entity")
	}

	return &commodity, nil
}

// GetWithUser gets a commodity with user context
func (r *CommodityRegistry) GetWithUser(ctx context.Context, id string) (*models.Commodity, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user context for RLS
	err := SetUserContext(ctx, r.dbx, userID)
	if err != nil {
		return nil, err
	}

	return r.get(ctx, r.dbx, id)
}

// ListWithUser lists commodities with user context
func (r *CommodityRegistry) ListWithUser(ctx context.Context) ([]*models.Commodity, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user context for RLS
	err := SetUserContext(ctx, r.dbx, userID)
	if err != nil {
		return nil, err
	}

	var commodities []*models.Commodity

	// Query the database for all commodities (atomic operation)
	for commodity, err := range ScanEntities[models.Commodity](ctx, r.dbx, r.tableNames.Commodities()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list commodities")
		}
		commodities = append(commodities, &commodity)
	}

	return commodities, nil
}

// UpdateWithUser updates a commodity with user context
func (r *CommodityRegistry) UpdateWithUser(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user_id on the commodity
	commodity.SetUserID(userID)

	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Set user context for RLS
	err = SetUserContext(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	// Check if the commodity exists
	_, err = r.get(ctx, tx, commodity.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	// Check if the area exists
	_, err = r.getArea(ctx, tx, commodity.AreaID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get area")
	}

	err = UpdateEntityByField(ctx, tx, r.tableNames.Commodities(), "id", commodity.ID, commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update entity")
	}

	return &commodity, nil
}

// DeleteWithUser deletes a commodity with user context
func (r *CommodityRegistry) DeleteWithUser(ctx context.Context, id string) error {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Set user context for RLS
	err = SetUserContext(ctx, tx, userID)
	if err != nil {
		return err
	}

	// Check if the commodity exists
	_, err = r.get(ctx, tx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get commodity")
	}

	err = DeleteEntityByField(ctx, tx, r.tableNames.Commodities(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete entity")
	}

	return nil
}

// CountWithUser counts commodities with user context
func (r *CommodityRegistry) CountWithUser(ctx context.Context) (int, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return 0, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user context for RLS
	err := SetUserContext(ctx, r.dbx, userID)
	if err != nil {
		return 0, err
	}

	return CountEntities(ctx, r.dbx, r.tableNames.Commodities())
}

// Enhanced methods with PostgreSQL-specific implementations

// SearchByTags searches commodities by tags using PostgreSQL JSONB operators
func (r *CommodityRegistry) SearchByTags(ctx context.Context, tags []string, operator registry.TagOperator) ([]*models.Commodity, error) {
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal tags")
	}

	var sql string
	switch operator {
	case registry.TagOperatorAND:
		sql = "SELECT * FROM " + r.tableNames.Commodities() + " WHERE tags @> $1"
	case registry.TagOperatorOR:
		sql = "SELECT * FROM " + r.tableNames.Commodities() + " WHERE tags && $1"
	default:
		return nil, fmt.Errorf("unsupported tag operator: %s", operator)
	}

	var commodities []*models.Commodity
	err = r.dbx.SelectContext(ctx, &commodities, sql, tagsJSON)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to search by tags")
	}

	return commodities, nil
}

// FindSimilar finds similar commodities using PostgreSQL trigram similarity
func (r *CommodityRegistry) FindSimilar(ctx context.Context, commodityID string, threshold float64) ([]*models.Commodity, error) {
	sql := fmt.Sprintf(`
		SELECT c.*, similarity(c.name, ref.name) as sim
		FROM %s c, %s ref
		WHERE ref.id = $1
		AND c.id != $1
		AND similarity(c.name, ref.name) > $2
		ORDER BY sim DESC
		LIMIT 10
	`, r.tableNames.Commodities(), r.tableNames.Commodities())

	var commodities []*models.Commodity
	err := r.dbx.SelectContext(ctx, &commodities, sql, commodityID, threshold)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to find similar commodities")
	}

	return commodities, nil
}

// FullTextSearch performs PostgreSQL full-text search on commodities
func (r *CommodityRegistry) FullTextSearch(ctx context.Context, query string, options ...registry.SearchOption) ([]*models.Commodity, error) {
	opts := &registry.SearchOptions{Limit: 100, Offset: 0}
	for _, opt := range options {
		opt(opts)
	}

	sql := fmt.Sprintf(`
		SELECT c.*, ts_rank(search_vector, plainto_tsquery($1)) as rank
		FROM %s c
		WHERE search_vector @@ plainto_tsquery($1)
		ORDER BY rank DESC
		LIMIT $2 OFFSET $3
	`, r.tableNames.Commodities())

	var commodities []*models.Commodity
	err := r.dbx.SelectContext(ctx, &commodities, sql, query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to execute full-text search")
	}

	return commodities, nil
}

// AggregateByArea aggregates commodities by area
func (r *CommodityRegistry) AggregateByArea(ctx context.Context, groupBy []string) ([]registry.AggregationResult, error) {
	sql := fmt.Sprintf(`
		SELECT
			area_id,
			COUNT(*) as count,
			AVG(COALESCE(converted_original_price, original_price)) as avg_price,
			SUM(COALESCE(converted_original_price, original_price)) as total_price
		FROM %s
		WHERE draft = false
		GROUP BY area_id
		ORDER BY count DESC
	`, r.tableNames.Commodities())

	rows, err := r.dbx.QueryContext(ctx, sql)
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

// CountByStatus counts commodities by status
func (r *CommodityRegistry) CountByStatus(ctx context.Context) (map[string]int, error) {
	sql := "SELECT status, COUNT(*) FROM " + r.tableNames.Commodities() + " GROUP BY status"

	rows, err := r.dbx.QueryContext(ctx, sql)
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

// CountByType counts commodities by type
func (r *CommodityRegistry) CountByType(ctx context.Context) (map[string]int, error) {
	sql := "SELECT type, COUNT(*) FROM " + r.tableNames.Commodities() + " GROUP BY type"

	rows, err := r.dbx.QueryContext(ctx, sql)
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

// FindByPriceRange finds commodities within a price range
func (r *CommodityRegistry) FindByPriceRange(ctx context.Context, minPrice, maxPrice float64, currency string) ([]*models.Commodity, error) {
	sql := fmt.Sprintf(`
		SELECT * FROM %s
		WHERE COALESCE(converted_original_price, original_price) BETWEEN $1 AND $2
		AND (original_price_currency = $3 OR $3 = '')
		ORDER BY COALESCE(converted_original_price, original_price)
	`, r.tableNames.Commodities())

	var commodities []*models.Commodity
	err := r.dbx.SelectContext(ctx, &commodities, sql, minPrice, maxPrice, currency)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to find by price range")
	}

	return commodities, nil
}

// FindByDateRange finds commodities within a date range
func (r *CommodityRegistry) FindByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Commodity, error) {
	sql := fmt.Sprintf(`
		SELECT * FROM %s
		WHERE purchase_date BETWEEN $1 AND $2
		ORDER BY purchase_date DESC
	`, r.tableNames.Commodities())

	var commodities []*models.Commodity
	err := r.dbx.SelectContext(ctx, &commodities, sql, startDate, endDate)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to find by date range")
	}

	return commodities, nil
}

// FindBySerialNumbers finds commodities by serial numbers
func (r *CommodityRegistry) FindBySerialNumbers(ctx context.Context, serialNumbers []string) ([]*models.Commodity, error) {
	serialJSON, err := json.Marshal(serialNumbers)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal serial numbers")
	}

	sql := fmt.Sprintf(`
		SELECT * FROM %s
		WHERE serial_number = ANY($1::text[])
		OR extra_serial_numbers ?| $1::text[]
		ORDER BY name
	`, r.tableNames.Commodities())

	var commodities []*models.Commodity
	err = r.dbx.SelectContext(ctx, &commodities, sql, serialJSON)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to find by serial numbers")
	}

	return commodities, nil
}
