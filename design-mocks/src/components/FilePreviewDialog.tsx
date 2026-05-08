import { useState } from "react"
import { Dialog, DialogContent } from "@/components/ui/dialog"
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
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Download, File, Trash2 } from "lucide-react"
import { FILE_TAGS, type AttachedFile } from "@/data/mock"
import { PdfViewerView } from "@/views/PdfViewerView"
import { ImageViewerView } from "@/views/ImageViewerView"

interface FilePreviewDialogProps {
  file: AttachedFile | null
  onClose: () => void
  onDelete?: (fileId: string) => void
}

function fileType(mimeType: string): "image" | "pdf" | "other" {
  if (mimeType.startsWith("image/")) return "image"
  if (mimeType === "application/pdf") return "pdf"
  return "other"
}

export function FilePreviewDialog({ file, onClose, onDelete }: FilePreviewDialogProps) {
  const [confirmDelete, setConfirmDelete] = useState(false)

  if (!file) return null

  const type = fileType(file.mimeType)

  function handleDelete() {
    onDelete?.(file!.id)
    setConfirmDelete(false)
    onClose()
  }

  // Full-viewport dialog for image and PDF viewers
  if (type === "pdf") {
    return (
      <>
        <Dialog open onOpenChange={(open) => !open && onClose()}>
          <DialogContent className="max-w-none w-screen h-screen p-0 rounded-none border-0 gap-0 [&>button]:hidden">
            <PdfViewerView
              onClose={onClose}
              file={file}
              onDelete={onDelete ? () => setConfirmDelete(true) : undefined}
            />
          </DialogContent>
        </Dialog>

        <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete file?</AlertDialogTitle>
              <AlertDialogDescription>
                <span className="font-medium text-foreground">{file.name}</span> will be permanently deleted. This cannot be undone.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                Delete
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </>
    )
  }

  if (type === "image") {
    return (
      <>
        <Dialog open onOpenChange={(open) => !open && onClose()}>
          <DialogContent className="max-w-none w-screen h-screen p-0 rounded-none border-0 gap-0 [&>button]:hidden">
            <ImageViewerView
              onClose={onClose}
              file={file}
              onDelete={onDelete ? () => setConfirmDelete(true) : undefined}
            />
          </DialogContent>
        </Dialog>

        <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete file?</AlertDialogTitle>
              <AlertDialogDescription>
                <span className="font-medium text-foreground">{file.name}</span> will be permanently deleted. This cannot be undone.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                Delete
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </>
    )
  }

  // "other" — small non-previewable dialog
  const tagObjs = file.tags.map((tid) => FILE_TAGS.find((t) => t.id === tid)).filter(Boolean)

  return (
    <>
      <Dialog open onOpenChange={(open) => !open && onClose()}>
        <DialogContent className="sm:max-w-md">
          <div className="flex items-start gap-3 pr-6">
            <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-muted">
              <File className="size-5 text-muted-foreground" />
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-base font-semibold leading-snug truncate">{file.name}</p>
              <p className="text-sm text-muted-foreground mt-0.5 flex items-center flex-wrap gap-x-2 gap-y-1">
                <span>{file.size}</span>
                <span>·</span>
                <span>Uploaded {file.uploadedAt}</span>
                {tagObjs.map((t) => t && (
                  <Badge key={t.id} variant="secondary" className={`h-4 px-1.5 text-[10px] ${t.color}`}>
                    {t.label}
                  </Badge>
                ))}
              </p>
            </div>
          </div>

          <div className="flex flex-col items-center justify-center gap-4 py-8 px-6 text-center rounded-xl border border-border bg-muted/40 mt-1">
            <div className="flex size-14 items-center justify-center rounded-xl bg-background border border-border">
              <File className="size-7 text-muted-foreground" />
            </div>
            <p className="text-xs text-muted-foreground max-w-xs">
              This file type cannot be previewed in the browser.
            </p>
            <Button size="sm" className="gap-1.5">
              <Download className="size-3.5" />
              Download
            </Button>
          </div>

          {onDelete && (
            <div className="flex justify-end pt-1">
              <Button
                variant="ghost"
                size="sm"
                className="gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive"
                onClick={() => setConfirmDelete(true)}
              >
                <Trash2 className="size-3.5" />
                Delete file
              </Button>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete file?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-medium text-foreground">{file.name}</span> will be permanently deleted. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
