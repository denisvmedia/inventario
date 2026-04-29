// Invite feature slice — public token-based invite flow + post-auth accept.
// API paths come straight from the OpenAPI spec; both endpoints live OUTSIDE
// the /g/{slug}/* group rewrite (the user has no slug yet at invite time).
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type InviteInfo = NonNullable<Schema<"jsonapi.InviteInfoAttr">>
export type GroupMembership = NonNullable<Schema<"models.GroupMembership">>

interface InviteInfoEnvelope {
  data?: {
    attributes?: InviteInfo
    type?: string
  }
}

interface GroupMembershipEnvelope {
  data?: {
    id?: string
    attributes?: GroupMembership
    type?: string
  }
}

// Public preview of an invite — does NOT require authentication. Returns
// `{ group_name, group_icon?, expired, used }` so the page can decide
// whether to offer "Accept", "Expired", or "Used".
export async function getInviteInfo(token: string, signal?: AbortSignal): Promise<InviteInfo> {
  const body = await http.get<InviteInfoEnvelope>(`/invites/${encodeURIComponent(token)}`, {
    signal,
  })
  return body.data?.attributes ?? {}
}

// Accepts an invite as the currently authenticated user. Returns the new
// membership (id + group_id) so callers can switch the active group right
// after accept.
export async function acceptInvite(token: string): Promise<GroupMembership & { id?: string }> {
  const body = await http.post<GroupMembershipEnvelope>(
    `/invites/${encodeURIComponent(token)}/accept`
  )
  return { ...(body.data?.attributes ?? {}), id: body.data?.id }
}
