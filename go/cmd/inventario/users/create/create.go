package create

import (
	"context"
	"fmt"
	"strings"
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

// validateAndSetup validates configuration and prints setup information
func (c *Command) validateAndSetup(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	// Check if this is a memory database and reject it
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("user creation is not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	fmt.Fprintln(out, "=== CREATE USER ===")
	fmt.Fprintf(out, "Database: %s\n", dbConfig.DBDSN)
	if cfg.DryRun {
		fmt.Fprintln(out, "Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Fprintln(out, "Mode: LIVE CREATION")
	}
	fmt.Fprintln(out)

	return c.validateBasicInputs(cfg)
}

// validateBasicInputs validates basic inputs before database connection
func (c *Command) validateBasicInputs(cfg *Config) error {
	// Do basic validation before connecting to database
	if cfg.Email == "" && !cfg.Interactive {
		return fmt.Errorf("email address is required")
	}
	if cfg.Tenant == "" && !cfg.Interactive {
		return fmt.Errorf("tenant is required")
	}
	if cfg.Password == "" && !cfg.Interactive {
		return fmt.Errorf("password is required in non-interactive mode")
	}

	// Validate email format if provided
	if cfg.Email != "" {
		if !strings.Contains(cfg.Email, "@") || !strings.Contains(cfg.Email, ".") {
			return fmt.Errorf("user validation failed: email must be in a valid format")
		}
	}

	// Validate role if provided
	if cfg.Role != "" {
		role := models.UserRole(cfg.Role)
		if err := role.Validate(); err != nil {
			return fmt.Errorf("user validation failed: %w", err)
		}
	}

	// Validate password if provided
	if cfg.Password != "" {
		if err := models.ValidatePassword(cfg.Password); err != nil {
			return fmt.Errorf("password validation failed: %w", err)
		}
	}

	return nil
}

// DatabaseConnection holds database connection and registries
type DatabaseConnection struct {
	DB             *sqlx.DB
	TenantRegistry *postgres.TenantRegistry
	UserRegistry   *postgres.UserRegistry
}

// connectAndCreateRegistries connects to database and creates registries
func (c *Command) connectAndCreateRegistries(dbConfig *shared.DatabaseConfig) (*DatabaseConnection, error) {
	// Connect to database
	db, err := sqlx.Open("postgres", dbConfig.DBDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create registries
	tenantRegistry := postgres.NewTenantRegistry(db)
	userRegistry := postgres.NewUserRegistry(db)

	return &DatabaseConnection{
		DB:             db,
		TenantRegistry: tenantRegistry,
		UserRegistry:   userRegistry,
	}, nil
}

// collectAndValidateUser collects and validates user information
func (c *Command) collectAndValidateUser(cfg *Config, tenantRegistry *postgres.TenantRegistry) (*models.User, *models.Tenant, error) {
	// Collect user information
	user, tenant, err := c.collectUserInfo(cfg, tenantRegistry)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to collect user information: %w", err)
	}

	// Validate user data
	if err := user.ValidateWithContext(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("user validation failed: %w", err)
	}

	return user, tenant, nil
}

// handleUserCreation handles dry run or actual user creation
func (c *Command) handleUserCreation(cfg *Config, user *models.User, tenant *models.Tenant, userRegistry *postgres.UserRegistry) error {
	out := c.Cmd().OutOrStdout()

	if cfg.DryRun {
		// Show what would be created
		fmt.Fprintln(out, "Would create user:")
		c.printUserInfo(user, tenant)
		fmt.Fprintln(out, "\n💡 To perform the actual creation, run the command without --dry-run")
		return nil
	}

	// Create the user
	createdUser, err := userRegistry.Create(context.Background(), *user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	fmt.Fprintln(out, "✅ User created successfully!")
	c.printUserInfo(createdUser, tenant)

	return nil
}

// collectUserInfo collects user information from flags and interactive prompts
func (c *Command) collectUserInfo(cfg *Config, tenantRegistry *postgres.TenantRegistry) (*models.User, *models.Tenant, error) {
	user := &models.User{
		Role:     models.UserRole(cfg.Role),
		IsActive: cfg.Active,
	}

	// Collect email
	if err := c.collectEmail(cfg); err != nil {
		return nil, nil, err
	}
	user.Email = cfg.Email

	// Collect and validate tenant
	tenant, err := c.collectTenant(cfg, tenantRegistry)
	if err != nil {
		return nil, nil, err
	}
	user.TenantID = tenant.ID

	// Collect name (optional, defaults to email)
	if cfg.Name == "" && cfg.Interactive {
		name, err := c.promptForInput("Display name", cfg.Email)
		if err != nil {
			return nil, nil, err
		}
		cfg.Name = name
	}
	if cfg.Name == "" {
		cfg.Name = cfg.Email
	}
	user.Name = cfg.Name

	// Collect password
	if cfg.Password == "" {
		if !cfg.Interactive {
			return nil, nil, fmt.Errorf("password is required in non-interactive mode")
		}
		password, err := c.promptForPassword("Password")
		if err != nil {
			return nil, nil, err
		}
		cfg.Password = password
	}

	// Validate and set password
	if err := models.ValidatePassword(cfg.Password); err != nil {
		return nil, nil, fmt.Errorf("password validation failed: %w", err)
	}
	if err := user.SetPassword(cfg.Password); err != nil {
		return nil, nil, fmt.Errorf("failed to set password: %w", err)
	}

	// Collect role if interactive
	if cfg.Interactive && cfg.Role == "user" {
		role, err := c.promptForInput("Role (admin/user)", "user")
		if err != nil {
			return nil, nil, err
		}
		if role != "" {
			cfg.Role = role
			user.Role = models.UserRole(role)
		}
	}

	return user, tenant, nil
}

// collectEmail collects and validates email address
func (c *Command) collectEmail(cfg *Config) error {
	if cfg.Email == "" && cfg.Interactive {
		email, err := c.promptForInput("Email address", "")
		if err != nil {
			return err
		}
		if email == "" {
			return fmt.Errorf("email address is required")
		}
		cfg.Email = email
	}
	if cfg.Email == "" {
		return fmt.Errorf("email address is required")
	}
	return nil
}

// collectTenant collects and validates tenant
func (c *Command) collectTenant(cfg *Config, tenantRegistry *postgres.TenantRegistry) (*models.Tenant, error) {
	if cfg.Tenant == "" && cfg.Interactive {
		tenant, err := c.promptForInput("Tenant ID or slug", "")
		if err != nil {
			return nil, err
		}
		if tenant == "" {
			return nil, fmt.Errorf("tenant is required")
		}
		cfg.Tenant = tenant
	}
	if cfg.Tenant == "" {
		return nil, fmt.Errorf("tenant is required")
	}

	// Look up tenant by ID or slug
	tenant, err := tenantRegistry.Get(context.Background(), cfg.Tenant)
	if err != nil {
		// Try by slug
		tenant, err = tenantRegistry.GetBySlug(context.Background(), cfg.Tenant)
		if err != nil {
			return nil, fmt.Errorf("tenant '%s' not found (tried both ID and slug)", cfg.Tenant)
		}
	}
	return tenant, nil
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
	tenantID := cfg.TenantID
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
