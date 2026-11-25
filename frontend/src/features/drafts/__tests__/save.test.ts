import { beforeEach, describe, expect, it } from "vitest"

import {
  clearDraftSession,
  clientKeyFor,
  promotionKeyFor,
  readBaseRevision,
  replaceClientKey,
  saveDraftWithFiles,
  writeBaseRevision,
} from "@/features/drafts/save"
import {
  discardedConflict,
  promoteDraft,
  promotedConflict,
  revisionConflict,
  saveDraft,
} from "@/features/drafts/api"
import { uploadPendingFiles, type PendingFile } from "@/features/commodities/draft"
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
    const promotion = promotionKeyFor("slot")
    expect(promotion).toMatch(/^promote:/)
    expect(promotionKeyFor("slot")).toBe(promotion)
    clearDraftSession("slot")
    expect(readBaseRevision("slot")).toBeNull()
    expect(clientKeyFor("slot")).not.toBe(second)
    expect(promotionKeyFor("slot")).not.toBe(promotion)
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

  it("does not overwrite a found draft another device saved since", async () => {
    // An earlier create made the draft, its response was lost, and another
    // device saved the draft before this retry.
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", client_key: "form:lost", values: { name: "Newer" }, revision: 2 },
    ])
    server.use(...fake.handlers)
    const err = await saveDraftWithFiles({
      draft: null,
      clientKey: "form:lost",
      values: { name: "Mine" },
      pendingFiles: [],
    }).catch((e: unknown) => e)
    expect(revisionConflict(err)).toEqual({ revision: 2 })
    expect(fake.drafts[0].values).toEqual({ name: "Newer" })
  })

  it("overwrites a found draft this slot saved last", async () => {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", client_key: "form:mine", values: { name: "Before reload" }, revision: 2 },
    ])
    server.use(...fake.handlers)
    const result = await saveDraftWithFiles({
      draft: null,
      clientKey: "form:mine",
      lastSavedRevision: 2,
      values: { name: "After reload" },
      pendingFiles: [],
    })
    expect(result.draft.revision).toBe(3)
    expect(fake.drafts[0].values).toEqual({ name: "After reload" })
  })

  it("treats a save whose response was lost as done", async () => {
    const fake = draftHandlers.backend(SLUG, [{ id: "d1", values: { name: "A" }, revision: 1 }])
    server.use(...fake.handlers)
    fake.loseResponse.put = true
    const lost = await saveDraftWithFiles({
      draft: { id: "d1", revision: 1 },
      clientKey: "form:put",
      values: { name: "B" },
      pendingFiles: [],
    }).catch((e: unknown) => e)
    expect(lost).toBeInstanceOf(Error)
    // The retry is based on the old revision, but the server already has
    // these values.
    const retried = await saveDraftWithFiles({
      draft: { id: "d1", revision: 1 },
      clientKey: "form:put",
      values: { name: "B" },
      pendingFiles: [],
    })
    expect(retried.draft.revision).toBe(2)
    expect(fake.drafts[0].revision).toBe(2)
  })

  it("stops on a draft finished or discarded elsewhere", async () => {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", values: {}, status: "promoted", promoted_commodity_id: "item-9" },
      { id: "d2", values: {}, status: "discarding" },
      { id: "d3", client_key: "form:done", values: {}, status: "promoted" },
      { id: "d4", client_key: "form:odd", values: {}, status: "archived" },
    ])
    server.use(...fake.handlers)
    const save = (draft: { id: string; revision: number } | null, clientKey = "form:x") =>
      saveDraftWithFiles({ draft, clientKey, values: { name: "x" }, pendingFiles: [] }).catch(
        (e: unknown) => e
      )
    expect(promotedConflict(await save({ id: "d1", revision: 1 }))).toEqual({
      commodityId: "item-9",
    })
    expect(discardedConflict(await save({ id: "d2", revision: 1 }))).toBe(true)
    // A retried create that finds its draft already finished.
    expect(promotedConflict(await save(null, "form:done"))).not.toBeNull()
    // A state this client does not know is never saved over.
    expect(discardedConflict(await save(null, "form:odd"))).toBe(true)
    expect(fake.drafts[3].values).toEqual({})
  })

  it("attaches a file an earlier attempt uploaded instead of uploading it again", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(...fake.handlers)
    fake.failAttach = 500
    const entry = pending("front.png")
    const uploaded: string[] = []
    const first = await saveDraftWithFiles({
      draft: null,
      clientKey: "form:upload",
      values: { name: "Drill" },
      pendingFiles: [entry],
      onFileUploaded: (_, id) => uploaded.push(id),
    })
    expect(first.failedFiles).toHaveLength(1)
    expect(uploaded).toHaveLength(1)
    expect(fake.uploads).toBe(1)

    fake.failAttach = null
    const saved: string[] = []
    await saveDraftWithFiles({
      draft: { id: first.draft.id, revision: first.draft.revision },
      clientKey: "form:upload",
      values: { name: "Drill" },
      pendingFiles: [{ ...entry, uploadedFileId: uploaded[0] }],
      onFileSaved: (e) => saved.push(e.file.name),
    })
    expect(fake.uploads).toBe(1)
    expect(saved).toEqual(["front.png"])
    expect(fake.files[0].links).toEqual([
      expect.objectContaining({ entity_type: "draft", entity_id: first.draft.id }),
    ])
  })

  it("uploads again when the remembered file is gone", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(...fake.handlers)
    const result = await saveDraftWithFiles({
      draft: null,
      clientKey: "form:gone",
      values: { name: "Drill" },
      pendingFiles: [{ ...pending("front.png"), uploadedFileId: "file-deleted" }],
    })
    expect(result.failedFiles).toEqual([])
    expect(fake.uploads).toBe(1)
  })

  it("links the files of the old draft to a new one", async () => {
    const fake = draftHandlers.backend(SLUG, [{ id: "d1", values: { name: "Old" } }])
    server.use(...fake.handlers)
    await saveDraftWithFiles({
      draft: { id: "d1", revision: 1 },
      clientKey: "form:old",
      values: { name: "Old" },
      pendingFiles: [pending("front.png")],
    })
    const result = await saveDraftWithFiles({
      draft: null,
      clientKey: "form:new",
      values: { name: "New" },
      pendingFiles: [],
      carryFilesFrom: "d1",
    })
    expect(result.failedCarried).toEqual([])
    expect(fake.files[0].links.map((l) => [l.entity_id, l.meta])).toEqual([
      ["d1", "images"],
      [result.draft.id, "images"],
    ])
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
    const item = await promoteDraft("d1", 2, "promote:a", values)
    expect(item.id).toBe("item-from-d1")
    expect(fake.files[0].links[0]).toMatchObject({
      entity_type: "commodity",
      entity_id: "item-from-d1",
    })
    expect(fake.promotions[0].body.meta).toEqual({ revision: 2, promotion_key: "promote:a" })
    const again = await promoteDraft("d1", 2, "promote:a", values)
    expect(again.id).toBe(item.id)
    await expect(saveDraft("d1", 2, { name: "x" })).rejects.toBeTruthy()
  })

  it("refuses another device's promotion of a finished draft", async () => {
    const fake = draftHandlers.backend(SLUG, [{ id: "d1", values: { name: "Drill" } }])
    server.use(...fake.handlers)
    const values = { name: "Drill", short_name: "Drill", type: "equipment" } as never
    await promoteDraft("d1", 1, "promote:phone", values)
    const err = await promoteDraft("d1", 1, "promote:laptop", values).catch((e: unknown) => e)
    expect(promotedConflict(err)).toEqual({ commodityId: "item-from-d1" })
  })
})

