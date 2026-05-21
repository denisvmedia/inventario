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

// Pagination envelope shared by the admin list endpoints.
export type AdminListMeta = Schema<"jsonapi.AdminListMeta">

// The active-impersonation snapshot powering the persistent banner.
export type ImpersonationState = Schema<"apiserver.ImpersonationStateResponse">
export type ImpersonationUser = Schema<"apiserver.ImpersonationUserView">

type AdminTenantsResponse = Schema<"jsonapi.AdminTenantsResponse">
type AdminTenantResponse = Schema<"jsonapi.AdminTenantResponse">
type AdminUsersResponse = Schema<"jsonapi.AdminUsersResponse">
type AdminGroupsResponse = Schema<"jsonapi.AdminGroupsResponse">

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
