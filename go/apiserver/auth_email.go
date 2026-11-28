package apiserver

import (
	"context"
	"time"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/notifications"
)

const (
	// detachedAuthEmailTimeout bounds detached auth-email sends that preserve
	// request-scoped values across registration and password-reset flows.
	// TODO(Phase 3): revisit this timeout once the real SMTP transport is implemented.
	detachedAuthEmailTimeout = 30 * time.Second
)

// newAccountEmailPrefs returns the preferences service the account emails
// read the recipient's language from, or nil when no settings registry is
// wired (the emails then render in English).
func newAccountEmailPrefs(settings registry.SettingsRegistryFactory) *notifications.Service {
	if settings == nil {
		return nil
	}
	return notifications.NewService(settings)
}

// withRecipientLanguage adds the recipient's UI language from their settings
// to ctx, so the email queue renders the account email in it. It reads the
// settings, so call it inside the detached send, not on the request path: a
// lookup that only happens for an existing account would make forgot-password
// and magic-link responses slower for known addresses than for unknown ones.
func withRecipientLanguage(ctx context.Context, prefs *notifications.Service, user *models.User) context.Context {
	if prefs == nil || user == nil {
		return ctx
	}
	return appctx.WithEmailLanguage(ctx, prefs.Language(ctx, user))
}
