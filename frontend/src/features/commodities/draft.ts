// Draft persistence + payload helpers for the commodity create/edit form.
//
// Extracted from CommodityFormDialog.tsx (#1988) so the anonymous
// "add your first item before login" flow can drive the SAME draft
// machinery the dialog uses: the landing wrapper stashes validated
// values via writeDraft, and the post-login FirstItemResolver replays
// them through readDraft → toRequest → createCommodity. Keeping these
// in a leaf module (no React, no GroupProvider dependency) lets the
// resolver import them without dragging the whole dialog in.
//
// Behaviour is identical to the in-dialog originals — this is a pure
// move. The dialog re-imports every symbol below.
import type { DeepPartial } from "react-hook-form"

import {
  attachFile,
  trashUnlinkedFile,
  uploadFile,
  updateFile,
  type FileLinkEntityType,
} from "@/features/files/api"
import { categoryFromMime } from "@/features/files/constants"
import { HttpError } from "@/lib/http"
import type { CommodityStatusValue } from "@/features/commodities/constants"
import type { CommodityFormInput } from "@/features/commodities/schemas"
import { formValueToPrice, priceToFormValue } from "@/features/commodities/price"
import type { CustomFieldValues } from "@/features/customFields/api"
import { toFormValues } from "@/features/customFields/values"
import type {
  Commodity,
  CreateCommodityRequest,
  UpdateCommodityRequest,
} from "@/features/commodities/api"

// PendingFile mirrors the Files-step model in CommodityFormDialog. Kept
// here (rather than imported from the dialog) so the upload helper has
// no dependency back on the React component — the resolver replays
// pending files loaded from IndexedDB (`loadPendingFiles`) which return
// this same shape.
export interface PendingFile {
  id: string
  file: File
  tags: string[]
  // The title the user typed (#1626). Unset while the file keeps its
  // default title, the file name without the extension; see
  // pendingFileTitle.
  title?: string
  // The server file an earlier draft save uploaded for this entry (#2812),
  // so a retry attaches it instead of uploading the bytes again.
  uploadedFileId?: string
  // What an AI scan put on the file (#1626), and what of it the user
  // rejected. See features/commodities/fileSuggestions.ts.
  suggestion?: PendingFileSuggestion
}

export interface PendingFileSuggestion {
  // The suggested tag, added to tags; marked as a suggestion while the file
  // has it.
  tag?: string
  // The suggested title, set as the title; marked while the title is it.
  title?: string
  // Suggested tags the user removed. No later scan adds them again.
  rejectedTags?: string[]
  // A suggested title the user changed. Once set, no later scan sets the
  // title, also when the user went back to the file name.
  rejectedTitle?: string
}

// The longest title the server takes for a file, in characters (Unicode
// code points, not UTF-16 units).
export const FILE_TITLE_MAX_LENGTH = 255

// capFileTitle cuts a title to the longest the server takes. It counts code
// points, as the server does, so a cut never splits a character that takes
// two UTF-16 units.
export function capFileTitle(title: string): string {
  const chars = Array.from(title)
  return chars.length > FILE_TITLE_MAX_LENGTH
    ? chars.slice(0, FILE_TITLE_MAX_LENGTH).join("")
    : title
}

// defaultFileTitle is the title a staged file starts with: its name without
// the extension.
export function defaultFileTitle(file: File): string {
  const dot = file.name.lastIndexOf(".")
  return dot > 0 ? file.name.slice(0, dot) : file.name
}

// pendingFileTitle is the title a staged file is saved with: the one the
// user typed, or the default when the field is blank.
export function pendingFileTitle(entry: PendingFile): string {
  return capFileTitle(entry.title?.trim() || defaultFileTitle(entry.file))
}

// placePendingFile gives an uploaded file the staged title and tags, then
// attaches it to the entity. Both requests can be repeated. The title is
// always sent: the upload names the file after the stored blob, and
// PUT /files/{id} replaces the title, the description and the tags together.
export async function placePendingFile(
  entry: PendingFile,
  fileId: string,
  target: { entityType: FileLinkEntityType; entityId: string }
): Promise<void> {
  await updateFile(fileId, { title: pendingFileTitle(entry), tags: entry.tags })
  const category = categoryFromMime(entry.file.type)
  await attachFile(fileId, {
    ...target,
    // "other" names no bucket, so the file keeps its category.
    meta: category === "other" ? undefined : category,
  })
}

