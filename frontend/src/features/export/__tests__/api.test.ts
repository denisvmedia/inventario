import { http, HttpResponse } from "msw"
import { beforeEach, describe, expect, it } from "vitest"

import {
  createExport,
  createRestore,
  deleteExport,
  exportDownloadPath,
  getExport,
  getRestore,
  importBackup,
  isExportTerminal,
  isRestoreTerminal,
  listExports,
  listRestores,
  uploadRestoreFile,
} from "@/features/export/api"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl } from "@/test/handlers"
import { server } from "@/test/server"

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("token")
  setCurrentGroupSlug("g1")
})

describe("features/export/api", () => {
  it("listExports flattens the JSON:API envelope and reads meta.exports", async () => {
    server.use(
      http.get(apiUrl("/g/g1/exports"), () =>
        HttpResponse.json({
          data: [
            {
              id: "e1",
              type: "exports",
              attributes: { type: "full_database", status: "completed" },
            },
            {
              id: "e2",
              type: "exports",
              attributes: { type: "selected_items", status: "in_progress" },
            },
          ],
          meta: { exports: 2 },
        })
      )
    )
    const { exports, total } = await listExports()
    expect(total).toBe(2)
    expect(exports).toHaveLength(2)
    expect(exports[0]).toMatchObject({ id: "e1", type: "full_database", status: "completed" })
  })

  it("listExports forwards include_deleted to the BE", async () => {
    let captured: URL | null = null
    server.use(
      http.get(apiUrl("/g/g1/exports"), ({ request }) => {
        captured = new URL(request.url)
        return HttpResponse.json({ data: [], meta: { exports: 0 } })
      })
    )
    await listExports({ includeDeleted: true })
    expect(captured!.searchParams.get("include_deleted")).toBe("true")
  })

  it("getExport rehydrates the resource on /exports/:id", async () => {
    server.use(
      http.get(apiUrl("/g/g1/exports/e1"), () =>
        HttpResponse.json({
          data: {
            id: "e1",
            type: "exports",
            attributes: { type: "full_database", status: "completed", file_count: 5 },
          },
        })
      )
    )
    const exp = await getExport("e1")
    expect(exp.id).toBe("e1")
    expect(exp.file_count).toBe(5)
  })

  it("createExport posts the JSON:API request and returns the created row", async () => {
    let body: unknown = null
    server.use(
      http.post(apiUrl("/g/g1/exports"), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json(
          {
            data: {
              id: "e1",
              type: "exports",
              attributes: { type: "full_database", status: "pending" },
            },
          },
          { status: 201 }
        )
      })
    )
    const exp = await createExport({
      type: "full_database",
      description: "weekly",
      include_file_data: true,
    })
    expect(exp.id).toBe("e1")
    expect(body).toEqual({
      data: {
        type: "exports",
        attributes: { type: "full_database", description: "weekly", include_file_data: true },
      },
    })
  })

  it("deleteExport hits /exports/:id and surfaces 204", async () => {
    server.use(
      http.delete(apiUrl("/g/g1/exports/e1"), () => new HttpResponse(null, { status: 204 }))
    )
    await expect(deleteExport("e1")).resolves.toBeUndefined()
  })

  it("uploadRestoreFile sends a multipart body and returns the staged path", async () => {
    let receivedContentType = ""
    server.use(
      http.post(apiUrl("/g/g1/uploads/restores"), ({ request }) => {
        receivedContentType = request.headers.get("content-type") ?? ""
        return HttpResponse.json({
          id: "uploads",
          type: "uploads",
          attributes: { fileNames: ["restores/2026/05/abc.xml"], type: "restores" },
        })
      })
    )
    const f = new File(["<export/>"], "backup.xml", { type: "application/xml" })
    const result = await uploadRestoreFile(f)
    // FormData triggers the browser-built `multipart/form-data; boundary=...`
    // header (the http wrapper deliberately doesn't override it). Asserting
    // that prefix is enough to prove the request went out as multipart.
    expect(receivedContentType).toMatch(/^multipart\/form-data/)
    expect(result.sourceFilePath).toBe("restores/2026/05/abc.xml")
  })

  it("uploadRestoreFile throws when the BE returns no fileNames", async () => {
    server.use(
      http.post(apiUrl("/g/g1/uploads/restores"), () =>
        HttpResponse.json({ id: "u", type: "uploads", attributes: { fileNames: [] } })
      )
    )
    const f = new File(["x"], "x.xml")
    await expect(uploadRestoreFile(f)).rejects.toThrow(/fileNames/)
  })

  it("importBackup posts the description+source_file_path and returns the imported export", async () => {
    let body: unknown = null
    server.use(
      http.post(apiUrl("/g/g1/exports/import"), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json(
          {
            data: {
              id: "imp-1",
              type: "exports",
              attributes: { type: "imported", status: "completed", imported: true },
            },
          },
          { status: 201 }
        )
      })
    )
    const result = await importBackup({
      description: "from disk",
      source_file_path: "restores/2026/05/abc.xml",
    })
    expect(result.id).toBe("imp-1")
    expect(result.imported).toBe(true)
    expect(body).toEqual({
      data: {
        type: "exports",
        attributes: { description: "from disk", source_file_path: "restores/2026/05/abc.xml" },
      },
    })
  })

  it("listRestores rehydrates the per-export restore list", async () => {
    server.use(
      http.get(apiUrl("/g/g1/exports/e1/restores"), () =>
        HttpResponse.json({
          data: [
            {
              id: "r1",
              type: "restores",
              attributes: { status: "completed", description: "first" },
            },
          ],
        })
      )
    )
    const { restores } = await listRestores("e1")
    expect(restores).toHaveLength(1)
    expect(restores[0].id).toBe("r1")
  })

  it("createRestore wraps the options block under attributes", async () => {
    let body: unknown = null
    server.use(
      http.post(apiUrl("/g/g1/exports/e1/restores"), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json(
          {
            data: {
              id: "r1",
              type: "restores",
              attributes: { status: "pending", description: "" },
            },
          },
          { status: 201 }
        )
      })
    )
    const result = await createRestore("e1", {
      description: "",
      options: { strategy: "merge_add", include_file_data: true, dry_run: true },
    })
    expect(result.id).toBe("r1")
    expect(body).toEqual({
      data: {
        type: "restores",
        attributes: {
          description: "",
          options: { strategy: "merge_add", include_file_data: true, dry_run: true },
        },
      },
    })
  })

  it("getRestore reads the per-export restore detail", async () => {
    server.use(
      http.get(apiUrl("/g/g1/exports/e1/restores/r1"), () =>
        HttpResponse.json({
          data: {
            id: "r1",
            type: "restores",
            attributes: { status: "running" },
          },
        })
      )
    )
    const result = await getRestore("e1", "r1")
    expect(result.status).toBe("running")
  })

  it("isExportTerminal / isRestoreTerminal flag completed and failed only", () => {
    expect(isExportTerminal("completed")).toBe(true)
    expect(isExportTerminal("failed")).toBe(true)
    expect(isExportTerminal("pending")).toBe(false)
    expect(isExportTerminal("in_progress")).toBe(false)
    expect(isExportTerminal(undefined)).toBe(false)

    expect(isRestoreTerminal("completed")).toBe(true)
    expect(isRestoreTerminal("failed")).toBe(true)
    expect(isRestoreTerminal("running")).toBe(false)
    expect(isRestoreTerminal("pending")).toBe(false)
  })

  it("exportDownloadPath builds the absolute /api/v1/g/<slug>/ download URL", () => {
    expect(exportDownloadPath("e1", "household", null)).toBe(
      "/api/v1/g/household/exports/e1/download"
    )
  })

  it("exportDownloadPath appends the access token as ?token= for <a href> downloads", () => {
    // The BE accepts JWT via Authorization or ?token=; <a href> doesn't
    // send Authorization, so the wrapper has to attach the token.
    expect(exportDownloadPath("e1", "household", "tok-abc")).toBe(
      "/api/v1/g/household/exports/e1/download?token=tok-abc"
    )
  })
})
