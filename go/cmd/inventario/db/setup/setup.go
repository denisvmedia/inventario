package setup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq" // PostgreSQL driver

	"github.com/denisvmedia/inventario/models"
)

// DataSetupManager handles initial dataset setup operations for tenant and user isolation
type DataSetupManager struct {
	db     *sql.DB
	writer io.Writer
}

// NewDataSetupManager creates a new DataSetupManager instance
func NewDataSetupManager(db *sql.DB, writer io.Writer) *DataSetupManager {
	return &DataSetupManager{
		db:     db,
		writer: writer,
	}
}

// SetupOptions contains configuration for initial dataset setup
type SetupOptions struct {
	DefaultTenantID   string
	DefaultTenantName string
	DefaultTenantSlug string
	AdminEmail        string
	AdminPassword     string
	AdminName         string
	DryRun            bool
}

// DefaultSetupOptions returns default setup options
func DefaultSetupOptions() SetupOptions {
	return SetupOptions{
		DefaultTenantID:   "default-tenant-id",
		DefaultTenantName: "Default Organization",
		DefaultTenantSlug: "default",
		AdminEmail:        "admin@example.com",
		AdminPassword:     "admin123",
		AdminName:         "System Administrator",
		DryRun:            false,
	}
}

// SetupResult contains the results of an initial dataset setup operation
type SetupResult struct {
	TenantsCreated      int
	UsersCreated        int
	UsersUpdated        int
	LocationsUpdated    int
	AreasUpdated        int
	CommoditiesUpdated  int
	FilesUpdated        int
	ExportsUpdated      int
	ImagesUpdated       int
	InvoicesUpdated     int
	ManualsUpdated      int
	RestoreOpsUpdated   int
	RestoreStepsUpdated int
	Errors              []string
}

// SetupInitialDataset performs the complete initial dataset setup for user isolation
func (m *DataSetupManager) SetupInitialDataset(ctx context.Context, opts SetupOptions) (*SetupResult, error) {
	result := &SetupResult{}

	if opts.DryRun {
		m.printf("=== DRY RUN MODE ===\n")
		m.printf("No actual changes will be made to the database\n\n")
	}

	// Start transaction for atomic setup
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Step 1: Create default tenant
	m.printf("Step 1: Creating default tenant...\n")
	if err := m.createDefaultTenant(ctx, tx, opts, result); err != nil {
		return result, fmt.Errorf("failed to create default tenant: %w", err)
	}

	// Step 2: Create or update admin user
	m.printf("Step 2: Creating/updating admin user...\n")
	adminUserID, err := m.createOrUpdateAdminUser(ctx, tx, opts, result)
	if err != nil {
		return result, fmt.Errorf("failed to create/update admin user: %w", err)
	}

	// Step 3: Assign users to default tenant
	m.printf("Step 3: Assigning users to default tenant...\n")
	if err := m.assignUsersToDefaultTenant(ctx, tx, opts, result); err != nil {
		return result, fmt.Errorf("failed to assign users to default tenant: %w", err)
	}

	// Step 4: Assign user_id to all entities
	m.printf("Step 4: Assigning user IDs to entities...\n")
	if err := m.assignUserIDsToEntities(ctx, tx, opts, adminUserID, result); err != nil {
		return result, fmt.Errorf("failed to assign user IDs to entities: %w", err)
	}

	// Step 5: Validate data integrity
	m.printf("Step 5: Validating data integrity...\n")
	if err := m.validateDataIntegrity(ctx, tx, result); err != nil {
		return result, fmt.Errorf("data integrity validation failed: %w", err)
	}

	if opts.DryRun {
		m.printf("✅ Dry run completed successfully!\n")
		m.printf("No changes were made to the database.\n")
		return result, nil
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("failed to commit transaction: %w", err)
	}

	m.printf("✅ Initial dataset setup completed successfully!\n")
	return result, nil
}

// printf writes formatted output to the writer
func (m *DataSetupManager) printf(format string, args ...any) {
	if m.writer != nil {
		fmt.Fprintf(m.writer, format, args...)
	}
}

