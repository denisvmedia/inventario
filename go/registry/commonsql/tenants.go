package commonsql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.TenantRegistry = (*TenantRegistry)(nil)

type TenantRegistry struct {
	dbx        *sqlx.DB
	tableNames TableNames
}

func NewTenantRegistry(dbx *sqlx.DB) *TenantRegistry {
	return NewTenantRegistryWithTableNames(dbx, DefaultTableNames)
}

func NewTenantRegistryWithTableNames(dbx *sqlx.DB, tableNames TableNames) *TenantRegistry {
	return &TenantRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
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

	// Generate a new ID if one is not already provided
	if tenant.GetID() == "" {
		tenant.SetID(generateID())
	}

	// Insert the tenant into the database (atomic operation)
	err := InsertEntity(ctx, r.dbx, r.tableNames.Tenants(), tenant)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert entity")
	}

	return &tenant, nil
}

func (r *TenantRegistry) Get(ctx context.Context, id string) (*models.Tenant, error) {
	if id == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "ID",
		)
	}

	var tenant models.Tenant
	err := ScanEntityByField(ctx, r.dbx, r.tableNames.Tenants(), "id", id, &tenant)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
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

	// Query the database for all tenants (atomic operation)
	for tenant, err := range ScanEntities[models.Tenant](ctx, r.dbx, r.tableNames.Tenants()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list tenants")
		}
		tenants = append(tenants, &tenant)
	}

	return tenants, nil
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

	err := UpdateEntityByField(ctx, r.dbx, r.tableNames.Tenants(), "id", tenant.GetID(), tenant)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update entity")
	}

	return &tenant, nil
}

func (r *TenantRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "ID",
		)
	}

	err := DeleteEntityByField(ctx, r.dbx, r.tableNames.Tenants(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete entity")
	}

	return nil
}

func (r *TenantRegistry) Count(ctx context.Context) (int, error) {
	count, err := CountEntities(ctx, r.dbx, r.tableNames.Tenants())
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count entities")
	}

	return count, nil
}

// GetBySlug returns a tenant by its slug
func (r *TenantRegistry) GetBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	if slug == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Slug",
		)
	}

	var tenant models.Tenant
	query := `SELECT * FROM ` + r.tableNames.Tenants() + ` WHERE slug = $1`
	err := r.dbx.GetContext(ctx, &tenant, query, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	query := `SELECT * FROM ` + r.tableNames.Tenants() + ` WHERE domain = $1`
	err := r.dbx.GetContext(ctx, &tenant, query, domain)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errkit.WithStack(registry.ErrNotFound,
				"entity_type", "Tenant",
				"domain", domain,
			)
		}
		return nil, errkit.Wrap(err, "failed to get tenant by domain")
	}

	return &tenant, nil
}
