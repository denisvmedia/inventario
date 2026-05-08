import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { ArrowLeft, ArrowRight, Loader2 } from "lucide-react"

import { CurrencyCombobox } from "@/components/CurrencyCombobox"
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
import { usePreviewMigration, useStartMigration } from "@/features/currency-migration/hooks"
import type { MigrationPreview } from "@/features/currency-migration/api"
import { useAppToast } from "@/hooks/useAppToast"
import { HttpError } from "@/lib/http"
import { formatCurrency } from "@/lib/intl"
import { cn } from "@/lib/utils"
import { getServerErrorCode, getServerErrorMeta, parseServerError } from "@/lib/server-error"

// Drives the 4-step wizard layout — picker → rate → preview → confirm.
// Step 0 isn't used; we 1-index for parity with the spec and the i18n
// step labels.
type Step = 1 | 2 | 3 | 4

interface MigrateCurrencyDialogProps {
  open: boolean
  onOpenChange: (next: boolean) => void
  groupName: string
  fromCurrency: string
  // Slug of the active group. Required because /groups/:groupId/settings
  // does not carry a :groupSlug, so the API/hook layer can't pull it
  // from GroupContext — the caller passes it from the loaded group
  // record. Empty string disables preview / start (keeps the wizard
  // mountable while the group fetch is in flight).
  groupSlug: string
}

// Truncates the user-typed string to at most 6 fraction digits without
// rounding (per the spec: "max 6 decimal places enforced via custom
// onChange parser that truncates additional digits"). Strips negatives
// and any non-digit/non-dot characters; rates are positive-only by
// design (the BE rejects ≤0). Preserves the trailing dot mid-typing
// so the field can still hold intermediate states like "1." or "1.5".
export function truncateRateInput(raw: string): string {
  // Strip everything but digits and the first decimal separator. Empty
  // string is allowed (the field is required, not "non-empty-while-typing").
  const cleaned = raw.replace(/,/g, ".").replace(/[^0-9.]/g, "")
  const firstDot = cleaned.indexOf(".")
  if (firstDot === -1) return cleaned
  // Drop secondary dots — typing "1.2.3" becomes "1.23".
  const intPart = cleaned.slice(0, firstDot)
  const fracPart = cleaned.slice(firstDot + 1).replace(/\./g, "")
  const truncated = fracPart.slice(0, 6)
  // Preserve a trailing dot the user is in the middle of typing
  // (e.g. "1." should stay "1." until they type the next digit).
  return truncated.length > 0 || raw.endsWith(".") || raw.endsWith(",")
    ? `${intPart}.${truncated}`
    : intPart
}

function parseRate(raw: string): number | null {
  const trimmed = raw.trim()
  if (!trimmed) return null
  const value = Number(trimmed)
  if (!Number.isFinite(value) || value <= 0) return null
  return value
}

// Renders "mm:ss" or just "ss" for the preview countdown. The BE returns
// `preview_expires_in_seconds` (int) and `preview_expires_at` (RFC3339);
// we use the absolute expiry for live ticking so a slow render doesn't
// drift the countdown forward.
function formatCountdown(secondsLeft: number): string {
  if (secondsLeft <= 0) return "0:00"
  const m = Math.floor(secondsLeft / 60)
  const s = Math.floor(secondsLeft % 60)
  return `${m}:${s.toString().padStart(2, "0")}`
}

function useCountdown(expiresAt: string | undefined): number {
  // Track "now" as state and refresh it from the interval. `Date.now()`
  // lives in the lazy useState initializer (allowed under the
  // react-hooks/purity rule) and inside the setInterval callback —
  // never in the render body. The arithmetic that derives the
  // remaining seconds is pure: same `expiresAt` + same `now` → same
  // output, so the wizard re-renders deterministically per tick.
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!expiresAt) return
    const interval = window.setInterval(() => setNow(Date.now()), 500)
    return () => window.clearInterval(interval)
  }, [expiresAt])
  if (!expiresAt) return 0
  const expiresMs = Date.parse(expiresAt)
  if (!Number.isFinite(expiresMs)) return 0
  return Math.max(0, Math.floor((expiresMs - now) / 1000))
}

