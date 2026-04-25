import { z } from 'zod'

import {
  COMMODITY_STATUSES,
  COMMODITY_STATUS_IN_USE,
} from '@/constants/commodityStatuses'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { CURRENCY_CZK } from '@/constants/currencies'

/**
 * Zod schema for the Create / Edit commodity forms.
 *
 * Mirrors the validation that lived in `CommodityForm.vue` (max
 * length on `short_name`, comments cap, non-negative prices,
 * required fields). The "Purchase Date cannot be in the future" rule
 * is enforced via a `superRefine` so the message attaches to the
 * `purchaseDate` field exactly like before.
 *
 * Per devdocs/frontend/forms.md (`vee-validate` + `zod` is the only
 * form stack), the schema lives next to the views that consume it.
 */

const commodityTypeIds = COMMODITY_TYPES.map((t) => t.id) as [string, ...string[]]
const commodityStatusIds = COMMODITY_STATUSES.map((s) => s.id) as [string, ...string[]]

const today = (): string => new Date().toISOString().split('T')[0]

export const commodityFormSchema = z
  .object({
    name: z
      .string({ required_error: 'Name is required' })
      .min(1, 'Name is required'),
    shortName: z
      .string({ required_error: 'Short Name is required' })
      .min(1, 'Short Name is required')
      .max(20, 'Short Name must be at most 20 characters'),
    type: z
      .string({ required_error: 'Type is required' })
      .refine((v) => commodityTypeIds.includes(v), {
        message: 'Type is required',
      }),
    areaId: z
      .string({ required_error: 'Area is required' })
      .min(1, 'Area is required'),
    count: z.coerce
      .number({ invalid_type_error: 'Count must be a number' })
      .int('Count must be an integer')
      .min(1, 'Count must be at least 1'),
    originalPrice: z.coerce
      .number({ invalid_type_error: 'Original Price must be a number' })
      .min(0, 'Original Price cannot be negative'),
    originalPriceCurrency: z
      .string({ required_error: 'Original Price Currency is required' })
      .min(1, 'Original Price Currency is required'),
    convertedOriginalPrice: z.coerce
      .number({ invalid_type_error: 'Converted Original Price must be a number' })
      .min(0, 'Converted Original Price cannot be negative'),
    currentPrice: z.coerce
      .number({ invalid_type_error: 'Current Price must be a number' })
      .min(0, 'Current Price cannot be negative'),
    serialNumber: z.string(),
    extraSerialNumbers: z.array(z.string()),
    partNumbers: z.array(z.string()),
    tags: z.array(z.string()),
    status: z
      .string({ required_error: 'Status is required' })
      .refine((v) => commodityStatusIds.includes(v), {
        message: 'Status is required',
      }),
    purchaseDate: z
      .string({ required_error: 'Purchase Date is required' })
      .min(1, 'Purchase Date is required'),
    urls: z.array(z.string()),
    comments: z
      .string()
      .max(1000, 'Comments cannot exceed 1000 characters'),
    draft: z.boolean(),
  })
  .superRefine((value, ctx) => {
    if (value.purchaseDate && value.purchaseDate > today()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['purchaseDate'],
        message: 'Purchase Date cannot be in the future',
      })
    }
  })

export type CommodityFormInput = z.infer<typeof commodityFormSchema>

export function defaultCommodityFormValues(
  overrides: Partial<CommodityFormInput> = {},
): CommodityFormInput {
  return {
    name: '',
    shortName: '',
    type: COMMODITY_TYPES[0].id,
    areaId: '',
    count: 1,
    originalPrice: 0,
    originalPriceCurrency: CURRENCY_CZK,
    convertedOriginalPrice: 0,
    currentPrice: 0,
    serialNumber: '',
    extraSerialNumbers: [],
    partNumbers: [],
    tags: [],
    status: COMMODITY_STATUS_IN_USE,
    purchaseDate: today(),
    urls: [],
    comments: '',
    draft: false,
    ...overrides,
  }
}
