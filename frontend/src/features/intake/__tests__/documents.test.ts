import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"

import { listIntakeDocuments, safeSourceUrl, toDocumentSource } from "@/features/intake/documents"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl, intakeHandlers } from "@/test/handlers"
import { server } from "@/test/server"

const SLUG = "household"

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  setCurrentGroupSlug(SLUG)
})

describe("safeSourceUrl", () => {
  it("keeps http and https addresses only", () => {
    expect(safeSourceUrl("https://paperless.example/documents/1/")).toBe(
      "https://paperless.example/documents/1/"
    )
    expect(safeSourceUrl("http://10.0.0.5:8000/documents/1/")).toBe(
      "http://10.0.0.5:8000/documents/1/"
    )
    expect(safeSourceUrl("javascript:alert(1)")).toBeNull()
    expect(safeSourceUrl("data:text/html,hi")).toBeNull()
    expect(safeSourceUrl("/documents/1/")).toBeNull()
    expect(safeSourceUrl("")).toBeNull()
    expect(safeSourceUrl(undefined)).toBeNull()
  })
})

describe("toDocumentSource", () => {
  it("shows the display name, or the system without one", () => {
    expect(toDocumentSource("Paperless-ngx", "paperless-ngx", "https://p.example/")).toEqual({
      name: "Paperless-ngx",
      url: "https://p.example/",
    })
    expect(toDocumentSource("  ", "paperless-ngx", "ftp://p.example/")).toEqual({
      name: "paperless-ngx",
      url: null,
    })
    expect(toDocumentSource("", "", "https://p.example/")).toBeNull()
  })
})

describe("listIntakeDocuments", () => {
  it("asks for the drafts and items named and reads each document", async () => {
    let query = ""
    server.use(
      http.get(apiUrl(`/g/${SLUG}/intake/documents`), ({ request }) => {
        query = new URL(request.url).search
        return HttpResponse.json({
          data: [
            {
              id: "doc-1",
              type: "intake_documents",
              attributes: {
                external_system: "paperless-ngx",
                source_name: "Paperless-ngx",
                source_url: "https://p.example/documents/1/",
                draft_id: "d1",
                commodity_id: "c1",
                state: "item",
              },
            },
            { id: "doc-2", type: "intake_documents", attributes: { state: "draft" } },
          ],
          meta: { documents: 2 },
        })
      })
    )
    const docs = await listIntakeDocuments({ draftIds: ["d1", "d2"], commodityIds: ["c1"] })
    expect(new URLSearchParams(query).get("draft_id")).toBe("d1,d2")
    expect(new URLSearchParams(query).get("commodity_id")).toBe("c1")
    // A document that names no system is left out.
    expect(docs).toEqual([
      {
        id: "doc-1",
        source: { name: "Paperless-ngx", url: "https://p.example/documents/1/" },
        draftId: "d1",
        commodityId: "c1",
        state: "item",
      },
    ])
  })

  it("filters the fake as the server does", async () => {
    server.use(
      ...intakeHandlers.documents(SLUG, [
        {
          id: "doc-1",
          external_system: "paperless-ngx",
          external_document_id: "1",
          commodity_id: "c1",
          state: "item",
        },
      ])
    )
    expect(await listIntakeDocuments({ commodityIds: ["c2"] })).toEqual([])
    expect(await listIntakeDocuments({ commodityIds: ["c1"] })).toHaveLength(1)
  })
})
