import { useState } from "react"
import {
  ChevronRight, MapPin, Package, Plus, MoveHorizontal as MoreHorizontal,
  Pencil, Trash2, ArrowLeft, Layers, FileText, Image as ImageIcon, File,
  Upload, Paperclip, ExternalLink, ChevronDown, Search, LayoutGrid, List,
  FileArchive, X, Receipt, BookOpen,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { ItemsPanel } from "@/components/ItemsPanel"
import { LocationDialog } from "@/components/LocationDialog"
import { AreaDialog } from "@/components/AreaDialog"
import { AddItemDialog } from "@/components/AddItemDialog"
import { FilePreviewDialog } from "@/components/FilePreviewDialog"
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
  MOCK_LOCATIONS,
  MOCK_AREAS,
  MOCK_ITEMS,
  MOCK_FILES,
  FILE_TAGS,
  warrantyStatus,
  type Location,
  type Area,
  type AttachedFile,
  type FileCategory,
} from "@/data/mock"
import { cn } from "@/lib/utils"

// ── Shared file utilities ─────────────────────────────────────────────────────

type MimeGroup = "pdf" | "image" | "archive" | "other"

function mimeGroup(mimeType: string): MimeGroup {
  if (mimeType === "application/pdf") return "pdf"
  if (mimeType.startsWith("image/")) return "image"
  if (mimeType.includes("zip") || mimeType.includes("archive")) return "archive"
  return "other"
}

const MIME_ICONS: Record<MimeGroup, React.ElementType> = {
  pdf: FileText, image: ImageIcon, archive: FileArchive, other: File,
}
const MIME_COLORS: Record<MimeGroup, string> = {
  pdf: "text-status-expired", image: "text-status-active", archive: "text-chart-4", other: "text-muted-foreground",
}
const MIME_BG: Record<MimeGroup, string> = {
  pdf: "bg-status-expired/10", image: "bg-status-active/10", archive: "bg-chart-4/10", other: "bg-muted",
}

const CAT_TABS: { id: FileCategory; label: string; icon: React.ElementType; color: string; bg: string }[] = [
  { id: "image",    label: "Photos",    icon: ImageIcon,   color: "text-status-active", bg: "bg-status-active/10" },
  { id: "invoice",  label: "Invoices",  icon: Receipt,     color: "text-chart-1",       bg: "bg-chart-1/10" },
  { id: "document", label: "Documents", icon: BookOpen,    color: "text-chart-3",       bg: "bg-chart-3/10" },
  { id: "other",    label: "Other",     icon: FileArchive, color: "text-chart-4",       bg: "bg-chart-4/10" },
]

function previewLabel(mimeType: string) {
  if (mimeType.startsWith("image/")) return "View"
  if (mimeType === "application/pdf") return "Open"
  return "Download"
}

// ── Reusable collapsible files panel ─────────────────────────────────────────

interface FilesPanelProps {
  title: string
  attachType: "location" | "commodity"
  attachId: string
}

