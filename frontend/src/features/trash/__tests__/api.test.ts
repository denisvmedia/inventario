import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"

import { restoreCommodities } from "@/features/trash/api"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("token")
  setCurrentGroupSlug("g1")
})

describe("restoreCommodities", () => {
  it("keeps what the first requests restored when a later one fails", async () => {
    const ids = Array.from({ length: 250 }, (_, i) => `c${i}`)
    let calls = 0
    server.use(
      http.post(apiUrl("/g/g1/trash/commodities/restore"), async ({ request }) => {
        calls += 1
        if (calls > 1) return HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 })
        const body = (await request.json()) as { data: { attributes: { ids: string[] } } }
        return HttpResponse.json({
          data: {
            type: "commodities.bulk_result",
            attributes: { succeeded: body.data.attributes.ids, failed: [], area_removed: ["c3"] },
          },
        })
      })
    )
    const out = await restoreCommodities(ids)
    expect(calls).toBe(2)
    expect(out.succeeded).toEqual(ids.slice(0, 100))
    expect(out.areaRemoved).toEqual(["c3"])
    expect(out.unknown).toEqual(ids.slice(100, 200))
    expect(out.failed.map((f) => f.id)).toEqual(ids.slice(200))
  })
})
