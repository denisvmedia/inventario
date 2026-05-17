import { useEffect, useRef, useState } from "react"
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
import { AREA_ICONS, IconPicker } from "@/components/locations/IconPicker"
import type { Area } from "@/features/areas/api"
import { areaSchema, type AreaFormInput } from "@/features/areas/schemas"
import type { Location } from "@/features/locations/api"
import { parseServerError } from "@/lib/server-error"

interface AreaFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // When provided, edit mode; prefills name + parent location.
  area?: Area | null
  // Available parent locations. The dialog hides the picker when only
  // one location exists (the parent is forced) and surfaces a `<select>`
  // when there are several. The list page is responsible for fetching
  // these.
  locations: Location[]
  // Initial parent — used when launching create from a specific
  // location's detail page so the dialog doesn't ask "where".
  defaultLocationId?: string
  onSubmit: (values: AreaFormInput) => Promise<unknown>
  isPending?: boolean
}

export function AreaFormDialog({
  open,
  onOpenChange,
  area,
  locations,
  defaultLocationId,
  onSubmit,
  isPending = false,
}: AreaFormDialogProps) {
  const { t } = useTranslation()
  const [serverError, setServerError] = useState<string | null>(null)
  const isEdit = !!area

  const form = useForm<AreaFormInput>({
    resolver: zodResolver(areaSchema),
    defaultValues: {
      name: "",
      // Pick a sensible default so the form isn't locked on first
      // mount. Edit mode overwrites this in the effect below.
      location_id: defaultLocationId ?? locations[0]?.id ?? "",
      icon: "",
    },
  })

  // Same shape as LocationFormDialog. Three reset triggers:
  //   1. open false → true.
  //   2. editing target id changes while open (route-param swap on
  //      `/areas/:id/edit` reuses the dialog instance).
  //   3. first prop with data after open (deep-link route mounts
  //      before useArea resolves).
  // Same-id reference churn (optimistic patch + rollback + onSettled
  // refetch) is ignored so the inline error survives. In create mode
  // there's no target id; we mark "prefilled" on the open transition
  // so subsequent prop-renders don't reset the user's typing.
  const wasOpenRef = useRef(false)
  const prefilledIdRef = useRef<string | undefined>(undefined)
  const createPrefilledRef = useRef(false)
  useEffect(() => {
    if (!open) {
      wasOpenRef.current = false
      prefilledIdRef.current = undefined
      createPrefilledRef.current = false
      return
    }
    const justOpened = !wasOpenRef.current
    const targetId = area?.id
    const targetChanged = targetId !== undefined && targetId !== prefilledIdRef.current
    const needCreateInit = !isEdit && !createPrefilledRef.current
    if (justOpened || targetChanged || needCreateInit) {
      form.reset({
        name: area?.name ?? "",
        location_id: area?.location_id ?? defaultLocationId ?? locations[0]?.id ?? "",
        icon: area?.icon ?? "",
      })
      setServerError(null)
      if (targetId !== undefined) prefilledIdRef.current = targetId
      if (!isEdit) createPrefilledRef.current = true
    }
    wasOpenRef.current = true
  }, [open, area, defaultLocationId, locations, form, isEdit])

  async function handle(values: AreaFormInput) {
    setServerError(null)
    try {
      await onSubmit(values)
      onOpenChange(false)
    } catch (err) {
      setServerError(parseServerError(err, t("locations:areaDialog.errorGeneric")))
    }
  }

  // Hide the parent picker when a single location exists — the value
  // is already forced in defaults and surfacing a one-option `<select>`
  // is just visual noise.
  const showLocationPicker = locations.length > 1

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md" data-testid="area-form-dialog">
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t("locations:areaDialog.editTitle") : t("locations:areaDialog.createTitle")}
          </DialogTitle>
          <DialogDescription>
            {isEdit
              ? t("locations:areaDialog.editDescription")
              : t("locations:areaDialog.createDescription")}
          </DialogDescription>
        </DialogHeader>

        <form
          id="area-form"
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
                  icons={AREA_ICONS}
                  label={t("locations:areaDialog.iconLabel")}
                  testIdPrefix="area-icon-picker"
                  disabled={isPending}
                />
              )}
            />
            {form.formState.errors.icon ? (
              <p className="text-xs text-destructive" data-testid="area-icon-error">
                {t(form.formState.errors.icon.message ?? "")}
              </p>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="area-name">
              {t("locations:areaDialog.nameLabel")}
              <span className="ms-0.5 text-destructive">*</span>
            </Label>
            <Input
              id="area-name"
              placeholder={t("locations:areaDialog.namePlaceholder")}
              autoComplete="off"
              maxLength={200}
              disabled={isPending}
              aria-invalid={!!form.formState.errors.name}
              data-testid="area-name-input"
              {...form.register("name")}
            />
            {form.formState.errors.name ? (
              <p className="text-xs text-destructive" data-testid="area-name-error">
                {t(form.formState.errors.name.message ?? "")}
              </p>
            ) : null}
          </div>

          {showLocationPicker ? (
            <div className="space-y-1.5">
              <Label htmlFor="area-location">{t("locations:areaDialog.locationLabel")}</Label>
              <Controller
                control={form.control}
                name="location_id"
                render={({ field }) => (
                  // Spread `field` so name / ref / onBlur land on the
                  // <select> too — without ref the form's focus-on-error
                  // logic targets nothing, and without onBlur RHF's
                  // touched/dirty tracking goes stale on this control.
                  <select
                    {...field}
                    id="area-location"
                    className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={isPending}
                    aria-invalid={!!form.formState.errors.location_id}
                    data-testid="area-location-select"
                  >
                    {locations.map((l) => (
                      <option key={l.id} value={l.id}>
                        {l.name}
                      </option>
                    ))}
                  </select>
                )}
              />
              {form.formState.errors.location_id ? (
                <p className="text-xs text-destructive" data-testid="area-location-error">
                  {t(form.formState.errors.location_id.message ?? "")}
                </p>
              ) : null}
            </div>
          ) : null}

          {serverError ? (
            <Alert variant="destructive" data-testid="area-form-server-error">
              <AlertDescription>{serverError}</AlertDescription>
            </Alert>
          ) : null}

          {/* DialogFooter must stay INSIDE the form. See the matching note in
              LocationFormDialog: webkit-macos drops the form-submission event
              when a `type="submit"` button binds to its form via the external
              `form="..."` attribute inside a Radix Dialog Portal. Keeping
              the button in-form lets the native submit flow fire. */}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isPending}
              data-testid="area-form-cancel"
            >
              {t("common:actions.cancel")}
            </Button>
            <Button type="submit" disabled={isPending} data-testid="area-form-submit">
              {isPending
                ? t("locations:areaDialog.submitting")
                : isEdit
                  ? t("locations:areaDialog.editSubmit")
                  : t("locations:areaDialog.createSubmit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
