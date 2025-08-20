package printcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/schema/bootstrap"
)

type Command struct {
	command.Base

	config Config
}

func New() *Command {
	c := &Command{}

	c.Base = command.NewBase(&cobra.Command{
		Use:   "print",
		Short: "Print bootstrap SQL migrations without applying them",
		Long: `Print all bootstrap SQL migrations that would be executed, with template
variables resolved, without actually applying them to the database.

This command is useful for reviewing the exact SQL statements that would be
executed before manually applying them or running the apply command. The output
can be redirected to a file for manual execution by a database administrator.

Template Variables:
  {{.Username}}             - Operational database username (default: inventario)
  {{.UsernameForMigrations}} - Migration database username (defaults to Username)

Examples:
  inventario migrate bootstrap print --db-dsn="postgres://admin:pass@localhost/inventario"
  inventario migrate bootstrap print --username=myapp --db-dsn="postgres://admin:pass@localhost/inventario"
  inventario migrate bootstrap print --username=myapp --username-for-migrations=migrator --db-dsn="postgres://admin:pass@localhost/inventario"

  # Save to file for manual execution
  inventario migrate bootstrap print --db-dsn="postgres://admin:pass@localhost/inventario" > bootstrap.sql

The printed SQL can be executed manually by a database administrator with
appropriate privileges.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.bootstrapPrint()
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	shared.TryReadSection("bootstrap", &c.config)
	shared.RegisterBootstrapFlags(c.Cmd(), &c.config.Username, &c.config.UsernameForMigrations)
}

// bootstrapPrint handles the bootstrap print subcommand
func (c *Command) bootstrapPrint() error {
	username := c.config.Username
	usernameForMigrations := c.config.UsernameForMigrations

	// Default usernameForMigrations to username if not provided
	if usernameForMigrations == "" {
		usernameForMigrations = username
	}

	// Create bootstrap migrator and print migrations
	migrator := bootstrap.New()
	templateData := bootstrap.TemplateData{
		Username:              username,
		UsernameForMigrations: usernameForMigrations,
	}

	err := migrator.Print(templateData)
	if err != nil {
		return fmt.Errorf("failed to print bootstrap migrations: %w", err)
	}

	return nil
}
