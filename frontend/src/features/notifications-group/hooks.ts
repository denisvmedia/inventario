import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  getGroupNotifications,
  patchGroupNotifications,
  type GroupNotificationsPatchRequest,
  type GroupNotificationsResponse,
} from "./api"
import { groupNotificationsKeys } from "./keys"

// useGroupNotifications reads the effective toggle state for the auth'd
// user in the given group. Gated on a truthy slug so the card can render
// placeholder content while the parent GroupSettings page is still
// resolving the group resource.
export function useGroupNotifications(groupSlug: string | undefined | null) {
  return useQuery<GroupNotificationsResponse>({
    queryKey: groupNotificationsKeys.group(groupSlug ?? ""),
    queryFn: ({ signal }) => getGroupNotifications(groupSlug!, signal),
    enabled: !!groupSlug,
  })
}

// useUpdateGroupNotifications applies an optimistic update against the
// query cache so the Switch row reflects the new state without waiting
// on the round-trip. On error we roll the cache back to the pre-flight
// value — the BE PATCH echoes the post-write state, so the success
// path just replaces the cache with the server's authoritative read.
export function useUpdateGroupNotifications(groupSlug: string | undefined | null) {
  const queryClient = useQueryClient()
  return useMutation<
    GroupNotificationsResponse,
    Error,
    GroupNotificationsPatchRequest,
    { previous: GroupNotificationsResponse | undefined }
  >({
    mutationFn: (body) => patchGroupNotifications(groupSlug!, body),
    onMutate: async (body) => {
      if (!groupSlug) return { previous: undefined }
      const key = groupNotificationsKeys.group(groupSlug)
      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<GroupNotificationsResponse>(key)
      if (previous) {
        queryClient.setQueryData<GroupNotificationsResponse>(key, {
          ...previous,
          ...(body.warranty_expiring_alerts != null
            ? { warranty_expiring_alerts: body.warranty_expiring_alerts }
            : {}),
          ...(body.weekly_digest != null ? { weekly_digest: body.weekly_digest } : {}),
        })
      }
      return { previous }
    },
    onError: (_err, _body, ctx) => {
      if (!groupSlug || !ctx?.previous) return
      queryClient.setQueryData(groupNotificationsKeys.group(groupSlug), ctx.previous)
    },
    onSuccess: (data) => {
      if (!groupSlug) return
      queryClient.setQueryData(groupNotificationsKeys.group(groupSlug), data)
    },
  })
}
