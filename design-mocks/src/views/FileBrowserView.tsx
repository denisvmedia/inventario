import { useState, useEffect, useRef, useCallback } from "react"
import { FileText, Image as ImageIcon, FileArchive, File, Upload, Search, LayoutGrid, List, X, Receipt, BookOpen, CloudUpload, CircleCheck as CheckCircle2, ChevronLeft, ChevronRight } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useIsMobile } from "@/hooks/use-mobile"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationPrevious,
  PaginationNext,
  PaginationEllipsis,
} from "@/components/ui/pagination"
import { FileDetailSheet } from "@/components/FileDetail"
import { cn } from "@/lib/utils"
import { MOCK_FILES, FILE_TAGS, resolveTags, type AttachedFile, type FileCategory } from "@/data/mock"
import { TagPill } from "@/components/TagPill"

const PAGE_SIZE = 12

type BrowserCategory = "all" | FileCategory

const CATEGORY_TABS: {
  id: BrowserCategory
  label: string
  icon: React.ElementType
  description: string
  color: string
  bg: string
}[] = [
  {
    id: "all",
    label: "All Files",
    icon: File,
    description: "Every file attached to your inventory",
    color: "text-muted-foreground",
    bg: "bg-muted",
  },
  {
    id: "image",
    label: "Photos",
    icon: ImageIcon,
    description: "Item photos — shown on cards and in galleries",
    color: "text-status-active",
    bg: "bg-status-active/10",
  },
  {
    id: "invoice",
    label: "Invoices",
    icon: Receipt,
    description: "Purchase receipts for insurance and reports",
    color: "text-chart-1",
    bg: "bg-chart-1/10",
  },
  {
    id: "document",
    label: "Documents",
    icon: BookOpen,
    description: "Manuals, warranties, certificates",
    color: "text-chart-3",
    bg: "bg-chart-3/10",
  },
  {
    id: "other",
    label: "Other",
    icon: FileArchive,
    description: "Backups and miscellaneous files",
    color: "text-chart-4",
    bg: "bg-chart-4/10",
  },
]

type MimeGroup = "pdf" | "image" | "archive" | "other"

function mimeGroup(mimeType: string): MimeGroup {
  if (mimeType === "application/pdf") return "pdf"
  if (mimeType.startsWith("image/")) return "image"
  if (mimeType.includes("zip") || mimeType.includes("archive")) return "archive"
  return "other"
}

const FILE_ICONS: Record<MimeGroup, React.ElementType> = {
  pdf: FileText,
  image: ImageIcon,
  archive: FileArchive,
  other: File,
}

const FILE_COLORS: Record<MimeGroup, string> = {
  pdf: "text-status-expired",
  image: "text-status-active",
  archive: "text-chart-4",
  other: "text-muted-foreground",
}

const FILE_BG: Record<MimeGroup, string> = {
  pdf: "bg-status-expired/10",
  image: "bg-status-active/10",
  archive: "bg-chart-4/10",
  other: "bg-muted",
}

type UploadStep = "select" | "metadata" | "uploading" | "done"

