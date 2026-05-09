import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import {
  AlertTriangle,
  Camera,
  ChevronLeft,
  ChevronRight,
  Plus,
  ScanText,
  Sparkles,
  X,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
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
import { Separator } from "@/components/ui/separator"
import { ComingSoonBanner } from "@/components/coming-soon/ComingSoonBanner"
import {
  COMMODITY_STATUSES,
  COMMODITY_TYPES,
  COMMODITY_TYPE_ICONS,
  warrantyStatus,
  type CommodityStatusValue,
  type CommodityTypeValue,
} from "@/features/commodities/constants"
import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import { commoditySchema, type CommodityFormInput } from "@/features/commodities/schemas"
import type {
  Commodity,
  CreateCommodityRequest,
  UpdateCommodityRequest,
} from "@/features/commodities/api"
import { cn } from "@/lib/utils"

interface AreaOption {
  id?: string
  name?: string
  location_id?: string
}

interface CommodityFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // "create" hides the status field (defaults to in_use), "edit"
  // surfaces it so the user can roll over a commodity to sold/lost/etc.
  mode: "create" | "edit"
  // Edit mode prefills from this object — `id` is preserved by the
  // caller, the dialog only consumes the attributes.
  initialValues?: Commodity
  areas: AreaOption[]
  defaultCurrency: string
  onSubmit: (values: CreateCommodityRequest & UpdateCommodityRequest) => Promise<void>
  isPending?: boolean
  // Stable localStorage key used to auto-save the form draft (per #1383).
  // The dialog rehydrates from storage when opening in create mode and
  // clears storage on successful submit. Pass undefined to disable
  // persistence — typically tests do this so each case starts clean.
  draftKey?: string
}

// "ai" is a create-only placeholder step that surfaces the planned
// "Fill with AI" photo-scan affordance from the design mock; the real
// scanner is tracked in #1540. The step has no form fields, so Next
// just advances. Edit mode skips it (the wizard restarts at Basics).
const ALL_STEPS = ["ai", "basics", "purchase", "warranty", "extras", "files"] as const
type StepKey = (typeof ALL_STEPS)[number]

// Per-step field allow-list — used by the Next button to validate only
// the current step's fields before advancing. Validating the whole form
// would block the user on fields they haven't seen yet. Warranty
// shipped under #1367 (date + notes); Files still renders a
// ComingSoonBanner because the unified Files surface lands in
// #1398/#1399.
const STEP_FIELDS: Record<StepKey, (keyof CommodityFormInput)[]> = {
  ai: [],
  basics: ["name", "short_name", "type", "area_id", "status", "count", "draft"],
  purchase: [
    "purchase_date",
    "original_price",
    "original_price_currency",
    "converted_original_price",
    "current_price",
    "serial_number",
  ],
  warranty: ["warranty_expires_at", "warranty_notes"],
  extras: ["tags", "comments", "urls", "extra_serial_numbers", "part_numbers"],
  files: [],
}

