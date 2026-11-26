import { describe, expect, it } from "vitest"

import { announcedRelease, hasUnreadRelease } from "@/features/whats-new/unread"
import type { WhatsNewMeta } from "@/lib/changelog"

describe("hasUnreadRelease", () => {
  it.each([
    // [build's newest release, last seen, dot]
    ["0.3.0", undefined, true],
    ["0.3.0", null, true],
    ["0.3.0", "", true],
    ["0.3.0", "0.2.2", true],
    ["0.3.0", "0.3.0", false],
    ["0.3.0", "0.10.0", false],
    // A rollback to an older server leaves the newer value alone.
    ["0.2.2", "0.3.0", false],
    ["1.0.0", "1.0.0-rc.2", true],
    // A stored value that is not a version counts as never opened.
    ["0.3.0", "garbage", true],
    // A build without any release has nothing to announce.
    [null, undefined, false],
    [null, "0.2.2", false],
  ] as const)("latest %s, last seen %s -> %s", (latest, lastSeen, dot) => {
    expect(hasUnreadRelease(latest, lastSeen)).toBe(dot)
  })
})

describe("announcedRelease", () => {
  // 0.2.2 changed only the deployment.
  const meta: WhatsNewMeta = {
    build: { kind: "release", version: "0.2.2" },
    latestVersion: "0.2.2",
    latestUserVersion: "0.2.1",
  }

  it("is the newest release for a system administrator", () => {
    expect(announcedRelease(meta, true)).toBe("0.2.2")
    expect(hasUnreadRelease(announcedRelease(meta, true), "0.2.1")).toBe(true)
  })

  it("skips a release with only Operators entries for anyone else", () => {
    expect(announcedRelease(meta, false)).toBe("0.2.1")
    expect(hasUnreadRelease(announcedRelease(meta, false), "0.2.1")).toBe(false)
    expect(hasUnreadRelease(announcedRelease(meta, false), "0.2.0")).toBe(true)
  })
})
