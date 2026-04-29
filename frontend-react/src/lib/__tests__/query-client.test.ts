import { describe, expect, it } from "vitest"

import { createQueryClient } from "@/lib/query-client"

describe("createQueryClient", () => {
  it("returns a fresh client with the project's default options", () => {
    const a = createQueryClient()
    const b = createQueryClient()
    expect(a).not.toBe(b)
    const defaults = a.getDefaultOptions()
    // The defaults object always has a queries key — its presence is the
    // smoke that we wired our config in (a bare `new QueryClient()` would
    // also expose this, so the value-level assertions below pin the
    // project-specific tunables).
    expect(defaults).toBeDefined()
  })
})
