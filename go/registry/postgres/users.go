package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.UserRegistry = (*UserRegistry)(nil)

type UserRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewUserRegistry(dbx *sqlx.DB) *UserRegistry {
	return NewUserRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewUserRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *UserRegistry {
	return &UserRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *UserRegistry) newSQLRegistry() *store.NonRLSRepository[models.User, *models.User] {
	return store.NewSQLRegistry[models.User](r.dbx, r.tableNames.Users())
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

	// Always generate a new server-side ID for security (ignore any user-provided ID)
	user.SetID(generateID())

	// If UserID is not set, set it to the user's own ID (self-reference)
	if user.UserID == "" {
		user.UserID = user.GetID()
	}

	reg := r.newSQLRegistry()

	err := reg.Create(ctx, user, func(ctx context.Context, tx *sqlx.Tx) error {
		// Check if a user with the same email already exists
		var existingUser models.User
		txReg := store.NewTxRegistry[models.User](tx, r.tableNames.Users())
		err := txReg.ScanOneByField(ctx, store.Pair("email", user.Email), &existingUser)
		if err == nil {
			return errkit.WithStack(registry.ErrEmailAlreadyExists,
				"email", user.Email,
			)
		} else if !errors.Is(err, store.ErrNotFound) {
			return errkit.Wrap(err, "failed to check for existing user")
		}
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create user")
	}

	return &user, nil
}

func (r *UserRegistry) Get(ctx context.Context, id string) (*models.User, error) {
	if id == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "ID",
		)
	}

	var user models.User
	reg := r.newSQLRegistry()
	err := reg.ScanOneByField(ctx, store.Pair("id", id), &user)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
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

	reg := r.newSQLRegistry()

	// Query the database for all users (atomic operation)
	for user, err := range reg.Scan(ctx) {
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

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, user, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update user")
	}

	return &user, nil
}

func (r *UserRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "ID",
		)
	}

	reg := r.newSQLRegistry()

	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errkit.Wrap(err, "failed to delete user")
	}

	return nil
}

func (r *UserRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count users")
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

	reg := r.newSQLRegistry()

	// Use Do to execute custom query logic
	var user models.User
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE tenant_id = $1 AND email = $2`, r.tableNames.Users())
		err := tx.GetContext(ctx, &user, query, tenantID, email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errkit.WithStack(registry.ErrNotFound,
					"entity_type", "User",
					"tenant_id", tenantID,
					"email", email,
				)
			}
			return errkit.Wrap(err, "failed to get user by email")
		}
		return nil
	})
	if err != nil {
		return nil, err
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
	reg := r.newSQLRegistry()

	for user, err := range reg.ScanByField(ctx, store.Pair("tenant_id", tenantID)) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list users by tenant")
		}
		users = append(users, &user)
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
	reg := r.newSQLRegistry()

	// Use Do to execute custom query logic for multiple field filtering
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE tenant_id = $1 AND role = $2 ORDER BY name`, r.tableNames.Users())
		err := tx.SelectContext(ctx, &users, query, tenantID, role)
		if err != nil {
			return errkit.Wrap(err, "failed to list users by role")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return users, nil
}
