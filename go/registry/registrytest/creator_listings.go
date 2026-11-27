package registrytest

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// CreatorListingsEnv is what RunCreatorListingsContract needs from a backend:
// a factory set and two members of one empty group, each with the context
// that acts as that member in the group, and an area of the group's one
// location to place items in.
type CreatorListingsEnv struct {
	Factories    *registry.FactorySet
	CtxA, CtxB   context.Context
	UserA, UserB string
	LocationID   string
	LocationName string
	AreaID       string
	AreaName     string
}

// creatorRows are the ids of the rows one member created.
type creatorRows struct {
	item, trashedItem, file, trashedFile       string
	loan, schedule, service, link, reminder    string
	draft, event                               string
	itemName, trashedItemName, draftUUID, user string
}

// RunCreatorListingsContract checks the creator listings the personal data
// export reads (#2934): each returns the rows of its List, or of its Trash
// listing, that one member created and none of the other member's, and the
// ref listings name every row of the group.
func RunCreatorListingsContract(t *testing.T, newEnv func(t *testing.T) CreatorListingsEnv) {
	c := qt.New(t)
	env := newEnv(t)
	a := seedCreatorRows(c, env, env.CtxA, env.UserA, "A")
	b := seedCreatorRows(c, env, env.CtxB, env.UserB, "B")
	// A's loan and history entry are on B's item: a row is the user's when
	// the user created it, whoever created the item.
	a.loan = creatorLoan(c, env, env.CtxA, b.item)
	a.event = creatorEvent(c, env, env.CtxA, b.item)
	b.loan = creatorLoan(c, env, env.CtxB, a.item)
	b.event = creatorEvent(c, env, env.CtxB, a.item)

	for _, m := range []struct {
		name string
		ctx  context.Context
		rows creatorRows
	}{{"A", env.CtxA, a}, {"B", env.CtxB, b}} {
		c.Run("member "+m.name, func(c *qt.C) {
			checkCreatorListings(c, env, m.ctx, m.rows)
		})
	}

	c.Run("refs name every row of the group", func(c *qt.C) {
		ctx := env.CtxA
		items, err := mustUserRegistry(c, ctx, env.Factories.CommodityRegistryFactory.CreateUserRegistry).ListRefs(ctx)
		c.Assert(err, qt.IsNil)
		names := map[string]string{}
		for _, ref := range items {
			c.Check(ref.UUID, qt.Not(qt.Equals), "")
			names[ref.ID] = ref.Name
		}
		c.Check(names, qt.DeepEquals, map[string]string{
			a.item: a.itemName, a.trashedItem: a.trashedItemName,
			b.item: b.itemName, b.trashedItem: b.trashedItemName,
		})

		drafts, err := mustUserRegistry(c, ctx, env.Factories.CommodityDraftRegistryFactory.CreateUserRegistry).ListRefs(ctx)
		c.Assert(err, qt.IsNil)
		uuids := map[string]string{}
		for _, ref := range drafts {
			uuids[ref.ID] = ref.UUID
		}
		c.Check(uuids, qt.DeepEquals, map[string]string{a.draft: a.draftUUID, b.draft: b.draftUUID})

		areas, err := mustUserRegistry(c, ctx, env.Factories.AreaRegistryFactory.CreateUserRegistry).ListRefs(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(areas, qt.HasLen, 1)
		c.Check(areas[0].ID, qt.Equals, env.AreaID)
		c.Check(areas[0].Name, qt.Equals, env.AreaName)
		c.Check(areas[0].LocationID, qt.Equals, env.LocationID)
		c.Check(areas[0].UUID, qt.Not(qt.Equals), "")

		locations, err := mustUserRegistry(c, ctx, env.Factories.LocationRegistryFactory.CreateUserRegistry).ListRefs(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(locations, qt.HasLen, 1)
		c.Check(locations[0].ID, qt.Equals, env.LocationID)
		c.Check(locations[0].Name, qt.Equals, env.LocationName)
	})
}

func mustUserRegistry[R any](c *qt.C, ctx context.Context, create func(context.Context) (R, error)) R {
	c.Helper()
	reg, err := create(ctx)
	c.Assert(err, qt.IsNil)
	return reg
}

// idsOf returns the ids of rows.
func idsOf[T any, P interface {
	*T
	GetID() string
}](rows []P) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.GetID())
	}
	return out
}

