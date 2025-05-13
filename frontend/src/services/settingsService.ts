import axios from 'axios'

const API_URL = '/api/v1/settings'

const settingsService = {
  getSettings() {
    return axios.get(API_URL, {
      headers: {
        'Accept': 'application/json'
      }
    })
  },

  updateSettings(settings: any) {
    return axios.put(API_URL, settings, {
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json'
      }
    })
  },

  patchSetting(field: string, value: any) {
    return axios.patch(`${API_URL}/${field}`, value, {
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json'
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
    return axios.get('/api/v1/currencies', {
      headers: {
        'Accept': 'application/json'
      }
    })
  }
}

export default settingsService
