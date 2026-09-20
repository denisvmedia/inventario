package backoffice_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/backoffice"
)

// Back-office MFA is the second factor on the only identity that can
// impersonate a customer, and all three of its operations were untested
// (#2114 N3). The guards below are the whole point of the surface: setup must
// not silently replace a working enrolment, regenerate must not invent one,
// and disable must actually remove the row rather than leave a secret behind.

// newMFAService builds an MFAService from a fixed root key. The key material
// is irrelevant to what these assert; only that encrypt and decrypt agree.
func newMFAService(c *qt.C) *services.MFAService {
	c.Helper()
	root := make([]byte, 32)
	for i := range root {
		root[i] = byte(i + 1)
	}
	svc, err := services.NewMFAService(root)
	c.Assert(err, qt.IsNil)
	return svc
}

// seedOperator creates a back-office user the MFA calls can target.
func seedOperator(c *qt.C, svc *backoffice.Service, email string) {
	c.Helper()
	_, err := svc.Bootstrap(context.Background(), backoffice.BootstrapRequest{
		Email: email,
		Name:  "Operator",
		Force: true,
	})
	c.Assert(err, qt.IsNil)
}

// TestSetupMFA_RefusesToClobberAnEnabledEnrolment is the guard that matters:
// re-running setup against an operator who already has a working
// authenticator would hand out a new secret and lock them out of their own
// account, with the old one still in their phone.
func TestSetupMFA_RefusesToClobberAnEnabledEnrolment(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)
	svc := newService(c)
	mfa := newMFAService(c)
	ctx := context.Background()

	seedOperator(c, svc, "ops@example.com")

	first, err := svc.SetupMFA(ctx, mfa, backoffice.MFASetupRequest{Email: "ops@example.com"})
	c.Assert(err, qt.IsNil)
	c.Assert(first.Secret, qt.Not(qt.Equals), "")
	c.Assert(first.BackupCodes, qt.HasLen, services.MFABackupCodeCount)

	_, err = svc.SetupMFA(ctx, mfa, backoffice.MFASetupRequest{Email: "ops@example.com"})
	c.Assert(err, qt.ErrorIs, backoffice.ErrMFAAlreadyEnabled)

	// The original enrolment is untouched — the point of refusing.
	row, err := fs.BackofficeUserMFASecretRegistry.Get(ctx, first.User.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(row.IsEnabled(), qt.IsTrue)

	// --force is the documented escape hatch, and it does replace the secret.
	forced, err := svc.SetupMFA(ctx, mfa,
		backoffice.MFASetupRequest{Email: "ops@example.com", Force: true})
	c.Assert(err, qt.IsNil)
	c.Assert(forced.Secret, qt.Not(qt.Equals), first.Secret)
}

// Regenerate replaces the backup codes and leaves the TOTP secret alone. An
// operator runs this after spending their codes; rotating the secret too
// would invalidate the authenticator they are still using.
func TestRegenerateBackupCodes_KeepsTheTOTPSecret(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)
	svc := newService(c)
	mfa := newMFAService(c)
	ctx := context.Background()

	seedOperator(c, svc, "ops@example.com")
	setup, err := svc.SetupMFA(ctx, mfa, backoffice.MFASetupRequest{Email: "ops@example.com"})
	c.Assert(err, qt.IsNil)

	before, err := fs.BackofficeUserMFASecretRegistry.Get(ctx, setup.User.ID)
	c.Assert(err, qt.IsNil)

	res, err := svc.RegenerateBackupCodes(ctx, mfa, "ops@example.com")
	c.Assert(err, qt.IsNil)
	c.Assert(res.BackupCodes, qt.HasLen, services.MFABackupCodeCount)

	after, err := fs.BackofficeUserMFASecretRegistry.Get(ctx, setup.User.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(after.SecretEncrypted, qt.Equals, before.SecretEncrypted,
		qt.Commentf("regenerating codes must not rotate the authenticator secret"))
	c.Assert(after.BackupCodesHashed, qt.Not(qt.DeepEquals), before.BackupCodesHashed)
	c.Assert(after.EnabledAt, qt.DeepEquals, before.EnabledAt)
}

// Regenerating for someone with no enrolment must say so rather than create
// one: a caller who reaches for `regenerate-codes` on an unenrolled operator
// has the wrong command, and silently enrolling them would produce codes for
// an authenticator that does not exist.
func TestRegenerateBackupCodes_RefusesWithoutEnrolment(t *testing.T) {
	c := qt.New(t)
	setupMemoryAsPostgres(c)
	svc := newService(c)
	mfa := newMFAService(c)

	seedOperator(c, svc, "ops@example.com")

	_, err := svc.RegenerateBackupCodes(context.Background(), mfa, "ops@example.com")
	c.Assert(err, qt.ErrorIs, backoffice.ErrMFANotEnrolled)
}

// Disable removes the row. A left-behind secret would mean an operator who
// believes MFA is off still fails login closed on the next attempt.
func TestDisableMFA_RemovesTheEnrolment(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)
	svc := newService(c)
	mfa := newMFAService(c)
	ctx := context.Background()

	seedOperator(c, svc, "ops@example.com")
	setup, err := svc.SetupMFA(ctx, mfa, backoffice.MFASetupRequest{Email: "ops@example.com"})
	c.Assert(err, qt.IsNil)

	user, err := svc.DisableMFA(ctx, "ops@example.com")
	c.Assert(err, qt.IsNil)
	c.Assert(user.ID, qt.Equals, setup.User.ID)

	_, err = fs.BackofficeUserMFASecretRegistry.Get(ctx, setup.User.ID)
	c.Assert(err, qt.IsNotNil, qt.Commentf("the enrolment row must be gone"))

	// And setup works again afterwards, without --force: disable really
	// returned the account to the unenrolled state.
	again, err := svc.SetupMFA(ctx, mfa, backoffice.MFASetupRequest{Email: "ops@example.com"})
	c.Assert(err, qt.IsNil)
	c.Assert(again.Secret, qt.Not(qt.Equals), "")
}

// Every entry point rejects an empty email rather than looking one up.
func TestMFAOperations_RequireAnEmail(t *testing.T) {
	c := qt.New(t)
	setupMemoryAsPostgres(c)
	svc := newService(c)
	mfa := newMFAService(c)
	ctx := context.Background()

	_, err := svc.SetupMFA(ctx, mfa, backoffice.MFASetupRequest{Email: "  "})
	c.Assert(err, qt.IsNotNil)

	_, err = svc.DisableMFA(ctx, "")
	c.Assert(err, qt.IsNotNil)

	_, err = svc.RegenerateBackupCodes(ctx, mfa, "")
	c.Assert(err, qt.IsNotNil)

	// A nil MFA service is a wiring mistake, not a user error.
	_, err = svc.SetupMFA(ctx, nil, backoffice.MFASetupRequest{Email: "ops@example.com"})
	c.Assert(err, qt.IsNotNil)
}
