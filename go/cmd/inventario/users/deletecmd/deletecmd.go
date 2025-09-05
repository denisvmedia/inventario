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

// Command represents the user delete command
type Command struct {
	command.Base

	config Config
}

// New creates the user delete command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("users.delete", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "delete <id-or-email>",
		Short: "Delete a user",
		Long: `Delete a user with confirmation prompts and impact assessment.

This command deletes a user and associated data. It shows user details
and requires confirmation unless --force is used.

WARNING: This operation is irreversible and will delete the user account
and any associated personal data.

SAFETY FEATURES:
  • User details display before deletion
  • Confirmation prompts (unless --force is used)
  • Dry-run support to preview deletion

Examples:
  # Delete user with confirmation
  inventario users delete admin@acme.com

  # Preview deletion
  inventario users delete admin@acme.com --dry-run

  # Force deletion without confirmation
  inventario users delete admin@acme.com --force

  # Delete by ID
  inventario users delete 550e8400-e29b-41d4-a716-446655440000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.deleteUser(&c.config, dbConfig, args[0])
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

// deleteUser handles the user deletion process
func (c *Command) deleteUser(cfg *Config, dbConfig *shared.DatabaseConfig, idOrEmail string) error {
	out := c.Cmd().OutOrStdout()

	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	// Check if this is a memory database and reject it
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("user deletion is not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	// Validate input
	if strings.TrimSpace(idOrEmail) == "" {
		return fmt.Errorf("user ID or email is required")
	}

	fmt.Fprintln(out, "=== DELETE USER ===")
	fmt.Fprintf(out, "Database: %s\n", dbConfig.DBDSN)
	fmt.Fprintf(out, "Target: %s\n", idOrEmail)
	if cfg.DryRun {
		fmt.Fprintln(out, "Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Fprintln(out, "Mode: LIVE DELETION")
	}
	fmt.Fprintln(out)

	// Create admin service
	adminService, err := admin.NewService(dbConfig)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := adminService.Close(); closeErr != nil {
			fmt.Fprintf(out, "Warning: failed to close admin service: %v\n", closeErr)
		}
	}()

	// Find the user to delete
	user, err := adminService.GetUser(context.Background(), idOrEmail)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	// Get tenant information
	tenant, err := adminService.GetTenant(context.Background(), user.TenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant information: %w", err)
	}

	// Show user information
	fmt.Printf("Found user: %s (%s)\n", user.Name, user.Email)
	fmt.Printf("Role: %s\n", user.Role)
	fmt.Printf("Active: %t\n", user.IsActive)
	fmt.Printf("Tenant: %s (%s)\n", tenant.Name, tenant.Slug)
	fmt.Printf("Created: %s\n", user.CreatedAt.Format("2006-01-02 15:04:05"))
	if user.LastLoginAt != nil {
		fmt.Printf("Last Login: %s\n", user.LastLoginAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("Last Login: <never>")
	}
	fmt.Println()

	fmt.Println("DELETION IMPACT:")
	fmt.Printf("  • User account will be permanently removed\n")
	fmt.Printf("  • All user data will be deleted\n")
	fmt.Printf("  • This operation cannot be undone\n\n")

	if cfg.DryRun {
		fmt.Println("💡 This is a dry run. To perform the actual deletion, run the command without --dry-run")
		return nil
	}

	// Confirmation prompt (unless forced)
	if !cfg.Force {
		if !c.confirmDeletion(user) {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	// Delete the user via service
	err = adminService.DeleteUser(context.Background(), idOrEmail)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	fmt.Println("✅ User deleted successfully!")
	fmt.Printf("Deleted user: %s (%s)\n", user.Name, user.Email)

	return nil
}



// confirmDeletion prompts for deletion confirmation
func (c *Command) confirmDeletion(user *models.User) bool {
	fmt.Printf("⚠️  Are you sure you want to delete user '%s' (%s)? [y/N]: ", user.Name, user.Email)

	var response string
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	if response == "y" || response == "yes" {
		// Double confirmation for safety
		fmt.Printf("⚠️  This will permanently delete the user account. Type '%s' to confirm: ", user.Email)

		var confirmation string
		fmt.Scanln(&confirmation)

		return confirmation == user.Email
	}

	return false
}
