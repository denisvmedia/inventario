// Pure data-layer for the platform admin surface (#1752 foundation).
// Thin wrappers over the generated OpenAPI types for the /api/v1/admin/*
// endpoints — the admin BE foundation (#1745) merged the routes. Hooks
// live in `./hooks.ts`. Mirrors `features/locations/api.ts`.
//
// The /admin/* endpoints are platform-wide: they are NOT under /g/{slug}/,
// so the http client's group rewrite leaves them untouched (no /admin
// prefix in GROUP_SCOPED_PREFIXES).
import { http, HttpError } from "@/lib/http"
import type { Schema } from "@/types"
import type { AdminGroupsParams, AdminTenantsParams, AdminTenantUsersParams } from "./keys"

// A tenant row as returned by GET /admin/tenants — carries computed
// user_count / group_count alongside the tenant identity. The detail
// endpoint (GET /admin/tenants/{id}) returns the very same shape.
export type AdminTenant = Schema<"jsonapi.AdminTenantListItem">

// A user row as returned by GET /admin/tenants/{id}/users — carries the
// computed group_membership_count alongside identity + activity.
export type AdminTenantUser = Schema<"jsonapi.AdminUserListItem">

// A group row as returned by GET /admin/groups — carries computed
// member_count and an owning-tenant chip.
export type AdminGroup = Schema<"jsonapi.AdminGroupListItem">

// A single group as returned by GET /admin/groups/{id} (and by the
// soft-delete DELETE, which echoes the post-transition row). Carries the
// same computed member_count / tenant chip as a list row plus created_by.
export type AdminGroupDetail = Schema<"jsonapi.AdminGroupDetail">

// The full per-user admin detail as returned by GET /admin/users/{id} —
// identity, is_active, last_login_at, group memberships, and the
// `active_session_count` (the BE returns a count, not a session list).
export type AdminUserDetail = Schema<"jsonapi.AdminUserDetail">

// A single group-membership row inside AdminUserDetail.
export type AdminUserGroupMembership = Schema<"jsonapi.AdminUserGroupMembership">

// A group role: viewer | user | admin | owner. Used by the membership
// editor's inline role <Select> and the add-member dialog.
export type GroupRole = Schema<"models.GroupRole">

// A member row as returned by GET /admin/groups/{id}/members — the
// membership identity (group_id, member_user_id, role, joined_at) plus a
// nested `user` chip (id, name, email). Every field is optional in the
// generated types (codegen quirk); the editor guards accordingly.
export type AdminGroupMember = Schema<"jsonapi.AdminGroupMember">

// Body for POST /admin/groups/{id}/members — the BE takes a resolved
// `userID` (NOT an email); the editor resolves email → userID client-side
// via listAdminTenantUsers before calling this.
export type AdminAddMemberRequest = Schema<"apiserver.AdminAddMemberRequest">

// Body for POST /admin/users/{id}/block — `reason` is required (max 500
// chars); `force` overrides the "cannot block another system admin" guard.
export type AdminBlockRequest = Schema<"apiserver.AdminBlockRequest">

// Body for POST /admin/users/{id}/unblock — `reason` is required (max 500).
export type AdminUnblockRequest = Schema<"apiserver.AdminUnblockRequest">

// The post-mutation user snapshot returned by block / unblock — a narrower
// identity view (id, email, name, is_active, is_system_admin, tenant_id).
export type AdminUserView = Schema<"apiserver.AdminUserView">

// Pagination envelope shared by the admin list endpoints.
export type AdminListMeta = Schema<"jsonapi.AdminListMeta">

// The active-impersonation snapshot powering the persistent banner.
export type ImpersonationState = Schema<"apiserver.ImpersonationStateResponse">
export type ImpersonationUser = Schema<"apiserver.ImpersonationUserView">

type AdminTenantsResponse = Schema<"jsonapi.AdminTenantsResponse">
type AdminTenantResponse = Schema<"jsonapi.AdminTenantResponse">
type AdminUsersResponse = Schema<"jsonapi.AdminUsersResponse">
type AdminGroupsResponse = Schema<"jsonapi.AdminGroupsResponse">
type AdminGroupResponse = Schema<"jsonapi.AdminGroupResponse">
type AdminUserResponse = Schema<"jsonapi.AdminUserResponse">
type AdminUserEnvelope = Schema<"apiserver.AdminUserEnvelope">
type AdminGroupMembersResponse = Schema<"jsonapi.AdminGroupMembersResponse">
// The add / role-change endpoints echo a *different* shape from the list:
// a single-resource envelope (`data.attributes`) carrying the membership
// view (group_id, member_user_id, role, tenant_id, joined_at) — NOT the
// list's `AdminGroupMember`. The editor refetches the list afterwards, so
// only the envelope's existence is asserted, not its contents.
type AdminMemberEnvelope = Schema<"apiserver.AdminMemberEnvelope">

