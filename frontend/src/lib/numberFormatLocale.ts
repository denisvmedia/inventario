// Number-/currency-formatting locale override store. Decouples the
// BCP-47 tag used by `Intl.*` formatters from the UI translation
// language so a user can read Czech-formatted prices on an English UI
// (see #1683 — formatCurrency was previously keyed on i18next's
// resolved language, which folded the two preferences together).
//
// The override is sourced from `appearance.number_format_locale` on the
// per-user settings endpoint, mirrored to localStorage for boot-time
// consistency (settings load is async), and exposed through a tiny
// pub-sub so the cached formatters in `intl.ts` can re-build on change.
//
// Empty / null override → the FE fallback chain in `intl.ts` resolves
// against `navigator.language` first, then i18next's resolved language.
import { i18next } from "@/i18n"

export const NUMBER_FORMAT_LOCALE_STORAGE_KEY = "inventario-number-format-locale"

// The dropdown's option list — explicit BCP-47 tags that exercise the
// distinct separator / sign / suffix conventions Intl picks up. We
// hand-pick rather than offering "every locale the browser supports"
// because most of the long tail produces ambiguous or duplicative
// numbers — e.g. en-AU and en-IE both format USD as "$1,234.50", so
// shipping both makes the dropdown noisier without adding fidelity.
// "" (empty) is the auto-detect fallback (see `currentLocale()` in
// intl.ts) and is rendered as the first option.
//
// Adding a locale: drop a tag below + add the translation in
// frontend/src/i18n/locales/{en,cs,ru}/settings.json under
// `appearance.numberFormatLocaleOptions.<tag>`. Tags that round-trip
// through Intl.getCanonicalLocales as themselves are safe; anything
// region-less (e.g. "fr" instead of "fr-FR") will fall back to the
// language-default region which may surprise users on Settings.
export const NUMBER_FORMAT_LOCALE_OPTIONS = [
  "en-US",
  "en-GB",
  "cs-CZ",
  "de-DE",
  "es-ES",
  "fr-FR",
  "hu-HU",
  "it-IT",
  "ja-JP",
  "nl-NL",
  "pl-PL",
  "pt-BR",
  "ru-RU",
  "sv-SE",
] as const
export type NumberFormatLocale = (typeof NUMBER_FORMAT_LOCALE_OPTIONS)[number]

type Listener = () => void

let current: string | null = null
const listeners = new Set<Listener>()
let hydrated = false

// hydrate reads localStorage once on first access. We don't run this at
// module-load time because tests construct multiple jsdom windows; a
// lazy read lets the test harness wipe storage between cases without
// us needing an init-time hook.
function hydrate(): void {
  if (hydrated || typeof window === "undefined") {
    hydrated = true
    return
  }
  hydrated = true
  const stored = window.localStorage.getItem(NUMBER_FORMAT_LOCALE_STORAGE_KEY)
  current = stored && stored.length > 0 ? stored : null
}

export function getNumberFormatLocaleOverride(): string | null {
  hydrate()
  return current
}

export function setNumberFormatLocaleOverride(next: string | null | undefined): void {
  hydrate()
  const normalized = next && next.length > 0 ? next : null
  if (normalized === current) return
  current = normalized
  if (typeof window !== "undefined") {
    if (normalized === null) {
      window.localStorage.removeItem(NUMBER_FORMAT_LOCALE_STORAGE_KEY)
    } else {
      window.localStorage.setItem(NUMBER_FORMAT_LOCALE_STORAGE_KEY, normalized)
    }
  }
  // Replay i18next's `languageChanged` so every `useTranslation()`
  // subscriber re-renders and re-evaluates its `formatCurrency()` /
  // `formatDate()` calls with the new override. We deliberately
  // piggyback on i18next's emitter instead of standing up a parallel
  // one: every page that renders user-facing strings already mounts
  // useTranslation(), so this gives instant updates without each
  // surface having to consume a new context.
  try {
    i18next.emit?.("languageChanged", i18next.resolvedLanguage ?? i18next.language ?? "en")
  } catch {
    // i18next.emit can throw if the instance isn't initialised (test
    // bootstrap edge case). Subscribers below pick up the change on the
    // next render anyway, so swallowing here is safe.
  }
  for (const l of listeners) l()
}

export function subscribeNumberFormatLocale(l: Listener): () => void {
  listeners.add(l)
  return () => {
    listeners.delete(l)
  }
}

// Test-only: reset cached override + storage so cases that flip the
// preference don't see stale state from a previous test.
export function __resetNumberFormatLocaleOverrideForTests(): void {
  current = null
  hydrated = false
  if (typeof window !== "undefined") {
    window.localStorage.removeItem(NUMBER_FORMAT_LOCALE_STORAGE_KEY)
  }
}
