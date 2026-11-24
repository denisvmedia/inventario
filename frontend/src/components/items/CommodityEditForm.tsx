import { useEffect, useMemo, useState, type ReactNode } from "react"
import { Controller, useForm, type FieldErrors } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { ArrowLeft, ChevronRight, ListChecks } from "lucide-react"

import { CurrencyCombobox } from "@/components/CurrencyCombobox"
import { ListPager } from "@/components/ListPager"
import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { TagPalette } from "@/components/tags/TagPalette"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Textarea } from "@/components/ui/textarea"
import type { Commodity, UpdateCommodityRequest } from "@/features/commodities/api"
import {
  COMMODITY_TYPES,
  COMMODITY_TYPE_ICONS,
  type CommodityTypeValue,
} from "@/features/commodities/constants"
import { buildDefaults, toRequest } from "@/features/commodities/draft"
import { buildCommoditySchema, type CommodityFormInput } from "@/features/commodities/schemas"
import { useAppToast } from "@/hooks/useAppToast"
import type { ListNavigation, ListNeighbor } from "@/hooks/useListNavigation"
import { useUnsavedChangesGuard } from "@/hooks/useUnsavedChangesGuard"
import { currencyMeta, priceInputPaddingClass } from "@/lib/currency-meta"
import { applyServerFieldErrors } from "@/lib/form-errors"
import { classifyServerError, type ClassifiedServerError } from "@/lib/server-error"
import { cn } from "@/lib/utils"
import { withId } from "@/lib/with-id"

export interface AreaOption {
  id?: string
  name?: string
  location_id?: string
}

export interface LocationOption {
  id?: string
  name?: string
}

// The fields this form shows. The rest of the record (links, extra serial
// numbers, part numbers, status, draft) is edited in the step-by-step dialog
// that "All fields" opens.
const FORM_FIELDS = [
  "name",
  "short_name",
  "count",
  "type",
  "serial_number",
  "area_id",
  "purchase_date",
  "original_price",
  "original_price_currency",
  "converted_original_price",
  "current_price",
  "warranty_expires_at",
  "warranty_notes",
  "comments",
  "tags",
] as const satisfies readonly (keyof CommodityFormInput)[]

export interface CommodityEditFormProps {
  commodity: Commodity
  areas: AreaOption[]
  locations: LocationOption[]
  groupCurrency: string
  /** Saves the record; rejects with the server's error. */
  onSubmit: (req: UpdateCommodityRequest) => Promise<unknown>
  isPending?: boolean
  /** The list the item was opened from; inactive when there is none. */
  nav: ListNavigation
  hrefFor: (target: ListNeighbor) => string
  /** Leaves the form for the item's read view. */
  onExit: () => void
  /** Moves to another item of the list, in its edit form. */
  onGoTo: (target: ListNeighbor) => void
  /** Opens the step-by-step dialog with the values entered so far. */
  onAllFields: (draft: Commodity) => void
  className?: string
  /** Extra classes for the sticky header, e.g. room for a sheet's close button. */
  headerClassName?: string
}

