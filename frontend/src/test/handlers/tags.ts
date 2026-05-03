import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// Default colour kept off-palette deliberately so tests that forget to
// set one trigger a clear visual mismatch instead of a silent passthrough.
type TagAttrs = { id: string; slug: string; label: string; color: string }
type TagWithUsage = TagAttrs & { meta?: { usage?: { commodities: number; files: number } } }

// list mirrors apiserver/tags.go::listTags → TagsResponse — entities are
// FLAT inside `data` with an optional inline `meta.usage` block when the
// caller passes ?include=usage. Mirrors the `meta` block on each row that
// the BE renders via TagListItem (see go/jsonapi/tags.go).
export function list(slug: string, items: TagWithUsage[] = [], meta: Record<string, unknown> = {}) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/tags`), () =>
      HttpResponse.json({
        data: items,
        meta: { tags: items.length, total: items.length, ...meta },
      })
    ),
  ]
}

// stats backs GET /tags/stats — small flat envelope, NOT JSON:API.
export function stats(
  slug: string,
  data: {
    tags_total: number
    items_tagged: number
    items_untagged: number
    files_tagged: number
    files_untagged: number
  }
) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/tags/stats`), () =>
      HttpResponse.json({ data })
    ),
  ]
}

export function detail(
  slug: string,
  id: string,
  attributes: TagAttrs,
  usage?: { commodities: number; files: number }
) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/tags/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({
        id,
        type: "tags",
        attributes,
        meta: usage ? { usage } : undefined,
      })
    ),
  ]
}

export function create(slug: string, attributes: TagAttrs) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/tags`), () =>
      HttpResponse.json({ id: attributes.id, type: "tags", attributes }, { status: 201 })
    ),
  ]
}

export function update(slug: string, id: string, attributes: TagAttrs) {
  return [
    http.patch(apiUrl(`/g/${encodeURIComponent(slug)}/tags/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({ id, type: "tags", attributes })
    ),
  ]
}

export function remove(slug: string, id: string, options: { conflict?: boolean } = {}) {
  return [
    http.delete(
      apiUrl(`/g/${encodeURIComponent(slug)}/tags/${encodeURIComponent(id)}`),
      ({ request }) => {
        const url = new URL(request.url)
        if (options.conflict && url.searchParams.get("force") !== "true") {
          return HttpResponse.json(
            {
              errors: [
                {
                  status: "409",
                  title: "tag is in use",
                  detail: "Pass force=true to strip references.",
                },
              ],
            },
            { status: 409 }
          )
        }
        return new HttpResponse(null, { status: 204 })
      }
    ),
  ]
}
