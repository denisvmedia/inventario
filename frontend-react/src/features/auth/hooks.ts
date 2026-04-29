import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { getAccessToken } from "@/lib/auth-storage"

import { getCurrentUser, logout, type CurrentUser } from "./api"
import { authKeys } from "./keys"

// Reads the authenticated user. The query only runs when an access token is
// present in localStorage — without a token /auth/me would 401, http.ts would
// try to refresh, the refresh would also 401, and we'd race the route guard
// to /login. Skipping the query for the no-token case lets ProtectedRoute
// handle the redirect cleanly via <Navigate> instead.
export function useCurrentUser() {
  return useQuery<CurrentUser>({
    queryKey: authKeys.currentUser(),
    queryFn: ({ signal }) => getCurrentUser(signal),
    enabled: !!getAccessToken(),
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
      await queryClient.cancelQueries({ queryKey: authKeys.currentUser() })
      const previousUser = queryClient.getQueryData<CurrentUser>(authKeys.currentUser())
      queryClient.removeQueries({ queryKey: authKeys.currentUser(), exact: true })
      return { previousUser }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.previousUser) {
        queryClient.setQueryData(authKeys.currentUser(), ctx.previousUser)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.all })
    },
  })
}
