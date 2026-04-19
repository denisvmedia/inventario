import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { useGroupStore } from '@/stores/groupStore'
import { CURRENCY_USD } from '@/constants/currencies'
import { getErrorMessage } from '@/utils/errorUtils'

// settingsStore used to own `mainCurrency` as a user-scoped setting. In #1248
// main currency moved onto the location group, so the store now reads it from
// the active group (via groupStore) and writes updates back through the
// group-scoped API. The existing API surface — `mainCurrency`,
// `fetchMainCurrency`, `updateMainCurrency` — is preserved so the many call
// sites that consume it (commodity list, commodity detail, valuation display)
// don't need to be rewritten in the same PR.

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
  const groupStore = useGroupStore()

  // isLoading/error are kept on this store (not the group store) so callers
  // that show a settings-scoped spinner or error banner keep working unchanged.
  const isLoading = ref<boolean>(false)
  const error = ref<string | null>(null)

  // mainCurrency is a computed shim over the active group's main_currency so
  // existing reactive consumers (templates, computed chains) keep receiving
  // updates when the user switches groups or the currency is changed.
  const mainCurrency = computed<string>(() => groupStore.currentGroupMainCurrency || CURRENCY_USD)

  // fetchMainCurrency is kept for compatibility with callers that still invoke
  // it on startup. The group's currency is already loaded as part of the group
  // list, so there is nothing to do here beyond clearing the error state.
  async function fetchMainCurrency(): Promise<void> {
    error.value = null
  }

  async function updateMainCurrency(currency: string, exchangeRate?: string): Promise<void> {
    isLoading.value = true
    error.value = null

    try {
      await groupStore.updateCurrentGroupMainCurrency(currency, exchangeRate)
    } catch (err: any) {
      console.error('Failed to update main currency:', err)

      error.value = extractSettingsErrorMessage(err, 'Failed to update main currency')

      throw err
    } finally {
      isLoading.value = false
    }
  }

  return {
    // State / computed
    mainCurrency,
    isLoading,
    error,

    // Actions
    fetchMainCurrency,
    updateMainCurrency,
  }
})
