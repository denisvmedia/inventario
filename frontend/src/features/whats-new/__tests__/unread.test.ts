import { describe, expect, it } from "vitest"

import { hasUnreadRelease } from "@/features/whats-new/unread"

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
