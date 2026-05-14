import { describe, expect, it } from "vitest"
import { Laptop, Monitor, Smartphone, TabletSmartphone } from "lucide-react"

import { parseUserAgent } from "@/features/security/ua"

describe("parseUserAgent", () => {
  it("returns Monitor + unknown shape for empty input", () => {
    const ua = parseUserAgent("")
    expect(ua.isUnknown).toBe(true)
    expect(ua.browser).toBeNull()
    expect(ua.os).toBeNull()
    expect(ua.deviceIcon).toBe(Monitor)
  })

  it("recognises Chrome on macOS as a Laptop", () => {
    const ua = parseUserAgent(
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
    )
    expect(ua.browser).toBe("Chrome")
    expect(ua.os).toBe("macOS")
    expect(ua.deviceIcon).toBe(Laptop)
    expect(ua.isUnknown).toBe(false)
  })

  it("recognises Safari on iOS as a Smartphone", () => {
    const ua = parseUserAgent(
      "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/605.1.15"
    )
    expect(ua.browser).toBe("Safari")
    expect(ua.os).toBe("iOS")
    expect(ua.deviceIcon).toBe(Smartphone)
  })

  it("recognises an iPad as a TabletSmartphone", () => {
    const ua = parseUserAgent(
      "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"
    )
    expect(ua.deviceIcon).toBe(TabletSmartphone)
  })

  it("prefers Edge over Chrome when both tokens are present", () => {
    const ua = parseUserAgent(
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0"
    )
    expect(ua.browser).toBe("Edge")
    expect(ua.os).toBe("Windows")
  })

  it("flags isUnknown=true when neither browser nor OS regex matches", () => {
    const ua = parseUserAgent("curl/8.4.0")
    expect(ua.browser).toBeNull()
    expect(ua.os).toBeNull()
    expect(ua.isUnknown).toBe(true)
  })
})
