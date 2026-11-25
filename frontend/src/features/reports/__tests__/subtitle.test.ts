import { describe, expect, it } from "vitest"

import { reportSubtitle } from "../subtitle"

describe("reportSubtitle", () => {
  it.each([
    [{ model: "Classic Pro", short_name: "Gaggia", name: "Espresso Machine" }, "Classic Pro"],
    [{ model: "", short_name: "Gaggia", name: "Espresso Machine" }, "Gaggia"],
    [{ model: "  ", short_name: "Toaster", name: "Toaster" }, ""],
    [{ name: "Toaster" }, ""],
  ])("renders %j as %j", (item, want) => {
    expect(reportSubtitle(item)).toBe(want)
  })
})
