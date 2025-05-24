package postgresql

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.CommodityRegistry = (*CommodityRegistry)(nil)

type CommodityRegistry struct {
	pool         *pgxpool.Pool
	areaRegistry registry.AreaRegistry
}

func NewCommodityRegistry(pool *pgxpool.Pool, areaRegistry registry.AreaRegistry) *CommodityRegistry {
	return &CommodityRegistry{
		pool:         pool,
		areaRegistry: areaRegistry,
	}
}

func (r *CommodityRegistry) Create(commodity models.Commodity) (*models.Commodity, error) {
	ctx := context.Background()

	// Validate the commodity
	err := commodity.ValidateWithContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Check if the area exists
	_, err = r.areaRegistry.Get(commodity.AreaID)
	if err != nil {
		return nil, errkit.Wrap(err, "area not found")
	}

	// Generate a new ID
	if commodity.ID == "" {
		commodity.SetID(generateID())
	}

	// Convert arrays to JSON
	extraSerialNumbers, err := json.Marshal(commodity.ExtraSerialNumbers)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal extra serial numbers")
	}

	partNumbers, err := json.Marshal(commodity.PartNumbers)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal part numbers")
	}

	tags, err := json.Marshal(commodity.Tags)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal tags")
	}

	urls, err := json.Marshal(commodity.URLs)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal URLs")
	}

	// Insert the commodity into the database
	_, err = r.pool.Exec(ctx, `
		INSERT INTO commodities (
			id, name, short_name, type, area_id, count, 
			original_price, original_price_currency, converted_original_price, current_price,
			serial_number, extra_serial_numbers, part_numbers, tags, status,
			purchase_date, registered_date, last_modified_date, urls, comments, draft
		) VALUES (
			$1, $2, $3, $4, $5, $6, 
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21
		)
	`, 
		commodity.ID, commodity.Name, commodity.ShortName, commodity.Type, commodity.AreaID, commodity.Count,
		commodity.OriginalPrice, commodity.OriginalPriceCurrency, commodity.ConvertedOriginalPrice, commodity.CurrentPrice,
		commodity.SerialNumber, extraSerialNumbers, partNumbers, tags, commodity.Status,
		commodity.PurchaseDate, commodity.RegisteredDate, commodity.LastModifiedDate, urls, commodity.Comments, commodity.Draft,
	)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create commodity")
	}

	// Add the commodity to the area
	err = r.areaRegistry.AddCommodity(commodity.AreaID, commodity.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to add commodity to area")
	}

	return &commodity, nil
}

func (r *CommodityRegistry) Get(id string) (*models.Commodity, error) {
	ctx := context.Background()
	var commodity models.Commodity
	var extraSerialNumbersJSON, partNumbersJSON, tagsJSON, urlsJSON []byte

	// Query the database for the commodity
	err := r.pool.QueryRow(ctx, `
		SELECT 
			id, name, short_name, type, area_id, count, 
			original_price, original_price_currency, converted_original_price, current_price,
			serial_number, extra_serial_numbers, part_numbers, tags, status,
			purchase_date, registered_date, last_modified_date, urls, comments, draft
		FROM commodities
		WHERE id = $1
	`, id).Scan(
		&commodity.ID, &commodity.Name, &commodity.ShortName, &commodity.Type, &commodity.AreaID, &commodity.Count,
		&commodity.OriginalPrice, &commodity.OriginalPriceCurrency, &commodity.ConvertedOriginalPrice, &commodity.CurrentPrice,
		&commodity.SerialNumber, &extraSerialNumbersJSON, &partNumbersJSON, &tagsJSON, &commodity.Status,
		&commodity.PurchaseDate, &commodity.RegisteredDate, &commodity.LastModifiedDate, &urlsJSON, &commodity.Comments, &commodity.Draft,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errkit.Wrap(registry.ErrNotFound, "commodity not found")
		}
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	// Unmarshal JSON arrays
	if err := json.Unmarshal(extraSerialNumbersJSON, &commodity.ExtraSerialNumbers); err != nil {
		return nil, errkit.Wrap(err, "failed to unmarshal extra serial numbers")
	}
	if err := json.Unmarshal(partNumbersJSON, &commodity.PartNumbers); err != nil {
		return nil, errkit.Wrap(err, "failed to unmarshal part numbers")
	}
	if err := json.Unmarshal(tagsJSON, &commodity.Tags); err != nil {
		return nil, errkit.Wrap(err, "failed to unmarshal tags")
	}
	if err := json.Unmarshal(urlsJSON, &commodity.URLs); err != nil {
		return nil, errkit.Wrap(err, "failed to unmarshal URLs")
	}

	return &commodity, nil
}

