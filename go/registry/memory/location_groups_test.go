package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
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
