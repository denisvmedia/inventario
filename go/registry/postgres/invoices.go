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

var _ registry.InvoiceRegistry = (*InvoiceRegistry)(nil)

type InvoiceRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

func NewInvoiceRegistry(dbx *sqlx.DB) *InvoiceRegistry {
	return NewInvoiceRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewInvoiceRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *InvoiceRegistry {
	return &InvoiceRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *InvoiceRegistry) MustWithCurrentUser(ctx context.Context) registry.InvoiceRegistry {
	return must.Must(r.WithCurrentUser(ctx))
}

func (r *InvoiceRegistry) WithCurrentUser(ctx context.Context) (registry.InvoiceRegistry, error) {
	tmp := *r

	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}
	tmp.userID = user.ID
	tmp.tenantID = user.TenantID
	tmp.service = false
	return &tmp, nil
}

func (r *InvoiceRegistry) WithServiceAccount() registry.InvoiceRegistry {
	tmp := *r
	tmp.userID = ""
	tmp.tenantID = ""
	tmp.service = true
	return &tmp
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
	// Always generate a new server-side ID for security (ignore any user-provided ID)
	invoice.SetID(generateID())
	invoice.SetTenantID(r.tenantID)
	invoice.SetUserID(r.userID)

	reg := r.newSQLRegistry()

	err := reg.Create(ctx, invoice, func(ctx context.Context, tx *sqlx.Tx) error {
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

	return &invoice, nil
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