func (r *CommodityRegistry) List() ([]*models.Commodity, error) {
	ctx := context.Background()
	var commodities []*models.Commodity

	// Query the database for all commodities
	rows, err := r.pool.Query(ctx, `
		SELECT 
			id, name, short_name, type, area_id, count, 
			original_price, original_price_currency, converted_original_price, current_price,
			serial_number, extra_serial_numbers, part_numbers, tags, status,
			purchase_date, registered_date, last_modified_date, urls, comments, draft
		FROM commodities
	`)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list commodities")
	}
	defer rows.Close()

	// Process each row
	for rows.Next() {
		var commodity models.Commodity
		var extraSerialNumbersJSON, partNumbersJSON, tagsJSON, urlsJSON []byte

		if err := rows.Scan(
			&commodity.ID, &commodity.Name, &commodity.ShortName, &commodity.Type, &commodity.AreaID, &commodity.Count,
			&commodity.OriginalPrice, &commodity.OriginalPriceCurrency, &commodity.ConvertedOriginalPrice, &commodity.CurrentPrice,
			&commodity.SerialNumber, &extraSerialNumbersJSON, &partNumbersJSON, &tagsJSON, &commodity.Status,
			&commodity.PurchaseDate, &commodity.RegisteredDate, &commodity.LastModifiedDate, &urlsJSON, &commodity.Comments, &commodity.Draft,
		); err != nil {
			return nil, errkit.Wrap(err, "failed to scan row")
		}

		// Unmarshal JSON arrays
		if err := json.Unmarshal(extraSerialNumbersJSON, &commodity.ExtraSerialNumbers); err != nil {
			return nil, errkit.Wrap(err, "failed to unmarshal extra serial numbers")
		}
		if err := json.Unmarshal(partNumbersJSON, &commodity.PartNumbers); err != nil {
			return nil, errkit.Wrap(err, "failed to unmarshal part numbers")
		}
		if err := json.Unmarshal(tagsJSON, &commodity.Tags); err != nil {
			return nil, errkit.Wrap(err, "failed to unmarshal tags")
		}
		if err := json.Unmarshal(urlsJSON, &commodity.URLs); err != nil {
			return nil, errkit.Wrap(err, "failed to unmarshal URLs")
		}

		commodities = append(commodities, &commodity)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating rows")
	}

	return commodities, nil
}

func (r *CommodityRegistry) Update(commodity models.Commodity) (*models.Commodity, error) {
	ctx := context.Background()

	// Validate the commodity
	err := commodity.ValidateWithContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Check if the commodity exists
	existingCommodity, err := r.Get(commodity.ID)
	if err != nil {
		return nil, err
	}

	// Check if the area exists
	_, err = r.areaRegistry.Get(commodity.AreaID)
	if err != nil {
		return nil, errkit.Wrap(err, "area not found")
	}

	// Convert arrays to JSON
	extraSerialNumbers, err := json.Marshal(commodity.ExtraSerialNumbers)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal extra serial numbers")
	}

	partNumbers, err := json.Marshal(commodity.PartNumbers)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal part numbers")
	}

	tags, err := json.Marshal(commodity.Tags)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal tags")
	}

	urls, err := json.Marshal(commodity.URLs)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal URLs")
	}

	// Begin a transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	// If the area ID has changed, update the area references
	if existingCommodity.AreaID != commodity.AreaID {
		// Remove the commodity from the old area
		err = r.areaRegistry.DeleteCommodity(existingCommodity.AreaID, commodity.ID)
		if err != nil {
			return nil, err
		}

		// Add the commodity to the new area
		err = r.areaRegistry.AddCommodity(commodity.AreaID, commodity.ID)
		if err != nil {
			return nil, err
		}
	}

	// Update the commodity in the database
	_, err = tx.Exec(ctx, `
		UPDATE commodities
		SET 
			name = $1, short_name = $2, type = $3, area_id = $4, count = $5, 
			original_price = $6, original_price_currency = $7, converted_original_price = $8, current_price = $9,
			serial_number = $10, extra_serial_numbers = $11, part_numbers = $12, tags = $13, status = $14,
			purchase_date = $15, registered_date = $16, last_modified_date = $17, urls = $18, comments = $19, draft = $20
		WHERE id = $21
	`, 
		commodity.Name, commodity.ShortName, commodity.Type, commodity.AreaID, commodity.Count,
		commodity.OriginalPrice, commodity.OriginalPriceCurrency, commodity.ConvertedOriginalPrice, commodity.CurrentPrice,
		commodity.SerialNumber, extraSerialNumbers, partNumbers, tags, commodity.Status,
		commodity.PurchaseDate, commodity.RegisteredDate, commodity.LastModifiedDate, urls, commodity.Comments, commodity.Draft,
		commodity.ID,
	)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update commodity")
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to commit transaction")
	}

	return &commodity, nil
}

