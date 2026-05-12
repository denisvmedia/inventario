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
	"log/slog"
	"sync"

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
//
// Callers that iterate many recipients (e.g. the warranty reminder
// worker sweeping every commodity) should prefer Cache.IsEnabled,
// which hits the SettingsRegistry once per user_id instead of once
// per (user, commodity, threshold) tuple.
func (s *Service) IsEnabled(ctx context.Context, user *models.User, category Category, channel Channel) bool {
	if user == nil || user.ID == "" {
		return defaultFor(category, channel)
	}
	settings, ok := s.fetchSettings(ctx, user)
	if !ok {
		return defaultFor(category, channel)
	}
	return decideEnabled(settings, category, channel)
}

// fetchSettings materialises a user-scoped SettingsRegistry for the
// target user and returns their stored toggles. The second return is
// false on any error (missing user, registry init failure, DB error)
// so the caller can drop to defaults.
func (s *Service) fetchSettings(ctx context.Context, user *models.User) (models.SettingsObject, bool) {
	// Inject the target user into context so the user-aware
	// SettingsRegistry's RLS filter resolves to *their* rows. We are
	// intentionally reading another user's prefs from a worker
	// goroutine that has no user of its own in context.
	userCtx := appctx.WithUser(ctx, user)
	reg, err := s.factory.CreateUserRegistry(userCtx)
	if err != nil {
		return models.SettingsObject{}, false
	}
	settings, err := reg.Get(userCtx)
	if err != nil {
		return models.SettingsObject{}, false
	}
	return settings, true
}

// decideEnabled is the pure decision function: given a SettingsObject,
// the category and the channel, return true if a notification should
// be delivered. Channel master switch overrides per-category toggle.
func decideEnabled(settings models.SettingsObject, category Category, channel Channel) bool {
	if !lookupChannel(settings, channel) {
		return false
	}
	return lookupCategory(settings, category)
}

// defaultFor returns the in-code default decision for a (category,
// channel) pair when no settings could be read (nil user, DB error,
// registry init failure). Both halves use the explicit-`ok` lookup so
// unknown category/channel values surface a slog warning rather than
// silently flipping to the zero value `false`.
func defaultFor(category Category, channel Channel) bool {
	return categoryDefaultFor(category) && channelDefaultFor(channel)
}

func lookupCategory(s models.SettingsObject, category Category) bool {
	d := categoryDefaultFor(category)
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
	d := channelDefaultFor(channel)
	switch channel {
	case ChannelEmail:
		return derefOr(s.NotificationsChannelEmail, d)
	case ChannelPush:
		return derefOr(s.NotificationsChannelPush, d)
	}
	return d
}

// categoryDefaultFor returns the in-code default for `category`. An
// unknown value (new enum constant added without the defaults map
// being updated) logs and falls back to `true` — assume new
// categories are intended to be enabled by default; the slog line
// surfaces the developer bug.
func categoryDefaultFor(category Category) bool {
	if d, ok := categoryDefaults[category]; ok {
		return d
	}
	slog.Warn(
		"notifications: unknown category, defaulting to enabled",
		"category", string(category),
	)
	return true
}

// channelDefaultFor mirrors categoryDefaultFor for delivery channels.
// Unknown channels default to `false` (suppressed) — the safer side
// for a brand-new channel (e.g. SMS) so users don't get spammed
// before the channel surface ships properly.
func channelDefaultFor(channel Channel) bool {
	if d, ok := channelDefaults[channel]; ok {
		return d
	}
	slog.Warn(
		"notifications: unknown channel, defaulting to suppressed",
		"channel", string(channel),
	)
	return false
}

func derefOr(b *bool, fallback bool) bool {
	if b == nil {
		return fallback
	}
	return *b
}

// Cache is a per-sweep wrapper around Service that fetches each user's
// SettingsObject at most once and reuses the cached value across every
// IsEnabled call. Use it from workers that fan out one
// (category, channel) lookup per recipient per row (e.g. the warranty
// reminder sweep): the worker calls Service.NewCache() at the start of
// each tick and discards the Cache when the sweep ends, so the next
// sweep observes the user's latest toggle flips.
//
// Cache is concurrency-safe — entries are written via sync.Map so a
// fan-out across goroutines doesn't race. There's intentionally no
// TTL: the lifetime of a Cache equals the lifetime of the sweep that
// created it.
type Cache struct {
	svc *Service
	// entries: userID -> *cacheEntry (pointer-or-nil; nil = fetched
	// and the user has no rows, which means defaults apply — but we
	// still want to avoid re-fetching for that user during this
	// sweep).
	entries cacheMap
}

// cacheEntry holds the per-user payload + a flag for whether the
// fetch succeeded.
type cacheEntry struct {
	settings models.SettingsObject
	ok       bool
}

// cacheMap is a thin sync.Map wrapper to keep IsEnabled readable.
type cacheMap struct {
	inner sync.Map // map[string]*cacheEntry (userID -> entry)
}

// NewCache returns a fresh per-sweep Cache. The Service can be reused
// across sweeps; the Cache cannot — it accumulates per-sweep state.
func (s *Service) NewCache() *Cache {
	return &Cache{svc: s}
}

// IsEnabled mirrors Service.IsEnabled but reads each user's settings
// through the sweep-scoped cache. First call per user_id hits the DB;
// every subsequent call returns from memory.
func (c *Cache) IsEnabled(ctx context.Context, user *models.User, category Category, channel Channel) bool {
	if user == nil || user.ID == "" {
		return defaultFor(category, channel)
	}
	settings, ok := c.lookup(ctx, user)
	if !ok {
		return defaultFor(category, channel)
	}
	return decideEnabled(settings, category, channel)
}

func (c *Cache) lookup(ctx context.Context, user *models.User) (models.SettingsObject, bool) {
	if v, ok := c.entries.inner.Load(user.ID); ok {
		entry := v.(*cacheEntry)
		return entry.settings, entry.ok
	}
	settings, ok := c.svc.fetchSettings(ctx, user)
	c.entries.inner.Store(user.ID, &cacheEntry{settings: settings, ok: ok})
	return settings, ok
}
