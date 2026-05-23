package memory_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// TestSystemAdminGrantRegistry_Grant_Idempotent verifies the second
// Grant call against the same user returns (true, nil) and does NOT
// add a second row. The CLI grant flow leans on this so re-running
// `inventario admin grant-system-admin alice@example.com` prints
// "already a system admin" rather than failing on a unique-constraint
// surfaced from the registry.
func TestSystemAdminGrantRegistry_Grant_Idempotent(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewSystemAdminGrantRegistry()

	hadGrant, err := r.Grant(ctx, "user-1", nil)
	c.Assert(err, qt.IsNil)
	c.Assert(hadGrant, qt.IsFalse)

	hadGrant, err = r.Grant(ctx, "user-1", nil)
	c.Assert(err, qt.IsNil)
	c.Assert(hadGrant, qt.IsTrue)

	grants, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(grants, qt.HasLen, 1)
}

// TestSystemAdminGrantRegistry_Exists_AfterGrantAndRevoke walks the
// happy path: a freshly-granted user reads as Exists=true; once
// revoked, reads as Exists=false. This is the RequireSystemAdmin
// middleware's hot path so any divergence between Grant/Revoke and
// the Exists view would 403 a legitimate admin or admit a revoked one.
func TestSystemAdminGrantRegistry_Exists_AfterGrantAndRevoke(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewSystemAdminGrantRegistry()
	// Pre-seed a second grant so the last-admin guard doesn't block the
	// revoke under test.
	_, err := r.Grant(ctx, "user-keeper", nil)
	c.Assert(err, qt.IsNil)

	exists, err := r.Exists(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)

	_, err = r.Grant(ctx, "user-1", nil)
	c.Assert(err, qt.IsNil)

	exists, err = r.Exists(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)

	hadGrant, err := r.RevokeAtomic(ctx, "user-1", false)
	c.Assert(err, qt.IsNil)
	c.Assert(hadGrant, qt.IsTrue)

	exists, err = r.Exists(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
}

// TestSystemAdminGrantRegistry_RevokeAtomic_NoGrant returns (false, nil)
// when the target user has no grant — the CLI revoke flow needs that
// to print "wasn't a system admin" rather than fail noisily on a no-op.
func TestSystemAdminGrantRegistry_RevokeAtomic_NoGrant(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewSystemAdminGrantRegistry()

	hadGrant, err := r.RevokeAtomic(ctx, "user-1", false)
	c.Assert(err, qt.IsNil)
	c.Assert(hadGrant, qt.IsFalse)
}

// TestSystemAdminGrantRegistry_RevokeAtomic_LastAdminGuard exercises
// the safety check: revoking the only remaining grant returns
// ErrLastSystemAdmin (hadGrant=true so the caller can show "would
// have removed your admin status, refusing"). Without allowZero, the
// platform must always retain at least one admin.
func TestSystemAdminGrantRegistry_RevokeAtomic_LastAdminGuard(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewSystemAdminGrantRegistry()
	_, err := r.Grant(ctx, "alice", nil)
	c.Assert(err, qt.IsNil)

	hadGrant, err := r.RevokeAtomic(ctx, "alice", false)
	c.Assert(errors.Is(err, registry.ErrLastSystemAdmin), qt.IsTrue,
		qt.Commentf("expected ErrLastSystemAdmin, got %v", err))
	c.Assert(hadGrant, qt.IsTrue)

	// Grant still present after the rejected revoke.
	exists, err := r.Exists(ctx, "alice")
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)
}

// TestSystemAdminGrantRegistry_RevokeAtomic_AllowZeroBypass verifies
// the --allow-zero CLI escape hatch. With allowZero=true, the last
// grant CAN be removed — used for deliberate platform shutdown only.
func TestSystemAdminGrantRegistry_RevokeAtomic_AllowZeroBypass(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewSystemAdminGrantRegistry()
	_, err := r.Grant(ctx, "alice", nil)
	c.Assert(err, qt.IsNil)

	hadGrant, err := r.RevokeAtomic(ctx, "alice", true)
	c.Assert(err, qt.IsNil)
	c.Assert(hadGrant, qt.IsTrue)

	grants, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(grants, qt.HasLen, 0)
}

