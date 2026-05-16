import { describe, expect, it } from "vitest"

import {
  CATEGORY_ORDER,
  SHORTCUTS,
  type ShortcutCategoryKey,
  formatCombo,
} from "@/features/shortcuts"

describe("shortcuts/registry", () => {
  it("contains the three shortcuts the cheat sheet ships with at MVP", () => {
    const ids = SHORTCUTS.map((s) => s.id)
    expect(ids).toEqual(["command-palette.open", "sidebar.toggle", "shortcuts.show"])
  })

  it("guarantees every entry uses a category from CATEGORY_ORDER", () => {
    const known = new Set<ShortcutCategoryKey>(CATEGORY_ORDER)
    for (const entry of SHORTCUTS) {
      expect(known).toContain(entry.categoryKey)
    }
  })

  it("uses unique stable ids per entry", () => {
    const ids = SHORTCUTS.map((s) => s.id)
    expect(new Set(ids).size).toBe(ids.length)
  })

  describe("formatCombo", () => {
    it("renders Mod as ⌘ on Mac", () => {
      expect(formatCombo("Mod+K", true)).toEqual([["⌘", "K"]])
    })

    it("renders Mod as Ctrl on non-Mac", () => {
      expect(formatCombo("Mod+K", false)).toEqual([["Ctrl", "K"]])
    })

    it("uppercases single-character keys", () => {
      expect(formatCombo("Mod+b", false)).toEqual([["Ctrl", "B"]])
    })

    it("leaves long key names alone", () => {
      expect(formatCombo("Esc", false)).toEqual([["Esc"]])
    })

    it("splits chord sequences on whitespace", () => {
      expect(formatCombo("g h", false)).toEqual([["G"], ["H"]])
    })

    it("renders modifier glyphs differently per platform", () => {
      expect(formatCombo("Shift+Alt+P", true)).toEqual([["⇧", "⌥", "P"]])
      expect(formatCombo("Shift+Alt+P", false)).toEqual([["Shift", "Alt", "P"]])
    })

    it("maps arrow keys to glyphs on both platforms", () => {
      expect(formatCombo("ArrowUp", false)).toEqual([["↑"]])
      expect(formatCombo("ArrowDown", true)).toEqual([["↓"]])
    })

    it("renders a bare '?' as the literal character", () => {
      expect(formatCombo("?", false)).toEqual([["?"]])
    })
  })
})
