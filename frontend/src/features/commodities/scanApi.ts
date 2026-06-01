// AI-vision scan client for the Add Item dialog. The BE handler
// (apiserver/commodity_scan.go) accepts a multipart form with 1..5
// `photos` fields and returns structured field-by-field guesses with
// per-field confidence + an optional `warnings` array. This module
// is read-only: the scan does not persist any commodity, so we DON'T
// invalidate any React Query cache after a successful scan.
//
// Auth/CSRF/group-slug rewriting flows through the shared `http`
// wrapper the same way `uploadFile` (features/files/api.ts) does;
// passing a `FormData` body lets the browser populate the correct
// `multipart/form-data; boundary=…` Content-Type header.
//
// Errors are propagated as the `HttpError` the wrapper throws — the
// CommodityFormDialog feeds that through `classifyServerError` plus
// `getServerErrorCode` so the typed banner can branch on the BE's
// `commodity_scan.*` code strings (rate_limited / too_many_photos /
// photo_too_large / unsupported_mime / provider_disabled /
// provider_timeout / provider_error).
import { http } from "@/lib/http"

// One of the three possible value shapes the BE returns inside
// `fields.<name>.value`. Each is independently optional — the BE
// emits the key only when the model produced something.
export type ScanFieldValue = string | number | string[]

export interface ScanFieldGuess<T extends ScanFieldValue = ScanFieldValue> {
  value: T
  confidence: number
}

export interface ScanWarning {
  code: "low_confidence" | "unreadable_serial" | "ambiguous_price" | "currency_inferred" | string
  field?: string
  detail?: string
}

// The shape after we strip the JSON:API envelope. Keys match the BE's
// canonical AI-vision field set in `go/internal/aivision` (`FieldName*`
// constants / `AllFieldNames`). The review UI consumes `Partial<…>`
// because the BE may omit any field for which the model returned no
// confident guess.
export type ScanFieldName =
  | "name"
  | "short_name"
  | "type"
  | "original_price"
  | "original_price_currency"
  | "serial_number"
  | "urls"
  | "purchase_date"
  | "comments"

export interface ScanResultFields {
  name?: ScanFieldGuess<string>
  short_name?: ScanFieldGuess<string>
  type?: ScanFieldGuess<string>
  original_price?: ScanFieldGuess<number>
  original_price_currency?: ScanFieldGuess<string>
  serial_number?: ScanFieldGuess<string>
  urls?: ScanFieldGuess<string[]>
  purchase_date?: ScanFieldGuess<string>
  comments?: ScanFieldGuess<string>
}

export interface ScanResult {
  fields: ScanResultFields
  warnings: ScanWarning[]
}

// On-the-wire JSON:API envelope. Kept loose because the only contract
// the FE relies on is the `data.attributes` shape; everything else
// (links, meta, etc.) is ignored.
interface ScanResponseEnvelope {
  data?: {
    type?: string
    id?: string
    attributes?: {
      fields?: Partial<Record<ScanFieldName, { value?: unknown; confidence?: number } | null>>
      warnings?: ScanWarning[] | null
    }
  }
}

interface ScanCommodityPhotosOptions {
  photos: File[]
  signal?: AbortSignal
  // Optional free-form hint forwarded as the multipart `hint` field
  // (the BE pipes it into the provider prompt). Currently the dialog
  // doesn't surface a hint input — the param exists so future "tell
  // the AI what this is" affordances don't need a fresh signature.
  hint?: string
  // Anonymous landing-page flow (#1988). When true the request targets
  // the PUBLIC, unauthenticated `/public/commodities/scan` endpoint
  // instead of the group-scoped `/commodities/scan`, and we opt out of
  // the http client's `/g/{slug}/` rewrite (there is no active group
  // before login). The BE returns the identical scan-result shape, so
  // `normalizeScanResponse` handles both paths unchanged. The public
  // route is gated behind the `public_scan` feature flag — when off it
  // 404s, which the typed error banner surfaces.
  anonymous?: boolean
}

