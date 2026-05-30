package restore

import (
	"context"

	"github.com/denisvmedia/inventario/backup/restore/processor"
	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// RestoreService handles backup restore operations with different strategies.
//
// The signer is threaded into the restore processor where the default `.inb`
// restorer uses it to verify the archive signature; the legacy XML restorer
// ignores it. The constructor signature is identical across both builds.
type RestoreService struct {
	factorySet     *registry.FactorySet
	entityService  *services.EntityService
	uploadLocation string
	signer         *backupsign.Signer
}

// NewRestoreService creates a new restore service.
func NewRestoreService(factorySet *registry.FactorySet, entityService *services.EntityService, uploadLocation string, signer *backupsign.Signer) *RestoreService {
	return &RestoreService{
		factorySet:     factorySet,
		entityService:  entityService,
		uploadLocation: uploadLocation,
		signer:         signer,
	}
}

// ProcessRestoreOperation processes a restore operation in the background with detailed logging.
func (s *RestoreService) ProcessRestoreOperation(ctx context.Context, restoreOperationID, uploadLocation string) error {
	return processor.NewRestoreOperationProcessor(restoreOperationID, s.factorySet, s.entityService, uploadLocation, s.signer).Process(ctx)
}
