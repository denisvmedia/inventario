// Package services — MFA helpers live alongside other auth services.
// This file owns every cryptographic concern of the TOTP feature:
//
//   - generating a fresh base32 TOTP secret + provisioning URI
//   - verifying a 6-digit code against the encrypted secret
//   - producing and consuming bcrypt-hashed single-use backup codes
//
// The HTTP layer is intentionally kept ignorant of bcrypt / TOTP /
// AES details; it only talks to MFAService.
package services

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/secrets"
)

const (
	// MFAIssuer is the human label that appears in authenticator apps
	// (Google Authenticator / 1Password / Authy / Bitwarden / Aegis)
	// alongside the user's email. Kept short and unbranded so the same
	// label works for self-hosted deployments.
	MFAIssuer = "Inventario"

	// MFASubkeyLabel namespaces the HKDF derivation. Bumping the
	// suffix is a breaking change for stored ciphertexts.
	MFASubkeyLabel = "inventario/mfa-secret-v1"

	// MFABackupCodeCount controls how many backup codes are minted by
	// Verify (first enrollment) and RegenerateBackupCodes. Per #1380:
	// "10 single-use codes is the convention. Default."
	MFABackupCodeCount = 10

	// backupCodeRawLen is the count of random bytes used per backup
	// code; the human-facing form is the standard RFC 4648 base32 of
	// these bytes (A–Z + 2–7) split into two five-character groups
	// (e.g. "ABCDE-FGHIJ"). 10 base32 chars = 50 bits of entropy —
	// comfortably more than a 6-digit TOTP code, sufficient for a
	// one-shot recovery.
	backupCodeRawLen = 6

	// totpPeriodSeconds is the TOTP step. RFC 6238 default of 30s as
	// requested in #1380. Don't tweak — every authenticator app
	// defaults to 30s and changing it requires re-enrollment.
	totpPeriodSeconds = 30

	// totpDigits sticks with the standard 6-digit code (RFC 6238).
	totpDigits = otp.DigitsSix

	// totpSkew = 1 step before/after, matching the "±1 step tolerance"
	// requirement in #1380. This covers clock drift between the user's
	// device and the server.
	totpSkew = 1
)

// MFAErrors are surfaced to the HTTP layer for mapping to status codes.
// The HTTP handler decides messaging (and whether to redact); the
// service refuses to leak whether the secret existed.
var (
	ErrMFANotEnrolled = errors.New("mfa: user has not enrolled")
	ErrMFAInvalidCode = errors.New("mfa: invalid verification code")
	ErrMFAEncryption  = errors.New("mfa: encryption failure")
)

// MFAEnrollment is the response body shape for the setup endpoint —
// pulled into its own type so the apiserver package doesn't have to
// import pquerna/otp.
type MFAEnrollment struct {
	// Secret is the base32 TOTP secret. Returned to the client only
	// during Setup so the user can manually type it into the
	// authenticator if the QR code can't be scanned.
	Secret string
	// ProvisioningURL is `otpauth://totp/Inventario:email?secret=...&issuer=Inventario&period=30&digits=6`.
	// The FE renders it as a QR code via a client-side library.
	ProvisioningURL string
}

// MFAService bundles the encryption key and clock so the TOTP and
// backup-code primitives are easy to unit test. Construct via
// NewMFAService(rootKey, now); rootKey is the operator-supplied JWT
// signing secret and the MFA-specific subkey is derived once at
// construction time.
type MFAService struct {
	encryptionKey []byte
	now           func() time.Time
	// totpRandReader feeds totp.Generate; tests override it so the
	// emitted secret is deterministic.
	totpRandReader interface {
		Read(p []byte) (n int, err error)
	}
}

// NewMFAService derives the MFA encryption subkey from rootKey and
// returns a service whose clock and randomness source default to the
// real ones. Pass a SubkeyLen-length rootKey or longer — the JWT
// secret already satisfies that via params validation.
func NewMFAService(rootKey []byte) (*MFAService, error) {
	key, err := secrets.DeriveSubkey(rootKey, MFASubkeyLabel)
	if err != nil {
		return nil, fmt.Errorf("mfa: derive subkey: %w", err)
	}
	return &MFAService{
		encryptionKey:  key,
		now:            time.Now,
		totpRandReader: rand.Reader,
	}, nil
}

