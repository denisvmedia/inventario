package services_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/notifications"
)

// recordingDigestEmailService captures the digests handed to the email layer.
// It embeds StubEmailService rather than restating the interface, so a future
// email type does not need a thirteenth no-op here.
type recordingDigestEmailService struct {
	services.StubEmailService
	mu       sync.Mutex
	sends    []recordedDigest
	failWith error
}

type recordedDigest struct {
	to     string
	name   string
	digest services.WeeklyDigestEmail
}

func (r *recordingDigestEmailService) SendWeeklyDigestEmail(_ context.Context, to, name string, digest services.WeeklyDigestEmail) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failWith != nil {
		return r.failWith
	}
	r.sends = append(r.sends, recordedDigest{to: to, name: name, digest: digest})
	return nil
}

func (r *recordingDigestEmailService) recorded() []recordedDigest {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedDigest, len(r.sends))
	copy(out, r.sends)
	return out
}

// digestFixture is a tenant with one user and as many groups as a test asks
// for, wired to memory registries.
type digestFixture struct {
	factorySet *registry.FactorySet
	regSet     *registry.Set
	tenantID   string
	userID     string
	email      string
	// userCtx carries the user the registries require for a write; the digest
	// service itself needs no user context, it runs as the worker.
	userCtx context.Context
	// sentAt is a Monday 09:00 UTC, so the covered window is the Monday to
	// Sunday before it.
	sentAt time.Time
	window services.WeeklyDigestWindow
}

func newDigestFixture(c *qt.C) *digestFixture {
	c.Helper()

	factorySet := memory.NewFactorySet()
	ctx := context.Background()
	const tenantID = "digest-tenant"

	_, err := factorySet.TenantRegistry.Create(ctx, models.Tenant{
		EntityID: models.EntityID{ID: tenantID},
		Name:     "Digest Tenant",
		Slug:     "digest-tenant",
		Status:   models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	user, err := factorySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               "member@example.com",
		Name:                "Digest Member",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	sentAt := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	return &digestFixture{
		factorySet: factorySet,
		regSet:     factorySet.CreateServiceRegistrySet(),
		tenantID:   tenantID,
		userID:     user.ID,
		email:      user.Email,
		userCtx:    appctx.WithUser(ctx, user),
		sentAt:     sentAt,
		window:     services.WeeklyDigestWindowFor(sentAt),
	}
}

// addGroup creates a group and makes the fixture's user a member of it.
func (f *digestFixture) addGroup(c *qt.C, name string) *models.LocationGroup {
	c.Helper()
	ctx := context.Background()

	group, err := f.factorySet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.tenantID},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                name,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           f.userID,
	})
	c.Assert(err, qt.IsNil)

	_, err = f.factorySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.tenantID},
		GroupID:             group.ID,
		MemberUserID:        f.userID,
		Role:                models.GroupRoleAdmin,
	})
	c.Assert(err, qt.IsNil)

	return group
}

// addCommodity creates a commodity in the group, optionally with a warranty
// expiry date.
func (f *digestFixture) addCommodity(c *qt.C, group *models.LocationGroup, name string, warranty *models.Date) *models.Commodity {
	c.Helper()

	commodity, err := f.regSet.CommodityRegistry.Create(f.userCtx, models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        f.tenantID,
			GroupID:         group.ID,
			CreatedByUserID: f.userID,
		},
		Name:              name,
		ShortName:         name,
		Type:              models.CommodityTypeEquipment,
		Status:            models.CommodityStatusInUse,
		Count:             1,
		WarrantyExpiresAt: warranty,
	})
	c.Assert(err, qt.IsNil)
	return commodity
}

// addEvent records a commodity event at the given instant.
func (f *digestFixture) addEvent(c *qt.C, group *models.LocationGroup, commodityID string, kind models.CommodityEventKind, at time.Time) {
	c.Helper()

	_, err := f.factorySet.CommodityEventRegistryFactory.CreateServiceRegistry().Create(f.userCtx, models.CommodityEvent{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        f.tenantID,
			GroupID:         group.ID,
			CreatedByUserID: f.userID,
		},
		CommodityID: commodityID,
		Kind:        kind,
		OccurredAt:  at,
	})
	c.Assert(err, qt.IsNil)
}

// addFile attaches a file created at the given instant.
func (f *digestFixture) addFile(c *qt.C, group *models.LocationGroup, at time.Time) {
	c.Helper()

	_, err := f.regSet.FileRegistry.Create(f.userCtx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        f.tenantID,
			GroupID:         group.ID,
			CreatedByUserID: f.userID,
		},
		Title:    "receipt",
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		File: &models.File{
			Path:         "receipt",
			OriginalPath: "receipt.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
		CreatedAt: at,
	})
	c.Assert(err, qt.IsNil)
}

