import { describe, expect, it } from "vitest"

import { PHOTO_CARD_IMAGE_SIZES } from "@/components/items/photoCardSizes"

// The width of a photo card's image box on the Items list, by window width,
// measured in Chromium with the sidebar open (#2900).
const MEASURED: [window: number, image: number][] = [
  [360, 122],
  [412, 148],
  [500, 192],
  [639, 261.5],
  [700, 262],
  [767, 262],
  [768, 198.8],
  [900, 264],
  [927, 277.5],
  [928, 179.3],
  [1000, 203.3],
  [1023, 211],
  [1100, 236.7],
  [1120, 178],
  [1200, 198],
  [1279, 217.8],
  [1280, 218],
  [1375, 241.8],
  [1376, 190],
  [1503, 215.4],
  [1504, 176.7],
  [1583, 189.8],
  [1584, 190],
  [1920, 190],
]

const THUMBNAILS = [150, 300, 600]

// The width the hint names at a window width: the first entry whose
// min-width matches, as a browser reads it. The entries are px, vw or
// calc(Nvw - Mpx).
function hintAt(window: number): number {
  for (const entry of PHOTO_CARD_IMAGE_SIZES.split(", ")) {
    const m = entry.match(/^\(min-width: (\d+)px\) (.+)$/)
    if (m && window < Number(m[1])) continue
    const length = m ? m[2] : entry
    const px = length.match(/^(\d+)px$/)
    if (px) return Number(px[1])
    const vw = length.match(/^([\d.]+)vw$/)
    if (vw) return (window * Number(vw[1])) / 100
    const calc = length.match(/^calc\(([\d.]+)vw - (\d+)px\)$/)
    if (!calc) throw new Error(`unexpected length ${length}`)
    return (window * Number(calc[1])) / 100 - Number(calc[2])
  }
  throw new Error("the hint has no fallback")
}

// The thumbnail a browser picks for a slot: the smallest at least as wide as
// the slot on the screen's pixel density.
const pick = (width: number, density: number) =>
  THUMBNAILS.find((w) => w >= width * density) ?? THUMBNAILS[THUMBNAILS.length - 1]

describe("PHOTO_CARD_IMAGE_SIZES", () => {
  it.each(MEASURED)("matches the image box at a %i px window", (window, image) => {
    const hint = hintAt(window)
    expect(Math.abs(hint - image)).toBeLessThanOrEqual(1)
    for (const density of [1, 2, 3]) {
      expect(pick(hint, density)).toBe(pick(image, density))
    }
  })
})