// TestSystemAdminGrantRegistry_RevokeAtomic_NoConcurrentLastAdminRevoke
// pins the race-safety guarantee — the analogue of the users-row test
// for the new grant table. Two concurrent revokes against distinct
// admins on a two-admin system MUST end with exactly one admin
// remaining, never zero. The memory backend uses the per-registry
// mutex as the atomicity boundary; postgres uses
// pg_advisory_xact_lock.
func TestSystemAdminGrantRegistry_RevokeAtomic_NoConcurrentLastAdminRevoke(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewSystemAdminGrantRegistry()
	_, err := r.Grant(ctx, "alice", nil)
	c.Assert(err, qt.IsNil)
	_, err = r.Grant(ctx, "bob", nil)
	c.Assert(err, qt.IsNil)

	var wg sync.WaitGroup
	results := make([]struct {
		hadGrant bool
		err      error
	}, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		results[0].hadGrant, results[0].err = r.RevokeAtomic(ctx, "alice", false)
	}()
	go func() {
		defer wg.Done()
		results[1].hadGrant, results[1].err = r.RevokeAtomic(ctx, "bob", false)
	}()
	wg.Wait()

	successes := 0
	rejections := 0
	for _, res := range results {
		switch {
		case res.err == nil && res.hadGrant:
			successes++
		case errors.Is(res.err, registry.ErrLastSystemAdmin):
			rejections++
		}
	}
	c.Assert(successes, qt.Equals, 1)
	c.Assert(rejections, qt.Equals, 1)

	grants, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(grants, qt.HasLen, 1)
}

// TestSystemAdminGrantRegistry_List_OrderedByGrantedAt verifies the
// list comes back sorted by granted_at ASC. The CLI table render
// expects oldest-first ordering so an operator scanning the list sees
// the platform's longest-serving admins at the top.
func TestSystemAdminGrantRegistry_List_OrderedByGrantedAt(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewSystemAdminGrantRegistry()
	_, err := r.Grant(ctx, "alice", nil)
	c.Assert(err, qt.IsNil)
	_, err = r.Grant(ctx, "bob", nil)
	c.Assert(err, qt.IsNil)
	_, err = r.Grant(ctx, "carol", nil)
	c.Assert(err, qt.IsNil)

	grants, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(grants, qt.HasLen, 3)
	// Pull just the user_ids out so the assertion reads obvious.
	ids := make([]string, len(grants))
	for i, g := range grants {
		ids[i] = g.UserID
	}
	c.Assert(ids, qt.DeepEquals, []string{"alice", "bob", "carol"})
}

// TestSystemAdminGrantRegistry_Grant_RecordsGrantedBy ensures a
// non-nil grantedBy makes it onto the row so the CLI list output
// can show the operator-of-record column.
func TestSystemAdminGrantRegistry_Grant_RecordsGrantedBy(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewSystemAdminGrantRegistry()
	op := "operator-1"
	_, err := r.Grant(ctx, "alice", &op)
	c.Assert(err, qt.IsNil)

	grants, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(grants, qt.HasLen, 1)
	c.Assert(grants[0].GrantedBy, qt.IsNotNil)
	c.Assert(*grants[0].GrantedBy, qt.Equals, "operator-1")
}

// TestSystemAdminGrantRegistry_Exists_EmptyUserID rejects empty user
// IDs with ErrFieldRequired instead of silently returning false —
// passing "" to Exists is a programming error in the caller and
// surfacing it loudly catches that before it becomes a security bug.
func TestSystemAdminGrantRegistry_Exists_EmptyUserID(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewSystemAdminGrantRegistry()
	_, err := r.Exists(ctx, "")
	c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue,
		qt.Commentf("expected ErrFieldRequired, got %v", err))
}