func (f *digestFixture) newService(email services.EmailService) *services.WeeklyDigestService {
	return services.NewWeeklyDigestService(f.factorySet, email,
		func(slug string) string { return "https://app.test/g/" + slug },
		func(slug string) string { return "https://app.test/g/" + slug + "/settings/notifications" },
		func(slug, id string) string { return "https://app.test/g/" + slug + "/items/" + id },
	)
}

// The headline behaviour: a user in two groups gets one email covering both,
// not one email per group.
func TestWeeklyDigestService_OneEmailPerUserAcrossGroups(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)
	inWindow := f.window.Start.Add(36 * time.Hour)

	household := f.addGroup(c, "Household")
	office := f.addGroup(c, "Office")

	drill := f.addCommodity(c, household, "Drill", nil)
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, inWindow)
	f.addFile(c, household, inWindow)
	f.addFile(c, household, inWindow)

	chair := f.addCommodity(c, office, "Chair", nil)
	f.addEvent(c, office, chair.ID, models.CommodityEventKindCreated, inWindow)

	email := &recordingDigestEmailService{}
	stats, err := f.newService(email).SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 1)

	sends := email.recorded()
	c.Assert(sends, qt.HasLen, 1)
	c.Assert(sends[0].to, qt.Equals, f.email)
	c.Assert(sends[0].digest.WeekStart, qt.Equals, "2026-09-21")
	c.Assert(sends[0].digest.WeekEnd, qt.Equals, "2026-09-27",
		qt.Commentf("the label must name the last covered day, not the exclusive end"))
	c.Assert(sends[0].digest.Groups, qt.HasLen, 2)

	byName := map[string]services.WeeklyDigestGroupCounts{}
	for _, g := range sends[0].digest.Groups {
		byName[g.GroupName] = g
	}
	c.Assert(byName["Household"].ItemsAdded, qt.Equals, 1)
	c.Assert(byName["Household"].FilesAdded, qt.Equals, 2)
	c.Assert(byName["Office"].ItemsAdded, qt.Equals, 1)
	c.Assert(byName["Office"].FilesAdded, qt.Equals, 0)
}

// A quiet week sends nothing. A digest that says "nothing happened" is how a
// weekly email teaches its reader to filter it.
func TestWeeklyDigestService_QuietWeekSendsNothing(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)

	household := f.addGroup(c, "Household")
	drill := f.addCommodity(c, household, "Drill", nil)
	// Activity a week before the covered window.
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.Start.AddDate(0, 0, -3))

	email := &recordingDigestEmailService{}
	stats, err := f.newService(email).SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 0)
	c.Assert(stats.Nothing, qt.Equals, 1)
	c.Assert(email.recorded(), qt.HasLen, 0)
}

// Only the covered week counts. The boundaries are what a naive ">= start" or
// "<= end" gets wrong, so both edges are pinned.
func TestWeeklyDigestService_CountsOnlyTheCoveredWeek(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)

	household := f.addGroup(c, "Household")
	drill := f.addCommodity(c, household, "Drill", nil)

	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.Start.Add(-time.Second))
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.Start)
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.End.Add(-time.Second))
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.End)

	email := &recordingDigestEmailService{}
	_, err := f.newService(email).SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)

	sends := email.recorded()
	c.Assert(sends, qt.HasLen, 1)
	c.Assert(sends[0].digest.Groups[0].ItemsAdded, qt.Equals, 2,
		qt.Commentf("the instant at the start is in, the one at the end is not"))
}

// One commodity edited repeatedly is one changed item. Counting event rows
// instead would report a single afternoon of tidying as a busy week.
func TestWeeklyDigestService_CountsAChangedItemOnce(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)
	inWindow := f.window.Start.Add(time.Hour)

	household := f.addGroup(c, "Household")
	drill := f.addCommodity(c, household, "Drill", nil)
	chair := f.addCommodity(c, household, "Chair", nil)

	for _, kind := range []models.CommodityEventKind{
		models.CommodityEventKindUpdated,
		models.CommodityEventKindStatusChanged,
		models.CommodityEventKindPriceChanged,
		models.CommodityEventKindMoved,
	} {
		f.addEvent(c, household, drill.ID, kind, inWindow)
	}
	f.addEvent(c, household, chair.ID, models.CommodityEventKindUpdated, inWindow)
	// A deletion is not activity worth reporting — there is nothing left to look at.
	f.addEvent(c, household, chair.ID, models.CommodityEventKindDeleted, inWindow)

	email := &recordingDigestEmailService{}
	_, err := f.newService(email).SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)

	sends := email.recorded()
	c.Assert(sends, qt.HasLen, 1)
	c.Assert(sends[0].digest.Groups[0].ItemsChanged, qt.Equals, 2)
	c.Assert(sends[0].digest.Groups[0].ItemsAdded, qt.Equals, 0)
}

