import { useTranslation } from "react-i18next"

import { FieldError } from "@/components/FieldError"
import { CustomFieldValueInput } from "@/components/customFields/CustomFieldsEditor"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import type { CustomField } from "@/features/customFields/api"
import type { CustomFieldFormValue, CustomFieldFormValues } from "@/features/customFields/values"
import { useItemTypeLookup } from "@/features/itemTypes/hooks"

export interface CustomFieldTemplatesFieldsetProps {
  // The field as the dialog has it: its type, and for a select field the
  // options that have an id (saved ones), which a default can name.
  field: CustomField
  // The template entries by item type id; the value is the default, empty
  // for none.
  value: CustomFieldFormValues
  onChange: (value: CustomFieldFormValues) => void
  // An error message key per item type id.
  errors: Record<string, string | undefined>
  groupCurrency: string
  disabled?: boolean
}

// CustomFieldTemplatesFieldset picks the item types whose items the item
// form offers the field on (#2803), each with an optional default the form
// fills in while the item has no value of its own. A link to an item takes
// no default.
export function CustomFieldTemplatesFieldset({
  field,
  value,
  onChange,
  errors,
  groupCurrency,
  disabled,
}: CustomFieldTemplatesFieldsetProps) {
  const { t } = useTranslation()
  const lookup = useItemTypeLookup()
  const takesDefault = field.type !== "item_link"

  function toggle(id: string) {
    const next = { ...value }
    if (id in next) delete next[id]
    else next[id] = { value: "", currency: field.type === "money" ? groupCurrency : "" }
    onChange(next)
  }

  return (
    <fieldset className="space-y-2" data-testid="custom-field-templates">
      <legend className="text-sm font-medium">{t("customFields:templates.label")}</legend>
      <p className="text-xs text-muted-foreground">
        {takesDefault
          ? t("customFields:templates.hint")
          : t("customFields:templates.hintNoDefault")}
      </p>
      {field.type === "select" && field.options.length === 0 ? (
        <p className="text-xs text-muted-foreground">{t("customFields:templates.optionsFirst")}</p>
      ) : null}
      <ul className="flex max-h-72 flex-col gap-2 overflow-y-auto rounded-md border border-border p-2">
        {lookup.types.map((type) => {
          const display = lookup.get(type.slug)
          const Icon = display?.Icon
          const checked = type.id in value
          const checkboxId = `custom-field-template-${type.id}`
          const inputId = `custom-field-template-default-${type.id}`
          const error = errors[type.id]
          const errorId = error ? `${inputId}-error` : undefined
          return (
            <li key={type.id} className="space-y-1.5">
              <div className="flex items-center gap-2">
                <Checkbox
                  id={checkboxId}
                  checked={checked}
                  onCheckedChange={() => toggle(type.id)}
                  disabled={disabled}
                  data-testid={`custom-field-template-${type.slug}`}
                />
                <Label htmlFor={checkboxId} className="flex items-center gap-1.5 font-normal">
                  {Icon ? (
                    <Icon aria-hidden="true" className="size-3.5 text-muted-foreground" />
                  ) : null}
                  {display?.label ?? type.slug}
                </Label>
              </div>
              {checked && takesDefault ? (
                <div className="space-y-1 ps-6">
                  <Label htmlFor={inputId} className="text-xs text-muted-foreground">
                    {t("customFields:templates.default")}
                  </Label>
                  <CustomFieldValueInput
                    field={field}
                    inputId={inputId}
                    value={value[type.id] as CustomFieldFormValue}
                    set={(patch) =>
                      onChange({ ...value, [type.id]: { ...value[type.id], ...patch } })
                    }
                    invalid={!!error}
                    describedBy={errorId}
                    groupCurrency={groupCurrency}
                    disabled={disabled}
                  />
                  <FieldError id={errorId} message={error} />
                </div>
              ) : null}
            </li>
          )
        })}
      </ul>
    </fieldset>
  )
}
