package services

import (
	"context"
	"strings"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/notifications"
)

// AccountSettingsPath is the app path of Account settings, where a user
// requests and downloads their personal data export.
const AccountSettingsPath = "/settings?section=account"

// PersonalDataExportNotifier emails a user that the personal data export
// they requested is ready (#2177). The user asked for the email by asking
// for the export, so it is sent whatever their notification settings say,
// in the language of their appearance settings.
type PersonalDataExportNotifier struct {
	email     EmailService
	prefs     *notifications.Service
	publicURL string
}

// NewPersonalDataExportNotifier returns a notifier sending through email and
// linking Account settings under publicURL.
func NewPersonalDataExportNotifier(email EmailService, settings registry.SettingsRegistryFactory, publicURL string) *PersonalDataExportNotifier {
	n := &PersonalDataExportNotifier{email: email, publicURL: strings.TrimRight(publicURL, "/")}
	if settings != nil {
		n.prefs = notifications.NewService(settings)
	}
	return n
}

// PersonalDataExportReady sends the email.
func (n *PersonalDataExportNotifier) PersonalDataExportReady(ctx context.Context, user *models.User, export *models.PersonalDataExport) error {
	if n.email == nil || user == nil || export == nil {
		return nil
	}
	if n.prefs != nil {
		ctx = appctx.WithEmailLanguage(ctx, n.prefs.Language(ctx, user))
	}
	return n.email.SendPersonalDataExportReadyEmail(ctx, user.Email, user.Name, n.publicURL+AccountSettingsPath, export.ExpiresAt)
}
