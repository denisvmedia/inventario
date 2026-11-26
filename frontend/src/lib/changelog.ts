// CHANGELOG.md as data for the What's new page (#1386).
//
// The Vite plugin in `vite-plugins/whats-new.ts` runs this at build time:
// it parses the repository's CHANGELOG.md, keeps the released versions the
// build contains, and hands the result to the app as a virtual module. The
// browser never parses Markdown. This module is pure (no DOM, no Node
// APIs) so the plugin, the app and the tests share it.
//
// The parser reads the subset of Markdown the changelog uses: `## [x.y.z] -
// date` release headings, `### Section` headings, `- ` list items with
// two-space continuation lines, paragraphs, and inline code, bold and links.
// Anything else in a release is kept as paragraph text rather than dropped.

export type ChangelogInline =
  | { type: "text"; text: string }
  | { type: "code"; text: string }
  | { type: "strong"; children: ChangelogInline[] }
  | { type: "link"; href: string; children: ChangelogInline[] }

export interface ChangelogSection {
  // The heading as written: "Added", "Fixed", "Operators", …
  title: string
  // Text under the heading that is not a list item.
  paragraphs: ChangelogInline[][]
  entries: ChangelogInline[][]
}

export interface ChangelogRelease {
  // Without the "v": "0.2.2".
  version: string
  // ISO date from the heading, or null when the heading has none.
  date: string | null
  // Text between the release heading and its first section, such as the
  // pointer 0.1.0 carries to its release notes.
  paragraphs: ChangelogInline[][]
  sections: ChangelogSection[]
}

// The section for the people who run the server. CONTRIBUTING.md puts it
// last in each release; the page shows it to system administrators only.
export const OPERATORS_SECTION = "Operators"

export interface ParseChangelogOptions {
  // Base URL relative links resolve against, e.g. the repository's blob
  // URL on master. Links that do not resolve to http(s) render as text.
  linkBase: string
}

const RELEASE_HEADING = /^##\s+\[([^\]]+)\](?:\s+-\s+(\d{4}-\d{2}-\d{2}))?/
const SECTION_HEADING = /^###\s+(.+?)\s*#*\s*$/
const LINK_DEFINITION = /^\[[^\]]+\]:\s/

