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
import { getServerErrorCode, parseServerError } from "./server-error"

export interface GlobalErrorToastMeta {
  suppressGlobalErrorToast?: boolean
}

export function notifyGlobalServerError(error: unknown, meta: unknown): void {
  if (!(error instanceof HttpError)) return
  if (isMetaSuppressed(meta)) return

  // 423 Locked is raised by the requireGroupNotMigrating middleware on
  // any commodity write (and by the symmetric in-handler check on
  // restore start) when a currency migration is in flight. Surface it
  // globally so a user who sneaks past the disabled CTA — race between
  // a colleague starting a migration and our group-detail refetch —
  // still gets a clear, translated message instead of a silent failure.
  if (error.status === 423) {
    toast.error(
      i18next.t("errors:lockedDuringMigration", {
        defaultValue: "Commodity changes are paused while a currency migration runs.",
      })
    )
    return
  }

  if (error.status < 500) return

  // Skip our own currency-migration codes — the wizard renders them
  // inline and these strings already carry their own user-facing copy
  // through the per-call onError handler. Belt-and-braces; the wizard
  // uses meta.suppressGlobalErrorToast already.
  const code = getServerErrorCode(error)
  if (code?.startsWith("currency_migration.")) return

  const fallback = i18next.t("errors:global.server", {
    defaultValue: "Server error. Please try again later.",
  })
  toast.error(parseServerError(error, fallback))
}

function isMetaSuppressed(meta: unknown): boolean {
  if (!meta || typeof meta !== "object") return false
  return (meta as GlobalErrorToastMeta).suppressGlobalErrorToast === true
}