func (r *CommodityRegistry) Delete(id string) error {
	ctx := context.Background()

	// Check if the commodity exists
	commodity, err := r.Get(id)
	if err != nil {
		return err
	}

	// Begin a transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	// Delete the commodity from the database
	_, err = tx.Exec(ctx, `
		DELETE FROM commodities
		WHERE id = $1
	`, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete commodity")
	}

	// Remove the commodity from the area
	err = r.areaRegistry.DeleteCommodity(commodity.AreaID, id)
	if err != nil {
		return err
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to commit transaction")
	}

	return nil
}

func (r *CommodityRegistry) Count() (int, error) {
	ctx := context.Background()
	var count int

	// Query the database for the count
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM commodities
	`).Scan(&count)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count commodities")
	}

	return count, nil
}

// File-related methods

func (r *CommodityRegistry) AddImage(commodityID, imageID string) error {
	ctx := context.Background()

	// Check if the commodity exists
	_, err := r.Get(commodityID)
	if err != nil {
		return err
	}

	// Check if the image exists and has the correct commodity ID
	var count int
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM images
		WHERE id = $1 AND commodity_id = $2
	`, imageID, commodityID).Scan(&count)
	if err != nil {
		return errkit.Wrap(err, "failed to check image")
	}
	if count == 0 {
		return errkit.Wrap(registry.ErrNotFound, "image not found or does not belong to this commodity")
	}

	return nil
}

func (r *CommodityRegistry) GetImages(commodityID string) ([]string, error) {
	ctx := context.Background()
	var images []string

	// Check if the commodity exists
	_, err := r.Get(commodityID)
	if err != nil {
		return nil, err
	}

	// Query the database for all images for the commodity
	rows, err := r.pool.Query(ctx, `
		SELECT id
		FROM images
		WHERE commodity_id = $1
	`, commodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list images")
	}
	defer rows.Close()

	// Process each row
	for rows.Next() {
		var imageID string
		if err := rows.Scan(&imageID); err != nil {
			return nil, errkit.Wrap(err, "failed to scan row")
		}
		images = append(images, imageID)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating rows")
	}

	return images, nil
}

func (r *CommodityRegistry) DeleteImage(commodityID, imageID string) error {
	ctx := context.Background()

	// Check if the commodity exists
	_, err := r.Get(commodityID)
	if err != nil {
		return err
	}

	// Check if the image exists and has the correct commodity ID
	var count int
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM images
		WHERE id = $1 AND commodity_id = $2
	`, imageID, commodityID).Scan(&count)
	if err != nil {
		return errkit.Wrap(err, "failed to check image")
	}
	if count == 0 {
		return errkit.Wrap(registry.ErrNotFound, "image not found or does not belong to this commodity")
	}

	// Delete the image from the database
	_, err = r.pool.Exec(ctx, `
		DELETE FROM images
		WHERE id = $1 AND commodity_id = $2
	`, imageID, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to delete image")
	}

	return nil
}

// Similar implementations for manuals and invoices

func (r *CommodityRegistry) AddManual(commodityID, manualID string) error {
	ctx := context.Background()

	// Check if the commodity exists
	_, err := r.Get(commodityID)
	if err != nil {
		return err
	}

	// Check if the manual exists and has the correct commodity ID
	var count int
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM manuals
		WHERE id = $1 AND commodity_id = $2
	`, manualID, commodityID).Scan(&count)
	if err != nil {
		return errkit.Wrap(err, "failed to check manual")
	}
	if count == 0 {
		return errkit.Wrap(registry.ErrNotFound, "manual not found or does not belong to this commodity")
	}

	return nil
}

