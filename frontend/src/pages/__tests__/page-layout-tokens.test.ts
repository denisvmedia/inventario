import { readFileSync, readdirSync, statSync } from "node:fs"
import { join } from "node:path"

import { describe, expect, it } from "vitest"

/*
  Page-layout convention guard — issue #1889.

  Top-level routes under `src/pages/**` must compose with the canonical
  `<Page>` wrapper (or sit in the special-purpose allow list below). This
  test enforces three rules:

  1. **No ad-hoc page widths.** A top-level route MUST NOT use a raw
     `max-w-*` literal (or the token classes `max-w-page-narrow` /
     `max-w-page-wide`) on its outermost wrapper. The width comes from
     `<Page width="narrow" | "wide" | "full">` and nothing else. Applies
     to ALL tags including `<Page>` itself — `<Page className="max-w-3xl">`
     would defeat the contract.

  2. **<Page> is imported and rendered.** Top-level routes must (a)
     import `@/components/ui/page` and (b) actually open a `<Page>` (or
     `<PageFrame>`, the dual-mode helper in `CommodityDetailPage`) JSX
     element. The combined check kills the "unused import" case the
     pure-import regex used to miss. We deliberately do NOT enforce
     "<Page> is the outermost element of every return" — that needs a
     real AST parser to handle inline JSX expressions whose `)` lives at
     the same indent as the `return`'s closer, and the import + JSX-use
     pair already catches the realistic failure modes. Header
     consistency falls out of this naturally: a page using `<Page>` is
     free to compose `<PageHeader>` (the canonical path) or, for
     entity-detail surfaces with custom chrome (icon + breadcrumb +
     action cluster), a hand-rolled header that follows canonical
     typography (`text-2xl/3xl font-semibold tracking-tight`). We don't
     grep typography directly because the false-positive rate is too
     high; the `<PageHeader>` primitive in `components/ui/page.tsx`
     covers the canonical path on its own.

  3. **Width tokens stay singular.** The `max-w-page-narrow` /
     `max-w-page-wide` utilities only exist inside the `<Page>` component
     (and the UIShowcasePage demo block) — pages shouldn't reach for them
     directly. Discourages drift around the abstraction.

  The guard is intentionally low-tech: it does line-based scanning rather
  than parsing JSX. False-positives are cheap (add to the allow list);
  false-negatives are dangerous, so we keep the regex broad.
*/

const PAGES_DIR = join(__dirname, "..")

// Special-purpose pages — error states, auth cards, modals, print, redirects.
// These have bespoke layouts that intentionally diverge from the canonical
// Page + PageHeader pattern (centered card, full-bleed media, etc.) and so
// are exempt from the width/h1 conventions.
const SPECIAL_PURPOSE_PAGES = new Set<string>([
  // Error/empty-state screens (centered card layout).
  "MaintenancePage.tsx",
  "NoGroupPage.tsx",
  "NotFound.tsx",
  "Placeholder.tsx",
  "UnexpectedErrorPage.tsx",
  "RootRedirect.tsx",
  "admin/AdminForbiddenPage.tsx",
  // Auth pages — own card-centered layout.
  "auth/ForgotPasswordPage.tsx",
  "auth/InviteAcceptPage.tsx",
  "auth/LoginPage.tsx",
  "auth/RegisterPage.tsx",
  "auth/ResetPasswordPage.tsx",
  "auth/VerifyEmailPage.tsx",
  "backoffice/BackofficeLoginPage.tsx",
  // Modal / print / sub-panel / layout shells.
  "commodities/CommodityCreateModal.tsx",
  "commodities/CommodityPrintPage.tsx",
  // Print-capable insurance report (#1370) — bespoke print sheet layout,
  // same precedent as CommodityPrintPage (toolbar + .print-sheet, no
  // <Page> wrapper). The report sheet provides its own max-width frame.
  "reports/InsuranceReportPage.tsx",
  "areas/AreaItemsPanel.tsx",
  "admin/AdminLayout.tsx",
  "admin/AdminPagination.tsx",
  "admin/admin-shared.tsx",
])

// Forbidden max-w literals on top-level page wrappers. Matches every
// Tailwind v4 max-w form pages might reach for — named tokens, arbitrary
// bracketed values (`max-w-[780px]`), CSS-variable references
// (`max-w-(--my-var)`), and broader screen variants
// (`max-w-screen-2xl`, etc). The `\b` boundary keeps the regex from
// matching inside other attribute names. Smaller `max-w-md` etc. inside
// form / banner regions are fine — the guard only inspects the outermost
// element of each return statement.
const FORBIDDEN_MAX_W = /\bmax-w-(?:\[[^\]]+\]|\([^)]+\)|[A-Za-z0-9_-]+)/

// Tags whose outermost-position role is "page wrapper" or "page-shaped
// alternative" (centred-card error state, etc). Bare-JSX returns from
// helper sub-components — `<span>`, `<button>`, `<Badge>`, `<Card>`,
// `<Link>`, etc. — are skipped, since those aren't roots of a route and
// frequently use `max-w-*` for their own inner sizing (e.g. a TenantChip
// `<span className="max-w-40">`).
const PAGE_WRAPPER_TAGS = new Set([
  "div",
  "section",
  "main",
  "header",
  "article",
  "Page",
  "PageFrame",
])

