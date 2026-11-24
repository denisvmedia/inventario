import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"

import { search, type CommodityAttrs } from "@/features/search/api"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"

const SLUG = "household"

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setCurrentGroupSlug(SLUG)
  setAccessToken("good-token")
})

describe("search", () => {
  it("wraps the flat models the backend returns into id/type/attributes", async () => {
    server.use(
      http.get(apiUrl(`/g/${SLUG}/search`), () =>
        HttpResponse.json({
          data: [{ id: "c1", name: "Drill", short_name: "drill", asset_number: 42 }],
          meta: { entity_type: "commodities", total: 1 },
        })
      )
    )
    const page = await search<CommodityAttrs>("A-0042", { type: "commodities" })
    expect(page.total).toBe(1)
    expect(page.results).toEqual([
      {
        id: "c1",
        type: "commodities",
        attributes: { id: "c1", name: "Drill", short_name: "drill", asset_number: 42 },
      },
    ])
  })

  it("passes rows that already carry attributes through unchanged", async () => {
    const row = { id: "c1", type: "commodities", attributes: { id: "c1", name: "Drill" } }
    server.use(http.get(apiUrl(`/g/${SLUG}/search`), () => HttpResponse.json({ data: [row] })))
    const page = await search<CommodityAttrs>("drill", { type: "commodities" })
    expect(page.results).toEqual([row])
  })
})
