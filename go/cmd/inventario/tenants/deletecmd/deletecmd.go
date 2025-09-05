package deletecmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/admin"
)

// Command represents the tenant delete command
type Command struct {
	command.Base

	config Config
}

// New creates the tenant delete command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("tenants.delete", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "delete <id-or-slug>",
		Short: "Delete a tenant",
		Long: `Delete a tenant with confirmation prompts and impact assessment.

This command deletes a tenant and all associated data. It shows the impact
of deletion (number of users, data that will be deleted) and requires
confirmation unless --force is used.

WARNING: This operation is irreversible and will delete all tenant data
including users, files, and other associated records.

SAFETY FEATURES:
  • Impact assessment showing what will be deleted
  • Confirmation prompts (unless --force is used)
  • Dry-run support to preview deletion
  • Cascade deletion handling

Examples:
  # Delete tenant with confirmation
  inventario tenants delete acme

  # Preview deletion impact
  inventario tenants delete acme --dry-run

  # Force deletion without confirmation
  inventario tenants delete acme --force

  # Delete by ID
  inventario tenants delete 550e8400-e29b-41d4-a716-446655440000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.deleteTenant(&c.config, dbConfig, args[0])
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// Dry run flag
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)

	// Delete options
	c.Cmd().Flags().BoolVar(&c.config.Force, "force", c.config.Force, "Skip confirmation prompts")
}

// deleteTenant handles the tenant deletion process
func (c *Command) deleteTenant(cfg *Config, dbConfig *shared.DatabaseConfig, idOrSlug string) error {
	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	// Check if this is a memory database and reject it
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("tenant deletion is not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	// Validate input
	if strings.TrimSpace(idOrSlug) == "" {
		return fmt.Errorf("tenant ID or slug is required")
	}

	fmt.Println("=== DELETE TENANT ===")
	fmt.Printf("Database: %s\n", dbConfig.DBDSN)
	fmt.Printf("Target: %s\n", idOrSlug)
	if cfg.DryRun {
		fmt.Println("Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Println("Mode: LIVE DELETION")
	}
	fmt.Println()

	// Create admin service
	adminService, err := admin.NewService(dbConfig)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := adminService.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close admin service: %v\n", closeErr)
		}
	}()

	// Find the tenant to delete
	tenant, err := adminService.GetTenant(context.Background(), idOrSlug)
	if err != nil {
		return fmt.Errorf("failed to find tenant: %w", err)
	}

	// Get impact assessment
	userCount, err := adminService.GetTenantUserCount(context.Background(), tenant.ID)
	if err != nil {
		return fmt.Errorf("failed to assess deletion impact: %w", err)
	}

	// Show tenant and impact information
	fmt.Printf("Found tenant: %s (%s)\n", tenant.Name, tenant.Slug)
	fmt.Printf("Created: %s\n", tenant.CreatedAt.Format("2006-01-02 15:04:05"))
	if tenant.Domain != nil && *tenant.Domain != "" {
		fmt.Printf("Domain: %s\n", *tenant.Domain)
	}
	fmt.Printf("Status: %s\n\n", tenant.Status)

	fmt.Println("DELETION IMPACT:")
	fmt.Printf("  • Users: %d will be deleted\n", userCount)
	fmt.Printf("  • All tenant data will be permanently removed\n")
	fmt.Printf("  • This operation cannot be undone\n\n")

	if cfg.DryRun {
		fmt.Println("💡 This is a dry run. To perform the actual deletion, run the command without --dry-run")
		return nil
	}

	// Confirmation prompt (unless forced)
	if !cfg.Force {
		if !c.confirmDeletion(tenant) {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	// Delete the tenant via service
	err = adminService.DeleteTenant(context.Background(), idOrSlug)
	if err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	fmt.Println("✅ Tenant deleted successfully!")
	fmt.Printf("Deleted tenant: %s (%s)\n", tenant.Name, tenant.Slug)
	fmt.Printf("Deleted %d associated users\n", userCount)

	return nil
}

// confirmDeletion prompts for deletion confirmation
func (c *Command) confirmDeletion(tenant *models.Tenant) bool {
	fmt.Printf("⚠️  Are you sure you want to delete tenant '%s' (%s)? [y/N]: ", tenant.Name, tenant.Slug)

	var response string
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	if response == "y" || response == "yes" {
		// Double confirmation for safety
		fmt.Printf("⚠️  This will permanently delete ALL data for this tenant. Type '%s' to confirm: ", tenant.Slug)

		var confirmation string
		fmt.Scanln(&confirmation)

		return confirmation == tenant.Slug
	}

	return false
}
