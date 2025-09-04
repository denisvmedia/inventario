package create

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/admin"
)

// RefactoredCommand represents the refactored tenant create command using service layer
type RefactoredCommand struct {
	*shared.Base
}

// NewRefactored creates a new refactored tenant create command
func NewRefactored(dbConfig *shared.DatabaseConfig) *RefactoredCommand {
	return &RefactoredCommand{
		Base: shared.NewBase(dbConfig),
	}
}

// Cmd returns the cobra command for refactored tenant creation
func (c *RefactoredCommand) Cmd() *cobra.Command {
	cfg := &Config{}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new tenant (refactored version)",
		Long: `Create a new tenant in the system.

This command allows you to create a new tenant with the specified name, slug, and domain.
The slug will be auto-generated from the name if not provided.

Examples:
  # Interactive mode
  inventario tenants create

  # With flags
  inventario tenants create --name="Acme Corp" --slug="acme" --domain="acme.com"

  # Dry run to preview
  inventario tenants create --name="Test" --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Run(cfg)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&cfg.Name, "name", "", "Tenant name")
	cmd.Flags().StringVar(&cfg.Slug, "slug", "", "Tenant slug (auto-generated if not provided)")
	cmd.Flags().StringVar(&cfg.Domain, "domain", "", "Tenant domain")
	cmd.Flags().StringVar(&cfg.Status, "status", "active", "Tenant status (active, suspended, inactive)")
	cmd.Flags().StringVar(&cfg.Settings, "settings", "{}", "Tenant settings as JSON")
	cmd.Flags().BoolVar(&cfg.Default, "default", false, "Mark this tenant as the default tenant")
	cmd.Flags().BoolVar(&cfg.DryRun, "dry-run", false, "Show what would be created without actually creating")
	cmd.Flags().BoolVar(&cfg.NoInteractive, "no-interactive", false, "Disable interactive prompts")

	return cmd
}

// Run executes the refactored tenant creation command
func (c *RefactoredCommand) Run(cfg *Config) error {
	out := c.Cmd().OutOrStdout()

	// 1. Parse and validate CLI arguments
	if err := c.validateConfig(cfg); err != nil {
		return err
	}

	// 2. Create admin service
	adminService, err := admin.NewService(c.DatabaseConfig())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := adminService.Close(); closeErr != nil {
			fmt.Fprintf(out, "Warning: failed to close admin service: %v\n", closeErr)
		}
	}()

	// 3. Collect input (interactive or from flags)
	tenantReq, err := c.collectTenantInput(cfg)
	if err != nil {
		return err
	}

	// 4. Show operation info
	c.printOperationInfo(cfg, tenantReq)

	// 5. Handle dry run
	if cfg.DryRun {
		c.printDryRunInfo(tenantReq)
		return nil
	}

	// 6. Delegate to service
	createdTenant, err := adminService.CreateTenant(context.Background(), *tenantReq)
	if err != nil {
		return err
	}

	// 7. Format and output result
	return c.outputResult(createdTenant, cfg)
}

// validateConfig validates the command configuration
func (c *RefactoredCommand) validateConfig(cfg *Config) error {
	// Validate status
	if cfg.Status != "" {
		validStatuses := []string{"active", "suspended", "inactive"}
		valid := false
		for _, status := range validStatuses {
			if cfg.Status == status {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid status '%s'. Valid options: %s", cfg.Status, strings.Join(validStatuses, ", "))
		}
	}

	// Validate settings JSON
	if cfg.Settings != "" && cfg.Settings != "{}" {
		var settings map[string]any
		if err := json.Unmarshal([]byte(cfg.Settings), &settings); err != nil {
			return fmt.Errorf("invalid settings JSON: %w", err)
		}
	}

	return nil
}

// collectTenantInput collects tenant input from flags or interactive prompts
func (c *RefactoredCommand) collectTenantInput(cfg *Config) (*admin.TenantCreateRequest, error) {
	out := c.Cmd().OutOrStdout()

	// Use flags if provided, otherwise prompt interactively
	name := cfg.Name
	if name == "" && !cfg.NoInteractive {
		fmt.Fprintf(out, "Tenant name: ")
		var input string
		fmt.Scanln(&input)
		name = input
	}

	if name == "" {
		return nil, fmt.Errorf("tenant name is required")
	}

	// Auto-generate slug if not provided
	slug := cfg.Slug
	if slug == "" {
		slug = c.generateSlug(name)
	}

	// Parse settings
	var settings map[string]any
	if cfg.Settings != "" && cfg.Settings != "{}" {
		if err := json.Unmarshal([]byte(cfg.Settings), &settings); err != nil {
			return nil, fmt.Errorf("invalid settings JSON: %w", err)
		}
	}

	// Parse status
	status := models.TenantStatusActive
	if cfg.Status != "" {
		switch cfg.Status {
		case "active":
			status = models.TenantStatusActive
		case "suspended":
			status = models.TenantStatusSuspended
		case "inactive":
			status = models.TenantStatusInactive
		}
	}

	// Handle domain
	var domain *string
	if cfg.Domain != "" {
		domain = &cfg.Domain
	}

	return &admin.TenantCreateRequest{
		Name:     name,
		Slug:     slug,
		Domain:   domain,
		Status:   status,
		Settings: settings,
		Default:  cfg.Default,
	}, nil
}

// generateSlug generates a slug from the tenant name
func (c *RefactoredCommand) generateSlug(name string) string {
	// Simple slug generation - convert to lowercase and replace spaces with hyphens
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove special characters (basic implementation)
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// printOperationInfo prints information about the operation
func (c *RefactoredCommand) printOperationInfo(cfg *Config, req *admin.TenantCreateRequest) {
	out := c.Cmd().OutOrStdout()

	fmt.Fprintln(out, "=== CREATE TENANT ===")
	if cfg.DryRun {
		fmt.Fprintln(out, "Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Fprintln(out, "Mode: LIVE CREATION")
	}
	fmt.Fprintln(out)
}

// printDryRunInfo prints what would be created in dry run mode
func (c *RefactoredCommand) printDryRunInfo(req *admin.TenantCreateRequest) {
	out := c.Cmd().OutOrStdout()

	fmt.Fprintln(out, "Would create tenant:")
	fmt.Fprintf(out, "  Name:     %s\n", req.Name)
	fmt.Fprintf(out, "  Slug:     %s\n", req.Slug)
	if req.Domain != nil {
		fmt.Fprintf(out, "  Domain:   %s\n", *req.Domain)
	}
	fmt.Fprintf(out, "  Status:   %s\n", req.Status)
	if len(req.Settings) > 0 {
		settingsJSON, _ := json.MarshalIndent(req.Settings, "  ", "  ")
		fmt.Fprintf(out, "  Settings: %s\n", settingsJSON)
	}
	fmt.Fprintln(out, "\n💡 To perform the actual creation, run the command without --dry-run")
}

// outputResult formats and outputs the creation result
func (c *RefactoredCommand) outputResult(tenant *models.Tenant, cfg *Config) error {
	out := c.Cmd().OutOrStdout()

	fmt.Fprintln(out, "✅ Tenant created successfully!")
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

	if cfg.Default {
		fmt.Fprintln(out, "\n📌 This tenant has been marked as the default tenant for the system.")
	}

	return nil
}
