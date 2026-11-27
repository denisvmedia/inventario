import { http, HttpResponse } from "msw"

import type { GroupRole } from "@/features/group/api"

import { apiUrl } from "."
import { fixtureUser, signedIn } from "./auth"

// Members live under /groups/{id}/members rather than /g/{slug}/...
// (the legacy backend keeps the membership endpoints group-id-keyed).
export function list(groupId: string, members: unknown[] = []) {
  return [
    http.get(apiUrl(`/groups/${encodeURIComponent(groupId)}/members`), () =>
      HttpResponse.json({ data: members })
    ),
  ]
}

// asRole signs a user in (the fixture user unless userId is given) and
// lists them as a member of groupId with the given role, so a page under
// AuthProvider knows what the caller may change (features/group/roles.ts).
// Tests of what a role is offered pass that role; other page tests pass
// "owner" to see every action.
export function asRole(groupId: string, role: GroupRole, userId?: string) {
  const id = userId ?? fixtureUser.id
  return [
    ...signedIn({ user: { id } }),
    ...list(groupId, [
      {
        id: `membership-${id}`,
        type: "memberships",
        attributes: { group_id: groupId, member_user_id: id, role },
      },
    ]),
  ]
}
