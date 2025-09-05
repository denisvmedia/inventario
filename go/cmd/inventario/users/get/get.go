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

// Command represents the user get command
type Command struct {
	command.Base

	config Config
}

// New creates the user get command
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Load default configuration from struct tags
	shared.TryReadSection("users.get", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "get <id-or-email>",
		Short: "Get detailed user information",
		Long: `Get detailed information about a specific user by ID or email.

This command displays comprehensive user information including tenant details,
role, active status, and metadata. It supports lookup by either user ID
or email address for convenience.

LOOKUP METHODS:
  • ID: Exact user ID match
  • Email: User email address match (case-sensitive)

OUTPUT FORMATS:
  • table: Human-readable formatted output (default)
  • json: JSON format for scripting

Examples:
  # Get user by email
  inventario users get admin@acme.com

  # Get user by ID
  inventario users get 550e8400-e29b-41d4-a716-446655440000

  # Output as JSON
  inventario users get admin@acme.com --output=json
  inventario users get admin@acme.com -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.getUser(&c.config, dbConfig, args[0])
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// Output flags
	c.Cmd().Flags().StringVarP(&c.config.Output, "output", "o", c.config.Output, "Output format (table, json)")
}

// getUser handles the user retrieval process
func (c *Command) getUser(cfg *Config, dbConfig *shared.DatabaseConfig, idOrEmail string) error {
	out := c.Cmd().OutOrStdout()

	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	// Check if this is a memory database and reject it
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("user retrieval is not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	// Validate output format
	if cfg.Output != "table" && cfg.Output != "json" {
		return fmt.Errorf("invalid output format '%s'. Supported formats: table, json", cfg.Output)
	}

	// Validate input
	if strings.TrimSpace(idOrEmail) == "" {
		return fmt.Errorf("user ID or email is required")
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

	// Get user via service
	user, err := adminService.GetUser(context.Background(), idOrEmail)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Get tenant information
	tenant, err := adminService.GetTenant(context.Background(), user.TenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant information: %w", err)
	}

	// Output results
	switch cfg.Output {
	case "json":
		return c.outputJSON(user, tenant)
	case "table":
		return c.outputTable(user, tenant)
	default:
		return fmt.Errorf("unsupported output format: %s", cfg.Output)
	}
}

// outputJSON outputs user information in JSON format
func (c *Command) outputJSON(user *models.User, tenant *models.Tenant) error {
	output := map[string]any{
		"user":   user,
		"tenant": tenant,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// outputTable outputs user information in table format
func (c *Command) outputTable(user *models.User, tenant *models.Tenant) error {
	out := c.Cmd().OutOrStdout()

	// Create table writer
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	// Print user information
	fmt.Fprintln(w, "FIELD\tVALUE")
	fmt.Fprintf(w, "ID\t%s\n", user.ID)
	fmt.Fprintf(w, "Email\t%s\n", user.Email)
	fmt.Fprintf(w, "Name\t%s\n", user.Name)
	fmt.Fprintf(w, "Role\t%s\n", user.Role)
	fmt.Fprintf(w, "Active\t%t\n", user.IsActive)
	fmt.Fprintf(w, "Tenant\t%s (%s)\n", tenant.Name, tenant.Slug)
	fmt.Fprintf(w, "Tenant ID\t%s\n", user.TenantID)
	fmt.Fprintf(w, "Created\t%s\n", user.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "Updated\t%s\n", user.UpdatedAt.Format("2006-01-02 15:04:05"))

	if user.LastLoginAt != nil {
		fmt.Fprintf(w, "Last Login\t%s\n", user.LastLoginAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Fprintln(w, "Last Login\t<never>")
	}

	// Flush table
	w.Flush()

	return nil
}
