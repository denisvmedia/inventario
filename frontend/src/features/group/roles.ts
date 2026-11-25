import { useOptionalAuth } from "@/features/auth/AuthContext"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { useMembers } from "@/features/group/hooks"

import type { GroupRole } from "./api"

// The caller's role in the current group, read from the members list (the
// role badge in the top bar reads the same query). Null while the list
// loads, outside a group or a signed-in session, and for a caller the list
// does not hold.
export function useMyGroupRole(): GroupRole | null {
  const userId = useOptionalAuth()?.user?.id
  const groupId = useOptionalCurrentGroup()?.currentGroup?.id
  const members = useMembers(userId ? groupId : undefined)
  if (!userId) return null
  return members.data?.find((m) => m.member_user_id === userId)?.role ?? null
}

// Locations and areas are structural: the server accepts writes to them from
// admins and owners only (structuralWriteGate in go/apiserver/apiserver.go).
export function canManageLocations(role: GroupRole | null): boolean {
  return role === "admin" || role === "owner"
}
