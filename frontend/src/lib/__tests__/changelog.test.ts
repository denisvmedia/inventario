import fs from "node:fs"
import path from "node:path"
import { describe, expect, it } from "vitest"

import {
  OPERATORS_SECTION,
  compareVersions,
  describeBuild,
  hasUserChanges,
  parseChangelog,
  parseInline,
  selectReleases,
  whatsNewMeta,
  type ChangelogRelease,
} from "@/lib/changelog"

const BASE = "https://github.com/acme/app/blob/master/"

const FIXTURE = `# Changelog

All notable changes are recorded here. See [CONTRIBUTING.md](CONTRIBUTING.md).

## [Unreleased]

### Added

- Something not released yet.

## [1.1.0] - 2026-12-01

### Added

- A **Views** menu on the \`/items\` page
  ([#12](https://github.com/acme/app/pull/12)).

- Second entry after a blank line,
  continued here.

### Fixed

- A fix.

### Operators

- \`--flag\` for the server.

## [1.0.0] - 2026-11-01

First release. Notes are in
[the release notes](.github/release-notes/v1.0.0.md).

## [0.9.0-rc.1] - 2026-10-01

### Security

- A security fix.

### Removed

[Unreleased]: https://github.com/acme/app/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/acme/app/compare/v1.0.0...v1.1.0
`

function versions(releases: ChangelogRelease[]): string[] {
  return releases.map((r) => r.version)
}

describe("parseChangelog", () => {
  const releases = parseChangelog(FIXTURE, { linkBase: BASE })

  it("reads the released versions with their dates and skips Unreleased", () => {
    expect(versions(releases)).toEqual(["1.1.0", "1.0.0", "0.9.0-rc.1"])
    expect(releases.map((r) => r.date)).toEqual(["2026-12-01", "2026-11-01", "2026-10-01"])
    const all = JSON.stringify(releases)
    expect(all).not.toContain("not released yet")
  })

  it("reads the sections in file order, Operators included", () => {
    const [latest] = releases
    expect(latest.sections.map((s) => s.title)).toEqual(["Added", "Fixed", OPERATORS_SECTION])
    expect(latest.sections[2].entries).toEqual([
      [
        { type: "code", text: "--flag" },
        { type: "text", text: " for the server." },
      ],
    ])
  })

  it("joins continuation lines and keeps entries apart across blank lines", () => {
    const added = releases[0].sections[0]
    expect(added.entries).toHaveLength(2)
    expect(added.entries[1]).toEqual([
      { type: "text", text: "Second entry after a blank line, continued here." },
    ])
    expect(added.entries[0]).toEqual([
      { type: "text", text: "A " },
      { type: "strong", children: [{ type: "text", text: "Views" }] },
      { type: "text", text: " menu on the " },
      { type: "code", text: "/items" },
      { type: "text", text: " page (" },
      {
        type: "link",
        href: "https://github.com/acme/app/pull/12",
        children: [{ type: "text", text: "#12" }],
      },
      { type: "text", text: ")." },
    ])
  })

  it("keeps the text of a release without sections and resolves relative links", () => {
    const first = releases[1]
    expect(first.sections).toEqual([])
    expect(first.paragraphs).toEqual([
      [
        { type: "text", text: "First release. Notes are in " },
        {
          type: "link",
          href: `${BASE}.github/release-notes/v1.0.0.md`,
          children: [{ type: "text", text: "the release notes" }],
        },
        { type: "text", text: "." },
      ],
    ])
  })

  it("drops an empty section and stops at the link definitions", () => {
    const rc = releases[2]
    expect(rc.sections.map((s) => s.title)).toEqual(["Security"])
    expect(JSON.stringify(rc)).not.toContain("compare")
  })
})

