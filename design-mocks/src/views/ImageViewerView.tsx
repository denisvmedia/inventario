import { useState, useEffect } from "react"
import {
  X,
  ZoomIn,
  ZoomOut,
  RotateCw,
  Download,
  Share2,
  ChevronLeft,
  ChevronRight,
  Maximize2,
  Info,
  Tag,
  Calendar,
  HardDrive,
  Trash2,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { useIsMobile } from "@/hooks/use-mobile"
import { cn } from "@/lib/utils"

interface ImageFile {
  id: string
  name: string
  item: string
  category: string
  size: string
  dimensions: string
  date: string
  color: string
}

const DEMO_IMAGES: ImageFile[] = [
  {
    id: "1",
    name: "Washing_Machine_photo.jpg",
    item: "Miele Washing Machine",
    category: "Photos",
    size: "1.8 MB",
    dimensions: "3024 × 4032",
    date: "Mar 16, 2022",
    color: "from-slate-200 to-slate-400",
  },
  {
    id: "2",
    name: "DeWalt_Drill_photo.jpg",
    item: "DeWalt Drill",
    category: "Photos",
    size: "2.1 MB",
    dimensions: "4032 × 3024",
    date: "Aug 14, 2019",
    color: "from-yellow-200 to-yellow-500",
  },
  {
    id: "3",
    name: "Sony_WH_Photo.jpg",
    item: "Sony WH-1000XM5",
    category: "Photos",
    size: "980 KB",
    dimensions: "2400 × 1600",
    date: "Sep 1, 2022",
    color: "from-zinc-700 to-zinc-900",
  },
]

interface ImageViewerViewProps {
  initialImageId?: string
  onClose?: () => void
  file?: { name: string; size: string; uploadedAt?: string }
  onDelete?: () => void
}

export function ImageViewerView({ initialImageId, onClose, file: _fileProp, onDelete }: ImageViewerViewProps) {
  const [currentIndex, setCurrentIndex] = useState(
    DEMO_IMAGES.findIndex((i) => i.id === (initialImageId ?? "1"))
  )
  const [zoom, setZoom] = useState(100)
  const [rotation, setRotation] = useState(0)
  const [showInfo, setShowInfo] = useState(false)
  const [fullscreen, setFullscreen] = useState(false)
  const isMobile = useIsMobile()

  const image = DEMO_IMAGES[currentIndex]

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose?.()
    }
    document.addEventListener("keydown", handleKeyDown)
    return () => document.removeEventListener("keydown", handleKeyDown)
  }, [onClose])

  function prev() {
    setCurrentIndex((i) => (i - 1 + DEMO_IMAGES.length) % DEMO_IMAGES.length)
    setZoom(100)
    setRotation(0)
  }
  function next() {
    setCurrentIndex((i) => (i + 1) % DEMO_IMAGES.length)
    setZoom(100)
    setRotation(0)
  }

  const infoContent = (
    <div className="p-4 space-y-5">
      <div>
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">Image info</p>
        <div className="space-y-2.5">
          {[
            { icon: HardDrive, label: "File size", value: image.size },
            { icon: Maximize2, label: "Dimensions", value: image.dimensions },
            { icon: Calendar, label: "Date", value: image.date },
            { icon: Tag, label: "Category", value: image.category },
          ].map(({ icon: Icon, label, value }) => (
            <div key={label} className="flex items-start gap-2.5">
              <Icon className="size-3.5 mt-0.5 text-muted-foreground shrink-0" />
              <div>
                <p className="text-[11px] text-muted-foreground">{label}</p>
                <p className="text-xs font-medium">{value}</p>
              </div>
            </div>
          ))}
        </div>
      </div>

      <Separator />
      <div>
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">Linked item</p>
        <div className="rounded-lg border border-border bg-muted/30 p-3">
          <p className="text-sm font-medium">{image.item}</p>
          <Badge variant="secondary" className="mt-1.5 text-xs">{image.category}</Badge>
        </div>
      </div>

      <Separator />
      <div className="space-y-1.5">
        <Button variant="outline" size="sm" className="w-full gap-1.5 justify-start">
          <Download className="size-3.5" />
          Download
        </Button>
        {!isMobile && (
          <Button variant="outline" size="sm" className="w-full gap-1.5 justify-start">
            <Share2 className="size-3.5" />
            Share link
          </Button>
        )}
      </div>

      <div className="text-center">
        <p className="text-xs text-muted-foreground">
          {currentIndex + 1} of {DEMO_IMAGES.length} images
        </p>
      </div>
    </div>
  )

  return (
    <div className={cn("flex flex-col bg-background", fullscreen ? "fixed inset-0 z-50" : "h-screen")}>
      {/* Topbar */}
      <div className="flex h-12 items-center gap-2 border-b border-border px-4 shrink-0">
        <Button variant="ghost" size="icon" className="size-8" onClick={onClose}>
          <X className="size-4" />
        </Button>
        <Separator orientation="vertical" className="h-4" />
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium truncate">{image.name}</p>
          {!isMobile && <p className="text-xs text-muted-foreground">{image.item}</p>}
        </div>
        <div className="flex items-center gap-1 ml-auto">
          <Button variant="ghost" size="icon" className="size-8" onClick={() => setShowInfo((v) => !v)}>
            <Info className={cn("size-4", showInfo ? "text-primary" : "text-muted-foreground")} />
          </Button>
          {!isMobile && (
            <Button variant="ghost" size="icon" className="size-8">
              <Share2 className="size-4" />
            </Button>
          )}
          <Button variant="ghost" size="icon" className="size-8">
            <Download className="size-4" />
          </Button>
          {onDelete && (
            <Button variant="ghost" size="icon" className="size-8 text-destructive hover:bg-destructive/10" onClick={onDelete}>
              <Trash2 className="size-4" />
            </Button>
          )}
          {!isMobile && (
            <Button variant="ghost" size="icon" className="size-8" onClick={() => setFullscreen((v) => !v)}>
              <Maximize2 className="size-4" />
            </Button>
          )}
        </div>
      </div>

      <div className="flex flex-1 min-h-0 overflow-hidden">
        {/* Main viewer */}
        <div className="flex-1 flex flex-col min-w-0">
          {/* Canvas */}
          <div className="flex-1 flex items-center justify-center bg-muted/30 relative overflow-hidden">
            {/* Previous button */}
            <button
              onClick={prev}
              className="absolute left-3 z-10 flex size-9 items-center justify-center rounded-full bg-background/80 border border-border shadow-sm backdrop-blur-sm hover:bg-background transition-colors"
            >
              <ChevronLeft className="size-5" />
            </button>

            {/* Image placeholder */}
            <div
              className="relative overflow-hidden rounded-lg shadow-2xl transition-all duration-200"
              style={{
                transform: `scale(${zoom / 100}) rotate(${rotation}deg)`,
                aspectRatio: rotation % 180 !== 0 ? "3/4" : "4/3",
                maxWidth: isMobile ? "calc(100% - 80px)" : "min(480px, 90%)",
                maxHeight: isMobile ? "calc(100% - 24px)" : undefined,
                width: isMobile ? "auto" : "480px",
                height: isMobile ? "100%" : "auto",
              }}
            >
              <div className={cn("absolute inset-0 bg-gradient-to-br", image.color)} />
              <div className="absolute inset-0 flex items-center justify-center">
                <div className="text-5xl opacity-30">
                  {currentIndex === 0 ? "🫧" : currentIndex === 1 ? "🔧" : "🎧"}
                </div>
              </div>
              <div className="absolute inset-0 opacity-20 mix-blend-overlay"
                style={{ backgroundImage: "url(\"data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)' opacity='1'/%3E%3C/svg%3E\")" }}
              />
            </div>

            {/* Next button */}
            <button
              onClick={next}
              className="absolute right-3 z-10 flex size-9 items-center justify-center rounded-full bg-background/80 border border-border shadow-sm backdrop-blur-sm hover:bg-background transition-colors"
            >
              <ChevronRight className="size-5" />
            </button>
          </div>

          {/* Zoom / thumbnail toolbar */}
          <div className="flex items-center justify-center gap-1.5 border-t border-border px-3 py-2 bg-background shrink-0">
            <Button
              variant="ghost" size="icon" className="size-8"
              onClick={() => setZoom((z) => Math.max(25, z - 25))}
              disabled={zoom <= 25}
            >
              <ZoomOut className="size-4" />
            </Button>
            <span className="w-12 text-center text-sm font-medium tabular-nums">{zoom}%</span>
            <Button
              variant="ghost" size="icon" className="size-8"
              onClick={() => setZoom((z) => Math.min(300, z + 25))}
              disabled={zoom >= 300}
            >
              <ZoomIn className="size-4" />
            </Button>
            <Separator orientation="vertical" className="h-4 mx-0.5" />
            <Button
              variant="ghost" size="icon" className="size-8"
              onClick={() => setRotation((r) => (r + 90) % 360)}
            >
              <RotateCw className="size-4" />
            </Button>
            {!isMobile && (
              <Button
                variant="ghost" size="sm" className="text-xs"
                onClick={() => { setZoom(100); setRotation(0) }}
              >
                Reset
              </Button>
            )}
            {isMobile && (zoom !== 100 || rotation !== 0) && (
              <Button
                variant="ghost" size="sm" className="text-xs"
                onClick={() => { setZoom(100); setRotation(0) }}
              >
                Reset
              </Button>
            )}

            {/* Thumbnails — desktop only */}
            {!isMobile && (
              <div className="ml-4 flex gap-1.5">
                {DEMO_IMAGES.map((img, i) => (
                  <button
                    key={img.id}
                    onClick={() => { setCurrentIndex(i); setZoom(100); setRotation(0) }}
                    className={cn(
                      "size-8 rounded overflow-hidden border-2 transition-all",
                      i === currentIndex ? "border-primary" : "border-transparent opacity-50 hover:opacity-80"
                    )}
                  >
                    <div className={cn("w-full h-full bg-gradient-to-br", img.color)} />
                  </button>
                ))}
              </div>
            )}

            {/* Mobile: dot indicators */}
            {isMobile && (
              <div className="ml-3 flex items-center gap-1.5">
                {DEMO_IMAGES.map((_, i) => (
                  <button
                    key={i}
                    onClick={() => { setCurrentIndex(i); setZoom(100); setRotation(0) }}
                    className={cn(
                      "rounded-full transition-all",
                      i === currentIndex
                        ? "size-2 bg-primary"
                        : "size-1.5 bg-muted-foreground/40 hover:bg-muted-foreground/70"
                    )}
                  />
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Info panel — desktop: sidebar, mobile: Sheet */}
        {isMobile ? (
          <Sheet open={showInfo} onOpenChange={setShowInfo}>
            <SheetContent side="bottom" className="max-h-[70vh] overflow-y-auto rounded-t-2xl px-0 pb-safe">
              <SheetHeader className="px-4 pb-2">
                <SheetTitle className="text-sm">Image info</SheetTitle>
              </SheetHeader>
              {infoContent}
            </SheetContent>
          </Sheet>
        ) : (
          showInfo && (
            <aside className="w-60 shrink-0 border-l border-border bg-card overflow-y-auto">
              {infoContent}
            </aside>
          )
        )}
      </div>
    </div>
  )
}
