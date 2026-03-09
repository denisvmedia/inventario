import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import {
  EXCHANGE_RATE_POSITIVE_MESSAGE,
  MAIN_CURRENCY_REQUIRED_MESSAGE,
  mapMainCurrencyMigrationError,
  normalizeMainCurrencyExchangeRate,
  useMainCurrencyMigration,
  validateMainCurrencyMigration,
} from '../mainCurrencyMigration'

describe('mainCurrencyMigration', () => {
  it('normalizes exchange-rate input consistently', () => {
    expect(normalizeMainCurrencyExchangeRate(undefined)).toBe('')
    expect(normalizeMainCurrencyExchangeRate(' 0.95 ')).toBe('0.95')
    expect(normalizeMainCurrencyExchangeRate(1.25)).toBe('1.25')
  })

  it('validates required main currency and positive exchange rate', () => {
    expect(validateMainCurrencyMigration('', '0')).toEqual({
      main_currency: MAIN_CURRENCY_REQUIRED_MESSAGE,
      exchange_rate: EXCHANGE_RATE_POSITIVE_MESSAGE,
    })
  })

  it('maps exchange-rate backend errors inline', () => {
    expect(mapMainCurrencyMigrationError('Exchange rate must be greater than zero', 'fallback')).toEqual({
      field: 'exchange_rate',
      message: 'Exchange rate must be greater than zero',
    })
  })

  it('submits, normalizes, and applies success side effects once', async () => {
    const settingsStore = {
      updateMainCurrency: vi.fn().mockResolvedValue(undefined),
      error: null,
    }
    const systemConfig = ref({ main_currency: 'EUR' })
    const isMainCurrencySet = ref(true)
    const originalMainCurrency = ref('USD')
    const exchangeRate = ref<string | number>(' 0.95 ')
    const formErrors = ref({
      main_currency: '',
      exchange_rate: '',
    })
    const error = ref<string | null>(null)
    const isSubmitting = ref(false)
    const onSuccess = vi.fn().mockResolvedValue(undefined)

    const { save } = useMainCurrencyMigration({
      settingsStore,
      systemConfig,
      isMainCurrencySet,
      originalMainCurrency,
      exchangeRate,
      formErrors,
      error,
      isSubmitting,
      fallbackErrorMessage: 'Failed to save System config',
      onUnchanged: vi.fn(),
      onSuccess,
    })

    await save()

    expect(settingsStore.updateMainCurrency).toHaveBeenCalledWith('EUR', '0.95')
    expect(originalMainCurrency.value).toBe('EUR')
    expect(exchangeRate.value).toBe('')
    expect(onSuccess).toHaveBeenCalledTimes(1)
    expect(isSubmitting.value).toBe(false)
  })
})