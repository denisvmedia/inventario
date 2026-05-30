import type { CommodityCover } from "@/features/commodities/api"
import type { ListedFile } from "@/features/files/api"

import type { PhotoSize, ReportPhoto } from "./components/PhotoSection"

// pickFileUrl resolves the best image URL for the requested size from a
// signed-URL payload. For `thumb` we prefer a small/medium thumbnail
// (smaller payload for the grid); for `full` we prefer the original `url`
// and only fall back to a thumbnail.
function pickFileUrl(
  signed: { url?: string; thumbnails?: Record<string, string> } | undefined,
  size: PhotoSize
): string | undefined {
  if (!signed) return undefined
  const thumb = signed.thumbnails?.small ?? signed.thumbnails?.medium ?? signed.thumbnails?.large
  if (size === "thumb") return thumb ?? signed.url
  return signed.url ?? thumb
}

// filesToPhotos maps the image-category files attached to a commodity
// (from `useFiles({ linkedEntityType: "commodity", category: "images" })`)
// to displayable report photos, dropping any row without a usable URL.
export function filesToPhotos(rows: ListedFile[], size: PhotoSize): ReportPhoto[] {
  const out: ReportPhoto[] = []
  for (const row of rows) {
    const url = pickFileUrl(row.signedUrl, size)
    if (!url) continue
    const name = row.file.title?.trim() || row.file.path?.trim() || row.file.id
    out.push({ url, name })
  }
  return out
}

// coverToPhotos maps a single list-mode cover descriptor to a one-element
// photo list (Location mode shows the cover thumbnail per item). The cover
// carries only a `thumbnails` map (no full-size URL), so both sizes resolve
// from it. Returns an empty list when the cover is missing or has no usable
// URL.
export function coverToPhotos(
  cover: CommodityCover | undefined,
  size: PhotoSize,
  name: string
): ReportPhoto[] {
  if (!cover?.thumbnails) return []
  const small = cover.thumbnails.small
  const medium = cover.thumbnails.medium
  const large = cover.thumbnails.large
  const url = size === "thumb" ? (small ?? medium ?? large) : (large ?? medium ?? small)
  if (!url) return []
  return [{ url, name }]
}
