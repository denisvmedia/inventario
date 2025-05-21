package models

type SettingsObject struct {
	MainCurrency      *string `configfield:"system.main_currency"`
	Theme             *string `configfield:"uiconfig.theme"`
	ShowDebugInfo     *bool   `configfield:"uiconfig.show_debug_info"`
	DefaultDateFormat *string `configfield:"uiconfig.default_date_format"`
}
