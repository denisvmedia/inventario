import { readFileSync } from "node:fs"
import { join } from "node:path"

import { describe, expect, it } from "vitest"

/*
  Reduced-motion guard (#2932).

  tw-animate-css has no prefers-reduced-motion rule. Its enter and exit
  keyframes take their start and end values from `--tw-enter-*` and
  `--tw-exit-*` properties, and `src/index.css` sets every one that moves
  (scale, translate, rotate) back to rest under `prefers-reduced-motion:
  reduce`. This test reads the library's own stylesheet, so a property it
  adds in a later version fails here until index.css resets it too. jsdom
  applies no media queries, so the rendered result is checked in
  e2e/tests/reduced-motion.spec.ts.
*/

const frontendRoot = join(__dirname, "..", "..")
const indexCss = readFileSync(join(frontendRoot, "src", "index.css"), "utf8")
const twAnimateCss = readFileSync(
  join(frontendRoot, "node_modules", "tw-animate-css", "dist", "tw-animate.css"),
  "utf8"
)

// Properties that only change how an element looks, not where it is.
const STILL = /-(opacity|blur)$/

function reducedMotionBlocks(css: string): string[] {
  const blocks: string[] = []
  const opener = /@media\s*\(\s*prefers-reduced-motion:\s*reduce\s*\)\s*\{/g
  for (let m = opener.exec(css); m; m = opener.exec(css)) {
    let depth = 1
    let i = opener.lastIndex
    for (; i < css.length && depth > 0; i++) {
      if (css[i] === "{") depth++
      else if (css[i] === "}") depth--
    }
    blocks.push(css.slice(opener.lastIndex, i - 1))
  }
  return blocks
}

describe("reduced motion", () => {
  const movement = [...new Set(twAnimateCss.match(/--tw-(?:enter|exit)-[a-z-]+/g) ?? [])]
    .filter((name) => !STILL.test(name))
    .sort()

  it("reads the movement properties of tw-animate-css", () => {
    expect(indexCss).toContain('@import "tw-animate-css"')
    expect(movement).toEqual(
      expect.arrayContaining(["--tw-enter-scale", "--tw-enter-translate-y", "--tw-exit-scale"])
    )
  })

  it("sets every movement property back to rest under reduce", () => {
    const reduce = reducedMotionBlocks(indexCss).join("\n")
    for (const name of movement) {
      const rest = name.endsWith("-scale") ? "1" : "0"
      expect(reduce, name).toMatch(new RegExp(`${name}\\s*:\\s*${rest}\\s*!important`))
    }
  })

  it("keeps the fade", () => {
    const reduce = reducedMotionBlocks(indexCss).join("\n")
    expect(reduce).not.toMatch(/--tw-(?:enter|exit)-opacity/)
  })
})
