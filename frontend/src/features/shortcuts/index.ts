// Public surface for the shortcuts feature (#1385).
//
//   - KeyboardShortcutsProvider — mount once at the Shell root; owns the
//     dialog state and the global `?` keydown listener.
//   - useKeyboardShortcutsDialog — hook used by Settings (and future
//     surfaces) to open the cheat sheet imperatively.
//   - SHORTCUTS / formatCombo / etc. — exported for tests and for any
//     UI that wants to render the registry inline (e.g. a future
//     onboarding tour step).
export { KeyboardShortcutsDialog } from "./KeyboardShortcutsDialog"
export { KeyboardShortcutsProvider, useKeyboardShortcutsDialog } from "./KeyboardShortcutsProvider"
export {
  CATEGORY_ORDER,
  SHORTCUTS,
  detectIsMac,
  formatCombo,
  type ShortcutCategoryKey,
  type ShortcutDef,
} from "./registry"
