package notifications_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services/notifications"
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
	// Other categories still enabled (only the explicit row flips).
	c.Assert(svc.IsEnabled(context.Background(), user, notifications.CategoryWeeklyDigest, notifications.ChannelEmail), qt.IsTrue)
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
