// Custom field conditions and sorts of the item query (#2803), as the API
// spells them: "<field id>:<op>" for set and unset, "<field id>:<op>:<value>"
// for the others (devdocs/item-query-contract.md, "Custom fields"). The
// query keeps them as written; this module reads them for the filter menu
// and the saved-view chips, and writes the ones the menu builds.
import type { CustomField, CustomFieldType } from "./api"

export const CUSTOM_FIELD_OPS = ["set", "unset", "eq", "contains", "gte", "lte", "in"] as const
export type CustomFieldOp = (typeof CUSTOM_FIELD_OPS)[number]

export interface CustomFieldCondition {
  fieldId: string
  op: CustomFieldOp
  // Everything after the second colon; "" for set and unset.
  value: string
}

// The comparisons each type takes, mirroring go/registry/custom_field_query.go.
const OPS_BY_TYPE: Record<CustomFieldType, readonly CustomFieldOp[]> = {
  text: ["contains", "eq", "set", "unset"],
  long_text: ["contains", "eq", "set", "unset"],
  url: ["contains", "eq", "set", "unset"],
  number: ["eq", "gte", "lte", "set", "unset"],
  money: ["eq", "gte", "lte", "set", "unset"],
  date: ["eq", "gte", "lte", "set", "unset"],
  boolean: ["eq", "set", "unset"],
  select: ["in", "set", "unset"],
  item_link: ["set", "unset"],
}

// The comparisons the filter menu offers for type, in the order it lists
// them. An item link compares by item id, which the menu does not ask for.
export function opsForType(type: CustomFieldType): readonly CustomFieldOp[] {
  return OPS_BY_TYPE[type]
}

export function opTakesValue(op: CustomFieldOp): boolean {
  return op !== "set" && op !== "unset"
}

function isOp(v: string): v is CustomFieldOp {
  return (CUSTOM_FIELD_OPS as readonly string[]).includes(v)
}

// Reads a condition, or returns null for one this client cannot read (the
// server answers it with a 400 the list shows).
export function parseCustomFieldCondition(raw: string): CustomFieldCondition | null {
  const first = raw.indexOf(":")
  if (first <= 0) return null
  const fieldId = raw.slice(0, first)
  const rest = raw.slice(first + 1)
  const second = rest.indexOf(":")
  const op = second < 0 ? rest : rest.slice(0, second)
  const value = second < 0 ? "" : rest.slice(second + 1)
  if (!isOp(op)) return null
  if (opTakesValue(op) !== (second >= 0)) return null
  return { fieldId, op, value }
}

export function formatCustomFieldCondition(cond: CustomFieldCondition): string {
  return opTakesValue(cond.op)
    ? `${cond.fieldId}:${cond.op}:${cond.value}`
    : `${cond.fieldId}:${cond.op}`
}

// The types the list sorts by; long text and item links do not sort and the
// server falls back to name.
const SORTABLE_TYPES: ReadonlySet<CustomFieldType> = new Set([
  "text",
  "url",
  "number",
  "money",
  "date",
  "boolean",
  "select",
])

export function isSortableField(field: Pick<CustomField, "type">): boolean {
  return SORTABLE_TYPES.has(field.type)
}

// Numbers, amounts and dates start descending (the newest or largest first),
// like the built-in date and price sorts; the rest start ascending.
export function sortsDescendingFirst(field: Pick<CustomField, "type">): boolean {
  return field.type === "number" || field.type === "money" || field.type === "date"
}

// The ids of the fields conditions or a sort name that the group does not
// have, once its fields are known.
export function staleCustomFieldIds(
  conditions: readonly string[],
  sortFieldId: string,
  fieldIds: ReadonlySet<string>
): string[] {
  const named = conditions
    .map((raw) => parseCustomFieldCondition(raw)?.fieldId ?? "")
    .filter(Boolean)
  if (sortFieldId) named.push(sortFieldId)
  return [...new Set(named.filter((id) => !fieldIds.has(id)))]
}
