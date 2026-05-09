import { useEffect, useMemo, useRef, useState, type ReactNode } from "react"
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
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { HttpError } from "@/lib/http"
import { parseServerError } from "@/lib/server-error"
import { ComingSoonBanner } from "@/components/coming-soon/ComingSoonBanner"
import { CurrencyCombobox } from "@/components/CurrencyCombobox"
import { currencyMeta } from "@/lib/currency-meta"
import {
  COMMODITY_STATUSES,
  COMMODITY_TYPES,
  COMMODITY_TYPE_ICONS,
  warrantyStatus,
  type CommodityStatusValue,
  type CommodityTypeValue,
} from "@/features/commodities/constants"
import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import { buildCommoditySchema, type CommodityFormInput } from "@/features/commodities/schemas"
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

interface LocationOption {
  id?: string
  name?: string
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
  locations: LocationOption[]
  defaultCurrency: string
  onSubmit: (values: CreateCommodityRequest & UpdateCommodityRequest) => Promise<void>
  isPending?: boolean
  // Stable localStorage key used to auto-save the form draft (per #1383).
  // The dialog rehydrates from storage when opening in create mode and
  // clears storage on successful submit. Pass undefined to disable
  // persistence — typically tests do this so each case starts clean.
  draftKey?: string
}

