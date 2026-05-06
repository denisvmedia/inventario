// Global "something blew up" toast for HTTP 5xx responses (issue #1210). Fires
// from TanStack Query's QueryCache/MutationCache onError so any silently failed
// request — query OR mutation — still surfaces a notification, even if the
// caller didn't wire its own `onError`.
//
// 4xx is intentionally ignored: those are application-level (validation,
// permissions, conflicts) and the call site usually maps them to the right
// inline message. 401 is handled inside http.ts (refresh + redirect). Network
// failures show up as a non-HttpError thrown by fetch and are also ignored
// here so e.g. an aborted request during navigation stays quiet.
//
// Per-mutation/per-query opt-out: set `meta: { suppressGlobalErrorToast: true }`
// when the caller already shows a richer error message and a generic toast
// would just duplicate it.
import { toast } from "sonner"

import { i18next } from "@/i18n"

import { HttpError } from "./http"
import { parseServerError } from "./server-error"

export interface GlobalErrorToastMeta {
  suppressGlobalErrorToast?: boolean
}

export function notifyGlobalServerError(error: unknown, meta: unknown): void {
  if (!(error instanceof HttpError)) return
  if (error.status < 500) return
  if (isMetaSuppressed(meta)) return
  const fallback = i18next.t("errors:global.server", {
    defaultValue: "Server error. Please try again later.",
  })
  toast.error(parseServerError(error, fallback))
}

function isMetaSuppressed(meta: unknown): boolean {
  if (!meta || typeof meta !== "object") return false
  return (meta as GlobalErrorToastMeta).suppressGlobalErrorToast === true
}
