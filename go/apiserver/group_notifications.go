package apiserver

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services/notifications"
)

// GroupNotifications mounts /g/{groupSlug}/notifications (issue #1648).
// The route lives on the group-scoped tree — group middleware already
// resolved (tenant_id, group_id) onto the context and the RegistrySet
// is user-aware; the handler scopes every read/write by the auth'd
// user, since per-group prefs are per-(user × group × category).
//
// Plumbing-only on purpose: the actual category catalogue lives in
// the notifications package; this surface just exposes the two FE
// toggles for the Notifications card. New toggles ship by adding a
// category constant + a key in `groupNotificationCategories`.
//
// Takes the FactorySet because the read path materialises a
// notifications.Service via SettingsRegistryFactory (the user-aware
// Set's SettingsRegistry can't be repurposed for a different user,
// but here we always read for the auth'd user, so a one-off Service
// per request is fine — handler-scoped, no caller fan-out).
func GroupNotifications(factorySet *registry.FactorySet) func(r chi.Router) {
	return func(r chi.Router) {
		r.Get("/", handleGetGroupNotifications(factorySet))
		r.Patch("/", handlePatchGroupNotifications(factorySet))
	}
}

// groupNotificationCategories is the catalogue the FE Notifications
// card consumes — keyed by the JSON field name, valued by the
// `notifications.Category` constant the BE persists. Add an entry
// here + an i18n key on the FE to surface a new toggle.
//
// `warranty_expiring_alerts` maps to CategoryWarrantyExpiry rather
// than a literal "warranty_expiring_alerts" string so the BE shares
// one category enum across user-global and per-group prefs.
var groupNotificationCategories = map[string]notifications.Category{
	"warranty_expiring_alerts": notifications.CategoryWarrantyExpiry,
	"weekly_digest":            notifications.CategoryWeeklyDigest,
}

// GroupNotificationsResponse + Request use json keys that match the
// FE's i18n keys (`warranty_expiring_alerts`, `weekly_digest`); the
// BE-side enum (`notifications.Category`) stays as a separate concept.
// Pointers on the patch request differentiate "key absent → don't
// touch this toggle" from "key present → upsert with this value".
// `null` is NOT distinguishable from absence with this representation
// (both decode to `nil *bool`); if a future "clear the override and
// fall through to user-global" feature ships, this DTO will need a
// presence-tracking type (e.g. json.RawMessage) — flagged for that
// follow-up.
type GroupNotificationsResponse struct {
	WarrantyExpiringAlerts bool `json:"warranty_expiring_alerts"`
	WeeklyDigest           bool `json:"weekly_digest"`
}

type GroupNotificationsPatchRequest struct {
	WarrantyExpiringAlerts *bool `json:"warranty_expiring_alerts,omitempty"`
	WeeklyDigest           *bool `json:"weekly_digest,omitempty"`
}

// handleGetGroupNotifications returns the effective on/off state for
// each toggle by composing the per-group override (if any) with the
// user-global pref (and falling back to the in-code default if neither
// is set).
// @Summary Get per-group notification preferences for the caller
// @Description Returns the effective on/off for each FE toggle, resolved per-group → user-global → in-code default (issue #1648 / #1537 item 2).
// @Tags groups
// @Produce json
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} GroupNotificationsResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal Server Error"
// @Router /g/{groupSlug}/notifications [get].
func handleGetGroupNotifications(factorySet *registry.FactorySet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := appctx.UserFromContext(ctx)
		if user == nil {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		regSet := RegistrySetFromContext(ctx)
		if regSet == nil {
			http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
			return
		}
		groupID, ok := resolveGroupIDForNotifications(ctx, regSet, user)
		if !ok {
			http.Error(w, "Group context missing", http.StatusBadRequest)
			return
		}

		// Per-request cache: every IsEnabledForGroup call fans out
		// into one SettingsRegistry.Get + one per-group prefs lookup,
		// so without memoisation a 2-toggle response does 4 round-
		// trips. The cache shares both materialisations across the
		// two toggle reads → 1 settings read + 1 group-prefs read.
		// New per request — the cache is intentionally throwaway.
		svc := notifications.NewService(factorySet.SettingsRegistryFactory)
		svc.SetGroupPrefs(factorySet.GroupNotificationPrefRegistry)
		cache := svc.NewCache()

		render.JSON(w, r, GroupNotificationsResponse{
			WarrantyExpiringAlerts: cache.IsEnabledForGroup(ctx, user, user.TenantID, groupID, notifications.CategoryWarrantyExpiry, notifications.ChannelEmail),
			WeeklyDigest:           cache.IsEnabledForGroup(ctx, user, user.TenantID, groupID, notifications.CategoryWeeklyDigest, notifications.ChannelEmail),
		})
	}
}

