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
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { checkUploadCapacity, updateFile, type FileCategory } from "@/features/files/api"
import { categoryFromMime } from "@/features/files/constants"
import { useInvalidateFiles, useUploadFile } from "@/features/files/hooks"
import { useAppToast } from "@/hooks/useAppToast"

type Step = "select" | "metadata" | "progress"

interface FileItem {
  id: string
  file: File
  // Per-file metadata captured in step 2 ("metadata"). Title defaults
  // to the dropped filename without extension; category is derived from
  // MIME type but the user can override before the upload kicks off.
  title: string
  category: FileCategory
  status: "pending" | "uploading" | "done" | "failed"
  error?: string
}

// Three-step upload dialog matching #1411's spec:
//   1. select   — drag-drop / browse, accumulate file list.
//   2. metadata — per-file title + category override before upload.
//   3. progress — slot-gate, then upload sequentially with live status.
//
// Cache invalidation is deferred until the batch finishes — `useUploadFile`
// no longer auto-invalidates per-mutation, so a 20-file upload doesn't
// trigger 20 list/counts refetches while the dialog is still open.
//
// `linkedEntity` (#1448) preselects a commodity / location as the parent
// of every uploaded file: after the multipart POST returns the new file
// id, we PUT /files/{id} with `linked_entity_type` + `linked_entity_id`
// so the file is attached, not orphaned. Linking failure marks the
// item as failed (since the user's intent was "attach"); a metadata-
// only failure when not linking stays non-fatal.
//
// `initialFiles` lets the page-level drop catcher pre-queue files into
// the dialog so the user doesn't have to drop again — the dialog opens
// in the select step with files already listed.
export interface UploadFilesDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  linkedEntity?: {
    type: "commodity" | "location"
    id: string
    name?: string
  }
  initialFiles?: File[]
}

