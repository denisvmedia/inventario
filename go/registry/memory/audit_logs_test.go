package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

// TestAuditLogRegistry_DeleteOlderThan_DeletesAllOlderRows pins the
// collect-then-delete fix: deleting the current ordered-map node
// mid-iteration unlinked it, so the previous implementation stopped
// after the first match and left every subsequent older row behind.
func TestAuditLogRegistry_DeleteOlderThan_DeletesAllOlderRows(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	reg := memory.NewAuditLogRegistry()

	// Insert several rows. Create() stamps Timestamp = time.Now(), so a
	// cutoff in the future makes every row "older" — exercising the loop
	// across all of them, not just the first.
	const total = 5
	for range total {
		_, err := reg.Create(ctx, models.AuditLog{Action: "login"})
		c.Assert(err, qt.IsNil)
	}

	all, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, total)

	err = reg.DeleteOlderThan(ctx, time.Now().Add(time.Hour))
	c.Assert(err, qt.IsNil)

	remaining, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(remaining, qt.HasLen, 0)
}

// TestAuditLogRegistry_DeleteOlderThan_KeepsNewerRows confirms the cutoff
// is honoured: rows at or after the cutoff survive.
func TestAuditLogRegistry_DeleteOlderThan_KeepsNewerRows(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	reg := memory.NewAuditLogRegistry()

	for range 3 {
		_, err := reg.Create(ctx, models.AuditLog{Action: "login"})
		c.Assert(err, qt.IsNil)
	}

	// A cutoff in the past leaves every (just-created) row in place.
	err := reg.DeleteOlderThan(ctx, time.Now().Add(-time.Hour))
	c.Assert(err, qt.IsNil)

	remaining, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(remaining, qt.HasLen, 3)
}
