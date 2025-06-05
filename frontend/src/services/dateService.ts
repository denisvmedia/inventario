import settingsService from '@/services/settingsService'

// Default date format if none is set in settings
const DEFAULT_DATE_FORMAT = 'DD/MM/YYYY'

// Cache for date format to avoid multiple API calls
let cachedDateFormat: string | null = null

export async function getDateFormat(): Promise<string> {
  if (cachedDateFormat) {
    return cachedDateFormat
  }
  
  try {
    const format = await settingsService.getDefaultDateFormat()
    cachedDateFormat = format || DEFAULT_DATE_FORMAT
    return cachedDateFormat
  } catch (error) {
    console.warn('Failed to load date format from settings, using default:', error)
    cachedDateFormat = DEFAULT_DATE_FORMAT
    return cachedDateFormat
  }
}

export function formatDate(dateString: string, format?: string): string {
  if (!dateString) return ''
  
  const date = new Date(dateString)
  if (isNaN(date.getTime())) return dateString // Return original if invalid
  
  const formatToUse = format || DEFAULT_DATE_FORMAT
  
  // Simple format mapping - can be extended as needed
  const day = String(date.getDate()).padStart(2, '0')
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const year = date.getFullYear()
  
  switch (formatToUse.toUpperCase()) {
    case 'DD/MM/YYYY':
      return `${day}/${month}/${year}`
    case 'MM/DD/YYYY':
      return `${month}/${day}/${year}`
    case 'YYYY-MM-DD':
      return `${year}-${month}-${day}`
    case 'YYYY/MM/DD':
      return `${year}/${month}/${day}`
    default:
      // Default to DD/MM/YYYY
      return `${day}/${month}/${year}`
  }
}

export async function formatDateWithSettings(dateString: string): Promise<string> {
  if (!dateString) return ''
  
  const format = await getDateFormat()
  return formatDate(dateString, format)
}

// Clear cache when needed (e.g., when settings are updated)
export function clearDateFormatCache(): void {
  cachedDateFormat = null
}