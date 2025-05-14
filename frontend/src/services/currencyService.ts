/**
 * Currency Service
 *
 * Provides access to currency-related functionality and data
 * Centralizes the store access to avoid repeated calls to useSettingsStore()
 */
// import { computed } from 'vue'
import { useSettingsStore } from '@/stores/settingsStore'

/**
 * Get the main currency from the settings store
 *
 * @returns The main currency code
 */
export function getMainCurrency(): string {
  return useSettingsStore().mainCurrency
}

/**
 * Format a price with the given currency
 *
 * @param price - The price to format
 * @param currency - The currency code to display (defaults to main currency)
 * @returns Formatted price string with currency
 */
export const formatPrice = (price: number, currency?: string): string => {
  if (isNaN(price)) return 'N/A'
  const currencyToUse = currency || getMainCurrency()
  return price.toFixed(2) + ' ' + currencyToUse
}

/**
 * Calculate price per unit for a commodity
 *
 * @param commodity - The commodity object
 * @returns The price per unit
 */
export const calculatePricePerUnit = (commodity: any): number => {
  const price = getDisplayPrice(commodity)
  if (isNaN(price)) return NaN

  const count = commodity.attributes.count || 1
  if (count === 0) return price

  // Calculate price per unit and round to 2 decimal places
  return price / count
}

/**
 * Get the display price for a commodity based on available price information
 *
 * @param commodity - The commodity object
 * @returns The price to display
 */
export const getDisplayPrice = (commodity: any): number => {
  const originalPrice = parseFloat(commodity.attributes.original_price) || 0
  const originalPriceCurrency = commodity.attributes.original_price_currency
  const originalPriceCurrencyIsMain = originalPriceCurrency === getMainCurrency()
  const convertedOriginalPrice = parseFloat(commodity.attributes.converted_original_price) || 0
  const currentPrice = parseFloat(commodity.attributes.current_price) || 0

  if (currentPrice > 0) {
    return currentPrice
  }

  if (originalPriceCurrencyIsMain && originalPrice > 0) {
    return originalPrice
  }

  if (convertedOriginalPrice > 0) {
    return convertedOriginalPrice
  }

  return NaN
}
