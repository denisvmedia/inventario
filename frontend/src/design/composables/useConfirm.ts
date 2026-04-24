import { useConfirmationStore } from '@/stores/confirmationStore'

/**
 * Visual flavour of the confirm button. Mirrors the values accepted
 * by the existing `confirmationStore.show`, keeping the composable
 * source-compatible with callers that migrate from
 * `utils/confirmationUtil`.
 */
export type ConfirmButtonClass =
  | 'primary'
  | 'danger'
  | 'warning'
  | 'success'
  | 'secondary'

/**
 * Options accepted by the {@link AppConfirm.confirm} call.
 */
export interface ConfirmOptions {
  title?: string
  message?: string
  confirmLabel?: string
  cancelLabel?: string
  confirmButtonClass?: ConfirmButtonClass
}

/**
 * Shape returned by {@link useConfirm}.
 */
export interface AppConfirm {
  /**
   * Open the confirmation dialog. Resolves to `true` when the user
   * accepts and `false` when they cancel or dismiss it.
   */
  confirm: (_options?: ConfirmOptions) => Promise<boolean>

  /**
   * Preset for destructive actions. Produces a red "Delete" button
   * and a pre-filled message that mentions the entity type.
   */
  confirmDelete: (_itemType: string, _options?: ConfirmOptions) => Promise<boolean>
}

/**
 * Composable facade over the legacy `confirmationStore`. Mirrors the
 * `useAppToast` style so views that move off PrimeVue can reach for
 * a single `useXxx` helper per concern. The composable keeps the
 * legacy store as its only backing today; Phase 2 swaps it for the
 * shadcn-vue `AlertDialog` primitive without touching call-sites.
 */
export function useConfirm(): AppConfirm {
  const store = useConfirmationStore()

  const confirm = (options: ConfirmOptions = {}): Promise<boolean> =>
    store.show(options)

  const confirmDelete = (itemType: string, options: ConfirmOptions = {}): Promise<boolean> =>
    store.show({
      title: options.title ?? 'Confirm Delete',
      message: options.message ?? `Are you sure you want to delete this ${itemType}?`,
      confirmLabel: options.confirmLabel ?? 'Delete',
      cancelLabel: options.cancelLabel ?? 'Cancel',
      confirmButtonClass: options.confirmButtonClass ?? 'danger',
    })

  return { confirm, confirmDelete }
}
