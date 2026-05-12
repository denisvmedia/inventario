// Package notifications wraps the per-user notification preferences
// stored on the settings table (one row per category × channel toggle,
// namespaced under `notifications.*`) and exposes a single helper —
// IsEnabled — that every non-transactional sender consults before
// queueing a message.
//
// Transactional categories (password reset, email verification) MUST
// NOT call IsEnabled. They are not opt-out-able: a password reset
// always succeeds regardless of preferences.
package notifications

import (
	"context"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// Category enumerates the opt-out-able notification kinds. Adding a new
// value here is safe at any time: the read path falls back to
// `categoryDefaults` for users that don't have a row yet, so no
// backfill / migration is required.
type Category string

const (
	CategoryWarrantyExpiry      Category = "warranty_expiry"
	CategoryMaintenanceReminder Category = "maintenance_reminder"
	CategoryWeeklyDigest        Category = "weekly_digest"
	CategoryPriceDrop           Category = "price_drop"
)

// Channel is the delivery medium. A user can globally silence a channel
// regardless of which categories they have enabled — useful for power
// users that disable push entirely while keeping email digests.
type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelPush  Channel = "push"
)

// categoryDefaults — the value used when the user has no explicit
// `notifications.<category>` row. All categories default to enabled so
// new users see notifications until they opt out.
var categoryDefaults = map[Category]bool{
	CategoryWarrantyExpiry:      true,
	CategoryMaintenanceReminder: true,
	CategoryWeeklyDigest:        true,
	CategoryPriceDrop:           true,
}

// channelDefaults — the value used when the user has no explicit
// `notifications.channel.<channel>` row. Email defaults on (the medium
// every user has registered for by signing up); push defaults off until
// the user explicitly grants browser/device permission.
var channelDefaults = map[Channel]bool{
	ChannelEmail: true,
	ChannelPush:  false,
}

// Service reads per-user notification preferences off the settings
// table via the SettingsRegistry factory. Construct one per process
// and share across senders.
type Service struct {
	factory registry.SettingsRegistryFactory
}

// NewService wires the factory used to materialise a user-scoped
// SettingsRegistry for each lookup. The factory itself is cheap to
// reuse — it's the underlying *SettingsRegistry that's per-user.
func NewService(factory registry.SettingsRegistryFactory) *Service {
	return &Service{factory: factory}
}

// IsEnabled reports whether the given category should be delivered to
// the given user via the given channel. It consults the user's
// per-category toggle AND the channel master switch — both must be on
// for the answer to be true. Missing rows fall back to the in-code
// defaults declared above.
//
// On registry / DB error we degrade to the defaults rather than
// silently suppress a notification the user expected by default: a
// transient outage shouldn't lose a warranty reminder.
func (s *Service) IsEnabled(ctx context.Context, user *models.User, category Category, channel Channel) bool {
	if user == nil || user.ID == "" {
		return categoryDefaults[category] && channelDefaults[channel]
	}

	// Inject the target user into context so the user-aware
	// SettingsRegistry's RLS filter resolves to *their* rows. We are
	// intentionally reading another user's prefs from a worker
	// goroutine that has no user of its own in context.
	userCtx := appctx.WithUser(ctx, user)
	reg, err := s.factory.CreateUserRegistry(userCtx)
	if err != nil {
		return categoryDefaults[category] && channelDefaults[channel]
	}
	settings, err := reg.Get(userCtx)
	if err != nil {
		return categoryDefaults[category] && channelDefaults[channel]
	}

	// Channel master switch: when explicitly false, every category on
	// that channel is suppressed irrespective of the per-category
	// toggle. When absent, falls back to the channel default.
	if !lookupChannel(settings, channel) {
		return false
	}
	return lookupCategory(settings, category)
}

func lookupCategory(s models.SettingsObject, category Category) bool {
	d := categoryDefaults[category]
	switch category {
	case CategoryWarrantyExpiry:
		return derefOr(s.NotificationsWarrantyExpiry, d)
	case CategoryMaintenanceReminder:
		return derefOr(s.NotificationsMaintenanceReminder, d)
	case CategoryWeeklyDigest:
		return derefOr(s.NotificationsWeeklyDigest, d)
	case CategoryPriceDrop:
		return derefOr(s.NotificationsPriceDrop, d)
	}
	return d
}

func lookupChannel(s models.SettingsObject, channel Channel) bool {
	d := channelDefaults[channel]
	switch channel {
	case ChannelEmail:
		return derefOr(s.NotificationsChannelEmail, d)
	case ChannelPush:
		return derefOr(s.NotificationsChannelPush, d)
	}
	return d
}

func derefOr(b *bool, fallback bool) bool {
	if b == nil {
		return fallback
	}
	return *b
}
