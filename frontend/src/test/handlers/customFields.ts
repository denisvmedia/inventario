import { http, HttpResponse } from "msw"

import type { CustomField } from "@/features/customFields/api"

import { capture } from "../capture"

import { apiUrl } from "."

function resource(field: CustomField) {
  return { id: field.id, type: "custom_fields", attributes: field }
}

function groupPath(slug: string, rest = "") {
  return apiUrl(`/g/${encodeURIComponent(slug)}/custom-fields${rest}`)
}

// field returns a custom field without values, with the given overrides.
export function field(
  overrides: Partial<CustomField> & Pick<CustomField, "id" | "name">
): CustomField {
  return {
    type: "text",
    options: [],
    templates: [],
    item_count: 0,
    trashed_item_count: 0,
    ...overrides,
  }
}

// defaults answers GET /custom-fields for any group with no fields.
// src/test/server.ts installs it for every test, because the item form and
// the item page read the group's fields (#2803); a test that cares about
// the fields overrides it with `list`.
export function defaults() {
  return [
    http.get(apiUrl("/g/:groupSlug/custom-fields"), () =>
      HttpResponse.json({ data: [], meta: { custom_fields: 0, limit: 200 } })
    ),
  ]
}

// list backs GET /custom-fields for one group.
export function list(slug: string, fields: CustomField[]) {
  return [
    http.get(groupPath(slug), () =>
      HttpResponse.json({
        data: fields.map(resource),
        meta: { custom_fields: fields.length, limit: 200 },
      })
    ),
  ]
}

// create backs POST /custom-fields, answering with response.
export function create(slug: string, response: CustomField, body = capture<unknown>()) {
  return [
    http.post(groupPath(slug), async ({ request }) => {
      body.value = await request.json()
      return HttpResponse.json({ data: resource(response) }, { status: 201 })
    }),
  ]
}

// update backs PATCH /custom-fields/{id}, answering with response.
export function update(slug: string, response: CustomField, body = capture<unknown>()) {
  return [
    http.patch(groupPath(slug, `/${encodeURIComponent(response.id)}`), async ({ request }) => {
      body.value = await request.json()
      return HttpResponse.json({ data: resource(response) })
    }),
  ]
}

// remove backs DELETE /custom-fields/{id}. Each request's delete_values (or
// "") is pushed onto calls. A request without delete_values=true is refused
// with the 409 of a field in use when counts are given.
export function remove(
  slug: string,
  fieldID: string,
  calls: string[] = [],
  counts?: { live: number; trashed: number }
) {
  return [
    http.delete(groupPath(slug, `/${encodeURIComponent(fieldID)}`), ({ request }) => {
      const deleteValues = new URL(request.url).searchParams.get("delete_values") ?? ""
      calls.push(deleteValues)
      if (counts && deleteValues !== "true") {
        return HttpResponse.json(
          {
            errors: [
              {
                status: "Conflict",
                code: "custom_field.in_use",
                detail: "items have values for this field",
                meta: { item_count: counts.live, trashed_item_count: counts.trashed },
              },
            ],
          },
          { status: 409 }
        )
      }
      return HttpResponse.json({
        meta: { deleted_values: counts ? counts.live + counts.trashed : 0 },
      })
    }),
  ]
}
