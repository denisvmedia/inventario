package list

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/postgres"
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

	// Build filter criteria
	filters, err := c.buildFilters(cfg, tenantRegistry)
	if err != nil {
		return fmt.Errorf("failed to build filters: %w", err)
	}

	// Get users with filtering and pagination
	users, err := c.getFilteredUsers(userRegistry, filters, cfg.Limit, cfg.Offset)
	if err != nil {
		return fmt.Errorf("failed to retrieve users: %w", err)
	}

	// Get total count for pagination info
	totalCount, err := c.getTotalCount(userRegistry, filters)
	if err != nil {
		return fmt.Errorf("failed to get total count: %w", err)
	}

	// Get tenant information for display
	tenantMap, err := c.getTenantMap(tenantRegistry, users)
	if err != nil {
		return fmt.Errorf("failed to get tenant information: %w", err)
	}

	// Output results
	switch cfg.Output {
	case "json":
		return c.outputJSON(users, totalCount, cfg, tenantMap)
	case "table":
		return c.outputTable(users, totalCount, cfg, tenantMap)
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

// buildFilters builds filter criteria
func (c *Command) buildFilters(cfg *Config, tenantRegistry *postgres.TenantRegistry) (map[string]any, error) {
	filters := make(map[string]any)

	// Tenant filter - resolve tenant slug to ID if needed
	if cfg.Tenant != "" {
		// Try to get tenant by ID first, then by slug
		tenant, err := tenantRegistry.Get(context.Background(), cfg.Tenant)
		if err != nil {
			// Try by slug
			tenant, err = tenantRegistry.GetBySlug(context.Background(), cfg.Tenant)
			if err != nil {
				return nil, fmt.Errorf("tenant '%s' not found (tried both ID and slug)", cfg.Tenant)
			}
		}
		filters["tenant_id"] = tenant.ID
	}

	// Role filter
	if cfg.Role != "" {
		filters["role"] = models.UserRole(cfg.Role)
	}

	// Active filter
	if cfg.Active != "" {
		active, _ := strconv.ParseBool(cfg.Active)
		filters["active"] = active
	}

	// Search filter
	if cfg.Search != "" {
		filters["search"] = cfg.Search
	}

	return filters, nil
}

// getFilteredUsers retrieves users with filtering and pagination
func (c *Command) getFilteredUsers(registry *postgres.UserRegistry, filters map[string]any, limit, offset int) ([]*models.User, error) {
	// For now, we'll get all users and filter in memory
	// In a production system, this would be done at the database level
	allUsers, err := registry.List(context.Background())
	if err != nil {
		return nil, err
	}

	// Apply filters
	var filteredUsers []*models.User
	for _, user := range allUsers {
		if c.matchesFilters(user, filters) {
			filteredUsers = append(filteredUsers, user)
		}
	}

	// Apply pagination
	start := offset
	if start > len(filteredUsers) {
		start = len(filteredUsers)
	}

	end := start + limit
	if end > len(filteredUsers) {
		end = len(filteredUsers)
	}

	return filteredUsers[start:end], nil
}

// matchesFilters checks if a user matches the given filters
func (c *Command) matchesFilters(user *models.User, filters map[string]any) bool {
	// Tenant filter
	if tenantID, ok := filters["tenant_id"]; ok {
		if tenantIDValue, ok := tenantID.(string); ok && user.TenantID != tenantIDValue {
			return false
		}
	}

	// Role filter
	if role, ok := filters["role"]; ok {
		if roleValue, ok := role.(models.UserRole); ok && user.Role != roleValue {
			return false
		}
	}

	// Active filter
	if active, ok := filters["active"]; ok {
		if activeValue, ok := active.(bool); ok && user.IsActive != activeValue {
			return false
		}
	}

	// Search filter (case-insensitive)
	if search, ok := filters["search"]; ok {
		searchStr := strings.ToLower(search.(string))
		if !strings.Contains(strings.ToLower(user.Email), searchStr) &&
			!strings.Contains(strings.ToLower(user.Name), searchStr) {
			return false
		}
	}

	return true
}

// getTotalCount gets the total count of users matching filters
func (c *Command) getTotalCount(registry *postgres.UserRegistry, filters map[string]any) (int, error) {
	// For now, we'll get all users and count in memory
	// In a production system, this would be done at the database level
	allUsers, err := registry.List(context.Background())
	if err != nil {
		return 0, err
	}

	count := 0
	for _, user := range allUsers {
		if c.matchesFilters(user, filters) {
			count++
		}
	}

	return count, nil
}

// getTenantMap gets tenant information for display
func (c *Command) getTenantMap(tenantRegistry *postgres.TenantRegistry, users []*models.User) (map[string]*models.Tenant, error) {
	tenantMap := make(map[string]*models.Tenant)

	for _, user := range users {
		if _, exists := tenantMap[user.TenantID]; !exists {
			tenant, err := tenantRegistry.Get(context.Background(), user.TenantID)
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
	if len(users) == 0 {
		fmt.Println("No users found.")
		return nil
	}

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

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
	fmt.Printf("\nShowing %d of %d users", len(users), totalCount)
	if cfg.Offset > 0 || cfg.Offset+len(users) < totalCount {
		fmt.Printf(" (offset: %d, limit: %d)", cfg.Offset, cfg.Limit)
	}
	fmt.Println()

	return nil
}
