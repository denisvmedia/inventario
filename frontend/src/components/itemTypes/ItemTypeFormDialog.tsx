import { useEffect, useRef, useState } from "react"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"

import { FieldError } from "@/components/FieldError"
import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { ItemTypeIconPicker } from "@/components/itemTypes/ItemTypeIconPicker"
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
  ITEM_TYPE_LIMIT,
  ITEM_TYPE_NAME_TAKEN,
  ITEM_TYPES_MAX,
  type ItemType,
} from "@/features/itemTypes/api"
import {
  builtinItemTypeLabel,
  findItemTypeByName,
  isBuiltinItemTypeSlug,
  itemTypeLabel,
} from "@/features/itemTypes/display"
import { DEFAULT_ITEM_TYPE_ICON } from "@/features/itemTypes/icons"
import {
  itemTypeFormSchema,
  type ItemTypeFormInput,
  type ItemTypeFormValues,
} from "@/features/itemTypes/schemas"
import { applyServerFieldErrors, shouldShowGenericError } from "@/lib/form-errors"
import {
  classifyServerError,
  getServerErrorCode,
  type ClassifiedServerError,
} from "@/lib/server-error"

const NAME_TAKEN_MESSAGE = "itemTypes:validation.nameTaken"
const NAME_REQUIRED_MESSAGE = "itemTypes:validation.nameRequired"

export interface ItemTypeFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // The type to edit; null or undefined creates one.
  itemType?: ItemType | null
  // The name a new type starts with.
  defaultName?: string
  // The group's types, to refuse a name another type already goes by
  // before the server does.
  types: readonly ItemType[]
  // Rejects with the server's error; the dialog then keeps what was typed
  // and shows the error. On success the dialog closes itself.
  onSubmit: (values: ItemTypeFormValues) => Promise<unknown>
  isPending?: boolean
  // Where the focus goes when the dialog closes, for a host that opens it
  // from somewhere other than a DialogTrigger (the item form's picker).
  onCloseAutoFocus?: (event: Event) => void
  // For a host that picks a type (the item form): when the name is taken,
  // the dialog offers to use the type that has it. onUseExisting receives
  // it, and the dialog then closes.
  onUseExisting?: (type: ItemType) => void
}

