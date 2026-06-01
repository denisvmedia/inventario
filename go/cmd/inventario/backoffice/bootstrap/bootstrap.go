// Package bootstrap implements `inventario backoffice bootstrap`.
//
// Stamps the first platform-operator identity into a fresh deployment
// (issue #1785 Phase 1). The command is idempotent on the email — a
// re-run with the same email prints "user already exists" and exits 0.
// A re-run with a different email refuses unless --force is passed, so
// an operator can't accidentally create a sprawl of back-office users.
package bootstrap

import (
	"errors"
	"fmt"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/backoffice"
)

// Config carries the bootstrap command's flags.
type Config struct {
	Email    string `yaml:"email" env:"EMAIL"`
	Name     string `yaml:"name" env:"NAME"`
	Role     string `yaml:"role" env:"ROLE"`
	Password string `yaml:"password" env:"PASSWORD"`
	Force    bool   `yaml:"force" env:"FORCE"`
	// Ensure relaxes the "a backoffice user already exists" refusal for
	// non-interactive provisioning (the Helm init-data Job, #1967): when an
	// operator already exists under a DIFFERENT email, --ensure makes the
	// command a benign no-op (exit 0) instead of exiting 1. It NEVER creates a
	// second operator (that's --force) and never weakens the interactive
	// safety net — it is strictly opt-in. See services/backoffice.BootstrapRequest.
	Ensure bool `yaml:"ensure" env:"ENSURE"`
	// MFAEnforced is a *bool so an OMITTED value (nil) stays distinct from an
	// explicit false. nil flows to the service, which applies the secure
	// default of true (see services/backoffice.resolveMFAEnforced): the
	// operator must enrol TOTP via `inventario backoffice mfa setup` before
	// /backoffice/login will issue a session token (a no-secret MFAEnforced=true
	// row fails closed with HTTP 501). A plain bool would be unsafe here —
	// shared.ReadSection overwrites this struct from a zero-valued wrapper, and
	// cleanenv can't tell an omitted YAML/env key from an explicit `false` (both
	// are the bool zero value), so an env-default on a bool could silently flip
	// an operator's explicit `mfa-enforced: false` back to true or leave a
	// partially-populated config section insecurely false. The pointer makes the
	// unset/true/false states explicit; demo/preview pass --mfa-enforced=false
	// to provision a password-only operator (issue #1967).
	//
	// NOTE: there is intentionally NO `env:` tag here. cleanenv (the env reader
	// behind shared.ReadSection) has no case for pointer kinds in its
	// parseValue switch, so a `*bool` falls through to the default arm and
	// returns `unsupported type .` — a logged-and-swallowed error
	// (TryReadSection is non-fatal) that spammed every Helm Job run. Nothing
	// relies on the Go-side env binding: the Helm Jobs read
	// INVENTARIO_BACKOFFICE_BOOTSTRAP_MFA_ENFORCED in shell and forward it as
	// the --mfa-enforced CLI flag (job-init-data.yaml / job-setup.yaml), and the
	// flag path (Flags().Changed) already gives flag > config/env > default. The
	// yaml tag is kept so a config.yaml `backoffice.bootstrap.mfa-enforced` key
	// still works; only the dead, error-spamming env binding is dropped.
	MFAEnforced *bool `yaml:"mfa-enforced"`
}

// Command is the cobra wrapper around `backoffice bootstrap`.
type Command struct {
	command.Base

	config Config
	// mfaEnforcedFlag backs the --mfa-enforced flag. cobra needs a concrete
	// bool; run() promotes it into the request only when the flag was
	// explicitly set (Flags().Changed), giving precedence
	// flag > config/env > secure default (nil → true at the service).
	mfaEnforcedFlag bool
}

