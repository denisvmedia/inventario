// Public surface for the i18n feature. Anything outside src/i18n/* should
// import from here (or via `useTranslation()` directly) — never reach into
// the i18next instance directly so we have a single chokepoint for config.
export {
  initI18n,
  buildOptions,
  i18next,
  I18N_NAMESPACES,
  SUPPORTED_LANGUAGES,
  __resetI18nForTests,
  type I18nNamespace,
  type SupportedLanguage,
} from "./i18next.config"
