// MSW handlers for personal data exports (#2177): GET, POST
// /users/me/data-exports and POST /users/me/data-exports/{id}/download-url.
import { http, HttpResponse } from "msw"

import type { PersonalDataExport, PersonalDataExportsList } from "@/features/data-exports/api"

import { apiUrl } from "."

const listPath = () => apiUrl("/users/me/data-exports")

// defaults answers the list with no exports. src/test/server.ts installs it
// for every test, because Account settings asks for the list on every
// render; a test that cares overrides it with `list`.
export function defaults() {
  return [http.get(listPath(), () => HttpResponse.json({ exports: [], retention_days: 7 }))]
}

export function list(body: PersonalDataExportsList) {
  return [http.get(listPath(), () => HttpResponse.json({ retention_days: 7, ...body }))]
}

// listSequence answers the list with each body in turn, then keeps answering
// with the last one, and counts the calls in onList.
export function listSequence(bodies: PersonalDataExportsList[], onList?: () => void) {
  let call = 0
  return [
    http.get(listPath(), () => {
      onList?.()
      const body = bodies[Math.min(call, bodies.length - 1)]
      call++
      return HttpResponse.json({ retention_days: 7, ...body })
    }),
  ]
}

export function requestCreated(created: PersonalDataExport, onRequest?: () => void) {
  return [
    http.post(listPath(), () => {
      onRequest?.()
      return HttpResponse.json(created, { status: 201 })
    }),
  ]
}

export function requestRateLimited(retryAt: string) {
  return [
    http.post(listPath(), () =>
      HttpResponse.json(
        {
          errors: [
            {
              status: "Too Many Requests",
              code: "data_export.rate_limited",
              meta: { retry_at: retryAt, retry_after_seconds: 3600 },
            },
          ],
        },
        { status: 429, headers: { "Retry-After": "3600" } }
      )
    ),
  ]
}

// downloadRefused answers the download-url request with a coded error, as
// the server does for an expired copy (404) or an unfinished one (409).
export function downloadRefused(id: string, status: 404 | 409, code: string) {
  return [
    http.post(apiUrl(`/users/me/data-exports/${id}/download-url`), () =>
      HttpResponse.json({ errors: [{ status: String(status), code }] }, { status })
    ),
  ]
}

export function downloadURL(id: string, url: string) {
  return [
    http.post(apiUrl(`/users/me/data-exports/${id}/download-url`), () =>
      HttpResponse.json({ url, expires_at: "2026-11-26T13:00:00Z" })
    ),
  ]
}