function walk(dir: string): string[] {
  const out: string[] = []
  // Sort entries up-front so the order of `violations` (and the test
  // failure output that ends up in CI logs) is stable across operating
  // systems and filesystems (Copilot feedback on #1889 — `readdirSync`
  // is OS-dependent without an explicit sort).
  const entries = readdirSync(dir).slice().sort()
  for (const entry of entries) {
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) {
      // Skip the tests directory itself.
      if (entry === "__tests__") continue
      out.push(...walk(full))
      continue
    }
    if (!entry.endsWith(".tsx")) continue
    if (entry.endsWith(".test.tsx")) continue
    out.push(full)
  }
  return out
}

function relativeName(path: string): string {
  return path.slice(PAGES_DIR.length + 1)
}

// Returns the outermost element of each return statement in the source,
// encoded as `${tag}::${attrs}`. The `attrs` slice is the full attribute
// region of the opening tag — keeping it raw (rather than parsing out the
// quoted `className` literal) means expressions like
// `className={cn("max-w-3xl", ...)}` still flow into the FORBIDDEN_MAX_W
// regex, instead of being silently skipped (CodeRabbit feedback on #1889).
//
// We capture three return shapes:
//   1. `return ( <Tag …> … </Tag> )`             — parenthesised JSX block
//   2. `return ( <> <RouteTitle/> <Tag …> … )`   — fragment + RouteTitle prefix
//   3. `return <Tag …>` / `return <Tag … />`     — bare JSX return (no parens)
//
// We can't fully parse JSX from a regex — this stays a heuristic. The
// fallback for anything we don't catch is the import + `<Page>` check
// (rule 2), which the violations-without-coverage would still fail.
function outermostElementClasses(src: string): string[] {
  const classes: string[] = []
  type Candidate = string

  // Form 1+2: parenthesised return blocks.
  const blockReturns = src.matchAll(/return\s*\(\s*([\s\S]*?)\n\s{0,4}\)/g)
  const blockHeads: Candidate[] = Array.from(blockReturns, (m) =>
    m[1].split("\n").slice(0, 30).join("\n")
  )

  // Form 3: bare-JSX returns — `return <Tag …>` on the same line, no parens.
  // Capture up to the rest of the line plus a few continuation lines so an
  // attribute-heavy opening tag still resolves.
  const inlineReturns = src.matchAll(/return\s+(<[A-Za-z][^\n]*(?:\n[^)]*?)?)/g)
  const inlineHeads: Candidate[] = Array.from(inlineReturns, (m) => m[1])

  const heads = [...blockHeads, ...inlineHeads]
  for (const head of heads) {
    // Strip a leading fragment opener so we see the next real element.
    const noFragment = head.replace(/^\s*<>\s*/, "")
    // Strip a leading <RouteTitle … /> (route titles are head-only and
    // don't carry width concerns).
    const noRouteTitle = noFragment.replace(/<RouteTitle[^>]*\/>\s*/, "")

    // Look for the first `<Tag …>` whose tag name starts with [A-Za-z].
    const first = noRouteTitle.match(/<([A-Za-z][\w.]*)\b([^>]*)>/)
    if (!first) continue
    const tag = first[1]
    const attrs = first[2]
    classes.push(`${tag}::${attrs}`)
  }
  return classes
}

