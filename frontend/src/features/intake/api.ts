// Data-layer functions for batch intake (#2810). Hooks live in `./hooks.ts`.
//
// An intake session is a batch of dropped files. Each source is one purchase
// (one file, or several files of one item); the server scans each source in
// the background and lists the products it detected as candidates. The member
// keeps some of them and asks for drafts: exactly one draft per kept product,
// so repeating the request after a lost response makes no second draft.
import { groupScopedPath, http } from "@/lib/http"
import { getServerErrorCode } from "@/lib/server-error"
import type { Schema } from "@/types"

type LimitsEnvelope = Schema<"jsonapi.IntakeLimitsResponse">
type SessionEnvelope = Schema<"jsonapi.IntakeSessionResponse">
type SessionsEnvelope = Schema<"jsonapi.IntakeSessionsResponse">
type CandidateEnvelope = Schema<"jsonapi.IntakeCandidateResponse">
type SessionView = Schema<"jsonapi.IntakeSessionView">
type SourceView = Schema<"jsonapi.IntakeSourceView">
type SourceFileView = Schema<"jsonapi.IntakeSourceFile">
type CandidateView = Schema<"models.IntakeCandidate">
type SummaryView = Schema<"models.IntakeSessionSummary">

export interface IntakeLimits {
  // False when the server has no AI scan provider: nothing can be scanned.
  enabled: boolean
  maxFilesPerBatch: number
  maxFilesPerSource: number
  maxFileBytes: number
  maxQueuedPerGroup: number
  // Sources of the group that wait for or are in a scan right now.
  queued: number
  ratePerHour: number
  acceptedMimeTypes: string[]
}

export type IntakeSessionState = "processing" | "review" | "done"
export type IntakeSourceStatus = "pending" | "scanning" | "scanned" | "failed" | "canceled"
export type IntakeCandidateOutcome = "pending" | "created" | "skipped" | "failed"

export interface IntakeSummary {
  sources: number
  files: number
  pending: number
  rateLimited: number
  scanning: number
  scanned: number
  failed: number
  canceled: number
  candidates: number
  selected: number
  undecided: number
  unfinished: number
  draftsCreated: number
  skipped: number
  candidatesFailed: number
}

export interface IntakeSession {
  id: string
  state: IntakeSessionState
  origin: string
  summary: IntakeSummary
  createdAt: string
  updatedAt: string
}

export interface IntakeSourceFile {
  id: string
  title: string
  mimeType: string
  sizeBytes: number
  url?: string
  thumbnail?: string
}

export interface IntakeSource {
  id: string
  status: IntakeSourceStatus
  oneItem: boolean
  attempts: number
  waitReason: string
  errorCode: string
  sortOrder: number
  files: IntakeSourceFile[]
}

export interface IntakeFieldGuess {
  value: unknown
  confidence: number
}

export interface IntakeCandidate {
  id: string
  sourceId: string
  sortOrder: number
  fields: Record<string, IntakeFieldGuess>
  acceptedFields: string[]
  quantity: number
  selected: boolean
  outcome: IntakeCandidateOutcome
  draftId: string
  errorCode: string
}

export interface IntakeSessionDetail extends IntakeSession {
  sources: IntakeSource[]
  candidates: IntakeCandidate[]
}

export interface IntakeDraftsResult {
  session: IntakeSessionDetail
  created: number
  skipped: number
  failed: number
}

export interface IntakeSessionsPage {
  sessions: IntakeSession[]
  total: number
}

export interface IntakeListOptions {
  page?: number
  perPage?: number
}

// The fields a member can accept or leave out on a detected product, in the
// order the review lists them. The server refuses any other name.
export const INTAKE_REVIEW_FIELDS = [
  "name",
  "short_name",
  "brand",
  "model",
  "type",
  "serial_number",
  "purchase_date",
  "seller",
  "warranty_expires_at",
  "original_price",
  "original_price_currency",
  "urls",
  "tags",
  "comments",
] as const

export type IntakeReviewField = (typeof INTAKE_REVIEW_FIELDS)[number]

export const INTAKE_MAX_QUANTITY = 100000

export const INTAKE_QUEUE_FULL = "intake.queue_full"
export const INTAKE_SOURCE_BUSY = "intake.source_busy"
export const INTAKE_SOURCE_HAS_DRAFTS = "intake.source_has_drafts"
export const INTAKE_CANDIDATE_FINISHED = "intake.candidate_finished"

