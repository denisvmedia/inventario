import { toast } from "sonner"

// Thin wrapper around `sonner.toast` so the call sites read consistently
// across the app (and so a future swap of the toast library only touches
// this file). Keeps the same surface as the legacy `useAppToast`
// composable so porting a feature is a one-import change.
//
// Each variant returns the toast id that sonner produces, so callers can
// hold on to it for `dismiss(id)` or `update(id, ...)` later — losing the
// return type would silently break those flows.
//
// The hook itself is stateless — sonner manages its own toast queue — and
// is shaped as a hook so callers can keep `const { success } = useAppToast()`
// out of habit and we can later add hook-aware behavior (per-user copy
// formatting, per-route gating) without churning every caller.
export interface AppToastApi {
  success: (
    message: Parameters<typeof toast.success>[0],
    options?: Parameters<typeof toast.success>[1]
  ) => ReturnType<typeof toast.success>
  error: (
    message: Parameters<typeof toast.error>[0],
    options?: Parameters<typeof toast.error>[1]
  ) => ReturnType<typeof toast.error>
  info: (
    message: Parameters<typeof toast.info>[0],
    options?: Parameters<typeof toast.info>[1]
  ) => ReturnType<typeof toast.info>
  warning: (
    message: Parameters<typeof toast.warning>[0],
    options?: Parameters<typeof toast.warning>[1]
  ) => ReturnType<typeof toast.warning>
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
