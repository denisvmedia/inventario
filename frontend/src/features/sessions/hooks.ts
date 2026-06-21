// TanStack Query hooks for the active-sessions slice (#1378).
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { listSessions, revokeAllOtherSessions, revokeSession } from "./api"
import { sessionsKeys } from "./keys"

export function useSessionsList() {
  return useQuery({
    queryKey: sessionsKeys.list(),
    queryFn: ({ signal }) => listSessions(signal),
    // Sessions are user-facing and need to feel live — a 30s staleTime
    // covers tab refocus while still letting "I just revoked X" reflect
    // immediately via the explicit invalidation in the mutation hooks.
    staleTime: 30 * 1000,
  })
}

export function useRevokeSession() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => revokeSession(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: sessionsKeys.all }),
  })
}

// useRevokeAllOtherSessions takes the id of the session the caller wants
// to keep — the row the list endpoint marked `is_current: true`. The id
// is REQUIRED because the BE needs an explicit signal (the refresh cookie
// is scoped to /api/v1/auth and isn't sent on /users/me/sessions) and
// because firing without one would wipe every session. The caller MUST
// only mutate when an is_current row exists (#2126).
export function useRevokeAllOtherSessions() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (keepSessionId: string) => revokeAllOtherSessions(keepSessionId),
    onSuccess: () => qc.invalidateQueries({ queryKey: sessionsKeys.all }),
  })
}
