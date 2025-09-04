package update

import (
	"context"
	"fmt"
	"strconv"
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

// Command represents the user update command
type Command struct {
	command.Base

	config Config
}

// New creates the user update command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("users.update", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "update <id-or-email>",
		Short: "Update user properties",
		Long: `Update user properties with interactive prompts or command-line flags.

This command allows updating user properties including email, name, role,
active status, tenant association, and password. It supports both interactive
mode for guided updates and flag-based mode for scripting.

UPDATABLE FIELDS:
  • Email: User email address (must be unique)
  • Name: Display name
  • Role: User role (admin, user)
  • Active: Whether the user is active (true, false)
  • Tenant: Move user to different tenant (by ID or slug)
  • Password: Change password (prompted securely)

INTERACTIVE MODE:
  Use --interactive to enable guided prompts for each field. Only fields
  that are changed will be updated.

VALIDATION:
  • Email format and uniqueness validation
  • Password strength validation
  • Role validation (admin, user)
  • Tenant existence validation

Examples:
  # Update user email
  inventario users update admin@acme.com --email="newadmin@acme.com"

  # Update multiple fields
  inventario users update admin@acme.com --name="New Name" --role="user"

  # Interactive update
  inventario users update admin@acme.com --interactive

  # Change password
  inventario users update admin@acme.com --password

  # Move user to different tenant
  inventario users update admin@acme.com --tenant="other-tenant"

  # Deactivate user
  inventario users update admin@acme.com --active=false

  # Preview changes
  inventario users update admin@acme.com --name="New Name" --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.updateUser(&c.config, dbConfig, args[0])
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// Dry run flag
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)

	// User update flags
	c.Cmd().Flags().StringVar(&c.config.Email, "email", c.config.Email, "Update user email")
	c.Cmd().Flags().StringVar(&c.config.Name, "name", c.config.Name, "Update user name")
	c.Cmd().Flags().StringVar(&c.config.Role, "role", c.config.Role, "Update user role (admin, user)")
	c.Cmd().Flags().StringVar(&c.config.Active, "active", c.config.Active, "Update active status (true, false)")
	c.Cmd().Flags().StringVar(&c.config.Tenant, "tenant", c.config.Tenant, "Move user to different tenant (ID or slug)")
	c.Cmd().Flags().BoolVar(&c.config.Password, "password", c.config.Password, "Change user password (prompted securely)")

	// Command behavior flags
	c.Cmd().Flags().BoolVar(&c.config.Interactive, "interactive", c.config.Interactive, "Enable interactive prompts for updates")

	// Handle no-interactive flag
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

// updateUser handles the user update process
func (c *Command) updateUser(cfg *Config, dbConfig *shared.DatabaseConfig, idOrEmail string) error {
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

	fmt.Println("=== UPDATE USER ===")
	fmt.Printf("Database: %s\n", dbConfig.DBDSN)
	fmt.Printf("Target: %s\n", idOrEmail)
	if cfg.DryRun {
		fmt.Println("Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Println("Mode: LIVE UPDATE")
	}
	fmt.Println()

	// Connect to database
	db, err := sqlx.Open("postgres", dbConfig.DBDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Create registries
	userRegistry := postgres.NewUserRegistry(db)
	tenantRegistry := postgres.NewTenantRegistry(db)

	// Find the user to update
	originalUser, err := c.findUser(userRegistry, idOrEmail)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	// Get current tenant info
	currentTenant, err := tenantRegistry.Get(context.Background(), originalUser.TenantID)
	if err != nil {
		return fmt.Errorf("failed to get current tenant: %w", err)
	}

	fmt.Printf("Found user: %s (%s)\n", originalUser.Name, originalUser.Email)
	fmt.Printf("Current tenant: %s (%s)\n\n", currentTenant.Name, currentTenant.Slug)

	// Collect updates
	updatedUser, hasChanges, err := c.collectUpdates(cfg, originalUser, tenantRegistry)
	if err != nil {
		return fmt.Errorf("failed to collect updates: %w", err)
	}

	if !hasChanges {
		fmt.Println("No changes specified.")
		return nil
	}

	// Validate updated user data
	if err := updatedUser.ValidateWithContext(context.Background()); err != nil {
		return fmt.Errorf("user validation failed: %w", err)
	}

	if cfg.DryRun {
		// Show what would be updated
		fmt.Println("Would update user with:")
		c.printChanges(originalUser, updatedUser, tenantRegistry)
		fmt.Println("\n💡 To perform the actual update, run the command without --dry-run")
		return nil
	}

	// Update the user
	finalUser, err := userRegistry.Update(context.Background(), *updatedUser)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	fmt.Println("✅ User updated successfully!")
	c.printUserInfo(finalUser, tenantRegistry)

	return nil
}

// findUser tries to find a user by ID or email
func (c *Command) findUser(registry *postgres.UserRegistry, idOrEmail string) (*models.User, error) {
	// Try by ID first
	user, err := registry.Get(context.Background(), idOrEmail)
	if err == nil {
		return user, nil
	}

	// Try by email - search across all users since we don't have tenant context
	users, err := registry.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	for _, user := range users {
		if user.Email == idOrEmail {
			return user, nil
		}
	}

	return nil, fmt.Errorf("user '%s' not found (tried both ID and email)", idOrEmail)
}

// collectUpdates collects updates from flags and interactive prompts
func (c *Command) collectUpdates(cfg *Config, original *models.User, tenantRegistry *postgres.TenantRegistry) (*models.User, bool, error) {
	updated := *original // Copy original user
	hasChanges := false

	// Update email
	if emailChanged, err := c.updateEmail(cfg, &updated, original); err != nil {
		return nil, false, err
	} else if emailChanged {
		hasChanges = true
	}

	// Update name
	if nameChanged, err := c.updateName(cfg, &updated, original); err != nil {
		return nil, false, err
	} else if nameChanged {
		hasChanges = true
	}

	// Update role
	if roleChanged, err := c.updateRole(cfg, &updated, original); err != nil {
		return nil, false, err
	} else if roleChanged {
		hasChanges = true
	}

	// Update active status
	if activeChanged, err := c.updateActive(cfg, &updated, original); err != nil {
		return nil, false, err
	} else if activeChanged {
		hasChanges = true
	}

	// Update tenant
	if tenantChanged, err := c.updateTenant(cfg, &updated, original, tenantRegistry); err != nil {
		return nil, false, err
	} else if tenantChanged {
		hasChanges = true
	}

	// Update password
	if passwordChanged, err := c.updatePassword(cfg, &updated); err != nil {
		return nil, false, err
	} else if passwordChanged {
		hasChanges = true
	}

	return &updated, hasChanges, nil
}

// updateEmail handles email field updates
func (c *Command) updateEmail(cfg *Config, updated, original *models.User) (bool, error) {
	if cfg.Email == "" && !cfg.Interactive {
		return false, nil
	}

	newEmail := cfg.Email
	if cfg.Interactive {
		email, err := c.promptForUpdate("Email", original.Email, cfg.Email)
		if err != nil {
			return false, err
		}
		newEmail = email
	}

	if newEmail != "" && newEmail != original.Email {
		// Basic email validation
		if !strings.Contains(newEmail, "@") || !strings.Contains(newEmail, ".") {
			return false, fmt.Errorf("invalid email format")
		}
		updated.Email = newEmail
		return true, nil
	}
	return false, nil
}

// updateName handles name field updates
func (c *Command) updateName(cfg *Config, updated, original *models.User) (bool, error) {
	if cfg.Name == "" && !cfg.Interactive {
		return false, nil
	}

	newName := cfg.Name
	if cfg.Interactive {
		name, err := c.promptForUpdate("Name", original.Name, cfg.Name)
		if err != nil {
			return false, err
		}
		newName = name
	}

	if newName != "" && newName != original.Name {
		updated.Name = newName
		return true, nil
	}
	return false, nil
}

// updateRole handles role field updates
func (c *Command) updateRole(cfg *Config, updated, original *models.User) (bool, error) {
	if cfg.Role == "" && !cfg.Interactive {
		return false, nil
	}

	newRole := cfg.Role
	if cfg.Interactive {
		role, err := c.promptForUpdate("Role", string(original.Role), cfg.Role)
		if err != nil {
			return false, err
		}
		newRole = role
	}

	if newRole != "" && newRole != string(original.Role) {
		// Validate role
		if newRole != "admin" && newRole != "user" {
			return false, fmt.Errorf("invalid role '%s'. Valid roles: admin, user", newRole)
		}
		updated.Role = models.UserRole(newRole)
		return true, nil
	}
	return false, nil
}

// updateActive handles active status field updates
func (c *Command) updateActive(cfg *Config, updated, original *models.User) (bool, error) {
	if cfg.Active == "" && !cfg.Interactive {
		return false, nil
	}

	newActive := cfg.Active
	if cfg.Interactive {
		active, err := c.promptForUpdate("Active", fmt.Sprintf("%t", original.IsActive), cfg.Active)
		if err != nil {
			return false, err
		}
		newActive = active
	}

	if newActive != "" {
		if newActive != "true" && newActive != "false" {
			return false, fmt.Errorf("invalid active value '%s'. Valid values: true, false", newActive)
		}
		newActiveValue, _ := strconv.ParseBool(newActive)
		if newActiveValue != original.IsActive {
			updated.IsActive = newActiveValue
			return true, nil
		}
	}
	return false, nil
}

// updateTenant handles tenant field updates
func (c *Command) updateTenant(cfg *Config, updated, original *models.User, tenantRegistry *postgres.TenantRegistry) (bool, error) {
	if cfg.Tenant == "" && !cfg.Interactive {
		return false, nil
	}

	newTenant := cfg.Tenant
	if cfg.Interactive {
		tenant, err := c.promptForUpdate("Tenant", original.TenantID, cfg.Tenant)
		if err != nil {
			return false, err
		}
		newTenant = tenant
	}

	if newTenant != "" && newTenant != original.TenantID {
		// Resolve tenant by ID or slug
		tenant, err := tenantRegistry.Get(context.Background(), newTenant)
		if err != nil {
			// Try by slug
			tenant, err = tenantRegistry.GetBySlug(context.Background(), newTenant)
			if err != nil {
				return false, fmt.Errorf("tenant '%s' not found (tried both ID and slug)", newTenant)
			}
		}
		updated.TenantID = tenant.ID
		return true, nil
	}
	return false, nil
}

// updatePassword handles password field updates
func (c *Command) updatePassword(cfg *Config, updated *models.User) (bool, error) {
	if !cfg.Password && !cfg.Interactive {
		return false, nil
	}

	if cfg.Password || (cfg.Interactive && c.promptYesNo("Change password?")) {
		newPassword, err := c.promptForPassword("New password")
		if err != nil {
			return false, err
		}
		if err := updated.SetPassword(newPassword); err != nil {
			return false, fmt.Errorf("failed to set password: %w", err)
		}
		return true, nil
	}
	return false, nil
}

// promptForUpdate prompts for a field update
func (c *Command) promptForUpdate(fieldName, currentValue, flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	fmt.Printf("%s [%s]: ", fieldName, currentValue)
	var input string
	fmt.Scanln(&input)

	if input == "" {
		return currentValue, nil
	}

	return input, nil
}

// promptYesNo prompts for a yes/no answer
func (c *Command) promptYesNo(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
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

	// Validate password
	if err := models.ValidatePassword(password); err != nil {
		return "", fmt.Errorf("password validation failed: %w", err)
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

// printChanges shows what changes would be made
func (c *Command) printChanges(original, updated *models.User, tenantRegistry *postgres.TenantRegistry) {
	if original.Email != updated.Email {
		fmt.Printf("  Email: %s → %s\n", original.Email, updated.Email)
	}
	if original.Name != updated.Name {
		fmt.Printf("  Name: %s → %s\n", original.Name, updated.Name)
	}
	if original.Role != updated.Role {
		fmt.Printf("  Role: %s → %s\n", original.Role, updated.Role)
	}
	if original.IsActive != updated.IsActive {
		fmt.Printf("  Active: %t → %t\n", original.IsActive, updated.IsActive)
	}
	if original.TenantID != updated.TenantID {
		// Get tenant names for display
		originalTenant, _ := tenantRegistry.Get(context.Background(), original.TenantID)
		updatedTenant, _ := tenantRegistry.Get(context.Background(), updated.TenantID)

		originalName := original.TenantID
		if originalTenant != nil {
			originalName = fmt.Sprintf("%s (%s)", originalTenant.Name, originalTenant.Slug)
		}

		updatedName := updated.TenantID
		if updatedTenant != nil {
			updatedName = fmt.Sprintf("%s (%s)", updatedTenant.Name, updatedTenant.Slug)
		}

		fmt.Printf("  Tenant: %s → %s\n", originalName, updatedName)
	}
	if original.PasswordHash != updated.PasswordHash {
		fmt.Println("  Password: <updated>")
	}
}

// printUserInfo prints user information in a formatted way
func (c *Command) printUserInfo(user *models.User, tenantRegistry *postgres.TenantRegistry) {
	fmt.Printf("  ID:       %s\n", user.ID)
	fmt.Printf("  Email:    %s\n", user.Email)
	fmt.Printf("  Name:     %s\n", user.Name)
	fmt.Printf("  Role:     %s\n", user.Role)
	fmt.Printf("  Active:   %t\n", user.IsActive)

	// Get tenant info
	tenant, err := tenantRegistry.Get(context.Background(), user.TenantID)
	if err == nil {
		fmt.Printf("  Tenant:   %s (%s)\n", tenant.Name, tenant.Slug)
	} else {
		fmt.Printf("  Tenant:   %s\n", user.TenantID)
	}
}
