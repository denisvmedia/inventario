/**
 * Utility functions for price formatting and calculations
 */

/**
 * Format a price with the given currency
 *
 * @param price - The price to format
 * @param currency - The currency code to display
 * @returns Formatted price string with currency
 */
export const formatPrice = (price: number, currency: string): string => {
  if (isNaN(price)) return 'N/A'
  return price.toFixed(2) + ' ' + currency
}

/**
 * Calculate price per unit for a commodity
 *
 * @param commodity - The commodity object
 * @param mainCurrency - The main currency code
 * @returns The price per unit
 */
export const calculatePricePerUnit = (commodity: any, mainCurrency: string): number => {
  const price = getDisplayPrice(commodity, mainCurrency)
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
 * @param mainCurrency - The main currency code
 * @returns The price to display
 */
export const getDisplayPrice = (commodity: any, mainCurrency: string): number => {
  const originalPrice = parseFloat(commodity.attributes.original_price) || 0
  const originalPriceCurrency = commodity.attributes.original_price_currency
  const originalPriceCurrencyIsMain = originalPriceCurrency === mainCurrency
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
