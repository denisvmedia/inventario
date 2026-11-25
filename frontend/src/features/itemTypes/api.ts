// Data layer for the group's item types (#2802). Hooks live in `./hooks.ts`.
// Backed by `/g/{slug}/item-types` and `/g/{slug}/item-types/{id}`.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

// An item type with the number of the group's items that have it. An item
// stores its type's slug, which never changes. A built-in type keeps an
// empty name until a member renames it; the app shows its translated
// default name meanwhile (see ./display.ts).
export type ItemType = Schema<"jsonapi.ItemTypeView"> & { id: string }

// Limit mirrors MaxItemTypeNameRunes in go/models/item_type.go.
export const ITEM_TYPE_NAME_MAX = 50

// JSON:API error codes of the item type endpoints.
// A create or rename whose name another type of the group already has.
export const ITEM_TYPE_NAME_TAKEN = "item_type.name_taken"
// A delete without reassign_to of a type that items still have, Trash
// included. The error's meta carries item_count and trashed_item_count.
export const ITEM_TYPE_IN_USE = "item_type.in_use"
// A delete of the group's last type.
export const ITEM_TYPE_LAST = "item_type.last"
// A create in a group that has ITEM_TYPES_MAX types.
export const ITEM_TYPE_LIMIT = "item_type.limit"
// Limit mirrors MaxItemTypesPerGroup in go/models/item_type.go.
export const ITEM_TYPES_MAX = 100

export interface ItemTypeInput {
  name: string
  icon: string
}

type ItemTypeAttributes = Schema<"jsonapi.ItemTypeView">

interface ItemTypeResource {
  id?: string
  type?: string
  attributes?: ItemTypeAttributes
}

interface ItemTypeEnvelope {
  data?: ItemTypeResource
}

interface ItemTypesEnvelope {
  data?: ItemTypeResource[]
  meta?: { item_types?: number }
}

interface ItemTypeDeleteEnvelope {
  meta?: { reassigned_items?: number }
}

function toItemType(resource: ItemTypeResource | undefined, what: string): ItemType {
  if (!resource?.attributes || !resource.id) {
    throw new Error(`Malformed ${what} response: missing data`)
  }
  return { ...resource.attributes, id: resource.id }
}

// listItemTypes returns the group's types: the built-in ones in their fixed
// order, then the others by name.
export async function listItemTypes(signal?: AbortSignal): Promise<ItemType[]> {
  const body = await http.get<ItemTypesEnvelope>("/item-types", { signal })
  return (body.data ?? []).map((row) => toItemType(row, "GET /item-types"))
}

export async function createItemType(input: ItemTypeInput): Promise<ItemType> {
  const body = await http.post<ItemTypeEnvelope>("/item-types", {
    data: { type: "item_types", attributes: input },
  })
  return toItemType(body.data, "POST /item-types")
}

export async function updateItemType(id: string, input: Partial<ItemTypeInput>): Promise<ItemType> {
  const body = await http.patch<ItemTypeEnvelope>(`/item-types/${encodeURIComponent(id)}`, {
    data: { id, type: "item_types", attributes: input },
  })
  return toItemType(body.data, `PATCH /item-types/${id}`)
}

// deleteItemType deletes a type. reassignTo is the id of the type its items
// move to; without it the server refuses a type that items still have.
// Resolves with the number of items moved.
export async function deleteItemType(id: string, reassignTo?: string): Promise<number> {
  const query = reassignTo ? `?reassign_to=${encodeURIComponent(reassignTo)}` : ""
  const body = await http.del<ItemTypeDeleteEnvelope | undefined>(
    `/item-types/${encodeURIComponent(id)}${query}`
  )
  return body?.meta?.reassigned_items ?? 0
}
