// Data layer for the group's custom fields (#2803). Hooks live in `./hooks.ts`.
// Backed by `/g/{slug}/custom-fields` and `/g/{slug}/custom-fields/{id}`.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export const CUSTOM_FIELD_TYPES = [
  "text",
  "long_text",
  "number",
  "money",
  "date",
  "boolean",
  "select",
  "url",
  "item_link",
] as const

export type CustomFieldType = (typeof CUSTOM_FIELD_TYPES)[number]

export type CustomFieldOption = Schema<"jsonapi.CustomFieldOptionView">

// A template entry: the field is offered on the items of this item type,
// prefilled with defaultValue while the item has no value of its own.
export interface CustomFieldTemplate {
  item_type_id: string
  default_value?: unknown
}

// A custom field with the number of items, live and in Trash, that have a
// value of it. Its type never changes.
export type CustomField = Omit<Schema<"jsonapi.CustomFieldView">, "type" | "templates"> & {
  id: string
  type: CustomFieldType
  name: string
  options: CustomFieldOption[]
  templates: CustomFieldTemplate[]
}

// An item's values, by field id, in the API form of each type: a string
// for text, long text, URL, date (YYYY-MM-DD), select (the option id) and
// link (the item id); a decimal string for number; a boolean for yes/no;
// and { amount, currency } for money. A missing key leaves the stored value
// as it is on a write; null clears it.
export type CustomFieldValues = Record<string, unknown>

// Limits mirror go/models/custom_field.go and custom_field_value.go.
export const CUSTOM_FIELD_NAME_MAX = 60
export const CUSTOM_FIELD_OPTIONS_MAX = 100
export const CUSTOM_FIELD_OPTION_LABEL_MAX = 100
export const CUSTOM_FIELDS_MAX = 200
export const CUSTOM_FIELD_TEXT_MAX = 200
export const CUSTOM_FIELD_LONG_TEXT_MAX = 10000
export const CUSTOM_FIELD_URL_MAX = 2048

// JSON:API error codes of the custom field endpoints.
export const CUSTOM_FIELD_NAME_TAKEN = "custom_field.name_taken"
// A delete without delete_values of a field items have values for, or one
// whose expected_values is not the current count. The error's meta carries
// item_count and trashed_item_count.
export const CUSTOM_FIELD_IN_USE = "custom_field.in_use"
export const CUSTOM_FIELD_LIMIT = "custom_field.limit_reached"
// An item write with a value its field refuses; the field errors are under
// data.attributes.custom_fields.<field id>.
export const CUSTOM_FIELD_INVALID_VALUE = "custom_field.invalid_value"

export interface CustomFieldInput {
  name: string
  type: CustomFieldType
  // Options of a select field, in order. An entry with the id of an
  // existing option keeps it and its values; one without an id adds an
  // option; an option left out is removed with its values.
  options?: { id?: string; label: string }[]
  templates?: CustomFieldTemplate[]
}

interface CustomFieldResource {
  id?: string
  type?: string
  attributes?: Schema<"jsonapi.CustomFieldView">
}

interface CustomFieldEnvelope {
  data?: CustomFieldResource
}

interface CustomFieldsEnvelope {
  data?: CustomFieldResource[]
  meta?: { custom_fields?: number; limit?: number }
}

interface CustomFieldDeleteEnvelope {
  meta?: { deleted_values?: number }
}

function toCustomField(resource: CustomFieldResource | undefined, what: string): CustomField {
  if (!resource?.attributes || !resource.id) {
    throw new Error(`Malformed ${what} response: missing data`)
  }
  const a = resource.attributes
  return {
    ...a,
    id: resource.id,
    name: a.name ?? "",
    type: (a.type ?? "text") as CustomFieldType,
    options: a.options ?? [],
    templates: (a.templates ?? []).map((t) => ({
      item_type_id: t.item_type_id ?? "",
      default_value: t.default_value ?? undefined,
    })),
  }
}

// listCustomFields returns the group's fields by name.
export async function listCustomFields(signal?: AbortSignal): Promise<CustomField[]> {
  const body = await http.get<CustomFieldsEnvelope>("/custom-fields", { signal })
  return (body.data ?? []).map((row) => toCustomField(row, "GET /custom-fields"))
}

export async function createCustomField(input: CustomFieldInput): Promise<CustomField> {
  const body = await http.post<CustomFieldEnvelope>("/custom-fields", {
    data: { type: "custom_fields", attributes: input },
  })
  return toCustomField(body.data, "POST /custom-fields")
}

export async function updateCustomField(
  id: string,
  input: Partial<CustomFieldInput>
): Promise<CustomField> {
  const body = await http.patch<CustomFieldEnvelope>(`/custom-fields/${encodeURIComponent(id)}`, {
    data: { id, type: "custom_fields", attributes: input },
  })
  return toCustomField(body.data, `PATCH /custom-fields/${id}`)
}

// deleteCustomField deletes a field. Without deleteValues the server
// refuses a field that items have values for; with it the values go too,
// and expectedValues, the number of items the member confirmed to lose a
// value, makes the server refuse the delete when another number has one.
// Resolves with the number of values deleted.
export async function deleteCustomField(
  id: string,
  deleteValues = false,
  expectedValues?: number
): Promise<number> {
  const params = new URLSearchParams()
  if (deleteValues) params.set("delete_values", "true")
  if (deleteValues && expectedValues !== undefined) {
    params.set("expected_values", String(expectedValues))
  }
  const query = params.size > 0 ? `?${params.toString()}` : ""
  const body = await http.del<CustomFieldDeleteEnvelope | undefined>(
    `/custom-fields/${encodeURIComponent(id)}${query}`
  )
  return body?.meta?.deleted_values ?? 0
}
