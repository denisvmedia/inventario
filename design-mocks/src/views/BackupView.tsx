import { useState, useRef } from "react"
import { HardDriveDownload, Upload, RotateCcw, Plus, Download, Trash2, CircleCheck as CheckCircle2, Circle as XCircle, Clock, Loader as Loader2, TriangleAlert as AlertTriangle, Eye, CloudUpload } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
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
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationPrevious,
  PaginationNext,
  PaginationEllipsis,
} from "@/components/ui/pagination"
import { cn } from "@/lib/utils"

const EXPORTS_PAGE_SIZE = 5

type ExportStatus = "pending" | "in_progress" | "completed" | "failed"
type ExportType = "full" | "locations" | "areas" | "commodities" | "imported"
type RestoreStrategy = "full_replace" | "merge_add" | "merge_update"

interface ExportRecord {
  id: string
  type: ExportType
  status: ExportStatus
  createdAt: string
  stats?: {
    locations: number
    areas: number
    commodities: number
    files: number
    sizeMb: number
  }
  error?: string
}

interface RestoreLog {
  icon: string
  text: string
  ok: boolean
}

const MOCK_EXPORTS: ExportRecord[] = [
  {
    id: "exp-1",
    type: "full",
    status: "completed",
    createdAt: "2026-04-26T08:30:00Z",
    stats: { locations: 3, areas: 12, commodities: 47, files: 23, sizeMb: 4.2 },
  },
  {
    id: "exp-2",
    type: "full",
    status: "completed",
    createdAt: "2026-04-15T14:00:00Z",
    stats: { locations: 3, areas: 11, commodities: 43, files: 19, sizeMb: 3.8 },
  },
  {
    id: "exp-3",
    type: "imported",
    status: "completed",
    createdAt: "2026-03-01T10:15:00Z",
    stats: { locations: 2, areas: 8, commodities: 30, files: 12, sizeMb: 2.1 },
  },
  {
    id: "exp-4",
    type: "full",
    status: "failed",
    createdAt: "2026-02-10T09:00:00Z",
    error: "Storage service unavailable during export.",
  },
]

const MOCK_RESTORE_LOGS: RestoreLog[] = [
  { icon: "📝", text: "Starting restore with strategy: Merge Update", ok: true },
  { icon: "✅", text: "Location 'Main House' — already exists, skipping", ok: true },
  { icon: "✅", text: "Location 'Garage' — already exists, skipping", ok: true },
  { icon: "🔄", text: "Area 'Kitchen' — updated 3 fields", ok: true },
  { icon: "🔄", text: "Area 'Living Room' — updated 1 field", ok: true },
  { icon: "📝", text: "Processing 47 commodities…", ok: true },
  { icon: "✅", text: "Commodity 'Dyson V15 Detect' — unchanged", ok: true },
  { icon: "🔄", text: "Commodity 'Samsung 65\" QLED TV' — updated purchase price", ok: true },
  { icon: "✅", text: "Commodity 'KitchenAid Stand Mixer' — unchanged", ok: true },
  { icon: "❌", text: "Commodity 'Unknown Item #22' — failed to map area, skipped", ok: false },
  { icon: "✅", text: "Restore completed with 1 warning", ok: true },
]

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function exportTypeLabel(type: ExportType) {
  const map: Record<ExportType, string> = {
    full: "Full Database",
    locations: "Locations",
    areas: "Areas",
    commodities: "Commodities",
    imported: "Imported",
  }
  return map[type]
}

function StatusBadge({ status }: { status: ExportStatus }) {
  if (status === "completed")
    return (
      <Badge variant="secondary" className="gap-1 bg-status-active/10 text-status-active border-0">
        <CheckCircle2 className="size-3" /> Completed
      </Badge>
    )
  if (status === "failed")
    return (
      <Badge variant="secondary" className="gap-1 bg-destructive/10 text-destructive border-0">
        <XCircle className="size-3" /> Failed
      </Badge>
    )
  if (status === "in_progress")
    return (
      <Badge variant="secondary" className="gap-1 bg-status-expiring/10 text-status-expiring border-0">
        <Loader2 className="size-3 animate-spin" /> In Progress
      </Badge>
    )
  return (
    <Badge variant="secondary" className="gap-1 border-0">
      <Clock className="size-3" /> Pending
    </Badge>
  )
}

