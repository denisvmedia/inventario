import { describe, expect, it } from "vitest"

import {
  LABEL_FORMATS,
  LABEL_FORMAT_IDS,
  defaultSheetFormat,
  isLabelFormatId,
  labelsPerSheet,
  paginateLabels,
} from "@/features/labels/formats"
import { labelTargetUrl, labelsPageHref } from "@/features/labels/links"

describe("label formats", () => {
  it.each(LABEL_FORMAT_IDS)("%s fits its labels on the page with equal side margins", (id) => {
    const f = LABEL_FORMATS[id]
    const usedWidth = f.margin.left + f.columns * f.label.width + (f.columns - 1) * f.gap.column
    const usedHeight = f.margin.top + f.rows * f.label.height + (f.rows - 1) * f.gap.row
    expect(usedWidth).toBeLessThanOrEqual(f.page.width + 1e-9)
    expect(usedHeight).toBeLessThanOrEqual(f.page.height + 1e-9)
    // Die-cut sheets are symmetric; a mismatch means a typo in the numbers.
    expect(f.page.width - usedWidth).toBeCloseTo(f.margin.left, 5)
    expect(f.page.height - usedHeight).toBeCloseTo(f.margin.top, 5)
  })

  it("knows its ids", () => {
    expect(isLabelFormatId("a4-3x7")).toBe(true)
    expect(isLabelFormatId("a5")).toBe(false)
    expect(isLabelFormatId(null)).toBe(false)
  })

  it.each([
    ["en-US", "letter-3x10"],
    ["fr-CA", "letter-3x10"],
    ["en-GB", "a4-3x7"],
    ["cs-CZ", "a4-3x7"],
    ["en", "a4-3x7"],
    [undefined, "a4-3x7"],
  ])("defaults %s to %s", (locale, want) => {
    expect(defaultSheetFormat(locale)).toBe(want)
  })
})

describe("paginateLabels", () => {
  const a4 = LABEL_FORMATS["a4-3x7"]

  it("fills sheets in order", () => {
    const labels = Array.from({ length: 23 }, (_, i) => i)
    const sheets = paginateLabels(labels, a4)
    expect(sheets).toHaveLength(2)
    expect(sheets[0]).toHaveLength(labelsPerSheet(a4))
    expect(sheets[1]).toEqual([21, 22])
  })

  it("leaves skipped positions empty on the first sheet", () => {
    const sheets = paginateLabels([1, 2, 3], a4, 2)
    expect(sheets).toEqual([[null, null, 1, 2, 3]])
  })

  it("clamps skip to one sheet", () => {
    const sheets = paginateLabels([1], a4, 500)
    expect(sheets[0]).toHaveLength(labelsPerSheet(a4))
    expect(sheets[0].at(-1)).toBe(1)
  })

  it("puts one label on each page for the single format", () => {
    expect(paginateLabels(["a", "b"], LABEL_FORMATS.single, 3)).toEqual([["a"], ["b"]])
  })

  it("returns no pages without labels", () => {
    expect(paginateLabels([], a4, 4)).toEqual([])
  })
})

describe("label links", () => {
  it("encodes the absolute page URL", () => {
    expect(labelTargetUrl("https://inv.example", "home", "commodities", "c 1")).toBe(
      "https://inv.example/g/home/commodities/c%201"
    )
    expect(labelTargetUrl("https://inv.example", "home", "areas", "a1")).toBe(
      "https://inv.example/g/home/areas/a1"
    )
  })

  it("builds the label page link", () => {
    expect(labelsPageHref("home", { items: ["c1", "c2"] })).toBe("/g/home/labels?item=c1&item=c2")
    expect(labelsPageHref("home", { areas: ["a1"], areaItems: "a1" })).toBe(
      "/g/home/labels?area=a1&area_items=a1"
    )
    expect(labelsPageHref("home", {})).toBe("/g/home/labels")
  })
})