function FilesPanel({ title, attachType, attachId }: FilesPanelProps) {
  const [expanded, setExpanded] = useState(false)
  const [dragging, setDragging] = useState(false)
  const [previewFile, setPreviewFile] = useState<AttachedFile | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AttachedFile | null>(null)
  const [deletedIds, setDeletedIds] = useState<Set<string>>(new Set())
  const [query, setQuery] = useState("")
  const [activeTags, setActiveTags] = useState<string[]>([])
  const [activeCategory, setActiveCategory] = useState<FileCategory | "all">("all")
  const [viewMode, setViewMode] = useState<"list" | "grid">("list")

  const allFiles = MOCK_FILES.filter(
    (f) => f.attachedTo.type === attachType && f.attachedTo.id === attachId && !deletedIds.has(f.id)
  )

  const filtered = allFiles.filter((f) => {
    if (activeCategory !== "all" && f.category !== activeCategory) return false
    if (activeTags.length > 0 && !activeTags.some((t) => f.tags.includes(t))) return false
    if (query && !f.name.toLowerCase().includes(query.toLowerCase())) return false
    return true
  })

  function handleDelete(fileId: string) {
    setDeletedIds((prev) => new Set([...prev, fileId]))
    setDeleteTarget(null)
  }

  function toggleTag(tagId: string) {
    setActiveTags((prev) => prev.includes(tagId) ? prev.filter((t) => t !== tagId) : [...prev, tagId])
  }

  const inputId = `files-input-${attachType}-${attachId}`

  return (
    <>
      <div className="rounded-xl border border-border bg-card overflow-hidden">
        {/* Header — always visible */}
        <button
          className="w-full flex items-center justify-between px-4 py-3 hover:bg-muted/40 transition-colors"
          onClick={() => setExpanded((v) => !v)}
        >
          <div className="flex items-center gap-2 text-sm font-medium">
            <Paperclip className="size-4 text-muted-foreground" />
            {title}
            {allFiles.length > 0 && (
              <span className="flex size-5 items-center justify-center rounded-full bg-muted text-xs font-medium tabular-nums">
                {allFiles.length}
              </span>
            )}
          </div>
          <ChevronDown className={cn("size-4 text-muted-foreground transition-transform", expanded && "rotate-180")} />
        </button>

        {expanded && (
          <div className="border-t border-border flex flex-col gap-3 p-4">
            {/* Upload drop zone */}
            <div
              className={cn(
                "flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed py-4 cursor-pointer transition-colors",
                dragging ? "border-primary bg-primary/5" : "border-border hover:border-primary/40 hover:bg-muted/30"
              )}
              onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
              onDragLeave={() => setDragging(false)}
              onDrop={(e) => { e.preventDefault(); setDragging(false) }}
              onClick={() => document.getElementById(inputId)?.click()}
            >
              <Upload className="size-4 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                Drop files or <span className="font-medium text-foreground">browse</span>
              </p>
              <input id={inputId} type="file" multiple className="sr-only" />
            </div>

            {/* Search + toolbar */}
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder="Search files…"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  className="pl-8 h-8 text-sm"
                />
              </div>
              <Button
                variant={viewMode === "list" ? "secondary" : "ghost"}
                size="icon" className="size-8 shrink-0"
                onClick={() => setViewMode("list")}
              >
                <List className="size-4" />
              </Button>
              <Button
                variant={viewMode === "grid" ? "secondary" : "ghost"}
                size="icon" className="size-8 shrink-0"
                onClick={() => setViewMode("grid")}
              >
                <LayoutGrid className="size-4" />
              </Button>
            </div>

            {/* Category switcher — segmented strip */}
            <div className="flex items-center gap-1 rounded-lg bg-muted/50 p-1">
              {([
                { id: "all" as const, label: "All", icon: Paperclip },
                ...CAT_TABS,
              ] as { id: FileCategory | "all"; label: string; icon: React.ElementType }[]).map((section) => {
                const count = section.id === "all"
                  ? allFiles.length
                  : allFiles.filter((f) => f.category === section.id).length
                const Icon = section.icon
                return (
                  <button
                    key={section.id}
                    onClick={() => setActiveCategory(section.id)}
                    className={cn(
                      "flex flex-1 items-center justify-center gap-1.5 rounded-md px-2 py-1.5 text-xs font-medium transition-all",
                      activeCategory === section.id
                        ? "bg-background text-foreground shadow-sm"
                        : "text-muted-foreground hover:text-foreground"
                    )}
                  >
                    <Icon className="size-3" />
                    <span className="hidden sm:inline">{section.label}</span>
                    {count > 0 && (
                      <span className={cn(
                        "flex size-4 items-center justify-center rounded-full text-[10px] font-semibold",
                        activeCategory === section.id ? "bg-muted text-foreground" : "bg-muted/60 text-muted-foreground"
                      )}>
                        {count}
                      </span>
                    )}
                  </button>
                )
              })}
            </div>

            {/* Tag filter pills */}
            <div className="flex flex-wrap items-center gap-1.5">
              {FILE_TAGS.map((tag) => {
                const isActive = activeTags.includes(tag.id)
                return (
                  <button
                    key={tag.id}
                    onClick={() => toggleTag(tag.id)}
                    className={cn(
                      "flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-medium transition-all",
                      isActive
                        ? "bg-primary text-primary-foreground border-primary"
                        : "bg-card text-muted-foreground border-border hover:text-foreground hover:border-foreground/30"
                    )}
                  >
                    #{tag.label}
                    {isActive && <X className="size-3" />}
                  </button>
                )
              })}
              {activeTags.length > 0 && (
                <button
                  onClick={() => setActiveTags([])}
                  className="text-xs text-muted-foreground hover:text-foreground transition-colors"
                >
                  Clear
                </button>
              )}
            </div>

            {/* File list / grid */}
            {filtered.length === 0 ? (
              <div className="flex flex-col items-center gap-2 py-6">
                <File className="size-8 text-muted-foreground/30" />
                <p className="text-sm text-muted-foreground">
                  {query || activeTags.length > 0 || activeCategory !== "all"
                    ? "No files match your filters."
                    : "No files attached yet."}
                </p>
              </div>
            ) : viewMode === "list" ? (
              <div className="rounded-xl border border-border overflow-hidden bg-card">
                <ul>
                  {filtered.map((f, i) => {
                    const g = mimeGroup(f.mimeType)
                    const Icon = MIME_ICONS[g]
                    const fileTags = FILE_TAGS.filter((t) => f.tags.includes(t.id))
                    const catTab = CAT_TABS.find((c) => c.id === f.category)
                    const CatIcon = catTab?.icon ?? File
                    const dateStr = new Date(f.uploadedAt).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })
                    return (
                      <li key={f.id}>
                        {i > 0 && <Separator />}
                        <div className="flex items-center gap-3 px-3 py-2.5">
                          <div className={cn("flex size-8 shrink-0 items-center justify-center rounded-lg", MIME_BG[g])}>
                            <Icon className={cn("size-4", MIME_COLORS[g])} />
                          </div>
                          <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium truncate">{f.name}</p>
                            <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                              <span className="text-xs text-muted-foreground">{f.size} · {dateStr}</span>
                              <div className={cn("flex items-center gap-1 rounded-full px-1.5 py-0.5", catTab?.bg ?? "bg-muted")}>
                                <CatIcon className={cn("size-2.5", catTab?.color ?? "text-muted-foreground")} />
                                <span className={cn("text-[10px] font-medium", catTab?.color ?? "text-muted-foreground")}>{catTab?.label ?? f.category}</span>
                              </div>
                              {fileTags.map((t) => (
                                <span key={t.id} className={cn("text-[10px] font-medium", t.color)}>#{t.label}</span>
                              ))}
                            </div>
                          </div>
                          <div className="flex items-center gap-1 shrink-0">
                            <Button
                              variant="ghost" size="sm" className="h-7 px-2 text-xs gap-1"
                              onClick={() => setPreviewFile(f)}
                            >
                              {previewLabel(f.mimeType)}
                              <ExternalLink className="size-3" />
                            </Button>
                            <Button
                              variant="ghost" size="icon" className="size-7 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                              onClick={() => setDeleteTarget(f)}
                            >
                              <Trash2 className="size-3.5" />
                            </Button>
                          </div>
                        </div>
                      </li>
                    )
                  })}
                </ul>
              </div>
            ) : (
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                {filtered.map((f) => {
                  const g = mimeGroup(f.mimeType)
                  const Icon = MIME_ICONS[g]
                  const catTab = CAT_TABS.find((c) => c.id === f.category)
                  const CatIcon = catTab?.icon ?? File
                  return (
                    <div
                      key={f.id}
                      className="group flex flex-col rounded-xl border border-border bg-card overflow-hidden cursor-pointer hover:shadow-md hover:-translate-y-0.5 transition-all"
                      onClick={() => setPreviewFile(f)}
                    >
                      {/* Preview area */}
                      <div className="relative w-full aspect-video overflow-hidden">
                        {f.thumbnailUrl ? (
                          <img src={f.thumbnailUrl} alt={f.name} className="absolute inset-0 w-full h-full object-cover" />
                        ) : (
                          <div className={cn("absolute inset-0 flex items-center justify-center", MIME_BG[g])}>
                            <Icon className={cn("size-10 opacity-80", MIME_COLORS[g])} strokeWidth={1.5} />
                          </div>
                        )}
                        <div className={cn("absolute top-1.5 left-1.5 flex items-center gap-1 rounded-full px-2 py-0.5", catTab?.bg ?? "bg-muted")}>
                          <CatIcon className={cn("size-2.5", catTab?.color ?? "text-muted-foreground")} />
                          <span className={cn("text-[10px] font-medium", catTab?.color ?? "text-muted-foreground")}>{catTab?.label ?? f.category}</span>
                        </div>
                        {/* Hover actions */}
                        <div className="absolute top-1.5 right-1.5 opacity-0 group-hover:opacity-100 transition-opacity flex gap-1">
                          <Button
                            variant="secondary" size="icon" className="size-6"
                            onClick={(e) => { e.stopPropagation(); setDeleteTarget(f) }}
                          >
                            <Trash2 className="size-3 text-destructive" />
                          </Button>
                        </div>
                      </div>
                      <div className="px-2.5 py-2 min-w-0">
                        <p className="text-xs font-medium truncate">{f.name}</p>
                        <p className="text-[10px] text-muted-foreground mt-0.5">{f.size}</p>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}

            {/* Count line */}
            {allFiles.length > 0 && (
              <p className="text-xs text-muted-foreground">
                {filtered.length} of {allFiles.length} file{allFiles.length !== 1 ? "s" : ""}
              </p>
            )}
          </div>
        )}
      </div>

      <FilePreviewDialog file={previewFile} onClose={() => setPreviewFile(null)} onDelete={(id) => handleDelete(id)} />

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
    </>
  )
}

type DrillLevel = "locations" | "areas" | "commodities"

interface LocationPickerViewProps {
  activeGroupId: string
  onItemClick?: (id: string) => void
}

export function LocationPickerView({ activeGroupId, onItemClick }: LocationPickerViewProps) {
  const [level, setLevel] = useState<DrillLevel>("locations")
  const [selectedLocation, setSelectedLocation] = useState<Location | null>(null)
  const [selectedArea, setSelectedArea] = useState<Area | null>(null)

  const [locationDialogOpen, setLocationDialogOpen] = useState(false)
  const [editingLocation, setEditingLocation] = useState<Location | null>(null)
  const [areaDialogOpen, setAreaDialogOpen] = useState(false)
  const [editingArea, setEditingArea] = useState<Area | null>(null)
  const [itemDialogOpen, setItemDialogOpen] = useState(false)
  const [deleteLocationTarget, setDeleteLocationTarget] = useState<Location | null>(null)
  const [deleteAreaTarget, setDeleteAreaTarget] = useState<Area | null>(null)
  const [deletedLocationIds, setDeletedLocationIds] = useState<Set<string>>(new Set())
  const [deletedAreaIds, setDeletedAreaIds] = useState<Set<string>>(new Set())

  const groupLocations = MOCK_LOCATIONS.filter((l) => l.groupId === activeGroupId && !deletedLocationIds.has(l.id))

  function openLocation(loc: Location) {
    setSelectedLocation(loc)
    setSelectedArea(null)
    setLevel("areas")
  }

  function openArea(area: Area) {
    setSelectedArea(area)
    setLevel("commodities")
  }

  function goBack() {
    if (level === "commodities") {
      setSelectedArea(null)
      setLevel("areas")
    } else if (level === "areas") {
      setSelectedLocation(null)
      setLevel("locations")
    }
  }

  const locationAreas = selectedLocation
    ? MOCK_AREAS.filter((a) => a.locationId === selectedLocation.id && !deletedAreaIds.has(a.id))
    : []

  const areaCommodities = selectedArea
    ? MOCK_ITEMS.filter((i) => i.areaId === selectedArea.id)
    : []

  const areaItemCounts = (locationId: string) =>
    MOCK_AREAS.filter((a) => a.locationId === locationId).reduce(
      (sum, area) => sum + MOCK_ITEMS.filter((i) => i.areaId === area.id).length,
      0
    )

  return (
    <div className="flex flex-col h-full">
      {/* Breadcrumb header */}
      <div className="flex items-center gap-2 px-6 py-4 border-b border-border sticky top-0 bg-background z-10">
        {level !== "locations" && (
          <Button variant="ghost" size="icon" className="size-8 -ml-1" onClick={goBack}>
            <ArrowLeft className="size-4" />
          </Button>
        )}
        <div className="flex items-center gap-1.5 text-sm min-w-0 flex-1">
          <button
            className={cn(
              "text-sm transition-colors shrink-0",
              level === "locations" ? "font-semibold text-foreground" : "text-muted-foreground hover:text-foreground"
            )}
            onClick={() => { setLevel("locations"); setSelectedLocation(null); setSelectedArea(null) }}
          >
            Locations
          </button>
          {selectedLocation && (
            <>
              <ChevronRight className="size-3.5 text-muted-foreground shrink-0" />
              <button
                className={cn(
                  "text-sm transition-colors truncate",
                  level === "areas" ? "font-semibold text-foreground" : "text-muted-foreground hover:text-foreground"
                )}
                onClick={() => { setLevel("areas"); setSelectedArea(null) }}
              >
                {selectedLocation.icon} {selectedLocation.name}
              </button>
            </>
          )}
          {selectedArea && (
            <>
              <ChevronRight className="size-3.5 text-muted-foreground shrink-0" />
              <span className="font-semibold text-foreground truncate">
                {selectedArea.icon} {selectedArea.name}
              </span>
            </>
          )}
        </div>

        {level === "locations" && (
          <Button
            size="sm" variant="outline" className="gap-1.5 shrink-0"
            onClick={() => { setEditingLocation(null); setLocationDialogOpen(true) }}
          >
            <Plus className="size-3.5" />
            Add location
          </Button>
        )}
        {level === "areas" && (
          <Button
            size="sm" variant="outline" className="gap-1.5 shrink-0"
            onClick={() => { setEditingArea(null); setAreaDialogOpen(true) }}
          >
            <Plus className="size-3.5" />
            Add area
          </Button>
        )}
        {level === "commodities" && (
          <Button
            size="sm" variant="outline" className="gap-1.5 shrink-0"
            onClick={() => setItemDialogOpen(true)}
          >
            <Plus className="size-3.5" />
            Add item
          </Button>
        )}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6">

        {/* ── Level 1: Locations ── */}
        {level === "locations" && (
          <div className="max-w-3xl mx-auto space-y-3">
            {groupLocations.length === 0 ? (
              <EmptyState
                icon={MapPin}
                title="No locations yet"
                description="Add your first location to start organizing your items."
              />
            ) : (
              groupLocations.map((loc) => {
                const itemCount = areaItemCounts(loc.id)
                const areaCount = MOCK_AREAS.filter((a) => a.locationId === loc.id).length
                return (
                  <button
                    key={loc.id}
                    className="group w-full flex items-center gap-4 rounded-2xl border border-border bg-card p-5 text-left transition-all hover:shadow-sm hover:-translate-y-0.5 hover:border-primary/20"
                    onClick={() => openLocation(loc)}
                  >
                    <div className="flex size-14 items-center justify-center rounded-xl bg-muted text-3xl shrink-0">
                      {loc.icon}
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-base font-semibold">{loc.name}</p>
                      {loc.description && (
                        <p className="text-sm text-muted-foreground mt-0.5 truncate">{loc.description}</p>
                      )}
                      <div className="flex items-center gap-3 mt-2">
                        <span className="flex items-center gap-1 text-xs text-muted-foreground">
                          <Layers className="size-3.5" />
                          {areaCount} area{areaCount !== 1 ? "s" : ""}
                        </span>
                        <span className="flex items-center gap-1 text-xs text-muted-foreground">
                          <Package className="size-3.5" />
                          {itemCount} item{itemCount !== 1 ? "s" : ""}
                        </span>
                      </div>
                    </div>
                    <div className="flex items-center gap-1">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button
                            variant="ghost" size="icon" className="size-8 opacity-0 group-hover:opacity-100"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <MoreHorizontal className="size-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem onClick={(e) => {
                            e.stopPropagation()
                            setEditingLocation(loc)
                            setLocationDialogOpen(true)
                          }}>
                            <Pencil className="size-4 mr-2" />Rename
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            className="text-destructive"
                            onClick={(e) => { e.stopPropagation(); setDeleteLocationTarget(loc) }}
                          >
                            <Trash2 className="size-4 mr-2" />Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                      <ChevronRight className="size-4 text-muted-foreground group-hover:text-foreground transition-colors" />
                    </div>
                  </button>
                )
              })
            )}
          </div>
        )}

        {/* ── Level 2: Areas ── */}
        {level === "areas" && selectedLocation && (
          <div className="max-w-3xl mx-auto flex flex-col gap-6">
            <div className="grid gap-3 sm:grid-cols-2">
              {locationAreas.length === 0 ? (
                <div className="col-span-2">
                  <EmptyState icon={Layers} title="No areas yet" description="Divide this location into areas (rooms, zones)." />
                </div>
              ) : (
                locationAreas.map((area) => {
                  const items = MOCK_ITEMS.filter((i) => i.areaId === area.id)
                  const expiring = items.filter((i) => warrantyStatus(i) === "expiring").length
                  return (
                    <button
                      key={area.id}
                      className="group flex items-start gap-3 rounded-xl border border-border bg-card p-4 text-left transition-all hover:shadow-sm hover:-translate-y-0.5 hover:border-primary/20"
                      onClick={() => openArea(area)}
                    >
                      <div className="flex size-10 items-center justify-center rounded-lg bg-muted text-xl shrink-0 mt-0.5">
                        {area.icon}
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-semibold">{area.name}</p>
                        <p className="text-xs text-muted-foreground mt-0.5">
                          {items.length} item{items.length !== 1 ? "s" : ""}
                        </p>
                        {expiring > 0 && (
                          <Badge variant="secondary" className="mt-1.5 h-5 text-[10px] bg-status-expiring/10 text-status-expiring border-0">
                            {expiring} warranty expiring
                          </Badge>
                        )}
                      </div>
                      <div className="flex items-center gap-1 shrink-0 mt-0.5">
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              variant="ghost" size="icon" className="size-7 opacity-0 group-hover:opacity-100"
                              onClick={(e) => e.stopPropagation()}
                            >
                              <MoreHorizontal className="size-3.5" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem onClick={(e) => {
                              e.stopPropagation()
                              setEditingArea(area)
                              setAreaDialogOpen(true)
                            }}>
                              <Pencil className="size-4 mr-2" />Rename
                            </DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              className="text-destructive"
                              onClick={(e) => { e.stopPropagation(); setDeleteAreaTarget(area) }}
                            >
                              <Trash2 className="size-4 mr-2" />Delete
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                        <ChevronRight className="size-4 text-muted-foreground group-hover:text-foreground transition-colors" />
                      </div>
                    </button>
                  )
                })
              )}
            </div>
            <FilesPanel title="Location Files" attachType="location" attachId={selectedLocation.id} />
          </div>
        )}

        {/* ── Level 3: Commodities — full ItemsPanel ── */}
        {level === "commodities" && selectedArea && (
          <div className="max-w-5xl mx-auto flex flex-col gap-6">
            <ItemsPanel
              items={areaCommodities}
              onItemClick={(id) => onItemClick?.(id)}
              onAddItem={() => setItemDialogOpen(true)}
              showStats
              defaultViewMode="list"
            />
            <FilesPanel title="Area Files" attachType="commodity" attachId={selectedArea.id} />
          </div>
        )}
      </div>

      {/* Dialogs */}
      <LocationDialog
        open={locationDialogOpen}
        onClose={() => { setLocationDialogOpen(false); setEditingLocation(null) }}
        location={editingLocation}
        onSave={() => { setLocationDialogOpen(false); setEditingLocation(null) }}
      />

      <AreaDialog
        open={areaDialogOpen}
        onClose={() => { setAreaDialogOpen(false); setEditingArea(null) }}
        locationName={selectedLocation?.name}
        area={editingArea}
        onSave={() => { setAreaDialogOpen(false); setEditingArea(null) }}
      />

      <AddItemDialog
        open={itemDialogOpen}
        onClose={() => setItemDialogOpen(false)}
        defaultAreaId={selectedArea?.id}
      />

      {/* Delete Location Confirmation */}
      <AlertDialog open={!!deleteLocationTarget} onOpenChange={(open) => !open && setDeleteLocationTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete location?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-medium text-foreground">{deleteLocationTarget?.icon} {deleteLocationTarget?.name}</span> and all its areas and items will be permanently deleted. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (deleteLocationTarget) {
                  setDeletedLocationIds((prev) => new Set([...prev, deleteLocationTarget.id]))
                  setDeleteLocationTarget(null)
                }
              }}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Delete Area Confirmation */}
      <AlertDialog open={!!deleteAreaTarget} onOpenChange={(open) => !open && setDeleteAreaTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete area?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-medium text-foreground">{deleteAreaTarget?.icon} {deleteAreaTarget?.name}</span> and all its items will be permanently deleted. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (deleteAreaTarget) {
                  setDeletedAreaIds((prev) => new Set([...prev, deleteAreaTarget.id]))
                  setDeleteAreaTarget(null)
                }
              }}
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

function EmptyState({
  icon: Icon,
  title,
  description,
}: {
  icon: React.ElementType
  title: string
  description: string
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border py-16">
      <Icon className="size-10 text-muted-foreground/30" />
      <div className="text-center">
        <p className="text-sm font-medium text-foreground">{title}</p>
        <p className="text-xs text-muted-foreground mt-1">{description}</p>
      </div>
    </div>
  )
}