// CommodityFormDialog hosts the multi-step Add Item / Edit form. Five
// steps mirror the design mock: Basics → Purchase → Warranty → Extras
// → Files. Files still renders a `ComingSoonBanner` because the unified
// Files surface (#1398/#1399) hasn't shipped; the user can step through
// it with Next and submit the rest of the form. Warranty shipped under
// #1367.
//
// The dialog uses react-hook-form for state + zod for validation.
// Per-step Next clicks call `trigger()` against the current step's
// fields only. On Save the dialog hands a fully-validated payload to
// the caller; the caller is responsible for closing the dialog on
// success.
export function CommodityFormDialog({
  open,
  onOpenChange,
  mode,
  initialValues,
  areas,
  defaultCurrency,
  onSubmit,
  isPending,
  draftKey,
}: CommodityFormDialogProps) {
  const { t } = useTranslation()
  // The AI step only renders in create mode; edit mode jumps straight
  // to Basics. The visible STEPS sequence drives both the stepper bar
  // and the Next/Back navigation, so deriving it from `mode` keeps
  // both surfaces consistent without a separate visibility flag.
  const STEPS = useMemo<readonly StepKey[]>(
    () => (mode === "create" ? ALL_STEPS : ALL_STEPS.filter((s) => s !== "ai")),
    [mode]
  )
  const initialStep: StepKey = mode === "create" ? "ai" : "basics"
  const [step, setStep] = useState<StepKey>(initialStep)
  const [serverError, setServerError] = useState<string | null>(null)
  // Drafts only persist for create mode — editing an existing item
  // never auto-saves to storage (the BE row is the canonical state).
  const persistDrafts = mode === "create" && !!draftKey

  const defaults = useMemo<CommodityFormInput>(
    () => buildDefaults(initialValues, defaultCurrency),
    [initialValues, defaultCurrency]
  )

  const form = useForm<CommodityFormInput>({
    resolver: zodResolver(commoditySchema),
    defaultValues: defaults,
    mode: "onBlur",
  })
  const {
    register,
    control,
    handleSubmit,
    formState: { errors },
    trigger,
    reset,
    setValue,
    watch,
  } = form

  // Reset to defaults whenever the dialog opens. In create mode we try
  // to rehydrate from the localStorage draft key first (per #1383) — if
  // the user partially filled the form on a previous visit, the values
  // come back. Otherwise we fall through to the static defaults.
  useEffect(() => {
    if (!open) return
    let starting = defaults
    if (persistDrafts && draftKey) {
      const restored = readDraft(draftKey)
      if (restored) {
        starting = { ...defaults, ...restored }
      }
    }
    reset(starting)
    setStep(initialStep)
    setServerError(null)
  }, [open, defaults, reset, persistDrafts, draftKey, initialStep])

  // Auto-save the form to localStorage on every change while the dialog
  // is open in create mode. Debounced to a single rAF tick so a burst
  // of typing doesn't write to storage on every keystroke.
  useEffect(() => {
    if (!open || !persistDrafts || !draftKey) return
    const subscription = watch((values) => {
      const id = window.requestAnimationFrame(() => writeDraft(draftKey, values))
      return () => window.cancelAnimationFrame(id)
    })
    return () => subscription.unsubscribe()
  }, [open, persistDrafts, draftKey, watch])

  function discardDraft() {
    if (draftKey) clearDraft(draftKey)
    reset(defaults)
    // Drop the user on Basics rather than the AI offer step. Discard
    // is only reachable on form steps (the AI step's footer doesn't
    // include it), so the user already chose to fill the form
    // manually — sending them back through the AI entry point would
    // be surprising. The AI step is only revisited on a fresh dialog
    // open.
    setStep("basics")
  }

  async function nextStep() {
    const fields = STEP_FIELDS[step]
    const ok = await trigger(fields, { shouldFocus: true })
    if (!ok) return
    const idx = STEPS.indexOf(step)
    if (idx < STEPS.length - 1) setStep(STEPS[idx + 1])
  }
  function prevStep() {
    const idx = STEPS.indexOf(step)
    if (idx > 0) setStep(STEPS[idx - 1])
  }

  const submit = async (values: CommodityFormInput) => {
    setServerError(null)
    try {
      await onSubmit(toRequest(values))
      // Submitted successfully — drop the draft so a fresh dialog open
      // doesn't replay yesterday's data.
      if (persistDrafts && draftKey) clearDraft(draftKey)
    } catch (err) {
      setServerError(err instanceof Error ? err.message : t("commodities:form.serverError"))
    }
  }

  const stepIndex = STEPS.indexOf(step)
  const isLastStep = stepIndex === STEPS.length - 1
  // #1554: a count > 1 row is a bundle of identical units and can't
  // carry warranty / loan / service. Watching the count value lets the
  // banner show up live as soon as the user types, and lets the
  // warranty step disable its inputs without waiting for a re-render.
  const liveCount = Number(watch("count"))
  const isBundle = Number.isFinite(liveCount) && liveCount > 1

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/* `max-h-[90vh] overflow-y-auto` keeps the whole dialog scrollable
          inside the viewport. Without it the centered-translate
          positioning lets a tall variant (e.g. the #1554 bundle banner +
          the 5-step wizard combined) push the footer below the visible
          viewport on small CI viewports, and Playwright's actionability
          check refuses to click an off-viewport Next button. */}
      <DialogContent
        className="max-w-2xl max-h-[90vh] overflow-y-auto"
        data-testid="commodity-form-dialog"
      >
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {step === "ai" ? (
              <>
                <Sparkles aria-hidden="true" className="size-4 text-amber-500" />
                {t("commodities:form.step.ai.title")}
              </>
            ) : mode === "create" ? (
              t("commodities:form.createTitle")
            ) : (
              t("commodities:form.editTitle")
            )}
          </DialogTitle>
          <DialogDescription>{t(`commodities:form.step.${step}.description`)}</DialogDescription>
        </DialogHeader>

        {/* Stepper hidden on the AI step to mirror the mock's
            AddItemDialog (L534 `{isFormStep && (...)}`) — the offer
            phase shows just title + body + footer, no progress chrome.
            On form steps the numbered stepper renders as before. */}
        {step !== "ai" ? (
          <>
            <ol
              className="flex items-center gap-2 text-xs text-muted-foreground"
              aria-label={t("commodities:form.stepperLabel")}
            >
              {STEPS.map((s, i) => (
                <li key={s} className="flex items-center gap-2">
                  <span
                    className={cn(
                      "size-5 rounded-full border text-center leading-[18px]",
                      i === stepIndex
                        ? "border-primary text-primary font-medium"
                        : i < stepIndex
                          ? "border-primary/40 bg-primary/10 text-primary"
                          : "border-border"
                    )}
                    aria-current={i === stepIndex ? "step" : undefined}
                  >
                    {i + 1}
                  </span>
                  <span className={cn(i === stepIndex && "font-medium text-foreground")}>
                    {t(`commodities:form.step.${s}.title`)}
                  </span>
                  {i < STEPS.length - 1 ? (
                    <ChevronRight className="size-3" aria-hidden="true" />
                  ) : null}
                </li>
              ))}
            </ol>

            <Separator />
          </>
        ) : null}

        {isBundle ? (
          <Alert
            variant="default"
            className="border-amber-300 bg-amber-50 text-amber-900 dark:bg-amber-950/30"
            data-testid="commodity-form-bundle-banner"
          >
            <AlertTriangle className="size-4" aria-hidden="true" />
            <AlertTitle>{t("commodities:trackingRestrictions.bannerTitle")}</AlertTitle>
            <AlertDescription>{t("commodities:trackingRestrictions.bannerBody")}</AlertDescription>
          </Alert>
        ) : null}

        <form
          id="commodity-form"
          onSubmit={handleSubmit(submit)}
          className="flex flex-col gap-4"
          noValidate
        >
          {step === "ai" ? <AiStep /> : null}
          {step === "basics" ? (
            <BasicsStep
              register={register}
              control={control}
              errors={errors}
              watch={watch}
              setValue={setValue}
              areas={areas}
              showStatus={mode === "edit"}
            />
          ) : null}
          {step === "purchase" ? (
            <PurchaseStep register={register} errors={errors} watch={watch} />
          ) : null}
          {step === "warranty" ? (
            <WarrantyStep register={register} errors={errors} watch={watch} isBundle={isBundle} />
          ) : null}
          {step === "extras" ? (
            <ExtrasStep register={register} errors={errors} watch={watch} setValue={setValue} />
          ) : null}
          {step === "files" ? <FilesStep /> : null}

          {serverError ? (
            <p className="text-sm text-destructive" data-testid="commodity-form-error">
              {serverError}
            </p>
          ) : null}
        </form>

        {step === "ai" ? (
          // AI-step footer mirrors the mock (AddItemDialog L657-L674):
          // Cancel (ghost, mr-auto) | Fill manually (outline) |
          // Scan photos (primary, Sparkles icon, disabled until at
          // least one photo is attached). Scanner backend is in #1540
          // so "Scan photos" stays disabled here.
          <DialogFooter className="gap-2">
            <Button
              type="button"
              variant="ghost"
              className="mr-auto"
              onClick={() => onOpenChange(false)}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={nextStep}
              data-testid="commodity-form-next"
            >
              {t("commodities:form.fillManually")}
            </Button>
            <Button
              type="button"
              disabled
              className="gap-1.5"
              title={t("commodities:form.step.ai.scanDisabledTitle")}
              data-testid="commodity-form-ai-scan"
            >
              <Sparkles aria-hidden="true" className="size-3.5" />
              {t("commodities:form.step.ai.scanPhotos")}
            </Button>
          </DialogFooter>
        ) : (
          <DialogFooter className="gap-2 sm:justify-between">
            <div className="flex items-center gap-2">
              <Button type="button" variant="ghost" onClick={prevStep} disabled={stepIndex === 0}>
                <ChevronLeft className="size-4" aria-hidden="true" />
                {t("commodities:form.back")}
              </Button>
              {persistDrafts ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={discardDraft}
                  data-testid="commodity-form-discard-draft"
                >
                  {t("commodities:form.discardDraft")}
                </Button>
              ) : null}
            </div>
            <div className="flex items-center gap-2">
              {isLastStep ? (
                <Button
                  type="submit"
                  form="commodity-form"
                  disabled={isPending}
                  data-testid="commodity-form-submit"
                >
                  {mode === "create"
                    ? t("commodities:form.submitCreate")
                    : t("commodities:form.submitEdit")}
                </Button>
              ) : (
                <Button type="button" onClick={nextStep} data-testid="commodity-form-next">
                  {t("commodities:form.next")}
                  <ChevronRight className="size-4" aria-hidden="true" />
                </Button>
              )}
            </div>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}

