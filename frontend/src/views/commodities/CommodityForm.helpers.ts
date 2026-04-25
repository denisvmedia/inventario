/**
 * Helpers shared by `CommodityCreateView` and `CommodityEditView`,
 * extracted from the legacy `CommodityForm.vue` during the Phase 4
 * design-system migration (Epic #1324, issue #1329).
 *
 * Pure functions — no Vue reactivity — so each view can compose them
 * with its own `useForm` instance.
 */
import type { CommodityFormInput } from './CommodityForm.schema'

export interface ApiResource<T = Record<string, unknown>> {
  id: string
  attributes: T
  relationships?: Record<string, { data?: { id: string } }>
}

export interface AreaAttributes {
  name: string
  location_id: string
}

export interface LocationAttributes {
  name: string
}

export interface CurrencyOption {
  code: string
  label: string
}

export interface AreaGroup {
  label: string
  code: string
  items: Array<{ id: string; attributes: { name: string } }>
}

export function buildCurrencyOptions(codes: string[]): CurrencyOption[] {
  const display = new Intl.DisplayNames(['en'], { type: 'currency' })
  return codes.map((code) => {
    let label = code
    try {
      label = display.of(code) ?? code
    } catch {
      // Intl.DisplayNames throws on malformed currency codes; fall
      // back to the bare code so the dropdown still renders.
    }
    return { code, label: `${label} (${code})` }
  })
}

export function buildGroupedAreas(
  areas: ApiResource<AreaAttributes>[],
  locations: ApiResource<LocationAttributes>[],
): AreaGroup[] {
  const groups: Record<string, AreaGroup> = {}
  for (const location of locations) {
    groups[location.id] = {
      label: location.attributes.name,
      code: location.id,
      items: [],
    }
  }
  for (const area of areas) {
    const group = groups[area.attributes.location_id]
    if (group) {
      group.items.push({ id: area.id, attributes: { name: area.attributes.name } })
    }
  }
  return Object.values(groups).filter((g) => g.items.length > 0)
}

export function buildCommodityAttributes(
  values: CommodityFormInput,
): Record<string, unknown> {
  return {
    name: values.name.trim(),
    short_name: values.shortName.trim(),
    type: values.type,
    area_id: values.areaId,
    count: values.count,
    original_price: values.originalPrice,
    original_price_currency: values.originalPriceCurrency,
    converted_original_price: values.convertedOriginalPrice,
    current_price: values.currentPrice,
    serial_number: values.serialNumber || null,
    extra_serial_numbers: values.extraSerialNumbers.length > 0 ? values.extraSerialNumbers : null,
    part_numbers: values.partNumbers.length > 0 ? values.partNumbers : null,
    tags: values.tags.length > 0 ? values.tags : null,
    status: values.status,
    purchase_date: values.purchaseDate,
    urls: values.urls.length > 0 ? values.urls : null,
    comments: values.comments || null,
    draft: values.draft,
  }
}

const SNAKE_TO_CAMEL_FIELDS: Record<string, keyof CommodityFormInput> = {
  name: 'name',
  short_name: 'shortName',
  type: 'type',
  area_id: 'areaId',
  count: 'count',
  original_price: 'originalPrice',
  original_price_currency: 'originalPriceCurrency',
  converted_original_price: 'convertedOriginalPrice',
  current_price: 'currentPrice',
  serial_number: 'serialNumber',
  status: 'status',
  purchase_date: 'purchaseDate',
  comments: 'comments',
}

/**
 * Maps a JSON:API error response onto a vee-validate-compatible
 * `setErrors` object (camelCase keys keyed by `CommodityFormInput`).
 *
 * The legacy backend ships per-field errors under
 * `errors[0].error.error.data.attributes`; that path is preserved so
 * the migration is invisible to the API.
 */
export function extractApiFieldErrors(err: unknown): Partial<Record<keyof CommodityFormInput, string>> {
  const apiErrors =
    (err as {
      response?: {
        data?: {
          errors?: Array<{
            error?: { error?: { data?: { attributes?: Record<string, string> } } }
          }>
        }
      }
    }).response?.data?.errors?.[0]?.error?.error?.data?.attributes ?? {}

  const fieldErrors: Partial<Record<keyof CommodityFormInput, string>> = {}
  for (const [snakeKey, message] of Object.entries(apiErrors)) {
    const camelKey = SNAKE_TO_CAMEL_FIELDS[snakeKey]
    if (camelKey && message) {
      fieldErrors[camelKey] = message
    }
  }
  return fieldErrors
}