// The create / edit form of an item type (#2802): a name and an icon. The
// mock has no item types; the dialog follows the Locations page's
// LocationFormDialog, as the seller dialog does.
export function ItemTypeFormDialog({
  open,
  onOpenChange,
  itemType,
  defaultName = "",
  types,
  onSubmit,
  isPending = false,
  onCloseAutoFocus,
  onUseExisting,
}: ItemTypeFormDialogProps) {
  const { t } = useTranslation()
  const [serverError, setServerError] = useState<ClassifiedServerError | null>(null)
  // The name last refused as taken.
  const [takenName, setTakenName] = useState("")
  const isEdit = !!itemType
  const builtinSlug = itemType && isBuiltinItemTypeSlug(itemType.slug) ? itemType.slug : undefined

  const form = useForm<ItemTypeFormInput, unknown, ItemTypeFormValues>({
    resolver: zodResolver(itemTypeFormSchema),
    defaultValues: { name: "", icon: DEFAULT_ITEM_TYPE_ICON },
  })
  const { errors } = form.formState

  // Prefill when the dialog opens, and not again while it stays open, so a
  // refetch of the types does not wipe what the user typed. A built-in type
  // that keeps its default name starts with an empty name, so saving it
  // unchanged keeps the name translated.
  const wasOpen = useRef(false)
  useEffect(() => {
    if (!open) {
      wasOpen.current = false
      return
    }
    if (wasOpen.current) return
    wasOpen.current = true
    form.reset({
      name: itemType ? (itemType.name ?? "") : defaultName,
      icon: itemType ? (itemType.icon ?? "") : DEFAULT_ITEM_TYPE_ICON,
    })
    setServerError(null)
    setTakenName("")
  }, [open, itemType, defaultName, form])

  function clashingType(name: string): ItemType | undefined {
    const found = findItemTypeByName(types, name)
    return found && found.id !== itemType?.id ? found : undefined
  }

  async function handle(values: ItemTypeFormValues) {
    setServerError(null)
    if (!values.name && !builtinSlug) {
      form.setError("name", { type: "required", message: NAME_REQUIRED_MESSAGE })
      return
    }
    if (values.name && clashingType(values.name)) {
      form.setError("name", { type: "taken", message: NAME_TAKEN_MESSAGE })
      setTakenName(values.name)
      return
    }
    try {
      await onSubmit(values)
      onOpenChange(false)
    } catch (err) {
      if (getServerErrorCode(err) === ITEM_TYPE_LIMIT) {
        setServerError({
          kind: "conflict",
          message: t("itemTypes:dialog.limit", { count: ITEM_TYPES_MAX }),
        })
        return
      }
      if (getServerErrorCode(err) === ITEM_TYPE_NAME_TAKEN) {
        form.setError("name", { type: "server", message: NAME_TAKEN_MESSAGE })
        setTakenName(values.name || (builtinSlug ? builtinItemTypeLabel(t, builtinSlug) : ""))
        return
      }
      const fieldResult = applyServerFieldErrors(err, form.setError, {
        fields: Object.keys(itemTypeFormSchema.shape),
      })
      setServerError(
        shouldShowGenericError(fieldResult)
          ? classifyServerError(err, t("itemTypes:dialog.errorGeneric"))
          : null
      )
    }
  }

  // The type that has the refused name, once the list knows it, and only
  // while the name error still shows (typing clears it).
  const offered =
    takenName && onUseExisting && errors.name?.message === NAME_TAKEN_MESSAGE
      ? clashingType(takenName)
      : undefined

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="sm:max-w-lg"
        onCloseAutoFocus={onCloseAutoFocus}
        data-testid="item-type-form-dialog"
      >
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t("itemTypes:dialog.editTitle") : t("itemTypes:dialog.createTitle")}
          </DialogTitle>
          <DialogDescription>{t("itemTypes:dialog.description")}</DialogDescription>
        </DialogHeader>

        <form className="flex flex-col gap-4 py-1" onSubmit={form.handleSubmit(handle)} noValidate>
          <div className="space-y-1.5">
            <Label htmlFor="item-type-name">
              {t("itemTypes:dialog.nameLabel")}
              {builtinSlug ? null : <span className="ms-0.5 text-destructive">*</span>}
            </Label>
            <Input
              id="item-type-name"
              placeholder={
                builtinSlug
                  ? builtinItemTypeLabel(t, builtinSlug)
                  : t("itemTypes:dialog.namePlaceholder")
              }
              autoComplete="off"
              disabled={isPending}
              aria-invalid={!!errors.name}
              aria-describedby={
                [
                  builtinSlug ? "item-type-name-hint" : "",
                  errors.name?.message ? "item-type-name-error" : "",
                ]
                  .filter(Boolean)
                  .join(" ") || undefined
              }
              data-testid="item-type-name-input"
              {...form.register("name")}
            />
            {builtinSlug ? (
              <p id="item-type-name-hint" className="text-xs text-muted-foreground">
                {t("itemTypes:dialog.builtinNameHint", {
                  name: builtinItemTypeLabel(t, builtinSlug),
                })}
              </p>
            ) : null}
            <FieldError
              id="item-type-name-error"
              testId="item-type-name-error"
              message={errors.name?.message}
            />
            {offered && onUseExisting ? (
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="h-7"
                onClick={() => {
                  onUseExisting(offered)
                  onOpenChange(false)
                }}
                data-testid="item-type-name-taken-use"
              >
                {t("itemTypes:dialog.useExisting", { name: itemTypeLabel(t, offered) })}
              </Button>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label id="item-type-icon-label">{t("itemTypes:dialog.iconLabel")}</Label>
            <Controller
              control={form.control}
              name="icon"
              render={({ field }) => (
                <ItemTypeIconPicker
                  value={field.value}
                  onChange={field.onChange}
                  labelledBy="item-type-icon-label"
                  disabled={isPending}
                />
              )}
            />
          </div>

          <ServerErrorBanner error={serverError} testId="item-type-form-server-error" />

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isPending}
              data-testid="item-type-form-cancel"
            >
              {t("common:actions.cancel")}
            </Button>
            <Button type="submit" disabled={isPending} data-testid="item-type-form-submit">
              {isPending
                ? t("itemTypes:dialog.submitting")
                : isEdit
                  ? t("itemTypes:dialog.editSubmit")
                  : t("itemTypes:dialog.createSubmit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
