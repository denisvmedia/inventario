package update

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/internal/input"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
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

This command allows you to update user information such as email, name,
active status, tenant assignment, and password. Only the fields you pass as
flags are changed; any flag you omit leaves the corresponding column
untouched.

To grant or revoke platform-wide system-admin privileges use
'inventario admin grant-system-admin' / 'revoke-system-admin' instead — that
privilege lives in a dedicated table, not on the user row.

IMPORTANT: This command ONLY supports PostgreSQL databases. Memory databases
are not supported for persistent user operations.

Examples:
  # Update the display name
  inventario users update user@example.com --name="New Name"

  # Deactivate (block) a user
  inventario users update user@example.com --active=false

  # Change the password (prompted securely)
  inventario users update user@example.com --password

  # Move the user to a different tenant
  inventario users update user@example.com --tenant=acme

  # Dry run to preview changes
  inventario users update user@example.com --name="New Name" --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return c.updateUser(&c.config, dbConfig, args[0])
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// Dry run flag
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)

	// User configuration flags. Use cmd.Flags().Changed(<name>) at run time to
	// decide which of these to apply, so an unset flag never overwrites a value.
	c.Cmd().Flags().StringVar(&c.config.Email, "email", c.config.Email, "New email address")
	c.Cmd().Flags().StringVar(&c.config.Name, "name", c.config.Name, "New display name")
	c.Cmd().Flags().BoolVar(&c.config.Active, "active", c.config.Active, "Whether the user is active")
	c.Cmd().Flags().StringVar(&c.config.Tenant, "tenant", c.config.Tenant, "New tenant ID or slug")
	c.Cmd().Flags().BoolVar(&c.config.Password, "password", c.config.Password, "Change the password (prompted securely)")
}

// updateUser handles the user update process
func (c *Command) updateUser(cfg *Config, dbConfig *shared.DatabaseConfig, idOrEmail string) error {
	out := c.Cmd().OutOrStdout()

	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	// Check if this is a memory database and reject it
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("user update is not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	// Validate input
	if strings.TrimSpace(idOrEmail) == "" {
		return fmt.Errorf("user ID or email is required")
	}

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
	fmt.Fprintf(out, "Database: %s\n", shared.RedactDSN(dbConfig.DBDSN))
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

	// Build the update request from the flags the operator actually provided.
	req, changes, err := c.buildUpdateRequest(cfg, adminService)
	if err != nil {
		return err
	}

	if len(changes) == 0 {
		fmt.Fprintln(out, "No changes requested. Provide one or more of --email, --name, --active, --tenant or --password.")
		return nil
	}

	fmt.Fprintln(out, "The following changes will be applied:")
	for _, change := range changes {
		fmt.Fprintf(out, "  • %s\n", change)
	}
	fmt.Fprintln(out)

	// Loudly warn about the partial nature of a cross-tenant move: only the
	// users row is reassigned, so the user's content and group memberships stay
	// behind and become inaccessible. Printed for both dry-run and live runs so
	// the operator sees it before deciding to proceed.
	if c.Cmd().Flags().Changed("tenant") {
		printCrossTenantWarning(out)
	}

	// Handle dry run
	if cfg.DryRun {
		fmt.Fprintln(out, "💡 This is a dry run. To perform the actual update, run the command without --dry-run")
		return nil
	}

	// Delegate to the service
	updatedUser, err := adminService.UpdateUser(context.Background(), idOrEmail, req)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	fmt.Fprintln(out, "✅ User updated successfully!")
	c.printUserInfo(updatedUser)

	return nil
}

// buildUpdateRequest assembles an admin.UserUpdateRequest from the flags that
// were explicitly set, and returns a human-readable list of the changes. Only
// flags the operator provided are included so unset flags never overwrite an
// existing column.
func (c *Command) buildUpdateRequest(cfg *Config, adminService *admin.Service) (admin.UserUpdateRequest, []string, error) {
	flags := c.Cmd().Flags()

	var req admin.UserUpdateRequest
	var changes []string

	if flags.Changed("email") {
		email := strings.TrimSpace(cfg.Email)
		if email == "" {
			return admin.UserUpdateRequest{}, nil, fmt.Errorf("--email cannot be empty or whitespace-only")
		}
		req.Email = &email
		changes = append(changes, fmt.Sprintf("email → %s", email))
	}

	if flags.Changed("name") {
		name := strings.TrimSpace(cfg.Name)
		if name == "" {
			return admin.UserUpdateRequest{}, nil, fmt.Errorf("--name cannot be empty or whitespace-only")
		}
		req.Name = &name
		changes = append(changes, fmt.Sprintf("name → %s", name))
	}

	if flags.Changed("active") {
		active := cfg.Active
		req.IsActive = &active
		changes = append(changes, fmt.Sprintf("active → %t", active))
	}

	if flags.Changed("tenant") {
		tenantID, err := c.resolveTenantID(cfg.Tenant, adminService)
		if err != nil {
			return admin.UserUpdateRequest{}, nil, err
		}
		req.TenantID = &tenantID
		changes = append(changes, fmt.Sprintf("tenant → %s", tenantID))
	}

	if flags.Changed("password") && cfg.Password {
		password, err := c.collectPassword()
		if err != nil {
			return admin.UserUpdateRequest{}, nil, err
		}
		req.Password = &password
		changes = append(changes, "password → (updated)")
	}

	return req, changes, nil
}

// resolveTenantID converts a tenant ID or slug to the canonical tenant ID.
func (c *Command) resolveTenantID(tenantIDOrSlug string, adminService *admin.Service) (string, error) {
	if strings.TrimSpace(tenantIDOrSlug) == "" {
		return "", fmt.Errorf("tenant ID or slug is required when --tenant is set")
	}

	tenant, err := adminService.GetTenant(context.Background(), tenantIDOrSlug)
	if err != nil {
		return "", fmt.Errorf("tenant not found: %w", err)
	}

	return tenant.ID, nil
}

// collectPassword prompts for a new password securely with strength validation.
func (c *Command) collectPassword() (string, error) {
	ctx := context.Background()
	reader := input.NewReader(os.Stdin, c.Cmd().OutOrStdout())
	passwordField := input.NewPasswordField("New password", reader).
		ValidateStrength()

	value, err := passwordField.Prompt(ctx)
	if err != nil {
		return "", err
	}
	passwordStr, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type returned from password field")
	}
	return passwordStr, nil
}

// printCrossTenantWarning emits a prominent warning that a --tenant move only
// reassigns the users row, leaving the user's content and memberships behind.
func printCrossTenantWarning(out io.Writer) {
	fmt.Fprintln(out, "⚠️  WARNING: --tenant moves only the users row. The user's existing content")
	fmt.Fprintln(out, "    (commodities, files, areas, locations, exports, tags) and group memberships")
	fmt.Fprintln(out, "    stay in the original tenant and will become inaccessible to them.")
	fmt.Fprintln(out, "    See https://github.com/denisvmedia/inventario/issues/2179")
	fmt.Fprintln(out)
}

// printUserInfo prints user information in a formatted way
func (c *Command) printUserInfo(user *models.User) {
	out := c.Cmd().OutOrStdout()

	fmt.Fprintf(out, "  ID:       %s\n", user.ID)
	fmt.Fprintf(out, "  Email:    %s\n", user.Email)
	fmt.Fprintf(out, "  Name:     %s\n", user.Name)
	fmt.Fprintf(out, "  Active:   %t\n", user.IsActive)
	fmt.Fprintf(out, "  Tenant:   %s\n", user.TenantID)
}