// FORM_STEPS is the canonical 5-step sequence the numbered stepper
// renders. The "ai" surface (mock AddItemDialog `step === -1`) is an
// *alternative entry path*, not a step — it never appears in the
// numbered stepper. Create mode opens on "ai" and the user either
// scans photos (gated on #1540) or hits "Fill manually" to land on
// step 1 (Basics). Edit mode skips "ai" entirely.
const FORM_STEPS = ["basics", "purchase", "warranty", "extras", "files"] as const
type FormStepKey = (typeof FORM_STEPS)[number]
type StepKey = "ai" | FormStepKey

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
  locations,
  defaultCurrency,
  onSubmit,
  isPending,
  draftKey,
}: CommodityFormDialogProps) {
  const { t } = useTranslation()
  // Create mode opens on the AI offer surface; edit mode jumps
  // straight to Basics (no scanner needed when the row already
  // exists). The numbered stepper iterates `FORM_STEPS` only — AI
  // is an alternative entry path, not part of the linear sequence.
  const initialStep: StepKey = mode === "create" ? "ai" : "basics"
  const [step, setStep] = useState<StepKey>(initialStep)
  // Tracks which form steps the user has already landed on, so the
  // segmented stepper bar lets them click back-and-forth between
  // already-seen surfaces without forcing a strict forward walk.
  // Edit mode opens directly on Basics, so Basics is visited from
  // mount; Create mode opens on the AI surface and form steps get
  // added on first arrival.
  const [visitedSteps, setVisitedSteps] = useState<Set<FormStepKey>>(
    () => new Set(mode === "create" ? [] : ["basics"])
  )
  const [serverError, setServerError] = useState<string | null>(null)
  // Save-as-draft confirmation. Open when the user dismisses the
  // dialog (Escape, click-outside, X button, or the explicit Cancel
  // footer button) while the form is dirty in create mode — gives
  // them a chance to save the half-filled wizard as a draft instead
  // of losing it.
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false)
  // Tracks whether the user has manually typed into Current Value.
  // Until they do, the input live-mirrors Original Price in the
  // same-currency case (BE field-level Required on CurrentPrice
  // forces a non-zero value even though PriceRule says it's
  // optional — see #1625). Once the user edits the field, the
  // mirror stops; flipping back to "untouched" requires a fresh
  // dialog open.
  const currentPriceManualRef = useRef(false)
  // Drafts only persist for create mode — editing an existing item
  // never auto-saves to storage (the BE row is the canonical state).
  const persistDrafts = mode === "create" && !!draftKey

  const defaults = useMemo<CommodityFormInput>(
    () => buildDefaults(initialValues, defaultCurrency),
    [initialValues, defaultCurrency]
  )

  // Schema closes over group currency: when the purchase currency
  // matches the group's, `converted_original_price` isn't required
  // (the original price is already in group currency). Re-built when
  // defaultCurrency changes — typically on group switch.
  const schema = useMemo(() => buildCommoditySchema(defaultCurrency), [defaultCurrency])
  const form = useForm<CommodityFormInput>({
    resolver: zodResolver(schema),
    defaultValues: defaults,
    mode: "onBlur",
  })
  const {
    register,
    control,
    handleSubmit,
    formState: { errors, isDirty },
    trigger,
    reset,
    setValue,
    setError,
    watch,
    getValues,
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
    // Edit mode opens with an existing current_price; treat that as
    // already user-set so the mirror never overwrites it. Create
    // mode starts clean — mirror is on until first manual edit.
    currentPriceManualRef.current = mode === "edit"
  }, [open, defaults, reset, persistDrafts, draftKey, initialStep, mode])

  // Mark each form step visited the moment we land on it. The
  // segmented stepper uses this to decide whether a forward jump is
  // allowed (only previously-seen surfaces are clickable). Also
  // clears any stale server-error banner from a prior submit attempt
  // so it doesn't follow the user across step navigation — the
  // banner lives between submits, not steps.

  useEffect(() => {
    setServerError(null)
    if (step === "ai") return
    setVisitedSteps((prev) => (prev.has(step) ? prev : new Set(prev).add(step)))
  }, [step])

  // Reset visited steps whenever the dialog opens — discardDraft
  // explicitly resets to Basics so its visited set should be a fresh
  // singleton; reopening for a new commodity should also start clean.

  useEffect(() => {
    if (!open) return
    setVisitedSteps(new Set(initialStep === "basics" ? ["basics"] : []))
  }, [open, initialStep])

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

  // Live mirror: in create mode, when purchase currency matches the
  // group currency and the user hasn't manually edited Current Value
  // yet, push the typed Original Price into Current Value so the
  // input visibly tracks. Stops as soon as the user types into the
  // Current Value field (PurchaseStep marks `currentPriceManualRef`
  // on user-driven onChange). Reset by the dialog-open effect above.
  // Tracking this with a ref + setValue rather than a derived render
  // value because BE field-level Required (#1625) means the mirrored
  // value needs to actually live in the form state, not just look
  // like it does in the DOM.
  useEffect(() => {
    if (!open || mode !== "create") return
    const subscription = watch((values, info) => {
      if (info.name !== "original_price" && info.name !== "original_price_currency") return
      if (currentPriceManualRef.current) return
      const purchaseCurrency = (values.original_price_currency ?? "").trim().toUpperCase()
      const groupCurrencyUpper = (defaultCurrency ?? "").trim().toUpperCase()
      if (!purchaseCurrency || purchaseCurrency !== groupCurrencyUpper) return
      const next = (values.original_price ?? "") as string
      // Use shouldDirty=false so the mirror doesn't itself flip the
      // form's dirty flag — that's reserved for real user intent.
      setValue("current_price", next, { shouldDirty: false, shouldValidate: false })
    })
    return () => subscription.unsubscribe()
  }, [open, mode, watch, setValue, defaultCurrency])

  async function nextStep() {
    const fields = STEP_FIELDS[step]
    const ok = await trigger(fields, { shouldFocus: true })
    if (!ok) return
    if (step === "ai") {
      // AI is the alternative entry point — Next ("Fill manually")
      // hands off to the first form step.
      setStep("basics")
      return
    }
    const idx = FORM_STEPS.indexOf(step)
    if (idx >= 0 && idx < FORM_STEPS.length - 1) setStep(FORM_STEPS[idx + 1])
  }
  function prevStep() {
    if (step === "ai") return
    const idx = FORM_STEPS.indexOf(step)
    // Prev is disabled on Basics (idx === 0) — the AI surface is an
    // alternative entry, not a previous step the user can rewind to.
    if (idx > 0) setStep(FORM_STEPS[idx - 1])
  }

  // requestClose intercepts dismiss intents (Escape / click-outside
  // / X / Cancel). Dirty create-mode wizards trigger the save-as-
  // draft confirmation; everything else closes immediately. The
  // confirm has three outcomes: Save as draft (preserve localStorage,
  // close), Discard (clear draft, close), Keep editing (do nothing).
  function requestClose() {
    if (mode === "create" && persistDrafts && isDirty) {
      setCloseConfirmOpen(true)
      return
    }
    onOpenChange(false)
  }

  function confirmCloseSaveDraft() {
    // Auto-save effect already wrote the latest values to localStorage;
    // nothing to do here besides closing.
    if (persistDrafts && draftKey) {
      writeDraft(draftKey, getValues())
    }
    setCloseConfirmOpen(false)
    onOpenChange(false)
  }

  function confirmCloseDiscard() {
    if (draftKey) clearDraft(draftKey)
    setCloseConfirmOpen(false)
    onOpenChange(false)
  }

  const submit = async (values: CommodityFormInput) => {
    setServerError(null)
    try {
      await onSubmit(toRequest(values, defaultCurrency))
      // Submitted successfully — drop the draft so a fresh dialog open
      // doesn't replay yesterday's data.
      if (persistDrafts && draftKey) clearDraft(draftKey)
    } catch (err) {
      // Map BE field-level validation errors back onto RHF fields
      // (so the failing input gets highlighted and the inline error
      // copy renders next to it) and jump to the step owning the
      // first failing field. Necessary even with FE schema mirrors —
      // BE rules can be stricter than what the FE schema models, and
      // we should never silently round-trip a 422.
      const fieldErrors = parseCommodityFieldErrors(err)
      if (fieldErrors) {
        for (const [name, message] of Object.entries(fieldErrors)) {
          setError(name as keyof CommodityFormInput, { type: "server", message })
        }
        const firstField = Object.keys(fieldErrors)[0]
        const targetStep = (FORM_STEPS as readonly string[]).find((s) =>
          STEP_FIELDS[s as StepKey].some((f) => f === firstField)
        ) as StepKey | undefined
        if (targetStep && targetStep !== step) setStep(targetStep)
      }
      // Pull the BE's actual error detail out of the HttpError envelope
      // (JSON:API `errors[0].detail` / `error` / `message`) instead of
      // showing the bare "Request to … failed with NNN" wrapper. Falls
      // back to a generic copy when the body has nothing useful.
      setServerError(parseServerError(err, t("commodities:form.serverError")))
    }
  }

  // The numbered stepper + Back/Next gating only know about FORM_STEPS.
  // On the AI surface the form-step index is reported as -1 (not in
  // the form sequence yet) — Back is disabled, Next ("Fill manually")
  // is wired separately above.
  const formStepIndex = step === "ai" ? -1 : FORM_STEPS.indexOf(step)
  const isLastStep = formStepIndex === FORM_STEPS.length - 1
  // #1554: a count > 1 row is a bundle of identical units and can't
  // carry warranty / loan / service. Watching the count value lets the
  // banner show up live as soon as the user types, and lets the
  // warranty step disable its inputs without waiting for a re-render.
  const liveCount = Number(watch("count"))
  const isBundle = Number.isFinite(liveCount) && liveCount > 1

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          requestClose()
          return
        }
        onOpenChange(next)
      }}
    >
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
          <DialogDescription>
            {step === "ai"
              ? t("commodities:form.step.ai.description")
              : t("commodities:form.stepCounter", {
                  current: formStepIndex + 1,
                  total: FORM_STEPS.length,
                  label: t(`commodities:form.step.${step}.title`),
                })}
          </DialogDescription>
        </DialogHeader>

        {/* Segmented progress bar — design-mock AddItemDialog L534-L552.
            Five thin pill segments, one per form step. Each segment:
              · `bg-primary` when current or already visited
              · `bg-muted` when still untouched
              · `bg-destructive` / `destructive/60` when the step has
                a validation error
            Visited segments are clickable, future ones aren't — back
            navigation is always free, forward only after the user
            has landed on the step at least once. Hover surfaces a
            tooltip with the step's label so the bars stay legible
            without numbered text. */}
        {step !== "ai" ? (
          <TooltipProvider delayDuration={120}>
            <ol
              className="flex items-center gap-1.5"
              aria-label={t("commodities:form.stepperLabel")}
            >
              {FORM_STEPS.map((s, i) => {
                const isCurrent = step === s
                const isVisited = visitedSteps.has(s)
                const hasError = STEP_FIELDS[s].some((f) => f in errors)
                const navigable = !isCurrent && (isVisited || i < formStepIndex)
                // Three-tier fill: solid primary for the current
                // segment, dimmed primary for already-visited
                // (regardless of whether it sits before or after
                // current — we want the user to *see* they jumped
                // back), muted for untouched future steps. Same
                // ladder in destructive when the step has errors.
                const fill = hasError
                  ? isCurrent
                    ? "bg-destructive"
                    : isVisited
                      ? "bg-destructive/40"
                      : "bg-destructive/20"
                  : isCurrent
                    ? "bg-primary"
                    : isVisited
                      ? "bg-primary/40"
                      : "bg-muted"
                const stepLabel = t(`commodities:form.step.${s}.title`)
                return (
                  <li key={s} className="flex-1">
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <button
                          type="button"
                          aria-label={stepLabel}
                          aria-current={isCurrent ? "step" : undefined}
                          aria-disabled={navigable ? undefined : true}
                          disabled={!navigable}
                          onClick={() => {
                            if (navigable) setStep(s)
                          }}
                          className={cn(
                            "h-1.5 w-full rounded-full transition-colors",
                            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
                            navigable ? "cursor-pointer hover:opacity-80" : "cursor-default",
                            fill
                          )}
                        />
                      </TooltipTrigger>
                      <TooltipContent>{stepLabel}</TooltipContent>
                    </Tooltip>
                  </li>
                )
              })}
            </ol>
          </TooltipProvider>
        ) : null}

        {/* Draft Switch row — design-mock AddItemDialog L555-L569.
            Lifted out of BasicsStep so the toggle stays visible across
            every form step. Hidden on the AI step (where the form
            isn't yet active). */}
        {step !== "ai" ? (
          <div className="flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-3 py-2.5">
            <Controller
              control={control}
              name="draft"
              render={({ field }) => (
                <Switch
                  id="commodity-draft"
                  checked={!!field.value}
                  onCheckedChange={(v) => field.onChange(!!v)}
                />
              )}
            />
            <div className="min-w-0 flex-1">
              <Label htmlFor="commodity-draft" className="cursor-pointer text-sm font-medium">
                {t("commodities:fields.draft")}
              </Label>
              <p className="text-xs text-muted-foreground">{t("commodities:fields.draftHelp")}</p>
            </div>
          </div>
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
          // Pressing Enter inside any field on an intermediate step
          // would otherwise trigger this form's submit (browser
          // default for `<form>` with an implicit submit), firing the
          // create mutation prematurely — the user reported a 422
          // appearing without ever clicking "Add item". Route the
          // implicit submit through nextStep when we're not on the
          // final step so Enter advances instead of submitting.
          // Always block native form submit. The wizard's Next /
          // Submit are wired via explicit onClick handlers; a native
          // submit (e.g. Enter inside a field, OR the post-render race
          // where clicking Next on Extras causes React to swap the
          // button into a submit-button at the same DOM coords mid-
          // click) would otherwise trigger the create mutation
          // unintentionally.
          onSubmit={(e) => e.preventDefault()}
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
              locations={locations}
              showStatus={mode === "edit"}
            />
          ) : null}
          {step === "purchase" ? (
            <PurchaseStep
              register={register}
              control={control}
              errors={errors}
              watch={watch}
              defaultCurrency={defaultCurrency}
              onCurrentPriceUserEdit={() => {
                currentPriceManualRef.current = true
              }}
            />
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
            <Button type="button" variant="ghost" className="mr-auto" onClick={requestClose}>
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
          <DialogFooter className="gap-2">
            <Button
              type="button"
              variant="ghost"
              className="mr-auto"
              onClick={requestClose}
              data-testid="commodity-form-cancel"
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={prevStep}
              disabled={formStepIndex <= 0}
            >
              <ChevronLeft className="size-4" aria-hidden="true" />
              {t("commodities:form.back")}
            </Button>
            {isLastStep ? (
              <Button
                type="button"
                onClick={() => void handleSubmit(submit)()}
                disabled={isPending}
                data-testid="commodity-form-submit"
              >
                {mode === "create"
                  ? t("commodities:form.submitCreate")
                  : t("commodities:form.submitEdit")}
              </Button>
            ) : (
              <Button type="button" onClick={nextStep} data-testid="commodity-form-next">
                {t("commodities:form.continue")}
                <ChevronRight className="size-4" aria-hidden="true" />
              </Button>
            )}
          </DialogFooter>
        )}
      </DialogContent>

      {/* Save-as-draft confirmation. Three outcomes: Save as draft
          (preserve localStorage + close), Discard (clear + close),
          Keep editing (Escape / Cancel button → keep wizard open).
          Mounted as a sibling Dialog inside the same Radix tree so
          focus management hands back to the wizard cleanly. */}
      <Dialog open={closeConfirmOpen} onOpenChange={setCloseConfirmOpen}>
        <DialogContent className="sm:max-w-sm" data-testid="commodity-form-close-confirm">
          <DialogHeader>
            <DialogTitle>{t("commodities:form.closeConfirm.title")}</DialogTitle>
            <DialogDescription>{t("commodities:form.closeConfirm.description")}</DialogDescription>
          </DialogHeader>
          <DialogFooter className="gap-2">
            <Button
              type="button"
              variant="ghost"
              className="mr-auto"
              onClick={() => setCloseConfirmOpen(false)}
              data-testid="commodity-form-close-confirm-cancel"
            >
              {t("commodities:form.closeConfirm.keepEditing")}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={confirmCloseDiscard}
              data-testid="commodity-form-close-confirm-discard"
            >
              {t("commodities:form.closeConfirm.discard")}
            </Button>
            <Button
              type="button"
              onClick={confirmCloseSaveDraft}
              data-testid="commodity-form-close-confirm-save"
            >
              {t("commodities:form.closeConfirm.saveDraft")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Dialog>
  )
}

