import { useEffect, useRef, useState } from "react"
import { Controller, useForm } from "react-hook-form"
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
import { IconPicker, LOCATION_ICONS } from "@/components/locations/IconPicker"
import type { Location } from "@/features/locations/api"
import { locationSchema, type LocationFormInput } from "@/features/locations/schemas"
import { applyServerFieldErrors, shouldShowGenericError } from "@/lib/form-errors"
import { classifyServerError, type ClassifiedServerError } from "@/lib/server-error"

interface LocationFormDialogProps {
  // True opens the dialog. Closing fires `onOpenChange(false)`.
  open: boolean
  onOpenChange: (open: boolean) => void
  // When provided the dialog is in edit mode and prefills the form.
  // `null` / `undefined` = create.
  location?: Location | null
  // Submit handler. Throws translate-able server errors which the
  // dialog renders as an inline `<Alert />`. The dialog calls
  // `onOpenChange(false)` on success itself; the host doesn't need to.
  onSubmit: (values: LocationFormInput) => Promise<unknown>
  // True while the host's mutation is in flight; disables submit + form
  // controls.
  isPending?: boolean
}

// LocationFormDialog renders the create / edit form as a modal —
// matches the design mock's `LocationDialog` (icon picker → name →
// description → address). The list page mounts it at any time and
// toggles `open`; deep-link routes (`/locations/new`,
// `/locations/:id/edit`) open it on mount via the same prop.
export function LocationFormDialog({
  open,
  onOpenChange,
  location,
  onSubmit,
  isPending = false,
}: LocationFormDialogProps) {
  const { t } = useTranslation()
  const [serverError, setServerError] = useState<ClassifiedServerError | null>(null)
  const isEdit = !!location

  const form = useForm<LocationFormInput>({
    resolver: zodResolver(locationSchema),
    defaultValues: { name: "", address: "", icon: "", description: "" },
  })

  // Reset/prefill fires on three triggers and three triggers only:
  //   1. open transitions false → true (typical open + clear stale error).
  //   2. the editing target id changes while the dialog stays open
  //      (deep-link navigation between `/locations/:id/edit` routes;
  //      LocationDetailPage keeps the same component instance when
  //      :id changes, so the dialog persists and we have to notice).
  //   3. the first `location` prop with data arrives after open
  //      (deep-link route mounts with open=true but the useLocation
  //      query still resolving — we prefill once it lands).
  // Refetch-induced reference churn for the SAME id is ignored, so
  // the catch in `handle` can set `serverError` and the refetch
  // doesn't wipe the inline alert (the original #1662 bug).
  const wasOpenRef = useRef(false)
  const prefilledIdRef = useRef<string | undefined>(undefined)
  useEffect(() => {
    if (!open) {
      wasOpenRef.current = false
      prefilledIdRef.current = undefined
      return
    }
    const justOpened = !wasOpenRef.current
    const targetId = location?.id
    const targetChanged = targetId !== undefined && targetId !== prefilledIdRef.current
    if (justOpened || targetChanged) {
      form.reset({
        name: location?.name ?? "",
        address: location?.address ?? "",
        icon: location?.icon ?? "",
        description: location?.description ?? "",
      })
      setServerError(null)
      prefilledIdRef.current = targetId ?? prefilledIdRef.current
    }
    wasOpenRef.current = true
  }, [open, location, form])

  async function handle(values: LocationFormInput) {
    setServerError(null)
    try {
      await onSubmit(values)
      onOpenChange(false)
    } catch (err) {
      // Map BE field-level validation errors (e.g. 422 `address: cannot be
      // blank`) back onto the inputs so the failing field is highlighted
      // with its inline message; only fall back to the generic banner for
      // non-field errors or anything that couldn't be placed on a field.
      const fieldResult = applyServerFieldErrors(err, form.setError, {
        fields: Object.keys(locationSchema.shape),
      })
      setServerError(
        shouldShowGenericError(fieldResult)
          ? classifyServerError(err, t("locations:dialog.errorGeneric"))
          : null
      )
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg" data-testid="location-form-dialog">
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t("locations:dialog.editTitle") : t("locations:dialog.createTitle")}
          </DialogTitle>
          <DialogDescription>
            {isEdit
              ? t("locations:dialog.editDescription")
              : t("locations:dialog.createDescription")}
          </DialogDescription>
        </DialogHeader>

        <form
          id="location-form"
          className="flex flex-col gap-4 py-1"
          onSubmit={form.handleSubmit(handle)}
          noValidate
        >
          <div className="space-y-1.5">
            <Controller
              control={form.control}
              name="icon"
              render={({ field }) => (
                <IconPicker
                  value={field.value}
                  onChange={field.onChange}
                  icons={LOCATION_ICONS}
                  label={t("locations:dialog.iconLabel")}
                  testIdPrefix="location-icon-picker"
                  disabled={isPending}
                />
              )}
            />
            <FieldError
              testId="location-icon-error"
              message={form.formState.errors.icon?.message}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="location-name">
              {t("locations:dialog.nameLabel")}
              <span className="ms-0.5 text-destructive">*</span>
            </Label>
            <Input
              id="location-name"
              placeholder={t("locations:dialog.namePlaceholder")}
              autoComplete="off"
              maxLength={200}
              disabled={isPending}
              aria-invalid={!!form.formState.errors.name}
              aria-describedby={form.formState.errors.name ? "location-name-error" : undefined}
              data-testid="location-name-input"
              {...form.register("name")}
            />
            <FieldError
              id="location-name-error"
              testId="location-name-error"
              message={form.formState.errors.name?.message}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="location-description">{t("locations:dialog.descriptionLabel")}</Label>
            <Textarea
              id="location-description"
              placeholder={t("locations:dialog.descriptionPlaceholder")}
              autoComplete="off"
              maxLength={200}
              rows={2}
              disabled={isPending}
              aria-invalid={!!form.formState.errors.description}
              aria-describedby={
                form.formState.errors.description ? "location-description-error" : undefined
              }
              data-testid="location-description-input"
              className="resize-none"
              {...form.register("description")}
            />
            <FieldError
              id="location-description-error"
              testId="location-description-error"
              message={form.formState.errors.description?.message}
            />
            <p className="text-[11px] text-muted-foreground">
              {t("locations:dialog.descriptionHelp")}
            </p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="location-address">{t("locations:dialog.addressLabel")}</Label>
            <Input
              id="location-address"
              placeholder={t("locations:dialog.addressPlaceholder")}
              autoComplete="off"
              maxLength={2000}
              disabled={isPending}
              aria-invalid={!!form.formState.errors.address}
              aria-describedby={
                form.formState.errors.address ? "location-address-error" : undefined
              }
              data-testid="location-address-input"
              {...form.register("address")}
            />
            <FieldError
              id="location-address-error"
              testId="location-address-error"
              message={form.formState.errors.address?.message}
            />
            <p className="text-[11px] text-muted-foreground">{t("locations:dialog.addressHelp")}</p>
          </div>

          <ServerErrorBanner error={serverError} testId="location-form-server-error" />

          {/* DialogFooter is rendered INSIDE the form. The submit button used
              to live outside the form and bind via `form="location-form"`, but
              the HTML "external form" attribute is unreliable on webkit when
              both elements are inside a Radix Dialog Portal — webkit-macos
              dropped the form-submission event entirely, so the click was a
              no-op and the POST never fired. Keeping the submit button as a
              normal in-form `type="submit"` sidesteps the issue. */}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isPending}
              data-testid="location-form-cancel"
            >
              {t("common:actions.cancel")}
            </Button>
            <Button type="submit" disabled={isPending} data-testid="location-form-submit">
              {isPending
                ? t("locations:dialog.submitting")
                : isEdit
                  ? t("locations:dialog.editSubmit")
                  : t("locations:dialog.createSubmit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
