import { act, render, screen } from "@testing-library/react"
import LanguageDetector from "i18next-browser-languagedetector"
import { useTranslation } from "react-i18next"
import { afterEach, describe, expect, it } from "vitest"

import { i18next } from "@/i18n"
import { buildOptions, lazyBackend } from "@/i18n/i18next.config"
import csCommon from "@/i18n/locales/cs/common.json"
import enCommon from "@/i18n/locales/en/common.json"
import ruCommon from "@/i18n/locales/ru/common.json"

// #3018: en is inline in `resources`; cs and ru come from the lazy backend.
// i18next asks the backend for a language that `resources` lacks only when
// partialBundledLanguages is set, so these tests go through the real backend
// and expect the strings of the cs/ru catalogs, not the en fallback.
function GoHome() {
  const { t } = useTranslation()
  return <p>{t("common:actions.goHome")}</p>
}

describe("lazy cs/ru catalogs (#3018)", () => {
  afterEach(async () => {
    await i18next.changeLanguage("en")
  })

  it("loads the Czech catalog when the language changes", async () => {
    await i18next.changeLanguage("cs")
    expect(i18next.resolvedLanguage).toBe("cs")
    expect(i18next.t("common:actions.goHome")).toBe(csCommon.actions.goHome)
  })

  it("loads the Russian catalog when the language changes", async () => {
    await i18next.changeLanguage("ru")
    expect(i18next.resolvedLanguage).toBe("ru")
    expect(i18next.t("common:actions.goHome")).toBe(ruCommon.actions.goHome)
  })

  it("re-renders a mounted component in the new language", async () => {
    render(<GoHome />)
    expect(screen.getByText(enCommon.actions.goHome)).toBeInTheDocument()
    await act(async () => {
      await i18next.changeLanguage("cs")
    })
    expect(screen.getByText(csCommon.actions.goHome)).toBeInTheDocument()
  })

  it("has the detected language loaded by the time init resolves", async () => {
    // main.tsx mounts the app after init resolves, so the first paint is in
    // the language the detector read from localStorage. A fresh instance keeps
    // the shared one (and react-i18next's global) untouched.
    window.localStorage.setItem("inventario-language", "cs")
    const instance = i18next.createInstance()
    await instance.use(lazyBackend).use(LanguageDetector).init(buildOptions({ debug: false }))
    expect(instance.resolvedLanguage).toBe("cs")
    expect(instance.t("common:actions.goHome")).toBe(csCommon.actions.goHome)
  })
})
