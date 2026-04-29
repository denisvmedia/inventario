// Pure data-layer functions for the group feature slice. Hooks live in
// `./hooks.ts`; React-aware code (the provider, useCurrentGroup) sits in
// `./GroupContext.tsx`.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type LocationGroup = Schema<"models.LocationGroup">

interface GroupResource {
  id: string
  type: string
  attributes: LocationGroup
}

interface GroupsListResponse {
  data: GroupResource[]
  meta?: unknown
}

// Returns the active location groups the authenticated user is a member of.
// JSON:API envelope is unwrapped here so consumers see a plain LocationGroup[].
export async function listGroups(signal?: AbortSignal): Promise<LocationGroup[]> {
  const body = await http.get<GroupsListResponse>("/groups", { signal })
  return (body.data ?? []).map((item) => ({ ...item.attributes, id: item.id }))
}
