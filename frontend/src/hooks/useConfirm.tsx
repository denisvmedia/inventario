import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react"
import { useTranslation } from "react-i18next"

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"

export interface ConfirmOptions {
  title: string
  description?: string
  confirmLabel?: string
  cancelLabel?: string
  destructive?: boolean
}

interface ConfirmContextValue {
  confirm: (options: ConfirmOptions) => Promise<boolean>
}

const ConfirmContext = createContext<ConfirmContextValue | undefined>(undefined)

// ConfirmProvider mounts a single Dialog at the app root and exposes a
// promise-based `confirm()` API matching the legacy `confirmationStore`
// contract. Resolving with `true` means the user clicked the confirm
// button; anything that closes the dialog otherwise (cancel, click outside,
// Escape) resolves with `false` so callers don't have to handle a
// rejected-promise edge case.
//
// We use the generic `Dialog` (not `AlertDialog`) deliberately: confirms
// here can be cancelled by clicking outside or pressing Escape, which is
// the Dialog contract; AlertDialog forces an explicit choice and is
// reserved for "irreversible" prompts that aren't always our shape.
//
// Mounting at the root rather than per-call keeps focus traps and
// scroll-locks predictable: there's only one dialog instance, so there's
// only one focus stack to reason about. If `confirm()` is called while a
// previous dialog is still open, the previous resolver is settled with
// `false` (treated as a cancel) before the new dialog takes over — that
// way every promise returned by `confirm()` is guaranteed to settle.
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const [options, setOptions] = useState<ConfirmOptions | null>(null)
  const { t } = useTranslation()
  // The promise's `resolve` is held in a ref so the close handlers can call
  // it without rebinding — the surrounding state changes (open/options)
  // would otherwise re-create the resolver every render and lose the
  // pending call.
  const resolverRef = useRef<((value: boolean) => void) | null>(null)

  const confirm = useCallback((next: ConfirmOptions) => {
    // Settle a previous in-flight confirm with `false` before replacing the
    // resolver. Without this, calling `confirm()` while a dialog is already
    // open would silently leak the first promise — it would never resolve.
    if (resolverRef.current) {
      resolverRef.current(false)
      resolverRef.current = null
    }
    setOptions(next)
    setOpen(true)
    return new Promise<boolean>((resolve) => {
      resolverRef.current = resolve
    })
  }, [])

  const close = useCallback((value: boolean) => {
    setOpen(false)
    resolverRef.current?.(value)
    resolverRef.current = null
  }, [])

  const value = useMemo(() => ({ confirm }), [confirm])

  return (
    <ConfirmContext.Provider value={value}>
      {children}
      <Dialog
        open={open}
        onOpenChange={(next) => {
          if (!next) close(false)
        }}
      >
        <DialogContent data-testid="confirm-dialog">
          <DialogHeader>
            <DialogTitle>{options?.title}</DialogTitle>
            {options?.description ? (
              <DialogDescription>{options.description}</DialogDescription>
            ) : null}
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => close(false)} data-testid="confirm-cancel">
              {options?.cancelLabel ?? t("common:actions.cancel")}
            </Button>
            <Button
              variant={options?.destructive ? "destructive" : "default"}
              onClick={() => close(true)}
              data-testid="confirm-accept"
            >
              {options?.confirmLabel ?? t("common:actions.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ConfirmContext.Provider>
  )
}

export function useConfirm(): ConfirmContextValue["confirm"] {
  const ctx = useContext(ConfirmContext)
  if (!ctx) throw new Error("useConfirm must be used within a ConfirmProvider")
  return ctx.confirm
}
