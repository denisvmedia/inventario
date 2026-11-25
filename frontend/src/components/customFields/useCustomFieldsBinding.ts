import type { FieldError, UseFormReturn } from "react-hook-form"

import type { CustomField } from "@/features/customFields/api"
import { withTemplate, type CustomFieldFormValue } from "@/features/customFields/values"
import type { CommodityFormInput } from "@/features/commodities/schemas"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { useItemTypeLookup } from "@/features/itemTypes/hooks"

// The error of one field's entry: the schema puts its message on .value,
// a 422 from the server on the entry itself (custom_fields.<id>).
type EntryError = (FieldError & { value?: FieldError }) | undefined

// useCustomFieldsBinding wires CustomFieldsEditor to an item form's
// custom_fields value, and applies an item type's template when the form
// picks the type.
export function useCustomFieldsBinding(
  form: UseFormReturn<CommodityFormInput>,
  fields: readonly CustomField[]
) {
  const itemTypes = useItemTypeLookup()
  const groupCurrency = useOptionalCurrentGroup()?.currentGroup?.group_currency ?? "USD"
  const values = form.watch("custom_fields")
  const entries = form.formState.errors.custom_fields as
    | Record<string, EntryError>
    | undefined
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

  // Shows the fields the template of the type with slug names, filled in
  // with their defaults where the form has no value. Called when the user
  // picks a type, not when a form opens on an item, so an untouched item
  // gains no values.
  function applyTemplate(slug: string | undefined) {
    const id = itemTypes.get(slug)?.type?.id
    if (!id) return
    const next = withTemplate(fields, id, form.getValues("custom_fields"), groupCurrency)
    if (next) form.setValue("custom_fields", next, { shouldDirty: true })
  }

  return { values, errors, onChange, applyTemplate }
}