describe("page-layout convention guard (issue #1889)", () => {
  const allPages = walk(PAGES_DIR).map((abs) => ({
    rel: relativeName(abs),
    src: readFileSync(abs, "utf-8"),
  }))

  it("scans at least one page (sanity)", () => {
    expect(allPages.length).toBeGreaterThan(20)
  })

  it("no top-level page wrapper uses a raw max-w-* literal outside the special-purpose allow list", () => {
    const violations: string[] = []
    for (const { rel, src } of allPages) {
      if (SPECIAL_PURPOSE_PAGES.has(rel.replace(/\\/g, "/"))) continue
      const outers = outermostElementClasses(src)
      for (const o of outers) {
        // `attrs` is the full attribute slice of the opening tag — keeping
        // it raw means className expressions like `className={cn(...)}`
        // are still inspectable (CodeRabbit feedback on #1889). The
        // `\b` boundary in FORBIDDEN_MAX_W keeps the regex from matching
        // inside other attribute names that happen to contain "max-w-".
        const sep = o.indexOf("::")
        const tag = o.slice(0, sep)
        const attrs = o.slice(sep + 2)
        // Only inspect tags that could plausibly be a page wrapper. Helper
        // sub-components defined alongside the page (TenantChip `<span>`,
        // status `<Badge>`, etc.) routinely set their own inner max-w-*
        // and should not be flagged.
        if (!PAGE_WRAPPER_TAGS.has(tag)) continue
        // The `<Page>` wrapper is the canonical entry — width comes from
        // the `width` prop, not a className literal. Pages that wrap with
        // <Page> may add other classes (`gap-8`, `relative`, …) but never
        // a raw `max-w-*` literal. Other top-level wrappers (`<div>`,
        // `<header>`, `<main>`) must also not carry a `max-w-*` literal.
        // No bypass for any tag in the allow list — `<Page className="max-w-3xl">`
        // would defeat the contract too (Copilot/CodeRabbit feedback on
        // #1889).
        const match = attrs.match(FORBIDDEN_MAX_W)
        if (match) {
          violations.push(`${rel}: <${tag} …${match[0]}…>`)
        }
      }
    }
    if (violations.length > 0) {
      throw new Error(
        `Found ${violations.length} ad-hoc max-w-* on top-level page wrapper(s):\n` +
          violations.map((v) => `  - ${v}`).join("\n") +
          `\n\nReplace with <Page width="narrow" | "wide" | "full"> from "@/components/ui/page". ` +
          `If the page is a centered error/auth/empty-state surface, add it to SPECIAL_PURPOSE_PAGES.`
      )
    }
    expect(violations).toEqual([])
  })

  it("non-special pages import and render the canonical <Page> primitive", () => {
    const violations: string[] = []
    for (const { rel, src } of allPages) {
      const normalised = rel.replace(/\\/g, "/")
      if (SPECIAL_PURPOSE_PAGES.has(normalised)) continue
      // Sub-panels mounted by other pages (and the AdminLayout itself,
      // which provides the Page wrapper for /admin/* children) don't need
      // their own Page.
      if (normalised === "admin/AdminLayout.tsx") continue
      if (normalised === "areas/AreaItemsPanel.tsx") continue
      // Admin sub-pages live under AdminLayout (which wraps them in
      // <Page>) and don't need their own. Heuristic: any file under
      // admin/ that's not the layout itself.
      if (normalised.startsWith("admin/")) continue
      // Two layered checks (Copilot / CodeRabbit feedback on #1889):
      //   1. `@/components/ui/page` is imported — kills the case where
      //      the import was added but nothing in the file uses it.
      //   2. `<Page>` (or the `PageFrame` dual-mode helper from
      //      `CommodityDetailPage`) is opened somewhere in the JSX, NOT
      //      just referenced in a string or a comment.
      // We deliberately stop short of requiring `<Page>` to be the
      // OUTERMOST element of every return. Doing that rigorously needs
      // a JSX/TS AST parser — a regex over `return (...)` blocks gets
      // confused by inline expressions whose `)` sits at the same indent
      // as the return's own closer, picking up a substring of the body
      // instead of the full return. The import + JSX-use check catches
      // the realistic failure modes (unused imports, nested-only usage)
      // without that cost; structural enforcement of "Page is the root
      // wrapper" lives in the rule-1 max-w guard above (which flags any
      // ad-hoc `max-w-*` on the actual outermost element across all
      // returns, regardless of tag).
      const importsPageModule = /from\s+["']@\/components\/ui\/page["']/.test(src)
      // Concat the bodies of all top-level `return (...)` blocks and look
      // for a `<Page>` or `<PageFrame>` opening tag inside that scope.
      // Scoping to return blocks (rather than scanning raw `src`) means a
      // stray `<Page>` literal in a JSDoc comment or a string can't spoof
      // the guard (CodeRabbit feedback on #1889). `\b` boundary keeps the
      // regex from matching `<PageHeader` (a sibling, not a wrapper) or a
      // hypothetical `<Pages>`.
      const returnJsx = Array.from(
        src.matchAll(/return\s*\(\s*([\s\S]*?)\n\s{0,4}\)/g),
        (m) => m[1]
      ).join("\n")
      const rendersPageWrapper = /<Page(?:Frame)?\b[^>]*\/?>/.test(returnJsx)
      if (!(importsPageModule && rendersPageWrapper)) {
        violations.push(rel)
      }
    }
    if (violations.length > 0) {
      throw new Error(
        `Found ${violations.length} top-level page(s) without <Page> from "@/components/ui/page":\n` +
          violations.map((v) => `  - ${v}`).join("\n") +
          `\n\nWrap the top-level return with <Page width="narrow" | "wide" | "full">. ` +
          `If the page is intentionally bespoke (error/auth/print), add it to SPECIAL_PURPOSE_PAGES.`
      )
    }
    expect(violations).toEqual([])
  })

  it("page width tokens (max-w-page-narrow / max-w-page-wide) only appear in <Page> or the UIShowcase demo", () => {
    const violations: string[] = []
    for (const { rel, src } of allPages) {
      if (rel.replace(/\\/g, "/") === "UIShowcasePage.tsx") continue
      if (/max-w-page-(narrow|wide)/.test(src)) {
        violations.push(rel)
      }
    }
    if (violations.length > 0) {
      throw new Error(
        `Found ${violations.length} page(s) reaching for max-w-page-* directly:\n` +
          violations.map((v) => `  - ${v}`).join("\n") +
          `\n\nUse <Page width="narrow" | "wide"> instead of the raw token class.`
      )
    }
    expect(violations).toEqual([])
  })
})