// New constructs the command with the supplied database config.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	// Populate config from an optional config.yaml `backoffice.bootstrap`
	// section + INVENTARIO_BACKOFFICE_BOOTSTRAP_* env. MFAEnforced is a *bool,
	// so an omitted key stays nil (→ secure default at the service) rather than
	// a silent insecure false.
	shared.TryReadSection("backoffice.bootstrap", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "bootstrap",
		Short: "Create the first back-office (platform-operator) user",
		Long: `Stamp the first back-office user into a fresh deployment.

Back-office users live OUTSIDE the tenant model: they have no tenant_id,
no group membership, and no RLS scoping. They authenticate against the
back-office auth plane (added in Phase 2 of issue #1785).

IDEMPOTENCY:

  • Re-running with the same --email prints "user already exists" and
    exits 0 (safe to call from provisioning scripts).
  • Re-running with a different --email refuses unless --force is
    passed, so a deployment can't accidentally accumulate operators.
  • For non-interactive provisioning that only needs to guarantee an
    operator exists (the Helm init-data Job, issue #1967), pass --ensure:
    when any operator already exists it prints "nothing to do" and exits 0
    instead of refusing. --ensure NEVER creates a second operator (that is
    --force) — it only converts the refusal into a benign no-op.

PASSWORD:

  • If --password is omitted, a strong random password is generated
    and printed ONCE to stdout. Copy it before the terminal scrolls —
    it is NOT stored anywhere and cannot be recovered.
  • If --password is supplied, it must meet the same complexity rules
    as models.ValidatePassword (8+ chars, upper, lower, digit).

ROLE:

  • Defaults to platform_admin (full mutation rights — the only sensible
    seed value).
  • support_agent is also accepted for non-first invocations under --force.

MFA:

  • --mfa-enforced defaults to true: the operator must enrol TOTP via
    "inventario backoffice mfa setup" before /backoffice/login will issue a
    session token. Until then login returns HTTP 501.
  • Pass --mfa-enforced=false to provision a password-only operator for
    demo/preview environments where no TOTP app is available (issue #1967).
    Never use this in production.

Examples:
  inventario backoffice bootstrap --email admin@example.com --name "Ops Admin"
  inventario backoffice bootstrap --email second@example.com --name "Second" --force
  inventario backoffice bootstrap --email a@example.com --name "A" --password 'Sup3rSecret!'`,
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
	c.Cmd().Flags().StringVar(&c.config.Name, "name", c.config.Name, "Back-office user display name (required)")
	c.Cmd().Flags().StringVar(&c.config.Role, "role", c.config.Role, "Back-office role: platform_admin (default) or support_agent")
	c.Cmd().Flags().StringVar(&c.config.Password, "password", c.config.Password, "Password (auto-generated if omitted)")
	c.Cmd().Flags().BoolVar(&c.config.Force, "force", c.config.Force, "Allow adding a second+ back-office user when one already exists")
	c.Cmd().Flags().BoolVar(&c.config.Ensure, "ensure", c.config.Ensure, "Non-interactive provisioning (#1967): when any operator already exists, exit 0 without creating another instead of refusing. Does NOT add a second (use --force)")
	c.Cmd().Flags().BoolVar(&c.mfaEnforcedFlag, "mfa-enforced", true, "Require TOTP enrollment before the operator can sign in (default true; set --mfa-enforced=false for demo/preview where password-only login is wanted)")
}

func (c *Command) run(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	// dbConfig.Validate already rejects non-postgres DSNs at the
	// shared-validator layer (memory://, file:// etc all fail with
	// "only support PostgreSQL"). No second guard needed here.
	if err := dbConfig.Validate(); err != nil {
		return errxtrace.Wrap("database configuration error", err)
	}

	email := strings.TrimSpace(cfg.Email)
	if email == "" {
		return errors.New("--email is required")
	}
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return errors.New("--name is required")
	}

	// Role defaults to platform_admin at the service layer; reject
	// invalid free-form values here so the CLI error message includes
	// the valid set without the service having to know about CLI
	// presentation.
	role := models.BackofficeRole(strings.TrimSpace(cfg.Role))
	if role == "" {
		role = models.BackofficeRolePlatformAdmin
	}
	if !role.IsValid() {
		return fmt.Errorf("--role must be one of: support_agent, platform_admin (got %q)", string(role))
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

	// MFA precedence: an explicit --mfa-enforced flag wins; otherwise fall back
	// to the config-file/env value (cfg.MFAEnforced, nil when unset). A nil
	// reaches the service as "use the secure default" (true). Flags().Changed
	// lets --mfa-enforced=false win while an omitted flag defers to config/env,
	// and an absent value everywhere stays enforced.
	mfaEnforced := cfg.MFAEnforced
	if c.Cmd().Flags().Changed("mfa-enforced") {
		v := c.mfaEnforcedFlag
		mfaEnforced = &v
	}

	result, err := svc.Bootstrap(c.Cmd().Context(), backoffice.BootstrapRequest{
		Email:       email,
		Name:        name,
		Role:        role,
		Password:    cfg.Password,
		Force:       cfg.Force,
		Ensure:      cfg.Ensure,
		MFAEnforced: mfaEnforced,
	})
	if err != nil {
		return errxtrace.Wrap("failed to bootstrap backoffice user", err)
	}

	if result.AlreadyExisted {
		fmt.Fprintf(out, "ℹ️  Back-office user %s (%s) already exists; no changes made.\n",
			result.User.Name, result.User.Email)
		return nil
	}

	// --ensure no-op: an operator already exists under a different email and
	// the caller only wanted to guarantee one exists. result.User MAY be nil
	// here (best-effort fetch), so never dereference it on this branch.
	if result.EnsuredExisting {
		if result.User != nil {
			fmt.Fprintf(out, "ℹ️  A back-office operator already exists (%s); --ensure: nothing to do.\n",
				result.User.Email)
		} else {
			fmt.Fprintln(out, "ℹ️  A back-office operator already exists; --ensure: nothing to do.")
		}
		return nil
	}

	fmt.Fprintf(out, "✅ Created back-office user %s (%s) with role %s.\n",
		result.User.Name, result.User.Email, string(result.User.Role))
	if result.GeneratedPassword != "" {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "🔑 Generated password (copy this NOW — it will not be shown again):")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "    "+result.GeneratedPassword)
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "The operator should sign in with this password and rotate it on first login.")
	}
	return nil
}
