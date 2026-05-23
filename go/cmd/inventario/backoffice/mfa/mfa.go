// Package mfa is the CLI command group for managing back-office MFA
// enrollments (issue #1785, Phase 4).
//
// Back-office MFA is enrolled OUT-OF-BAND: there is no over-HTTP
// self-service surface — the operator runs `inventario backoffice mfa
// setup` from a privileged shell to mint the TOTP secret + backup
// codes, hands the QR / backup codes to the back-office user via a
// secure channel, and the back-office user then signs in via
// /backoffice/auth/login → /backoffice/auth/login/mfa.
//
// Three subcommands:
//
//   - setup:   generate fresh TOTP secret + 10 backup codes, persist.
//   - disable: wipe the row (idempotent).
//   - regenerate-backup-codes: refresh the 10 codes (TOTP stays).
//
// All three require the same JWT secret the server uses — the TOTP
// secret is encrypted with an HKDF-derived subkey, so a CLI run with a
// different secret produces a row that the server cannot decrypt. The
// `--jwt-secret` flag (or INVENTARIO_BACKOFFICE_MFA_JWT_SECRET env)
// keeps the CLI explicit; mismatches fail at verification rather than
// silently degrading.
package mfa

import (
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/backoffice/mfa/disable"
	"github.com/denisvmedia/inventario/cmd/inventario/backoffice/mfa/regeneratecodes"
	"github.com/denisvmedia/inventario/cmd/inventario/backoffice/mfa/setup"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

// New constructs the parent `backoffice mfa` command and registers its
// subcommands. Mounted from cmd/inventario/backoffice/backoffice.go.
func New(dbConfig *shared.DatabaseConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mfa",
		Short: "Manage back-office user MFA enrollments",
		Long: `MFA commands for back-office (platform-operator) users.

Back-office MFA is enrolled out-of-band: the operator runs ` + "`inventario backoffice mfa setup`" + `
to mint the TOTP secret + backup codes, then hands them to the
back-office user via a secure channel. The user signs in through the
back-office auth plane and is challenged for the TOTP code on every
login.

IMPORTANT: These commands ONLY support PostgreSQL databases. Memory
databases cannot persist enrollment rows across restarts.

The TOTP secret is encrypted at rest using an HKDF-derived subkey of
the server's JWT secret. The CLI MUST be invoked with the same JWT
secret the server is configured with; otherwise the encrypted row is
undecryptable when the user attempts to sign in.

USAGE EXAMPLES:

  Enrol a back-office user:
    inventario backoffice mfa setup --email admin@example.com

  Force a fresh enrollment (overwriting an existing one):
    inventario backoffice mfa setup --email admin@example.com --force

  Wipe an enrollment (e.g. lost authenticator app):
    inventario backoffice mfa disable --email admin@example.com --confirm

  Mint a fresh set of backup codes (TOTP secret untouched):
    inventario backoffice mfa regenerate-backup-codes --email admin@example.com`,
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(setup.New(dbConfig).Cmd())
	cmd.AddCommand(disable.New(dbConfig).Cmd())
	cmd.AddCommand(regeneratecodes.New(dbConfig).Cmd())

	return cmd
}
