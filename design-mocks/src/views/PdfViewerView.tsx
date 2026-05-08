import { useState, useEffect } from "react"
import {
  X,
  ZoomIn,
  ZoomOut,
  ChevronLeft,
  ChevronRight,
  Download,
  Share2,
  Search,
  RotateCw,
  FileText,
  Tag,
  Calendar,
  HardDrive,
  BookOpen,
  Info,
  Trash2,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
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

interface PdfFile {
  name: string
  item: string
  category: string
  size: string
  pages: number
  date: string
}

const DEMO_PDF: PdfFile = {
  name: "Samsung_Fridge_Manual.pdf",
  item: "Samsung Refrigerator RF23M8590SG",
  category: "Manuals",
  size: "4.2 MB",
  pages: 12,
  date: "Jun 20, 2020",
}

const PAGE_CONTENT = [
  { title: "Safety Instructions", subtitle: "Important safety information before use", type: "cover" },
  { title: "Product Overview", subtitle: "Components and features", type: "text" },
  { title: "Installation Guide", subtitle: "Step-by-step setup", type: "text" },
  { title: "Control Panel", subtitle: "Display and button functions", type: "diagram" },
  { title: "Temperature Settings", subtitle: "Optimal cooling configurations", type: "table" },
  { title: "Water Filter", subtitle: "DA29-00020B replacement guide", type: "text" },
  { title: "Ice Maker", subtitle: "Setup and maintenance", type: "text" },
  { title: "Cleaning Guide", subtitle: "Interior and exterior care", type: "text" },
  { title: "Troubleshooting", subtitle: "Error codes and solutions", type: "table" },
  { title: "Error Codes", subtitle: "Complete error reference", type: "table" },
  { title: "Warranty", subtitle: "Coverage terms and conditions", type: "text" },
  { title: "Specifications", subtitle: "Technical product data", type: "table" },
]

interface PdfViewerViewProps {
  onClose?: () => void
  file?: { name: string; size: string; uploadedAt?: string }
  onDelete?: () => void
}

export function PdfViewerView({ onClose, file: fileProp, onDelete }: PdfViewerViewProps) {
  const [page, setPage] = useState(1)
  const [zoom, setZoom] = useState(100)
  const [rotation, setRotation] = useState(0)
  const [showInfo, setShowInfo] = useState(false)
  const [showSearch, setShowSearch] = useState(false)
  const [searchQuery, setSearchQuery] = useState("")
  const [thumbnailsOpen, setThumbnailsOpen] = useState(false)
  const isMobile = useIsMobile()

  const totalPages = DEMO_PDF.pages
  const currentPage = PAGE_CONTENT[page - 1]

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose?.()
    }
    document.addEventListener("keydown", handleKeyDown)
    return () => document.removeEventListener("keydown", handleKeyDown)
  }, [onClose])

  function goTo(p: number) {
    setPage(Math.max(1, Math.min(totalPages, p)))
  }

  const infoContent = (
    <div className="p-4 space-y-5">
      <div>
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-3">Document</p>
        <div className="space-y-2.5">
          {[
            { icon: HardDrive, label: "File size", value: DEMO_PDF.size },
            { icon: BookOpen, label: "Pages", value: `${DEMO_PDF.pages} pages` },
            { icon: Calendar, label: "Date", value: DEMO_PDF.date },
            { icon: Tag, label: "Category", value: DEMO_PDF.category },
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
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">Table of contents</p>
        <nav className="space-y-0.5">
          {PAGE_CONTENT.map((pg, i) => (
            <button
              key={i}
              onClick={() => { goTo(i + 1); if (isMobile) setShowInfo(false) }}
              className={cn(
                "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition-colors",
                page === i + 1 ? "bg-accent font-medium" : "text-muted-foreground hover:bg-muted hover:text-foreground"
              )}
            >
              <span className="w-4 text-[10px] tabular-nums shrink-0">{i + 1}</span>
              <span className="truncate">{pg.title}</span>
            </button>
          ))}
        </nav>
      </div>

      <Separator />
      <div>
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">Linked item</p>
        <div className="rounded-lg border border-border bg-muted/30 p-3">
          <p className="text-xs font-medium leading-tight">{DEMO_PDF.item}</p>
          <Badge variant="secondary" className="mt-1.5 text-xs">{DEMO_PDF.category}</Badge>
        </div>
      </div>

      <div className="space-y-1.5">
        <Button variant="outline" size="sm" className="w-full gap-1.5 justify-start">
          <Download className="size-3.5" />
          Download PDF
        </Button>
      </div>
    </div>
  )

  return (
    <div className="flex flex-col h-screen bg-background">
      {/* Topbar */}
      <div className="flex h-12 items-center gap-2 border-b border-border px-4 shrink-0">
        <Button variant="ghost" size="icon" className="size-8" onClick={onClose}>
          <X className="size-4" />
        </Button>
        <Separator orientation="vertical" className="h-4" />

        <div className="flex items-center gap-2 min-w-0 flex-1">
          <FileText className="size-4 text-status-expired shrink-0" />
          <div className="min-w-0">
            <p className="text-sm font-medium truncate">{fileProp?.name ?? DEMO_PDF.name}</p>
          </div>
          {!isMobile && (
            <Badge variant="secondary" className="text-xs shrink-0">{DEMO_PDF.category}</Badge>
          )}
        </div>

        <div className="flex items-center gap-1 ml-auto">
          {/* Search toggle — always visible */}
          <Button
            variant="ghost" size="icon" className="size-8"
            onClick={() => setShowSearch((v) => !v)}
          >
            <Search className={cn("size-4", showSearch ? "text-primary" : "text-muted-foreground")} />
          </Button>
          {/* Thumbnails toggle — always visible */}
          <Button variant="ghost" size="icon" className="size-8" onClick={() => setThumbnailsOpen((v) => !v)}>
            <BookOpen className={cn("size-4", thumbnailsOpen ? "text-primary" : "text-muted-foreground")} />
          </Button>
          <Button variant="ghost" size="icon" className="size-8" onClick={() => setShowInfo((v) => !v)}>
            <Info className={cn("size-4", showInfo ? "text-primary" : "text-muted-foreground")} />
          </Button>
          <Button variant="ghost" size="icon" className="size-8">
            <Download className="size-4" />
          </Button>
          {!isMobile && (
            <Button variant="ghost" size="icon" className="size-8">
              <Share2 className="size-4" />
            </Button>
          )}
          {onDelete && (
            <Button variant="ghost" size="icon" className="size-8 text-destructive hover:bg-destructive/10" onClick={onDelete}>
              <Trash2 className="size-4" />
            </Button>
          )}
        </div>
      </div>

      {/* Search bar — collapsible */}
      {showSearch && (
        <div className="border-b border-border px-4 py-2 bg-muted/30 shrink-0">
          <div className="flex items-center gap-2 max-w-sm">
            <div className="relative flex-1">
              <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="Search in document…"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-8 h-7 text-xs"
                autoFocus
              />
            </div>
            {searchQuery && (
              <span className="text-xs text-muted-foreground">0 results</span>
            )}
          </div>
        </div>
      )}

      <div className="flex flex-1 min-h-0 overflow-hidden">
        {/* Thumbnail sidebar — desktop only inline, mobile Sheet */}
        {!isMobile && thumbnailsOpen && (
          <aside className="w-32 shrink-0 border-r border-border bg-muted/20 overflow-y-auto py-2 px-2">
            <div className="space-y-1.5">
              {PAGE_CONTENT.map((pg, i) => (
                <button
                  key={i}
                  onClick={() => goTo(i + 1)}
                  className={cn(
                    "w-full rounded-lg border overflow-hidden text-left transition-all",
                    page === i + 1 ? "border-primary shadow-sm" : "border-border hover:border-muted-foreground/40"
                  )}
                >
                  <div className="bg-white p-1.5 min-h-16 flex flex-col gap-1">
                    <div className="h-1.5 rounded-full bg-muted w-3/4" />
                    <div className="h-1 rounded-full bg-muted w-1/2" />
                    {pg.type === "table" && (
                      <div className="mt-1 space-y-0.5">
                        {[...Array(3)].map((_, j) => (
                          <div key={j} className="flex gap-0.5">
                            <div className="h-1 rounded-sm bg-muted flex-1" />
                            <div className="h-1 rounded-sm bg-muted flex-1" />
                          </div>
                        ))}
                      </div>
                    )}
                    {pg.type === "diagram" && (
                      <div className="mt-1 flex items-center justify-center">
                        <div className="size-5 rounded border border-muted" />
                      </div>
                    )}
                    {pg.type === "cover" && (
                      <div className="mt-0.5 h-2 rounded bg-muted/60 w-full" />
                    )}
                  </div>
                  <div className="bg-muted/40 px-1 py-0.5">
                    <p className="text-[9px] text-muted-foreground text-center">{i + 1}</p>
                  </div>
                </button>
              ))}
            </div>
          </aside>
        )}

        {/* Page canvas */}
        <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
          <div className={cn(
            "flex-1 bg-muted/20 overflow-auto",
            isMobile ? "flex flex-col p-0" : "flex items-center justify-center p-6"
          )}>
            <div
              className={cn(
                "relative bg-white overflow-hidden transition-transform duration-200",
                isMobile ? "w-full rounded-none shadow-none flex-1" : "rounded-lg shadow-xl max-w-[480px] w-full"
              )}
              style={{
                transform: `scale(${zoom / 100}) rotate(${rotation}deg)`,
                minHeight: isMobile ? "100%" : "640px",
              }}
            >
              {/* PDF page content simulation */}
              <div className="p-6 sm:p-8 h-full flex flex-col gap-4">
                <div className="flex items-start justify-between gap-4 pb-4 border-b border-gray-200">
                  <div>
                    <div className="w-24 h-6 rounded bg-gray-200 mb-1.5" />
                    <p className="text-xs text-gray-400">{DEMO_PDF.item}</p>
                  </div>
                  <div className="text-right">
                    <p className="text-[10px] text-gray-400">Page {page} of {totalPages}</p>
                  </div>
                </div>

                <div>
                  <h2 className="text-base font-bold text-gray-800">{currentPage.title}</h2>
                  <p className="text-xs text-gray-500 mt-0.5">{currentPage.subtitle}</p>
                </div>

                {currentPage.type === "cover" && (
                  <div className="flex-1 flex flex-col gap-3 mt-2">
                    <div className="h-2 rounded-full bg-gray-100 w-full" />
                    <div className="h-2 rounded-full bg-gray-100 w-5/6" />
                    <div className="h-2 rounded-full bg-gray-100 w-full" />
                    <div className="mt-2 rounded-lg bg-amber-50 border border-amber-200 p-3">
                      <div className="h-2 rounded-full bg-amber-200 w-3/4 mb-1.5" />
                      <div className="h-1.5 rounded-full bg-amber-100 w-full" />
                      <div className="h-1.5 rounded-full bg-amber-100 w-4/5 mt-1" />
                    </div>
                    <div className="h-2 rounded-full bg-gray-100 w-full" />
                    <div className="h-2 rounded-full bg-gray-100 w-2/3" />
                  </div>
                )}

                {currentPage.type === "text" && (
                  <div className="flex-1 flex flex-col gap-2 mt-2">
                    {[...Array(8)].map((_, i) => (
                      <div key={i} className={`h-2 rounded-full bg-gray-100 ${
                        i % 4 === 3 ? "w-2/3" : i % 3 === 2 ? "w-5/6" : "w-full"
                      }`} />
                    ))}
                    <div className="my-2" />
                    <div className="rounded-lg bg-blue-50 border border-blue-100 p-3 space-y-1.5">
                      <div className="h-2 rounded-full bg-blue-200 w-1/2" />
                      <div className="h-1.5 rounded-full bg-blue-100 w-full" />
                      <div className="h-1.5 rounded-full bg-blue-100 w-4/5" />
                    </div>
                    {[...Array(5)].map((_, i) => (
                      <div key={i} className={`h-2 rounded-full bg-gray-100 ${i % 2 ? "w-4/5" : "w-full"}`} />
                    ))}
                  </div>
                )}

                {currentPage.type === "table" && (
                  <div className="flex-1 mt-2">
                    <div className="rounded-lg border border-gray-200 overflow-hidden">
                      <div className="grid grid-cols-3 gap-0 bg-gray-100">
                        {["Code", "Description", "Action"].map((h) => (
                          <div key={h} className="px-2 py-1.5 border-r last:border-r-0 border-gray-200">
                            <p className="text-[10px] font-semibold text-gray-600">{h}</p>
                          </div>
                        ))}
                      </div>
                      {[...Array(6)].map((_, i) => (
                        <div key={i} className={`grid grid-cols-3 gap-0 border-t border-gray-100 ${i % 2 ? "bg-gray-50" : "bg-white"}`}>
                          {[...Array(3)].map((_, j) => (
                            <div key={j} className="px-2 py-2 border-r last:border-r-0 border-gray-100">
                              <div className={`h-1.5 rounded-full bg-gray-200 ${j === 1 ? "w-full" : "w-2/3"}`} />
                            </div>
                          ))}
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {currentPage.type === "diagram" && (
                  <div className="flex-1 flex items-center justify-center mt-2">
                    <div className="relative w-48 sm:w-56 h-48 sm:h-56 rounded-xl border-2 border-gray-200 bg-gray-50 flex items-center justify-center">
                      <div className="absolute inset-4 rounded-lg border border-gray-200 bg-white flex items-center justify-center">
                        <div className="text-3xl opacity-20">🖥</div>
                      </div>
                      {[
                        { top: "10%", left: "-20%", text: "A" },
                        { top: "40%", left: "-20%", text: "B" },
                        { top: "70%", right: "-20%", text: "C" },
                        { top: "20%", right: "-20%", text: "D" },
                      ].map((l, i) => (
                        <div key={i} className="absolute flex items-center gap-1" style={{ top: l.top, left: l.left, right: l.right }}>
                          <div className="size-4 rounded-full border border-gray-400 bg-white flex items-center justify-center">
                            <span className="text-[9px] font-semibold text-gray-500">{l.text}</span>
                          </div>
                          <div className="h-px w-4 bg-gray-300" />
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                <div className="pt-3 border-t border-gray-100 flex justify-between">
                  <p className="text-[9px] text-gray-300">{DEMO_PDF.name}</p>
                  <p className="text-[9px] text-gray-300">{page}</p>
                </div>
              </div>
            </div>
          </div>

          {/* Navigation + zoom toolbar */}
          <div className="border-t border-border bg-background shrink-0">
            {/* Page navigation row */}
            <div className="flex items-center justify-center gap-2 px-4 py-2">
              <Button variant="ghost" size="icon" className="size-8" onClick={() => goTo(page - 1)} disabled={page <= 1}>
                <ChevronLeft className="size-4" />
              </Button>
              <div className="flex items-center gap-1.5">
                <Input
                  type="number"
                  value={page}
                  onChange={(e) => goTo(parseInt(e.target.value) || 1)}
                  className="h-7 w-12 text-center text-sm [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                  min={1}
                  max={totalPages}
                />
                <span className="text-sm text-muted-foreground">/ {totalPages}</span>
              </div>
              <Button variant="ghost" size="icon" className="size-8" onClick={() => goTo(page + 1)} disabled={page >= totalPages}>
                <ChevronRight className="size-4" />
              </Button>

              {/* Zoom controls — inline on desktop, after separator; hidden on mobile until expanded */}
              {!isMobile && (
                <>
                  <Separator orientation="vertical" className="h-4 mx-1" />
                  <Button variant="ghost" size="icon" className="size-8" onClick={() => setZoom((z) => Math.max(25, z - 25))} disabled={zoom <= 25}>
                    <ZoomOut className="size-4" />
                  </Button>
                  <span className="w-12 text-center text-sm font-medium tabular-nums">{zoom}%</span>
                  <Button variant="ghost" size="icon" className="size-8" onClick={() => setZoom((z) => Math.min(200, z + 25))} disabled={zoom >= 200}>
                    <ZoomIn className="size-4" />
                  </Button>
                  <Button variant="ghost" size="icon" className="size-8" onClick={() => setRotation((r) => (r + 90) % 360)}>
                    <RotateCw className="size-4" />
                  </Button>
                </>
              )}

              {/* Mobile: compact zoom buttons */}
              {isMobile && (
                <>
                  <Separator orientation="vertical" className="h-4 mx-1" />
                  <Button variant="ghost" size="icon" className="size-8" onClick={() => setZoom((z) => Math.max(25, z - 25))} disabled={zoom <= 25}>
                    <ZoomOut className="size-4" />
                  </Button>
                  <span className="w-10 text-center text-xs font-medium tabular-nums text-muted-foreground">{zoom}%</span>
                  <Button variant="ghost" size="icon" className="size-8" onClick={() => setZoom((z) => Math.min(200, z + 25))} disabled={zoom >= 200}>
                    <ZoomIn className="size-4" />
                  </Button>
                </>
              )}
            </div>
          </div>
        </div>

        {/* Info panel — desktop sidebar, mobile Sheet */}
        {isMobile ? (
          <Sheet open={showInfo} onOpenChange={setShowInfo}>
            <SheetContent side="bottom" className="max-h-[80vh] overflow-y-auto rounded-t-2xl px-0">
              <SheetHeader className="px-4 pb-2">
                <SheetTitle className="text-sm">Document info</SheetTitle>
              </SheetHeader>
              {infoContent}
            </SheetContent>
          </Sheet>
        ) : (
          showInfo && (
            <aside className="w-56 shrink-0 border-l border-border bg-card overflow-y-auto">
              {infoContent}
            </aside>
          )
        )}

        {/* Mobile: thumbnails Sheet */}
        {isMobile && (
          <Sheet open={thumbnailsOpen} onOpenChange={setThumbnailsOpen}>
            <SheetContent side="bottom" className="max-h-[60vh] rounded-t-2xl px-0">
              <SheetHeader className="px-4 pb-2">
                <SheetTitle className="text-sm">Pages</SheetTitle>
              </SheetHeader>
              <div className="overflow-x-auto px-4 pb-4">
                <div className="flex gap-2 w-max">
                  {PAGE_CONTENT.map((pg, i) => (
                    <button
                      key={i}
                      onClick={() => { goTo(i + 1); setThumbnailsOpen(false) }}
                      className={cn(
                        "w-20 rounded-lg border overflow-hidden text-left shrink-0 transition-all",
                        page === i + 1 ? "border-primary shadow-sm" : "border-border"
                      )}
                    >
                      <div className="bg-white p-1.5 h-24 flex flex-col gap-1">
                        <div className="h-1.5 rounded-full bg-muted w-3/4" />
                        <div className="h-1 rounded-full bg-muted w-1/2" />
                        <div className="mt-1 text-[8px] text-muted-foreground leading-tight line-clamp-2">{pg.title}</div>
                      </div>
                      <div className="bg-muted/40 px-1 py-0.5">
                        <p className="text-[9px] text-muted-foreground text-center">{i + 1}</p>
                      </div>
                    </button>
                  ))}
                </div>
              </div>
            </SheetContent>
          </Sheet>
        )}
      </div>
    </div>
  )
}
