// MFA operations for the back-office plane (issue #1785, Phase 4).
//
// The CLI surface is the ONLY enrollment path: there is no over-HTTP
// self-service MFA setup for back-office users. Three operations:
//
//   - Setup: generate a fresh TOTP secret + 10 backup codes, persist
//     the row with EnabledAt=now. Refuses to clobber an existing
//     enabled row unless --force is passed (in which case Setup
//     behaves like a full re-enroll).
//   - Disable: wipe the row. Idempotent — disabling a non-enrolled
//     user is a no-op success.
//   - Regenerate backup codes: replaces the row's backup_codes_hashed
//     with a fresh set of 10, keeps the TOTP secret untouched. Returns
//     ErrMFANotEnrolled when the user has no row.

package backoffice

import (
	"context"
	"errors"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// ErrMFANotEnrolled is returned by RegenerateBackupCodes (and reported
// by Setup when --force is omitted on a row that's already enabled).
// Distinct from the generic registry.ErrBackofficeMFASecretNotFound so
// CLI callers can render the right human message.
var ErrMFANotEnrolled = errx.NewSentinel("back-office user has no MFA enrollment")

// ErrMFAAlreadyEnabled is returned by Setup when the back-office user
// already has an enabled enrollment row and --force was NOT supplied.
// The CLI surfaces it with a hint about --force; the same sentinel is
// reusable by future surfaces that want to refuse a destructive
// re-enrollment without explicit consent.
var ErrMFAAlreadyEnabled = errx.NewSentinel("back-office user already has MFA enabled")

// MFASetupRequest carries the inputs to Service.SetupMFA.
type MFASetupRequest struct {
	Email string
	Force bool // overwrite an existing enrollment
}

// MFASetupResult captures the outcome of a successful Setup call. The
// plaintext secret + provisioning URL + backup codes are shown ONCE by
// the CLI and never persisted; the registry only stores the encrypted
// secret + hashed backup codes.
type MFASetupResult struct {
	User            *models.BackofficeUser
	Secret          string   // base32 TOTP secret (plain — shown once)
	ProvisioningURL string   // otpauth:// URL the CLI prints + (optionally) QRs
	BackupCodes     []string // plaintext backup codes (shown once)
}

// MFARegenerateResult carries a freshly-issued backup code set.
type MFARegenerateResult struct {
	User        *models.BackofficeUser
	BackupCodes []string
}

// SetupMFA generates a fresh TOTP secret + backup code set for the
// supplied back-office user, encrypts the secret, hashes the codes, and
// upserts the row with EnabledAt=now. Refuses to overwrite an existing
// enabled row unless req.Force is true.
//
// The MFAService is supplied by the caller (CLI) rather than constructed
// internally because constructing it requires the JWT root key, which
// lives in the CLI's shared config. Keeping the dependency injected
// also makes the service straightforward to unit-test with a stub.
func (s *Service) SetupMFA(ctx context.Context, mfaSvc *services.MFAService, req MFASetupRequest) (*MFASetupResult, error) {
	if s.factorySet == nil || s.factorySet.BackofficeUserRegistry == nil || s.factorySet.BackofficeUserMFASecretRegistry == nil {
		return nil, errors.New("backoffice registries not configured")
	}
	if mfaSvc == nil {
		return nil, errors.New("MFA service is required")
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		return nil, errors.New("email is required")
	}

	user, err := s.factorySet.BackofficeUserRegistry.GetByEmail(ctx, email)
	if err != nil {
		return nil, errxtrace.Wrap("failed to look up back-office user", err)
	}

	// Refuse to clobber an enabled enrollment unless --force is passed.
	existing, getErr := s.factorySet.BackofficeUserMFASecretRegistry.Get(ctx, user.ID)
	switch {
	case errors.Is(getErr, registry.ErrBackofficeMFASecretNotFound):
		// No existing enrollment — proceed.
	case getErr != nil:
		return nil, errxtrace.Wrap("failed to check existing MFA enrollment", getErr)
	case existing != nil && existing.IsEnabled() && !req.Force:
		return nil, errxtrace.Classify(ErrMFAAlreadyEnabled, errx.Attrs("admin_id", user.ID))
	}

	enrollment, err := mfaSvc.GenerateEnrollment(user.Email)
	if err != nil {
		return nil, errxtrace.Wrap("failed to generate TOTP enrollment", err)
	}
	encrypted, err := mfaSvc.EncryptSecret(enrollment.Secret)
	if err != nil {
		return nil, errxtrace.Wrap("failed to encrypt TOTP secret", err)
	}
	plain, hashes, err := mfaSvc.GenerateBackupCodes(services.MFABackupCodeCount)
	if err != nil {
		return nil, errxtrace.Wrap("failed to generate backup codes", err)
	}

	row := models.BackofficeUserMFASecret{
		BackofficeUserID:  user.ID,
		SecretEncrypted:   encrypted,
		BackupCodesHashed: models.ValuerSlice[string](hashes),
	}
	stored, err := s.factorySet.BackofficeUserMFASecretRegistry.Upsert(ctx, row)
	if err != nil {
		return nil, errxtrace.Wrap("failed to persist MFA enrollment", err)
	}

	// Mark the row enabled in a separate call so the registry's
	// MarkEnabled invariant fires (and a future Setup-without-enable
	// flow stays compositional).
	now := nowInUTC()
	if err := s.factorySet.BackofficeUserMFASecretRegistry.MarkEnabled(ctx, user.ID, now); err != nil {
		return nil, errxtrace.Wrap("failed to mark MFA enrollment enabled", err)
	}
	stored.EnabledAt = &now

	return &MFASetupResult{
		User:            user,
		Secret:          enrollment.Secret,
		ProvisioningURL: enrollment.ProvisioningURL,
		BackupCodes:     plain,
	}, nil
}

// DisableMFA wipes the back-office user's MFA enrollment row. Idempotent:
// disabling an already-disabled user is a no-op success — matches the
// CLI's contract that re-running the disable command is safe.
func (s *Service) DisableMFA(ctx context.Context, email string) (*models.BackofficeUser, error) {
	if s.factorySet == nil || s.factorySet.BackofficeUserRegistry == nil || s.factorySet.BackofficeUserMFASecretRegistry == nil {
		return nil, errors.New("backoffice registries not configured")
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, errors.New("email is required")
	}

	user, err := s.factorySet.BackofficeUserRegistry.GetByEmail(ctx, email)
	if err != nil {
		return nil, errxtrace.Wrap("failed to look up back-office user", err)
	}

	if err := s.factorySet.BackofficeUserMFASecretRegistry.Delete(ctx, user.ID); err != nil {
		return nil, errxtrace.Wrap("failed to delete MFA enrollment", err)
	}
	return user, nil
}

// RegenerateBackupCodes replaces the row's backup_codes_hashed with a
// fresh set of 10 codes, keeping the TOTP secret untouched. Returns
// ErrMFANotEnrolled when the user has no row — the caller (CLI) surfaces
// this with a hint to run `setup` first instead of a generic "not found".
func (s *Service) RegenerateBackupCodes(ctx context.Context, mfaSvc *services.MFAService, email string) (*MFARegenerateResult, error) {
	if s.factorySet == nil || s.factorySet.BackofficeUserRegistry == nil || s.factorySet.BackofficeUserMFASecretRegistry == nil {
		return nil, errors.New("backoffice registries not configured")
	}
	if mfaSvc == nil {
		return nil, errors.New("MFA service is required")
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, errors.New("email is required")
	}

	user, err := s.factorySet.BackofficeUserRegistry.GetByEmail(ctx, email)
	if err != nil {
		return nil, errxtrace.Wrap("failed to look up back-office user", err)
	}

	existing, err := s.factorySet.BackofficeUserMFASecretRegistry.Get(ctx, user.ID)
	if err != nil {
		if errors.Is(err, registry.ErrBackofficeMFASecretNotFound) {
			return nil, errxtrace.Classify(ErrMFANotEnrolled, errx.Attrs("admin_id", user.ID))
		}
		return nil, errxtrace.Wrap("failed to look up MFA enrollment", err)
	}

	plain, hashes, err := mfaSvc.GenerateBackupCodes(services.MFABackupCodeCount)
	if err != nil {
		return nil, errxtrace.Wrap("failed to generate backup codes", err)
	}

	// Preserve the existing TOTP secret + enrolled timestamp; only the
	// backup codes change. Upsert is the atomic-replace primitive — the
	// registry handles the delete-and-insert under one tx so a partial
	// rewrite is impossible.
	row := models.BackofficeUserMFASecret{
		BackofficeUserID:  user.ID,
		SecretEncrypted:   existing.SecretEncrypted,
		BackupCodesHashed: models.ValuerSlice[string](hashes),
		EnabledAt:         existing.EnabledAt,
		LastUsedAt:        existing.LastUsedAt,
		CreatedAt:         existing.CreatedAt,
	}
	if _, err := s.factorySet.BackofficeUserMFASecretRegistry.Upsert(ctx, row); err != nil {
		return nil, errxtrace.Wrap("failed to persist regenerated backup codes", err)
	}
	// Upsert wipes EnabledAt on insert; re-mark it so the row stays
	// usable for login.
	if existing.EnabledAt != nil {
		if err := s.factorySet.BackofficeUserMFASecretRegistry.MarkEnabled(ctx, user.ID, *existing.EnabledAt); err != nil {
			return nil, errxtrace.Wrap("failed to re-mark MFA enrollment enabled after regenerate", err)
		}
	}

	return &MFARegenerateResult{
		User:        user,
		BackupCodes: plain,
	}, nil
}
