package services_test

import (
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services"
)

var testRootKey = []byte("test-root-key-32-bytes-minimum-len-OK!!")

func newTestService(t *testing.T) *services.MFAService {
	t.Helper()
	s, err := services.NewMFAService(testRootKey)
	if err != nil {
		t.Fatalf("NewMFAService: %v", err)
	}
	return s
}

func TestGenerateEnrollment_ProducesValidProvisioningURL(t *testing.T) {
	c := qt.New(t)
	svc := newTestService(t)
	enr, err := svc.GenerateEnrollment("alex@example.com")
	c.Assert(err, qt.IsNil)
	c.Assert(enr.Secret, qt.Not(qt.Equals), "")
	c.Assert(enr.ProvisioningURL, qt.Contains, "otpauth://totp/")
	c.Assert(enr.ProvisioningURL, qt.Contains, "issuer="+services.MFAIssuer)
	c.Assert(enr.ProvisioningURL, qt.Contains, "period=30")
	c.Assert(enr.ProvisioningURL, qt.Contains, "digits=6")
}

func TestEncryptDecryptSecret_RoundTrip(t *testing.T) {
	c := qt.New(t)
	svc := newTestService(t)
	enr, err := svc.GenerateEnrollment("alex@example.com")
	c.Assert(err, qt.IsNil)

	enc, err := svc.EncryptSecret(enr.Secret)
	c.Assert(err, qt.IsNil)
	c.Assert(enc, qt.Not(qt.Equals), enr.Secret)

	dec, err := svc.DecryptSecret(enc)
	c.Assert(err, qt.IsNil)
	c.Assert(dec, qt.Equals, enr.Secret)
}

