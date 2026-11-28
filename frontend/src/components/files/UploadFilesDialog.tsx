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
import { NativeSelect } from "@/components/ui/native-select"
import { TagsInput } from "@/components/files/TagsInput"
import {
  attachFile,
  checkUploadCapacity,
  deleteFile,
  listFileDuplicates,
  updateFile,
  type FileCategory,
  type FileLinkEntityType,
} from "@/features/files/api"
import { categoryFromMime } from "@/features/files/constants"
import { useInvalidateFiles, useUploadFile } from "@/features/files/hooks"
import { useTagLabel } from "@/features/tags/hooks"
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
  // Per-file tags entered inline on step 2 (#1622). Persisted with the
  // PUT /files/{id} call right after the multipart POST — no separate
  // "edit tags later" step is required for the conventional Invoice /
  // Warranty / Manual annotations.
  tags: string[]
  status: "pending" | "uploading" | "done" | "failed"
  error?: string
  // Issue #1451 option C — "use as cover photo?". Defaults to true on
  // the first photo when the commodity has no cover yet, otherwise
  // false. The metadata step exposes a checkbox when the category is
  // photos and the parent passed `onSetCover`. After upload completes,
  // the first uploaded item with `useAsCover === true` becomes the
  // explicit cover via `onSetCover(uploadedFileId)`.
  useAsCover?: boolean
  // Holds the BE-assigned id once the upload+link round trip succeeds
  // — needed because the cover-set call needs the persisted file id,
  // not the temporary client-side id we use during the dialog.
  uploadedFileId?: string
  // An existing file of the group with the same content (same SHA-256 on
  // the BE). The progress step offers to use it instead of the new copy.
  duplicate?: { id: string; title: string }
  duplicateState?: "open" | "working" | "kept" | "replaced" | "failed"
}

// linkMeta maps the category picked in the metadata step to the bucket on
// the entity's Files tab. "Other" names no bucket: the BE then files it in
// the entity's default bucket and keeps the category.
function linkMeta(entityType: FileLinkEntityType, category: FileCategory): string | undefined {
  if (category === "images") return "images"
  if (category === "documents" && entityType === "commodity") return "documents"
  return undefined
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
// `linkedEntity` (#1448) preselects a commodity / location / area as
// the parent of every uploaded file: after the multipart POST returns
// the new file id, we POST /files/{id}/links so the file is attached,
// not orphaned. Linking failure marks the item as failed (since the
// user's intent was "attach"); a metadata-only failure stays non-fatal.
//
// After each upload the dialog asks GET /files/{id}/duplicates. When the
// group already holds a file with the same content, the progress row
// offers to attach the existing file instead (or, without a linked
// entity, to drop the new copy); either way the new copy is deleted.
//
// `initialFiles` lets the page-level drop catcher pre-queue files into
// the dialog so the user doesn't have to drop again — the dialog opens
// in the select step with files already listed.
//
// Issue #1451 option C: when `linkedEntity.type === "commodity"` and
// the parent passes `onSetCover`, photos pick up a "Use as cover photo"
// checkbox in the metadata step. The first photo defaults to ON
// when `commodityHasCover` is false. After upload completes, the first
// item with `useAsCover === true` becomes the explicit cover.
export interface UploadFilesDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  linkedEntity?: {
    type: "commodity" | "location" | "area"
    id: string
    name?: string
  }
  initialFiles?: File[]
  // Mutation entry-point — when omitted the cover checkbox isn't shown
  // (uploads from the global Files page have no parent commodity).
  onSetCover?: (fileId: string) => void
  // Whether the parent commodity already has a cover (explicit or
  // first-photo auto-pick). Influences the default state of the
  // "use as cover" checkbox on the first uploaded photo.
  commodityHasCover?: boolean
}

