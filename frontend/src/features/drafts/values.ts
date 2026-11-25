// Reading the values of a server draft (#2812) into the item form.
//
// A draft holds what a client stored, and API clients other than this form
// can store drafts too. The server checks the types of the keys the form
// knows, but a value the form cannot use must still not break the dialog:
// each known field that does not have the form's type is dropped, so the
// form's default takes its place. Numbers are accepted where the form keeps
// a number as text. Keys the form does not know are kept, so saving the
// draft from the form does not lose them.
import { z } from "zod"

import type { DraftValues } from "./api"

const text = z.string().optional().catch(undefined)
const numberText = z
  .union([z.string(), z.number().transform((n) => String(n))])
  .optional()
  .catch(undefined)
const textList = z
  .array(z.unknown())
  .transform((list) => list.filter((v): v is string => typeof v === "string"))
  .optional()
  .catch(undefined)
const flag = z.boolean().optional().catch(undefined)

const draftValuesSchema = z.looseObject({
  name: text,
  short_name: text,
  brand: text,
  model: text,
  type: text,
  area_id: text,
  seller_id: text,
  status: text,
  original_price_currency: text,
  serial_number: text,
  purchase_date: text,
  comments: text,
  warranty_expires_at: text,
  warranty_notes: text,
  count: numberText,
  original_price: numberText,
  converted_original_price: numberText,
  current_price: numberText,
  extra_serial_numbers: textList,
  part_numbers: textList,
  tags: textList,
  urls: textList,
  draft: flag,
})

// parseDraftValues returns the values of raw the form can use. It never
// throws: anything that is not an object gives no values.
export function parseDraftValues(raw: unknown): DraftValues {
  const parsed = draftValuesSchema.safeParse(raw)
  if (!parsed.success) return {}
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(parsed.data)) {
    if (value !== undefined) out[key] = value
  }
  return out as DraftValues
}
