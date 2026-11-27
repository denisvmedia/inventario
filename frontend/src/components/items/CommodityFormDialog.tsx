import { useEffect, useEffectEvent, useId, useMemo, useRef, useState, type ReactNode } from "react"
import { Trans, useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import {
  AlertCircle,
  AlertTriangle,
  BookOpen,
  CheckCircle2,
  ChevronLeft,
  ChevronDown,
  ChevronRight,
  File as FileIcon,
  HardDrive,
  Image as ImageIcon,
  Loader2,
  Plus,
  RefreshCw,
  Sparkles,
  Tag as TagIcon,
  Upload,
  X,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { TagsInput } from "@/components/files/TagsInput"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { applyServerFieldErrors } from "@/lib/form-errors"
import { HttpError } from "@/lib/http"
import { clearPendingFiles, loadPendingFiles, savePendingFiles } from "@/lib/pending-files-store"
import {
  type ClassifiedServerError,
  classifyServerError,
  extractFieldErrorCodes,
  getServerErrorCode,
} from "@/lib/server-error"
import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { useQueryClient } from "@tanstack/react-query"

import { categoryFromMime, FILE_TAG_PILLS } from "@/features/files/constants"
import { useTagPillLabel } from "@/features/files/labels"
import {
  buildDefaults,
  capFileTitle,
  clearDraft,
  defaultFileTitle,
  mergePendingFiles,
  readDraft,
  toRequest,
  uploadPendingFiles,
  writeDraft,
  type PendingFile,
} from "@/features/commodities/draft"
import { fileKeys } from "@/features/files/keys"
import { draftKeys } from "@/features/drafts/keys"
import { storageKeys } from "@/features/storage/keys"
import { trashKeys } from "@/features/trash/keys"
import { useTagAutocomplete, useTagLabel } from "@/features/tags/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { CurrencyCombobox } from "@/components/CurrencyCombobox"
import { currencyMeta, priceInputPaddingClass } from "@/lib/currency-meta"
import { COMMODITY_STATUSES, warrantyStatus } from "@/features/commodities/constants"
import { ItemTypeField } from "@/components/itemTypes/ItemTypeField"
import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import {
  LocationAreaFields,
  type AreaOption,
  type LocationOption,
} from "@/components/locations/LocationAreaFields"
import { buildCommoditySchema, type CommodityFormInput } from "@/features/commodities/schemas"
import type {
  Commodity,
  CreateCommodityRequest,
  LinkedItemSummary,
  UpdateCommodityRequest,
} from "@/features/commodities/api"
import { CustomFieldsEditor } from "@/components/customFields/CustomFieldsEditor"
import { useCustomFieldsBinding } from "@/components/customFields/useCustomFieldsBinding"
import {
  CUSTOM_FIELD_INVALID_VALUE,
  type CustomField,
  type CustomFieldValues,
} from "@/features/customFields/api"
import { useCustomFields } from "@/features/customFields/hooks"
import {
  customFieldsRequest,
  toFormValue,
  type CustomFieldFormValues,
} from "@/features/customFields/values"
import {
  AiScanStep,
  type ScanAcceptedValues,
  type ScanAcceptMeta,
  type ScanEnteredValues,
} from "@/components/items/AiScanStep"
import {
  applyFileSuggestions,
  hasSuggestedTitle,
  suggestedTagOf,
  withTags,
  withTitle,
  type FileSuggestion,
} from "@/features/commodities/fileSuggestions"
import type { ScanFieldName } from "@/features/commodities/scanApi"
import { useItemTypeLookup } from "@/features/itemTypes/hooks"
import { tagKeys } from "@/features/tags/keys"
import { normaliseSlug } from "@/features/tags/slug"
import { BrandModelFields } from "@/components/items/BrandModelFields"
import { DraftSaveAlert, type DraftSaveState } from "@/components/items/DraftSaveAlert"
import { suggestShortName } from "@/features/commodities/shortName"
import {
  discardDraft,
  discardedConflict,
  DraftStateError,
  getDraft,
  listDraftFiles,
  promotedConflict,
  revisionConflict,
  type DraftValues,
} from "@/features/drafts/api"
import { draftSellerName } from "@/features/drafts/values"
import {
  useDraft,
  useDraftFiles,
  useInvalidateDrafts,
  type DraftRef,
} from "@/features/drafts/hooks"
import {
  clearDraftSession,
  clientKeyFor,
  forgetBaseRevision,
  forgetItemDraft,
  itemDraftOf,
  promotionKeyFor,
  readBaseRevision,
  rememberItemDraft,
  replaceClientKey,
  sameDraftValues,
  saveDraftWithFiles,
  writeBaseRevision,
} from "@/features/drafts/save"
import {
  deleteFile,
  detachFile,
  ordinaryLinks,
  trashUnlinkedFile,
  updateFile,
  type ListedFile,
} from "@/features/files/api"
import { SellerField } from "@/components/sellers/SellerField"
import { findSellerByName } from "@/features/sellers/api"
import { useSellers } from "@/features/sellers/hooks"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { useSectionVisible } from "@/features/group/sections"
import { cn } from "@/lib/utils"

const NO_CUSTOM_FIELDS: readonly CustomField[] = []

interface CommodityFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // "create" hides the status field (defaults to in_use), "edit"
  // surfaces it so the user can roll over a commodity to sold/lost/etc.
  mode: "create" | "edit"
  // Edit mode prefills from this object — `id` is preserved by the
  // caller, the dialog only consumes the attributes.
  initialValues?: Commodity
  // Names of the items the item's link fields point at (meta.linked_items).
  linkedItems?: Record<string, LinkedItemSummary>
  areas: AreaOption[]
  locations: LocationOption[]
  defaultCurrency: string
  // Returns the persisted commodity (or its id) on success so the
  // dialog can run post-create work — currently uploading + linking
  // pending files from the Files step. Callers in edit mode may
  // return void (the row already exists, nothing to link).
  // In create mode `draft` names the server draft the form was opened
  // from (#2812); the caller promotes it instead of creating an item.
  onSubmit: (
    values: CreateCommodityRequest & UpdateCommodityRequest,
    draft?: DraftRef
  ) => Promise<{ id?: string } | void>
  isPending?: boolean
  // Stable localStorage key used to auto-save the form draft (per #1383).
  // The dialog rehydrates from storage when opening in create mode and
  // clears storage on successful submit. Pass undefined to disable
  // persistence — typically tests do this so each case starts clean.
  draftKey?: string
  // Anonymous landing-page flow (#1988). When true:
  //   (a) the AI scan step targets the public /public/commodities/scan
  //       endpoint (no group, no auth);
  //   (b) the submit-success path becomes a pure hand-off — the dialog
  //       does NOT clear the draft, NOT upload files, and NOT navigate.
  //       The wrapper (AnonymousCommodityDialog) owns persistence + the
  //       redirect-to-login, and the post-login FirstItemResolver replays
  //       the draft + pending files into the real group. This guarantees
  //       no entered data is lost before the user has an account.
  // Defaults to false so every authenticated caller is unchanged.
  anonymous?: boolean
  // Gates the AI photo-scan entry step in create mode (#1988 follow-up).
  // When false, create mode opens directly on Basics (manual entry) and
  // the "ai" surface never mounts — used by the anonymous landing flow
  // when the `public_scan` deployment flag is off, so the "add your first
  // item" CTA still works without offering a scan endpoint that would
  // 404. Defaults to true: every authenticated caller keeps the AI offer
  // (its own /commodities/scan endpoint degrades to a typed 503 banner
  // when no vision provider is configured, which the step already
  // handles).
  enableAiScan?: boolean
  // Seeds the `draft` toggle for a NEW item (ignored in edit mode, where
  // the record's own value wins). The anonymous landing flow (#1988)
  // passes true so a first-time visitor only fills name/short_name/type
  // — price/date/etc. relax to optional. Defaults false: authenticated
  // create keeps a full (non-draft) item by default.
  defaultDraft?: boolean
  // Anonymous flow (#1988): invoked when the user picks "Save as draft" in
  // the dismiss-confirm instead of completing the wizard. The draft has
  // already been written to localStorage; the wrapper uses this to set the
  // pending-first-item marker and route to login (same hand-off as a full
  // submit), so an anonymous "save as draft" still ends up on the login
  // page rather than silently back on the landing surface. When provided
  // (anonymous only) it OWNS closing/navigation — the dialog won't also call
  // onOpenChange. Defaults undefined: authenticated save-as-draft just
  // persists locally and closes.
  onSaveDraft?: () => void
  // Server drafts (#2812). In create mode for a signed-in user, "Save as
  // draft" stores the form and its files on the server as a draft of the
  // group, which can be finished on any device. Local recovery stays as
  // the fallback while a save has not succeeded.
  serverDrafts?: boolean
  // Opens this server draft to finish it. Needs serverDrafts.
  resumeDraftId?: string | null
}

// Files step model lives in features/commodities/draft.ts as
// `PendingFile` (imported above). Conscious deviation from the mock's
// three categorized dropzones (Photos / Receipts / Documents): one
// upload field handles every attachment, and the BE FileCategory is
// derived from MIME via `categoryFromMime` at submit time (images /
// documents / other). The user sees one file list, can attach a
// free-form `tags` chip per file inline, and re-categorisation stays
// available post-create via the file detail page.

// FORM_STEPS is the canonical 5-step sequence the numbered stepper
// renders. The "ai" surface (mock AddItemDialog `step === -1`) is an
// *alternative entry path*, not a step — it never appears in the
// numbered stepper. Create mode opens on "ai" and the user either
// scans photos (gated on #1540) or hits "Fill manually" to land on
// step 1 (Basics). Edit mode skips "ai" entirely.
const FORM_STEPS = ["basics", "purchase", "warranty", "extras", "files"] as const
type FormStepKey = (typeof FORM_STEPS)[number]
const FORM_STEPS_WITHOUT_WARRANTY: readonly FormStepKey[] = FORM_STEPS.filter(
  (s) => s !== "warranty"
)

// formStepsFor returns the form steps, without Warranty for a group that
// hides warranties (#1607) and a form that holds none.
function formStepsFor(withWarranty: boolean): readonly FormStepKey[] {
  return withWarranty ? FORM_STEPS : FORM_STEPS_WITHOUT_WARRANTY
}

function holdsWarranty(values: {
  warranty_expires_at?: string | null
  warranty_notes?: string | null
}): boolean {
  return !!values.warranty_expires_at?.trim() || !!values.warranty_notes?.trim()
}
type StepKey = "ai" | FormStepKey

// Where a staged file is in the save that runs or last ran (#1626). A file
// with no status waits for its turn.
type FileUploadStatus = "uploading" | "uploaded" | "failed"

// A server draft save that went through, with the ids of the staged files
// it attached.
interface SavedDraft extends DraftRef {
  savedEntries: string[]
  // The draft is an item already, so this save stored nothing. Add item
  // promotes it anyway: with the form's own promotion key the server returns
  // the item an earlier attempt made.
  promoted?: boolean
}

// Per-step field allow-list — used by the Next button to validate only
// the current step's fields before advancing. Validating the whole form
// would block the user on fields they haven't seen yet. Warranty
// shipped under #1367 (date + notes); Files still renders a
// ComingSoonBanner because the unified Files surface lands in
// #1398/#1399.
const STEP_FIELDS: Record<StepKey, (keyof CommodityFormInput)[]> = {
  ai: [],
  // area_id stays in the Basics set so a server-side area_id error (only
  // reachable in edit mode now) still maps back to Basics via the 422
  // handler — but it never blocks Next: area is optional (#1986/#1987) so
  // its schema is a bare z.string() that always passes step validation.
  basics: [
    "name",
    "short_name",
    "brand",
    "model",
    "urls",
    "type",
    "area_id",
    "status",
    "count",
    "draft",
  ],
  purchase: [
    "purchase_date",
    "seller_id",
    "original_price",
    "original_price_currency",
    "converted_original_price",
    "current_price",
    "serial_number",
  ],
  warranty: ["warranty_expires_at", "warranty_notes"],
  extras: ["tags", "comments", "extra_serial_numbers", "part_numbers", "custom_fields"],
  files: [],
}

// The status details of an item that is not in use. The form does not show
// them; Change Status on the item page sets them with the status (#2915).
const STATUS_DETAIL_FIELDS: ReadonlySet<string> = new Set([
  "status_date",
  "status_note",
  "sale_price",
])

// resumeStep is the step a resumed server draft opens on (#2794), so Continue
// picks up where the draft was left instead of walking every step again: the
// first step with a field the form refuses (`refused`: the form marks those
// fields as it opens), else the step after the last one the draft holds
// values on. The earlier steps stay one click away, on the stepper and with
// Back.
function resumeStep(
  schema: ReturnType<typeof buildCommoditySchema>,
  values: CommodityFormInput,
  defaults: CommodityFormInput,
  steps: readonly FormStepKey[]
): { step: FormStepKey; refused: boolean } {
  const parsed = schema.safeParse(values)
  if (!parsed.success) {
    const failing = new Set(parsed.error.issues.map((issue) => String(issue.path[0])))
    const invalid = steps.find((s) => STEP_FIELDS[s].some((f) => failing.has(f)))
    if (invalid) return { step: invalid, refused: true }
  }
  let lastFilled = -1
  steps.forEach((s, i) => {
    const filled = STEP_FIELDS[s].some(
      (f) => f !== "draft" && JSON.stringify(values[f]) !== JSON.stringify(defaults[f])
    )
    if (filled) lastFilled = i
  })
  return { step: steps[Math.min(lastFilled + 1, steps.length - 1)], refused: false }
}

// mergeTags adds the scanned tags the form does not hold yet to its tags, in
// their order (#1626). The form holds slugs and the scan reads labels, so
// both are compared by the slug the server makes of them: "Garden Tools"
// is the tag garden-tools.
function mergeTags(current: string[], scanned: string[]): string[] {
  const seen = new Set(current.map(normaliseSlug))
  const out = [...current]
  for (const tag of scanned) {
    const slug = normaliseSlug(tag)
    if (slug === "" || seen.has(slug)) continue
    seen.add(slug)
    out.push(tag)
  }
  return out
}

// uploadedLeftovers lists the files a server save uploaded for these staged
// entries but did not attach (#1626).
function uploadedLeftovers(entries: PendingFile[]): string[] {
  return entries.flatMap((entry) => (entry.uploadedFileId ? [entry.uploadedFileId] : []))
}

// CommodityFormDialog hosts the multi-step Add Item / Edit form. Five
// steps mirror the design mock: Basics → Purchase → Warranty → Extras
// → Files. Files still renders a `ComingSoonBanner` because the unified
// Files surface (#1398/#1399) hasn't shipped; the user can step through
// it with Next and submit the rest of the form. Warranty shipped under
// #1367.
//
// The dialog uses react-hook-form for state + zod for validation.
// Per-step Next clicks call `trigger()` against the current step's
// fields only. On Save the dialog hands a fully-validated payload to
// the caller; the caller is responsible for closing the dialog on
// success.
export function CommodityFormDialog({
  open,
  onOpenChange,
  mode,
  initialValues,
  areas,
  locations,
  defaultCurrency,
  onSubmit,
  isPending,
  draftKey,
  anonymous = false,
  enableAiScan = true,
  defaultDraft = false,
  onSaveDraft,
  linkedItems,
  serverDrafts = false,
  resumeDraftId,
}: CommodityFormDialogProps) {
  const { t } = useTranslation()
  // Create mode opens on the AI offer surface; edit mode jumps
  // straight to Basics (no scanner needed when the row already
  // exists). The numbered stepper iterates `FORM_STEPS` only — AI
  // is an alternative entry path, not part of the linear sequence.
  // When `enableAiScan` is off (anonymous flow with public_scan
  // disabled) create mode also opens on Basics — the scan surface
  // would only 404, so we skip straight to manual entry.
  //
  // `aiStepAvailable` means "the AI scan surface is reachable in this
  // dialog instance" — i.e. AI vision is enabled. It gates the Basics-step
  // Back button, which rewinds to the scanner, in ANY mode: this is the
  // requested behaviour so that reopening a saved draft for editing can
  // still jump to AI scan to finish filling it. Only the OPENING step stays
  // create-only — edit mode opens on Basics (don't force the scanner when a
  // row already exists). (Copilot flagged this vs the original PR text; the
  // edit-mode rewind is intentional and the PR description was updated.)
  const aiStepAvailable = enableAiScan
  const serverMode = mode === "create" && !anonymous && serverDrafts
  const resumeId = serverMode ? (resumeDraftId ?? null) : null
  const initialStep: StepKey = mode === "create" && enableAiScan && !resumeId ? "ai" : "basics"
  const [step, setStep] = useState<StepKey>(initialStep)
  // Tracks which form steps the user has already landed on, so the
  // segmented stepper bar lets them click back-and-forth between
  // already-seen surfaces without forcing a strict forward walk.
  // Edit mode starts with every step visited: the record already has
  // values on each of them, so any step can be opened from the start
  // (#2795). Create mode opens on the AI surface and form steps get
  // added on first arrival.
  const openingVisitedSteps = useMemo<FormStepKey[]>(
    () => (mode === "edit" ? [...FORM_STEPS] : initialStep === "basics" ? ["basics"] : []),
    [mode, initialStep]
  )
  const [visitedSteps, setVisitedSteps] = useState<Set<FormStepKey>>(
    () => new Set(openingVisitedSteps)
  )
  const [serverError, setServerError] = useState<ClassifiedServerError | null>(null)
  // Save-as-draft confirmation. Open when the user dismisses the
  // dialog (Escape, click-outside, X button, or the explicit Cancel
  // footer button) while the form is dirty in create mode — gives
  // them a chance to save the half-filled wizard as a draft instead
  // of losing it.
  const [closeConfirmOpen, setCloseConfirmOpen] = useState(false)
  // Pending file attachments collected by the Files step. Files
  // are NOT uploaded as the user picks them; we batch the uploads
  // immediately after the commodity is created so the BE has the
  // commodity_id to link them against. Single flat list — category
  // is derived from MIME at submit time, per-file tags ride along
  // and land on the file row via PUT /files/:id.
  const [pendingFiles, setPendingFiles] = useState<PendingFile[]>([])
  // Gates the IDB save mirror so it doesn't fire — and overwrite the
  // stored entries with `[]` — before `loadPendingFiles` has had a
  // chance to read what was there. Without this, the open-effect's
  // `setPendingFiles([])` reset triggers the save effect, which
  // commits an empty IDB record before the load completes; on the
  // next open, IDB has nothing to restore. Strict Mode's double-
  // mount makes the race deterministic in dev.
  const [pendingFilesLoaded, setPendingFilesLoaded] = useState(false)
  // The status of each staged file in the server save that runs or last ran,
  // by pending id (#1626).
  const [fileStatus, setFileStatus] = useState<Record<string, FileUploadStatus>>({})
  // Add item in flight: in a form that saves to the server, the save of the
  // draft with its files and then the promotion. The ref keeps a second
  // press (Enter, a double click) from starting another one before the
  // disabled button renders.
  const [adding, setAdding] = useState(false)
  const addingRef = useRef(false)
  // Discard is moving this form's uploads to Trash.
  const [discarding, setDiscarding] = useState(false)
  // Counts the openings of the dialog: it moves on every open, close and
  // unmount. Add item remembers the opening it started in and stops once
  // that opening is over, even when the dialog is open again by then.
  // Browser Back can close the dialog or leave the page while it runs.
  const openingRef = useRef(0)
  // A seller name the AI scan read that matches none of the group's sellers
  // (#2804). The Purchase step offers to create it; nothing is created
  // before the user agrees.
  const [scannedSeller, setScannedSeller] = useState("")
  const sellers = useSellers()
  const itemTypes = useItemTypeLookup()
  const toast = useAppToast()
  // Direct queryClient invalidation rather than `useInvalidateFiles`
  // — the files-feature hook reads `useCurrentGroup`, but the
  // CommodityFormDialog is rendered from places that may not nest
  // under <GroupProvider> (notably the unit-test harness). The
  // commodity's group slug is the canonical scope for files
  // invalidation; we already accept it implicitly via `defaultCurrency`'s
  // resolution path. Falling back to the broad `fileKeys.all`
  // invalidation when no slug is reachable is safe — it just refetches
  // every active files query.
  const queryClient = useQueryClient()
  // True when the dialog opened with a previously-saved draft
  // restored from localStorage. RHF treats the rehydrated values as
  // the new "defaults" → isDirty is false even though the form is
  // visibly populated. Without this flag, Cancel skips the
  // save-as-draft confirm and the user loses the draft silently
  // (the same problem the auto-save was trying to prevent).
  const [draftRehydrated, setDraftRehydrated] = useState(false)
  // Local recovery slot. A resumed server draft gets a slot of its own, so
  // opening one draft never overwrites the unsaved input of another or of
  // a new item.
  const localKey = draftKey && resumeId ? `${draftKey}:${resumeId}` : draftKey
  // Drafts only persist for create mode — editing an existing item
  // never auto-saves to storage (the BE row is the canonical state).
  const persistDrafts = mode === "create" && !!localKey
  // Add item goes through the server draft (#1626): the form and every
  // staged file are saved as the draft first, and the draft is promoted only
  // once it holds them all, so the item never misses a file the user picked.
  const saveThroughDraft = serverMode && !!localKey
  // Server draft session (#2812): the draft this form saves to, with the
  // revision its values are based on.
  const resumedQuery = useDraft(resumeId)
  const resumed = resumedQuery.data
  const resumeLoading = !!resumeId && resumedQuery.isPending
  const [serverDraft, setServerDraft] = useState<DraftRef | null>(null)
  // The resume notices describe why the form opened empty; once the form
  // holds the draft, a later refetch (after promoting it here, say) must
  // not raise them.
  const resumeFailed =
    !!resumeId && (resumedQuery.isError || resumed?.status === "closed") && !serverDraft
  const resumePromoted = resumed?.status === "promoted" && !serverDraft
  const [draftSave, setDraftSave] = useState<DraftSaveState>({ kind: "idle" })
  // True when the form opened with input kept on this device that the
  // server does not have.
  const [restoredUnsaved, setRestoredUnsaved] = useState(false)
  const draftFilesQuery = useDraftFiles(serverMode ? serverDraft?.id : null)
  const invalidateDrafts = useInvalidateDrafts()
  // The resumed draft seeds the form once per opening; a refetch must not
  // wipe what the user typed since.
  const seededRef = useRef<string | null>(null)
  // Whether the short name was typed by the user (or set by the AI scan);
  // until then it follows the name (#2812).
  const shortNameEdited = useRef(false)
  // A server save in flight. Every path into saveToServer (the buttons,
  // Enter on the last step, the close confirm) checks it, so one save runs
  // at a time even before the disabled buttons render.
  const savingRef = useRef(false)
  // The draft whose saved files a "save as a new draft" links to the new
  // one, kept until a save has linked them all. It belongs to one visit of
  // the form: opening or discarding the form forgets it.
  const carryFromRef = useRef<string | null>(null)
  const navigate = useNavigate()

  const defaults = useMemo<CommodityFormInput>(
    () => buildDefaults(initialValues, defaultCurrency, defaultDraft),
    [initialValues, defaultCurrency, defaultDraft]
  )

  // Schema closes over group currency: when the purchase currency
  // matches the group's, `converted_original_price` isn't required
  // (the original price is already in group currency). Re-built when
  // defaultCurrency changes — typically on group switch.
  // The group's custom fields (#2803): their values are checked here and
  // edited on the Extras step. None outside a group (the landing page).
  const customFieldsQuery = useCustomFields()
  const customFields = customFieldsQuery.data ?? NO_CUSTOM_FIELDS
  const schema = useMemo(
    () => buildCommoditySchema(defaultCurrency, customFields),
    [defaultCurrency, customFields]
  )
  const form = useForm<CommodityFormInput>({
    resolver: zodResolver(schema),
    defaultValues: defaults,
    mode: "onBlur",
  })
  const {
    register,
    control,
    handleSubmit,
    formState: { errors, isDirty },
    trigger,
    reset,
    setValue,
    setError,
    watch,
    getValues,
  } = form
  const customFieldsBinding = useCustomFieldsBinding(form, customFields)
  // Draft mode: the "Save as draft" switch is on in a form that saves to
  // the server. Save draft is then offered on every step.
  const draftMode = serverMode && !!watch("draft")
  // A group that hides warranties (#1607) leaves the Warranty step out
  // while the form holds no warranty. A warranty the item has, one a
  // resumed or restored draft brings and one typed in all keep the step,
  // and once this opening of the dialog has held one the step stays, so
  // clearing the date does not take the step away. The warranty fields
  // stay in the form values either way, so saving keeps what they hold.
  const showWarranties = useSectionVisible("warranties")
  const [warrantySeen, setWarrantySeen] = useState(false)
  const formSteps = formStepsFor(
    showWarranties ||
      warrantySeen ||
      holdsWarranty({
        warranty_expires_at: watch("warranty_expires_at"),
        warranty_notes: watch("warranty_notes"),
      })
  )

  // The step a resumed draft opens on, checked against the schema as it is
  // when the draft opens. The schema follows the group's custom fields, and
  // their arrival or a refetch must not run the reset below again: that
  // would send the form back to its first step and drop the input.
  const resumeStepOf = useEffectEvent((values: CommodityFormInput, base: CommodityFormInput) =>
    resumeStep(schema, values, base, formStepsFor(showWarranties || holdsWarranty(values)))
  )

  // Reset to defaults whenever the dialog opens. In create mode we try
  // to rehydrate from the localStorage draft key first (per #1383) — if
  // the user partially filled the form on a previous visit, the values
  // come back. Otherwise we fall through to the static defaults.
  useEffect(() => {
    if (!open) {
      seededRef.current = null
      carryFromRef.current = null
      return
    }
    if (resumeId) {
      if (resumeLoading || seededRef.current === resumeId) return
      seededRef.current = resumeId
    }
    let starting = defaults
    let restoredFromDraft = false
    let unsaved = false
    // A resumed draft opens to be finished: the item path is the default
    // and Save draft is one switch away.
    const openDraft = resumed && resumed.status === "open" ? resumed : null
    let baseRevision: number | null = openDraft?.revision ?? null
    if (openDraft) {
      starting = { ...defaults, ...(openDraft.values as Partial<CommodityFormInput>), draft: false }
    }
    if (persistDrafts && localKey) {
      const restored = readDraft(localKey)
      if (restored) {
        const merged = { ...starting, ...restored }
        unsaved = !sameDraftValues(merged, starting)
        starting = merged
        restoredFromDraft = true
        if (openDraft) baseRevision = readBaseRevision(localKey) ?? baseRevision
      }
    }
    reset(starting)
    setWarrantySeen(holdsWarranty(starting))
    carryFromRef.current = null
    shortNameEdited.current =
      mode === "edit" ||
      (starting.short_name.trim() !== "" && starting.short_name !== suggestShortName(starting.name))
    setServerDraft(openDraft && baseRevision ? { id: openDraft.id, revision: baseRevision } : null)
    if (openDraft && baseRevision && localKey) writeBaseRevision(localKey, baseRevision)
    setDraftSave({ kind: "idle" })
    setRestoredUnsaved(serverMode && unsaved)
    if (resumeId) {
      // A resumed draft opens on the step it still needs, with the steps
      // before it shown as visited. A step it opens on because of a field
      // the form refuses shows that field marked, as Next would.
      const resume = openDraft ? resumeStepOf(starting, defaults) : null
      const landing = resume ? resume.step : initialStep
      const reached = FORM_STEPS.indexOf(landing as FormStepKey)
      setStep(landing)
      setVisitedSteps(new Set(reached > 0 ? FORM_STEPS.slice(0, reached + 1) : openingVisitedSteps))
      if (resume?.refused) void trigger(STEP_FIELDS[resume.step])
    } else {
      setStep(initialStep)
    }
    setServerError(null)
    // A seller the scan of a batch read but the group lacks is offered on
    // the Purchase step, as after a scan in this form.
    setScannedSeller(openDraft ? draftSellerName(starting as DraftValues) : "")
    // Track whether reset() seeded values from a localStorage draft.
    // RHF uses the seeded values as the new "defaults" so isDirty
    // stays false — the Cancel → save-as-draft gate has to read this
    // flag too, otherwise a rehydrated draft would close silently.
    setDraftRehydrated(restoredFromDraft)
    // Pending files restoration. On mobile (esp. Android Chrome) the
    // tab can be killed while a native picker is open; without IDB-
    // backed persistence the user sees their staged files vanish on
    // dialog re-open. `loadPendingFiles` is async; while it resolves
    // we render the empty list. If the user interacts with the form
    // before the load completes their actions still win — the load
    // only fires when both the current state is empty and the IDB
    // returns non-empty.
    setPendingFiles([])
    setPendingFilesLoaded(false)
    setFileStatus({})

    // Pending files restoration. Browser-level reloads (manual,
    // mobile-OS tab kill, etc) wipe in-memory state; IDB persists
    // through them. `loadPendingFiles` is async — until it resolves
    // (and the load gate flips true) the save mirror skips, so we
    // never overwrite stored entries with the freshly-reset `[]`.
    if (persistDrafts && localKey) {
      let cancelled = false
      void loadPendingFiles(localKey).then((restored) => {
        if (cancelled) return
        if (restored.length > 0) {
          setPendingFiles((prev) => (prev.length === 0 ? restored : prev))
        }
        setPendingFilesLoaded(true)
      })
      return () => {
        cancelled = true
      }
    } else {
      // No persistence at all: the mirror is gated on the same flag,
      // flip it true so any later in-session changes (rare without
      // a draftKey) still propagate.
      setPendingFilesLoaded(true)
    }
  }, [
    open,
    defaults,
    reset,
    persistDrafts,
    localKey,
    initialStep,
    mode,
    resumeId,
    resumeLoading,
    resumed,
    serverMode,
    openingVisitedSteps,
    trigger,
  ])

  useEffect(() => {
    const opening = openingRef.current + 1
    openingRef.current = opening
    return () => {
      openingRef.current = opening + 1
    }
  }, [open])

  // Mark each form step visited the moment we land on it. The
  // segmented stepper uses this to decide whether a forward jump is
  // allowed (only previously-seen surfaces are clickable). Also
  // clears any stale server-error banner from a prior submit attempt
  // so it doesn't follow the user across step navigation — the
  // banner lives between submits, not steps.

  useEffect(() => {
    setServerError(null)
    if (step === "ai") return
    setVisitedSteps((prev) => (prev.has(step) ? prev : new Set(prev).add(step)))
  }, [step])

  // Reset visited steps whenever the dialog opens — discardDraft
  // explicitly resets to Basics so its visited set should be a fresh
  // singleton; reopening for a new commodity should also start clean.

  // A resumed draft sets its own once it has loaded (the reset effect above).
  useEffect(() => {
    if (!open || resumeId) return
    setVisitedSteps(new Set(openingVisitedSteps))
  }, [open, openingVisitedSteps, resumeId])

  // Keep the Warranty step for the rest of this opening once the form has
  // held a warranty (see showWarranties above).
  useEffect(() => {
    if (!open) return
    const subscription = watch((values) => {
      if (holdsWarranty(values)) setWarrantySeen(true)
    })
    return () => subscription.unsubscribe()
  }, [open, watch])

  // Auto-save the form to localStorage on every change while the dialog
  // is open in create mode. Debounced to a single rAF tick so a burst
  // of typing doesn't write to storage on every keystroke. A write still
  // waiting when the dialog closes is dropped: the save and discard paths
  // clear the stored draft before closing, and a late write would put the
  // closed form's values back.
  useEffect(() => {
    if (!open || !persistDrafts || !localKey) return
    let frame = 0
    const subscription = watch((values) => {
      window.cancelAnimationFrame(frame)
      frame = window.requestAnimationFrame(() => writeDraft(localKey, values))
    })
    return () => {
      subscription.unsubscribe()
      window.cancelAnimationFrame(frame)
    }
  }, [open, persistDrafts, localKey, watch])

  // The short name follows the name until the user types one (#2812), so
  // quick capture needs one name. Create mode only: an existing item keeps
  // the short name it has.
  useEffect(() => {
    if (!open || mode !== "create") return
    const subscription = watch((values, { name }) => {
      if (name !== "name" || shortNameEdited.current) return
      const suggestion = suggestShortName(values.name ?? "")
      if ((values.short_name ?? "") !== suggestion) {
        setValue("short_name", suggestion, { shouldDirty: true })
      }
    })
    return () => subscription.unsubscribe()
  }, [open, mode, watch, setValue])

  // Mirror pendingFiles into IndexedDB so the staged Files-step
  // attachments survive a manual reload, mobile tab-kill, etc. Gated
  // on `pendingFilesLoaded` so the open-effect's `setPendingFiles([])`
  // reset doesn't race ahead of the IDB read and overwrite the stored
  // entries with an empty array.
  useEffect(() => {
    if (!open || !persistDrafts || !localKey || !pendingFilesLoaded) return
    void savePendingFiles(localKey, pendingFiles)
  }, [open, persistDrafts, localKey, pendingFiles, pendingFilesLoaded])

  async function nextStep() {
    // A server draft takes the form as it is, so draft mode walks the
    // steps without checking them.
    if (!draftMode) {
      const fields = STEP_FIELDS[step]
      const ok = await trigger(fields, { shouldFocus: true })
      if (!ok) return
    }
    if (step === "ai") {
      // AI is the alternative entry point — Next ("Fill manually")
      // hands off to the first form step.
      setStep("basics")
      return
    }
    const idx = formSteps.indexOf(step)
    if (idx >= 0 && idx < formSteps.length - 1) setStep(formSteps[idx + 1])
  }
  function prevStep() {
    if (step === "ai") return
    if (step === "basics") {
      // Back from the first form step rewinds to the AI scan entry when
      // it's available (AI enabled), so a user who picked "Fill manually"
      // can re-open the scanner. Without the AI surface, Basics is the
      // first screen and Back is a no-op.
      if (aiStepAvailable) setStep("ai")
      return
    }
    const idx = formSteps.indexOf(step)
    if (idx > 0) setStep(formSteps[idx - 1])
  }

  // requestClose intercepts dismiss intents (Escape / click-outside
  // / X / Cancel). The confirm appears when the wizard is "carrying
  // unsaved value" — that's either a fresh user edit (`isDirty`) OR
  // a previously-saved draft that auto-rehydrated on this open
  // (`draftRehydrated`). The latter is the subtle case: RHF uses
  // the rehydrated values as the new defaults so isDirty=false even
  // though the form is visibly populated; without the second branch
  // Cancel would close silently and the user would think they lost
  // the draft. Three outcomes: Save as draft (preserve localStorage,
  // close), Discard (clear draft, close), Keep editing (do nothing).
  function requestClose() {
    if (draftSave.kind === "saving" || addingRef.current || discarding) return
    const carriesFiles = serverMode && pendingFiles.length > 0
    if (mode === "create" && persistDrafts && (isDirty || draftRehydrated || carriesFiles)) {
      setCloseConfirmOpen(true)
      return
    }
    onOpenChange(false)
  }

  async function confirmCloseSaveDraft() {
    // A signed-in user's draft goes to the server. When the save fails the
    // confirm closes onto the form, which shows why and keeps the input.
    if (serverMode) {
      const saved = await saveToServer()
      setCloseConfirmOpen(false)
      if (saved) onOpenChange(false)
      return
    }
    // Auto-save effect already wrote the latest values to localStorage;
    // re-write defensively so the final keystroke can't be stranded.
    if (persistDrafts && localKey) {
      writeDraft(localKey, getValues())
    }
    setCloseConfirmOpen(false)
    // Anonymous flow (#1988): a "save as draft" can't persist server-side
    // without an account, so hand off to login (set the marker + navigate)
    // exactly like a full submit — otherwise the user is dumped back on the
    // landing page with no path to actually keep their item. The wrapper
    // owns the navigation, so we DON'T also call onOpenChange here.
    if (onSaveDraft) {
      onSaveDraft()
      return
    }
    onOpenChange(false)
  }

  async function confirmCloseDiscard() {
    if (discarding) return
    // Drops what this device kept. A resumed server draft stays as it was
    // last saved; a draft Add item made for this form is discarded, and the
    // files this form uploaded but never attached go to Trash (#1626).
    setDiscarding(true)
    const itemDraft = localKey ? itemDraftOf(localKey) : null
    const outcome = await cleanUpUploads(itemDraft, uploadedLeftovers(pendingFiles))
    setDiscarding(false)
    if (outcome.added) toast.info(t("commodities:form.discardAlreadyAdded"))
    if (outcome.draftFailed) toast.error(t("commodities:drafts.discardFailed"))
    if (outcome.failedFiles > 0) {
      toast.error(t("commodities:form.leftoverFilesFailed", { count: outcome.failedFiles }))
    }
    if (localKey) {
      clearDraft(localKey)
      void clearPendingFiles(localKey)
      clearDraftSession(localKey)
    }
    carryFromRef.current = null
    setCloseConfirmOpen(false)
    onOpenChange(false)
  }

  // cleanUpUploads discards the draft Add item made for this form and moves
  // the given uploads to Trash when nothing links them. Discarding a draft
  // sends its files that nothing else uses to Trash as well. The draft is
  // discarded only while it is open at the revision this form saved: one
  // the user saved since, from the Inbox or another device, is theirs, and
  // one that became an item is left as it is. It resolves what happened.
  async function cleanUpUploads(
    itemDraft: string | null,
    fileIds: string[]
  ): Promise<{ added: boolean; draftFailed: boolean; failedFiles: number }> {
    const outcome = { added: false, draftFailed: false, failedFiles: 0 }
    if (!itemDraft && fileIds.length === 0) return outcome
    const group = formGroup()
    if (itemDraft) {
      try {
        const current = await getDraft(itemDraft, { group })
        if (current.status === "promoted") outcome.added = true
        else if (
          current.status === "open" &&
          localKey &&
          current.revision === readBaseRevision(localKey)
        ) {
          await discardDraft(itemDraft, { group })
        }
      } catch (err) {
        // A draft that is gone has nothing left to discard.
        if (!(err instanceof HttpError && err.status === 404)) outcome.draftFailed = true
      }
    }
    const results = await Promise.allSettled(fileIds.map((id) => trashUnlinkedFile(id, { group })))
    outcome.failedFiles = results.filter((r) => r.status === "rejected").length
    const slug = group ?? ""
    void invalidateDrafts()
    void queryClient.invalidateQueries({ queryKey: trashKeys.group(slug) })
    void queryClient.invalidateQueries({ queryKey: fileKeys.group(slug) })
    void queryClient.invalidateQueries({ queryKey: storageKeys.group(slug) })
    return outcome
  }

  // removePendingFile takes a staged file off the form. A file an earlier
  // save uploaded but did not attach goes to Trash.
  function removePendingFile(entry: PendingFile) {
    setPendingFiles((prev) => prev.filter((pending) => pending.id !== entry.id))
    if (entry.uploadedFileId) void cleanUpUploads(null, [entry.uploadedFileId])
  }

  // settleSavedFiles takes the files a save attached to the draft off the
  // staged list. They show under the draft's saved files from then on, so
  // that list is read first and a file never drops out of both.
  async function settleSavedFiles(draftId: string, entryIds: string[]) {
    if (entryIds.length === 0) return
    try {
      await queryClient.fetchQuery({
        queryKey: draftKeys.files(activeSlug ?? "", draftId),
        queryFn: ({ signal }) => listDraftFiles(draftId, { signal, group: formGroup() }),
      })
    } catch {
      // The list shows them once it loads.
    }
    const done = new Set(entryIds)
    setPendingFiles((prev) => prev.filter((pending) => !done.has(pending.id)))
    setFileStatus((prev) =>
      Object.fromEntries(Object.entries(prev).filter(([id]) => !done.has(id)))
    )
  }

  // saveToServer stores the form and its staged files as a server draft
  // (#2812). It resolves the draft once it holds everything; otherwise null,
  // and the form shows why, keeping the input here and in local recovery.
  // forItem is the save Add item runs before the promotion (#1626): it keeps
  // the staged files and the local recovery until the item exists.
  async function saveToServer(
    opts: { asNew?: boolean; forItem?: boolean } = {}
  ): Promise<SavedDraft | null> {
    if (!localKey || savingRef.current) return null
    savingRef.current = true
    setDraftSave({ kind: "saving" })
    setServerError(null)
    const values = getValues() as DraftValues
    writeDraft(localKey, values)
    let target = serverDraft
    let clientKey: string
    if (opts.asNew) {
      // A new draft of its own: a new client key and no base revision. After
      // a conflict the old draft still exists, and its saved files are
      // linked to the new one too. A draft that became an item keeps its
      // files with the item; a discarded draft's files go to Trash.
      carryFromRef.current = draftSave.kind === "conflict" && serverDraft ? serverDraft.id : null
      target = null
      setServerDraft(null)
      forgetBaseRevision(localKey)
      clientKey = replaceClientKey(localKey)
    } else {
      clientKey = clientKeyFor(localKey)
    }
    // A slot that never saved gets its draft from this save. Made by Add
    // item, that draft is the item's, not one the user asked for.
    const firstSave = !target && readBaseRevision(localKey) === null
    // The staged files stay listed while the save runs, each with its
    // status; the ones it attaches leave the list when it ends.
    const staged = pendingFiles
    const savedEntries: string[] = []
    // Add item ends with the opening it started in; the form shown after
    // that belongs to the next opening, which this save leaves alone.
    const opening = openingRef.current
    const over = () => !!opts.forItem && openingRef.current !== opening
    const mark = (id: string, status: FileUploadStatus) => {
      if (!over()) setFileStatus((prev) => ({ ...prev, [id]: status }))
    }
    setFileStatus({})
    try {
      const result = await saveDraftWithFiles({
        group: formGroup(),
        draft: target,
        clientKey,
        lastSavedRevision: readBaseRevision(localKey) ?? 1,
        values,
        // Add item on a draft the form already has only adds the files: the
        // promotion carries the values, so a failed Add item leaves a draft
        // the user saved as they saved it.
        keepValues: !!opts.forItem && !!target,
        pendingFiles: staged,
        carryFilesFrom: carryFromRef.current,
        // The draft is known before any file is sent, so a reload or a
        // closed dialog in the middle of the uploads still finds it.
        onDraft: (draft) => {
          writeBaseRevision(localKey, draft.revision)
          if (opts.forItem && firstSave) rememberItemDraft(localKey, draft.id)
        },
        shouldStop: over,
        onFileStart: (entry) => mark(entry.id, "uploading"),
        onFileUploaded: (entry, fileId) =>
          setPendingFiles((prev) =>
            prev.map((pending) =>
              pending.id === entry.id ? { ...pending, uploadedFileId: fileId } : pending
            )
          ),
        onFileSaved: (entry) => {
          savedEntries.push(entry.id)
          mark(entry.id, "uploaded")
        },
      })
      void invalidateDrafts()
      if (result.stopped || over()) {
        // The dialog closed while Add item ran, after its last file too: no
        // item is made. The draft keeps the files that reached it, the local
        // recovery keeps the form, and the next opening adds the item from
        // there.
        return null
      }
      const saved: SavedDraft = {
        id: result.draft.id,
        revision: result.draft.revision,
        savedEntries,
      }
      setServerDraft({ id: saved.id, revision: saved.revision })
      if (!opts.forItem) forgetItemDraft(localKey)
      if (result.failedCarried.length === 0) carryFromRef.current = null
      const failedNames = [
        ...result.failedCarried,
        ...result.failedFiles.map((f) => f.entry.file.name),
      ]
      if (failedNames.length > 0) {
        for (const f of result.failedFiles) mark(f.entry.id, "failed")
        await settleSavedFiles(saved.id, savedEntries)
        const first = result.failedFiles[0]?.error
        const fallback = opts.forItem
          ? t("commodities:form.addFailed")
          : t("commodities:drafts.saveFailed")
        setDraftSave({
          kind: "error",
          message: classifyServerError(first, fallback).message,
          failedFiles: failedNames,
          forItem: opts.forItem,
        })
        return null
      }
      setDraftSave({ kind: "idle" })
      // Add item promotes the draft next; until the item exists the files
      // stay listed as uploaded and the local recovery stays.
      if (opts.forItem) return saved
      // The server holds everything now; local recovery has nothing left
      // to recover.
      setPendingFiles((prev) => prev.filter((pending) => !savedEntries.includes(pending.id)))
      clearDraft(localKey)
      void clearPendingFiles(localKey)
      clearDraftSession(localKey)
      toast.success(t("commodities:drafts.saved"))
      return saved
    } catch (err) {
      if (over()) return null
      // The save stopped before it could finish the files: they stay staged
      // for the next attempt, which reuses what this one uploaded.
      setFileStatus({})
      // Add item retried after a promotion whose answer was lost finds its
      // draft already an item, also after a reload. The promotion that
      // follows tells whether this form made it: one with this form's key
      // answers with the item.
      const promotedId =
        (err instanceof DraftStateError ? err.meta.draft_id : undefined) || target?.id
      if (opts.forItem && promotedId && promotedConflict(err)) {
        setDraftSave({ kind: "idle" })
        return {
          id: promotedId,
          revision: target?.revision ?? readBaseRevision(localKey) ?? 1,
          savedEntries: [],
          promoted: true,
        }
      }
      if (opts.forItem) showDraftRefusal(err, t("commodities:form.addFailed"), true)
      else showDraftRefusal(err, t("commodities:drafts.saveFailed"))
      return null
    } finally {
      savingRef.current = false
    }
  }

  // showDraftRefusal puts a failed save or promotion of the draft in the
  // draft alert: a conflict with another device's save, a draft finished or
  // discarded elsewhere, or any other failure with Retry. forItem marks a
  // failure of Add item, whose Retry adds the item.
  function showDraftRefusal(err: unknown, fallback: string, forItem = false) {
    if (revisionConflict(err)) {
      // A retried create can find a draft another device saved since; the
      // form then works on that draft, so "Load the newer version" can load
      // it.
      const foundId = err instanceof DraftStateError ? err.meta.draft_id : undefined
      if (foundId && localKey) {
        setServerDraft({ id: foundId, revision: readBaseRevision(localKey) ?? 1 })
      }
      setDraftSave({ kind: "conflict" })
      return
    }
    const promoted = promotedConflict(err)
    if (promoted) {
      setDraftSave({ kind: "promoted", commodityId: promoted.commodityId })
      return
    }
    if (discardedConflict(err)) {
      setDraftSave({ kind: "discarded" })
      return
    }
    // A type the group no longer has (deleted while the draft was open) is
    // refused on values.type; it is shown on the type field, as an item
    // save shows it.
    if (extractFieldErrorCodes(err)?.["values.type"]) {
      const message = t("errors:validation.item_type_unknown")
      setError("type", { type: "server", message })
      setStep("basics")
      setDraftSave({ kind: "error", message, failedFiles: [], forItem })
      return
    }
    setDraftSave({
      kind: "error",
      message: classifyServerError(err, fallback).message,
      failedFiles: [],
      forItem,
    })
  }

  // openPromotedItem leaves the form for the item the draft became on
  // another device. What this form held is dropped from this device; Save
  // as a new draft is the way to keep it.
  function openPromotedItem(commodityId: string) {
    void cleanUpUploads(null, uploadedLeftovers(pendingFiles))
    if (localKey) {
      clearDraft(localKey)
      void clearPendingFiles(localKey)
      clearDraftSession(localKey)
    }
    onOpenChange(false)
    const slug = activeSlug ?? ""
    navigate(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityId)}`)
  }

  async function saveDraftAndClose(opts: { asNew?: boolean } = {}) {
    if (await saveToServer(opts)) onOpenChange(false)
  }

  // loadNewerDraft settles a conflict by taking the version saved
  // elsewhere. Staged files stay staged.
  async function loadNewerDraft() {
    if (!serverDraft || !localKey) return
    try {
      const fresh = await getDraft(serverDraft.id, { group: formGroup() })
      const next = {
        ...defaults,
        ...(fresh.values as Partial<CommodityFormInput>),
        draft: getValues("draft"),
      }
      reset(next)
      writeDraft(localKey, next)
      setServerDraft({ id: fresh.id, revision: fresh.revision })
      writeBaseRevision(localKey, fresh.revision)
      setDraftSave({ kind: "idle" })
      void draftFilesQuery.refetch()
    } catch (err) {
      setDraftSave({
        kind: "error",
        message: classifyServerError(err, t("commodities:drafts.saveFailed")).message,
        failedFiles: [],
      })
    }
  }

  // removeSavedFile takes a file off the draft. A file the draft alone
  // used goes to Trash; a file attached elsewhere stays there.
  async function removeSavedFile(row: ListedFile) {
    if (!serverDraft) return
    const link = (row.file.links ?? []).find(
      (l) => l.entity_type === "draft" && l.entity_id === serverDraft.id
    )
    const group = formGroup()
    try {
      if (link?.id) {
        const rest = await detachFile(row.file.id, link.id, { group })
        if (ordinaryLinks(rest).length === 0) await deleteFile(row.file.id, { group })
      }
      void draftFilesQuery.refetch()
      void invalidateDrafts()
    } catch (err) {
      toast.error(classifyServerError(err, t("commodities:drafts.removeFileFailed")).message)
    }
  }

  // updateSavedFile saves a saved file's title and tags (#1626). The saves
  // of one file run one after the other, in the order the user made the
  // changes: two quick edits would otherwise race, and the first PUT landing
  // last would undo the second.
  const savedFileSaves = useRef(new Map<string, Promise<boolean>>())
  function updateSavedFile(
    row: ListedFile,
    change: { title: string; tags: string[] }
  ): Promise<boolean> {
    const id = row.file.id
    const before = savedFileSaves.current.get(id) ?? Promise.resolve(true)
    const run = before.then(() => putSavedFile(row, change))
    savedFileSaves.current.set(id, run)
    void run.then(() => {
      if (savedFileSaves.current.get(id) === run) savedFileSaves.current.delete(id)
    })
    return run
  }

  // putSavedFile sends one change of a saved file. PUT /files/{id} replaces
  // the description too, so the file's own is sent back. The answer goes
  // into the draft's file list, so the row shows what the server holds
  // without reading the list again.
  async function putSavedFile(
    row: ListedFile,
    change: { title: string; tags: string[] }
  ): Promise<boolean> {
    const key = draftKeys.files(activeSlug ?? "", serverDraft?.id ?? "")
    const current =
      queryClient.getQueryData<ListedFile[]>(key)?.find((r) => r.file.id === row.file.id)?.file ??
      row.file
    let saved: ListedFile["file"]
    try {
      saved = await updateFile(
        row.file.id,
        { title: change.title, description: current.description ?? "", tags: change.tags },
        { group: formGroup() }
      )
    } catch (err) {
      toast.error(classifyServerError(err, t("commodities:drafts.updateFileFailed")).message)
      return false
    }
    queryClient.setQueryData<ListedFile[]>(key, (rows) =>
      rows?.map((r) => (r.file.id === saved.id ? { ...r, file: { ...r.file, ...saved } } : r))
    )
    void queryClient.invalidateQueries({ queryKey: fileKeys.group(activeSlug ?? "") })
    return true
  }

  const submit = async (values: CommodityFormInput) => {
    if (addingRef.current) return
    const opening = openingRef.current
    addingRef.current = true
    setAdding(true)
    setServerError(null)
    // In a form that saves to the server, the draft this save made, holding
    // the form and every staged file.
    let saved: SavedDraft | null = null
    try {
      if (saveThroughDraft) {
        // The draft must hold every file before it becomes the item. A file
        // that fails stops here with nothing created: the alert names it,
        // and Retry runs this again from where it stopped.
        saved = await saveToServer({ forItem: true })
        // The promotion is not sent once the opening is over.
        if (!saved || openingRef.current !== opening) return
      }
      const target = saved ?? serverDraft
      // A promotion carries the slot's promotion key, the same for every
      // attempt until one succeeds.
      const draftRef: DraftRef | undefined = target
        ? {
            id: target.id,
            revision: target.revision,
            ...(localKey ? { promotionKey: promotionKeyFor(localKey) } : {}),
            group: formGroup(),
          }
        : undefined
      const created = await onSubmit(
        {
          // A save replaces the whole item, so in edit mode what the form
          // does not show (the status date, note and sale price, the cover)
          // goes back as the item has it (#2915).
          ...(mode === "edit" ? initialValues : undefined),
          ...toRequest(values, defaultCurrency),
          custom_fields: customFieldsRequest(
            customFields,
            initialValues?.custom_fields as CustomFieldValues | undefined,
            values.custom_fields
          ) as Commodity["custom_fields"],
        },
        draftRef
      )
      // Anonymous flow (#1988): the wrapper's onSubmit has just stashed
      // the draft + the pending-first-item marker and is about to redirect
      // to login. We must NOT clear the draft, NOT upload files (there is
      // no commodity id and no auth yet), and NOT reset pendingFiles — the
      // post-login FirstItemResolver replays everything verbatim into the
      // real group. Bailing here is the no-data-loss guarantee.
      if (anonymous) return
      if (saved && !saved.promoted) {
        // The promotion moved the draft's files to the item.
        setPendingFiles([])
        setFileStatus({})
      } else {
        // Without a server draft (edit mode, or a draft that was an item
        // before this save), the staged files are uploaded and linked once
        // the caller has saved the item: POST /uploads/file, PUT /files/:id,
        // POST /files/:id/links. Fire-and-forget so the caller's close +
        // navigate path doesn't wait on the upload loop; files-list
        // invalidation surfaces the uploads on whatever page the user lands
        // on. Per-file failures are toasted; the item is already saved, and
        // the file can be attached from the item's Files tab. In create mode
        // the caller returns the new item; in edit mode the existing record
        // is the link target.
        const linkTargetId = created?.id ?? initialValues?.id
        const filesToUpload = pendingFiles
        if (linkTargetId) {
          if (filesToUpload.length > 0) {
            void uploadPendingFiles(
              filesToUpload,
              linkTargetId,
              (entry, err) => {
                toast.error(t("commodities:form.fileUploadFailed", { name: entry.file.name }))
                console.error("file attach failed", entry.file.name, err)
              },
              { group: formGroup() }
            ).then(() => queryClient.invalidateQueries({ queryKey: fileKeys.all }))
          }
          // Reset state so the next dialog open starts clean.
          setPendingFiles([])
        }
      }
      // Submitted successfully — drop the draft so a fresh dialog open
      // doesn't replay yesterday's data. Clear both the localStorage
      // draft (form fields) and the IDB pending-files store.
      if (persistDrafts && localKey) {
        clearDraft(localKey)
        void clearPendingFiles(localKey)
        clearDraftSession(localKey)
      }
    } catch (err) {
      // The promotion failed with every file on the draft: they show as the
      // draft's saved files, and a retry has no file left to upload.
      if (saved) await settleSavedFiles(saved.id, saved.savedEntries)
      // The draft was saved elsewhere after this form loaded it: the
      // draft alert offers to load that version or keep this one as a
      // new draft, as for a save.
      if (
        (saved ?? serverDraft) &&
        (revisionConflict(err) || promotedConflict(err) || discardedConflict(err))
      ) {
        showDraftRefusal(err, t("commodities:form.serverError"))
        return
      }
      // Map BE field-level validation errors back onto RHF fields
      // (so the failing input gets highlighted and the inline error
      // copy renders next to it) and jump to the step owning the
      // first failing field. Necessary even with FE schema mirrors —
      // BE rules can be stricter than what the FE schema models, and
      // we should never silently round-trip a 422.
      // A refused custom field value often means a field or an option was
      // deleted meanwhile; read the group's fields again.
      if (getServerErrorCode(err) === CUSTOM_FIELD_INVALID_VALUE) void customFieldsQuery.refetch()
      const fieldResult = applyServerFieldErrors(err, setError, {
        fields: Object.values(STEP_FIELDS).flat(),
      })
      if (fieldResult && fieldResult.mapped.length > 0) {
        // Compound paths like `urls.0` need the root segment when
        // looking up which step owns the field.
        const firstFieldRoot = fieldResult.mapped[0].split(".")[0]
        const targetStep = (formSteps as readonly string[]).find((s) =>
          STEP_FIELDS[s as StepKey].some((f) => f === firstFieldRoot)
        ) as StepKey | undefined
        if (targetStep && targetStep !== step) setStep(targetStep)
      }
      // Pull the BE's actual error detail out of the HttpError envelope
      // (JSON:API `errors[0].detail` / `error` / `message`) instead of
      // showing the bare "Request to … failed with NNN" wrapper, and
      // classify it so the banner can pick the right title + decide
      // whether to offer a Retry button (network / unknown only —
      // validation / conflict need the user to edit before resubmit).
      // Falls back to a generic copy when the body has nothing useful.
      //
      // Unlike the single-step forms (which suppress the banner via
      // shouldShowGenericError once every field error maps), this
      // multi-step form ALWAYS keeps the banner as an always-on summary:
      // inline errors point at the offending step, the banner gives the
      // overall status. Don't "simplify" this to match the others — it
      // would hide the step-jump cue.
      //
      // The form sends the status, date, note and sale price the item had
      // when it opened. The server refuses them only when the status was
      // changed since, which its own text would not explain (#2915).
      const statusRefused = Object.keys(fieldResult?.unmapped ?? {}).some((path) =>
        STATUS_DETAIL_FIELDS.has(path.split(".")[0])
      )
      setServerError(
        statusRefused
          ? { kind: "validation", message: t("commodities:form.statusChanged") }
          : classifyServerError(err, t("commodities:form.serverError"))
      )
    } finally {
      addingRef.current = false
      setAdding(false)
    }
  }

  // Client-side validation failed on Save. In edit mode Save is offered on
  // every step, so the failing field can sit on another one: open the
  // first step that has an error.
  function onInvalid(invalid: Record<string, unknown>) {
    const failing = new Set(Object.keys(invalid))
    const target = formSteps.find((s) => STEP_FIELDS[s].some((f) => failing.has(f)))
    if (target && target !== step) setStep(target)
  }
  const submitForm = () => void handleSubmit(submit, onInvalid)()

  // Slug + currency list for the AI scan step. `useOptionalCurrentGroup`
  // returns undefined when the dialog is mounted outside a GroupProvider
  // (the unit-test harness does this for the non-AI flow). The AI step
  // gates its scan button behind a non-empty slug, so the empty-string
  // fallback is safe — the wire request would just 404, which the
  // typed error banner surfaces. The currencies query reuses the same
  // `/currencies` endpoint the CurrencyCombobox queries, so the
  // unknown-currency rejection in AiScanStep matches what the user
  // sees in the manual currency picker.
  const groupContext = useOptionalCurrentGroup()
  const activeSlug = groupContext?.currentGroup?.slug ?? null
  // The group the form opened in (#2920). The requests of a save, a discard
  // or a file change go there, so one still running when the member moves
  // to another group, with browser Back for example, ends in the group it
  // started in.
  const openedInRef = useRef<string | null>(null)
  useEffect(() => {
    if (!open) {
      openedInRef.current = null
      return
    }
    openedInRef.current ??= activeSlug
  }, [open, activeSlug])
  // formGroup is the group the form's requests go to.
  const formGroup = () => openedInRef.current ?? activeSlug ?? undefined

  // enteredScanValues lists what the form holds that the user entered, for
  // the review of an AI scan (#1626): such a field shows both values and
  // starts unticked, so a scan never replaces it unless the user ticks it.
  // In edit mode every value the form holds counts: the item has it saved.
  // A new item's value counts when it differs from what a new, empty form
  // holds; the group currency a new form starts with does not count, and
  // neither does a custom field value equal to the default the item type's
  // template puts there, nor a short name the form derived from the name.
  // A value typed, accepted from an earlier scan, or brought by a resumed or
  // restored draft counts.
  function enteredScanValues(): ScanEnteredValues {
    const values = getValues()
    const editing = mode === "edit"
    const fields: ScanEnteredValues["fields"] = {}
    const text = (key: ScanFieldName & keyof CommodityFormInput) => {
      const v = values[key]
      if (typeof v === "string" && v.trim() !== "") fields[key] = v.trim()
    }
    text("name")
    if (editing || shortNameEdited.current) text("short_name")
    text("brand")
    text("model")
    text("serial_number")
    text("purchase_date")
    text("original_price")
    text("comments")
    if (showWarranties) text("warranty_expires_at")
    if (values.type) fields.type = itemTypes.get(values.type)?.label ?? values.type
    if (values.seller_id) {
      fields.seller =
        (sellers.data ?? []).find((s) => s.id === values.seller_id)?.name ?? values.seller_id
    }
    const currency = values.original_price_currency?.trim().toUpperCase() ?? ""
    if (currency !== "" && (editing || currency !== defaultCurrency.trim().toUpperCase())) {
      fields.original_price_currency = currency
    }
    const urls = (values.urls ?? []).map((u) => u.trim()).filter((u) => u !== "")
    if (urls.length > 0) fields.urls = urls.join(", ")
    if ((values.tags ?? []).length > 0) {
      // The labels the Extras step showed, when the group's tags are loaded.
      const catalog = queryClient.getQueryData<Array<{ slug?: string; label?: string }>>(
        tagKeys.labels(activeSlug ?? "", "commodity")
      )
      const label = (slug: string) => catalog?.find((tag) => tag.slug === slug)?.label || slug
      fields.tags = values.tags.map(label).join(", ")
    }
    const customFieldsEntered: CustomFieldFormValues = {}
    const typeId = itemTypes.get(values.type)?.type?.id
    for (const def of customFields) {
      const value = values.custom_fields?.[def.id]
      if (!value || value.value.trim() === "") continue
      const template =
        !editing && typeId ? def.templates.find((t) => t.item_type_id === typeId) : undefined
      if (template && template.default_value !== undefined && template.default_value !== null) {
        const preset = toFormValue(template.default_value)
        if (preset.value === value.value && preset.currency === value.currency) continue
      }
      customFieldsEntered[def.id] = value
    }
    return { fields, customFields: customFieldsEntered }
  }

  // handleAiAccept consumes the user's accepted-fields subset (from
  // the AiScanStep review phase) and pushes each value into the RHF
  // form before advancing to Basics. setValue with shouldDirty:true so
  // the close-confirm gate picks up the prefill — a user who reviews
  // and accepts BEFORE typing anything else still has unsaved value
  // worth a save-as-draft prompt. shouldValidate=false because the AI
  // can return partial values (e.g. just a name) that aren't enough
  // to pass the per-field validators yet; the Basics step revalidates
  // on first Next click anyway.
  function handleAiAccept(
    values: ScanAcceptedValues,
    _meta: ScanAcceptMeta,
    sourceFiles: File[] = [],
    fileSuggestions: FileSuggestion[] = []
  ) {
    const apply = (
      name: keyof CommodityFormInput,
      value: CommodityFormInput[keyof CommodityFormInput]
    ) => {
      setValue(name, value, { shouldDirty: true, shouldValidate: false })
    }
    if (values.name !== undefined) apply("name", values.name)
    if (values.short_name !== undefined) {
      shortNameEdited.current = values.short_name.trim() !== ""
      apply("short_name", values.short_name)
    }
    if (values.brand !== undefined) apply("brand", values.brand)
    if (values.model !== undefined) apply("model", values.model)
    if (values.type !== undefined) apply("type", values.type)
    if (values.serial_number !== undefined) apply("serial_number", values.serial_number)
    if (values.purchase_date !== undefined) apply("purchase_date", values.purchase_date)
    if (values.seller !== undefined) {
      // A seller the group has is picked; another name waits on the
      // Purchase step for the user to create it.
      const known = findSellerByName(sellers.data ?? [], values.seller)
      if (known) {
        apply("seller_id", known.id)
        setScannedSeller("")
      } else {
        setScannedSeller(values.seller)
      }
    }
    // A group that hides warranties gets no warranty from a scan (#1607).
    if (showWarranties && values.warranty_expires_at !== undefined) {
      apply("warranty_expires_at", values.warranty_expires_at)
    }
    if (values.original_price !== undefined) apply("original_price", values.original_price)
    if (values.original_price_currency !== undefined) {
      apply("original_price_currency", values.original_price_currency)
    }
    if (values.urls !== undefined) {
      // Validation runs on first Next-click into the Extras step
      // (the wizard's `trigger(currentStepFields)` already covers urls
      // there). Don't call `trigger` here — it queues an async
      // re-validation that races other userEvent interactions in unit
      // tests and shows up as intermittent 5s timeouts on the wizard
      // walk-through path.
      apply("urls", values.urls)
    }
    if (values.comments !== undefined) apply("comments", values.comments)
    // Scanned tags join the form's tags, never replace them (#1626).
    if (values.tags !== undefined) apply("tags", mergeTags(getValues("tags") ?? [], values.tags))
    if (values.custom_fields !== undefined) {
      apply("custom_fields", { ...(getValues("custom_fields") ?? {}), ...values.custom_fields })
    }
    // The type read brings its template, after the values read so those win.
    if (values.type !== undefined) customFieldsBinding.applyTemplate(values.type)
    // #1983 Part A: keep the photos / PDFs the user scanned. They join the
    // Files-step queue (deduped against anything already staged there), so
    // the existing post-create uploadPendingFiles() attaches them with the
    // new commodity id — and the anonymous flow's IDB mirror + FirstItem-
    // Resolver replay carry them across the login hand-off for free. The
    // user can still drop any of them from the Files step before saving.
    // Each file then gets the tag and the title the scan suggests for it
    // (#1626), unless the user set them; a file already staged gets them too.
    if (sourceFiles.length > 0) {
      setPendingFiles((prev) =>
        applyFileSuggestions(mergePendingFiles(prev, sourceFiles), fileSuggestions)
      )
    }
    setStep("basics")
  }

  // The numbered stepper + Back/Next gating only know about FORM_STEPS.
  // On the AI surface the form-step index is reported as -1 (not in
  // the form sequence yet) — Back is disabled, Next ("Fill manually")
  // is wired separately above.
  const formStepIndex = step === "ai" ? -1 : formSteps.indexOf(step)
  const isLastStep = formStepIndex === formSteps.length - 1
  // #1554: a count > 1 row is a bundle of identical units and can't
  // carry warranty / loan / service. Watching the count value lets the
  // banner show up live as soon as the user types, and lets the
  // warranty step disable its inputs without waiting for a re-render.
  const liveCount = Number(watch("count"))
  const isBundle = Number.isFinite(liveCount) && liveCount > 1
  const draftSaving = draftSave.kind === "saving"
  const submitting = adding || !!isPending
  // A save, Add item or Discard runs: the form and its files stay as they
  // are until it ends.
  const busy = draftSaving || submitting || discarding
  // How far the save that runs is with the staged files (#1626), for the
  // busy button and screen readers.
  const uploadTotal = draftSaving ? pendingFiles.length : 0
  const uploadDone = pendingFiles.filter((entry) => fileStatus[entry.id] === "uploaded").length
  const uploadProgress =
    uploadTotal > 0 && uploadDone < uploadTotal
      ? t("commodities:form.uploadingFiles", { done: uploadDone, total: uploadTotal })
      : null

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          requestClose()
          return
        }
        onOpenChange(next)
      }}
    >
      {/* Below `sm` the dialog fills the screen ("fullscreen-mobile"):
          the header and footer stick and the steps scroll between them.
          From `sm` up, `max-h-[90vh] overflow-y-auto` keeps the card
          scrollable inside the viewport. Without it the centered-translate
          positioning lets a tall variant (e.g. the #1554 bundle banner +
          the 5-step wizard combined) push the footer below the visible
          viewport on small CI viewports, and Playwright's actionability
          check refuses to click an off-viewport Next button. */}
      <DialogContent
        variant="fullscreen-mobile"
        // `interpolate-size: allow-keywords` lets CSS animate height
        // to/from `auto`, so step swaps and within-step reveals (e.g.
        // "+ Add" URL row, "This item has multiple serial numbers")
        // ease into the new size instead of snapping. The transition
        // covers both the content height and the centred-translate
        // recalc Radix performs when the box grows or shrinks.
        //
        // `overflow-x-hidden` is a belt-and-suspenders cap: even if a
        // child somehow exceeds the content area, it gets clipped
        // instead of pushing the dialog wider.
        className="overflow-x-hidden overflow-y-auto sm:max-h-[90vh] sm:max-w-2xl transition-[height,max-height,transform] duration-200 ease-out [interpolate-size:allow-keywords]"
        data-testid="commodity-form-dialog"
      >
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {step === "ai" ? (
              <>
                <Sparkles aria-hidden="true" className="size-4 text-amber-500" />
                {t("commodities:form.step.ai.title")}
              </>
            ) : mode === "create" ? (
              t("commodities:form.createTitle")
            ) : (
              t("commodities:form.editTitle")
            )}
          </DialogTitle>
          <DialogDescription>
            {step === "ai"
              ? t("commodities:form.step.ai.description")
              : t("commodities:form.stepCounter", {
                  current: formStepIndex + 1,
                  total: formSteps.length,
                  label: t(`commodities:form.step.${step}.title`),
                })}
          </DialogDescription>
        </DialogHeader>

        {/* Segmented progress bar — design-mock AddItemDialog L534-L552.
            Five thin pill segments, one per form step. Each segment:
              · `bg-primary` when current or already visited
              · `bg-muted` when still untouched
              · `bg-destructive` / `destructive/60` when the step has
                a validation error
            Visited segments are clickable, future ones aren't — back
            navigation is always free, forward only after the user
            has landed on the step at least once. Hover surfaces a
            tooltip with the step's label so the bars stay legible
            without numbered text. */}
        {step !== "ai" ? (
          <TooltipProvider delayDuration={120}>
            <ol
              className="flex items-center gap-1.5"
              aria-label={t("commodities:form.stepperLabel")}
            >
              {formSteps.map((s, i) => {
                const isCurrent = step === s
                const isVisited = visitedSteps.has(s)
                const hasError = STEP_FIELDS[s].some((f) => f in errors)
                const navigable = !busy && !isCurrent && (isVisited || i < formStepIndex)
                // Three-tier fill: solid primary for the current
                // segment, dimmed primary for already-visited
                // (regardless of whether it sits before or after
                // current — we want the user to *see* they jumped
                // back), muted for untouched future steps. Same
                // ladder in destructive when the step has errors.
                const fill = hasError
                  ? isCurrent
                    ? "bg-destructive"
                    : isVisited
                      ? "bg-destructive/40"
                      : "bg-destructive/20"
                  : isCurrent
                    ? "bg-primary"
                    : isVisited
                      ? "bg-primary/40"
                      : "bg-muted"
                const stepLabel = t(`commodities:form.step.${s}.title`)
                return (
                  <li key={s} className="flex-1">
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <button
                          type="button"
                          aria-label={stepLabel}
                          aria-current={isCurrent ? "step" : undefined}
                          aria-disabled={navigable ? undefined : true}
                          disabled={!navigable}
                          onClick={() => {
                            if (navigable) setStep(s)
                          }}
                          className={cn(
                            "h-1.5 w-full rounded-full transition-colors",
                            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
                            navigable ? "cursor-pointer hover:opacity-80" : "cursor-default",
                            fill
                          )}
                        />
                      </TooltipTrigger>
                      {/* The tooltip slides in over the thin bar; letting the
                          pointer through keeps a quick click on the bar. Inside
                          the modal, Radix sets an inline pointer-events: auto,
                          so only an !important rule overrides it. */}
                      <TooltipContent className="pointer-events-none!">{stepLabel}</TooltipContent>
                    </Tooltip>
                  </li>
                )
              })}
            </ol>
          </TooltipProvider>
        ) : null}

        {/* Draft Switch row — design-mock AddItemDialog L555-L569.
            Lifted out of BasicsStep so the toggle stays visible across
            every form step. Hidden on the AI step (where the form
            isn't yet active). */}
        {step !== "ai" && !resumeLoading ? (
          <div className="flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-3 py-2.5">
            <Controller
              control={control}
              name="draft"
              render={({ field }) => (
                <Switch
                  id="commodity-draft"
                  checked={!!field.value}
                  disabled={busy}
                  onCheckedChange={(v) => {
                    field.onChange(!!v)
                    // The foreign-currency rule (a known foreign price
                    // needs a group-currency amount) applies only to a
                    // non-draft item. Re-trigger validation on the price
                    // paths so its error clears as soon as the user opts
                    // into a draft, and comes back if they opt out.
                    void trigger([
                      "purchase_date",
                      "original_price",
                      "converted_original_price",
                      "current_price",
                    ])
                  }}
                />
              )}
            />
            <div className="min-w-0 flex-1">
              <Label htmlFor="commodity-draft" className="cursor-pointer text-sm font-medium">
                {t("commodities:fields.draft")}
              </Label>
              <p className="text-xs text-muted-foreground">{t("commodities:fields.draftHelp")}</p>
            </div>
          </div>
        ) : null}

        {resumeFailed ? (
          <Alert variant="destructive" data-testid="draft-resume-failed">
            <AlertTriangle className="size-4" aria-hidden="true" />
            <AlertTitle>{t("commodities:drafts.notFoundTitle")}</AlertTitle>
            <AlertDescription>{t("commodities:drafts.notFoundBody")}</AlertDescription>
          </Alert>
        ) : null}
        {resumePromoted ? (
          <Alert data-testid="draft-resume-promoted">
            <AlertTriangle className="size-4" aria-hidden="true" />
            <AlertTitle>{t("commodities:drafts.promotedTitle")}</AlertTitle>
            <AlertDescription>{t("commodities:drafts.promotedBody")}</AlertDescription>
          </Alert>
        ) : null}
        {restoredUnsaved ? (
          <Alert data-testid="draft-restored-locally">
            <HardDrive className="size-4" aria-hidden="true" />
            <AlertTitle>{t("commodities:drafts.restoredTitle")}</AlertTitle>
            <AlertDescription>{t("commodities:drafts.restoredBody")}</AlertDescription>
          </Alert>
        ) : null}

        {isBundle ? (
          <Alert
            variant="default"
            className="border-amber-300 bg-amber-50 text-amber-900 dark:bg-amber-950/30"
            data-testid="commodity-form-bundle-banner"
          >
            <AlertTriangle className="size-4" aria-hidden="true" />
            <AlertTitle>{t("commodities:trackingRestrictions.bannerTitle")}</AlertTitle>
            <AlertDescription>{t("commodities:trackingRestrictions.bannerBody")}</AlertDescription>
          </Alert>
        ) : null}

        {/* eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- the `<form>` is the natural owner of the form-level Enter shortcut; moving the listener to a wrapper div would lose the implicit-submit semantics RHF relies on, and Radix Select / textarea / chip inputs already swallow Enter so the handler only fires for plain `<input>`s. */}
        <form
          id="commodity-form"
          // Pressing Enter inside any field on an intermediate step
          // would otherwise trigger this form's submit (browser
          // default for `<form>` with an implicit submit), firing the
          // create mutation prematurely — the user reported a 422
          // appearing without ever clicking "Add item". Route the
          // implicit submit through nextStep when we're not on the
          // final step so Enter advances instead of submitting.
          // Always block native form submit. The wizard's Next /
          // Submit are wired via explicit onClick handlers; a native
          // submit (e.g. Enter inside a field, OR the post-render race
          // where clicking Next on Extras causes React to swap the
          // button into a submit-button at the same DOM coords mid-
          // click) would otherwise trigger the create mutation
          // unintentionally.
          onSubmit={(e) => e.preventDefault()}
          onKeyDown={(e) => {
            // Enter on a plain `<input>` advances the step (or
            // submits on the last one), matching keyboard-form
            // muscle memory.
            //
            // CRITICAL: a child input that handles its own Enter
            // (TagsInput / ChipInput call `preventDefault()` to
            // commit the chip without bubbling) must be a no-op
            // here — otherwise the same Enter both commits the
            // chip AND advances the step. Bail when
            // `e.defaultPrevented`, then check tagName, then check
            // the step. The `onSubmit` above still blocks the
            // native browser submit so a post-render Next-→-Submit
            // button swap can't cause an unintended POST.
            if (e.key !== "Enter") return
            if (e.defaultPrevented) return
            const target = e.target as HTMLElement | null
            if (!target || target.tagName !== "INPUT") return
            e.preventDefault()
            if (step === "ai") return
            if (isLastStep && draftMode) {
              void saveDraftAndClose()
            } else if (isLastStep) {
              submitForm()
            } else {
              void nextStep()
            }
          }}
          className="flex min-w-0 flex-col gap-4"
          noValidate
        >
          <StepResizeWrapper>
            {/* A resumed draft fills the form once it has loaded, so the
                fields stay hidden until then: input typed earlier would
                be replaced. */}
            {resumeLoading ? (
              <p className="py-6 text-sm text-muted-foreground" data-testid="draft-resume-loading">
                {t("commodities:drafts.loading")}
              </p>
            ) : null}
            {!resumeLoading && step === "ai" ? (
              <AiScanStep
                slug={activeSlug ?? ""}
                defaultCurrency={defaultCurrency}
                onAccept={handleAiAccept}
                readEntered={enteredScanValues}
                onSkip={() => setStep("basics")}
                anonymous={anonymous}
                omitWarranty={!showWarranties}
              />
            ) : null}
            {!resumeLoading && step === "basics" ? (
              <BasicsStep
                register={register}
                control={control}
                errors={errors}
                watch={watch}
                setValue={setValue}
                trigger={trigger}
                areas={areas}
                locations={locations}
                showStatus={mode === "edit"}
                showLocationPicker={mode === "edit"}
                onTypeChange={customFieldsBinding.applyTemplate}
                onShortNameInput={(value: string) => {
                  shortNameEdited.current = value.trim() !== ""
                }}
              />
            ) : null}
            {!resumeLoading && step === "purchase" ? (
              <PurchaseStep
                register={register}
                control={control}
                errors={errors}
                watch={watch}
                defaultCurrency={defaultCurrency}
                // Sellers belong to a group; the anonymous flow has none.
                showSeller={!anonymous && !!activeSlug}
                scannedSeller={scannedSeller}
                onScannedSellerDone={() => setScannedSeller("")}
              />
            ) : null}
            {!resumeLoading && step === "warranty" ? (
              <WarrantyStep register={register} errors={errors} watch={watch} isBundle={isBundle} />
            ) : null}
            {!resumeLoading && step === "extras" ? (
              <ExtrasStep
                register={register}
                errors={errors}
                watch={watch}
                setValue={setValue}
                customFieldsEditor={
                  customFields.length > 0 ? (
                    <div
                      className="flex flex-col gap-3 rounded-xl border border-border p-3"
                      data-testid="commodity-form-custom-fields"
                    >
                      <Label className="text-sm font-medium">
                        {t("customFields:section.title")}
                      </Label>
                      <CustomFieldsEditor
                        fields={customFields}
                        values={customFieldsBinding.values}
                        onChange={customFieldsBinding.onChange}
                        errors={customFieldsBinding.errors}
                        linkedItems={linkedItems}
                        selfId={mode === "edit" ? initialValues?.id : undefined}
                        groupCurrency={defaultCurrency}
                      />
                    </div>
                  ) : null
                }
              />
            ) : null}
            {!resumeLoading && step === "files" ? (
              <FilesStep
                pendingFiles={pendingFiles}
                setPendingFiles={setPendingFiles}
                onRemovePending={removePendingFile}
                statuses={fileStatus}
                busy={busy}
                savedFiles={serverMode && serverDraft ? (draftFilesQuery.data ?? []) : []}
                onRemoveSaved={removeSavedFile}
                onUpdateSaved={updateSavedFile}
              />
            ) : null}
          </StepResizeWrapper>

          <DraftSaveAlert
            state={draftSave}
            // A failed Add item retries Add item: the save picks up where it
            // stopped, and the promotion follows.
            onRetry={() =>
              draftSave.kind === "error" && draftSave.forItem
                ? submitForm()
                : void saveDraftAndClose()
            }
            onLoadNewer={() => void loadNewerDraft()}
            onSaveAsNew={() => void saveDraftAndClose({ asNew: true })}
            onOpenItem={openPromotedItem}
          />

          <ServerErrorBanner
            error={serverError}
            // Retry re-runs the form's existing submit pipeline with
            // the current values (the user has neither edited nor
            // navigated away — the same payload is still in RHF state).
            // Only `network` / `unknown` kinds show the button; the
            // banner gates that internally via `isRetryableKind`.
            onRetry={submitForm}
            isRetrying={submitting}
            testId="commodity-form-error"
          />

          <p
            className="sr-only"
            role="status"
            aria-live="polite"
            data-testid="commodity-form-progress"
          >
            {uploadProgress ?? ""}
          </p>
        </form>

        {step === "ai" ? (
          // AI-step footer: only the Cancel/close affordance lives in
          // the dialog chrome. The four-phase state machine (offer /
          // scanning / review / error) owns its own primary actions
          // inline — "Scan files" / "Use these values" / "Start over" /
          // "Fill manually" / "Cancel scan" — so the footer stays
          // tight and never wrestles with phase-specific buttons.
          <DialogFooter className="gap-2">
            <Button type="button" variant="ghost" className="mr-auto" onClick={requestClose}>
              {t("common:actions.cancel")}
            </Button>
          </DialogFooter>
        ) : (
          <DialogFooter className="gap-2">
            <Button
              type="button"
              variant="ghost"
              className="mr-auto"
              onClick={requestClose}
              disabled={busy}
              data-testid="commodity-form-cancel"
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={prevStep}
              // Enabled on Basics too when the AI scan surface exists —
              // Back there rewinds to the scanner (see prevStep). Without
              // it, Basics is the first screen so Back stays disabled.
              disabled={busy || (formStepIndex <= 0 && !aiStepAvailable)}
            >
              <ChevronLeft className="size-4" aria-hidden="true" />
              {t("commodities:form.back")}
            </Button>
            {/* Edit mode offers Save on every step (#2795): changing one
                field must not mean walking to the last step. Continue
                stays as the secondary action. */}
            {!isLastStep ? (
              <Button
                type="button"
                variant={mode === "edit" || draftMode ? "outline" : "default"}
                onClick={nextStep}
                disabled={resumeLoading || busy}
                data-testid="commodity-form-next"
              >
                {t("commodities:form.continue")}
                <ChevronRight className="size-4" aria-hidden="true" />
              </Button>
            ) : null}
            {draftMode ? (
              <Button
                type="button"
                onClick={() => void saveDraftAndClose()}
                disabled={busy}
                data-testid="commodity-form-save-draft"
              >
                {draftSaving ? (
                  <>
                    <RefreshCw className="size-3.5 animate-spin" aria-hidden="true" />
                    {uploadProgress ?? t("commodities:drafts.saving")}
                  </>
                ) : (
                  t("commodities:drafts.saveDraft")
                )}
              </Button>
            ) : null}
            {!draftMode && (isLastStep || mode === "edit") ? (
              <Button
                type="button"
                onClick={submitForm}
                disabled={busy}
                data-testid="commodity-form-submit"
              >
                {submitting ? (
                  <>
                    <RefreshCw className="size-3.5 animate-spin" aria-hidden="true" />
                    {uploadProgress ?? t("commodities:form.submitting")}
                  </>
                ) : mode === "create" ? (
                  t("commodities:form.submitCreate")
                ) : (
                  t("commodities:form.submitEdit")
                )}
              </Button>
            ) : null}
          </DialogFooter>
        )}
      </DialogContent>

      {/* Save-as-draft confirmation. Three outcomes: Save as draft
          (preserve localStorage + close), Discard (clear + close),
          Keep editing (Escape / Cancel button → keep wizard open).
          Mounted as a sibling Dialog inside the same Radix tree so
          focus management hands back to the wizard cleanly. */}
      <Dialog open={closeConfirmOpen} onOpenChange={setCloseConfirmOpen}>
        <DialogContent className="sm:max-w-sm" data-testid="commodity-form-close-confirm">
          <DialogHeader>
            <DialogTitle>{t("commodities:form.closeConfirm.title")}</DialogTitle>
            <DialogDescription>
              {serverMode
                ? t("commodities:form.closeConfirm.serverDescription")
                : anonymous
                  ? t("commodities:form.closeConfirm.deviceDescription")
                  : t("commodities:form.closeConfirm.description")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="gap-2">
            <Button
              type="button"
              variant="ghost"
              className="mr-auto"
              onClick={() => setCloseConfirmOpen(false)}
              disabled={discarding}
              data-testid="commodity-form-close-confirm-cancel"
            >
              {t("commodities:form.closeConfirm.keepEditing")}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => void confirmCloseDiscard()}
              disabled={draftSaving || discarding}
              data-testid="commodity-form-close-confirm-discard"
            >
              {discarding ? (
                <RefreshCw className="size-3.5 animate-spin" aria-hidden="true" />
              ) : null}
              {resumeId
                ? t("commodities:form.closeConfirm.discardChanges")
                : t("commodities:form.closeConfirm.discard")}
            </Button>
            <Button
              type="button"
              onClick={() => void confirmCloseSaveDraft()}
              disabled={draftSaving || discarding}
              data-testid="commodity-form-close-confirm-save"
            >
              {draftSaving ? (
                <>
                  <RefreshCw className="size-3.5 animate-spin" aria-hidden="true" />
                  {t("commodities:drafts.saving")}
                </>
              ) : anonymous ? (
                t("commodities:form.closeConfirm.keepOnDevice")
              ) : (
                t("commodities:form.closeConfirm.saveDraft")
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Dialog>
  )
}

// FieldLabel renders a `<Label>` plus an optional asterisk
// indicator. The asterisk lives in a sibling `<span>` rather than
// inside the label so the label's `textContent` stays equal to the
// field name — `getByLabelText(/^Name$/i)` in tests still matches
// without having to special-case the marker. `aria-hidden` keeps
// screen readers from announcing the asterisk; the input's
// `aria-required` attribute carries the semantic.
function FieldLabel({
  htmlFor,
  required,
  children,
}: {
  htmlFor: string
  required?: boolean
  children: ReactNode
}) {
  // Always render the asterisk slot — hidden via `invisible` when
  // not required — so the row's height is stable regardless of
  // dynamic required-ness. Without this, toggling Save-as-draft
  // would shift every label's baseline by the asterisk's line
  // height, jittering the whole step body vertically.
  return (
    <div className="flex items-baseline gap-1">
      <Label htmlFor={htmlFor}>{children}</Label>
      <span aria-hidden="true" className={cn("text-destructive", !required && "invisible")}>
        *
      </span>
    </div>
  )
}

// ---- Step 1: Basics -----------------------------------------------------

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- RHF types thread generics through every helper; concrete types here are noisy.
function BasicsStep(props: any) {
  const { t } = useTranslation()
  const {
    register,
    control,
    errors,
    watch,
    setValue,
    trigger,
    areas,
    locations,
    showStatus,
    showLocationPicker,
    onTypeChange,
    onShortNameInput,
  } = props
  return (
    <div className="space-y-4 py-2" data-testid="commodity-form-basics-step">
      <div className="flex flex-col gap-1.5">
        <FieldLabel htmlFor="commodity-name" required>
          {t("commodities:fields.name")}
        </FieldLabel>
        <Input
          id="commodity-name"
          aria-required
          placeholder={t("commodities:fields.namePlaceholder")}
          {...register("name")}
          aria-invalid={!!errors.name}
        />
        <FieldError error={errors.name} />
      </div>

      <div className="flex flex-col gap-1.5">
        <FieldLabel htmlFor="commodity-short-name" required>
          {t("commodities:fields.shortName")}
        </FieldLabel>
        <Input
          id="commodity-short-name"
          maxLength={40}
          className="font-mono text-sm"
          aria-required
          placeholder={t("commodities:fields.shortNamePlaceholder")}
          {...register("short_name", {
            onChange: (e: React.ChangeEvent<HTMLInputElement>) =>
              onShortNameInput?.(e.target.value),
          })}
          aria-invalid={!!errors.short_name}
        />
        {errors.short_name ? (
          <FieldError error={errors.short_name} />
        ) : (
          <p className="text-xs text-muted-foreground">{t("commodities:fields.shortNameHelp")}</p>
        )}
      </div>

      <BrandModelFields control={control} register={register} errors={errors} />

      <div className="flex flex-col gap-1">
        <UrlList
          label={t("commodities:fields.urls")}
          helper={t("commodities:fields.urlsHelp")}
          addLabel={t("commodities:fields.urlsAdd")}
          placeholder={t("commodities:fields.urlsPlaceholder")}
          values={watch("urls") ?? []}
          onChange={(next) => setValue("urls", next, { shouldDirty: true })}
          onRowBlur={() => {
            // Re-run zod validation for the urls array as soon as a
            // row blurs — surfaces "enter a valid URL" inline before
            // the user reaches Submit. (Form mode is "onBlur" but
            // the UrlList inputs aren't `register()`-bound, so we
            // trigger explicitly.)
            void trigger("urls")
          }}
          testId="commodity-urls"
          // Pull per-index errors out of RHF so the message renders
          // right under the offending row. Both sources land in the
          // same `errors.urls[idx]` slot:
          //   - in-place zod (urlOrEmpty refinement) — message is an
          //     i18n key.
          //   - server validation — message is the raw BE string.
          // Run both through `t()`; i18next returns the input verbatim
          // when no key matches, so raw BE strings pass through.
          rowErrors={
            errors.urls && typeof errors.urls === "object"
              ? Object.entries(errors.urls as Record<string, { message?: string }>).reduce<
                  Array<string | undefined>
                >((acc, [idx, v]) => {
                  const idxNum = Number(idx)
                  if (Number.isFinite(idxNum) && v?.message) acc[idxNum] = t(v.message)
                  return acc
                }, [])
              : undefined
          }
        />
        {/* Top-level (non-indexed) urls errors still fall back here. */}
        <FieldError error={errors.urls} />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="flex flex-col gap-1.5">
          <FieldLabel htmlFor="commodity-type" required>
            {t("commodities:fields.type")}
          </FieldLabel>
          <Controller
            control={control}
            name="type"
            render={({ field }) => (
              <ItemTypeField
                id="commodity-type"
                value={field.value ?? ""}
                onChange={(slug: string) => {
                  field.onChange(slug)
                  onTypeChange?.(slug)
                }}
                ariaRequired
                invalid={!!errors.type}
                errorMessage={errors.type?.message}
              />
            )}
          />
          <FieldError error={errors.type} />
        </div>
        <div className="flex flex-col gap-1.5">
          <FieldLabel htmlFor="commodity-count" required>
            {t("commodities:fields.count")}
          </FieldLabel>
          <Input
            id="commodity-count"
            type="number"
            min={1}
            aria-required
            {...register("count")}
            aria-invalid={!!errors.count}
          />
          <FieldError error={errors.count} />
        </div>
      </div>

      {/* Location/Area picker — edit mode only (#1987). The create
          dialog no longer asks for a location; items can be created
          unassigned and a location offered afterwards via a banner.
          In edit mode the picker also lets the user un-assign (Clear)
          since area is now optional (#1986). */}
      {showLocationPicker ? (
        <LocationAreaFields
          areas={areas}
          locations={locations}
          areaId={watch("area_id") ?? ""}
          // Validating drops a server error left on the area by the last save.
          onAreaChange={(next) =>
            setValue("area_id", next, { shouldDirty: true, shouldValidate: true })
          }
          idPrefix="commodity"
          areaError={errors.area_id}
          renderLabel={(htmlFor, text) => <FieldLabel htmlFor={htmlFor}>{text}</FieldLabel>}
        />
      ) : null}

      {/* The status is shown, not changed, here (#2915): leaving In Use
          needs a date, and Sold a sale price, which Change Status on the
          item page asks for. */}
      {showStatus ? (
        <div className="flex flex-col gap-1.5">
          <FieldLabel htmlFor="commodity-status">{t("commodities:fields.status")}</FieldLabel>
          <Select value={watch("status") || undefined} disabled>
            <SelectTrigger
              id="commodity-status"
              className="w-full"
              aria-describedby="commodity-status-help"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {COMMODITY_STATUSES.map((s) => (
                <SelectItem key={s} value={s}>
                  {t(`commodities:status.${s}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p id="commodity-status-help" className="text-xs text-muted-foreground">
            {t("commodities:form.statusHelp")}
          </p>
          <FieldError error={errors.status} />
        </div>
      ) : null}
    </div>
  )
}

