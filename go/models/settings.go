package models

import (
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/internal/typekit"
)

type Setting struct {
	Name  string `db:"name"`
	Value any    `db:"value"`
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