// The second sweep in the same week sends nothing: the claim row is what stops
// a restart or a second replica from sending a duplicate.
func TestWeeklyDigestService_SendsOncePerWeek(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)

	household := f.addGroup(c, "Household")
	drill := f.addCommodity(c, household, "Drill", nil)
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.Start.Add(time.Hour))

	email := &recordingDigestEmailService{}
	svc := f.newService(email)

	first, err := svc.SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)
	c.Assert(first.Sent, qt.Equals, 1)

	// A later tick on the same Monday, and one on the Wednesday after.
	for _, again := range []time.Time{f.sentAt.Add(time.Hour), f.sentAt.AddDate(0, 0, 2)} {
		stats, sweepErr := svc.SendOnce(context.Background(), again)
		c.Assert(sweepErr, qt.IsNil)
		c.Assert(stats.Sent, qt.Equals, 0)
		c.Assert(stats.AlreadySent, qt.Equals, 1)
	}
	c.Assert(email.recorded(), qt.HasLen, 1)

	// The following week is a different claim and sends again, given activity.
	nextWeek := f.sentAt.AddDate(0, 0, 7)
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, nextWeek.AddDate(0, 0, -3))
	stats, err := svc.SendOnce(context.Background(), nextWeek)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 1)
	c.Assert(email.recorded(), qt.HasLen, 2)
}

// A send that fails must not consume the week: the claim is released so the
// next tick tries again.
func TestWeeklyDigestService_ReleasesTheClaimWhenSendingFails(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)

	household := f.addGroup(c, "Household")
	drill := f.addCommodity(c, household, "Drill", nil)
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.Start.Add(time.Hour))

	email := &recordingDigestEmailService{failWith: errors.New("queue down")}
	svc := f.newService(email)

	stats, err := svc.SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 1)
	c.Assert(stats.Sent, qt.Equals, 0)

	// The queue recovers and the same week goes out, rather than being lost.
	email.mu.Lock()
	email.failWith = nil
	email.mu.Unlock()

	stats, err = svc.SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 1,
		qt.Commentf("a failed send consumed the week"))
	c.Assert(email.recorded(), qt.HasLen, 1)
}

// Warranties inside the lookahead reach the digest, sorted by date and capped.
func TestWeeklyDigestService_ListsUpcomingWarranties(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)
	inWindow := f.window.Start.Add(time.Hour)

	household := f.addGroup(c, "Household")

	soon := models.Date(f.sentAt.AddDate(0, 0, 10).Format(time.DateOnly))
	later := models.Date(f.sentAt.AddDate(0, 0, 25).Format(time.DateOnly))
	beyond := models.Date(f.sentAt.AddDate(0, 0, 90).Format(time.DateOnly))
	past := models.Date(f.sentAt.AddDate(0, 0, -5).Format(time.DateOnly))

	f.addCommodity(c, household, "Later", &later)
	f.addCommodity(c, household, "Soon", &soon)
	f.addCommodity(c, household, "Beyond", &beyond)
	f.addCommodity(c, household, "Expired", &past)

	// Something has to have happened, or there is no digest to attach these to.
	drill := f.addCommodity(c, household, "Drill", nil)
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, inWindow)

	email := &recordingDigestEmailService{}
	_, err := f.newService(email).SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)

	sends := email.recorded()
	c.Assert(sends, qt.HasLen, 1)

	var names []string
	for _, u := range sends[0].digest.Upcoming {
		c.Assert(u.Kind, qt.Equals, services.WeeklyDigestUpcomingWarranty)
		// The template ends each line with the group name; left unset it
		// renders as an empty pair of brackets.
		c.Assert(u.GroupName, qt.Equals, "Household")
		names = append(names, u.Name)
	}
	c.Assert(names, qt.DeepEquals, []string{"Soon", "Later"},
		qt.Commentf("nearest first; anything past the lookahead or already expired is out"))
	c.Assert(sends[0].digest.Upcoming[0].URL, qt.Contains, "/items/")
}

