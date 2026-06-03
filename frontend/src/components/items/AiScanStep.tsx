// AI vision scan step for the Add Item dialog (#1720). Replaces the
// inert offer-only stub with a four-phase state machine:
//
//   offer    → user drops/picks 1..5 photos or PDFs, clicks "Scan files"
//   scanning → mutation in flight; user can Cancel via AbortController
//   review   → per-field checkboxes + confidence chips + warnings;
//              "Use these values" prefills the wizard and advances
//   error    → ServerErrorBanner over the offer phase with the BE's
//              typed `commodity_scan.*` code rendered as detail
//
// Anatomy mirrors `design-mocks/src/components/AddItemDialog.tsx`
// L789-L856 for the offer phase, L744-L758 for scanning, and
// L761-L786 for review. The mock's "AI extracted the following"
// header + grid pattern is preserved; the design deviation tracker
// note line is removed (the feature ships now).
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import {
  AlertCircle,
  Camera,
  CheckCircle2,
  FileText,
  Plus,
  ScanText,
  Sparkles,
  X,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { currencyMeta } from "@/lib/currency-meta"
import {
  classifyServerError,
  getServerErrorCode,
  type ClassifiedServerError,
} from "@/lib/server-error"
import { http } from "@/lib/http"
import { cn } from "@/lib/utils"
import { COMMODITY_TYPES, type CommodityTypeValue } from "@/features/commodities/constants"
import { useScanCommodityPhotos } from "@/features/commodities/scanHooks"
import type {
  ScanFieldGuess,
  ScanFieldName,
  ScanResult,
  ScanWarning,
} from "@/features/commodities/scanApi"

// Mirrors `services.AllowedMIMETypes` on the BE. The `<input accept>`
// attribute is a hint the browser may honour loosely (HEIC files on
// Android Chrome come back with empty MIME types), so we re-check at
// staging time and reject anything unrecognised inline. `application/pdf`
// (#1983) lets the user prefill from a receipt / invoice / manual, not
// just a product photo.
const ACCEPTED_MIME = new Set([
  "image/jpeg",
  "image/jpg",
  "image/png",
  "image/webp",
  "image/heic",
  "image/heif",
  "application/pdf",
])
const ACCEPTED_EXT = /\.(jpe?g|png|webp|heic|heif|pdf)$/i
// Hard cap (mirrors BE) — additional rejections fire from the BE side.
const MAX_PHOTOS = 5

// Mirrors the commodity form's short_name limit (schemas.ts + the Go model's
// validation.Length(1, 40)). An AI-guessed short_name longer than this is
// truncated on accept so it never trips the Basics-step validator.
const SHORT_NAME_MAX_LEN = 40

// isPdfFile reports whether a staged file is a PDF document rather than an
// image. PDFs can't be rendered as an <img> thumbnail, so the staged tile
// shows a document icon + filename instead. The MIME check is primary; the
// extension fallback covers browsers that hand back an empty `type`.
function isPdfFile(file: File): boolean {
  return file.type.toLowerCase() === "application/pdf" || /\.pdf$/i.test(file.name)
}

interface StagedPhoto {
  id: string
  file: File
  // Object-URL preview for image thumbnails. Empty for PDFs — they render
  // as a document tile, so no bitmap URL is created (and nothing to revoke).
  preview: string
}

export type ScanAcceptedField = ScanFieldName

// ScanAcceptMeta carries follow-up notes the caller can surface on
// the next step. Today the only signal is a currency ISO the model
// guessed but that isn't in the known-currencies list — the caller
// can render a one-line banner so the user knows why their currency
// picker reset to its default.
export interface ScanAcceptMeta {
  droppedCurrency?: string | null
}

export interface ScanAcceptedValues {
  // Subset of the form schema. Values are already coerced to the
  // shape the form expects — strings for date/currency, string for
  // price (the schema stores numerics as strings), and string[] for
  // urls. The parent caller spreads these into `setValue` calls.
  name?: string
  short_name?: string
  type?: CommodityTypeValue
  original_price?: string
  original_price_currency?: string
  serial_number?: string
  urls?: string[]
  purchase_date?: string
  warranty_expires_at?: string
  comments?: string
}

interface AiScanStepProps {
  // Active group slug. Required by `useScanCommodityPhotos` (read-only
  // dependency today; threaded through for symmetry with other
  // group-scoped mutations).
  slug: string
  // Tenant's preferred currency ISO. Used as the fallback if the
  // currencies list hasn't loaded by the time the user clicks "Use
  // these values" — keeps the wizard usable when the BE is reachable
  // for the scan endpoint but the (separate) /currencies endpoint is
  // slow or unmocked in a test.
  defaultCurrency: string
  // Called when the user clicks "Use these values" — receives only
  // the checked fields, already coerced into form-shape, plus a meta
  // object the caller can use to surface follow-up notes on the next
  // step (e.g. a dropped unknown-currency code). Caller is
  // responsible for `setValue`-ing each into RHF and advancing the
  // wizard to Basics.
  onAccept: (values: ScanAcceptedValues, meta?: ScanAcceptMeta) => void
  // Called when the user clicks "Fill manually" (offer/review/error)
  // — caller advances to Basics without any prefill.
  onSkip: () => void
  // Anonymous landing-page flow (#1988). When true the scan POSTs to the
  // public, unauthenticated /public/commodities/scan endpoint (group
  // rewrite skipped) instead of the group-scoped one. Same response
  // shape; the review/accept UI is unchanged.
  anonymous?: boolean
}

// AiScanStep ports the full mock AI phase, with the dropzone wired
// to a real file input + the scanning/review/error states backed by
// the `useScanCommodityPhotos` mutation. The state machine lives
// here, not in CommodityFormDialog, so a future redesign that moves
// the AI surface to a separate route can lift this component as-is.
export function AiScanStep({
  slug,
  defaultCurrency,
  onAccept,
  onSkip,
  anonymous = false,
}: AiScanStepProps) {
  const { t } = useTranslation()
  const [photos, setPhotos] = useState<StagedPhoto[]>([])
  const [stagingError, setStagingError] = useState<string | null>(null)
  const [result, setResult] = useState<ScanResult | null>(null)
  const [acceptedFields, setAcceptedFields] = useState<Set<ScanFieldName>>(new Set())
  const [serverError, setServerError] = useState<ClassifiedServerError | null>(null)
  const [errorCode, setErrorCode] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const scan = useScanCommodityPhotos(slug, anonymous)

  // Validate a model-guessed currency against the server's REAL supported
  // list (the same `/currencies` endpoint + ["currencies"] cache key the
  // CurrencyCombobox uses), not just the tenant default. The old behaviour
  // accepted a guess only when it equalled the default and dropped every
  // other valid ISO — so scanning a CZK invoice under a USD/EUR group
  // showed "not on the supported list" even though CZK is fully supported.
  //
  // The query is LAZY (`enabled` only once a file is staged) so it never
  // fires on the AI-step mount path — unrelated wizard tests that walk
  // straight to "Fill manually" don't trip the MSW unhandled-request guard.
  // It's skipped entirely in the anonymous landing flow, where there is no
  // group/auth to resolve `/currencies` against; there (and while the query
  // is still loading) we fall back to the tenant default so the set is never
  // empty.
  const currenciesQuery = useQuery<string[]>({
    queryKey: ["currencies"],
    queryFn: ({ signal }) => http.get<string[]>("/currencies", { signal }),
    enabled: !anonymous && photos.length > 0,
    staleTime: 5 * 60 * 1000,
  })
  const knownCurrencies = useMemo(() => {
    const supported = currenciesQuery.data
    return supported && supported.length > 0 ? supported : [defaultCurrency]
  }, [currenciesQuery.data, defaultCurrency])

  // Revoke object-URLs on unmount. We stash the live list of previews
  // in a ref so the cleanup only fires once at unmount — a `[photos]`
  // dep would revoke still-staged previews on every list change, and
  // the next render's `<img src=…>` would read an already-revoked URL
  // (Safari + iOS Chrome surface this as a broken image silently).
  // Individual previews are also revoked at the call site in
  // `removePhoto` so a removed thumbnail's bitmap is freed eagerly.
  const photosRef = useRef<StagedPhoto[]>(photos)
  useEffect(() => {
    photosRef.current = photos
  }, [photos])
  useEffect(() => {
    return () => {
      for (const p of photosRef.current) URL.revokeObjectURL(p.preview)
    }
  }, [])

  // Compute which currencies the BE accepts as a lowercased set so
  // the case-insensitive match below stays O(1).
  const currencySet = useMemo(
    () => new Set(knownCurrencies.map((c) => c.trim().toUpperCase())),
    [knownCurrencies]
  )

  const handleFiles = useCallback(
    (files: File[]) => {
      if (files.length === 0) return
      setStagingError(null)
      const rejectFile = (file: File): string | null => {
        const mime = file.type.toLowerCase()
        if (ACCEPTED_MIME.has(mime)) return null
        if (mime === "" && ACCEPTED_EXT.test(file.name)) return null
        return t("commodities:form.step.ai.errors.unsupportedMime")
      }
      const additions: StagedPhoto[] = []
      for (const file of files) {
        const reject = rejectFile(file)
        if (reject) {
          setStagingError(reject)
          continue
        }
        additions.push({
          id:
            typeof crypto !== "undefined" && crypto.randomUUID
              ? crypto.randomUUID()
              : `${file.name}-${file.size}-${file.lastModified}-${Math.random()}`,
          file,
          // No object URL for PDFs — they render as a document tile, not
          // an <img>, so there's no bitmap to create or revoke.
          preview: isPdfFile(file) ? "" : URL.createObjectURL(file),
        })
      }
      setPhotos((prev) => {
        const next = [...prev, ...additions]
        if (next.length > MAX_PHOTOS) {
          // Drop overflow; the BE rejects > 5 anyway, and trimming on
          // the FE keeps the dropzone visibly honest. Surface a hint
          // so the user knows why the picker swallowed some files.
          setStagingError(t("commodities:form.step.ai.errors.tooManyPhotos"))
          // Revoke previews that won't make it into state.
          for (const overflow of next.slice(MAX_PHOTOS)) URL.revokeObjectURL(overflow.preview)
          return next.slice(0, MAX_PHOTOS)
        }
        return next
      })
    },
    [t]
  )

  function removePhoto(id: string) {
    setPhotos((prev) => {
      const drop = prev.find((p) => p.id === id)
      if (drop) URL.revokeObjectURL(drop.preview)
      return prev.filter((p) => p.id !== id)
    })
    setStagingError(null)
  }

  function clearPhotos() {
    for (const p of photos) URL.revokeObjectURL(p.preview)
    setPhotos([])
    setStagingError(null)
  }

  async function runScan() {
    if (photos.length === 0 || scan.isPending) return
    setServerError(null)
    setErrorCode(null)
    const ac = new AbortController()
    abortRef.current = ac
    try {
      const r = await scan.mutateAsync({
        photos: photos.map((p) => p.file),
        signal: ac.signal,
      })
      setResult(r)
      // Default-accept every field with confidence ≥ 0.3 (the
      // low-confidence threshold for the chip styling — anything below
      // is the model essentially guessing). Users can re-check
      // individually in the review phase before applying.
      const defaults = new Set<ScanFieldName>()
      for (const key of Object.keys(r.fields) as ScanFieldName[]) {
        const guess = r.fields[key]
        if (guess && guess.confidence >= 0.3) defaults.add(key)
      }
      setAcceptedFields(defaults)
    } catch (err) {
      // AbortError fires when the user clicks Cancel — silently roll
      // back to the offer phase instead of rendering a scary banner.
      if (ac.signal.aborted) return
      setServerError(
        classifyServerError(err, t("commodities:form.step.ai.errors.providerError.title"))
      )
      setErrorCode(getServerErrorCode(err))
    } finally {
      abortRef.current = null
    }
  }

  function cancelScan() {
    abortRef.current?.abort()
    abortRef.current = null
    scan.reset()
  }

  function applyAccepted() {
    if (!result) return
    const out: ScanAcceptedValues = {}
    let droppedCurrency: string | null = null
    for (const key of Object.keys(result.fields) as ScanFieldName[]) {
      if (!acceptedFields.has(key)) continue
      const guess = result.fields[key]
      if (!guess) continue
      const value = guess.value
      switch (key) {
        case "name":
        case "serial_number":
        case "comments":
        case "purchase_date":
        case "warranty_expires_at":
          if (typeof value === "string" && value.trim() !== "") out[key] = value
          break
        case "short_name":
          if (typeof value === "string" && value.trim() !== "") {
            // Cap at the form's 40-char limit (SHORT_NAME_MAX_LEN) so an
            // over-long AI guess pre-fills truncated instead of failing
            // validation on the Basics step. The prompt already steers the
            // model to ≤40; this is the defensive backstop.
            out.short_name = value.trim().slice(0, SHORT_NAME_MAX_LEN)
          }
          break
        case "type":
          if (typeof value === "string" && isKnownType(value)) {
            out.type = value
          }
          break
        case "original_price":
          if (typeof value === "number" && Number.isFinite(value)) {
            // Schema stores numeric inputs as strings.
            out.original_price = String(value)
          }
          break
        case "original_price_currency":
          if (typeof value === "string") {
            const upper = value.trim().toUpperCase()
            if (currencySet.has(upper)) {
              out.original_price_currency = upper
            } else {
              // Captured so we can surface a one-line note in the
              // review banner before navigating to Basics — without
              // this the unknown ISO would silently disappear and the
              // user would wonder where their currency picker reset
              // to default.
              droppedCurrency = upper
            }
          }
          break
        case "urls":
          if (Array.isArray(value)) {
            const cleaned = value.filter((u) => typeof u === "string" && u.trim() !== "")
            if (cleaned.length > 0) out.urls = cleaned
          }
          break
      }
    }
    // Pass-through if no checked fields produced a value — the
    // wizard still advances to Basics so the user isn't stuck.
    onAccept(out, {
      droppedCurrency,
    })
    // Side-effect: clear staged photos so a re-take starts clean
    // (also relevant for the implicit revocation cleanup).
    clearPhotos()
    setResult(null)
    setAcceptedFields(new Set())
  }

  function retakePhotos() {
    clearPhotos()
    setResult(null)
    setAcceptedFields(new Set())
    setServerError(null)
    setErrorCode(null)
  }

  function toggleField(key: ScanFieldName, next: boolean) {
    setAcceptedFields((prev) => {
      const out = new Set(prev)
      if (next) out.add(key)
      else out.delete(key)
      return out
    })
  }

  const phase: "offer" | "scanning" | "review" | "error" = (() => {
    if (scan.isPending) return "scanning"
    if (result) return "review"
    if (serverError) return "error"
    return "offer"
  })()

  return (
    <div
      className="flex flex-col gap-4 py-2"
      data-testid="commodity-form-ai-step"
      data-ai-phase={phase}
    >
      {phase === "scanning" ? (
        <ScanningPanel onCancel={cancelScan} />
      ) : phase === "review" && result ? (
        <ReviewPanel
          result={result}
          acceptedFields={acceptedFields}
          onToggleField={toggleField}
          onAccept={applyAccepted}
          onRetake={retakePhotos}
          onSkip={onSkip}
          knownCurrencies={currencySet}
        />
      ) : (
        <OfferPanel
          photos={photos}
          stagingError={stagingError}
          onAdd={handleFiles}
          onRemove={removePhoto}
          onScan={runScan}
          onSkip={onSkip}
          serverError={serverError}
          errorCode={errorCode}
          onRetry={runScan}
          isRetrying={scan.isPending}
        />
      )}
    </div>
  )
}

// ---- Offer phase ----------------------------------------------------

interface OfferPanelProps {
  photos: StagedPhoto[]
  stagingError: string | null
  onAdd: (files: File[]) => void
  onRemove: (id: string) => void
  onScan: () => void
  onSkip: () => void
  serverError: ClassifiedServerError | null
  errorCode: string | null
  onRetry: () => void
  isRetrying: boolean
}

function OfferPanel({
  photos,
  stagingError,
  onAdd,
  onRemove,
  onScan,
  onSkip,
  serverError,
  errorCode,
  onRetry,
  isRetrying,
}: OfferPanelProps) {
  const { t } = useTranslation()
  const [dragActive, setDragActive] = useState(false)

  function pick() {
    document.getElementById("ai-photo-input")?.click()
  }

  // Override the banner title for the typed `commodity_scan.*` error
  // codes so the user sees the rate-limited / provider-disabled
  // headline instead of the generic "Something went wrong".
  const titleOverride = errorCodeTitle(errorCode, t)

  return (
    <>
      {serverError ? (
        <ServerErrorBanner
          error={serverError}
          onRetry={photos.length > 0 ? onRetry : undefined}
          isRetrying={isRetrying}
          testId="commodity-form-ai-error"
          titleOverride={titleOverride ?? undefined}
        />
      ) : null}

      <div className="grid grid-cols-2 gap-3">
        <div className="flex flex-col gap-2 rounded-xl border border-border bg-muted/20 p-3">
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary/10">
            <Camera aria-hidden="true" className="size-4 text-primary" />
          </div>
          <p className="text-xs font-semibold">{t("commodities:form.step.ai.fullItem.title")}</p>
          <p className="text-[11px] leading-relaxed text-muted-foreground">
            {t("commodities:form.step.ai.fullItem.description")}
          </p>
        </div>
        <div className="flex flex-col gap-2 rounded-xl border border-border bg-muted/20 p-3">
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary/10">
            <ScanText aria-hidden="true" className="size-4 text-primary" />
          </div>
          <p className="text-xs font-semibold">{t("commodities:form.step.ai.label.title")}</p>
          <p className="text-[11px] leading-relaxed text-muted-foreground">
            {t("commodities:form.step.ai.label.description")}
          </p>
        </div>
      </div>

      <div>
        {/* The dropzone itself is a button-shaped surface; making it a
            real <button> would forbid the nested <input type="file">
            child markup React expects. Keep the role/keyboard handler
            in lockstep so screen readers see it as an actionable
            target. */}
        <div
          role="button"
          tabIndex={0}
          aria-label={t("commodities:form.step.ai.dropzone.primary")}
          data-testid="commodity-form-ai-dropzone"
          className={cn(
            "flex flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed py-6 transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
            dragActive || photos.length > 0
              ? "border-primary/40 bg-primary/5"
              : "border-border hover:border-primary/40 hover:bg-muted/30"
          )}
          onClick={pick}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault()
              pick()
            }
          }}
          onDragOver={(e) => {
            e.preventDefault()
            setDragActive(true)
          }}
          onDragLeave={() => setDragActive(false)}
          onDrop={(e) => {
            e.preventDefault()
            setDragActive(false)
            onAdd(Array.from(e.dataTransfer.files))
          }}
        >
          {photos.length > 0 ? (
            <div className="flex flex-wrap justify-center gap-2 px-3">
              {photos.map((p) => (
                <div key={p.id} className="group relative" data-testid="commodity-form-ai-thumb">
                  {isPdfFile(p.file) ? (
                    <div
                      className="flex size-14 flex-col items-center justify-center gap-0.5 rounded-lg border border-border bg-muted/40 px-1"
                      title={p.file.name}
                      data-testid="commodity-form-ai-thumb-pdf"
                    >
                      <FileText aria-hidden="true" className="size-5 text-muted-foreground" />
                      <span className="w-full truncate text-center text-[8px] leading-tight text-muted-foreground">
                        {p.file.name}
                      </span>
                    </div>
                  ) : (
                    <img
                      src={p.preview}
                      alt={p.file.name}
                      className="size-14 rounded-lg border border-border object-cover"
                    />
                  )}
                  <button
                    type="button"
                    aria-label={t("commodities:form.step.ai.offer.staged.remove")}
                    className="absolute -top-1.5 -right-1.5 flex size-4 items-center justify-center rounded-full border border-border bg-background text-muted-foreground shadow-sm hover:text-foreground"
                    onClick={(e) => {
                      e.stopPropagation()
                      onRemove(p.id)
                    }}
                  >
                    <X aria-hidden="true" className="size-2.5" />
                  </button>
                </div>
              ))}
              {photos.length < MAX_PHOTOS ? (
                <button
                  type="button"
                  aria-label={t("commodities:form.step.ai.dropzone.primary")}
                  className="flex size-14 items-center justify-center rounded-lg border-2 border-dashed border-border bg-muted/30 text-muted-foreground transition-colors hover:border-primary/40 hover:text-foreground"
                  onClick={(e) => {
                    e.stopPropagation()
                    pick()
                  }}
                >
                  <Plus aria-hidden="true" className="size-4" />
                </button>
              ) : null}
            </div>
          ) : (
            <>
              <div className="flex size-10 items-center justify-center rounded-xl bg-amber-500/10">
                <Sparkles aria-hidden="true" className="size-5 text-amber-500" />
              </div>
              <p className="text-sm text-muted-foreground">
                {dragActive
                  ? t("commodities:form.step.ai.offer.dropzone.activeHint")
                  : t("commodities:form.step.ai.dropzone.primary")}
              </p>
              <p className="text-xs text-muted-foreground">
                {t("commodities:form.step.ai.dropzone.hint")}
              </p>
            </>
          )}
          <input
            id="ai-photo-input"
            type="file"
            multiple
            accept="image/jpeg,image/jpg,image/png,image/webp,image/heic,image/heif,application/pdf,.pdf"
            className="sr-only"
            data-testid="commodity-form-ai-file-input"
            onClick={(e) => e.stopPropagation()}
            onChange={(e) => {
              const files = Array.from(e.target.files ?? [])
              e.target.value = ""
              onAdd(files)
            }}
          />
        </div>
        {stagingError ? (
          <p
            className="mt-2 text-center text-xs text-destructive"
            data-testid="commodity-form-ai-staging-error"
            role="alert"
          >
            {stagingError}
          </p>
        ) : null}
      </div>

      {photos.length > 0 ? (
        <p className="text-center text-xs text-muted-foreground">
          {t("commodities:form.step.ai.offer.staged.title", { count: photos.length })}
        </p>
      ) : null}

      <div className="flex items-center justify-between gap-2">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={onSkip}
          data-testid="commodity-form-ai-fill-manually"
        >
          {t("commodities:form.fillManually")}
        </Button>
        <Button
          type="button"
          onClick={onScan}
          disabled={photos.length === 0 || isRetrying}
          className="gap-1.5"
          data-testid="commodity-form-ai-scan"
        >
          <Sparkles aria-hidden="true" className="size-3.5" />
          {t("commodities:form.step.ai.scanPhotos")}
        </Button>
      </div>
    </>
  )
}