// ---- Step 1: Basics -----------------------------------------------------

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- RHF types thread generics through every helper; concrete types here are noisy.
function BasicsStep(props: any) {
  const { t } = useTranslation()
  const { register, control, errors, areas, showStatus } = props
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div className="sm:col-span-2 flex flex-col gap-1.5">
        <Label htmlFor="commodity-name">{t("commodities:fields.name")}</Label>
        <Input id="commodity-name" {...register("name")} aria-invalid={!!errors.name} />
        <FieldError error={errors.name} />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-short-name">{t("commodities:fields.shortName")}</Label>
        <Input
          id="commodity-short-name"
          {...register("short_name")}
          aria-invalid={!!errors.short_name}
        />
        <FieldError error={errors.short_name} />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-count">{t("commodities:fields.count")}</Label>
        <Input
          id="commodity-count"
          type="number"
          min={1}
          {...register("count")}
          aria-invalid={!!errors.count}
        />
        <FieldError error={errors.count} />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-type">{t("commodities:fields.type")}</Label>
        <Controller
          control={control}
          name="type"
          render={({ field }) => (
            <select
              id="commodity-type"
              value={field.value}
              onChange={field.onChange}
              className="border-input bg-transparent rounded-md border px-3 py-2 text-sm"
              aria-invalid={!!errors.type}
            >
              <option value="">{t("commodities:fields.typePlaceholder")}</option>
              {COMMODITY_TYPES.map((tp) => (
                <option key={tp} value={tp}>
                  {COMMODITY_TYPE_ICONS[tp as CommodityTypeValue]} {t(`commodities:type.${tp}`)}
                </option>
              ))}
            </select>
          )}
        />
        <FieldError error={errors.type} />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-area">{t("commodities:fields.area")}</Label>
        <Controller
          control={control}
          name="area_id"
          render={({ field }) => (
            <select
              id="commodity-area"
              value={field.value}
              onChange={field.onChange}
              className="border-input bg-transparent rounded-md border px-3 py-2 text-sm"
              aria-invalid={!!errors.area_id}
            >
              <option value="">{t("commodities:fields.areaPlaceholder")}</option>
              {(areas as AreaOption[]).map((a) => (
                <option key={a.id} value={a.id ?? ""}>
                  {a.name}
                </option>
              ))}
            </select>
          )}
        />
        <FieldError error={errors.area_id} />
      </div>
      {showStatus ? (
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="commodity-status">{t("commodities:fields.status")}</Label>
          <Controller
            control={control}
            name="status"
            render={({ field }) => (
              <select
                id="commodity-status"
                value={field.value}
                onChange={field.onChange}
                className="border-input bg-transparent rounded-md border px-3 py-2 text-sm"
                aria-invalid={!!errors.status}
              >
                {COMMODITY_STATUSES.map((s) => (
                  <option key={s} value={s}>
                    {t(`commodities:status.${s}`)}
                  </option>
                ))}
              </select>
            )}
          />
          <FieldError error={errors.status} />
        </div>
      ) : null}
      <div className="sm:col-span-2 flex items-center gap-2">
        <Controller
          control={control}
          name="draft"
          render={({ field }) => (
            <Checkbox
              id="commodity-draft"
              checked={field.value}
              onCheckedChange={(v) => field.onChange(!!v)}
            />
          )}
        />
        <Label htmlFor="commodity-draft" className="text-sm font-normal">
          {t("commodities:fields.draft")}
        </Label>
      </div>
    </div>
  )
}

