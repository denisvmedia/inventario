import { http, HttpResponse } from "msw"

import { capture } from "@/test/capture"
import type { Schema } from "@/types"

import { apiUrl } from "."

type ItemTypeAttrs = Schema<"jsonapi.ItemTypeView"> & { id: string }

const BUILTINS: { slug: string; icon: string }[] = [
  { slug: "white_goods", icon: "refrigerator" },
  { slug: "electronics", icon: "laptop" },
  { slug: "equipment", icon: "wrench" },
  { slug: "furniture", icon: "armchair" },
  { slug: "clothes", icon: "shirt" },
  { slug: "other", icon: "package" },
]

// builtins returns the six types every group starts with, unrenamed and
// without items, as GET /item-types lists them (#2802).
export function builtins(): ItemTypeAttrs[] {
  return BUILTINS.map(({ slug, icon }) => ({
    id: `type-${slug}`,
    slug,
    name: "",
    icon,
    builtin: true,
    item_count: 0,
    trashed_item_count: 0,
    draft_count: 0,
  }))
}

function resource(type: ItemTypeAttrs) {
  return { id: type.id, type: "item_types", attributes: type }
}

function groupPath(slug: string, rest = "") {
  return apiUrl(`/g/${encodeURIComponent(slug)}/item-types${rest}`)
}

// defaults answers GET /item-types for any group with the built-in types.
// src/test/server.ts installs it for every test, because every page that
// shows an item resolves its type through this list; a test that cares
// about the types overrides it with `list`.
export function defaults() {
  return [
    http.get(apiUrl("/g/:groupSlug/item-types"), () =>
      HttpResponse.json({ data: builtins().map(resource), meta: { item_types: BUILTINS.length } })
    ),
  ]
}

// list backs GET /item-types for one group.
export function list(slug: string, types: ItemTypeAttrs[] = builtins()) {
  return [
    http.get(groupPath(slug), () =>
      HttpResponse.json({ data: types.map(resource), meta: { item_types: types.length } })
    ),
  ]
}

// create backs POST /item-types. The request body lands in `body.value`.
export function create(slug: string, response: ItemTypeAttrs, body = capture<unknown>()) {
  return [
    http.post(groupPath(slug), async ({ request }) => {
      body.value = await request.json()
      return HttpResponse.json({ data: resource(response) }, { status: 201 })
    }),
  ]
}

// nameTaken answers POST /item-types with the 409 of a duplicate name.
export function nameTaken(slug: string) {
  return [
    http.post(groupPath(slug), () =>
      HttpResponse.json(
        {
          errors: [
            {
              status: "Conflict",
              code: "item_type.name_taken",
              detail: "an item type with this name already exists",
            },
          ],
        },
        { status: 409 }
      )
    ),
  ]
}

// limit answers POST /item-types with the 409 of a group that already has
// the most types it keeps.
export function limit(slug: string) {
  return [
    http.post(groupPath(slug), () =>
      HttpResponse.json(
        {
          errors: [
            {
              status: "Conflict",
              code: "item_type.limit",
              error: { error: "a group keeps at most 100 item types", type: "*errors.errorString" },
              meta: { limit: 100 },
            },
          ],
        },
        { status: 409 }
      )
    ),
  ]
}

// update backs PATCH /item-types/{id}. The request body lands in `body.value`.
export function update(slug: string, response: ItemTypeAttrs, body = capture<unknown>()) {
  return [
    http.patch(groupPath(slug, `/${encodeURIComponent(response.id)}`), async ({ request }) => {
      body.value = await request.json()
      return HttpResponse.json({ data: resource(response) })
    }),
  ]
}

// remove backs DELETE /item-types/{id}. Each request's reassign_to (or "")
// is pushed onto `calls`, and the response reports `moved` items moved.
export function remove(slug: string, typeID: string, moved = 0, calls: string[] = []) {
  return [
    http.delete(groupPath(slug, `/${encodeURIComponent(typeID)}`), ({ request }) => {
      calls.push(new URL(request.url).searchParams.get("reassign_to") ?? "")
      return HttpResponse.json({ meta: { reassigned_items: moved } })
    }),
  ]
}

// inUse answers DELETE /item-types/{id} without reassign_to with the 409 of
// a type that items or drafts have, and with reassign_to like `remove`.
export function inUse(
  slug: string,
  typeID: string,
  counts: { live: number; trashed: number; drafts?: number },
  calls: string[] = []
) {
  return [
    http.delete(groupPath(slug, `/${encodeURIComponent(typeID)}`), ({ request }) => {
      const target = new URL(request.url).searchParams.get("reassign_to") ?? ""
      calls.push(target)
      if (target) {
        return HttpResponse.json({ meta: { reassigned_items: counts.live + counts.trashed } })
      }
      return HttpResponse.json(
        {
          errors: [
            {
              status: "Conflict",
              code: "item_type.in_use",
              detail: "items still have this type; choose a type to move them to",
              meta: {
                item_count: counts.live,
                trashed_item_count: counts.trashed,
                draft_count: counts.drafts ?? 0,
              },
            },
          ],
        },
        { status: 409 }
      )
    }),
  ]
}

// UNKNOWN_TYPE_BODY is the 422 an item write gets for a type its group does
// not have, byte for byte as the server sends it: the field error sits under
// data.attributes.type with the item_type_unknown validation code.
export const UNKNOWN_TYPE_BODY = {
  errors: [
    {
      status: "Unprocessable Entity",
      error: {
        error: { data: { attributes: { type: "no item type with this slug in the group" } } },
        errorCodes: { data: { attributes: { type: { code: "item_type_unknown" } } } },
        type: "validation.Errors",
      },
      code: "item_type.unknown",
    },
  ],
}

// unknownOnUpdate answers PUT /commodities/{id} with UNKNOWN_TYPE_BODY.
export function unknownOnUpdate(slug: string, commodityID: string) {
  return [
    http.put(
      apiUrl(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}`),
      () => HttpResponse.json(UNKNOWN_TYPE_BODY, { status: 422 })
    ),
  ]
}
