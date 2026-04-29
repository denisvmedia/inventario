import { defineConfig } from "i18next-cli"

// Drives `npm run i18n:extract` and `npm run i18n:check`. The CLI walks every
// .ts/.tsx file in src/ (excluding tests + the i18n implementation itself)
// and writes the extracted key tree to `src/i18n/locales/en/$NAMESPACE.json`.
//
// `preservePatterns` covers dynamic keys we deliberately can't extract at
// build time:
//   - `stubs:*` — PlaceholderPage uses `t(`stubs:${titleKey}`)`. titleKey is
//     a TS union narrowed against the en/stubs.json shape (compile-time
//     safety), so a missing key here is caught by tsc rather than the
//     extractor.
//
// `i18n:check` is what CI runs: it runs this extractor in dry-run mode and
// fails when adding `t("foo.bar")` in code without a matching entry in en
// would change the on-disk catalog.

export default defineConfig({
  locales: ["en"],
  extract: {
    input: [
      "src/**/*.{ts,tsx}",
      "!src/**/*.test.{ts,tsx}",
      "!src/**/__tests__/**",
      "!src/test/**",
      "!src/i18n/**",
    ],
    output: "src/i18n/locales/{{language}}/{{namespace}}.json",
    defaultNS: "common",
    keySeparator: ".",
    nsSeparator: ":",
    contextSeparator: "_",
    functions: ["t", "*.t"],
    transComponents: ["Trans"],
    // `stubs:*` — PlaceholderPage uses `t(`stubs:${key}`)` (titleKey is
    //   narrowed to `keyof typeof enStubs` so the typecheck catches misses).
    // `common:nav.*` — AppSidebar / CommandPalette use `t(`common:${entry.labelKey}`)`
    //   where labelKey is one of "nav.dashboard", "nav.locations", … The
    //   key set is enumerated in en/common.json's `nav` object; if a key
    //   here gets out of sync with the NavEntry list, the missing-key
    //   handler in dev surfaces it (saveMissing+console.warn).
    preservePatterns: ["stubs:*", "common:nav.*"],
  },
})
