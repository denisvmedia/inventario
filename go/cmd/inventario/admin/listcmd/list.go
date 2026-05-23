// Package listcmd implements `inventario admin list-system-admins`.
// The package is named `listcmd` (not `list`) to avoid shadowing the
// stdlib `container/list` import in callers and to mirror the
// `users/list` / `users/deletecmd` naming used elsewhere in the tree.
package listcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/services/admin"
)

// Config carries the list-system-admins command's flags.
type Config struct {
	Output string `yaml:"output" env:"OUTPUT" env-default:"table"`
}

// Command is the `admin list-system-admins` cobra wrapper.
type Command struct {
	command.Base

	config Config
}

// New constructs the command with the supplied database config.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	shared.TryReadSection("admin.list_system_admins", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "list-system-admins",
		Short: "List all platform system administrators",
		Long: `List every grant in the system_admin_grants table joined to
its user row.

Output columns: id, email, name, granted_at.

The ` + "`granted_at`" + ` column is sourced directly from the
system_admin_grants row (#1784) and reflects the moment the grant was
issued — not the most recent write to the user row.

OUTPUT FORMATS:
  • table: human-readable formatted output (default)
  • json:  JSON format for scripting

Examples:
  inventario admin list-system-admins
  inventario admin list-system-admins --output=json`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.run(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	c.Cmd().Flags().StringVarP(&c.config.Output, "output", "o", c.config.Output, "Output format (table, json)")
}

func (c *Command) run(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	if err := dbConfig.Validate(); err != nil {
		return errxtrace.Wrap("database configuration error", err)
	}
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return errors.New("admin commands are not supported for memory databases; use PostgreSQL")
	}
	if cfg.Output != "table" && cfg.Output != "json" {
		return fmt.Errorf("invalid output format %q. Supported formats: table, json", cfg.Output)
	}

	adminService, err := admin.NewService(dbConfig)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := adminService.Close(); closeErr != nil {
			fmt.Fprintf(out, "Warning: failed to close admin service: %v\n", closeErr)
		}
	}()

	admins, err := adminService.ListSystemAdmins(c.Cmd().Context())
	if err != nil {
		return errxtrace.Wrap("failed to list system admins", err)
	}

	switch cfg.Output {
	case "json":
		return outputJSON(out, admins)
	default:
		return c.outputTable(admins)
	}
}

// listingJSON is the wire shape the --output=json variant emits.
// Intentionally reduced from the full *models.User row: list-system-admins
// answers "who currently holds the system-admin privilege and when was
// it granted", not "give me the user-account identity record". Fields
// dropped on purpose: created_at, updated_at, tenant_id, is_active —
// those belong to the user-account identity, not to the grant. Existing
// scripts that walk `id` / `email` / `name` keep working unchanged;
// scripts that need the dropped fields can join against `users` via the
// returned id.
type listingJSON struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	GrantedAt string `json:"granted_at"`
}

func outputJSON(out io.Writer, admins []*admin.SystemAdminListing) error {
	rows := make([]listingJSON, 0, len(admins))
	for _, a := range admins {
		rows = append(rows, listingJSON{
			ID:        a.User.ID,
			Email:     a.User.Email,
			Name:      a.User.Name,
			GrantedAt: a.GrantedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(rows)
}

func (c *Command) outputTable(admins []*admin.SystemAdminListing) error {
	out := c.Cmd().OutOrStdout()

	if len(admins) == 0 {
		fmt.Fprintln(out, "No system administrators are currently registered.")
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "ID\tEMAIL\tNAME\tGRANTED_AT")
	for _, a := range admins {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			a.User.ID,
			a.User.Email,
			a.User.Name,
			a.GrantedAt.Format("2006-01-02 15:04:05"),
		)
	}
	return nil
}
