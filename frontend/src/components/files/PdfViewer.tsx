import { type PointerEvent as ReactPointerEvent, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { ChevronLeft, ChevronRight, Download, Maximize2, Minus, Plus } from "lucide-react"

import { Button } from "@/components/ui/button"
import { pdfjsLib } from "@/lib/pdfjs"

const MIN_SCALE = 0.5
const MAX_SCALE = 3
const SCALE_STEP = 0.25
// Inline panel preview opens at 100% (the side-panel default); zoom in/out
// from there. The fullscreen reader has its own fit-to-width default.
const DEFAULT_SCALE = 1

// pdfjs-dist canvas viewer ported from the legacy
// `frontend/src/components/PDFViewerCanvas.vue`. Supports paged
// navigation, zoom in/out, and download. Multi-page "view all"
// rendering from the legacy is intentionally omitted — keeps memory
// bounded for 200-page PDFs and matches the more common single-page
// reading flow on the React side.
export interface PdfViewerProps {
  url: string
  onError?: (err: Error) => void
  // When provided, the toolbar surfaces a fullscreen affordance. The
  // file-detail sheet passes this on its inline viewer to pop the PDF into a
  // fullscreen dialog (the dialog's own viewer omits it, so there's no
  // recursive button).
  onRequestFullscreen?: () => void
}

export function PdfViewer({ url, onError, onRequestFullscreen }: PdfViewerProps) {
  const { t } = useTranslation()
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  // Drag-to-pan bookkeeping: pointer origin + scroll offset captured on
  // pointerdown. Kept in a ref so the move handler doesn't re-render the
  // viewer on every mouse move.
  const panRef = useRef<{ x: number; y: number; left: number; top: number } | null>(null)
  const [pdf, setPdf] = useState<pdfjsLib.PDFDocumentProxy | null>(null)
  const [page, setPage] = useState(1)
  const [scale, setScale] = useState(DEFAULT_SCALE)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  // Download progress in [0, 1] while the document streams in, or null when
  // the total size is unknown (some servers omit Content-Length) — the bar
  // falls back to an indeterminate pulse in that case.
  const [progress, setProgress] = useState<number | null>(null)

  // Load the document once per URL change; pdfjs's getDocument returns
  // a worker-backed proxy so a swapped URL needs a fresh handle.
  useEffect(() => {
    let cancelled = false
    // Effect synchronises a fresh load with an external system (pdfjs);
    // resetting the loading + error state up front is part of that sync.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLoading(true)
    setError(null)
    setProgress(null)
    const task = pdfjsLib.getDocument({ url })
    // pdf.js streams the document and reports byte progress; surface it so
    // the user sees a determinate loading bar like a browser's native viewer.
    task.onProgress = ({ loaded, total }: { loaded: number; total: number }) => {
      if (cancelled) return
      setProgress(total > 0 ? Math.min(1, loaded / total) : null)
    }
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
    let renderTask: pdfjsLib.RenderTask | null = null
    const canvas = canvasRef.current
    const ctx = canvas.getContext("2d")
    if (!ctx) return
    pdf.getPage(page).then((pageProxy) => {
      if (cancelled) return
      // Render at device-pixel resolution for crispness on HiDPI screens,
      // but pin the canvas's *displayed* size to the scale-only viewport so
      // the on-screen size tracks the zoom indicator 1:1. The CSS size is
      // what overflows the surrounding `overflow-auto` container, so zooming
      // in genuinely enlarges the page and the result can be scrolled.
      const dpr = window.devicePixelRatio || 1
      const viewport = pageProxy.getViewport({ scale })
      const renderViewport = pageProxy.getViewport({ scale: scale * dpr })
      canvas.width = Math.floor(renderViewport.width)
      canvas.height = Math.floor(renderViewport.height)
      canvas.style.width = `${Math.floor(viewport.width)}px`
      canvas.style.height = `${Math.floor(viewport.height)}px`
      renderTask = pageProxy.render({ canvas, canvasContext: ctx, viewport: renderViewport })
      renderTask.promise.catch(() => {
        // Cancelled renders throw; nothing to do.
      })
    })
    // Cancel any in-flight render on cleanup so a rapid scale/page change
    // can't start a second render() while the previous one still owns the
    // canvas — pdf.js otherwise throws "Cannot use the same canvas during
    // multiple render operations" (swallowed above, leaving stale output).
    return () => {
      cancelled = true
      renderTask?.cancel()
    }
  }, [pdf, page, scale])

  // Click-and-drag panning of a zoomed page. The canvas overflows the
  // `overflow-auto` container, so panning is just driving scrollLeft/scrollTop
  // from the pointer delta (a "grab hand"). Mouse only — touch/pen keep the
  // browser's native momentum scrolling.
  function onPanStart(e: ReactPointerEvent<HTMLDivElement>) {
    if (e.pointerType !== "mouse" || e.button !== 0) return
    const el = scrollRef.current
    if (!el) return
    // Nothing to pan to while the page fits — leave clicks alone.
    if (el.scrollWidth <= el.clientWidth && el.scrollHeight <= el.clientHeight) return
    panRef.current = { x: e.clientX, y: e.clientY, left: el.scrollLeft, top: el.scrollTop }
    el.setPointerCapture(e.pointerId)
  }

  function onPanMove(e: ReactPointerEvent<HTMLDivElement>) {
    const el = scrollRef.current
    const pan = panRef.current
    if (!el || !pan) return
    el.scrollLeft = pan.left - (e.clientX - pan.x)
    el.scrollTop = pan.top - (e.clientY - pan.y)
  }

  function onPanEnd(e: ReactPointerEvent<HTMLDivElement>) {
    const el = scrollRef.current
    if (el?.hasPointerCapture(e.pointerId)) el.releasePointerCapture(e.pointerId)
    panRef.current = null
  }

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
        {onRequestFullscreen ? (
          <Button
            variant="outline"
            size="sm"
            onClick={onRequestFullscreen}
            disabled={loading}
            aria-label={t("files:viewer.fullscreen", { defaultValue: "Fullscreen" })}
            data-testid="pdf-viewer-fullscreen"
          >
            <Maximize2 className="size-4" aria-hidden="true" />
          </Button>
        ) : null}
        <Button asChild variant="outline" size="sm" data-testid="pdf-viewer-download">
          <a href={url} download>
            <Download className="mr-2 size-4" aria-hidden="true" />
            {t("files:detail.download")}
          </a>
        </Button>
      </div>
      {loading ? (
        <div
          className="flex aspect-[4/5] w-full flex-col items-center justify-center gap-3 rounded-md border text-sm text-muted-foreground"
          data-testid="pdf-viewer-loading"
        >
          <span>{t("common:loading", { defaultValue: "Loading…" })}</span>
          <div
            className="h-1.5 w-40 overflow-hidden rounded-full bg-muted"
            role="progressbar"
            aria-label={t("common:loading", { defaultValue: "Loading…" })}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={progress === null ? undefined : Math.round(progress * 100)}
          >
            <div
              className={
                progress === null
                  ? "h-full w-1/3 animate-pulse rounded-full bg-primary"
                  : "h-full rounded-full bg-primary transition-[width] duration-150 ease-out"
              }
              style={progress === null ? undefined : { width: `${Math.round(progress * 100)}%` }}
              data-testid="pdf-viewer-progress-bar"
            />
          </div>
          {progress !== null ? (
            <span className="tabular-nums text-xs" data-testid="pdf-viewer-progress-label">
              {Math.round(progress * 100)}%
            </span>
          ) : null}
        </div>
      ) : error ? (
        <div
          className="flex aspect-[4/5] w-full items-center justify-center rounded-md border bg-muted/40 text-sm text-destructive"
          data-testid="pdf-viewer-error"
        >
          {t("files:detail.previewLoadError")}
        </div>
      ) : (
        <div
          ref={scrollRef}
          className="cursor-grab overflow-auto rounded-md border bg-muted/40 p-2 active:cursor-grabbing"
          onPointerDown={onPanStart}
          onPointerMove={onPanMove}
          onPointerUp={onPanEnd}
          onPointerCancel={onPanEnd}
          data-testid="pdf-viewer-scroll"
        >
          {/* `min-w-fit` keeps this row at least as wide as the canvas so an
              over-container zoom overflows into the scroll area instead of
              being clamped; `justify-center` centres the page while it still
              fits, and because the row's left edge starts at the container's
              left edge the start of a zoomed page stays scrollable. The grab
              cursor + pointer handlers add click-and-drag panning so the parts
              that don't fit on screen can be dragged into view. */}
          <div className="flex min-w-fit justify-center">
            <canvas ref={canvasRef} className="block" data-testid="pdf-viewer-canvas" />
          </div>
        </div>
      )}
    </div>
  )
}
