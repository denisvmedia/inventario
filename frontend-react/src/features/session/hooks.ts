import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { getCurrentUser, logout, type CurrentUser } from "./api"
import { sessionKeys } from "./keys"

// Reads the authenticated user. The HTTP layer attaches the bearer token,
// so a 401 here is real (no session) — TanStack Query surfaces it as `error`
// and the route guard (#1404) maps it to a /login redirect.
export function useCurrentUser() {
  return useQuery<CurrentUser>({
    queryKey: sessionKeys.currentUser(),
    queryFn: ({ signal }) => getCurrentUser(signal),
  })
}

// Optimistic logout: drop the cached user before the server confirms so the
// UI can show the logged-out shell immediately. We `removeQueries` rather
// than `setQueryData(..., null)` so the cache stays type-true to the query's
// declared `CurrentUser` shape — consumers see `data === undefined`, never a
// surprise `null`. If the request fails, restore the snapshot.
export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, void, { previousUser: CurrentUser | undefined }>({
    mutationFn: () => logout(),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: sessionKeys.currentUser() })
      const previousUser = queryClient.getQueryData<CurrentUser>(sessionKeys.currentUser())
      queryClient.removeQueries({ queryKey: sessionKeys.currentUser(), exact: true })
      return { previousUser }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.previousUser) {
        queryClient.setQueryData(sessionKeys.currentUser(), ctx.previousUser)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: sessionKeys.all })
    },
  })
}