// ---- Step 2: Purchase ---------------------------------------------------

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- see BasicsStep
function PurchaseStep(props: any) {
  const { t } = useTranslation()
  const {
    register,
    control,
    errors,
    watch,
    defaultCurrency,
    showSeller,
    scannedSeller,
    onScannedSellerDone,
  } = props
  // No Purchase field is required, on a draft or a reviewed item: a
  // blank date or price is saved as unknown, "0" as a known zero (a
  // gift, a free item). See `features/commodities/schemas.ts`.
  // Foreign-currency check mirrors the mock's `isForeignCurrency`
  // (AddItemDialog L1154). When the purchase currency matches the
  // group's currency the converted-price field is moot — the original
  // price is already in group currency. Surface it only when the two
  // diverge so we don't make the user re-type the same amount.
  const purchaseCurrency = (watch("original_price_currency") as string | undefined) ?? ""
  const groupCurrency = (defaultCurrency as string | undefined) ?? ""
  const isForeignCurrency =
    !!purchaseCurrency &&
    !!groupCurrency &&
    purchaseCurrency.toUpperCase() !== groupCurrency.toUpperCase()
  // Inline price-input prefix mirrors the picked currency's symbol
  // (mock AddItemDialog L1153 + L1177 reads `currencySymbol` from the
  // CURRENCIES list). Falls back to the bare code when no metadata is
  // known so unfamiliar currencies still render legibly.
  const purchaseSymbol = currencyMeta(purchaseCurrency || groupCurrency || "USD").symbol
  const groupSymbol = currencyMeta(groupCurrency || "USD").symbol
  const purchasePadClass = priceInputPaddingClass(purchaseSymbol)
  const groupPadClass = priceInputPaddingClass(groupSymbol)
  return (
    <div className="space-y-4 py-2" data-testid="commodity-form-purchase-step">
      <div className="flex flex-col gap-1.5">
        <FieldLabel htmlFor="commodity-purchase-date">
          {t("commodities:fields.purchaseDate")}
        </FieldLabel>
        <Input
          id="commodity-purchase-date"
          type="date"
          aria-describedby="commodity-purchase-date-help"
          {...register("purchase_date")}
          aria-invalid={!!errors.purchase_date}
        />
        {errors.purchase_date ? (
          <FieldError error={errors.purchase_date} />
        ) : (
          <p id="commodity-purchase-date-help" className="text-xs text-muted-foreground">
            {t("commodities:fields.purchaseDateHelp")}
          </p>
        )}
      </div>

      {showSeller ? (
        <Controller
          control={control}
          name="seller_id"
          render={({ field }) => (
            <SellerField
              id="commodity-seller"
              value={field.value ?? ""}
              onChange={(next) => field.onChange(next)}
              error={errors.seller_id}
              suggestion={scannedSeller}
              onSuggestionDone={onScannedSellerDone}
              renderLabel={(htmlFor, text) => <FieldLabel htmlFor={htmlFor}>{text}</FieldLabel>}
            />
          )}
        />
      ) : null}

      {/* Mock AddItemDialog L1173-L1196: combined "Purchase Price" row
          — price input with leading currency symbol on the left, the
          compact CurrencyCombobox on the right. The price field grows
          (flex-1) and the combobox holds a fixed code-width so the
          two never wrestle for space. */}
      <div className="flex flex-col gap-1.5">
        <FieldLabel htmlFor="commodity-original-price">
          {t("commodities:fields.originalPrice")}
        </FieldLabel>
        <div className="flex gap-2">
          <div className="relative flex-1">
            <span
              aria-hidden="true"
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 select-none text-sm text-muted-foreground"
            >
              {purchaseSymbol}
            </span>
            <Input
              id="commodity-original-price"
              type="number"
              step="0.01"
              min={0}
              placeholder="0"
              className={purchasePadClass}
              aria-describedby="commodity-original-price-help"
              {...register("original_price")}
              aria-invalid={!!errors.original_price}
            />
          </div>
          <Controller
            control={control}
            name="original_price_currency"
            render={({ field }) => (
              <CurrencyCombobox
                id="commodity-currency"
                value={field.value ?? ""}
                onChange={field.onChange}
                ariaInvalid={!!errors.original_price_currency}
                variant="compact"
              />
            )}
          />
        </div>
        {errors.original_price ? (
          <FieldError error={errors.original_price} />
        ) : errors.original_price_currency ? (
          <FieldError error={errors.original_price_currency} />
        ) : (
          <p id="commodity-original-price-help" className="text-xs text-muted-foreground">
            {t("commodities:fields.originalPriceHelp")}
          </p>
        )}
      </div>

      {/* Mock AddItemDialog L1198-L1233: foreign-currency variant
          renders an amber-bordered card with the "Foreign currency
          detected" banner, the Converted Purchase Price field, an OR
          divider, and the Current Value field. Same-currency drops
          the converted-price field entirely (the original price is
          already in group currency) and renders a plain Current
          Value field below. */}
      {isForeignCurrency ? (
        <div className="flex flex-col gap-3 rounded-lg border border-amber-300/60 bg-amber-50 p-3 dark:border-amber-900/60 dark:bg-amber-950/30">
          <p className="text-xs leading-relaxed text-amber-900 dark:text-amber-200">
            <span className="font-semibold">{t("commodities:fields.foreignCurrencyDetected")}</span>{" "}
            {t("commodities:fields.foreignCurrencyBanner", { groupCurrency })}
          </p>
          {/* Foreign-currency mode: a known purchase price needs one
              of (converted, current) to be known, a zero included;
              neither field is individually required, so neither
              carries the asterisk. The "or" divider makes the OR
              relationship visible; schema-level
              NO_PRICE_IN_GROUP_CURRENCY catches the both-blank case. */}
          <div className="flex flex-col gap-1.5">
            <FieldLabel htmlFor="commodity-converted-price">
              {t("commodities:fields.convertedOriginalPrice", { groupCurrency })}
            </FieldLabel>
            <div className="relative">
              <span
                aria-hidden="true"
                className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 select-none text-sm text-muted-foreground"
              >
                {groupSymbol}
              </span>
              <Input
                id="commodity-converted-price"
                type="number"
                step="0.01"
                min={0}
                placeholder="0"
                className={cn("bg-background", groupPadClass)}
                {...register("converted_original_price")}
                aria-invalid={!!errors.converted_original_price}
              />
            </div>
            {errors.converted_original_price ? (
              <FieldError error={errors.converted_original_price} />
            ) : (
              <p className="text-xs text-amber-800/80 dark:text-amber-300/80">
                {t("commodities:fields.convertedOriginalPriceHint")}
              </p>
            )}
          </div>
          <div className="flex items-center gap-2">
            <div className="h-px flex-1 bg-amber-300/60 dark:bg-amber-900/60" />
            <span className="text-[10px] font-medium uppercase tracking-wide text-amber-700 dark:text-amber-300">
              {t("commodities:fields.foreignCurrencyOr")}
            </span>
            <div className="h-px flex-1 bg-amber-300/60 dark:bg-amber-900/60" />
          </div>
          <div className="flex flex-col gap-1.5">
            <FieldLabel htmlFor="commodity-current-price">
              {t("commodities:fields.currentPriceForeign", { groupCurrency })}
            </FieldLabel>
            <div className="relative">
              <span
                aria-hidden="true"
                className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 select-none text-sm text-muted-foreground"
              >
                {groupSymbol}
              </span>
              <Input
                id="commodity-current-price"
                type="number"
                step="0.01"
                min={0}
                placeholder="0"
                className={cn("bg-background", groupPadClass)}
                {...register("current_price")}
                aria-invalid={!!errors.current_price}
              />
            </div>
            {errors.current_price ? (
              <FieldError error={errors.current_price} />
            ) : (
              <p className="text-xs text-amber-800/80 dark:text-amber-300/80">
                {t("commodities:fields.currentPriceHelp")}
              </p>
            )}
          </div>
        </div>
      ) : (
        <div className="flex flex-col gap-1.5">
          {/* #1625: same-currency Current Value is optional — the original
              price already carries the canonical amount in the group's
              currency. The label drops the "required" asterisk and the
              input drops aria-required accordingly. */}
          <FieldLabel htmlFor="commodity-current-price">
            {t("commodities:fields.currentPrice")}
          </FieldLabel>
          <div className="relative">
            <span
              aria-hidden="true"
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 select-none text-sm text-muted-foreground"
            >
              {groupSymbol}
            </span>
            <Input
              id="commodity-current-price"
              type="number"
              step="0.01"
              min={0}
              placeholder="0"
              className={groupPadClass}
              {...register("current_price")}
              aria-invalid={!!errors.current_price}
            />
          </div>
          {errors.current_price ? (
            <FieldError error={errors.current_price} />
          ) : (
            <p className="text-xs text-muted-foreground">
              {t("commodities:fields.currentPriceHelp")}
            </p>
          )}
        </div>
      )}

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-serial">{t("commodities:fields.serialNumber")}</Label>
        <Input
          id="commodity-serial"
          className="font-mono text-sm"
          placeholder={t("commodities:fields.serialNumberPlaceholder")}
          {...register("serial_number")}
        />
        <p className="text-xs text-muted-foreground">{t("commodities:fields.serialNumberHelp")}</p>
      </div>
    </div>
  )
}