// parseChangelog returns every released version in the file, in file order.
// The Unreleased section and anything that is not a version are skipped.
export function parseChangelog(
  markdown: string,
  options: ParseChangelogOptions
): ChangelogRelease[] {
  const releases: ChangelogRelease[] = []
  let release: ChangelogRelease | null = null
  let section: ChangelogSection | null = null
  // Lines of the block being read: a list item or a paragraph.
  let block: { kind: "entry" | "paragraph"; lines: string[] } | null = null

  const flush = () => {
    if (!block || !release) {
      block = null
      return
    }
    const inlines = parseInline(block.lines.join(" "), options.linkBase)
    if (block.kind === "entry" && section) {
      section.entries.push(inlines)
    } else if (section) {
      section.paragraphs.push(inlines)
    } else {
      release.paragraphs.push(inlines)
    }
    block = null
  }

  for (const raw of markdown.split(/\r?\n/)) {
    const line = raw.replace(/\s+$/, "")

    if (line.startsWith("## ")) {
      flush()
      section = null
      const match = RELEASE_HEADING.exec(line)
      const version = match ? normalizeVersion(match[1]) : null
      if (match && version) {
        release = { version, date: match[2] ?? null, paragraphs: [], sections: [] }
        releases.push(release)
      } else {
        // [Unreleased], or a heading that names no version.
        release = null
      }
      continue
    }
    if (!release) continue

    const sectionMatch = SECTION_HEADING.exec(line)
    if (sectionMatch) {
      flush()
      section = { title: sectionMatch[1], paragraphs: [], entries: [] }
      release.sections.push(section)
      continue
    }
    if (LINK_DEFINITION.test(line)) {
      // The link definitions at the foot of the file end the last release.
      flush()
      release = null
      section = null
      continue
    }
    if (line === "") {
      // A blank line ends a paragraph. A list item may go on after it with
      // an indented line, so it stays open until something else starts.
      if (block?.kind === "paragraph") flush()
      continue
    }
    if (/^[-*]\s+/.test(line)) {
      flush()
      block = { kind: "entry", lines: [line.replace(/^[-*]\s+/, "")] }
      continue
    }
    if (/^\s+/.test(line) && block?.kind === "entry") {
      block.lines.push(line.trim())
      continue
    }
    // Paragraph text; a deeper heading reads as its text.
    if (block?.kind === "entry") flush()
    const text = line.replace(/^#{4,}\s+/, "").trim()
    if (block?.kind === "paragraph") {
      block.lines.push(text)
    } else {
      block = { kind: "paragraph", lines: [text] }
    }
  }
  flush()

  // Drop sections left without content, e.g. a heading with nothing under it.
  for (const r of releases) {
    r.sections = r.sections.filter((s) => s.entries.length > 0 || s.paragraphs.length > 0)
  }
  return releases
}

// parseInline turns the inline Markdown of one entry into tokens: `code`,
// **bold** and [text](url). Everything else is text.
export function parseInline(source: string, linkBase: string): ChangelogInline[] {
  const out: ChangelogInline[] = []
  let text = ""
  const pushText = () => {
    if (text) out.push({ type: "text", text })
    text = ""
  }

  let i = 0
  while (i < source.length) {
    const ch = source[i]
    if (ch === "`") {
      const end = source.indexOf("`", i + 1)
      if (end > i) {
        pushText()
        out.push({ type: "code", text: source.slice(i + 1, end) })
        i = end + 1
        continue
      }
    }
    if (ch === "*" && source.startsWith("**", i)) {
      const end = findClosing(source, "**", i + 2)
      if (end > i + 2) {
        pushText()
        out.push({ type: "strong", children: parseInline(source.slice(i + 2, end), linkBase) })
        i = end + 2
        continue
      }
    }
    if (ch === "[") {
      const link = readLink(source, i)
      if (link) {
        pushText()
        const children = parseInline(link.label, linkBase)
        const href = resolveHref(link.href, linkBase)
        if (href) {
          out.push({ type: "link", href, children })
        } else {
          out.push(...children)
        }
        i = link.end
        continue
      }
    }
    if (ch === "\\" && i + 1 < source.length && /[\\`*_[\]()#!-]/.test(source[i + 1])) {
      text += source[i + 1]
      i += 2
      continue
    }
    text += ch
    i++
  }
  pushText()
  return out
}

// findClosing finds `marker` at or after `from`, skipping code spans so a
// `**` inside backticks does not close the bold text.
function findClosing(source: string, marker: string, from: number): number {
  let i = from
  while (i < source.length) {
    if (source[i] === "`") {
      const end = source.indexOf("`", i + 1)
      if (end < 0) return -1
      i = end + 1
      continue
    }
    if (source.startsWith(marker, i)) return i
    i++
  }
  return -1
}

// readLink reads `[label](href)` starting at `start`, allowing brackets
// nested in the label and a code span that contains brackets.
function readLink(
  source: string,
  start: number
): { label: string; href: string; end: number } | null {
  let depth = 0
  let i = start
  for (; i < source.length; i++) {
    const ch = source[i]
    if (ch === "`") {
      const end = source.indexOf("`", i + 1)
      if (end < 0) return null
      i = end
      continue
    }
    if (ch === "[") depth++
    if (ch === "]") {
      depth--
      if (depth === 0) break
    }
  }
  if (depth !== 0 || source[i + 1] !== "(") return null
  const close = closingParen(source, i + 2)
  if (close < 0) return null
  const href = source.slice(i + 2, close).trim()
  if (!href || /\s/.test(href)) return null
  return { label: source.slice(start + 1, i), href, end: close + 1 }
}

// closingParen finds the `)` that ends a link destination starting at
// `from`. Parentheses inside the destination nest, as in
// `https://en.wikipedia.org/wiki/Foo_(bar)`, the way CommonMark reads them.
function closingParen(source: string, from: number): number {
  let depth = 0
  for (let i = from; i < source.length; i++) {
    const ch = source[i]
    if (ch === "(") depth++
    if (ch === ")") {
      if (depth === 0) return i
      depth--
    }
  }
  return -1
}

function resolveHref(href: string, linkBase: string): string | null {
  try {
    const url = new URL(href, linkBase)
    return url.protocol === "https:" || url.protocol === "http:" ? url.toString() : null
  } catch {
    return null
  }
}

// --- Versions -------------------------------------------------------------

interface SemVer {
  major: number
  minor: number
  patch: number
  prerelease: string[]
}

const SEMVER = /^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$/

function parseSemVer(version: string): SemVer | null {
  const m = SEMVER.exec(version.trim())
  if (!m) return null
  return {
    major: Number(m[1]),
    minor: Number(m[2]),
    patch: Number(m[3]),
    prerelease: m[4] ? m[4].split(".") : [],
  }
}

// normalizeVersion returns the version without the "v", or null when it
// is not a version ("Unreleased").
export function normalizeVersion(version: string): string | null {
  return parseSemVer(version) ? version.trim().replace(/^v/, "") : null
}

// compareVersions orders two versions by semantic-versioning precedence:
// negative when a < b. A string that is not a version sorts below every
// version.
export function compareVersions(a: string, b: string): number {
  const va = parseSemVer(a)
  const vb = parseSemVer(b)
  if (!va || !vb) return (va ? 1 : 0) - (vb ? 1 : 0)
  for (const key of ["major", "minor", "patch"] as const) {
    if (va[key] !== vb[key]) return va[key] - vb[key]
  }
  // A version without a pre-release ranks above one with it.
  if (va.prerelease.length === 0 || vb.prerelease.length === 0) {
    return vb.prerelease.length - va.prerelease.length
  }
  const n = Math.max(va.prerelease.length, vb.prerelease.length)
  for (let i = 0; i < n; i++) {
    const pa = va.prerelease[i]
    const pb = vb.prerelease[i]
    if (pa === undefined) return -1
    if (pb === undefined) return 1
    if (pa === pb) continue
    const na = /^\d+$/.test(pa)
    const nb = /^\d+$/.test(pb)
    if (na && nb) return Number(pa) - Number(pb)
    if (na) return -1
    if (nb) return 1
    return pa < pb ? -1 : 1
  }
  return 0
}

// --- The build ------------------------------------------------------------

// BuildStamp describes the version string the build was given (the same
// one the version badge reads, see lib/app-version.ts).
export type BuildStamp =
  // A release, e.g. "v0.2.2" or goreleaser's "0.2.2".
  | { kind: "release"; version: string }
  // Anything else: "dev", "edge", "pr-123", or `git describe` output such
  // as "v0.2.2-14-gabc1234-dirty". `base` is the release a describe
  // string starts from, null when the string names none.
  | { kind: "development"; label: string; base: string | null }

const GIT_DESCRIBE = /^v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)-\d+-g[0-9a-f]+(?:-dirty)?$/
const DIRTY_TAG = /^v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)-dirty$/

