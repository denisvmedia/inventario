// Data-layer for the per-group notification prefs slice (issue #1648).
// Two operations only: read the effective state, patch any subset of
// toggles. The BE composes per-group override + user-global + default
// behind these endpoints — see `services/notifications/preferences.go`.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type GroupNotificationsResponse = Schema<"apiserver.GroupNotificationsResponse">
export type GroupNotificationsPatchRequest = Schema<"apiserver.GroupNotificationsPatchRequest">

// getGroupNotifications + patchGroupNotifications both take the slug
// explicitly so the calls work from non-group routes. GroupSettings
// (`/groups/:groupId/settings`) is one such non-group route; mirroring
// the choice already made for getGroupPlan in #1656 keeps the slice
// uniform.
export async function getGroupNotifications(
  groupSlug: string,
  signal?: AbortSignal
): Promise<GroupNotificationsResponse> {
  return http.get<GroupNotificationsResponse>(`/g/${encodeURIComponent(groupSlug)}/notifications`, {
    signal,
  })
}

export async function patchGroupNotifications(
  groupSlug: string,
  body: GroupNotificationsPatchRequest
): Promise<GroupNotificationsResponse> {
  return http.patch<GroupNotificationsResponse>(
    `/g/${encodeURIComponent(groupSlug)}/notifications`,
    body
  )
}
