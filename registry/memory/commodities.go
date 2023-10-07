package memory

import (
	"sync"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.CommodityRegistry = (*CommodityRegistry)(nil)

type baseCommodityRegistry = Registry[models.Commodity, *models.Commodity]
type CommodityRegistry struct {
	*baseCommodityRegistry

	areaRegistry registry.AreaRegistry
	imagesLock   sync.RWMutex
	images       models.CommodityImages
	manualsLock  sync.RWMutex
	manuals      models.CommodityManuals
	invoicesLock sync.RWMutex
	invoices     models.CommodityInvoices
}

func NewCommodityRegistry(areaRegistry registry.AreaRegistry) *CommodityRegistry {
	return &CommodityRegistry{
		baseCommodityRegistry: NewRegistry[models.Commodity, *models.Commodity](),
		areaRegistry:          areaRegistry,
		images:                make(models.CommodityImages),
		manuals:               make(models.CommodityManuals),
		invoices:              make(models.CommodityInvoices),
	}
}

func (r *CommodityRegistry) Create(commodity models.Commodity) (*models.Commodity, error) {
	err := validation.Validate(&commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	_, err = r.areaRegistry.Get(commodity.AreaID)
	if err != nil {
		return nil, errkit.Wrap(err, "area not found")
	}

	newCommodity, err := r.baseCommodityRegistry.Create(commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create commodity")
	}

	r.areaRegistry.AddCommodity(commodity.AreaID, newCommodity.ID)

	return newCommodity, err
}

func (r *CommodityRegistry) Delete(id string) error {
	commodity, err := r.baseCommodityRegistry.Get(id)
	if err != nil {
		return err
	}

	err = r.baseCommodityRegistry.Delete(id)
	if err != nil {
		return err
	}

	r.areaRegistry.DeleteCommodity(commodity.AreaID, id)

	return nil
}

func (r *CommodityRegistry) AddImage(commodityID, imageID string) {
	r.imagesLock.Lock()
	r.images[commodityID] = append(r.images[commodityID], imageID)
	r.imagesLock.Unlock()
}

func (r *CommodityRegistry) GetImages(commodityID string) []string {
	r.imagesLock.RLock()
	images := make([]string, len(r.images[commodityID]))
	copy(images, r.images[commodityID])
	r.imagesLock.RUnlock()

	return images
}

func (r *CommodityRegistry) DeleteImage(commodityID, imageID string) {
	r.imagesLock.Lock()
	for i, foundImageID := range r.images[commodityID] {
		if foundImageID == imageID {
			r.images[commodityID] = append(r.images[commodityID][:i], r.images[commodityID][i+1:]...)
			break
		}
	}
	r.imagesLock.Unlock()
}

func (r *CommodityRegistry) AddManual(commodityID, manualID string) {
	r.manualsLock.Lock()
	r.manuals[commodityID] = append(r.manuals[commodityID], manualID)
	r.manualsLock.Unlock()
}

func (r *CommodityRegistry) GetManuals(commodityID string) []string {
	r.manualsLock.RLock()
	manuals := make([]string, len(r.manuals[commodityID]))
	copy(manuals, r.manuals[commodityID])
	r.manualsLock.RUnlock()

	return manuals
}

func (r *CommodityRegistry) DeleteManual(commodityID, manualID string) {
	r.manualsLock.Lock()
	for i, foundManualID := range r.manuals[commodityID] {
		if foundManualID == manualID {
			r.manuals[commodityID] = append(r.manuals[commodityID][:i], r.manuals[commodityID][i+1:]...)
			break
		}
	}
	r.manualsLock.Unlock()
}

func (r *CommodityRegistry) AddInvoice(commodityID, invoiceID string) {
	r.invoicesLock.Lock()
	r.invoices[commodityID] = append(r.invoices[commodityID], invoiceID)
	r.invoicesLock.Unlock()
}

func (r *CommodityRegistry) GetInvoices(commodityID string) []string {
	r.invoicesLock.RLock()
	invoices := make([]string, len(r.invoices[commodityID]))
	copy(invoices, r.invoices[commodityID])
	r.invoicesLock.RUnlock()

	return invoices
}

func (r *CommodityRegistry) DeleteInvoice(commodityID, invoiceID string) {
	r.invoicesLock.Lock()
	for i, foundInvoiceID := range r.invoices[commodityID] {
		if foundInvoiceID == invoiceID {
			r.invoices[commodityID] = append(r.invoices[commodityID][:i], r.invoices[commodityID][i+1:]...)
			break
		}
	}
	r.invoicesLock.Unlock()
}