// ---- Step 3: Warranty ---------------------------------------------------

// WarrantyStep renders the first-class warranty inputs (#1367):
// expiry date + notes. Status (active/expiring/expired/none) is
// computed live from the entered date and shown as a pill preview so
// the user sees how the row will surface on the list page before
// saving.
//
// On bundles (#1554, count > 1) the inputs are disabled ONLY when
// they're empty — i.e. there's nothing for the user to clean up. Per-
// field disabling lets a legacy bundle that already carries warranty
// data (the migration is log-only, so legacy rows pass through
// unmodified) be cleared from the UI. The same step always renders the
// "split into separate items" hint so the disabled state never looks
// like a bug.
//
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- see BasicsStep
function WarrantyStep(props: any) {
  const { t } = useTranslation()
  const { register, errors, watch, isBundle } = props
  const expiresAt = watch("warranty_expires_at") as string | undefined
  const notes = watch("warranty_notes") as string | undefined
  const status = warrantyStatusFromDate(expiresAt)
  // Only disable when the field is empty AND the row is a bundle. A
  // populated input stays editable so the user can clear / fix legacy
  // data that pre-dates the constraint.
  const expiresAtEmpty = !expiresAt || expiresAt.trim() === ""
  const notesEmpty = !notes || notes.trim() === ""
  const expiresAtDisabled = isBundle && expiresAtEmpty
  const notesDisabled = isBundle && notesEmpty
  return (
    <div className="space-y-4 py-2" data-testid="commodity-form-warranty-step">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-warranty-expires-at">
          {t("commodities:fields.warrantyExpiresAt")}
        </Label>
        <Input
          id="commodity-warranty-expires-at"
          type="date"
          {...register("warranty_expires_at")}
          aria-invalid={!!errors.warranty_expires_at}
          disabled={expiresAtDisabled}
          data-testid="commodity-form-warranty-expires-at"
        />
        {errors.warranty_expires_at ? (
          <FieldError error={errors.warranty_expires_at} />
        ) : (
          <p className="text-xs text-muted-foreground">
            {isBundle
              ? t("commodities:trackingRestrictions.warrantyStepHint")
              : t("commodities:fields.warrantyExpiresAtHelp")}
          </p>
        )}
      </div>
      {status !== "none" && !isBundle ? (
        <WarrantyBadge
          status={status}
          className="w-fit"
          data-testid="commodity-form-warranty-status"
        />
      ) : null}
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-warranty-notes">{t("commodities:fields.warrantyNotes")}</Label>
        <Textarea
          id="commodity-warranty-notes"
          rows={3}
          className="resize-none"
          placeholder={t("commodities:fields.warrantyNotesPlaceholder")}
          {...register("warranty_notes")}
          aria-invalid={!!errors.warranty_notes}
          disabled={notesDisabled}
          data-testid="commodity-form-warranty-notes"
        />
        <FieldError error={errors.warranty_notes} />
      </div>
    </div>
  )
}

