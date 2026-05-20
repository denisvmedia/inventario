import { useState } from "react"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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
  FileText,
  Image as ImageIcon,
  FileArchive,
  File,
  Receipt,
  BookOpen,
  Download,
  Trash2,
  ExternalLink,
  Calendar,
  HardDrive,
  Tag,
  Link as LinkIcon,
  Pencil,
  Layers,
} from "lucide-react"
import { FilePreviewDialog } from "@/components/FilePreviewDialog"
import { FILE_TAGS, FILE_CATEGORY_CONFIG, resolveTags, type AttachedFile, type FileCategory } from "@/data/mock"
import { TagPill } from "@/components/TagPill"
import { cn } from "@/lib/utils"

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

const MIME_LABELS: Record<MimeGroup, string> = {
  pdf: "PDF Document",
  image: "Image",
  archive: "Archive",
  other: "File",
}

function formatDate(d: string) {
  return new Date(d).toLocaleDateString("en-US", { year: "numeric", month: "long", day: "numeric" })
}

interface DetailRowProps {
  icon: React.ElementType
  label: string
  value: React.ReactNode
}

function DetailRow({ icon: Icon, label, value }: DetailRowProps) {
  return (
    <div className="flex items-start gap-3 py-2.5">
      <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <div className="flex-1 min-w-0">
        <p className="text-xs text-muted-foreground mb-0.5">{label}</p>
        <div className="text-sm font-medium break-all">{value}</div>
      </div>
    </div>
  )
}

// ─── Edit Metadata Dialog ─────────────────────────────────────────────────────

const CATEGORY_ICONS: Record<FileCategory, React.ElementType> = {
  image: ImageIcon,
  invoice: Receipt,
  document: BookOpen,
  other: File,
}

const CATEGORY_COLORS: Record<FileCategory, string> = {
  image:    "text-status-active",
  invoice:  "text-chart-1",
  document: "text-chart-3",
  other:    "text-muted-foreground",
}

interface EditMetadataDialogProps {
  open: boolean
  file: AttachedFile
  onClose: () => void
}

