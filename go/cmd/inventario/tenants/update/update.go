package update

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

// Command represents the tenant update command
type Command struct {
	command.Base

	config Config
}

// New creates the tenant update command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("tenants.update", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "update <id-or-slug>",
		Short: "Update tenant properties",
		Long: `Update tenant properties with interactive prompts or command-line flags.

This command allows updating tenant properties including name, slug, domain,
status, and settings. It supports both interactive mode for guided updates
and flag-based mode for scripting.

UPDATABLE FIELDS:
  • Name: Human-readable tenant name
  • Slug: URL-friendly identifier (must be unique)
  • Domain: Associated domain name
  • Status: Tenant status (active, suspended, inactive)
  • Settings: JSON object with tenant-specific settings

INTERACTIVE MODE:
  Use --interactive to enable guided prompts for each field. Only fields
  that are changed will be updated.

VALIDATION:
  • Slug format: lowercase, alphanumeric, hyphens only
  • Slug uniqueness: must be unique across all tenants
  • Domain uniqueness: must be unique if provided
  • Settings: must be valid JSON if provided

Examples:
  # Update tenant name
  inventario tenants update acme --name="Acme Corporation Ltd"

  # Update multiple fields
  inventario tenants update acme --name="New Name" --domain="newdomain.com"

  # Interactive update
  inventario tenants update acme --interactive

  # Update settings
  inventario tenants update acme --settings='{"theme": "dark", "features": ["api"]}'

  # Preview changes
  inventario tenants update acme --name="New Name" --dry-run

  # Update by ID
  inventario tenants update 550e8400-e29b-41d4-a716-446655440000 --status=suspended`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.updateTenant(&c.config, dbConfig, args[0])
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// Dry run flag
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)

	// Tenant update flags
	c.Cmd().Flags().StringVar(&c.config.Name, "name", c.config.Name, "Update tenant name")
	c.Cmd().Flags().StringVar(&c.config.Slug, "slug", c.config.Slug, "Update tenant slug")
	c.Cmd().Flags().StringVar(&c.config.Domain, "domain", c.config.Domain, "Update tenant domain")
	c.Cmd().Flags().StringVar(&c.config.Status, "status", c.config.Status, "Update tenant status (active, suspended, inactive)")
	c.Cmd().Flags().StringVar(&c.config.Settings, "settings", c.config.Settings, "Update tenant settings as JSON")

	// Command behavior flags
	c.Cmd().Flags().BoolVar(&c.config.Interactive, "interactive", c.config.Interactive, "Enable interactive prompts for updates")

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