// warrantyStatusFromDate is the live preview equivalent of the BE's
// ComputeWarrantyStatus — delegates to the shared `warrantyStatus()`
// helper so the form pill, the list-page pill, the BE filter, and
// the worker reminder cadence all agree on the 60-day boundary +
// UTC-midnight anchor. Returns "none" for empty / unparseable input
// so the preview block stays hidden.
function warrantyStatusFromDate(d: string | undefined) {
  return warrantyStatus({ warranty_expires_at: d })
}

// ---- Step 4: Extras -----------------------------------------------------

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- see BasicsStep
function ExtrasStep(props: any) {
  const { t } = useTranslation()
  const tagLabel = useTagLabel("commodity")
  // customFieldsEditor: the custom fields section (#2803), or null.
  const { register, errors, watch, setValue } = props
  const extraSerials: string[] = watch("extra_serial_numbers") ?? []
  const partNumbers: string[] = watch("part_numbers") ?? []
  // Reveal-on-click toggles for the two rarely-used numeric fields.
  // Auto-revealed when the field already carries values (edit mode,
  // or after the user navigates back to this step) so we don't hide
  // data the user already entered.
  const [revealExtraSerials, setRevealExtraSerials] = useState(false)
  const [revealPartNumbers, setRevealPartNumbers] = useState(false)
  const showExtraSerials = revealExtraSerials || extraSerials.length > 0
  const showPartNumbers = revealPartNumbers || partNumbers.length > 0
  return (
    <div className="space-y-4 py-2" data-testid="commodity-form-extras-step">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="commodity-comments">{t("commodities:fields.comments")}</Label>
        <Textarea
          id="commodity-comments"
          rows={3}
          className="resize-none"
          placeholder={t("commodities:fields.commentsPlaceholder")}
          {...register("comments")}
          aria-invalid={!!errors.comments}
        />
        <FieldError error={errors.comments} />
      </div>
      {/* Tags: tinted-card CTA wrapper + inline suggestion chips when
          empty. Conscious deviation from the mock's flat input — see
          devdocs/frontend/design-deviations.md (Items / Commodities).
          The chips are top group tags from `useTagAutocomplete("")` —
          one tap drops the slug into `values`, after which the chip
          row hides and the user falls back to the standard popover-
          on-focus dropdown via TagsInput's `autocomplete`. */}
      <div className="flex flex-col gap-2 rounded-xl border border-border bg-muted/20 p-3">
        <div className="flex items-center gap-2">
          <div className="flex size-6 items-center justify-center rounded-md bg-chart-1/15">
            <TagIcon aria-hidden="true" className="size-3.5 text-chart-1" />
          </div>
          <Label htmlFor="commodity-tags-input" className="text-sm font-medium">
            {t("commodities:fields.tags")}
          </Label>
        </div>
        <TagsInput
          values={watch("tags")}
          onChange={(next) => setValue("tags", next, { shouldDirty: true })}
          placeholder={t("commodities:fields.tagsPlaceholder")}
          testId="commodity-tags"
          autocomplete
          kind="commodity"
          labelOf={tagLabel}
        />
        <p className="text-xs text-muted-foreground">{t("commodities:fields.tagsHelp")}</p>
        <TagsSuggestionChips
          selected={watch("tags") ?? []}
          onPick={(slug) =>
            setValue("tags", [...(watch("tags") ?? []), slug], {
              shouldDirty: true,
              shouldValidate: false,
            })
          }
        />
      </div>
      {showExtraSerials ? (
        <div className="animate-in fade-in slide-in-from-top-1 duration-200">
          <ChipInput
            label={t("commodities:fields.extraSerialNumbers")}
            helper={t("commodities:fields.extraSerialNumbersHelp")}
            values={extraSerials}
            onChange={(next) => setValue("extra_serial_numbers", next, { shouldDirty: true })}
            testId="commodity-extra-serials"
          />
        </div>
      ) : (
        <button
          type="button"
          onClick={() => setRevealExtraSerials(true)}
          className="flex items-center gap-1 self-start text-xs text-muted-foreground transition-colors hover:text-foreground"
          data-testid="commodity-extra-serials-reveal"
        >
          <ChevronDown className="size-3.5" aria-hidden="true" />
          {t("commodities:fields.revealExtraSerials")}
        </button>
      )}
      {showPartNumbers ? (
        <div className="animate-in fade-in slide-in-from-top-1 duration-200">
          <ChipInput
            label={t("commodities:fields.partNumbers")}
            helper={t("commodities:fields.partNumbersHelp")}
            values={partNumbers}
            onChange={(next) => setValue("part_numbers", next, { shouldDirty: true })}
            testId="commodity-part-numbers"
          />
        </div>
      ) : (
        <button
          type="button"
          onClick={() => setRevealPartNumbers(true)}
          className="flex items-center gap-1 self-start text-xs text-muted-foreground transition-colors hover:text-foreground"
          data-testid="commodity-part-numbers-reveal"
        >
          <ChevronDown className="size-3.5" aria-hidden="true" />
          {t("commodities:fields.revealPartNumbers")}
        </button>
      )}
      {props.customFieldsEditor}
    </div>
  )
}

