package create

import (
	"context"
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/denisvmedia/inventario/cmd/internal/command"
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

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
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

// promptForInput prompts the user for input with a default value
func (c *Command) promptForInput(prompt, defaultValue string) (string, error) {
	out := c.Cmd().OutOrStdout()

	if defaultValue != "" {
		fmt.Fprintf(out, "%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Fprintf(out, "%s: ", prompt)
	}

	var input string
	fmt.Scanln(&input)

	if input == "" && defaultValue != "" {
		return defaultValue, nil
	}

	return input, nil
}

// promptForPassword prompts for a password with hidden input
func (c *Command) promptForPassword(prompt string) (string, error) {
	out := c.Cmd().OutOrStdout()

	fmt.Fprintf(out, "%s: ", prompt)

	// Read password without echoing to terminal
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Fprintln(out) // Add newline after password input

	password := string(passwordBytes)
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	// Confirm password
	fmt.Fprintf(out, "Confirm %s: ", prompt)
	confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("failed to read password confirmation: %w", err)
	}
	fmt.Fprintln(out) // Add newline after confirmation

	if string(confirmBytes) != password {
		return "", fmt.Errorf("passwords do not match")
	}

	return password, nil
}

// collectUserRequest collects user information and converts to service request
func (c *Command) collectUserRequest(cfg *Config, adminService *admin.Service) (*admin.UserCreateRequest, error) {
	// Collect email
	email := cfg.Email
	if email == "" && cfg.Interactive {
		emailInput, err := c.promptForInput("Email", "")
		if err != nil {
			return nil, err
		}
		email = emailInput
	}
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	// Collect name
	name := cfg.Name
	if name == "" && cfg.Interactive {
		nameInput, err := c.promptForInput("Full name", "")
		if err != nil {
			return nil, err
		}
		name = nameInput
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	// Collect password
	password := cfg.Password
	if password == "" {
		passwordInput, err := c.promptForPassword("Password")
		if err != nil {
			return nil, err
		}
		password = passwordInput
	}

	// Get tenant ID from tenant slug/ID
	tenantID := cfg.Tenant
	if tenantID == "" {
		return nil, fmt.Errorf("tenant ID is required")
	}

	// Validate tenant exists
	_, err := adminService.GetTenant(context.Background(), tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
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
