package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
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
