import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"

import {
  bulkDeleteFiles,
  bulkReclassifyFiles,
  bulkUpdateFileTags,
  checkUploadCapacity,
  deleteFile,
  getCategoryCounts,
  getFile,
  listFiles,
  trashUnlinkedFile,
  updateFile,
  uploadFile,
} from "@/features/files/api"
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

describe("features/files/api", () => {
  it("listFiles reads the FLAT FilesResponse data shape and pairs signed urls by id", async () => {
    // BE emits `data: []FileEntity` directly (not wrapped in
    // `{id,type,attributes}`); see apiserver/files.go::listFiles.
    server.use(
      http.get(apiUrl("/g/g1/files"), () =>
        HttpResponse.json({
          data: [
            { id: "f1", title: "A", category: "images", mime_type: "image/jpeg" },
            { id: "f2", title: "B", category: "documents", mime_type: "application/pdf" },
          ],
          meta: { total: 2, signed_urls: { f1: { url: "u1" } } },
        })
      )
    )
    const result = await listFiles({ category: "images", search: "x", tags: ["t1"] })
    expect(result.total).toBe(2)
    expect(result.files).toHaveLength(2)
    expect(result.files[0].file.title).toBe("A")
    expect(result.files[0].signedUrl?.url).toBe("u1")
  })

  it("listFiles forwards linked_entity_type + linked_entity_id query params when both are set", async () => {
    // Captures the request URL so a future regression in
    // listFiles' param-builder (e.g. dropping one half of the pair)
    // fails this test instead of silently returning all group files.
    let captured: URL | null = null
    server.use(
      http.get(apiUrl("/g/g1/files"), ({ request }) => {
        captured = new URL(request.url)
        return HttpResponse.json({ data: [], meta: { total: 0 } })
      })
    )
    await listFiles({
      linkedEntityType: "commodity",
      linkedEntityId: "com-1",
      perPage: 10,
    })
    expect(captured).not.toBeNull()
    expect(captured!.searchParams.get("linked_entity_type")).toBe("commodity")
    expect(captured!.searchParams.get("linked_entity_id")).toBe("com-1")
    expect(captured!.searchParams.get("limit")).toBe("10")
  })

  it("listFiles omits linked_entity_* params when only one half is provided", async () => {
    // BE rejects partial pairs with 400; the FE never builds them.
    let captured: URL | null = null
    server.use(
      http.get(apiUrl("/g/g1/files"), ({ request }) => {
        captured = new URL(request.url)
        return HttpResponse.json({ data: [], meta: { total: 0 } })
      })
    )
    await listFiles({ linkedEntityType: "commodity" })
    expect(captured!.searchParams.has("linked_entity_type")).toBe(false)
    expect(captured!.searchParams.has("linked_entity_id")).toBe(false)
  })

  it("getCategoryCounts forwards the type/search/tags filter and returns the counts payload", async () => {
    server.use(
      http.get(apiUrl("/g/g1/files/category-counts"), () =>
        HttpResponse.json({
          data: { images: 3, documents: 1, other: 0, all: 4 },
        })
      )
    )
    const counts = await getCategoryCounts({ search: "x", tags: ["a"] })
    expect(counts.all).toBe(4)
    expect(counts.images).toBe(3)
  })

  it("getFile throws when the response is missing attributes (FlatFileResponse shape)", async () => {
    server.use(
      http.get(apiUrl("/g/g1/files/f1"), () => HttpResponse.json({ id: "f1", type: "files" }))
    )
    await expect(getFile("f1")).rejects.toThrow(/missing attributes/)
  })

  it("getFile returns the inline signed URL when present", async () => {
    server.use(
      http.get(apiUrl("/g/g1/files/f1"), () =>
        HttpResponse.json({
          id: "f1",
          type: "files",
          attributes: { id: "f1", title: "X" },
          meta: { signed_urls: { f1: { url: "u" } } },
        })
      )
    )
    const out = await getFile("f1")
    expect(out.file.title).toBe("X")
    expect(out.signedUrl?.url).toBe("u")
  })

  it("updateFile sends the JSON:API envelope and returns the updated file", async () => {
    server.use(
      http.put(apiUrl("/g/g1/files/f1"), () =>
        HttpResponse.json({ id: "f1", type: "files", attributes: { id: "f1", title: "New" } })
      )
    )
    const out = await updateFile("f1", { title: "New" })
    expect(out.title).toBe("New")
    expect(out.id).toBe("f1")
  })

  it("deleteFile resolves on 204", async () => {
    server.use(http.delete(apiUrl("/g/g1/files/f1"), () => new HttpResponse(null, { status: 204 })))
    await expect(deleteFile("f1")).resolves.toBeUndefined()
  })

  it("trashUnlinkedFile trashes only a file nothing links, and keeps one it cannot read", async () => {
    const deleted: string[] = []
    const attributes: Record<string, unknown> = {
      f1: { id: "f1", links: [] },
      f2: {
        id: "f2",
        links: [{ id: "l1", file_id: "f2", entity_type: "commodity", entity_id: "c1" }],
      },
    }
    server.use(
      http.get(apiUrl("/g/g1/files/:id"), ({ params }) => {
        const id = String(params.id)
        if (id === "gone") return HttpResponse.json({ errors: [] }, { status: 404 })
        // f3 answers without attributes, so its links are unknown.
        return HttpResponse.json({ id, type: "files", attributes: attributes[id] })
      }),
      http.delete(apiUrl("/g/g1/files/:id"), ({ params }) => {
        deleted.push(String(params.id))
        return new HttpResponse(null, { status: 204 })
      })
    )
    await expect(trashUnlinkedFile("f1")).resolves.toBe(true)
    await expect(trashUnlinkedFile("f2")).resolves.toBe(false)
    await expect(trashUnlinkedFile("gone")).resolves.toBe(false)
    await expect(trashUnlinkedFile("f3")).rejects.toThrow(/missing attributes/)
    expect(deleted).toEqual(["f1"])
  })

  it("bulkDeleteFiles aggregates succeeded + failed", async () => {
    server.use(
      http.post(apiUrl("/g/g1/files/bulk-delete"), () =>
        HttpResponse.json({
          data: {
            type: "files",
            attributes: {
              succeeded: ["f1"],
              failed: [{ id: "f2", error: "nope" }],
            },
          },
        })
      )
    )
    const out = await bulkDeleteFiles(["f1", "f2"])
    expect(out.succeeded).toEqual(["f1"])
    expect(out.failed[0].id).toBe("f2")
  })

  it("bulkReclassifyFiles fans out PUTs and aggregates outcomes", async () => {
    let calls = 0
    server.use(
      http.put(apiUrl("/g/g1/files/f1"), () => {
        calls++
        return HttpResponse.json({
          id: "f1",
          type: "files",
          attributes: { id: "f1", category: "documents" },
        })
      }),
      http.put(apiUrl("/g/g1/files/f2"), () => {
        calls++
        return HttpResponse.json({ error: "boom" }, { status: 500 })
      })
    )
    const out = await bulkReclassifyFiles(["f1", "f2"], "documents")
    expect(calls).toBe(2)
    expect(out.succeeded).toEqual(["f1"])
    expect(out.failed[0].id).toBe("f2")
  })

  it("checkUploadCapacity unwraps the slot envelope into the FE shape", async () => {
    server.use(
      http.get(apiUrl("/g/g1/upload-slots/check"), () =>
        HttpResponse.json({
          data: {
            attributes: {
              operation_name: "files-upload",
              active_uploads: 0,
              max_uploads: 4,
              available_uploads: 4,
              can_start_upload: true,
            },
          },
        })
      )
    )
    const out = await checkUploadCapacity()
    expect(out.canStart).toBe(true)
    expect(out.max).toBe(4)
  })

  it("uploadFile rejects when the response is missing id or attributes", async () => {
    server.use(
      http.post(apiUrl("/g/g1/uploads/file"), () =>
        HttpResponse.json({ type: "files", attributes: {} }, { status: 201 })
      )
    )
    const f = new File(["x"], "x.txt", { type: "text/plain" })
    await expect(uploadFile(f)).rejects.toThrow(/missing id/)
  })

  it("bulkUpdateFileTags sends every request to the group the change started in", async () => {
    const ids = Array.from({ length: 150 }, (_, i) => `f${i}`)
    const sent: string[][] = []
    const elsewhere: unknown[] = []
    server.use(
      http.post(apiUrl("/g/g1/files/bulk-update"), async ({ request }) => {
        const body = (await request.json()) as { data: { attributes: { ids: string[] } } }
        sent.push(body.data.attributes.ids)
        // The user switches groups while the first request runs.
        setCurrentGroupSlug("g2")
        return HttpResponse.json({
          data: { attributes: { succeeded: body.data.attributes.ids, failed: [] } },
        })
      }),
      http.post(apiUrl("/g/g2/files/bulk-update"), async ({ request }) => {
        elsewhere.push(await request.json())
        return HttpResponse.json({ data: { attributes: { succeeded: [], failed: [] } } })
      })
    )
    const out = await bulkUpdateFileTags(ids, { add: ["receipt"], remove: [] })
    expect(sent.map((chunk) => chunk.length)).toEqual([100, 50])
    expect(elsewhere).toEqual([])
    expect(out.succeeded).toEqual(ids)
  })

  it("bulkUpdateFileTags keeps what the first requests changed when a later one fails", async () => {
    const ids = Array.from({ length: 250 }, (_, i) => `f${i}`)
    let calls = 0
    server.use(
      http.post(apiUrl("/g/g1/files/bulk-update"), async ({ request }) => {
        calls += 1
        if (calls > 1) return HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 })
        const body = (await request.json()) as { data: { attributes: { ids: string[] } } }
        return HttpResponse.json({
          data: { attributes: { succeeded: body.data.attributes.ids, failed: [] } },
        })
      })
    )
    const out = await bulkUpdateFileTags(ids, { add: ["receipt"], remove: [] })
    expect(calls).toBe(2)
    expect(out.succeeded).toEqual(ids.slice(0, 100))
    expect(out.unknown).toEqual(ids.slice(100, 200))
    expect(out.failed.map((f) => f.id)).toEqual(ids.slice(200))
  })
})
