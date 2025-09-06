package create

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/internal/input"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/admin"
)

// Command represents the user creation command
type Command struct {
	command.Base

	config Config
}

// New creates the user creation command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("users.create", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		Long: `Create a new user with interactive prompts or command-line flags.

This command creates a new user in the PostgreSQL database with proper validation,
password strength checking, and tenant association. It supports both interactive 
mode (default) and flag-based mode, similar to the Linux 'adduser' command.

REQUIRED FIELDS:
  • Email: Valid email address (must be unique)
  • Password: Strong password (hidden input in interactive mode)
  • Tenant: Tenant ID or slug to associate the user with

OPTIONAL FIELDS:
  • Name: Display name (defaults to email if not provided)
  • Role: User role - 'admin' or 'user' (defaults to 'user')
  • Active: Whether the user is active (defaults to true)

INTERACTIVE MODE:
  By default, the command runs in interactive mode, prompting for all required
  information with secure password input. Use --no-interactive to disable prompts.

PASSWORD REQUIREMENTS:
  • Minimum 8 characters
  • At least one uppercase letter
  • At least one lowercase letter
  • At least one digit

VALIDATION:
  • Email format validation
  • Email uniqueness within tenant
  • Password strength validation
  • Tenant existence validation
  • Role validation

Examples:
  # Create user interactively (like Linux adduser)
  inventario users create

  # Create user with flags
  inventario users create --email="admin@acme.com" --tenant="acme" --role="admin"

  # Preview user creation
  inventario users create --dry-run --email="test@example.com" --tenant="acme"

  # Non-interactive mode
  inventario users create --no-interactive --email="user@corp.com" --password="SecurePass123" --tenant="corp"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.createUser(&c.config, dbConfig)
		},
	})

	c.registerFlags(dbConfig)

	return c
}

func (c *Command) registerFlags(dbConfig *shared.DatabaseConfig) {
	// Database flags
	shared.RegisterLocalDatabaseFlags(c.Cmd(), dbConfig)

	// Dry run flag
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)

	// User configuration flags
	c.Cmd().Flags().StringVar(&c.config.Email, "email", c.config.Email, "User email address (required)")
	c.Cmd().Flags().StringVar(&c.config.Password, "password", c.config.Password, "User password (prompted securely if not provided)")
	c.Cmd().Flags().StringVar(&c.config.Name, "name", c.config.Name, "User display name (defaults to email)")
	c.Cmd().Flags().StringVar(&c.config.Role, "role", c.config.Role, "User role (admin, user)")
	c.Cmd().Flags().StringVar(&c.config.Tenant, "tenant", c.config.Tenant, "Tenant ID or slug (required)")
	c.Cmd().Flags().BoolVar(&c.config.Active, "active", c.config.Active, "Whether the user is active")

	// Command behavior flags
	c.Cmd().Flags().BoolVar(&c.config.Interactive, "interactive", c.config.Interactive, "Enable interactive prompts")

	// Handle no-interactive flag by using a separate variable and post-processing
	var noInteractive bool
	c.Cmd().Flags().BoolVar(&noInteractive, "no-interactive", false, "Disable interactive prompts")

	// Set up pre-run to handle no-interactive flag
	originalPreRun := c.Cmd().PreRunE
	c.Cmd().PreRunE = func(cmd *cobra.Command, args []string) error {
		if noInteractive {
			c.config.Interactive = false
		}
		if originalPreRun != nil {
			return originalPreRun(cmd, args)
		}
		return nil
	}
}

// createUser handles the user creation process
func (c *Command) createUser(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	// 1. Create admin service
	adminService, err := admin.NewService(dbConfig)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := adminService.Close(); closeErr != nil {
			fmt.Fprintf(out, "Warning: failed to close admin service: %v\n", closeErr)
		}
	}()

	// 2. Show operation info
	fmt.Fprintln(out, "=== CREATE USER ===")
	if cfg.DryRun {
		fmt.Fprintln(out, "Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Fprintln(out, "Mode: LIVE CREATION")
	}
	fmt.Fprintln(out)

	// 3. Collect user information and convert to service request
	userReq, err := c.collectUserRequest(cfg, adminService)
	if err != nil {
		return fmt.Errorf("failed to collect user information: %w", err)
	}

	// 4. Handle dry run
	if cfg.DryRun {
		fmt.Fprintln(out, "Would create user:")
		c.printUserRequest(userReq)
		fmt.Fprintln(out, "\n💡 To perform the actual creation, run the command without --dry-run")
		return nil
	}

	// 5. Delegate to service
	createdUser, err := adminService.CreateUser(context.Background(), *userReq)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// 6. Format and output result
	fmt.Fprintln(out, "✅ User created successfully!")
	c.printUserInfo(createdUser, nil) // We'll get tenant info separately if needed

	return nil
}

// collectUserRequest collects user information and converts to service request
func (c *Command) collectUserRequest(cfg *Config, adminService *admin.Service) (*admin.UserCreateRequest, error) {
	email, err := c.collectEmail(cfg)
	if err != nil {
		return nil, err
	}

	name, err := c.collectName(cfg)
	if err != nil {
		return nil, err
	}

	password, err := c.collectPassword(cfg)
	if err != nil {
		return nil, err
	}

	tenantID, err := c.collectTenantID(cfg, adminService)
	if err != nil {
		return nil, err
	}

	// Parse role
	role := models.UserRoleUser
	if cfg.Role != "" {
		role = models.UserRole(cfg.Role)
	}

	return &admin.UserCreateRequest{
		Email:    email,
		Password: password,
		Name:     name,
		TenantID: tenantID,
		Role:     role,
		IsActive: cfg.Active,
	}, nil
}