// newPendingId mints a stable id for a staged file row. Prefers
// crypto.randomUUID (present in every browser the app targets) and falls
// back to a content-derived string for the rare environment without it
// (older jsdom, locked-down embedded webviews) so list keys stay unique.
// Internal to this module — the only caller is mergePendingFiles.
function newPendingId(file: File): string {
  if (typeof crypto !== "undefined" && crypto.randomUUID) return crypto.randomUUID()
  return `${file.name}-${file.size}-${file.lastModified}-${Math.random()}`
}

// fileSignature is a cheap identity for a staged File — name + size +
// last-modified. Used to de-dupe the AI-scan source files against files
// the user also picks in the Files step (and vice-versa) so the same
// bytes are never attached twice (#1983 dedup open question).
export function fileSignature(file: File): string {
  return `${file.name}::${file.size}::${file.lastModified}`
}

// mergePendingFiles folds `incoming` File objects into the existing pending
// list as PendingFile entries, skipping any whose signature already matches
// an entry already in the list. Returns the same array reference when there
// is nothing new to add so React can bail out of a no-op state update.
//
// This is how AI-scan source files (#1983 Part A) join the Files-step queue:
// the dialog merges the files the user scanned into `pendingFiles`, and the
// save attaches them to the item like any other staged file — images land in
// the `images` bucket and PDFs in `documents` (the category is derived from
// MIME by both the FE row and the BE).
export function mergePendingFiles(existing: PendingFile[], incoming: File[]): PendingFile[] {
  if (incoming.length === 0) return existing
  const seen = new Set(existing.map((entry) => fileSignature(entry.file)))
  const additions: PendingFile[] = []
  for (const file of incoming) {
    const sig = fileSignature(file)
    if (seen.has(sig)) continue
    seen.add(sig)
    additions.push({ id: newPendingId(file), file, tags: [] })
  }
  return additions.length > 0 ? [...existing, ...additions] : existing
}

// ---- Draft persistence helpers ------------------------------------------

// readDraft pulls the previously-saved form values for `key` (per
// #1383). Returns undefined when nothing is stored or the JSON has
// rotted; callers fall back to defaults in either case.
export function readDraft(key: string): Partial<CommodityFormInput> | undefined {
  if (typeof window === "undefined") return undefined
  try {
    const raw = window.localStorage.getItem(key)
    if (!raw) return undefined
    const parsed = JSON.parse(raw) as Partial<CommodityFormInput>
    return parsed
  } catch {
    return undefined
  }
}

// values is what react-hook-form's watch hands over, typed as a deep
// partial; at run time it is the whole form.
export function writeDraft(key: string, values: DeepPartial<CommodityFormInput>): void {
  if (typeof window === "undefined") return
  try {
    window.localStorage.setItem(key, JSON.stringify(values))
  } catch {
    // Quota / private mode / disabled storage — drop silently. Drafts
    // are an enhancement, not a guarantee.
  }
}

export function clearDraft(key: string): void {
  if (typeof window === "undefined") return
  try {
    window.localStorage.removeItem(key)
  } catch {
    // see writeDraft
  }
}

