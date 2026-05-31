import {
  type PointerEvent as ReactPointerEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react"
import { useTranslation } from "react-i18next"
import {
  ChevronLeft,
  ChevronRight,
  Download,
  Maximize,
  Minus,
  MoveHorizontal,
  PanelLeft,
  Plus,
  Rows3,
  Square,
  X,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { pdfjsLib } from "@/lib/pdfjs"

const MIN_SCALE = 0.25
const MAX_SCALE = 5
const SCALE_STEP = 0.25
const THUMB_WIDTH = 116
// Gutter subtracted from the available area before computing a fit scale so
// the page never butts against the panel edges.
const FIT_GUTTER = 32

type FitMode = "width" | "page" | null
type ViewMode = "continuous" | "paged"

function clampScale(v: number) {
  return Math.min(MAX_SCALE, Math.max(MIN_SCALE, v))
}

// Render a single pdf page into a canvas at `scale`, sharpening on HiDPI by
// drawing at scale*dpr device pixels while pinning the CSS box to `scale`.
// Returns a cleanup that cancels the in-flight render (so rapid scale/page
// changes can't run two render() calls on the same canvas).
function usePageRender(
  pdf: pdfjsLib.PDFDocumentProxy | null,
  pageNumber: number,
  scale: number,
  canvasRef: React.RefObject<HTMLCanvasElement | null>,
  enabled = true
) {
  useEffect(() => {
    if (!enabled || !pdf || !canvasRef.current) return
    let cancelled = false
    let task: pdfjsLib.RenderTask | null = null
    pdf.getPage(pageNumber).then((page) => {
      const canvas = canvasRef.current
      if (cancelled || !canvas) return
      const ctx = canvas.getContext("2d")
      if (!ctx) return
      const dpr = window.devicePixelRatio || 1
      const viewport = page.getViewport({ scale })
      const renderViewport = page.getViewport({ scale: scale * dpr })
      canvas.width = Math.floor(renderViewport.width)
      canvas.height = Math.floor(renderViewport.height)
      canvas.style.width = `${Math.floor(viewport.width)}px`
      canvas.style.height = `${Math.floor(viewport.height)}px`
      task = page.render({ canvas, canvasContext: ctx, viewport: renderViewport })
      task.promise.catch(() => {
        // Cancelled renders throw; nothing to do.
      })
    })
    return () => {
      cancelled = true
      task?.cancel()
    }
  }, [pdf, pageNumber, scale, canvasRef, enabled])
}

// jsdom (tests) has no IntersectionObserver; fall back to rendering eagerly.
const HAS_IO = typeof IntersectionObserver !== "undefined"

interface PageProps {
  pdf: pdfjsLib.PDFDocumentProxy
  pageNumber: number
  scale: number
  base: { width: number; height: number }
  root: HTMLElement | null
  registerRef?: (pageNumber: number, el: HTMLDivElement | null) => void
}

// One page in the continuous strip. Reserves its full (scaled) box up front so
// the scrollbar reflects the whole document, and only keeps a canvas rendered
// while the page is near the viewport: a page that scrolls far away is evicted
// (its canvas freed) and re-rendered on return — so a 200-page PDF never holds
// 200 full-size canvases at once. (The current-page indicator is derived from
// scroll position in the parent, Chrome-style: the page nearest the viewport
// top — it reads the always-present wrapper boxes, not the canvases, so
// eviction doesn't disturb it.)
function ContinuousPage({ pdf, pageNumber, scale, base, root, registerRef }: PageProps) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [render, setRender] = useState(!HAS_IO)

  useEffect(() => {
    registerRef?.(pageNumber, wrapRef.current)
    return () => registerRef?.(pageNumber, null)
  }, [registerRef, pageNumber])

  useEffect(() => {
    if (!HAS_IO) return
    const el = wrapRef.current
    if (!el) return
    // Toggle both ways (the 600px margin keeps a buffer above/below the
    // viewport rendered so scrolling stays smooth) so off-screen pages can be
    // evicted, not just lazily mounted.
    const io = new IntersectionObserver(
      (entries) => setRender(entries[entries.length - 1].isIntersecting),
      { root: root ?? null, rootMargin: "600px 0px" }
    )
    io.observe(el)
    return () => io.disconnect()
  }, [root])

  usePageRender(pdf, pageNumber, scale, canvasRef, render)

  // Free the bitmap once the page is evicted so its memory is reclaimed;
  // usePageRender repaints it when `render` flips back on. The wrapper keeps its
  // reserved size, so this doesn't shift layout.
  useEffect(() => {
    if (render) return
    const canvas = canvasRef.current
    if (canvas) {
      canvas.width = 0
      canvas.height = 0
    }
  }, [render])

  return (
    <div
      ref={wrapRef}
      data-page={pageNumber}
      data-testid={`pdf-full-page-${pageNumber}`}
      className="relative shrink-0 bg-white shadow-md"
      style={{ width: Math.floor(base.width * scale), height: Math.floor(base.height * scale) }}
    >
      <canvas ref={canvasRef} className="block" />
    </div>
  )
}

function PagedPage({ pdf, pageNumber, scale, base }: Omit<PageProps, "root" | "registerRef">) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  usePageRender(pdf, pageNumber, scale, canvasRef, true)
  return (
    <div
      data-testid={`pdf-full-page-${pageNumber}`}
      className="relative shrink-0 bg-white shadow-md"
      style={{ width: Math.floor(base.width * scale), height: Math.floor(base.height * scale) }}
    >
      <canvas ref={canvasRef} className="block" />
    </div>
  )
}