// SetClock overrides the wall clock. Used in tests to verify the
// ±1 step tolerance window.
func (s *MFAService) SetClock(now func() time.Time) { s.now = now }

// GenerateEnrollment produces a fresh TOTP secret and provisioning URL
// for the user. The caller is responsible for encrypting the returned
// MFAEnrollment.Secret into a UserMFASecret row (via EncryptSecret)
// before the user sees the QR code — though in practice the apiserver
// does both inside the same handler.
func (s *MFAService) GenerateEnrollment(userEmail string) (*MFAEnrollment, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      MFAIssuer,
		AccountName: userEmail,
		Period:      totpPeriodSeconds,
		Digits:      totpDigits,
		Algorithm:   otp.AlgorithmSHA1,
		SecretSize:  20, // 160-bit secret, RFC 4226 §4 R6.
		Rand:        s.totpRandReader,
	})
	if err != nil {
		return nil, fmt.Errorf("mfa: generate totp: %w", err)
	}
	return &MFAEnrollment{
		Secret:          key.Secret(),
		ProvisioningURL: key.URL(),
	}, nil
}

// EncryptSecret wraps a base32 TOTP secret with the service's
// encryption key. Use this to populate UserMFASecret.SecretEncrypted
// before persisting.
func (s *MFAService) EncryptSecret(plaintext string) (string, error) {
	enc, err := secrets.EncryptString(s.encryptionKey, plaintext)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrMFAEncryption, err.Error())
	}
	return enc, nil
}

// DecryptSecret unwraps a previously-encrypted secret. Returns
// ErrMFAEncryption when the ciphertext is unreadable (corruption,
// rotated key, wrong version) — the caller should map this to a
// generic 500 / 401 without leaking the underlying error to the user.
func (s *MFAService) DecryptSecret(ciphertext string) (string, error) {
	dec, err := secrets.DecryptString(s.encryptionKey, ciphertext)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrMFAEncryption, err.Error())
	}
	return dec, nil
}

// VerifyTOTP returns true when `code` is the current 6-digit TOTP for
// the user's stored secret, allowing ±1 step of clock skew.
// Whitespace and hyphens in code are tolerated (paste-friendly).
func (s *MFAService) VerifyTOTP(stored models.UserMFASecret, code string) (bool, error) {
	if stored.SecretEncrypted == "" {
		return false, ErrMFANotEnrolled
	}
	plain, err := s.DecryptSecret(stored.SecretEncrypted)
	if err != nil {
		return false, err
	}
	cleaned := normalizeCode(code)
	// A non-6-digit code is invalid by length alone — the pquerna
	// library returns a non-nil error in that case, but the auth
	// flow only cares about boolean validity. Treat shape failures
	// as "wrong code" instead of a 500 so brute-force probing with
	// short strings can't smoke out which paths log errors.
	if len(cleaned) != totpDigits.Length() {
		return false, nil
	}
	ok, err := totp.ValidateCustom(cleaned, plain, s.now(), totp.ValidateOpts{
		Period:    totpPeriodSeconds,
		Skew:      totpSkew,
		Digits:    totpDigits,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return false, fmt.Errorf("mfa: validate totp: %w", err)
	}
	return ok, nil
}

// GenerateBackupCodes returns N freshly-generated plaintext backup
// codes alongside their bcrypt hashes. The plaintext slice is shown to
// the user once and must NEVER be persisted; the hashed slice goes
// into UserMFASecret.BackupCodesHashed.
//
// Plaintext codes are in "ABCDE-FGHIJ" form — 10 base32 characters
// (Crockford alphabet, uppercased) with a hyphen for readability.
func (s *MFAService) GenerateBackupCodes(n int) (plaintext []string, hashes []string, err error) {
	if n <= 0 {
		n = MFABackupCodeCount
	}
	plaintext = make([]string, 0, n)
	hashes = make([]string, 0, n)
	for range n {
		raw := make([]byte, backupCodeRawLen)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, fmt.Errorf("mfa: rand: %w", err)
		}
		code := formatBackupCode(raw)
		// Hash the *normalized* form so future verification can
		// compare against any cosmetic variant the user types
		// (lowercase, spaces, missing/extra hyphen). The plaintext
		// returned to the user keeps the hyphen for readability.
		hash, err := bcrypt.GenerateFromPassword([]byte(normalizeBackupCode(code)), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, fmt.Errorf("mfa: bcrypt: %w", err)
		}
		plaintext = append(plaintext, code)
		hashes = append(hashes, string(hash))
	}
	return plaintext, hashes, nil
}

