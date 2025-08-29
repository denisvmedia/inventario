package restore

import (
	"context"

	"github.com/denisvmedia/inventario/backup/restore/processor"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// RestoreService handles XML restore operations with different strategies
type RestoreService struct {
	registrySet    *registry.Set
	entityService  *services.EntityService
	uploadLocation string
}

// NewRestoreService creates a new restore service
func NewRestoreService(registrySet *registry.Set, entityService *services.EntityService, uploadLocation string) *RestoreService {
	return &RestoreService{
		registrySet:    registrySet,
		entityService:  entityService,
		uploadLocation: uploadLocation,
	}
}

// ProcessRestoreOperation processes a restore operation in the background with detailed logging
func (s *RestoreService) ProcessRestoreOperation(ctx context.Context, restoreOperationID, uploadLocation string) error {
	return processor.NewRestoreOperationProcessor(restoreOperationID, s.registrySet, s.entityService, uploadLocation).Process(ctx)
}