interface ThumbProps {
  pdf: pdfjsLib.PDFDocumentProxy
  pageNumber: number
  base: { width: number; height: number }
  active: boolean
  root: HTMLElement | null
  onClick: () => void
}

// One page-thumbnail in the rail. Like `ContinuousPage`, it reserves its box up
// front and only renders the canvas once it scrolls near the rail's viewport —
// so a 200-page PDF doesn't kick off 200 thumbnail renders the moment the
// sidebar mounts (which would undercut the lazy page rendering).
function Thumbnail({ pdf, pageNumber, base, active, root, onClick }: ThumbProps) {
  const wrapRef = useRef<HTMLButtonElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const scale = THUMB_WIDTH / base.width
  const [render, setRender] = useState(!HAS_IO)

  useEffect(() => {
    if (!HAS_IO || render) return
    const el = wrapRef.current
    if (!el) return
    const io = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) setRender(true)
      },
      { root: root ?? null, rootMargin: "200px 0px" }
    )
    io.observe(el)
    return () => io.disconnect()
  }, [root, render])

  usePageRender(pdf, pageNumber, scale, canvasRef, render)

  // Reserve the thumbnail box up front so the rail's scroll height is correct
  // before the lazy render fills it (and IntersectionObserver has a stable,
  // non-zero-height target to watch).
  const boxHeight = Math.floor(base.height * scale)

  return (
    <button
      ref={wrapRef}
      type="button"
      onClick={onClick}
      aria-current={active ? "page" : undefined}
      data-testid={`pdf-full-thumb-${pageNumber}`}
      className="flex w-full flex-col items-center gap-1 rounded-md p-2 text-xs text-muted-foreground hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <span
        className={cn(
          "overflow-hidden rounded-sm border bg-white",
          active ? "ring-2 ring-primary" : "ring-1 ring-border"
        )}
        style={{ width: THUMB_WIDTH, height: boxHeight }}
      >
        <canvas ref={canvasRef} className="block" />
      </span>
      <span className={cn("tabular-nums", active && "font-medium text-foreground")}>
        {pageNumber}
      </span>
    </button>
  )
}

export interface PdfFullViewerProps {
  url: string
  title?: string
  onClose?: () => void
}