// createDefaultTenant creates the default tenant if it doesn't exist
func (m *DataSetupManager) createDefaultTenant(ctx context.Context, tx *sql.Tx, opts SetupOptions, result *SetupResult) error {
	// Check if tenant already exists
	var exists bool
	err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM tenants WHERE id = $1)", opts.DefaultTenantID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check tenant existence: %w", err)
	}

	if exists {
		m.printf("  Default tenant '%s' already exists\n", opts.DefaultTenantID)
		return nil
	}

	if opts.DryRun {
		m.printf("  Would create default tenant: %s (%s)\n", opts.DefaultTenantName, opts.DefaultTenantSlug)
		result.TenantsCreated = 1
		return nil
	}

	// Create default tenant
	now := time.Now()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tenants (id, name, slug, domain, status, settings, created_at, updated_at)
		VALUES ($1, $2, $3, NULL, 'active', '{}', $4, $5)`,
		opts.DefaultTenantID, opts.DefaultTenantName, opts.DefaultTenantSlug, now, now)
	if err != nil {
		return fmt.Errorf("failed to create default tenant: %w", err)
	}

	result.TenantsCreated = 1
	m.printf("  ✅ Created default tenant: %s\n", opts.DefaultTenantName)
	return nil
}

// createOrUpdateAdminUser creates or updates the admin user
func (m *DataSetupManager) createOrUpdateAdminUser(ctx context.Context, tx *sql.Tx, opts SetupOptions, result *SetupResult) (string, error) {
	// Check if admin user already exists
	var existingUserID string
	err := tx.QueryRowContext(ctx, "SELECT id FROM users WHERE email = $1", opts.AdminEmail).Scan(&existingUserID)

	if errors.Is(err, sql.ErrNoRows) {
		// Create new admin user
		adminUserID := uuid.New().String()

		if opts.DryRun {
			m.printf("  Would create admin user: %s (%s)\n", opts.AdminName, opts.AdminEmail)
			result.UsersCreated = 1
			return adminUserID, nil
		}

		// Create user with hashed password
		user := models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: adminUserID},
				TenantID: opts.DefaultTenantID,
				UserID:   adminUserID, // Self-reference
			},
			Email:    opts.AdminEmail,
			Name:     opts.AdminName,
			Role:     models.UserRoleAdmin,
			IsActive: true,
		}

		err = user.SetPassword(opts.AdminPassword)
		if err != nil {
			return "", fmt.Errorf("failed to hash password: %w", err)
		}

		now := time.Now()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO users (id, email, password_hash, name, role, is_active, tenant_id, user_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			adminUserID, user.Email, user.PasswordHash, user.Name, user.Role, user.IsActive,
			user.TenantID, user.UserID, now, now)
		if err != nil {
			return "", fmt.Errorf("failed to create admin user: %w", err)
		}

		result.UsersCreated = 1
		m.printf("  ✅ Created admin user: %s\n", opts.AdminName)
		return adminUserID, nil
	} else if err != nil {
		return "", fmt.Errorf("failed to check user existence: %w", err)
	}

	// User exists, update tenant_id if needed
	if opts.DryRun {
		m.printf("  Would update existing admin user: %s\n", opts.AdminEmail)
		result.UsersUpdated = 1
		return existingUserID, nil
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE users 
		SET tenant_id = $1, user_id = COALESCE(NULLIF(user_id, ''), id), updated_at = $2
		WHERE id = $3`,
		opts.DefaultTenantID, time.Now(), existingUserID)
	if err != nil {
		return "", fmt.Errorf("failed to update admin user: %w", err)
	}

	result.UsersUpdated = 1
	m.printf("  ✅ Updated existing admin user: %s\n", opts.AdminEmail)
	return existingUserID, nil
}

