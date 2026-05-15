package mfareset

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/services/admin"
)

// Command is the operator-facing "wipe a user's TOTP enrollment" command.
// The recovery story per #1380 v1 is "contact support"; this is the
// support-side action that lets an operator clear the secret + backup
// codes so the user can re-enroll through Settings → Privacy & Security.
type Command struct {
	command.Base

	config Config
}

// New creates the user mfa-reset command.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	shared.TryReadSection("users.mfa_reset", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "mfa-reset <id-or-email>",
		Short: "Reset a user's multi-factor authentication enrollment",
		Long: `Reset (remove) a user's TOTP enrollment so they can re-enroll.

This is the operator-side recovery flow for users who lost access to
their authenticator app. It deletes the user's user_mfa_secrets row
(secret + bcrypt-hashed backup codes) and appends an "mfa_admin_reset"
event to the user's login history.

The user keeps their password — they just stop being prompted for a
second factor at sign-in, and can re-enable MFA from
Settings → Privacy & Security whenever they want.

SAFETY FEATURES:
  • User details display before reset
  • Confirmation prompts (unless --force is used)
  • Dry-run support to preview the operation
  • Idempotent: succeeds silently if the user has no MFA enrolled

Examples:
  # Reset with confirmation
  inventario users mfa-reset admin@acme.com

  # Preview the reset
  inventario users mfa-reset admin@acme.com --dry-run

  # Force reset without prompts
  inventario users mfa-reset admin@acme.com --force

  # Reset by ID
  inventario users mfa-reset 550e8400-e29b-41d4-a716-446655440000`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return c.resetMFA(&c.config, dbConfig, args[0])
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)
	c.Cmd().Flags().BoolVar(&c.config.Force, "force", c.config.Force, "Skip confirmation prompts")
}

func (c *Command) resetMFA(cfg *Config, dbConfig *shared.DatabaseConfig, idOrEmail string) error {
	out := c.Cmd().OutOrStdout()

	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("MFA reset is not supported for memory databases as they don't provide persistence. Please use a PostgreSQL database")
	}

	if strings.TrimSpace(idOrEmail) == "" {
		return fmt.Errorf("user ID or email is required")
	}

	fmt.Fprintln(out, "=== RESET USER MFA ===")
	fmt.Fprintf(out, "Database: %s\n", dbConfig.DBDSN)
	fmt.Fprintf(out, "Target: %s\n", idOrEmail)
	if cfg.DryRun {
		fmt.Fprintln(out, "Mode: DRY RUN (no changes will be made)")
	} else {
		fmt.Fprintln(out, "Mode: LIVE RESET")
	}
	fmt.Fprintln(out)

	adminService, err := admin.NewService(dbConfig)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := adminService.Close(); closeErr != nil {
			fmt.Fprintf(out, "Warning: failed to close admin service: %v\n", closeErr)
		}
	}()

	user, err := adminService.GetUser(context.Background(), idOrEmail)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	fmt.Fprintf(out, "Found user: %s (%s)\n", user.Name, user.Email)
	fmt.Fprintf(out, "Active: %t\n", user.IsActive)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "RESET IMPACT:")
	fmt.Fprintln(out, "  • The user's TOTP secret will be deleted")
	fmt.Fprintln(out, "  • All backup codes will be invalidated")
	fmt.Fprintln(out, "  • The user can re-enroll via Settings → Privacy & Security")
	fmt.Fprintln(out, "  • An 'mfa_admin_reset' row will appear in their login history")
	fmt.Fprintln(out)

	if cfg.DryRun {
		fmt.Fprintln(out, "💡 This is a dry run. To perform the actual reset, run the command without --dry-run")
		return nil
	}

	if !cfg.Force {
		if !c.confirmReset(user.Email, user.Name) {
			fmt.Fprintln(out, "Reset cancelled.")
			return nil
		}
	}

	_, hadEnrollment, err := adminService.ResetUserMFA(context.Background(), idOrEmail)
	if err != nil {
		return fmt.Errorf("failed to reset MFA: %w", err)
	}

	if hadEnrollment {
		fmt.Fprintln(out, "✅ MFA enrollment removed successfully.")
	} else {
		fmt.Fprintln(out, "ℹ️  No MFA enrollment was present — nothing to remove.")
	}
	fmt.Fprintf(out, "User: %s (%s)\n", user.Name, user.Email)
	return nil
}

func (c *Command) confirmReset(email, name string) bool {
	out := c.Cmd().OutOrStdout()

	fmt.Fprintf(out, "⚠️  Reset MFA for '%s' (%s)? [y/N]: ", name, email)
	var response string
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
