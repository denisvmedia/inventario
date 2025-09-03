package restore

import (
	"context"

	"github.com/denisvmedia/inventario/backup/restore/processor"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// RestoreService handles XML restore operations with different strategies
type RestoreService struct {
	factorySet     *registry.FactorySet
	entityService  *services.EntityService
	uploadLocation string
}

// NewRestoreService creates a new restore service
func NewRestoreService(factorySet *registry.FactorySet, entityService *services.EntityService, uploadLocation string) *RestoreService {
	return &RestoreService{
		factorySet:     factorySet,
		entityService:  entityService,
		uploadLocation: uploadLocation,
	}
}

// ProcessRestoreOperation processes a restore operation in the background with detailed logging
func (s *RestoreService) ProcessRestoreOperation(ctx context.Context, restoreOperationID, uploadLocation string) error {
	return processor.NewRestoreOperationProcessor(restoreOperationID, s.factorySet, s.entityService, uploadLocation).Process(ctx)
}
