// Data-layer functions for server drafts (#2812). Hooks live in `./hooks.ts`.
//
// A draft is an unfinished item saved on the server under the group. Its
// values are the item form as entered, stored without validation; its files
// are ordinary files attached with entity type "draft". Promoting a draft
// creates the item and moves the files to it.
import { http, type InGroup } from "@/lib/http"
import { getServerErrorCode, getServerErrorMeta } from "@/lib/server-error"
import { listFiles, type ListedFile } from "@/features/files/api"
import type { Commodity, CreateCommodityRequest } from "@/features/commodities/api"
import type { CommodityFormInput } from "@/features/commodities/schemas"
import type { Schema } from "@/types"

import { parseDraftValues } from "./values"

type DraftView = Schema<"jsonapi.CommodityDraftView">
type DraftEnvelope = Schema<"jsonapi.CommodityDraftResponse">

// The form values a draft carries. Anything the form had at save time;
// a draft saved by another client may hold fewer keys.
export type DraftValues = Partial<CommodityFormInput>

export interface Draft {
  id: string
  clientKey: string
  title: string
  values: DraftValues
  revision: number
  // "closed" is any other state: a draft being discarded, or a state a newer
  // server adds. The form never saves to or promotes such a draft.
  status: "open" | "promoted" | "closed"
  promotedCommodityId?: string
  fileCount: number
  updatedAt: string
}

export const DRAFT_REVISION_CONFLICT = "draft.revision_conflict"
export const DRAFT_PROMOTED = "draft.promoted"
export const DRAFT_DISCARDED = "draft.discarded"

// DraftStateError is a draft refusal the client found itself rather than
// read from an error response: a retried create that returns a draft
// another device has saved or finished since. The helpers below treat it
// like the server's coded 409.
export class DraftStateError extends Error {
  readonly code: string
  readonly meta: Record<string, string>
  constructor(code: string, meta: Record<string, string> = {}) {
    super(code)
    this.name = "DraftStateError"
    this.code = code
    this.meta = meta
  }
}

function codeOf(err: unknown): string | null {
  if (err instanceof DraftStateError) return err.code
  return getServerErrorCode(err)
}

function metaOf(err: unknown): Record<string, string> | null {
  if (err instanceof DraftStateError) return err.meta
  return getServerErrorMeta(err)
}

function toStatus(status: string | undefined): Draft["status"] {
  if (status === "open" || status === "promoted") return status
  return "closed"
}

function toDraft(id: string | undefined, view: DraftView | undefined): Draft {
  return {
    id: id ?? view?.id ?? "",
    clientKey: view?.client_key ?? "",
    title: view?.title ?? "",
    // Read leniently: a value the form cannot use falls back to the
    // form's default instead of breaking the dialog.
    values: parseDraftValues(view?.values),
    revision: view?.revision ?? 0,
    status: toStatus(view?.status),
    promotedCommodityId: view?.promoted_commodity_id || undefined,
    fileCount: view?.file_count ?? 0,
    updatedAt: view?.updated_at ?? "",
  }
}

function fromEnvelope(body: DraftEnvelope): Draft {
  return toDraft(body.data?.id, body.data?.attributes)
}

// The options of a read: its abort signal, and the group to read from.
export type ReadOptions = { signal?: AbortSignal } & InGroup

export async function getDraft(id: string, opts: ReadOptions = {}): Promise<Draft> {
  const body = await http.get<DraftEnvelope>(`/drafts/${encodeURIComponent(id)}`, opts)
  return fromEnvelope(body)
}

// createDraft stores a new draft. A key the group already has returns that
// draft unchanged, so repeating the call after a lost response is safe.
export async function createDraft(
  clientKey: string,
  values: DraftValues,
  opts: InGroup = {}
): Promise<Draft> {
  const body = await http.post<DraftEnvelope>(
    "/drafts",
    { data: { type: "commodity_drafts", attributes: { client_key: clientKey, values } } },
    { group: opts.group }
  )
  return fromEnvelope(body)
}

// saveDraft replaces the values. revision is the one the values were based
// on; a stale one fails with DRAFT_REVISION_CONFLICT.
export async function saveDraft(
  id: string,
  revision: number,
  values: DraftValues,
  opts: InGroup = {}
): Promise<Draft> {
  const body = await http.put<DraftEnvelope>(
    `/drafts/${encodeURIComponent(id)}`,
    { data: { type: "commodity_drafts", attributes: { revision, values } } },
    { group: opts.group }
  )
  return fromEnvelope(body)
}

export async function discardDraft(id: string, opts: InGroup = {}): Promise<void> {
  await http.del(`/drafts/${encodeURIComponent(id)}`, { group: opts.group })
}

type CommodityEnvelope = Schema<"jsonapi.CommodityResponse">

// promoteDraft creates the item from the finished form and moves the
// draft's files to it. promotionKey identifies this promotion: repeating it
// with the same key returns the same item, while another key gets
// DRAFT_PROMOTED, so a second device cannot land on the first one's item.
export async function promoteDraft(
  id: string,
  revision: number,
  promotionKey: string,
  values: CreateCommodityRequest,
  opts: InGroup = {}
): Promise<Commodity & { id: string }> {
  const body = await http.post<CommodityEnvelope>(
    `/drafts/${encodeURIComponent(id)}/promote`,
    {
      data: { type: "commodities", attributes: values },
      meta: { revision, promotion_key: promotionKey },
    },
    { group: opts.group }
  )
  if (!body.data?.id || !body.data.attributes) {
    throw new Error(`Malformed POST /drafts/${id}/promote response: missing id or attributes`)
  }
  return { ...body.data.attributes, id: body.data.id }
}

// listDraftFiles returns the live files attached to the draft.
export async function listDraftFiles(id: string, opts: ReadOptions = {}): Promise<ListedFile[]> {
  const { files } = await listFiles({
    linkedEntityType: "draft",
    linkedEntityId: id,
    perPage: 100,
    signal: opts.signal,
    group: opts.group,
  })
  return files
}

// isRevisionConflict reports a save refused because the draft changed
// elsewhere, with the revision the server has now.
export function revisionConflict(err: unknown): { revision: number } | null {
  if (codeOf(err) !== DRAFT_REVISION_CONFLICT) return null
  const raw = metaOf(err)?.revision
  return { revision: raw === undefined ? 0 : Number(raw) }
}

// promotedConflict reports a draft that already became an item, with the
// item when the server names it.
export function promotedConflict(err: unknown): { commodityId?: string } | null {
  if (codeOf(err) !== DRAFT_PROMOTED) return null
  return { commodityId: metaOf(err)?.commodity_id || undefined }
}

// discardedConflict reports a draft that is being or has been discarded
// elsewhere.
export function discardedConflict(err: unknown): boolean {
  return codeOf(err) === DRAFT_DISCARDED
}
