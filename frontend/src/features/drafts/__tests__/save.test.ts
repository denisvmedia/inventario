import { beforeEach, describe, expect, it } from "vitest"

import {
  clearDraftSession,
  clientKeyFor,
  readBaseRevision,
  replaceClientKey,
  saveDraftWithFiles,
  writeBaseRevision,
} from "@/features/drafts/save"
import { promoteDraft, revisionConflict, saveDraft } from "@/features/drafts/api"
import type { PendingFile } from "@/features/commodities/draft"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { draftHandlers } from "@/test/handlers"
import { server } from "@/test/server"

const SLUG = "household"

function pending(name: string, body = name, type = "image/png"): PendingFile {
  return { id: `p-${name}`, file: new File([body], name, { type }), tags: [] }
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setCurrentGroupSlug(SLUG)
  setAccessToken("good-token")
  window.localStorage.clear()
  window.sessionStorage.clear()
})

describe("client key", () => {
  it("is kept per form slot until replaced or cleared", () => {
    const first = clientKeyFor("slot")
    expect(first).toMatch(/^form:/)
    expect(clientKeyFor("slot")).toBe(first)
    expect(clientKeyFor("other")).not.toBe(first)
    const second = replaceClientKey("slot")
    expect(second).not.toBe(first)
    expect(clientKeyFor("slot")).toBe(second)
    writeBaseRevision("slot", 3)
    expect(readBaseRevision("slot")).toBe(3)
    clearDraftSession("slot")
    expect(readBaseRevision("slot")).toBeNull()
    expect(clientKeyFor("slot")).not.toBe(second)
  })
})

describe("saveDraftWithFiles", () => {
  it("creates the draft and attaches the staged files to it", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(...fake.handlers)
    const saved: string[] = []
    const result = await saveDraftWithFiles({
      draft: null,
      clientKey: "form:a",
      values: { name: "Drill" },
      pendingFiles: [pending("front.png"), pending("receipt.pdf", "r", "application/pdf")],
      onFileSaved: (entry) => saved.push(entry.file.name),
    })
    expect(result.failedFiles).toEqual([])
    expect(result.draft.title).toBe("Drill")
    expect(saved).toEqual(["front.png", "receipt.pdf"])
    expect(fake.files).toHaveLength(2)
    // The link bucket follows the file type the browser reports.
    expect(fake.files.map((f) => f.links[0])).toEqual([
      expect.objectContaining({ entity_type: "draft", entity_id: result.draft.id, meta: "images" }),
      expect.objectContaining({
        entity_type: "draft",
        entity_id: result.draft.id,
        meta: "documents",
      }),
    ])
  })

  it("makes one draft when a save is repeated after a lost response", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(...fake.handlers)
    const file = pending("front.png")
    fake.nextUploads.push({ name: "front.png", size: file.file.size })
    await saveDraftWithFiles({
      draft: null,
      clientKey: "form:retry",
      values: { name: "Drill" },
      pendingFiles: [file],
    })
    // The client never saw the answer, so it repeats the whole save with
    // newer values and the same file.
    const again = await saveDraftWithFiles({
      draft: null,
      clientKey: "form:retry",
      values: { name: "Cordless drill" },
      pendingFiles: [file],
    })
    expect(fake.creates).toBe(2)
    expect(fake.drafts).toHaveLength(1)
    expect(fake.drafts[0].values).toEqual({ name: "Cordless drill" })
    expect(again.draft.revision).toBe(2)
    expect(fake.uploads).toBe(1)
  })

  it("reports a file that failed and keeps the values saved", async () => {
    const fake = draftHandlers.backend(SLUG)
    fake.failUploads = 500
    server.use(...fake.handlers)
    const saved: string[] = []
    const result = await saveDraftWithFiles({
      draft: null,
      clientKey: "form:b",
      values: { name: "Lamp" },
      pendingFiles: [pending("lamp.png")],
      onFileSaved: (entry) => saved.push(entry.file.name),
    })
    expect(fake.drafts).toHaveLength(1)
    expect(saved).toEqual([])
    expect(result.failedFiles.map((f) => f.entry.file.name)).toEqual(["lamp.png"])
  })

  it("refuses a save based on an old revision", async () => {
    const fake = draftHandlers.backend(SLUG, [{ id: "d1", values: { name: "A" }, revision: 4 }])
    server.use(...fake.handlers)
    const err = await saveDraftWithFiles({
      draft: { id: "d1", revision: 3 },
      clientKey: "form:c",
      values: { name: "B" },
      pendingFiles: [],
    }).catch((e: unknown) => e)
    expect(revisionConflict(err)).toEqual({ revision: 4 })
    expect(fake.drafts[0].values).toEqual({ name: "A" })
  })
})

describe("promoteDraft", () => {
  it("moves the draft's files to the new item and returns the same item when repeated", async () => {
    const fake = draftHandlers.backend(SLUG, [{ id: "d1", values: { name: "Drill" } }])
    server.use(...fake.handlers)
    await saveDraftWithFiles({
      draft: { id: "d1", revision: 1 },
      clientKey: "form:d",
      values: { name: "Drill" },
      pendingFiles: [pending("front.png")],
    })
    const values = { name: "Drill", short_name: "Drill", type: "equipment" } as never
    const item = await promoteDraft("d1", 2, values)
    expect(item.id).toBe("item-from-d1")
    expect(fake.files[0].links[0]).toMatchObject({
      entity_type: "commodity",
      entity_id: "item-from-d1",
    })
    const again = await promoteDraft("d1", 2, values)
    expect(again.id).toBe(item.id)
    await expect(saveDraft("d1", 2, { name: "x" })).rejects.toBeTruthy()
  })
})
