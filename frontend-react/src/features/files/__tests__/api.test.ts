import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"

import {
  bulkDeleteFiles,
  bulkReclassifyFiles,
  checkUploadCapacity,
  deleteFile,
  getCategoryCounts,
  getFile,
  listFiles,
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
  it("listFiles unwraps the JSON:API envelope and returns rows + total", async () => {
    server.use(
      http.get(apiUrl("/g/g1/files"), () =>
        HttpResponse.json({
          data: [
            { id: "f1", type: "files", attributes: { id: "f1", title: "A" } },
            { id: "f2", type: "files", attributes: { id: "f2", title: "B" } },
          ],
          meta: { total: 2, signed_urls: { f1: { url: "u1" } } },
        })
      )
    )
    const result = await listFiles({ category: "photos", search: "x", tags: ["t1"] })
    expect(result.total).toBe(2)
    expect(result.files).toHaveLength(2)
    expect(result.files[0].file.title).toBe("A")
    expect(result.files[0].signedUrl?.url).toBe("u1")
  })

  it("getCategoryCounts forwards the type/search/tags filter and returns the counts payload", async () => {
    server.use(
      http.get(apiUrl("/g/g1/files/category-counts"), () =>
        HttpResponse.json({
          data: { photos: 3, invoices: 0, documents: 1, other: 0, all: 4 },
        })
      )
    )
    const counts = await getCategoryCounts({ search: "x", tags: ["a"] })
    expect(counts.all).toBe(4)
    expect(counts.photos).toBe(3)
  })

  it("getFile throws when the response is missing data.attributes", async () => {
    server.use(
      http.get(apiUrl("/g/g1/files/f1"), () =>
        HttpResponse.json({ data: { id: "f1", type: "files" } })
      )
    )
    await expect(getFile("f1")).rejects.toThrow(/missing data\.attributes/)
  })

  it("getFile returns the inline signed URL when present", async () => {
    server.use(
      http.get(apiUrl("/g/g1/files/f1"), () =>
        HttpResponse.json({
          data: {
            id: "f1",
            type: "files",
            attributes: { id: "f1", title: "X" },
            meta: { signed_urls: { f1: { url: "u" } } },
          },
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
        HttpResponse.json({
          data: { id: "f1", type: "files", attributes: { id: "f1", title: "New" } },
        })
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
          data: { id: "f1", type: "files", attributes: { id: "f1", category: "documents" } },
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

  it("uploadFile rejects when the response is missing data.id or data.attributes", async () => {
    server.use(
      http.post(apiUrl("/g/g1/uploads/file"), () =>
        HttpResponse.json({ data: { type: "files", attributes: {} } }, { status: 201 })
      )
    )
    const f = new File(["x"], "x.txt", { type: "text/plain" })
    await expect(uploadFile(f)).rejects.toThrow(/missing data\.id/)
  })
})