// Fullscreen, Chrome-style PDF reader: a page-thumbnail rail, continuous-scroll
// or single-page modes, and a fit-to-width / fit-to-page control on top of
// manual zoom. Deliberately omits print / drawing / rotate. The lighter inline
// `PdfViewer` stays the in-panel preview; this is what the detail sheet's
// fullscreen expand hosts.
export function PdfFullViewer({ url, title, onClose }: PdfFullViewerProps) {
  const { t } = useTranslation()
  // State-backed (callback ref) so the ResizeObserver + IntersectionObserver
  // can use the scroll element as their root without reading a ref during
  // render (react-hooks/refs). A parallel ref carries the same node for
  // imperative scroll reads/writes (drag-to-pan), since state is immutable.
  const scrollElRef = useRef<HTMLDivElement | null>(null)
  const [rootEl, setRootEl] = useState<HTMLDivElement | null>(null)
  const setScrollEl = useCallback((el: HTMLDivElement | null) => {
    scrollElRef.current = el
    setRootEl(el)
  }, [])
  // The thumbnail rail's own scroll container, used as the IntersectionObserver
  // root so thumbnails render lazily as they scroll into the rail's viewport.
  const [sidebarEl, setSidebarEl] = useState<HTMLDivElement | null>(null)
  // Drag-to-pan bookkeeping (pointer origin + scroll offset), kept in a ref so
  // moves don't re-render the viewer.
  const panRef = useRef<{ x: number; y: number; left: number; top: number } | null>(null)
  const pageEls = useRef(new Map<number, HTMLDivElement>())

  const [pdf, setPdf] = useState<pdfjsLib.PDFDocumentProxy | null>(null)
  const [base, setBase] = useState<{ width: number; height: number } | null>(null)
  const [progress, setProgress] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)

  const [scale, setScale] = useState(1)
  const [fitMode, setFitMode] = useState<FitMode>("width")
  const [viewMode, setViewMode] = useState<ViewMode>("continuous")
  const [page, setPage] = useState(1)
  // Editable draft for the page-number field, kept separate from the committed
  // `page` so typing a multi-digit number doesn't navigate on every keystroke.
  const [pageInput, setPageInput] = useState("1")
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [area, setArea] = useState({ width: 0, height: 0 })

  const numPages = pdf?.numPages ?? 0

  // Load the document + page-1 base dimensions (used for fit math + slot sizing).
  useEffect(() => {
    let cancelled = false
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setPdf(null)
    setBase(null)
    setError(null)
    setProgress(null)
    setPage(1)
    const task = pdfjsLib.getDocument({ url })
    task.onProgress = ({ loaded, total }: { loaded: number; total: number }) => {
      if (!cancelled) setProgress(total > 0 ? Math.min(1, loaded / total) : null)
    }
    task.promise
      .then(async (doc) => {
        const first = await doc.getPage(1)
        if (cancelled) return
        const vp = first.getViewport({ scale: 1 })
        setBase({ width: vp.width, height: vp.height })
        setPdf(doc)
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message)
      })
    return () => {
      cancelled = true
      task.destroy()
    }
  }, [url])

  // Track the scroll area's size so fit-to-width / fit-to-page can react to
  // viewport + sidebar changes.
  useEffect(() => {
    if (!rootEl || typeof ResizeObserver === "undefined") return
    const ro = new ResizeObserver(() => {
      setArea({ width: rootEl.clientWidth, height: rootEl.clientHeight })
    })
    ro.observe(rootEl)
    // Initial sync measure so the first fit scale lands before the first
    // resize callback fires.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setArea({ width: rootEl.clientWidth, height: rootEl.clientHeight })
    return () => ro.disconnect()
  }, [rootEl])

  // Recompute the scale whenever a fit mode is active and the area / document
  // changes. Manual zoom clears the fit mode so it stops snapping back.
  useEffect(() => {
    if (!fitMode || !base || area.width <= 0) return
    const fitW = (area.width - FIT_GUTTER) / base.width
    const fitP = Math.min(fitW, (area.height - FIT_GUTTER) / base.height)
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setScale(clampScale(fitMode === "width" ? fitW : fitP))
  }, [fitMode, base, area])

  const registerRef = useCallback((n: number, el: HTMLDivElement | null) => {
    if (el) pageEls.current.set(n, el)
    else pageEls.current.delete(n)
  }, [])

  const goToPage = useCallback(
    (n: number) => {
      const clamped = Math.max(1, Math.min(numPages || 1, n))
      setPage(clamped)
      if (viewMode === "continuous") {
        pageEls.current.get(clamped)?.scrollIntoView({ block: "start", behavior: "smooth" })
      }
    },
    [numPages, viewMode]
  )

  // Mirror the committed page back into the editable field whenever it changes
  // from elsewhere (nav buttons, wheel-flip, the scroll indicator), so the field
  // shows the real page once a navigation settles.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setPageInput(String(page))
  }, [page])

  const commitPageInput = useCallback(() => {
    goToPage(Number(pageInput) || 1)
  }, [goToPage, pageInput])

  // Current-page indicator in continuous mode (Chrome-style): the page whose
  // top edge sits nearest the viewport top. Driven off scroll so the counter
  // tracks both wheel/scrollbar scrolling and programmatic jumps once they
  // settle. rAF-throttled to keep big documents smooth.
  useEffect(() => {
    if (!rootEl || viewMode !== "continuous" || !pdf) return
    let frame = 0
    const update = () => {
      frame = 0
      const top = rootEl.getBoundingClientRect().top
      let best = 1
      let bestDist = Infinity
      for (const [n, el] of pageEls.current) {
        const r = el.getBoundingClientRect()
        if (r.bottom <= top) continue // fully scrolled past
        const dist = Math.abs(r.top - top)
        if (dist < bestDist) {
          bestDist = dist
          best = n
        }
      }
      setPage((cur) => (cur === best ? cur : best))
    }
    const onScroll = () => {
      if (!frame) frame = requestAnimationFrame(update)
    }
    rootEl.addEventListener("scroll", onScroll, { passive: true })
    return () => {
      rootEl.removeEventListener("scroll", onScroll)
      if (frame) cancelAnimationFrame(frame)
    }
  }, [rootEl, viewMode, pdf, numPages])

  // Single-page mode: the wheel flips whole pages (one flick = one page).
  // A short cooldown stops a single momentum scroll from skipping several.
  // Native (non-passive) listener so preventDefault stops the inner scroll.
  // (A tall, zoomed-in page is still reachable via the scrollbar / drag; use
  // fit-to-page to read it whole.)
  useEffect(() => {
    if (!rootEl || viewMode !== "paged" || !pdf) return
    let accum = 0
    let lastFlip = 0
    const onWheel = (e: WheelEvent) => {
      if (e.ctrlKey) return // leave pinch-zoom alone
      e.preventDefault()
      const now = performance.now()
      if (now - lastFlip < 250) return
      accum += e.deltaY
      const THRESHOLD = 30
      if (accum >= THRESHOLD) {
        accum = 0
        lastFlip = now
        setPage((p) => Math.min(numPages, p + 1))
      } else if (accum <= -THRESHOLD) {
        accum = 0
        lastFlip = now
        setPage((p) => Math.max(1, p - 1))
      }
    }
    rootEl.addEventListener("wheel", onWheel, { passive: false })
    return () => rootEl.removeEventListener("wheel", onWheel)
  }, [rootEl, viewMode, pdf, numPages])

  const zoom = useCallback((delta: number) => {
    setFitMode(null)
    setScale((s) => clampScale(s + delta))
  }, [])

  const cycleFit = useCallback(() => {
    setFitMode((m) => (m === "width" ? "page" : "width"))
  }, [])

  // Click-and-drag panning of the page area (mouse only; touch/pen keep native
  // scrolling). Drives scrollLeft/scrollTop from the pointer delta — works in
  // both continuous and single-page modes whenever there's overflow to pan.
  function onPanStart(e: ReactPointerEvent<HTMLDivElement>) {
    if (e.pointerType !== "mouse" || e.button !== 0) return
    const el = scrollElRef.current
    if (!el) return
    if (el.scrollWidth <= el.clientWidth && el.scrollHeight <= el.clientHeight) return
    panRef.current = { x: e.clientX, y: e.clientY, left: el.scrollLeft, top: el.scrollTop }
    el.setPointerCapture(e.pointerId)
  }

  function onPanMove(e: ReactPointerEvent<HTMLDivElement>) {
    const el = scrollElRef.current
    const pan = panRef.current
    if (!el || !pan) return
    el.scrollLeft = pan.left - (e.clientX - pan.x)
    el.scrollTop = pan.top - (e.clientY - pan.y)
  }

  function onPanEnd(e: ReactPointerEvent<HTMLDivElement>) {
    const el = scrollElRef.current
    if (el?.hasPointerCapture(e.pointerId)) el.releasePointerCapture(e.pointerId)
    panRef.current = null
  }

  const zoomPercent = Math.round(scale * 100)

  return (
    <div className="flex h-full min-h-0 flex-col bg-muted/30" data-testid="pdf-full-viewer">
      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-1.5 border-b bg-background px-3 py-2 text-sm">
        <Button
          variant="ghost"
          size="icon"
          onClick={() => setSidebarOpen((v) => !v)}
          aria-pressed={sidebarOpen}
          aria-label={t("files:viewer.toggleThumbnails", {
            defaultValue: "Toggle page thumbnails",
          })}
          data-testid="pdf-full-sidebar-toggle"
        >
          <PanelLeft className="size-4" aria-hidden="true" />
        </Button>
        {title ? (
          <span className="line-clamp-1 max-w-[28ch] text-sm font-medium" title={title}>
            {title}
          </span>
        ) : null}

        <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" />

        <Button
          variant="ghost"
          size="icon"
          onClick={() => goToPage(page - 1)}
          disabled={page <= 1}
          aria-label={t("files:viewer.prevPage", { defaultValue: "Previous page" })}
          data-testid="pdf-full-prev"
        >
          <ChevronLeft className="size-4" aria-hidden="true" />
        </Button>
        <span className="tabular-nums" data-testid="pdf-full-page-indicator">
          <input
            type="number"
            min={1}
            max={numPages || 1}
            value={pageInput}
            onChange={(e) => setPageInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                commitPageInput()
                e.currentTarget.blur()
              }
            }}
            onBlur={commitPageInput}
            aria-label={t("files:viewer.pageNumber", { defaultValue: "Page number" })}
            data-testid="pdf-full-page-input"
            className="w-12 rounded border bg-background px-1 py-0.5 text-center tabular-nums [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none"
          />
          <span className="px-1 text-muted-foreground">/ {numPages || "—"}</span>
        </span>
        <Button
          variant="ghost"
          size="icon"
          onClick={() => goToPage(page + 1)}
          disabled={page >= numPages}
          aria-label={t("files:viewer.nextPage", { defaultValue: "Next page" })}
          data-testid="pdf-full-next"
        >
          <ChevronRight className="size-4" aria-hidden="true" />
        </Button>

        <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" />

        <Button
          variant="ghost"
          size="icon"
          onClick={() => zoom(-SCALE_STEP)}
          aria-label={t("files:viewer.zoomOut", { defaultValue: "Zoom out" })}
          data-testid="pdf-full-zoom-out"
        >
          <Minus className="size-4" aria-hidden="true" />
        </Button>
        <span className="min-w-12 text-center tabular-nums" data-testid="pdf-full-zoom-level">
          {zoomPercent}%
        </span>
        <Button
          variant="ghost"
          size="icon"
          onClick={() => zoom(SCALE_STEP)}
          aria-label={t("files:viewer.zoomIn", { defaultValue: "Zoom in" })}
          data-testid="pdf-full-zoom-in"
        >
          <Plus className="size-4" aria-hidden="true" />
        </Button>
        <Button
          variant={fitMode ? "secondary" : "ghost"}
          size="icon"
          onClick={cycleFit}
          aria-pressed={!!fitMode}
          aria-label={
            fitMode === "page"
              ? t("files:viewer.fitWidth", { defaultValue: "Fit to width" })
              : t("files:viewer.fitPage", { defaultValue: "Fit to page" })
          }
          title={
            fitMode === "page"
              ? t("files:viewer.fitWidth", { defaultValue: "Fit to width" })
              : t("files:viewer.fitPage", { defaultValue: "Fit to page" })
          }
          data-testid="pdf-full-fit"
        >
          {fitMode === "page" ? (
            <MoveHorizontal className="size-4" aria-hidden="true" />
          ) : (
            <Maximize className="size-4" aria-hidden="true" />
          )}
        </Button>
        <Button
          variant="ghost"
          size="icon"
          onClick={() => setViewMode((m) => (m === "continuous" ? "paged" : "continuous"))}
          aria-pressed={viewMode === "paged"}
          aria-label={
            viewMode === "continuous"
              ? t("files:viewer.singlePageView", { defaultValue: "Single page view" })
              : t("files:viewer.continuousView", { defaultValue: "Continuous scroll" })
          }
          title={
            viewMode === "continuous"
              ? t("files:viewer.singlePageView", { defaultValue: "Single page view" })
              : t("files:viewer.continuousView", { defaultValue: "Continuous scroll" })
          }
          data-testid="pdf-full-mode"
        >
          {viewMode === "continuous" ? (
            <Square className="size-4" aria-hidden="true" />
          ) : (
            <Rows3 className="size-4" aria-hidden="true" />
          )}
        </Button>

        <span className="ml-auto" />
        <Button asChild variant="outline" size="sm" data-testid="pdf-full-download">
          <a href={url} download>
            <Download className="mr-2 size-4" aria-hidden="true" />
            {t("files:detail.download")}
          </a>
        </Button>
        {onClose ? (
          <Button variant="outline" size="sm" onClick={onClose} data-testid="pdf-full-close">
            <X className="mr-2 size-4" aria-hidden="true" />
            {t("files:viewer.close", { defaultValue: "Close" })}
          </Button>
        ) : null}
      </div>

      {/* Body: thumbnail rail + page area */}
      <div className="flex min-h-0 flex-1">
        {sidebarOpen && pdf && base ? (
          <div
            ref={setSidebarEl}
            className="w-36 shrink-0 overflow-y-auto border-r bg-background"
            data-testid="pdf-full-sidebar"
          >
            <ul className="flex flex-col gap-1 p-2">
              {Array.from({ length: numPages }, (_, i) => i + 1).map((n) => (
                <li key={n}>
                  <Thumbnail
                    pdf={pdf}
                    pageNumber={n}
                    base={base}
                    active={n === page}
                    root={sidebarEl}
                    onClick={() => goToPage(n)}
                  />
                </li>
              ))}
            </ul>
          </div>
        ) : null}

        <div
          ref={setScrollEl}
          className="min-w-0 flex-1 cursor-grab overflow-auto active:cursor-grabbing"
          onPointerDown={onPanStart}
          onPointerMove={onPanMove}
          onPointerUp={onPanEnd}
          onPointerCancel={onPanEnd}
          data-testid="pdf-full-scroll"
        >
          {error ? (
            <div
              className="flex h-full items-center justify-center p-6 text-sm text-destructive"
              data-testid="pdf-full-error"
            >
              {t("files:detail.previewLoadError")}
            </div>
          ) : !pdf || !base ? (
            <div
              className="flex h-full flex-col items-center justify-center gap-3 p-6 text-sm text-muted-foreground"
              data-testid="pdf-full-loading"
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
                  style={
                    progress === null ? undefined : { width: `${Math.round(progress * 100)}%` }
                  }
                  data-testid="pdf-full-progress-bar"
                />
              </div>
            </div>
          ) : viewMode === "continuous" ? (
            <div className="flex min-w-fit flex-col items-center gap-4 p-4">
              {Array.from({ length: numPages }, (_, i) => i + 1).map((n) => (
                <ContinuousPage
                  key={n}
                  pdf={pdf}
                  pageNumber={n}
                  scale={scale}
                  base={base}
                  root={rootEl}
                  registerRef={registerRef}
                />
              ))}
            </div>
          ) : (
            <div className="flex min-w-fit justify-center p-4">
              <PagedPage pdf={pdf} pageNumber={page} scale={scale} base={base} />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
