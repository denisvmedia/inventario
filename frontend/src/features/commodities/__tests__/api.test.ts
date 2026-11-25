import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"

import {
  bulkUpdateCommodities,
  setCommodityCover,
  tagCountsOptions,
} from "@/features/commodities/api"
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

describe("tagCountsOptions", () => {
  it("sets the tag filter aside, unreadable tag values included", () => {
    expect(
      tagCountsOptions({
        page: 2,
        perPage: 24,
        search: "drill",
        tags: ["red"],
        tagMode: "any",
        noTags: true,
        sort: "count",
        unreadable: { tag_mode: "some", no_tags: "x", lent_out: "maybe" },
      })
    ).toMatchObject({
      page: undefined,
      perPage: undefined,
      search: "drill",
      tags: undefined,
      tagMode: undefined,
      noTags: undefined,
      sort: undefined,
      unreadable: { lent_out: "maybe" },
    })
    expect(tagCountsOptions({ unreadable: { tag_mode: "some" } }).unreadable).toBeUndefined()
  })
})

describe("bulkUpdateCommodities", () => {
  it("sends a query with the count and digest the user saw", async () => {
    let body: unknown = null
    server.use(
      http.post(apiUrl(`/g/${SLUG}/commodities/bulk-update`), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({ data: { attributes: { succeeded: ["c1"], failed: [] } } })
      })
    )
    await bulkUpdateCommodities(
      { query: "tags=garage", expectedCount: 1, expectedDigest: "d1" },
      { type: "electronics" }
    )
    expect(body).toEqual({
      data: {
        type: "commodities",
        attributes: {
          query: "tags=garage",
          expected_count: 1,
          expected_digest: "d1",
          patch: { type: "electronics" },
        },
      },
    })
  })

  it("keeps what the first requests changed when a later one fails", async () => {
    const ids = Array.from({ length: 150 }, (_, i) => `c${i}`)
    let calls = 0
    server.use(
      http.post(apiUrl(`/g/${SLUG}/commodities/bulk-update`), async ({ request }) => {
        calls += 1
        if (calls > 1) return HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 })
        const body = (await request.json()) as { data: { attributes: { ids: string[] } } }
        return HttpResponse.json({
          data: {
            attributes: {
              succeeded: body.data.attributes.ids.slice(1),
              failed: [
                {
                  id: "c0",
                  name: "First",
                  code: "invalid",
                  error: "short_name: cannot be blank.",
                  field: "short_name",
                  reason: "validation_is_empty",
                },
              ],
            },
          },
        })
      })
    )
    const out = await bulkUpdateCommodities({ ids }, { type: "electronics" })
    expect(calls).toBe(2)
    expect(out.succeeded).toEqual(ids.slice(1, 100))
    expect(out.failed[0]).toEqual({
      id: "c0",
      name: "First",
      code: "invalid",
      error: "short_name: cannot be blank.",
      field: "short_name",
      reason: "validation_is_empty",
    })
    expect(out.failed.slice(1)).toEqual(
      ids.slice(100).map((id) => ({ id, name: "", code: "not_sent", error: "" }))
    )
  })
})
