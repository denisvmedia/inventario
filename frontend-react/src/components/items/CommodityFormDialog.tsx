import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { ChevronLeft, ChevronRight, Plus, X } from "lucide-react"

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
import {
  COMMODITY_STATUSES,
  COMMODITY_TYPES,
  COMMODITY_TYPE_ICONS,
  type CommodityStatusValue,
  type CommodityTypeValue,
} from "@/features/commodities/constants"
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
}

const STEPS = ["basics", "purchase", "extras"] as const
type StepKey = (typeof STEPS)[number]

// Per-step field allow-list — used by the Next button to validate only
// the current step's fields before advancing. Validating the whole form
// would block the user on fields they haven't seen yet.
const STEP_FIELDS: Record<StepKey, (keyof CommodityFormInput)[]> = {
  basics: ["name", "short_name", "type", "area_id", "status", "count", "draft"],
  purchase: [
    "purchase_date",
    "original_price",
    "original_price_currency",
    "converted_original_price",
    "current_price",
    "serial_number",
  ],
  extras: ["tags", "comments", "urls", "extra_serial_numbers", "part_numbers"],
}

// CommodityFormDialog hosts the multi-step Add Item / Edit form. Three
// steps to keep the UI scannable: Basics → Purchase → Extras. Warranty
// and Files steps from the design mock fold into Extras and the detail
// page respectively, since the BE for first-class warranties (#1367)
// and the unified Files surface (#1398/#1399) hasn't shipped yet.
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
}: CommodityFormDialogProps) {
  const { t } = useTranslation()
  const [step, setStep] = useState<StepKey>("basics")
  const [serverError, setServerError] = useState<string | null>(null)

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

  // Reset to defaults whenever the dialog opens — handles re-using the
  // same dialog instance for a fresh create after an edit (or vice versa).
  useEffect(() => {
    if (open) {
      reset(defaults)
      setStep("basics")
      setServerError(null)
    }
  }, [open, defaults, reset])

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
    } catch (err) {
      setServerError(err instanceof Error ? err.message : t("commodities:form.serverError"))
    }
  }

  const stepIndex = STEPS.indexOf(step)
  const isLastStep = stepIndex === STEPS.length - 1

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {mode === "create"
              ? t("commodities:form.createTitle")
              : t("commodities:form.editTitle")}
          </DialogTitle>
          <DialogDescription>{t(`commodities:form.step.${step}.description`)}</DialogDescription>
        </DialogHeader>

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
              {i < STEPS.length - 1 ? <ChevronRight className="size-3" aria-hidden="true" /> : null}
            </li>
          ))}
        </ol>

        <Separator />

        <form
          id="commodity-form"
          onSubmit={handleSubmit(submit)}
          className="flex flex-col gap-4"
          noValidate
        >
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
          {step === "extras" ? (
            <ExtrasStep register={register} errors={errors} watch={watch} setValue={setValue} />
          ) : null}

          {serverError ? (
            <p className="text-sm text-destructive" data-testid="commodity-form-error">
              {serverError}
            </p>
          ) : null}
        </form>

        <DialogFooter className="gap-2 sm:justify-between">
          <Button type="button" variant="ghost" onClick={prevStep} disabled={stepIndex === 0}>
            <ChevronLeft className="size-4" aria-hidden="true" />
            {t("commodities:form.back")}
          </Button>
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
        </DialogFooter>
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

// ---- Step 3: Extras -----------------------------------------------------

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

// buildDefaults populates the form with safe initial values. For edit
// mode it carries the existing record; for create mode the only
// pre-populated bits are the count (1), status (in_use), draft (false),
// and the group's main currency. Everything else stays empty so the
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
  }
}

// toRequest maps the validated form input into the BE-shaped envelope's
// attributes. Numbers come out of the form as strings (see schemas.ts);
// we convert here. `urls` flows through as string[] even though
// openapi-typescript types it as a single string (see buildDefaults).
function toRequest(values: CommodityFormInput): CreateCommodityRequest & UpdateCommodityRequest {
  const num = (v: string): number | undefined => (v === "" ? undefined : Number(v))
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
    purchase_date: values.purchase_date,
    urls: values.urls as unknown as string,
    comments: values.comments,
    draft: values.draft,
  }
}