// scanCommodityPhotos issues a multipart POST with the picked images.
// In the authenticated flow it hits /g/{slug}/commodities/scan (the
// group slug is resolved by the shared http wrapper from the active
// GroupContext). In the anonymous flow (#1988) it hits the public
// /public/commodities/scan with the group rewrite skipped. Throws an
// HttpError on any non-2xx so the CommodityFormDialog can route through
// classifyServerError / getServerErrorCode.
export async function scanCommodityPhotos(opts: ScanCommodityPhotosOptions): Promise<ScanResult> {
  const form = new FormData()
  for (const file of opts.photos) {
    // The BE handler reads the `photos` slice from the multipart form
    // and accepts repeated entries with the same field name. Don't
    // index the key — Go's `FormFile`/`File` only sees one entry per
    // name unless they're all sent under the same key.
    form.append("photos", file, file.name)
  }
  if (opts.hint?.trim()) {
    form.append("hint", opts.hint.trim())
  }
  const path = opts.anonymous ? `/public/commodities/scan` : `/commodities/scan`
  const body = await http.post<ScanResponseEnvelope>(path, form, {
    signal: opts.signal,
    // The public path is un-prefixed (no /g/{slug}/); skip the rewrite so
    // the request lands on /api/v1/public/commodities/scan verbatim.
    skipGroupRewrite: opts.anonymous,
  })
  return normalizeScanResponse(body)
}

// normalizeScanResponse strips the JSON:API envelope and drops fields
// only when the BE omitted the value entirely (null/undefined). Missing
// confidence is normalized to 0 so callers can still review otherwise-
// usable values.
// Exported for tests so a recorded BE fixture can be normalized
// without round-tripping through `http`.
export function normalizeScanResponse(env: ScanResponseEnvelope): ScanResult {
  const attrs = env.data?.attributes
  const rawFields = attrs?.fields ?? {}
  const fields: ScanResultFields = {}
  for (const key of Object.keys(rawFields) as ScanFieldName[]) {
    const guess = rawFields[key]
    if (!guess) continue
    const value = guess.value
    if (value === null || value === undefined) continue
    const confidence = typeof guess.confidence === "number" ? guess.confidence : 0
    // Type-narrowing happens at the read sites; keeping `value` as
    // `unknown` here would force every consumer through a `String(...)`
    // dance. The BE schema guarantees the shape per-field so a
    // structural cast is safe — `value` is one of string | number |
    // string[] per the OpenAPI spec.
    assignField(fields, key, value, confidence)
  }
  return {
    fields,
    warnings: Array.isArray(attrs?.warnings) ? attrs.warnings : [],
  }
}

function assignField(
  out: ScanResultFields,
  key: ScanFieldName,
  value: unknown,
  confidence: number
): void {
  switch (key) {
    case "name":
    case "short_name":
    case "type":
    case "original_price_currency":
    case "serial_number":
    case "purchase_date":
    case "comments":
      if (typeof value === "string") {
        out[key] = { value, confidence }
      }
      break
    case "original_price":
      if (typeof value === "number" && Number.isFinite(value)) {
        out[key] = { value, confidence }
      } else if (typeof value === "string" && value.trim() !== "") {
        // Some providers return prices as strings; coerce here so the
        // review row gets a plain number to format. Use Number.isFinite
        // (not !isNaN) so "Infinity"/"-Infinity" — which Number() parses
        // happily — don't survive into the form as an unusable value.
        const n = Number(value)
        if (Number.isFinite(n)) {
          out[key] = { value: n, confidence }
        }
      }
      break
    case "urls":
      if (Array.isArray(value)) {
        const strs = value.filter((u): u is string => typeof u === "string" && u.trim() !== "")
        if (strs.length > 0) out[key] = { value: strs, confidence }
      }
      break
  }
}
