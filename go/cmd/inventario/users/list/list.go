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

// Command represents the user list command
type Command struct {
	command.Base

	config Config
}

// New creates the user list command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("users.list", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "list",
		Short: "List users",
		Long: `List users with filtering and pagination options.

This command displays users in a table format by default, with options for
filtering by tenant, role, active status, and searching by email or name.

FILTERING:
  • Tenant: Filter by tenant ID or slug
  • Role: Filter by user role (admin, user)
  • Active: Filter by active status (true, false)
  • Search: Search by email or name (case-insensitive)

PAGINATION:
  • Limit: Maximum number of users to display (default: 50)
  • Offset: Number of users to skip (default: 0)

OUTPUT FORMATS:
  • table: Human-readable table format (default)
  • json: JSON format for scripting

Examples:
  # List all users
  inventario users list

  # List users in specific tenant
  inventario users list --tenant=acme

  # List admin users only
  inventario users list --role=admin

  # List active users only
  inventario users list --active=true

  # Search for users containing "john"
  inventario users list --search=john

  # Combine filters
  inventario users list --tenant=acme --role=admin --active=true

  # Output as JSON
  inventario users list --output=json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.listUsers(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// Dry run flag
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)

	// Filtering flags
	c.Cmd().Flags().StringVar(&c.config.Tenant, "tenant", c.config.Tenant, "Filter by tenant ID or slug")
	c.Cmd().Flags().StringVar(&c.config.Role, "role", c.config.Role, "Filter by user role (admin, user)")
	c.Cmd().Flags().StringVar(&c.config.Active, "active", c.config.Active, "Filter by active status (true, false)")
	c.Cmd().Flags().StringVar(&c.config.Search, "search", c.config.Search, "Search by email or name")

	// Pagination flags
	c.Cmd().Flags().IntVar(&c.config.Limit, "limit", c.config.Limit, "Maximum number of users to display")
	c.Cmd().Flags().IntVar(&c.config.Offset, "offset", c.config.Offset, "Number of users to skip")

	// Output flags
	c.Cmd().Flags().StringVar(&c.config.Output, "output", c.config.Output, "Output format (table, json)")
}

// listUsers handles the user listing process
func (c *Command) listUsers(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	// Check if this is a memory database and reject it
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("user listing is not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	// Validate output format
	if cfg.Output != "table" && cfg.Output != "json" {
		return fmt.Errorf("invalid output format '%s'. Supported formats: table, json", cfg.Output)
	}

	// Validate filters
	if err := c.validateFilters(cfg); err != nil {
		return err
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
	listReq := admin.UserListRequest{
		TenantID: cfg.Tenant,
		Role:     cfg.Role,
		Search:   cfg.Search,
		Limit:    cfg.Limit,
		Offset:   cfg.Offset,
	}
	if cfg.Active != "" {
		active := cfg.Active == "true"
		listReq.Active = &active
	}

	// Get users via service
	response, err := adminService.ListUsers(context.Background(), listReq)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	// Build tenant map for display
	tenantMap, err := c.buildTenantMap(adminService, response.Users)
	if err != nil {
		return fmt.Errorf("failed to get tenant information: %w", err)
	}

	// Output results
	switch cfg.Output {
	case "json":
		return c.outputJSON(response.Users, response.TotalCount, cfg, tenantMap)
	case "table":
		return c.outputTable(response.Users, response.TotalCount, cfg, tenantMap)
	default:
		return fmt.Errorf("unsupported output format: %s", cfg.Output)
	}
}

// validateFilters validates filter parameters
func (c *Command) validateFilters(cfg *Config) error {
	// Validate role filter
	if cfg.Role != "" {
		validRoles := []string{"admin", "user"}
		valid := false
		for _, role := range validRoles {
			if cfg.Role == role {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid role '%s'. Valid roles: %s", cfg.Role, strings.Join(validRoles, ", "))
		}
	}

	// Validate active filter
	if cfg.Active != "" {
		if cfg.Active != "true" && cfg.Active != "false" {
			return fmt.Errorf("invalid active value '%s'. Valid values: true, false", cfg.Active)
		}
	}

	// Validate pagination
	if cfg.Limit < 1 {
		return fmt.Errorf("limit must be at least 1")
	}
	if cfg.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}

	return nil
}

// outputJSON outputs users in JSON format
func (c *Command) outputJSON(users []*models.User, totalCount int, cfg *Config, tenantMap map[string]*models.Tenant) error {
	// Enhance users with tenant information
	type UserWithTenant struct {
		*models.User
		Tenant *models.Tenant `json:"tenant"`
	}

	var enhancedUsers []UserWithTenant
	for _, user := range users {
		enhancedUsers = append(enhancedUsers, UserWithTenant{
			User:   user,
			Tenant: tenantMap[user.TenantID],
		})
	}

	output := map[string]any{
		"users":       enhancedUsers,
		"total_count": totalCount,
		"limit":       cfg.Limit,
		"offset":      cfg.Offset,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// outputTable outputs users in table format
func (c *Command) outputTable(users []*models.User, totalCount int, cfg *Config, tenantMap map[string]*models.Tenant) error {
	out := c.Cmd().OutOrStdout()

	if len(users) == 0 {
		fmt.Fprintln(out, "No users found.")
		return nil
	}

	// Create table writer
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	// Print header
	fmt.Fprintln(w, "ID\tEMAIL\tNAME\tROLE\tACTIVE\tTENANT\tCREATED")

	// Print users
	for _, user := range users {
		tenant := tenantMap[user.TenantID]
		tenantName := tenant.Name
		if tenant.Slug != "<unknown>" {
			tenantName = fmt.Sprintf("%s (%s)", tenant.Name, tenant.Slug)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%t\t%s\t%s\n",
			user.ID,
			user.Email,
			user.Name,
			user.Role,
			user.IsActive,
			tenantName,
			user.CreatedAt.Format("2006-01-02 15:04"),
		)
	}

	// Flush table
	w.Flush()

	// Print pagination info
	fmt.Fprintf(out, "\nShowing %d of %d users", len(users), totalCount)
	if cfg.Offset > 0 || cfg.Offset+len(users) < totalCount {
		fmt.Fprintf(out, " (offset: %d, limit: %d)", cfg.Offset, cfg.Limit)
	}
	fmt.Fprintln(out)

	return nil
}

// buildTenantMap builds a map of tenant information for display
func (c *Command) buildTenantMap(adminService *admin.Service, users []*models.User) (map[string]*models.Tenant, error) {
	tenantMap := make(map[string]*models.Tenant)

	for _, user := range users {
		if _, exists := tenantMap[user.TenantID]; !exists {
			tenant, err := adminService.GetTenant(context.Background(), user.TenantID)
			if err != nil {
				// If we can't get tenant info, create a placeholder
				placeholder := &models.Tenant{
					Name: "<unknown>",
					Slug: "<unknown>",
				}
				placeholder.SetID(user.TenantID)
				tenantMap[user.TenantID] = placeholder
			} else {
				tenantMap[user.TenantID] = tenant
			}
		}
	}

	return tenantMap, nil
}
