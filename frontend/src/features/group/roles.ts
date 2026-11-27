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

// Items and what belongs to them (files, tags, sellers, item types, custom
// fields, loans, services, maintenance, reminders, drafts, batch intake) take
// writes from users, admins and owners; viewers only read
// (contentWriteGate in go/apiserver/apiserver.go).
export function canEditContent(role: GroupRole | null): boolean {
  return role === "user" || role === "admin" || role === "owner"
}

export interface GroupPermissions {
  // The caller's role in the current group; null while it is unknown.
  role: GroupRole | null
  // May create, change and delete items and what belongs to them.
  canEditContent: boolean
  // May create, change and delete locations and areas.
  canManageLocations: boolean
}

// What the caller may change in the current group, by the server's write
// gates. A page offers a write action only when the matching flag is true.
// Both are false while the role is unknown (the member list still loading,
// or failed), so an action shows once the role is known and never to a
// role the server would refuse.
export function useGroupPermissions(): GroupPermissions {
  const role = useMyGroupRole()
  return {
    role,
    canEditContent: canEditContent(role),
    canManageLocations: canManageLocations(role),
  }
}
