// Package regeneratecodes implements
// `inventario backoffice mfa regenerate-backup-codes`.
//
// Mints a fresh set of 10 single-use backup codes for an
// already-enrolled back-office user. The TOTP secret is NOT touched —
// the user's authenticator app keeps working. Any previously-issued
// backup codes are invalidated.
//
// Returns ErrMFANotEnrolled (mapped to a friendly CLI message) when the
// user has no enrollment row — the operator should run `setup` instead.
package regeneratecodes

import (
	"errors"
	"fmt"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/backoffice/mfa/internal"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/services"
	"github.com/denisvmedia/inventario/services/backoffice"
)

// Config carries the regenerate-backup-codes command's flags.
type Config struct {
	Email     string `yaml:"email" env:"EMAIL"`
	JWTSecret string `yaml:"jwt_secret" env:"JWT_SECRET"`
}

// Command is the cobra wrapper around `backoffice mfa regenerate-backup-codes`.
type Command struct {
	command.Base

	config Config
}

// New constructs the command with the supplied database config.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	shared.TryReadSection("backoffice.mfa.regenerate_backup_codes", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "regenerate-backup-codes",
		Short: "Mint a fresh set of MFA backup codes for a back-office user",
		Long: `Replace a back-office user's backup codes with a fresh set of 10.

The TOTP secret is left untouched — the user's authenticator app keeps
working. Any previously-issued backup codes are invalidated.

This requires --jwt-secret because the underlying registry call needs
an MFAService instance even though the TOTP secret is not re-generated
(the service is built once and reused across MFA call sites).

Examples:
  inventario backoffice mfa regenerate-backup-codes --email admin@example.com
  INVENTARIO_BACKOFFICE_MFA_JWT_SECRET=... inventario backoffice mfa regenerate-backup-codes --email a@example.com`,
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
	c.Cmd().Flags().StringVar(&c.config.JWTSecret, "jwt-secret", c.config.JWTSecret, "JWT secret (same one the server uses; 32+ bytes plain or 64+ hex chars)")
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
	secret, err := internal.ResolveJWTSecret(cfg.JWTSecret)
	if err != nil {
		return err
	}
	mfaSvc, err := services.NewMFAService(secret)
	if err != nil {
		return errxtrace.Wrap("failed to construct MFA service", err)
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

	result, err := svc.RegenerateBackupCodes(c.Cmd().Context(), mfaSvc, email)
	if err != nil {
		if errors.Is(err, backoffice.ErrMFANotEnrolled) {
			return fmt.Errorf("back-office user %s has no MFA enrollment; run `inventario backoffice mfa setup --email %s` first", email, email)
		}
		return errxtrace.Wrap("failed to regenerate backup codes", err)
	}

	fmt.Fprintf(out, "✅ Backup codes regenerated for back-office user %s (%s)\n", result.User.Name, result.User.Email)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "🧯 NEW BACKUP CODES (single-use recovery — shown ONCE):")
	fmt.Fprintln(out, "")
	for _, code := range result.BackupCodes {
		fmt.Fprintln(out, "    "+code)
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Hand them to the back-office user via a secure channel.")
	fmt.Fprintln(out, "Previously-issued backup codes are now invalid.")
	return nil
}
