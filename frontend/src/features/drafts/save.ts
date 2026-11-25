// Saving the item form as a server draft (#2812), outside React so the
// steps can be tested on their own.
//
// A save is safe to repeat. The draft is created with a client key that the
// form keeps until the save succeeds, so a retry after a lost response finds
// the draft instead of making a second one. A file whose content is already
// attached to the draft is not uploaded again.
import { attachFile, updateFile, uploadFile, type ListedFile } from "@/features/files/api"
import { categoryFromMime } from "@/features/files/constants"
import type { PendingFile } from "@/features/commodities/draft"

import { createDraft, listDraftFiles, saveDraft, type Draft, type DraftValues } from "./api"

// Keys kept next to a form's local recovery slot. The client key lives in
// sessionStorage, so two tabs with a new form each get a draft of their
// own; a reload keeps it, so a retry after a lost response still finds the
// draft. The base revision of a resumed draft lives in localStorage with
// the values it belongs to.
const CLIENT_KEY_SUFFIX = ":client-key"
const BASE_REVISION_SUFFIX = ":base-revision"

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

// The draft revision the locally kept values of a resumed draft are based
// on. Kept with the values so a save after a reload is checked against it.
export function readBaseRevision(localKey: string): number | null {
  const raw = read("local", localKey + BASE_REVISION_SUFFIX)
  if (raw === null) return null
  const n = Number(raw)
  return Number.isInteger(n) && n > 0 ? n : null
}

export function writeBaseRevision(localKey: string, revision: number): void {
  write("local", localKey + BASE_REVISION_SUFFIX, String(revision))
}

// clearDraftSession forgets the client key and base revision of a slot.
export function clearDraftSession(localKey: string): void {
  remove("session", localKey + CLIENT_KEY_SUFFIX)
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
  values: DraftValues
  pendingFiles: PendingFile[]
  // Called for each pending file once it is attached to the draft.
  onFileSaved?: (entry: PendingFile) => void
}

export interface SaveDraftResult {
  draft: Draft
  failedFiles: { entry: PendingFile; error: unknown }[]
}

// saveDraftWithFiles stores the values, then uploads and attaches the
// pending files one by one. A failure of the values save throws; a failed
// file is reported in failedFiles and stays pending.
export async function saveDraftWithFiles(input: SaveDraftInput): Promise<SaveDraftResult> {
  let draft: Draft
  if (input.draft) {
    draft = await saveDraft(input.draft.id, input.draft.revision, input.values)
  } else {
    draft = await createDraft(input.clientKey, input.values)
    // A retry finds the draft an earlier attempt made, with the values of
    // that attempt.
    if (draft.status === "open" && canonical(draft.values) !== canonical(input.values)) {
      draft = await saveDraft(draft.id, draft.revision, input.values)
    }
  }

  const failedFiles: SaveDraftResult["failedFiles"] = []
  if (input.pendingFiles.length === 0) return { draft, failedFiles }

  let attached: ListedFile[] = []
  try {
    attached = await listDraftFiles(draft.id)
  } catch {
    // Without the list a retry may upload a file twice; better than not
    // saving it.
  }
  for (const entry of input.pendingFiles) {
    try {
      if (!(await alreadyAttached(entry, attached))) {
        const uploaded = await uploadFile(entry.file)
        if (entry.tags.length > 0) {
          await updateFile(uploaded.file.id, { tags: entry.tags })
        }
        const category = categoryFromMime(entry.file.type)
        await attachFile(uploaded.file.id, {
          entityType: "draft",
          entityId: draft.id,
          meta: category === "other" ? undefined : category,
        })
      }
      input.onFileSaved?.(entry)
    } catch (error) {
      failedFiles.push({ entry, error })
    }
  }
  return { draft, failedFiles }
}
