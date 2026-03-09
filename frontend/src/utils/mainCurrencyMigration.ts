import type { Ref } from 'vue'

export interface MainCurrencyMigrationFormErrors {
  main_currency: string
  exchange_rate: string
}

interface MainCurrencyMigrationStore {
  updateMainCurrency(currency: string, exchangeRate?: string): Promise<void>
  error: string | null
}

interface UseMainCurrencyMigrationOptions<
  TSystemConfig extends { main_currency: string },
  TFormErrors extends MainCurrencyMigrationFormErrors,
> {
  settingsStore: MainCurrencyMigrationStore
  systemConfig: Ref<TSystemConfig>
  isMainCurrencySet: Ref<boolean>
  originalMainCurrency: Ref<string>
  exchangeRate: Ref<string | number>
  formErrors: Ref<TFormErrors>
  error: Ref<string | null>
  isSubmitting: Ref<boolean>
  fallbackErrorMessage: string
  onSuccess: () => void | Promise<void>
  onUnchanged: () => void | Promise<void>
  logError?: (err: unknown) => void
}

export const MAIN_CURRENCY_REQUIRED_MESSAGE = 'Main Currency is required'
export const EXCHANGE_RATE_POSITIVE_MESSAGE = 'Exchange rate must be a positive number'

export function normalizeMainCurrencyExchangeRate(exchangeRate: string | number | null | undefined): string {
  return exchangeRate == null ? '' : String(exchangeRate).trim()
}

export function validateMainCurrencyMigration(
  mainCurrency: string,
  exchangeRate: string | number | null | undefined,
): MainCurrencyMigrationFormErrors {
  const formErrors = {
    main_currency: '',
    exchange_rate: '',
  }

  if (!mainCurrency) {
    formErrors.main_currency = MAIN_CURRENCY_REQUIRED_MESSAGE
  }

  const normalizedExchangeRate = normalizeMainCurrencyExchangeRate(exchangeRate)
  if (normalizedExchangeRate) {
    const parsedExchangeRate = Number(normalizedExchangeRate)
    if (!Number.isFinite(parsedExchangeRate) || parsedExchangeRate <= 0) {
      formErrors.exchange_rate = EXCHANGE_RATE_POSITIVE_MESSAGE
    }
  }

  return formErrors
}

export function mapMainCurrencyMigrationError(errorMessage: string | null | undefined, fallbackErrorMessage: string): {
  field: 'exchange_rate' | 'global'
  message: string
} {
  const message = errorMessage || fallbackErrorMessage

  if (message.toLowerCase().includes('exchange rate')) {
    return {
      field: 'exchange_rate',
      message,
    }
  }

  return {
    field: 'global',
    message,
  }
}

function hasValidationErrors(formErrors: MainCurrencyMigrationFormErrors): boolean {
  return Boolean(formErrors.main_currency || formErrors.exchange_rate)
}

export function useMainCurrencyMigration<
  TSystemConfig extends { main_currency: string },
  TFormErrors extends MainCurrencyMigrationFormErrors,
>(options: UseMainCurrencyMigrationOptions<TSystemConfig, TFormErrors>) {
  const resetErrors = () => {
    options.formErrors.value.main_currency = ''
    options.formErrors.value.exchange_rate = ''
    options.error.value = null
  }

  const save = async () => {
    resetErrors()

    const validationErrors = validateMainCurrencyMigration(
      options.systemConfig.value.main_currency,
      options.exchangeRate.value,
    )

    options.formErrors.value.main_currency = validationErrors.main_currency
    options.formErrors.value.exchange_rate = validationErrors.exchange_rate

    if (hasValidationErrors(validationErrors)) {
      return
    }

    if (
      options.isMainCurrencySet.value
      && options.systemConfig.value.main_currency === options.originalMainCurrency.value
    ) {
      await options.onUnchanged()
      return
    }

    options.isSubmitting.value = true

    try {
      const normalizedExchangeRate = normalizeMainCurrencyExchangeRate(options.exchangeRate.value)

      await options.settingsStore.updateMainCurrency(
        options.systemConfig.value.main_currency,
        normalizedExchangeRate || undefined,
      )

      options.originalMainCurrency.value = options.systemConfig.value.main_currency
      options.exchangeRate.value = ''

      await options.onSuccess()
    } catch (err) {
      const mappedError = mapMainCurrencyMigrationError(
        options.settingsStore.error,
        options.fallbackErrorMessage,
      )

      if (mappedError.field === 'exchange_rate') {
        options.formErrors.value.exchange_rate = mappedError.message
      } else {
        options.error.value = mappedError.message
      }

      options.logError?.(err)
    } finally {
      options.isSubmitting.value = false
    }
  }

  return {
    save,
  }
}