// assignUsersToDefaultTenant assigns all users to the default tenant
func (m *DataSetupManager) assignUsersToDefaultTenant(ctx context.Context, tx *sql.Tx, opts SetupOptions, result *SetupResult) error {
	if opts.DryRun {
		var count int
		err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE tenant_id IS NULL OR tenant_id = ''").Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to count users without tenant: %w", err)
		}
		m.printf("  Would assign %d users to default tenant\n", count)
		return nil
	}

	// Update users without tenant_id
	res, err := tx.ExecContext(ctx, `
		UPDATE users
		SET tenant_id = $1, user_id = COALESCE(NULLIF(user_id, ''), id), updated_at = $2
		WHERE tenant_id IS NULL OR tenant_id = ''`,
		opts.DefaultTenantID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to assign users to default tenant: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected > 0 {
		result.UsersUpdated += int(rowsAffected)
		m.printf("  ✅ Assigned %d users to default tenant\n", rowsAffected)
	} else {
		m.printf("  ✅ All users already have tenant assignments\n")
	}

	return nil
}

// assignUserIDsToEntities assigns user_id to all entities that don't have one
func (m *DataSetupManager) assignUserIDsToEntities(ctx context.Context, tx *sql.Tx, opts SetupOptions, adminUserID string, result *SetupResult) error {
	// Define entity tables and their update logic
	entities := []struct {
		table       string
		description string
		resultField *int
	}{
		{"locations", "locations", &result.LocationsUpdated},
		{"areas", "areas", &result.AreasUpdated},
		{"commodities", "commodities", &result.CommoditiesUpdated},
		{"files", "files", &result.FilesUpdated},
		{"exports", "exports", &result.ExportsUpdated},
		{"images", "images", &result.ImagesUpdated},
		{"invoices", "invoices", &result.InvoicesUpdated},
		{"manuals", "manuals", &result.ManualsUpdated},
		{"restore_operations", "restore operations", &result.RestoreOpsUpdated},
		{"restore_steps", "restore steps", &result.RestoreStepsUpdated},
	}

	for _, entity := range entities {
		if err := m.assignUserIDToTable(ctx, tx, entity.table, entity.description, adminUserID, opts, entity.resultField); err != nil {
			return fmt.Errorf("failed to assign user IDs to %s: %w", entity.table, err)
		}
	}

	return nil
}