// ---- Step 5: Files (stub) ----------------------------------------------

// `uploadPendingFiles` lives in features/commodities/draft.ts (imported
// above) so the anonymous first-item resolver can replay the same
// upload+link flow after login.

interface FilesStepProps {
  pendingFiles: PendingFile[]
  setPendingFiles: (next: PendingFile[] | ((prev: PendingFile[]) => PendingFile[])) => void
  // Takes a staged file off the form; defaults to dropping it from the list.
  onRemovePending?: (entry: PendingFile) => void
  // The status of each staged file in the save that runs or last ran.
  statuses?: Record<string, FileUploadStatus>
  // A save runs: files can be neither added, edited nor removed.
  busy?: boolean
  // Files already saved with the server draft the form edits (#2812).
  savedFiles?: ListedFile[]
  onRemoveSaved?: (row: ListedFile) => void
  // Saves a saved file's title and tags through the files API (#1626).
  // Resolves whether it worked.
  onUpdateSaved?: (row: ListedFile, change: { title: string; tags: string[] }) => Promise<boolean>
}

// FilesStep collects attachments locally; they reach the server when the
// form is saved. Conscious deviation from the mock's three-bucket layout: a
// single dropzone with auto-categorisation by MIME (images / documents /
// other), and an inline title and tag chip-input per file the user can fill
// while still inside the wizard. While a save runs, each file shows whether
// it is uploading, uploaded or failed (#1626).
function FilesStep({
  pendingFiles,
  setPendingFiles,
  onRemovePending,
  statuses = {},
  busy = false,
  savedFiles = [],
  onRemoveSaved,
  onUpdateSaved,
}: FilesStepProps) {
  const { t } = useTranslation()
  // `useId` so the input/label association is stable but unique even
  // if multiple FilesSteps mount in the same tree (and so React's HMR
  // reconciliation doesn't see id collisions).
  const inputId = useId()
  const [dragging, setDragging] = useState(false)
  function add(files: File[]) {
    if (files.length === 0 || busy) return
    // mergePendingFiles de-dupes by file signature, so re-picking a file
    // that's already staged (incl. one carried over from the AI scan,
    // #1983) is a no-op rather than a double-attach.
    setPendingFiles((prev) => mergePendingFiles(prev, files))
  }
  function remove(entry: PendingFile) {
    if (onRemovePending) onRemovePending(entry)
    else setPendingFiles((prev) => prev.filter((pending) => pending.id !== entry.id))
  }
  function patch(id: string, change: (entry: PendingFile) => PendingFile) {
    setPendingFiles((prev) => prev.map((entry) => (entry.id === id ? change(entry) : entry)))
  }
  // A file a save is still placing is listed as staged, not twice.
  const staged = new Set(pendingFiles.flatMap((entry) => entry.uploadedFileId ?? []))
  const saved = savedFiles.filter((row) => !staged.has(row.file.id))
  return (
    <div className="min-w-0 space-y-3 py-2" data-testid="commodity-form-files-step">
      <p className="text-xs text-muted-foreground">{t("commodities:form.step.files.intro")}</p>
      {/* `<label htmlFor>` activates the file input natively on tap —
          no JS-driven `.click()` that some Android Chrome builds drop
          when the user-gesture context crosses event handlers. The
          input itself is hidden inside the same label so the gesture
          stays unbroken from tap → picker. */}
      <label
        htmlFor={inputId}
        aria-disabled={busy || undefined}
        onDragOver={(e) => {
          e.preventDefault()
          if (!busy) setDragging(true)
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault()
          setDragging(false)
          add(Array.from(e.dataTransfer.files ?? []))
        }}
        className={cn(
          "flex w-full cursor-pointer flex-col items-center justify-center gap-1.5 rounded-xl border-2 border-dashed py-6 transition-colors",
          busy
            ? "cursor-not-allowed border-border opacity-60"
            : dragging
              ? "border-primary bg-primary/5"
              : "border-border hover:border-primary/40 hover:bg-muted/30"
        )}
        data-testid="commodity-files-dropzone"
      >
        <Upload aria-hidden="true" className="size-5 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">{t("commodities:form.step.files.dropzone")}</p>
        <p className="text-xs text-muted-foreground">{t("commodities:form.step.files.hint")}</p>
        <input
          id={inputId}
          type="file"
          multiple
          disabled={busy}
          className="sr-only"
          onChange={(e) => {
            add(Array.from(e.target.files ?? []))
            e.target.value = ""
          }}
        />
      </label>
      {saved.length > 0 ? (
        <div className="min-w-0 space-y-1.5" data-testid="draft-saved-files">
          <p className="text-xs font-medium text-muted-foreground">
            {t("commodities:drafts.savedFiles")}
          </p>
          <ul className="flex min-w-0 flex-col gap-1.5">
            {saved.map((row) => (
              <SavedFileRow
                key={row.file.id}
                row={row}
                busy={busy}
                onRemove={onRemoveSaved && !busy ? () => onRemoveSaved(row) : undefined}
                onSave={onUpdateSaved ? (change) => onUpdateSaved(row, change) : undefined}
              />
            ))}
          </ul>
        </div>
      ) : null}
      {pendingFiles.length > 0 ? (
        <ul className="flex min-w-0 flex-col gap-1.5">
          {pendingFiles.map((entry) => (
            <PendingFileRow
              key={entry.id}
              entry={entry}
              status={statuses[entry.id]}
              busy={busy}
              onRemove={() => remove(entry)}
              onTagsChange={(tags) => patch(entry.id, (e) => withTags(e, tags))}
              onTitleChange={(title) => patch(entry.id, (e) => withTitle(e, title))}
            />
          ))}
        </ul>
      ) : null}
    </div>
  )
}

