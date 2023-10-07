package registry

import (
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

type PIDable[T any] interface {
	*T
	IDable
}

type IDable interface {
	GetID() string
	SetID(id string)
}

type Registry[T any] interface {
	// Create creates a new T in the registry.
	Create(T) (*T, error)

	// Get returns a T from the registry.
	Get(id string) (*T, error)

	// List returns a list of Ts from the registry.
	List() ([]*T, error)

	// Update updates a T in the registry.
	Update(T) (*T, error)

	// Delete deletes a T from the registry.
	Delete(id string) error

	// Count returns the number of Ts in the registry.
	Count() (int, error)
}

type AreaRegistry interface {
	Registry[models.Area]

	AddCommodity(areaID, commodityID string) error
	GetCommodities(areaID string) ([]string, error)
	DeleteCommodity(areaID, commodityID string) error
}

type CommodityRegistry interface {
	Registry[models.Commodity]

	AddImage(commodityID, imageID string)
	GetImages(commodityID string) []string
	DeleteImage(commodityID, imageID string)

	AddManual(commodityID, manualID string)
	GetManuals(commodityID string) []string
	DeleteManual(commodityID, manualID string)

	AddInvoice(commodityID, invoiceID string)
	GetInvoices(commodityID string) []string
	DeleteInvoice(commodityID, invoiceID string)
}

type LocationRegistry interface {
	Registry[models.Location]

	AddArea(locationID, areaID string) error
	GetAreas(locationID string) ([]string, error)
	DeleteArea(locationID, areaID string) error
}

type ImageRegistry interface {
	Registry[models.Image]
}

type InvoiceRegistry interface {
	Registry[models.Invoice]
}

type ManualRegistry interface {
	Registry[models.Manual]
}

type Set struct {
	LocationRegistry  LocationRegistry
	AreaRegistry      AreaRegistry
	CommodityRegistry CommodityRegistry
	ImageRegistry     ImageRegistry
	InvoiceRegistry   InvoiceRegistry
	ManualRegistry    ManualRegistry
}

func (s *Set) Validate() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&s.LocationRegistry, validation.Required),
		//validation.Field(&s.AreaRegistry, validation.Required),
		//validation.Field(&s.CommodityRegistry, validation.Required),
		//validation.Field(&s.ImageRegistry, validation.Required),
		//validation.Field(&s.ManualRegistry, validation.Required),
		//validation.Field(&s.InvoiceRegistry, validation.Required),
	)

	return validation.ValidateStruct(s, fields...)
}
