// Drives `npm run i18n:extract` and `npm run i18n:check`.
//
// The parser walks every .ts/.tsx file in src/ (excluding tests) and writes
// the extracted key tree to `src/i18n/locales/en/$NAMESPACE.json`. Existing
// human-curated translations are preserved (`keepRemoved: true`); the parser
// only adds missing keys with an empty default value so the next reviewer
// has to fill in the copy explicitly.
//
// `i18n:check` is what CI runs: it runs this extractor and then asserts the
// repo's en/* catalogs match the extractor's output (via `git diff
// --exit-code`). Adding `t("foo.bar")` in code without first writing
// `foo.bar` into the matching en JSON file will fail CI.

export default {
  locales: ["en"],
  output: "src/i18n/locales/$LOCALE/$NAMESPACE.json",
  defaultNamespace: "common",
  namespaceSeparator: ":",
  keySeparator: ".",
  pluralSeparator: "_",
  contextSeparator: "_",
  input: [
    "src/**/*.{ts,tsx}",
    "!src/**/*.test.{ts,tsx}",
    "!src/**/__tests__/**",
    "!src/test/**",
    "!src/i18n/**",
  ],
  sort: true,
  createOldCatalogs: false,
  // Don't strip keys we've curated by hand (e.g. `auth: { _: "..." }`
  // placeholders for future-issue copy). The check below catches missing
  // keys via the diff, not via removal.
  keepRemoved: true,
  // Empty default forces a human to write the copy in en/* — the parser
  // never invents translations.
  defaultValue: "",
}