// The item's edit form, in place of the read view: the mock's ItemEditForm
// (design-mocks ItemDetail.tsx) with its editing header. Sections Identity,
// Location, Purchase, Warranty, Notes & Tags; Cancel and Save Changes stay at
// the bottom of the view while it scrolls.
//
// Opened from a list, the header also holds Previous / Next and the footer
// "Save and next", which follow the list's filters and order
// (hooks/useListNavigation.ts). Leaving with unsaved edits asks first.
export function CommodityEditForm({
  commodity,
  areas,
  locations,
  groupCurrency,
  onSubmit,
  isPending,
  nav,
  hrefFor,
  onExit,
  onGoTo,
  onAllFields,
  className,
  headerClassName,
}: CommodityEditFormProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const [serverError, setServerError] = useState<ClassifiedServerError | null>(null)
  // A failing field this form does not show; only the dialog can fix it.
  const [hiddenFieldError, setHiddenFieldError] = useState(false)

  const defaults = useMemo(
    () => buildDefaults(commodity, groupCurrency),
    [commodity, groupCurrency]
  )
  const schema = useMemo(() => buildCommoditySchema(groupCurrency), [groupCurrency])
  const form = useForm<CommodityFormInput>({
    resolver: zodResolver(schema),
    defaultValues: defaults,
    mode: "onBlur",
  })
  const {
    register,
    control,
    watch,
    setValue,
    getValues,
    formState: { errors, isDirty },
  } = form

  async function persist(values: CommodityFormInput): Promise<boolean> {
    setServerError(null)
    setHiddenFieldError(false)
    try {
      // Spread over the record so what the form does not cover (status
      // metadata, cover) goes back unchanged.
      await onSubmit({ ...commodity, ...toRequest(values, groupCurrency) })
      form.reset(values)
      toast.success(t("commodities:toast.updated"))
      return true
    } catch (err) {
      const result = applyServerFieldErrors(err, form.setError, { fields: FORM_FIELDS })
      const hidden = !!result && Object.keys(result.unmapped).length > 0
      setHiddenFieldError(hidden)
      // The banner is for a failure no field explains.
      if (!hidden && (!result || result.mapped.length === 0)) {
        setServerError(classifyServerError(err, t("commodities:form.serverError")))
      }
      return false
    }
  }

  function onInvalid(invalid: FieldErrors<CommodityFormInput>) {
    const shown = new Set<string>(FORM_FIELDS)
    setHiddenFieldError(Object.keys(invalid).some((field) => !shown.has(field)))
  }

  // Validates, then saves. Resolves false on a validation or server error;
  // the form keeps the entered values and shows the errors.
  async function save(): Promise<boolean> {
    let ok = false
    await form.handleSubmit(async (values) => {
      ok = await persist(values)
    }, onInvalid)()
    return ok
  }

  const { guard, dialog } = useUnsavedChangesGuard({ dirty: isDirty, onSave: save })

  async function handleSave() {
    if (!isDirty || (await save())) onExit()
  }

  async function handleSaveAndNext() {
    // The target is read before saving: an edit that takes this item out of
    // the list's filter must not change which item comes next.
    const target = nav.next
    if (!target) return
    if (isDirty && !(await save())) return
    onGoTo(target)
  }

  function openAllFields() {
    setHiddenFieldError(false)
    onAllFields({ ...commodity, ...toRequest(getValues(), groupCurrency) })
  }

  const count = Number(watch("count"))
  const isBundle = Number.isFinite(count) && count > 1
  const expiresAt = watch("warranty_expires_at")
  const warrantyNotes = watch("warranty_notes")
  const purchaseCurrency = watch("original_price_currency") ?? ""
  const isForeignCurrency =
    !!purchaseCurrency &&
    !!groupCurrency &&
    purchaseCurrency.toUpperCase() !== groupCurrency.toUpperCase()
  const purchaseSymbol = currencyMeta(purchaseCurrency || groupCurrency || "USD").symbol
  const groupSymbol = currencyMeta(groupCurrency || "USD").symbol

  return (
    <div className={cn("flex flex-col gap-0", className)} data-testid="commodity-edit-form">
      <div
        className={cn(
          "sticky top-0 z-10 mb-5 flex items-center gap-2 border-b border-border bg-background pt-5 pb-4",
          headerClassName
        )}
      >
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="-ml-1 size-8 shrink-0"
          onClick={() => guard(onExit)}
          aria-label={t("commodities:editForm.stopEditing")}
          data-testid="commodity-edit-back"
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
        </Button>
        <div className="min-w-0 flex-1">
          <p className="text-sm font-semibold leading-tight">{t("commodities:editForm.editing")}</p>
          <p className="truncate text-xs text-muted-foreground">{commodity.name}</p>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="shrink-0 gap-1.5"
          onClick={openAllFields}
          data-testid="commodity-edit-all-fields"
        >
          <ListChecks className="size-3.5" aria-hidden="true" />
          {t("commodities:editForm.allFields")}
        </Button>
        {nav.active ? (
          <ListPager
            nav={nav}
            hrefFor={hrefFor}
            onNavigate={(target) => guard(() => onGoTo(target))}
            labels={{
              pager: t("commodities:listNav.label"),
              previous: t("commodities:listNav.previous"),
              next: t("commodities:listNav.next"),
            }}
            testIdPrefix="commodity-edit"
            className="shrink-0"
          />
        ) : null}
      </div>

      {nav.active && !nav.loading && !nav.inList ? (
        <p
          className="-mt-2 mb-4 text-sm text-muted-foreground"
          data-testid="commodity-edit-not-in-list"
        >
          {t("commodities:listNav.notInList")}
        </p>
      ) : null}

      <form
        id="commodity-edit-form"
        onSubmit={(e) => {
          e.preventDefault()
          void handleSave()
        }}
        className="flex flex-col gap-5 py-2"
        noValidate
      >
        <div className="space-y-3">
          <SectionLabel>{t("commodities:editForm.sections.identity")}</SectionLabel>
          <EditField label={t("commodities:fields.name")} htmlFor="edit-name" error={errors.name}>
            <Input id="edit-name" {...register("name")} aria-invalid={!!errors.name} />
          </EditField>
          <div className="grid grid-cols-2 gap-3">
            <EditField
              label={t("commodities:fields.shortName")}
              htmlFor="edit-short-name"
              error={errors.short_name}
            >
              <Input
                id="edit-short-name"
                maxLength={40}
                className="font-mono text-sm"
                {...register("short_name")}
                aria-invalid={!!errors.short_name}
              />
            </EditField>
            <EditField
              label={t("commodities:fields.count")}
              htmlFor="edit-count"
              error={errors.count}
            >
              <Input
                id="edit-count"
                type="number"
                min={1}
                {...register("count")}
                aria-invalid={!!errors.count}
              />
            </EditField>
          </div>
          <EditField
            label={t("commodities:fields.type")}
            htmlFor="edit-category"
            error={errors.type}
          >
            <Controller
              control={control}
              name="type"
              render={({ field }) => (
                <Select value={field.value || undefined} onValueChange={field.onChange}>
                  <SelectTrigger id="edit-category" className="w-full" aria-invalid={!!errors.type}>
                    <SelectValue placeholder={t("commodities:fields.typePlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    {COMMODITY_TYPES.map((tp) => {
                      const Icon = COMMODITY_TYPE_ICONS[tp as CommodityTypeValue]
                      return (
                        <SelectItem key={tp} value={tp}>
                          <Icon aria-hidden="true" />
                          {t(`commodities:type.${tp}`)}
                        </SelectItem>
                      )
                    })}
                  </SelectContent>
                </Select>
              )}
            />
          </EditField>
          <EditField
            label={t("commodities:fields.serialNumber")}
            htmlFor="edit-serial"
            error={errors.serial_number}
          >
            <Input id="edit-serial" className="font-mono text-sm" {...register("serial_number")} />
          </EditField>
        </div>

        <Separator />

        <div className="space-y-3">
          <SectionLabel>{t("commodities:editForm.sections.location")}</SectionLabel>
          <LocationAreaFields
            areas={areas}
            locations={locations}
            areaId={watch("area_id") ?? ""}
            onAreaChange={(next) => setValue("area_id", next, { shouldDirty: true })}
            idPrefix="edit"
            areaError={errors.area_id}
          />
        </div>

        <Separator />

        <div className="space-y-3">
          <SectionLabel>{t("commodities:editForm.sections.purchase")}</SectionLabel>
          <EditField
            label={t("commodities:fields.purchaseDate")}
            htmlFor="edit-purchased"
            error={errors.purchase_date}
          >
            <Input
              id="edit-purchased"
              type="date"
              {...register("purchase_date")}
              aria-invalid={!!errors.purchase_date}
            />
          </EditField>
          <div className="grid grid-cols-2 gap-3">
            <EditField
              label={t("commodities:fields.originalPrice")}
              htmlFor="edit-price"
              error={errors.original_price ?? errors.original_price_currency}
            >
              <div className="flex gap-2">
                <PriceInput
                  id="edit-price"
                  symbol={purchaseSymbol}
                  invalid={!!errors.original_price}
                  {...register("original_price")}
                />
                <Controller
                  control={control}
                  name="original_price_currency"
                  render={({ field }) => (
                    <CurrencyCombobox
                      id="edit-currency"
                      value={field.value ?? ""}
                      onChange={field.onChange}
                      ariaInvalid={!!errors.original_price_currency}
                      variant="compact"
                    />
                  )}
                />
              </div>
            </EditField>
            <EditField
              label={t("commodities:fields.currentPrice")}
              htmlFor="edit-value"
              error={errors.current_price}
            >
              <PriceInput
                id="edit-value"
                symbol={groupSymbol}
                invalid={!!errors.current_price}
                {...register("current_price")}
              />
            </EditField>
          </div>
          {isForeignCurrency ? (
            <EditField
              label={t("commodities:fields.convertedOriginalPrice", { groupCurrency })}
              htmlFor="edit-converted-price"
              hint={t("commodities:fields.convertedOriginalPriceHint")}
              error={errors.converted_original_price}
            >
              <PriceInput
                id="edit-converted-price"
                symbol={groupSymbol}
                invalid={!!errors.converted_original_price}
                {...register("converted_original_price")}
              />
            </EditField>
          ) : null}
        </div>

        <Separator />

        <div className="space-y-3">
          <SectionLabel>{t("commodities:editForm.sections.warranty")}</SectionLabel>
          {/* A bundle (count > 1) carries no warranty (#1554). An empty
              field is disabled; a filled one stays editable so legacy data
              can be cleared. */}
          <EditField
            label={t("commodities:editForm.warrantyExpiry")}
            htmlFor="edit-warranty-exp"
            hint={
              isBundle
                ? t("commodities:trackingRestrictions.warrantyStepHint")
                : t("commodities:editForm.warrantyExpiryHint")
            }
            error={errors.warranty_expires_at}
          >
            <Input
              id="edit-warranty-exp"
              type="date"
              disabled={isBundle && !expiresAt?.trim()}
              {...register("warranty_expires_at")}
              aria-invalid={!!errors.warranty_expires_at}
            />
          </EditField>
          <EditField
            label={t("commodities:fields.warrantyNotes")}
            htmlFor="edit-warranty-notes"
            error={errors.warranty_notes}
          >
            <Textarea
              id="edit-warranty-notes"
              rows={2}
              className="resize-none"
              disabled={isBundle && !warrantyNotes?.trim()}
              {...register("warranty_notes")}
            />
          </EditField>
        </div>

        <Separator />

        <div className="space-y-3">
          <SectionLabel>{t("commodities:editForm.sections.notesTags")}</SectionLabel>
          <EditField
            label={t("commodities:fields.comments")}
            htmlFor="edit-notes"
            error={errors.comments}
          >
            <Textarea
              id="edit-notes"
              rows={3}
              className="resize-none"
              {...register("comments")}
              aria-invalid={!!errors.comments}
            />
          </EditField>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="edit-tags-input">{t("commodities:fields.tags")}</Label>
            <TagPalette
              kind="commodity"
              values={watch("tags") ?? []}
              onChange={(next) => setValue("tags", next, { shouldDirty: true })}
              inputId="edit-tags-input"
              testId="commodity-edit-tags"
            />
          </div>
        </div>

        {hiddenFieldError ? (
          <Alert variant="destructive" data-testid="commodity-edit-hidden-error">
            <AlertDescription className="flex flex-wrap items-center gap-2">
              {t("commodities:editForm.hiddenFieldError")}
              <Button type="button" variant="outline" size="sm" onClick={openAllFields}>
                {t("commodities:editForm.allFields")}
              </Button>
            </AlertDescription>
          </Alert>
        ) : null}

        <ServerErrorBanner
          error={serverError}
          onRetry={() => void save()}
          isRetrying={!!isPending}
          testId="commodity-edit-error"
        />
      </form>

      <div className="sticky bottom-0 z-10 flex flex-col gap-5 bg-background pt-5">
        <Separator />
        <div className="flex gap-2 pb-2">
          <Button
            type="button"
            variant="outline"
            className="flex-1"
            onClick={() => guard(onExit)}
            data-testid="commodity-edit-cancel"
          >
            {t("common:actions.cancel")}
          </Button>
          <Button
            type="submit"
            form="commodity-edit-form"
            variant={nav.active ? "outline" : "default"}
            className="flex-1"
            disabled={isPending}
            data-testid="commodity-edit-save"
          >
            {t("commodities:editForm.save")}
          </Button>
          {nav.active ? (
            <Button
              type="button"
              className="flex-1 gap-1"
              onClick={handleSaveAndNext}
              disabled={isPending || !nav.next}
              data-testid="commodity-edit-save-next"
            >
              {t("commodities:editForm.saveAndNext")}
              <ChevronRight className="size-4" aria-hidden="true" />
            </Button>
          ) : null}
        </div>
      </div>

      {dialog}
    </div>
  )
}

