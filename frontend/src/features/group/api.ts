// Pure data-layer functions for the group feature slice. Hooks live in
// `./hooks.ts`; React-aware code (the provider, useCurrentGroup) sits in
// `./GroupContext.tsx`.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type LocationGroup = Schema<"models.LocationGroup">
export type GroupRole = Schema<"models.GroupRole">
export type GroupMembership = Schema<"models.GroupMembership">
export type GroupInvite = Schema<"models.GroupInvite">
export type GroupSection = Schema<"models.GroupSection">
// The Collection set (#1607) as a group holds it: its item types and custom
// fields, each missing, added by the set, or the group's own of that name.
export type ItemSetView = Schema<"jsonapi.ItemSetView">
export type ItemSetItemTypeView = Schema<"jsonapi.ItemSetItemTypeView">
export type ItemSetFieldView = Schema<"jsonapi.ItemSetFieldView">
export type ItemSetMeta = Schema<"jsonapi.ItemSetMeta">
export type GroupCreateItemSetOutcome = Schema<"jsonapi.GroupCreateItemSetOutcome">
// MembershipWithUser is the shape returned by the members list endpoint
// after the #1533 expansion. The BE joins users so the FE can render
// avatar / name / email in one round-trip; the legacy attribute set
// (group_id / member_user_id / role / joined_at) stays in place so
// existing references keep working until the page rebuild lands.
export type MembershipWithUser = Schema<"jsonapi.MembershipWithUserAttr">

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
  meta?: Schema<"jsonapi.LocationGroupCreateMeta">
}

