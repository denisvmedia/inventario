// Data-layer functions for server drafts (#2812). Hooks live in `./hooks.ts`.
//
// A draft is an unfinished item saved on the server under the group. Its
// values are the item form as entered, stored without validation; its files
// are ordinary files attached with entity type "draft". Promoting a draft
// creates the item and moves the files to it.
import { http } from "@/lib/http"
import { getServerErrorCode, getServerErrorMeta } from "@/lib/server-error"
import { listFiles, type ListedFile } from "@/features/files/api"
import type { Commodity, CreateCommodityRequest } from "@/features/commodities/api"
import type { CommodityFormInput } from "@/features/commodities/schemas"
import type { Schema } from "@/types"

type DraftView = Schema<"jsonapi.CommodityDraftView">
type DraftEnvelope = Schema<"jsonapi.CommodityDraftResponse">
type DraftsEnvelope = Schema<"jsonapi.CommodityDraftsResponse">

// The form values a draft carries. Anything the form had at save time;
// a draft saved by another client may hold fewer keys.
export type DraftValues = Partial<CommodityFormInput>

export interface Draft {
  id: string
  clientKey: string
  title: string
  values: DraftValues
  revision: number
  status: "open" | "promoted"
  promotedCommodityId?: string
  fileCount: number
  updatedAt: string
}

export interface DraftsPage {
  drafts: Draft[]
  total: number
}

export interface DraftListOptions {
  page?: number
  perPage?: number
}

export const DRAFT_REVISION_CONFLICT = "draft.revision_conflict"
export const DRAFT_PROMOTED = "draft.promoted"

function toDraft(id: string | undefined, view: DraftView | undefined): Draft {
  return {
    id: id ?? view?.id ?? "",
    clientKey: view?.client_key ?? "",
    title: view?.title ?? "",
    values: (view?.values ?? {}) as DraftValues,
    revision: view?.revision ?? 0,
    status: view?.status === "promoted" ? "promoted" : "open",
    promotedCommodityId: view?.promoted_commodity_id || undefined,
    fileCount: view?.file_count ?? 0,
    updatedAt: view?.updated_at ?? "",
  }
}

function fromEnvelope(body: DraftEnvelope): Draft {
  return toDraft(body.data?.id, body.data?.attributes)
}

export async function listDrafts(
  opts: DraftListOptions = {},
  signal?: AbortSignal
): Promise<DraftsPage> {
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  const qs = params.toString()
  const body = await http.get<DraftsEnvelope>(qs ? `/drafts?${qs}` : "/drafts", { signal })
  return {
    drafts: (body.data ?? []).map((view) => toDraft(view.id, view)),
    total: body.meta?.drafts ?? 0,
  }
}

export async function getDraft(id: string, signal?: AbortSignal): Promise<Draft> {
  const body = await http.get<DraftEnvelope>(`/drafts/${encodeURIComponent(id)}`, { signal })
  return fromEnvelope(body)
}

// createDraft stores a new draft. A key the group already has returns that
// draft unchanged, so repeating the call after a lost response is safe.
export async function createDraft(clientKey: string, values: DraftValues): Promise<Draft> {
  const body = await http.post<DraftEnvelope>("/drafts", {
    data: { type: "commodity_drafts", attributes: { client_key: clientKey, values } },
  })
  return fromEnvelope(body)
}

// saveDraft replaces the values. revision is the one the values were based
// on; a stale one fails with DRAFT_REVISION_CONFLICT.
export async function saveDraft(id: string, revision: number, values: DraftValues): Promise<Draft> {
  const body = await http.put<DraftEnvelope>(`/drafts/${encodeURIComponent(id)}`, {
    data: { type: "commodity_drafts", attributes: { revision, values } },
  })
  return fromEnvelope(body)
}

export async function discardDraft(id: string): Promise<void> {
  await http.del(`/drafts/${encodeURIComponent(id)}`)
}

type CommodityEnvelope = Schema<"jsonapi.CommodityResponse">

// promoteDraft creates the item from the finished form and moves the
// draft's files to it. Repeating it returns the same item.
export async function promoteDraft(
  id: string,
  revision: number,
  values: CreateCommodityRequest
): Promise<Commodity & { id: string }> {
  const body = await http.post<CommodityEnvelope>(`/drafts/${encodeURIComponent(id)}/promote`, {
    data: { type: "commodities", attributes: values },
    meta: { revision },
  })
  if (!body.data?.id || !body.data.attributes) {
    throw new Error(`Malformed POST /drafts/${id}/promote response: missing id or attributes`)
  }
  return { ...body.data.attributes, id: body.data.id }
}

// listDraftFiles returns the live files attached to the draft.
export async function listDraftFiles(id: string, signal?: AbortSignal): Promise<ListedFile[]> {
  const { files } = await listFiles({
    linkedEntityType: "draft",
    linkedEntityId: id,
    perPage: 100,
    signal,
  })
  return files
}

// isRevisionConflict reports a save refused because the draft changed
// elsewhere, with the revision the server has now.
export function revisionConflict(err: unknown): { revision: number } | null {
  if (getServerErrorCode(err) !== DRAFT_REVISION_CONFLICT) return null
  const raw = getServerErrorMeta(err)?.revision
  return { revision: raw === undefined ? 0 : Number(raw) }
}

// promotedConflict reports a draft that already became an item.
export function promotedConflict(err: unknown): { commodityId?: string } | null {
  if (getServerErrorCode(err) !== DRAFT_PROMOTED) return null
  return { commodityId: getServerErrorMeta(err)?.commodity_id || undefined }
}
