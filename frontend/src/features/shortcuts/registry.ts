// Central registry of every keyboard shortcut surfaced by the app.
// KeyboardShortcutsDialog renders this array — adding a new entry here
// is the one and only step needed to make a shortcut show up in the
// cheat sheet (#1385). The actual key listener still lives in whichever
// component owns the behavior (the registry is a display contract, not
// a binding mechanism), so the convention is: wire the listener in the
// component AND add a matching descriptor here.
//
// Combo notation:
//   - "Mod"           → ⌘ on macOS, Ctrl on every other platform.
//   - "Shift", "Alt", "Meta", "Ctrl" → literal modifiers (rendered with
//     platform-appropriate glyphs by formatCombo).
//   - Single-character literals are uppercased ("k" → "K"). Special keys
//     ("Esc", "Enter", "Tab", "ArrowUp", "/") render as-is or with their
//     conventional glyph.
//   - Chord steps are space-separated: `"g h"` means "press G, then H".
//   - Modifier-and-key chords use `+`: `"Mod+K"`, `"Mod+Shift+P"`.

export type ShortcutCategoryKey = "navigation" | "actions" | "search" | "help" | "layout"

export interface ShortcutDef {
  // Stable identifier — used as a React key and a test-id hook.
  id: string
  // Combo string in the notation above.
  combo: string
  // i18n key for the action label (fully namespaced, e.g.
  // "common:shortcuts.entries.openPalette"). Resolved at render time by
  // the dialog so platform users see translated copy.
  labelKey: string
  // Category bucket used to group entries in the cheat sheet.
  categoryKey: ShortcutCategoryKey
}

// The catalog. Ordered roughly by likely-discovery frequency: search
// surfaces first (Cmd+K is the headline shortcut), then layout, then
// help. Order within a category controls the row order in the dialog.
export const SHORTCUTS: readonly ShortcutDef[] = [
  {
    id: "command-palette.open",
    combo: "Mod+K",
    labelKey: "common:shortcuts.entries.openPalette",
    categoryKey: "search",
  },
  {
    id: "sidebar.toggle",
    combo: "Mod+B",
    labelKey: "common:shortcuts.entries.toggleSidebar",
    categoryKey: "layout",
  },
  {
    id: "shortcuts.show",
    combo: "?",
    labelKey: "common:shortcuts.entries.showShortcuts",
    categoryKey: "help",
  },
] as const

// Category render order is independent of registry-insertion order so
// the dialog UI is stable when entries get added or reordered.
export const CATEGORY_ORDER: readonly ShortcutCategoryKey[] = [
  "search",
  "navigation",
  "actions",
  "layout",
  "help",
]

// Detect whether the user is on a Mac-family device. We treat iPad and
// iPhone the same as desktop macOS for the purposes of glyph rendering
// even though those platforms don't expose a physical Command key — the
// underlying assumption is that anyone reading this dialog on a
// touchscreen is on a hardware keyboard or AssistiveTouch where ⌘ is
// the intuitive symbol.
export function detectIsMac(): boolean {
  if (typeof navigator === "undefined") return false
  return /Mac|iPhone|iPad|iPod/i.test(navigator.userAgent)
}

// Split a combo string into chord steps, then each chord into the
// individual keys it requires. The returned shape is
// `chord[][]` so the dialog can render multi-step chords like
// "g h" as two separate `<kbd>` groups joined by a "then" separator.
export function formatCombo(combo: string, isMac: boolean): string[][] {
  return combo.split(" ").map((chord) => chord.split("+").map((key) => formatKey(key, isMac)))
}

function formatKey(key: string, isMac: boolean): string {
  switch (key) {
    case "Mod":
      return isMac ? "⌘" : "Ctrl"
    case "Shift":
      return isMac ? "⇧" : "Shift"
    case "Alt":
      return isMac ? "⌥" : "Alt"
    case "Meta":
      return isMac ? "⌘" : "Meta"
    case "Ctrl":
      return isMac ? "⌃" : "Ctrl"
    case "Enter":
      return isMac ? "↵" : "Enter"
    case "ArrowUp":
      return "↑"
    case "ArrowDown":
      return "↓"
    case "ArrowLeft":
      return "←"
    case "ArrowRight":
      return "→"
    default:
      return key.length === 1 ? key.toUpperCase() : key
  }
}
