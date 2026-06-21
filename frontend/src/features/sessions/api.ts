// Active-sessions API client (issue #1378). Mirrors the BE shape from
// apiserver/users_me.go — SessionView. The endpoint lives under
// /api/v1/users/me/sessions and intentionally does NOT go through the
// group-scoped rewrite in lib/http (the route is tenant-scoped).
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type SessionView = Schema<"apiserver.SessionView">
export type SessionsListResponse = Schema<"apiserver.SessionsListResponse">

export async function listSessions(signal?: AbortSignal): Promise<SessionsListResponse> {
  // skipGroupRewrite: lib/http rewrites bare resource prefixes (commodities,
  // files, etc.) under /g/{slug}/ when a group is active. /users/me is not
  // in GROUP_SCOPED_PREFIXES so this is currently a no-op, but the flag
  // makes the intent explicit and survives future expansions of the rewrite
  // table.
  return http.get<SessionsListResponse>("/users/me/sessions", { signal, skipGroupRewrite: true })
}

export async function revokeSession(id: string): Promise<void> {
  await http.del(`/users/me/sessions/${encodeURIComponent(id)}`, { skipGroupRewrite: true })
}

// revokeAllOtherSessions revokes every session except the one identified
// as current. The id of the row the list endpoint flagged
// `is_current: true` is REQUIRED and passed via `?keep_id=`: the refresh
// cookie is path-scoped to /api/v1/auth — it isn't sent on
// /users/me/sessions, so the BE can't fall back to hashing the cookie
// here. The parameter is intentionally non-optional so a call site that
// can't resolve a current session (e.g. an impersonation session, which
// has no is_current row) is a compile error rather than a silent
// wipe-all (#2126). The matching BE guard refuses an empty keep_id with
// 400, but we never send that shape from here.
export async function revokeAllOtherSessions(keepSessionId: string): Promise<void> {
  await http.del(`/users/me/sessions?keep_id=${encodeURIComponent(keepSessionId)}`, {
    skipGroupRewrite: true,
  })
}
