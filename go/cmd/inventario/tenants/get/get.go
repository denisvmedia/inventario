package get

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/admin"
)

// Command represents the tenant get command
type Command struct {
	command.Base

	config Config
}

// New creates the tenant get command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("tenants.get", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "get <id-or-slug>",
		Short: "Get detailed tenant information",
		Long: `Get detailed information about a specific tenant by ID or slug.

This command displays comprehensive tenant information including associated
user count, settings, and metadata. It supports lookup by either tenant ID
or slug for convenience.

LOOKUP METHODS:
  • ID: Exact tenant ID match
  • Slug: Tenant slug match (case-sensitive)

OUTPUT FORMATS:
  • table: Human-readable formatted output (default)
  • json: JSON format for scripting

Examples:
  # Get tenant by slug
  inventario tenants get acme

  # Get tenant by ID
  inventario tenants get 550e8400-e29b-41d4-a716-446655440000

  # Output as JSON
  inventario tenants get acme --output=json
  inventario tenants get acme -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.getTenant(&c.config, dbConfig, args[0])
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// Output flags
	c.Cmd().Flags().StringVarP(&c.config.Output, "output", "o", c.config.Output, "Output format (table, json)")
}

// getTenant handles the tenant retrieval process
func (c *Command) getTenant(cfg *Config, dbConfig *shared.DatabaseConfig, idOrSlug string) error {
	out := c.Cmd().OutOrStdout()

	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	// Check if this is a memory database and reject it
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("tenant retrieval is not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	// Validate output format
	if cfg.Output != "table" && cfg.Output != "json" {
		return fmt.Errorf("invalid output format '%s'. Supported formats: table, json", cfg.Output)
	}

	// Validate input
	if strings.TrimSpace(idOrSlug) == "" {
		return fmt.Errorf("tenant ID or slug is required")
	}

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

	// Get tenant via service
	tenant, err := adminService.GetTenant(context.Background(), idOrSlug)
	if err != nil {
		return fmt.Errorf("failed to get tenant: %w", err)
	}

	// Get user count
	userCount, err := adminService.GetTenantUserCount(context.Background(), tenant.ID)
	if err != nil {
		return fmt.Errorf("failed to get user count: %w", err)
	}

	// Output results
	switch cfg.Output {
	case "json":
		return c.outputJSON(tenant, userCount)
	case "table":
		return c.outputTable(tenant, userCount)
	default:
		return fmt.Errorf("unsupported output format: %s", cfg.Output)
	}
}

// outputJSON outputs tenant information in JSON format
func (c *Command) outputJSON(tenant *models.Tenant, userCount int) error {
	output := map[string]any{
		"tenant":     tenant,
		"user_count": userCount,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// outputTable outputs tenant information in table format
func (c *Command) outputTable(tenant *models.Tenant, userCount int) error {
	out := c.Cmd().OutOrStdout()

	// Create table writer
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	// Print tenant information
	fmt.Fprintln(w, "FIELD\tVALUE")
	fmt.Fprintf(w, "ID\t%s\n", tenant.ID)
	fmt.Fprintf(w, "Name\t%s\n", tenant.Name)
	fmt.Fprintf(w, "Slug\t%s\n", tenant.Slug)

	if tenant.Domain != nil && *tenant.Domain != "" {
		fmt.Fprintf(w, "Domain\t%s\n", *tenant.Domain)
	} else {
		fmt.Fprintln(w, "Domain\t<not set>")
	}

	fmt.Fprintf(w, "Status\t%s\n", tenant.Status)
	fmt.Fprintf(w, "User Count\t%d\n", userCount)
	fmt.Fprintf(w, "Created\t%s\n", tenant.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "Updated\t%s\n", tenant.UpdatedAt.Format("2006-01-02 15:04:05"))

	// Flush table
	w.Flush()

	// Print settings if they exist
	if len(tenant.Settings) > 0 {
		fmt.Fprintln(out, "\nSettings:")
		settingsJSON, err := json.MarshalIndent(tenant.Settings, "", "  ")
		if err != nil {
			fmt.Fprintf(out, "Error formatting settings: %v\n", err)
		} else {
			fmt.Fprintln(out, string(settingsJSON))
		}
	}

	return nil
}
