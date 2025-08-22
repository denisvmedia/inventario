package data

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/db/setup"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

// Command represents the initial dataset setup command
type Command struct {
	command.Base

	config Config
}

// New creates the initial dataset setup command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("migrate.data", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "data",
		Short: "Setup initial dataset with tenant and user structure",
		Long: `Setup initial dataset with proper tenant and user structure by creating a default tenant
and assigning user_id to all existing entities.

This command performs the following operations:
1. Creates a default tenant for all existing data
2. Creates or updates an admin user
3. Assigns all users to the default tenant
4. Assigns user_id to all entities (locations, areas, commodities, etc.)
5. Validates data integrity after setup

IMPORTANT: This is a one-time setup that should be run after applying
the database schema migrations that add tenant_id and user_id columns.

The setup is atomic - if any step fails, all changes are rolled back.

Examples:
  # Preview what would be setup (dry run)
  inventario migrate data --dry-run

  # Perform the actual setup with custom tenant name
  inventario migrate data --default-tenant-name="My Organization" --admin-email="admin@myorg.com"

  # Setup with all custom options
  inventario migrate data \
    --default-tenant-name="Acme Corp" \
    --default-tenant-slug="acme" \
    --admin-email="admin@acme.com" \
    --admin-password="secure-password" \
    --admin-name="System Administrator"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.setupData(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// Dry run flag
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)

	// Default tenant configuration
	c.Cmd().Flags().StringVar(&c.config.DefaultTenantID, "default-tenant-id", c.config.DefaultTenantID, "ID for the default tenant")
	c.Cmd().Flags().StringVar(&c.config.DefaultTenantName, "default-tenant-name", c.config.DefaultTenantName, "Name for the default tenant")
	c.Cmd().Flags().StringVar(&c.config.DefaultTenantSlug, "default-tenant-slug", c.config.DefaultTenantSlug, "Slug for the default tenant")

	// Admin user configuration
	c.Cmd().Flags().StringVar(&c.config.AdminEmail, "admin-email", c.config.AdminEmail, "Email for the admin user")
	c.Cmd().Flags().StringVar(&c.config.AdminPassword, "admin-password", c.config.AdminPassword, "Password for the admin user")
	c.Cmd().Flags().StringVar(&c.config.AdminName, "admin-name", c.config.AdminName, "Name for the admin user")
}

// setupData handles the initial dataset setup process
func (c *Command) setupData(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	dsn := dbConfig.DBDSN

	if dsn == "" {
		return fmt.Errorf("database DSN is required")
	}

	fmt.Println("=== INITIAL DATASET SETUP ===")
	fmt.Printf("Database: %s\n", dsn)
	if cfg.DryRun {
		fmt.Println("Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Println("Mode: LIVE SETUP")
	}
	fmt.Println()

	// Connect to database
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Create setup manager with os.Stdout as writer
	setupManager := setup.NewDataSetupManager(db, os.Stdout)

	// Prepare setup options
	opts := setup.SetupOptions{
		DefaultTenantID:   cfg.DefaultTenantID,
		DefaultTenantName: cfg.DefaultTenantName,
		DefaultTenantSlug: cfg.DefaultTenantSlug,
		AdminEmail:        cfg.AdminEmail,
		AdminPassword:     cfg.AdminPassword,
		AdminName:         cfg.AdminName,
		DryRun:            cfg.DryRun,
	}

	// Perform setup
	result, err := setupManager.SetupInitialDataset(context.Background(), opts)
	if err != nil {
		if result != nil {
			result.PrintSetupSummary(os.Stdout)
		}
		return fmt.Errorf("setup failed: %w", err)
	}

	// Print summary
	result.PrintSetupSummary(os.Stdout)

	if cfg.DryRun {
		fmt.Println("\n💡 To perform the actual setup, run the command without --dry-run")
	} else {
		fmt.Println("\n🎉 Initial dataset setup completed successfully!")
		fmt.Println("Your data is now properly structured with tenant and user isolation.")
	}

	return nil
}