export function UploadFilesDialog({
  open,
  onOpenChange,
  linkedEntity,
  initialFiles,
}: UploadFilesDialogProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const upload = useUploadFile()
  const invalidate = useInvalidateFiles()
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

  // Pre-queue files passed in by the page-level drop catcher. We seed
  // once per `open` transition so a re-render with the same array does
  // not duplicate items. The effect intentionally depends on `open`
  // and the array identity — callers should pass a stable array (state)
  // for the lifetime of the dialog, not a fresh `[...]` on every render.
  useEffect(() => {
    if (!open) return
    if (!initialFiles?.length) return
    setItems((prev) => {
      if (prev.length > 0) return prev
      return initialFiles.map((f) => ({
        id: `${f.name}-${f.size}-${f.lastModified}-${Math.random().toString(36).slice(2, 8)}`,
        file: f,
        title: defaultTitle(f.name),
        category: categoryFromMime(f.type),
        status: "pending",
      }))
    })
  }, [open, initialFiles])

  function defaultTitle(name: string): string {
    const lastDot = name.lastIndexOf(".")
    return lastDot > 0 ? name.slice(0, lastDot) : name
  }

  function addFiles(files: FileList | File[]) {
    const next: FileItem[] = Array.from(files).map((f) => ({
      id: `${f.name}-${f.size}-${f.lastModified}-${Math.random().toString(36).slice(2, 8)}`,
      file: f,
      title: defaultTitle(f.name),
      category: categoryFromMime(f.type),
      status: "pending",
    }))
    setItems((prev) => [...prev, ...next])
  }

  function removeItem(id: string) {
    setItems((prev) => prev.filter((it) => it.id !== id))
  }

  function patchItem(id: string, patch: Partial<FileItem>) {
    setItems((prev) => prev.map((it) => (it.id === id ? { ...it, ...patch } : it)))
  }

  async function startUpload() {
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
      patchItem(item.id, { status: "uploading" })
      try {
        const result = await upload.mutateAsync(item.file)
        // Apply per-file metadata overrides only if they differ from
        // the BE-derived defaults — avoids a no-op PUT for the common
        // "user accepted defaults" path. When `linkedEntity` is set,
        // we ALWAYS PUT to attach the file (the BE upload endpoint
        // does not read linked_entity_* off the multipart form), so
        // the file becomes a child of the commodity / location instead
        // of an orphan.
        const titleChanged = item.title !== defaultTitle(item.file.name)
        const categoryChanged = item.category !== result.file.category
        const needsLinking = !!linkedEntity
        if (titleChanged || categoryChanged || needsLinking) {
          try {
            await updateFile(result.file.id, {
              title: item.title,
              category: item.category,
              ...(linkedEntity
                ? {
                    linked_entity_type: linkedEntity.type,
                    linked_entity_id: linkedEntity.id,
                  }
                : {}),
            })
          } catch (err) {
            if (needsLinking) {
              // Linking is the user's stated intent ("attach files to
              // this commodity"). If it fails the file ended up as
              // an orphan on disk — surface that as a per-item failure
              // so the user can retry from the global Files page
              // instead of silently believing the attach worked.
              failed++
              patchItem(item.id, {
                status: "failed",
                error: err instanceof Error ? err.message : String(err),
              })
              continue
            }
            // Metadata-only failure (no linking requested): file is on
            // disk; the user can edit it from the detail sheet later.
          }
        }
        succeeded++
        patchItem(item.id, { status: "done" })
      } catch (err) {
        failed++
        patchItem(item.id, {
          status: "failed",
          error: err instanceof Error ? err.message : String(err),
        })
      }
    }
    // Single batch invalidation — list + counts refetch once after the
    // last upload settles, not on every per-file mutation.
    invalidate.all()
    if (failed === 0) {
      toast.success(t("files:upload.success", { count: succeeded }))
    } else {
      toast.warning(t("files:upload.partial", { succeeded, failed }))
    }
  }

  const totalDone = items.filter((it) => it.status === "done").length
  const allDone =
    step === "progress" && items.every((it) => it.status === "done" || it.status === "failed")

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[80vh] sm:max-w-2xl" data-testid="files-upload-dialog">
        <DialogHeader>
          <DialogTitle>
            {linkedEntity?.name
              ? t("files:upload.attachTitleWithName", { name: linkedEntity.name })
              : linkedEntity
                ? t("files:upload.attachTitle")
                : t("files:upload.title")}
          </DialogTitle>
          <DialogDescription>
            {step === "select"
              ? t("files:upload.step1DropHint")
              : step === "metadata"
                ? t("files:upload.step2Hint")
                : t("files:upload.step3Title")}
          </DialogDescription>
        </DialogHeader>

        {step === "select" ? (
          <SelectStep
            items={items}
            dragOver={dragOver}
            setDragOver={setDragOver}
            fileInputRef={fileInputRef}
            addFiles={addFiles}
            removeItem={removeItem}
          />
        ) : step === "metadata" ? (
          <MetadataStep items={items} onPatch={patchItem} />
        ) : (
          <ProgressStep items={items} totalDone={totalDone} />
        )}

        <DialogFooter>
          {step === "select" ? (
            <>
              <Button variant="outline" onClick={() => onOpenChange(false)}>
                {t("common:actions.cancel")}
              </Button>
              <Button
                onClick={() => setStep("metadata")}
                disabled={items.length === 0}
                data-testid="files-upload-next"
              >
                {t("files:upload.step1NextWith", { count: items.length })}
              </Button>
            </>
          ) : step === "metadata" ? (
            <>
              <Button variant="outline" onClick={() => setStep("select")}>
                {t("files:upload.back")}
              </Button>
              <Button
                onClick={startUpload}
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

interface SelectStepProps {
  items: FileItem[]
  dragOver: boolean
  setDragOver: (v: boolean) => void
  fileInputRef: React.RefObject<HTMLInputElement | null>
  addFiles: (files: FileList | File[]) => void
  removeItem: (id: string) => void
}

function SelectStep({
  items,
  dragOver,
  setDragOver,
  fileInputRef,
  addFiles,
  removeItem,
}: SelectStepProps) {
  const { t } = useTranslation()
  return (
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
        <ul
          className="max-h-64 divide-y overflow-y-auto rounded-md border"
          data-testid="files-upload-list"
        >
          {items.map((it) => (
            <li
              key={it.id}
              className="flex items-center gap-2 px-3 py-2 text-sm"
              data-testid={`files-upload-list-item-${it.id}`}
            >
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
  )
}

interface MetadataStepProps {
  items: FileItem[]
  onPatch: (id: string, patch: Partial<FileItem>) => void
}

function MetadataStep({ items, onPatch }: MetadataStepProps) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col gap-3">
      <ul
        className="max-h-[60vh] divide-y overflow-y-auto rounded-md border"
        data-testid="files-upload-metadata-list"
      >
        {items.map((it) => (
          <li
            key={it.id}
            className="flex flex-col gap-2 p-3 text-sm sm:flex-row sm:items-end"
            data-testid={`files-upload-metadata-item-${it.id}`}
          >
            <div className="flex-1">
              <Label htmlFor={`meta-title-${it.id}`} className="text-xs text-muted-foreground">
                {it.file.name}
              </Label>
              <Input
                id={`meta-title-${it.id}`}
                value={it.title}
                onChange={(e) => onPatch(it.id, { title: e.target.value })}
                data-testid={`files-upload-meta-title-${it.id}`}
              />
            </div>
            <div className="sm:w-44">
              <Label htmlFor={`meta-category-${it.id}`} className="text-xs text-muted-foreground">
                {t("files:edit.fields.category")}
              </Label>
              <select
                id={`meta-category-${it.id}`}
                value={it.category}
                onChange={(e) => onPatch(it.id, { category: e.target.value as FileCategory })}
                data-testid={`files-upload-meta-category-${it.id}`}
                className="h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm"
              >
                <option value="photos">
                  {t("files:categoryPhotos", { defaultValue: "Photos" })}
                </option>
                <option value="invoices">
                  {t("files:categoryInvoices", { defaultValue: "Invoices" })}
                </option>
                <option value="documents">
                  {t("files:categoryDocuments", { defaultValue: "Documents" })}
                </option>
                <option value="other">{t("files:categoryOther", { defaultValue: "Other" })}</option>
              </select>
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}

interface ProgressStepProps {
  items: FileItem[]
  totalDone: number
}

function ProgressStep({ items, totalDone }: ProgressStepProps) {
  const { t } = useTranslation()
  return (
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
            // Composite testid: stable per-item identifier + status
            // suffix so Playwright/RTL selectors can match a specific
            // file's progress row without ambiguity. (The earlier
            // status-only testid was non-unique across multiple files
            // sharing a status.)
            data-testid={`files-upload-progress-item-${it.id}`}
            data-status={it.status}
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
  )
}
