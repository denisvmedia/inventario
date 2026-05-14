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

export function useRevokeAllOtherSessions() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => revokeAllOtherSessions(),
    onSuccess: () => qc.invalidateQueries({ queryKey: sessionsKeys.all }),
  })
}
