import { useState } from "react"
import { useTranslation } from "react-i18next"
import { ChevronDown, ListFilter, X } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import type { CustomField } from "@/features/customFields/api"
import { customFieldConditionLabel } from "@/features/customFields/conditionLabel"
import {
  formatCustomFieldCondition,
  opsForType,
  opTakesValue,
  type CustomFieldOp,
} from "@/features/customFields/conditions"
import { cn } from "@/lib/utils"

// The server takes at most this many cf conditions (commodityquery).
export const CUSTOM_FIELD_CONDITIONS_MAX = 20

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/

export interface CustomFieldFilterMenuProps {
  // The group's fields, by name; undefined while they load.
  fields: readonly CustomField[] | undefined
  // The query's conditions, as the API spells them.
  conditions: readonly string[]
  onChange: (conditions: string[]) => void
}

interface Draft {
  fieldId: string
  op: CustomFieldOp | ""
  value: string
  optionIds: string[]
}

const EMPTY_DRAFT: Draft = { fieldId: "", op: "", value: "", optionIds: [] }

// The value a draft condition carries, or null while it is incomplete.
function draftValue(field: CustomField, draft: Draft): string | null {
  if (!draft.op) return null
  if (!opTakesValue(draft.op)) return ""
  switch (field.type) {
    case "select":
      return draft.optionIds.length > 0 ? draft.optionIds.join(",") : null
    case "number":
    case "money": {
      const v = draft.value.trim()
      return v !== "" && Number.isFinite(Number(v)) ? v : null
    }
    case "date":
      return DATE_RE.test(draft.value) ? draft.value : null
    case "boolean":
      return draft.value === "true" || draft.value === "false" ? draft.value : null
    default:
      return draft.value.trim() !== "" ? draft.value.trim() : null
  }
}