// buildDefaults populates the form with safe initial values. For edit
// mode it carries the existing record; for create mode the only
// pre-populated bits are the count (1), status (in_use), draft, and the
// group currency. Everything else stays empty so the user fills it in.
// An unknown price or purchase date stays an empty field, so saving the
// form unchanged keeps it unknown.
//
// `defaultDraft` seeds the `draft` toggle for a brand-new item (it has
// no effect in edit mode, where the record's own `draft` wins). The
// anonymous landing flow (#1988) passes `true` so a first-time visitor
// only has to enter name/short_name/type — price/date become optional —
// keeping the "add your first item" path as short as possible.
export function buildDefaults(
  initial: Commodity | undefined,
  currency: string,
  defaultDraft = false
): CommodityFormInput {
  // `urls` is typed as `string` by openapi-typescript because the BE
  // model uses `swaggertype:"string"` on a JSONB column; at runtime it's
  // an array of strings. Coerce safely.
  const urls = Array.isArray(initial?.urls) ? (initial.urls as unknown as string[]) : []
  // Numeric defaults are kept as strings here (and throughout the
  // form) so the schema's input type stays string — see schemas.ts
  // for the rationale. They convert to JS numbers at submit time
  // inside toRequest.
  return {
    name: initial?.name ?? "",
    short_name: initial?.short_name ?? "",
    brand: initial?.brand ?? "",
    model: initial?.model ?? "",
    type: (initial?.type as string) ?? "",
    area_id: initial?.area_id ?? "",
    seller_id: initial?.seller_id ?? "",
    status: (initial?.status as string) ?? "in_use",
    count: initial?.count !== undefined ? String(initial.count) : "1",
    original_price: priceToFormValue(initial?.original_price),
    original_price_currency: (initial?.original_price_currency as string) || currency,
    converted_original_price: priceToFormValue(initial?.converted_original_price),
    current_price: priceToFormValue(initial?.current_price),
    serial_number: initial?.serial_number ?? "",
    extra_serial_numbers: initial?.extra_serial_numbers ?? [],
    part_numbers: initial?.part_numbers ?? [],
    tags: initial?.tags ?? [],
    purchase_date: initial?.purchase_date ?? "",
    urls,
    comments: initial?.comments ?? "",
    draft: initial?.draft ?? defaultDraft,
    warranty_expires_at: (initial?.warranty_expires_at as string) ?? "",
    warranty_notes: initial?.warranty_notes ?? "",
    custom_fields: toFormValues(initial?.custom_fields as CustomFieldValues | undefined),
  }
}

// toRequest maps the validated form input into the BE-shaped envelope's
// attributes. Numbers come out of the form as strings (see schemas.ts);
// we convert here. A blank price is sent as null, which the BE stores as
// unknown; "0" is a known zero. `urls` flows through as string[] even
// though openapi-typescript types it as a single string (see buildDefaults).
export function toRequest(
  values: CommodityFormInput,
  groupCurrency: string
): CreateCommodityRequest & UpdateCommodityRequest {
  // Date fields are PDate (pointer-to-Date) on the BE — `Date.UnmarshalJSON`
  // rejects empty strings as "cannot parse \"\" as \"2006\"". Omit the
  // field entirely when the input is blank so the BE sees a missing
  // value (decoded as nil pointer) rather than an invalid date string.
  const date = (v: string): string | undefined => {
    const trimmed = v.trim()
    return trimmed === "" ? undefined : trimmed
  }
  // When the purchase currency is the group's, the purchase price already
  // is the group-currency amount: the mock hides the converted-price field
  // (AddItemDialog L1198 isForeignCurrency = false branch) and the BE
  // rejects a nonzero converted price (PriceRule.ErrConvertedPriceNotZero
  // in go/models/rules/price.go), so it is sent as unknown. In a foreign
  // currency the converted price is whatever the user typed, unknown when
  // blank; nothing is copied into it from the current value.
  const sameCurrency =
    !!groupCurrency &&
    values.original_price_currency.trim().toUpperCase() === groupCurrency.trim().toUpperCase()
  const converted = sameCurrency ? null : formValueToPrice(values.converted_original_price)
  return {
    name: values.name.trim(),
    short_name: values.short_name.trim(),
    brand: values.brand.trim(),
    model: values.model.trim(),
    type: values.type,
    // Omit when empty so the BE creates/leaves the item unassigned
    // (#1986) rather than rejecting "" as a missing area FK. An explicit
    // clear in edit mode (area_id = "") therefore un-assigns the item.
    area_id: values.area_id || undefined,
    // Omitted when empty, which the BE stores as no seller, so clearing the
    // picker unlinks the seller.
    seller_id: values.seller_id || undefined,
    status: values.status as CommodityStatusValue,
    count: Number(values.count),
    original_price: formValueToPrice(values.original_price),
    original_price_currency: values.original_price_currency,
    converted_original_price: converted,
    current_price: formValueToPrice(values.current_price),
    serial_number: values.serial_number.trim(),
    extra_serial_numbers: values.extra_serial_numbers,
    part_numbers: values.part_numbers,
    tags: values.tags,
    purchase_date: date(values.purchase_date),
    // Drop blank rows the user added but never filled — sending `[""]`
    // would trip the BE's per-URL Host/Scheme validation.
    urls: values.urls.map((u) => u.trim()).filter((u) => u !== "") as unknown as string,
    comments: values.comments,
    draft: values.draft,
    warranty_expires_at: date(values.warranty_expires_at),
    warranty_notes: values.warranty_notes,
  }
}

