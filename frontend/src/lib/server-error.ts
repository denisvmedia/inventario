// Tiny adapter that turns a thrown HttpError (or anything else) into a
// human-readable string the auth pages can render under the form.
//
// Backend error envelopes seen in the wild:
//   - JSON:API errors:     { errors: [{ detail: "…" }, …] }
//   - Plain object error:  { error: "…", message: "…" }
//   - Plain string body:   "Email already taken"
//
// All three reduce to a single message — the auth surface doesn't (yet)
// surface per-field validation, so we don't bother building a field map.
import { HttpError } from "./http"

interface JsonApiError {
  detail?: string
  title?: string
  code?: string
  meta?: Record<string, string>
}

interface ErrorEnvelope {
  errors?: JsonApiError[]
  error?: string
  message?: string
}

function pickEnvelopeMessage(env: ErrorEnvelope): string | null {
  const first = env.errors?.[0]
  if (first?.detail) return first.detail
  if (first?.title) return first.title
  if (env.error) return env.error
  if (env.message) return env.message
  return null
}

// Returns a user-facing message. Pass `fallback` for the copy to use when no
// useful detail can be extracted (e.g. a 5xx with a stringified HTML page).
export function parseServerError(err: unknown, fallback: string): string {
  if (err instanceof HttpError) {
    const data = err.data
    if (typeof data === "string") {
      const trimmed = data.trim()
      if (trimmed) return trimmed
    } else if (data && typeof data === "object") {
      const msg = pickEnvelopeMessage(data as ErrorEnvelope)
      if (msg) return msg
    }
    return fallback
  }
  if (err instanceof Error && err.message) return err.message
  return fallback
}

// Discriminated kind so banners can switch headline + retry affordance
// without each caller re-deriving the classification.
//
//   - network    fetch threw before any HTTP response (offline, DNS,
//                CORS preflight failed, etc.). Always safe to retry.
//   - validation BE rejected the payload (400 / 422). User has to edit
//                inputs — retry is *not* offered.
//   - conflict   stale write / unique violation (409 / 412 / 423).
//                User has to reconcile — retry is not offered.
//   - unknown    everything else (403, 404, 5xx, parser errors).
//                Retry is offered since the cause may be transient.
export type ServerErrorKind = "network" | "validation" | "conflict" | "unknown"

export interface ClassifiedServerError {
  kind: ServerErrorKind
  message: string
}

// Maps a thrown value to a kind + the same user-facing message
// `parseServerError` would return. Network errors are anything that
// isn't an HttpError (the http layer only constructs HttpError when a
// real HTTP response came back); validation/conflict are decided by
// status code. The message body still wins over the kind's generic
// copy when one is present — kind is a hint for affordances (Retry
// button visibility, title copy), not a replacement for the BE's
// detail string.
export function classifyServerError(err: unknown, fallback: string): ClassifiedServerError {
  if (!(err instanceof HttpError)) {
    const msg = err instanceof Error && err.message ? err.message : fallback
    return { kind: "network", message: msg }
  }
  const message = parseServerError(err, fallback)
  if (err.status === 400 || err.status === 422) {
    return { kind: "validation", message }
  }
  if (err.status === 409 || err.status === 412 || err.status === 423) {
    return { kind: "conflict", message }
  }
  return { kind: "unknown", message }
}

// Convenience for banner components — validation and conflict need
// user action before a re-submit could succeed, so the Retry button
// is hidden in those cases. Network/unknown are retried in place.
export function isRetryableKind(kind: ServerErrorKind): boolean {
  return kind === "network" || kind === "unknown"
}

// Extracts the JSON:API error code from the first `errors[]` entry, if
// present. Returns null otherwise. Lets callers branch on stable
// machine-readable codes (e.g. "currency_migration.preview_expired") rather
// than parsing the human-readable detail string.
export function getServerErrorCode(err: unknown): string | null {
  if (!(err instanceof HttpError)) return null
  const data = err.data
  if (!data || typeof data !== "object") return null
  const first = (data as ErrorEnvelope).errors?.[0]
  return first?.code ?? null
}

// Extracts the JSON:API error `meta` object from the first `errors[]`
// entry. Values are strings on the wire (the BE serializes the meta map
// with `swaggertype:"object,string"`); callers parse the known keys
// (e.g. `retry_after_seconds`, `migration_id`, `status`) into their
// real types.
export function getServerErrorMeta(err: unknown): Record<string, string> | null {
  if (!(err instanceof HttpError)) return null
  const data = err.data
  if (!data || typeof data !== "object") return null
  const first = (data as ErrorEnvelope).errors?.[0]
  return first?.meta ?? null
}