// FieldLabel renders a `<Label>` plus an optional asterisk
// indicator. The asterisk lives in a sibling `<span>` rather than
// inside the label so the label's `textContent` stays equal to the
// field name — `getByLabelText(/^Name$/i)` in tests still matches
// without having to special-case the marker. `aria-hidden` keeps
// screen readers from announcing the asterisk; the input's
// `aria-required` attribute carries the semantic.
function FieldLabel({
  htmlFor,
  required,
  children,
}: {
  htmlFor: string
  required?: boolean
  children: ReactNode
}) {
  // Always render the asterisk slot — hidden via `invisible` when
  // not required — so the row's height is stable regardless of
  // dynamic required-ness. Without this, toggling Save-as-draft
  // would shift every label's baseline by the asterisk's line
  // height, jittering the whole step body vertically.
  return (
    <div className="flex items-baseline gap-1">
      <Label htmlFor={htmlFor}>{children}</Label>
      <span aria-hidden="true" className={cn("text-destructive", !required && "invisible")}>
        *
      </span>
    </div>
  )
}

// ---- Step 1: Basics -----------------------------------------------------

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- RHF types thread generics through every helper; concrete types here are noisy.
function BasicsStep(props: any) {
  const { t } = useTranslation()
  const { register, control, errors, watch, setValue, areas, locations, showStatus } = props
  // Mock AddItemDialog L1074-L1091: Location and Area are paired
  // selects. The form schema only carries `area_id` (the BE resolves
  // location via the area), so the location_id lives in local UI
  // state and is derived from the selected area on edit. Picking a
  // different location clears the area.
  const allAreas = areas as AreaOption[]
  const areaIdValue = (watch("area_id") as string | undefined) ?? ""
  // selectedLocationId is UI-only — the form schema only carries
  // area_id. Initialise from the current area_id (covers edit mode
  // and dialog re-opens that re-mount BasicsStep). After mount the
  // user owns this state via handleLocationChange; we deliberately
  // don't keep an area→location useEffect — it would race with
  // setValue("area_id", "") inside handleLocationChange and snap the
  // location back to "" the moment the user picked a new one.
  const [selectedLocationId, setSelectedLocationId] = useState<string>(() => {
    if (!areaIdValue) return ""
    const match = allAreas.find((a) => a.id === areaIdValue)
    return match?.location_id ?? ""
  })
  const visibleAreas = selectedLocationId
    ? allAreas.filter((a) => a.location_id === selectedLocationId)
    : []
  const visibleLocations = locations as LocationOption[]
  function handleLocationChange(next: string) {
    if (next === selectedLocationId) return
    setSelectedLocationId(next)
    // Clear area when location changes — the previous area belongs
    // to a different location and would be invalid for the BE's
    // group-aware uniqueness checks.
    setValue("area_id", "", { shouldDirty: true, shouldValidate: false })
  }
  return (
    <div className="space-y-4 py-2">
      <div className="flex flex-col gap-1.5">
        <FieldLabel htmlFor="commodity-name" required>
          {t("commodities:fields.name")}
        </FieldLabel>
        <Input
          id="commodity-name"
          aria-required
          {...register("name")}
          aria-invalid={!!errors.name}
        />
        <FieldError error={errors.name} />
      </div>

      <div className="flex flex-col gap-1.5">
        <FieldLabel htmlFor="commodity-short-name" required>
          {t("commodities:fields.shortName")}
        </FieldLabel>
        <Input
          id="commodity-short-name"
          maxLength={22}
          className="font-mono text-sm"
          aria-required
          {...register("short_name")}
          aria-invalid={!!errors.short_name}
        />
        {errors.short_name ? (
          <FieldError error={errors.short_name} />
        ) : (
          <p className="text-xs text-muted-foreground">{t("commodities:fields.shortNameHelp")}</p>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="flex flex-col gap-1.5">
          <FieldLabel htmlFor="commodity-type" required>
            {t("commodities:fields.type")}
          </FieldLabel>
          <Controller
            control={control}
            name="type"
            render={({ field }) => (
              <Select value={field.value || undefined} onValueChange={field.onChange}>
                <SelectTrigger
                  id="commodity-type"
                  className="w-full"
                  aria-required
                  aria-invalid={!!errors.type}
                >
                  <SelectValue placeholder={t("commodities:fields.typePlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {COMMODITY_TYPES.map((tp) => (
                    <SelectItem key={tp} value={tp}>
                      <span className="mr-1">{COMMODITY_TYPE_ICONS[tp as CommodityTypeValue]}</span>
                      {t(`commodities:type.${tp}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
          <FieldError error={errors.type} />
        </div>
        <div className="flex flex-col gap-1.5">
          <FieldLabel htmlFor="commodity-count" required>
            {t("commodities:fields.count")}
          </FieldLabel>
          <Input
            id="commodity-count"
            type="number"
            min={1}
            aria-required
            {...register("count")}
            aria-invalid={!!errors.count}
          />
          <FieldError error={errors.count} />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="flex flex-col gap-1.5">
          <FieldLabel htmlFor="commodity-location" required>
            {t("commodities:fields.location")}
          </FieldLabel>
          <Select value={selectedLocationId || undefined} onValueChange={handleLocationChange}>
            <SelectTrigger id="commodity-location" className="w-full" aria-required>
              <SelectValue placeholder={t("commodities:fields.locationPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              {visibleLocations.map((l) => (
                <SelectItem key={l.id} value={l.id ?? ""}>
                  {l.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-1.5">
          <FieldLabel htmlFor="commodity-area" required>
            {t("commodities:fields.area")}
          </FieldLabel>
          <Controller
            control={control}
            name="area_id"
            render={({ field }) => (
              <Select
                // Re-key on selectedLocationId so a location swap
                // remounts the Select with a clean Radix internal
                // state — without this, Radix keeps the previously-
                // selected label visible in the trigger even though
                // the controlled value has been reset to "" and the
                // option is no longer in the list, leaving the field
                // looking blank rather than restoring the
                // "Pick an area" placeholder.
                key={selectedLocationId || "no-location"}
                value={field.value || undefined}
                onValueChange={field.onChange}
                disabled={!selectedLocationId}
              >
                <SelectTrigger
                  id="commodity-area"
                  className="w-full"
                  aria-required
                  aria-invalid={!!errors.area_id}
                >
                  <SelectValue
                    placeholder={
                      selectedLocationId
                        ? t("commodities:fields.areaPlaceholder")
                        : t("commodities:fields.areaPlaceholderNoLocation")
                    }
                  />
                </SelectTrigger>
                <SelectContent>
                  {visibleAreas.map((a) => (
                    <SelectItem key={a.id} value={a.id ?? ""}>
                      {a.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
          <FieldError error={errors.area_id} />
        </div>
      </div>

      {showStatus ? (
        <div className="flex flex-col gap-1.5">
          <FieldLabel htmlFor="commodity-status" required>
            {t("commodities:fields.status")}
          </FieldLabel>
          <Controller
            control={control}
            name="status"
            render={({ field }) => (
              <Select value={field.value || undefined} onValueChange={field.onChange}>
                <SelectTrigger
                  id="commodity-status"
                  className="w-full"
                  aria-required
                  aria-invalid={!!errors.status}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {COMMODITY_STATUSES.map((s) => (
                    <SelectItem key={s} value={s}>
                      {t(`commodities:status.${s}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
          <FieldError error={errors.status} />
        </div>
      ) : null}
    </div>
  )
}

// Currency-prefix padding for the price inputs. The mock renders the
// symbol absolutely-positioned at `left-3` and uses a static `pl-6`
// for the input — that overlaps for any 2+ character symbol (`Kč`,
// `NZ$`, `د.إ`). Pick a class derived from the symbol's character
// count instead so even a 4-char prefix has space.
function priceInputPaddingClass(symbol: string): string {
  // Codepoint-aware length; emoji-style multi-byte symbols still
  // count as one glyph.
  const len = [...(symbol || "$")].length
  if (len <= 1) return "pl-7"
  if (len === 2) return "pl-9"
  if (len === 3) return "pl-12"
  return "pl-14"
}

// ---- Step 2: Purchase ---------------------------------------------------

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- see BasicsStep
function PurchaseStep(props: any) {
  const { t } = useTranslation()
  const { register, control, errors, watch, defaultCurrency, onCurrentPriceUserEdit } = props
  // Wrap RHF's register output for current_price so user-driven
  // onChange events also flip the dialog's
  // `currentPriceManualRef` — once that fires, the dialog's
  // live-mirror effect stops auto-filling Current Value with
  // Original Price.
  const currentPriceReg = register("current_price")
  // Required-ness on Purchase fields is dynamic. Drafts relax all
  // four (purchase_date / original_price / converted / current);
  // commodity submits require them. Schema is in
  // `features/commodities/schemas.ts` superRefine `whenNotDraft`.
  const isDraft = !!watch("draft")
  const requireWhenNotDraft = !isDraft
  // Foreign-currency check mirrors the mock's `isForeignCurrency`
  // (AddItemDialog L1154). When the purchase currency matches the
  // group's currency the converted-price field is moot — the original
  // price is already in group currency. Surface it only when the two
  // diverge so we don't make the user re-type the same amount.
  const purchaseCurrency = (watch("original_price_currency") as string | undefined) ?? ""
  const groupCurrency = (defaultCurrency as string | undefined) ?? ""
  const isForeignCurrency =
    !!purchaseCurrency &&
    !!groupCurrency &&
    purchaseCurrency.toUpperCase() !== groupCurrency.toUpperCase()
  // Inline price-input prefix mirrors the picked currency's symbol
  // (mock AddItemDialog L1153 + L1177 reads `currencySymbol` from the
  // CURRENCIES list). Falls back to the bare code when no metadata is
  // known so unfamiliar currencies still render legibly.
  const purchaseSymbol = currencyMeta(purchaseCurrency || groupCurrency || "USD").symbol
  const groupSymbol = currencyMeta(groupCurrency || "USD").symbol
  const purchasePadClass = priceInputPaddingClass(purchaseSymbol)
  const groupPadClass = priceInputPaddingClass(groupSymbol)
  return (
    <div className="space-y-4 py-2">
      <div className="flex flex-col gap-1.5">
        <FieldLabel htmlFor="commodity-purchase-date" required={requireWhenNotDraft}>
          {t("commodities:fields.purchaseDate")}
        </FieldLabel>
        <Input
          id="commodity-purchase-date"
          type="date"
          aria-required={requireWhenNotDraft || undefined}
          {...register("purchase_date")}
          aria-invalid={!!errors.purchase_date}
        />
        <FieldError error={errors.purchase_date} />
      </div>

      {/* Mock AddItemDialog L1173-L1196: combined "Purchase Price" row
          — price input with leading currency symbol on the left, the
          compact CurrencyCombobox on the right. The price field grows
          (flex-1) and the combobox holds a fixed code-width so the
          two never wrestle for space. */}
      <div className="flex flex-col gap-1.5">
        <FieldLabel htmlFor="commodity-original-price" required={requireWhenNotDraft}>
          {t("commodities:fields.originalPrice")}
        </FieldLabel>
        <div className="flex gap-2">
          <div className="relative flex-1">
            <span
              aria-hidden="true"
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 select-none text-sm text-muted-foreground"
            >
              {purchaseSymbol}
            </span>
            <Input
              id="commodity-original-price"
              type="number"
              step="0.01"
              min={0}
              placeholder="0"
              className={purchasePadClass}
              aria-required={requireWhenNotDraft || undefined}
              {...register("original_price")}
              aria-invalid={!!errors.original_price}
            />
          </div>
          <Controller
            control={control}
            name="original_price_currency"
            render={({ field }) => (
              <CurrencyCombobox
                id="commodity-currency"
                value={field.value ?? ""}
                onChange={field.onChange}
                ariaInvalid={!!errors.original_price_currency}
                variant="compact"
              />
            )}
          />
        </div>
        {errors.original_price ? (
          <FieldError error={errors.original_price} />
        ) : errors.original_price_currency ? (
          <FieldError error={errors.original_price_currency} />
        ) : (
          <p className="text-xs text-muted-foreground">
            {t("commodities:fields.originalPriceHelp")}
          </p>
        )}
      </div>

      {/* Mock AddItemDialog L1198-L1233: foreign-currency variant
          renders an amber-bordered card with the "Foreign currency
          detected" banner, the Converted Purchase Price field, an OR
          divider, and the Current Value field. Same-currency drops
          the converted-price field entirely (the original price is
          already in group currency) and renders a plain Current
          Value field below. */}
      {isForeignCurrency ? (
        <div className="flex flex-col gap-3 rounded-lg border border-amber-300/60 bg-amber-50 p-3 dark:border-amber-900/60 dark:bg-amber-950/30">
          <p className="text-xs leading-relaxed text-amber-900 dark:text-amber-200">
            <span className="font-semibold">{t("commodities:fields.foreignCurrencyDetected")}</span>{" "}
            {t("commodities:fields.foreignCurrencyBanner", { groupCurrency })}
          </p>
          <div className="flex flex-col gap-1.5">
            <FieldLabel htmlFor="commodity-converted-price" required={requireWhenNotDraft}>
              {t("commodities:fields.convertedOriginalPrice", { groupCurrency })}
            </FieldLabel>
            <div className="relative">
              <span
                aria-hidden="true"
                className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 select-none text-sm text-muted-foreground"
              >
                {groupSymbol}
              </span>
              <Input
                id="commodity-converted-price"
                type="number"
                step="0.01"
                min={0}
                placeholder="0"
                className={cn("bg-background", groupPadClass)}
                aria-required={requireWhenNotDraft || undefined}
                {...register("converted_original_price")}
                aria-invalid={!!errors.converted_original_price}
              />
            </div>
            {errors.converted_original_price ? (
              <FieldError error={errors.converted_original_price} />
            ) : (
              <p className="text-xs text-amber-800/80 dark:text-amber-300/80">
                {t("commodities:fields.convertedOriginalPriceHint")}
              </p>
            )}
          </div>
          <div className="flex items-center gap-2">
            <div className="h-px flex-1 bg-amber-300/60 dark:bg-amber-900/60" />
            <span className="text-[10px] font-medium uppercase tracking-wide text-amber-700 dark:text-amber-300">
              {t("commodities:fields.foreignCurrencyOr")}
            </span>
            <div className="h-px flex-1 bg-amber-300/60 dark:bg-amber-900/60" />
          </div>
          <div className="flex flex-col gap-1.5">
            <FieldLabel htmlFor="commodity-current-price" required={requireWhenNotDraft}>
              {t("commodities:fields.currentPriceForeign", { groupCurrency })}
            </FieldLabel>
            <div className="relative">
              <span
                aria-hidden="true"
                className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 select-none text-sm text-muted-foreground"
              >
                {groupSymbol}
              </span>
              <Input
                id="commodity-current-price"
                type="number"
                step="0.01"
                min={0}
                placeholder="0"
                className={cn("bg-background", groupPadClass)}
                {...currentPriceReg}
                onChange={(e) => {
                  onCurrentPriceUserEdit?.()
                  currentPriceReg.onChange(e)
                }}
                aria-required={requireWhenNotDraft || undefined}
                aria-invalid={!!errors.current_price}
              />
            </div>
            {errors.current_price ? (
              <FieldError error={errors.current_price} />
            ) : (
              <p className="text-xs text-amber-800/80 dark:text-amber-300/80">
                {t("commodities:fields.currentPriceHelp")}
              </p>
            )}
          </div>
        </div>
      ) : (
        <div className="flex flex-col gap-1.5">
          <FieldLabel htmlFor="commodity-current-price" required={requireWhenNotDraft}>
            {t("commodities:fields.currentPrice")}
          </FieldLabel>
          <div className="relative">
            <span
              aria-hidden="true"
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 select-none text-sm text-muted-foreground"
            >
              {groupSymbol}
            </span>
            <Input
              id="commodity-current-price"
              type="number"
              step="0.01"
              min={0}
              placeholder="0"
              className={groupPadClass}
              {...currentPriceReg}
              onChange={(e) => {
                onCurrentPriceUserEdit?.()
                currentPriceReg.onChange(e)
              }}
              aria-required={requireWhenNotDraft || undefined}
              aria-invalid={!!errors.current_price}
            />
          </div>
          {errors.current_price ? (
            <FieldError error={errors.current_price} />
          ) : (
            <p className="text-xs text-muted-foreground">
              {t("commodities:fields.currentPriceHelp")}
            </p>
          )}
        </div>
      )}

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-serial">{t("commodities:fields.serialNumber")}</Label>
        <Input
          id="commodity-serial"
          className="font-mono text-sm"
          placeholder={t("commodities:fields.serialNumberPlaceholder")}
          {...register("serial_number")}
        />
        <p className="text-xs text-muted-foreground">{t("commodities:fields.serialNumberHelp")}</p>
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
    <div className="space-y-4 py-2" data-testid="commodity-form-warranty-step">
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
        {errors.warranty_expires_at ? (
          <FieldError error={errors.warranty_expires_at} />
        ) : (
          <p className="text-xs text-muted-foreground">
            {isBundle
              ? t("commodities:trackingRestrictions.warrantyStepHint")
              : t("commodities:fields.warrantyExpiresAtHelp")}
          </p>
        )}
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
        <Textarea
          id="commodity-warranty-notes"
          rows={3}
          className="resize-none"
          placeholder={t("commodities:fields.warrantyNotesPlaceholder")}
          {...register("warranty_notes")}
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
    <div className="space-y-4 py-2">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-comments">{t("commodities:fields.comments")}</Label>
        <Textarea
          id="commodity-comments"
          rows={3}
          className="resize-none"
          placeholder={t("commodities:fields.commentsPlaceholder")}
          {...register("comments")}
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

// parseCommodityFieldErrors extracts per-field validation messages
// from the BE's 422 envelope so we can map them back onto RHF.
//
// The Inventario BE wraps validation errors in:
//   {
//     "errors": [
//       {
//         "status": "Unprocessable Entity",
//         "error": {                 // jsonapi.Error.UserError (raw JSON)
//           "type": "validation.Errors",
//           "error": {               // ozzo / jellydator validation envelope
//             "data": {
//               "attributes": {
//                 "<field>": "<message>"
//               }
//             }
//           }
//         }
//       }
//     ]
//   }
//
// Returns a flat `{ field: message }` map limited to known commodity
// form fields, or null when the response doesn't match the shape.
function parseCommodityFieldErrors(err: unknown): Record<string, string> | null {
  if (!(err instanceof HttpError)) return null
  const data = err.data as unknown
  if (!data || typeof data !== "object") return null
  const errorsArr = (data as { errors?: unknown }).errors
  if (!Array.isArray(errorsArr) || errorsArr.length === 0) return null
  const first = errorsArr[0]
  if (!first || typeof first !== "object") return null
  const userErr = (first as { error?: unknown }).error
  if (!userErr || typeof userErr !== "object") return null
  const inner = (userErr as { error?: unknown }).error
  if (!inner || typeof inner !== "object") return null
  const dataObj = (inner as { data?: unknown }).data
  if (!dataObj || typeof dataObj !== "object") return null
  const attrs = (dataObj as { attributes?: unknown }).attributes
  if (!attrs || typeof attrs !== "object") return null
  // Only keep keys that actually live on the form (filter unknown
  // server-only fields out so RHF.setError doesn't reject the path).
  const known = new Set<string>()
  for (const fields of Object.values(STEP_FIELDS)) {
    for (const f of fields) known.add(f)
  }
  const result: Record<string, string> = {}
  for (const [k, v] of Object.entries(attrs)) {
    if (typeof v === "string" && known.has(k)) result[k] = v
  }
  return Object.keys(result).length > 0 ? result : null
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
function toRequest(
  values: CommodityFormInput,
  groupCurrency: string
): CreateCommodityRequest & UpdateCommodityRequest {
  const num = (v: string): number | undefined => (v === "" ? undefined : Number(v))
  // Date fields are PDate (pointer-to-Date) on the BE — `Date.UnmarshalJSON`
  // rejects empty strings as "cannot parse \"\" as \"2006\"". Omit the
  // field entirely when the input is blank so the BE sees a missing
  // value (decoded as nil pointer) rather than an invalid date string.
  const date = (v: string): string | undefined => {
    const trimmed = v.trim()
    return trimmed === "" ? undefined : trimmed
  }
  // BE rule (commodity.go:378 + the matching custom validator):
  // when the purchase currency matches the group's currency,
  // `converted_original_price` MUST be 0 — the original price is
  // already expressed in group currency, so a non-zero converted
  // amount would conflict. The mock hides the converted-price field
  // entirely in this case (AddItemDialog L1198 isForeignCurrency =
  // false branch); we mirror that visually, and force the value to
  // 0 here so the BE's same-currency invariant is satisfied. Foreign
  // currency: pass through whatever the user typed.
  const original = num(values.original_price)
  const convertedFromForm = num(values.converted_original_price)
  const sameCurrency =
    !!groupCurrency &&
    values.original_price_currency.trim().toUpperCase() === groupCurrency.trim().toUpperCase()
  const converted = sameCurrency ? 0 : convertedFromForm
  // TODO(#1625): remove this current-price mirror once the BE drops
  // `validation.Required` from `CurrentPrice` in commodity.go:382.
  // PriceRule's design intent is that current_price=0 is valid in the
  // same-currency case (the unit test in price_test.go:42-47 covers
  // this), but the field-level `Required` contradicts the rule and
  // makes BE refuse any same-currency row with an empty Current
  // Value. Mirror original_price into current_price when same-currency
  // AND the user didn't fill the field, so they don't have to type
  // the same number twice. Foreign-currency keeps the user's input.
  const currentFromForm = num(values.current_price)
  const current = sameCurrency ? (currentFromForm ?? original) : currentFromForm
  return {
    name: values.name.trim(),
    short_name: values.short_name.trim(),
    type: values.type as CommodityTypeValue,
    area_id: values.area_id,
    status: values.status as CommodityStatusValue,
    count: Number(values.count),
    original_price: original,
    original_price_currency: values.original_price_currency,
    converted_original_price: converted,
    current_price: current,
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
