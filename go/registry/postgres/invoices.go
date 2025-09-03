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

// InvoiceRegistryFactory creates InvoiceRegistry instances with proper context
type InvoiceRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// InvoiceRegistry is a context-aware registry that can only be created through the factory
type InvoiceRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

var _ registry.InvoiceRegistry = (*InvoiceRegistry)(nil)
var _ registry.InvoiceRegistryFactory = (*InvoiceRegistryFactory)(nil)

func NewInvoiceRegistry(dbx *sqlx.DB) *InvoiceRegistryFactory {
	return NewInvoiceRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewInvoiceRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *InvoiceRegistryFactory {
	return &InvoiceRegistryFactory{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// Factory methods implementing registry.InvoiceRegistryFactory

func (f *InvoiceRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.InvoiceRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *InvoiceRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.InvoiceRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}

	return &InvoiceRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     user.ID,
		tenantID:   user.TenantID,
		service:    false,
	}, nil
}

func (f *InvoiceRegistryFactory) CreateServiceRegistry() registry.InvoiceRegistry {
	return &InvoiceRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     "",
		tenantID:   "",
		service:    true,
	}
}

func (r *InvoiceRegistry) Get(ctx context.Context, id string) (*models.Invoice, error) {
	return r.get(ctx, id)
}

func (r *InvoiceRegistry) List(ctx context.Context) ([]*models.Invoice, error) {
	var invoices []*models.Invoice

	reg := r.newSQLRegistry()

	// Query the database for all invoices (atomic operation)
	for invoice, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list invoices")
		}
		invoices = append(invoices, &invoice)
	}

	return invoices, nil
}

func (r *InvoiceRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count invoices")
	}

	return cnt, nil
}

func (r *InvoiceRegistry) Create(ctx context.Context, invoice models.Invoice) (*models.Invoice, error) {
	// ID, TenantID, and UserID are now set automatically by RLSRepository.Create

	reg := r.newSQLRegistry()

	createdInvoice, err := reg.Create(ctx, invoice, func(ctx context.Context, tx *sqlx.Tx) error {
		// Check if the commodity exists
		var commodity models.Commodity
		commodityReg := store.NewTxRegistry[models.Commodity](tx, r.tableNames.Commodities())
		err := commodityReg.ScanOneByField(ctx, store.Pair("id", invoice.CommodityID), &commodity)
		if err != nil {
			return errkit.Wrap(err, "failed to get commodity")
		}
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create invoice")
	}

	return &createdInvoice, nil
}

func (r *InvoiceRegistry) Update(ctx context.Context, invoice models.Invoice) (*models.Invoice, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, invoice, func(ctx context.Context, tx *sqlx.Tx, dbInvoice models.Invoice) error {
		// Check if the commodity exists
		var commodity models.Commodity
		commodityReg := store.NewTxRegistry[models.Commodity](tx, r.tableNames.Commodities())
		err := commodityReg.ScanOneByField(ctx, store.Pair("id", invoice.CommodityID), &commodity)
		if err != nil {
			return errkit.Wrap(err, "failed to get commodity")
		}
		// TODO: what if commodity has changed, allow or not? (currently allowed)
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update invoice")
	}

	return &invoice, nil
}

func (r *InvoiceRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, nil)
	return err
}

func (r *InvoiceRegistry) newSQLRegistry() *store.RLSRepository[models.Invoice, *models.Invoice] {
	if r.service {
		return store.NewServiceSQLRegistry[models.Invoice](r.dbx, r.tableNames.Invoices())
	}
	return store.NewUserAwareSQLRegistry[models.Invoice](r.dbx, r.userID, r.tenantID, r.tableNames.Invoices())
}

func (r *InvoiceRegistry) get(ctx context.Context, id string) (*models.Invoice, error) {
	var invoice models.Invoice
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get invoice")
	}

	return &invoice, nil
}