describe("parseInline", () => {
  it("renders a link that is not http(s) as its text", () => {
    expect(parseInline("[click](javascript:void)", BASE)).toEqual([{ type: "text", text: "click" }])
    expect(parseInline("[mail](mailto:a@b.c)", BASE)).toEqual([{ type: "text", text: "mail" }])
  })

  it("keeps unmatched markers as text", () => {
    expect(parseInline("a `b and **c and [d", BASE)).toEqual([
      { type: "text", text: "a `b and **c and [d" },
    ])
    expect(parseInline("`secrets.oauth*` keys", BASE)).toEqual([
      { type: "code", text: "secrets.oauth*" },
      { type: "text", text: " keys" },
    ])
  })

  it("keeps balanced parentheses in a link destination", () => {
    expect(parseInline("See [Foo](https://e.com/wiki/Foo_(bar)) now.", BASE)).toEqual([
      { type: "text", text: "See " },
      {
        type: "link",
        href: "https://e.com/wiki/Foo_(bar)",
        children: [{ type: "text", text: "Foo" }],
      },
      { type: "text", text: " now." },
    ])
    // The usual case, a link inside parentheses, still ends at the first `)`.
    expect(parseInline("(see [#12](https://e.com/pull/12)).", BASE)).toEqual([
      { type: "text", text: "(see " },
      { type: "link", href: "https://e.com/pull/12", children: [{ type: "text", text: "#12" }] },
      { type: "text", text: ")." },
    ])
    // An unbalanced destination is not a link.
    expect(parseInline("[x](https://e.com/a_(b", BASE)).toEqual([
      { type: "text", text: "[x](https://e.com/a_(b" },
    ])
  })

  it("reads code inside a link label and bold text", () => {
    expect(parseInline("**See [`x`](https://e.com/x)**", BASE)).toEqual([
      {
        type: "strong",
        children: [
          { type: "text", text: "See " },
          { type: "link", href: "https://e.com/x", children: [{ type: "code", text: "x" }] },
        ],
      },
    ])
  })
})

describe("compareVersions", () => {
  it("orders by semantic-versioning precedence", () => {
    const sorted = [
      "1.0.0",
      "0.10.0",
      "0.2.2",
      "1.0.0-rc.1",
      "1.0.0-rc.10",
      "1.0.0-beta",
      "0.2.10",
    ].sort(compareVersions)
    expect(sorted).toEqual([
      "0.2.2",
      "0.2.10",
      "0.10.0",
      "1.0.0-beta",
      "1.0.0-rc.1",
      "1.0.0-rc.10",
      "1.0.0",
    ])
  })

  it("accepts a leading v and ranks non-versions lowest", () => {
    expect(compareVersions("v0.2.2", "0.2.2")).toBe(0)
    expect(compareVersions("edge", "0.0.1")).toBeLessThan(0)
  })
})

describe("describeBuild", () => {
  it.each([
    ["v0.2.2", { kind: "release", version: "0.2.2" }],
    ["0.2.2", { kind: "release", version: "0.2.2" }],
    ["v1.0.0-rc.1", { kind: "release", version: "1.0.0-rc.1" }],
    ["dev", { kind: "development", label: "dev", base: null }],
    ["edge", { kind: "development", label: "edge", base: null }],
    ["pr-123", { kind: "development", label: "pr-123", base: null }],
    ["abc1234", { kind: "development", label: "abc1234", base: null }],
    ["v0.2.2-14-gabc1234", { kind: "development", label: "v0.2.2-14-gabc1234", base: "0.2.2" }],
    [
      "v0.2.2-14-gabc1234-dirty",
      { kind: "development", label: "v0.2.2-14-gabc1234-dirty", base: "0.2.2" },
    ],
    ["v0.2.2-dirty", { kind: "development", label: "v0.2.2-dirty", base: "0.2.2" }],
  ])("%s", (stamp, expected) => {
    expect(describeBuild(stamp)).toEqual(expected)
  })
})

describe("selectReleases", () => {
  const releases = parseChangelog(FIXTURE, { linkBase: BASE })

  it("keeps the releases up to and including the build's version, newest first", () => {
    expect(versions(selectReleases(releases, describeBuild("v1.0.0")))).toEqual([
      "1.0.0",
      "0.9.0-rc.1",
    ])
    expect(versions(selectReleases(releases, describeBuild("v1.1.0")))).toEqual([
      "1.1.0",
      "1.0.0",
      "0.9.0-rc.1",
    ])
  })

  it("cuts a pre-release build below its release", () => {
    expect(versions(selectReleases(releases, describeBuild("v1.1.0-rc.1")))).toEqual([
      "1.0.0",
      "0.9.0-rc.1",
    ])
  })

  it("cuts a describe build at the release it starts from", () => {
    expect(versions(selectReleases(releases, describeBuild("v1.0.0-3-gabc1234")))).toEqual([
      "1.0.0",
      "0.9.0-rc.1",
    ])
  })

  it("keeps every release for a build without a release version", () => {
    expect(versions(selectReleases(releases, describeBuild("edge")))).toEqual([
      "1.1.0",
      "1.0.0",
      "0.9.0-rc.1",
    ])
  })
})

