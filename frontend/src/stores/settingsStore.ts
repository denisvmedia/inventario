import { defineStore } from 'pinia'
import { ref } from 'vue'
import settingsService from '@/services/settingsService'
import { CURRENCY_USD } from '@/constants/currencies'

export const useSettingsStore = defineStore('settings', () => {
  // State
  const mainCurrency = ref<string>(CURRENCY_USD) // Default to CZK
  const isLoading = ref<boolean>(false)
  const error = ref<string | null>(null)

  // Actions
  async function fetchMainCurrency() {
    isLoading.value = true
    error.value = null

    try {
      const currency = await settingsService.getMainCurrency()
      if (currency) {
        mainCurrency.value = currency
      }
    } catch (err) {
      console.error('Failed to load main currency from settings:', err)
      error.value = 'Failed to load main currency'
      // Continue with default currency
    } finally {
      isLoading.value = false
    }
  }

  async function updateMainCurrency(currency: string) {
    isLoading.value = true
    error.value = null

    try {
      await settingsService.updateMainCurrency(currency)
      mainCurrency.value = currency
    } catch (err: any) {
      console.error('Failed to update main currency:', err)

      // Check if this is the specific error about main currency already being set
      if (err.response && err.response.status === 422 &&
          err.response.data && err.response.data.includes('main currency already set')) {
        error.value = 'Main currency has already been set and cannot be changed'
      } else {
        error.value = 'Failed to update main currency'
      }

      throw err
    } finally {
      isLoading.value = false
    }
  }

  return {
    // State
    mainCurrency,
    isLoading,
    error,

    // Actions
    fetchMainCurrency,
    updateMainCurrency
  }
})
