// Personal API token client (#2808). Mirrors apiserver/api_tokens.go. The
// endpoints live under /api/v1/users/me/api-tokens and are tenant-scoped,
// not group-scoped, so the group rewrite in lib/http is skipped.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type APITokenView = Schema<"apiserver.APITokenView">
export type APITokensListResponse = Schema<"apiserver.APITokensListResponse">
export type CreateAPITokenRequest = Schema<"apiserver.CreateAPITokenRequest">
export type CreateAPITokenResponse = Schema<"apiserver.CreateAPITokenResponse">
export type APITokenScope = NonNullable<APITokenView["scope"]>

export const API_TOKEN_SCOPES: readonly APITokenScope[] = ["read_only", "read_write"] as const

export async function listAPITokens(signal?: AbortSignal): Promise<APITokensListResponse> {
  return http.get<APITokensListResponse>("/users/me/api-tokens", { signal, skipGroupRewrite: true })
}

export async function createAPIToken(body: CreateAPITokenRequest): Promise<CreateAPITokenResponse> {
  return http.post<CreateAPITokenResponse>("/users/me/api-tokens", body, { skipGroupRewrite: true })
}

export async function revokeAPIToken(id: string): Promise<void> {
  await http.del(`/users/me/api-tokens/${encodeURIComponent(id)}`, { skipGroupRewrite: true })
}