// handlePatchGroupNotifications upserts the per-group override for
// each toggle present in the request body. Missing keys are left
// untouched.
// @Summary Update per-group notification preferences for the caller
// @Description Upserts the per-group override for each toggle present in the body. Missing keys are left untouched (issue #1648 / #1537 item 2).
// @Tags groups
// @Accept json
// @Produce json
// @Param groupSlug path string true "Group slug"
// @Param data body GroupNotificationsPatchRequest true "Toggles to upsert"
// @Success 200 {object} GroupNotificationsResponse "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal Server Error"
// @Router /g/{groupSlug}/notifications [patch].
func handlePatchGroupNotifications(factorySet *registry.FactorySet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := appctx.UserFromContext(ctx)
		if user == nil {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		regSet := RegistrySetFromContext(ctx)
		if regSet == nil || factorySet.GroupNotificationPrefRegistry == nil {
			http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
			return
		}
		groupID, ok := resolveGroupIDForNotifications(ctx, regSet, user)
		if !ok {
			http.Error(w, "Group context missing", http.StatusBadRequest)
			return
		}

		var req GroupNotificationsPatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Apply each present toggle as an upsert. Doing it serially
		// keeps the surface tiny — the alternative (a single multi-
		// category upsert query) doesn't pay off until more toggles
		// ship, and the unique index on (tenant, group, user,
		// category) keeps concurrent flips safe anyway.
		if req.WarrantyExpiringAlerts != nil {
			if err := upsertGroupNotificationPref(ctx, factorySet.GroupNotificationPrefRegistry, user.TenantID, groupID, user.ID, groupNotificationCategories["warranty_expiring_alerts"], *req.WarrantyExpiringAlerts); err != nil {
				internalServerError(w, r, err)
				return
			}
		}
		if req.WeeklyDigest != nil {
			if err := upsertGroupNotificationPref(ctx, factorySet.GroupNotificationPrefRegistry, user.TenantID, groupID, user.ID, groupNotificationCategories["weekly_digest"], *req.WeeklyDigest); err != nil {
				internalServerError(w, r, err)
				return
			}
		}

		// Echo the post-write effective state — same shape as GET so
		// the FE can reuse its decoder for both calls. Cache shares
		// the settings + per-group prefs reads across both toggle
		// lookups (see the GET handler for the same trick).
		svc := notifications.NewService(factorySet.SettingsRegistryFactory)
		svc.SetGroupPrefs(factorySet.GroupNotificationPrefRegistry)
		cache := svc.NewCache()
		render.JSON(w, r, GroupNotificationsResponse{
			WarrantyExpiringAlerts: cache.IsEnabledForGroup(ctx, user, user.TenantID, groupID, notifications.CategoryWarrantyExpiry, notifications.ChannelEmail),
			WeeklyDigest:           cache.IsEnabledForGroup(ctx, user, user.TenantID, groupID, notifications.CategoryWeeklyDigest, notifications.ChannelEmail),
		})
	}
}

func upsertGroupNotificationPref(ctx context.Context, reg registry.GroupNotificationPrefRegistry, tenantID, groupID, userID string, category notifications.Category, enabled bool) error {
	_, err := reg.Upsert(ctx, models.GroupNotificationPref{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
		},
		GroupID:  groupID,
		UserID:   userID,
		Category: string(category),
		Enabled:  enabled,
	})
	return err
}

// resolveGroupIDForNotifications resolves the active group's database
// ID. The /g/{groupSlug}/... middleware chain already loaded the
// LocationGroup onto the context (group-scoped routes carry it
// alongside tenant + user for RLS); we just read it back.
func resolveGroupIDForNotifications(ctx context.Context, _ *registry.Set, _ *models.User) (string, bool) {
	groupID := appctx.GroupIDFromContext(ctx)
	return groupID, groupID != ""
}
