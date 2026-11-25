import { useEffect, useRef, useState } from "react"
import { Controller, useFieldArray, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Plus, X } from "lucide-react"

import { FieldError } from "@/components/FieldError"
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
} from "@/features/customFields/api"
import {
  customFieldFormSchema,
  type CustomFieldFormInput,
  type CustomFieldFormValues,
} from "@/features/customFields/schemas"
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
  // shows it. On success the dialog closes itself.
  onSubmit: (values: CustomFieldFormValues) => Promise<unknown>
  isPending?: boolean
}

// The create / edit form of a custom field (#2803): a name, a type that is
// fixed once the field exists, and the options of a select field. The mock
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
    setServerError(null)
  }, [open, field, form])

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
    try {
      await onSubmit(values)
      onOpenChange(false)
    } catch (err) {
      const code = getServerErrorCode(err)
      if (code === CUSTOM_FIELD_LIMIT) {
        setServerError({
          kind: "conflict",
          message: t("customFields:dialog.limit", { count: CUSTOM_FIELDS_MAX }),
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
      <DialogContent className="sm:max-w-lg" data-testid="custom-field-form-dialog">
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
              maxLength={CUSTOM_FIELD_NAME_MAX}
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
                  onValueChange={typeField.onChange}
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
                const usage = field?.options.find((o) => o.id && o.id === option.id)?.item_count
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
                        maxLength={CUSTOM_FIELD_OPTION_LABEL_MAX}
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
                    <FieldError
                      message={error}
                      values={{ max: CUSTOM_FIELD_OPTION_LABEL_MAX }}
                    />
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
