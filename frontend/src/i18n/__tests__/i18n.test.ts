import { describe, expect, it, beforeAll } from "vitest"

import { i18next, initI18n } from "@/i18n"

describe("i18n boot", () => {
  beforeAll(async () => {
    await initI18n({ lng: "en" })
  })

  it("resolves keys from the bundled en namespaces", () => {
    expect(i18next.t("dashboard:heading")).toBe("Overview")
    expect(i18next.t("errors:notFound.heading")).toBe("Page not found")
    expect(i18next.t("common:actions.goHome")).toBe("Go home")
  })

  it("interpolates variables into common:trackedBy", () => {
    expect(i18next.t("common:trackedBy", { ref: "#1404" })).toBe("Tracked by #1404.")
  })

  it("renders the documentTitle pattern with title + brand", () => {
    expect(i18next.t("common:documentTitle", { title: "Settings", brand: "Inventario" })).toBe(
      "Settings · Inventario"
    )
  })
})

// #2089: cs + ru catalogs must stay at key-parity with the canonical en
// catalog. The runtime lazy backend (import.meta.glob in i18next.config) is
// not exercised in jsdom, so we load the JSON statically and compare key
// sets — a missing or renamed key (which would silently fall back to en in
// the UI) fails CI. cs/ru may carry EXTRA keys (language-specific plural
// categories `_few` / `_many` beyond en's `_one` / `_other`), so the
// invariant is "every en leaf key exists in cs and ru", not strict equality.
type Json = Record<string, unknown>

const enFiles = import.meta.glob<{ default: Json }>("../locales/en/*.json", { eager: true })
const csFiles = import.meta.glob<{ default: Json }>("../locales/cs/*.json", { eager: true })
const ruFiles = import.meta.glob<{ default: Json }>("../locales/ru/*.json", { eager: true })

function flattenKeys(obj: Json, prefix = ""): string[] {
  const keys: string[] = []
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (v !== null && typeof v === "object" && !Array.isArray(v)) {
      keys.push(...flattenKeys(v as Json, key))
    } else {
      keys.push(key)
    }
  }
  return keys
}

function byNamespace(files: Record<string, { default: Json }>): Map<string, Json> {
  const map = new Map<string, Json>()
  for (const [path, mod] of Object.entries(files)) {
    const base = path.split("/").pop() ?? ""
    map.set(base.replace(/\.json$/, ""), mod.default)
  }
  return map
}

const enNs = byNamespace(enFiles)
const csNs = byNamespace(csFiles)
const ruNs = byNamespace(ruFiles)

describe("i18n catalog parity (#2089)", () => {
  for (const [ns, enCatalog] of enNs) {
    const enKeys = flattenKeys(enCatalog)

    it(`cs/${ns} covers every en key`, () => {
      const csKeys = new Set(flattenKeys(csNs.get(ns) ?? {}))
      expect(enKeys.filter((k) => !csKeys.has(k))).toEqual([])
    })

    it(`ru/${ns} covers every en key`, () => {
      const ruKeys = new Set(flattenKeys(ruNs.get(ns) ?? {}))
      expect(enKeys.filter((k) => !ruKeys.has(k))).toEqual([])
    })
  }
})
