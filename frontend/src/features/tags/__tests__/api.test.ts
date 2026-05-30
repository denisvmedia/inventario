import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"

import {
  autocompleteTags,
  createTag,
  deleteTag,
  getTag,
  getTagStats,
  listTags,
  updateTag,
} from "@/features/tags/api"
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

describe("features/tags/api", () => {
  it("listTags flattens TagListItem rows and exposes meta.usage as `usage`", async () => {
    // BE emits `data: []TagListItem` — each row is a flattened Tag plus
    // an optional inline `meta.usage` block when ?include=usage is set.
    // See go/jsonapi/tags.go::TagListItem and apiserver/tags.go::listTags.
    server.use(
      http.get(apiUrl("/g/g1/tags"), () =>
        HttpResponse.json({
          data: [
            {
              id: "t1",
              slug: "kitchen",
              label: "Kitchen",
              color: "amber",
              meta: { usage: { commodities: 3, files: 1 } },
            },
            {
              id: "t2",
              slug: "garden",
              label: "Garden",
              color: "green",
            },
          ],
          meta: { tags: 2, total: 2 },
        })
      )
    )

    const result = await listTags({ kind: "commodity", includeUsage: true })
    expect(result.total).toBe(2)
    expect(result.tags).toHaveLength(2)
    expect(result.tags[0].tag.slug).toBe("kitchen")
    expect(result.tags[0].tag).not.toHaveProperty("meta")
    expect(result.tags[0].usage).toEqual({ commodities: 3, files: 1 })
    expect(result.tags[1].usage).toBeUndefined()
  })

  it("listTags wires include/sort/order/q query params through to the BE", async () => {
    let captured: URL | null = null
    server.use(
      http.get(apiUrl("/g/g1/tags"), ({ request }) => {
        captured = new URL(request.url)
        return HttpResponse.json({ data: [], meta: { tags: 0, total: 0 } })
      })
    )
    await listTags({ kind: "file", search: "k", sort: "usage", order: "desc", includeUsage: true })
    expect(captured).not.toBeNull()
    expect(captured!.searchParams.get("kind")).toBe("file")
    expect(captured!.searchParams.get("q")).toBe("k")
    expect(captured!.searchParams.get("sort")).toBe("usage")
    expect(captured!.searchParams.get("order")).toBe("desc")
    expect(captured!.searchParams.get("include")).toBe("usage")
  })

  it("getTagStats reads the flat envelope from /tags/stats", async () => {
    server.use(
      http.get(apiUrl("/g/g1/tags/stats"), () =>
        HttpResponse.json({
          data: {
            tags_total: 12,
            items_tagged: 50,
            items_untagged: 7,
            files_tagged: 30,
            files_untagged: 5,
          },
        })
      )
    )
    const stats = await getTagStats()
    expect(stats.tags_total).toBe(12)
    expect(stats.items_untagged).toBe(7)
    expect(stats.files_tagged).toBe(30)
  })

  it("getTag returns flat attributes + usage from the JSON:API detail envelope", async () => {
    server.use(
      http.get(apiUrl("/g/g1/tags/t1"), () =>
        HttpResponse.json({
          id: "t1",
          type: "tags",
          attributes: { id: "t1", slug: "kitchen", label: "Kitchen", color: "amber" },
          meta: { usage: { commodities: 2, files: 0 } },
        })
      )
    )
    const result = await getTag("t1")
    expect(result.tag.slug).toBe("kitchen")
    expect(result.tag.id).toBe("t1")
    expect(result.usage?.commodities).toBe(2)
  })

  it("createTag posts a JSON:API envelope and returns the created tag", async () => {
    server.use(
      http.post(apiUrl("/g/g1/tags"), () =>
        HttpResponse.json(
          {
            id: "new",
            type: "tags",
            attributes: { id: "new", slug: "kitchen", label: "Kitchen", color: "amber" },
          },
          { status: 201 }
        )
      )
    )
    const created = await createTag({
      kind: "commodity",
      slug: "kitchen",
      label: "Kitchen",
      color: "amber",
    })
    expect(created.id).toBe("new")
    expect(created.slug).toBe("kitchen")
  })

  it("updateTag PATCHes via JSON:API and returns the updated tag", async () => {
    server.use(
      http.patch(apiUrl("/g/g1/tags/t1"), () =>
        HttpResponse.json({
          id: "t1",
          type: "tags",
          attributes: { id: "t1", slug: "kitchen-2", label: "Kitchen", color: "blue" },
        })
      )
    )
    const updated = await updateTag("t1", { slug: "kitchen-2", color: "blue" })
    expect(updated.slug).toBe("kitchen-2")
    expect(updated.color).toBe("blue")
  })

  it("deleteTag appends ?force=true when force is requested", async () => {
    let capturedForce: string | null = null
    server.use(
      http.delete(apiUrl("/g/g1/tags/t1"), ({ request }) => {
        capturedForce = new URL(request.url).searchParams.get("force")
        return new HttpResponse(null, { status: 204 })
      })
    )
    await deleteTag("t1", true)
    expect(capturedForce).toBe("true")
  })

  it("deleteTag omits force when not requested", async () => {
    let capturedForce: string | null = "untouched"
    server.use(
      http.delete(apiUrl("/g/g1/tags/t1"), ({ request }) => {
        capturedForce = new URL(request.url).searchParams.get("force")
        return new HttpResponse(null, { status: 204 })
      })
    )
    await deleteTag("t1")
    expect(capturedForce).toBeNull()
  })

  it("autocompleteTags reads the flat data envelope", async () => {
    server.use(
      http.get(apiUrl("/g/g1/tags/autocomplete"), () =>
        HttpResponse.json({
          data: [
            { id: "t1", slug: "kitchen", label: "Kitchen", color: "amber" },
            { id: "t2", slug: "kid-room", label: "Kid Room", color: "green" },
          ],
        })
      )
    )
    const result = await autocompleteTags("ki", 10, { kind: "commodity" })
    expect(result).toHaveLength(2)
    expect(result[0].slug).toBe("kitchen")
  })
})
