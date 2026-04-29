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