function SectionLabel({ children }: { children: ReactNode }) {
  return (
    <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
      {children}
    </p>
  )
}

// The mock's `F` field: label, control, optional hint. An error replaces the
// hint.
export function EditField({
  label,
  htmlFor,
  hint,
  error,
  children,
}: {
  label: string
  htmlFor: string
  hint?: string
  error?: { message?: string }
  children: ReactNode
}) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={htmlFor} className="text-sm font-medium">
        {label}
      </Label>
      {children}
      {error?.message ? (
        <p className="text-xs text-destructive" role="alert">
          {t(error.message)}
        </p>
      ) : hint ? (
        <p className="text-xs text-muted-foreground">{hint}</p>
      ) : null}
    </div>
  )
}

function PriceInput({
  symbol,
  invalid,
  ...props
}: React.ComponentProps<"input"> & { symbol: string; invalid?: boolean }) {
  return (
    <div className="relative flex-1">
      <span
        aria-hidden="true"
        className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 select-none text-sm text-muted-foreground"
      >
        {symbol}
      </span>
      <Input
        type="number"
        min={0}
        step="0.01"
        className={priceInputPaddingClass(symbol)}
        aria-invalid={invalid}
        {...props}
      />
    </div>
  )
}

export interface LocationAreaFieldsProps {
  areas: AreaOption[]
  locations: LocationOption[]
  areaId: string
  onAreaChange: (areaId: string) => void
  /** The selects get ids `<prefix>-location` and `<prefix>-area`. */
  idPrefix: string
  areaError?: { message?: string }
}

