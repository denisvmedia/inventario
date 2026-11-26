package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"go.5x5.cz/inventario/cmd/internal/command"
	"go.5x5.cz/inventario/cmd/internal/input"
	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/internal/defaults"
	"go.5x5.cz/inventario/internal/filekit"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/admin"
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

Moving a user to another tenant (--tenant) is allowed only for a user who
belongs to no group and owns or authored nothing in their tenant: no items,
files, locations, loans, history entries or any other content. The typical
case is an account created in the wrong tenant. Otherwise the move is
refused, nothing is changed, and the command lists what blocks it, for
example "member of 2 groups, 14 items, 3 files". Moving a user together with
their content is not supported.

A move signs the user out everywhere: every session, refresh token and API
token is deleted, so the user signs in again. The password, MFA, language and
other settings, linked sign-in providers and login history move with the user.
The profile photo is copied into the new tenant's storage, which is why the
command needs the same --upload-location the server uses. A --dry-run with
--tenant runs the checks the move runs before it writes: the target tenant
exists and is active, no user there has the same email, the user owns nothing,
and the photo is in the upload location.

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

  # Move a user created in the wrong tenant (no groups, no content)
  inventario users update user@example.com --tenant=acme --upload-location=s3://bucket

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
	c.Cmd().Flags().StringVar(&c.config.UploadLocation, "upload-location", c.config.UploadLocation,
		"Object-store location of uploaded files; a tenant move copies the profile photo there (defaults to the configured upload location)")
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

	moving := req.TenantID != nil && *req.TenantID != originalUser.TenantID
	if moving {
		// A move copies the profile photo, so the dry run checks the same
		// storage the live run copies from.
		wireAvatarStorage(cfg, adminService)
	}

	// Handle dry run
	if cfg.DryRun {
		if moving {
			c.reportTenantMoveCheck(adminService, originalUser, *req.TenantID)
		}
		fmt.Fprintln(out, "💡 This is a dry run. To perform the actual update, run the command without --dry-run")
		return nil
	}

	return c.applyUpdate(adminService, idOrEmail, req, moving)
}

// wireAvatarStorage gives the admin service the photo storage a tenant move
// copies the profile photo in: --upload-location, or the default location.
func wireAvatarStorage(cfg *Config, adminService *admin.Service) {
	uploadLocation := strings.TrimSpace(cfg.UploadLocation)
	if uploadLocation == "" {
		uploadLocation = defaults.GetUploadLocation()
	}
	adminService.SetAvatarService(services.NewAvatarService(adminService.FactorySet(), filekit.NormalizeFileURL(uploadLocation)))
}

// applyUpdate runs a live update. A refused tenant move prints what blocks it.
//
//revive:disable-next-line:flag-parameter // moving is derived from req and the stored user by the caller, which already has both.
func (c *Command) applyUpdate(adminService *admin.Service, idOrEmail string, req admin.UserUpdateRequest, moving bool) error {
	out := c.Cmd().OutOrStdout()
	updatedUser, err := adminService.UpdateUser(context.Background(), idOrEmail, req)
	if blocked, ok := errors.AsType[*registry.UserTenantMoveBlockedError](err); ok {
		printTenantMoveBlocked(out, blocked.Owned)
		return fmt.Errorf("failed to update user: %w", err)
	}
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	if moving {
		fmt.Fprintln(out, "The user was moved and signed out everywhere; they sign in again in the new tenant.")
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

// reportTenantMoveCheck prints, for a dry run, whether the tenant move would
// go through. It runs the checks the live move runs before it writes.
func (c *Command) reportTenantMoveCheck(adminService *admin.Service, user *models.User, tenantID string) {
	out := c.Cmd().OutOrStdout()
	err := adminService.CheckUserTenantMove(context.Background(), user.ID, tenantID)
	if blocked, ok := errors.AsType[*registry.UserTenantMoveBlockedError](err); ok {
		printTenantMoveBlocked(out, blocked.Owned)
		return
	}
	if err != nil {
		fmt.Fprintf(out, "Tenant move check: the move would fail: %v\n\n", err)
		return
	}
	fmt.Fprintln(out, "Tenant move check: allowed (the user belongs to no group and owns no content).")
	fmt.Fprintln(out)
}

// printTenantMoveBlocked lists what keeps the user in their tenant.
func printTenantMoveBlocked(out io.Writer, owned registry.UserOwnedContent) {
	fmt.Fprintln(out, "Tenant move refused. The user still has:")
	for _, entry := range owned {
		fmt.Fprintf(out, "  - %s\n", registry.UserOwnedContent{entry})
	}
	fmt.Fprintln(out, "A user moves to another tenant only once they belong to no group and own no content.")
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
