import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  changeMemberRole,
  createGroup,
  createInvite,
  deleteGroup,
  getGroup,
  leaveGroup,
  listGroups,
  listInvites,
  listMembers,
  removeMember,
  revokeInvite,
  updateGroup,
  type CreateGroupRequest,
  type DeleteGroupRequest,
  type GroupInvite,
  type GroupMembership,
  type GroupRole,
  type LocationGroup,
  type UpdateGroupRequest,
} from "./api"
import { groupKeys } from "./keys"

// Fetches the user's active groups. The list is cheap, doesn't change often,
// and is read by the GroupProvider, the GroupRequiredRoute guard, and the
// group switcher in the app shell — one cache entry serves all three.
export function useGroups() {
  return useQuery<LocationGroup[]>({
    queryKey: groupKeys.list(),
    queryFn: ({ signal }) => listGroups(signal),
  })
}

export function useGroup(groupId: string | undefined) {
  return useQuery<LocationGroup>({
    queryKey: groupKeys.detail(groupId ?? ""),
    queryFn: ({ signal }) => getGroup(groupId!, signal),
    enabled: !!groupId,
  })
}

export function useMembers(groupId: string | undefined) {
  return useQuery<Array<GroupMembership & { id?: string }>>({
    queryKey: groupKeys.members(groupId ?? ""),
    queryFn: ({ signal }) => listMembers(groupId!, signal),
    enabled: !!groupId,
  })
}

export function useInvites(groupId: string | undefined, opts: { enabled?: boolean } = {}) {
  return useQuery<Array<GroupInvite & { id?: string }>>({
    queryKey: groupKeys.invites(groupId ?? ""),
    queryFn: ({ signal }) => listInvites(groupId!, signal),
    enabled: !!groupId && (opts.enabled ?? true),
  })
}

// --- Mutations -----------------------------------------------------------

// Creating a group: invalidate the list so the new entry appears in the
// sidebar / RootRedirect on next refetch. Caller is responsible for
// navigating to /g/{newSlug}.
export function useCreateGroup() {
  const queryClient = useQueryClient()
  return useMutation<LocationGroup, Error, CreateGroupRequest>({
    mutationFn: (req) => createGroup(req),
    onSuccess: async () => {
      // Await the refetch so that callers (NoGroupPage's onboarding flow,
      // CreateGroupPage's redirect) can safely navigate to "/" — the
      // RootRedirect guard reads `groups` synchronously, and a stale-cache
      // read while invalidation is still pending would bounce the user
      // straight back to /no-group. `invalidateQueries` resolves once the
      // refetch settles.
      await queryClient.invalidateQueries({ queryKey: groupKeys.list() })
    },
  })
}

interface UpdateGroupVars {
  groupId: string
  patch: UpdateGroupRequest
}

export function useUpdateGroup() {
  const queryClient = useQueryClient()
  return useMutation<LocationGroup, Error, UpdateGroupVars>({
    mutationFn: ({ groupId, patch }) => updateGroup(groupId, patch),
    onSuccess: (group, { groupId }) => {
      queryClient.setQueryData(groupKeys.detail(groupId), group)
      queryClient.invalidateQueries({ queryKey: groupKeys.list() })
    },
  })
}

interface DeleteGroupVars extends DeleteGroupRequest {
  groupId: string
}

// On success the BE flips status to pending_deletion and the row eventually
// drops out of /groups. We blow away the entire group namespace so the
// sidebar + RootRedirect refetch immediately.
export function useDeleteGroup() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, DeleteGroupVars>({
    mutationFn: ({ groupId, confirm_word, password }) =>
      deleteGroup(groupId, { confirm_word, password }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: groupKeys.all })
    },
  })
}

export function useLeaveGroup() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, { groupId: string }>({
    mutationFn: ({ groupId }) => leaveGroup(groupId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: groupKeys.all })
    },
  })
}

interface ChangeRoleVars {
  groupId: string
  memberUserId: string
  role: GroupRole
}

export function useChangeMemberRole() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, ChangeRoleVars>({
    mutationFn: ({ groupId, memberUserId, role }) => changeMemberRole(groupId, memberUserId, role),
    onSuccess: (_void, { groupId }) => {
      queryClient.invalidateQueries({ queryKey: groupKeys.members(groupId) })
    },
  })
}

interface RemoveMemberVars {
  groupId: string
  memberUserId: string
}

export function useRemoveMember() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, RemoveMemberVars>({
    mutationFn: ({ groupId, memberUserId }) => removeMember(groupId, memberUserId),
    onSuccess: (_void, { groupId }) => {
      queryClient.invalidateQueries({ queryKey: groupKeys.members(groupId) })
    },
  })
}

export function useCreateInvite() {
  const queryClient = useQueryClient()
  return useMutation<GroupInvite & { id?: string }, Error, { groupId: string }>({
    mutationFn: ({ groupId }) => createInvite(groupId),
    onSuccess: (_invite, { groupId }) => {
      queryClient.invalidateQueries({ queryKey: groupKeys.invites(groupId) })
    },
  })
}

interface RevokeInviteVars {
  groupId: string
  inviteId: string
}

export function useRevokeInvite() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, RevokeInviteVars>({
    mutationFn: ({ groupId, inviteId }) => revokeInvite(groupId, inviteId),
    onSuccess: (_void, { groupId }) => {
      queryClient.invalidateQueries({ queryKey: groupKeys.invites(groupId) })
    },
  })
}