// ---- Step 2: Purchase ---------------------------------------------------

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- see BasicsStep
function PurchaseStep(props: any) {
  const { t } = useTranslation()
  const { register, errors } = props
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-purchase-date">{t("commodities:fields.purchaseDate")}</Label>
        <Input
          id="commodity-purchase-date"
          type="date"
          {...register("purchase_date")}
          aria-invalid={!!errors.purchase_date}
        />
        <FieldError error={errors.purchase_date} />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-original-price">{t("commodities:fields.originalPrice")}</Label>
        <Input
          id="commodity-original-price"
          type="number"
          step="0.01"
          min={0}
          {...register("original_price")}
          aria-invalid={!!errors.original_price}
        />
        <FieldError error={errors.original_price} />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-currency">{t("commodities:fields.currency")}</Label>
        <Input
          id="commodity-currency"
          maxLength={3}
          {...register("original_price_currency")}
          aria-invalid={!!errors.original_price_currency}
        />
        <FieldError error={errors.original_price_currency} />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-current-price">{t("commodities:fields.currentPrice")}</Label>
        <Input
          id="commodity-current-price"
          type="number"
          step="0.01"
          min={0}
          {...register("current_price")}
          aria-invalid={!!errors.current_price}
        />
        <FieldError error={errors.current_price} />
      </div>
      <div className="sm:col-span-2 flex flex-col gap-1.5">
        <Label htmlFor="commodity-converted-price">
          {t("commodities:fields.convertedOriginalPrice")}
        </Label>
        <Input
          id="commodity-converted-price"
          type="number"
          step="0.01"
          min={0}
          {...register("converted_original_price")}
          aria-invalid={!!errors.converted_original_price}
        />
        <p className="text-xs text-muted-foreground">
          {t("commodities:fields.convertedOriginalPriceHint")}
        </p>
      </div>
      <div className="sm:col-span-2 flex flex-col gap-1.5">
        <Label htmlFor="commodity-serial">{t("commodities:fields.serialNumber")}</Label>
        <Input id="commodity-serial" {...register("serial_number")} />
      </div>
    </div>
  )
}