export interface AttachPendingFilesOptions {
  // Checked before each file and after each upload. True stops the run; a
  // file uploaded once it is true goes to Trash instead of onto the item.
  shouldStop?: () => boolean
  onFileStart?: (entry: PendingFile) => void
  // The file reached the server as fileId and is not attached yet.
  onFileUploaded?: (entry: PendingFile, fileId: string) => void
  // The file is titled, tagged and attached.
  onFileAttached?: (entry: PendingFile) => void
}

export interface AttachPendingFilesResult {
  // fileId is the upload the file has on the server, not on the item.
  failed: Array<{ entry: PendingFile; error: unknown; fileId?: string }>
  // The run stopped on shouldStop; the files from there on were not tried.
  stopped: boolean
}

// attachPendingFiles uploads the staged files one at a time and attaches each
// to an existing item (#2916). A file an earlier run uploaded is attached as
// it is, unless it was deleted since; a file that fails is reported and the
// run goes on with the next one.
export async function attachPendingFiles(
  pending: PendingFile[],
  commodityId: string,
  opts: AttachPendingFilesOptions = {}
): Promise<AttachPendingFilesResult> {
  const target = { entityType: "commodity" as const, entityId: commodityId }
  const failed: AttachPendingFilesResult["failed"] = []
  for (const entry of pending) {
    if (opts.shouldStop?.()) return { failed, stopped: true }
    opts.onFileStart?.(entry)
    let fileId = entry.uploadedFileId
    try {
      let placed = false
      if (fileId) {
        try {
          await placePendingFile(entry, fileId, target)
          placed = true
        } catch (err) {
          if (!(err instanceof HttpError && err.status === 404)) throw err
          fileId = undefined
        }
      }
      if (!placed) {
        fileId = (await uploadFile(entry.file)).file.id
        opts.onFileUploaded?.(entry, fileId)
        // The form was left while the file was on its way: nothing will
        // attach it, so it goes to Trash rather than stay without an item.
        if (opts.shouldStop?.()) {
          await trashUnlinkedFile(fileId).catch(() => false)
          return { failed, stopped: true }
        }
        await placePendingFile(entry, fileId, target)
      }
      opts.onFileAttached?.(entry)
    } catch (error) {
      failed.push({ entry, error, fileId })
    }
  }
  return { failed, stopped: false }
}

// uploadPendingFiles uploads each staged file, sets its title and tags, then
// attaches it to the freshly-created commodity (POST /uploads/file, PUT
// /files/:id, POST /files/:id/links). Per-file failures are reported via
// `onError`; the commodity itself is already persisted, so a failed attach
// is non-fatal (the user can retry from the detail page).
export async function uploadPendingFiles(
  pending: PendingFile[],
  commodityId: string,
  onError: (entry: PendingFile, err: unknown) => void
): Promise<void> {
  const place = (entry: PendingFile, fileId: string) =>
    placePendingFile(entry, fileId, { entityType: "commodity", entityId: commodityId })
  const work = pending.map(async (entry) => {
    try {
      // A file a draft save already uploaded is attached as it is, unless
      // it was deleted since.
      if (entry.uploadedFileId) {
        try {
          await place(entry, entry.uploadedFileId)
          return
        } catch (err) {
          if (!(err instanceof HttpError && err.status === 404)) throw err
        }
      }
      const result = await uploadFile(entry.file)
      await place(entry, result.file.id)
    } catch (err) {
      onError(entry, err)
    }
  })
  await Promise.all(work)
}
