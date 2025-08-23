package postgres

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.UserRegistry = (*UserRegistry)(nil)

type UserRegistry struct {
	dbx        *sqlx.DB
	tableNames TableNames
}

func NewUserRegistry(dbx *sqlx.DB) *UserRegistry {
	return NewUserRegistryWithTableNames(dbx, DefaultTableNames)
}

func NewUserRegistryWithTableNames(dbx *sqlx.DB, tableNames TableNames) *UserRegistry {
	return &UserRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *UserRegistry) Create(ctx context.Context, user models.User) (*models.User, error) {
	if user.Email == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Email",
		)
	}

	if user.Name == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Name",
		)
	}

	if user.TenantID == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "TenantID",
		)
	}

	// Generate a new ID if one is not already provided
	if user.GetID() == "" {
		user.SetID(generateID())
	}

	// If UserID is not set, set it to the user's own ID (self-reference)
	if user.UserID == "" {
		user.UserID = user.GetID()
	}

	// Insert the user into the database (atomic operation)
	err := InsertEntity(ctx, r.dbx, r.tableNames.Users(), user)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert entity")
	}

	return &user, nil
}

func (r *UserRegistry) Get(ctx context.Context, id string) (*models.User, error) {
	if id == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "ID",
		)
	}

	// For authentication operations, we need to bypass RLS
	// Reset role to default to avoid RLS restrictions during user lookup
	_, err := r.dbx.ExecContext(ctx, "RESET ROLE")
	if err != nil {
		// If RESET ROLE fails, continue anyway - might not be using PostgreSQL with RLS
		slog.With("error", err).Warn("Failed to reset database role for user lookup")
	}

	var user models.User
	err = ScanEntityByField(ctx, r.dbx, r.tableNames.Users(), "id", id, &user)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, errkit.WithStack(registry.ErrNotFound,
				"entity_type", "User",
				"entity_id", id,
			)
		}
		return nil, errkit.Wrap(err, "failed to get entity")
	}

	return &user, nil
}

func (r *UserRegistry) List(ctx context.Context) ([]*models.User, error) {
	var users []*models.User

	// Query the database for all users (atomic operation)
	for user, err := range ScanEntities[models.User](ctx, r.dbx, r.tableNames.Users()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list users")
		}
		users = append(users, &user)
	}

	return users, nil
}

func (r *UserRegistry) Update(ctx context.Context, user models.User) (*models.User, error) {
	if user.GetID() == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "ID",
		)
	}

	if user.Email == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Email",
		)
	}

	if user.Name == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Name",
		)
	}

	if user.TenantID == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "TenantID",
		)
	}

	err := UpdateEntityByField(ctx, r.dbx, r.tableNames.Users(), "id", user.GetID(), user)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update entity")
	}

	return &user, nil
}

func (r *UserRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "ID",
		)
	}

	err := DeleteEntityByField(ctx, r.dbx, r.tableNames.Users(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete entity")
	}

	return nil
}

func (r *UserRegistry) Count(ctx context.Context) (int, error) {
	count, err := CountEntities(ctx, r.dbx, r.tableNames.Users())
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count entities")
	}

	return count, nil
}

// GetByEmail returns a user by email within a tenant
func (r *UserRegistry) GetByEmail(ctx context.Context, tenantID, email string) (*models.User, error) {
	if tenantID == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "TenantID",
		)
	}

	if email == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Email",
		)
	}

	// For authentication operations, we need to bypass RLS
	// Reset role to default to avoid RLS restrictions during user lookup
	_, err := r.dbx.ExecContext(ctx, "RESET ROLE")
	if err != nil {
		// If RESET ROLE fails, continue anyway - might not be using PostgreSQL with RLS
		slog.With("error", err).Warn("Failed to reset database role for user lookup")
	}

	var user models.User
	query := `SELECT * FROM ` + r.tableNames.Users() + ` WHERE tenant_id = $1 AND email = $2`
	err = r.dbx.GetContext(ctx, &user, query, tenantID, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errkit.WithStack(registry.ErrNotFound,
				"entity_type", "User",
				"tenant_id", tenantID,
				"email", email,
			)
		}
		return nil, errkit.Wrap(err, "failed to get user by email")
	}

	return &user, nil
}

// ListByTenant returns all users for a tenant
func (r *UserRegistry) ListByTenant(ctx context.Context, tenantID string) ([]*models.User, error) {
	if tenantID == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "TenantID",
		)
	}

	var users []*models.User
	query := `SELECT * FROM ` + r.tableNames.Users() + ` WHERE tenant_id = $1 ORDER BY name`
	err := r.dbx.SelectContext(ctx, &users, query, tenantID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list users by tenant")
	}

	return users, nil
}

// ListByRole returns all users with a specific role within a tenant
func (r *UserRegistry) ListByRole(ctx context.Context, tenantID string, role models.UserRole) ([]*models.User, error) {
	if tenantID == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "TenantID",
		)
	}

	var users []*models.User
	query := `SELECT * FROM ` + r.tableNames.Users() + ` WHERE tenant_id = $1 AND role = $2 ORDER BY name`
	err := r.dbx.SelectContext(ctx, &users, query, tenantID, role)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list users by role")
	}

	return users, nil
}