// CustomFieldFilterMenu narrows the items list by custom field values
// (#2803): it lists the query's conditions, each removable, and builds a
// new one from a field, a comparison its type takes, and a value. A
// condition on a deleted field stays listed, marked, so it can be removed.
export function CustomFieldFilterMenu({ fields, conditions, onChange }: CustomFieldFilterMenuProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState<Draft>(EMPTY_DRAFT)
  const field = fields?.find((f) => f.id === draft.fieldId)
  const value = field ? draftValue(field, draft) : null
  const full = conditions.length >= CUSTOM_FIELD_CONDITIONS_MAX

  function add() {
    if (!field || !draft.op || value === null) return
    const raw = formatCustomFieldCondition({ fieldId: field.id, op: draft.op, value })
    if (!conditions.includes(raw)) onChange([...conditions, raw])
    setDraft(EMPTY_DRAFT)
  }

  function remove(raw: string) {
    onChange(conditions.filter((c) => c !== raw))
  }

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) setDraft(EMPTY_DRAFT)
      }}
    >
      <PopoverTrigger asChild>
        <Button
          variant={conditions.length > 0 ? "default" : "outline"}
          size="sm"
          className="gap-1.5"
          data-testid="commodities-filter-custom-fields"
        >
          <ListFilter className="size-3.5" aria-hidden="true" />
          {t("customFields:filter.button")}
          {conditions.length > 0 ? (
            <Badge variant="secondary" className="ml-0.5 h-4 px-1 text-xs">
              {conditions.length}
            </Badge>
          ) : null}
          <ChevronDown className="size-3.5" aria-hidden="true" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 p-0">
        <div className="px-3 py-2 text-sm font-medium">{t("customFields:filter.title")}</div>
        <Separator />
        {conditions.length > 0 ? (
          <ul className="flex flex-col gap-1 p-2" data-testid="commodities-filter-cf-conditions">
            {conditions.map((raw, index) => {
              const { label, stale } = customFieldConditionLabel(t, raw, fields)
              return (
                <li
                  key={raw}
                  className={cn(
                    "flex items-center gap-2 rounded-md px-2 py-1 text-sm",
                    stale ? "bg-status-expiring/10 text-status-expiring" : "bg-muted/60"
                  )}
                  data-testid={`commodities-filter-cf-condition-${index}`}
                  data-stale={stale ? "true" : undefined}
                >
                  <span className="min-w-0 flex-1 truncate">{label}</span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-6 shrink-0"
                    onClick={() => remove(raw)}
                    aria-label={t("customFields:filter.remove", { condition: label })}
                    data-testid={`commodities-filter-cf-remove-${index}`}
                  >
                    <X className="size-3.5" aria-hidden="true" />
                  </Button>
                </li>
              )
            })}
          </ul>
        ) : (
          <p className="px-3 py-2 text-sm text-muted-foreground">
            {t("customFields:filter.none")}
          </p>
        )}
        <Separator />
        {full ? (
          <p className="px-3 py-2 text-sm text-muted-foreground">
            {t("customFields:filter.full", { max: CUSTOM_FIELD_CONDITIONS_MAX })}
          </p>
        ) : fields && fields.length > 0 ? (
          <div className="flex flex-col gap-2 p-3" data-testid="commodities-filter-cf-new">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="cf-filter-field" className="text-xs">
                {t("customFields:filter.field")}
              </Label>
              <Select
                value={draft.fieldId}
                onValueChange={(id) => {
                  const next = fields.find((f) => f.id === id)
                  setDraft({ ...EMPTY_DRAFT, fieldId: id, op: next ? opsForType(next.type)[0] : "" })
                }}
              >
                <SelectTrigger
                  id="cf-filter-field"
                  className="h-8 w-full"
                  data-testid="commodities-filter-cf-field"
                >
                  <SelectValue placeholder={t("customFields:filter.pickField")} />
                </SelectTrigger>
                <SelectContent>
                  {fields.map((f) => (
                    <SelectItem key={f.id} value={f.id}>
                      {f.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {field ? (
              <>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="cf-filter-op" className="text-xs">
                    {t("customFields:filter.op")}
                  </Label>
                  <Select
                    value={draft.op}
                    onValueChange={(op) => setDraft({ ...draft, op: op as CustomFieldOp })}
                  >
                    <SelectTrigger
                      id="cf-filter-op"
                      className="h-8 w-full"
                      data-testid="commodities-filter-cf-op"
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {opsForType(field.type).map((op) => (
                        <SelectItem key={op} value={op}>
                          {t(`customFields:filter.ops.${op}`)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                {draft.op && opTakesValue(draft.op) ? (
                  <ValueInput field={field} draft={draft} onChange={setDraft} />
                ) : null}
              </>
            ) : null}
            <Button
              type="button"
              size="sm"
              className="self-end"
              disabled={value === null}
              onClick={add}
              data-testid="commodities-filter-cf-add"
            >
              {t("customFields:filter.add")}
            </Button>
          </div>
        ) : null}
      </PopoverContent>
    </Popover>
  )
}

function ValueInput({
  field,
  draft,
  onChange,
}: {
  field: CustomField
  draft: Draft
  onChange: (draft: Draft) => void
}) {
  const { t } = useTranslation()
  const label = (
    <Label htmlFor="cf-filter-value" className="text-xs">
      {t("customFields:filter.value")}
    </Label>
  )
  if (field.type === "select") {
    return (
      <fieldset className="flex flex-col gap-1.5" data-testid="commodities-filter-cf-options">
        <legend className="mb-1.5 text-xs font-medium">{t("customFields:filter.value")}</legend>
        <div className="flex max-h-40 flex-col gap-1.5 overflow-y-auto">
          {field.options.map((o) => {
            const checked = draft.optionIds.includes(o.id ?? "")
            return (
              <label key={o.id} className="flex items-center gap-2 text-sm">
                <Checkbox
                  checked={checked}
                  onCheckedChange={() =>
                    onChange({
                      ...draft,
                      optionIds: checked
                        ? draft.optionIds.filter((id) => id !== o.id)
                        : [...draft.optionIds, o.id ?? ""],
                    })
                  }
                  data-testid={`commodities-filter-cf-option-${o.id}`}
                />
                {o.label}
              </label>
            )
          })}
        </div>
      </fieldset>
    )
  }
  if (field.type === "boolean") {
    return (
      <div className="flex flex-col gap-1.5">
        {label}
        <Select value={draft.value} onValueChange={(v) => onChange({ ...draft, value: v })}>
          <SelectTrigger
            id="cf-filter-value"
            className="h-8 w-full"
            data-testid="commodities-filter-cf-value"
          >
            <SelectValue placeholder={t("customFields:filter.pickValue")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="true">{t("customFields:detail.yes")}</SelectItem>
            <SelectItem value="false">{t("customFields:detail.no")}</SelectItem>
          </SelectContent>
        </Select>
      </div>
    )
  }
  const type =
    field.type === "date"
      ? "date"
      : field.type === "number" || field.type === "money"
        ? "number"
        : "text"
  return (
    <div className="flex flex-col gap-1.5">
      {label}
      <Input
        id="cf-filter-value"
        type={type}
        step={type === "number" ? "any" : undefined}
        className="h-8"
        value={draft.value}
        onChange={(e) => onChange({ ...draft, value: e.target.value })}
        data-testid="commodities-filter-cf-value"
      />
    </div>
  )
}