export interface AdminTenantsResult {
  tenants: AdminTenant[]
  meta: AdminListMeta
}

export interface AdminTenantUsersResult {
  users: AdminTenantUser[]
  meta: AdminListMeta
}

export interface AdminGroupsResult {
  groups: AdminGroup[]
  meta: AdminListMeta
}

// Lists tenants across the whole platform. Pagination + free-text search
// (?q matches name/slug/domain) + sort are all server-side; the caller
// passes them through `params`.
export async function listAdminTenants(
  params: AdminTenantsParams = {},
  signal?: AbortSignal
): Promise<AdminTenantsResult> {
  const query = new URLSearchParams()
  if (params.page !== undefined) query.set("page", String(params.page))
  if (params.perPage !== undefined) query.set("per_page", String(params.perPage))
  if (params.q) query.set("q", params.q)
  if (params.sort) query.set("sort", params.sort)
  if (params.order) query.set("order", params.order)
  const qs = query.toString()
  const path = qs ? `/admin/tenants?${qs}` : "/admin/tenants"
  const body = await http.get<AdminTenantsResponse>(path, { signal })
  return {
    tenants: body.data ?? [],
    meta: body.meta ?? {},
  }
}

// Reads a single tenant by ID. The BE returns the same AdminTenantListItem
// shape as a list row (computed user_count / group_count, no nested
// users/groups) — those listings live behind their own endpoints.
export async function getAdminTenant(tenantId: string, signal?: AbortSignal): Promise<AdminTenant> {
  const body = await http.get<AdminTenantResponse>(
    `/admin/tenants/${encodeURIComponent(tenantId)}`,
    { signal }
  )
  // The BE returns HTTP 404 for a missing tenant; a 200 with no `data`
  // (or a `data` object lacking an `id`) would be a malformed response.
  // Fail fast instead of masking it — an empty `{}` would otherwise
  // silently render as not-found, hiding a backend bug behind a 404-like
  // UI.
  if (!body.data || !body.data.id) {
    throw new Error(`Admin tenant response for "${tenantId}" is missing its payload`)
  }
  return body.data
}

// Lists the users belonging to one tenant. Pagination + free-text search
// (?q matches email/name) + the tri-state `?is_active` filter + sort are
// all server-side. `is_active` is only sent when explicitly true/false —
// omitting it entirely is the BE's "no filter" signal.
export async function listAdminTenantUsers(
  tenantId: string,
  params: AdminTenantUsersParams = {},
  signal?: AbortSignal
): Promise<AdminTenantUsersResult> {
  const query = new URLSearchParams()
  if (params.page !== undefined) query.set("page", String(params.page))
  if (params.perPage !== undefined) query.set("per_page", String(params.perPage))
  if (params.q) query.set("q", params.q)
  if (params.isActive !== undefined) query.set("is_active", String(params.isActive))
  if (params.sort) query.set("sort", params.sort)
  if (params.order) query.set("order", params.order)
  const qs = query.toString()
  const base = `/admin/tenants/${encodeURIComponent(tenantId)}/users`
  const body = await http.get<AdminUsersResponse>(qs ? `${base}?${qs}` : base, { signal })
  return {
    users: body.data ?? [],
    meta: body.meta ?? {},
  }
}

// Lists location groups across the platform. The tenant detail page pins
// `tenantID` so the Groups tab only shows that tenant's groups; `status`
// is the optional exact-match lifecycle filter.
export async function listAdminGroups(
  params: AdminGroupsParams = {},
  signal?: AbortSignal
): Promise<AdminGroupsResult> {
  const query = new URLSearchParams()
  if (params.tenantID) query.set("tenantID", params.tenantID)
  if (params.page !== undefined) query.set("page", String(params.page))
  if (params.perPage !== undefined) query.set("per_page", String(params.perPage))
  if (params.q) query.set("q", params.q)
  if (params.status) query.set("status", params.status)
  if (params.sort) query.set("sort", params.sort)
  if (params.order) query.set("order", params.order)
  const qs = query.toString()
  const path = qs ? `/admin/groups?${qs}` : "/admin/groups"
  const body = await http.get<AdminGroupsResponse>(path, { signal })
  return {
    groups: body.data ?? [],
    meta: body.meta ?? {},
  }
}