// ---- Scanning phase -------------------------------------------------

function ScanningPanel({ onCancel }: { onCancel: () => void }) {
  const { t } = useTranslation()
  return (
    <div
      className="flex flex-col items-center justify-center gap-4 py-10 text-center"
      data-testid="commodity-form-ai-scanning"
    >
      <div className="relative flex size-14 items-center justify-center rounded-2xl bg-amber-500/10">
        <Sparkles aria-hidden="true" className="size-7 animate-pulse text-amber-500" />
      </div>
      <div>
        <p className="text-sm font-medium">{t("commodities:form.step.ai.scanning.title")}</p>
        <p className="mt-1 text-xs text-muted-foreground">
          {t("commodities:form.step.ai.scanning.subtitle")}
        </p>
      </div>
      <div className="h-1.5 w-full max-w-48 overflow-hidden rounded-full bg-muted">
        <div className="h-full w-2/3 animate-pulse rounded-full bg-amber-500" />
      </div>
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={onCancel}
        data-testid="commodity-form-ai-cancel"
      >
        {t("commodities:form.step.ai.scanning.cancel")}
      </Button>
    </div>
  )
}

// ---- Review phase ---------------------------------------------------

interface ReviewPanelProps {
  result: ScanResult
  acceptedFields: Set<ScanFieldName>
  onToggleField: (key: ScanFieldName, next: boolean) => void
  onAccept: () => void
  onRetake: () => void
  onSkip: () => void
  knownCurrencies: Set<string>
}

