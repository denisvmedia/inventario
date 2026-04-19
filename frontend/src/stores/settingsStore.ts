import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { useGroupStore } from '@/stores/groupStore'
import { CURRENCY_USD } from '@/constants/currencies'

// settingsStore used to own `mainCurrency` as a user-scoped setting. In #1248
// main currency moved onto the location group (set once at group creation and
// immutable after — a currency-migration tool is tracked under #202), so the
// store now reads it from the active group via groupStore.
//
// The read-only `mainCurrency` + no-op `fetchMainCurrency` surface is kept
// so existing reactive consumers (commodity list, valuation displays, …)
// don't need a simultaneous call-site rewrite.

export const useSettingsStore = defineStore('settings', () => {
  const groupStore = useGroupStore()

  // isLoading/error are kept on this store so callers that render a
  // settings-scoped spinner or error banner keep working unchanged.
  const isLoading = ref<boolean>(false)
  const error = ref<string | null>(null)

  // mainCurrency is a computed shim over the active group's main_currency
  // so existing reactive consumers (templates, computed chains) keep
  // receiving updates when the user switches groups.
  const mainCurrency = computed<string>(() => groupStore.currentGroupMainCurrency || CURRENCY_USD)

  // fetchMainCurrency is kept for compatibility with callers that still
  // invoke it on startup. The group's currency is already loaded as part
  // of the group list, so there's nothing to do here beyond clearing the
  // error state.
  async function fetchMainCurrency(): Promise<void> {
    error.value = null
  }

  return {
    mainCurrency,
    isLoading,
    error,

    fetchMainCurrency,
  }
})