func checkCreatorListings(c *qt.C, env CreatorListingsEnv, ctx context.Context, want creatorRows) {
	user := want.user
	fs := env.Factories
	check := func(what string, ids []string, err error, wantIDs ...string) {
		c.Helper()
		c.Assert(err, qt.IsNil, qt.Commentf("%s", what))
		c.Check(ids, qt.ContentEquals, wantIDs, qt.Commentf("%s", what))
	}

	items := mustUserRegistry(c, ctx, fs.CommodityRegistryFactory.CreateUserRegistry)
	live, err := items.ListCreatedBy(ctx, user)
	check("items", idsOf(live), err, want.item)
	trashed, err := items.ListTrashedCreatedBy(ctx, user)
	check("items in Trash", idsOf(trashed), err, want.trashedItem)

	files := mustUserRegistry(c, ctx, fs.FileRegistryFactory.CreateUserRegistry)
	liveFiles, err := files.ListCreatedBy(ctx, user)
	check("files", idsOf(liveFiles), err, want.file)
	if err == nil && len(liveFiles) == 1 {
		c.Check(liveFiles[0].Links, qt.HasLen, 1, qt.Commentf("a file comes with its links"))
	}
	trashedFiles, err := files.ListTrashedCreatedBy(ctx, user)
	check("files in Trash", idsOf(trashedFiles), err, want.trashedFile)

	loans, err := mustUserRegistry(c, ctx, fs.CommodityLoanRegistryFactory.CreateUserRegistry).ListCreatedBy(ctx, user)
	check("loans", idsOf(loans), err, want.loan)
	schedules, err := mustUserRegistry(c, ctx, fs.MaintenanceScheduleRegistryFactory.CreateUserRegistry).ListCreatedBy(ctx, user)
	check("maintenance schedules", idsOf(schedules), err, want.schedule)
	services, err := mustUserRegistry(c, ctx, fs.CommodityServiceRegistryFactory.CreateUserRegistry).ListCreatedBy(ctx, user)
	check("service records", idsOf(services), err, want.service)
	links, err := mustUserRegistry(c, ctx, fs.SupplyLinkRegistryFactory.CreateUserRegistry).ListCreatedBy(ctx, user)
	check("supply links", idsOf(links), err, want.link)
	reminders, err := mustUserRegistry(c, ctx, fs.CommodityReminderRegistryFactory.CreateUserRegistry).ListCreatedBy(ctx, user)
	check("reminders", idsOf(reminders), err, want.reminder)
	drafts, err := mustUserRegistry(c, ctx, fs.CommodityDraftRegistryFactory.CreateUserRegistry).ListCreatedBy(ctx, user)
	check("drafts", idsOf(drafts), err, want.draft)
	events, err := mustUserRegistry(c, ctx, fs.CommodityEventRegistryFactory.CreateUserRegistry).ListCreatedBy(ctx, user)
	check("history entries", idsOf(events), err, want.event)
}

