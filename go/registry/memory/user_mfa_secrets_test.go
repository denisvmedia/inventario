package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// TestUserMFASecrets_ConsumeBackupCodeAtomic_RemovesMatchedHash
// pins the contract that the registry rewrites BackupCodesHashed +
// touches LastUsedAt under a single critical section, so two serial
// calls with the same plaintext can never both succeed (#1645
// review). Concurrent stress is not necessary here — the memory
// registry holds the write lock for the duration; the postgres
// implementation relies on SELECT … FOR UPDATE to provide the same
// guarantee in a real DB.
func TestUserMFASecrets_ConsumeBackupCodeAtomic_RemovesMatchedHash(t *testing.T) {
	c := qt.New(t)
	r := memory.NewUserMFASecretRegistry()

	plaintexts := []string{"BACKUP01", "BACKUP02", "BACKUP03"}
	hashes := make([]string, 0, len(plaintexts))
	for _, p := range plaintexts {
		h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.MinCost)
		c.Assert(err, qt.IsNil)
		hashes = append(hashes, string(h))
	}

	mfa := models.UserMFASecret{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: "t1",
			UserID:   "u1",
		},
		SecretEncrypted:   "irrelevant-here",
		BackupCodesHashed: models.ValuerSlice[string](hashes),
	}
	_, err := r.Create(context.Background(), mfa)
	c.Assert(err, qt.IsNil)

	// matchHash returns true for the second code only.
	match := func(target string) func(stored string) bool {
		want := []byte(target)
		return func(stored string) bool {
			return bcrypt.CompareHashAndPassword([]byte(stored), want) == nil
		}
	}

	now := time.Now()
	consumed, err := r.ConsumeBackupCodeAtomic(context.Background(), "t1", "u1", now, match("BACKUP02"))
	c.Assert(err, qt.IsNil)
	c.Assert(consumed, qt.IsTrue)

	row, err := r.GetByUser(context.Background(), "t1", "u1")
	c.Assert(err, qt.IsNil)
	c.Assert(len(row.BackupCodesHashed), qt.Equals, 2)
	c.Assert(row.LastUsedAt, qt.IsNotNil)
	c.Assert(row.LastUsedAt.Equal(now), qt.IsTrue)

	// Re-consuming the same plaintext must miss — the hash has been removed.
	consumed2, err := r.ConsumeBackupCodeAtomic(context.Background(), "t1", "u1", time.Now(), match("BACKUP02"))
	c.Assert(err, qt.IsNil)
	c.Assert(consumed2, qt.IsFalse)

	// Other codes still consume successfully.
	consumed3, err := r.ConsumeBackupCodeAtomic(context.Background(), "t1", "u1", time.Now(), match("BACKUP01"))
	c.Assert(err, qt.IsNil)
	c.Assert(consumed3, qt.IsTrue)
}

func TestUserMFASecrets_ConsumeBackupCodeAtomic_NotFound(t *testing.T) {
	c := qt.New(t)
	r := memory.NewUserMFASecretRegistry()
	_, err := r.ConsumeBackupCodeAtomic(context.Background(), "t1", "u1", time.Now(),
		func(string) bool { return true })
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestUserMFASecrets_ConsumeBackupCodeAtomic_ValidatesInputs(t *testing.T) {
	c := qt.New(t)
	r := memory.NewUserMFASecretRegistry()
	matcher := func(string) bool { return false }
	_, err := r.ConsumeBackupCodeAtomic(context.Background(), "", "u1", time.Now(), matcher)
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
	_, err = r.ConsumeBackupCodeAtomic(context.Background(), "t1", "", time.Now(), matcher)
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
	_, err = r.ConsumeBackupCodeAtomic(context.Background(), "t1", "u1", time.Now(), nil)
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
}
