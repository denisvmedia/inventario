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
import type { AdminTenantsParams } from "./keys"

// A tenant row as returned by GET /admin/tenants — carries computed
// user_count / group_count alongside the tenant identity.
export type AdminTenant = Schema<"jsonapi.AdminTenantListItem">

// Pagination envelope shared by the admin list endpoints.
export type AdminListMeta = Schema<"jsonapi.AdminListMeta">

// The active-impersonation snapshot powering the persistent banner.
export type ImpersonationState = Schema<"apiserver.ImpersonationStateResponse">
export type ImpersonationUser = Schema<"apiserver.ImpersonationUserView">

type AdminTenantsResponse = Schema<"jsonapi.AdminTenantsResponse">

export interface AdminTenantsResult {
  tenants: AdminTenant[]
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
