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
	"github.com/denisvmedia/inventario/models"
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
		Long: `List every user with the system-admin flag set.

Output columns: id, email, name, granted_at.

NOTE: the ` + "`granted_at`" + ` column is sourced from the user row's
updated_at — the schema does not yet carry a dedicated grant timestamp.
For users whose only post-grant write was the grant itself this reads
correctly; for users edited later (name change, password reset) it
reflects the most recent write, not the grant moment. A dedicated
column will be added when the audit-trail UI lands (#1744 umbrella).

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

func outputJSON(out io.Writer, admins []*models.User) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(admins)
}

func (c *Command) outputTable(admins []*models.User) error {
	out := c.Cmd().OutOrStdout()

	if len(admins) == 0 {
		fmt.Fprintln(out, "No system administrators are currently registered.")
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "ID\tEMAIL\tNAME\tGRANTED_AT (proxy: updated_at)")
	for _, u := range admins {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			u.ID,
			u.Email,
			u.Name,
			u.UpdatedAt.Format("2006-01-02 15:04:05"),
		)
	}
	return nil
}
