package postgres

import (
	"context"
	"errors"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.LocationGroupRegistry = (*LocationGroupRegistry)(nil)

type LocationGroupRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewLocationGroupRegistry(dbx *sqlx.DB) *LocationGroupRegistry {
	return &LocationGroupRegistry{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
	}
}

// newSQLRegistry returns an RLSRepository in service mode. location_groups has
// RLS enabled with a tenant-isolation policy on inventario_app, so running via
// the background-worker role (which has a bypass policy) is how we reach this
// tenant-scoped table from flows that manage tenant scoping in application
// code — the GroupService layer passes tenant_id explicitly to GetBySlug /
// ListByTenant / the slug-uniqueness check. Same pattern as RefreshTokenRegistry.
func (r *LocationGroupRegistry) newSQLRegistry() *store.RLSRepository[models.LocationGroup, *models.LocationGroup] {
	return store.NewServiceSQLRegistry[models.LocationGroup, *models.LocationGroup](r.dbx, r.tableNames.LocationGroups())
}

func (r *LocationGroupRegistry) Get(ctx context.Context, id string) (*models.LocationGroup, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var group models.LocationGroup
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &group)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "LocationGroup",
				"entity_id", id,
			))
		}
		return nil, errxtrace.Wrap("failed to get location group", err)
	}

	return &group, nil
}

func (r *LocationGroupRegistry) List(ctx context.Context) ([]*models.LocationGroup, error) {
	var groups []*models.LocationGroup

	reg := r.newSQLRegistry()

	for group, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list location groups", err)
		}
		groups = append(groups, &group)
	}

	return groups, nil
}

func (r *LocationGroupRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count location groups", err)
	}

	return count, nil
}

func (r *LocationGroupRegistry) Create(ctx context.Context, group models.LocationGroup) (*models.LocationGroup, error) {
	if group.Name == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}

	if group.Slug == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Slug"))
	}

	if group.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	// CreatedBy and UserID are NOT NULL columns (schema-level FK to users.id).
	// Without this check, a caller constructing a LocationGroup without
	// setting CreatedBy would silently insert an empty string and blow up
	// at the FK violation rather than here with a meaningful field_name.
	if group.CreatedBy == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "CreatedBy"))
	}

	reg := r.newSQLRegistry()

	createdGroup, err := reg.Create(ctx, group, func(ctx context.Context, tx *sqlx.Tx) error {
		// Scope the slug-uniqueness lookup to the (tenant_id, slug) unique
		// index directly instead of scanning every row that happens to match
		// the slug — much cheaper on a tenant with many groups.
		var existing models.LocationGroup
		txReg := store.NewTxRegistry[models.LocationGroup](tx, r.tableNames.LocationGroups())
		err := txReg.ScanOneByFields(ctx, []store.FieldValue{
			store.Pair("tenant_id", group.TenantID),
			store.Pair("slug", group.Slug),
		}, &existing)
		if err == nil {
			return errxtrace.Classify(registry.ErrSlugAlreadyExists, errx.Attrs("slug", group.Slug))
		}
		if !errors.Is(err, store.ErrNotFound) {
			return errxtrace.Wrap("failed to check for existing group", err)
		}
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to create location group", err)
	}

	return &createdGroup, nil
}

func (r *LocationGroupRegistry) Update(ctx context.Context, group models.LocationGroup) (*models.LocationGroup, error) {
	if group.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	if group.Name == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, group, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update location group", err)
	}

	return &group, nil
}

func (r *LocationGroupRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errxtrace.Wrap("failed to delete location group", err)
	}

	return nil
}

func (r *LocationGroupRegistry) GetBySlug(ctx context.Context, tenantID, slug string) (*models.LocationGroup, error) {
	if slug == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Slug"))
	}

	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	reg := r.newSQLRegistry()

	for group, err := range reg.ScanByField(ctx, store.Pair("slug", slug)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to get location group by slug", err)
		}
		if group.TenantID == tenantID {
			return &group, nil
		}
	}

	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
		"entity_type", "LocationGroup",
		"slug", slug,
	))
}

func (r *LocationGroupRegistry) ListByTenant(ctx context.Context, tenantID string) ([]*models.LocationGroup, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	var groups []*models.LocationGroup
	reg := r.newSQLRegistry()

	for group, err := range reg.ScanByField(ctx, store.Pair("tenant_id", tenantID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list location groups by tenant", err)
		}
		groups = append(groups, &group)
	}

	return groups, nil
}
