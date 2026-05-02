import { http, HttpResponse } from "msw"

import { apiUrl } from "."

export function list(slug: string, items: unknown[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/locations`), () =>
      HttpResponse.json({ data: items })
    ),
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
