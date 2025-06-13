package registry

import (
	"context"

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
	Create(context.Context, T) (*T, error)

	// Get returns a T from the registry.
	Get(ctx context.Context, id string) (*T, error)

	// List returns a list of Ts from the registry.
	List(context.Context) ([]*T, error)

	// Update updates a T in the registry.
	Update(context.Context, T) (*T, error)

	// Delete deletes a T from the registry.
	Delete(ctx context.Context, id string) error

	// Count returns the number of Ts in the registry.
	Count(context.Context) (int, error)
}

type AreaRegistry interface {
	Registry[models.Area]

	AddCommodity(ctx context.Context, areaID, commodityID string) error
	GetCommodities(ctx context.Context, areaID string) ([]string, error)
	DeleteCommodity(ctx context.Context, areaID, commodityID string) error

	// DeleteRecursive deletes an area and all its commodities
	DeleteRecursive(ctx context.Context, id string) error
}

type CommodityRegistry interface {
	Registry[models.Commodity]

	AddImage(ctx context.Context, commodityID, imageID string) error
	GetImages(ctx context.Context, commodityID string) ([]string, error)
	DeleteImage(ctx context.Context, commodityID, imageID string) error

	AddManual(ctx context.Context, commodityID, manualID string) error
	GetManuals(ctx context.Context, commodityID string) ([]string, error)
	DeleteManual(ctx context.Context, commodityID, manualID string) error

	AddInvoice(ctx context.Context, commodityID, invoiceID string) error
	GetInvoices(ctx context.Context, commodityID string) ([]string, error)
	DeleteInvoice(ctx context.Context, commodityID, invoiceID string) error
}

type LocationRegistry interface {
	Registry[models.Location]

	AddArea(ctx context.Context, locationID, areaID string) error
	GetAreas(ctx context.Context, locationID string) ([]string, error)
	DeleteArea(ctx context.Context, locationID, areaID string) error

	// DeleteRecursive deletes a location and all its areas and commodities
	DeleteRecursive(ctx context.Context, id string) error
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

type SettingsRegistry interface {
	Get(ctx context.Context) (models.SettingsObject, error)
	Save(context.Context, models.SettingsObject) error
	Patch(ctx context.Context, configfield string, value any) error
}

type ExportRegistry interface {
	Registry[models.Export]

	// ListWithDeleted returns all exports including soft deleted ones
	ListWithDeleted(ctx context.Context) ([]*models.Export, error)

	// ListDeleted returns only soft deleted exports
	ListDeleted(ctx context.Context) ([]*models.Export, error)

	// HardDelete permanently deletes an export from the database
	HardDelete(ctx context.Context, id string) error
}



type RestoreOperationRegistry interface {
	Registry[models.RestoreOperation]

	// ListByExport returns all restore operations for an export
	ListByExport(ctx context.Context, exportID string) ([]*models.RestoreOperation, error)
}

type RestoreStepRegistry interface {
	Registry[models.RestoreStep]

	// ListByRestoreOperation returns all restore steps for a restore operation
	ListByRestoreOperation(ctx context.Context, restoreOperationID string) ([]*models.RestoreStep, error)

	// DeleteByRestoreOperation deletes all restore steps for a restore operation
	DeleteByRestoreOperation(ctx context.Context, restoreOperationID string) error
}

type Set struct {
	LocationRegistry         LocationRegistry
	AreaRegistry             AreaRegistry
	CommodityRegistry        CommodityRegistry
	ImageRegistry            ImageRegistry
	InvoiceRegistry          InvoiceRegistry
	ManualRegistry           ManualRegistry
	SettingsRegistry         SettingsRegistry
	ExportRegistry           ExportRegistry
	RestoreOperationRegistry RestoreOperationRegistry
	RestoreStepRegistry      RestoreStepRegistry
}

func (s *Set) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&s.LocationRegistry, validation.Required),
		validation.Field(&s.AreaRegistry, validation.Required),
		validation.Field(&s.CommodityRegistry, validation.Required),
		validation.Field(&s.ImageRegistry, validation.Required),
		validation.Field(&s.ManualRegistry, validation.Required),
		validation.Field(&s.InvoiceRegistry, validation.Required),
		validation.Field(&s.SettingsRegistry, validation.Required),
		validation.Field(&s.ExportRegistry, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, s, fields...)
}