// Picks the top-N commodities by absolute current-price delta. The BE
// already returns the diffs sorted; we slice client-side and let
// `sliceLimit` flow from the spec (top-5).
function topDeltas(diffs: MigrationPreview["diffs"], limit = 5) {
  if (!diffs) return []
  // Defensive sort — the BE may already do this, but the contract isn't
  // guaranteed and the slice is cheap.
  const withDelta = diffs.map((d) => {
    const before = d.current_price_before ?? 0
    const after = d.current_price_after ?? 0
    return { ...d, delta: after - before }
  })
  withDelta.sort((a, b) => Math.abs(b.delta) - Math.abs(a.delta))
  return withDelta.slice(0, limit)
}

export function MigrateCurrencyDialog({
  open,
  onOpenChange,
  groupName,
  fromCurrency,
  groupSlug,
}: MigrateCurrencyDialogProps) {
  // We tag each open transition with a counter so the body re-mounts
  // on every reopen — that's how we keep wizard state (step, picked
  // currency, rate input, preview) clean without an `open`-watching
  // effect that would trip the react-hooks/set-state-in-effect lint.
  const [openCount, setOpenCount] = useState(0)
  const handleOpenChange = (next: boolean) => {
    if (next) setOpenCount((n) => n + 1)
    onOpenChange(next)
  }
  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      {open ? (
        <MigrateCurrencyDialogBody
          key={openCount}
          groupName={groupName}
          fromCurrency={fromCurrency}
          groupSlug={groupSlug}
          onClose={() => onOpenChange(false)}
        />
      ) : null}
    </Dialog>
  )
}

interface MigrateCurrencyDialogBodyProps {
  groupName: string
  fromCurrency: string
  groupSlug: string
  onClose: () => void
}