function ReviewPanel({
  result,
  acceptedFields,
  onToggleField,
  onAccept,
  onRetake,
  onSkip,
  knownCurrencies,
}: ReviewPanelProps) {
  const { t } = useTranslation()
  // Group warnings by field so each row can render its own inline
  // notes. Field-less warnings (the BE may emit `field: undefined`
  // for global issues) fall through to the top of the review block.
  const warningsByField = useMemo(() => {
    const map = new Map<string, ScanWarning[]>()
    const global: ScanWarning[] = []
    for (const w of result.warnings) {
      if (w.field) {
        const list = map.get(w.field) ?? []
        list.push(w)
        map.set(w.field, list)
      } else {
        global.push(w)
      }
    }
    return { map, global }
  }, [result.warnings])

  const fieldOrder: ScanFieldName[] = [
    "name",
    "short_name",
    "type",
    "serial_number",
    "purchase_date",
    "warranty_expires_at",
    "original_price",
    "original_price_currency",
    "urls",
    "comments",
  ]

  return (
    <>
      <div
        className="flex items-center gap-2 rounded-lg border border-status-active/20 bg-status-active/10 px-3 py-2.5"
        data-testid="commodity-form-ai-review"
      >
        <CheckCircle2 aria-hidden="true" className="size-4 shrink-0 text-status-active" />
        <p className="text-sm font-medium text-status-active">
          {t("commodities:form.step.ai.review.title")}
        </p>
      </div>
      <p className="text-xs text-muted-foreground">
        {t("commodities:form.step.ai.review.subtitle")}
      </p>

      {warningsByField.global.length > 0 ? (
        <Alert>
          <AlertCircle aria-hidden="true" className="size-4" />
          <AlertDescription>
            {warningsByField.global.map((w, i) => (
              <p key={i} className="text-xs">
                {warningMessage(w, t)}
              </p>
            ))}
          </AlertDescription>
        </Alert>
      ) : null}

      <div className="flex flex-col gap-2">
        {fieldOrder.map((key) => {
          const guess = result.fields[key]
          if (!guess) return null
          const warnings = warningsByField.map.get(key) ?? []
          const currencyUnknown =
            key === "original_price_currency" &&
            typeof guess.value === "string" &&
            !knownCurrencies.has(guess.value.toUpperCase())
          return (
            <ReviewRow
              key={key}
              fieldKey={key}
              guess={guess}
              checked={acceptedFields.has(key)}
              onToggle={(next) => onToggleField(key, next)}
              warnings={warnings}
              currencyUnknown={currencyUnknown}
            />
          )
        })}
      </div>

      <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={onSkip}
          className="mr-auto"
          data-testid="commodity-form-ai-fill-manually"
        >
          {t("commodities:form.fillManually")}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={onRetake}
          data-testid="commodity-form-ai-retake"
        >
          {t("commodities:form.step.ai.review.retake")}
        </Button>
        <Button
          type="button"
          size="sm"
          onClick={onAccept}
          className="gap-1.5"
          data-testid="commodity-form-ai-use-values"
        >
          <CheckCircle2 aria-hidden="true" className="size-3.5" />
          {t("commodities:form.step.ai.review.useValues")}
        </Button>
      </div>
    </>
  )
}

