import axios from 'axios'

const API_URL = '/api/v1/settings'

const settingsService = {
  getSettings() {
    return axios.get(API_URL, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  getSetting(id: string) {
    return axios.get(`${API_URL}/${id}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  createSetting(id: string, value: any) {
    const payload = {
      data: {
        type: 'settings',
        id: id,
        attributes: {
          value: value
        }
      }
    }

    return axios.post(API_URL, payload, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  updateSetting(id: string, value: any) {
    const payload = {
      data: {
        type: 'settings',
        id: id,
        attributes: {
          value: value
        }
      }
    }

    return axios.put(`${API_URL}/${id}`, payload, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  deleteSetting(id: string) {
    return axios.delete(`${API_URL}/${id}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  // Specific settings methods
  getUIConfig() {
    return this.getSetting('ui_config')
  },

  updateUIConfig(config: any) {
    return this.updateSetting('ui_config', config)
  },

  getSystemConfig() {
    return this.getSetting('system_config')
  },

  updateSystemConfig(config: any) {
    return this.updateSetting('system_config', config)
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
