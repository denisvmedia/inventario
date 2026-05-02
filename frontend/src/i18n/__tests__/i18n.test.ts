import { describe, expect, it, beforeAll } from "vitest"

import { i18next, initI18n } from "@/i18n"

describe("i18n boot", () => {
  beforeAll(async () => {
    await initI18n({ lng: "en" })
  })

  it("resolves keys from the bundled en namespaces", () => {
    expect(i18next.t("dashboard:heading")).toBe("Welcome to Inventario")
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

  it("falls back to en when switched to cs (cs catalog is empty)", async () => {
    await i18next.changeLanguage("cs")
    expect(i18next.t("dashboard:heading")).toBe("Welcome to Inventario")
    // Restore so subsequent suites don't see a stale lng.
    await i18next.changeLanguage("en")
  })

  it("falls back to en when switched to ru", async () => {
    await i18next.changeLanguage("ru")
    expect(i18next.t("errors:notFound.heading")).toBe("Page not found")
    await i18next.changeLanguage("en")
  })
})
