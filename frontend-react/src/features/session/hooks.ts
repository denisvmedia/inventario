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
// UI can show the logged-out shell immediately. If the request fails, restore
// what we had — but in practice a logout error means the session is already
// dead from the user's POV, so we keep them logged out.
export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, void, { previousUser: CurrentUser | undefined }>({
    mutationFn: () => logout(),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: sessionKeys.currentUser() })
      const previousUser = queryClient.getQueryData<CurrentUser>(sessionKeys.currentUser())
      queryClient.setQueryData<CurrentUser | null>(sessionKeys.currentUser(), null)
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