// useFileTagLabel names a file tag: the group's label for it, or for one of
// the curated tags the group has no tag for yet (a tag the scan suggests),
// the pill's translated name.
function useFileTagLabel(): (slug: string) => string {
  const tagLabel = useTagLabel("file")
  const pillLabel = useTagPillLabel()
  return (slug) => {
    const label = tagLabel(slug)
    const pill = FILE_TAG_PILLS.find((p) => p.id === slug)
    return label === slug && pill ? pillLabel(pill.id) : label
  }
}

// The color of a curated file tag, for its suggestion chip.
function fileTagColor(slug: string): string | undefined {
  return FILE_TAG_PILLS.find((p) => p.id === slug)?.colorClass
}

interface SavedFileRowProps {
  row: ListedFile
  busy: boolean
  onRemove?: () => void
  // Saves the title and the tags; without it they are shown read-only.
  onSave?: (change: { title: string; tags: string[] }) => Promise<boolean>
}

// SavedFileRow is a file the server draft already holds: it needs no
// upload, and removing it takes it off the draft. Its title and tags can be
// changed here (#1626), so the ones batch intake gave it are seen and put
// right before the item is added; a change is saved on the file at once, a
// title when the field loses focus.
function SavedFileRow({ row, busy, onRemove, onSave }: SavedFileRowProps) {
  const { t } = useTranslation()
  const tagLabel = useFileTagLabel()
  const titleId = useId()
  const name = `${row.file.path || row.file.title || ""}${row.file.ext ?? ""}`
  const defaultTitle = row.file.path || row.file.title || ""
  const serverTitle = row.file.title ?? ""
  const serverTags = Array.isArray(row.file.tags) ? row.file.tags : []
  // What the user changed and is not saved yet; null shows the file's value.
  const [draftTitle, setDraftTitle] = useState<string | null>(null)
  const [draftTags, setDraftTags] = useState<string[] | null>(null)
  const isImage = row.file.category === "images"
  const Icon = isImage ? ImageIcon : row.file.category === "documents" ? BookOpen : FileIcon
  const readOnly = !onSave
  const titleToSave = (raw: string) => capFileTitle(raw.trim()) || defaultTitle

  async function commitTitle() {
    if (draftTitle === null) return
    const next = titleToSave(draftTitle)
    if (next === serverTitle) {
      setDraftTitle(null)
      return
    }
    const typed = draftTitle
    await onSave?.({ title: next, tags: draftTags ?? serverTags })
    // Saved or not, the field shows the file's title again, unless the user
    // typed on meanwhile.
    setDraftTitle((current) => (current === typed ? null : current))
  }

  async function changeTags(next: string[]) {
    setDraftTags(next)
    await onSave?.({
      title: draftTitle !== null ? titleToSave(draftTitle) : serverTitle,
      tags: next,
    })
    setDraftTags((current) => (current === next ? null : current))
  }

  return (
    <li
      className="flex min-w-0 flex-col gap-1.5 overflow-hidden rounded-lg border border-border bg-card px-3 py-2"
      data-testid={`draft-saved-file-${row.file.id}`}
    >
      <div className="flex min-w-0 items-center gap-2">
        <div className="flex size-6 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
          <Icon aria-hidden="true" className="size-3.5" />
        </div>
        <p className="min-w-0 flex-1 truncate text-sm font-medium">{name}</p>
        <span className="shrink-0 text-xs text-muted-foreground">
          {formatBytes(row.file.size_bytes ?? 0)}
        </span>
        {onRemove ? (
          <button
            type="button"
            aria-label={t("commodities:drafts.removeFile", { name })}
            onClick={onRemove}
            className="shrink-0 text-muted-foreground hover:text-foreground"
          >
            <X aria-hidden="true" className="size-3.5" />
          </button>
        ) : null}
      </div>
      <fieldset disabled={busy || readOnly} className="flex min-w-0 flex-col gap-1.5">
        <Label htmlFor={titleId} className="sr-only">
          {t("commodities:form.step.files.titleLabel", { name })}
        </Label>
        <Input
          id={titleId}
          value={draftTitle ?? serverTitle}
          placeholder={defaultTitle}
          onChange={(e) => setDraftTitle(capFileTitle(e.target.value))}
          onBlur={() => void commitTitle()}
          onKeyDown={(e) => {
            // Enter saves the title instead of submitting the whole form.
            if (e.key === "Enter") {
              e.preventDefault()
              void commitTitle()
            }
          }}
          className="h-7 px-1.5 text-xs md:text-xs"
          data-testid={`draft-saved-file-title-${row.file.id}`}
        />
        <TagsInput
          values={draftTags ?? serverTags}
          onChange={(next) => void changeTags(next)}
          placeholder={t("commodities:form.step.files.tagsPlaceholder")}
          testId={`draft-saved-file-tags-${row.file.id}`}
          autocomplete
          compact
          kind="file"
          labelOf={tagLabel}
        />
      </fieldset>
    </li>
  )
}

