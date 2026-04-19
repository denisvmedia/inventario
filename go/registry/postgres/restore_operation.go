package postgres

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// RestoreOperationRegistryFactory creates RestoreOperationRegistry instances with proper context
type RestoreOperationRegistryFactory struct {
	dbx                 *sqlx.DB
	tableNames          store.TableNames
	restoreStepRegistry *RestoreStepRegistryFactory
}

// RestoreOperationRegistry is a context-aware registry that can only be created through the factory
type RestoreOperationRegistry struct {
	dbx                 *sqlx.DB
	tableNames          store.TableNames
	tenantID            string
	groupID             string
	createdByUserID     string
	service             bool
	restoreStepRegistry registry.RestoreStepRegistry
}

var _ registry.RestoreOperationRegistry = (*RestoreOperationRegistry)(nil)
var _ registry.RestoreOperationRegistryFactory = (*RestoreOperationRegistryFactory)(nil)

func NewRestoreOperationRegistry(dbx *sqlx.DB, restoreStepRegistry *RestoreStepRegistryFactory) *RestoreOperationRegistryFactory {
	return NewRestoreOperationRegistryWithTableNames(dbx, store.DefaultTableNames, restoreStepRegistry)
}

func NewRestoreOperationRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames, restoreStepRegistry *RestoreStepRegistryFactory) *RestoreOperationRegistryFactory {
	return &RestoreOperationRegistryFactory{
		dbx:                 dbx,
		tableNames:          tableNames,
		restoreStepRegistry: restoreStepRegistry,
	}
}

// Factory methods implementing registry.RestoreOperationRegistryFactory

func (f *RestoreOperationRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.RestoreOperationRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *RestoreOperationRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.RestoreOperationRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user ID from context", err)
	}

	// Create user-aware restore step registry
	userAwareStepRegistry, err := f.restoreStepRegistry.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to set user context on restore step registry", err)
	}

	return &RestoreOperationRegistry{
		dbx:                 f.dbx,
		tableNames:          f.tableNames,
		restoreStepRegistry: userAwareStepRegistry,
		tenantID:            user.TenantID,
		groupID:             appctx.GroupIDFromContext(ctx),
		createdByUserID:     user.ID,
		service:             false,
	}, nil
}

func (f *RestoreOperationRegistryFactory) CreateServiceRegistry() registry.RestoreOperationRegistry {
	// Create service-aware restore step registry
	serviceStepRegistry := f.restoreStepRegistry.CreateServiceRegistry()

	return &RestoreOperationRegistry{
		dbx:                 f.dbx,
		tableNames:          f.tableNames,
		restoreStepRegistry: serviceStepRegistry,
		service:             true,
	}
}

func (r *RestoreOperationRegistry) Get(ctx context.Context, id string) (*models.RestoreOperation, error) {
	return r.get(ctx, id)
}

func (r *RestoreOperationRegistry) List(ctx context.Context) ([]*models.RestoreOperation, error) {
	var operations []*models.RestoreOperation

	reg := r.newSQLRegistry()

	// Query the database for all restore operations (atomic operation)
	for operation, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list restore operations", err)
		}

		// Load associated steps for each operation
		err = r.loadSteps(ctx, &operation)
		if err != nil {
			return nil, errxtrace.Wrap("failed to load steps for operation", err)
		}

		operations = append(operations, &operation)
	}

	return operations, nil
}

func (r *RestoreOperationRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count restore operations", err)
	}

	return cnt, nil
}

func (r *RestoreOperationRegistry) Create(ctx context.Context, operation models.RestoreOperation) (*models.RestoreOperation, error) {
	if err := operation.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("validation failed", err)
	}

	// Set timestamps
	operation.CreatedDate = models.PNow()

	// ID, TenantID, and UserID are now set automatically by RLSRepository.Create

	// Set default status if not set
	if operation.Status == "" {
		operation.Status = models.RestoreStatusPending
	}
	operation.SetTenantID(r.tenantID)
	operation.SetGroupID(r.groupID)
	operation.SetCreatedByUserID(r.createdByUserID)

	reg := r.newSQLRegistry()

	createdOperation, err := reg.Create(ctx, operation, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create restore operation", err)
	}

	return &createdOperation, nil
}

func (r *RestoreOperationRegistry) Update(ctx context.Context, operation models.RestoreOperation) (*models.RestoreOperation, error) {
	if err := operation.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("validation failed", err)
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, operation, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update restore operation", err)
	}

	return &operation, nil
}

func (r *RestoreOperationRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, func(ctx context.Context, tx *sqlx.Tx) error {
		// Delete associated steps first (due to foreign key constraint)
		if err := r.restoreStepRegistry.DeleteByRestoreOperation(ctx, id); err != nil {
			return errxtrace.Wrap("failed to delete restore steps", err)
		}
		return nil
	})
	return err
}

func (r *RestoreOperationRegistry) newSQLRegistry() *store.RLSGroupRepository[models.RestoreOperation, *models.RestoreOperation] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.RestoreOperation](r.dbx, r.tableNames.RestoreOperations())
	}
	return store.NewGroupAwareSQLRegistry[models.RestoreOperation](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.RestoreOperations())
}

func (r *RestoreOperationRegistry) get(ctx context.Context, id string) (*models.RestoreOperation, error) {
	var operation models.RestoreOperation
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &operation)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get restore operation", err)
	}

	// Load associated steps
	err = r.loadSteps(ctx, &operation)
	if err != nil {
		return nil, errxtrace.Wrap("failed to load steps", err)
	}

	return &operation, nil
}

func (r *RestoreOperationRegistry) loadSteps(ctx context.Context, operation *models.RestoreOperation) error {
	// Load associated steps
	steps, err := r.restoreStepRegistry.ListByRestoreOperation(ctx, operation.ID)
	if err != nil {
		return errxtrace.Wrap("failed to load restore steps", err)
	}

	// Convert to slice of values instead of pointers for JSON serialization
	operation.Steps = make([]models.RestoreStep, len(steps))
	for i, step := range steps {
		operation.Steps[i] = *step
	}

	return nil
}

func (r *RestoreOperationRegistry) ListByExport(ctx context.Context, exportID string) ([]*models.RestoreOperation, error) {
	var operations []*models.RestoreOperation

	reg := r.newSQLRegistry()
	for operation, err := range reg.ScanByField(ctx, store.Pair("export_id", exportID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list restore operations by export", err)
		}

		// Load associated steps for each operation
		err = r.loadSteps(ctx, &operation)
		if err != nil {
			return nil, errxtrace.Wrap("failed to load steps for operation", err)
		}

		operations = append(operations, &operation)
	}

	return operations, nil
}
