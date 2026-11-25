// Saving the item form as a server draft (#2812), outside React so the
// steps can be tested on their own.
//
// A save is safe to repeat. The draft is created with a client key that the
// form keeps until the save succeeds, so a retry after a lost response finds
// the draft instead of making a second one. A file whose content is already
// attached to the draft, or that an earlier attempt uploaded, is not uploaded
// again.
import { attachFile, updateFile, uploadFile, type ListedFile } from "@/features/files/api"
import { categoryFromMime } from "@/features/files/constants"
import type { PendingFile } from "@/features/commodities/draft"
import { HttpError } from "@/lib/http"

import {
  createDraft,
  discardedConflict,
  DRAFT_PROMOTED,
  DRAFT_REVISION_CONFLICT,
  DraftStateError,
  getDraft,
  listDraftFiles,
  promotedConflict,
  revisionConflict,
  saveDraft,
  type Draft,
  type DraftValues,
} from "./api"

// Keys kept next to a form's local recovery slot. The client key lives in
// sessionStorage, so two tabs with a new form each get a draft of their
// own; a reload keeps it, so a retry after a lost response still finds the
// draft. The base revision, the one the slot last saved or loaded, lives in
// localStorage with the values it belongs to.
const CLIENT_KEY_SUFFIX = ":client-key"
const BASE_REVISION_SUFFIX = ":base-revision"
// The promotion key lives in sessionStorage next to the client key: a
// retry of a promotion whose response was lost sends the same key and gets
// the item it made.
const PROMOTION_KEY_SUFFIX = ":promotion-key"

// Files larger than this are matched by name and size only: hashing reads
// the whole file into memory.
const HASH_LIMIT_BYTES = 64 * 1024 * 1024

type Area = "local" | "session"

function storage(area: Area): Storage | null {
  if (typeof window === "undefined") return null
  try {
    return area === "local" ? window.localStorage : window.sessionStorage
  } catch {
    return null
  }
}

function read(area: Area, key: string): string | null {
  try {
    return storage(area)?.getItem(key) ?? null
  } catch {
    return null
  }
}

function write(area: Area, key: string, value: string): void {
  try {
    storage(area)?.setItem(key, value)
  } catch {
    // Quota or disabled storage: the form still works, a retry after a
    // reload just cannot reuse the key.
  }
}

function remove(area: Area, key: string): void {
  try {
    storage(area)?.removeItem(key)
  } catch {
    // see write
  }
}

function newClientKey(): string {
  const id =
    typeof crypto !== "undefined" && crypto.randomUUID
      ? crypto.randomUUID()
      : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
  return `form:${id}`
}

// clientKeyFor returns the client key of the form whose local slot is
// localKey, minting and keeping one on first use.
export function clientKeyFor(localKey: string): string {
  const stored = read("session", localKey + CLIENT_KEY_SUFFIX)
  if (stored) return stored
  const key = newClientKey()
  write("session", localKey + CLIENT_KEY_SUFFIX, key)
  return key
}

// replaceClientKey gives the form a new identity, for "save as a new draft".
export function replaceClientKey(localKey: string): string {
  const key = newClientKey()
  write("session", localKey + CLIENT_KEY_SUFFIX, key)
  return key
}

// promotionKeyFor returns the key of the promotion the form in localKey is
// making, minting and keeping one on first use. It is kept until the
// promotion succeeds.
export function promotionKeyFor(localKey: string): string {
  const stored = read("session", localKey + PROMOTION_KEY_SUFFIX)
  if (stored) return stored
  const key = newClientKey().replace(/^form:/, "promote:")
  write("session", localKey + PROMOTION_KEY_SUFFIX, key)
  return key
}

// The draft revision this slot last saved, or that the locally kept values
// of a resumed draft are based on. Kept with the values so a save after a
// reload is checked against it.
export function readBaseRevision(localKey: string): number | null {
  const raw = read("local", localKey + BASE_REVISION_SUFFIX)
  if (raw === null) return null
  const n = Number(raw)
  return Number.isInteger(n) && n > 0 ? n : null
}

export function writeBaseRevision(localKey: string, revision: number): void {
  write("local", localKey + BASE_REVISION_SUFFIX, String(revision))
}

