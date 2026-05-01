import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { ChevronLeft, ChevronRight, Download, Minus, Plus } from "lucide-react"

import { Button } from "@/components/ui/button"
import { pdfjsLib } from "@/lib/pdfjs"

const MIN_SCALE = 0.5
const MAX_SCALE = 3
const SCALE_STEP = 0.25
const DEFAULT_SCALE = 1.5

// pdfjs-dist canvas viewer ported from the legacy
// `frontend/src/components/PDFViewerCanvas.vue`. Supports paged
// navigation, zoom in/out, and download. Multi-page "view all"
// rendering from the legacy is intentionally omitted — keeps memory
// bounded for 200-page PDFs and matches the more common single-page
// reading flow on the React side.
export interface PdfViewerProps {
  url: string
  onError?: (err: Error) => void
}

export function PdfViewer({ url, onError }: PdfViewerProps) {
  const { t } = useTranslation()
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [pdf, setPdf] = useState<pdfjsLib.PDFDocumentProxy | null>(null)
  const [page, setPage] = useState(1)
  const [scale, setScale] = useState(DEFAULT_SCALE)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Load the document once per URL change; pdfjs's getDocument returns
  // a worker-backed proxy so a swapped URL needs a fresh handle.
  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    const task = pdfjsLib.getDocument({ url })
    task.promise
      .then((doc) => {
        if (cancelled) return
        setPdf(doc)
        setPage(1)
        setLoading(false)
      })
      .catch((err: Error) => {
        if (cancelled) return
        setError(err.message)
        setLoading(false)
        onError?.(err)
      })
    return () => {
      cancelled = true
      task.destroy()
    }
  }, [url, onError])

  // Render the current page whenever pdf / page / scale changes. The
  // canvas keeps its DOM identity so the browser doesn't repaint twice
  // (clear → fill); pdfjs renders directly into the existing element.
  useEffect(() => {
    if (!pdf || !canvasRef.current) return
    let cancelled = false
    const canvas = canvasRef.current
    const ctx = canvas.getContext("2d")
    if (!ctx) return
    pdf.getPage(page).then((pageProxy) => {
      if (cancelled) return
      const viewport = pageProxy.getViewport({ scale })
      canvas.width = viewport.width
      canvas.height = viewport.height
      const renderTask = pageProxy.render({ canvas, canvasContext: ctx, viewport })
      renderTask.promise.catch(() => {
        // Cancelled renders throw; nothing to do.
      })
    })
    return () => {
      cancelled = true
    }
  }, [pdf, page, scale])

  const numPages = pdf?.numPages ?? 0

  return (
    <div className="flex flex-col gap-2" data-testid="file-pdf-viewer">
      <div className="flex flex-wrap items-center gap-2 rounded-md border bg-muted/40 p-2 text-sm">
        <Button
          variant="outline"
          size="sm"
          onClick={() => setPage((p) => Math.max(1, p - 1))}
          disabled={page <= 1 || loading}
          aria-label={t("files:viewer.prevPage", { defaultValue: "Previous page" })}
          data-testid="pdf-viewer-prev"
        >
          <ChevronLeft className="size-4" aria-hidden="true" />
        </Button>
        <span className="tabular-nums" data-testid="pdf-viewer-page-info">
          {loading ? "—" : `${page} / ${numPages}`}
        </span>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setPage((p) => Math.min(numPages, p + 1))}
          disabled={page >= numPages || loading}
          aria-label={t("files:viewer.nextPage", { defaultValue: "Next page" })}
          data-testid="pdf-viewer-next"
        >
          <ChevronRight className="size-4" aria-hidden="true" />
        </Button>
        <span className="mx-2 h-4 w-px bg-border" aria-hidden="true" />
        <Button
          variant="outline"
          size="sm"
          onClick={() => setScale((s) => Math.max(MIN_SCALE, s - SCALE_STEP))}
          disabled={loading}
          aria-label={t("files:viewer.zoomOut", { defaultValue: "Zoom out" })}
          data-testid="pdf-viewer-zoom-out"
        >
          <Minus className="size-4" aria-hidden="true" />
        </Button>
        <span className="min-w-12 text-center tabular-nums" data-testid="pdf-viewer-zoom-level">
          {Math.round(scale * 100)}%
        </span>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setScale((s) => Math.min(MAX_SCALE, s + SCALE_STEP))}
          disabled={loading}
          aria-label={t("files:viewer.zoomIn", { defaultValue: "Zoom in" })}
          data-testid="pdf-viewer-zoom-in"
        >
          <Plus className="size-4" aria-hidden="true" />
        </Button>
        <span className="ml-auto" />
        <Button asChild variant="outline" size="sm" data-testid="pdf-viewer-download">
          <a href={url} download>
            <Download className="mr-2 size-4" aria-hidden="true" />
            {t("files:detail.download")}
          </a>
        </Button>
      </div>
      {loading ? (
        <div className="flex aspect-[4/5] w-full items-center justify-center rounded-md border text-sm text-muted-foreground">
          {t("common:loading", { defaultValue: "Loading…" })}
        </div>
      ) : error ? (
        <div
          className="flex aspect-[4/5] w-full items-center justify-center rounded-md border bg-muted/40 text-sm text-destructive"
          data-testid="pdf-viewer-error"
        >
          {t("files:detail.previewLoadError")}
        </div>
      ) : (
        <div className="overflow-auto rounded-md border bg-muted/40 p-2">
          <canvas ref={canvasRef} className="mx-auto block max-w-full" />
        </div>
      )}
    </div>
  )
}
