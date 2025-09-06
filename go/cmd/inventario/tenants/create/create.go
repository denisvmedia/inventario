package create

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/internal/input"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/admin"
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
	fmt.Fprintln(out, "=== CREATE TENANT ===")
	if cfg.DryRun {
		fmt.Fprintln(out, "Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Fprintln(out, "Mode: LIVE CREATION")
	}
	fmt.Fprintln(out)

	// 3. Collect tenant information and convert to service request
	tenantReq, err := c.collectTenantRequest(cfg)
	if err != nil {
		return fmt.Errorf("failed to collect tenant information: %w", err)
	}

	// 4. Handle dry run
	if cfg.DryRun {
		fmt.Fprintln(out, "Would create tenant:")
		c.printTenantRequest(tenantReq)
		fmt.Fprintln(out, "\n💡 To perform the actual creation, run the command without --dry-run")
		return nil
	}

	// 5. Delegate to service
	createdTenant, err := adminService.CreateTenant(context.Background(), *tenantReq)
	if err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	// 6. Format and output result
	fmt.Fprintln(out, "✅ Tenant created successfully!")
	c.printTenantInfo(createdTenant)

	if cfg.Default {
		fmt.Fprintln(out, "\n📌 This tenant has been marked as the default tenant for the system.")
	}

	return nil
}

// collectTenantRequest collects tenant information and converts to service request
func (c *Command) collectTenantRequest(cfg *Config) (*admin.TenantCreateRequest, error) {
	ctx := context.Background()

	// Collect name
	if cfg.Name == "" && cfg.Interactive {
		reader := input.NewReader(os.Stdin, c.Cmd().OutOrStdout())
		nameField := input.NewStringField("Tenant name", reader).
			Required().
			MinLength(1)

		value, err := nameField.Prompt(ctx)
		if err != nil {
			return nil, err
		}
		name, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected type returned from name field")
		}
		cfg.Name = name
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("tenant name is required")
	}

	// Collect or generate slug
	if cfg.Slug == "" {
		slug, err := c.collectSlug(cfg, ctx)
		if err != nil {
			return nil, err
		}
		cfg.Slug = slug
	}

	// Collect domain (optional)
	if cfg.Domain == "" && cfg.Interactive {
		reader := input.NewReader(os.Stdin, c.Cmd().OutOrStdout())
		domainField := input.NewStringField("Tenant domain (optional)", reader).
			Optional()

		value, err := domainField.Prompt(ctx)
		if err != nil {
			return nil, err
		}
		domain, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected type returned from domain field")
		}
		cfg.Domain = domain
	}

	// Parse settings if provided
	var settings map[string]any
	if cfg.Settings != "" {
		if err := json.Unmarshal([]byte(cfg.Settings), &settings); err != nil {
			return nil, fmt.Errorf("invalid settings JSON: %w", err)
		}
	}

	// Parse status
	status := models.TenantStatusActive
	if cfg.Status != "" {
		status = models.TenantStatus(cfg.Status)
	}

	// Handle domain
	var domain *string
	if cfg.Domain != "" {
		domain = &cfg.Domain
	}

	return &admin.TenantCreateRequest{
		Name:     cfg.Name,
		Slug:     cfg.Slug,
		Domain:   domain,
		Status:   status,
		Settings: settings,
		Default:  cfg.Default,
	}, nil
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

// printTenantRequest prints tenant request information for dry run
func (c *Command) printTenantRequest(req *admin.TenantCreateRequest) {
	out := c.Cmd().OutOrStdout()

	fmt.Fprintf(out, "  Name:     %s\n", req.Name)
	fmt.Fprintf(out, "  Slug:     %s\n", req.Slug)
	if req.Domain != nil && *req.Domain != "" {
		fmt.Fprintf(out, "  Domain:   %s\n", *req.Domain)
	}
	fmt.Fprintf(out, "  Status:   %s\n", req.Status)
	if len(req.Settings) > 0 {
		settingsJSON, _ := json.MarshalIndent(req.Settings, "  ", "  ")
		fmt.Fprintf(out, "  Settings: %s\n", settingsJSON)
	}
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

// collectSlug collects or generates slug for the tenant
func (c *Command) collectSlug(cfg *Config, ctx context.Context) (string, error) {
	generatedSlug := c.generateSlug(cfg.Name)
	if !cfg.Interactive {
		return generatedSlug, nil
	}

	reader := input.NewReader(os.Stdin, c.Cmd().OutOrStdout())
	slugField := input.NewStringField("Tenant slug", reader).
		Default(generatedSlug).
		ValidateSlug()

	value, err := slugField.Prompt(ctx)
	if err != nil {
		return "", err
	}
	slug, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type returned from slug field")
	}
	return slug, nil
}