// toIntakeSummary reads the counts of a batch; the inbox uses it too.
export function toIntakeSummary(s: SummaryView | undefined): IntakeSummary {
  return {
    sources: s?.sources ?? 0,
    files: s?.files ?? 0,
    pending: s?.pending ?? 0,
    rateLimited: s?.rate_limited ?? 0,
    scanning: s?.scanning ?? 0,
    scanned: s?.scanned ?? 0,
    failed: s?.failed ?? 0,
    canceled: s?.canceled ?? 0,
    candidates: s?.candidates ?? 0,
    selected: s?.selected ?? 0,
    undecided: s?.undecided ?? 0,
    unfinished: s?.unfinished ?? 0,
    draftsCreated: s?.drafts_created ?? 0,
    skipped: s?.skipped ?? 0,
    candidatesFailed: s?.candidates_failed ?? 0,
  }
}

export function toIntakeState(state: string | undefined): IntakeSessionState {
  if (state === "processing" || state === "review") return state
  return "done"
}

function toSourceStatus(status: string | undefined): IntakeSourceStatus {
  switch (status) {
    case "pending":
    case "scanning":
    case "scanned":
    case "failed":
      return status
    default:
      return "canceled"
  }
}

function toOutcome(outcome: string | undefined): IntakeCandidateOutcome {
  switch (outcome) {
    case "created":
    case "skipped":
    case "failed":
      return outcome
    default:
      return "pending"
  }
}

function toSession(id: string | undefined, view: SessionView | undefined): IntakeSession {
  return {
    id: id ?? view?.id ?? "",
    state: toIntakeState(view?.state),
    origin: view?.origin ?? "upload",
    summary: toIntakeSummary(view?.summary),
    createdAt: view?.created_at ?? "",
    updatedAt: view?.updated_at ?? "",
  }
}

function toFile(view: SourceFileView): IntakeSourceFile {
  const thumbs = view.thumbnails ?? {}
  return {
    id: view.id ?? "",
    title: view.title || view.path || "",
    mimeType: view.mime_type ?? "",
    sizeBytes: view.size_bytes ?? 0,
    url: view.inline_url || view.url || undefined,
    thumbnail: thumbs.small ?? thumbs.medium ?? undefined,
  }
}

function toSource(view: SourceView): IntakeSource {
  return {
    id: view.id ?? "",
    status: toSourceStatus(view.status),
    oneItem: view.one_item ?? false,
    attempts: view.attempts ?? 0,
    waitReason: view.wait_reason ?? "",
    errorCode: view.error_code ?? "",
    sortOrder: view.sort_order ?? 0,
    files: (view.files ?? []).map(toFile),
  }
}

function toCandidate(view: CandidateView | undefined, id?: string): IntakeCandidate {
  const fields: Record<string, IntakeFieldGuess> = {}
  for (const [key, guess] of Object.entries(view?.fields ?? {})) {
    fields[key] = { value: guess?.value, confidence: guess?.confidence ?? 0 }
  }
  return {
    id: id ?? view?.id ?? "",
    sourceId: view?.source_id ?? "",
    sortOrder: view?.sort_order ?? 0,
    fields,
    acceptedFields: view?.accepted_fields ?? [],
    quantity: view?.quantity ?? 1,
    selected: view?.selected ?? false,
    outcome: toOutcome(view?.outcome),
    draftId: view?.draft_id ?? "",
    errorCode: view?.error_code ?? "",
  }
}

function toDetail(body: SessionEnvelope): IntakeSessionDetail {
  const attrs = body.data?.attributes
  return {
    ...toSession(body.data?.id, attrs),
    sources: (attrs?.sources ?? []).map(toSource).sort((a, b) => a.sortOrder - b.sortOrder),
    candidates: (attrs?.candidates ?? []).map((c) => toCandidate(c)),
  }
}

export async function getIntakeLimits(signal?: AbortSignal): Promise<IntakeLimits> {
  const body = await http.get<LimitsEnvelope>("/intake/limits", { signal })
  const a = body.data?.attributes ?? {}
  return {
    enabled: a.enabled ?? false,
    maxFilesPerBatch: a.max_files_per_batch ?? 0,
    maxFilesPerSource: a.max_files_per_source ?? 0,
    maxFileBytes: a.max_file_bytes ?? 0,
    maxQueuedPerGroup: a.max_queued_per_group ?? 0,
    queued: a.queued ?? 0,
    ratePerHour: a.rate_limit_per_hour ?? 0,
    acceptedMimeTypes: a.accepted_mime_types ?? [],
  }
}

export async function listIntakeSessions(
  opts: IntakeListOptions = {},
  signal?: AbortSignal
): Promise<IntakeSessionsPage> {
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  const qs = params.toString()
  const body = await http.get<SessionsEnvelope>(
    qs ? `/intake/sessions?${qs}` : "/intake/sessions",
    { signal }
  )
  return {
    sessions: (body.data ?? []).map((view) => toSession(view.id, view)),
    total: body.meta?.sessions ?? 0,
  }
}

export async function getIntakeSession(
  id: string,
  signal?: AbortSignal
): Promise<IntakeSessionDetail> {
  const body = await http.get<SessionEnvelope>(`/intake/sessions/${encodeURIComponent(id)}`, {
    signal,
  })
  return toDetail(body)
}

