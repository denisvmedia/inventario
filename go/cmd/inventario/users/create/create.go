package create

import (
	"context"
	"fmt"
	"strings"
	"syscall"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/postgres"
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
	// Validate and setup
	if err := c.validateAndSetup(cfg, dbConfig); err != nil {
		return err
	}

	// Connect to database and create registries
	dbConn, err := c.connectAndCreateRegistries(dbConfig)
	if err != nil {
		return err
	}
	defer dbConn.DB.Close()

	// Collect and validate user information
	user, tenant, err := c.collectAndValidateUser(cfg, dbConn.TenantRegistry)
	if err != nil {
		return err
	}

	// Handle dry run or create user
	return c.handleUserCreation(cfg, user, tenant, dbConn.UserRegistry)
}

// validateAndSetup validates configuration and prints setup information
func (c *Command) validateAndSetup(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	// Check if this is a memory database and reject it
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("user creation is not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	fmt.Println("=== CREATE USER ===")
	fmt.Printf("Database: %s\n", dbConfig.DBDSN)
	if cfg.DryRun {
		fmt.Println("Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Println("Mode: LIVE CREATION")
	}
	fmt.Println()

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
	if cfg.DryRun {
		// Show what would be created
		fmt.Println("Would create user:")
		c.printUserInfo(user, tenant)
		fmt.Println("\n💡 To perform the actual creation, run the command without --dry-run")
		return nil
	}

	// Create the user
	createdUser, err := userRegistry.Create(context.Background(), *user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	fmt.Println("✅ User created successfully!")
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
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
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
	fmt.Printf("%s: ", prompt)

	// Read password without echoing to terminal
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Println() // Add newline after password input

	password := string(passwordBytes)
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	// Confirm password
	fmt.Printf("Confirm %s: ", prompt)
	confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("failed to read password confirmation: %w", err)
	}
	fmt.Println() // Add newline after confirmation

	if string(confirmBytes) != password {
		return "", fmt.Errorf("passwords do not match")
	}

	return password, nil
}

// printUserInfo prints user information in a formatted way
func (c *Command) printUserInfo(user *models.User, tenant *models.Tenant) {
	fmt.Printf("  ID:       %s\n", user.ID)
	fmt.Printf("  Email:    %s\n", user.Email)
	fmt.Printf("  Name:     %s\n", user.Name)
	fmt.Printf("  Role:     %s\n", user.Role)
	fmt.Printf("  Active:   %t\n", user.IsActive)
	fmt.Printf("  Tenant:   %s (%s)\n", tenant.Name, tenant.Slug)
}