describe("uploadPendingFiles", () => {
  it("attaches a file a draft save uploaded to the new item without uploading it again", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(...fake.handlers)
    fake.failAttach = 500
    const entry = pending("front.png")
    let uploadedId = ""
    await saveDraftWithFiles({
      draft: null,
      clientKey: "form:item",
      values: { name: "Drill" },
      pendingFiles: [entry],
      onFileUploaded: (_, id) => (uploadedId = id),
    })
    fake.failAttach = null
    const errors: unknown[] = []
    await uploadPendingFiles([{ ...entry, uploadedFileId: uploadedId }], "item-1", (_, err) =>
      errors.push(err)
    )
    expect(errors).toEqual([])
    expect(fake.uploads).toBe(1)
    expect(fake.files[0].links).toEqual([
      expect.objectContaining({ entity_type: "commodity", entity_id: "item-1" }),
    ])
  })

  it("uploads a file again when the one a draft save uploaded is gone", async () => {
    const fake = draftHandlers.backend(SLUG)
    server.use(...fake.handlers)
    const errors: unknown[] = []
    await uploadPendingFiles(
      [{ ...pending("front.png"), uploadedFileId: "file-deleted" }],
      "item-1",
      (_, err) => errors.push(err)
    )
    expect(errors).toEqual([])
    expect(fake.uploads).toBe(1)
  })
})