// ---- Step 3: Warranty ---------------------------------------------------

// WarrantyStep renders the first-class warranty inputs (#1367):
// expiry date + notes. Status (active/expiring/expired/none) is
// computed live from the entered date and shown as a pill preview so
// the user sees how the row will surface on the list page before
// saving.
//
// On bundles (#1554, count > 1) the inputs are disabled ONLY when
// they're empty — i.e. there's nothing for the user to clean up. Per-
// field disabling lets a legacy bundle that already carries warranty
// data (the migration is log-only, so legacy rows pass through
// unmodified) be cleared from the UI. The same step always renders the
// "split into separate items" hint so the disabled state never looks
// like a bug.
//
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- see BasicsStep
function WarrantyStep(props: any) {
  const { t } = useTranslation()
  const { register, errors, watch, isBundle } = props
  const expiresAt = watch("warranty_expires_at") as string | undefined
  const notes = watch("warranty_notes") as string | undefined
  const status = warrantyStatusFromDate(expiresAt)
  // Only disable when the field is empty AND the row is a bundle. A
  // populated input stays editable so the user can clear / fix legacy
  // data that pre-dates the constraint.
  const expiresAtEmpty = !expiresAt || expiresAt.trim() === ""
  const notesEmpty = !notes || notes.trim() === ""
  const expiresAtDisabled = isBundle && expiresAtEmpty
  const notesDisabled = isBundle && notesEmpty
  return (
    <div className="flex flex-col gap-4" data-testid="commodity-form-warranty-step">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-warranty-expires-at">
          {t("commodities:fields.warrantyExpiresAt")}
        </Label>
        <Input
          id="commodity-warranty-expires-at"
          type="date"
          {...register("warranty_expires_at")}
          aria-invalid={!!errors.warranty_expires_at}
          disabled={expiresAtDisabled}
          data-testid="commodity-form-warranty-expires-at"
        />
        <p className="text-xs text-muted-foreground">
          {isBundle
            ? t("commodities:trackingRestrictions.warrantyStepHint")
            : t("commodities:fields.warrantyExpiresAtHelp")}
        </p>
        <FieldError error={errors.warranty_expires_at} />
      </div>
      {status !== "none" && !isBundle ? (
        <WarrantyBadge
          status={status}
          className="w-fit"
          data-testid="commodity-form-warranty-status"
        />
      ) : null}
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-warranty-notes">{t("commodities:fields.warrantyNotes")}</Label>
        <textarea
          id="commodity-warranty-notes"
          rows={3}
          {...register("warranty_notes")}
          className="border-input bg-transparent rounded-md border px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50"
          aria-invalid={!!errors.warranty_notes}
          disabled={notesDisabled}
          data-testid="commodity-form-warranty-notes"
        />
        <FieldError error={errors.warranty_notes} />
      </div>
    </div>
  )
}

