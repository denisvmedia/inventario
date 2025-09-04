package list

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

// Command represents the tenant list command
type Command struct {
	command.Base

	config Config
}

// New creates the tenant list command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("tenants.list", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "list",
		Short: "List tenants",
		Long: `List tenants with filtering and pagination options.

This command displays tenants in a table format by default, with options for
filtering by status, searching by name or slug, and controlling pagination.

FILTERING:
  • Status: Filter by tenant status (active, suspended, inactive)
  • Search: Search by tenant name or slug (case-insensitive)

PAGINATION:
  • Limit: Maximum number of tenants to display (default: 50)
  • Offset: Number of tenants to skip (default: 0)

OUTPUT FORMATS:
  • table: Human-readable table format (default)
  • json: JSON format for scripting

Examples:
  # List all tenants
  inventario tenants list

  # List only active tenants
  inventario tenants list --status=active

  # Search for tenants containing "acme"
  inventario tenants list --search=acme

  # List first 10 tenants
  inventario tenants list --limit=10

  # Get tenants 11-20
  inventario tenants list --limit=10 --offset=10

  # Output as JSON
  inventario tenants list --output=json

  # Combine filters
  inventario tenants list --status=active --search=corp --limit=5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.listTenants(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// Dry run flag
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)

	// Filtering flags
	c.Cmd().Flags().StringVar(&c.config.Status, "status", c.config.Status, "Filter by tenant status (active, suspended, inactive)")
	c.Cmd().Flags().StringVar(&c.config.Search, "search", c.config.Search, "Search by tenant name or slug")

	// Pagination flags
	c.Cmd().Flags().IntVar(&c.config.Limit, "limit", c.config.Limit, "Maximum number of tenants to display")
	c.Cmd().Flags().IntVar(&c.config.Offset, "offset", c.config.Offset, "Number of tenants to skip")

	// Output flags
	c.Cmd().Flags().StringVar(&c.config.Output, "output", c.config.Output, "Output format (table, json)")
}

// listTenants handles the tenant listing process
func (c *Command) listTenants(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	// Check if this is a memory database and reject it
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("tenant listing is not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	// Validate output format
	if cfg.Output != "table" && cfg.Output != "json" {
		return fmt.Errorf("invalid output format '%s'. Supported formats: table, json", cfg.Output)
	}

	// Validate status filter
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
			return fmt.Errorf("invalid status '%s'. Valid statuses: %s", cfg.Status, strings.Join(validStatuses, ", "))
		}
	}

	// Validate pagination
	if cfg.Limit < 1 {
		return fmt.Errorf("limit must be at least 1")
	}
	if cfg.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}

	// Create admin service
	adminService, err := admin.NewService(dbConfig)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := adminService.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close admin service: %v\n", closeErr)
		}
	}()

	// Build service request
	listReq := admin.TenantListRequest{
		Status: cfg.Status,
		Search: cfg.Search,
		Limit:  cfg.Limit,
		Offset: cfg.Offset,
	}

	// Get tenants via service
	response, err := adminService.ListTenants(context.Background(), listReq)
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}

	// Output results
	switch cfg.Output {
	case "json":
		return c.outputJSON(response.Tenants, response.TotalCount, cfg)
	case "table":
		return c.outputTable(response.Tenants, response.TotalCount, cfg)
	default:
		return fmt.Errorf("unsupported output format: %s", cfg.Output)
	}
}



// outputJSON outputs tenants in JSON format
func (c *Command) outputJSON(tenants []*models.Tenant, totalCount int, cfg *Config) error {
	output := map[string]any{
		"tenants":     tenants,
		"total_count": totalCount,
		"limit":       cfg.Limit,
		"offset":      cfg.Offset,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// outputTable outputs tenants in table format
func (c *Command) outputTable(tenants []*models.Tenant, totalCount int, cfg *Config) error {
	out := c.Cmd().OutOrStdout()

	if len(tenants) == 0 {
		fmt.Fprintln(out, "No tenants found.")
		return nil
	}

	// Create table writer
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	// Print header
	fmt.Fprintln(w, "ID\tNAME\tSLUG\tDOMAIN\tSTATUS\tCREATED")

	// Print tenants
	for _, tenant := range tenants {
		domain := ""
		if tenant.Domain != nil {
			domain = *tenant.Domain
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			tenant.ID,
			tenant.Name,
			tenant.Slug,
			domain,
			tenant.Status,
			tenant.CreatedAt.Format("2006-01-02 15:04"),
		)
	}

	// Flush table
	w.Flush()

	// Print pagination info
	fmt.Fprintf(out, "\nShowing %d of %d tenants", len(tenants), totalCount)
	if cfg.Offset > 0 || cfg.Offset+len(tenants) < totalCount {
		fmt.Fprintf(out, " (offset: %d, limit: %d)", cfg.Offset, cfg.Limit)
	}
	fmt.Fprintln(out)

	return nil
}
