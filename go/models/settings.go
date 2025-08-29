package models

import (
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/internal/typekit"
)

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="settings" comment="Enable RLS for multi-tenant setting isolation"
//migrator:schema:rls:policy name="setting_isolation" table="settings" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures settings can only be accessed and modified by their tenant and user with required contexts"
//migrator:schema:rls:policy name="setting_background_worker_access" table="settings" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all settings for processing"

//migrator:schema:table name="settings"
type Setting struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID
	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `db:"name"`
	//migrator:schema:field name="value" type="JSONB" not_null="true"
	Value any `db:"value"`
}

// PostgreSQL-specific indexes for settings
type SettingIndexes struct {
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

type SettingName string

var _ = must.Must(typekit.StructToMap(&SettingsObject{}))

const (
	SettingNameSystemMainCurrency        SettingName = "system.main_currency"
	SettingNameUIConfigTheme             SettingName = "uiconfig.theme"
	SettingNameUIConfigShowDebugInfo     SettingName = "uiconfig.show_debug_info"
	SettingNameUIConfigDefaultDateFormat SettingName = "uiconfig.default_date_format"
)

type SettingsObject struct {
	MainCurrency      *string `configfield:"system.main_currency"`
	Theme             *string `configfield:"uiconfig.theme"`
	ShowDebugInfo     *bool   `configfield:"uiconfig.show_debug_info"`
	DefaultDateFormat *string `configfield:"uiconfig.default_date_format"`
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
