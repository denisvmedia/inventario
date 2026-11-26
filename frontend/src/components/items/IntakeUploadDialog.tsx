import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { AlertCircle, CheckCircle2, FileText, Loader2, Plus, X } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import {
  createIntakeSession,
  INTAKE_QUEUE_FULL,
  intakeUploadTarget,
  isIntakeError,
  uploadIntakeFile,
  type IntakeLimits,
  type IntakeSessionDetail,
  type IntakeUploadTarget,
} from "@/features/intake/api"
import { useInvalidateIntake } from "@/features/intake/hooks"
import { trashUnlinkedFile } from "@/features/files/api"
import { formatBytes, formatList } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"
import { cn } from "@/lib/utils"

// Why a dropped file is not sent.
type Rejection = "heic" | "type" | "size" | "count"

// HEIC and HEIF photos, which iPhones take by default. The scan cannot read
// them, so they get a reason of their own that says what to do instead.
const HEIC_TYPES = new Set([
  "image/heic",
  "image/heif",
  "image/heic-sequence",
  "image/heif-sequence",
])

// Short names of the file types the scan reads, for the reason a file is
// not sent.
const TYPE_LABELS: Record<string, string> = {
  "application/pdf": "PDF",
  "image/jpeg": "JPEG",
  "image/png": "PNG",
  "image/webp": "WebP",
  "image/gif": "GIF",
}

function typeLabel(mime: string): string {
  return TYPE_LABELS[mime] ?? (mime.split("/").pop() ?? mime).toUpperCase()
}

interface StagedFile {
  key: string
  file: File
  status: "staged" | "uploading" | "uploaded" | "failed"
  fileId?: string
}

// Extensions the server accepts when the browser reports no MIME type.
const EXTENSION_TYPES: Record<string, string> = {
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  png: "image/png",
  webp: "image/webp",
  gif: "image/gif",
  heic: "image/heic",
  heif: "image/heif",
  pdf: "application/pdf",
}

function mimeOf(file: File): string {
  if (file.type) return file.type
  const ext = file.name.split(".").pop()?.toLowerCase() ?? ""
  return EXTENSION_TYPES[ext] ?? ""
}

