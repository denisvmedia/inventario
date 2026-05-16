import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"

import { setCommodityCover } from "@/features/commodities/api"
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

describe("setCommodityCover", () => {
  it("sends PATCH /commodities/{id}/cover with the file_id payload and folds meta.cover into the result", async () => {
    let captured: { id: string; body: unknown } | null = null
    server.use(
      http.patch(apiUrl(`/g/${SLUG}/commodities/c1/cover`), async ({ request }) => {
        captured = { id: "c1", body: await request.json() }
        return HttpResponse.json({
          data: {
            id: "c1",
            type: "commodities",
            attributes: { id: "c1", name: "Macbook", type: "electronics" },
          },
          meta: {
            cover: {
              file_id: "f-explicit",
              source: "explicit",
              thumbnails: { small: "https://example.test/small.jpg" },
            },
          },
        })
      })
    )
    const result = await setCommodityCover("c1", "f-explicit")
    expect(captured).not.toBeNull()
    expect(captured!.body).toEqual({
      data: { type: "commodity_cover", attributes: { file_id: "f-explicit" } },
    })
    expect(result.commodity.id).toBe("c1")
    expect(result.commodity.cover).toEqual({
      fileId: "f-explicit",
      source: "explicit",
      thumbnails: { small: "https://example.test/small.jpg" },
    })
  })

  it("clears the override by sending file_id: null", async () => {
    let body: unknown = null
    server.use(
      http.patch(apiUrl(`/g/${SLUG}/commodities/c1/cover`), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({
          data: {
            id: "c1",
            type: "commodities",
            attributes: { id: "c1", name: "Macbook" },
          },
          meta: {
            cover: {
              file_id: "f-fallback",
              source: "first_photo",
              thumbnails: { small: "https://example.test/small.jpg" },
            },
          },
        })
      })
    )
    const result = await setCommodityCover("c1", null)
    expect(body).toEqual({
      data: { type: "commodity_cover", attributes: { file_id: null } },
    })
    // After clear, the response surfaces the fallback first_photo cover —
    // the FE renders it the same way as any other cover.
    expect(result.commodity.cover?.source).toBe("first_photo")
  })

  it("treats a response with thumbnail-less cover as no cover (preserves icon fallback)", async () => {
    server.use(
      http.patch(apiUrl(`/g/${SLUG}/commodities/c1/cover`), () =>
        HttpResponse.json({
          data: {
            id: "c1",
            type: "commodities",
            attributes: { id: "c1", name: "Macbook" },
          },
          meta: {
            cover: {
              file_id: "f-explicit",
              source: "explicit",
              thumbnails: {},
            },
          },
        })
      )
    )
    const result = await setCommodityCover("c1", "f-explicit")
    // No usable thumbnail → normalizeCover returns undefined.
    expect(result.commodity.cover).toBeUndefined()
  })
})
