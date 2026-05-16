import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react"

import { KeyboardShortcutsDialog } from "./KeyboardShortcutsDialog"

interface KeyboardShortcutsContextValue {
  // True while the cheat-sheet modal is open.
  open: boolean
  // Imperative open/close used by Settings → Help → Keyboard shortcuts
  // (and any future surface that wants a click-to-open button).
  setOpen: (open: boolean) => void
  // Convenience toggle wired to the global `?` listener.
  toggle: () => void
}

const Context = createContext<KeyboardShortcutsContextValue | undefined>(undefined)

interface KeyboardShortcutsProviderProps {
  children: ReactNode
}

// Mounts at the Shell root so every authenticated page can:
//   1. open the cheat sheet via `?` (Shift + /) — handled here,
//   2. trigger it imperatively via `useKeyboardShortcutsDialog().setOpen(true)`.
//
// The `?` listener follows the same skip-when-typing rule the CommandPalette
// applies for Cmd+K: if the focused element is an `<input>`, `<textarea>`,
// `<select>`, or a contenteditable, the keypress is left alone so users can
// type the literal character into the field.
export function KeyboardShortcutsProvider({ children }: KeyboardShortcutsProviderProps) {
  const [open, setOpen] = useState(false)

  const toggle = useCallback(() => setOpen((prev) => !prev), [])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      // We only react to a bare `?`. Modifier-laden combos like Cmd+? or
      // Shift+Alt+? are intentionally ignored so we don't conflict with
      // browser/OS shortcuts that include the same physical key.
      if (event.key !== "?") return
      if (event.metaKey || event.ctrlKey || event.altKey) return
      if (isEditableTarget(event.target)) return
      event.preventDefault()
      setOpen((prev) => !prev)
    }
    window.addEventListener("keydown", onKeyDown)
    return () => window.removeEventListener("keydown", onKeyDown)
  }, [])

  const value = useMemo<KeyboardShortcutsContextValue>(
    () => ({ open, setOpen, toggle }),
    [open, toggle]
  )

  return (
    <Context.Provider value={value}>
      {children}
      <KeyboardShortcutsDialog open={open} onOpenChange={setOpen} />
    </Context.Provider>
  )
}

// Consumed by any surface that wants to open the cheat sheet
// imperatively — most importantly the Settings → Help → Keyboard
// shortcuts row. Throws when called outside the provider so we surface
// integration bugs early instead of silently no-oping.
export function useKeyboardShortcutsDialog(): KeyboardShortcutsContextValue {
  const ctx = useContext(Context)
  if (!ctx) {
    throw new Error("useKeyboardShortcutsDialog must be used inside <KeyboardShortcutsProvider>")
  }
  return ctx
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable) return true
  const tag = target.tagName
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT"
}