describe("whatsNewMeta", () => {
  const releases = parseChangelog(
    `## [1.2.0] - 2026-12-03

### Operators

- A chart fix.

## [1.1.0] - 2026-12-02

### Fixed

- A fix.

## [1.0.0] - 2026-12-01

First release.
`,
    { linkBase: BASE }
  )

  it("names the newest release and the newest one with more than Operators", () => {
    expect(releases.map(hasUserChanges)).toEqual([false, true, true])
    expect(whatsNewMeta(describeBuild("v1.2.0"), releases)).toEqual({
      build: { kind: "release", version: "1.2.0" },
      latestVersion: "1.2.0",
      latestUserVersion: "1.1.0",
    })
  })

  it("counts a release whose only text is a paragraph", () => {
    expect(
      whatsNewMeta(describeBuild("v1.0.0"), selectReleases(releases, describeBuild("v1.0.0")))
    ).toEqual({
      build: { kind: "release", version: "1.0.0" },
      latestVersion: "1.0.0",
      latestUserVersion: "1.0.0",
    })
  })

  it("is empty without releases", () => {
    expect(whatsNewMeta(describeBuild("dev"), [])).toEqual({
      build: { kind: "development", label: "dev", base: null },
      latestVersion: null,
      latestUserVersion: null,
    })
  })
})

// The real file has to stay readable by the parser: a release whose heading
// the parser misses would silently vanish from the page, and a section
// heading it does not know would show under a heading nobody translated.
// The checks read the lines themselves rather than the parser's output, so
// they cover Unreleased too, which the parser skips. CI runs this file on
// any change to CHANGELOG.md (the `changelog` filter), not only on frontend
// changes.
describe("CHANGELOG.md", () => {
  const markdown = fs.readFileSync(path.resolve(__dirname, "../../../../CHANGELOG.md"), "utf8")
  const lines = markdown.split(/\r?\n/)
  const releases = parseChangelog(markdown, { linkBase: BASE })

  // `[Unreleased]`, or `[x.y.z] - YYYY-MM-DD` with an optional pre-release
  // and no `+build` metadata, which the last-seen setting refuses.
  const HEADING =
    /^## \[(Unreleased|\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\](?: - (\d{4}-\d{2}-\d{2}))?$/
  const KNOWN_SECTIONS = [
    "Added",
    "Changed",
    "Deprecated",
    "Removed",
    "Fixed",
    "Security",
    OPERATORS_SECTION,
  ]

  // Every `## ` heading with the `### ` headings under it.
  const blocks: Array<{ heading: string; sections: string[] }> = []
  const orphanSections: string[] = []
  for (const line of lines) {
    if (line.startsWith("## ")) {
      blocks.push({ heading: line, sections: [] })
    } else if (line.startsWith("### ")) {
      const block = blocks[blocks.length - 1]
      if (block) block.sections.push(line.slice(4).trim())
      else orphanSections.push(line)
    }
  }
  const named = blocks.map((b) => ({ ...b, match: HEADING.exec(b.heading) }))
  const versionBlocks = named.filter((b) => b.match && b.match[1] !== "Unreleased")

  it("has release headings of the form [Unreleased] or [x.y.z] - YYYY-MM-DD", () => {
    expect(blocks.length).toBeGreaterThan(0)
    for (const b of named) {
      expect(b.match, `not a release heading: ${b.heading}`).not.toBeNull()
      if (b.match && b.match[1] !== "Unreleased") {
        expect(b.match[2], `no date: ${b.heading}`).toBeDefined()
        expect(Number.isNaN(Date.parse(b.match[2] ?? "")), `bad date: ${b.heading}`).toBe(false)
      } else if (b.match) {
        expect(b.match[2], `Unreleased has a date: ${b.heading}`).toBeUndefined()
      }
    }
    expect(orphanSections, "section headings before the first release").toEqual([])
  })

  it("names each version once, newest first", () => {
    const names = named.map((b) => b.match?.[1] ?? b.heading)
    expect(new Set(names).size, `a release heading repeats: ${names.join(", ")}`).toBe(names.length)
    const versionNames = versionBlocks.map((b) => b.match?.[1] ?? "")
    expect([...versionNames].sort((a, b) => compareVersions(b, a))).toEqual(versionNames)
  })

  it("yields one release per version heading", () => {
    expect(versions(releases)).toEqual(versionBlocks.map((b) => b.match?.[1]))
    expect(versions(selectReleases(releases, describeBuild("dev")))).toEqual(versions(releases))
  })

  it("uses only the Keep a Changelog headings and Operators, each once, Operators last", () => {
    for (const b of blocks) {
      for (const t of b.sections) expect(KNOWN_SECTIONS, `${b.heading}: ### ${t}`).toContain(t)
      expect(new Set(b.sections).size, `${b.heading}: a section heading repeats`).toBe(
        b.sections.length
      )
      const ops = b.sections.indexOf(OPERATORS_SECTION)
      if (ops >= 0) {
        expect(ops, `${b.heading}: Operators is not the last section`).toBe(b.sections.length - 1)
      }
    }
  })
})
