/**
 * A small in-memory store of PDF bytes, keyed by file rather than by URL.
 *
 * The inline panel and the fullscreen reader each call `getDocument` on their
 * own, and the download endpoint answers `Cache-Control: no-store` so the
 * browser cache cannot help either. Opening a PDF inline and expanding it
 * therefore fetched the same multi-megabyte file twice, and so did every
 * close-and-reopen (#1977).
 *
 * The two viewers do not receive the same URL — one is signed for inline
 * disposition, the other for attachment, and each carries its own signature
 * and expiry. What they do share is the file id, which the signer puts in the
 * path and repeats as `fid`, so that is the key.
 *
 * Caching bytes we have already been given does not widen the signed-URL
 * window: nothing is re-requested with an expired URL, there is simply no
 * second request. The cache lives for the page session and is never persisted.
 */

// Enough for a handful of large manuals without letting a long session hold
// an unbounded amount of memory.
const MAX_BYTES = 64 * 1024 * 1024
const MAX_ENTRIES = 8

// Insertion-ordered, which is what makes the eviction below least-recent-first.
const cache = new Map<string, Uint8Array>()
let totalBytes = 0

/**
 * The file id a signed download URL refers to, or null when the URL is not one
 * of ours. Both the inline and the attachment URL put the id in the last path
 * segment and repeat it as `fid`; the path is used because it is present even
 * on a URL built without the query parameters.
 */
export function pdfCacheKey(url: string): string | null {
  try {
    const parsed = new URL(url, window.location.origin)
    if (!parsed.pathname.includes("/files/download/")) return null
    const fid = parsed.searchParams.get("fid")
    if (fid) return fid
    const last = parsed.pathname.split("/").filter(Boolean).pop()
    return last || null
  } catch {
    return null
  }
}

/** The cached bytes for a URL's file, or undefined. */
export function getCachedPdfBytes(url: string): Uint8Array | undefined {
  const key = pdfCacheKey(url)
  if (!key) return undefined
  const hit = cache.get(key)
  if (!hit) return undefined
  // Re-insert so eviction sees this as the most recent.
  cache.delete(key)
  cache.set(key, hit)
  return hit
}

/** Remember a document's bytes. A file too large for the budget is not cached. */
export function cachePdfBytes(url: string, bytes: Uint8Array): void {
  const key = pdfCacheKey(url)
  if (!key || bytes.byteLength === 0 || bytes.byteLength > MAX_BYTES) return

  const existing = cache.get(key)
  if (existing) {
    totalBytes -= existing.byteLength
    cache.delete(key)
  }
  cache.set(key, bytes)
  totalBytes += bytes.byteLength

  while (cache.size > MAX_ENTRIES || totalBytes > MAX_BYTES) {
    const oldest = cache.keys().next()
    if (oldest.done) break
    const evicted = cache.get(oldest.value)
    cache.delete(oldest.value)
    totalBytes -= evicted?.byteLength ?? 0
  }
}

/**
 * pdf.js transfers the buffer it is handed to its worker, which detaches it —
 * so a cached entry must be copied before it is passed on, or the second
 * reader would find the bytes gone.
 */
export function detachableCopy(bytes: Uint8Array): Uint8Array {
  return new Uint8Array(bytes)
}

export function __resetPdfBytesCacheForTests(): void {
  cache.clear()
  totalBytes = 0
}

/**
 * Keep a freshly-loaded document's bytes for the next viewer. `getData`
 * copies them out of the worker, which is the only way to get at what pdf.js
 * downloaded; a failure is not worth surfacing, it only means the next open
 * pays for the download again.
 */
export function rememberBytes(doc: { getData?: () => Promise<Uint8Array> }, url: string): void {
  // Optional because the caller holds whatever pdf.js handed back; a build
  // without getData should cost the cache, not the viewer.
  if (typeof doc.getData !== "function") return
  void doc
    .getData()
    .then((bytes) => cachePdfBytes(url, bytes))
    .catch(() => {})
}
