package update

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/services/admin"
)

// Command represents the user update command
type Command struct {
	command.Base

	config Config
}

// New creates a new user update command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("users.update", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "update <user-id-or-email>",
		Short: "Update an existing user",
		Long: `Update an existing user in the system.

This command allows you to update user information such as email, name, role,
active status, tenant assignment, and password.

Examples:
  # Interactive mode
  inventario users update user@example.com

  # With flags
  inventario users update user@example.com --name="New Name" --role=admin

  # Dry run to preview changes
  inventario users update user@example.com --name="New Name" --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.updateUser(&c.config, dbConfig, args[0])
		},
	})

	return c
}

// updateUser handles the user update process
func (c *Command) updateUser(cfg *Config, dbConfig *shared.DatabaseConfig, idOrEmail string) error {
	out := c.Cmd().OutOrStdout()

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

	fmt.Fprintln(out, "=== UPDATE USER ===")
	fmt.Fprintf(out, "Target: %s\n", idOrEmail)
	if cfg.DryRun {
		fmt.Fprintln(out, "Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Fprintln(out, "Mode: LIVE UPDATE")
	}
	fmt.Fprintln(out)

	// Find the user to update
	originalUser, err := adminService.GetUser(context.Background(), idOrEmail)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	// Get current tenant info
	currentTenant, err := adminService.GetTenant(context.Background(), originalUser.TenantID)
	if err != nil {
		return fmt.Errorf("failed to get current tenant: %w", err)
	}

	fmt.Fprintf(out, "Found user: %s (%s)\n", originalUser.Name, originalUser.Email)
	fmt.Fprintf(out, "Current tenant: %s (%s)\n\n", currentTenant.Name, currentTenant.Slug)

	// For now, just show that we found the user and would update it
	// TODO: Implement proper update logic with service layer
	fmt.Fprintln(out, "User update functionality needs to be implemented with service layer")
	fmt.Fprintln(out, "This is a placeholder to fix compilation issues")

	return nil
}
