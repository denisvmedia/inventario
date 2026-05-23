// Package disable implements `inventario backoffice mfa disable`.
//
// Wipes a back-office user's MFA enrollment row. Idempotent: disabling
// a non-enrolled user is a no-op success. The --confirm flag is
// required to guard against accidental invocations — a stray
// `inventario backoffice mfa disable --email admin` without --confirm
// returns an error rather than wiping the row.
//
// Disabling a back-office user with MFAEnforced=true puts the account
// into the "501 backoffice.mfa_not_implemented" branch of the login
// flow — the user cannot sign in until a fresh `setup` runs. This is
// intentional: an operator who disables MFA without immediately
// re-enrolling has explicitly broken the account.
package disable

import (
	"errors"
	"fmt"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/services/backoffice"
)

// Config carries the disable command's flags.
type Config struct {
	Email   string `yaml:"email" env:"EMAIL"`
	Confirm bool   `yaml:"confirm" env:"CONFIRM"`
}

// Command is the cobra wrapper around `backoffice mfa disable`.
type Command struct {
	command.Base

	config Config
}

// New constructs the command with the supplied database config.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	shared.TryReadSection("backoffice.mfa.disable", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "disable",
		Short: "Wipe a back-office user's MFA enrollment",
		Long: `Remove a back-office user's TOTP enrollment and backup codes.

REQUIRES --confirm to actually perform the wipe — a stray invocation
without --confirm returns an error rather than mutating data.

If the user has mfa_enforced=true (the Phase 4 default for every
freshly-bootstrapped back-office user), they will be unable to sign
in until a fresh ` + "`inventario backoffice mfa setup`" + ` runs. This is
intentional: an operator who disables MFA without re-enrolling has
explicitly broken the account.

Idempotent: disabling a non-enrolled user is a no-op success.

Examples:
  inventario backoffice mfa disable --email admin@example.com --confirm`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.run(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	c.Cmd().Flags().StringVar(&c.config.Email, "email", c.config.Email, "Back-office user email (required)")
	c.Cmd().Flags().BoolVar(&c.config.Confirm, "confirm", c.config.Confirm, "Required: confirm the destructive operation")
}

func (c *Command) run(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	if err := dbConfig.Validate(); err != nil {
		return errxtrace.Wrap("database configuration error", err)
	}

	email := strings.TrimSpace(cfg.Email)
	if email == "" {
		return errors.New("--email is required")
	}
	if !cfg.Confirm {
		return errors.New("--confirm is required to wipe an MFA enrollment")
	}

	svc, err := backoffice.NewService(dbConfig)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := svc.Close(); closeErr != nil {
			fmt.Fprintf(out, "Warning: failed to close backoffice service: %v\n", closeErr)
		}
	}()

	user, err := svc.DisableMFA(c.Cmd().Context(), email)
	if err != nil {
		return errxtrace.Wrap("failed to disable MFA", err)
	}

	fmt.Fprintf(out, "✅ MFA enrollment wiped for back-office user %s (%s)\n", user.Name, user.Email)
	if user.MFAEnforced {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "⚠️  This user has mfa_enforced=true. They cannot sign in until you")
		fmt.Fprintln(out, "    run `inventario backoffice mfa setup` to issue a fresh enrollment.")
	}
	return nil
}
