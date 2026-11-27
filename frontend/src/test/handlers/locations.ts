import { http, HttpResponse } from "msw"

import { apiUrl, serverPage } from "."

export function list(slug: string, items: unknown[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/locations`), () =>
      HttpResponse.json({ data: items })
    ),
  ]
}

// paged answers like the server: `page` and `per_page` pick a slice of
// `items`, and meta reports the total. Each request's URL is appended to
// `requests` when given.
export function paged(slug: string, items: unknown[], requests: string[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/locations`), ({ request }) => {
      requests.push(request.url)
      const { rows, page, perPage, totalPages } = serverPage(request.url, items)
      return HttpResponse.json({
        data: rows,
        meta: { locations: items.length, page, per_page: perPage, total_pages: totalPages },
      })
    }),
  ]
}

export function detail(slug: string, id: string, item: unknown) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({ data: item })
    ),
  ]
}

export function error(slug: string, status = 500) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/locations`), () =>
      HttpResponse.json({ error: "boom" }, { status })
    ),
  ]
}

export function create(slug: string, response: unknown) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/locations`), () =>
      HttpResponse.json({ data: response }, { status: 201 })
    ),
  ]
}

export function update(slug: string, id: string, response: unknown) {
  return [
    http.put(apiUrl(`/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({ data: response })
    ),
  ]
}

export function updateError(slug: string, id: string, status = 500) {
  return [
    http.put(apiUrl(`/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({ error: "boom" }, { status })
    ),
  ]
}

export function remove(slug: string, id: string) {
  return [
    http.delete(
      apiUrl(`/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(id)}`),
      () => new HttpResponse(null, { status: 204 })
    ),
  ]
}

export function removeError(slug: string, id: string, status = 500) {
  return [
    http.delete(apiUrl(`/g/${encodeURIComponent(slug)}/locations/${encodeURIComponent(id)}`), () =>
      HttpResponse.json({ error: "boom" }, { status })
    ),
  ]
}