// collectEmail collects email from config or prompts user
func (c *Command) collectEmail(cfg *Config) (string, error) {
	email := cfg.Email
	if email == "" && cfg.Interactive {
		ctx := context.Background()
		reader := input.NewReader(os.Stdin, c.Cmd().OutOrStdout())
		emailField := input.NewStringField("Email", reader).
			Required().
			ValidateEmail()

		value, err := emailField.Prompt(ctx)
		if err != nil {
			return "", err
		}
		emailStr, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("unexpected type returned from email field")
		}
		email = emailStr
	}
	if email == "" {
		return "", fmt.Errorf("email is required")
	}
	return email, nil
}

// collectName collects name from config or prompts user
func (c *Command) collectName(cfg *Config) (string, error) {
	name := cfg.Name
	if name == "" && cfg.Interactive {
		ctx := context.Background()
		reader := input.NewReader(os.Stdin, c.Cmd().OutOrStdout())
		nameField := input.NewStringField("Full name", reader).
			Required().
			MinLength(1)

		value, err := nameField.Prompt(ctx)
		if err != nil {
			return "", err
		}
		nameStr, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("unexpected type returned from name field")
		}
		name = nameStr
	}
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	return name, nil
}

// collectPassword collects password from config or prompts user
func (c *Command) collectPassword(cfg *Config) (string, error) {
	password := cfg.Password
	if password == "" {
		ctx := context.Background()
		reader := input.NewReader(os.Stdin, c.Cmd().OutOrStdout())
		passwordField := input.NewPasswordField("Password", reader).
			ValidateStrength()

		value, err := passwordField.Prompt(ctx)
		if err != nil {
			return "", err
		}
		passwordStr, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("unexpected type returned from password field")
		}
		password = passwordStr
	}
	return password, nil
}

// collectTenantID collects tenant ID from config or prompts user and returns the actual tenant ID
func (c *Command) collectTenantID(cfg *Config, adminService *admin.Service) (string, error) {
	tenantIDOrSlug := cfg.Tenant

	// For interactive mode, use validation and prompt
	if tenantIDOrSlug == "" && cfg.Interactive {
		ctx := context.Background()
		reader := input.NewReader(os.Stdin, c.Cmd().OutOrStdout())

		// Create tenant validator
		tenantValidator := c.createTenantValidator(adminService)

		tenantField := input.NewStringField("Tenant ID or slug", reader).
			Required().
			MinLength(1).
			ValidateCustom(tenantValidator)

		value, err := tenantField.Prompt(ctx)
		if err != nil {
			return "", err
		}
		tenantIDStr, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("unexpected type returned from tenant field")
		}
		tenantIDOrSlug = tenantIDStr
	}

	if tenantIDOrSlug == "" {
		return "", fmt.Errorf("tenant ID is required")
	}

	// Convert tenant slug/ID to actual tenant ID for database operations
	tenant, err := adminService.GetTenant(context.Background(), tenantIDOrSlug)
	if err != nil {
		return "", fmt.Errorf("tenant not found: %w", err)
	}

	// Return the actual tenant ID for database operations
	return tenant.ID, nil
}

// createTenantValidator creates a validator function for tenant ID/slug validation
func (c *Command) createTenantValidator(adminService *admin.Service) func(string) error {
	return func(tenantID string) error {
		// Try to get the tenant
		_, err := adminService.GetTenant(context.Background(), tenantID)
		if err != nil {
			// Get available tenants to show in error message
			availableTenants, listErr := c.getAvailableTenants(adminService)
			if listErr != nil {
				return input.NewAnswerError(fmt.Sprintf("Tenant '%s' not found", tenantID))
			}

			if len(availableTenants) == 0 {
				return input.NewAnswerError("No tenants available. Please create a tenant first.")
			}

			return input.NewAnswerError(fmt.Sprintf("Tenant '%s' not found. Available tenants: %s",
				tenantID, strings.Join(availableTenants, ", ")))
		}
		return nil
	}
}

// getAvailableTenants retrieves a list of available tenant slugs
func (c *Command) getAvailableTenants(adminService *admin.Service) ([]string, error) {
	listReq := admin.TenantListRequest{
		Limit: 100, // Get up to 100 tenants for the error message
	}

	response, err := adminService.ListTenants(context.Background(), listReq)
	if err != nil {
		return nil, err
	}

	var slugs []string
	for _, tenant := range response.Tenants {
		slugs = append(slugs, tenant.Slug)
	}

	return slugs, nil
}

// printUserRequest prints user request information for dry run
func (c *Command) printUserRequest(req *admin.UserCreateRequest) {
	out := c.Cmd().OutOrStdout()

	fmt.Fprintf(out, "  Email:    %s\n", req.Email)
	fmt.Fprintf(out, "  Name:     %s\n", req.Name)
	fmt.Fprintf(out, "  Role:     %s\n", req.Role)
	fmt.Fprintf(out, "  Active:   %t\n", req.IsActive)
	fmt.Fprintf(out, "  Tenant:   %s\n", req.TenantID)
}

// printUserInfo prints user information in a formatted way
func (c *Command) printUserInfo(user *models.User, tenant *models.Tenant) {
	out := c.Cmd().OutOrStdout()

	fmt.Fprintf(out, "  ID:       %s\n", user.ID)
	fmt.Fprintf(out, "  Email:    %s\n", user.Email)
	fmt.Fprintf(out, "  Name:     %s\n", user.Name)
	fmt.Fprintf(out, "  Role:     %s\n", user.Role)
	fmt.Fprintf(out, "  Active:   %t\n", user.IsActive)
	if tenant != nil {
		fmt.Fprintf(out, "  Tenant:   %s (%s)\n", tenant.Name, tenant.Slug)
	} else {
		fmt.Fprintf(out, "  Tenant:   %s\n", user.TenantID)
	}
}
