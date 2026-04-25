import { toast as sonnerToast, type ExternalToast } from 'vue-sonner'

/**
 * Typed facade over `vue-sonner` that is the only supported way for
 * application code to surface toasts once the strangler-fig migration
 * (Epic #1324) moves a caller off PrimeVue's `useToast`.
 *
 * Keeping this tiny wrapper gives us three things:
 *   - a single seam to swap `vue-sonner` for something else later,
 *   - ergonomic `error(err)` that accepts either an `Error` or a string
 *     (PrimeVue call-sites currently pass both shapes),
 *   - a neutral `ToastOptions` alias so views don't import from the
 *     underlying library directly and we can ban `vue-sonner` imports
 *     outside this module in PR 0.8 (see imports-and-bans.md).
 */
export type ToastOptions = ExternalToast

/**
 * Shape returned by {@link useAppToast}. Exposed separately so
 * components that stash the facade on their instance can type the field.
 */
export interface AppToast {
  success: (_title: string, _opts?: ToastOptions) => string | number
  error: (_err: Error | string, _opts?: ToastOptions) => string | number
  warning: (_title: string, _opts?: ToastOptions) => string | number
  info: (_title: string, _opts?: ToastOptions) => string | number
  dismiss: (_id?: string | number) => void
}

export function useAppToast(): AppToast {
  return {
    success: (title, opts) => sonnerToast.success(title, opts),
    error: (err, opts) =>
      sonnerToast.error(typeof err === 'string' ? err : err.message, opts),
    warning: (title, opts) => sonnerToast.warning(title, opts),
    info: (title, opts) => sonnerToast.info(title, opts),
    dismiss: (id) => sonnerToast.dismiss(id),
  }
}
