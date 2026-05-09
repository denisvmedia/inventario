// Storage usage API client (#1388). The endpoint is plain JSON (not
// JSON:API), so the wire shape from the BE handler is returned as-is.
import { http } from "@/lib/http"

export interface StorageBreakdown {
  images: number
  invoices: number
  documents: number
  other: number
  exports: number
}

export interface StorageUsage {
  // Total bytes used across every bucket.
  used_bytes: number
  // Per-group quota in bytes. `null` means "unlimited" — the FE shows
  // the absolute number without a progress bar in that case. Today the
  // BE always returns a number, but the type stays nullable so the
  // upcoming plans-aware path is wire-compatible.
  quota_bytes: number | null
  breakdown: StorageBreakdown
}

export async function getStorageUsage(signal?: AbortSignal): Promise<StorageUsage> {
  return http.get<StorageUsage>("/storage-usage", { signal })
}

// getStorageUsageForSlug bypasses the active-group rewrite and hits
// `/g/<slug>/storage-usage` directly. The Settings card uses this so
// the panel works on the personal `/settings` route (which has no
// active group in the URL) by falling back to the user's first
// group.
export async function getStorageUsageForSlug(
  slug: string,
  signal?: AbortSignal
): Promise<StorageUsage> {
  return http.get<StorageUsage>(`/g/${encodeURIComponent(slug)}/storage-usage`, {
    signal,
    skipGroupRewrite: true,
  })
}
