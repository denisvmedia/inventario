import { useEffect, useRef, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"

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
import { Textarea } from "@/components/ui/textarea"
import { FieldError } from "@/components/FieldError"
import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { SELLER_NAME_TAKEN, type Seller } from "@/features/sellers/api"
import {
  EMPTY_SELLER_FORM,
  sellerFormSchema,
  type SellerFormInput,
  type SellerFormValues,
} from "@/features/sellers/schemas"
import { applyServerFieldErrors, shouldShowGenericError } from "@/lib/form-errors"
import {
  classifyServerError,
  getServerErrorCode,
  type ClassifiedServerError,
} from "@/lib/server-error"

const NAME_TAKEN_MESSAGE = "sellers:validation.nameTaken"

export interface SellerFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // The seller to edit; null or undefined creates one.
  seller?: Seller | null
  // The name a new seller starts with, e.g. the one an AI scan read.
  defaultName?: string
  // Rejects with the server's error; the dialog then keeps what was typed
  // and shows the error. On success the dialog closes itself.
  onSubmit: (values: SellerFormValues) => Promise<unknown>
  isPending?: boolean
  // Where the focus goes when the dialog closes, for a host that opens it
  // from somewhere other than a DialogTrigger (the item form's picker).
  onCloseAutoFocus?: (event: Event) => void
  // For a host that picks a seller (the item form): when the server refuses
  // the name as taken, findExisting looks up the seller that has it, and the
  // dialog offers to use that seller instead. onUseExisting receives it, and
  // the dialog then closes.
  findExisting?: (name: string) => Seller | undefined
  onUseExisting?: (seller: Seller) => void
}

// The create / edit form of a seller (#2804): name, website, contact, note.
// The mock has no sellers; the dialog follows the Locations page's
// LocationFormDialog.
export function SellerFormDialog({
  open,
  onOpenChange,
  seller,
  defaultName = "",
  onSubmit,
  isPending = false,
  onCloseAutoFocus,
  findExisting,
  onUseExisting,
}: SellerFormDialogProps) {
  const { t } = useTranslation()
  const [serverError, setServerError] = useState<ClassifiedServerError | null>(null)
  // The name the server last refused as taken.
  const [takenName, setTakenName] = useState("")
  const isEdit = !!seller

  const form = useForm<SellerFormInput, unknown, SellerFormValues>({
    resolver: zodResolver(sellerFormSchema),
    defaultValues: EMPTY_SELLER_FORM,
  })
  const { errors } = form.formState

  // Prefill when the dialog opens, and not again while it stays open, so a
  // refetch of the seller does not wipe what the user typed.
  const wasOpen = useRef(false)
  useEffect(() => {
    if (!open) {
      wasOpen.current = false
      return
    }
    if (wasOpen.current) return
    wasOpen.current = true
    form.reset({
      name: seller?.name ?? defaultName,
      website: seller?.website ?? "",
      contact: seller?.contact ?? "",
      note: seller?.note ?? "",
    })
    setServerError(null)
    setTakenName("")
  }, [open, seller, defaultName, form])

  async function handle(values: SellerFormValues) {
    setServerError(null)
    try {
      await onSubmit(values)
      onOpenChange(false)
    } catch (err) {
      if (getServerErrorCode(err) === SELLER_NAME_TAKEN) {
        form.setError("name", { type: "server", message: NAME_TAKEN_MESSAGE })
        setTakenName(values.name)
        return
      }
      const fieldResult = applyServerFieldErrors(err, form.setError, {
        fields: Object.keys(sellerFormSchema.shape),
      })
      setServerError(
        shouldShowGenericError(fieldResult)
          ? classifyServerError(err, t("sellers:dialog.errorGeneric"))
          : null
      )
    }
  }

  // The seller that has the refused name, once the refreshed list knows it,
  // and only while the name error still shows (typing clears it).
  const existing =
    takenName && onUseExisting && errors.name?.message === NAME_TAKEN_MESSAGE
      ? findExisting?.(takenName)
      : undefined
  const offered = existing && existing.id !== seller?.id ? existing : undefined

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        variant="fullscreen-mobile"
        className="sm:max-w-lg"
        onCloseAutoFocus={onCloseAutoFocus}
        data-testid="seller-form-dialog"
      >
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t("sellers:dialog.editTitle") : t("sellers:dialog.createTitle")}
          </DialogTitle>
          <DialogDescription>{t("sellers:dialog.description")}</DialogDescription>
        </DialogHeader>

        <form className="flex flex-col gap-4 py-1" onSubmit={form.handleSubmit(handle)} noValidate>
          <div className="space-y-1.5">
            <Label htmlFor="seller-name">
              {t("sellers:dialog.nameLabel")}
              <span className="ms-0.5 text-destructive">*</span>
            </Label>
            <Input
              id="seller-name"
              placeholder={t("sellers:dialog.namePlaceholder")}
              autoComplete="off"
              disabled={isPending}
              aria-invalid={!!errors.name}
              aria-describedby={errors.name?.message ? "seller-name-error" : undefined}
              data-testid="seller-name-input"
              {...form.register("name")}
            />
            <FieldError
              id="seller-name-error"
              testId="seller-name-error"
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
                data-testid="seller-name-taken-use"
              >
                {t("sellers:dialog.useExisting", { name: offered.name })}
              </Button>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="seller-website">{t("sellers:dialog.websiteLabel")}</Label>
            <Input
              id="seller-website"
              type="url"
              inputMode="url"
              placeholder="https://"
              autoComplete="off"
              disabled={isPending}
              aria-invalid={!!errors.website}
              aria-describedby={errors.website?.message ? "seller-website-error" : undefined}
              data-testid="seller-website-input"
              {...form.register("website")}
            />
            <FieldError
              id="seller-website-error"
              testId="seller-website-error"
              message={errors.website?.message}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="seller-contact">{t("sellers:dialog.contactLabel")}</Label>
            <Input
              id="seller-contact"
              placeholder={t("sellers:dialog.contactPlaceholder")}
              autoComplete="off"
              disabled={isPending}
              aria-invalid={!!errors.contact}
              aria-describedby={errors.contact?.message ? "seller-contact-error" : undefined}
              data-testid="seller-contact-input"
              {...form.register("contact")}
            />
            <FieldError
              id="seller-contact-error"
              testId="seller-contact-error"
              message={errors.contact?.message}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="seller-note">{t("sellers:dialog.noteLabel")}</Label>
            <Textarea
              id="seller-note"
              placeholder={t("sellers:dialog.notePlaceholder")}
              rows={3}
              disabled={isPending}
              aria-invalid={!!errors.note}
              aria-describedby={errors.note?.message ? "seller-note-error" : undefined}
              data-testid="seller-note-input"
              className="resize-none"
              {...form.register("note")}
            />
            <FieldError
              id="seller-note-error"
              testId="seller-note-error"
              message={errors.note?.message}
            />
          </div>

          <ServerErrorBanner error={serverError} testId="seller-form-server-error" />

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isPending}
              data-testid="seller-form-cancel"
            >
              {t("common:actions.cancel")}
            </Button>
            <Button type="submit" disabled={isPending} data-testid="seller-form-submit">
              {isPending
                ? t("sellers:dialog.submitting")
                : isEdit
                  ? t("sellers:dialog.editSubmit")
                  : t("sellers:dialog.createSubmit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
