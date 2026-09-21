package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
)

// The memory registry's concurrency test covers its own mutex and says so:
// the postgres path leans on `SELECT … FOR UPDATE` instead, and that is a
// property of the database rather than of the Go code. It is testable now
// that this lane runs against a real postgres (#2114 N8), and it is worth
// testing because the failure is an authentication bypass — a backup code
// accepted twice is a backup code that was never single-use.
func TestUserMFASecretRegistryPostgres_ConsumeBackupCodeAtomic_Concurrent(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	reg := postgres.NewFactorySet(dbx).CreateServiceRegistrySet().UserMFASecretRegistry

	ctx := context.Background()

	const target = "RACE0-TARGT"
	const spare = "SPARE-CODE1"
	targetHash := hashBackupCode(c, target)
	spareHash := hashBackupCode(c, spare)

	created, err := reg.Create(ctx, models.UserMFASecret{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		SecretEncrypted:   "irrelevant-to-this-test",
		BackupCodesHashed: models.ValuerSlice[string]{targetHash, spareHash},
	})
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = reg.Delete(context.Background(), created.ID) })

	matcher := func(stored string) bool {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(target)) == nil
	}

	const goroutines = 8
	var wg sync.WaitGroup
	results := make([]bool, goroutines)
	errs := make([]error, goroutines)
	start := make(chan struct{})

	wg.Add(goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			<-start
			ok, cerr := reg.ConsumeBackupCodeAtomic(
				context.Background(), user.TenantID, user.ID, time.Now(), matcher,
			)
			results[idx] = ok
			errs[idx] = cerr
		}(i)
	}
	close(start)
	wg.Wait()

	consumed := 0
	for i, ok := range results {
		c.Check(errs[i], qt.IsNil, qt.Commentf("goroutine %d", i))
		if ok {
			consumed++
		}
	}
	c.Assert(consumed, qt.Equals, 1,
		qt.Commentf("%d of %d racers consumed the same backup code", consumed, goroutines))

	// The row is left with the untouched code and nothing else: the winner
	// removed exactly one hash and the losers removed none.
	after, err := reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(after.BackupCodesHashed), qt.HasLen, 1)
	c.Check(bcrypt.CompareHashAndPassword([]byte(after.BackupCodesHashed[0]), []byte(spare)), qt.IsNil)
}

// A code that matches nothing leaves the row alone rather than consuming an
// arbitrary one, and the caller maps the false to a 401.
func TestUserMFASecretRegistryPostgres_ConsumeBackupCodeAtomic_NoMatchChangesNothing(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	reg := postgres.NewFactorySet(dbx).CreateServiceRegistrySet().UserMFASecretRegistry

	ctx := context.Background()
	hash := hashBackupCode(c, "KEEP0-THIS1")

	created, err := reg.Create(ctx, models.UserMFASecret{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		SecretEncrypted:   "irrelevant-to-this-test",
		BackupCodesHashed: models.ValuerSlice[string]{hash},
	})
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = reg.Delete(context.Background(), created.ID) })

	ok, err := reg.ConsumeBackupCodeAtomic(ctx, user.TenantID, user.ID, time.Now(),
		func(string) bool { return false })
	c.Assert(err, qt.IsNil)
	c.Check(ok, qt.IsFalse)

	after, err := reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Check([]string(after.BackupCodesHashed), qt.DeepEquals, []string{hash})
}

// A user with no MFA row is not a match; it is a lookup that found nothing.
func TestUserMFASecretRegistryPostgres_ConsumeBackupCodeAtomic_NoRow(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	reg := postgres.NewFactorySet(dbx).CreateServiceRegistrySet().UserMFASecretRegistry

	_, err = reg.ConsumeBackupCodeAtomic(context.Background(), user.TenantID,
		"00000000-0000-0000-0000-000000000000", time.Now(), func(string) bool { return true })
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func hashBackupCode(c *qt.C, plaintext string) string {
	c.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.MinCost)
	c.Assert(err, qt.IsNil)
	return string(hash)
}
