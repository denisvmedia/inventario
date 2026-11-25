// Conversions between an item's custom field values as the API sends them
// (see CustomFieldValues in ./api) and the strings the item form edits.
import {
  CUSTOM_FIELD_LONG_TEXT_MAX,
  CUSTOM_FIELD_TEXT_MAX,
  CUSTOM_FIELD_URL_MAX,
  type CustomField,
  type CustomFieldType,
  type CustomFieldValues,
} from "./api"

// The form keeps every value as text, like the other item inputs: a
// number as typed, yes/no as "true"/"false", a select value as the option
// id, a link as the item id, money as its amount with the currency next to
// it. "" is no value.
export interface CustomFieldFormValue {
  value: string
  // The currency of a money value; empty for every other type.
  currency: string
}

export type CustomFieldFormValues = Record<string, CustomFieldFormValue>

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v)
}

// hasValue reports whether raw is a stored value: anything but a missing
// key and null. Zero, false and "" are values.
export function hasValue(values: CustomFieldValues | undefined, id: string): boolean {
  return values !== undefined && id in values && values[id] !== null
}

// toFormValue reads one stored value into the form's text. The API form of
// each type tells the types apart where it matters (money is an object,
// yes/no a boolean), so the field's definition is not needed.
export function toFormValue(raw: unknown): CustomFieldFormValue {
  if (raw === null || raw === undefined) return { value: "", currency: "" }
  if (isRecord(raw)) {
    const amount = raw.amount
    return {
      value: amount === null || amount === undefined ? "" : String(amount),
      currency: typeof raw.currency === "string" ? raw.currency : "",
    }
  }
  if (typeof raw === "boolean") return { value: raw ? "true" : "false", currency: "" }
  return { value: String(raw), currency: "" }
}

// toFormValues reads an item's stored values. A value of a field this
// client does not know yet is read too; the form shows only the fields it
// knows, and never sends the others.
export function toFormValues(values: CustomFieldValues | undefined): CustomFieldFormValues {
  const out: CustomFieldFormValues = {}
  for (const [id, raw] of Object.entries(values ?? {})) {
    if (raw !== null && raw !== undefined) out[id] = toFormValue(raw)
  }
  return out
}

// toApiValue turns the form's text into the API value, null for no value.
// Text is trimmed; an empty text clears the value, since the form has no
// way to tell an empty text from none.
export function toApiValue(type: CustomFieldType, v: CustomFieldFormValue): unknown {
  const text = v.value.trim()
  if (text === "") return null
  switch (type) {
    case "number":
      return Number(text)
    case "money":
      return { amount: Number(text), currency: v.currency.trim().toUpperCase() }
    case "boolean":
      return text === "true"
    default:
      return text
  }
}

// customFieldsRequest returns the custom_fields of an item write: the
// value of every known field the form holds one for, and null for a known
// field that stored has a key for and the form no longer holds a value
// for. stored is the item the form started from; a key with null in it (a
// value cleared before the form opened, see the edit form's "All fields")
// is cleared again. Fields this client does not know are left out, so the
// server keeps their values; an unchanged value is not checked again
// there. Undefined when there is nothing to send.
export function customFieldsRequest(
  fields: readonly CustomField[],
  stored: CustomFieldValues | undefined,
  current: CustomFieldFormValues | undefined
): CustomFieldValues | undefined {
  const out: CustomFieldValues = {}
  for (const f of fields) {
    const v = current?.[f.id]
    if (v && v.value.trim() !== "") {
      out[f.id] = toApiValue(f.type, v)
    } else if (stored && f.id in stored) {
      out[f.id] = null
    }
  }
  return Object.keys(out).length > 0 ? out : undefined
}

// shownFields returns the fields an item form shows: those the form holds
// an entry for (a stored value, or a field the member added), in the
// group's order (by name).
export function shownFields(
  fields: readonly CustomField[],
  current: CustomFieldFormValues | undefined
): CustomField[] {
  return fields.filter((f) => current !== undefined && f.id in current)
}

// validateFormValue checks what the server would refuse and returns the
// last segment of an i18n key under customFields:errors, or undefined.
export function validateFormValue(field: CustomField, v: CustomFieldFormValue): string | undefined {
  const text = v.value.trim()
  if (text === "") return undefined
  switch (field.type) {
    case "number":
    case "money": {
      if (!/^-?\d+(\.\d+)?$/.test(text)) return "number"
      const [int, frac = ""] = text.replace("-", "").split(".")
      const intMax = field.type === "money" ? 13 : 15
      const fracMax = field.type === "money" ? 2 : 10
      if (int.length > intMax || frac.length > fracMax) return "numberTooLong"
      if (field.type === "money" && !/^[A-Za-z]{3}$/.test(v.currency.trim())) return "currency"
      return undefined
    }
    case "text":
      return Array.from(text).length > CUSTOM_FIELD_TEXT_MAX ? "tooLong" : undefined
    case "long_text":
      return Array.from(text).length > CUSTOM_FIELD_LONG_TEXT_MAX ? "tooLong" : undefined
    case "url":
      if (Array.from(text).length > CUSTOM_FIELD_URL_MAX) return "tooLong"
      try {
        const u = new URL(text)
        return u.protocol === "http:" || u.protocol === "https:" ? undefined : "url"
      } catch {
        return "url"
      }
    case "date":
      return /^\d{4}-\d{2}-\d{2}$/.test(text) ? undefined : "date"
    default:
      return undefined
  }
}