// assignUserIDToTable assigns user_id to a specific table
func (m *DataSetupManager) assignUserIDToTable(ctx context.Context, tx *sql.Tx, table, description, adminUserID string, opts SetupOptions, resultField *int) error {
	if opts.DryRun {
		var count int
		err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE user_id IS NULL OR user_id = ''", table)).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to count %s without user_id: %w", table, err)
		}
		m.printf("  Would assign user IDs to %d %s\n", count, description)
		*resultField = count
		return nil
	}

	// Strategy: Assign all entities to the admin user
	// In a real setup, you might want more sophisticated logic based on creation patterns
	res, err := tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s
		SET user_id = $1, tenant_id = COALESCE(NULLIF(tenant_id, ''), $2)
		WHERE user_id IS NULL OR user_id = ''`, table),
		adminUserID, opts.DefaultTenantID)
	if err != nil {
		return fmt.Errorf("failed to assign user IDs to %s: %w", table, err)
	}

	rowsAffected, _ := res.RowsAffected()
	*resultField = int(rowsAffected)

	if rowsAffected > 0 {
		m.printf("  ✅ Assigned user IDs to %d %s\n", rowsAffected, description)
	} else {
		m.printf("  ✅ All %s already have user assignments\n", description)
	}

	return nil
}

// validateDataIntegrity validates that all entities have proper user_id assignments
func (m *DataSetupManager) validateDataIntegrity(ctx context.Context, tx *sql.Tx, result *SetupResult) error {
	// Define validation queries for each entity type
	validations := []struct {
		table       string
		description string
	}{
		{"users", "users"},
		{"locations", "locations"},
		{"areas", "areas"},
		{"commodities", "commodities"},
		{"files", "files"},
		{"exports", "exports"},
		{"images", "images"},
		{"invoices", "invoices"},
		{"manuals", "manuals"},
		{"restore_operations", "restore operations"},
		{"restore_steps", "restore steps"},
	}

	m.printf("  Checking entity user_id assignments...\n")
	for _, validation := range validations {
		var missingCount int
		err := tx.QueryRowContext(ctx,
			fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE user_id IS NULL OR user_id = ''", validation.table)).Scan(&missingCount)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to validate %s: %v", validation.table, err))
			continue
		}

		if missingCount > 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("%d %s have missing user_id", missingCount, validation.description))
		} else {
			m.printf("    ✅ All %s have user_id assigned\n", validation.description)
		}
	}

	// Validate foreign key constraints
	m.printf("  Checking foreign key constraints...\n")
	fkValidations := []struct {
		table       string
		description string
		query       string
	}{
		{"users", "users with invalid tenant references",
			"SELECT COUNT(*) FROM users u LEFT JOIN tenants t ON u.tenant_id = t.id WHERE t.id IS NULL"},
		{"users", "users with invalid user_id self-references",
			"SELECT COUNT(*) FROM users u1 LEFT JOIN users u2 ON u1.user_id = u2.id WHERE u2.id IS NULL"},
		{"locations", "locations with invalid user references",
			"SELECT COUNT(*) FROM locations l LEFT JOIN users u ON l.user_id = u.id WHERE u.id IS NULL"},
		{"areas", "areas with invalid user references",
			"SELECT COUNT(*) FROM areas a LEFT JOIN users u ON a.user_id = u.id WHERE u.id IS NULL"},
		{"commodities", "commodities with invalid user references",
			"SELECT COUNT(*) FROM commodities c LEFT JOIN users u ON c.user_id = u.id WHERE u.id IS NULL"},
	}

	for _, fkValidation := range fkValidations {
		var invalidCount int
		err := tx.QueryRowContext(ctx, fkValidation.query).Scan(&invalidCount)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to validate %s: %v", fkValidation.description, err))
			continue
		}

		if invalidCount > 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("%d %s", invalidCount, fkValidation.description))
		} else {
			m.printf("    ✅ No invalid %s\n", fkValidation.description)
		}
	}

	if len(result.Errors) > 0 {
		return fmt.Errorf("data integrity validation failed with %d errors", len(result.Errors))
	}

	m.printf("  ✅ Data integrity validation passed\n")
	return nil
}

// PrintSetupSummary prints a summary of the setup results to the writer
func (r *SetupResult) PrintSetupSummary(writer io.Writer) {
	fmt.Fprintf(writer, "\n=== INITIAL DATASET SETUP SUMMARY ===\n")
	fmt.Fprintf(writer, "Tenants created: %d\n", r.TenantsCreated)
	fmt.Fprintf(writer, "Users created: %d\n", r.UsersCreated)
	fmt.Fprintf(writer, "Users updated: %d\n", r.UsersUpdated)
	fmt.Fprintf(writer, "Locations updated: %d\n", r.LocationsUpdated)
	fmt.Fprintf(writer, "Areas updated: %d\n", r.AreasUpdated)
	fmt.Fprintf(writer, "Commodities updated: %d\n", r.CommoditiesUpdated)
	fmt.Fprintf(writer, "Files updated: %d\n", r.FilesUpdated)
	fmt.Fprintf(writer, "Exports updated: %d\n", r.ExportsUpdated)
	fmt.Fprintf(writer, "Images updated: %d\n", r.ImagesUpdated)
	fmt.Fprintf(writer, "Invoices updated: %d\n", r.InvoicesUpdated)
	fmt.Fprintf(writer, "Manuals updated: %d\n", r.ManualsUpdated)
	fmt.Fprintf(writer, "Restore operations updated: %d\n", r.RestoreOpsUpdated)
	fmt.Fprintf(writer, "Restore steps updated: %d\n", r.RestoreStepsUpdated)

	if len(r.Errors) > 0 {
		fmt.Fprintf(writer, "\n❌ Errors encountered: %d\n", len(r.Errors))
		for i, err := range r.Errors {
			fmt.Fprintf(writer, "  %d. %s\n", i+1, err)
		}
	} else {
		fmt.Fprintf(writer, "\n✅ No errors encountered\n")
	}
}
