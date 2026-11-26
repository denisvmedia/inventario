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

import { toFormValue, type CustomFieldFormValue } from "@/features/customFields/values"

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

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v)
}

function isScalar(v: unknown): v is string | number | boolean {
  return typeof v === "string" || typeof v === "number" || typeof v === "boolean"
}

// draftCustomFieldValue reads one custom field entry of a draft. The form
// saves {value, currency} as text; a draft saved through the API may hold
// the value as an item stores it (text, a number, yes/no, or money as
// {amount, currency}), which is read the way the edit form reads an item.
// Anything else gives undefined.
function draftCustomFieldValue(raw: unknown): CustomFieldFormValue | undefined {
  if (isRecord(raw) && "value" in raw) {
    if (!isScalar(raw.value)) return undefined
    return {
      value: String(raw.value),
      currency: typeof raw.currency === "string" ? raw.currency : "",
    }
  }
  if (isRecord(raw) && "amount" in raw) return toFormValue(raw)
  return isScalar(raw) ? toFormValue(raw) : undefined
}

// The custom field values (#2803), by field id. An entry the form cannot
// read is dropped, so the field opens empty.
const customFields = z
  .record(z.string(), z.unknown())
  .transform((entries) => {
    const out: Record<string, CustomFieldFormValue> = {}
    for (const [id, raw] of Object.entries(entries)) {
      const v = draftCustomFieldValue(raw)
      if (v) out[id] = v
    }
    return out
  })
  .optional()
  .catch(undefined)

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
  custom_fields: customFields,
  seller_name: text,
})

// draftSellerName is the seller a draft names without picking one of the
// group's sellers: batch intake (#2810) stores a scanned seller the group
// does not have under "seller_name", for the form to offer to create.
export function draftSellerName(values: DraftValues): string {
  const raw = (values as Record<string, unknown>).seller_name
  if (typeof raw !== "string" || values.seller_id) return ""
  return raw.trim()
}

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