// withPreferences attaches a notification service backed by the fixture's own
// registries, so a test can switch the digest off per group.
func (f *digestFixture) withPreferences(svc *services.WeeklyDigestService) *services.WeeklyDigestService {
	prefs := notifications.NewService(f.factorySet.SettingsRegistryFactory)
	prefs.SetGroupPrefs(f.factorySet.GroupNotificationPrefRegistry)
	return svc.WithPreferences(prefs)
}

// setDigest writes the per-group override that switches the digest on or off.
func (f *digestFixture) setDigest(c *qt.C, group *models.LocationGroup, enabled bool) {
	c.Helper()

	_, err := f.factorySet.GroupNotificationPrefRegistry.Create(context.Background(), models.GroupNotificationPref{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.tenantID},
		GroupID:             group.ID,
		UserID:              f.userID,
		Category:            string(notifications.CategoryWeeklyDigest),
		Enabled:             enabled,
	})
	c.Assert(err, qt.IsNil)
}

// The toggle has to decide whether the email arrives, which is the whole reason
// this issue exists: the preference shipped in the UI with nothing behind it.
//
// The digest is opt-in — notifications.categoryDefaults has weekly_digest off,
// unlike every other category — so the interesting cases are "nothing was
// enabled" and "one group was".
func TestWeeklyDigestService_RespectsTheGroupToggle(t *testing.T) {
	c := qt.New(t)

	// setup returns a fixture with two groups that both had activity, so the
	// only thing deciding the outcome is the preference.
	setup := func(c *qt.C) (*digestFixture, *models.LocationGroup, *models.LocationGroup) {
		f := newDigestFixture(c)
		inWindow := f.window.Start.Add(time.Hour)
		household := f.addGroup(c, "Household")
		office := f.addGroup(c, "Office")
		for _, g := range []*models.LocationGroup{household, office} {
			item := f.addCommodity(c, g, "Thing in "+g.Name, nil)
			f.addEvent(c, g, item.ID, models.CommodityEventKindCreated, inWindow)
		}
		return f, household, office
	}

	c.Run("nobody is opted in, so nothing is sent", func(c *qt.C) {
		f, _, _ := setup(c)

		email := &recordingDigestEmailService{}
		stats, err := f.withPreferences(f.newService(email)).SendOnce(context.Background(), f.sentAt)
		c.Assert(err, qt.IsNil)
		c.Assert(stats.Sent, qt.Equals, 0)
		c.Assert(email.recorded(), qt.HasLen, 0)
	})

	c.Run("only the group the user opted into is in the digest", func(c *qt.C) {
		f, household, _ := setup(c)
		f.setDigest(c, household, true)

		email := &recordingDigestEmailService{}
		stats, err := f.withPreferences(f.newService(email)).SendOnce(context.Background(), f.sentAt)
		c.Assert(err, qt.IsNil)
		c.Assert(stats.Sent, qt.Equals, 1)

		sends := email.recorded()
		c.Assert(sends, qt.HasLen, 1)
		c.Assert(sends[0].digest.Groups, qt.HasLen, 1)
		c.Assert(sends[0].digest.Groups[0].GroupName, qt.Equals, "Household")
	})

	c.Run("opting into one group and out of the other is the same thing", func(c *qt.C) {
		f, household, office := setup(c)
		f.setDigest(c, household, true)
		f.setDigest(c, office, false)

		email := &recordingDigestEmailService{}
		_, err := f.withPreferences(f.newService(email)).SendOnce(context.Background(), f.sentAt)
		c.Assert(err, qt.IsNil)

		sends := email.recorded()
		c.Assert(sends, qt.HasLen, 1)
		c.Assert(sends[0].digest.Groups, qt.HasLen, 1)
		c.Assert(sends[0].digest.Groups[0].GroupName, qt.Equals, "Household")
	})
}

// An inactive user gets no mail whatever their groups did — blocking an account
// has to stop the email it would otherwise keep sending.
func TestWeeklyDigestService_SkipsInactiveUsers(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)

	household := f.addGroup(c, "Household")
	drill := f.addCommodity(c, household, "Drill", nil)
	f.addEvent(c, household, drill.ID, models.CommodityEventKindCreated, f.window.Start.Add(time.Hour))

	user, err := f.factorySet.UserRegistry.Get(context.Background(), f.userID)
	c.Assert(err, qt.IsNil)
	user.IsActive = false
	_, err = f.factorySet.UserRegistry.Update(context.Background(), *user)
	c.Assert(err, qt.IsNil)

	email := &recordingDigestEmailService{}
	stats, err := f.newService(email).SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 0)
	c.Assert(email.recorded(), qt.HasLen, 0)
}