interface ReviewRowProps {
  fieldKey: ScanFieldName
  guess: ScanFieldGuess
  checked: boolean
  onToggle: (next: boolean) => void
  warnings: ScanWarning[]
  currencyUnknown: boolean
}

function ReviewRow({
  fieldKey,
  guess,
  checked,
  onToggle,
  warnings,
  currencyUnknown,
}: ReviewRowProps) {
  const { t } = useTranslation()
  const confidence = guess.confidence
  const confidenceBand: "high" | "medium" | "low" =
    confidence >= 0.75 ? "high" : confidence >= 0.5 ? "medium" : "low"
  const isLow = confidenceBand === "low"
  return (
    <div
      className="rounded-lg border border-border bg-muted/20 px-3 py-2.5"
      data-testid={`commodity-form-ai-row-${fieldKey}`}
      data-confidence={confidenceBand}
    >
      <label className="flex items-start gap-3">
        <Checkbox
          checked={checked}
          onCheckedChange={(next) => onToggle(next === true)}
          className="mt-0.5"
          data-testid={`commodity-form-ai-row-${fieldKey}-check`}
          aria-label={t(`commodities:fields.${fieldLabelKey(fieldKey)}`)}
        />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <p className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
              {t(`commodities:fields.${fieldLabelKey(fieldKey)}`)}
            </p>
            <Badge
              variant="secondary"
              className={cn(
                "text-[10px] font-medium",
                isLow ? "bg-destructive/10 text-destructive" : undefined
              )}
            >
              {t(`commodities:form.step.ai.review.confidence.${confidenceBand}`, {
                percent: Math.round(confidence * 100),
              })}
            </Badge>
          </div>
          <p
            className="mt-1 truncate text-sm font-medium"
            data-testid={`commodity-form-ai-row-${fieldKey}-value`}
          >
            {formatGuessValue(fieldKey, guess.value)}
          </p>
          {currencyUnknown ? (
            <p className="mt-1 text-[11px] text-muted-foreground">
              {t("commodities:form.step.ai.review.currencyInferred")}
            </p>
          ) : null}
          {warnings.length > 0 ? (
            <Alert className="mt-2 border-border bg-muted/30">
              <AlertCircle aria-hidden="true" className="size-4" />
              <AlertTitle className="text-xs font-semibold">
                {warningTitle(warnings[0].code, t)}
              </AlertTitle>
              <AlertDescription>
                {warnings.map((w, i) => (
                  <p key={i} className="text-[11px]">
                    {w.detail ?? w.code}
                  </p>
                ))}
              </AlertDescription>
            </Alert>
          ) : null}
        </div>
      </label>
    </div>
  )
}

