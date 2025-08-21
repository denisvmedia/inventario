import systemService from './systemService'

/**
 * Service to check if settings are properly configured
 */
const settingsCheckService = {
  /**
   * Check if settings exist and are properly configured
   * @returns Promise<boolean> True if settings exist, false otherwise
   */
  async hasSettings(): Promise<boolean> {
    try {
      // Use the public system endpoint instead of protected settings endpoint
      const response = await systemService.getSystemInfo()
      const settings = response.data.settings

      // Check if essential settings are defined
      // We consider settings to exist if at least MainCurrency is set
      return !!settings.MainCurrency
    } catch (error) {
      console.error('Error checking settings:', error)
      return false
    }
  },

  /**
   * Get default settings object
   * @returns Default settings object
   */
  getDefaultSettings() {
    return {
      MainCurrency: 'USD',
      Theme: 'light',
      ShowDebugInfo: false,
      DefaultDateFormat: 'YYYY-MM-DD'
    }
  }
}

export default settingsCheckService
