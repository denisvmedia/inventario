package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// OperationSlotRegistry implements registry.OperationSlotRegistry for PostgreSQL
type OperationSlotRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

// NewOperationSlotRegistry creates a new PostgreSQL operation slot registry
func NewOperationSlotRegistry(db *sql.DB, tableNames store.TableNames, service bool, userID, tenantID string) *OperationSlotRegistry {
	return &OperationSlotRegistry{
		dbx:        sqlx.NewDb(db, "postgres"),
		tableNames: tableNames,
		userID:     userID,
		tenantID:   tenantID,
		service:    service,
	}
}

// newSQLRegistry creates a new SQL registry for this operation slot registry
func (r *OperationSlotRegistry) newSQLRegistry() *store.RLSRepository[models.OperationSlot, *models.OperationSlot] {
	if r.service {
		return store.NewServiceSQLRegistry[models.OperationSlot, *models.OperationSlot](r.dbx, r.tableNames.OperationSlots())
	}
	return store.NewUserAwareSQLRegistry[models.OperationSlot, *models.OperationSlot](r.dbx, r.userID, r.tenantID, r.tableNames.OperationSlots())
}

// Create creates a new operation slot
func (r *OperationSlotRegistry) Create(ctx context.Context, slot models.OperationSlot) (*models.OperationSlot, error) {
	reg := r.newSQLRegistry()
	createdSlot, err := reg.Create(ctx, slot, nil)
	if err != nil {
		return nil, err
	}
	return &createdSlot, nil
}

// Get retrieves an operation slot by ID
func (r *OperationSlotRegistry) Get(ctx context.Context, id string) (*models.OperationSlot, error) {
	var slot models.OperationSlot
	reg := r.newSQLRegistry()
	err := reg.ScanOneByField(ctx, store.Pair("id", id), &slot)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get operation slot")
	}
	return &slot, nil
}

// List returns all operation slots
func (r *OperationSlotRegistry) List(ctx context.Context) ([]*models.OperationSlot, error) {
	var slots []*models.OperationSlot
	reg := r.newSQLRegistry()

	for slot, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan operation slots")
		}
		slots = append(slots, &slot)
	}

	return slots, nil
}

// Update updates an operation slot
func (r *OperationSlotRegistry) Update(ctx context.Context, slot models.OperationSlot) (*models.OperationSlot, error) {
	reg := r.newSQLRegistry()
	err := reg.Update(ctx, slot, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update operation slot")
	}
	return &slot, nil
}

// Delete deletes an operation slot
func (r *OperationSlotRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	return reg.Delete(ctx, id, nil)
}

// Count returns the number of operation slots
func (r *OperationSlotRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()
	return reg.Count(ctx)
}

// GetSlot retrieves a specific slot for a user and operation
func (r *OperationSlotRegistry) GetSlot(ctx context.Context, userID, operationName string, slotID int) (*models.OperationSlot, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, user_id, slot_id, operation_name, created_at, expires_at
		FROM %s 
		WHERE user_id = $1 AND operation_name = $2 AND slot_id = $3 AND expires_at > NOW()
		LIMIT 1`,
		r.tableNames.OperationSlots())

	var slot models.OperationSlot
	err := r.dbx.QueryRowContext(ctx, query, userID, operationName, slotID).Scan(
		&slot.ID,
		&slot.TenantID,
		&slot.UserID,
		&slot.SlotID,
		&slot.OperationName,
		&slot.CreatedAt,
		&slot.ExpiresAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errkit.Wrap(registry.ErrNotFound, "operation slot not found")
		}
		return nil, errkit.Wrap(err, "failed to get operation slot")
	}

	return &slot, nil
}

// ReleaseSlot removes a specific slot for a user and operation
func (r *OperationSlotRegistry) ReleaseSlot(ctx context.Context, userID, operationName string, slotID int) error {
	query := fmt.Sprintf(`
		DELETE FROM %s 
		WHERE user_id = $1 AND operation_name = $2 AND slot_id = $3`,
		r.tableNames.OperationSlots())

	result, err := r.dbx.ExecContext(ctx, query, userID, operationName, slotID)
	if err != nil {
		return errkit.Wrap(err, "failed to release operation slot")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errkit.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errkit.Wrap(registry.ErrNotFound, "operation slot not found")
	}

	return nil
}

// GetActiveSlotCount returns the number of active (non-expired) slots for a user and operation
func (r *OperationSlotRegistry) GetActiveSlotCount(ctx context.Context, userID, operationName string) (int, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM %s 
		WHERE user_id = $1 AND operation_name = $2 AND expires_at > NOW()`,
		r.tableNames.OperationSlots())

	var count int
	err := r.dbx.QueryRowContext(ctx, query, userID, operationName).Scan(&count)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to get active slot count")
	}

	return count, nil
}

// GetNextSlotID returns the next available slot ID for a user and operation
func (r *OperationSlotRegistry) GetNextSlotID(ctx context.Context, userID, operationName string) (int, error) {
	query := fmt.Sprintf(`
		SELECT COALESCE(MAX(slot_id), 0) + 1 
		FROM %s 
		WHERE user_id = $1 AND operation_name = $2`,
		r.tableNames.OperationSlots())

	var nextID int
	err := r.dbx.QueryRowContext(ctx, query, userID, operationName).Scan(&nextID)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to get next slot ID")
	}

	return nextID, nil
}

