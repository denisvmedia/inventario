package memory_test

import (
	"context"
	"sync"
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
	c.Assert(row.BackupCodesHashed, qt.HasLen, 2)
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

// TestUserMFASecrets_ConsumeBackupCodeAtomic_Concurrent loads the
// "Atomic" in the method name with actual concurrent traffic. N
// goroutines all try to consume the same plaintext code; exactly one
// must observe consumed=true and the others must observe consumed=false.
//
// The memory impl's `r.lock.Lock()` is the unit-under-test here. The
// postgres impl relies on `SELECT … FOR UPDATE` to provide the same
// guarantee against a real DB; that path needs an integration test
// against a live postgres which we don't run in unit-test CI.
func TestUserMFASecrets_ConsumeBackupCodeAtomic_Concurrent(t *testing.T) {
	c := qt.New(t)
	r := memory.NewUserMFASecretRegistry()

	const plaintext = "RACE0-TARGT"
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.MinCost)
	c.Assert(err, qt.IsNil)

	mfa := models.UserMFASecret{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: "t1", UserID: "u1"},
		SecretEncrypted:         "irrelevant-here",
		BackupCodesHashed:       models.ValuerSlice[string]{string(hash)},
	}
	_, err = r.Create(context.Background(), mfa)
	c.Assert(err, qt.IsNil)

	matcher := func(stored string) bool {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(plaintext)) == nil
	}

	const goroutines = 16
	var wg sync.WaitGroup
	results := make([]bool, goroutines)
	errs := make([]error, goroutines)
	start := make(chan struct{})
	wg.Add(goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			<-start
			ok, err := r.ConsumeBackupCodeAtomic(
				context.Background(), "t1", "u1", time.Now(), matcher,
			)
			results[idx] = ok
			errs[idx] = err
		}(i)
	}
	close(start)
	wg.Wait()

	winners := 0
	for i, ok := range results {
		c.Assert(errs[i], qt.IsNil, qt.Commentf("goroutine %d errored", i))
		if ok {
			winners++
		}
	}
	c.Assert(winners, qt.Equals, 1, qt.Commentf("expected exactly one winner, got %d (results=%v)", winners, results))

	// Post-condition: the row's BackupCodesHashed is now empty.
	row, err := r.GetByUser(context.Background(), "t1", "u1")
	c.Assert(err, qt.IsNil)
	c.Assert(row.BackupCodesHashed, qt.HasLen, 0)
}
