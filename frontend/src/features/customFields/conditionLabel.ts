import type { TFunction } from "i18next"

import { formatDate } from "@/lib/intl"

import type { CustomField } from "./api"
import { parseCustomFieldCondition, type CustomFieldCondition } from "./conditions"

// The value of cond in words: option labels for a select, Yes / No, a
// formatted date. An option the field no longer has is named as deleted.
function conditionValue(t: TFunction, cond: CustomFieldCondition, field: CustomField): string {
  switch (field.type) {
    case "boolean":
      return cond.value === "true" || cond.value === "1"
        ? t("customFields:detail.yes")
        : t("customFields:detail.no")
    case "select":
      return cond.value
        .split(",")
        .map((id) => id.trim())
        .filter(Boolean)
        .map(
          (id) =>
            field.options.find((o) => o.id === id)?.label ?? t("customFields:filter.deletedOption")
        )
        .join(", ")
    case "date":
      return formatDate(cond.value, { style: "short" })
    case "item_link":
      return t("customFields:filter.anItem")
    default:
      return cond.value
  }
}

export interface ConditionLabel {
  label: string
  // The condition names a field the group no longer has.
  stale: boolean
}

// customFieldConditionLabel spells out one cf condition for a chip:
// "Plate contains “AB”", "Fuel is Diesel, Electric", "Bought at least 3".
// A condition on a deleted field reads "Deleted field" and is stale; fields
// is undefined while the group's fields are not known, and then the field
// is named generically without being called deleted.
export function customFieldConditionLabel(
  t: TFunction,
  raw: string,
  fields: readonly CustomField[] | undefined
): ConditionLabel {
  const cond = parseCustomFieldCondition(raw)
  if (!cond) return { label: raw, stale: false }
  const field = fields?.find((f) => f.id === cond.fieldId)
  if (!field) {
    return fields
      ? { label: t("customFields:filter.deletedField"), stale: true }
      : { label: t("customFields:filter.someField"), stale: false }
  }
  return {
    label: t(`customFields:filter.chip.${cond.op}`, {
      field: field.name,
      value: conditionValue(t, cond, field),
    }),
    stale: false,
  }
}
