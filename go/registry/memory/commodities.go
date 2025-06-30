package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.CommodityRegistry = (*CommodityRegistry)(nil)

type baseCommodityRegistry = Registry[models.Commodity, *models.Commodity]
type CommodityRegistry struct {
	*baseCommodityRegistry

	areaRegistry registry.AreaRegistry
	fileRegistry registry.FileRegistry
	imagesLock   sync.RWMutex
	images       models.CommodityImages
	manualsLock  sync.RWMutex
	manuals      models.CommodityManuals
	invoicesLock sync.RWMutex
	invoices     models.CommodityInvoices
}

func NewCommodityRegistry(areaRegistry registry.AreaRegistry, fileRegistry registry.FileRegistry) *CommodityRegistry {
	return &CommodityRegistry{
		baseCommodityRegistry: NewRegistry[models.Commodity, *models.Commodity](),
		areaRegistry:          areaRegistry,
		fileRegistry:          fileRegistry,
		images:                make(models.CommodityImages),
		manuals:               make(models.CommodityManuals),
		invoices:              make(models.CommodityInvoices),
	}
}

func (r *CommodityRegistry) Create(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	_, err := r.areaRegistry.Get(ctx, commodity.AreaID)
	if err != nil {
		return nil, errkit.Wrap(err, "area not found")
	}

	newCommodity, err := r.baseCommodityRegistry.Create(ctx, commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create commodity")
	}

	err = r.areaRegistry.AddCommodity(ctx, commodity.AreaID, newCommodity.ID)

	return newCommodity, err
}

func (r *CommodityRegistry) Delete(ctx context.Context, id string) error {
	commodity, err := r.baseCommodityRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get commodity")
	}

	err = r.baseCommodityRegistry.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete commodity")
	}

	err = r.areaRegistry.DeleteCommodity(ctx, commodity.AreaID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete commodity from area")
	}

	return nil
}

// DeleteRecursive deletes a commodity and all its linked files recursively
func (r *CommodityRegistry) DeleteRecursive(ctx context.Context, id string) error {
	// First, get all linked files for this commodity
	if r.fileRegistry != nil {
		files, err := r.fileRegistry.ListByLinkedEntity(ctx, "commodity", id)
		if err != nil {
			return errkit.Wrap(err, "failed to get linked files")
		}

		// Delete all linked files
		for _, file := range files {
			err = r.fileRegistry.Delete(ctx, file.ID)
			if err != nil {
				return errkit.Wrap(err, fmt.Sprintf("failed to delete linked file %s", file.ID))
			}
		}
	}

	// Then delete the commodity itself
	return r.Delete(ctx, id)
}

func (r *CommodityRegistry) AddImage(_ context.Context, commodityID, imageID string) error {
	r.imagesLock.Lock()
	r.images[commodityID] = append(r.images[commodityID], imageID)
	r.imagesLock.Unlock()

	return nil
}

func (r *CommodityRegistry) GetImages(_ context.Context, commodityID string) ([]string, error) {
	r.imagesLock.RLock()
	images := make([]string, len(r.images[commodityID]))
	copy(images, r.images[commodityID])
	r.imagesLock.RUnlock()

	return images, nil
}

func (r *CommodityRegistry) DeleteImage(_ context.Context, commodityID, imageID string) error {
	r.imagesLock.Lock()
	for i, foundImageID := range r.images[commodityID] {
		if foundImageID == imageID {
			r.images[commodityID] = append(r.images[commodityID][:i], r.images[commodityID][i+1:]...)
			break
		}
	}
	r.imagesLock.Unlock()

	return nil
}

func (r *CommodityRegistry) AddManual(_ context.Context, commodityID, manualID string) error {
	r.manualsLock.Lock()
	r.manuals[commodityID] = append(r.manuals[commodityID], manualID)
	r.manualsLock.Unlock()

	return nil
}

func (r *CommodityRegistry) GetManuals(_ context.Context, commodityID string) ([]string, error) {
	r.manualsLock.RLock()
	manuals := make([]string, len(r.manuals[commodityID]))
	copy(manuals, r.manuals[commodityID])
	r.manualsLock.RUnlock()

	return manuals, nil
}

func (r *CommodityRegistry) DeleteManual(_ context.Context, commodityID, manualID string) error {
	r.manualsLock.Lock()
	for i, foundManualID := range r.manuals[commodityID] {
		if foundManualID == manualID {
			r.manuals[commodityID] = append(r.manuals[commodityID][:i], r.manuals[commodityID][i+1:]...)
			break
		}
	}
	r.manualsLock.Unlock()

	return nil
}

func (r *CommodityRegistry) AddInvoice(_ context.Context, commodityID, invoiceID string) error {
	r.invoicesLock.Lock()
	r.invoices[commodityID] = append(r.invoices[commodityID], invoiceID)
	r.invoicesLock.Unlock()

	return nil
}

func (r *CommodityRegistry) GetInvoices(_ context.Context, commodityID string) ([]string, error) {
	r.invoicesLock.RLock()
	invoices := make([]string, len(r.invoices[commodityID]))
	copy(invoices, r.invoices[commodityID])
	r.invoicesLock.RUnlock()

	return invoices, nil
}

func (r *CommodityRegistry) DeleteInvoice(_ context.Context, commodityID, invoiceID string) error {
	r.invoicesLock.Lock()
	for i, foundInvoiceID := range r.invoices[commodityID] {
		if foundInvoiceID == invoiceID {
			r.invoices[commodityID] = append(r.invoices[commodityID][:i], r.invoices[commodityID][i+1:]...)
			break
		}
	}
	r.invoicesLock.Unlock()

	return nil
}

func (r *CommodityRegistry) Update(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	// Call the base registry's Update method
	updatedCommodity, err := r.baseCommodityRegistry.Update(ctx, commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update commodity")
	}

	return updatedCommodity, nil
}
