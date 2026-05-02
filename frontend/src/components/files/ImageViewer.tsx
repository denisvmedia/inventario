import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { ChevronLeft, ChevronRight, Maximize2, Minus, Plus, RotateCcw, X } from "lucide-react"

import { Button } from "@/components/ui/button"

const MIN_SCALE = 0.5
const MAX_SCALE = 6
const STEP = 0.25
// Wheel deltaY arrives in pixels; tame it so a normal mouse-wheel
// flick doesn't snap to MAX_SCALE. The legacy Vue viewer used the
// same coefficient.
const WHEEL_COEFFICIENT = 0.0015

// One image in the gallery navigator — id is used by the parent to map
// the view-changed notification back to a route param so deep-links
// stay in sync.
export interface GalleryImage {
  id: string
  url: string
  alt: string
}

// Lightweight image viewer: fullscreen <img> with mouse-wheel zoom +
// click-and-drag pan (kept off the main thread by translating in CSS,
// not React state on every mousemove). Replaces the legacy
// `frontend/src/components/ImageViewerView.vue` for the React stack.
//
// Two callable shapes:
//   1. Single — pass `url` + `alt`. The arrow keys are no-ops.
//   2. Gallery — pass `siblings` + `index` + `onIndexChange`. ←/→
//      cycle through siblings (with wrap-around), the toolbar surfaces
//      previous/next buttons, and the parent gets notified so it can
//      sync the route or its own selection state.
export type ImageViewerProps =
  | {
      open: boolean
      onOpenChange: (open: boolean) => void
      url: string
      alt: string
      siblings?: never
      index?: never
      onIndexChange?: never
    }
  | {
      open: boolean
      onOpenChange: (open: boolean) => void
      url?: never
      alt?: never
      siblings: GalleryImage[]
      index: number
      onIndexChange?: (next: number) => void
    }

