import { onBeforeUnmount, onMounted } from 'vue'

/**
 * Modifier keys recognised by {@link useKeyboardShortcuts}. The
 * `mod` key is platform-aware: it matches `metaKey` on macOS and
 * `ctrlKey` everywhere else, so a single binding covers both
 * Cmd+K (Mac) and Ctrl+K (Windows / Linux).
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

const isMac = (): boolean => {
  if (typeof navigator === 'undefined') return false
  return /(Mac|iPhone|iPad|iPod)/.test(navigator.platform || navigator.userAgent || '')
}

function modifiersMatch(event: KeyboardEvent, mods: ShortcutModifier[] | undefined): boolean {
  const want = new Set(mods ?? [])
  const wantMeta = want.has('meta') || (want.has('mod') && isMac())
  const wantCtrl = want.has('ctrl') || (want.has('mod') && !isMac())
  const wantAlt = want.has('alt')
  const wantShift = want.has('shift')

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
