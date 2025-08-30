package postgres

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.TenantRegistry = (*TenantRegistry)(nil)

type TenantRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewTenantRegistry(dbx *sqlx.DB) *TenantRegistry {
	return NewTenantRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewTenantRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *TenantRegistry {
	return &TenantRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *TenantRegistry) newSQLRegistry() *store.NonRLSRepository[models.Tenant, *models.Tenant] {
	return store.NewSQLRegistry[models.Tenant, *models.Tenant](r.dbx, r.tableNames.Tenants())
}

func (r *TenantRegistry) Get(ctx context.Context, id string) (*models.Tenant, error) {
	if id == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "ID",
		)
	}

	var tenant models.Tenant
	reg := r.newSQLRegistry()
	err := reg.ScanOneByField(ctx, store.Pair("id", id), &tenant)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errkit.WithStack(registry.ErrNotFound,
				"entity_type", "Tenant",
				"entity_id", id,
			)
		}
		return nil, errkit.Wrap(err, "failed to get entity")
	}

	return &tenant, nil
}

func (r *TenantRegistry) List(ctx context.Context) ([]*models.Tenant, error) {
	var tenants []*models.Tenant

	reg := r.newSQLRegistry()

	// Query the database for all tenants (atomic operation)
	for tenant, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list tenants")
		}
		tenants = append(tenants, &tenant)
	}

	return tenants, nil
}

func (r *TenantRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count tenants")
	}

	return count, nil
}

func (r *TenantRegistry) Create(ctx context.Context, tenant models.Tenant) (*models.Tenant, error) {
	if tenant.Name == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Name",
		)
	}

	if tenant.Slug == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Slug",
		)
	}

	// ID is now set automatically by NonRLSRepository.Create

	reg := r.newSQLRegistry()

	err := reg.Create(ctx, tenant, func(ctx context.Context, tx *sqlx.Tx) error {
		// Check if a tenant with the same slug already exists
		var existingTenant models.Tenant
		txReg := store.NewTxRegistry[models.Tenant](tx, r.tableNames.Tenants())
		err := txReg.ScanOneByField(ctx, store.Pair("slug", tenant.Slug), &existingTenant)
		if err == nil {
			return errkit.WithStack(registry.ErrSlugAlreadyExists,
				"slug", tenant.Slug,
			)
		} else if !errors.Is(err, store.ErrNotFound) {
			return errkit.Wrap(err, "failed to check for existing tenant")
		}
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create tenant")
	}

	return &tenant, nil
}

func (r *TenantRegistry) Update(ctx context.Context, tenant models.Tenant) (*models.Tenant, error) {
	if tenant.GetID() == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "ID",
		)
	}

	if tenant.Name == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Name",
		)
	}

	if tenant.Slug == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Slug",
		)
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, tenant, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update tenant")
	}

	return &tenant, nil
}

func (r *TenantRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "ID",
		)
	}

	reg := r.newSQLRegistry()

	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errkit.Wrap(err, "failed to delete tenant")
	}

	return nil
}

// GetBySlug returns a tenant by its slug
func (r *TenantRegistry) GetBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	if slug == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Slug",
		)
	}

	var tenant models.Tenant
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("slug", slug), &tenant)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errkit.WithStack(registry.ErrNotFound,
				"entity_type", "Tenant",
				"slug", slug,
			)
		}
		return nil, errkit.Wrap(err, "failed to get tenant by slug")
	}

	return &tenant, nil
}

// GetByDomain returns a tenant by its domain
func (r *TenantRegistry) GetByDomain(ctx context.Context, domain string) (*models.Tenant, error) {
	if domain == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Domain",
		)
	}

	var tenant models.Tenant
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("domain", domain), &tenant)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errkit.WithStack(registry.ErrNotFound,
				"entity_type", "Tenant",
				"domain", domain,
			)
		}
		return nil, errkit.Wrap(err, "failed to get tenant by domain")
	}

	return &tenant, nil
}
