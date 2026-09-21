package notifications_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services/notifications"
)

// userCtx returns a context carrying `user` so the user-aware
// SettingsRegistry materialised under it scopes its reads/writes to
// that user's rows. The tests below seed prefs from a user-scoped
// context and read them back from a service-scoped context to mirror
// what the real worker path does.
func userCtx(t *testing.T, userID, tenantID string) context.Context {
	t.Helper()
	return appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			EntityID: models.EntityID{ID: userID},
		},
	})
}

func TestIsEnabled_defaults(t *testing.T) {
	c := qt.New(t)
	factory := memory.NewSettingsRegistryFactory()
	svc := notifications.NewService(factory)

	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "t1",
			EntityID: models.EntityID{ID: "u1"},
		},
	}

	// No rows seeded → defaults apply. WarrantyExpiry default = true,
	// ChannelEmail default = true → enabled.
	c.Assert(svc.IsEnabled(context.Background(), user, notifications.CategoryWarrantyExpiry, notifications.ChannelEmail), qt.IsTrue)
	// ChannelPush default = false → push notifications start disabled.
	c.Assert(svc.IsEnabled(context.Background(), user, notifications.CategoryWarrantyExpiry, notifications.ChannelPush), qt.IsFalse)
}

func TestIsEnabled_categoryToggleOff(t *testing.T) {
	c := qt.New(t)
	factory := memory.NewSettingsRegistryFactory()
	svc := notifications.NewService(factory)

	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "t1",
			EntityID: models.EntityID{ID: "u1"},
		},
	}
	ctx := userCtx(t, user.ID, user.TenantID)

	reg, err := factory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	off := false
	c.Assert(reg.Save(ctx, models.SettingsObject{
		NotificationsWarrantyExpiry: &off,
	}), qt.IsNil)

	c.Assert(svc.IsEnabled(context.Background(), user, notifications.CategoryWarrantyExpiry, notifications.ChannelEmail), qt.IsFalse)
	// Other categories fall through to their in-code default (only the
	// explicit row above flipped). CategoryPriceDrop is the unambiguous
	// "defaults to true" probe — CategoryWeeklyDigest defaults to false
	// (#1648 mock parity), so it can't carry this assertion any more.
	c.Assert(svc.IsEnabled(context.Background(), user, notifications.CategoryPriceDrop, notifications.ChannelEmail), qt.IsTrue)
}

func TestIsEnabled_channelMasterSwitchOff(t *testing.T) {
	c := qt.New(t)
	factory := memory.NewSettingsRegistryFactory()
	svc := notifications.NewService(factory)

	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "t1",
			EntityID: models.EntityID{ID: "u1"},
		},
	}
	ctx := userCtx(t, user.ID, user.TenantID)
	reg, err := factory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	off := false
	c.Assert(reg.Save(ctx, models.SettingsObject{
		NotificationsChannelEmail: &off,
	}), qt.IsNil)

	// Channel master switch suppresses every category on that channel
	// regardless of per-category toggle.
	c.Assert(svc.IsEnabled(context.Background(), user, notifications.CategoryWarrantyExpiry, notifications.ChannelEmail), qt.IsFalse)
	c.Assert(svc.IsEnabled(context.Background(), user, notifications.CategoryWeeklyDigest, notifications.ChannelEmail), qt.IsFalse)
}

func TestIsEnabled_nilUser_returnsDefaults(t *testing.T) {
	c := qt.New(t)
	svc := notifications.NewService(memory.NewSettingsRegistryFactory())

	// Defensive — a nil user should fall back to defaults and not
	// panic. The warranty worker passes a non-nil user; this guards
	// against future callers that may forget.
	c.Assert(svc.IsEnabled(context.Background(), nil, notifications.CategoryWarrantyExpiry, notifications.ChannelEmail), qt.IsTrue)
	c.Assert(svc.IsEnabled(context.Background(), nil, notifications.CategoryWarrantyExpiry, notifications.ChannelPush), qt.IsFalse)
}