// warrantyStatusFromDate is the live preview equivalent of the BE's
// ComputeWarrantyStatus — delegates to the shared `warrantyStatus()`
// helper so the form pill, the list-page pill, the BE filter, and
// the worker reminder cadence all agree on the 60-day boundary +
// UTC-midnight anchor. Returns "none" for empty / unparseable input
// so the preview block stays hidden.
function warrantyStatusFromDate(d: string | undefined) {
  return warrantyStatus({ warranty_expires_at: d })
}

// ---- Step 4: Extras -----------------------------------------------------

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- see BasicsStep
function ExtrasStep(props: any) {
  const { t } = useTranslation()
  const { register, errors, watch, setValue } = props
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-comments">{t("commodities:fields.comments")}</Label>
        <textarea
          id="commodity-comments"
          rows={3}
          {...register("comments")}
          className="border-input bg-transparent rounded-md border px-3 py-2 text-sm"
          aria-invalid={!!errors.comments}
        />
        <FieldError error={errors.comments} />
      </div>
      <ChipInput
        label={t("commodities:fields.tags")}
        helper={t("commodities:fields.tagsHelp")}
        values={watch("tags")}
        onChange={(next) => setValue("tags", next, { shouldDirty: true })}
        testId="commodity-tags"
      />
      <ChipInput
        label={t("commodities:fields.extraSerialNumbers")}
        helper={t("commodities:fields.extraSerialNumbersHelp")}
        values={watch("extra_serial_numbers")}
        onChange={(next) => setValue("extra_serial_numbers", next, { shouldDirty: true })}
        testId="commodity-extra-serials"
      />
      <ChipInput
        label={t("commodities:fields.partNumbers")}
        helper={t("commodities:fields.partNumbersHelp")}
        values={watch("part_numbers")}
        onChange={(next) => setValue("part_numbers", next, { shouldDirty: true })}
        testId="commodity-part-numbers"
      />
      <ChipInput
        label={t("commodities:fields.urls")}
        helper={t("commodities:fields.urlsHelp")}
        values={watch("urls")}
        onChange={(next) => setValue("urls", next, { shouldDirty: true })}
        testId="commodity-urls"
      />
    </div>
  )
}

// ---- Step 5: Files (stub) ----------------------------------------------

// ---- Step 0: Fill with AI (placeholder) ---------------------------------

// AiStep ports the design-mock `AiPhotoStep` "offer" phase 1:1 — see
// `design-mocks/src/components/AddItemDialog.tsx` L789-L856. Anatomy
// is identical: two photo-type cards (full-item / label) with the
// `bg-primary/10` icon tile, then the dashed dropzone with the
// `bg-amber-500/10` Sparkles tile and the "Drop photos here or
// browse" copy. The scanner backend (AI vision service + scanning /
// review phases) is tracked in #1540, so the inputs render inert
// here. A single muted line under the dropzone hint tags the surface
// as a preview and links to the tracker — that's the only deviation
// from the mock's offer phase. The wizard's Next button is relabelled
// "Fill manually" while on this step to mirror the mock footer copy.
function AiStep() {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col gap-4 py-2" data-testid="commodity-form-ai-step">
      <div className="grid grid-cols-2 gap-3">
        <div className="flex flex-col gap-2 rounded-xl border border-border bg-muted/20 p-3">
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary/10">
            <Camera className="size-4 text-primary" />
          </div>
          <p className="text-xs font-semibold">{t("commodities:form.step.ai.fullItem.title")}</p>
          <p className="text-[11px] leading-relaxed text-muted-foreground">
            {t("commodities:form.step.ai.fullItem.description")}
          </p>
        </div>
        <div className="flex flex-col gap-2 rounded-xl border border-border bg-muted/20 p-3">
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary/10">
            <ScanText className="size-4 text-primary" />
          </div>
          <p className="text-xs font-semibold">{t("commodities:form.step.ai.label.title")}</p>
          <p className="text-[11px] leading-relaxed text-muted-foreground">
            {t("commodities:form.step.ai.label.description")}
          </p>
        </div>
      </div>
      <div className="flex flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed border-border py-6">
        <div className="flex size-10 items-center justify-center rounded-xl bg-amber-500/10">
          <Sparkles className="size-5 text-amber-500" aria-hidden="true" />
        </div>
        <p className="text-sm text-muted-foreground">
          {t("commodities:form.step.ai.dropzone.primary")}
        </p>
        <p className="text-xs text-muted-foreground">
          {t("commodities:form.step.ai.dropzone.hint")}
        </p>
      </div>
      {/* Mirrors the mock's "Add at least one photo to enable AI
          scanning, or tap Fill manually below." hint placement
          (one muted line under the dropzone), repurposed as the
          tracker disclosure for #1540. */}
      <p
        className="text-center text-xs text-muted-foreground"
        data-testid="commodity-form-ai-coming-soon"
      >
        {t("commodities:form.step.ai.comingSoon")}{" "}
        <a
          href="https://github.com/denisvmedia/inventario/issues/1540"
          target="_blank"
          rel="noreferrer"
          className="font-medium underline underline-offset-2"
        >
          {t("commodities:form.step.ai.trackerLink")}
        </a>
      </p>
    </div>
  )
}

