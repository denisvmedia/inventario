import { afterEach, describe, expect, it, vi } from "vitest"

import { appVersion, shortAppVersion } from "@/lib/app-version"

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("appVersion", () => {
  it("is the version the build injected", () => {
    vi.stubGlobal("__APP_VERSION__", "v0.2.2")
    expect(appVersion()).toBe("v0.2.2")
  })

  it("is dev when the build injected none", () => {
    expect(appVersion()).toBe("dev")
  })
})

describe("shortAppVersion", () => {
  it.each([
    ["v0.2.2", "0.2"],
    ["0.2.2", "0.2"],
    ["v1.10.3", "1.10"],
    ["1.2.0-beta.1", "1.2"],
    ["v0.2.2-5-gabc1234-dirty", "0.2"],
    ["v2.0.0+build.7", "2.0"],
    ["1.2", "1.2"],
  ])("trims %s to %s", (version, short) => {
    expect(shortAppVersion(version)).toBe(short)
  })

  it.each(["dev", "edge", "pr-2790", "SNAPSHOT-abc1234", "abc1234", "v1", ""])(
    "has no release number in %j",
    (version) => {
      expect(shortAppVersion(version)).toBeNull()
    }
  )

  it("reads the injected version by default", () => {
    vi.stubGlobal("__APP_VERSION__", "v0.3.1")
    expect(shortAppVersion()).toBe("0.3")
  })
})
