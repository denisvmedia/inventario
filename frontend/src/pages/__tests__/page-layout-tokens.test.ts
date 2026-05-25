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
  "areas/AreaItemsPanel.tsx",
  "admin/AdminLayout.tsx",
  "admin/AdminPagination.tsx",
  "admin/admin-shared.tsx",
])

// Forbidden max-w literals on top-level page wrappers. The full set of
// Tailwind v4 max-w tokens — pages must not reach for any of these
// directly. (Smaller `max-w-prose` / `max-w-md` etc. are fine inside form
// or banner regions; the guard only looks at the *outermost* element of
// each return statement.)
const FORBIDDEN_MAX_W = /max-w-(none|xs|sm|md|lg|xl|2xl|3xl|4xl|5xl|6xl|7xl|prose|screen-[a-z]+)/

function walk(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir)) {
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

// Returns the className value of the OUTERMOST element of each return
// statement in the source. Simple regex — captures `<X className="…"` where
// X is a div, Page, Frame, header, main, section, or fragment-but-typed.
// JSX fragments (<>) are followed by the next element, which we recurse
// into one level (covers `return ( <> <RouteTitle/> <div className=…> )`).
function outermostElementClasses(src: string): string[] {
  const classes: string[] = []
  // Match `return (` followed by JSX. Capture multi-line up to the matching `)`.
  // We can't fully parse — we just look for the first non-fragment opener
  // and extract its className.
  const returns = src.matchAll(/return\s*\(\s*([\s\S]*?)\n\s{0,4}\)/g)
  for (const ret of returns) {
    const body = ret[1]
    // Skip lines that look like inline JSX inside a non-top-level function
    // (e.g. small helper components). Heuristic: only look at the first
    // ~30 lines of each match.
    const head = body.split("\n").slice(0, 30).join("\n")

    // Strip a leading fragment opener so we see the next real element.
    const noFragment = head.replace(/^\s*<>\s*/, "")
    // The outermost element after stripping any leading <RouteTitle … />
    // (route titles are head-only and don't carry width concerns).
    const noRouteTitle = noFragment.replace(/<RouteTitle[^>]*\/>\s*/, "")

    // Look for the first `<Tag …>` whose tag name starts with [A-Za-z].
    const first = noRouteTitle.match(/<([A-Za-z][\w.]*)\b([^>]*)>/)
    if (!first) continue
    const tag = first[1]
    const attrs = first[2]

    // Pull className value if present (single or double quoted, template literals).
    const classMatch =
      attrs.match(/className\s*=\s*"([^"]+)"/) ||
      attrs.match(/className\s*=\s*'([^']+)'/) ||
      attrs.match(/className\s*=\s*\{`([^`]+)`\}/)
    if (classMatch) {
      classes.push(`${tag}::${classMatch[1]}`)
    } else {
      classes.push(`${tag}::`)
    }
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
        const [tag, className] = o.split("::")
        // The `<Page>` wrapper is the canonical entry — width comes from
        // the `width` prop, not a className literal. Pages that wrap with
        // <Page> may add other classes (`gap-8`, `relative`, …) but never
        // a raw `max-w-*` literal. Other top-level wrappers (`<div>`,
        // `<header>`, `<main>`) must also not carry a `max-w-*` literal.
        // No bypass for any tag — `<Page className="max-w-3xl">` would
        // defeat the contract too (Copilot/CodeRabbit feedback on #1889).
        if (FORBIDDEN_MAX_W.test(className ?? "")) {
          violations.push(`${rel}: <${tag} className="…${className}…">`)
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
      // Match `<Page` or `<PageFrame` opening tags. `\b` boundary stops
      // it from matching `<PageHeader` (which is a separate sibling, not
      // a wrapper) or `<Pages` (no such component, but defensive).
      const rendersPageWrapper = /<Page(?:Frame)?\s/.test(src) || /<Page(?:Frame)?>/.test(src)
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