interface PendingFileRowProps {
  entry: PendingFile
  status?: FileUploadStatus
  busy: boolean
  onRemove: () => void
  onTagsChange: (tags: string[]) => void
  onTitleChange: (title: string) => void
}

function PendingFileRow({
  entry,
  status,
  busy,
  onRemove,
  onTagsChange,
  onTitleChange,
}: PendingFileRowProps) {
  const { t } = useTranslation()
  const tagLabel = useFileTagLabel()
  const titleId = useId()
  const suggestedTitleId = useId()
  const category = categoryFromMime(entry.file.type)
  const defaultTitle = defaultFileTitle(entry.file)
  // What an AI scan put on the file and the user kept (#1626).
  const suggestedTag = suggestedTagOf(entry)
  const titleSuggested = hasSuggestedTitle(entry)
  // Pick a small visual cue per derived category. Stays consistent
  // with other surfaces' chart-* tinting (Photos = green status,
  // Documents = blue chart-3, Other = neutral muted).
  const categoryClass =
    category === "images"
      ? "bg-status-active/15 text-status-active"
      : category === "documents"
        ? "bg-chart-3/15 text-chart-3"
        : "bg-muted text-muted-foreground"
  const CategoryIcon =
    category === "images" ? ImageIcon : category === "documents" ? BookOpen : FileIcon
  const categoryLabel =
    category === "images"
      ? t("files:categoryImages")
      : category === "documents"
        ? t("files:categoryDocuments")
        : t("files:categoryOther")
  return (
    <li
      className="flex min-w-0 flex-col gap-1.5 overflow-hidden rounded-lg border border-border bg-card px-3 py-2"
      data-testid="commodity-files-pending"
      data-status={status ?? "staged"}
    >
      <div className="flex min-w-0 items-center gap-2">
        <div
          className={cn(
            "flex size-6 shrink-0 items-center justify-center rounded-md",
            categoryClass
          )}
        >
          <CategoryIcon aria-hidden="true" className="size-3.5" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">
            {entry.file.name}
            {status ? (
              <span className="sr-only">
                {" "}
                {status === "uploading"
                  ? t("commodities:form.step.files.status.uploading")
                  : status === "uploaded"
                    ? t("commodities:form.step.files.status.uploaded")
                    : t("commodities:form.step.files.status.failed")}
              </span>
            ) : null}
          </p>
          {status === "failed" ? (
            <p className="text-[11px] leading-tight text-destructive" aria-hidden="true">
              {t("commodities:form.step.files.status.failed")}
            </p>
          ) : (
            <p
              className="text-[11px] leading-tight text-muted-foreground"
              title={t("commodities:form.step.files.categoryAutoTitle") ?? undefined}
            >
              <Trans
                i18nKey="commodities:form.step.files.categoryAuto"
                values={{ category: categoryLabel }}
                components={{
                  1: (
                    <span
                      className={cn(
                        "inline-flex items-center rounded px-1 font-medium",
                        categoryClass
                      )}
                    />
                  ),
                }}
              />
            </p>
          )}
        </div>
        <FileUploadIcon status={status} />
        <span className="shrink-0 text-xs text-muted-foreground">
          {formatBytes(entry.file.size)}
        </span>
        {!busy ? (
          <button
            type="button"
            aria-label={t("common:actions.delete")}
            onClick={onRemove}
            className="shrink-0 text-muted-foreground hover:text-foreground"
          >
            <X aria-hidden="true" className="size-3.5" />
          </button>
        ) : null}
      </div>
      {/* A fieldset so one flag locks both inputs while a save runs. */}
      <fieldset disabled={busy} className="flex min-w-0 flex-col gap-1.5">
        <Label htmlFor={titleId} className="sr-only">
          {t("commodities:form.step.files.titleLabel", { name: entry.file.name })}
        </Label>
        <div className="flex min-w-0 items-center gap-1.5">
          {titleSuggested ? (
            <>
              <Sparkles
                aria-hidden="true"
                className="size-3.5 shrink-0 text-amber-500"
                data-testid={`commodity-files-title-suggested-${entry.id}`}
              />
              <span id={suggestedTitleId} className="sr-only">
                {t("commodities:form.step.files.suggestedTitle")}
              </span>
            </>
          ) : null}
          <Input
            id={titleId}
            value={entry.title ?? defaultTitle}
            placeholder={defaultTitle}
            aria-describedby={titleSuggested ? suggestedTitleId : undefined}
            title={titleSuggested ? t("commodities:form.step.files.suggestedTitle") : undefined}
            onChange={(e) => onTitleChange(capFileTitle(e.target.value))}
            onKeyDown={(e) => {
              // Enter in a file title would submit the whole form.
              if (e.key === "Enter") e.preventDefault()
            }}
            className="h-7 min-w-0 flex-1 px-1.5 text-xs md:text-xs"
            data-testid={`commodity-files-title-${entry.id}`}
          />
        </div>
        <TagsInput
          values={entry.tags}
          onChange={onTagsChange}
          placeholder={t("commodities:form.step.files.tagsPlaceholder")}
          testId={`commodity-files-tags-${entry.id}`}
          autocomplete
          compact
          kind="file"
          labelOf={tagLabel}
          suggested={suggestedTag ? [suggestedTag] : undefined}
          suggestedLabel={t("commodities:form.step.files.suggestedTag")}
          chipClassName={(value) => (value === suggestedTag ? fileTagColor(value) : undefined)}
        />
      </fieldset>
    </li>
  )
}

