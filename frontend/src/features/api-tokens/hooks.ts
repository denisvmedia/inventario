// TanStack Query hooks for personal API tokens (#2808).
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  createAPIToken,
  listAPITokens,
  revokeAPIToken,
  type CreateAPITokenRequest,
  type CreateAPITokenResponse,
} from "./api"
import { apiTokensKeys } from "./keys"

export function useAPITokensList() {
  return useQuery({
    queryKey: apiTokensKeys.list(),
    queryFn: ({ signal }) => listAPITokens(signal),
    // last_used_at moves at most every five minutes on the server, so a
    // short staleTime is plenty to keep the list honest on refocus.
    staleTime: 30 * 1000,
  })
}

// The response carries the token itself. It is handed to the caller and
// never written to the query cache, so it is gone once the reveal closes.
export function useCreateAPIToken() {
  const qc = useQueryClient()
  return useMutation<CreateAPITokenResponse, Error, CreateAPITokenRequest>({
    mutationFn: (body) => createAPIToken(body),
    onSuccess: () => qc.invalidateQueries({ queryKey: apiTokensKeys.all }),
  })
}

export function useRevokeAPIToken() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => revokeAPIToken(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: apiTokensKeys.all }),
  })
}
