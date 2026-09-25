import "fake-indexeddb/auto"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { Blob as NodeBlob, File as NodeFile } from "node:buffer"

import {
  clearPendingFiles,
  loadPendingFiles,
  savePendingFiles,
  type StoredPendingFile,
} from "@/lib/pending-files-store"

// The store exists because Android Chrome unloads the tab while a native
// file picker is open: the File objects the user just staged are gone on
// reload unless they round-trip through IndexedDB. These tests are about
// that round-trip surviving intact, and about a broken store degrading to
// "no staged files" rather than throwing into a submit handler.

const DB_NAME = "inventario-drafts"
const STORE_NAME = "pending-files"

function entry(name: string, body = "bytes", tags: string[] = []): StoredPendingFile {
  return { id: `p-${name}`, file: new File([body], name, { type: "image/png" }), tags }
}

const DB_VERSION = 1

// writeRaw puts an arbitrary value under `key`, bypassing savePendingFiles,
// so the reader's shape check can be driven with records it would never
// write itself (an older release, a hand-edited store).
//
// It opens at the store's own version and creates the object store the same
// way the store does. Opening without a version creates the database at
// version 1 with nothing in it, and every later call then fails with
// NotFoundError because the upgrade that would have created the store never
// fires again — which is a failure that depends on whether this helper or
// the store touched IndexedDB first.
async function writeRaw(key: string, value: unknown): Promise<void> {
  const db = await new Promise<IDBDatabase>((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      if (!req.result.objectStoreNames.contains(STORE_NAME)) {
        req.result.createObjectStore(STORE_NAME)
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, "readwrite")
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
    tx.objectStore(STORE_NAME).put(value, key)
  })
  db.close()
}

let warn: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  // jsdom's File is not structured-cloneable, so fake-indexeddb stores it
  // as an empty object and every round-trip assertion below would pass for
  // the wrong reason. Node's File implements the same spec and does clone,
  // which is what a real browser's IndexedDB does with a File. Swapping it
  // in is what makes this environment model the browser rather than jsdom.
  vi.stubGlobal("File", NodeFile)
  vi.stubGlobal("Blob", NodeBlob)
  // Every degraded path warns; silence it so a failing assertion is the
  // only thing in the output.
  warn = vi.spyOn(console, "warn").mockImplementation(() => {})
})

afterEach(() => {
  warn.mockRestore()
  vi.unstubAllGlobals()
})

describe("pending-files-store", () => {
  it("round-trips the staged files, bytes and tags included", async () => {
    await savePendingFiles("draft:round-trip", [entry("a.png", "hello", ["receipt"])])

    const loaded = await loadPendingFiles("draft:round-trip")

    expect(loaded).toHaveLength(1)
    expect(loaded[0].id).toBe("p-a.png")
    expect(loaded[0].tags).toEqual(["receipt"])
    expect(loaded[0].file).toBeInstanceOf(File)
    expect(loaded[0].file.name).toBe("a.png")
    expect(await loaded[0].file.text()).toBe("hello")
  })

  it("returns nothing for a key that was never written", async () => {
    expect(await loadPendingFiles("draft:never-written")).toEqual([])
  })

  it("replaces the whole list rather than appending to it", async () => {
    await savePendingFiles("draft:replace", [entry("first.png")])
    await savePendingFiles("draft:replace", [entry("second.png")])

    const loaded = await loadPendingFiles("draft:replace")
    expect(loaded.map((e) => e.file.name)).toEqual(["second.png"])
  })

  it("keeps drafts apart and clears only the key it was asked for", async () => {
    await savePendingFiles("draft:keep", [entry("keep.png")])
    await savePendingFiles("draft:drop", [entry("drop.png")])

    await clearPendingFiles("draft:drop")

    expect(await loadPendingFiles("draft:drop")).toEqual([])
    expect(await loadPendingFiles("draft:keep")).toHaveLength(1)
  })

  it("clearing a key that holds nothing is not an error", async () => {
    await expect(clearPendingFiles("draft:absent")).resolves.toBeUndefined()
  })

  it("drops records that are not shaped like a staged file", async () => {
    await writeRaw("draft:malformed", [
      entry("good.png"),
      { id: "no-file", tags: [] },
      { id: 42, file: new File(["x"], "bad-id.png"), tags: [] },
      { id: "no-tags", file: new File(["x"], "no-tags.png") },
      null,
    ])

    const loaded = await loadPendingFiles("draft:malformed")
    expect(loaded.map((e) => e.file.name)).toEqual(["good.png"])
  })

  it("treats a non-list record as empty", async () => {
    await writeRaw("draft:not-a-list", { id: "p1" })
    expect(await loadPendingFiles("draft:not-a-list")).toEqual([])
  })

  it("degrades to no staged files when IndexedDB is unavailable", async () => {
    // A fresh module instance: the real one has already cached an open
    // database, and the availability check only runs on the first open.
    vi.resetModules()
    vi.stubGlobal("indexedDB", undefined)
    const store = await import("@/lib/pending-files-store")

    // Submit must go through even with no store to read from — losing the
    // staged photos is recoverable, a thrown submit handler is not.
    await expect(store.savePendingFiles("draft:no-idb", [entry("a.png")])).resolves.toBeUndefined()
    await expect(store.loadPendingFiles("draft:no-idb")).resolves.toEqual([])
    await expect(store.clearPendingFiles("draft:no-idb")).resolves.toBeUndefined()
    expect(warn).toHaveBeenCalled()
  })
})
