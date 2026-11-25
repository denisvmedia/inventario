import { http, HttpResponse } from "msw"

import { capture } from "@/test/capture"
import type { Schema } from "@/types"

import { apiUrl } from "."

type SellerAttrs = Schema<"jsonapi.SellerView"> & { id: string }

function resource(seller: SellerAttrs) {
  return { id: seller.id, type: "sellers", attributes: seller }
}

// list backs GET /sellers: JSON:API resources under `data`, each with its
// item count (#2804).
export function list(slug: string, sellers: SellerAttrs[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/sellers`), () =>
      HttpResponse.json({ data: sellers.map(resource), meta: { sellers: sellers.length } })
    ),
  ]
}

// create backs POST /sellers. The request body lands in `body.value`.
export function create(slug: string, response: SellerAttrs, body = capture<unknown>()) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/sellers`), async ({ request }) => {
      body.value = await request.json()
      return HttpResponse.json({ data: resource(response) }, { status: 201 })
    }),
  ]
}

// nameTaken answers POST /sellers with the 409 of a duplicate name.
export function nameTaken(slug: string) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/sellers`), () =>
      HttpResponse.json(
        {
          errors: [
            {
              status: "Conflict",
              code: "seller.name_taken",
              detail: "a seller with this name already exists",
            },
          ],
        },
        { status: 409 }
      )
    ),
  ]
}

export function update(slug: string, response: SellerAttrs, body = capture<unknown>()) {
  return [
    http.patch(
      apiUrl(`/g/${encodeURIComponent(slug)}/sellers/${encodeURIComponent(response.id)}`),
      async ({ request }) => {
        body.value = await request.json()
        return HttpResponse.json({ data: resource(response) })
      }
    ),
  ]
}

// remove backs DELETE /sellers/{id}; `deleted` records each deleted id.
export function remove(slug: string, sellerID: string, deleted: string[] = []) {
  return [
    http.delete(
      apiUrl(`/g/${encodeURIComponent(slug)}/sellers/${encodeURIComponent(sellerID)}`),
      () => {
        deleted.push(sellerID)
        return new HttpResponse(null, { status: 204 })
      }
    ),
  ]
}
