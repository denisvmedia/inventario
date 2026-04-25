import { onBeforeUnmount, onMounted } from 'vue'

/**
 * Modifier keys recognised by {@link useKeyboardShortcuts}. The
 * `mod` key matches EITHER `metaKey` or `ctrlKey`, so a single
 * binding covers Cmd+K on macOS, Ctrl+K on Windows / Linux, and
 * the various host-vs-browser-platform mismatches that show up under
 * Playwright (chromium / firefox sometimes report `navigator.platform
 * === 'Linux'` even on a macOS host while still firing `metaKey` for
 * `Meta`-keyed presses; treating them as equivalent sidesteps the
 * `isMac()` guess entirely). The explicit `meta` / `ctrl` modifiers
 * stay strict if a caller really wants one or the other.
 */
export type ShortcutModifier = 'mod' | 'ctrl' | 'meta' | 'alt' | 'shift'

export interface ShortcutBinding {
  /** Lower-cased `KeyboardEvent.key` (e.g. `'k'`, `'/'`, `'enter'`). */
  key: string
  /** Modifier keys that must all be held. Order does not matter. */
  modifiers?: ShortcutModifier[]
  /**
   * Handler invoked when the shortcut matches. Receives the original
   * `KeyboardEvent` so consumers can call `preventDefault()` /
   * `stopPropagation()` if needed.
   */
  handler: (_event: KeyboardEvent) => void
  /**
   * When false (default), shortcuts fire even if the active element is
   * an input / textarea / contenteditable. Set true to suppress them
   * inside text fields. Cmd+K / Ctrl+K typically wants false so the
   * palette stays reachable while focus is in an input.
   */
  ignoreInInput?: boolean
}

function modifiersMatch(event: KeyboardEvent, mods: ShortcutModifier[] | undefined): boolean {
  const want = new Set(mods ?? [])
  const wantAlt = want.has('alt')
  const wantShift = want.has('shift')

  // `mod` is the platform-agnostic modifier — accept EITHER metaKey or
  // ctrlKey when the binding asks for it. `navigator.platform` is not a
  // reliable platform signal across browsers (chromium / firefox under
  // Playwright on macOS still report 'Linux' / 'Win32' depending on the
  // device emulation), so treat them as equivalent and don't gate on
  // `isMac()`. Explicit `meta` / `ctrl` modifiers stay strict.
  if (want.has('mod')) {
    const modPressed = !!event.metaKey || !!event.ctrlKey
    if (!modPressed) return false
    // Other modifiers still must match exactly.
    return (
      !!event.altKey === wantAlt &&
      !!event.shiftKey === wantShift
    )
  }

  const wantMeta = want.has('meta')
  const wantCtrl = want.has('ctrl')

  return (
    !!event.metaKey === wantMeta &&
    !!event.ctrlKey === wantCtrl &&
    !!event.altKey === wantAlt &&
    !!event.shiftKey === wantShift
  )
}

function isInsideInput(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  return target.isContentEditable
}

/**
 * Register a list of keyboard shortcuts for the lifetime of the
 * calling component. Bindings are attached to `window` so they fire
 * regardless of which element currently has focus (subject to
 * `ignoreInInput` per binding).
 *
 * The composable is intentionally stateless — multiple components can
 * register their own shortcut sets without coordinating; each binding
 * runs its own handler when the keystroke matches.
 */
export function useKeyboardShortcuts(bindings: ShortcutBinding[]): void {
  function onKeyDown(event: KeyboardEvent) {
    for (const binding of bindings) {
      if (event.key.toLowerCase() !== binding.key.toLowerCase()) continue
      if (!modifiersMatch(event, binding.modifiers)) continue
      if (binding.ignoreInInput && isInsideInput(event.target)) continue
      binding.handler(event)
    }
  }

  onMounted(() => {
    window.addEventListener('keydown', onKeyDown)
  })
  onBeforeUnmount(() => {
    window.removeEventListener('keydown', onKeyDown)
  })
}
