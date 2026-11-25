import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react"

import { CommandPalette } from "@/components/CommandPalette"

interface CommandPaletteContextValue {
  open: boolean
  // `opener` is the element focus returns to when the palette closes. It
  // defaults to the element focused at opening; a click passes its button,
  // because Safari does not focus a button on click.
  setOpen: (open: boolean, opener?: HTMLElement) => void
}

const Context = createContext<CommandPaletteContextValue | undefined>(undefined)

// Spread on every element that opens the palette (the TopBar's search
// entries).
export const commandPaletteTriggerProps = { "data-command-palette-trigger": "" } as const

// Owns the command palette's open state, so the palette opens from two
// places: Cmd/Ctrl+K anywhere, and the search entry in the TopBar (a click
// or a tap, for people who do not know the shortcut or have no keyboard).
export function CommandPaletteProvider({ children }: { children: ReactNode }) {
  const [open, setOpenState] = useState(false)
  const openRef = useRef(false)
  // Each opening mounts a new palette. One reopened during its closing
  // animation would keep the previous query, skip moving focus to its
  // input, and end up under its own overlay.
  const [session, setSession] = useState(0)
  // The element focused when the palette opened. The palette has no Radix
  // trigger, so without this Radix would drop focus on the body at close.
  const returnFocus = useRef<HTMLElement | null>(null)

  const setOpen = useCallback((next: boolean, opener?: HTMLElement) => {
    if (next && !openRef.current) {
      const active = document.activeElement
      returnFocus.current =
        opener ?? (active instanceof HTMLElement && active !== document.body ? active : null)
      setSession((n) => n + 1)
    }
    openRef.current = next
    setOpenState(next)
  }, [])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const isShortcut =
        (event.key === "k" || event.key === "K") && (event.metaKey || event.ctrlKey)
      if (!isShortcut) return
      event.preventDefault()
      setOpen(!openRef.current)
    }
    window.addEventListener("keydown", onKeyDown)
    return () => window.removeEventListener("keydown", onKeyDown)
  }, [setOpen])

  // A result that navigated away may have taken the element with it; focus
  // then stays where the new page puts it. A palette replaced by a new one
  // reports its close late, after the new one has opened; focus stays in
  // the new one.
  const onCloseAutoFocus = useCallback((event: Event) => {
    if (openRef.current) return
    const target = returnFocus.current
    returnFocus.current = null
    if (!target?.isConnected) return
    event.preventDefault()
    target.focus()
  }, [])

  // Radix reports a click outside the dialog at the end of the click. A
  // click on a search entry during the closing animation reopens the
  // palette first and would then close it again as an outside click. A
  // trigger is under the overlay while the palette is open, so it never
  // needs to close it.
  const onInteractOutside = useCallback((event: Event) => {
    const target = event.target
    if (target instanceof Element && target.closest("[data-command-palette-trigger]")) {
      event.preventDefault()
    }
  }, [])

  const value = useMemo<CommandPaletteContextValue>(() => ({ open, setOpen }), [open, setOpen])

  return (
    <Context.Provider value={value}>
      {children}
      <CommandPalette
        key={session}
        open={open}
        onOpenChange={setOpen}
        onCloseAutoFocus={onCloseAutoFocus}
        onInteractOutside={onInteractOutside}
      />
    </Context.Provider>
  )
}

// Used by the TopBar's search entry. Throws outside the provider so a
// missing provider shows up at once instead of as a dead button.
export function useCommandPalette(): CommandPaletteContextValue {
  const ctx = useContext(Context)
  if (!ctx) throw new Error("useCommandPalette must be used inside <CommandPaletteProvider>")
  return ctx
}