// seedSettings writes a settings object for the user through a user-scoped
// registry, which is what puts the rows where the service will look for them.
func seedSettings(c *qt.C, factory registry.SettingsRegistryFactory, user *models.User, settings models.SettingsObject) {
	c.Helper()
	ctx := appctx.WithUser(context.Background(), user)
	reg, err := factory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(reg.Save(ctx, settings), qt.IsNil)
}

func notifUser(id, tenantID string) *models.User {
	return &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			EntityID: models.EntityID{ID: id},
		},
	}
}

// The resolution chain #1648 specifies, in the order that matters: the
// channel kill-switch is user-wide and a per-group toggle must not talk its
// way past it.
func TestIsEnabledForGroup_ResolutionChain(t *testing.T) {
	c := qt.New(t)
	factory := memory.NewSettingsRegistryFactory()
	prefs := memory.NewGroupNotificationPrefRegistry()
	svc := notifications.NewService(factory)
	svc.SetGroupPrefs(prefs)
	user := notifUser("u1", "t1")
	ctx := context.Background()

	// No override anywhere: the user-global default answers.
	c.Assert(svc.IsEnabledForGroup(ctx, user, "t1", "g1",
		notifications.CategoryWarrantyExpiry, notifications.ChannelEmail), qt.IsTrue)

	// A per-group row turns it off for this group only.
	_, err := prefs.Create(ctx, models.GroupNotificationPref{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "t1"},
		GroupID:             "g1",
		UserID:              "u1",
		Category:            string(notifications.CategoryWarrantyExpiry),
		Enabled:             false,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(svc.IsEnabledForGroup(ctx, user, "t1", "g1",
		notifications.CategoryWarrantyExpiry, notifications.ChannelEmail), qt.IsFalse)
	// A different group is untouched — that is the whole point of a
	// per-group override.
	c.Assert(svc.IsEnabledForGroup(ctx, user, "t1", "g2",
		notifications.CategoryWarrantyExpiry, notifications.ChannelEmail), qt.IsTrue)

	// A per-group row can also turn something ON that is off user-globally.
	off := false
	seedSettings(c, factory, user, models.SettingsObject{NotificationsPriceDrop: &off})
	c.Assert(svc.IsEnabledForGroup(ctx, user, "t1", "g1",
		notifications.CategoryPriceDrop, notifications.ChannelEmail), qt.IsFalse)
	_, err = prefs.Create(ctx, models.GroupNotificationPref{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "t1"},
		GroupID:             "g1",
		UserID:              "u1",
		Category:            string(notifications.CategoryPriceDrop),
		Enabled:             true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(svc.IsEnabledForGroup(ctx, user, "t1", "g1",
		notifications.CategoryPriceDrop, notifications.ChannelEmail), qt.IsTrue)
}

func TestIsEnabledForGroup_ChannelKillSwitchWins(t *testing.T) {
	c := qt.New(t)
	factory := memory.NewSettingsRegistryFactory()
	prefs := memory.NewGroupNotificationPrefRegistry()
	svc := notifications.NewService(factory)
	svc.SetGroupPrefs(prefs)
	user := notifUser("u1", "t1")
	ctx := context.Background()

	off := false
	seedSettings(c, factory, user, models.SettingsObject{NotificationsChannelEmail: &off})
	_, err := prefs.Create(ctx, models.GroupNotificationPref{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "t1"},
		GroupID:             "g1",
		UserID:              "u1",
		Category:            string(notifications.CategoryWarrantyExpiry),
		Enabled:             true,
	})
	c.Assert(err, qt.IsNil)

	// "Never email me" is a user-wide statement. A per-group opt-in for one
	// category must not talk its way past it.
	c.Assert(svc.IsEnabledForGroup(ctx, user, "t1", "g1",
		notifications.CategoryWarrantyExpiry, notifications.ChannelEmail), qt.IsFalse)
}

// Without the #1648 registry wired, the group form must answer exactly like
// the user-global one rather than failing or silencing everything.
func TestIsEnabledForGroup_DegradesWithoutGroupPrefs(t *testing.T) {
	c := qt.New(t)
	factory := memory.NewSettingsRegistryFactory()
	svc := notifications.NewService(factory)
	user := notifUser("u1", "t1")
	ctx := context.Background()

	c.Assert(svc.IsEnabledForGroup(ctx, user, "t1", "g1",
		notifications.CategoryWarrantyExpiry, notifications.ChannelEmail),
		qt.Equals,
		svc.IsEnabled(ctx, user, notifications.CategoryWarrantyExpiry, notifications.ChannelEmail))

	// An empty tenant or group is the same situation: nothing to look up.
	c.Assert(svc.IsEnabledForGroup(ctx, user, "", "",
		notifications.CategoryWarrantyExpiry, notifications.ChannelEmail), qt.IsTrue)
}

func TestIsEnabledForGroup_NilUser(t *testing.T) {
	c := qt.New(t)
	svc := notifications.NewService(memory.NewSettingsRegistryFactory())

	// Defaults, not a panic and not a blanket false: a missing recipient is
	// a caller bug, and swallowing the notification hides it.
	c.Assert(svc.IsEnabledForGroup(context.Background(), nil, "t1", "g1",
		notifications.CategoryWarrantyExpiry, notifications.ChannelEmail), qt.IsTrue)
	c.Assert(svc.IsEnabledForGroup(context.Background(), notifUser("", "t1"), "t1", "g1",
		notifications.CategoryWarrantyExpiry, notifications.ChannelPush), qt.IsFalse)
}

func TestLanguage(t *testing.T) {
	c := qt.New(t)
	factory := memory.NewSettingsRegistryFactory()
	svc := notifications.NewService(factory)
	user := notifUser("u1", "t1")

	// Unset: the renderer falls back to English, so "" is the answer rather
	// than a guess.
	c.Assert(svc.Language(context.Background(), user), qt.Equals, "")

	lang := "  cs  "
	seedSettings(c, factory, user, models.SettingsObject{AppearanceLanguage: &lang})
	// Trimmed: a stray space would miss the translation bundle.
	c.Assert(svc.Language(context.Background(), user), qt.Equals, "cs")
}

// The per-sweep cache has to answer identically to the service; its reason to
// exist is round-trips, not different behavior.
func TestCache_MatchesTheServiceAndReadsOnce(t *testing.T) {
	c := qt.New(t)
	factory := memory.NewSettingsRegistryFactory()
	prefs := memory.NewGroupNotificationPrefRegistry()
	svc := notifications.NewService(factory)
	svc.SetGroupPrefs(prefs)
	user := notifUser("u1", "t1")
	ctx := context.Background()

	off := false
	lang := "ru"
	seedSettings(c, factory, user, models.SettingsObject{
		NotificationsWarrantyExpiry: &off,
		AppearanceLanguage:          &lang,
	})
	_, err := prefs.Create(ctx, models.GroupNotificationPref{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "t1"},
		GroupID:             "g1",
		UserID:              "u1",
		Category:            string(notifications.CategoryPriceDrop),
		Enabled:             false,
	})
	c.Assert(err, qt.IsNil)

	cache := svc.NewCache()
	for range 3 {
		c.Assert(cache.IsEnabled(ctx, user, notifications.CategoryWarrantyExpiry, notifications.ChannelEmail),
			qt.Equals, svc.IsEnabled(ctx, user, notifications.CategoryWarrantyExpiry, notifications.ChannelEmail))
		c.Assert(cache.IsEnabledForGroup(ctx, user, "t1", "g1", notifications.CategoryPriceDrop, notifications.ChannelEmail),
			qt.Equals, svc.IsEnabledForGroup(ctx, user, "t1", "g1", notifications.CategoryPriceDrop, notifications.ChannelEmail))
		c.Assert(cache.Language(ctx, user), qt.Equals, "ru")
	}

	// A nil recipient takes the same route as the uncached form.
	c.Assert(cache.IsEnabled(ctx, nil, notifications.CategoryWarrantyExpiry, notifications.ChannelPush), qt.IsFalse)
	c.Assert(cache.Language(ctx, nil), qt.Equals, "")
	c.Assert(cache.IsEnabledForGroup(ctx, nil, "t1", "g1",
		notifications.CategoryWarrantyExpiry, notifications.ChannelEmail), qt.IsTrue)
}
