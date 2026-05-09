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
      // settings:storage.breakdown.* — StorageCard maps over BREAKDOWN_KEYS
      //   (photos/invoices/documents/exports/other) and resolves each label
      //   via `t(\`settings:storage.breakdown.${key}\`)`. The extractor only
      //   sees the template literal.
      "settings:storage.breakdown.*",
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
      // commodities:validation.* — same pattern as auth/groups/locations:
      // schema messages in features/commodities/schemas.ts are plain
      // strings flowing through RHF errors[name].message at render time.
      "commodities:validation.*",
      // commodities:{type,status,sort}.* — built from
      // `t(\`commodities:type.${tp}\`)` / `t(\`commodities:status.${s}\`)`
      // / `t(\`commodities:sort.${field}\`)` lookups in the list page,
      // detail page, and form dialog. Enum sets enumerated in
      // features/commodities/constants.ts.
      "commodities:type.*",
      "commodities:status.*",
      "commodities:sort.*",
      "commodities:warranty.*",
      // commodities:warrantyStatus.* — WarrantyTab + WarrantyStep + the
      //   warranties list page render the pill via
      //   `t(\`commodities:warrantyStatus.${status}\`)` over the closed
      //   CommodityWarrantyStatus union (active/expiring/expired/none).
      "commodities:warrantyStatus.*",
      // commodities:detail.warranty.* — i18next pluralization on the
      //   days-remaining + expiredAgo lines (`_one` / `_other` suffixes
      //   re-stamped on every extract pass without a wildcard).
      "commodities:detail.warranty.*",
      // commodities:detail.filesTab.chip.* — CommodityFilesTab builds
      //   the segmented chip labels via
      //   `t(`commodities:detail.filesTab.chip.${chip.labelKey}`)`
      //   over the closed `all/photos/invoices/documents` union.
      // commodities:detail.filesTab.{drop,empty,cta}* — same chip-bar
      //   surface resolves the upload-zone copy, the chip-aware empty
      //   state, and the per-row View/Open/Download CTA via dynamic
      //   template lookups against ChipDef + a mime-keyed lookup. The
      //   extractor sees only the template literal in each case.
      "commodities:detail.filesTab.chip.*",
      "commodities:detail.filesTab.drop*",
      "commodities:detail.filesTab.empty*",
      "commodities:detail.filesTab.cta*",
      // warranties:list.tab.* — WarrantiesListPage builds tab labels
      //   via `t(\`warranties:list.tab.${s}\`)` over the closed
      //   {all,active,expiring,expired,none} union.
      // warranties:list.empty.* — same shape, per-tab empty state copy.
      "warranties:list.tab.*",
      "warranties:list.empty.*",
      // commodities:detail.historyEvent.loanField.* — built from
      //   `t(\`commodities:detail.historyEvent.loanField.${key}\`)` over
      //   the four mutable loan fields (borrower_name, borrower_contact,
      //   borrower_note, due_back_at) that snapshotLoanDiff persists
      //   in the BE event payload. Dynamic keys; the extractor can't
      //   see the literals.
      "commodities:detail.historyEvent.loanField.*",
      // commodities:bulk.{deleteTitle,deleteDescription,selected,
      //   bulkDeleted,bulkMoved,moveTitle,moveDescription,subtitle}_one /
      //   _other — i18next pluralization suffixes. The list/dialog code
      //   passes `count` to t() and i18next picks the right suffix at
      //   resolve time; the extractor sees only the singular key.
      "commodities:bulk.*",
      "commodities:toast.*",
      "commodities:list.subtitle*",
      // tags:stats.* — TagsStatsBar renders five tiles via a const map
      //   `{ key: 'tags_total', labelKey: 'tags:stats.tagsTotal' } …`,
      //   so the `t(labelKey)` call is dynamic from the extractor's POV.
      // tags:color.* — TagColorPicker uses `t(\`tags:color.${color}\`)`
      //   over the closed enum (amber/green/blue/orange/red/muted).
      // tags:validation.* — schema messages in features/tags/schemas.ts
      //   are plain strings flowing through RHF errors[name].message →
      //   t() at render time (same pattern as auth:validation.*).
      // tags:list.usage{Items,Files}_* and tags:usage.{items,files}_* —
      //   i18next pluralization suffixes; the extractor sees only the
      //   singular form when t() is called with `count`.
      "tags:stats.*",
      "tags:color.*",
      "tags:validation.*",
      "tags:list.usageItems*",
      "tags:list.usageFiles*",
      "tags:usage.items*",
      "tags:usage.files*",
      // exports:status.* — ExportStatusBadge resolves the badge label via
      //   `t(\`exports:status.${status}\`)` over the closed
      //   ExportStatus | RestoreStatus union.
      // exports:scope.* — ExportRow / ExportDetailPage build the scope
      //   label via `t(\`exports:scope.${exp.type ?? 'fullDatabase'}\`)`
      //   over models.ExportType.
      // exports:restore.strategyLabel.* /
      // exports:restore.strategyDescription.* — restore form + history list
      //   build them via `t(\`exports:restore.strategyLabel.${strategy}\`)`
      //   over the closed RESTORE_STRATEGIES union.
      // exports:detail.counts.* — list / detail call
      //   `t('exports:detail.counts.locations', { count })` etc.; the
      //   `_one` / `_other` plural suffixes need to survive extract.
      "exports:status.*",
      "exports:scope.*",
      "exports:restore.strategyLabel.*",
      "exports:restore.strategyDescription.*",
      "exports:detail.counts.*",
      "exports:detail.scopeSelectedItems*",
      // exports:wizard.scope.* — Step1 of the New-export wizard reads
      //   `t(titleKey)` / `t(hintKey)` where the keys are passed in from
      //   the parent page as `"exports:wizard.scope.<option>"`. Closed
      //   set: full_database / selected_items.
      "exports:wizard.scope.*",
      // exports:wizard.step*Title — WizardSteps maps over a const items
      //   array `{ index, titleKey: "exports:wizard.stepNTitle" }` and
      //   resolves the label via `t(item.titleKey)`. Static-analysis
      //   only sees the static usages of step1Title / step3Title; the
      //   step2Title key is reached only via the array.
      "exports:wizard.step*Title",
      // loans:validation.* — zod schema messages in
      //   features/loans/schemas.ts are plain strings, surfaced through
      //   RHF errors[name].message → t() at render time. Same pattern
      //   as auth:validation.*.
      // loans:current.overdue / loans:list.overdueStatus — i18next
      //   pluralization on the days-overdue badge; the extractor needs
      //   the wildcard to keep the `_one` / `_other` suffixes after a
      //   re-run (otherwise the suffix gets re-stamped as a placeholder
      //   string that points back at itself, which CI then surfaces as
      //   "key has changed").
      "loans:validation.*",
      "loans:current.overdue*",
      "loans:list.overdueStatus*",
      // loans:list.state* — LoansListPage builds the tab labels via
      //   `t(\`loans:list.state${capitalize(s)}\`)` over the closed
      //   LoanState union (all/open/overdue/returned). The extractor
      //   sees only the template literal.
      "loans:list.state*",
      // services:* — same patterns as the loans namespace. Schema
      //   messages flow through RHF errors[name].message at render time;
      //   the list-page tab labels are built via a template literal over
      //   the ServiceState union (all/open/overdue/completed); plural
      //   suffixes need surviving wildcards. See #1508.
      "services:validation.*",
      "services:current.overdue*",
      "services:list.overdueStatus*",
      "services:list.state*",
      // commodities:detail.historyEvent.serviceField.* — built from
      //   `t(\`commodities:detail.historyEvent.serviceField.${key}\`)` over
      //   the mutable service fields the BE event payload tracks (provider_*,
      //   reason, expected_return_at, cost). Dynamic keys.
      "commodities:detail.historyEvent.serviceField.*",
      // groups:migration.status.* — CurrencyMigrationStatusBadge resolves
      //   the badge label via `t(\`groups:migration.status.${status}\`)` over
      //   the closed CurrencyMigrationStatus union (pending/running/
      //   completed/failed). #1553.
      // groups:settings.dialog.step* — the wizard's WizardSteps maps over
      //   a const items array `{ index, titleKey: "groups:settings.dialog.stepN" }`
      //   and resolves each label via `t(item.titleKey)`. Static-analysis
      //   only sees `t(item.titleKey)`, not the four step labels behind it.
      "groups:migration.status.*",
      "groups:settings.dialog.step*",
    ],
  },
})
