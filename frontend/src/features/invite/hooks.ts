import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { groupKeys } from "@/features/group/keys"

import { acceptInvite, getInviteInfo, type GroupMembership, type InviteInfo } from "./api"
import { inviteKeys } from "./keys"

// Reads the public invite preview. Disabled when token is empty so the
// caller can mount the hook before deciding whether to fire the request.
export function useInviteInfo(token: string | undefined) {
  return useQuery<InviteInfo>({
    queryKey: inviteKeys.info(token ?? ""),
    queryFn: ({ signal }) => getInviteInfo(token!, signal),
    enabled: !!token,
    retry: false,
  })
}

// Accepts an invite. On success we invalidate the groups list so the new
// membership shows up in the sidebar / RootRedirect immediately, without
// waiting for a stale-time refresh.
export function useAcceptInvite() {
  const queryClient = useQueryClient()
  return useMutation<GroupMembership & { id?: string }, Error, string>({
    mutationFn: (token) => acceptInvite(token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: groupKeys.all })
    },
  })
}
