// Data layer of the saved views slice. A saved view keeps an item query
// (see features/commodities/query) under a name; the server stores it in
// the API form of the contract and validates it on every write. Hooks live
// in ./hooks.ts.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

type Attributes = Schema<"jsonapi.SavedViewAttributes">

export type SavedViewLayout = "" | "grid" | "compact" | "list"

export interface SavedViewQueryError {
  parameter: string
  reason: string
}

export interface SavedView {
  id: string
  name: string
  icon: string
  // The API query string of the item query, without the leading "?".
  query: string
  queryVersion: number
  // grid or list; empty keeps the viewer's own default.
  layout: SavedViewLayout
  shared: boolean
  showInSidebar: boolean
  showOnDashboard: boolean
  sortOrder: number
  owned: boolean
  canEdit: boolean
  canDelete: boolean
  // Set when the stored query cannot run. Such a view has no count and is
  // not opened.
  queryError?: SavedViewQueryError
}

export interface SavedViewLimits {
  maxNameLength: number
  maxPerUser: number
  maxShared: number
  canShare: boolean
}

export interface SavedViewList {
  views: SavedView[]
  limits: SavedViewLimits
}

interface ListEnvelope {
  data?: Schema<"jsonapi.SavedViewData">[]
  meta?: Schema<"jsonapi.SavedViewsMeta">
}

interface DetailEnvelope {
  data?: Schema<"jsonapi.SavedViewData">
}

interface CountsEnvelope {
  data?: Schema<"jsonapi.SavedViewCounts">
}

// The limits the server keeps when an older server leaves them out.
const DEFAULT_LIMITS: SavedViewLimits = {
  maxNameLength: 60,
  maxPerUser: 50,
  maxShared: 50,
  canShare: false,
}

function toSavedView(data: Schema<"jsonapi.SavedViewData"> | undefined): SavedView {
  const a: Attributes = data?.attributes ?? {}
  return {
    id: data?.id ?? "",
    name: a.name ?? "",
    icon: a.icon ?? "bookmark",
    query: a.query ?? "",
    queryVersion: a.query_version ?? 0,
    layout: a.layout ?? "",
    shared: a.shared ?? false,
    showInSidebar: a.show_in_sidebar ?? false,
    showOnDashboard: a.show_on_dashboard ?? false,
    sortOrder: a.sort_order ?? 0,
    owned: a.owned ?? false,
    canEdit: a.can_edit ?? false,
    canDelete: a.can_delete ?? false,
    queryError: a.query_error
      ? { parameter: a.query_error.parameter ?? "", reason: a.query_error.reason ?? "" }
      : undefined,
  }
}

export async function listSavedViews(signal?: AbortSignal): Promise<SavedViewList> {
  const body = await http.get<ListEnvelope>("/saved-views", { signal })
  const limits = body.meta?.limits
  return {
    views: (body.data ?? []).map(toSavedView),
    limits: {
      maxNameLength: limits?.max_name_length ?? DEFAULT_LIMITS.maxNameLength,
      maxPerUser: limits?.max_per_user ?? DEFAULT_LIMITS.maxPerUser,
      maxShared: limits?.max_shared ?? DEFAULT_LIMITS.maxShared,
      canShare: limits?.can_share ?? DEFAULT_LIMITS.canShare,
    },
  }
}

// The number of items each view selects, by view ID. The server counts
// through the path of the items list, so a count matches the opened list.
// A view whose query cannot run has no entry.
export async function countSavedViews(signal?: AbortSignal): Promise<Record<string, number>> {
  const body = await http.get<CountsEnvelope>("/saved-views/counts", { signal })
  return body.data?.counts ?? {}
}

export interface SavedViewInput {
  name: string
  icon: string
  query: string
  layout: SavedViewLayout
  shared: boolean
  showInSidebar: boolean
  showOnDashboard: boolean
}

export async function createSavedView(input: SavedViewInput): Promise<SavedView> {
  const body = await http.post<DetailEnvelope>("/saved-views", {
    data: {
      type: "saved_views",
      attributes: {
        name: input.name,
        icon: input.icon,
        query: input.query,
        layout: input.layout,
        shared: input.shared,
        show_in_sidebar: input.showInSidebar,
        show_on_dashboard: input.showOnDashboard,
      },
    },
  })
  return toSavedView(body.data)
}

export interface UpdateSavedViewRequest {
  id: string
  patch: Partial<SavedViewInput>
}

export async function updateSavedView({ id, patch }: UpdateSavedViewRequest): Promise<SavedView> {
  const attributes: Record<string, unknown> = {}
  if (patch.name !== undefined) attributes.name = patch.name
  if (patch.icon !== undefined) attributes.icon = patch.icon
  if (patch.query !== undefined) attributes.query = patch.query
  if (patch.layout !== undefined) attributes.layout = patch.layout
  if (patch.shared !== undefined) attributes.shared = patch.shared
  if (patch.showInSidebar !== undefined) attributes.show_in_sidebar = patch.showInSidebar
  if (patch.showOnDashboard !== undefined) attributes.show_on_dashboard = patch.showOnDashboard
  const body = await http.patch<DetailEnvelope>(`/saved-views/${encodeURIComponent(id)}`, {
    data: { id, type: "saved_views", attributes },
  })
  return toSavedView(body.data)
}

export async function deleteSavedView(id: string): Promise<void> {
  await http.del(`/saved-views/${encodeURIComponent(id)}`)
}

// Puts the listed views in the given order. They keep the positions they
// hold between them; views left out do not move.
export async function reorderSavedViews(ids: string[]): Promise<void> {
  await http.post("/saved-views/reorder", {
    data: { type: "saved_views_reorder", attributes: { ids } },
  })
}