// Reads a single location group by ID. The BE returns the AdminGroupDetail
// shape — the same computed member_count / tenant chip as a list row, plus
// created_by.
export async function getAdminGroup(
  groupId: string,
  signal?: AbortSignal
): Promise<AdminGroupDetail> {
  const body = await http.get<AdminGroupResponse>(`/admin/groups/${encodeURIComponent(groupId)}`, {
    signal,
  })
  // The BE returns HTTP 404 for a missing group; a 200 with no `data`
  // (or a `data` object lacking an `id`) would be a malformed response.
  // Fail fast instead of masking it — an empty `{}` would otherwise
  // silently render as not-found, hiding a backend bug behind a 404-like
  // UI.
  if (!body.data || !body.data.id) {
    throw new Error(`Admin group response for "${groupId}" is missing its payload`)
  }
  return body.data
}

// Reads a single user's full admin detail by ID (GET /admin/users/{id}):
// identity, is_active, last_login_at, group memberships, and the
// active-session count. The BE returns HTTP 404 for a missing user; a 200
// with no `data` (or a `data` object lacking an `id`) is a malformed
// response — fail fast rather than masking it as a 404-like empty state
// (mirrors getAdminTenant).
export async function getAdminUser(userId: string, signal?: AbortSignal): Promise<AdminUserDetail> {
  const body = await http.get<AdminUserResponse>(`/admin/users/${encodeURIComponent(userId)}`, {
    signal,
  })
  if (!body.data || !body.data.id) {
    throw new Error(`Admin user response for "${userId}" is missing its payload`)
  }
  return body.data
}

// Soft-deletes a location group. The BE flips the group to
// `pending_deletion` and returns HTTP 200 with the post-transition
// AdminGroupDetail row — the same shape getAdminGroup returns. The call is
// idempotent: re-deleting an already-`pending_deletion` group also returns
// 200 with the unchanged row, so the caller never has to special-case it.
export async function softDeleteAdminGroup(
  groupId: string,
  signal?: AbortSignal
): Promise<AdminGroupDetail> {
  const body = await http.del<AdminGroupResponse>(`/admin/groups/${encodeURIComponent(groupId)}`, {
    signal,
  })
  // Same fail-fast guard as getAdminGroup: a 200 with no usable `data` is
  // a malformed response, not a successful delete.
  if (!body.data || !body.data.id) {
    throw new Error(`Admin group delete response for "${groupId}" is missing its payload`)
  }
  return body.data
}

// Blocks a user (POST /admin/users/{id}/block). `reason` is required and
// capped at 500 chars by the BE; `force` overrides the "cannot block
// another system admin" guard. Returns the post-transition user snapshot.
// Typed 422 codes the caller branches on: `admin.block.self_blocked`,
// `admin.block.admin_requires_force`, `admin.block.reason_required`,
// `admin.block.reason_too_long`.
export async function blockAdminUser(
  userId: string,
  payload: AdminBlockRequest
): Promise<AdminUserView> {
  const body = await http.post<AdminUserEnvelope>(
    `/admin/users/${encodeURIComponent(userId)}/block`,
    payload
  )
  // The BE returns the post-transition snapshot in `data.attributes`. A
  // 200 with a missing/incomplete envelope is a malformed response — fail
  // fast rather than yielding `{}`, which would otherwise be patched into
  // the user-detail cache as `is_active: undefined` (mirrors getAdminUser).
  const attributes = body.data?.attributes
  if (!attributes || !attributes.id || typeof attributes.is_active !== "boolean") {
    throw new Error(`Admin block response for "${userId}" is missing its payload`)
  }
  return attributes
}

// Unblocks a user (POST /admin/users/{id}/unblock). `reason` is required
// and capped at 500 chars; reason-validation 422 codes are shared with the
// block endpoint (`admin.block.reason_required` / `admin.block.reason_too_long`).
export async function unblockAdminUser(
  userId: string,
  payload: AdminUnblockRequest
): Promise<AdminUserView> {
  const body = await http.post<AdminUserEnvelope>(
    `/admin/users/${encodeURIComponent(userId)}/unblock`,
    payload
  )
  // Fail fast on a missing/incomplete envelope — see blockAdminUser.
  const attributes = body.data?.attributes
  if (!attributes || !attributes.id || typeof attributes.is_active !== "boolean") {
    throw new Error(`Admin unblock response for "${userId}" is missing its payload`)
  }
  return attributes
}

