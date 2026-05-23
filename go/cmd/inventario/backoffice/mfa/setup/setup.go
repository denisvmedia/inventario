// Package setup implements `inventario backoffice mfa setup`.
//
// Generates a fresh TOTP secret + 10 backup codes for a back-office
// user, persists the encrypted secret + hashed codes, and stamps
// EnabledAt=now so the row is immediately usable for sign-in. The
// secret + provisioning URL + plaintext backup codes are printed to
// stdout ONCE — the operator is responsible for handing them to the
// back-office user via a secure channel (the CLI does not email them).
package setup

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

// Config carries the setup command's flags.
type Config struct {
	Email     string `yaml:"email" env:"EMAIL"`
	JWTSecret string `yaml:"jwt_secret" env:"JWT_SECRET"`
	Force     bool   `yaml:"force" env:"FORCE"`
}

// Command is the cobra wrapper around `backoffice mfa setup`.
type Command struct {
	command.Base

	config Config
}

// New constructs the command with the supplied database config.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	shared.TryReadSection("backoffice.mfa.setup", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "setup",
		Short: "Enrol a back-office user in MFA",
		Long: `Generate a fresh TOTP secret + 10 backup codes for a back-office user.

The TOTP secret + otpauth:// provisioning URL + backup codes are
printed ONCE to stdout. The operator is responsible for handing them
to the back-office user via a secure channel.

The TOTP secret is encrypted at rest using an HKDF-derived subkey of
the supplied --jwt-secret. The CLI MUST be invoked with the same JWT
secret the server is configured with; otherwise the encrypted row is
undecryptable when the user attempts to sign in.

If the user already has an enabled enrollment, the command refuses
unless --force is passed (in which case it overwrites the row,
invalidating any previously-issued QR / backup codes).

Examples:
  inventario backoffice mfa setup --email admin@example.com
  inventario backoffice mfa setup --email admin@example.com --force
  INVENTARIO_BACKOFFICE_MFA_JWT_SECRET=... inventario backoffice mfa setup --email a@example.com`,
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
	c.Cmd().Flags().BoolVar(&c.config.Force, "force", c.config.Force, "Overwrite an existing enrollment")
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

	result, err := svc.SetupMFA(c.Cmd().Context(), mfaSvc, backoffice.MFASetupRequest{
		Email: email,
		Force: cfg.Force,
	})
	if err != nil {
		if errors.Is(err, backoffice.ErrMFAAlreadyEnabled) {
			return fmt.Errorf("back-office user %s already has MFA enabled; pass --force to re-enrol (this invalidates any previously-issued QR / backup codes)", email)
		}
		return errxtrace.Wrap("failed to set up MFA", err)
	}

	fmt.Fprintf(out, "✅ MFA enrolled for back-office user %s (%s)\n", result.User.Name, result.User.Email)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "🔑 TOTP SECRET (enter manually if QR scan fails — shown ONCE):")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "    "+result.Secret)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "📷 OTPAUTH URL (paste into a QR generator for the user — shown ONCE):")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "    "+result.ProvisioningURL)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "🧯 BACKUP CODES (single-use recovery — shown ONCE):")
	fmt.Fprintln(out, "")
	for _, code := range result.BackupCodes {
		fmt.Fprintln(out, "    "+code)
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Hand the above to the back-office user via a secure channel.")
	fmt.Fprintln(out, "They will be prompted for a TOTP code on every back-office sign-in.")
	return nil
}
