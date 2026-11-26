package services_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

func TestGroupService_CreateGroup_HidesNothingByDefault(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Household", "", "", "")
	c.Assert(err, qt.IsNil)
	c.Assert(group.HiddenSections, qt.DeepEquals, models.GroupSections{})

	stored, err := svc.GetGroup(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.HiddenSections, qt.DeepEquals, models.GroupSections{})
}

func TestGroupService_CreateGroup_WithHiddenSections(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Stamps", "", "", "",
		services.WithHiddenSections(models.GroupSectionWarranties, models.GroupSectionWarranties))
	c.Assert(err, qt.IsNil)
	c.Assert(group.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties})
	c.Assert(group.HidesSection(models.GroupSectionWarranties), qt.IsTrue)

	stored, err := svc.GetGroup(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties})
}

func TestGroupService_CreateGroup_RefusesUnknownSection(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	_, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Stamps", "", "", "",
		services.WithHiddenSections("tags"))
	c.Assert(err, qt.ErrorIs, services.ErrInvalidGroupSection)

	groups, err := svc.ListUserGroups(ctx, "tenant-1", "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(groups, qt.HasLen, 0)
}

func TestGroupService_SetHiddenSections(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Household", "🏠", "Things at home", "EUR")
	c.Assert(err, qt.IsNil)

	updated, err := svc.SetHiddenSections(ctx, group.ID,
		models.GroupSectionUpcoming, models.GroupSectionLoans, models.GroupSectionUpcoming)
	c.Assert(err, qt.IsNil)
	// Stored once each, in the order the settings page lists them.
	c.Assert(updated.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionLoans, models.GroupSectionUpcoming})
	// The rest of the group stays as it was.
	c.Assert(updated.Name, qt.Equals, "Household")
	c.Assert(updated.Icon, qt.Equals, "🏠")
	c.Assert(updated.Description, qt.Equals, "Things at home")
	c.Assert(updated.GroupCurrency, qt.Equals, models.Currency("EUR"))

	// Renaming the group leaves the hidden sections alone.
	renamed, err := svc.UpdateGroup(ctx, group.ID, "Home", "🏠", "Things at home")
	c.Assert(err, qt.IsNil)
	c.Assert(renamed.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionLoans, models.GroupSectionUpcoming})

	// An update that carries the setting replaces it.
	replaced, err := svc.UpdateGroup(ctx, group.ID, "Home", "🏠", "Things at home",
		services.WithHiddenSections(models.GroupSectionMaintenance))
	c.Assert(err, qt.IsNil)
	c.Assert(replaced.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionMaintenance})

	// No sections shows everything again.
	shown, err := svc.SetHiddenSections(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(shown.HiddenSections, qt.DeepEquals, models.GroupSections{})

	stored, err := svc.GetGroup(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.HiddenSections, qt.DeepEquals, models.GroupSections{})
}

func TestGroupService_SetHiddenSections_RefusesUnknownSection(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Household", "", "", "",
		services.WithHiddenSections(models.GroupSectionLoans))
	c.Assert(err, qt.IsNil)

	_, err = svc.SetHiddenSections(ctx, group.ID, models.GroupSectionWarranties, "files")
	c.Assert(err, qt.ErrorIs, services.ErrInvalidGroupSection)

	stored, err := svc.GetGroup(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionLoans})
}

func TestGroupService_SetHiddenSections_RefusesGroupPendingDeletion(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "To Delete", "", "", "")
	c.Assert(err, qt.IsNil)
	c.Assert(svc.InitiateGroupDeletion(ctx, group.ID, "To Delete", "To Delete"), qt.IsNil)

	_, err = svc.SetHiddenSections(ctx, group.ID, models.GroupSectionWarranties)
	c.Assert(err, qt.ErrorIs, services.ErrGroupNotActive)
}
