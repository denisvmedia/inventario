import api from './api'

const API_URL = '/api/v1/settings'

const settingsService = {
  getSettings() {
    return api.get(API_URL)
  },

  updateSettings(settings: any) {
    return api.put(API_URL, settings)
  },

  patchSetting(field: string, value: any) {
    // The backend expects a JSON value in the request body
    // Override the default JSON API content type for this specific endpoint
    return api.patch(`${API_URL}/${field}`, value, {
      headers: {
        'Content-Type': 'application/json'
      }
    })
  },

  // Specific settings methods
  getTheme() {
    return this.getSettings().then(response => {
      return response.data.Theme || null;
    });
  },

  updateTheme(theme: string) {
    return this.patchSetting('uiconfig.theme', theme);
  },

  getShowDebugInfo() {
    return this.getSettings().then(response => {
      return response.data.ShowDebugInfo || false;
    });
  },

  updateShowDebugInfo(show: boolean) {
    return this.patchSetting('uiconfig.show_debug_info', show);
  },

  getMainCurrency() {
    return this.getSettings().then(response => {
      return response.data.MainCurrency || null;
    });
  },

  updateMainCurrency(currency: string) {
    return this.patchSetting('system.main_currency', currency);
  },

  getDefaultDateFormat() {
    return this.getSettings().then(response => {
      return response.data.DefaultDateFormat || null;
    });
  },

  updateDefaultDateFormat(format: string) {
    return this.patchSetting('uiconfig.default_date_format', format);
  },

  getCurrencies() {
    return api.get('/api/v1/currencies')
  }
}

export default settingsService