interface PendingFile {
  id: string
  file: File
  // metadata
  name: string
  category: FileCategory
  tags: string[]
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function guessCategoryFromMime(mimeType: string): FileCategory {
  if (mimeType.startsWith("image/")) return "image"
  if (mimeType === "application/pdf") return "invoice"
  return "other"
}

function mimeIconAndColor(mimeType: string): { icon: React.ElementType; color: string; bg: string } {
  if (mimeType.startsWith("image/")) return { icon: ImageIcon, color: "text-status-active", bg: "bg-status-active/10" }
  if (mimeType === "application/pdf") return { icon: FileText, color: "text-status-expired", bg: "bg-status-expired/10" }
  if (mimeType.includes("zip") || mimeType.includes("archive")) return { icon: FileArchive, color: "text-chart-4", bg: "bg-chart-4/10" }
  return { icon: File, color: "text-muted-foreground", bg: "bg-muted" }
}

const UPLOAD_CATEGORY_TABS: { id: FileCategory; label: string; icon: React.ElementType; color: string; bg: string }[] = [
  { id: "image",    label: "Photo",    icon: ImageIcon,  color: "text-status-active", bg: "bg-status-active/10" },
  { id: "invoice",  label: "Invoice",  icon: Receipt,    color: "text-chart-1",        bg: "bg-chart-1/10" },
  { id: "document", label: "Document", icon: BookOpen,   color: "text-chart-3",        bg: "bg-chart-3/10" },
  { id: "other",    label: "Other",    icon: FileArchive,color: "text-chart-4",        bg: "bg-chart-4/10" },
]

interface UploadDialogProps {
  open: boolean
  onClose: () => void
}

function UploadDialog({ open, onClose }: UploadDialogProps) {
  const [step, setStep] = useState<UploadStep>("select")
  const [pendingFiles, setPendingFiles] = useState<PendingFile[]>([])
  const [metaIndex, setMetaIndex] = useState(0)
  const [isDragging, setIsDragging] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  function addFiles(files: FileList | File[]) {
    const arr = Array.from(files)
    setPendingFiles((prev) => [
      ...prev,
      ...arr.map((f) => ({
        id: crypto.randomUUID(),
        file: f,
        name: f.name.replace(/\.[^.]+$/, ""),
        category: guessCategoryFromMime(f.type),
        tags: [],
      })),
    ])
  }

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(false)
    if (e.dataTransfer.files.length) addFiles(e.dataTransfer.files)
  }, [])

  function removeFile(id: string) {
    setPendingFiles((prev) => prev.filter((f) => f.id !== id))
  }

  function updateMeta(id: string, patch: Partial<Pick<PendingFile, "name" | "category" | "tags">>) {
    setPendingFiles((prev) => prev.map((f) => f.id === id ? { ...f, ...patch } : f))
  }

  function toggleTag(fileId: string, tagId: string) {
    const file = pendingFiles.find((f) => f.id === fileId)
    if (!file) return
    const next = file.tags.includes(tagId)
      ? file.tags.filter((t) => t !== tagId)
      : [...file.tags, tagId]
    updateMeta(fileId, { tags: next })
  }

  function handleContinue() {
    setMetaIndex(0)
    setStep("metadata")
  }

  function handleUpload() {
    setStep("uploading")
    setTimeout(() => setStep("done"), 1500)
  }

  function handleClose() {
    setPendingFiles([])
    setStep("select")
    setMetaIndex(0)
    onClose()
  }

  const currentFile = pendingFiles[metaIndex]

  // ── Step: Select files ────────────────────────────────────────
  if (step === "select") {
    return (
      <Dialog open={open} onOpenChange={(o) => !o && handleClose()}>
        <DialogContent className="sm:max-w-md gap-0 p-0 overflow-hidden max-w-[calc(100vw-2rem)]">
          <DialogHeader className="px-6 pt-6 pb-4 overflow-hidden">
            <DialogTitle>Upload files</DialogTitle>
            <p className="text-sm text-muted-foreground mt-0.5">
              Add photos, invoices, and documents to your inventory.
            </p>
          </DialogHeader>

          <div className="px-6 pb-2 flex flex-col gap-3 min-w-0 overflow-hidden">
            <div
              onDrop={handleDrop}
              onDragOver={(e) => { e.preventDefault(); setIsDragging(true) }}
              onDragLeave={() => setIsDragging(false)}
              onClick={() => inputRef.current?.click()}
              className={cn(
                "flex flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed py-8 transition-all cursor-pointer",
                isDragging
                  ? "border-primary bg-primary/5 scale-[1.01]"
                  : "border-border bg-muted/30 hover:border-primary/50 hover:bg-muted/50"
              )}
            >
              <div className={cn("flex size-12 items-center justify-center rounded-xl transition-colors", isDragging ? "bg-primary/10" : "bg-muted")}>
                <CloudUpload className={cn("size-6 transition-colors", isDragging ? "text-primary" : "text-muted-foreground")} strokeWidth={1.5} />
              </div>
              <div className="text-center">
                <p className="text-sm font-medium">{isDragging ? "Drop files here" : "Drop files here"}</p>
                <p className="text-xs text-muted-foreground mt-0.5">
                  or <span className="text-foreground underline underline-offset-2">browse</span> to choose
                </p>
              </div>
              <p className="text-xs text-muted-foreground">Images, PDFs, documents up to 50 MB</p>
            </div>

            <input ref={inputRef} type="file" multiple className="sr-only"
              onChange={(e) => e.target.files && addFiles(e.target.files)} />

            {pendingFiles.length > 0 && (
              <div className="flex flex-col rounded-lg border border-border bg-card overflow-hidden max-h-52 overflow-y-auto min-w-0">
                {pendingFiles.map((item, i) => {
                  const { icon: FileIcon, color, bg } = mimeIconAndColor(item.file.type)
                  return (
                    <div key={item.id} className="min-w-0">
                      {i > 0 && <Separator />}
                      <div className="flex items-center gap-3 px-3 py-2.5 min-w-0">
                        <div className={cn("flex size-8 shrink-0 items-center justify-center rounded-lg", bg)}>
                          <FileIcon className={cn("size-4", color)} />
                        </div>
                        <div className="flex-1 min-w-0 overflow-hidden">
                          <p className="text-sm font-medium truncate">{item.name || item.file.name}</p>
                          <p className="text-xs text-muted-foreground">{formatFileSize(item.file.size)}</p>
                        </div>
                        <button type="button" onClick={() => removeFile(item.id)}
                          className="text-muted-foreground hover:text-foreground transition-colors shrink-0">
                          <X className="size-4" />
                        </button>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          <DialogFooter className="px-6 py-4 border-t border-border gap-2">
            <Button variant="outline" onClick={handleClose}>Cancel</Button>
            <Button disabled={!pendingFiles.length} onClick={handleContinue} className="gap-1.5">
              Continue
              <ChevronRight className="size-4" />
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    )
  }

  // ── Step: Metadata per file ───────────────────────────────────
  if (step === "metadata" && currentFile) {
    const { icon: FileIcon, color, bg } = mimeIconAndColor(currentFile.file.type)
    const isFirst = metaIndex === 0
    const isLast = metaIndex === pendingFiles.length - 1

    return (
      <Dialog open={open} onOpenChange={(o) => !o && handleClose()}>
        <DialogContent className="sm:max-w-md gap-0 p-0 overflow-hidden max-w-[calc(100vw-2rem)]">
          <DialogHeader className="px-6 pt-6 pb-4 overflow-hidden">
            <div className="flex items-center justify-between gap-2 min-w-0">
              <DialogTitle className="truncate">File details</DialogTitle>
              <span className="text-xs text-muted-foreground tabular-nums">
                {metaIndex + 1} / {pendingFiles.length}
              </span>
            </div>
            <p className="text-sm text-muted-foreground mt-0.5">
              Add metadata to help organise this file in your inventory.
            </p>
          </DialogHeader>

          <div className="px-6 pb-4 flex flex-col gap-4 min-w-0 overflow-hidden">
            {/* File preview row */}
            <div className={cn("flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-3 py-2.5 overflow-hidden")}>
              <div className={cn("flex size-9 shrink-0 items-center justify-center rounded-lg", bg)}>
                <FileIcon className={cn("size-5", color)} />
              </div>
              <div className="flex-1 min-w-0 overflow-hidden">
                <p className="text-sm font-medium truncate">{currentFile.file.name}</p>
                <p className="text-xs text-muted-foreground">{formatFileSize(currentFile.file.size)}</p>
              </div>
            </div>

            {/* Name */}
            <div className="flex flex-col gap-1.5 min-w-0">
              <Label htmlFor="upload-name">Display name</Label>
              <Input
                id="upload-name"
                value={currentFile.name}
                onChange={(e) => updateMeta(currentFile.id, { name: e.target.value })}
                placeholder="Enter a display name…"
                className="w-full"
              />
            </div>

            {/* Category */}
            <div className="flex flex-col gap-1.5">
              <Label>Category</Label>
              <div className="grid grid-cols-4 gap-1.5">
                {UPLOAD_CATEGORY_TABS.map((cat) => {
                  const CatIcon = cat.icon
                  const active = currentFile.category === cat.id
                  return (
                    <button
                      key={cat.id}
                      type="button"
                      onClick={() => updateMeta(currentFile.id, { category: cat.id })}
                      className={cn(
                        "flex flex-col items-center gap-1.5 rounded-lg border py-2.5 px-1 text-xs font-medium transition-colors",
                        active
                          ? "border-primary bg-primary/5 text-foreground"
                          : "border-border bg-background text-muted-foreground hover:border-primary/40"
                      )}
                    >
                      <CatIcon className={cn("size-4", active ? cat.color : "text-muted-foreground")} />
                      {cat.label}
                    </button>
                  )
                })}
              </div>
            </div>

            {/* Tags */}
            <div className="flex flex-col gap-1.5">
              <Label>Tags</Label>
              <div className="flex flex-wrap gap-1.5">
                {FILE_TAGS.map((tag) => {
                  const active = currentFile.tags.includes(tag.id)
                  return (
                    <button
                      key={tag.id}
                      type="button"
                      onClick={() => toggleTag(currentFile.id, tag.id)}
                      className={cn(
                        "rounded-full border px-2.5 py-1 text-xs font-medium transition-colors",
                        active
                          ? "border-primary bg-primary text-primary-foreground"
                          : "border-border bg-background text-muted-foreground hover:border-primary/50"
                      )}
                    >
                      {tag.label}
                    </button>
                  )
                })}
              </div>
            </div>
          </div>

          <DialogFooter className="px-6 py-4 border-t border-border gap-2">
            <Button variant="outline" onClick={() => isFirst ? setStep("select") : setMetaIndex((i) => i - 1)} className="gap-1.5">
              <ChevronLeft className="size-4" />
              {isFirst ? "Back" : "Previous"}
            </Button>
            <div className="flex-1" />
            {!isLast ? (
              <Button onClick={() => setMetaIndex((i) => i + 1)} className="gap-1.5">
                Next
                <ChevronRight className="size-4" />
              </Button>
            ) : (
              <Button onClick={handleUpload} className="gap-1.5">
                <Upload className="size-4" />
                Upload {pendingFiles.length} file{pendingFiles.length !== 1 ? "s" : ""}
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    )
  }

  // ── Step: Uploading / Done ────────────────────────────────────
  return (
    <Dialog open={open} onOpenChange={(o) => !o && handleClose()}>
      <DialogContent className="sm:max-w-md gap-0 p-0 overflow-hidden">
        <DialogHeader className="px-6 pt-6 pb-4">
          <DialogTitle>{step === "done" ? "Upload complete" : "Uploading…"}</DialogTitle>
        </DialogHeader>

        <div className="px-6 pb-4 flex flex-col gap-1 max-h-72 overflow-y-auto">
          {pendingFiles.map((item, i) => {
            const { icon: FileIcon, color, bg } = mimeIconAndColor(item.file.type)
            return (
              <div key={item.id}>
                {i > 0 && <Separator />}
                <div className="flex items-center gap-3 py-2.5">
                  <div className={cn("flex size-8 shrink-0 items-center justify-center rounded-lg", bg)}>
                    <FileIcon className={cn("size-4", color)} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium truncate">{item.name || item.file.name}</p>
                    <p className="text-xs text-muted-foreground">{item.category}</p>
                  </div>
                  {step === "done" ? (
                    <CheckCircle2 className="size-4 shrink-0 text-status-active" />
                  ) : (
                    <div className="size-4 shrink-0 rounded-full border-2 border-primary border-t-transparent animate-spin" />
                  )}
                </div>
              </div>
            )
          })}
        </div>

        {step === "done" && (
          <DialogFooter className="px-6 py-4 border-t border-border">
            <Button onClick={handleClose} className="gap-1.5">
              <CheckCircle2 className="size-4" />
              Done
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}

interface FileBrowserViewProps {
  onOpenFile?: (file: AttachedFile) => void
}

export function FileBrowserView({ onOpenFile: _onOpenFile }: FileBrowserViewProps) {
  const isMobile = useIsMobile()
  const [activeCategory, setActiveCategory] = useState<BrowserCategory>("all")
  const [query, setQuery] = useState("")
  const [activeTags, setActiveTags] = useState<string[]>([])
  const [viewMode, setViewMode] = useState<"grid" | "list">("list")
  const [selectedFile, setSelectedFile] = useState<AttachedFile | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AttachedFile | null>(null)
  const [deletedIds, setDeletedIds] = useState<Set<string>>(new Set())
  const [currentPage, setCurrentPage] = useState(1)
  const [uploadOpen, setUploadOpen] = useState(false)

  function handleDelete(fileId: string) {
    setDeletedIds((prev) => new Set([...prev, fileId]))
    setDeleteTarget(null)
  }

  function toggleTag(tagId: string) {
    setActiveTags((prev) =>
      prev.includes(tagId) ? prev.filter((t) => t !== tagId) : [...prev, tagId]
    )
  }

  const liveFiles = MOCK_FILES.filter((f) => !deletedIds.has(f.id))

  const filtered = liveFiles.filter((f) => {
    if (activeCategory !== "all" && f.category !== activeCategory) return false
    if (activeTags.length > 0 && !activeTags.some((t) => f.tags.includes(t))) return false
    if (query && !f.name.toLowerCase().includes(query.toLowerCase())) return false
    return true
  })

  useEffect(() => {
    setCurrentPage(1)
  }, [query, activeTags, deletedIds, activeCategory])

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const paginated = filtered.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE)

  const countByCategory = (cat: FileCategory) => liveFiles.filter((f) => f.category === cat).length

  const totalSize = "26.3 MB"
  const activeCategoryTab = CATEGORY_TABS.find((t) => t.id === activeCategory)!

  return (
    <div className="flex flex-col gap-6 p-6 max-w-5xl mx-auto w-full">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Files</h1>
          <p className="mt-1 text-muted-foreground">
            Photos, invoices, and documents attached to your inventory.
          </p>
        </div>
        <Button size="sm" className="gap-1.5 shrink-0" onClick={() => setUploadOpen(true)}>
          <Upload className="size-4" />
          Upload
        </Button>
      </div>

      {/* Category tabs — primary navigation */}
      {isMobile ? (
        <Select value={activeCategory} onValueChange={(v) => setActiveCategory(v as BrowserCategory)}>
          <SelectTrigger className="w-full">
            <SelectValue>
              {(() => {
                const tab = CATEGORY_TABS.find((t) => t.id === activeCategory)!
                const Icon = tab.icon
                const count = tab.id === "all" ? liveFiles.length : countByCategory(tab.id as FileCategory)
                return (
                  <span className="flex items-center gap-2 min-w-0">
                    <span className={cn("flex size-5 shrink-0 items-center justify-center rounded-md pointer-events-none", tab.bg)}>
                      <Icon className={cn("size-3 pointer-events-none", tab.color)} />
                    </span>
                    <span className="font-medium">{tab.label}</span>
                    <span className="text-muted-foreground text-sm">({count})</span>
                  </span>
                )
              })()}
            </SelectValue>
          </SelectTrigger>
          <SelectContent position="popper" className="w-[--radix-select-trigger-width]">
            {CATEGORY_TABS.map((tab) => {
              const Icon = tab.icon
              const count = tab.id === "all" ? liveFiles.length : countByCategory(tab.id as FileCategory)
              return (
                <SelectItem key={tab.id} value={tab.id}>
                  <span className={cn("flex size-5 shrink-0 items-center justify-center rounded-md", tab.bg)}>
                    <Icon className={cn("size-3", tab.color)} />
                  </span>
                  <span>{tab.label}</span>
                  <span className="ml-auto text-muted-foreground pl-4">{count}</span>
                </SelectItem>
              )
            })}
          </SelectContent>
        </Select>
      ) : (
        <div className="grid grid-cols-5 gap-2">
          {CATEGORY_TABS.map((tab) => {
            const Icon = tab.icon
            const count = tab.id === "all" ? liveFiles.length : countByCategory(tab.id as FileCategory)
            const isActive = activeCategory === tab.id
            return (
              <button
                key={tab.id}
                onClick={() => setActiveCategory(tab.id)}
                className={cn(
                  "group flex flex-col items-start gap-1.5 rounded-xl border p-3 text-left transition-all",
                  isActive
                    ? "border-primary bg-primary/5 shadow-sm"
                    : "border-border bg-card hover:border-primary/30 hover:shadow-sm"
                )}
              >
                <div className={cn(
                  "flex size-7 items-center justify-center rounded-lg transition-colors",
                  isActive ? `${tab.bg} ${tab.color}` : "bg-muted text-muted-foreground"
                )}>
                  <Icon className="size-3.5" />
                </div>
                <div className="min-w-0 w-full">
                  <p className={cn("text-xs font-semibold truncate", isActive ? "text-foreground" : "text-muted-foreground group-hover:text-foreground")}>
                    {tab.label}
                  </p>
                  <p className={cn("text-lg font-bold leading-none mt-0.5", isActive ? "text-foreground" : "text-muted-foreground")}>
                    {count}
                  </p>
                </div>
              </button>
            )
          })}
        </div>
      )}

      {/* Active category description + toolbar */}
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <div className={cn("flex size-5 items-center justify-center rounded-md", activeCategoryTab.bg)}>
            {(() => { const Icon = activeCategoryTab.icon; return <Icon className={cn("size-3", activeCategoryTab.color)} /> })()}
          </div>
          <p className="text-xs text-muted-foreground flex-1">{activeCategoryTab.description}</p>
          <div className="flex gap-1">
            <Button variant={viewMode === "list" ? "secondary" : "ghost"} size="icon" className="size-8" onClick={() => setViewMode("list")}>
              <List className="size-4" />
            </Button>
            <Button variant={viewMode === "grid" ? "secondary" : "ghost"} size="icon" className="size-8" onClick={() => setViewMode("grid")}>
              <LayoutGrid className="size-4" />
            </Button>
          </div>
        </div>

        <div className="flex flex-col gap-2">
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="Search files…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="pl-8 w-full"
            />
          </div>
          {/* Tag filter pills */}
          <div className="flex items-center gap-1.5 flex-wrap">
            {FILE_TAGS.map((tag) => {
              const isActive = activeTags.includes(tag.id)
              return (
                <button
                  key={tag.id}
                  onClick={() => toggleTag(tag.id)}
                  className={cn(
                    "flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium transition-all border",
                    isActive
                      ? "bg-primary text-primary-foreground border-primary"
                      : "bg-card text-muted-foreground border-border hover:text-foreground hover:border-foreground/30"
                  )}
                >
                  {tag.label}
                  {isActive && <X className="size-3" />}
                </button>
              )
            })}
            {activeTags.length > 0 && (
              <button
                onClick={() => setActiveTags([])}
                className="text-xs text-muted-foreground hover:text-foreground transition-colors ml-1"
              >
                Clear all
              </button>
            )}
          </div>
        </div>
      </div>

      {/* File list */}
      {filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border py-16">
          {(() => { const Icon = activeCategoryTab.icon; return <Icon className={cn("size-10 opacity-30", activeCategoryTab.color)} /> })()}
          <p className="text-sm text-muted-foreground">
            {query || activeTags.length > 0
              ? "No files match your search."
              : activeCategory === "all"
              ? "No files yet."
              : `No ${activeCategoryTab.label.toLowerCase()} yet.`}
          </p>
        </div>
      ) : viewMode === "list" ? (
        <div className="rounded-xl border border-border overflow-hidden bg-card">
          {/* Desktop header — hidden on mobile */}
          <div className="hidden sm:grid grid-cols-[auto_1fr_auto_auto_auto] gap-4 px-4 py-2 bg-muted/50 border-b border-border">
            <span className="w-5" />
            <span className="text-xs font-medium text-muted-foreground">Name</span>
            <span className="text-xs font-medium text-muted-foreground w-20 text-center">Category</span>
            <span className="text-xs font-medium text-muted-foreground w-28 text-right">Uploaded</span>
            <span className="text-xs font-medium text-muted-foreground w-16 text-right">Size</span>
          </div>
          <ul>
            {paginated.map((file, i) => {
              const group = mimeGroup(file.mimeType)
              const Icon = FILE_ICONS[group]
              const isSelected = selectedFile?.id === file.id
              const fileTags = resolveTags(file.tags)
              const catTab = CATEGORY_TABS.find((t) => t.id === file.category)
              const CatIcon = catTab?.icon ?? File
              const dateStr = new Date(file.uploadedAt).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })
              return (
                <li key={file.id}>
                  {i > 0 && <Separator />}
                  {/* Desktop row */}
                  <div
                    className={cn(
                      "hidden sm:grid grid-cols-[auto_1fr_auto_auto_auto] gap-4 items-center px-4 py-3 cursor-pointer transition-colors",
                      isSelected ? "bg-accent" : "hover:bg-muted/40"
                    )}
                    onClick={() => setSelectedFile(isSelected ? null : file)}
                  >
                    <Icon className={cn("size-4 shrink-0", FILE_COLORS[group])} />
                    <div className="min-w-0">
                      <p className="text-sm font-medium truncate">{file.name}</p>
                      <div className="flex items-center gap-1.5 mt-0.5 flex-wrap">
                        <span className="text-xs text-muted-foreground truncate">{file.attachedTo.name}</span>
                        {fileTags.map((tag) => (
                          <TagPill key={tag.id} tag={tag} size="xs" />
                        ))}
                      </div>
                    </div>
                    <div className={cn("flex items-center gap-1 rounded-full px-2 py-0.5 w-20 justify-center", catTab?.bg ?? "bg-muted")}>
                      <CatIcon className={cn("size-3 shrink-0", catTab?.color ?? "text-muted-foreground")} />
                      <span className={cn("text-[10px] font-medium", catTab?.color ?? "text-muted-foreground")}>{catTab?.label ?? file.category}</span>
                    </div>
                    <span className="text-xs text-muted-foreground w-28 text-right">{dateStr}</span>
                    <span className="text-xs text-muted-foreground w-16 text-right">{file.size}</span>
                  </div>
                  {/* Mobile row — two-line card */}
                  <div
                    className={cn(
                      "flex sm:hidden items-start gap-3 px-4 py-3 cursor-pointer transition-colors",
                      isSelected ? "bg-accent" : "active:bg-muted/40"
                    )}
                    onClick={() => setSelectedFile(isSelected ? null : file)}
                  >
                    <div className={cn("flex size-9 shrink-0 items-center justify-center rounded-lg mt-0.5", FILE_BG[group])}>
                      <Icon className={cn("size-4", FILE_COLORS[group])} />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium truncate leading-tight">{file.name}</p>
                      <p className="text-xs text-muted-foreground truncate mt-0.5">{file.attachedTo.name}</p>
                      <div className="flex items-center gap-2 mt-1.5 flex-wrap">
                        <div className={cn("flex items-center gap-1 rounded-full px-2 py-0.5", catTab?.bg ?? "bg-muted")}>
                          <CatIcon className={cn("size-2.5 shrink-0", catTab?.color ?? "text-muted-foreground")} />
                          <span className={cn("text-[10px] font-medium", catTab?.color ?? "text-muted-foreground")}>{catTab?.label ?? file.category}</span>
                        </div>
                        {fileTags.map((tag) => (
                          <TagPill key={tag.id} tag={tag} size="xs" />
                        ))}
                      </div>
                    </div>
                    <div className="flex flex-col items-end shrink-0 gap-0.5 mt-0.5">
                      <span className="text-xs text-muted-foreground">{file.size}</span>
                      <span className="text-[10px] text-muted-foreground whitespace-nowrap">{dateStr}</span>
                    </div>
                  </div>
                </li>
              )
            })}
          </ul>
        </div>
      ) : (
        <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {paginated.map((file) => {
            const group = mimeGroup(file.mimeType)
            const Icon = FILE_ICONS[group]
            const isSelected = selectedFile?.id === file.id
            const fileTags = resolveTags(file.tags)
            const catTab = CATEGORY_TABS.find((t) => t.id === file.category)
            const CatIcon = catTab?.icon ?? File
            return (
              <div
                key={file.id}
                className={cn(
                  "group flex flex-col rounded-xl border cursor-pointer transition-all overflow-hidden w-full",
                  isSelected ? "border-primary bg-primary/5" : "border-border bg-card hover:shadow-md hover:-translate-y-0.5"
                )}
                onClick={() => setSelectedFile(isSelected ? null : file)}
              >
                {/* Preview area */}
                <div className="relative w-full aspect-video overflow-hidden">
                  {file.thumbnailUrl ? (
                    <img
                      src={file.thumbnailUrl}
                      alt={file.name}
                      className="absolute inset-0 w-full h-full object-cover"
                    />
                  ) : (
                    <div className={cn("absolute inset-0 flex items-center justify-center", FILE_BG[group])}>
                      <Icon className={cn("size-12 opacity-80", FILE_COLORS[group])} strokeWidth={1.5} />
                    </div>
                  )}
                  {/* Category badge overlay */}
                  <div className={cn(
                    "absolute top-2 left-2 flex items-center gap-1 rounded-full px-2 py-0.5",
                    catTab?.bg ?? "bg-muted"
                  )}>
                    <CatIcon className={cn("size-2.5", catTab?.color ?? "text-muted-foreground")} />
                    <span className={cn("text-[10px] font-medium", catTab?.color ?? "text-muted-foreground")}>
                      {catTab?.label ?? file.category}
                    </span>
                  </div>
                </div>

                {/* Info */}
                <div className="min-w-0 px-3 py-2.5">
                  <p className="text-sm font-medium truncate leading-tight" title={file.name}>
                    {file.name}
                  </p>
                  <p className="text-xs text-muted-foreground truncate mt-0.5" title={file.attachedTo.name}>
                    {file.attachedTo.name}
                  </p>
                  <div className="flex items-center gap-x-2 gap-y-0.5 mt-1.5 flex-wrap">
                    {fileTags.map((tag) => (
                      <TagPill key={tag.id} tag={tag} size="xs" />
                    ))}
                    <span className="text-[10px] text-muted-foreground ml-auto">{file.size}</span>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex flex-col items-center gap-3 sm:flex-row sm:justify-between">
          <p className="text-sm text-muted-foreground">
            Showing {(currentPage - 1) * PAGE_SIZE + 1}–{Math.min(currentPage * PAGE_SIZE, filtered.length)} of {filtered.length} files
          </p>
          <Pagination className="w-auto mx-0">
            <PaginationContent>
              <PaginationItem>
                <PaginationPrevious
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  aria-disabled={currentPage === 1}
                  className={cn(currentPage === 1 && "pointer-events-none opacity-50")}
                />
              </PaginationItem>
              {Array.from({ length: totalPages }, (_, i) => i + 1).map((page) => {
                const showPage =
                  page === 1 ||
                  page === totalPages ||
                  Math.abs(page - currentPage) <= 1
                const showEllipsisBefore = page === currentPage - 2 && currentPage - 2 > 1
                const showEllipsisAfter = page === currentPage + 2 && currentPage + 2 < totalPages
                if (showEllipsisBefore) {
                  return (
                    <PaginationItem key={`ellipsis-before-${page}`}>
                      <PaginationEllipsis />
                    </PaginationItem>
                  )
                }
                if (showEllipsisAfter) {
                  return (
                    <PaginationItem key={`ellipsis-after-${page}`}>
                      <PaginationEllipsis />
                    </PaginationItem>
                  )
                }
                if (!showPage) return null
                return (
                  <PaginationItem key={page}>
                    <PaginationLink
                      isActive={page === currentPage}
                      onClick={() => setCurrentPage(page)}
                    >
                      {page}
                    </PaginationLink>
                  </PaginationItem>
                )
              })}
              <PaginationItem>
                <PaginationNext
                  onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                  aria-disabled={currentPage === totalPages}
                  className={cn(currentPage === totalPages && "pointer-events-none opacity-50")}
                />
              </PaginationItem>
            </PaginationContent>
          </Pagination>
        </div>
      )}

      <div className="text-xs text-muted-foreground">
        {filtered.length} file{filtered.length !== 1 ? "s" : ""} · {totalSize} total
      </div>

      <UploadDialog open={uploadOpen} onClose={() => setUploadOpen(false)} />

      {/* File detail sheet */}
      <FileDetailSheet
        file={selectedFile}
        onClose={() => setSelectedFile(null)}
        onDelete={(id) => {
          handleDelete(id)
          setSelectedFile(null)
        }}
      />

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete file?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-medium text-foreground">{deleteTarget?.name}</span> will be permanently deleted. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteTarget && handleDelete(deleteTarget.id)}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

export type { AttachedFile as FileEntry }