function EditMetadataDialog({ open, file, onClose }: EditMetadataDialogProps) {
  const [name, setName] = useState(file.name)
  const [selectedTags, setSelectedTags] = useState<string[]>(file.tags)
  const [category, setCategory] = useState<FileCategory>(file.category ?? "other")

  function toggleTag(id: string) {
    setSelectedTags((prev) =>
      prev.includes(id) ? prev.filter((t) => t !== id) : [...prev, id]
    )
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Edit file metadata</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-1">
          <div className="space-y-1.5">
            <Label htmlFor="file-name">File name</Label>
            <Input
              id="file-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>

          <div className="space-y-1.5">
            <Label>Category</Label>
            <div className="grid grid-cols-4 gap-1.5">
              {(Object.keys(FILE_CATEGORY_CONFIG) as FileCategory[]).map((cat) => {
                const CatIcon = CATEGORY_ICONS[cat]
                const cfg = FILE_CATEGORY_CONFIG[cat]
                const active = category === cat
                return (
                  <button
                    key={cat}
                    type="button"
                    onClick={() => setCategory(cat)}
                    className={cn(
                      "flex flex-col items-center gap-1 rounded-lg border py-2.5 px-1 text-xs font-medium transition-colors",
                      active
                        ? "border-primary bg-primary/5 text-foreground"
                        : "border-border bg-background text-muted-foreground hover:border-primary/40"
                    )}
                  >
                    <CatIcon className={cn("size-4", active ? CATEGORY_COLORS[cat] : "text-muted-foreground")} />
                    {cfg.label}
                  </button>
                )
              })}
            </div>
          </div>

          <div className="space-y-1.5">
            <Label>Tags</Label>
            <div className="flex flex-wrap gap-1.5">
              {FILE_TAGS.map((tag) => (
                <button
                  key={tag.id}
                  type="button"
                  onClick={() => toggleTag(tag.id)}
                  className={cn(
                    "rounded-full border px-2.5 py-0.5 text-xs font-medium transition-colors",
                    selectedTags.includes(tag.id)
                      ? "border-primary bg-primary text-primary-foreground"
                      : "border-border bg-background text-muted-foreground hover:border-primary/50"
                  )}
                >
                  {tag.label}
                </button>
              ))}
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={onClose}>Save</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface FileDetailSheetProps {
  file: AttachedFile | null
  onClose: () => void
  onDelete: (id: string) => void
}

export function FileDetailSheet({ file, onClose, onDelete }: FileDetailSheetProps) {
  const [previewOpen, setPreviewOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)

  const group = file ? mimeGroup(file.mimeType) : "other"
  const Icon = FILE_ICONS[group]
  const fileTags = file ? resolveTags(file.tags) : []

  function handleDelete() {
    if (!file) return
    onDelete(file.id)
    setDeleteOpen(false)
    onClose()
  }

  return (
    <>
      <Sheet open={!!file} onOpenChange={(open) => !open && onClose()}>
        <SheetContent className="w-full sm:max-w-md flex flex-col gap-0 overflow-y-auto p-0">
          {file && (
            <div className="flex flex-col gap-0 px-5 pb-5">
              {/* Preview area */}
              <div className="relative w-full aspect-video overflow-hidden rounded-b-xl -mx-5 w-[calc(100%+2.5rem)]">
                {file.thumbnailUrl ? (
                  <img
                    src={file.thumbnailUrl}
                    alt={file.name}
                    className="absolute inset-0 w-full h-full object-cover"
                  />
                ) : (
                  <div className={cn("absolute inset-0 flex items-center justify-center", FILE_BG[group])}>
                    <Icon className={cn("size-16 opacity-70", FILE_COLORS[group])} strokeWidth={1.25} />
                  </div>
                )}
              </div>

              {/* Header */}
              <SheetHeader className="pt-5 pb-4 px-0">
                <div className="flex items-start gap-3">
                  <div className={cn("flex size-10 shrink-0 items-center justify-center rounded-lg", FILE_BG[group])}>
                    <Icon className={cn("size-5", FILE_COLORS[group])} />
                  </div>
                  <div className="flex-1 min-w-0 pr-6">
                    <SheetTitle className="text-base leading-snug break-all">{file.name}</SheetTitle>
                    <SheetDescription className="mt-0.5 text-xs">
                      {MIME_LABELS[group]} · {file.size}
                    </SheetDescription>
                  </div>
                </div>

                {fileTags.length > 0 && (
                  <div className="flex flex-wrap gap-1.5 pt-1">
                    {fileTags.map((tag) => (
                      <TagPill key={tag.id} tag={tag} size="sm" />
                    ))}
                  </div>
                )}
              </SheetHeader>

              {/* Action buttons */}
              <div className="flex gap-2 pb-5">
                <Button
                  variant="default"
                  size="sm"
                  className="flex-1 gap-1.5"
                  onClick={() => setPreviewOpen(true)}
                >
                  <ExternalLink className="size-3.5" />
                  {file.mimeType.startsWith("image/") ? "View" : file.mimeType === "application/pdf" ? "Open" : "Download"}
                </Button>
                <Button variant="outline" size="sm" className="gap-1.5" onClick={() => setEditOpen(true)}>
                  <Pencil className="size-3.5" />
                  Edit
                </Button>
                <Button variant="outline" size="sm" className="gap-1.5">
                  <Download className="size-3.5" />
                  Download
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  className="text-destructive hover:bg-destructive/10 hover:text-destructive border-border"
                  onClick={() => setDeleteOpen(true)}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>

              <Separator />

              {/* Metadata */}
              <div className="mt-1">
                <DetailRow
                  icon={LinkIcon}
                  label="Attached to"
                  value={file.attachedTo.name}
                />
                <Separator />
                <DetailRow
                  icon={Calendar}
                  label="Uploaded"
                  value={formatDate(file.uploadedAt)}
                />
                <Separator />
                <DetailRow
                  icon={HardDrive}
                  label="File size"
                  value={file.size}
                />
                <Separator />
                <DetailRow
                  icon={FileText}
                  label="Type"
                  value={file.mimeType}
                />
                {file.category && (
                  <>
                    <Separator />
                    <DetailRow
                      icon={Layers}
                      label="Category"
                      value={(() => {
                        const cat = file.category as FileCategory
                        const CatIcon = CATEGORY_ICONS[cat]
                        return (
                          <span className={cn("inline-flex items-center gap-1", CATEGORY_COLORS[cat])}>
                            <CatIcon className="size-3.5" />
                            {FILE_CATEGORY_CONFIG[cat].label}
                          </span>
                        )
                      })()}
                    />
                  </>
                )}
                {fileTags.length > 0 && (
                  <>
                    <Separator />
                    <DetailRow
                      icon={Tag}
                      label="Tags"
                      value={
                        <div className="flex flex-wrap gap-1 mt-0.5">
                          {fileTags.map((tag) => (
                            <TagPill key={tag.id} tag={tag} size="xs" />
                          ))}
                        </div>
                      }
                    />
                  </>
                )}
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>

      {file && (
        <FilePreviewDialog
          file={previewOpen ? file : null}
          onClose={() => setPreviewOpen(false)}
          onDelete={(id) => { onDelete(id); setPreviewOpen(false); onClose() }}
        />
      )}

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete file?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-medium text-foreground">{file?.name}</span> will be permanently deleted. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {file && editOpen && (
        <EditMetadataDialog
          open={editOpen}
          file={file}
          onClose={() => setEditOpen(false)}
        />
      )}
    </>
  )
}
