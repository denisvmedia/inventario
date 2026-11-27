import { useEffect, useRef, useState } from "react"
import { Controller, useFieldArray, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Plus, X } from "lucide-react"

import { FieldError } from "@/components/FieldError"
import { CustomFieldTemplatesFieldset } from "@/components/customFields/CustomFieldTemplatesFieldset"
import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  CUSTOM_FIELD_LIMIT,
  CUSTOM_FIELD_NAME_MAX,
  CUSTOM_FIELD_NAME_TAKEN,
  CUSTOM_FIELD_OPTION_LABEL_MAX,
  CUSTOM_FIELD_OPTIONS_MAX,
  CUSTOM_FIELD_TYPES,
  CUSTOM_FIELDS_MAX,
  type CustomField,
  type CustomFieldTemplate,
} from "@/features/customFields/api"
import {
  customFieldFormSchema,
  type CustomFieldFormInput,
  type CustomFieldFormValues,
} from "@/features/customFields/schemas"
import {
  toApiValue,
  toFormValue,
  validateFormValue,
  type CustomFieldFormValues as CustomFieldValueForms,
} from "@/features/customFields/values"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { applyServerFieldErrors, shouldShowGenericError } from "@/lib/form-errors"
import { nameKey } from "@/lib/name-key"
import {
  classifyServerError,
  getServerErrorCode,
  type ClassifiedServerError,
} from "@/lib/server-error"

const NAME_TAKEN_MESSAGE = "customFields:validation.nameTaken"

export interface CustomFieldFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // The field to edit; null creates one.
  field: CustomField | null
  // The group's fields, to refuse a taken name before the server does.
  fields: readonly CustomField[]
  // Rejects with the server's error; the dialog keeps what was typed and
  // shows it. On success the dialog closes itself. templates are the item
  // types the field is offered on, with their defaults.
  onSubmit: (values: CustomFieldFormValues, templates: CustomFieldTemplate[]) => Promise<unknown>
  isPending?: boolean
}

