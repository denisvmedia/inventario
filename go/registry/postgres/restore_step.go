package postgres

import (
	"context"

	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// RestoreStepRegistryFactory creates RestoreStepRegistry instances with proper context
type RestoreStepRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// RestoreStepRegistry is a context-aware registry that can only be created through the factory
type RestoreStepRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

var _ registry.RestoreStepRegistry = (*RestoreStepRegistry)(nil)
var _ registry.RestoreStepRegistryFactory = (*RestoreStepRegistryFactory)(nil)

func NewRestoreStepRegistry(dbx *sqlx.DB) *RestoreStepRegistryFactory {
	return NewRestoreStepRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewRestoreStepRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *RestoreStepRegistryFactory {
	return &RestoreStepRegistryFactory{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// Factory methods implementing registry.RestoreStepRegistryFactory

func (f *RestoreStepRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.RestoreStepRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *RestoreStepRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.RestoreStepRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}

	return &RestoreStepRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     user.ID,
		tenantID:   user.TenantID,
		service:    false,
	}, nil
}

func (f *RestoreStepRegistryFactory) CreateServiceRegistry() registry.RestoreStepRegistry {
	return &RestoreStepRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     "",
		tenantID:   "",
		service:    true,
	}
}

func (r *RestoreStepRegistry) Get(ctx context.Context, id string) (*models.RestoreStep, error) {
	return r.get(ctx, id)
}

func (r *RestoreStepRegistry) List(ctx context.Context) ([]*models.RestoreStep, error) {
	var steps []*models.RestoreStep

	reg := r.newSQLRegistry()

	// Query the database for all restore steps (atomic operation)
	for step, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list restore steps")
		}
		steps = append(steps, &step)
	}

	return steps, nil
}

func (r *RestoreStepRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count restore steps")
	}

	return cnt, nil
}

func (r *RestoreStepRegistry) Create(ctx context.Context, step models.RestoreStep) (*models.RestoreStep, error) {
	if err := step.ValidateWithContext(ctx); err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Set timestamps
	step.CreatedDate = models.PNow()
	step.UpdatedDate = models.PNow()

	// ID, TenantID, and UserID are now set automatically by RLSRepository.Create

	reg := r.newSQLRegistry()

	err := reg.Create(ctx, step, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create restore step")
	}

	return &step, nil
}

func (r *RestoreStepRegistry) Update(ctx context.Context, step models.RestoreStep) (*models.RestoreStep, error) {
	if err := step.ValidateWithContext(ctx); err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Update timestamp
	step.UpdatedDate = models.PNow()

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, step, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update restore step")
	}

	return &step, nil
}

func (r *RestoreStepRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, nil)
	return err
}

func (r *RestoreStepRegistry) newSQLRegistry() *store.RLSRepository[models.RestoreStep, *models.RestoreStep] {
	if r.service {
		return store.NewServiceSQLRegistry[models.RestoreStep](r.dbx, r.tableNames.RestoreSteps())
	}
	return store.NewUserAwareSQLRegistry[models.RestoreStep](r.dbx, r.userID, r.tenantID, r.tableNames.RestoreSteps())
}

func (r *RestoreStepRegistry) get(ctx context.Context, id string) (*models.RestoreStep, error) {
	var step models.RestoreStep
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &step)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get restore step")
	}

	return &step, nil
}

func (r *RestoreStepRegistry) ListByRestoreOperation(ctx context.Context, restoreOperationID string) ([]*models.RestoreStep, error) {
	var steps []*models.RestoreStep

	reg := r.newSQLRegistry()
	for step, err := range reg.ScanByField(ctx, store.Pair("restore_operation_id", restoreOperationID)) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list restore steps by operation")
		}
		steps = append(steps, &step)
	}

	return steps, nil
}

func (r *RestoreStepRegistry) DeleteByRestoreOperation(ctx context.Context, restoreOperationID string) error {
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		txReg := store.NewTxRegistry[models.RestoreStep](tx, r.tableNames.RestoreSteps())
		err := txReg.DeleteByField(ctx, store.Pair("restore_operation_id", restoreOperationID))
		if err != nil {
			return errkit.Wrap(err, "failed to delete restore steps by operation")
		}
		return nil
	})
	return err
}
