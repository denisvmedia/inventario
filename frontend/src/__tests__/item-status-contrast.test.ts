import { readFileSync } from "node:fs"
import { join } from "node:path"

import { describe, expect, it } from "vitest"

import {
  COMMODITY_STATUSES,
  COMMODITY_STATUS_TEXT_TONES,
  COMMODITY_STATUS_TONES,
  TERMINAL_COMMODITY_STATUSES,
} from "@/features/commodities/constants"

/*
  Item status contrast guard (#2926).

  A status pill draws its label in the status color on a tint of that color.
  The pill sits on a card (list cards and rows) or on the page background
  (the item page and its sheet). accessibility.md asks for 4.5:1 for text.
  This test takes the classes each status uses from COMMODITY_STATUS_TONES,
  reads the colors behind them from src/index.css, draws the tint the way the
  browser does and measures the label against it, in light and dark mode.
  It measures the Change Status buttons of the item page as well, which use
  the same text color, and checks that no two statuses look alike.

  Tailwind compiles `bg-x/10` to `color-mix(in oklab, var(--x) 10%,
  transparent)`, which is color x at alpha 0.1. The browser composites that
  over the surface in gamma-encoded sRGB. The colors must lie inside sRGB, so
  that what is measured here is what every display shows.
*/

type Oklch = { l: number; c: number; h: number; alpha: number }
type Vec3 = [number, number, number]
type Mode = "light" | "dark"

const css = readFileSync(join(__dirname, "..", "index.css"), "utf8").replace(
  /\/\*[\s\S]*?\*\//g,
  ""
)

function declarations(selector: string): Map<string, string> {
  const escaped = selector.replace(/[.]/g, "\\.")
  const body = new RegExp(`^${escaped}\\s*\\{([^}]*)\\}`, "m").exec(css)?.[1]
  if (body === undefined) throw new Error(`no ${selector} block in index.css`)
  const out = new Map<string, string>()
  for (const m of body.matchAll(/--([\w-]+)\s*:\s*([^;]+);/g)) out.set(m[1], m[2].trim())
  return out
}

const root = declarations(":root")
const tokens: Record<Mode, Map<string, string>> = {
  light: root,
  dark: new Map([...root, ...declarations(".dark")]),
}

function part(raw: string): number {
  return raw.endsWith("%") ? Number(raw.slice(0, -1)) / 100 : Number(raw)
}

function token(mode: Mode, name: string): Oklch {
  const value = tokens[mode].get(name)
  const m = value?.match(/^oklch\(\s*([\d.]+%?)\s+([\d.]+)\s+([\d.]+)\s*(?:\/\s*([\d.]+%?)\s*)?\)$/)
  if (!m) throw new Error(`--${name} in ${mode} mode is not an oklch() color: ${value}`)
  return { l: part(m[1]), c: Number(m[2]), h: Number(m[3]), alpha: m[4] ? part(m[4]) : 1 }
}

function oklab({ l, c, h }: Oklch): Vec3 {
  const rad = (h * Math.PI) / 180
  return [l, c * Math.cos(rad), c * Math.sin(rad)]
}

// OKLab to linear sRGB, with the matrices from Björn Ottosson's definition.
function linearSrgb(color: Oklch): Vec3 {
  const [l, a, b] = oklab(color)
  const l3 = (l + 0.3963377774 * a + 0.2158037573 * b) ** 3
  const m3 = (l - 0.1055613458 * a - 0.0638541728 * b) ** 3
  const s3 = (l - 0.0894841775 * a - 1.291485548 * b) ** 3
  return [
    4.0767416621 * l3 - 3.3077115913 * m3 + 0.2309699292 * s3,
    -1.2684380046 * l3 + 2.6097574011 * m3 - 0.3413193965 * s3,
    -0.0041960863 * l3 - 0.7034186147 * m3 + 1.707614701 * s3,
  ]
}

function inGamut(color: Oklch): boolean {
  return linearSrgb(color).every((x) => x >= -1e-4 && x <= 1 + 1e-4)
}

