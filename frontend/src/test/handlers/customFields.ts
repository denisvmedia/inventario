import { http, HttpResponse } from "msw"

import type { CustomField } from "@/features/customFields/api"

import { apiUrl } from "."

function resource(field: CustomField) {
  return { id: field.id, type: "custom_fields", attributes: field }
}

function groupPath(slug: string, rest = "") {
  return apiUrl(`/g/${encodeURIComponent(slug)}/custom-fields${rest}`)
}

// field returns a custom field without values, with the given overrides.
export function field(overrides: Partial<CustomField> & Pick<CustomField, "id" | "name">): CustomField {
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