function MigrateCurrencyDialogBody({
  groupName,
  fromCurrency,
  groupSlug,
  onClose,
}: MigrateCurrencyDialogBodyProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const previewMutation = usePreviewMigration(groupSlug)
  const startMutation = useStartMigration(groupSlug)

  const [step, setStep] = useState<Step>(1)
  const [toCurrency, setToCurrency] = useState("")
  const [rateInput, setRateInput] = useState("")
  const [preview, setPreview] = useState<MigrationPreview | null>(null)
  const [confirmInput, setConfirmInput] = useState("")
  const [previewError, setPreviewError] = useState<string | null>(null)
  const [confirmError, setConfirmError] = useState<string | null>(null)

  const rateValue = useMemo(() => parseRate(rateInput), [rateInput])
  const sameAsCurrent = toCurrency && toCurrency.toUpperCase() === fromCurrency.toUpperCase()
  const secondsLeft = useCountdown(preview?.preview_expires_at)
  const previewExpired = preview != null && secondsLeft <= 0

  function close() {
    onClose()
  }

  async function handlePreview() {
    setPreviewError(null)
    if (!toCurrency || sameAsCurrent || rateValue == null) return
    try {
      const body = await previewMutation.mutateAsync({
        from_currency: fromCurrency,
        to_currency: toCurrency,
        exchange_rate: rateValue,
      })
      setPreview(body)
      setStep(3)
    } catch (err) {
      // Preview itself shouldn't 409/422 in the happy path — the BE
      // validates inputs against the live group state. Surface the
      // server message inline so the user can correct the rate or
      // currency and re-submit.
      setPreviewError(parseServerError(err, t("groups:settings.dialog.previewFailed")))
    }
  }

  async function handleStart() {
    setConfirmError(null)
    if (!preview?.preview_token) {
      setConfirmError(t("errors:currencyMigrationTokenInvalid"))
      return
    }
    if (confirmInput.trim() !== groupName) {
      setConfirmError(t("groups:validation.confirmWordMismatch"))
      return
    }
    try {
      await startMutation.mutateAsync({
        from_currency: fromCurrency,
        to_currency: toCurrency,
        exchange_rate: rateValue ?? Number(preview.exchange_rate ?? 0),
        preview_token: preview.preview_token,
      })
      close()
    } catch (err) {
      handleStartError(err)
    }
  }

  // 4xx routing per spec:
  //   409 preview_expired / state_changed → toast + step 3 with the
  //     same wizard state, ready to re-submit preview.
  //   409 migration_in_progress → toast + close (history surfaces it).
  //   409 restore_in_progress → toast (do not close — user may want to
  //     wait and retry).
  //   422 token_invalid / other 422 → inline error.
  //   429 daily_cap_reached → toast with localized retry-at time.
  function handleStartError(err: unknown) {
    if (!(err instanceof HttpError)) {
      setConfirmError(parseServerError(err, t("groups:migration.toastStartFailed")))
      return
    }
    const code = getServerErrorCode(err)
    const meta = getServerErrorMeta(err)
    if (err.status === 409) {
      if (code === "currency_migration.preview_expired") {
        toast.error(t("errors:currencyMigrationPreviewExpired"))
        // Drop the stale token and walk the user back to step 2 so
        // they can re-issue a preview from a fresh rate input.
        setPreview(null)
        setConfirmInput("")
        setStep(2)
        return
      }
      if (code === "currency_migration.state_changed") {
        toast.error(t("errors:currencyMigrationStateChanged"))
        setPreview(null)
        setConfirmInput("")
        setStep(2)
        return
      }
      if (code === "currency_migration.migration_in_progress") {
        toast.error(t("groups:migration.toastInProgress"))
        close()
        return
      }
      if (code === "currency_migration.restore_in_progress") {
        toast.error(t("errors:currencyMigrationRestoreInProgress"))
        return
      }
    }
    if (err.status === 422) {
      if (code === "currency_migration.token_invalid") {
        setConfirmError(t("errors:currencyMigrationTokenInvalid"))
        return
      }
      setConfirmError(parseServerError(err, t("groups:migration.toastStartFailed")))
      return
    }
    if (err.status === 429 && code === "currency_migration.daily_cap_reached") {
      const retryAtText = formatRetryAfter(meta?.retry_after_seconds)
      toast.error(
        t("errors:currencyMigrationDailyCapReached", {
          retry_at_local_time: retryAtText,
        })
      )
      close()
      return
    }
    setConfirmError(parseServerError(err, t("groups:migration.toastStartFailed")))
  }

  return (
    <DialogContent className="sm:max-w-2xl" data-testid="migrate-currency-dialog" data-step={step}>
      <DialogHeader>
        <DialogTitle>{t("groups:settings.dialog.title")}</DialogTitle>
        <DialogDescription>{t("groups:settings.migrateCurrencyHelp")}</DialogDescription>
      </DialogHeader>

      <WizardSteps step={step} />

      {step === 1 && (
        <StepTarget
          current={fromCurrency}
          value={toCurrency}
          onChange={setToCurrency}
          sameAsCurrent={!!sameAsCurrent}
        />
      )}

      {step === 2 && (
        <StepRate
          from={fromCurrency}
          to={toCurrency}
          rateInput={rateInput}
          onRateChange={setRateInput}
          errorMessage={previewError}
          isLoading={previewMutation.isPending}
        />
      )}

      {step === 3 && preview && (
        <StepPreview
          preview={preview}
          from={fromCurrency}
          to={toCurrency}
          secondsLeft={secondsLeft}
          expired={previewExpired}
        />
      )}

      {step === 4 && preview && (
        <StepConfirm
          groupName={groupName}
          confirmInput={confirmInput}
          onConfirmChange={setConfirmInput}
          errorMessage={confirmError}
        />
      )}

      <DialogFooter className="flex flex-row items-center justify-between gap-2 sm:justify-between">
        <div>
          {step > 1 && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="gap-1.5"
              onClick={() => setStep((step - 1) as Step)}
              disabled={previewMutation.isPending || startMutation.isPending}
              data-testid="wizard-back"
            >
              <ArrowLeft className="size-3.5" aria-hidden="true" />
              {t("groups:settings.dialog.back")}
            </Button>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="ghost"
            onClick={close}
            disabled={previewMutation.isPending || startMutation.isPending}
            data-testid="wizard-cancel"
          >
            {t("groups:settings.dialog.cancel")}
          </Button>
          {step === 1 && (
            <Button
              type="button"
              disabled={!toCurrency || !!sameAsCurrent}
              onClick={() => setStep(2)}
              data-testid="wizard-next"
              className="gap-1.5"
            >
              {t("groups:settings.dialog.continue")}
              <ArrowRight className="size-3.5" aria-hidden="true" />
            </Button>
          )}
          {step === 2 && (
            <Button
              type="button"
              disabled={rateValue == null || previewMutation.isPending}
              onClick={handlePreview}
              data-testid="wizard-preview"
              className="gap-1.5"
            >
              {previewMutation.isPending && (
                <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
              )}
              {t("groups:settings.dialog.continue")}
              {!previewMutation.isPending && <ArrowRight className="size-3.5" aria-hidden="true" />}
            </Button>
          )}
          {step === 3 && (
            <Button
              type="button"
              disabled={previewExpired}
              onClick={() => setStep(4)}
              data-testid="wizard-confirm"
              className="gap-1.5"
            >
              {t("groups:settings.dialog.continue")}
              <ArrowRight className="size-3.5" aria-hidden="true" />
            </Button>
          )}
          {step === 4 && (
            <Button
              type="button"
              disabled={
                startMutation.isPending || previewExpired || confirmInput.trim() !== groupName
              }
              onClick={handleStart}
              data-testid="wizard-submit"
              className="gap-1.5"
            >
              {startMutation.isPending && (
                <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
              )}
              {startMutation.isPending
                ? t("groups:settings.dialog.starting")
                : t("groups:settings.dialog.submit")}
            </Button>
          )}
        </div>
      </DialogFooter>
    </DialogContent>
  )
}

function WizardSteps({ step }: { step: Step }) {
  const { t } = useTranslation()
  const items: Array<{ index: Step; titleKey: string }> = [
    { index: 1, titleKey: "groups:settings.dialog.stepTarget" },
    { index: 2, titleKey: "groups:settings.dialog.stepRate" },
    { index: 3, titleKey: "groups:settings.dialog.stepPreview" },
    { index: 4, titleKey: "groups:settings.dialog.stepConfirm" },
  ]
  return (
    <ol
      className="flex flex-wrap items-center gap-2 text-xs"
      data-testid="wizard-steps"
      aria-label="wizard"
    >
      {items.map((item, idx) => {
        const active = item.index === step
        const done = item.index < step
        return (
          <li key={item.index} className="flex items-center gap-2">
            <span
              className={cn(
                "inline-flex size-5 items-center justify-center rounded-full border text-[11px] font-semibold",
                active && "border-primary bg-primary text-primary-foreground",
                done && !active && "border-primary/40 bg-primary/10 text-primary",
                !active && !done && "border-muted text-muted-foreground"
              )}
              data-testid={`wizard-step-${item.index}`}
              data-active={active || undefined}
            >
              {item.index}
            </span>
            <span className={cn("text-xs", active ? "font-medium" : "text-muted-foreground")}>
              {t(item.titleKey)}
            </span>
            {idx < items.length - 1 && (
              <span aria-hidden="true" className="px-1 text-muted-foreground">
                /
              </span>
            )}
          </li>
        )
      })}
    </ol>
  )
}

function StepTarget({
  current,
  value,
  onChange,
  sameAsCurrent,
}: {
  current: string
  value: string
  onChange: (next: string) => void
  sameAsCurrent: boolean
}) {
  const { t } = useTranslation()
  return (
    <div className="space-y-4 py-2">
      <div className="space-y-1.5">
        <Label>{t("groups:settings.dialog.currentCurrencyLabel")}</Label>
        <Input
          value={current}
          readOnly
          disabled
          className="font-mono uppercase"
          data-testid="wizard-current-currency"
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="wizard-target-currency">{t("groups:settings.dialog.targetLabel")}</Label>
        <CurrencyCombobox
          id="wizard-target-currency"
          value={value}
          onChange={onChange}
          ariaInvalid={sameAsCurrent}
        />
        <p className="text-[11px] text-muted-foreground">
          {t("groups:settings.dialog.targetHelp")}
        </p>
        {sameAsCurrent ? (
          <p className="text-xs text-destructive" data-testid="wizard-target-same-error">
            {t("groups:settings.dialog.targetSameAsCurrent", { current })}
          </p>
        ) : null}
      </div>
    </div>
  )
}

function StepRate({
  from,
  to,
  rateInput,
  onRateChange,
  errorMessage,
  isLoading,
}: {
  from: string
  to: string
  rateInput: string
  onRateChange: (next: string) => void
  errorMessage: string | null
  isLoading: boolean
}) {
  const { t } = useTranslation()
  const numeric = parseRate(rateInput)
  return (
    <div className="space-y-4 py-2">
      <div className="space-y-1.5">
        <Label htmlFor="wizard-rate">{t("groups:settings.dialog.rateLabel")}</Label>
        <Input
          id="wizard-rate"
          // text + inputMode="decimal" instead of type="number" because
          // some browsers normalize/forbid intermediate states like a
          // trailing dot (e.g. "1.") on numeric inputs, which would
          // undermine the truncateRateInput parser. The parser owns
          // sanitization; the BE owns final validation. Mobile keyboards
          // still get the numeric pad via inputMode="decimal".
          type="text"
          inputMode="decimal"
          autoComplete="off"
          disabled={isLoading}
          value={rateInput}
          onChange={(e) => onRateChange(truncateRateInput(e.target.value))}
          aria-invalid={rateInput.length > 0 && numeric == null}
          data-testid="wizard-rate-input"
        />
        <p className="text-[11px] text-muted-foreground">
          {t("groups:settings.dialog.rateHelp", { from, to })} —{" "}
          {t("groups:settings.dialog.rateMaxDecimalsHint")}
        </p>
        {rateInput.length > 0 && numeric == null ? (
          <p className="text-xs text-destructive" data-testid="wizard-rate-error">
            {t("groups:settings.dialog.rateRequired")}
          </p>
        ) : null}
      </div>
      {errorMessage ? (
        <Alert variant="destructive" data-testid="wizard-preview-error">
          <AlertDescription>{errorMessage}</AlertDescription>
        </Alert>
      ) : null}
    </div>
  )
}

function StepPreview({
  preview,
  from,
  to,
  secondsLeft,
  expired,
}: {
  preview: MigrationPreview
  from: string
  to: string
  secondsLeft: number
  expired: boolean
}) {
  const { t } = useTranslation()
  const totalBefore = preview.total_current_before ?? 0
  const totalAfter = preview.total_current_after ?? 0
  const top = topDeltas(preview.diffs)
  const commodityCount = preview.commodity_count ?? 0
  // No "% change" column in this card. Before and after are denominated
  // in DIFFERENT currencies, so a numeric delta or percentage between
  // them is conceptually meaningless — it would read as "the inventory
  // got cheaper" when nothing actually changed in real value, just the
  // unit of measurement. The per-row top-5 deltas below show both
  // values side by side so users can sanity-check the rate they
  // entered without inviting that misread.
  return (
    <div className="space-y-4 py-2" data-testid="wizard-preview-body">
      <p className="text-sm text-muted-foreground">
        {t("groups:settings.dialog.previewCommodityCount", { count: commodityCount })}
      </p>
      <div className="grid grid-cols-2 gap-3 rounded-lg border bg-muted/20 p-3 text-sm">
        <div>
          <p className="text-[11px] uppercase tracking-wide text-muted-foreground">
            {t("groups:settings.dialog.previewTotalBefore")}
          </p>
          <p className="font-mono" data-testid="wizard-total-before">
            {formatCurrency(totalBefore, from)}
          </p>
        </div>
        <div>
          <p className="text-[11px] uppercase tracking-wide text-muted-foreground">
            {t("groups:settings.dialog.previewTotalAfter")}
          </p>
          <p className="font-mono" data-testid="wizard-total-after">
            {formatCurrency(totalAfter, to)}
          </p>
        </div>
      </div>

      {top.length > 0 ? (
        <div className="space-y-1.5">
          <p className="text-xs font-medium text-muted-foreground">
            {t("groups:settings.dialog.previewTopDeltasTitle")}
          </p>
          <ul className="rounded-md border divide-y" data-testid="wizard-top-deltas">
            {top.map((d, i) => (
              <li
                key={d.commodity_id ?? d.commodity_name ?? `delta-${i}`}
                className="flex items-center justify-between gap-3 px-3 py-2 text-sm"
              >
                <span className="truncate">{d.commodity_name ?? "—"}</span>
                <span className="font-mono text-xs text-muted-foreground">
                  {formatCurrency(d.current_price_before ?? 0, from)}
                  {" → "}
                  {formatCurrency(d.current_price_after ?? 0, to)}
                </span>
              </li>
            ))}
          </ul>
        </div>
      ) : (
        <p className="text-xs text-muted-foreground">
          {t("groups:settings.dialog.previewNoDiffs")}
        </p>
      )}

      <p
        className={cn("text-xs", expired ? "text-destructive" : "text-muted-foreground")}
        data-testid="wizard-preview-countdown"
      >
        {expired
          ? t("groups:settings.dialog.previewExpired")
          : t("groups:settings.dialog.previewExpiresIn", {
              value: formatCountdown(secondsLeft),
            })}
      </p>
    </div>
  )
}

function StepConfirm({
  groupName,
  confirmInput,
  onConfirmChange,
  errorMessage,
}: {
  groupName: string
  confirmInput: string
  onConfirmChange: (next: string) => void
  errorMessage: string | null
}) {
  const { t } = useTranslation()
  return (
    <div className="space-y-4 py-2">
      <p className="text-sm text-muted-foreground">
        {t("groups:settings.dialog.confirmHelp", { name: groupName })}
      </p>
      <div className="space-y-1.5">
        <Label htmlFor="wizard-confirm-name">
          {t("groups:settings.deleteDialog.confirmWordLabel")}
        </Label>
        <Input
          id="wizard-confirm-name"
          autoComplete="off"
          placeholder={groupName}
          value={confirmInput}
          onChange={(e) => onConfirmChange(e.target.value)}
          aria-invalid={!!errorMessage}
          data-testid="wizard-confirm-input"
        />
      </div>
      {errorMessage ? (
        <Alert variant="destructive" data-testid="wizard-confirm-error">
          <AlertDescription>{errorMessage}</AlertDescription>
        </Alert>
      ) : null}
    </div>
  )
}

// Renders meta.retry_after_seconds (a string per the JSON:API meta
// contract) as a localized time-of-day. Falls back to the raw seconds
// when parsing fails or the BE didn't include the meta — the toast
// still tells the user to wait, just less precisely.
export function formatRetryAfter(secondsRaw: string | undefined): string {
  if (!secondsRaw) return "—"
  const seconds = Number(secondsRaw)
  if (!Number.isFinite(seconds) || seconds <= 0) return "—"
  const target = new Date(Date.now() + seconds * 1000)
  return target.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" })
}
