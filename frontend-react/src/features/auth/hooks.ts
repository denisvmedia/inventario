import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { getAccessToken } from "@/lib/auth-storage"

import {
  forgotPassword,
  getCurrentUser,
  login,
  logout,
  register,
  resendVerification,
  resetPassword,
  verifyEmail,
  type CurrentUser,
  type RegisterRequest,
} from "./api"
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

interface LoginVars {
  email: string
  password: string
}

// Login mutation: on success, seed the cached user so ProtectedRoute can
// short-circuit the boot probe and render the protected tree without a
// /auth/me round-trip. We invalidate the auth namespace afterward so any
// stale "user" data anywhere in the cache settles to the new identity.
export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation<CurrentUser | undefined, Error, LoginVars>({
    mutationFn: ({ email, password }) => login(email, password),
    onSuccess: (user) => {
      if (user) {
        queryClient.setQueryData(authKeys.currentUser(), user)
      }
      // Refresh anything else that depends on auth (e.g. groups list).
      queryClient.invalidateQueries({ queryKey: authKeys.all })
    },
  })
}

export function useRegister() {
  return useMutation<string, Error, RegisterRequest>({
    mutationFn: (req) => register(req),
  })
}

export function useVerifyEmail() {
  return useMutation<string, Error, string>({
    mutationFn: (token) => verifyEmail(token),
  })
}

export function useResendVerification() {
  return useMutation<string, Error, string>({
    mutationFn: (email) => resendVerification(email),
  })
}

export function useForgotPassword() {
  return useMutation<string, Error, string>({
    mutationFn: (email) => forgotPassword(email),
  })
}

interface ResetPasswordVars {
  token: string
  newPassword: string
}

export function useResetPassword() {
  return useMutation<string, Error, ResetPasswordVars>({
    mutationFn: ({ token, newPassword }) => resetPassword(token, newPassword),
  })
}
