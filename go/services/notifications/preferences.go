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
// table via the SettingsRegistry factory. When a per-group preferences
// registry is provided (issue #1648), IsEnabledForGroup additionally
// consults the group override and falls back to the user-global value;
// the existing IsEnabled path remains user-global only. Construct one
// per process and share across senders.
type Service struct {
	factory    registry.SettingsRegistryFactory
	groupPrefs registry.GroupNotificationPrefRegistry // nullable: legacy callers without #1648
}

// NewService wires the factory used to materialise a user-scoped
// SettingsRegistry for each lookup. The factory itself is cheap to
// reuse — it's the underlying *SettingsRegistry that's per-user.
func NewService(factory registry.SettingsRegistryFactory) *Service {
	return &Service{factory: factory}
}

// SetGroupPrefs wires the per-group preferences registry that backs
// IsEnabledForGroup (issue #1648). Optional — when nil the service
// behaves exactly like the user-global-only #1373 surface and
// IsEnabledForGroup degrades to IsEnabled.
func (s *Service) SetGroupPrefs(prefs registry.GroupNotificationPrefRegistry) {
	s.groupPrefs = prefs
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

// IsEnabledForGroup mirrors IsEnabled but additionally consults the
// per-group override row from #1648. Resolution chain:
//
//  1. Channel master switch (user-global) → if off, return false.
//  2. Per-group row for (user, group, category) → if present, return
//     its `enabled` flag.
//  3. User-global per-category toggle → fall back to its value (or
//     the in-code default if no row is set).
//
// Reason for step 1: the channel kill-switch is a user-wide preference
// ("never push", "never email"); a per-group toggle for one category
// shouldn't override that. If the user wants warranty alerts in group
// A but disabled the email channel entirely, the email still doesn't
// get sent — which is what they asked for.
//
// When the service has no group-prefs registry wired (legacy embedding
// without #1648) or no per-group row exists, the answer matches
// IsEnabled and the FE Notifications card reads the user-global
// value as the effective state.
func (s *Service) IsEnabledForGroup(ctx context.Context, user *models.User, tenantID, groupID string, category Category, channel Channel) bool {
	if user == nil || user.ID == "" {
		return defaultFor(category, channel)
	}
	settings, ok := s.fetchSettings(ctx, user)
	if !ok {
		// User-global fetch failed → degrade to defaults. Don't try
		// the per-group lookup either: if the user has no settings
		// row reachable, the per-group override is undefined without
		// a baseline.
		return defaultFor(category, channel)
	}
	if !lookupChannel(settings, channel) {
		return false
	}
	if override, found := s.fetchGroupOverride(ctx, user, tenantID, groupID, category); found {
		return override
	}
	return lookupCategory(settings, category)
}

// fetchGroupOverride reads the per-group row (if any) for (user,
// group, category). Returns (enabled, true) when a row is present;
// (false, false) on miss / nil registry / error. A registry error
// returns false-not-found so callers fall back to the user-global
// pref rather than silently flipping the category off.
func (s *Service) fetchGroupOverride(ctx context.Context, user *models.User, tenantID, groupID string, category Category) (enabled, found bool) {
	if s.groupPrefs == nil || tenantID == "" || groupID == "" {
		return false, false
	}
	prefs, err := s.groupPrefs.ListByUserGroup(ctx, tenantID, groupID, user.ID)
	if err != nil {
		slog.Warn(
			"notifications: per-group prefs lookup failed, falling back to user-global",
			"tenant_id", tenantID,
			"group_id", groupID,
			"user_id", user.ID,
			"error", err,
		)
		return false, false
	}
	for _, p := range prefs {
		if Category(p.Category) == category {
			return p.Enabled, true
		}
	}
	return false, false
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

// Cache is a per-sweep wrapper around Service that memoises the
// per-user SettingsRegistry fetch. Use it from workers that fan out
// one (category, channel) lookup per recipient per row (e.g. the
// warranty reminder sweep): the worker calls Service.NewCache() at
// the start of each tick and discards the Cache when the sweep ends,
// so the next sweep observes the user's latest toggle flips.
//
// Concurrency: Cache is goroutine-SAFE (the underlying sync.Map
// serialises reads/writes) but NOT singleflight. Two concurrent
// lookups for the same not-yet-cached user_id may both trigger a
// SettingsRegistry.Get(); whichever Store lands last wins. The
// warranty worker iterates commodities sequentially in one goroutine
// so the Cache is exactly-once in practice — we deliberately skip
// the singleflight wiring (and the `golang.org/x/sync` dep) until a
// concurrent caller materialises. Either way the cache never
// corrupts state — duplicate fetches just waste a DB read.
//
// There's intentionally no TTL: the lifetime of a Cache equals the
// lifetime of the sweep that created it.
type Cache struct {
	svc *Service
	// entries: userID -> *cacheEntry. The pointer is always non-nil
	// once stored (success-or-failure is encoded in cacheEntry.ok).
	entries cacheMap
	// groupEntries: (userID,groupID) -> *groupCacheEntry. Stores the
	// per-group override rows for #1648. A separate map (not stacked
	// on cacheEntry) because the lifetimes differ: a single user can
	// be a member of many groups inside one sweep, but the sweep
	// itself only hits one (or zero) of them per recipient.
	groupEntries sync.Map
}

// cacheEntry holds the per-user payload + a flag for whether the
// underlying SettingsRegistry.Get() succeeded. `ok == false` means
// the lookup failed (registry init / DB error) — callers should fall
// back to defaults instead of trusting an empty SettingsObject.
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
// through the sweep-scoped cache. The first call per user_id hits the
// SettingsRegistry; every subsequent call returns from memory. See
// the Cache type comment for the concurrency caveat — under a fan-out
// of concurrent calls for the same uncached user, the registry may
// be hit a small number of extra times before the entry stabilises.
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

// IsEnabledForGroup is the per-sweep cached form of
// Service.IsEnabledForGroup. Same resolution chain: channel master
// switch → per-group row → user-global category. Both the user
// settings and the per-(user, group) override list are memoised in
// the same Cache so the worker hits each underlying store once per
// (user[, group]) per sweep.
func (c *Cache) IsEnabledForGroup(ctx context.Context, user *models.User, tenantID, groupID string, category Category, channel Channel) bool {
	if user == nil || user.ID == "" {
		return defaultFor(category, channel)
	}
	settings, ok := c.lookup(ctx, user)
	if !ok {
		return defaultFor(category, channel)
	}
	if !lookupChannel(settings, channel) {
		return false
	}
	if override, found := c.lookupGroupOverride(ctx, user, tenantID, groupID, category); found {
		return override
	}
	return lookupCategory(settings, category)
}

// groupCacheEntry holds the per-(user, group) override list + a flag
// for whether the underlying registry call succeeded. ok=false → fall
// through to user-global (mirrors Service.fetchGroupOverride).
type groupCacheEntry struct {
	byCategory map[Category]bool
	ok         bool
}

func (c *Cache) lookupGroupOverride(ctx context.Context, user *models.User, tenantID, groupID string, category Category) (enabled, found bool) {
	if c.svc.groupPrefs == nil || tenantID == "" || groupID == "" {
		return false, false
	}
	key := user.ID + "|" + tenantID + "|" + groupID
	if v, loaded := c.groupEntries.Load(key); loaded {
		if entry, typeOK := v.(*groupCacheEntry); typeOK {
			if !entry.ok {
				return false, false
			}
			val, found := entry.byCategory[category]
			return val, found
		}
	}
	prefs, err := c.svc.groupPrefs.ListByUserGroup(ctx, tenantID, groupID, user.ID)
	entry := &groupCacheEntry{ok: err == nil}
	if err == nil {
		entry.byCategory = make(map[Category]bool, len(prefs))
		for _, p := range prefs {
			entry.byCategory[Category(p.Category)] = p.Enabled
		}
	} else {
		slog.Warn(
			"notifications: per-group prefs cache lookup failed, falling back to user-global",
			"tenant_id", tenantID,
			"group_id", groupID,
			"user_id", user.ID,
			"error", err,
		)
	}
	c.groupEntries.Store(key, entry)
	if !entry.ok {
		return false, false
	}
	val, found := entry.byCategory[category]
	return val, found
}

func (c *Cache) lookup(ctx context.Context, user *models.User) (models.SettingsObject, bool) {
	if v, ok := c.entries.inner.Load(user.ID); ok {
		// Comma-ok type assertion: the map is private to this package
		// and only ever stores *cacheEntry, but a defensive check
		// keeps the linter happy AND guards against a future caller
		// stuffing the wrong type via reflection / generics.
		if entry, typeOK := v.(*cacheEntry); typeOK {
			return entry.settings, entry.ok
		}
	}
	settings, ok := c.svc.fetchSettings(ctx, user)
	c.entries.inner.Store(user.ID, &cacheEntry{settings: settings, ok: ok})
	return settings, ok
}
