// IDB-backed store for pending file attachments staged in the
// CommodityFormDialog's Files step. Local component state is enough
// for desktop, but Android Chrome aggressively unloads tabs while a
// native file picker is open — when the picker dismisses, the tab
// reloads, and any in-memory `File` references are gone. Blobs can't
// round-trip through `localStorage` (it's string-only), but
// IndexedDB preserves them natively via structured cloning.
//
// We keep the API minimal: save the whole list per draft key, load
// it back on dialog open, clear it on submit-success or explicit
// discard. All operations swallow errors and warn — IDB write
// failures shouldn't block submit (the user can still re-pick files
// the old way).

import type { PendingFileSuggestion } from "@/features/commodities/draft"

const DB_NAME = "inventario-drafts"
const STORE_NAME = "pending-files"
const DB_VERSION = 1

export interface StoredPendingFile {
  id: string
  file: File
  tags: string[]
  // The title the user typed for the file (#1626).
  title?: string
  // Set once a server draft save uploaded the file (#2812).
  uploadedFileId?: string
  // The tag and title an AI scan suggested for the file, and the ones the
  // user rejected (#1626).
  suggestion?: PendingFileSuggestion
}

// suggestionOf keeps a stored suggestion only in the shape the form writes;
// a part of another shape is dropped, not the whole entry.
function suggestionOf(raw: unknown): PendingFileSuggestion | undefined {
  if (!raw || typeof raw !== "object") return undefined
  const { tag, title, rejectedTags, rejectedTitle } = raw as Record<string, unknown>
  const out: PendingFileSuggestion = {}
  if (typeof tag === "string") out.tag = tag
  if (typeof title === "string") out.title = title
  if (Array.isArray(rejectedTags)) {
    const tags = rejectedTags.filter((t): t is string => typeof t === "string")
    if (tags.length > 0) out.rejectedTags = tags
  }
  if (typeof rejectedTitle === "string") out.rejectedTitle = rejectedTitle
  return Object.keys(out).length > 0 ? out : undefined
}

let dbPromise: Promise<IDBDatabase> | null = null

function openDB(): Promise<IDBDatabase> {
  if (typeof indexedDB === "undefined") {
    return Promise.reject(new Error("IndexedDB not available"))
  }
  if (!dbPromise) {
    dbPromise = new Promise<IDBDatabase>((resolve, reject) => {
      const req = indexedDB.open(DB_NAME, DB_VERSION)
      req.onupgradeneeded = () => {
        const db = req.result
        if (!db.objectStoreNames.contains(STORE_NAME)) {
          db.createObjectStore(STORE_NAME)
        }
      }
      req.onsuccess = () => resolve(req.result)
      req.onerror = () => reject(req.error ?? new Error("Failed to open IDB"))
      req.onblocked = () => reject(new Error("IDB open blocked"))
    }).catch((err) => {
      // Reset so future calls get a fresh attempt instead of a
      // permanently-rejected promise.
      dbPromise = null
      throw err
    })
  }
  return dbPromise
}

export async function savePendingFiles(key: string, entries: StoredPendingFile[]): Promise<void> {
  try {
    const db = await openDB()
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, "readwrite")
      tx.oncomplete = () => resolve()
      tx.onerror = () => reject(tx.error ?? new Error("IDB save failed"))
      tx.onabort = () => reject(tx.error ?? new Error("IDB save aborted"))
      tx.objectStore(STORE_NAME).put(entries, key)
    })
  } catch (err) {
    console.warn("[pending-files] save failed", err)
  }
}

export async function loadPendingFiles(key: string): Promise<StoredPendingFile[]> {
  try {
    const db = await openDB()
    const result = await new Promise<unknown>((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, "readonly")
      const req = tx.objectStore(STORE_NAME).get(key)
      req.onsuccess = () => resolve(req.result)
      req.onerror = () => reject(req.error ?? new Error("IDB load failed"))
    })
    if (!Array.isArray(result)) return []
    // Defensive shape-check: only return entries that have all
    // expected fields. Old / malformed records get dropped.
    return result
      .filter(
        (entry): entry is StoredPendingFile =>
          !!entry &&
          typeof entry === "object" &&
          typeof (entry as StoredPendingFile).id === "string" &&
          (entry as StoredPendingFile).file instanceof File &&
          Array.isArray((entry as StoredPendingFile).tags) &&
          ["undefined", "string"].includes(typeof (entry as StoredPendingFile).title) &&
          ["undefined", "string"].includes(typeof (entry as StoredPendingFile).uploadedFileId)
      )
      .map((entry) => {
        const { suggestion, ...rest } = entry
        const kept = suggestionOf(suggestion)
        return kept ? { ...rest, suggestion: kept } : rest
      })
  } catch (err) {
    console.warn("[pending-files] load failed", err)
    return []
  }
}

export async function clearPendingFiles(key: string): Promise<void> {
  try {
    const db = await openDB()
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, "readwrite")
      tx.oncomplete = () => resolve()
      tx.onerror = () => reject(tx.error ?? new Error("IDB clear failed"))
      tx.onabort = () => reject(tx.error ?? new Error("IDB clear aborted"))
      tx.objectStore(STORE_NAME).delete(key)
    })
  } catch (err) {
    console.warn("[pending-files] clear failed", err)
  }
}
