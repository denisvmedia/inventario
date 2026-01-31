package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

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
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}

	if user.Name == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}

	if user.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	// We need to handle user creation specially because of the self-referencing foreign key
	// We'll create the user with a custom implementation that handles the UserID properly

	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errxtrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, store.RollbackOrCommit(tx, err))
	}()

	// Generate a new server-side ID for security (ignore any user-provided ID)
	generatedID := uuid.New().String()
	user.ID = generatedID

	// Set UserID to self-reference if not already set
	if user.UserID == "" {
		user.UserID = generatedID
	}

	// Check if a user with the same email already exists
	var existingUser models.User
	txReg := store.NewTxRegistry[models.User](tx, r.tableNames.Users())
	err = txReg.ScanOneByField(ctx, store.Pair("email", user.Email), &existingUser)
	if err == nil {
		return nil, errxtrace.Classify(registry.ErrEmailAlreadyExists, errx.Attrs("email", user.Email))
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, errxtrace.Wrap("failed to check for existing user", err)
	}

	// Insert the user
	err = txReg.Insert(ctx, user)
	if err != nil {
		return nil, errxtrace.Wrap("failed to insert user", err)
	}

	return &user, nil
}

func (r *UserRegistry) Get(ctx context.Context, id string) (*models.User, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var user models.User
	reg := r.newSQLRegistry()
	err := reg.ScanOneByField(ctx, store.Pair("id", id), &user)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "User",
				"entity_id", id,
			))
		}
		return nil, errxtrace.Wrap("failed to get entity", err)
	}

	return &user, nil
}

func (r *UserRegistry) List(ctx context.Context) ([]*models.User, error) {
	var users []*models.User

	reg := r.newSQLRegistry()

	// Query the database for all users (atomic operation)
	for user, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list users", err)
		}
		users = append(users, &user)
	}

	return users, nil
}

func (r *UserRegistry) Update(ctx context.Context, user models.User) (*models.User, error) {
	if user.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	if user.Email == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}

	if user.Name == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}

	if user.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, user, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update user", err)
	}

	return &user, nil
}

func (r *UserRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errxtrace.Wrap("failed to delete user", err)
	}

	return nil
}

func (r *UserRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count users", err)
	}

	return count, nil
}

// GetByEmail returns a user by email within a tenant
func (r *UserRegistry) GetByEmail(ctx context.Context, tenantID, email string) (*models.User, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	if email == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}

	reg := r.newSQLRegistry()

	// Use Do to execute custom query logic
	var user models.User
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE tenant_id = $1 AND email = $2`, r.tableNames.Users())
		err := tx.GetContext(ctx, &user, query, tenantID, email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "User",
					"tenant_id", tenantID,
					"email", email,
				))
			}
			return errxtrace.Wrap("failed to get user by email", err)
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
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	var users []*models.User
	reg := r.newSQLRegistry()

	for user, err := range reg.ScanByField(ctx, store.Pair("tenant_id", tenantID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list users by tenant", err)
		}
		users = append(users, &user)
	}

	return users, nil
}

// ListByRole returns all users with a specific role within a tenant
func (r *UserRegistry) ListByRole(ctx context.Context, tenantID string, role models.UserRole) ([]*models.User, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	var users []*models.User
	reg := r.newSQLRegistry()

	// Use Do to execute custom query logic for multiple field filtering
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE tenant_id = $1 AND role = $2 ORDER BY name`, r.tableNames.Users())
		err := tx.SelectContext(ctx, &users, query, tenantID, role)
		if err != nil {
			return errxtrace.Wrap("failed to list users by role", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return users, nil
}
