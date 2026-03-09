import { defineStore } from 'pinia'
import { ref } from 'vue'
import settingsService from '@/services/settingsService'
import { CURRENCY_USD } from '@/constants/currencies'
import { getErrorMessage } from '@/utils/errorUtils'

function extractSettingsErrorMessage(err: any, fallbackMessage: string): string {
  const responseData = err?.response?.data

  if (typeof responseData === 'string' && responseData.trim()) {
    return responseData.trim()
  }

  if (typeof responseData?.message === 'string' && responseData.message.trim()) {
    return responseData.message.trim()
  }

  return getErrorMessage(err, undefined, fallbackMessage)
}

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

  async function updateMainCurrency(currency: string, exchangeRate?: string) {
    isLoading.value = true
    error.value = null

    try {
      await settingsService.updateMainCurrency(currency, exchangeRate)
      mainCurrency.value = currency
    } catch (err: any) {
      console.error('Failed to update main currency:', err)

      error.value = extractSettingsErrorMessage(err, 'Failed to update main currency')

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
