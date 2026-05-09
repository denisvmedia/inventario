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