// seedCreatorRows creates, as the member of ctx, one row of each kind the
// creator listings read, on a live item, and an item and a file in Trash.
// The loan and the history entry are added by the caller.
func seedCreatorRows(c *qt.C, env CreatorListingsEnv, ctx context.Context, userID, tag string) creatorRows {
	c.Helper()
	fs := env.Factories
	rows := creatorRows{user: userID, itemName: "Drill " + tag, trashedItemName: "Saw " + tag}
	items := mustUserRegistry(c, ctx, fs.CommodityRegistryFactory.CreateUserRegistry)
	newItem := func(name string) string {
		item, err := items.Create(ctx, models.Commodity{
			Name: name, ShortName: name, Type: models.CommodityTypeOther,
			AreaID: new(env.AreaID), Count: 1, Status: models.CommodityStatusInUse,
		})
		c.Assert(err, qt.IsNil)
		return item.ID
	}
	rows.item = newItem(rows.itemName)
	rows.trashedItem = newItem(rows.trashedItemName)

	files := mustUserRegistry(c, ctx, fs.FileRegistryFactory.CreateUserRegistry)
	newFile := func(name string) string {
		file, err := files.Create(ctx, models.FileEntity{
			Title: name, Type: models.FileTypeDocument, Category: models.FileCategoryDocuments,
			Links: []models.FileLink{{EntityType: models.FileLinkCommodity, EntityID: rows.item, Meta: models.FileLinkMetaDocuments}},
			File:  &models.File{Path: name, OriginalPath: "files/" + name, Ext: ".pdf", MIMEType: "application/pdf"},
		})
		c.Assert(err, qt.IsNil)
		return file.ID
	}
	rows.file = newFile("manual-" + tag + ".pdf")
	rows.trashedFile = newFile("receipt-" + tag + ".pdf")
	now := time.Now()
	c.Assert(files.Trash(ctx, rows.trashedFile, userID, "", now), qt.IsNil)

	schedule, err := mustUserRegistry(c, ctx, fs.MaintenanceScheduleRegistryFactory.CreateUserRegistry).Create(ctx, models.MaintenanceSchedule{
		CommodityID: rows.item, Title: "Sharpen " + tag, IntervalDays: 90, NextDueAt: "2027-01-10", Enabled: true,
	})
	c.Assert(err, qt.IsNil)
	rows.schedule = schedule.ID
	service, err := mustUserRegistry(c, ctx, fs.CommodityServiceRegistryFactory.CreateUserRegistry).Create(ctx, models.CommodityService{
		CommodityID: rows.item, ProviderName: "Repair shop " + tag, Reason: "Blade", SentAt: "2026-05-01", CostCurrency: "USD",
	})
	c.Assert(err, qt.IsNil)
	rows.service = service.ID
	link, err := mustUserRegistry(c, ctx, fs.SupplyLinkRegistryFactory.CreateUserRegistry).Create(ctx, models.SupplyLink{
		CommodityID: rows.item, Label: "Spare blades " + tag, URL: "https://example.com/blades",
	})
	c.Assert(err, qt.IsNil)
	rows.link = link.ID
	reminder, err := mustUserRegistry(c, ctx, fs.CommodityReminderRegistryFactory.CreateUserRegistry).Create(ctx, models.CommodityReminder{
		CommodityID: rows.item, Title: "Oil " + tag, DueOn: models.Date("2027-01-01"),
		RemindDaysBefore: models.ValuerSlice[int]{7}, RepeatUnit: models.ReminderRepeatMonth, RepeatInterval: 1,
	})
	c.Assert(err, qt.IsNil)
	rows.reminder = reminder.ID
	draft, created, err := mustUserRegistry(c, ctx, fs.CommodityDraftRegistryFactory.CreateUserRegistry).CreateOrGet(ctx, models.CommodityDraft{
		ClientKey: "creator-" + tag, Values: models.CommodityDraftValues{"name": "Lamp " + tag},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	rows.draft, rows.draftUUID = draft.ID, draft.UUID

	_, err = items.Trash(ctx, rows.trashedItem, userID, now)
	c.Assert(err, qt.IsNil)
	return rows
}

func creatorLoan(c *qt.C, env CreatorListingsEnv, ctx context.Context, itemID string) string {
	c.Helper()
	loan, err := mustUserRegistry(c, ctx, env.Factories.CommodityLoanRegistryFactory.CreateUserRegistry).Create(ctx, models.CommodityLoan{
		CommodityID: itemID, BorrowerName: "Neighbor", LentAt: models.Date("2026-05-01"),
	})
	c.Assert(err, qt.IsNil)
	return loan.ID
}

func creatorEvent(c *qt.C, env CreatorListingsEnv, ctx context.Context, itemID string) string {
	c.Helper()
	event, err := mustUserRegistry(c, ctx, env.Factories.CommodityEventRegistryFactory.CreateUserRegistry).Create(ctx, models.CommodityEvent{
		CommodityID: itemID, Kind: models.CommodityEventKindUpdated, OccurredAt: time.Now(),
	})
	c.Assert(err, qt.IsNil)
	return event.ID
}
