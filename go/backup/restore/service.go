package restore

import (
	"context"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/backup/restore/processor"
	"go.5x5.cz/inventario/backup/stall"
	"go.5x5.cz/inventario/internal/backupsign"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
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

// StalledRestoreError is stored on a restore whose worker stopped while it
// ran.
const StalledRestoreError = "the worker stopped before the restore finished"

// RecoverStalled fails the restores whose worker stopped in the middle of the
// run, which stay running otherwise, and counts each as dead-lettered. A
// restore is never queued again: it may have written part of its backup. See
// package stall for when a run counts as stopped.
func (s *RestoreService) RecoverStalled(ctx context.Context, now time.Time) (int, error) {
	reg := s.factorySet.RestoreOperationRegistryFactory.CreateServiceRegistry()
	failed, err := reg.FailStalled(ctx, stall.NewSweep(now, StalledRestoreError))
	if err != nil {
		return 0, errxtrace.Wrap("failed to fail stalled restores", err)
	}
	for range failed {
		metrics.RecordJobDeadLettered(string(models.WorkerTypeRestore))
	}
	return failed, nil
}

// ProcessRestoreOperation processes a restore operation in the background with detailed logging.
func (s *RestoreService) ProcessRestoreOperation(ctx context.Context, restoreOperationID, uploadLocation string) error {
	return processor.NewRestoreOperationProcessor(restoreOperationID, s.factorySet, s.entityService, uploadLocation, s.signer).Process(ctx)
}