function newKey(prefix: string): string {
  return typeof crypto !== "undefined" && crypto.randomUUID
    ? `${prefix}:${crypto.randomUUID()}`
    : `${prefix}:${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
}

function stage(files: File[]): StagedFile[] {
  return files.map((file) => ({ key: newKey("file"), file, status: "staged" }))
}

export interface IntakeUploadDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  limits: IntakeLimits
  // The files dropped on the page; the dialog takes them when it opens.
  initialFiles: File[]
  onQueued: (session: IntakeSessionDetail) => void
}

// IntakeUploadDialog sends dropped files to batch intake (#2810): it uploads
// each file, then queues the batch for the background scan. Each file is a
// purchase of its own unless the member says that all files show one item.
export function IntakeUploadDialog(props: IntakeUploadDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      {props.open ? <IntakeUploadForm {...props} /> : null}
    </Dialog>
  )
}

function IntakeUploadForm({
  onOpenChange,
  limits,
  initialFiles,
  onQueued,
}: IntakeUploadDialogProps) {
  const { t } = useTranslation()
  const invalidate = useInvalidateIntake()
  const [files, setFiles] = useState<StagedFile[]>(() => stage(initialFiles))
  const [oneItem, setOneItem] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  // One key per opening of the dialog: a retry after a lost response finds
  // the batch the first attempt queued instead of queuing it twice.
  const clientKey = useRef(newKey("intake"))
  // The group the batch goes to, read when the first upload starts.
  const target = useRef<IntakeUploadTarget | null>(null)
  // The files this dialog uploaded that no batch holds yet, and the upload
  // run in flight (#1626). Closing the dialog before the batch is queued
  // moves those files to Trash once the run has stopped.
  const unqueued = useRef(new Set<string>())
  const running = useRef<Promise<void> | null>(null)
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    const leftovers = unqueued.current
    return () => {
      mounted.current = false
      void (running.current ?? Promise.resolve()).finally(() =>
        trashUploads(target.current, [...leftovers]).then(() => leftovers.clear())
      )
    }
  }, [])

  const accepted = new Set(limits.acceptedMimeTypes)
  const maxFiles = oneItem
    ? Math.min(limits.maxFilesPerBatch, limits.maxFilesPerSource)
    : limits.maxFilesPerBatch
  // The queue holds at most maxQueuedPerGroup purchases; a batch of one
  // item is one purchase.
  const queueRoom = Math.max(0, limits.maxQueuedPerGroup - limits.queued)

  function rejection(f: StagedFile, index: number): Rejection | null {
    const mime = mimeOf(f.file)
    if (accepted.size > 0 && !accepted.has(mime)) return HEIC_TYPES.has(mime) ? "heic" : "type"
    if (limits.maxFileBytes > 0 && f.file.size > limits.maxFileBytes) return "size"
    if (maxFiles > 0 && index >= maxFiles) return "count"
    return null
  }

  // The files to send, in the order they were dropped; rejected files stay
  // listed with the reason but are not sent.
  const rejections = new Map<string, Rejection>()
  const sendable: StagedFile[] = []
  for (const f of files) {
    const reason = rejection(f, sendable.length)
    if (reason) rejections.set(f.key, reason)
    else sendable.push(f)
  }
  const purchases = oneItem ? Math.min(1, sendable.length) : sendable.length
  const queueFull = purchases > queueRoom
  const uploaded = files.filter((f) => f.status === "uploaded").length

  function addFiles(list: FileList | null) {
    if (!list || list.length === 0) return
    setFiles((prev) => [...prev, ...stage(Array.from(list))])
    setError(null)
  }

  function removeFile(key: string) {
    setFiles((prev) => prev.filter((f) => f.key !== key))
  }

  function setStatus(key: string, patch: Partial<StagedFile>) {
    setFiles((prev) => prev.map((f) => (f.key === key ? { ...f, ...patch } : f)))
  }

  async function submit() {
    if (busy || sendable.length === 0 || queueFull) return
    const run = send()
    running.current = run
    await run
  }

  // send uploads the files and queues the batch. Once the dialog is closed
  // it stops: it sends nothing more and queues nothing.
  async function send() {
    setBusy(true)
    setError(null)
    target.current ??= intakeUploadTarget()
    const dest = target.current
    const ids: string[] = []
    let failed = false
    // One upload at a time keeps within the per-user upload slots.
    for (const f of sendable) {
      if (!mounted.current) return
      if (f.fileId) {
        ids.push(f.fileId)
        continue
      }
      setStatus(f.key, { status: "uploading" })
      try {
        const id = await uploadIntakeFile(dest, f.file)
        unqueued.current.add(id)
        ids.push(id)
        setStatus(f.key, { status: "uploaded", fileId: id })
      } catch (err) {
        failed = true
        setStatus(f.key, { status: "failed" })
        if (mounted.current)
          setError(parseServerError(err, t("commodities:intake.upload.uploadFailed")))
      }
    }
    if (failed || !mounted.current) {
      if (mounted.current) setBusy(false)
      return
    }
    try {
      const sources = oneItem
        ? [{ fileIds: ids, oneItem: true }]
        : ids.map((id) => ({ fileIds: [id], oneItem: false }))
      const session = await createIntakeSession(dest, clientKey.current, sources)
      // The batch holds these files now. One uploaded for a file that was
      // left out since (the one-item switch takes fewer files) is not
      // needed.
      for (const id of ids) unqueued.current.delete(id)
      const spare = [...unqueued.current]
      unqueued.current.clear()
      void trashUploads(dest, spare)
      await invalidate()
      if (mounted.current) onQueued(session)
    } catch (err) {
      if (!mounted.current) return
      setError(
        isIntakeError(err, INTAKE_QUEUE_FULL)
          ? t("commodities:intake.upload.queueFull")
          : parseServerError(err, t("commodities:intake.upload.queueFailed"))
      )
    } finally {
      if (mounted.current) setBusy(false)
    }
  }

  function reasonText(reason: Rejection): string {
    switch (reason) {
      case "heic":
        return t("commodities:intake.upload.rejectHeic")
      case "type":
        return t("commodities:intake.upload.rejectType", {
          types: formatList(limits.acceptedMimeTypes.map(typeLabel)),
        })
      case "size":
        return t("commodities:intake.upload.rejectSize", {
          size: formatBytes(limits.maxFileBytes),
        })
      case "count":
        return t("commodities:intake.upload.rejectCount", { count: maxFiles })
    }
  }

  return (
    <DialogContent className="sm:max-w-lg" data-testid="intake-upload-dialog">
      <DialogHeader>
        <DialogTitle>{t("commodities:intake.upload.title")}</DialogTitle>
        <DialogDescription>{t("commodities:intake.upload.description")}</DialogDescription>
      </DialogHeader>

      <p className="text-xs text-muted-foreground" data-testid="intake-upload-limits">
        {t("commodities:intake.upload.limits", {
          count: limits.maxFilesPerBatch,
          size: formatBytes(limits.maxFileBytes),
          rate: limits.ratePerHour,
        })}
      </p>

      {files.length > 0 ? (
        <ul
          className="flex max-h-64 flex-col divide-y divide-border overflow-y-auto rounded-md border"
          data-testid="intake-upload-files"
        >
          {files.map((f) => {
            const reason = rejections.get(f.key)
            return (
              <li
                key={f.key}
                className="flex min-w-0 items-center gap-2 px-3 py-2"
                data-testid="intake-upload-file"
              >
                <FileStatusIcon status={f.status} rejected={!!reason} />
                <div className="min-w-0 flex-1">
                  <p className={cn("truncate text-sm", reason && "text-muted-foreground")}>
                    {f.file.name}
                    {f.status === "uploading" || f.status === "uploaded" ? (
                      <span className="sr-only">
                        {" "}
                        {f.status === "uploading"
                          ? t("commodities:intake.upload.fileUploading")
                          : t("commodities:intake.upload.fileUploaded")}
                      </span>
                    ) : null}
                  </p>
                  <p
                    className={cn(
                      "text-xs",
                      reason || f.status === "failed" ? "text-destructive" : "text-muted-foreground"
                    )}
                  >
                    {reason
                      ? reasonText(reason)
                      : f.status === "failed"
                        ? t("commodities:intake.upload.fileFailed")
                        : formatBytes(f.file.size)}
                  </p>
                </div>
                {!busy && f.status !== "uploaded" ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-7 shrink-0"
                    onClick={() => removeFile(f.key)}
                    aria-label={t("commodities:intake.upload.remove", { name: f.file.name })}
                  >
                    <X className="size-3.5" aria-hidden="true" />
                  </Button>
                ) : null}
              </li>
            )
          })}
        </ul>
      ) : (
        <p className="text-sm text-muted-foreground" data-testid="intake-upload-empty">
          {t("commodities:intake.upload.empty")}
        </p>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="gap-2"
          onClick={() => inputRef.current?.click()}
          disabled={busy}
          data-testid="intake-upload-add"
        >
          <Plus className="size-4" aria-hidden="true" />
          {t("commodities:intake.upload.addFiles")}
        </Button>
        <input
          ref={inputRef}
          type="file"
          multiple
          hidden
          accept={limits.acceptedMimeTypes.join(",")}
          onChange={(e) => {
            addFiles(e.target.files)
            e.target.value = ""
          }}
          data-testid="intake-upload-input"
        />
        <div className="flex items-center gap-2">
          <Switch
            id="intake-one-item"
            checked={oneItem}
            onCheckedChange={setOneItem}
            disabled={busy}
            data-testid="intake-upload-one-item"
          />
          <Label htmlFor="intake-one-item" className="text-sm font-normal">
            {t("commodities:intake.upload.oneItem")}
          </Label>
        </div>
      </div>
      <p className="text-xs text-muted-foreground">
        {oneItem
          ? t("commodities:intake.upload.oneItemHint")
          : t("commodities:intake.upload.separateHint")}
      </p>

      <p className="sr-only" role="status" aria-live="polite" data-testid="intake-upload-progress">
        {busy
          ? t("commodities:intake.upload.uploading", { done: uploaded, total: sendable.length })
          : ""}
      </p>

      {queueFull ? (
        <Alert variant="destructive" data-testid="intake-upload-queue-full">
          <AlertCircle aria-hidden="true" />
          <AlertDescription>
            {t("commodities:intake.upload.queueRoom", { count: queueRoom })}
          </AlertDescription>
        </Alert>
      ) : null}
      {error ? (
        <Alert variant="destructive" data-testid="intake-upload-error">
          <AlertCircle aria-hidden="true" />
          <AlertTitle>{t("commodities:intake.upload.errorTitle")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <DialogFooter>
        <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>
          {t("common:actions.cancel")}
        </Button>
        <Button
          type="button"
          onClick={() => void submit()}
          disabled={busy || sendable.length === 0 || queueFull}
          className="gap-2"
          data-testid="intake-upload-submit"
        >
          {busy ? <Loader2 className="size-4 animate-spin" aria-hidden="true" /> : null}
          {busy
            ? t("commodities:intake.upload.uploading", {
                done: uploaded,
                total: sendable.length,
              })
            : error
              ? t("commodities:intake.upload.retry")
              : t("commodities:intake.upload.submit", { count: sendable.length })}
        </Button>
      </DialogFooter>
    </DialogContent>
  )
}

// trashUploads moves files the dialog uploaded but no batch holds to Trash.
// A file the batch holds after all (a queue request whose answer was lost)
// is linked to it and stays.
async function trashUploads(target: IntakeUploadTarget | null, ids: string[]): Promise<void> {
  if (!target || ids.length === 0) return
  const results = await Promise.allSettled(ids.map((id) => trashUnlinkedFile(id, target.filesPath)))
  const failed = results.filter((r) => r.status === "rejected").length
  if (failed > 0) console.warn(`[intake] ${failed} uploaded files could not be moved to Trash`)
}

function FileStatusIcon({ status, rejected }: { status: StagedFile["status"]; rejected: boolean }) {
  const className = "size-4 shrink-0"
  if (rejected || status === "failed") {
    return <AlertCircle className={cn(className, "text-destructive")} aria-hidden="true" />
  }
  if (status === "uploading") {
    return (
      <Loader2 className={cn(className, "animate-spin text-muted-foreground")} aria-hidden="true" />
    )
  }
  if (status === "uploaded") {
    return <CheckCircle2 className={cn(className, "text-status-active")} aria-hidden="true" />
  }
  return <FileText className={cn(className, "text-muted-foreground")} aria-hidden="true" />
}
