import { useEffect, useState } from "react"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"

import { Alert, AlertDescription } from "@/components/ui/alert"
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
import { IconPicker, LOCATION_ICONS } from "@/components/locations/IconPicker"
import type { Location } from "@/features/locations/api"
import { locationSchema, type LocationFormInput } from "@/features/locations/schemas"
import { parseServerError } from "@/lib/server-error"

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
  const [serverError, setServerError] = useState<string | null>(null)
  const isEdit = !!location

  const form = useForm<LocationFormInput>({
    resolver: zodResolver(locationSchema),
    defaultValues: { name: "", address: "", icon: "", description: "" },
  })

  // Reset the form whenever the dialog reopens or the editing target
  // changes — without this, opening the dialog after editing one
  // location would prefill with the previous location's name.
  useEffect(() => {
    // Sync external (open, location) props → form values + serverError.
    if (open) {
      form.reset({
        name: location?.name ?? "",
        address: location?.address ?? "",
        icon: location?.icon ?? "",
        description: location?.description ?? "",
      })
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setServerError(null)
    }
  }, [open, location, form])

  async function handle(values: LocationFormInput) {
    setServerError(null)
    try {
      await onSubmit(values)
      onOpenChange(false)
    } catch (err) {
      setServerError(parseServerError(err, t("locations:dialog.errorGeneric")))
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
            {form.formState.errors.icon ? (
              <p className="text-xs text-destructive" data-testid="location-icon-error">
                {t(form.formState.errors.icon.message ?? "")}
              </p>
            ) : null}
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
              data-testid="location-name-input"
              {...form.register("name")}
            />
            {form.formState.errors.name ? (
              <p className="text-xs text-destructive" data-testid="location-name-error">
                {t(form.formState.errors.name.message ?? "")}
              </p>
            ) : null}
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
              data-testid="location-description-input"
              className="resize-none"
              {...form.register("description")}
            />
            {form.formState.errors.description ? (
              <p className="text-xs text-destructive" data-testid="location-description-error">
                {t(form.formState.errors.description.message ?? "")}
              </p>
            ) : null}
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
              data-testid="location-address-input"
              {...form.register("address")}
            />
            {form.formState.errors.address ? (
              <p className="text-xs text-destructive" data-testid="location-address-error">
                {t(form.formState.errors.address.message ?? "")}
              </p>
            ) : null}
            <p className="text-[11px] text-muted-foreground">{t("locations:dialog.addressHelp")}</p>
          </div>

          {serverError ? (
            <Alert variant="destructive" data-testid="location-form-server-error">
              <AlertDescription>{serverError}</AlertDescription>
            </Alert>
          ) : null}
        </form>

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
          <Button
            type="submit"
            form="location-form"
            disabled={isPending}
            data-testid="location-form-submit"
          >
            {isPending
              ? t("locations:dialog.submitting")
              : isEdit
                ? t("locations:dialog.editSubmit")
                : t("locations:dialog.createSubmit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