// fieldLabelKey maps a scan-field name onto the existing
// `commodities:fields.*` translation tree. Keys differ in casing
// (`short_name` → `shortName`, `original_price_currency` →
// `originalPriceCurrencyHelp`-adjacent), so the label namespace gets
// its own lookup table.
function fieldLabelKey(field: ScanFieldName): string {
  switch (field) {
    case "name":
      return "name"
    case "short_name":
      return "shortName"
    case "type":
      return "type"
    case "serial_number":
      return "serialNumber"
    case "purchase_date":
      return "purchaseDate"
    case "original_price":
      return "originalPrice"
    case "original_price_currency":
      return "originalPriceHelp"
    case "urls":
      return "urls"
    case "warranty_expires_at":
      return "warrantyExpiresAt"
    case "comments":
      return "comments"
  }
}

function formatGuessValue(field: ScanFieldName, value: unknown): string {
  if (value === null || value === undefined) return "—"
  if (Array.isArray(value)) return value.join(", ")
  if (field === "original_price_currency" && typeof value === "string") {
    const meta = currencyMeta(value)
    return `${meta.code} — ${meta.name}`
  }
  if (field === "original_price" && typeof value === "number") {
    // Locale-agnostic two-decimal render — the BE rounds to two
    // decimals already; we just want to avoid trailing-zero noise.
    return value.toLocaleString(undefined, {
      minimumFractionDigits: 0,
      maximumFractionDigits: 2,
    })
  }
  return String(value)
}