func (r *CommodityRegistry) GetManuals(commodityID string) ([]string, error) {
	ctx := context.Background()
	var manuals []string

	// Check if the commodity exists
	_, err := r.Get(commodityID)
	if err != nil {
		return nil, err
	}

	// Query the database for all manuals for the commodity
	rows, err := r.pool.Query(ctx, `
		SELECT id
		FROM manuals
		WHERE commodity_id = $1
	`, commodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list manuals")
	}
	defer rows.Close()

	// Process each row
	for rows.Next() {
		var manualID string
		if err := rows.Scan(&manualID); err != nil {
			return nil, errkit.Wrap(err, "failed to scan row")
		}
		manuals = append(manuals, manualID)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating rows")
	}

	return manuals, nil
}

func (r *CommodityRegistry) DeleteManual(commodityID, manualID string) error {
	ctx := context.Background()

	// Check if the commodity exists
	_, err := r.Get(commodityID)
	if err != nil {
		return err
	}

	// Check if the manual exists and has the correct commodity ID
	var count int
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM manuals
		WHERE id = $1 AND commodity_id = $2
	`, manualID, commodityID).Scan(&count)
	if err != nil {
		return errkit.Wrap(err, "failed to check manual")
	}
	if count == 0 {
		return errkit.Wrap(registry.ErrNotFound, "manual not found or does not belong to this commodity")
	}

	// Delete the manual from the database
	_, err = r.pool.Exec(ctx, `
		DELETE FROM manuals
		WHERE id = $1 AND commodity_id = $2
	`, manualID, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to delete manual")
	}

	return nil
}

func (r *CommodityRegistry) AddInvoice(commodityID, invoiceID string) error {
	ctx := context.Background()

	// Check if the commodity exists
	_, err := r.Get(commodityID)
	if err != nil {
		return err
	}

	// Check if the invoice exists and has the correct commodity ID
	var count int
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM invoices
		WHERE id = $1 AND commodity_id = $2
	`, invoiceID, commodityID).Scan(&count)
	if err != nil {
		return errkit.Wrap(err, "failed to check invoice")
	}
	if count == 0 {
		return errkit.Wrap(registry.ErrNotFound, "invoice not found or does not belong to this commodity")
	}

	return nil
}

func (r *CommodityRegistry) GetInvoices(commodityID string) ([]string, error) {
	ctx := context.Background()
	var invoices []string

	// Check if the commodity exists
	_, err := r.Get(commodityID)
	if err != nil {
		return nil, err
	}

	// Query the database for all invoices for the commodity
	rows, err := r.pool.Query(ctx, `
		SELECT id
		FROM invoices
		WHERE commodity_id = $1
	`, commodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list invoices")
	}
	defer rows.Close()

	// Process each row
	for rows.Next() {
		var invoiceID string
		if err := rows.Scan(&invoiceID); err != nil {
			return nil, errkit.Wrap(err, "failed to scan row")
		}
		invoices = append(invoices, invoiceID)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating rows")
	}

	return invoices, nil
}

func (r *CommodityRegistry) DeleteInvoice(commodityID, invoiceID string) error {
	ctx := context.Background()

	// Check if the commodity exists
	_, err := r.Get(commodityID)
	if err != nil {
		return err
	}

	// Check if the invoice exists and has the correct commodity ID
	var count int
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM invoices
		WHERE id = $1 AND commodity_id = $2
	`, invoiceID, commodityID).Scan(&count)
	if err != nil {
		return errkit.Wrap(err, "failed to check invoice")
	}
	if count == 0 {
		return errkit.Wrap(registry.ErrNotFound, "invoice not found or does not belong to this commodity")
	}

	// Delete the invoice from the database
	_, err = r.pool.Exec(ctx, `
		DELETE FROM invoices
		WHERE id = $1 AND commodity_id = $2
	`, invoiceID, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to delete invoice")
	}

	return nil
}