export interface IntakeSourceInput {
  fileIds: string[]
  oneItem: boolean
}

export interface IntakeUploadTarget {
  // Where the batch goes, read once when the batch starts, so a group switch
  // during the uploads does not send the later requests to another group.
  uploadPath: string
  sessionsPath: string
}

export function intakeUploadTarget(): IntakeUploadTarget {
  return {
    uploadPath: groupScopedPath("/uploads/file"),
    sessionsPath: groupScopedPath("/intake/sessions"),
  }
}

interface UploadEnvelope {
  id?: string
  attributes?: unknown
}

// uploadIntakeFile uploads one dropped file as a standalone file and returns
// its id. The session links it once it is created.
export async function uploadIntakeFile(target: IntakeUploadTarget, file: File): Promise<string> {
  const form = new FormData()
  form.append("file", file)
  const body = await http.post<UploadEnvelope>(target.uploadPath, form, {
    skipGroupRewrite: true,
  })
  if (!body.id) throw new Error("Malformed /uploads/file response: missing id")
  return body.id
}

// createIntakeSession queues the batch. A client key the group already has
// returns that session unchanged, so repeating the call is safe.
export async function createIntakeSession(
  target: IntakeUploadTarget,
  clientKey: string,
  sources: IntakeSourceInput[]
): Promise<IntakeSessionDetail> {
  const body = await http.post<SessionEnvelope>(
    target.sessionsPath,
    {
      data: {
        type: "intake_sessions",
        attributes: {
          client_key: clientKey,
          sources: sources.map((s) => ({ file_ids: s.fileIds, one_item: s.oneItem })),
        },
      },
    },
    { skipGroupRewrite: true }
  )
  return toDetail(body)
}

export interface IntakeCandidateReview {
  selected?: boolean
  acceptedFields?: string[]
  quantity?: number
}

export async function reviewIntakeCandidate(
  sessionId: string,
  candidateId: string,
  review: IntakeCandidateReview
): Promise<IntakeCandidate> {
  const body = await http.patch<CandidateEnvelope>(
    `/intake/sessions/${encodeURIComponent(sessionId)}/candidates/${encodeURIComponent(candidateId)}`,
    {
      data: {
        type: "intake_candidates",
        attributes: {
          selected: review.selected,
          accepted_fields: review.acceptedFields,
          quantity: review.quantity,
        },
      },
    }
  )
  return toCandidate(body.data?.attributes, body.data?.id)
}

// createIntakeDrafts makes the drafts of the kept products that have none
// and marks the others skipped. Products that already have their draft keep
// it, so the call can be repeated.
export async function createIntakeDrafts(sessionId: string): Promise<IntakeDraftsResult> {
  const body = await http.post<SessionEnvelope>(
    `/intake/sessions/${encodeURIComponent(sessionId)}/drafts`
  )
  return {
    session: toDetail(body),
    created: body.meta?.created ?? 0,
    skipped: body.meta?.skipped ?? 0,
    failed: body.meta?.failed ?? 0,
  }
}

export async function cancelIntakeSession(sessionId: string): Promise<IntakeSessionDetail> {
  const body = await http.post<SessionEnvelope>(
    `/intake/sessions/${encodeURIComponent(sessionId)}/cancel`
  )
  return toDetail(body)
}

export async function resumeIntakeSession(sessionId: string): Promise<IntakeSessionDetail> {
  const body = await http.post<SessionEnvelope>(
    `/intake/sessions/${encodeURIComponent(sessionId)}/resume`
  )
  return toDetail(body)
}

export async function rescanIntakeSource(
  sessionId: string,
  sourceId: string
): Promise<IntakeSessionDetail> {
  const body = await http.post<SessionEnvelope>(
    `/intake/sessions/${encodeURIComponent(sessionId)}/sources/${encodeURIComponent(sourceId)}/rescan`
  )
  return toDetail(body)
}

export async function deleteIntakeSession(sessionId: string): Promise<void> {
  await http.del(`/intake/sessions/${encodeURIComponent(sessionId)}`)
}

export function isIntakeError(err: unknown, code: string): boolean {
  return getServerErrorCode(err) === code
}

// candidateText reads a text field of a detected product, or "".
export function candidateText(c: IntakeCandidate, field: string): string {
  const v = c.fields[field]?.value
  return typeof v === "string" ? v.trim() : ""
}

// candidatePrice reads the line amount of a detected product with its
// currency, or null when the scan read no number.
export function candidatePrice(c: IntakeCandidate): { amount: number; currency: string } | null {
  const v = c.fields.original_price?.value
  const amount = typeof v === "number" ? v : typeof v === "string" ? Number(v) : NaN
  if (!Number.isFinite(amount)) return null
  return { amount, currency: candidateText(c, "original_price_currency").toUpperCase() }
}