export function describeBuild(stamp: string): BuildStamp {
  const label = stamp.trim()
  const describe = GIT_DESCRIBE.exec(label) ?? DIRTY_TAG.exec(label)
  if (describe) return { kind: "development", label, base: describe[1] }
  const version = normalizeVersion(label)
  if (version) return { kind: "release", version }
  return { kind: "development", label, base: null }
}

// selectReleases keeps the releases the build contains, newest first: up to
// and including the release a build was cut from, or every release for a
// build that names none.
export function selectReleases(
  releases: ChangelogRelease[],
  build: BuildStamp
): ChangelogRelease[] {
  const cutoff = build.kind === "release" ? build.version : build.base
  return releases
    .filter((r) => cutoff === null || compareVersions(r.version, cutoff) <= 0)
    .sort((a, b) => compareVersions(b.version, a.version))
}

// WhatsNewMeta is what the app shell needs for the unread dot, kept apart
// from the release list so the list stays in the page's lazy chunk.
export interface WhatsNewMeta {
  build: BuildStamp
  // The newest release on the page, or null when there is none. System
  // administrators, who see every section, get the dot for it.
  latestVersion: string | null
  // The newest release with something to read for a user who is not a
  // system administrator, that is, with more than an Operators section.
  // Those users get the dot for this one, so a release that changed only
  // the deployment does not open to an empty card.
  latestUserVersion: string | null
}

// hasUserChanges reports whether a release shows anything to a user who is
// not a system administrator.
export function hasUserChanges(release: ChangelogRelease): boolean {
  return (
    release.paragraphs.length > 0 || release.sections.some((s) => s.title !== OPERATORS_SECTION)
  )
}

// whatsNewMeta builds the meta for `releases`, which `selectReleases` has
// put newest first.
export function whatsNewMeta(build: BuildStamp, releases: ChangelogRelease[]): WhatsNewMeta {
  return {
    build,
    latestVersion: releases[0]?.version ?? null,
    latestUserVersion: releases.find(hasUserChanges)?.version ?? null,
  }
}