interface MembershipsResponseEnvelope {
  data?: Array<{
    id?: string
    attributes?: MembershipWithUser
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
// `name` is required server-side on create; `icon` and `group_currency` are
// optional. group_currency is set once on create and immutable on PATCH.
type GroupAttributes = Schema<"jsonapi.LocationGroupAttributes">

function envelope(attributes: Partial<GroupAttributes>): {
  data: { type: string; attributes: Partial<GroupAttributes> }
} {
  return { data: { type: "groups", attributes } }
}

export interface CreateGroupRequest {
  name: string
  icon?: string
  group_currency?: string
  // The sections the new group hides for every member.
  hidden_sections?: GroupSection[]
  // Adds the Collection set to the new group in the same request (#1607).
  collection_set?: boolean
}

// A created group, with how adding the Collection set went when the
// request asked for it. The group exists either way; when collectionSet
// says it was not added, adding it from the group settings finishes it.
export type CreatedGroup = LocationGroup & { collectionSet?: GroupCreateItemSetOutcome }

export async function createGroup(req: CreateGroupRequest): Promise<CreatedGroup> {
  const body = await http.post<GroupResponseEnvelope>("/groups", envelope(req))
  if (!body.data?.attributes) {
    throw new Error("Create-group response missing data.attributes")
  }
  const created: CreatedGroup = { ...body.data.attributes, id: body.data.id }
  if (body.meta?.collection_set) created.collectionSet = body.meta.collection_set
  return created
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

// PUT /groups/{id}/hidden-sections replaces the sections the group hides
// for all its members (#1607) and leaves the rest of the group as it is.
// Admin or owner only; the server drops duplicates and refuses unknown
// sections.
export async function setHiddenSections(
  groupId: string,
  sections: readonly GroupSection[]
): Promise<LocationGroup> {
  const body = await http.put<GroupResponseEnvelope>(
    `/groups/${encodeURIComponent(groupId)}/hidden-sections`,
    { data: { type: "groups", attributes: { hidden_sections: sections } } }
  )
  if (!body.data?.attributes) {
    throw new Error("Hidden-sections response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

interface ItemSetEnvelope {
  data?: { id?: string; type?: string; attributes?: ItemSetView }
  meta?: ItemSetMeta
}

// The Collection set as the group holds it, with what an add or a remove
// did when it is the answer to one.
export interface CollectionSetResult {
  set: ItemSetView
  meta?: ItemSetMeta
}

function unwrapItemSet(body: ItemSetEnvelope): CollectionSetResult {
  if (!body.data?.attributes) {
    throw new Error("Collection-set response missing data.attributes")
  }
  return { set: body.data.attributes, meta: body.meta }
}

function collectionSetPath(groupId: string): string {
  return `/groups/${encodeURIComponent(groupId)}/collection-set`
}

// GET /groups/{id}/collection-set lists the set's item types and custom
// fields and how the group holds each. Any member may read it.
export async function getCollectionSet(
  groupId: string,
  signal?: AbortSignal
): Promise<ItemSetView> {
  return unwrapItemSet(await http.get<ItemSetEnvelope>(collectionSetPath(groupId), { signal })).set
}

// POST adds the set: it creates what the group lacks and uses a type or
// field of the same name the group has. Adding again creates only what is
// missing. Admin or owner only.
export async function addCollectionSet(groupId: string): Promise<CollectionSetResult> {
  return unwrapItemSet(await http.post<ItemSetEnvelope>(collectionSetPath(groupId)))
}

// The rows a remove may delete: those the confirmation listed as deleted.
export interface CollectionSetRemoval {
  itemTypeIds: string[]
  customFieldIds: string[]
}

// DELETE removes the set: it deletes the listed types and fields that are
// still the set's and unused when it runs, never one it is not given, and
// names in meta what it deleted and what it kept of the list. Safe to
// repeat. Admin or owner only.
export async function removeCollectionSet(
  groupId: string,
  removal: CollectionSetRemoval
): Promise<CollectionSetResult> {
  const { data } = await http.request<ItemSetEnvelope>(collectionSetPath(groupId), {
    method: "DELETE",
    body: {
      data: {
        type: "item_sets",
        attributes: {
          item_type_ids: removal.itemTypeIds,
          custom_field_ids: removal.customFieldIds,
        },
      },
    },
  })
  return unwrapItemSet(data)
}

// removalOf lists what a confirmation built from view shows as deleted.
export function removalOf(view: ItemSetView): CollectionSetRemoval {
  return {
    itemTypeIds: (view.item_types ?? [])
      .filter((row) => row.removable && row.item_type_id)
      .map((row) => row.item_type_id!),
    customFieldIds: (view.custom_fields ?? [])
      .filter((row) => row.removable && row.custom_field_id)
      .map((row) => row.custom_field_id!),
  }
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

export type MemberRow = MembershipWithUser & { id?: string }

export async function listMembers(groupId: string, signal?: AbortSignal): Promise<MemberRow[]> {
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

export interface CreateInviteRequest {
  // Optional. When set, the BE persists invitee_email and dispatches
  // an email via EmailService. When empty, the invite is the legacy
  // copy-paste-token flow (the admin shares the URL out-of-band).
  email?: string
  // Optional. Defaults to "user" server-side. The dialog only exposes
  // viewer / user / admin — owner is by-transfer only.
  role?: GroupRole
}

export async function createInvite(
  groupId: string,
  req: CreateInviteRequest = {}
): Promise<GroupInvite & { id?: string }> {
  // The BE accepts a missing body as the legacy default (token-only,
  // user role). Send an envelope only when at least one attribute was
  // supplied so the legacy callers keep producing the same wire shape.
  const hasAttrs = !!(req.email || req.role)
  const body = await http.post<InviteResponseEnvelope>(
    `/groups/${encodeURIComponent(groupId)}/invites`,
    hasAttrs
      ? { data: { type: "invites", attributes: { email: req.email, role: req.role } } }
      : undefined
  )
  if (!body.data?.attributes) {
    throw new Error("Create-invite response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

export async function resendInvite(
  groupId: string,
  inviteId: string
): Promise<GroupInvite & { id?: string }> {
  const body = await http.post<InviteResponseEnvelope>(
    `/groups/${encodeURIComponent(groupId)}/invites/${encodeURIComponent(inviteId)}/resend`
  )
  if (!body.data?.attributes) {
    throw new Error("Resend-invite response missing data.attributes")
  }
  return { ...body.data.attributes, id: body.data.id }
}

export async function revokeInvite(groupId: string, inviteId: string): Promise<void> {
  await http.del<void>(
    `/groups/${encodeURIComponent(groupId)}/invites/${encodeURIComponent(inviteId)}`
  )
}
