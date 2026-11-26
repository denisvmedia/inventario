import type { FieldError, UseFormReturn } from "react-hook-form"

import type { CustomField } from "@/features/customFields/api"
import type { CustomFieldFormValue } from "@/features/customFields/values"
import type { CommodityFormInput } from "@/features/commodities/schemas"

// The error of one field's entry: the schema puts its message on .value,
// a 422 from the server on the entry itself (custom_fields.<id>).
type EntryError = (FieldError & { value?: FieldError }) | undefined

// useCustomFieldsBinding wires CustomFieldsEditor to an item form's
// custom_fields value.
export function useCustomFieldsBinding(
  form: UseFormReturn<CommodityFormInput>,
  fields: readonly CustomField[]
) {
  const values = form.watch("custom_fields")
  const entries = form.formState.errors.custom_fields as Record<string, EntryError> | undefined
  const errors: Record<string, string | undefined> = {}
  for (const f of fields) {
    const e = entries?.[f.id]
    errors[f.id] = e?.value?.message ?? e?.message
  }

  function onChange(id: string, value: CustomFieldFormValue | undefined) {
    const next = { ...(form.getValues("custom_fields") ?? {}) }
    if (value) next[id] = value
    else delete next[id]
    form.setValue("custom_fields", next, { shouldDirty: true })
    form.clearErrors(`custom_fields.${id}`)
  }

  return { values, errors, onChange }
}