// FilesStep is a placeholder until the unified Files surface ships
// (#1398/#1399). Today commodity-scoped attachments still flow through
// the legacy `/commodities/{id}/{images,invoices,manuals}` routes —
// those are exposed on the detail page rather than here so the create
// flow stays simple. The user can attach files after the commodity
// exists.
function FilesStep() {
  return (
    <div className="flex flex-col gap-2" data-testid="commodity-form-files-step">
      <ComingSoonBanner surface="filesUnification" />
    </div>
  )
}

// ---- Helpers ------------------------------------------------------------

interface ChipInputProps {
  label: string
  helper?: string
  values: string[]
  onChange: (next: string[]) => void
  testId?: string
}

function ChipInput({ label, helper, values, onChange, testId }: ChipInputProps) {
  const [draft, setDraft] = useState("")
  function commit() {
    const trimmed = draft.trim()
    if (!trimmed) return
    if (values.includes(trimmed)) {
      setDraft("")
      return
    }
    onChange([...values, trimmed])
    setDraft("")
  }
  return (
    <div className="flex flex-col gap-1.5" data-testid={testId}>
      <Label>{label}</Label>
      <div className="flex flex-wrap items-center gap-1.5 rounded-md border border-input px-2 py-1.5">
        {values.map((v) => (
          <Badge
            key={v}
            variant="secondary"
            className="gap-1 h-5 px-1.5 text-xs"
            data-testid={`${testId}-chip`}
          >
            {v}
            <button
              type="button"
              className="ml-0.5 inline-flex items-center"
              aria-label={`remove ${v}`}
              onClick={() => onChange(values.filter((x) => x !== v))}
            >
              <X className="size-3" aria-hidden="true" />
            </button>
          </Badge>
        ))}
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === ",") {
              e.preventDefault()
              commit()
            } else if (e.key === "Backspace" && draft === "" && values.length > 0) {
              onChange(values.slice(0, -1))
            }
          }}
          onBlur={commit}
          className="flex-1 min-w-24 bg-transparent text-sm outline-none"
          data-testid={`${testId}-input`}
        />
        {draft.trim() ? (
          <button
            type="button"
            className="text-muted-foreground hover:text-foreground"
            aria-label="add"
            onClick={commit}
          >
            <Plus className="size-3.5" aria-hidden="true" />
          </button>
        ) : null}
      </div>
      {helper ? <p className="text-xs text-muted-foreground">{helper}</p> : null}
    </div>
  )
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- RHF FieldError generics noise
function FieldError({ error }: { error: any }) {
  const { t } = useTranslation()
  if (!error?.message) return null
  return (
    <p className="text-xs text-destructive" role="alert">
      {t(error.message)}
    </p>
  )
}

// ---- Draft persistence helpers ------------------------------------------

// readDraft pulls the previously-saved form values for `key` (per
// #1383). Returns undefined when nothing is stored or the JSON has
// rotted; callers fall back to defaults in either case.
function readDraft(key: string): Partial<CommodityFormInput> | undefined {
  if (typeof window === "undefined") return undefined
  try {
    const raw = window.localStorage.getItem(key)
    if (!raw) return undefined
    const parsed = JSON.parse(raw) as Partial<CommodityFormInput>
    return parsed
  } catch {
    return undefined
  }
}

