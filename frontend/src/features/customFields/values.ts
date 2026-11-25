// Conversions between an item's custom field values as the API sends them
// (see CustomFieldValues in ./api) and the strings the item form edits.
import type { CustomField, CustomFieldType, CustomFieldValues } from "./api"

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

// toFormValue reads one stored value into the form's text.
export function toFormValue(type: CustomFieldType, raw: unknown): CustomFieldFormValue {
  if (raw === null || raw === undefined) return { value: "", currency: "" }
  if (type === "money" && isRecord(raw)) {
    const amount = raw.amount
    return {
      value: amount === null || amount === undefined ? "" : String(amount),
      currency: typeof raw.currency === "string" ? raw.currency : "",
    }
  }
  if (type === "boolean") return { value: raw === true ? "true" : "false", currency: "" }
  return { value: String(raw), currency: "" }
}

// toFormValues reads the stored values of fields.
export function toFormValues(
  fields: readonly CustomField[],
  values: CustomFieldValues | undefined
): CustomFieldFormValues {
  const out: CustomFieldFormValues = {}
  for (const f of fields) {
    if (hasValue(values, f.id)) out[f.id] = toFormValue(f.type, values?.[f.id])
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

function sameFormValue(a: CustomFieldFormValue | undefined, b: CustomFieldFormValue | undefined) {
  const av = a?.value.trim() ?? ""
  const bv = b?.value.trim() ?? ""
  if (av !== bv) return false
  return av === "" || (a?.currency ?? "").trim() === (b?.currency ?? "").trim()
}

// toApiValues returns the values to send: only the fields whose form value
// differs from what was stored, so a value the form did not touch is left
// as it is (a link to an item in Trash, a field this client does not know
// yet). A cleared field is sent as null. Undefined when nothing changed.
export function toApiValues(
  fields: readonly CustomField[],
  initial: CustomFieldFormValues,
  current: CustomFieldFormValues
): CustomFieldValues | undefined {
  const out: CustomFieldValues = {}
  for (const f of fields) {
    if (sameFormValue(initial[f.id], current[f.id])) continue
    out[f.id] = toApiValue(f.type, current[f.id] ?? { value: "", currency: "" })
  }
  return Object.keys(out).length > 0 ? out : undefined
}

// The fields an item form shows: those the item has a value for, then those
// the member added in this form, in the group's order (by name).
export function shownFields(
  fields: readonly CustomField[],
  stored: CustomFieldFormValues,
  added: readonly string[]
): CustomField[] {
  return fields.filter((f) => f.id in stored || added.includes(f.id))
}

// validateFormValue checks what the server would refuse and returns an
// i18n key under commodities:customFields.errors, or undefined.
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
    case "url":
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
