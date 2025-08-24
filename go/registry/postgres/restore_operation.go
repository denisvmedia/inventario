package postgres

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.RestoreOperationRegistry = (*RestoreOperationRegistry)(nil)

type RestoreOperationRegistry struct {
	dbx                 *sqlx.DB
	tableNames          store.TableNames
	userID              string
	tenantID            string
	restoreStepRegistry registry.RestoreStepRegistry
}

func NewRestoreOperationRegistry(dbx *sqlx.DB, restoreStepRegistry registry.RestoreStepRegistry) *RestoreOperationRegistry {
	return NewRestoreOperationRegistryWithTableNames(dbx, store.DefaultTableNames, restoreStepRegistry)
}

func NewRestoreOperationRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames, restoreStepRegistry registry.RestoreStepRegistry) *RestoreOperationRegistry {
	return &RestoreOperationRegistry{
		dbx:                 dbx,
		tableNames:          tableNames,
		restoreStepRegistry: restoreStepRegistry,
	}
}

func (r *RestoreOperationRegistry) WithCurrentUser(ctx context.Context) (registry.RestoreOperationRegistry, error) {
	tmp := *r

	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}
	tmp.userID = user.ID
	tmp.tenantID = user.TenantID

	// Also update the restore step registry with user context
	userAwareStepRegistry, err := tmp.restoreStepRegistry.WithCurrentUser(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to set user context on restore step registry")
	}
	tmp.restoreStepRegistry = userAwareStepRegistry

	return &tmp, nil
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
			return nil, errkit.Wrap(err, "failed to list restore operations")
		}

		// Load associated steps for each operation
		err = r.loadSteps(ctx, &operation)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to load steps for operation")
		}

		operations = append(operations, &operation)
	}

	return operations, nil
}

func (r *RestoreOperationRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count restore operations")
	}

	return cnt, nil
}

func (r *RestoreOperationRegistry) Create(ctx context.Context, operation models.RestoreOperation) (*models.RestoreOperation, error) {
	if err := operation.ValidateWithContext(ctx); err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Set timestamps
	operation.CreatedDate = models.PNow()

	// Generate ID if not set
	if operation.ID == "" {
		operation.ID = generateID()
	}

	// Set default status if not set
	if operation.Status == "" {
		operation.Status = models.RestoreStatusPending
	}

	reg := r.newSQLRegistry()

	err := reg.Create(ctx, operation, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create restore operation")
	}

	return &operation, nil
}

func (r *RestoreOperationRegistry) Update(ctx context.Context, operation models.RestoreOperation) (*models.RestoreOperation, error) {
	if err := operation.ValidateWithContext(ctx); err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, operation, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update restore operation")
	}

	return &operation, nil
}

func (r *RestoreOperationRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, func(ctx context.Context, tx *sqlx.Tx) error {
		// Delete associated steps first (due to foreign key constraint)
		if err := r.restoreStepRegistry.DeleteByRestoreOperation(ctx, id); err != nil {
			return errkit.Wrap(err, "failed to delete restore steps")
		}
		return nil
	})
	return err
}

func (r *RestoreOperationRegistry) newSQLRegistry() *store.RLSRepository[models.RestoreOperation] {
	return store.NewUserAwareSQLRegistry[models.RestoreOperation](r.dbx, r.userID, r.tableNames.RestoreOperations())
}

func (r *RestoreOperationRegistry) get(ctx context.Context, id string) (*models.RestoreOperation, error) {
	var operation models.RestoreOperation
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &operation)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get restore operation")
	}

	// Load associated steps
	err = r.loadSteps(ctx, &operation)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to load steps")
	}

	return &operation, nil
}

func (r *RestoreOperationRegistry) loadSteps(ctx context.Context, operation *models.RestoreOperation) error {
	// Load associated steps
	steps, err := r.restoreStepRegistry.ListByRestoreOperation(ctx, operation.ID)
	if err != nil {
		return errkit.Wrap(err, "failed to load restore steps")
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
			return nil, errkit.Wrap(err, "failed to list restore operations by export")
		}

		// Load associated steps for each operation
		err = r.loadSteps(ctx, &operation)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to load steps for operation")
		}

		operations = append(operations, &operation)
	}

	return operations, nil
}
