import { useCallback, useEffect, useRef, useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

export interface UnsavedChangesGuardOptions {
  /** The form holds edits that are not saved. */
  dirty: boolean
  /**
   * Saves the edits. Resolves true when the save succeeded; false (for a
   * validation or server error the form now shows) keeps the user editing.
   */
  onSave: () => Promise<boolean>
}

export interface UnsavedChangesGuard {
  /**
   * Runs `leave` at once when nothing is unsaved. Otherwise asks first:
   * Save runs `leave` after a successful save, Discard runs it straight away,
   * Keep editing stays.
   */
  guard: (leave: () => void) => void
  /** The question dialog; render it once inside the page. */
  dialog: ReactNode
}

// Leaving a form with unsaved edits asks Save / Discard / Keep editing.
//
// It covers the ways out the app controls: the page's own buttons (wrap their
// navigation in `guard`), clicks on in-app links anywhere on the page (the
// sidebar, a breadcrumb, a Previous / Next link), and closing or reloading the
// tab (the browser's own prompt; it cannot offer Save). The browser's Back
// button is not covered: the app's router (BrowserRouter) cannot hold a
// history step back, so Back leaves without asking.
export function useUnsavedChangesGuard({
  dirty,
  onSave,
}: UnsavedChangesGuardOptions): UnsavedChangesGuard {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const leaveRef = useRef<(() => void) | null>(null)

  const guard = useCallback(
    (leave: () => void) => {
      if (!dirty) {
        leave()
        return
      }
      leaveRef.current = leave
      setOpen(true)
    },
    [dirty]
  )

  function finish(proceed: boolean) {
    const leave = leaveRef.current
    leaveRef.current = null
    setOpen(false)
    if (proceed) leave?.()
  }

  async function saveAndLeave() {
    setSaving(true)
    try {
      const ok = await onSave()
      finish(ok)
    } finally {
      setSaving(false)
    }
  }

  // In-app links outside the page's own buttons: hold the click, ask, then
  // follow the link. Capture phase, so the router's <Link> handler never runs
  // for a held click.
  useEffect(() => {
    if (!dirty) return
    function onClick(event: MouseEvent) {
      if (event.defaultPrevented || event.button !== 0) return
      if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
      const target = event.target instanceof Element ? event.target : null
      const anchor = target?.closest("a[href]")
      if (!(anchor instanceof HTMLAnchorElement)) return
      if ((anchor.target && anchor.target !== "_self") || anchor.hasAttribute("download")) return
      const url = new URL(anchor.href, window.location.href)
      if (url.origin !== window.location.origin) return
      const here = window.location
      if (url.pathname === here.pathname && url.search === here.search) return
      event.preventDefault()
      event.stopPropagation()
      guard(() => navigate(`${url.pathname}${url.search}${url.hash}`))
    }
    document.addEventListener("click", onClick, true)
    return () => document.removeEventListener("click", onClick, true)
  }, [dirty, guard, navigate])

  useEffect(() => {
    if (!dirty) return
    function onBeforeUnload(event: BeforeUnloadEvent) {
      event.preventDefault()
      // Older browsers only prompt when returnValue is set.
      event.returnValue = ""
    }
    window.addEventListener("beforeunload", onBeforeUnload)
    return () => window.removeEventListener("beforeunload", onBeforeUnload)
  }, [dirty])

  const dialog = (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !saving) finish(false)
      }}
    >
      <DialogContent className="sm:max-w-sm" data-testid="unsaved-changes-dialog">
        <DialogHeader>
          <DialogTitle>{t("common:unsavedChanges.title")}</DialogTitle>
          <DialogDescription>{t("common:unsavedChanges.description")}</DialogDescription>
        </DialogHeader>
        <DialogFooter className="gap-2">
          <Button
            type="button"
            variant="ghost"
            className="sm:mr-auto"
            onClick={() => finish(false)}
            disabled={saving}
            data-testid="unsaved-changes-keep-editing"
          >
            {t("common:unsavedChanges.keepEditing")}
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => finish(true)}
            disabled={saving}
            data-testid="unsaved-changes-discard"
          >
            {t("common:unsavedChanges.discard")}
          </Button>
          <Button
            type="button"
            onClick={saveAndLeave}
            disabled={saving}
            data-testid="unsaved-changes-save"
          >
            {t("common:unsavedChanges.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )

  return { guard, dialog }
}
