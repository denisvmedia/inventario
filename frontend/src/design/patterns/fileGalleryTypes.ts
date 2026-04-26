/** Signed URL metadata returned by file listing endpoints. */
export interface FileGallerySignedUrlData {
  url?: string
  thumbnails?: Record<string, string>
}

/** Map of file IDs to signed preview/download URLs. */
export type FileGallerySignedUrls = Record<string, FileGallerySignedUrlData>
