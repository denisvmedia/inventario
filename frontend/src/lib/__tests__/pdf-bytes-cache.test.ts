import { beforeEach, describe, expect, it } from "vitest"

import {
  __resetPdfBytesCacheForTests,
  cachePdfBytes,
  detachableCopy,
  getCachedPdfBytes,
  pdfCacheKey,
  rememberBytes,
} from "@/lib/pdf-bytes-cache"

const inlineUrl = (fid: string) =>
  `/api/v1/files/download/files/${fid}?sig=aaa&exp=1700000000&uid=u1&fid=${fid}&disposition=inline`
const attachmentUrl = (fid: string) =>
  `/api/v1/files/download/files/${fid}?sig=bbb&exp=1700000900&uid=u1&fid=${fid}`

beforeEach(() => {
  __resetPdfBytesCacheForTests()
})

describe("pdfCacheKey", () => {
  // The inline and attachment URLs for one file differ in signature, expiry
  // and disposition — keying on the URL would miss the very case this cache
  // exists for.
  it("reads the same key from the inline and attachment URLs", () => {
    expect(pdfCacheKey(inlineUrl("file-1"))).toBe("file-1")
    expect(pdfCacheKey(attachmentUrl("file-1"))).toBe("file-1")
    expect(inlineUrl("file-1")).not.toBe(attachmentUrl("file-1"))
  })

  it("falls back to the path when there is no fid parameter", () => {
    expect(pdfCacheKey("/api/v1/files/download/files/file-2")).toBe("file-2")
  })

  it("declines a URL that is not a signed download", () => {
    expect(pdfCacheKey("https://example.test/somewhere/else.pdf")).toBeNull()
    expect(pdfCacheKey("not a url at all")).toBeNull()
  })
})

describe("the byte cache", () => {
  it("serves one viewer's bytes to the other", () => {
    cachePdfBytes(attachmentUrl("file-1"), new Uint8Array([1, 2, 3]))
    expect(getCachedPdfBytes(inlineUrl("file-1"))).toEqual(new Uint8Array([1, 2, 3]))
  })

  it("keeps files apart", () => {
    cachePdfBytes(attachmentUrl("file-1"), new Uint8Array([1]))
    expect(getCachedPdfBytes(attachmentUrl("file-2"))).toBeUndefined()
  })

  it("ignores a URL it cannot key, rather than storing under a wrong key", () => {
    cachePdfBytes("https://example.test/doc.pdf", new Uint8Array([9]))
    expect(getCachedPdfBytes("https://example.test/doc.pdf")).toBeUndefined()
  })

  it("does not store an empty document", () => {
    cachePdfBytes(attachmentUrl("file-3"), new Uint8Array())
    expect(getCachedPdfBytes(attachmentUrl("file-3"))).toBeUndefined()
  })

  it("evicts the least recently used once past the entry budget", () => {
    for (let i = 0; i < 9; i++) {
      cachePdfBytes(attachmentUrl(`file-${i}`), new Uint8Array([i]))
    }
    // Nine stored, eight kept: the first one out is the one longest unused.
    expect(getCachedPdfBytes(attachmentUrl("file-0"))).toBeUndefined()
    expect(getCachedPdfBytes(attachmentUrl("file-8"))).toEqual(new Uint8Array([8]))
  })

  it("a read counts as use, so a re-read survives eviction", () => {
    cachePdfBytes(attachmentUrl("file-a"), new Uint8Array([1]))
    for (let i = 0; i < 7; i++) {
      cachePdfBytes(attachmentUrl(`filler-${i}`), new Uint8Array([i]))
    }
    // Touch the oldest, then push one more in: the untouched filler goes first.
    expect(getCachedPdfBytes(attachmentUrl("file-a"))).toEqual(new Uint8Array([1]))
    cachePdfBytes(attachmentUrl("file-b"), new Uint8Array([2]))
    expect(getCachedPdfBytes(attachmentUrl("file-a"))).toEqual(new Uint8Array([1]))
    expect(getCachedPdfBytes(attachmentUrl("filler-0"))).toBeUndefined()
  })
})

describe("detachableCopy", () => {
  // pdf.js transfers the buffer it is handed to its worker, which detaches it.
  // Handing over the cached entry itself would empty the cache on first use.
  it("returns a copy, so detaching it leaves the original intact", () => {
    const stored = new Uint8Array([1, 2, 3])
    const copy = detachableCopy(stored)
    expect(copy).toEqual(stored)
    expect(copy.buffer).not.toBe(stored.buffer)
  })
})

describe("rememberBytes", () => {
  it("stores what the document hands back", async () => {
    const bytes = new Uint8Array([7, 7, 7])
    rememberBytes({ getData: () => Promise.resolve(bytes) }, attachmentUrl("file-x"))
    await Promise.resolve()
    await Promise.resolve()
    expect(getCachedPdfBytes(attachmentUrl("file-x"))).toEqual(bytes)
  })

  it("survives a document without getData", () => {
    expect(() => rememberBytes({}, attachmentUrl("file-y"))).not.toThrow()
    expect(getCachedPdfBytes(attachmentUrl("file-y"))).toBeUndefined()
  })

  it("survives getData rejecting", async () => {
    rememberBytes(
      { getData: () => Promise.reject(new Error("worker gone")) },
      attachmentUrl("file-z")
    )
    await Promise.resolve()
    await Promise.resolve()
    expect(getCachedPdfBytes(attachmentUrl("file-z"))).toBeUndefined()
  })
})
