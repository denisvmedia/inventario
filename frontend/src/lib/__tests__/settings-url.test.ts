import { describe, expect, it } from "vitest"

import { SETTINGS_HELP_URL } from "@/lib/settings-url"

describe("SETTINGS_HELP_URL", () => {
  // The sidebar's Help row and the /help redirect both use this. What the URL
  // has to carry is the `section` parameter SettingsPage reads — a target that
  // lost it would land on Account, which is the failure worth catching, and
  // neither call site can drift from the other while they share this.
  it("points at the settings page with the help section selected", () => {
    const url = new URL(SETTINGS_HELP_URL, "https://example.test")
    expect(url.pathname).toBe("/settings")
    expect(url.searchParams.get("section")).toBe("help")
  })
})
