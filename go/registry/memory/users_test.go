package memory_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// TestUserRegistry_Create_StampsTimestamps verifies the memory backend
// fills CreatedAt / UpdatedAt with the current time when the caller
// didn't supply them. Mirrors the postgres `default_expr="CURRENT_TIMESTAMP"`
// on the same columns, so /profile renders a real "Member since"
// timestamp under dev-mode (memory://) and the seed path.
func TestUserRegistry_Create_StampsTimestamps(t *testing.T) {
	c := qt.New(t)

	r := memory.NewUserRegistry()
	before := time.Now().UTC()

	u, err := r.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Email:               "alice@example.com",
		Name:                "Alice",
	})
	c.Assert(err, qt.IsNil)

	after := time.Now().UTC()
	c.Assert(u.CreatedAt.IsZero(), qt.IsFalse, qt.Commentf("CreatedAt should be stamped"))
	c.Assert(u.UpdatedAt.IsZero(), qt.IsFalse, qt.Commentf("UpdatedAt should be stamped"))
	c.Assert(!u.CreatedAt.Before(before), qt.IsTrue)
	c.Assert(!u.CreatedAt.After(after), qt.IsTrue)
}

// TestUserRegistry_Create_PreservesExplicitTimestamps verifies a caller-set
// CreatedAt survives. Tests that pin a specific seeded date (e.g. for
// time-travel assertions) must not be overwritten by the default stamp.
func TestUserRegistry_Create_PreservesExplicitTimestamps(t *testing.T) {
	c := qt.New(t)

	r := memory.NewUserRegistry()
	fixed := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	u, err := r.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Email:               "bob@example.com",
		Name:                "Bob",
		CreatedAt:           fixed,
		UpdatedAt:           fixed,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(u.CreatedAt.Equal(fixed), qt.IsTrue)
	c.Assert(u.UpdatedAt.Equal(fixed), qt.IsTrue)
}

// TestUserRegistry_RevokeSystemAdminAtomic_NoConcurrentLastAdminRevoke
// pins the race-safety guarantee: two concurrent revokes against
// distinct admins on a two-admin system MUST end with exactly one admin
// remaining, never zero. Before the atomic registry method existed the
// service ran ListSystemAdmins+Update as separate calls and both
// goroutines could pass `len > 1` then both clear their flag.
//
// The memory backend uses the registry write mutex (held across the
// count check + update) as the atomicity boundary; postgres uses
// pg_advisory_xact_lock. This test exercises the memory path and the
// postgres path has an analogous integration test under the postgres
// build tag.
func TestUserRegistry_RevokeSystemAdminAtomic_NoConcurrentLastAdminRevoke(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewUserRegistry()
	mkAdmin := func(email string) *models.User {
		u, err := r.Create(ctx, models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
			Email:               email,
			Name:                email,
			IsSystemAdmin:       true,
		})
		c.Assert(err, qt.IsNil)
		return u
	}
	a := mkAdmin("alice@example.com")
	b := mkAdmin("bob@example.com")

	// Two goroutines race to revoke distinct admins on a 2-admin system.
	// One MUST win (returns hadFlag=true, nil err); the other MUST be
	// rejected with ErrLastSystemAdmin. Either ordering is valid.
	var wg sync.WaitGroup
	results := make([]struct {
		hadFlag bool
		err     error
	}, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		results[0].hadFlag, results[0].err = r.RevokeSystemAdminAtomic(ctx, a.ID, false)
	}()
	go func() {
		defer wg.Done()
		results[1].hadFlag, results[1].err = r.RevokeSystemAdminAtomic(ctx, b.ID, false)
	}()
	wg.Wait()

	// Exactly one succeeded; exactly one returned ErrLastSystemAdmin.
	successes := 0
	rejections := 0
	for _, res := range results {
		switch {
		case res.err == nil && res.hadFlag:
			successes++
		case errors.Is(res.err, registry.ErrLastSystemAdmin):
			rejections++
		}
	}
	c.Assert(successes, qt.Equals, 1)
	c.Assert(rejections, qt.Equals, 1)

	// And the registry must still report exactly one admin.
	admins, err := r.ListSystemAdmins(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(admins, qt.HasLen, 1)
}