function warningTitle(code: string, t: (k: string) => string): string {
  switch (code) {
    case "low_confidence":
      return t("commodities:form.step.ai.review.warning.lowConfidence")
    case "unreadable_serial":
      return t("commodities:form.step.ai.review.warning.unreadableSerial")
    case "ambiguous_price":
      return t("commodities:form.step.ai.review.warning.ambiguousPrice")
    case "currency_inferred":
      return t("commodities:form.step.ai.review.warning.currencyInferred")
    case "multiple_items":
      return t("commodities:form.step.ai.review.warning.multipleItems")
    default:
      return code
  }
}

// warningMessage renders a global (field-less) warning. Known codes get a
// localized message; everything else falls back to the provider's English
// detail (or the bare code). `multiple_items` (#1983) is the one the user
// most needs to read — the document had several products and only the most
// prominent one was pre-filled.
function warningMessage(w: ScanWarning, t: (k: string) => string): string {
  if (w.code === "multiple_items") {
    return t("commodities:form.step.ai.review.warning.multipleItems")
  }
  return w.detail ?? w.code
}

function errorCodeTitle(code: string | null, t: (k: string) => string): string | null {
  switch (code) {
    case "commodity_scan.rate_limited":
      return t("commodities:form.step.ai.errors.rateLimited.title")
    case "commodity_scan.too_many_photos":
      return t("commodities:form.step.ai.errors.tooManyPhotos")
    case "commodity_scan.photo_too_large":
      return t("commodities:form.step.ai.errors.photoTooLarge")
    case "commodity_scan.unsupported_mime":
      return t("commodities:form.step.ai.errors.unsupportedMime")
    case "commodity_scan.provider_disabled":
      return t("commodities:form.step.ai.errors.providerDisabled.title")
    case "commodity_scan.provider_timeout":
      return t("commodities:form.step.ai.errors.providerTimeout.title")
    case "commodity_scan.provider_error":
      return t("commodities:form.step.ai.errors.providerError.title")
    default:
      return null
  }
}

function isKnownType(value: string): value is CommodityTypeValue {
  return (COMMODITY_TYPES as readonly string[]).includes(value)
}