export function BackupView() {
  const [exports, setExports] = useState<ExportRecord[]>(MOCK_EXPORTS)
  const [createOpen, setCreateOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [restoreOpen, setRestoreOpen] = useState(false)
  const [restoreLogsOpen, setRestoreLogsOpen] = useState(false)
  const [selectedExport, setSelectedExport] = useState<ExportRecord | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<ExportRecord | null>(null)
  const [restoreStrategy, setRestoreStrategy] = useState<RestoreStrategy>("merge_update")
  const [dryRun, setDryRun] = useState(false)
  const [exportType, setExportType] = useState<ExportType>("full")
  const [importing, setImporting] = useState(false)
  const [dragOver, setDragOver] = useState(false)
  const [exportsPage, setExportsPage] = useState(1)
  const fileRef = useRef<HTMLInputElement>(null)

  const exportsTotalPages = Math.max(1, Math.ceil(exports.length / EXPORTS_PAGE_SIZE))
  const paginatedExports = exports.slice((exportsPage - 1) * EXPORTS_PAGE_SIZE, exportsPage * EXPORTS_PAGE_SIZE)

  function handleCreateExport() {
    const newExport: ExportRecord = {
      id: `exp-${Date.now()}`,
      type: exportType,
      status: "in_progress",
      createdAt: new Date().toISOString(),
    }
    setExports((prev) => [newExport, ...prev])
    setExportsPage(1)
    setCreateOpen(false)
    // Simulate completion after delay
    setTimeout(() => {
      setExports((prev) =>
        prev.map((e) =>
          e.id === newExport.id
            ? { ...e, status: "completed", stats: { locations: 3, areas: 12, commodities: 47, files: 23, sizeMb: 4.2 } }
            : e
        )
      )
    }, 2500)
  }

  function handleImportFile(file: File) {
    if (!file) return
    setImporting(true)
    setTimeout(() => {
      const imported: ExportRecord = {
        id: `exp-import-${Date.now()}`,
        type: "imported",
        status: "completed",
        createdAt: new Date().toISOString(),
        stats: { locations: 2, areas: 7, commodities: 28, files: 10, sizeMb: file.size / (1024 * 1024) },
      }
      setExports((prev) => [imported, ...prev])
      setExportsPage(1)
      setImporting(false)
      setImportOpen(false)
    }, 1800)
  }

  function handleDrop(e: React.DragEvent) {
    e.preventDefault()
    setDragOver(false)
    const file = e.dataTransfer.files[0]
    if (file) handleImportFile(file)
  }

  function handleFileInput(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (file) handleImportFile(file)
    e.target.value = ""
  }

  function handleRestore() {
    setRestoreOpen(false)
    setRestoreLogsOpen(true)
  }

  function handleDelete(id: string) {
    setExports((prev) => prev.filter((e) => e.id !== id))
  }

  const EXPORT_TYPES: { value: ExportType; label: string; description: string }[] = [
    { value: "full", label: "Full Database", description: "All locations, areas, items, and files" },
    { value: "locations", label: "Locations only", description: "Location records without nested data" },
    { value: "areas", label: "Areas only", description: "Area records without items" },
    { value: "commodities", label: "Items only", description: "All inventory items and their files" },
  ]

  const STRATEGIES: { value: RestoreStrategy; label: string; description: string; risk: "low" | "medium" | "high" }[] = [
    {
      value: "merge_add",
      label: "Merge — Add only",
      description: "Only adds records missing from the current database. Existing data is never modified.",
      risk: "low",
    },
    {
      value: "merge_update",
      label: "Merge — Add & Update",
      description: "Creates missing records and updates existing ones. Unrelated data is untouched.",
      risk: "medium",
    },
    {
      value: "full_replace",
      label: "Full Replace",
      description: "Wipes the entire database then restores from backup. All current data will be lost.",
      risk: "high",
    },
  ]

  return (
    <div className="flex flex-col gap-8 p-6 max-w-4xl mx-auto w-full">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Backup &amp; Restore</h1>
          <p className="mt-1 text-muted-foreground">
            Export your inventory to a file, import a previous backup, or restore data from an export.
          </p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <Button variant="outline" className="gap-2 flex-1 sm:flex-none" onClick={() => setImportOpen(true)}>
            <Upload className="size-4" />
            Import
          </Button>
          <Button className="gap-2 flex-1 sm:flex-none" onClick={() => setCreateOpen(true)}>
            <Plus className="size-4" />
            Create Export
          </Button>
        </div>
      </div>

      {/* Exports list */}
      <div className="flex flex-col gap-3">
        <div className="flex items-center gap-2">
          <HardDriveDownload className="size-4 text-muted-foreground" />
          <h2 className="text-base font-semibold">Exports</h2>
          <span className="text-xs text-muted-foreground ml-auto">{exports.length} export{exports.length !== 1 ? "s" : ""}</span>
        </div>

        {exports.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-border py-16">
            <HardDriveDownload className="size-10 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">No exports yet. Create your first backup above.</p>
          </div>
        ) : (
          <div className="flex flex-col gap-2">
            {paginatedExports.map((exp) => (
              <div
                key={exp.id}
                className="group flex flex-col gap-3 rounded-xl border border-border bg-card px-4 py-4 sm:flex-row sm:items-center sm:gap-4 sm:px-5"
              >
                {/* Top row on mobile: icon + info + delete */}
                <div className="flex items-start gap-3 sm:contents">
                  {/* Icon */}
                  <div className={cn(
                    "flex size-10 items-center justify-center rounded-lg shrink-0",
                    exp.status === "completed" ? "bg-status-active/10" : exp.status === "failed" ? "bg-destructive/10" : "bg-muted"
                  )}>
                    {exp.status === "in_progress"
                      ? <Loader2 className="size-5 text-muted-foreground animate-spin" />
                      : exp.status === "failed"
                      ? <XCircle className="size-5 text-destructive" />
                      : <HardDriveDownload className="size-5 text-status-active" />
                    }
                  </div>

                  {/* Info */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-sm font-semibold">{exportTypeLabel(exp.type)}</span>
                      <StatusBadge status={exp.status} />
                    </div>
                    <p className="text-xs text-muted-foreground mt-0.5">{formatDate(exp.createdAt)}</p>
                    {exp.stats && (
                      <div className="flex items-center gap-3 mt-1.5 flex-wrap">
                        <span className="text-xs text-muted-foreground">{exp.stats.locations} locations</span>
                        <span className="text-xs text-muted-foreground">{exp.stats.areas} areas</span>
                        <span className="text-xs text-muted-foreground">{exp.stats.commodities} items</span>
                        <span className="text-xs text-muted-foreground">{exp.stats.files} files</span>
                        <span className="text-xs text-muted-foreground">{exp.stats.sizeMb.toFixed(1)} MB</span>
                      </div>
                    )}
                    {exp.error && (
                      <p className="text-xs text-destructive mt-1 flex items-center gap-1">
                        <AlertTriangle className="size-3" /> {exp.error}
                      </p>
                    )}
                  </div>

                  {/* Delete — always visible on mobile, hover on desktop */}
                  <Button
                    size="icon" variant="ghost"
                    className="size-8 text-muted-foreground hover:text-destructive sm:opacity-0 sm:group-hover:opacity-100 transition-opacity shrink-0"
                    onClick={() => setDeleteTarget(exp)}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>

                {/* Actions row — always visible on mobile */}
                {exp.status === "completed" && (
                  <div className="flex items-center gap-2 sm:shrink-0 sm:opacity-0 sm:group-hover:opacity-100 transition-opacity">
                    <Button
                      size="sm" variant="outline" className="gap-1.5 flex-1 sm:flex-none"
                      onClick={() => {
                        setSelectedExport(exp)
                        setRestoreOpen(true)
                      }}
                    >
                      <RotateCcw className="size-3.5" />
                      Restore
                    </Button>
                    <Button size="sm" variant="outline" className="gap-1.5 flex-1 sm:flex-none">
                      <Download className="size-3.5" />
                      Download
                    </Button>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}

        {/* Pagination */}
        {exportsTotalPages > 1 && (
          <div className="flex flex-col items-center gap-3 sm:flex-row sm:justify-between mt-1">
            <p className="text-sm text-muted-foreground">
              Showing {(exportsPage - 1) * EXPORTS_PAGE_SIZE + 1}–{Math.min(exportsPage * EXPORTS_PAGE_SIZE, exports.length)} of {exports.length} exports
            </p>
            <Pagination className="w-auto mx-0">
              <PaginationContent>
                <PaginationItem>
                  <PaginationPrevious
                    onClick={() => setExportsPage((p) => Math.max(1, p - 1))}
                    aria-disabled={exportsPage === 1}
                    className={cn(exportsPage === 1 && "pointer-events-none opacity-50")}
                  />
                </PaginationItem>
                {Array.from({ length: exportsTotalPages }, (_, i) => i + 1).map((page) => {
                  const showPage =
                    page === 1 ||
                    page === exportsTotalPages ||
                    Math.abs(page - exportsPage) <= 1
                  const showEllipsisBefore = page === exportsPage - 2 && exportsPage - 2 > 1
                  const showEllipsisAfter = page === exportsPage + 2 && exportsPage + 2 < exportsTotalPages
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
                        isActive={page === exportsPage}
                        onClick={() => setExportsPage(page)}
                      >
                        {page}
                      </PaginationLink>
                    </PaginationItem>
                  )
                })}
                <PaginationItem>
                  <PaginationNext
                    onClick={() => setExportsPage((p) => Math.min(exportsTotalPages, p + 1))}
                    aria-disabled={exportsPage === exportsTotalPages}
                    className={cn(exportsPage === exportsTotalPages && "pointer-events-none opacity-50")}
                  />
                </PaginationItem>
              </PaginationContent>
            </Pagination>
          </div>
        )}
      </div>

      {/* Import Dialog */}
      <Dialog open={importOpen} onOpenChange={(v) => { if (!v && !importing) setImportOpen(false) }}>
        <DialogContent className="sm:max-w-md gap-0 p-0 overflow-hidden">
          <DialogHeader className="px-6 pt-6 pb-4">
            <DialogTitle>Import Backup File</DialogTitle>
            <DialogDescription>
              Upload an XML backup file exported from Inventario.
            </DialogDescription>
          </DialogHeader>

          <div className="px-6 pb-4">
            <div
              className={cn(
                "flex flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed py-10 transition-all cursor-pointer",
                dragOver
                  ? "border-primary bg-primary/5 scale-[1.01]"
                  : importing
                  ? "border-border bg-muted/30 cursor-default"
                  : "border-border bg-muted/30 hover:border-primary/50 hover:bg-muted/50"
              )}
              onDragOver={(e) => { e.preventDefault(); if (!importing) setDragOver(true) }}
              onDragLeave={() => setDragOver(false)}
              onDrop={(e) => { if (!importing) handleDrop(e) }}
              onClick={() => !importing && fileRef.current?.click()}
            >
              {importing ? (
                <>
                  <Loader2 className="size-10 text-primary animate-spin" />
                  <p className="text-sm text-muted-foreground">Importing backup…</p>
                </>
              ) : (
                <>
                  <div className={cn(
                    "flex size-12 items-center justify-center rounded-xl transition-colors",
                    dragOver ? "bg-primary/10" : "bg-muted"
                  )}>
                    <CloudUpload className={cn("size-6 transition-colors", dragOver ? "text-primary" : "text-muted-foreground")} strokeWidth={1.5} />
                  </div>
                  <div className="text-center">
                    <p className="text-sm font-medium">{dragOver ? "Drop file here" : "Drop file here"}</p>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      or <span className="text-foreground underline underline-offset-2">browse</span> to choose
                    </p>
                  </div>
                  <p className="text-xs text-muted-foreground">XML backup files from Inventario</p>
                </>
              )}
              <input ref={fileRef} type="file" accept=".xml" className="sr-only" onChange={(e) => { handleFileInput(e); }} />
            </div>
          </div>

          <div className="px-6 py-4 border-t border-border flex justify-end">
            <Button variant="outline" onClick={() => setImportOpen(false)} disabled={importing}>Cancel</Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Create Export Dialog */}
      <Dialog open={createOpen} onOpenChange={(v) => !v && setCreateOpen(false)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Create Export</DialogTitle>
            <DialogDescription>Choose what to include in the backup file.</DialogDescription>
          </DialogHeader>

          <div className="flex flex-col gap-3 py-1">
            <RadioGroup value={exportType} onValueChange={(v) => setExportType(v as ExportType)}>
              {EXPORT_TYPES.map((et) => (
                <label
                  key={et.value}
                  className={cn(
                    "flex items-start gap-3 rounded-lg border p-4 cursor-pointer transition-colors",
                    exportType === et.value
                      ? "border-primary bg-primary/5"
                      : "border-border hover:border-primary/30"
                  )}
                >
                  <RadioGroupItem value={et.value} className="mt-0.5" />
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-semibold">{et.label}</p>
                    <p className="text-xs text-muted-foreground mt-0.5">{et.description}</p>
                  </div>
                </label>
              ))}
            </RadioGroup>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button className="gap-2" onClick={handleCreateExport}>
              <HardDriveDownload className="size-4" />
              Create Export
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Restore Dialog */}
      <Dialog open={restoreOpen} onOpenChange={(v) => !v && setRestoreOpen(false)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Restore from Export</DialogTitle>
            {selectedExport && (
              <DialogDescription>
                {exportTypeLabel(selectedExport.type)} — {formatDate(selectedExport.createdAt)}
              </DialogDescription>
            )}
          </DialogHeader>

          <div className="flex flex-col gap-4 py-1">
            <div className="flex flex-col gap-2">
              <Label className="text-sm font-semibold">Restore Strategy</Label>
              <RadioGroup value={restoreStrategy} onValueChange={(v) => setRestoreStrategy(v as RestoreStrategy)}>
                {STRATEGIES.map((s) => (
                  <label
                    key={s.value}
                    className={cn(
                      "flex items-start gap-3 rounded-lg border p-4 cursor-pointer transition-colors",
                      restoreStrategy === s.value
                        ? "border-primary bg-primary/5"
                        : "border-border hover:border-primary/30"
                    )}
                  >
                    <RadioGroupItem value={s.value} className="mt-0.5" />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <p className="text-sm font-semibold">{s.label}</p>
                        <span className={cn(
                          "text-[10px] font-medium px-1.5 py-0.5 rounded-full",
                          s.risk === "low" && "bg-status-active/10 text-status-active",
                          s.risk === "medium" && "bg-status-expiring/10 text-status-expiring",
                          s.risk === "high" && "bg-destructive/10 text-destructive",
                        )}>
                          {s.risk === "low" ? "Safe" : s.risk === "medium" ? "Moderate risk" : "Destructive"}
                        </span>
                      </div>
                      <p className="text-xs text-muted-foreground mt-0.5">{s.description}</p>
                    </div>
                  </label>
                ))}
              </RadioGroup>
            </div>

            {restoreStrategy === "full_replace" && (
              <div className="flex items-start gap-3 rounded-lg border border-destructive/40 bg-destructive/5 p-4">
                <AlertTriangle className="size-4 text-destructive shrink-0 mt-0.5" />
                <p className="text-sm text-destructive">
                  <strong>Warning:</strong> This will permanently delete all current data before restoring. This action cannot be undone.
                </p>
              </div>
            )}

            <div className="flex items-center justify-between rounded-lg border border-border bg-muted/40 px-4 py-3">
              <div>
                <p className="text-sm font-medium">Dry Run</p>
                <p className="text-xs text-muted-foreground">Preview what would change without modifying data</p>
              </div>
              <Switch checked={dryRun} onCheckedChange={setDryRun} />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setRestoreOpen(false)}>Cancel</Button>
            <Button
              variant={restoreStrategy === "full_replace" ? "destructive" : "default"}
              className="gap-2"
              onClick={handleRestore}
            >
              <RotateCcw className="size-4" />
              {dryRun ? "Preview Restore" : "Restore Now"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Export Confirmation */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete export?</AlertDialogTitle>
            <AlertDialogDescription>
              The <span className="font-medium text-foreground">{deleteTarget ? exportTypeLabel(deleteTarget.type) : ""}</span> export from {deleteTarget ? formatDate(deleteTarget.createdAt) : ""} will be permanently deleted. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => { if (deleteTarget) { handleDelete(deleteTarget.id); setDeleteTarget(null) } }}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Restore Logs Dialog */}
      <Dialog open={restoreLogsOpen} onOpenChange={(v) => !v && setRestoreLogsOpen(false)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              {dryRun ? <Eye className="size-4" /> : <RotateCcw className="size-4" />}
              {dryRun ? "Restore Preview" : "Restore Complete"}
            </DialogTitle>
            <DialogDescription>
              {dryRun ? "No changes were made. Review what would happen." : "Restore operation finished. Review the log below."}
            </DialogDescription>
          </DialogHeader>

          <ScrollArea className="h-64 rounded-lg border border-border bg-muted/30 p-4">
            <div className="flex flex-col gap-1.5 font-mono text-xs">
              {MOCK_RESTORE_LOGS.map((log, i) => (
                <p key={i} className={cn("leading-relaxed", !log.ok && "text-destructive")}>
                  {log.icon} {log.text}
                </p>
              ))}
            </div>
          </ScrollArea>

          <DialogFooter>
            <Button onClick={() => setRestoreLogsOpen(false)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
