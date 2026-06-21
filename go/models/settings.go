package models

import (
	"github.com/go-extras/go-kit/must"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/typekit"
)

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="settings" comment="Enable RLS for multi-tenant setting isolation"
//migrator:schema:rls:policy name="setting_isolation" table="settings" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures settings can only be accessed and modified by their tenant and user with required contexts"
//migrator:schema:rls:policy name="setting_background_worker_access" table="settings" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all settings for processing"

//migrator:schema:table name="settings"
type Setting struct {
	//migrator:embedded mode="inline"
	TenantUserAwareEntityID
	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `db:"name"`
	//migrator:schema:field name="value" type="JSONB" not_null="true"
	Value any `db:"value"`
}

// PostgreSQL-specific indexes for settings
type SettingIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore)
	//migrator:schema:index name="idx_settings_uuid" fields="uuid" unique="true" table="settings"
	_ int

	// Unique index for tenant + user + name combination (ensures one setting per user per name)
	//migrator:schema:index name="idx_settings_tenant_user_name" fields="tenant_id,user_id,name" unique="true" table="settings"
	_ int

	// Index for tenant-based queries
	//migrator:schema:index name="idx_settings_tenant_id" fields="tenant_id" table="settings"
	_ int

	// Index for user-based queries
	//migrator:schema:index name="idx_settings_user_id" fields="user_id" table="settings"
	_ int

	// GIN index for JSONB value field for complex queries
	//migrator:schema:index name="settings_value_gin_idx" fields="value" type="GIN" table="settings"
	_ int
}

type JSONBValue struct {
	Data any
}

// RegistrationMode controls how new user registrations are handled.
type RegistrationMode string

const (
	// RegistrationModeOpen allows anyone to register; account is activated after email verification.
	RegistrationModeOpen RegistrationMode = "open"
	// RegistrationModeApproval allows anyone to register but an admin must approve the account.
	RegistrationModeApproval RegistrationMode = "approval"
	// RegistrationModeClosed disables self-service registration entirely.
	RegistrationModeClosed RegistrationMode = "closed"
)

// Validate implements the validation.Validatable interface for RegistrationMode.
func (rm RegistrationMode) Validate() error {
	switch rm {
	case RegistrationModeOpen, RegistrationModeApproval, RegistrationModeClosed:
		return nil
	default:
		return validation.NewError("validation_invalid_registration_mode", "must be one of: open, approval, closed")
	}
}

type SettingName string

var _ = must.Must(typekit.StructToMap(&SettingsObject{}))

const (
	SettingNameUIConfigTheme             SettingName = "uiconfig.theme"
	SettingNameUIConfigShowDebugInfo     SettingName = "uiconfig.show_debug_info"
	SettingNameUIConfigDefaultDateFormat SettingName = "uiconfig.default_date_format"

	// Notification preferences. Missing rows = use defaults (defined in
	// `categoryDefaults` / `channelDefaults` inside
	// go/services/notifications/preferences.go). Adding a new category
	// never requires a backfill because the absence of a row is treated
	// as the in-code default.
	SettingNameNotificationsWarrantyExpiry      SettingName = "notifications.warranty_expiry"
	SettingNameNotificationsMaintenanceReminder SettingName = "notifications.maintenance_reminder"
	SettingNameNotificationsWeeklyDigest        SettingName = "notifications.weekly_digest"
	SettingNameNotificationsPriceDrop           SettingName = "notifications.price_drop"
	SettingNameNotificationsLoanReminder        SettingName = "notifications.loan_reminder"
	SettingNameNotificationsChannelEmail        SettingName = "notifications.channel.email"
	SettingNameNotificationsChannelPush         SettingName = "notifications.channel.push"

	// Per-user appearance preferences. `default_items_view` is consumed by
	// the commodities list page as the initial view mode (grid / list).
	// `preferred_display_currency` is a personal display-formatting hint;
	// it is NOT used to override the per-group commodity currency on
	// stored values (see deviations log in PR-A for the wiring scope).
	// `number_format_locale` is the BCP-47 tag (e.g. "cs-CZ") used by
	// the FE `Intl.*` formatters. Unset/empty falls back to a
	// browser → UI-language chain on the FE — see
	// frontend/src/lib/intl.ts. Decoupling this from the UI language
	// lets a user read a Czech-formatted price tag on an English UI.
	SettingNameAppearanceDefaultItemsView         SettingName = "appearance.default_items_view"
	SettingNameAppearancePreferredDisplayCurrency SettingName = "appearance.preferred_display_currency"
	SettingNameAppearanceNumberFormatLocale       SettingName = "appearance.number_format_locale"
	// `language` is the user's chosen UI language (short code: en/cs/ru).
	// Unlike `number_format_locale` (a formatting-only hint), this IS the UI
	// language and is the source of truth the backend reads to localize
	// transactional emails (#2090). Unset falls back to English.
	SettingNameAppearanceLanguage SettingName = "appearance.language"
)

// SettingsObject is the user-scoped key/value blob persisted to the
// `settings` table — one row per non-nil field. Nil pointers mean
// "not set" and the read path falls back to defaults defined in code.
type SettingsObject struct {
	Theme             *string `configfield:"uiconfig.theme"`
	ShowDebugInfo     *bool   `configfield:"uiconfig.show_debug_info"`
	DefaultDateFormat *string `configfield:"uiconfig.default_date_format"`

	// Notification category toggles. Each category controls a class of
	// outbound notifications (warranty expiry mailers, weekly digests,
	// etc.). Transactional senders — password reset, email verification —
	// never consult these and so cannot be opted out.
	NotificationsWarrantyExpiry      *bool `configfield:"notifications.warranty_expiry"`
	NotificationsMaintenanceReminder *bool `configfield:"notifications.maintenance_reminder"`
	NotificationsWeeklyDigest        *bool `configfield:"notifications.weekly_digest"`
	NotificationsPriceDrop           *bool `configfield:"notifications.price_drop"`
	NotificationsLoanReminder        *bool `configfield:"notifications.loan_reminder"`
	// Channel toggles act as a master switch per delivery channel: when
	// false, no category-level notification is delivered through that
	// channel regardless of the per-category toggle.
	NotificationsChannelEmail *bool `configfield:"notifications.channel.email"`
	NotificationsChannelPush  *bool `configfield:"notifications.channel.push"`

	// Per-user appearance preferences.
	AppearanceDefaultItemsView         *string `configfield:"appearance.default_items_view"`
	AppearancePreferredDisplayCurrency *string `configfield:"appearance.preferred_display_currency"`
	AppearanceNumberFormatLocale       *string `configfield:"appearance.number_format_locale"`
	// AppearanceLanguage is the user's UI language (en/cs/ru) and the source
	// of truth for localizing transactional emails (#2090).
	AppearanceLanguage *string `configfield:"appearance.language"`
}

func (s *SettingsObject) Set(field string, value any) error {
	return typekit.SetFieldByConfigfieldTag(s, field, value)
}

func (s *SettingsObject) Get(field string) (any, error) {
	return typekit.GetFieldByConfigfieldTag(s, field)
}

func (s *SettingsObject) ToMap() map[string]any {
	return must.Must(typekit.StructToMap(s))
}
