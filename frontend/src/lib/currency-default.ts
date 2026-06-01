import i18next from "i18next"

// Infers a sensible default ISO-4217 currency from the browser locale for the
// anonymous first-item flow (#1988), where there is no group yet to read a
// currency from. Seeds the create dialog's prices and the auto-created "Main"
// group. Best-effort only: the user can override via the currency combobox,
// and the post-login resolver re-runs price conversion with the real group
// currency, so a wrong guess is never fatal. Falls back to USD.

// ISO 3166-1 alpha-2 region (upper-case) → ISO-4217 currency. Covers the
// shipped UI locales (en/cs/ru) plus the common cases; anything unmapped
// falls back to USD.
const REGION_TO_CURRENCY: Record<string, string> = {
  US: "USD",
  CA: "CAD",
  GB: "GBP",
  IE: "EUR",
  CZ: "CZK",
  SK: "EUR",
  PL: "PLN",
  HU: "HUF",
  RU: "RUB",
  UA: "UAH",
  BY: "BYN",
  KZ: "KZT",
  DE: "EUR",
  FR: "EUR",
  ES: "EUR",
  IT: "EUR",
  NL: "EUR",
  BE: "EUR",
  AT: "EUR",
  PT: "EUR",
  FI: "EUR",
  GR: "EUR",
  CH: "CHF",
  SE: "SEK",
  NO: "NOK",
  DK: "DKK",
  AU: "AUD",
  NZ: "NZD",
  JP: "JPY",
  CN: "CNY",
  IN: "INR",
  BR: "BRL",
  MX: "MXN",
  ZA: "ZAR",
  TR: "TRY",
}

// Bare-language fallbacks for locales with no region tag (e.g. "cs", "ru").
const LANG_TO_CURRENCY: Record<string, string> = {
  en: "USD",
  cs: "CZK",
  ru: "RUB",
}

const FALLBACK = "USD"

// The base language (no region) the app has actually resolved to. i18next
// reads its persisted language from localStorage before falling back to the
// browser, so a non-default value here reflects a deliberate user switch.
function appLanguage(): string | undefined {
  const l = i18next.resolvedLanguage || i18next.language
  return l ? l.split("-")[0].toLowerCase() : undefined
}

function browserLocale(): string {
  if (typeof navigator !== "undefined" && navigator.language) return navigator.language
  return i18next.resolvedLanguage || i18next.language || "en"
}

export function inferDefaultCurrency(): string {
  // A deliberately-chosen non-English UI language wins: English is the
  // default/fallback, so a resolved "cs"/"ru" means the user explicitly
  // switched and a CZK/RUB seed is what they expect — even on an en-* browser
  // (the case CodeRabbit flagged). We do NOT short-circuit on "en" because
  // that's the default: there, the browser locale's region is the richer
  // signal (en-GB → GBP, en-US → USD) than the bare language.
  const appLang = appLanguage()
  if (appLang && appLang !== "en" && LANG_TO_CURRENCY[appLang]) {
    return LANG_TO_CURRENCY[appLang]
  }
  const locale = browserLocale()
  const parts = locale.split("-")
  // "cs-CZ" → region "CZ"; a bare "cs" has no region segment.
  const region = parts.length > 1 ? parts[parts.length - 1].toUpperCase() : ""
  if (region && REGION_TO_CURRENCY[region]) return REGION_TO_CURRENCY[region]
  const lang = parts[0]?.toLowerCase()
  return (lang && LANG_TO_CURRENCY[lang]) || FALLBACK
}