function encode(x: number): number {
  const v = Math.min(1, Math.max(0, x))
  return v <= 0.0031308 ? 12.92 * v : 1.055 * v ** (1 / 2.4) - 0.055
}

function decode(x: number): number {
  return x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4
}

function srgb(color: Oklch): Vec3 {
  return linearSrgb(color).map(encode) as Vec3
}

function over(fg: Vec3, alpha: number, bg: Vec3): Vec3 {
  return fg.map((f, i) => alpha * f + (1 - alpha) * bg[i]) as Vec3
}

function luminance(rgb: Vec3): number {
  const [r, g, b] = rgb.map(decode)
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(a: Vec3, b: Vec3): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

// The text color and the tint of a status, read from its tone classes.
function tone(status: (typeof COMMODITY_STATUSES)[number]) {
  const classes = COMMODITY_STATUS_TONES[status].split(/\s+/)
  const text = classes.find((c) => c.startsWith("text-"))?.slice("text-".length)
  const bg = classes.find((c) => c.startsWith("bg-"))?.match(/^bg-([\w-]+?)(?:\/(\d+))?$/)
  if (!text || !bg) throw new Error(`${status} has no text or bg class`)
  return { text, tint: bg[1], tintAlpha: bg[2] ? Number(bg[2]) / 100 : 1 }
}

const MODES: Mode[] = ["light", "dark"]
const SURFACES = ["card", "background"] as const
const cases = MODES.flatMap((mode) => COMMODITY_STATUSES.map((status) => [mode, status] as const))

describe("item status colors", () => {
  it.each(cases)("%s mode: the %s label reaches 4.5:1 on its tint", (mode, status) => {
    const { text, tint, tintAlpha } = tone(status)
    const label = srgb(token(mode, text))
    const tintColor = token(mode, tint)
    for (const surface of SURFACES) {
      const behind = srgb(token(mode, surface))
      const fill = over(srgb(tintColor), tintAlpha * tintColor.alpha, behind)
      expect(contrast(label, fill), `on the ${surface}`).toBeGreaterThanOrEqual(4.5)
    }
  })

  it.each(cases)("%s mode: the %s color lies inside sRGB", (mode, status) => {
    const { text, tint } = tone(status)
    expect(inGamut(token(mode, text)), text).toBe(true)
    expect(inGamut(token(mode, tint)), tint).toBe(true)
  })

  // A Change Status button is an outline button in a `bg-muted/30` row on
  // the page: `bg-background` in light mode, `bg-input/30` over the row in
  // dark mode. Its label has the pill's text color.
  const buttons = MODES.flatMap((mode) =>
    TERMINAL_COMMODITY_STATUSES.map((status) => [mode, status] as const)
  )
  it.each(buttons)("%s mode: the %s Change Status button reaches 4.5:1", (mode, status) => {
    const text = COMMODITY_STATUS_TEXT_TONES[status]
    expect(COMMODITY_STATUS_TONES[status].split(/\s+/)).toContain(text)
    const page = srgb(token(mode, "background"))
    let surface = page
    if (mode === "dark") {
      const row = over(srgb(token(mode, "muted")), 0.3, page)
      const input = token(mode, "input")
      surface = over(srgb(input), 0.3 * input.alpha, row)
    }
    const label = srgb(token(mode, text.slice("text-".length)))
    expect(contrast(label, surface)).toBeGreaterThanOrEqual(4.5)
  })

  // 0.08 is four times the OKLab distance that CSS Color 4 gamut mapping
  // treats as just noticeable (0.02).
  it.each(MODES)("%s mode: no two statuses share a color", (mode) => {
    for (const [i, a] of COMMODITY_STATUSES.entries()) {
      for (const b of COMMODITY_STATUSES.slice(i + 1)) {
        const [pa, pb] = [oklab(token(mode, tone(a).text)), oklab(token(mode, tone(b).text))]
        const distance = Math.hypot(pa[0] - pb[0], pa[1] - pb[1], pa[2] - pb[2])
        expect(distance, `${a} and ${b}`).toBeGreaterThanOrEqual(0.08)
      }
    }
  })
})
