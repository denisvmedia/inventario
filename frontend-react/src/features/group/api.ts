// Pure data-layer functions for the group feature slice. Hooks live in
// `./hooks.ts`; React-aware code (the provider, useCurrentGroup) sits in
// `./GroupContext.tsx`.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type LocationGroup = Schema<"models.LocationGroup">
export type GroupRole = Schema<"models.GroupRole">
export type GroupMembership = Schema<"models.GroupMembership">
export type GroupInvite = Schema<"models.GroupInvite">

interface GroupResource {
  id: string
  type: string
  attributes: LocationGroup
}

interface GroupsListResponse {
  data: GroupResource[]
  meta?: unknown
}

interface GroupResponseEnvelope {
  data?: {
    id?: string
    attributes?: LocationGroup
    type?: string
  }
}

interface MembershipsResponseEnvelope {
  data?: Array<{
    id?: string
    attributes?: GroupMembership
    type?: string
  }>
}

interface InvitesResponseEnvelope {
  data?: Array<{
    id?: string
    attributes?: GroupInvite
    type?: string
  }>
}

interface InviteResponseEnvelope {
  data?: {
    id?: string
    attributes?: GroupInvite
    type?: string
  }
}

// Returns the active location groups the authenticated user is a member of.
// JSON:API envelope is unwrapped here so consumers see a plain LocationGroup[].
export async function listGroups(signal?: AbortSignal): Promise<LocationGroup[]> {
  const body = await http.get<GroupsListResponse>("/groups", { signal })
  return (body.data ?? []).map((item) => ({ ...item.attributes, id: item.id }))
}

export async function getGroup(groupId: string, signal?: AbortSignal): Promise<LocationGroup> {
  const body = await http.get<GroupResponseEnvelope>(`/groups/${encodeURIComponent(groupId)}`, {
    signal,
  })
  if (!body.data?.attributes) {
    throw new Error(`Group ${groupId} response missing data.attributes`)
  }
  return { ...body.data.attributes, id: body.data.id }
}

// LocationGroupRequest envelope shape: { data: { type: "groups", attributes: {...} } }.
// `name` is required server-side on create; `icon` and `main_currency` are
// optional. main_currency is set once on create and immutable on PATCH.
function envelope(attributes: Partial<LocationGroup>): {
  data: { type: string; attributes: Partial<LocationGroup> }
} {
  return { data: { type: "groups", attributes } }
}

export interface CreateGroupRequest {
  name: string
  icon?: string
  main_currency?: string
}

export async function createGroup(req: CreateGroupRequest): Promise<LocationGroup> {
  const body = await http.post<GroupResponseEnvelope>("/groups", envelope(req))
  if (!body.data?.attributes) {
    throw new Error("Create-group response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

export interface UpdateGroupRequest {
  name?: string
  // Empty string clears the icon (matches BE: empty means "no icon").
  icon?: string
}

export async function updateGroup(
  groupId: string,
  req: UpdateGroupRequest
): Promise<LocationGroup> {
  // BE expects PATCH on /groups/{id} with a LocationGroupRequest envelope.
  const body = await http.patch<GroupResponseEnvelope>(
    `/groups/${encodeURIComponent(groupId)}`,
    envelope(req)
  )
  if (!body.data?.attributes) {
    throw new Error("Update-group response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

export interface DeleteGroupRequest {
  confirm_word: string
  password: string
}

// DELETE /groups/{id} — async deletion. 204 on success, 422 on bad
// confirm_word or password. Caller should compare confirm_word to the
// group's current name client-side to avoid a server round-trip on
// obvious typos, but the server is authoritative.
//
// We use `http.request` rather than `http.del` because the helper
// excludes `body` (DELETE-with-body is unusual but the BE accepts the
// confirm envelope on DELETE here per the OpenAPI spec).
export async function deleteGroup(groupId: string, req: DeleteGroupRequest): Promise<void> {
  await http.request<void>(`/groups/${encodeURIComponent(groupId)}`, {
    method: "DELETE",
    body: req,
  })
}

export async function leaveGroup(groupId: string): Promise<void> {
  await http.post<void>(`/groups/${encodeURIComponent(groupId)}/leave`)
}

// --- Members --------------------------------------------------------------

export async function listMembers(
  groupId: string,
  signal?: AbortSignal
): Promise<Array<GroupMembership & { id?: string }>> {
  const body = await http.get<MembershipsResponseEnvelope>(
    `/groups/${encodeURIComponent(groupId)}/members`,
    { signal }
  )
  return (body.data ?? []).map((m) => ({ ...(m.attributes ?? {}), id: m.id }))
}

export async function changeMemberRole(
  groupId: string,
  memberUserId: string,
  role: GroupRole
): Promise<void> {
  // PATCH /groups/{id}/members/{userId} expects a JSON:API envelope with
  // attributes.role. The schema lives at jsonapi.GroupMembershipRoleRequest
  // → data.attributes.role.
  await http.patch<unknown>(
    `/groups/${encodeURIComponent(groupId)}/members/${encodeURIComponent(memberUserId)}`,
    { data: { attributes: { role } } }
  )
}

export async function removeMember(groupId: string, memberUserId: string): Promise<void> {
  await http.del<void>(
    `/groups/${encodeURIComponent(groupId)}/members/${encodeURIComponent(memberUserId)}`
  )
}

// --- Invites --------------------------------------------------------------

export async function listInvites(
  groupId: string,
  signal?: AbortSignal
): Promise<Array<GroupInvite & { id?: string }>> {
  const body = await http.get<InvitesResponseEnvelope>(
    `/groups/${encodeURIComponent(groupId)}/invites`,
    { signal }
  )
  return (body.data ?? []).map((i) => ({ ...(i.attributes ?? {}), id: i.id }))
}

export async function createInvite(groupId: string): Promise<GroupInvite & { id?: string }> {
  const body = await http.post<InviteResponseEnvelope>(
    `/groups/${encodeURIComponent(groupId)}/invites`
  )
  if (!body.data?.attributes) {
    throw new Error("Create-invite response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

export async function revokeInvite(groupId: string, inviteId: string): Promise<void> {
  await http.del<void>(
    `/groups/${encodeURIComponent(groupId)}/invites/${encodeURIComponent(inviteId)}`
  )
}
