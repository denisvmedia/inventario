// Data-layer functions for the inbox (#2794). Hooks live in `./hooks.ts`.
//
// The inbox lists what in the group still needs attention: one entry per
// record, with every reason it is there for. The server merges the sources
// and counts each record once, so the counter never comes from the length
// of a page.
import { setCommodityDraftMark, type Commodity } from "@/features/commodities/api"
import {
  toIntakeState,
  toIntakeSummary,
  type IntakeSessionState,
  type IntakeSummary,
} from "@/features/intake/api"
import { toDocumentSource, type DocumentSource } from "@/features/intake/documents"
import { http } from "@/lib/http"
import type { Schema } from "@/types"

type InboxEnvelope = Schema<"jsonapi.InboxResponse">
type InboxCountEnvelope = Schema<"jsonapi.InboxCountResponse">
type InboxCountsView = Schema<"jsonapi.InboxCounts">
type InboxChecksEnvelope = Schema<"jsonapi.InboxChecksResponse">

// The reasons this client knows how to show and act on, in display order.
// An entry can carry a reason a newer server added; the page shows it
// without an action.
export const INBOX_REASONS = ["draft", "item_draft", "intake"] as const
export type InboxReason = (typeof INBOX_REASONS)[number]

export function isInboxReason(value: string): value is InboxReason {
  return (INBOX_REASONS as readonly string[]).includes(value)
}

export interface InboxEntry {
  // "draft" for a server draft of a new item, "commodity" for an item,
  // "intake_session" for a batch of files (#2810). A newer server may send
  // another kind.
  kind: string
  id: string
  title: string
  reasons: string[]
  // When the record was last saved; for a batch, when it was added.
  updatedAt: string | null
  fileCount: number
  // The progress of a batch of files; null for other kinds.
  intake: { state: IntakeSessionState; summary: IntakeSummary } | null
  // The other system a draft came from (#2809); null for a draft made in
  // the app and for other kinds.
  source: DocumentSource | null
}

export interface InboxCounts {
  // Each record once, whatever its number of reasons.
  total: number
  // Per reason, the records that have it.
  reasons: Record<string, number>
}

export interface InboxPage {
  entries: InboxEntry[]
  // The entries the reason filter keeps.
  total: number
  // The whole inbox, without the reason filter.
  counts: InboxCounts
}

export interface InboxListOptions {
  reason?: InboxReason
  page?: number
  perPage?: number
}

function toCounts(view: InboxCountsView | undefined): InboxCounts {
  return { total: view?.total ?? 0, reasons: { ...(view?.reasons ?? {}) } }
}

export async function listInbox(
  opts: InboxListOptions = {},
  signal?: AbortSignal
): Promise<InboxPage> {
  const params = new URLSearchParams()
  if (opts.reason) params.set("reason", opts.reason)
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  const qs = params.toString()
  const body = await http.get<InboxEnvelope>(qs ? `/inbox?${qs}` : "/inbox", { signal })
  return {
    entries: (body.data ?? []).map((e) => ({
      kind: e.kind ?? "",
      id: e.id ?? "",
      title: e.title ?? "",
      reasons: [...(e.reasons ?? [])],
      updatedAt: e.updated_at ?? null,
      fileCount: e.file_count ?? 0,
      intake: e.intake
        ? { state: toIntakeState(e.intake.state), summary: toIntakeSummary(e.intake.summary) }
        : null,
      source: e.source ? toDocumentSource(e.source.name, null, e.source.url) : null,
    })),
    total: body.meta?.entries ?? 0,
    counts: toCounts(body.meta?.counts),
  }
}

export async function getInboxCount(signal?: AbortSignal): Promise<InboxCounts> {
  const body = await http.get<InboxCountEnvelope>("/inbox/count", { signal })
  return toCounts(body.data)
}

// An optional quality check next to the inbox: the items that lack one
// piece of information. It is not part of the inbox or its counts. query is
// the check's item query in the API's parameter names; count is the total
// the items list reports for it.
export interface InboxCheck {
  key: string
  query: string
  count: number
}

export async function getInboxChecks(signal?: AbortSignal): Promise<InboxCheck[]> {
  const body = await http.get<InboxChecksEnvelope>("/inbox/checks", { signal })
  return (body.data ?? []).map((c) => ({
    key: c.key ?? "",
    query: c.query ?? "",
    count: c.count ?? 0,
  }))
}

// setCommodityDraft marks an item as draft or clears the mark. The server
// changes the mark alone, so an edit of the item saved at the same time
// stays.
export function setCommodityDraft(id: string, draft: boolean): Promise<Commodity> {
  return setCommodityDraftMark(id, draft)
}
