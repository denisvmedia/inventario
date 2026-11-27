import { toast } from "sonner"

import { globalErrorToastMessage } from "./global-error-toast"
import { parseServerError } from "./server-error"

// toastMutationError reports a failed action (a delete, a revoke, a settings
// toggle) in one toast. The server's message wins, and fallback is shown only
// when the response carries none, so a refused 409 or 422 tells the user why.
//
// An error the global toast already reports (a 5xx, a 423, a request that
// never reached the server) is left to it, so one failure shows one toast.
// That holds only for a request sent through a TanStack mutation or query
// that keeps the global toast: call this from a mutation's onError or from a
// catch around mutateAsync. A mutation with meta.suppressGlobalErrorToast
// shows every error itself, with toast.error(parseServerError(err, fallback)).
export function toastMutationError(error: unknown, fallback: string): void {
  if (globalErrorToastMessage(error) !== null) return
  toast.error(parseServerError(error, fallback))
}
