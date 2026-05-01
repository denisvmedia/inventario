import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Maximize2, Minus, Plus, RotateCcw, X } from "lucide-react"

import { Button } from "@/components/ui/button"

const MIN_SCALE = 0.5
const MAX_SCALE = 6
const STEP = 0.25
// Wheel deltaY arrives in pixels; tame it so a normal mouse-wheel
// flick doesn't snap to MAX_SCALE. The legacy Vue viewer used the
// same coefficient.
const WHEEL_COEFFICIENT = 0.0015

// Lightweight image viewer: fullscreen <img> with mouse-wheel zoom +
// click-and-drag pan (kept off the main thread by translating in CSS,
// not React state on every mousemove). Replaces the legacy
// `frontend/src/components/ImageViewerView.vue` for the React stack;
// the keyboard arrow-nav between siblings called out in #1411 is left
// for the upcoming gallery work — this viewer takes a single signed
// URL.
export interface ImageViewerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  url: string
  alt: string
}

export function ImageViewer({ open, onOpenChange, url, alt }: ImageViewerProps) {
  const { t } = useTranslation()
  const [scale, setScale] = useState(1)
  const [offset, setOffset] = useState({ x: 0, y: 0 })
  const draggingRef = useRef<{ x: number; y: number } | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  const reset = useCallback(() => {
    setScale(1)
    setOffset({ x: 0, y: 0 })
  }, [])

  // Reset zoom + pan whenever a fresh image opens; without this, a
  // previous file's zoom state would carry over and the new image
  // would render off-centre.
  useEffect(() => {
    if (open) reset()
  }, [open, url, reset])

  // Esc closes the viewer (Dialog/Sheet primitives in this codebase
  // already trap focus + handle Esc, but this component is a plain
  // overlay so we wire it explicitly).
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onOpenChange(false)
      else if (e.key === "+" || e.key === "=") setScale((s) => clamp(s + STEP))
      else if (e.key === "-") setScale((s) => clamp(s - STEP))
      else if (e.key === "0") reset()
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [open, onOpenChange, reset])

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
        <p className="line-clamp-1 max-w-[60vw] text-sm">{alt}</p>
        <div className="flex items-center gap-1">
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
