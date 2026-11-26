import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { Plus, X } from "lucide-react"

import { CurrencyCombobox } from "@/components/CurrencyCombobox"
import { FieldError } from "@/components/FieldError"
import { ItemLinkPicker, type LinkedItem } from "@/components/customFields/ItemLinkPicker"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import {
  CUSTOM_FIELD_LONG_TEXT_MAX,
  CUSTOM_FIELD_TEXT_MAX,
  CUSTOM_FIELD_URL_MAX,
  type CustomField,
} from "@/features/customFields/api"
import {
  shownFields,
  type CustomFieldFormValue,
  type CustomFieldFormValues,
} from "@/features/customFields/values"

const EMPTY: CustomFieldFormValue = { value: "", currency: "" }

export interface CustomFieldsEditorProps {
  // The group's fields, by name.
  fields: readonly CustomField[]
  // The form's entries by field id; a key is a field the form shows.
  values: CustomFieldFormValues | undefined
  // Sets one entry; undefined takes the field off the form, which clears
  // its value on save.
  onChange: (id: string, value: CustomFieldFormValue | undefined) => void
  // An error message (an i18n key or a server message) per field id.
  errors?: Record<string, string | undefined>
  linkedItems?: Record<string, LinkedItem>
  // The item being edited, which a link cannot name.
  selfId?: string
  // The currency a new money value starts with.
  groupCurrency: string
  // The Custom fields page, for the link under the add menu.
  manageHref?: string
  disabled?: boolean
}

