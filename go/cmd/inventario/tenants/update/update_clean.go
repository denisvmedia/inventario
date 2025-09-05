package update

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/admin"
)

// Command represents the tenant update command
type Command struct {
	command.Base

	config Config
}

// New creates a new tenant update command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("tenants.update", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "update <tenant-id-or-slug>",
		Short: "Update an existing tenant",
		Long: `Update an existing tenant in the system.

This command allows you to update tenant information such as name, slug, domain, 
status, and settings.

Examples:
  # Interactive mode
  inventario tenants update acme

  # With flags
  inventario tenants update acme --name="Acme Corporation" --status=active

  # Dry run to preview changes
  inventario tenants update acme --name="New Name" --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.updateTenant(&c.config, dbConfig, args[0])
		},
	})

	return c
}

// updateTenant handles the tenant update process
func (c *Command) updateTenant(cfg *Config, dbConfig *shared.DatabaseConfig, idOrSlug string) error {
	out := c.Cmd().OutOrStdout()

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

	fmt.Fprintln(out, "=== UPDATE TENANT ===")
	fmt.Fprintf(out, "Target: %s\n", idOrSlug)
	if cfg.DryRun {
		fmt.Fprintln(out, "Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Fprintln(out, "Mode: LIVE UPDATE")
	}
	fmt.Fprintln(out)

	// Find the tenant to update
	originalTenant, err := adminService.GetTenant(context.Background(), idOrSlug)
	if err != nil {
		return fmt.Errorf("failed to find tenant: %w", err)
	}

	fmt.Fprintf(out, "Found tenant: %s (%s)\n\n", originalTenant.Name, originalTenant.Slug)

	// Collect updates and convert to service request
	updateReq, hasChanges, err := c.collectUpdateRequest(cfg, originalTenant)
	if err != nil {
		return fmt.Errorf("failed to collect updates: %w", err)
	}

	if !hasChanges {
		fmt.Fprintln(out, "No changes specified.")
		return nil
	}

	if cfg.DryRun {
		// Show what would be updated
		fmt.Fprintln(out, "Would update tenant with:")
		c.printUpdateRequest(updateReq)
		fmt.Fprintln(out, "\n💡 To perform the actual update, run the command without --dry-run")
		return nil
	}

	// Update the tenant via service
	finalTenant, err := adminService.UpdateTenant(context.Background(), idOrSlug, *updateReq)
	if err != nil {
		return fmt.Errorf("failed to update tenant: %w", err)
	}

	fmt.Fprintln(out, "✅ Tenant updated successfully!")
	c.printTenantInfo(finalTenant)

	return nil
}

// collectUpdateRequest collects updates and converts to service request
func (c *Command) collectUpdateRequest(cfg *Config, original *models.Tenant) (*admin.TenantUpdateRequest, bool, error) {
	req := &admin.TenantUpdateRequest{}
	hasChanges := false

	// Update name
	if cfg.Name != "" && cfg.Name != original.Name {
		req.Name = &cfg.Name
		hasChanges = true
	} else if cfg.Interactive {
		name, err := c.promptForUpdate("Name", original.Name, cfg.Name)
		if err != nil {
			return nil, false, err
		}
		if name != "" && name != original.Name {
			req.Name = &name
			hasChanges = true
		}
	}

	// Update slug
	if cfg.Slug != "" && cfg.Slug != original.Slug {
		req.Slug = &cfg.Slug
		hasChanges = true
	} else if cfg.Interactive {
		slug, err := c.promptForUpdate("Slug", original.Slug, cfg.Slug)
		if err != nil {
			return nil, false, err
		}
		if slug != "" && slug != original.Slug {
			req.Slug = &slug
			hasChanges = true
		}
	}

	// Update domain
	originalDomain := ""
	if original.Domain != nil {
		originalDomain = *original.Domain
	}
	if cfg.Domain != "" && cfg.Domain != originalDomain {
		req.Domain = &cfg.Domain
		hasChanges = true
	} else if cfg.Interactive {
		domain, err := c.promptForUpdate("Domain", originalDomain, cfg.Domain)
		if err != nil {
			return nil, false, err
		}
		if domain != originalDomain {
			req.Domain = &domain
			hasChanges = true
		}
	}

	// Update status
	if cfg.Status != "" && models.TenantStatus(cfg.Status) != original.Status {
		status := models.TenantStatus(cfg.Status)
		req.Status = &status
		hasChanges = true
	}

	// Update settings
	if cfg.Settings != "" {
		var settings map[string]any
		if err := json.Unmarshal([]byte(cfg.Settings), &settings); err != nil {
			return nil, false, fmt.Errorf("invalid settings JSON: %w", err)
		}
		req.Settings = settings
		hasChanges = true
	}

	return req, hasChanges, nil
}

// printUpdateRequest prints what would be updated in dry run mode
func (c *Command) printUpdateRequest(req *admin.TenantUpdateRequest) {
	out := c.Cmd().OutOrStdout()

	if req.Name != nil {
		fmt.Fprintf(out, "  Name:     %s\n", *req.Name)
	}
	if req.Slug != nil {
		fmt.Fprintf(out, "  Slug:     %s\n", *req.Slug)
	}
	if req.Domain != nil {
		fmt.Fprintf(out, "  Domain:   %s\n", *req.Domain)
	}
	if req.Status != nil {
		fmt.Fprintf(out, "  Status:   %s\n", *req.Status)
	}
	if req.Settings != nil {
		settingsJSON, _ := json.MarshalIndent(req.Settings, "  ", "  ")
		fmt.Fprintf(out, "  Settings: %s\n", settingsJSON)
	}
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
