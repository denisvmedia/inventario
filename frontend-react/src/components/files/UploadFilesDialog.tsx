import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { CheckCircle2, FileIcon, Upload, X, XCircle } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useUploadFile } from "@/features/files/hooks"
import { checkUploadCapacity } from "@/features/files/api"
import { useAppToast } from "@/hooks/useAppToast"

type Step = "select" | "progress"

interface FileItem {
  id: string
  file: File
  status: "pending" | "uploading" | "done" | "failed"
  error?: string
}

// Two-step upload dialog. The metadata step from the issue is folded
// into the existing file detail/edit flow so users can refine title,
// category, and tags after upload — keeps this dialog focused on the
// transfer itself, which is where slot-gating + per-file progress
// matter. Per-file metadata edits land via a follow-up that's listed in
// the PR body.
export interface UploadFilesDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function UploadFilesDialog({ open, onOpenChange }: UploadFilesDialogProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const upload = useUploadFile()
  const [step, setStep] = useState<Step>("select")
  const [items, setItems] = useState<FileItem[]>([])
  const [dragOver, setDragOver] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const reset = useCallback(() => {
    setStep("select")
    setItems([])
  }, [])

  useEffect(() => {
    if (!open) reset()
  }, [open, reset])

  function addFiles(files: FileList | File[]) {
    const next: FileItem[] = Array.from(files).map((f) => ({
      id: `${f.name}-${f.size}-${f.lastModified}-${Math.random().toString(36).slice(2, 8)}`,
      file: f,
      status: "pending",
    }))
    setItems((prev) => [...prev, ...next])
  }

  function removeItem(id: string) {
    setItems((prev) => prev.filter((it) => it.id !== id))
  }

  async function start() {
    if (items.length === 0) {
      toast.error(t("files:upload.errors.noFiles"))
      return
    }
    let capacity
    try {
      capacity = await checkUploadCapacity()
    } catch {
      toast.error(t("files:upload.slotCheckFailed"))
      return
    }
    if (!capacity.canStart) {
      toast.warning(
        t("files:upload.slotBusy", {
          seconds: capacity.retryAfterSeconds ?? "?",
        })
      )
      return
    }
    setStep("progress")
    let succeeded = 0
    let failed = 0
    for (const item of items) {
      setItems((prev) =>
        prev.map((it) => (it.id === item.id ? { ...it, status: "uploading" } : it))
      )
      try {
        await upload.mutateAsync(item.file)
        succeeded++
        setItems((prev) =>
          prev.map((it) => (it.id === item.id ? { ...it, status: "done" } : it))
        )
      } catch (err) {
        failed++
        setItems((prev) =>
          prev.map((it) =>
            it.id === item.id
              ? {
                  ...it,
                  status: "failed",
                  error: err instanceof Error ? err.message : String(err),
                }
              : it
          )
        )
      }
    }
    if (failed === 0) {
      toast.success(t("files:upload.success", { count: succeeded }))
    } else {
      toast.warning(t("files:upload.partial", { succeeded, failed }))
    }
  }

  const totalDone = items.filter((it) => it.status === "done").length
  const allDone = step === "progress" && items.every((it) => it.status === "done" || it.status === "failed")

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[80vh] sm:max-w-2xl" data-testid="files-upload-dialog">
        <DialogHeader>
          <DialogTitle>{t("files:upload.title")}</DialogTitle>
          <DialogDescription>
            {step === "select"
              ? t("files:upload.step1DropHint")
              : t("files:upload.step3Title")}
          </DialogDescription>
        </DialogHeader>

        {step === "select" ? (
          <div className="flex flex-col gap-3">
            <div
              role="button"
              tabIndex={0}
              data-testid="files-upload-dropzone"
              onDragOver={(e) => {
                e.preventDefault()
                setDragOver(true)
              }}
              onDragLeave={() => setDragOver(false)}
              onDrop={(e) => {
                e.preventDefault()
                setDragOver(false)
                if (e.dataTransfer.files?.length) addFiles(e.dataTransfer.files)
              }}
              onClick={() => fileInputRef.current?.click()}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault()
                  fileInputRef.current?.click()
                }
              }}
              className={`flex cursor-pointer flex-col items-center justify-center gap-2 rounded-md border-2 border-dashed p-8 text-sm transition-colors ${
                dragOver ? "border-primary bg-primary/5" : "border-input"
              }`}
            >
              <Upload className="size-8 text-muted-foreground" aria-hidden="true" />
              <p>{t("files:upload.step1DropHint")}</p>
              <Button type="button" variant="outline" size="sm">
                {t("files:upload.step1Browse")}
              </Button>
              <input
                ref={fileInputRef}
                type="file"
                multiple
                hidden
                data-testid="files-upload-input"
                onChange={(e) => {
                  if (e.target.files?.length) addFiles(e.target.files)
                  e.target.value = ""
                }}
              />
            </div>
            {items.length > 0 ? (
              <ul className="max-h-64 divide-y overflow-y-auto rounded-md border" data-testid="files-upload-list">
                {items.map((it) => (
                  <li key={it.id} className="flex items-center gap-2 px-3 py-2 text-sm">
                    <FileIcon className="size-4 text-muted-foreground" aria-hidden="true" />
                    <span className="flex-1 truncate">{it.file.name}</span>
                    <button
                      type="button"
                      className="text-muted-foreground hover:text-foreground"
                      aria-label={t("files:upload.step1RemoveFile", { name: it.file.name })}
                      onClick={() => removeItem(it.id)}
                    >
                      <X className="size-4" aria-hidden="true" />
                    </button>
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            <div
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={items.length}
              aria-valuenow={totalDone}
              data-testid="files-upload-progress"
              className="h-2 w-full overflow-hidden rounded-full bg-muted"
            >
              <div
                className="h-full bg-primary transition-[width] duration-200"
                style={{
                  width: `${items.length === 0 ? 0 : (totalDone / items.length) * 100}%`,
                }}
              />
            </div>
            <p className="text-sm text-muted-foreground">
              {t("files:upload.uploadDone", {
                count: totalDone,
                total: items.length,
                defaultValue: "{{count}} of {{total}} uploaded",
              })}
            </p>
            <ul className="max-h-64 divide-y overflow-y-auto rounded-md border">
              {items.map((it) => (
                <li
                  key={it.id}
                  className="flex items-center gap-2 px-3 py-2 text-sm"
                  data-testid={`files-upload-item-${it.status}`}
                >
                  {it.status === "done" ? (
                    <CheckCircle2 className="size-4 text-emerald-500" aria-hidden="true" />
                  ) : it.status === "failed" ? (
                    <XCircle className="size-4 text-destructive" aria-hidden="true" />
                  ) : (
                    <FileIcon className="size-4 text-muted-foreground" aria-hidden="true" />
                  )}
                  <span className="flex-1 truncate">
                    {it.status === "uploading"
                      ? t("files:upload.uploading", { name: it.file.name })
                      : it.file.name}
                  </span>
                  {it.error ? (
                    <span className="text-xs text-destructive" title={it.error}>
                      {t("files:upload.uploadFailed")}
                    </span>
                  ) : null}
                </li>
              ))}
            </ul>
          </div>
        )}

        <DialogFooter>
          {step === "select" ? (
            <>
              <Button variant="outline" onClick={() => onOpenChange(false)}>
                {t("common:actions.cancel")}
              </Button>
              <Button
                onClick={start}
                disabled={items.length === 0 || upload.isPending}
                data-testid="files-upload-start"
              >
                {t("files:upload.startUpload", { count: items.length })}
              </Button>
            </>
          ) : (
            <Button
              onClick={() => onOpenChange(false)}
              disabled={!allDone}
              data-testid="files-upload-close"
            >
              {t("files:upload.close")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