// Lists the members of one location group (GET /admin/groups/{id}/members).
// The BE returns the members in a flat `{ data: [...] }` envelope with no
// pagination or query params; an empty group is `{ "data": [] }` (a valid
// empty state, not an error) and an unknown group is HTTP 404 (surfaces as
// a thrown HttpError the caller branches on). A 200 with no `data` key at
// all is a malformed response — `?? []` would mask it as an empty group,
// so we treat a missing `data` array as a hard error (mirrors the
// fail-fast guards on the detail endpoints above).
export async function listAdminGroupMembers(
  groupId: string,
  signal?: AbortSignal
): Promise<AdminGroupMember[]> {
  const body = await http.get<AdminGroupMembersResponse>(
    `/admin/groups/${encodeURIComponent(groupId)}/members`,
    { signal }
  )
  if (!Array.isArray(body.data)) {
    throw new Error(`Admin group members response for "${groupId}" is missing its payload`)
  }
  return body.data
}

// Adds a user to a group (POST /admin/groups/{id}/members). The BE takes a
// resolved `userID` plus the granted `role`; the membership editor resolves
// the typed email to a userID via listAdminTenantUsers before calling this.
// On success the BE returns the `AdminMemberEnvelope` shape (a single
// `data.attributes` resource) — different from the list's row shape. The
// editor refetches the list on success, so this returns void: the envelope
// is consumed only as a fail-fast "did the write land" check.
//
// Typed 422 codes the caller branches on: `admin.member.tenant_mismatch`,
// `admin.member.invalid_role`, `admin.member.user_required`. An uncoded
// 422 (membership cap reached / already a member) and a 404 (unknown group
// or user) surface via the thrown HttpError's body.
export async function addAdminGroupMember(
  groupId: string,
  payload: AdminAddMemberRequest
): Promise<void> {
  const body = await http.post<AdminMemberEnvelope>(
    `/admin/groups/${encodeURIComponent(groupId)}/members`,
    payload
  )
  // A 201 with no usable envelope is a malformed response — fail fast
  // rather than reporting a phantom success (mirrors blockAdminUser).
  if (!body.data?.id) {
    throw new Error(`Admin add-member response for "${groupId}" is missing its payload`)
  }
}

// Removes a user from a group (DELETE /admin/groups/{id}/members/{userID}).
// The BE returns HTTP 204 with no body. Typed 422 codes the caller branches
// on: `group.last_owner` (removing the last owner) and `group.last_member`
// (removing the last member); a 404 means the user is not a member.
export async function removeAdminGroupMember(groupId: string, userId: string): Promise<void> {
  await http.del<null>(
    `/admin/groups/${encodeURIComponent(groupId)}/members/${encodeURIComponent(userId)}`
  )
}

// Changes a member's role (PATCH /admin/groups/{id}/members/{userID}). On
// success the BE returns the `AdminMemberEnvelope` shape; the editor
// refetches the list afterwards, so this returns void and only fail-fast
// asserts the envelope landed. Typed 422 codes: `group.last_owner`
// (demoting the sole owner) and `admin.member.invalid_role`.
export async function updateAdminGroupMemberRole(
  groupId: string,
  userId: string,
  role: GroupRole
): Promise<void> {
  const body = await http.patch<AdminMemberEnvelope>(
    `/admin/groups/${encodeURIComponent(groupId)}/members/${encodeURIComponent(userId)}`,
    { role }
  )
  if (!body.data?.id) {
    throw new Error(`Admin role-change response for "${groupId}" is missing its payload`)
  }
}

// Reads the active impersonation session for the current caller. The BE
// returns `{ active: false }` with no other fields when no session is in
// progress; the banner uses `active` as its sole render gate.
//
// The endpoint 403s for a plain (non-admin, non-impersonated) user — and
// that is itself a definitive "you are not impersonating anyone" answer,
// not an error condition. We translate the 403 into an inactive state so
// the query resolves cleanly for every authenticated user rather than
// parking in an error state for the non-admin majority. A 401 (genuinely
// signed out) and 5xx still propagate as errors.
export async function getImpersonationState(signal?: AbortSignal): Promise<ImpersonationState> {
  try {
    return await http.get<ImpersonationState>("/admin/impersonation/current", { signal })
  } catch (error) {
    if (error instanceof HttpError && error.status === 403) {
      return { active: false }
    }
    throw error
  }
}