// FileUploadIcon shows where a staged file is in the save, as the batch
// intake dialog does; nothing while it waits.
function FileUploadIcon({ status }: { status?: FileUploadStatus }) {
  const className = "size-4 shrink-0"
  if (status === "uploading") {
    return (
      <Loader2 className={cn(className, "animate-spin text-muted-foreground")} aria-hidden="true" />
    )
  }
  if (status === "uploaded") {
    return <CheckCircle2 className={cn(className, "text-status-active")} aria-hidden="true" />
  }
  if (status === "failed") {
    return <AlertCircle className={cn(className, "text-destructive")} aria-hidden="true" />
  }
  return null
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

// ---- Helpers ------------------------------------------------------------

// TagsSuggestionChips renders 5 ghost-styled, tappable chips of the
// most popular group tags below the Tags input on the Extras step.
// One tap drops the slug into `selected` via `onPick`. The component
// hides itself once the user has any tag selected — at that point the
// regular popover-on-focus dropdown (built into TagsInput) takes
// over. The CTA's job is the empty-state nudge, not a permanent
// fixture.
//
// Reads from the same `useTagAutocomplete("")` query the TagsInput's
// AutocompleteSink uses; TanStack dedupes by query key, so a single
// network request feeds both surfaces.
function TagsSuggestionChips({
  selected,
  onPick,
  testId,
}: {
  selected: string[]
  onPick: (slug: string) => void
  testId?: string
}) {
  // Commodity-kind — chips on the Extras step are a nudge for tagging
  // the commodity itself, so the candidate pool is item-tags only.
  const remote = useTagAutocomplete("", 8, { enabled: true, kind: "commodity" })
  // Hide once the user has selected any tag — the chips' job was the
  // first-tag nudge.
  if (selected.length > 0) return null
  // Picking a chip stores the slug; the chip itself shows the label.
  const candidates = (remote.data ?? []).filter((tag) => !selected.includes(tag.slug)).slice(0, 5)
  if (candidates.length === 0) return null
  return (
    <div
      className="flex flex-wrap gap-1.5 animate-in fade-in duration-200"
      data-testid={testId ?? "commodity-tags-suggestions"}
    >
      {candidates.map((tag) => (
        <button
          key={tag.slug}
          type="button"
          onClick={() => onPick(tag.slug)}
          className="inline-flex items-center gap-1 rounded-full border border-dashed border-border bg-background px-2 py-0.5 text-xs text-muted-foreground transition-colors hover:border-primary/40 hover:bg-muted/40 hover:text-foreground"
        >
          <Plus aria-hidden="true" className="size-3" />
          {tag.label || tag.slug}
        </button>
      ))}
    </div>
  )
}

// StepResizeWrapper drives an explicit pixel height on the wizard
// step container so the dialog height interpolates smoothly when the
// user navigates Basics ↔ Purchase ↔ … ↔ Files (each step has a
// different natural height). `interpolate-size: allow-keywords` alone
// doesn't catch this — React swaps the children in one synchronous
// commit, so the browser never sees two distinct auto-resolved
// heights to interpolate between. ResizeObserver gives us the
// post-layout pixel height; we feed that back into the wrapper's
// inline style and let the CSS `transition-[height]` rule animate
// between the old and new pixel values.
//
// The height applies from `sm` up only. Below `sm` the dialog fills the
// screen and scrolls, so a step keeps its natural height and swaps
// without animating a box whose size the user cannot see change.
function StepResizeWrapper({ children }: { children: ReactNode }) {
  const innerRef = useRef<HTMLDivElement>(null)
  const [height, setHeight] = useState<number | null>(null)
  // First measurement commits without animation so the dialog opens
  // at its natural size instead of expanding into it from 0.
  const [transitionsReady, setTransitionsReady] = useState(false)
  useEffect(() => {
    const node = innerRef.current
    if (!node) return
    const obs = new ResizeObserver(([entry]) => {
      const next = entry.contentRect.height
      setHeight((prev) => (prev === next ? prev : next))
    })
    obs.observe(node)
    return () => obs.disconnect()
  }, [])
  useEffect(() => {
    if (height === null || transitionsReady) return
    // Defer enabling transitions until after the first measured
    // height has actually committed to the DOM, so the initial
    // height: null → height: <px> swap is paint-instant.
    const id = window.requestAnimationFrame(() => setTransitionsReady(true))
    return () => window.cancelAnimationFrame(id)
  }, [height, transitionsReady])
  return (
    <div
      style={height === null ? undefined : { height: `${height}px` }}
      className={cn(
        // `overflow-clip` + `overflow-clip-margin` extends the clip
        // box outward so focus rings (3px outside the input box) stay
        // visible — `overflow-hidden` was eating them on inputs near
        // the wrapper edge.
        "overflow-clip [overflow-clip-margin:6px]",
        // Only an important rule overrides the inline height.
        "max-sm:h-auto!",
        transitionsReady && "sm:transition-[height] sm:duration-200 sm:ease-out"
      )}
    >
      <div ref={innerRef}>{children}</div>
    </div>
  )
}

interface ChipInputProps {
  label: string
  helper?: string
  placeholder?: string
  values: string[]
  onChange: (next: string[]) => void
  testId?: string
}

function ChipInput({ label, helper, placeholder, values, onChange, testId }: ChipInputProps) {
  const [draft, setDraft] = useState("")
  function commit() {
    const trimmed = draft.trim()
    if (!trimmed) return
    if (values.includes(trimmed)) {
      setDraft("")
      return
    }
    onChange([...values, trimmed])
    setDraft("")
  }
  return (
    <div className="flex flex-col gap-1.5" data-testid={testId}>
      <Label>{label}</Label>
      <div className="flex flex-wrap items-center gap-1.5 rounded-md border border-input px-2 py-1.5">
        {values.map((v) => (
          <Badge
            key={v}
            variant="secondary"
            className="gap-1 h-5 px-1.5 text-xs"
            data-testid={`${testId}-chip`}
          >
            {v}
            <button
              type="button"
              className="ml-0.5 inline-flex items-center"
              aria-label={`remove ${v}`}
              onClick={() => onChange(values.filter((x) => x !== v))}
            >
              <X className="size-3" aria-hidden="true" />
            </button>
          </Badge>
        ))}
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === ",") {
              e.preventDefault()
              commit()
            } else if (e.key === "Backspace" && draft === "" && values.length > 0) {
              onChange(values.slice(0, -1))
            }
          }}
          onBlur={commit}
          placeholder={values.length === 0 ? placeholder : undefined}
          className="flex-1 min-w-24 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          data-testid={`${testId}-input`}
        />
        {draft.trim() ? (
          <button
            type="button"
            className="text-muted-foreground hover:text-foreground"
            aria-label="add"
            onClick={commit}
          >
            <Plus className="size-3.5" aria-hidden="true" />
          </button>
        ) : null}
      </div>
      {helper ? <p className="text-xs text-muted-foreground">{helper}</p> : null}
    </div>
  )
}

interface UrlListProps {
  label: string
  helper?: string
  addLabel: string
  placeholder?: string
  values: string[]
  onChange: (next: string[]) => void
  testId?: string
  // Server-side per-row validation messages keyed by row index. When
  // the BE rejects `urls.0` we render the message right below that
  // input so the user doesn't have to count rows in a banner.
  rowErrors?: Array<string | undefined>
  // Fired after each row's input loses focus and any auto-https
  // promotion has been committed. The parent uses it to re-trigger
  // form-wide validation so the in-place "valid URL?" hint shows up
  // before the user reaches Submit.
  onRowBlur?: (idx: number) => void
}

// UrlList — `Label` header with an inline "+ Add" affordance on the
// right; helper text under the header but only while empty (the rows
// themselves carry the affordance once present); each row is one
// full-width URL input + a trailing remove button.
//
// Mirrors `design-mocks/src/components/AddItemDialog.tsx` Product URLs
// section (L1309-L1339) one-for-one — minus the Label sub-input. The
// mock pairs each URL with a free-form label string, but the BE model
// (`go/models/url.go`: `type URL net/url.URL`) only stores raw URLs;
// adding labels is BE-blocked and tracked separately.
function UrlList({
  label,
  helper,
  addLabel,
  placeholder,
  values,
  onChange,
  testId,
  rowErrors,
  onRowBlur,
}: UrlListProps) {
  const { t } = useTranslation()
  // Always render at least one input row so the user sees an input
  // ready to type into without first clicking "+ Add". The form state
  // stays empty (`values = []`) until they actually type, so we don't
  // submit a single empty string to the BE on no-op flows.
  const isPhantomFirstRow = values.length === 0
  const displayCount = Math.max(values.length, 1)
  // `leavingIdx` holds the row index currently fading out. The row
  // stays mounted with the `animate-out` class for `EXIT_MS`, then
  // the splice runs. Without this the row would unmount instantly
  // and skip its exit animation entirely.
  const EXIT_MS = 150
  const [leavingIdx, setLeavingIdx] = useState<number | null>(null)
  const exitTimerRef = useRef<number | undefined>(undefined)
  useEffect(() => {
    return () => {
      if (exitTimerRef.current !== undefined) window.clearTimeout(exitTimerRef.current)
    }
  }, [])
  function update(idx: number, next: string) {
    if (isPhantomFirstRow) {
      // First keystroke into the phantom row promotes it into real
      // form state.
      onChange([next])
      return
    }
    onChange(values.map((v, i) => (i === idx ? next : v)))
  }
  function remove(idx: number) {
    if (leavingIdx !== null) {
      // A previous remove is still animating; flush it immediately so
      // the user's rapid clicks don't pile up timers.
      if (exitTimerRef.current !== undefined) window.clearTimeout(exitTimerRef.current)
    }
    setLeavingIdx(idx)
    exitTimerRef.current = window.setTimeout(() => {
      onChange(values.filter((_, i) => i !== idx))
      setLeavingIdx(null)
      exitTimerRef.current = undefined
    }, EXIT_MS)
  }
  function add() {
    // The phantom first row exists only in the UI — `values` is still
    // `[]` at that point, so a naive `[...values, ""]` would yield
    // `[""]` (still one visible row) and the user would have to click
    // again. Promote the phantom into real state, then append.
    const promoted = values.length === 0 ? [""] : values
    onChange([...promoted, ""])
  }
  return (
    <div className="flex flex-col gap-1.5" data-testid={testId}>
      <Label className="flex items-center justify-between">
        {label}
        <button
          type="button"
          onClick={add}
          className="flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
          data-testid={testId ? `${testId}-add` : undefined}
        >
          <Plus className="size-3" aria-hidden="true" />
          {addLabel}
        </button>
      </Label>
      <div className="flex flex-col gap-2">
        {Array.from({ length: displayCount }).map((_, idx) => {
          const value = values[idx] ?? ""
          // The first row is always un-removable — clicking "+ Add"
          // promotes the list to two rows and only then does X appear
          // on every row. Removing back down to a single row makes
          // that lone row un-removable again.
          const showRemove = displayCount > 1
          const isLeaving = leavingIdx === idx
          const rowError = rowErrors?.[idx]
          return (
            <div
              key={idx}
              className={cn(
                "flex flex-col gap-1",
                // Animate only the rows added beyond the always-on
                // first row, so the initial render doesn't fade in
                // every time the form mounts.
                idx > 0 && !isLeaving && "animate-in fade-in slide-in-from-top-1 duration-150",
                isLeaving &&
                  "animate-out fade-out slide-out-to-top-1 duration-150 fill-mode-forwards"
              )}
            >
              <div className="flex items-center">
                <Input
                  value={value}
                  type="url"
                  placeholder={placeholder}
                  className={cn(
                    "flex-1 text-sm",
                    rowError && "border-destructive focus-visible:ring-destructive/20"
                  )}
                  aria-invalid={!!rowError}
                  onChange={(e) => update(idx, e.target.value)}
                  onBlur={(e) => {
                    // Auto-prepend `https://` when the user typed a
                    // bare host (no scheme) — saves them remembering
                    // the prefix for every link they paste. Empty
                    // values stay empty (filtered out at submit).
                    const raw = e.target.value
                    const trimmed = raw.trim()
                    if (trimmed === "") {
                      if (raw !== "") update(idx, "")
                    } else if (!/:\/\//.test(trimmed)) {
                      update(idx, `https://${trimmed}`)
                    } else if (raw !== trimmed) {
                      update(idx, trimmed)
                    }
                    onRowBlur?.(idx)
                  }}
                  data-testid={testId ? `${testId}-row-${idx}` : undefined}
                />
                {/* X-button wrapper is always rendered. We animate ITS
                    explicit width + margin-left from 0 → (16px + 8px
                    gap) when the row becomes removable; the flex-1
                    Input next to it reflows continuously per frame as
                    the sibling width interpolates, so the layout
                    reshuffle reads as a smooth slide instead of a
                    snap. `flex-1` itself can't be transitioned, but
                    a sibling's animated width drags the flex-1 width
                    along with it. */}
                <div
                  className={cn(
                    "flex shrink-0 items-center overflow-hidden transition-[width,margin-left,opacity] duration-150 ease-out",
                    showRemove ? "ml-2 w-4 opacity-100" : "ml-0 w-0 opacity-0"
                  )}
                  aria-hidden={!showRemove}
                >
                  <button
                    type="button"
                    aria-label={t("common:actions.delete")}
                    tabIndex={showRemove ? 0 : -1}
                    onClick={() => remove(idx)}
                    className="text-muted-foreground transition-colors hover:text-foreground"
                  >
                    <X className="size-4" aria-hidden="true" />
                  </button>
                </div>
              </div>
              {rowError ? (
                <p className="text-xs text-destructive" role="alert">
                  {rowError}
                </p>
              ) : null}
            </div>
          )
        })}
      </div>
      {helper ? <p className="text-xs text-muted-foreground">{helper}</p> : null}
    </div>
  )
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- RHF FieldError generics noise
function FieldError({ error }: { error: any }) {
  const { t } = useTranslation()
  if (!error?.message) return null
  return (
    <p className="text-xs text-destructive" role="alert">
      {t(error.message)}
    </p>
  )
}