export function UploadFilesDialog({
  open,
  onOpenChange,
  linkedEntity,
  initialFiles,
  onSetCover,
  commodityHasCover,
}: UploadFilesDialogProps) {
  const coverEligible = !!onSetCover && linkedEntity?.type === "commodity"
  const { t } = useTranslation()
  const toast = useAppToast()
  const upload = useUploadFile()
  const invalidate = useInvalidateFiles()
  const [step, setStep] = useState<Step>("select")
  const [items, setItems] = useState<FileItem[]>([])
  // `coverTouched` flips to true the first time the user toggles a
  // "Use as cover photo" checkbox. After that, applyCoverDefaults stops
  // re-promoting the first photo when the items array changes (e.g.
  // adding / removing a file or re-categorising one), so an explicit
  // uncheck isn't silently undone — Copilot review on PR #1504.
  const [coverTouched, setCoverTouched] = useState(false)
  const [dragOver, setDragOver] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  // The upload that became the cover in this batch, so swapping it for an
  // existing duplicate can move the cover along.
  const coverTargetRef = useRef<string | null>(null)

  const reset = useCallback(() => {
    setStep("select")
    setItems([])
    setCoverTouched(false)
  }, [])

  useEffect(() => {
    // Sync external `open` prop → internal step/items/cover state.
    // eslint-disable-next-line react-hooks/set-state-in-effect
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
    // Seed local items list from the external initialFiles prop.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setItems((prev) => {
      if (prev.length > 0) return prev
      const seeded = initialFiles.map((f) => ({
        id: `${f.name}-${f.size}-${f.lastModified}-${Math.random().toString(36).slice(2, 8)}`,
        file: f,
        title: defaultTitle(f.name),
        category: categoryFromMime(f.type),
        tags: [],
        status: "pending" as const,
      }))
      return applyCoverDefaults(seeded, coverEligible, commodityHasCover, coverTouched)
    })
    // coverTouched is intentionally read but not in deps — the effect
    // only fires on `open` transitions when the queue is empty, and we
    // want the latest value at that moment.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, initialFiles, coverEligible, commodityHasCover])

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
      tags: [],
      status: "pending" as const,
    }))
    setItems((prev) =>
      applyCoverDefaults([...prev, ...next], coverEligible, commodityHasCover, coverTouched)
    )
  }

  function removeItem(id: string) {
    setItems((prev) => prev.filter((it) => it.id !== id))
  }

  function patchItem(id: string, patch: Partial<FileItem>) {
    setItems((prev) => prev.map((it) => (it.id === id ? { ...it, ...patch } : it)))
  }

  function patchCategory(id: string, category: FileCategory) {
    setItems((prev) => {
      const next = prev.map((it) => {
        if (it.id !== id) return it
        // Switching away from photos clears the cover flag — only
        // photos are valid covers per the BE validator.
        return {
          ...it,
          category,
          useAsCover: category === "images" ? it.useAsCover : false,
        }
      })
      return applyCoverDefaults(next, coverEligible, commodityHasCover, coverTouched)
    })
  }

  function setCoverFlag(id: string, value: boolean) {
    // Mark the cover decision as "user-driven" so applyCoverDefaults
    // stops re-promoting the first photo on subsequent items changes.
    // Stays sticky until the dialog closes (reset() clears it).
    setCoverTouched(true)
    setItems((prev) =>
      // Only one item can carry the cover flag — flipping a new one ON
      // resets every other to OFF so the user can't queue two "first"
      // covers and have the upload race them.
      prev.map((it) =>
        it.id === id ? { ...it, useAsCover: value } : value ? { ...it, useAsCover: false } : it
      )
    )
  }

  function patchTags(id: string, tags: string[]) {
    setItems((prev) => prev.map((it) => (it.id === id ? { ...it, tags } : it)))
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
    // Issue #1451 option C — capture the BE-assigned id of the first
    // photo flagged `useAsCover`. Only the first one is promoted; the
    // checkbox UI already enforces that at most one is true at a time.
    let coverTarget: string | null = null
    coverTargetRef.current = null
    for (const item of items) {
      patchItem(item.id, { status: "uploading" })
      try {
        const result = await upload.mutateAsync(item.file)
        // Apply per-file metadata overrides only if they differ from
        // the BE-derived defaults — avoids a no-op PUT for the common
        // "user accepted defaults" path. When `linkedEntity` is set, the
        // file is then attached through POST /files/{id}/links (the
        // upload endpoint takes no link fields).
        const titleChanged = item.title !== defaultTitle(item.file.name)
        const categoryChanged = item.category !== result.file.category
        const tagsChanged = item.tags.length > 0
        if (titleChanged || categoryChanged || tagsChanged) {
          try {
            await updateFile(result.file.id, {
              title: item.title,
              category: item.category,
              ...(tagsChanged ? { tags: item.tags } : {}),
            })
          } catch {
            // Metadata-only failure: the file is on disk; the user can
            // edit it from the detail sheet later.
          }
        }
        if (linkedEntity) {
          try {
            await attachFile(result.file.id, {
              entityType: linkedEntity.type,
              entityId: linkedEntity.id,
              meta: linkMeta(linkedEntity.type, item.category),
            })
          } catch (err) {
            // Linking is the user's stated intent ("attach files to this
            // commodity"). If it fails the file is a standalone file —
            // surface that as a per-item failure so the user can attach
            // it from the Files page instead of believing it worked.
            failed++
            patchItem(item.id, {
              status: "failed",
              error: err instanceof Error ? err.message : String(err),
            })
            continue
          }
        }
        succeeded++
        let duplicate: FileItem["duplicate"]
        try {
          const matches = await listFileDuplicates(result.file.id)
          const first = matches[0]?.file
          if (first) duplicate = { id: first.id, title: first.title || first.path || first.id }
        } catch {
          // The duplicate check is advisory; the upload itself succeeded.
        }
        patchItem(item.id, {
          status: "done",
          uploadedFileId: result.file.id,
          duplicate,
          duplicateState: duplicate ? "open" : undefined,
        })
        if (
          coverTarget === null &&
          coverEligible &&
          item.useAsCover === true &&
          item.category === "images"
        ) {
          coverTarget = result.file.id
          coverTargetRef.current = coverTarget
        }
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

    // Issue #1451 option C — promote the first successfully-uploaded
    // photo flagged `useAsCover` to the explicit cover. Best-effort:
    // failure surfaces through onSetCover's own toast, the upload
    // toast above is already authoritative for the upload result.
    if (coverTarget) {
      onSetCover?.(coverTarget)
    }
  }

  // useExisting swaps a fresh upload for the file the group already holds:
  // the existing file is attached to the linked entity (when there is one)
  // and the new copy is deleted. A cover chosen on the new copy moves to the
  // existing file.
  async function useExisting(item: FileItem) {
    if (!item.duplicate || !item.uploadedFileId) return
    patchItem(item.id, { duplicateState: "working" })
    try {
      if (linkedEntity) {
        await attachFile(item.duplicate.id, {
          entityType: linkedEntity.type,
          entityId: linkedEntity.id,
          meta: linkMeta(linkedEntity.type, item.category),
        })
      }
      await deleteFile(item.uploadedFileId)
      if (coverTargetRef.current === item.uploadedFileId) {
        coverTargetRef.current = item.duplicate.id
        onSetCover?.(item.duplicate.id)
      }
      patchItem(item.id, { duplicateState: "replaced", uploadedFileId: item.duplicate.id })
    } catch {
      patchItem(item.id, { duplicateState: "failed" })
      toast.error(t("files:upload.duplicate.failed"))
    }
    invalidate.all()
  }

  function keepBoth(item: FileItem) {
    patchItem(item.id, { duplicateState: "kept" })
  }

  const totalDone = items.filter((it) => it.status === "done").length
  const allDone =
    step === "progress" && items.every((it) => it.status === "done" || it.status === "failed")

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        variant="fullscreen-mobile"
        className="sm:max-h-[80vh] sm:max-w-2xl"
        data-testid="files-upload-dialog"
      >
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
          <MetadataStep
            items={items}
            onPatch={patchItem}
            onChangeCategory={patchCategory}
            onChangeTags={patchTags}
            coverEligible={coverEligible}
            onSetCover={setCoverFlag}
          />
        ) : (
          <ProgressStep
            items={items}
            totalDone={totalDone}
            linked={!!linkedEntity}
            onUseExisting={useExisting}
            onKeepBoth={keepBoth}
          />
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
          className="max-h-64 divide-y overflow-y-auto rounded-md border max-sm:max-h-none"
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
  onChangeCategory: (id: string, category: FileCategory) => void
  // Per-file tag editor (#1622). The metadata step now accepts tags
  // inline — the deferred "Tags can be added later" hint is gone.
  // Empty array means "no tags"; the upload only PATCHes when the
  // user actually entered one.
  onChangeTags: (id: string, tags: string[]) => void
  // Cover-photo wiring (issue #1451 option C). When eligible, the row
  // for each photo grows a "Use as cover photo" checkbox. `onSetCover`
  // toggles the flag and (UploadFilesDialog) clears every other item
  // so only one item's flag is true at a time.
  coverEligible: boolean
  onSetCover: (id: string, value: boolean) => void
}

function MetadataStep({
  items,
  onPatch,
  onChangeCategory,
  onChangeTags,
  coverEligible,
  onSetCover,
}: MetadataStepProps) {
  const { t } = useTranslation()
  const tagLabel = useTagLabel("file")
  return (
    <div className="flex flex-col gap-3">
      <ul
        className="max-h-[60vh] divide-y overflow-y-auto rounded-md border max-sm:max-h-none"
        data-testid="files-upload-metadata-list"
      >
        {items.map((it) => (
          <li
            key={it.id}
            className="flex flex-col gap-2 p-3 text-sm"
            data-testid={`files-upload-metadata-item-${it.id}`}
          >
            <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
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
                <NativeSelect
                  id={`meta-category-${it.id}`}
                  value={it.category}
                  onChange={(e) => onChangeCategory(it.id, e.target.value as FileCategory)}
                  data-testid={`files-upload-meta-category-${it.id}`}
                  wrapperClassName="w-full"
                >
                  <option value="images">
                    {t("files:categoryImages", { defaultValue: "Images" })}
                  </option>
                  <option value="documents">
                    {t("files:categoryDocuments", { defaultValue: "Documents" })}
                  </option>
                  <option value="other">
                    {t("files:categoryOther", { defaultValue: "Other" })}
                  </option>
                </NativeSelect>
              </div>
            </div>
            {coverEligible && it.category === "images" ? (
              <label className="flex items-center gap-2 text-xs text-muted-foreground">
                <input
                  type="checkbox"
                  checked={!!it.useAsCover}
                  onChange={(e) => onSetCover(it.id, e.target.checked)}
                  data-testid={`files-upload-meta-cover-${it.id}`}
                  className="size-4"
                />
                {t("files:upload.useAsCover", {
                  defaultValue: "Use as cover photo",
                })}
              </label>
            ) : null}
            <TagsInput
              label={t("files:edit.fields.tags", { defaultValue: "Tags" })}
              values={it.tags}
              onChange={(next) => onChangeTags(it.id, next)}
              placeholder={t("files:upload.tagsPlaceholder", {
                defaultValue: "Add tag…",
              })}
              testId={`files-upload-meta-tags-${it.id}`}
              autocomplete
              kind="file"
              compact
              labelOf={tagLabel}
            />
          </li>
        ))}
      </ul>
    </div>
  )
}

// applyCoverDefaults guarantees the "first photo of a coverless
// commodity defaults to useAsCover=true" invariant after any change to
// the items array. No-op when:
//   - the dialog is not commodity-bound (`coverEligible=false`),
//   - the commodity already has a cover (auto-pick or explicit), or
//   - the user has already touched the cover checkbox at least once
//     (`coverTouched=true`). Without this, an unchecked box would be
//     silently re-checked when the user adds or removes another file —
//     Copilot review on PR #1504.
//
// Some item already carrying the flag is also a no-op so we don't
// re-promote when the user picked a different row.
function applyCoverDefaults(
  items: FileItem[],
  coverEligible: boolean,
  commodityHasCover: boolean | undefined,
  coverTouched: boolean
): FileItem[] {
  if (!coverEligible || commodityHasCover || coverTouched) return items
  if (items.some((it) => it.useAsCover === true)) return items
  let promoted = false
  return items.map((it) => {
    if (promoted) return it
    if (it.category !== "images") return it
    promoted = true
    return { ...it, useAsCover: true }
  })
}

interface ProgressStepProps {
  items: FileItem[]
  totalDone: number
  linked: boolean
  onUseExisting: (item: FileItem) => void
  onKeepBoth: (item: FileItem) => void
}

function ProgressStep({ items, totalDone, linked, onUseExisting, onKeepBoth }: ProgressStepProps) {
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
      <ul className="max-h-64 divide-y overflow-y-auto rounded-md border max-sm:max-h-none">
        {items.map((it) => (
          <li
            key={it.id}
            className="flex flex-wrap items-center gap-2 px-3 py-2 text-sm"
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
            {it.duplicate ? (
              <DuplicateNotice
                item={it}
                linked={linked}
                onUseExisting={() => onUseExisting(it)}
                onKeepBoth={() => onKeepBoth(it)}
              />
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  )
}

interface DuplicateNoticeProps {
  item: FileItem
  linked: boolean
  onUseExisting: () => void
  onKeepBoth: () => void
}

// DuplicateNotice is the per-row offer to use the file the group already
// holds instead of the copy just uploaded.
function DuplicateNotice({ item, linked, onUseExisting, onKeepBoth }: DuplicateNoticeProps) {
  const { t } = useTranslation()
  const title = item.duplicate?.title ?? ""
  if (item.duplicateState === "replaced") {
    return (
      <p
        className="basis-full text-xs text-muted-foreground"
        data-testid="files-upload-duplicate-done"
      >
        {linked
          ? t("files:upload.duplicate.attached", { title })
          : t("files:upload.duplicate.removedCopy", { title })}
      </p>
    )
  }
  if (item.duplicateState === "kept") return null
  return (
    <div
      className="flex basis-full flex-wrap items-center gap-2 rounded-md bg-muted px-2 py-1.5 text-xs"
      data-testid="files-upload-duplicate"
    >
      <span className="flex-1">{t("files:upload.duplicate.found", { title })}</span>
      <Button
        size="sm"
        variant="outline"
        onClick={onUseExisting}
        disabled={item.duplicateState === "working"}
        data-testid="files-upload-duplicate-use-existing"
      >
        {linked
          ? t("files:upload.duplicate.attachExisting")
          : t("files:upload.duplicate.removeCopy")}
      </Button>
      <Button
        size="sm"
        variant="ghost"
        onClick={onKeepBoth}
        disabled={item.duplicateState === "working"}
        data-testid="files-upload-duplicate-keep-both"
      >
        {t("files:upload.duplicate.keepBoth")}
      </Button>
    </div>
  )
}
