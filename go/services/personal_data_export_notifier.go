package services

import (
	"context"
	"log/slog"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"

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
//
// A user whose tenant is not active gets no email (tenantGate): they cannot
// sign in, and the download is refused until the tenant is active again. The
// export itself is kept and stays listed in Account settings.
type PersonalDataExportNotifier struct {
	email     EmailService
	prefs     *notifications.Service
	tenants   registry.TenantRegistry
	publicURL string
}

// NewPersonalDataExportNotifier returns a notifier sending through email,
// checking the user's tenant in tenants and linking Account settings under
// publicURL.
func NewPersonalDataExportNotifier(email EmailService, settings registry.SettingsRegistryFactory, tenants registry.TenantRegistry, publicURL string) *PersonalDataExportNotifier {
	n := &PersonalDataExportNotifier{email: email, tenants: tenants, publicURL: strings.TrimRight(publicURL, "/")}
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
	active, err := newTenantGate(n.tenants).active(ctx, user.TenantID)
	if err != nil {
		return errxtrace.Wrap("personal data export: check tenant", err)
	}
	if !active {
		slog.InfoContext(ctx, "Personal data export ready, no email: the user's tenant is not active",
			"user_id", user.ID, "tenant_id", user.TenantID, "export_id", export.ID)
		return nil
	}
	if n.prefs != nil {
		ctx = appctx.WithEmailLanguage(ctx, n.prefs.Language(ctx, user))
	}
	return n.email.SendPersonalDataExportReadyEmail(ctx, user.Email, user.Name, n.publicURL+AccountSettingsPath, export.ExpiresAt)
}
