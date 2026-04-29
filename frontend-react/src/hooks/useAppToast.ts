import { toast } from "sonner"

// Thin wrapper around `sonner.toast` so the call sites read consistently
// across the app (and so a future swap of the toast library only touches
// this file). Keeps the same surface as the legacy `useAppToast`
// composable so porting a feature is a one-import change.
//
// The hook itself is stateless — sonner manages its own toast queue — and
// is shaped as a hook so callers can keep `const { success } = useAppToast()`
// out of habit and we can later add hook-aware behavior (per-user copy
// formatting, per-route gating) without churning every caller.
export interface AppToastApi {
  success: (message: string, options?: Parameters<typeof toast.success>[1]) => void
  error: (message: string, options?: Parameters<typeof toast.error>[1]) => void
  info: (message: string, options?: Parameters<typeof toast.info>[1]) => void
  warning: (message: string, options?: Parameters<typeof toast.warning>[1]) => void
  promise: typeof toast.promise
  dismiss: typeof toast.dismiss
}

export function useAppToast(): AppToastApi {
  return {
    success: (message, options) => toast.success(message, options),
    error: (message, options) => toast.error(message, options),
    info: (message, options) => toast.info(message, options),
    warning: (message, options) => toast.warning(message, options),
    promise: toast.promise.bind(toast),
    dismiss: toast.dismiss.bind(toast),
  }
}
