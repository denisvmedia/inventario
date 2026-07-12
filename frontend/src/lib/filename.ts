// appendExt gives `name` the extension `ext`, unless it already ends with it.
//
// `files.path` is NOMINALLY the name without its extension, but nothing enforces
// that and the API accepts one that carries it ("receipt.pdf"), so concatenating
// blindly renders — and offers for download — `receipt.pdf.pdf` (#2250).
//
// Case-insensitive: a user who typed "Receipt.PDF" gets one extension, not two.
//
// The server does the same for the `Content-Disposition` header
// (filekit.DownloadName), and that is the one that DECIDES the saved filename —
// per RFC 6266 the header takes priority over an <a download> attribute. This
// helper keeps the displayed name and the download attribute honest alongside it.
export function appendExt(name: string, ext: string | undefined | null): string {
  if (!name || !ext) return name
  return name.toLowerCase().endsWith(ext.toLowerCase()) ? name : `${name}${ext}`
}
