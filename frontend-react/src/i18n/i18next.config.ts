import i18next, { type i18n as I18nInstance, type InitOptions } from "i18next"
import LanguageDetector from "i18next-browser-languagedetector"
import resourcesToBackend from "i18next-resources-to-backend"
import { initReactI18next } from "react-i18next"

import * as enAuth from "./locales/en/auth.json"
import * as enCommodities from "./locales/en/commodities.json"
import * as enCommon from "./locales/en/common.json"
import * as enDashboard from "./locales/en/dashboard.json"
import * as enErrors from "./locales/en/errors.json"
import * as enExports from "./locales/en/exports.json"
import * as enFiles from "./locales/en/files.json"
import * as enGroups from "./locales/en/groups.json"
import * as enLocations from "./locales/en/locations.json"
import * as enMembers from "./locales/en/members.json"
import * as enSettings from "./locales/en/settings.json"
import * as enStubs from "./locales/en/stubs.json"
import * as enTags from "./locales/en/tags.json"

// Locales we ship today. `cs` and `ru` are present as namespace stubs (mostly
// empty objects) and fall back to `en`; real translations land in follow-ups.
export const SUPPORTED_LANGUAGES = ["en", "cs", "ru"] as const
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number]

// Namespaces map 1:1 to JSON files under locales/<lng>/. Adding a new
// namespace means adding a new file in *every* locale; the i18n:check
// script (see scripts/i18n-check.mjs) enforces that the en/ tree stays
// the canonical source.
export const I18N_NAMESPACES = [
  "common",
  "auth",
  "dashboard",
  "locations",
  "commodities",
  "files",
  "tags",
  "exports",
  "settings",
  "members",
  "groups",
  "stubs",
  "errors",
] as const
export type I18nNamespace = (typeof I18N_NAMESPACES)[number]

// English is bundled statically — it is the fallback for every key, so the
// app must not require an extra round-trip to render its very first frame.
// `* as` import lets us survive both `--esModuleInterop` and the legacy
// `module.exports` shape that some bundlers emit for `.json`.
const enResources = {
  common: enCommon,
  auth: enAuth,
  dashboard: enDashboard,
  locations: enLocations,
  commodities: enCommodities,
  files: enFiles,
  tags: enTags,
  exports: enExports,
  settings: enSettings,
  members: enMembers,
  groups: enGroups,
  stubs: enStubs,
  errors: enErrors,
} as const

// Lazy backend for cs/ru only. We list cs and ru explicitly via
// `import.meta.glob` so Vite splits each (lng, ns) pair into its own chunk
// AND knows en/ stays out of the dynamic set (en is already inlined via
// `resources`, so a "both static and dynamic" import on the en files would
// emit the noisy INEFFECTIVE_DYNAMIC_IMPORT warning at build time).
type LocaleLoaders = Record<string, () => Promise<{ default: unknown }>>
const csLoaders = import.meta.glob<{ default: unknown }>("./locales/cs/*.json") as LocaleLoaders
const ruLoaders = import.meta.glob<{ default: unknown }>("./locales/ru/*.json") as LocaleLoaders

const lazyBackend = resourcesToBackend(async (lng: string, ns: string) => {
  if (lng === "en") return enResources[ns as I18nNamespace] ?? {}
  const loaders: LocaleLoaders | null = lng === "cs" ? csLoaders : lng === "ru" ? ruLoaders : null
  if (!loaders) return {}
  const path = `./locales/${lng}/${ns}.json`
  const loader = loaders[path]
  if (!loader) return {}
  const mod = await loader()
  return (mod.default ?? mod) as object
})

export interface CreateI18nOptions {
  // When set, skip browser language detection (used by tests + SSR pre-render).
  lng?: SupportedLanguage
  // When true, console.warn on every missing key. Defaults to dev-only.
  debug?: boolean
}

// buildOptions is exported so tests can construct the same config without
// running the side-effecting init below.
export function buildOptions(opts: CreateI18nOptions = {}): InitOptions {
  return {
    lng: opts.lng,
    fallbackLng: "en",
    supportedLngs: [...SUPPORTED_LANGUAGES],
    // List the ns explicitly so i18next preloads them on init rather than
    // lazy-loading on first useTranslation() call (which would race the
    // first paint).
    ns: [...I18N_NAMESPACES],
    defaultNS: "common",
    nonExplicitSupportedLngs: true, // "en-GB" → "en"
    load: "languageOnly",
    debug: opts.debug ?? import.meta.env?.DEV ?? false,
    interpolation: {
      // React already escapes; double-escaping turns "&" into "&amp;amp;".
      escapeValue: false,
    },
    react: {
      // We don't need Suspense for the en bundle (it's static); cs/ru
      // resolve before render via the languagedetector pre-init. Disabling
      // Suspense lets us reuse the same boot path in tests where waiting
      // on Suspense without a SuspenseList would make assertions awkward.
      useSuspense: false,
    },
    resources: { en: enResources },
    // Surface missing keys early in dev. The `parseMissingKeyHandler` is
    // what i18next renders for an unresolved key — we leave it as the key
    // itself in prod (that's the i18next default) and tag it with a
    // `⟪…⟫` marker in dev so missing strings stand out on the page.
    saveMissing: import.meta.env?.DEV ?? false,
    missingKeyHandler: import.meta.env?.DEV
      ? (lngs, ns, key) => {
          // Single console.warn per (ns, key) pair — i18next dedupes by
          // calling the handler once per missing lookup, so this lights
          // up Chrome devtools without spamming the console mid-render.
          console.warn(`[i18n] missing key: ${ns}:${key} (lng=${lngs.join(",")})`)
        }
      : undefined,
    detection: {
      // localStorage > navigator. Settings page (#1414) writes here when the
      // user picks a non-default locale.
      order: ["localStorage", "navigator", "htmlTag"],
      caches: ["localStorage"],
      lookupLocalStorage: "inventario-language",
    },
  }
}

// initI18n creates a configured i18next instance and runs init(). Idempotent —
// calling it twice returns the same shared instance (we expose this in
// `index.ts` so the React tree can mount it before any useTranslation()
// runs).
//
// We cache the init promise rather than the instance so concurrent callers
// during a single boot share one in-flight init and never see partial state.
// We deliberately do NOT expose a "reset" helper: re-running `.use(...)` on
// the shared singleton would re-register plugins and accumulate state. If a
// future test needs a fresh instance, build it via `i18next.createInstance()`
// rather than tearing down this one.
let initPromise: Promise<I18nInstance> | null = null
export function initI18n(opts: CreateI18nOptions = {}): Promise<I18nInstance> {
  if (initPromise) return initPromise
  initPromise = i18next
    .use(lazyBackend)
    .use(LanguageDetector)
    .use(initReactI18next)
    .init(buildOptions(opts))
    .then(() => i18next)
  return initPromise
}

export { i18next }