// CleanupExpiredSlots removes all expired slots and returns the count of deleted slots
func (r *OperationSlotRegistry) CleanupExpiredSlots(ctx context.Context) (int, error) {
	query := fmt.Sprintf(`
		DELETE FROM %s 
		WHERE expires_at <= NOW()`,
		r.tableNames.OperationSlots())

	result, err := r.dbx.ExecContext(ctx, query)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to cleanup expired slots")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, errkit.Wrap(err, "failed to get rows affected")
	}

	return int(rowsAffected), nil
}

// GetOperationStats returns statistics about slot usage across all operations
func (r *OperationSlotRegistry) GetOperationStats(ctx context.Context) (map[string]models.OperationStats, error) {
	query := fmt.Sprintf(`
		SELECT 
			operation_name,
			COUNT(*) as active_slots,
			COUNT(DISTINCT user_id) as total_users
		FROM %s 
		WHERE expires_at > NOW()
		GROUP BY operation_name`,
		r.tableNames.OperationSlots())

	rows, err := r.dbx.QueryContext(ctx, query)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get operation statistics")
	}
	defer rows.Close()

	stats := make(map[string]models.OperationStats)

	for rows.Next() {
		var operationName string
		var activeSlots, totalUsers int

		err := rows.Scan(&operationName, &activeSlots, &totalUsers)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan operation statistics")
		}

		// Note: MaxSlots and AvgUtilization will be calculated by the service layer
		// since it has access to the configuration
		stats[operationName] = models.OperationStats{
			OperationName:  operationName,
			ActiveSlots:    activeSlots,
			TotalUsers:     totalUsers,
			MaxSlots:       0, // Will be set by service layer
			AvgUtilization: 0, // Will be calculated by service layer
		}
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating operation statistics")
	}

	return stats, nil
}

// GetUserSlotStats returns slot usage statistics for a specific user
func (r *OperationSlotRegistry) GetUserSlotStats(ctx context.Context, userID string) (map[string]int, error) {
	query := fmt.Sprintf(`
		SELECT operation_name, COUNT(*) 
		FROM %s 
		WHERE user_id = $1 AND expires_at > NOW()
		GROUP BY operation_name`,
		r.tableNames.OperationSlots())

	rows, err := r.dbx.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user slot statistics")
	}
	defer rows.Close()

	stats := make(map[string]int)

	for rows.Next() {
		var operationName string
		var count int

		err := rows.Scan(&operationName, &count)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan user slot statistics")
		}

		stats[operationName] = count
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating user slot statistics")
	}

	return stats, nil
}

// GetExpiredSlots returns all expired slots (for testing/debugging)
func (r *OperationSlotRegistry) GetExpiredSlots(ctx context.Context) ([]models.OperationSlot, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, user_id, slot_id, operation_name, created_at, expires_at
		FROM %s 
		WHERE expires_at <= NOW()
		ORDER BY expires_at ASC`,
		r.tableNames.OperationSlots())

	rows, err := r.dbx.QueryContext(ctx, query)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get expired slots")
	}
	defer rows.Close()

	var slots []models.OperationSlot

	for rows.Next() {
		var slot models.OperationSlot
		err := rows.Scan(
			&slot.ID,
			&slot.TenantID,
			&slot.UserID,
			&slot.SlotID,
			&slot.OperationName,
			&slot.CreatedAt,
			&slot.ExpiresAt,
		)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan expired slot")
		}

		slots = append(slots, slot)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating expired slots")
	}

	return slots, nil
}

// OperationSlotRegistryFactory implements registry.OperationSlotRegistryFactory for PostgreSQL
type OperationSlotRegistryFactory struct {
	db         *sqlx.DB
	tableNames store.TableNames
}

// NewOperationSlotRegistryFactory creates a new PostgreSQL operation slot registry factory
func NewOperationSlotRegistryFactory(db *sqlx.DB) *OperationSlotRegistryFactory {
	return &OperationSlotRegistryFactory{
		db:         db,
		tableNames: store.NewTableNames(),
	}
}

// CreateUserRegistry creates a new registry with user context from the provided context
func (f *OperationSlotRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.OperationSlotRegistry, error) {
	user := appctx.UserFromContext(ctx)
	if user == nil {
		return nil, errkit.Wrap(registry.ErrInvalidInput, "user context required")
	}

	return NewOperationSlotRegistry(f.db.DB, f.tableNames, false, user.ID, user.TenantID), nil
}

// MustCreateUserRegistry creates a new registry with user context, panics on error
func (f *OperationSlotRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.OperationSlotRegistry {
	reg, err := f.CreateUserRegistry(ctx)
	if err != nil {
		panic(err)
	}
	return reg
}

// CreateServiceRegistry creates a new registry with service account context
func (f *OperationSlotRegistryFactory) CreateServiceRegistry() registry.OperationSlotRegistry {
	return NewOperationSlotRegistry(f.db.DB, f.tableNames, true, "", "")
}
