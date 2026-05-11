import { useState, useEffect } from "react"
import { Upload, X, FileText, Image, File, Paperclip } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { cn, makeId } from "@/lib/utils"
import type { Location } from "@/data/mock"

const LOCATION_ICONS = ["🏠", "🏡", "🏢", "🏗️", "🏚️", "🚗", "🌿", "🌊", "⛺", "🏕️", "🔑", "📦"]

interface AttachedFileDraft {
  id: string
  name: string
  size: string
  mimeType: string
}

interface LocationDialogProps {
  open: boolean
  onClose: () => void
  /** If provided, dialog is in edit mode */
  location?: Location | null
  onSave: (data: { name: string; icon: string; description: string }) => void
}

export function LocationDialog({ open, onClose, location, onSave }: LocationDialogProps) {
  const isEdit = !!location

  const [name, setName] = useState("")
  const [icon, setIcon] = useState("🏠")
  const [description, setDescription] = useState("")
  const [attachedFiles, setAttachedFiles] = useState<AttachedFileDraft[]>([])
  const [dragging, setDragging] = useState(false)

  useEffect(() => {
    if (open) {
      setName(location?.name ?? "")
      setIcon(location?.icon ?? "🏠")
      setDescription(location?.description ?? "")
      setAttachedFiles([])
    }
  }, [open, location])

  function handleSave() {
    if (!name.trim()) return
    onSave({ name: name.trim(), icon, description: description.trim() })
  }

  function handleFileInput(e: React.ChangeEvent<HTMLInputElement>) {
    const files = Array.from(e.target.files ?? [])
    addFiles(files)
    e.target.value = ""
  }

  function addFiles(files: File[]) {
    const drafts: AttachedFileDraft[] = files.map((f) => ({
      id: makeId(),
      name: f.name,
      size: formatSize(f.size),
      mimeType: f.type,
    }))
    setAttachedFiles((prev) => [...prev, ...drafts])
  }

  function removeFile(id: string) {
    setAttachedFiles((prev) => prev.filter((f) => f.id !== id))
  }

  function formatSize(bytes: number) {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }

  function fileIcon(mimeType: string) {
    if (mimeType === "application/pdf") return <FileText className="size-4 text-status-expired" />
    if (mimeType.startsWith("image/")) return <Image className="size-4 text-status-active" />
    return <File className="size-4 text-muted-foreground" />
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit location" : "Add location"}</DialogTitle>
          <DialogDescription>
            {isEdit ? "Update the details for this location." : "Add a new physical location to your group."}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-5 py-1">
          {/* Icon picker */}
          <div className="flex flex-col gap-2">
            <Label>Icon</Label>
            <div className="flex flex-wrap gap-1.5">
              {LOCATION_ICONS.map((ic) => (
                <button
                  key={ic}
                  type="button"
                  onClick={() => setIcon(ic)}
                  className={cn(
                    "flex size-9 items-center justify-center rounded-lg text-xl transition-all border",
                    icon === ic
                      ? "border-primary bg-primary/10 scale-110"
                      : "border-border bg-muted hover:border-primary/40"
                  )}
                >
                  {ic}
                </button>
              ))}
            </div>
          </div>

          {/* Name */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="loc-name">Name <span className="text-destructive">*</span></Label>
            <Input
              id="loc-name"
              placeholder="e.g. Main House, Garage, Cottage…"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
          </div>

          {/* Description */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="loc-desc">Description</Label>
            <Textarea
              id="loc-desc"
              placeholder="Short description of this location…"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={2}
              className="resize-none"
            />
          </div>

          <Separator />

          {/* File attachments */}
          <div className="flex flex-col gap-2">
            <Label className="flex items-center gap-1.5">
              <Paperclip className="size-3.5" />
              Attachments
            </Label>

            {/* Drop zone */}
            <div
              className={cn(
                "relative flex flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed py-6 transition-colors cursor-pointer",
                dragging ? "border-primary bg-primary/5" : "border-border hover:border-primary/40 hover:bg-muted/30"
              )}
              onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
              onDragLeave={() => setDragging(false)}
              onDrop={(e) => {
                e.preventDefault()
                setDragging(false)
                addFiles(Array.from(e.dataTransfer.files))
              }}
              onClick={() => document.getElementById("loc-file-input")?.click()}
            >
              <Upload className="size-5 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                Drop files here or <span className="text-foreground font-medium">browse</span>
              </p>
              <p className="text-xs text-muted-foreground">PDFs, images, documents</p>
              <input
                id="loc-file-input"
                type="file"
                multiple
                className="sr-only"
                onChange={handleFileInput}
              />
            </div>

            {/* File list */}
            {attachedFiles.length > 0 && (
              <ul className="flex flex-col gap-1 mt-1">
                {attachedFiles.map((f) => (
                  <li key={f.id} className="flex items-center gap-2 rounded-lg border border-border bg-card px-3 py-2">
                    {fileIcon(f.mimeType)}
                    <span className="flex-1 text-sm truncate min-w-0">{f.name}</span>
                    <span className="text-xs text-muted-foreground shrink-0">{f.size}</span>
                    <button
                      type="button"
                      onClick={() => removeFile(f.id)}
                      className="text-muted-foreground hover:text-foreground transition-colors shrink-0"
                    >
                      <X className="size-3.5" />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={handleSave} disabled={!name.trim()}>
            {isEdit ? "Save changes" : "Add location"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