// CustomFieldsEditor edits an item's custom field values (#2803) in the
// item forms: the fields the item has a value for, one input per type,
// and an "Add field" menu with the group's other fields. The mock has no
// custom fields; the inputs follow the item form's own (Input, Textarea,
// Select, CurrencyCombobox next to an amount), and the add menu follows
// the items list's column menu.
export function CustomFieldsEditor({
  fields,
  values,
  onChange,
  errors,
  linkedItems,
  selfId,
  groupCurrency,
  manageHref,
  disabled,
}: CustomFieldsEditorProps) {
  const { t } = useTranslation()
  const shown = shownFields(fields, values)
  const addable = fields.filter((f) => !(values && f.id in values))

  return (
    <div className="flex flex-col gap-4" data-testid="custom-fields-editor">
      {shown.map((field) => {
        const inputId = `custom-field-${field.id}`
        const value = values?.[field.id] ?? EMPTY
        const error = errors?.[field.id]
        const errorId = error ? `${inputId}-error` : undefined
        const set = (next: Partial<CustomFieldFormValue>) =>
          onChange(field.id, { ...value, ...next })
        return (
          <div key={field.id} className="space-y-1.5" data-testid={`custom-field-${field.id}`}>
            <div className="flex items-center justify-between gap-2">
              <Label htmlFor={inputId}>{field.name}</Label>
              <Button
                type="button"
                size="icon"
                variant="ghost"
                className="size-6 text-muted-foreground"
                onClick={() => onChange(field.id, undefined)}
                disabled={disabled}
                aria-label={t("customFields:section.remove", { name: field.name })}
                data-testid={`custom-field-${field.id}-remove`}
              >
                <X aria-hidden="true" className="size-3.5" />
              </Button>
            </div>
            <FieldInput
              field={field}
              inputId={inputId}
              value={value}
              set={set}
              invalid={!!error}
              describedBy={errorId}
              linkedItems={linkedItems}
              selfId={selfId}
              groupCurrency={groupCurrency}
              disabled={disabled}
            />
            <FieldError id={errorId} testId={`custom-field-${field.id}-error`} message={error} />
          </div>
        )
      })}

      <div className="flex flex-wrap items-center gap-3">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={disabled || addable.length === 0}
              aria-label={t("customFields:section.addLabel")}
              data-testid="custom-field-add"
            >
              <Plus aria-hidden="true" className="mr-1 size-3.5" />
              {t("customFields:section.add")}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="max-h-72 overflow-y-auto">
            {addable.map((field) => (
              <DropdownMenuItem
                key={field.id}
                onSelect={() =>
                  onChange(field.id, {
                    value: "",
                    currency: field.type === "money" ? groupCurrency : "",
                  })
                }
                data-testid={`custom-field-add-${field.id}`}
              >
                <span className="truncate">{field.name}</span>
                <span className="ml-auto pl-3 text-xs text-muted-foreground">
                  {t(`customFields:types.${field.type}`)}
                </span>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
        {addable.length === 0 && shown.length > 0 ? (
          <span className="text-xs text-muted-foreground">
            {t("customFields:section.noneLeft")}
          </span>
        ) : null}
        {manageHref ? (
          <Link
            to={manageHref}
            className="text-xs text-muted-foreground underline-offset-4 hover:underline"
            data-testid="custom-field-manage"
          >
            {t("customFields:section.manage")}
          </Link>
        ) : null}
      </div>
    </div>
  )
}

interface FieldInputProps {
  field: CustomField
  inputId: string
  value: CustomFieldFormValue
  set: (next: Partial<CustomFieldFormValue>) => void
  invalid: boolean
  describedBy?: string
  linkedItems?: Record<string, LinkedItem>
  selfId?: string
  groupCurrency: string
  disabled?: boolean
}

function FieldInput({
  field,
  inputId,
  value,
  set,
  invalid,
  describedBy,
  linkedItems,
  selfId,
  groupCurrency,
  disabled,
}: FieldInputProps) {
  const { t } = useTranslation()
  const common = {
    id: inputId,
    disabled,
    "aria-invalid": invalid,
    "aria-describedby": describedBy,
    "data-testid": `${inputId}-input`,
  }
  switch (field.type) {
    case "long_text":
      return (
        <Textarea
          {...common}
          rows={3}
          maxLength={CUSTOM_FIELD_LONG_TEXT_MAX}
          value={value.value}
          onChange={(e) => set({ value: e.target.value })}
        />
      )
    case "number":
      return (
        <Input
          {...common}
          inputMode="decimal"
          value={value.value}
          onChange={(e) => set({ value: e.target.value })}
        />
      )
    case "money":
      return (
        <div className="flex gap-2">
          <Input
            {...common}
            inputMode="decimal"
            className="flex-1"
            value={value.value}
            onChange={(e) => set({ value: e.target.value })}
          />
          <CurrencyCombobox
            variant="compact"
            value={value.currency || groupCurrency}
            onChange={(currency) => set({ currency })}
            disabled={disabled}
            placeholder={t("customFields:section.currencyLabel", { name: field.name })}
            ariaLabel={t("customFields:section.currencyLabel", { name: field.name })}
            ariaInvalid={invalid}
          />
        </div>
      )
    case "date":
      return (
        <Input
          {...common}
          type="date"
          value={value.value}
          onChange={(e) => set({ value: e.target.value })}
        />
      )
    case "url":
      return (
        <Input
          {...common}
          type="url"
          inputMode="url"
          maxLength={CUSTOM_FIELD_URL_MAX}
          placeholder="https://"
          value={value.value}
          onChange={(e) => set({ value: e.target.value })}
        />
      )
    case "boolean":
      return (
        <Select
          value={value.value}
          onValueChange={(next) => set({ value: next })}
          disabled={disabled}
        >
          <SelectTrigger
            id={inputId}
            className="w-full"
            aria-invalid={invalid}
            aria-describedby={describedBy}
            data-testid={`${inputId}-input`}
          >
            <SelectValue placeholder={t("customFields:section.selectPlaceholder")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="true">{t("customFields:section.yes")}</SelectItem>
            <SelectItem value="false">{t("customFields:section.no")}</SelectItem>
          </SelectContent>
        </Select>
      )
    case "select":
      return (
        <Select
          value={value.value}
          onValueChange={(next) => set({ value: next })}
          disabled={disabled}
        >
          <SelectTrigger
            id={inputId}
            className="w-full"
            aria-invalid={invalid}
            aria-describedby={describedBy}
            data-testid={`${inputId}-input`}
          >
            <SelectValue placeholder={t("customFields:section.selectPlaceholder")} />
          </SelectTrigger>
          <SelectContent>
            {field.options.map((option) => (
              <SelectItem key={option.id} value={option.id ?? ""}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )
    case "item_link":
      return (
        <ItemLinkPicker
          id={inputId}
          value={value.value}
          onChange={(id) => set({ value: id })}
          linkedItems={linkedItems}
          selfId={selfId}
          disabled={disabled}
          ariaInvalid={invalid}
        />
      )
    default:
      return (
        <Input
          {...common}
          maxLength={CUSTOM_FIELD_TEXT_MAX}
          value={value.value}
          onChange={(e) => set({ value: e.target.value })}
        />
      )
  }
}