function writeDraft(key: string, values: Partial<CommodityFormInput>): void {
  if (typeof window === "undefined") return
  try {
    window.localStorage.setItem(key, JSON.stringify(values))
  } catch {
    // Quota / private mode / disabled storage — drop silently. Drafts
    // are an enhancement, not a guarantee.
  }
}

function clearDraft(key: string): void {
  if (typeof window === "undefined") return
  try {
    window.localStorage.removeItem(key)
  } catch {
    // see writeDraft
  }
}

// buildDefaults populates the form with safe initial values. For edit
// mode it carries the existing record; for create mode the only
// pre-populated bits are the count (1), status (in_use), draft (false),
// and the group currency. Everything else stays empty so the
// user fills it in.
function buildDefaults(initial: Commodity | undefined, currency: string): CommodityFormInput {
  // `urls` is typed as `string` by openapi-typescript because the BE
  // model uses `swaggertype:"string"` on a JSONB column; at runtime it's
  // an array of strings. Coerce safely.
  const urls = Array.isArray(initial?.urls) ? (initial.urls as unknown as string[]) : []
  // Numeric defaults are kept as strings here (and throughout the
  // form) so the schema's input type stays string — see schemas.ts
  // for the rationale. They convert to JS numbers at submit time
  // inside toRequest.
  const numStr = (n: number | undefined): string => (n === undefined ? "" : String(n))
  return {
    name: initial?.name ?? "",
    short_name: initial?.short_name ?? "",
    type: (initial?.type as string) ?? "",
    area_id: initial?.area_id ?? "",
    status: (initial?.status as string) ?? "in_use",
    count: initial?.count !== undefined ? String(initial.count) : "1",
    original_price: numStr(initial?.original_price),
    original_price_currency: (initial?.original_price_currency as string) ?? currency,
    converted_original_price: numStr(initial?.converted_original_price),
    current_price: numStr(initial?.current_price),
    serial_number: initial?.serial_number ?? "",
    extra_serial_numbers: initial?.extra_serial_numbers ?? [],
    part_numbers: initial?.part_numbers ?? [],
    tags: initial?.tags ?? [],
    purchase_date: (initial?.purchase_date as string) ?? "",
    urls,
    comments: initial?.comments ?? "",
    draft: initial?.draft ?? false,
    warranty_expires_at: (initial?.warranty_expires_at as string) ?? "",
    warranty_notes: initial?.warranty_notes ?? "",
  }
}

// toRequest maps the validated form input into the BE-shaped envelope's
// attributes. Numbers come out of the form as strings (see schemas.ts);
// we convert here. `urls` flows through as string[] even though
// openapi-typescript types it as a single string (see buildDefaults).
function toRequest(values: CommodityFormInput): CreateCommodityRequest & UpdateCommodityRequest {
  const num = (v: string): number | undefined => (v === "" ? undefined : Number(v))
  // Date fields are PDate (pointer-to-Date) on the BE — `Date.UnmarshalJSON`
  // rejects empty strings as "cannot parse \"\" as \"2006\"". Omit the
  // field entirely when the input is blank so the BE sees a missing
  // value (decoded as nil pointer) rather than an invalid date string.
  const date = (v: string): string | undefined => {
    const trimmed = v.trim()
    return trimmed === "" ? undefined : trimmed
  }
  return {
    name: values.name.trim(),
    short_name: values.short_name.trim(),
    type: values.type as CommodityTypeValue,
    area_id: values.area_id,
    status: values.status as CommodityStatusValue,
    count: Number(values.count),
    original_price: num(values.original_price),
    original_price_currency: values.original_price_currency,
    converted_original_price: num(values.converted_original_price),
    current_price: num(values.current_price),
    serial_number: values.serial_number.trim(),
    extra_serial_numbers: values.extra_serial_numbers,
    part_numbers: values.part_numbers,
    tags: values.tags,
    purchase_date: date(values.purchase_date),
    urls: values.urls as unknown as string,
    comments: values.comments,
    draft: values.draft,
    warranty_expires_at: date(values.warranty_expires_at),
    warranty_notes: values.warranty_notes,
  }
}
