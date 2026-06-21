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
      // errors:validation.* — backend field-validation messages keyed by the
      //   BE's stable validation code, resolved via
      //   `i18next.t(\`errors:validation.${code}\`)` in lib/form-errors.ts
      //   (applyServerFieldErrors). The codes come off the 422 `errorCodes`
      //   tree at runtime, so the extractor only sees the template literal. (#1990)
      "errors:validation.*",
      // common:onboarding.steps.* — OnboardingTour renders each of the 7
      //   step titles + descriptions via
      //   `t(\`common:onboarding.steps.${step.key}.title\`)` over the closed
      //   StepKey union (welcome, addItem, navDashboard, navLocations,
      //   navItems, navWarranties, navFiles). Dynamic keys; the extractor
      //   sees only the template literal. (#1543)
      "common:onboarding.steps.*",
      "auth:validation.*",
      "auth:passwordStrength.*",
      "auth:session.*",
      "settings:sections.*",
      "settings:appearance.themeOptions.*",
      "settings:appearance.localeOptions.*",
      "settings:appearance.defaultViewOptions.*",
      // settings:appearance.numberFormatLocaleOptions.* — AppearanceSection
      //   builds one <option> per BCP-47 tag in NUMBER_FORMAT_LOCALE_OPTIONS,
      //   so keys are interpolated via a template literal that the extractor
      //   can't enumerate statically. (#1683)
      "settings:appearance.numberFormatLocaleOptions.*",
      "settings:help.rows.*",
      // feedback:types.* — FeedbackDialog iterates the FEEDBACK_TYPES
      //   union (feedback / bug / feature / question) and resolves each
      //   chip label via `t(\`feedback:types.${type}\`)`. (#1387)
      // feedback:validation.* — zod schema messages in the dialog are
      //   plain strings, surfaced via RHF errors[name].message → t() at
      //   render time. Same pattern as auth:validation.*.
      "feedback:types.*",
      "feedback:validation.*",
      // settings:notifications.{groups,rows,errors}.* — NotificationsSection
      //   builds keys from a NOTIFICATION_GROUPS registry (group ids:
      //   reminders/updates/channels; row ids: warrantyExpiry,
      //   maintenanceReminder, weeklyDigest, priceDrop, channelEmail,
      //   channelPush). The extractor sees the string literals stashed in
      //   the registry but not the t() call sites that consume them.
      "settings:notifications.groups.*",
      "settings:notifications.rows.*",
      "settings:notifications.errors.*",
      // settings:storage.breakdown.* — StorageCard maps over BREAKDOWN_KEYS
      //   (photos/invoices/documents/exports/other) and resolves each label
      //   via `t(\`settings:storage.breakdown.${key}\`)`. The extractor only
      //   sees the template literal.
      "settings:storage.breakdown.*",
      // groups:validation.* — schema messages in features/group/schemas.ts
      // are plain strings, surfaced through RHF errors[name].message →
      // t() at render time. Same pattern as auth:validation.*.
      "groups:validation.*",
      // groups:settings.sections.* — GroupSettingsPage's sidebar nav
      // resolves entries via `t(\`groups:settings.sections.${id}\`)`
      // where id is one of "info" | "members" | "data" | "management"
      // (SectionId union, narrowed at compile time). Same pattern as
      // settings:sections.* on the user Preferences page — extractor
      // sees only the template literal.
      "groups:settings.sections.*",
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
      // locations:deleteWithItems.* — DeleteWithItemsDialog (#2137) builds
      //   the per-container copy via `t(\`locations:deleteWithItems.${kind}Title\`)`
      //   etc. over the closed DeleteContainerKind union (area | location),
      //   plus i18next plural suffixes on the *Description / *UnlinkHelp /
      //   cascadeHelp lines. The extractor sees only the template literals
      //   for the kind-prefixed keys, so preserve the whole subtree.
      "locations:deleteWithItems.*",
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
      // commodities:form.step.ai.review.confidence.* — AiScanStep
      //   resolves the confidence chip via
      //   `t(\`commodities:form.step.ai.review.confidence.${band}\`)`
      //   over the closed `high` | `medium` | `low` union, with the
      //   percent value interpolated from the per-field guess. Same
      //   pattern as commodities:warrantyStatus.*. (#1720)
      // commodities:form.step.ai.errors.* — typed BE codes
      //   (`commodity_scan.<kind>`) map onto these titles via the
      //   errorCodeTitle switch inside AiScanStep. The switch lists
      //   each code statically, but the wildcard documents the i18n
      //   namespace and protects the keys from a re-extract pass.
      "commodities:form.step.ai.review.confidence.*",
      "commodities:form.step.ai.errors.*",
      "commodities:form.step.ai.offer.staged.title*",
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
      //   over the closed `all/images/invoices/documents/other` union
      //   (renamed `photos`→`images` + `other` chip added in the
      //   71c7f9c update).
      // commodities:detail.filesTab.{drop,empty,cta}* — same chip-bar
      //   surface resolves the upload-zone copy, the chip-aware empty
      //   state, and the per-row View/Open/Download CTA via dynamic
      //   template lookups against ChipDef + a mime-keyed lookup. The
      //   extractor sees only the template literal in each case.
      "commodities:detail.filesTab.chip.*",
      "commodities:detail.filesTab.drop*",
      "commodities:detail.filesTab.empty*",
      "commodities:detail.filesTab.cta*",
      // commodities:detail.statusTransitionDialog.{description,errors,
      //   notePlaceholder}.* — StatusTransitionDialog (#1611) builds copy
      //   via `t(`commodities:detail.statusTransitionDialog.description.${targetStatus}`)`
      //   over the closed forward-transition set
      //   (sold/lost/disposed/written_off), plus dynamic error-message
      //   keys flowing through RHF errors[name].message (zod schemas).
      "commodities:detail.statusTransitionDialog.*",
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
      // maintenance:validation.* — zod schema messages in
      //   features/maintenance/schemas.ts surface through RHF
      //   errors[name].message at render time. Same pattern as
      //   loans / services. #1368.
      // maintenance:row.dueInDays{,_one,_other} /
      //   overdueByDays{,_one,_other} / intervalLabel{,_one,_other} —
      //   plural variants resolved via the `count` interpolation,
      //   extractor sees only the base key.
      "maintenance:validation.*",
      "maintenance:row.dueInDays*",
      "maintenance:row.overdueByDays*",
      "maintenance:row.intervalLabel*",
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
      // groups:settings.sections.* — GroupSettingsPage's nav resolves each
      //   item via `t(\`groups:settings.sections.${id}\`)` over the closed
      //   SectionId union (info/members/data/management). The info,
      //   members, and management section bodies also call
      //   `t("groups:settings.sections.<id>")` explicitly for their
      //   `<SectionTitle>`, so the extractor finds those keys; the `data`
      //   section uses a different title (`groups:settings.data.title`)
      //   so its sidebar label is reachable only through the template
      //   literal. Preserve the whole subtree so future section
      //   additions don't trip the same drift. (#1637)
      "groups:migration.status.*",
      "groups:settings.dialog.step*",
      "groups:settings.sections.*",
      // settings:loginHistory.outcomes.* + .methods.* — LoginHistoryPage
      //   resolves the badge label via a static OUTCOME_I18N_KEY /
      //   METHOD_I18N_KEY lookup map keyed on the BE enum value
      //   (models.LoginOutcome / LoginMethod). The extractor sees only
      //   `t(OUTCOME_I18N_KEY[outcome])` so each per-variant key has to
      //   survive the sweep via the wildcard.
      "settings:loginHistory.outcomes.*",
      "settings:loginHistory.methods.*",
      // common:serverError.*.title — ServerErrorBanner picks the title via
      //   `t(\`common:serverError.${kind}.title\`)` over the closed
      //   ServerErrorKind union (network/validation/conflict/unknown). The
      //   extractor sees only the template literal; the four titles live in
      //   en/common.json's serverError subtree.
      "common:serverError.*",
      // common:shortcuts.categories.* — KeyboardShortcutsDialog (#1385)
      //   resolves each section heading via
      //   `t(\`common:shortcuts.categories.${category}\`)` over the closed
      //   ShortcutCategoryKey union (search/navigation/actions/layout/help).
      // common:shortcuts.entries.* — every ShortcutDef in
      //   features/shortcuts/registry.ts carries the label as a fully
      //   qualified key under this subtree; the dialog reads it back via
      //   `t(entry.labelKey)`, so the extractor sees only the lookup.
      "common:shortcuts.categories.*",
      "common:shortcuts.entries.*",
      // admin:nav.* — AdminLayout's secondary nav resolves each entry via
      //   `t(entry.labelKey)` over the ADMIN_NAV const array (tenants /
      //   users / groups). admin:nav.tenants is also reached statically
      //   through useNavLabel's switch; the users / groups labels are
      //   reachable only through the template-literal lookup.
      // admin:tenants.stats.* — AdminTenantsPage maps over a STAT_TILES
      //   const array and resolves each label via `t(stat.labelKey)`.
      // admin:tenants.status.* — admin-shared.tsx's TenantStatusBadge
      //   resolves the label via `t(\`tenants.status.${status}\`)` over the
      //   closed tenant-status union (active/inactive/suspended). Dynamic
      //   key; the extractor sees only the template literal.
      // admin:tenantDetail.groups.status.* — admin-shared.tsx's
      //   GroupStatusBadge resolves the label via
      //   `t(\`tenantDetail.groups.status.${status}\`)` over the closed
      //   group-status union (active/pending_deletion). Same pattern.
      "admin:nav.*",
      "admin:tenants.stats.*",
      "admin:tenants.status.*",
      "admin:tenantDetail.groups.status.*",
      // admin:userDetail.errors.* — AdminUserDetailPage resolves the inline
      //   block/unblock error banner via `t(\`userDetail.errors.${errorKey}\`)`
      //   where errorKey is a flat segment mapped from the BE's dotted 422
      //   codes (BLOCK_ERROR_KEY: selfBlocked / adminRequiresForce /
      //   reasonRequired / reasonTooLong). Dynamic key; the extractor sees
      //   only the template literal.
      // admin:userDetail.roles.* — admin-shared.tsx's RoleBadge resolves the
      //   membership-role label via `t(ROLE_CONFIG[role].i18nKey)` over the
      //   closed models.GroupRole union (viewer/user/admin/owner).
      // admin:userDetail.sessions.count* — the session-count summary calls
      //   `t("userDetail.sessions.count", { count })`; i18next re-stamps the
      //   `_one` / `_other` plural suffixes on every extract, so the wildcard
      //   keeps them stable.
      "admin:userDetail.errors.*",
      "admin:userDetail.roles.*",
      "admin:userDetail.sessions.count*",
      // admin:groupDetail.members.errors.* — MembershipEditor.tsx resolves
      //   the inline add / remove / role-change error banners via
      //   `t(\`groupDetail.members.errors.${suffix}\`)` where `suffix` is a
      //   flat segment mapped from the BE's dotted 422 codes (MEMBER_ERROR_KEY:
      //   tenantMismatch / invalidRole / lastOwner / lastMember) plus a
      //   `generic` catch-all. Dynamic key; the extractor sees only the
      //   template literal.
      "admin:groupDetail.members.errors.*",
    ],
  },
})