// forgetBaseRevision drops the slot's base revision, for a form that starts
// a new draft.
export function forgetBaseRevision(localKey: string): void {
  remove("local", localKey + BASE_REVISION_SUFFIX)
}

// clearDraftSession forgets the client key, base revision and promotion key
// of a slot.
export function clearDraftSession(localKey: string): void {
  remove("session", localKey + CLIENT_KEY_SUFFIX)
  remove("session", localKey + PROMOTION_KEY_SUFFIX)
  remove("local", localKey + BASE_REVISION_SUFFIX)
}

// canonical serializes values with sorted keys, so two value sets compare
// equal however the server ordered the JSON object.
function canonical(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonical).join(",")}]`
  if (value && typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>)
      .filter(([, v]) => v !== undefined)
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
    return `{${entries.map(([k, v]) => `${JSON.stringify(k)}:${canonical(v)}`).join(",")}}`
  }
  return JSON.stringify(value ?? null)
}

// sameDraftValues reports whether two value sets hold the same input.
export function sameDraftValues(a: unknown, b: unknown): boolean {
  return canonical(a) === canonical(b)
}

async function sha256Hex(file: File): Promise<string | null> {
  if (file.size > HASH_LIMIT_BYTES) return null
  try {
    if (typeof crypto === "undefined" || !crypto.subtle) return null
    const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer())
    return Array.from(new Uint8Array(digest))
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("")
  } catch {
    return null
  }
}

function fileName(row: ListedFile): string {
  return `${row.file.path ?? ""}${row.file.ext ?? ""}`
}

// alreadyAttached reports whether the draft already has this file: the same
// SHA-256 when both are known, otherwise the same name and size.
async function alreadyAttached(entry: PendingFile, attached: ListedFile[]): Promise<boolean> {
  if (attached.length === 0) return false
  const hash = await sha256Hex(entry.file)
  return attached.some((row) => {
    const known = row.file.sha256
    if (hash && known) return known === hash
    return fileName(row) === entry.file.name && row.file.size_bytes === entry.file.size
  })
}

export interface SaveDraftInput {
  // The draft the form is editing, with the revision its values are based
  // on; null for a form that has no draft yet.
  draft: { id: string; revision: number } | null
  clientKey: string
  // For a form without a draft: the revision this slot last saved, when an
  // earlier save made the draft but the form lost track of it (a reload, a
  // lost response). 1 when the slot never saved.
  lastSavedRevision?: number
  values: DraftValues
  pendingFiles: PendingFile[]
  // A draft whose saved files the new draft should get too, for "save as a
  // new draft". The files are linked, not copied.
  carryFilesFrom?: string | null
  // Called for each pending file once it is uploaded, with the file's id,
  // so a retry attaches that file instead of uploading it again.
  onFileUploaded?: (entry: PendingFile, fileId: string) => void
  // Called for each pending file once it is attached to the draft.
  onFileSaved?: (entry: PendingFile) => void
}

export interface SaveDraftResult {
  draft: Draft
  failedFiles: { entry: PendingFile; error: unknown }[]
  // Names of the files of carryFilesFrom that could not be linked.
  failedCarried: string[]
}

// isDraftRefusal reports an error that ends the save: the draft became an
// item or is being discarded elsewhere, so no file can be added to it.
function isDraftRefusal(err: unknown): boolean {
  return !!promotedConflict(err) || discardedConflict(err)
}

// putValues saves values over revision. When the server says the draft
// changed meanwhile but already holds exactly these values, an earlier
// attempt of this same save went through and only its response was lost:
// the save is done, at the revision the server has.
async function putValues(id: string, revision: number, values: DraftValues): Promise<Draft> {
  try {
    return await saveDraft(id, revision, values)
  } catch (err) {
    if (!revisionConflict(err)) throw err
    let fresh: Draft
    try {
      fresh = await getDraft(id)
    } catch {
      throw err
    }
    if (fresh.status === "open" && sameDraftValues(fresh.values, values)) return fresh
    throw err
  }
}

// createOrFind creates the draft, or finds the one an earlier attempt with
// the same client key made. The found draft is overwritten only when this
// slot saved it last; a draft another device saved or finished since is a
// conflict, so its input is never lost.
async function createOrFind(input: SaveDraftInput): Promise<Draft> {
  const draft = await createDraft(input.clientKey, input.values)
  if (draft.status === "promoted") {
    throw new DraftStateError(DRAFT_PROMOTED, {
      commodity_id: draft.promotedCommodityId ?? "",
      draft_id: draft.id,
    })
  }
  if (sameDraftValues(draft.values, input.values)) return draft
  const expected = input.lastSavedRevision ?? 1
  if (draft.revision !== expected) {
    throw new DraftStateError(DRAFT_REVISION_CONFLICT, {
      revision: String(draft.revision),
      draft_id: draft.id,
    })
  }
  return putValues(draft.id, draft.revision, input.values)
}

// carryFiles links the saved files of draft from to draft to, keeping each
// file's bucket. It returns the names of the files it could not link.
async function carryFiles(from: string, to: string): Promise<string[]> {
  let files: ListedFile[]
  try {
    files = await listDraftFiles(from)
  } catch {
    return []
  }
  const failed: string[] = []
  for (const row of files) {
    const link = (row.file.links ?? []).find(
      (l) => l.entity_type === "draft" && l.entity_id === from
    )
    try {
      await attachFile(row.file.id, {
        entityType: "draft",
        entityId: to,
        meta: link?.meta || undefined,
      })
    } catch (err) {
      if (isDraftRefusal(err)) throw err
      failed.push(fileName(row))
    }
  }
  return failed
}

// MissingUpload marks a pending file no earlier attempt uploaded.
class MissingUpload extends Error {}

// isGone reports a request about a file that no longer exists.
function isGone(err: unknown): boolean {
  return err instanceof HttpError && err.status === 404
}

// placeFile tags an uploaded file and attaches it to the draft. Both steps
// can be repeated.
async function placeFile(entry: PendingFile, fileId: string, draftId: string): Promise<void> {
  if (entry.tags.length > 0) {
    await updateFile(fileId, { tags: entry.tags })
  }
  const category = categoryFromMime(entry.file.type)
  await attachFile(fileId, {
    entityType: "draft",
    entityId: draftId,
    meta: category === "other" ? undefined : category,
  })
}

// saveDraftWithFiles stores the values, then uploads and attaches the
// pending files one by one. A failure of the values save throws, as does a
// draft that became an item or is being discarded; a failed file is
// reported in failedFiles and stays pending.
export async function saveDraftWithFiles(input: SaveDraftInput): Promise<SaveDraftResult> {
  const draft = input.draft
    ? await putValues(input.draft.id, input.draft.revision, input.values)
    : await createOrFind(input)

  const failedCarried =
    input.carryFilesFrom && input.carryFilesFrom !== draft.id
      ? await carryFiles(input.carryFilesFrom, draft.id)
      : []

  const failedFiles: SaveDraftResult["failedFiles"] = []
  if (input.pendingFiles.length === 0) return { draft, failedFiles, failedCarried }

  let attached: ListedFile[] = []
  try {
    attached = await listDraftFiles(draft.id)
  } catch {
    // Without the list a retry may upload a file twice; better than not
    // saving it.
  }
  for (const entry of input.pendingFiles) {
    try {
      const done = entry.uploadedFileId
        ? attached.some((row) => row.file.id === entry.uploadedFileId)
        : await alreadyAttached(entry, attached)
      if (!done) {
        // A file an earlier attempt uploaded is attached as it is; only a
        // file that never reached the server, or that was deleted since, is
        // uploaded.
        try {
          if (!entry.uploadedFileId) throw new MissingUpload()
          await placeFile(entry, entry.uploadedFileId, draft.id)
        } catch (err) {
          if (!(err instanceof MissingUpload) && !isGone(err)) throw err
          const fileId = (await uploadFile(entry.file)).file.id
          input.onFileUploaded?.(entry, fileId)
          await placeFile(entry, fileId, draft.id)
        }
      }
      input.onFileSaved?.(entry)
    } catch (error) {
      if (isDraftRefusal(error)) throw error
      failedFiles.push({ entry, error })
    }
  }
  return { draft, failedFiles, failedCarried }
}
