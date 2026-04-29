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
//
// We deliberately throw when `data.attributes` is missing rather than
// returning `{}`. An empty object is truthy and would slip past the
// `!invite` guard in InviteAcceptPage; both `expired` and `used` would
// read as `undefined`, falling into the "actionable" branch and offering
// Accept on a token whose state we can't actually confirm. Throwing makes
// the page render the invalid-invite panel instead — fail-closed.
export async function getInviteInfo(token: string, signal?: AbortSignal): Promise<InviteInfo> {
  const body = await http.get<InviteInfoEnvelope>(`/invites/${encodeURIComponent(token)}`, {
    signal,
  })
  const attributes = body.data?.attributes
  if (!attributes) {
    throw new Error("Invite response is missing data.attributes")
  }
  return attributes
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