// updateTenant handles the tenant update process
func (c *Command) updateTenant(cfg *Config, dbConfig *shared.DatabaseConfig, idOrSlug string) error {
	out := c.Cmd().OutOrStdout()

	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	// Check if this is a memory database and reject it
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("tenant update is not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	// Validate input
	if strings.TrimSpace(idOrSlug) == "" {
		return fmt.Errorf("tenant ID or slug is required")
	}

	fmt.Fprintln(out, "=== UPDATE TENANT ===")
	fmt.Fprintf(out, "Database: %s\n", dbConfig.DBDSN)
	fmt.Fprintf(out, "Target: %s\n", idOrSlug)
	if cfg.DryRun {
		fmt.Fprintln(out, "Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Fprintln(out, "Mode: LIVE UPDATE")
	}
	fmt.Fprintln(out)

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

	// Find the tenant to update
	originalTenant, err := c.findTenant(tenantRegistry, idOrSlug)
	if err != nil {
		return fmt.Errorf("failed to find tenant: %w", err)
	}

	fmt.Fprintf(out, "Found tenant: %s (%s)\n\n", originalTenant.Name, originalTenant.Slug)

	// Collect updates
	updatedTenant, hasChanges, err := c.collectUpdates(cfg, originalTenant)
	if err != nil {
		return fmt.Errorf("failed to collect updates: %w", err)
	}

	if !hasChanges {
		fmt.Fprintln(out, "No changes specified.")
		return nil
	}

	// Validate updated tenant data
	if err := updatedTenant.ValidateWithContext(context.Background()); err != nil {
		return fmt.Errorf("tenant validation failed: %w", err)
	}

	if cfg.DryRun {
		// Show what would be updated
		fmt.Fprintln(out, "Would update tenant with:")
		c.printChanges(originalTenant, updatedTenant)
		fmt.Fprintln(out, "\n💡 To perform the actual update, run the command without --dry-run")
		return nil
	}

	// Update the tenant
	finalTenant, err := tenantRegistry.Update(context.Background(), *updatedTenant)
	if err != nil {
		return fmt.Errorf("failed to update tenant: %w", err)
	}

	fmt.Fprintln(out, "✅ Tenant updated successfully!")
	c.printTenantInfo(finalTenant)

	return nil
}

// findTenant tries to find a tenant by ID or slug
func (c *Command) findTenant(registry *postgres.TenantRegistry, idOrSlug string) (*models.Tenant, error) {
	// Try by ID first
	tenant, err := registry.Get(context.Background(), idOrSlug)
	if err == nil {
		return tenant, nil
	}

	// Try by slug
	tenant, err = registry.GetBySlug(context.Background(), idOrSlug)
	if err != nil {
		return nil, fmt.Errorf("tenant '%s' not found (tried both ID and slug)", idOrSlug)
	}

	return tenant, nil
}

// collectUpdates collects updates from flags and interactive prompts
func (c *Command) collectUpdates(cfg *Config, original *models.Tenant) (*models.Tenant, bool, error) {
	updated := *original // Copy original tenant
	hasChanges := false

	// Update name
	if nameChanged, err := c.updateName(cfg, &updated, original); err != nil {
		return nil, false, err
	} else if nameChanged {
		hasChanges = true
	}

	// Update slug
	if slugChanged, err := c.updateSlug(cfg, &updated, original); err != nil {
		return nil, false, err
	} else if slugChanged {
		hasChanges = true
	}

	// Update domain
	if domainChanged, err := c.updateDomain(cfg, &updated, original); err != nil {
		return nil, false, err
	} else if domainChanged {
		hasChanges = true
	}

	// Update status
	if statusChanged, err := c.updateStatus(cfg, &updated, original); err != nil {
		return nil, false, err
	} else if statusChanged {
		hasChanges = true
	}

	// Update settings
	if settingsChanged, err := c.updateSettings(cfg, &updated, original); err != nil {
		return nil, false, err
	} else if settingsChanged {
		hasChanges = true
	}

	return &updated, hasChanges, nil
}

// updateName handles name field updates
func (c *Command) updateName(cfg *Config, updated, original *models.Tenant) (bool, error) {
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

// updateSlug handles slug field updates
func (c *Command) updateSlug(cfg *Config, updated, original *models.Tenant) (bool, error) {
	if cfg.Slug == "" && !cfg.Interactive {
		return false, nil
	}

	newSlug := cfg.Slug
	if cfg.Interactive {
		slug, err := c.promptForUpdate("Slug", original.Slug, cfg.Slug)
		if err != nil {
			return false, err
		}
		newSlug = slug
	}

	if newSlug != "" && newSlug != original.Slug {
		if !c.isValidSlug(newSlug) {
			return false, fmt.Errorf("invalid slug format. Slug must contain only lowercase letters, numbers, and hyphens")
		}
		updated.Slug = newSlug
		return true, nil
	}
	return false, nil
}

// updateDomain handles domain field updates
func (c *Command) updateDomain(cfg *Config, updated, original *models.Tenant) (bool, error) {
	if cfg.Domain == "" && !cfg.Interactive {
		return false, nil
	}

	newDomain := cfg.Domain
	if cfg.Interactive {
		currentDomain := ""
		if original.Domain != nil {
			currentDomain = *original.Domain
		}
		domain, err := c.promptForUpdate("Domain", currentDomain, cfg.Domain)
		if err != nil {
			return false, err
		}
		newDomain = domain
	}

	currentDomain := ""
	if original.Domain != nil {
		currentDomain = *original.Domain
	}

	if newDomain != currentDomain {
		if newDomain == "" {
			updated.Domain = nil
		} else {
			updated.Domain = &newDomain
		}
		return true, nil
	}
	return false, nil
}

// updateStatus handles status field updates
func (c *Command) updateStatus(cfg *Config, updated, original *models.Tenant) (bool, error) {
	if cfg.Status == "" && !cfg.Interactive {
		return false, nil
	}

	newStatus := cfg.Status
	if cfg.Interactive {
		status, err := c.promptForUpdate("Status", string(original.Status), cfg.Status)
		if err != nil {
			return false, err
		}
		newStatus = status
	}

	if newStatus != "" && newStatus != string(original.Status) {
		validStatuses := []string{"active", "suspended", "inactive"}
		valid := false
		for _, status := range validStatuses {
			if newStatus == status {
				valid = true
				break
			}
		}
		if !valid {
			return false, fmt.Errorf("invalid status '%s'. Valid statuses: %s", newStatus, strings.Join(validStatuses, ", "))
		}
		updated.Status = models.TenantStatus(newStatus)
		return true, nil
	}
	return false, nil
}

// updateSettings handles settings field updates
func (c *Command) updateSettings(cfg *Config, updated, original *models.Tenant) (bool, error) {
	if cfg.Settings == "" && !cfg.Interactive {
		return false, nil
	}

	newSettings := cfg.Settings
	if cfg.Interactive {
		currentSettings := ""
		if original.Settings != nil {
			settingsJSON, _ := json.Marshal(original.Settings)
			currentSettings = string(settingsJSON)
		}
		settings, err := c.promptForUpdate("Settings (JSON)", currentSettings, cfg.Settings)
		if err != nil {
			return false, err
		}
		newSettings = settings
	}

	if newSettings != "" {
		var settings models.TenantSettings
		if err := json.Unmarshal([]byte(newSettings), &settings); err != nil {
			return false, fmt.Errorf("invalid settings JSON: %w", err)
		}
		updated.Settings = settings
		return true, nil
	}
	return false, nil
}

// isValidSlug validates slug format
func (c *Command) isValidSlug(slug string) bool {
	// Slug should be lowercase, alphanumeric, and hyphens only
	matched, _ := regexp.MatchString(`^[a-z0-9-]+$`, slug)
	return matched && !strings.HasPrefix(slug, "-") && !strings.HasSuffix(slug, "-")
}

// promptForUpdate prompts for a field update
func (c *Command) promptForUpdate(fieldName, currentValue, flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	out := c.Cmd().OutOrStdout()
	fmt.Fprintf(out, "%s [%s]: ", fieldName, currentValue)
	var input string
	fmt.Scanln(&input)

	if input == "" {
		return currentValue, nil
	}

	return input, nil
}

// printChanges shows what changes would be made
func (c *Command) printChanges(original, updated *models.Tenant) {
	if original.Name != updated.Name {
		fmt.Printf("  Name: %s → %s\n", original.Name, updated.Name)
	}
	if original.Slug != updated.Slug {
		fmt.Printf("  Slug: %s → %s\n", original.Slug, updated.Slug)
	}

	originalDomain := ""
	if original.Domain != nil {
		originalDomain = *original.Domain
	}
	updatedDomain := ""
	if updated.Domain != nil {
		updatedDomain = *updated.Domain
	}
	if originalDomain != updatedDomain {
		fmt.Printf("  Domain: %s → %s\n", originalDomain, updatedDomain)
	}

	if original.Status != updated.Status {
		fmt.Printf("  Status: %s → %s\n", original.Status, updated.Status)
	}

	// Settings comparison is complex, so just show if they changed
	originalSettings, _ := json.Marshal(original.Settings)
	updatedSettings, _ := json.Marshal(updated.Settings)
	if string(originalSettings) != string(updatedSettings) {
		fmt.Println("  Settings: <updated>")
	}
}

// printTenantInfo prints tenant information in a formatted way
func (c *Command) printTenantInfo(tenant *models.Tenant) {
	fmt.Printf("  ID:       %s\n", tenant.ID)
	fmt.Printf("  Name:     %s\n", tenant.Name)
	fmt.Printf("  Slug:     %s\n", tenant.Slug)
	if tenant.Domain != nil && *tenant.Domain != "" {
		fmt.Printf("  Domain:   %s\n", *tenant.Domain)
	}
	fmt.Printf("  Status:   %s\n", tenant.Status)
	if len(tenant.Settings) > 0 {
		settingsJSON, _ := json.MarshalIndent(tenant.Settings, "  ", "  ")
		fmt.Printf("  Settings: %s\n", settingsJSON)
	}
}