// The mock's paired Location / Area selects. Only the area is stored; the
// location follows from it and narrows the areas offered. Picking another
// location clears the area, and "Clear location" leaves the item unassigned.
export function LocationAreaFields({
  areas,
  locations,
  areaId,
  onAreaChange,
  idPrefix,
  areaError,
}: LocationAreaFieldsProps) {
  const { t } = useTranslation()
  const locationOfArea = (id: string) => areas.find((a) => a.id === id)?.location_id ?? ""
  const [locationId, setLocationId] = useState(() => locationOfArea(areaId))
  // The areas can arrive after the first render; fill the location in once
  // they do, unless the user has picked one already.
  useEffect(() => {
    if (locationId || !areaId) return
    const found = areas.find((a) => a.id === areaId)?.location_id
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time back-fill once the areas query resolves; a location the user picked is left alone.
    if (found) setLocationId(found)
  }, [areas, areaId, locationId])
  const visibleAreas = locationId ? areas.filter((a) => a.location_id === locationId) : []

  function pickLocation(next: string) {
    if (next === locationId) return
    setLocationId(next)
    onAreaChange("")
  }

  return (
    <div className="grid grid-cols-2 gap-3">
      <div className="flex flex-col gap-1.5">
        <div className="flex items-center justify-between gap-2">
          <Label htmlFor={`${idPrefix}-location`} className="text-sm font-medium">
            {t("commodities:fields.location")}
          </Label>
          {locationId ? (
            <button
              type="button"
              onClick={() => pickLocation("")}
              className="text-xs text-muted-foreground transition-colors hover:text-foreground"
              data-testid={`${idPrefix}-location-clear`}
            >
              {t("commodities:fields.clearLocation")}
            </button>
          ) : null}
        </div>
        <Select value={locationId || undefined} onValueChange={pickLocation}>
          <SelectTrigger id={`${idPrefix}-location`} className="w-full">
            <SelectValue placeholder={t("commodities:fields.locationPlaceholder")} />
          </SelectTrigger>
          <SelectContent>
            {withId(locations).map((l) => (
              <SelectItem key={l.id} value={l.id}>
                {l.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor={`${idPrefix}-area`} className="text-sm font-medium">
          {t("commodities:fields.area")}
        </Label>
        <Select
          // Remount on a location change so the trigger drops the old area's
          // label and shows the placeholder again.
          key={locationId || "no-location"}
          value={areaId || undefined}
          onValueChange={onAreaChange}
          disabled={!locationId}
        >
          <SelectTrigger id={`${idPrefix}-area`} className="w-full" aria-invalid={!!areaError}>
            <SelectValue
              placeholder={
                locationId
                  ? t("commodities:fields.areaPlaceholder")
                  : t("commodities:fields.areaPlaceholderNoLocation")
              }
            />
          </SelectTrigger>
          <SelectContent>
            {withId(visibleAreas).map((a) => (
              <SelectItem key={a.id} value={a.id}>
                {a.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {areaError?.message ? (
          <p className="text-xs text-destructive" role="alert">
            {t(areaError.message)}
          </p>
        ) : null}
      </div>
    </div>
  )
}