export function ImageViewer(props: ImageViewerProps) {
  const { open, onOpenChange } = props
  const isGallery = "siblings" in props && Array.isArray(props.siblings)
  const siblings = isGallery ? (props.siblings as GalleryImage[]) : []
  const index = isGallery ? (props.index as number) : 0
  const safeIndex = siblings.length > 0 ? Math.max(0, Math.min(index, siblings.length - 1)) : 0
  const current = isGallery ? siblings[safeIndex] : null
  const url = current?.url ?? props.url ?? ""
  const alt = current?.alt ?? props.alt ?? ""
  const { t } = useTranslation()
  const [scale, setScale] = useState(1)
  const [offset, setOffset] = useState({ x: 0, y: 0 })
  const draggingRef = useRef<{ x: number; y: number } | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  const reset = useCallback(() => {
    setScale(1)
    setOffset({ x: 0, y: 0 })
  }, [])

  const onIndexChange = isGallery ? props.onIndexChange : undefined
  const goPrev = useCallback(() => {
    if (!isGallery || siblings.length === 0) return
    const next = (safeIndex - 1 + siblings.length) % siblings.length
    onIndexChange?.(next)
  }, [isGallery, siblings.length, safeIndex, onIndexChange])
  const goNext = useCallback(() => {
    if (!isGallery || siblings.length === 0) return
    const next = (safeIndex + 1) % siblings.length
    onIndexChange?.(next)
  }, [isGallery, siblings.length, safeIndex, onIndexChange])

  // Reset zoom + pan whenever a fresh image opens; without this, a
  // previous file's zoom state would carry over and the new image
  // would render off-centre. URL is the dependency because gallery
  // navigation swaps the underlying url without unmounting.
  useEffect(() => {
    if (open) reset()
  }, [open, url, reset])

  // Esc closes the viewer (Dialog/Sheet primitives in this codebase
  // already trap focus + handle Esc, but this component is a plain
  // overlay so we wire it explicitly). Arrow keys cycle through the
  // gallery when one is provided.
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onOpenChange(false)
      else if (e.key === "+" || e.key === "=") setScale((s) => clamp(s + STEP))
      else if (e.key === "-") setScale((s) => clamp(s - STEP))
      else if (e.key === "0") reset()
      else if (e.key === "ArrowLeft") goPrev()
      else if (e.key === "ArrowRight") goNext()
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [open, onOpenChange, reset, goPrev, goNext])

  if (!open) return null

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={alt}
      data-testid="file-image-viewer"
      className="fixed inset-0 z-50 flex flex-col bg-black/90 text-white"
    >
      <div className="flex items-center justify-between gap-2 p-3">
        <p className="line-clamp-1 max-w-[60vw] text-sm">
          {alt}
          {isGallery && siblings.length > 1 ? (
            <span
              className="ml-2 text-xs text-white/60 tabular-nums"
              data-testid="image-viewer-position"
            >
              {safeIndex + 1} / {siblings.length}
            </span>
          ) : null}
        </p>
        <div className="flex items-center gap-1">
          {isGallery && siblings.length > 1 ? (
            <>
              <Button
                variant="ghost"
                size="icon"
                className="text-white hover:bg-white/10 hover:text-white"
                onClick={goPrev}
                aria-label={t("files:viewer.prevImage", { defaultValue: "Previous image" })}
                data-testid="image-viewer-prev"
              >
                <ChevronLeft className="size-4" aria-hidden="true" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="text-white hover:bg-white/10 hover:text-white"
                onClick={goNext}
                aria-label={t("files:viewer.nextImage", { defaultValue: "Next image" })}
                data-testid="image-viewer-next"
              >
                <ChevronRight className="size-4" aria-hidden="true" />
              </Button>
              <span className="mx-1 h-4 w-px bg-white/20" aria-hidden="true" />
            </>
          ) : null}
          <Button
            variant="ghost"
            size="icon"
            className="text-white hover:text-white hover:bg-white/10"
            onClick={() => setScale((s) => clamp(s - STEP))}
            aria-label={t("files:viewer.zoomOut", { defaultValue: "Zoom out" })}
            data-testid="image-viewer-zoom-out"
          >
            <Minus className="size-4" aria-hidden="true" />
          </Button>
          <span
            className="min-w-12 text-center text-xs tabular-nums"
            data-testid="image-viewer-zoom-level"
          >
            {Math.round(scale * 100)}%
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="text-white hover:text-white hover:bg-white/10"
            onClick={() => setScale((s) => clamp(s + STEP))}
            aria-label={t("files:viewer.zoomIn", { defaultValue: "Zoom in" })}
            data-testid="image-viewer-zoom-in"
          >
            <Plus className="size-4" aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="text-white hover:text-white hover:bg-white/10"
            onClick={reset}
            aria-label={t("files:viewer.reset", { defaultValue: "Reset zoom" })}
            data-testid="image-viewer-reset"
          >
            <RotateCcw className="size-4" aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="text-white hover:text-white hover:bg-white/10"
            onClick={() => containerRef.current?.requestFullscreen?.()}
            aria-label={t("files:viewer.fullscreen", { defaultValue: "Fullscreen" })}
            data-testid="image-viewer-fullscreen"
          >
            <Maximize2 className="size-4" aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="text-white hover:text-white hover:bg-white/10"
            onClick={() => onOpenChange(false)}
            aria-label={t("files:viewer.close", { defaultValue: "Close" })}
            data-testid="image-viewer-close"
          >
            <X className="size-5" aria-hidden="true" />
          </Button>
        </div>
      </div>
      {/* The viewport is mouse-only sugar — keyboard zoom (+/-/0) and Esc
          are bound at the document level above, the toolbar covers the
          accessible action set, so the static-div interactions are an
          intentional enhancement, not the only path. */}
      {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions */}
      <div
        ref={containerRef}
        className="flex flex-1 cursor-grab items-center justify-center overflow-hidden active:cursor-grabbing"
        onWheel={(e) => {
          e.preventDefault()
          setScale((s) => clamp(s - e.deltaY * WHEEL_COEFFICIENT * s))
        }}
        onMouseDown={(e) => {
          draggingRef.current = { x: e.clientX - offset.x, y: e.clientY - offset.y }
        }}
        onMouseMove={(e) => {
          if (!draggingRef.current) return
          setOffset({
            x: e.clientX - draggingRef.current.x,
            y: e.clientY - draggingRef.current.y,
          })
        }}
        onMouseUp={() => {
          draggingRef.current = null
        }}
        onMouseLeave={() => {
          draggingRef.current = null
        }}
      >
        <img
          src={url}
          alt={alt}
          draggable={false}
          data-testid="image-viewer-img"
          className="max-h-none max-w-none select-none"
          style={{
            transform: `translate(${offset.x}px, ${offset.y}px) scale(${scale})`,
            transformOrigin: "center center",
            transition: draggingRef.current ? "none" : "transform 80ms ease-out",
          }}
        />
      </div>
    </div>
  )
}

function clamp(v: number) {
  return Math.min(MAX_SCALE, Math.max(MIN_SCALE, v))
}
