package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

func TestLocationGroupRegistry_HiddenSections_RoundTrip(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	reg := memory.NewLocationGroupRegistry()

	created, err := reg.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Slug:                "abcdefghijklmnopqrstuv",
		Name:                "Stamps",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           "user-1",
		HiddenSections:      models.GroupSections{models.GroupSectionWarranties},
	})
	c.Assert(err, qt.IsNil)

	got, err := reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties})

	got.HiddenSections = models.GroupSections{models.GroupSectionLoans, models.GroupSectionUpcoming}
	_, err = reg.Update(ctx, *got)
	c.Assert(err, qt.IsNil)

	bySlug, err := reg.GetBySlug(ctx, "tenant-1", "abcdefghijklmnopqrstuv")
	c.Assert(err, qt.IsNil)
	c.Assert(bySlug.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionLoans, models.GroupSectionUpcoming})

	listed, err := reg.ListByTenant(ctx, "tenant-1")
	c.Assert(err, qt.IsNil)
	c.Assert(listed, qt.HasLen, 1)
	c.Assert(listed[0].HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionLoans, models.GroupSectionUpcoming})
}

func TestLocationGroupRegistry_SetHiddenSections(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	reg := memory.NewLocationGroupRegistry()

	created, err := reg.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Slug:                "abcdefghijklmnopqrstuv",
		Name:                "Books",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           "user-1",
	})
	c.Assert(err, qt.IsNil)

	// A rename stored after the caller read the group is kept.
	stale, err := reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	renamed := *stale
	renamed.Name = "Book collection"
	_, err = reg.Update(ctx, renamed)
	c.Assert(err, qt.IsNil)

	at := time.Now().Add(time.Minute)
	got, applied, err := reg.SetHiddenSections(ctx, created.ID, models.GroupSections{models.GroupSectionLoans, models.GroupSectionWarranties}, at)
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
	c.Assert(got.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties, models.GroupSectionLoans})
	stored, err := reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Name, qt.Equals, "Book collection")
	c.Assert(stored.UpdatedAt.Equal(at), qt.IsTrue)

	// A group whose deletion started is left as it is.
	pending := *stored
	pending.Status = models.LocationGroupStatusPendingDeletion
	_, err = reg.Update(ctx, pending)
	c.Assert(err, qt.IsNil)
	_, applied, err = reg.SetHiddenSections(ctx, created.ID, models.GroupSections{}, time.Now())
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsFalse)
	stored, err = reg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties, models.GroupSectionLoans})

	_, _, err = reg.SetHiddenSections(ctx, "no-such-group", models.GroupSections{}, time.Now())
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
