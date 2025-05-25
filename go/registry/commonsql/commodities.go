package commonsql

import (
	"context"
	"errors"

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

	// Generate a new ID
	commodity.SetID(generateID())

	//// Convert arrays to JSON
	//extraSerialNumbers, err := json.Marshal(commodity.ExtraSerialNumbers)
	//if err != nil {
	//	return nil, errkit.Wrap(err, "failed to marshal extra serial numbers")
	//}
	//
	//partNumbers, err := json.Marshal(commodity.PartNumbers)
	//if err != nil {
	//	return nil, errkit.Wrap(err, "failed to marshal part numbers")
	//}
	//
	//tags, err := json.Marshal(commodity.Tags)
	//if err != nil {
	//	return nil, errkit.Wrap(err, "failed to marshal tags")
	//}
	//
	//urls, err := json.Marshal(commodity.URLs)
	//if err != nil {
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
	//if err := json.Unmarshal(extraSerialNumbersJSON, &commodity.ExtraSerialNumbers); err != nil {
	//	return nil, errkit.Wrap(err, "failed to unmarshal extra serial numbers")
	//}
	//if err := json.Unmarshal(partNumbersJSON, &commodity.PartNumbers); err != nil {
	//	return nil, errkit.Wrap(err, "failed to unmarshal part numbers")
	//}
	//if err := json.Unmarshal(tagsJSON, &commodity.Tags); err != nil {
	//	return nil, errkit.Wrap(err, "failed to unmarshal tags")
	//}
	//if err := json.Unmarshal(urlsJSON, &commodity.URLs); err != nil {
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