// ConsumeBackupCode walks stored.BackupCodesHashed in constant order
// looking for a match against `code`. On a hit, it returns the
// remaining (post-consumption) slice — callers persist this back.
// Returns (nil, false, nil) on no match (without erroring, so the
// HTTP layer can map that to the same 401 as a wrong TOTP).
//
// Bcrypt's per-hash work is intentional: even if an attacker scrapes
// the column, brute-forcing a single backup code costs ~100ms per
// guess on the build's cost factor.
//
// Production HTTP handlers should prefer
// UserMFASecretRegistry.ConsumeBackupCodeAtomic (which takes the
// matcher returned by MatchBackupCode) so two concurrent requests
// can't both succeed against the same code (#1645 review). This
// method stays around for tests + the simple lookup case.
func (s *MFAService) ConsumeBackupCode(stored models.UserMFASecret, code string) ([]string, bool, error) {
	matcher := s.MatchBackupCode(code)
	if matcher == nil {
		return nil, false, nil
	}
	remaining := make([]string, 0, len(stored.BackupCodesHashed))
	matched := false
	for _, hash := range stored.BackupCodesHashed {
		if !matched && matcher(hash) {
			matched = true
			continue // skip — this code is consumed
		}
		remaining = append(remaining, hash)
	}
	if !matched {
		return nil, false, nil
	}
	return remaining, true, nil
}

// MatchBackupCode returns a closure that bcrypt-compares stored hashes
// against the user-supplied `code` (post-normalisation). The closure
// is what the registry's ConsumeBackupCodeAtomic feeds each candidate
// hash from inside the row-locked transaction. Returns nil when the
// code normalises to the empty string — the registry treats nil as
// "no match" without entering the lock at all.
func (s *MFAService) MatchBackupCode(code string) func(hash string) bool {
	cleaned := normalizeBackupCode(code)
	if cleaned == "" {
		return nil
	}
	cleanedBytes := []byte(cleaned)
	return func(hash string) bool {
		return bcrypt.CompareHashAndPassword([]byte(hash), cleanedBytes) == nil
	}
}

// VerifyPassword is a small convenience so the Disable endpoint can
// gate behind "knows the password" without duplicating the bcrypt
// compare. Returns true if `password` matches user.PasswordHash.
// Uses subtle.ConstantTimeEq on the bcrypt result to defang timing
// channels — bcrypt itself is already constant-time per cost factor,
// but this keeps callers from branching on the bool early.
func VerifyPassword(user *models.User, password string) bool {
	if user == nil || user.PasswordHash == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return subtle.ConstantTimeEq(boolToInt32(err == nil), 1) == 1
}

func boolToInt32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// normalizeCode strips whitespace, hyphens, and dots so a code pasted
// from an email or read aloud still verifies. The 6-digit TOTP space
// only uses 0-9.
func normalizeCode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizeBackupCode upper-cases and strips non-base32 characters so
// "abcde-fghij" and " ABCDE FGHIJ " all hit the same hash. The
// generator emits codes from the standard RFC 4648 base32 alphabet
// (A–Z plus 2–7) — we tolerate stray characters from paste noise but
// only A–Z / 0–9 are kept; the hash compare on the normalised form
// is the source of truth either way.
func normalizeBackupCode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// formatBackupCode produces "ABCDE-FGHIJ" from 6 random bytes using the
// standard RFC 4648 base32 alphabet (A–Z plus 2–7). 6 bytes encode to
// exactly 10 characters with no padding — keep that invariant in mind
// before swapping in a different encoding.
func formatBackupCode(raw []byte) string {
	enc := strings.TrimRight(base32.StdEncoding.EncodeToString(raw), "=")
	if len(enc) < 10 {
		// Defensive — base32 of 6 bytes is always exactly 10 chars,
		// but in case the alphabet changes guard against panics.
		return enc
	}
	return enc[:5] + "-" + enc[5:10]
}
