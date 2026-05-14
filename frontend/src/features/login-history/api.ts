// Login-history API client (issue #1379).
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type LoginEventView = Schema<"apiserver.LoginEventView">
export type LoginHistoryResponse = Schema<"apiserver.LoginHistoryResponse">

// listLoginHistory caps the BE-side server with ?limit= so callers can
// page-down (the BE caps internally at 500 — anything larger is
// effectively meaningless because of the 90-day retention).
export async function listLoginHistory(
  limit: number,
  signal?: AbortSignal
): Promise<LoginHistoryResponse> {
  const q = limit > 0 ? `?limit=${limit}` : ""
  return http.get<LoginHistoryResponse>(`/users/me/login-history${q}`, {
    signal,
    skipGroupRewrite: true,
  })
}
