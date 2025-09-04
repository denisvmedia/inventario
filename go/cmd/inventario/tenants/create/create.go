package create

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// Command represents the tenant creation command
type Command struct {
	command.Base

	config Config
}

// New creates the tenant creation command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("tenants.create", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "create",
		Short: "Create a new tenant",
		Long: `Create a new tenant with interactive prompts or command-line flags.

This command creates a new tenant in the PostgreSQL database with proper validation
and uniqueness checks. It supports both interactive mode (default) and flag-based mode.

REQUIRED FIELDS:
  • Name: Human-readable tenant name
  • Slug: URL-friendly identifier (auto-generated if not provided)

OPTIONAL FIELDS:
  • Domain: Associated domain name
  • Status: Tenant status (defaults to 'active')
  • Settings: JSON object with tenant-specific settings

INTERACTIVE MODE:
  By default, the command runs in interactive mode, prompting for all required
  information. Use --no-interactive to disable prompts and rely only on flags.

VALIDATION:
  • Slug format: lowercase, alphanumeric, hyphens only
  • Slug uniqueness: must be unique across all tenants
  • Domain uniqueness: must be unique if provided
  • Settings: must be valid JSON if provided

Examples:
  # Create tenant interactively
  inventario tenants create

  # Create tenant with flags
  inventario tenants create --name="Acme Corp" --slug="acme" --domain="acme.com"

  # Create default tenant for initial setup
  inventario tenants create --name="Default Org" --default

  # Preview tenant creation
  inventario tenants create --dry-run --name="Test Org"

  # Non-interactive mode
  inventario tenants create --no-interactive --name="Corp" --slug="corp"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.createTenant(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// Dry run flag
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)

	// Tenant configuration flags
	c.Cmd().Flags().StringVar(&c.config.Name, "name", c.config.Name, "Tenant name (required)")
	c.Cmd().Flags().StringVar(&c.config.Slug, "slug", c.config.Slug, "Tenant slug (auto-generated if not provided)")
	c.Cmd().Flags().StringVar(&c.config.Domain, "domain", c.config.Domain, "Tenant domain")
	c.Cmd().Flags().StringVar(&c.config.Status, "status", c.config.Status, "Tenant status (active, suspended, inactive)")
	c.Cmd().Flags().StringVar(&c.config.Settings, "settings", c.config.Settings, "Tenant settings as JSON")

	// Command behavior flags
	c.Cmd().Flags().BoolVar(&c.config.Interactive, "interactive", c.config.Interactive, "Enable interactive prompts")

	// Handle no-interactive flag by using a separate variable and post-processing
	var noInteractive bool
	c.Cmd().Flags().BoolVar(&noInteractive, "no-interactive", false, "Disable interactive prompts")
	c.Cmd().Flags().BoolVar(&c.config.Default, "default", c.config.Default, "Mark this tenant as the default tenant")

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

// createTenant handles the tenant creation process
func (c *Command) createTenant(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	// Check if this is a memory database and reject it
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("tenant creation is not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	fmt.Fprintln(out, "=== CREATE TENANT ===")
	fmt.Fprintf(out, "Database: %s\n", dbConfig.DBDSN)
	if cfg.DryRun {
		fmt.Fprintln(out, "Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Fprintln(out, "Mode: LIVE CREATION")
	}
	fmt.Fprintln(out)

	// Collect tenant information
	tenant, err := c.collectTenantInfo(cfg)
	if err != nil {
		return fmt.Errorf("failed to collect tenant information: %w", err)
	}

	// Validate tenant data
	if err := tenant.ValidateWithContext(context.Background()); err != nil {
		return fmt.Errorf("tenant validation failed: %w", err)
	}

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

	// Create tenant registry
	tenantRegistry := postgres.NewTenantRegistry(db)

	if cfg.DryRun {
		// Show what would be created
		fmt.Fprintln(out, "Would create tenant:")
		c.printTenantInfo(tenant)
		fmt.Fprintln(out, "\n💡 To perform the actual creation, run the command without --dry-run")
		return nil
	}

	// Create the tenant
	createdTenant, err := tenantRegistry.Create(context.Background(), *tenant)
	if err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	fmt.Fprintln(out, "✅ Tenant created successfully!")
	c.printTenantInfo(createdTenant)

	if cfg.Default {
		fmt.Fprintln(out, "\n📌 This tenant has been marked as the default tenant for the system.")
	}

	return nil
}

// collectTenantInfo collects tenant information from flags and interactive prompts
func (c *Command) collectTenantInfo(cfg *Config) (*models.Tenant, error) {
	tenant := &models.Tenant{
		Status: models.TenantStatus(cfg.Status),
	}

	// Collect name
	if cfg.Name == "" && cfg.Interactive {
		name, err := c.promptForInput("Tenant name", "")
		if err != nil {
			return nil, err
		}
		if name == "" {
			return nil, fmt.Errorf("tenant name is required")
		}
		cfg.Name = name
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("tenant name is required")
	}
	tenant.Name = cfg.Name

	// Collect or generate slug
	if cfg.Slug == "" {
		generatedSlug := c.generateSlug(cfg.Name)
		if cfg.Interactive {
			slug, err := c.promptForInput("Tenant slug", generatedSlug)
			if err != nil {
				return nil, err
			}
			cfg.Slug = slug
		} else {
			cfg.Slug = generatedSlug
		}
	}
	tenant.Slug = cfg.Slug

	// Collect domain (optional)
	if cfg.Domain == "" && cfg.Interactive {
		domain, err := c.promptForInput("Tenant domain (optional)", "")
		if err != nil {
			return nil, err
		}
		cfg.Domain = domain
	}
	if cfg.Domain != "" {
		tenant.Domain = &cfg.Domain
	}

	// Parse settings if provided
	if cfg.Settings != "" {
		var settings models.TenantSettings
		if err := json.Unmarshal([]byte(cfg.Settings), &settings); err != nil {
			return nil, fmt.Errorf("invalid settings JSON: %w", err)
		}
		tenant.Settings = settings
	}

	return tenant, nil
}

// generateSlug generates a URL-friendly slug from the tenant name
func (c *Command) generateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)

	// Replace spaces and special characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Remove leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	// Limit length
	if len(slug) > 50 {
		slug = slug[:50]
		slug = strings.TrimSuffix(slug, "-")
	}

	return slug
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

// printTenantInfo prints tenant information in a formatted way
func (c *Command) printTenantInfo(tenant *models.Tenant) {
	out := c.Cmd().OutOrStdout()

	fmt.Fprintf(out, "  ID:       %s\n", tenant.ID)
	fmt.Fprintf(out, "  Name:     %s\n", tenant.Name)
	fmt.Fprintf(out, "  Slug:     %s\n", tenant.Slug)
	if tenant.Domain != nil && *tenant.Domain != "" {
		fmt.Fprintf(out, "  Domain:   %s\n", *tenant.Domain)
	}
	fmt.Fprintf(out, "  Status:   %s\n", tenant.Status)
	if len(tenant.Settings) > 0 {
		settingsJSON, _ := json.MarshalIndent(tenant.Settings, "  ", "  ")
		fmt.Fprintf(out, "  Settings: %s\n", settingsJSON)
	}
}
