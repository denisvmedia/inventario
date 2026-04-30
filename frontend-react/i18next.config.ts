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
    // `auth:validation.*` — zod schemas in src/features/auth/schemas.ts hold
    //   the keys as plain strings (not t() calls); RHF's `errors[name].message`
    //   carries them through to a t() call at render time, but the extractor
    //   sees only the dynamic lookup.
    // `auth:passwordStrength.*` and `auth:session.*` — both use index-table
    //   lookups (`t(STRENGTH_LABELS[score])`, `t(SESSION_REASON_KEY[reason])`)
    //   that the extractor can't statically resolve.
    // `settings:sections.*`, `settings:appearance.themeOptions.*`,
    //   `settings:appearance.localeOptions.*`, `settings:help.rows.*` —
    //   the SettingsPage builds keys from registry maps (theme ids,
    //   supported languages, section ids, help-row ids), so the extractor
    //   sees only the template literal and can't enumerate the entries.
    preservePatterns: [
      "stubs:*",
      "common:nav.*",
      "auth:validation.*",
      "auth:passwordStrength.*",
      "auth:session.*",
      "settings:sections.*",
      "settings:appearance.themeOptions.*",
      "settings:appearance.localeOptions.*",
      "settings:help.rows.*",
      // groups:validation.* — schema messages in features/group/schemas.ts
      // are plain strings, surfaced through RHF errors[name].message →
      // t() at render time. Same pattern as auth:validation.*.
      "groups:validation.*",
      // members:roles.* — built from `t(\`members:roles.${role}\`)`
      // template lookups in MembersPage. The role set is models.GroupRole
      // (admin | user); missing keys surface in the dev console via the
      // saveMissing handler.
      "members:roles.*",
      // search:groups.* — built from `t(\`search:groups.${type}\`)` per
      // resource (commodities/locations/areas/files/tags). The set is
      // SearchableType in features/search/api.ts plus the BE-blocked
      // "tags" type that ships as a stub.
      "search:groups.*",
      // search:queryHints.* — resolved via a HINT_KEYS lookup table on
      // the empty-state page (`t(HINT_KEYS[h])`), so the extractor
      // can't see the literal keys.
      "search:queryHints.*",
      // search:resultCard.* — also a static lookup but using string
      // literals; preserve to keep the namespace tidy across future
      // extracts even if the t() calls remain visible.
      "search:resultCard.*",
      // locations:validation.* — schema messages in
      // features/{locations,areas}/schemas.ts hold the keys as plain
      // strings, surfaced through RHF errors[name].message → t() at
      // render time. Same pattern as auth:validation.* / groups:validation.*.
      "locations:validation.*",
    ],
  },
})