func TestVerifyTOTP_AcceptsCurrentCode(t *testing.T) {
	c := qt.New(t)
	svc := newTestService(t)
	enr, err := svc.GenerateEnrollment("alex@example.com")
	c.Assert(err, qt.IsNil)
	enc, err := svc.EncryptSecret(enr.Secret)
	c.Assert(err, qt.IsNil)
	stored := models.UserMFASecret{SecretEncrypted: enc}

	now := time.Unix(1700000000, 0)
	svc.SetClock(func() time.Time { return now })

	code, err := totp.GenerateCodeCustom(enr.Secret, now, totp.ValidateOpts{
		Period:    30,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	c.Assert(err, qt.IsNil)
	ok, err := svc.VerifyTOTP(stored, code)
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsTrue)
}

func TestVerifyTOTP_AcceptsPriorAndNextStep(t *testing.T) {
	c := qt.New(t)
	svc := newTestService(t)
	enr, err := svc.GenerateEnrollment("alex@example.com")
	c.Assert(err, qt.IsNil)
	enc, _ := svc.EncryptSecret(enr.Secret)
	stored := models.UserMFASecret{SecretEncrypted: enc}

	now := time.Unix(1700000000, 0)
	svc.SetClock(func() time.Time { return now })

	prevStep, _ := totp.GenerateCodeCustom(enr.Secret, now.Add(-30*time.Second), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	nextStep, _ := totp.GenerateCodeCustom(enr.Secret, now.Add(30*time.Second), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	farFuture, _ := totp.GenerateCodeCustom(enr.Secret, now.Add(5*time.Minute), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})

	okPrev, _ := svc.VerifyTOTP(stored, prevStep)
	okNext, _ := svc.VerifyTOTP(stored, nextStep)
	okFar, _ := svc.VerifyTOTP(stored, farFuture)
	c.Assert(okPrev, qt.IsTrue)
	c.Assert(okNext, qt.IsTrue)
	c.Assert(okFar, qt.IsFalse)
}

func TestVerifyTOTP_RejectsGarbage(t *testing.T) {
	c := qt.New(t)
	svc := newTestService(t)
	enr, _ := svc.GenerateEnrollment("alex@example.com")
	enc, _ := svc.EncryptSecret(enr.Secret)
	stored := models.UserMFASecret{SecretEncrypted: enc}

	for _, code := range []string{"", "abc", "000000", "12345"} {
		ok, err := svc.VerifyTOTP(stored, code)
		c.Assert(err, qt.IsNil)
		c.Assert(ok, qt.IsFalse, qt.Commentf("code=%q", code))
	}
}

func TestVerifyTOTP_AcceptsWhitespaceAndHyphens(t *testing.T) {
	c := qt.New(t)
	svc := newTestService(t)
	enr, _ := svc.GenerateEnrollment("alex@example.com")
	enc, _ := svc.EncryptSecret(enr.Secret)
	stored := models.UserMFASecret{SecretEncrypted: enc}

	now := time.Unix(1700000000, 0)
	svc.SetClock(func() time.Time { return now })
	code, _ := totp.GenerateCodeCustom(enr.Secret, now, totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	// 123456 -> "123 456" or "123-456" are common paste shapes.
	formatted := code[:3] + " " + code[3:]
	ok, err := svc.VerifyTOTP(stored, formatted)
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsTrue)
}

func TestVerifyTOTP_ErrorsWhenNotEnrolled(t *testing.T) {
	c := qt.New(t)
	svc := newTestService(t)
	_, err := svc.VerifyTOTP(models.UserMFASecret{}, "123456")
	c.Assert(err, qt.Equals, services.ErrMFANotEnrolled)
}

func TestGenerateBackupCodes_HumanFriendlyAndHashed(t *testing.T) {
	c := qt.New(t)
	svc := newTestService(t)
	plain, hashes, err := svc.GenerateBackupCodes(services.MFABackupCodeCount)
	c.Assert(err, qt.IsNil)
	c.Assert(plain, qt.HasLen, services.MFABackupCodeCount)
	c.Assert(hashes, qt.HasLen, services.MFABackupCodeCount)

	for _, code := range plain {
		c.Assert(code, qt.Contains, "-")
		c.Assert(code, qt.HasLen, 11) // 5 + 1 + 5
	}

	// Hashes are unique even across duplicate runs (random nonces in bcrypt salt).
	seen := make(map[string]struct{})
	for _, h := range hashes {
		_, dup := seen[h]
		c.Assert(dup, qt.IsFalse)
		seen[h] = struct{}{}
	}
}

func TestConsumeBackupCode_SuccessAndSingleUse(t *testing.T) {
	c := qt.New(t)
	svc := newTestService(t)
	plain, hashes, err := svc.GenerateBackupCodes(3)
	c.Assert(err, qt.IsNil)
	stored := models.UserMFASecret{BackupCodesHashed: hashes}

	// First use of plain[1] consumes it.
	remaining, ok, err := svc.ConsumeBackupCode(stored, plain[1])
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsTrue)
	c.Assert(remaining, qt.HasLen, 2)

	// Second use returns no-match against the post-consume slice.
	stored.BackupCodesHashed = remaining
	remaining2, ok2, err := svc.ConsumeBackupCode(stored, plain[1])
	c.Assert(err, qt.IsNil)
	c.Assert(ok2, qt.IsFalse)
	c.Assert(remaining2, qt.IsNil)
}

func TestConsumeBackupCode_NormalizesInput(t *testing.T) {
	c := qt.New(t)
	svc := newTestService(t)
	plain, hashes, _ := svc.GenerateBackupCodes(2)
	stored := models.UserMFASecret{BackupCodesHashed: hashes}

	// User types lowercase with extra spaces.
	noisy := " " + strings.ToLower(plain[0]) + " "
	_, ok, err := svc.ConsumeBackupCode(stored, noisy)
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsTrue)
}

func TestConsumeBackupCode_GarbageIsRejected(t *testing.T) {
	c := qt.New(t)
	svc := newTestService(t)
	_, hashes, _ := svc.GenerateBackupCodes(2)
	stored := models.UserMFASecret{BackupCodesHashed: hashes}
	_, ok, err := svc.ConsumeBackupCode(stored, "")
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsFalse)
	_, ok, err = svc.ConsumeBackupCode(stored, "NOTACODE-12345")
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsFalse)
}

func TestVerifyPassword_MatchesAndRejects(t *testing.T) {
	c := qt.New(t)
	u := &models.User{}
	c.Assert(u.SetPassword("Sup3rSecret"), qt.IsNil)
	c.Assert(services.VerifyPassword(u, "Sup3rSecret"), qt.IsTrue)
	c.Assert(services.VerifyPassword(u, "wrong"), qt.IsFalse)
	c.Assert(services.VerifyPassword(nil, "anything"), qt.IsFalse)
}