// The create / edit form of a custom field (#2803): a name, a type that is
// fixed once the field exists, the options of a select field, and the item
// types whose items the item form offers the field on. The mock
// has no custom fields; the dialog follows ItemTypeFormDialog, with the
// options as a list of inputs in the way the item form lists its links.
export function CustomFieldFormDialog({
  open,
  onOpenChange,
  field,
  fields,
  onSubmit,
  isPending = false,
}: CustomFieldFormDialogProps) {
  const { t } = useTranslation()
  const [serverError, setServerError] = useState<ClassifiedServerError | null>(null)
  const isEdit = !!field
  const groupCurrency = useOptionalCurrentGroup()?.currentGroup?.group_currency ?? "USD"
  // The template entries by item type id, with the default as typed.
  const [templates, setTemplates] = useState<CustomFieldValueForms>({})
  const [templateErrors, setTemplateErrors] = useState<Record<string, string | undefined>>({})

  const form = useForm<CustomFieldFormInput, unknown, CustomFieldFormValues>({
    resolver: zodResolver(customFieldFormSchema),
    defaultValues: { name: "", type: "text", options: [] },
  })
  const { errors } = form.formState
  const options = useFieldArray({ control: form.control, name: "options" })
  const type = form.watch("type")
  const currentOptions = form.watch("options")

  // Prefill when the dialog opens, and not again while it stays open, so a
  // refetch of the fields does not wipe what was typed.
  const wasOpen = useRef(false)
  useEffect(() => {
    if (!open) {
      wasOpen.current = false
      return
    }
    if (wasOpen.current) return
    wasOpen.current = true
    form.reset({
      name: field?.name ?? "",
      type: field?.type ?? "text",
      options: (field?.options ?? []).map((o) => ({ id: o.id ?? "", label: o.label ?? "" })),
    })
    setTemplates(
      Object.fromEntries(
        (field?.templates ?? []).map((entry) => [
          entry.item_type_id,
          toFormValue(entry.default_value),
        ])
      )
    )
    setTemplateErrors({})
    setServerError(null)
  }, [open, field, form])

  // The field as the template defaults see it: the type picked, and of a
  // select field the options already saved, which a default can name.
  const draftField: CustomField = {
    ...(field ?? { id: "", name: "", options: [], templates: [] }),
    type,
    options: currentOptions.filter((o) => o.id).map((o) => ({ id: o.id, label: o.label })),
  }

  // Items that lose their value when the removed options go.
  const keptIds = new Set(currentOptions.map((o) => o.id).filter(Boolean))
  const clearedItems = (field?.options ?? [])
    .filter((o) => o.id && !keptIds.has(o.id))
    .reduce((sum, o) => sum + (o.item_count ?? 0), 0)

  async function handle(values: CustomFieldFormValues) {
    setServerError(null)
    const clash = fields.find((f) => f.id !== field?.id && nameKey(f.name) === nameKey(values.name))
    if (clash) {
      form.setError("name", { type: "taken", message: NAME_TAKEN_MESSAGE })
      return
    }
    // A link to an item takes no default; an option that is not saved yet
    // cannot be one.
    const entries: CustomFieldTemplate[] = []
    const nextErrors: Record<string, string | undefined> = {}
    const checkedField = { ...draftField, type: values.type }
    const savedOptions = new Set(checkedField.options.map((o) => o.id))
    for (const [itemTypeId, v] of Object.entries(templates)) {
      const key = values.type === "item_link" ? undefined : validateFormValue(checkedField, v)
      if (key) nextErrors[itemTypeId] = `customFields:errors.${key}`
      const removedOption = values.type === "select" && !savedOptions.has(v.value)
      const def = values.type === "item_link" || removedOption ? null : toApiValue(values.type, v)
      entries.push({ item_type_id: itemTypeId, default_value: def })
    }
    setTemplateErrors(nextErrors)
    if (Object.keys(nextErrors).length > 0) return
    try {
      await onSubmit(values, entries)
      onOpenChange(false)
    } catch (err) {
      const code = getServerErrorCode(err)
      if (code === CUSTOM_FIELD_LIMIT) {
        setServerError({
          kind: "conflict",
          message: t("customFields:dialog.limit", { max: CUSTOM_FIELDS_MAX }),
        })
        return
      }
      if (code === CUSTOM_FIELD_NAME_TAKEN) {
        form.setError("name", { type: "server", message: NAME_TAKEN_MESSAGE })
        return
      }
      const fieldResult = applyServerFieldErrors(err, form.setError, {
        fields: ["name", "type", "options"],
      })
      setServerError(
        shouldShowGenericError(fieldResult)
          ? classifyServerError(err, t("customFields:dialog.errorGeneric"))
          : null
      )
    }
  }

  const optionsError = errors.options?.message ?? errors.options?.root?.message

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        variant="fullscreen-mobile"
        className="sm:max-w-lg"
        data-testid="custom-field-form-dialog"
      >
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t("customFields:dialog.editTitle") : t("customFields:dialog.createTitle")}
          </DialogTitle>
          <DialogDescription>{t("customFields:dialog.description")}</DialogDescription>
        </DialogHeader>

        <form className="flex flex-col gap-4 py-1" onSubmit={form.handleSubmit(handle)} noValidate>
          <div className="space-y-1.5">
            <Label htmlFor="custom-field-name">
              {t("customFields:dialog.nameLabel")}
              <span className="ms-0.5 text-destructive">*</span>
            </Label>
            <Input
              id="custom-field-name"
              placeholder={t("customFields:dialog.namePlaceholder")}
              autoComplete="off"
              disabled={isPending}
              aria-invalid={!!errors.name}
              aria-describedby={errors.name ? "custom-field-name-error" : undefined}
              data-testid="custom-field-name-input"
              {...form.register("name")}
            />
            <FieldError
              id="custom-field-name-error"
              testId="custom-field-name-error"
              message={errors.name?.message}
              values={{ max: CUSTOM_FIELD_NAME_MAX }}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="custom-field-type">{t("customFields:dialog.typeLabel")}</Label>
            <Controller
              control={form.control}
              name="type"
              render={({ field: typeField }) => (
                <Select
                  value={typeField.value}
                  onValueChange={(next) => {
                    typeField.onChange(next)
                    // A default ticked while the field had another type has
                    // no currency yet; the money input shows the group's,
                    // so the default takes it. A currency picked stays.
                    if (next === "money") {
                      setTemplates((prev) =>
                        Object.fromEntries(
                          Object.entries(prev).map(([id, v]) => [
                            id,
                            v.currency ? v : { ...v, currency: groupCurrency },
                          ])
                        )
                      )
                    }
                  }}
                  disabled={isPending || isEdit}
                >
                  <SelectTrigger
                    id="custom-field-type"
                    className="w-full"
                    aria-describedby={isEdit ? "custom-field-type-hint" : undefined}
                    data-testid="custom-field-type-select"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {CUSTOM_FIELD_TYPES.map((value) => (
                      <SelectItem
                        key={value}
                        value={value}
                        data-testid={`custom-field-type-option-${value}`}
                      >
                        {t(`customFields:types.${value}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
            {isEdit ? (
              <p id="custom-field-type-hint" className="text-xs text-muted-foreground">
                {t("customFields:dialog.typeLocked")}
              </p>
            ) : null}
          </div>

          {type === "select" ? (
            <fieldset className="space-y-2" data-testid="custom-field-options">
              <legend className="text-sm font-medium">
                {t("customFields:dialog.optionsLabel")}
              </legend>
              {options.fields.map((option, index) => {
                // option.id is the field array's own key; the option's id
                // is in the form values.
                const optionId = currentOptions[index]?.id
                const usage = optionId
                  ? field?.options.find((o) => o.id === optionId)?.item_count
                  : undefined
                const error = errors.options?.[index]?.label?.message
                return (
                  <div key={option.id} className="space-y-1">
                    <div className="flex items-center gap-2">
                      <Input
                        aria-label={t("customFields:dialog.optionPlaceholder", {
                          index: index + 1,
                        })}
                        placeholder={t("customFields:dialog.optionPlaceholder", {
                          index: index + 1,
                        })}
                        disabled={isPending}
                        aria-invalid={!!error}
                        data-testid={`custom-field-option-input-${index}`}
                        {...form.register(`options.${index}.label`)}
                      />
                      {usage ? (
                        <span className="shrink-0 text-xs text-muted-foreground">
                          {t("customFields:dialog.optionUsage", { count: usage })}
                        </span>
                      ) : null}
                      <Button
                        type="button"
                        size="icon"
                        variant="ghost"
                        className="size-8 shrink-0"
                        onClick={() => options.remove(index)}
                        disabled={isPending}
                        aria-label={t("customFields:dialog.removeOption", { index: index + 1 })}
                        data-testid={`custom-field-option-remove-${index}`}
                      >
                        <X aria-hidden="true" className="size-4" />
                      </Button>
                    </div>
                    <FieldError message={error} values={{ max: CUSTOM_FIELD_OPTION_LABEL_MAX }} />
                  </div>
                )
              })}
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => options.append({ label: "" })}
                disabled={isPending || options.fields.length >= CUSTOM_FIELD_OPTIONS_MAX}
                data-testid="custom-field-option-add"
              >
                <Plus aria-hidden="true" className="mr-1 size-3.5" />
                {t("customFields:dialog.addOption")}
              </Button>
              <FieldError
                testId="custom-field-options-error"
                message={optionsError}
                values={{ max: CUSTOM_FIELD_OPTIONS_MAX }}
              />
              {clearedItems > 0 ? (
                <p
                  className="text-xs text-status-expiring"
                  data-testid="custom-field-options-cleared"
                >
                  {t("customFields:dialog.removedOptions", { count: clearedItems })}
                </p>
              ) : null}
            </fieldset>
          ) : null}

          <CustomFieldTemplatesFieldset
            field={draftField}
            value={templates}
            onChange={setTemplates}
            errors={templateErrors}
            groupCurrency={groupCurrency}
            disabled={isPending}
          />

          <ServerErrorBanner error={serverError} testId="custom-field-form-server-error" />

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isPending}
              data-testid="custom-field-form-cancel"
            >
              {t("common:actions.cancel")}
            </Button>
            <Button type="submit" disabled={isPending} data-testid="custom-field-form-submit">
              {isPending
                ? t("customFields:dialog.submitting")
                : isEdit
                  ? t("customFields:dialog.editSubmit")
                  : t("customFields:dialog.createSubmit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
