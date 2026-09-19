package services

import (
	"context"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services/notifications"
)

// withReminderLanguage wraps ctx with the recipient's chosen UI language
// (their appearance.language setting, read through the per-sweep prefs cache
// so no extra DB round-trip is incurred when it follows an IsEnabled gate)
// so the async email renderer localizes the reminder. A nil cache or user
// leaves ctx unchanged, and an unset language resolves to English at render
// time. #2090
func withReminderLanguage(ctx context.Context, prefsCache *notifications.Cache, user *models.User) context.Context {
	if prefsCache == nil || user == nil {
		return ctx
	}
	return appctx.WithEmailLanguage(ctx, prefsCache.Language(ctx, user))
}
