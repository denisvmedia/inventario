// TanStack Query hook for the login-history slice (#1379).
import { useQuery } from "@tanstack/react-query"

import { listLoginHistory } from "./api"
import { loginHistoryKeys } from "./keys"

export function useLoginHistory(limit = 100) {
  return useQuery({
    queryKey: loginHistoryKeys.list(limit),
    queryFn: ({ signal }) => listLoginHistory(limit, signal),
    // Short staleTime: a refocus after several minutes should re-pull
    // the list so a new failed-login attempt is visible without a
    // hard reload.
    staleTime: 30 * 1000,
  })
}
