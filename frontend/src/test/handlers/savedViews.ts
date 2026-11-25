import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// A saved view as GET /g/{slug}/saved-views returns it (attributes of
// jsonapi.SavedViewAttributes plus the id).
export interface SavedViewFixture {
  id: string
  name: string
  icon?: string
  query: string
  query_version?: number
  layout?: "" | "grid" | "compact" | "list"
  shared?: boolean
  show_in_sidebar?: boolean
  show_on_dashboard?: boolean
  sort_order?: number
  owned?: boolean
  can_edit?: boolean
  can_delete?: boolean
  query_error?: { parameter: string; reason: string }
}

export interface SavedViewLimitsFixture {
  max_name_length?: number
  max_per_user?: number
  max_shared?: number
  can_share?: boolean
}

function toData(view: SavedViewFixture) {
  const { id, ...attributes } = view
  return {
    id,
    type: "saved_views",
    attributes: {
      icon: "bookmark",
      query_version: 1,
      layout: "",
      shared: false,
      show_in_sidebar: true,
      show_on_dashboard: false,
      sort_order: 0,
      owned: true,
      can_edit: true,
      can_delete: true,
      ...attributes,
    },
  }
}

const base = (slug: string) => `/g/${encodeURIComponent(slug)}/saved-views`

// list backs GET /saved-views with the views in the order given.
export function list(
  slug: string,
  views: SavedViewFixture[] = [],
  limits: SavedViewLimitsFixture = {}
) {
  return [
    http.get(apiUrl(base(slug)), () =>
      HttpResponse.json({
        data: views.map(toData),
        meta: {
          total: views.length,
          limits: {
            max_name_length: 60,
            max_per_user: 50,
            max_shared: 50,
            can_share: true,
            ...limits,
          },
        },
      })
    ),
  ]
}

// counts backs GET /saved-views/counts.
export function counts(slug: string, byId: Record<string, number> = {}) {
  return [
    http.get(apiUrl(`${base(slug)}/counts`), () => HttpResponse.json({ data: { counts: byId } })),
  ]
}

// none answers both reads with nothing saved, for tests of the pages that
// show saved views but are not about them.
export function none(slug: string) {
  return [...list(slug), ...counts(slug)]
}

// create backs POST /saved-views. onBody sees the request attributes; the
// response echoes them as a view with the given id.
export function create(
  slug: string,
  id: string,
  onBody?: (attributes: Record<string, unknown>) => void
) {
  return [
    http.post(apiUrl(base(slug)), async ({ request }) => {
      const body = (await request.json()) as { data: { attributes: Record<string, unknown> } }
      onBody?.(body.data.attributes)
      return HttpResponse.json(
        { data: toData({ id, ...(body.data.attributes as Omit<SavedViewFixture, "id">) }) },
        { status: 201 }
      )
    }),
  ]
}

// update backs PATCH /saved-views/{id}, merging the request into view.
export function update(
  slug: string,
  view: SavedViewFixture,
  onBody?: (attributes: Record<string, unknown>) => void
) {
  return [
    http.patch(apiUrl(`${base(slug)}/${encodeURIComponent(view.id)}`), async ({ request }) => {
      const body = (await request.json()) as { data: { attributes: Record<string, unknown> } }
      onBody?.(body.data.attributes)
      return HttpResponse.json({ data: toData({ ...view, ...body.data.attributes }) })
    }),
  ]
}

// remove backs DELETE /saved-views/{id}.
export function remove(slug: string, id: string, onCall?: () => void) {
  return [
    http.delete(apiUrl(`${base(slug)}/${encodeURIComponent(id)}`), () => {
      onCall?.()
      return new HttpResponse(null, { status: 204 })
    }),
  ]
}

// reorder backs POST /saved-views/reorder.
export function reorder(slug: string, onIds?: (ids: string[]) => void) {
  return [
    http.post(apiUrl(`${base(slug)}/reorder`), async ({ request }) => {
      const body = (await request.json()) as { data: { attributes: { ids: string[] } } }
      onIds?.(body.data.attributes.ids)
      return new HttpResponse(null, { status: 204 })
    }),
  ]
}
