package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// UserConcurrencySlotRegistryFactory creates UserConcurrencySlotRegistry instances with proper context
type UserConcurrencySlotRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// UserConcurrencySlotRegistry is a context-aware registry that can only be created through the factory
type UserConcurrencySlotRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

var _ registry.UserConcurrencySlotRegistry = (*UserConcurrencySlotRegistry)(nil)
var _ registry.UserConcurrencySlotRegistryFactory = (*UserConcurrencySlotRegistryFactory)(nil)

func NewUserConcurrencySlotRegistry(dbx *sqlx.DB) *UserConcurrencySlotRegistryFactory {
	return NewUserConcurrencySlotRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewUserConcurrencySlotRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *UserConcurrencySlotRegistryFactory {
	return &UserConcurrencySlotRegistryFactory{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// Factory methods implementing registry.UserConcurrencySlotRegistryFactory

func (f *UserConcurrencySlotRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.UserConcurrencySlotRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *UserConcurrencySlotRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.UserConcurrencySlotRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}

	return &UserConcurrencySlotRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     user.ID,
		tenantID:   user.TenantID,
		service:    false,
	}, nil
}

func (f *UserConcurrencySlotRegistryFactory) CreateServiceRegistry() registry.UserConcurrencySlotRegistry {
	return &UserConcurrencySlotRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     "",
		tenantID:   "",
		service:    true,
	}
}

// Get returns a user concurrency slot by ID
func (r *UserConcurrencySlotRegistry) Get(ctx context.Context, id string) (*models.UserConcurrencySlot, error) {
	var slot models.UserConcurrencySlot
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &slot)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user concurrency slot")
	}

	return &slot, nil
}

// List returns all user concurrency slots
func (r *UserConcurrencySlotRegistry) List(ctx context.Context) ([]*models.UserConcurrencySlot, error) {
	var slots []*models.UserConcurrencySlot
	reg := r.newSQLRegistry()

	for slot, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list user concurrency slots")
		}
		slots = append(slots, &slot)
	}

	return slots, nil
}

// Count returns the number of user concurrency slots
func (r *UserConcurrencySlotRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count user concurrency slots")
	}

	return cnt, nil
}

// Create creates a new user concurrency slot
func (r *UserConcurrencySlotRegistry) Create(ctx context.Context, slot models.UserConcurrencySlot) (*models.UserConcurrencySlot, error) {
	reg := r.newSQLRegistry()

	result, err := reg.Create(ctx, slot, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create user concurrency slot")
	}

	return &result, nil
}

// Update updates a user concurrency slot
func (r *UserConcurrencySlotRegistry) Update(ctx context.Context, slot models.UserConcurrencySlot) (*models.UserConcurrencySlot, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, slot, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update user concurrency slot")
	}

	return &slot, nil
}

// Delete deletes a user concurrency slot
func (r *UserConcurrencySlotRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()

	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errkit.Wrap(err, "failed to delete user concurrency slot")
	}

	return nil
}

// AcquireSlot attempts to acquire a concurrency slot for a user
func (r *UserConcurrencySlotRegistry) AcquireSlot(ctx context.Context, userID, jobID string, maxSlots int, slotDuration time.Duration) (*models.UserConcurrencySlot, error) {
	reg := r.newSQLRegistry()
	var result *models.UserConcurrencySlot

	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Check current slot counts for the user
		var activeCount int
		countQuery := fmt.Sprintf(`
			SELECT COUNT(*)
			FROM %s
			WHERE user_id = $1 AND status = 'active'`, r.tableNames.UserConcurrencySlots())

		err := tx.QueryRowContext(ctx, countQuery, userID).Scan(&activeCount)
		if err != nil {
			return errkit.Wrap(err, "failed to count existing slots")
		}

		// Check limits
		if activeCount >= maxSlots {
			return registry.ErrResourceLimitExceeded
		}

		// Create the slot
		slot := models.UserConcurrencySlot{
			JobID:     jobID,
			Status:    models.SlotStatusActive,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Use the RLS repository to create the slot (it will set user/tenant context)
		txReg := store.NewTxRegistry[models.UserConcurrencySlot](tx, r.tableNames.UserConcurrencySlots())
		err = txReg.Insert(ctx, slot)
		if err != nil {
			return errkit.Wrap(err, "failed to create concurrency slot")
		}

		result = &slot
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// ReleaseSlot releases a concurrency slot
func (r *UserConcurrencySlotRegistry) ReleaseSlot(ctx context.Context, userID, jobID string) error {
	reg := r.newSQLRegistry()

	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE user_id = $1 AND job_id = $2`, r.tableNames.UserConcurrencySlots())
		result, err := tx.ExecContext(ctx, query, userID, jobID)
		if err != nil {
			return errkit.Wrap(err, "failed to release concurrency slot")
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return errkit.Wrap(err, "failed to get rows affected")
		}

		if rowsAffected == 0 {
			return registry.ErrNotFound
		}
		return nil
	})

	return err
}

// GetUserSlots returns all slots for a user
func (r *UserConcurrencySlotRegistry) GetUserSlots(ctx context.Context, userID string) ([]*models.UserConcurrencySlot, error) {
	var slots []*models.UserConcurrencySlot
	reg := r.newSQLRegistry()

	for slot, err := range reg.ScanByField(ctx, store.Pair("user_id", userID)) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list user slots")
		}
		slots = append(slots, &slot)
	}

	return slots, nil
}

// CleanupExpiredSlots removes expired slots
func (r *UserConcurrencySlotRegistry) CleanupExpiredSlots(ctx context.Context) error {
	reg := r.newSQLRegistry()

	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Remove slots that are older than 1 hour (expired)
		expiredBefore := time.Now().Add(-1 * time.Hour)
		query := fmt.Sprintf(`
			DELETE FROM %s
			WHERE created_at < $1`, r.tableNames.UserConcurrencySlots())

		_, err := tx.ExecContext(ctx, query, expiredBefore)
		if err != nil {
			return errkit.Wrap(err, "failed to cleanup expired slots")
		}
		return nil
	})

	return err
}

func (r *UserConcurrencySlotRegistry) newSQLRegistry() *store.RLSRepository[models.UserConcurrencySlot, *models.UserConcurrencySlot] {
	if r.service {
		return store.NewServiceSQLRegistry[models.UserConcurrencySlot](r.dbx, r.tableNames.UserConcurrencySlots())
	}
	return store.NewUserAwareSQLRegistry[models.UserConcurrencySlot](r.dbx, r.userID, r.tenantID, r.tableNames.UserConcurrencySlots())
}
